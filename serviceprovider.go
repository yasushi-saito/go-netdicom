// This file defines ServiceProvider (i.e., a DICOM server).

package netdicom

import (
	"fmt"
	"net"
	"sync"

	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-dicom/dicomio"
	"github.com/yasushi-saito/go-netdicom/dimse"
	"github.com/yasushi-saito/go-netdicom/sopclass"
	"v.io/x/lib/vlog"
)

// Per-TCP-connection state for dispatching commands.
type providerCommandDispatcher struct {
	downcallCh chan stateEvent // for sending PDUs to the statemachine.
	params     ServiceProviderParams

	mu             sync.Mutex
	activeCommands map[uint16]*providerCommandState // guarded by mu
}

func (dc *providerCommandDispatcher) findOrCreateCommand(
	messageID uint16,
	cm *contextManager,
	context contextManagerEntry) (*providerCommandState, bool) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	if cs, ok := dc.activeCommands[messageID]; ok {
		return cs, true
	}
	cs := &providerCommandState{
		parent:    dc,
		messageID: messageID,
		cm:        cm,
		context:   context,
		upcallCh:  make(chan upcallEvent, 128),
	}
	dc.activeCommands[messageID] = cs
	vlog.VI(1).Infof("Start provider command %v", messageID)
	return cs, false
}

func (dc *providerCommandDispatcher) deleteCommand(cs *providerCommandState) {
	dc.mu.Lock()
	vlog.VI(1).Infof("Finish provider command %v", cs.messageID)
	if _, ok := dc.activeCommands[cs.messageID]; !ok {
		panic(fmt.Sprintf("cs %+v", cs))
	}
	delete(dc.activeCommands, cs.messageID)
	dc.mu.Unlock()
}

// Per-command-invocation state.
type providerCommandState struct {
	parent    *providerCommandDispatcher // parent dispatcher
	messageID uint16                     // PROVIDER MessageID
	context   contextManagerEntry        // the transfersyntax/sopclass for this command.
	cm        *contextManager            // For looking up context -> transfersyntax/sopclass mappings

	// upcallCh streams PROVIDER command+data for the given messageID.
	upcallCh chan upcallEvent
}

func (cs *providerCommandState) handleCStore(c *dimse.C_STORE_RQ, data []byte) {
	status := dimse.Status{Status: dimse.StatusUnrecognizedOperation}
	if cs.parent.params.CStore != nil {
		status = cs.parent.params.CStore(
			cs.context.transferSyntaxUID,
			c.AffectedSOPClassUID,
			c.AffectedSOPInstanceUID,
			data)
	}
	resp := &dimse.C_STORE_RSP{
		AffectedSOPClassUID:       c.AffectedSOPClassUID,
		MessageIDBeingRespondedTo: c.MessageID,
		CommandDataSetType:        dimse.CommandDataSetTypeNull,
		AffectedSOPInstanceUID:    c.AffectedSOPInstanceUID,
		Status:                    status,
	}
	cs.sendMessage(resp, nil)
}

func (cs *providerCommandState) handleCFind(c *dimse.C_FIND_RQ, data []byte) {
	if cs.parent.params.CFind == nil {
		cs.sendMessage(&dimse.C_FIND_RSP{
			AffectedSOPClassUID:       c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        dimse.CommandDataSetTypeNull,
			Status:                    dimse.Status{Status: dimse.StatusUnrecognizedOperation, ErrorComment: "No callback found for C-FIND"},
		}, nil)
		return
	}
	elems, err := readElementsInBytes(data, cs.context.transferSyntaxUID)
	if err != nil {
		cs.sendMessage(&dimse.C_FIND_RSP{
			AffectedSOPClassUID:       c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        dimse.CommandDataSetTypeNull,
			Status:                    dimse.Status{Status: dimse.StatusUnrecognizedOperation, ErrorComment: err.Error()},
		}, nil)
		return
	}
	vlog.VI(1).Infof("C-FIND-RQ payload: %s", elementsString(elems))

	status := dimse.Status{Status: dimse.StatusSuccess}
	responseCh := cs.parent.params.CFind(cs.context.transferSyntaxUID, c.AffectedSOPClassUID, elems)
	for resp := range responseCh {
		if resp.Err != nil {
			status = dimse.Status{
				Status:       dimse.CFindUnableToProcess,
				ErrorComment: resp.Err.Error(),
			}
			break
		}
		vlog.VI(1).Infof("C-FIND-RSP: %s", elementsString(resp.Elements))
		payload, err := writeElementsToBytes(resp.Elements, cs.context.transferSyntaxUID)
		if err != nil {
			vlog.Errorf("C-FIND: encode error %v", err)
			status = dimse.Status{
				Status:       dimse.CFindUnableToProcess,
				ErrorComment: err.Error(),
			}
			break
		}
		cs.sendMessage(&dimse.C_FIND_RSP{
			AffectedSOPClassUID:       c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        dimse.CommandDataSetTypeNonNull,
			Status:                    dimse.Status{Status: dimse.StatusPending},
		}, payload)
	}
	cs.sendMessage(&dimse.C_FIND_RSP{
		AffectedSOPClassUID:       c.AffectedSOPClassUID,
		MessageIDBeingRespondedTo: c.MessageID,
		CommandDataSetType:        dimse.CommandDataSetTypeNull,
		Status:                    status}, nil)
	// Drain the responses in case of errors
	for _ = range responseCh {
	}
}

func (cs *providerCommandState) handleCMove(c *dimse.C_MOVE_RQ, data []byte) {
	sendError := func(err error) {
		cs.sendMessage(&dimse.C_MOVE_RSP{
			AffectedSOPClassUID:       c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        dimse.CommandDataSetTypeNull,
			Status:                    dimse.Status{Status: dimse.StatusUnrecognizedOperation, ErrorComment: err.Error()},
		}, nil)
	}
	if cs.parent.params.CMove == nil {
		cs.sendMessage(&dimse.C_MOVE_RSP{
			AffectedSOPClassUID:       c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        dimse.CommandDataSetTypeNull,
			Status:                    dimse.Status{Status: dimse.StatusUnrecognizedOperation, ErrorComment: "No callback found for C-MOVE"},
		}, nil)
		return
	}
	remoteHostPort, ok := cs.parent.params.RemoteAEs[c.MoveDestination]
	if !ok {
		sendError(fmt.Errorf("C-MOVE destination '%v' not registered in the server", c.MoveDestination))
		return
	}
	elems, err := readElementsInBytes(data, cs.context.transferSyntaxUID)
	if err != nil {
		sendError(err)
		return
	}
	vlog.VI(1).Infof("C-MOVE-RQ payload: %s", elementsString(elems))
	responseCh := cs.parent.params.CMove(cs.context.transferSyntaxUID, c.AffectedSOPClassUID, elems)
	status := dimse.Status{Status: dimse.StatusSuccess}
	var numSuccesses, numFailures uint16
	for resp := range responseCh {
		if resp.Err != nil {
			status = dimse.Status{
				Status:       dimse.CFindUnableToProcess,
				ErrorComment: resp.Err.Error(),
			}
			break
		}
		vlog.Infof("C-MOVE: Sending %v to %v(%s)", resp.Path, c.MoveDestination, remoteHostPort)
		err := runCStoreOnNewAssociation(cs.parent.params.AETitle, c.MoveDestination, remoteHostPort, resp.DataSet)
		if err != nil {
			vlog.Errorf("C-MOVE: C-store of %v to %v(%v) failed: %v", resp.Path, c.MoveDestination, remoteHostPort, err)
			numFailures++
		} else {
			numSuccesses++
		}
		cs.sendMessage(&dimse.C_MOVE_RSP{
			AffectedSOPClassUID:            c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo:      c.MessageID,
			CommandDataSetType:             dimse.CommandDataSetTypeNull,
			NumberOfRemainingSuboperations: uint16(resp.Remaining),
			NumberOfCompletedSuboperations: numSuccesses,
			NumberOfFailedSuboperations:    numFailures,
			Status: dimse.Status{Status: dimse.StatusPending},
		}, nil)
	}
	cs.sendMessage(&dimse.C_MOVE_RSP{
		AffectedSOPClassUID:            c.AffectedSOPClassUID,
		MessageIDBeingRespondedTo:      c.MessageID,
		CommandDataSetType:             dimse.CommandDataSetTypeNull,
		NumberOfCompletedSuboperations: numSuccesses,
		NumberOfFailedSuboperations:    numFailures,
		Status: status}, nil)
	// Drain the responses in case of errors
	for _ = range responseCh {
	}
}

func (cs *providerCommandState) handleCGet(c *dimse.C_GET_RQ, data []byte) {
	sendError := func(err error) {
		cs.sendMessage(&dimse.C_GET_RSP{
			AffectedSOPClassUID:       c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        dimse.CommandDataSetTypeNull,
			Status:                    dimse.Status{Status: dimse.StatusUnrecognizedOperation, ErrorComment: err.Error()},
		}, nil)
	}
	if cs.parent.params.CGet == nil {
		cs.sendMessage(&dimse.C_GET_RSP{
			AffectedSOPClassUID:       c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        dimse.CommandDataSetTypeNull,
			Status:                    dimse.Status{Status: dimse.StatusUnrecognizedOperation, ErrorComment: "No callback found for C-GET"},
		}, nil)
		return
	}
	elems, err := readElementsInBytes(data, cs.context.transferSyntaxUID)
	if err != nil {
		sendError(err)
		return
	}
	vlog.VI(1).Infof("C-GET-RQ payload: %s", elementsString(elems))
	responseCh := cs.parent.params.CGet(cs.context.transferSyntaxUID, c.AffectedSOPClassUID, elems)
	status := dimse.Status{Status: dimse.StatusSuccess}
	var numSuccesses, numFailures uint16
	for resp := range responseCh {
		if resp.Err != nil {
			status = dimse.Status{
				Status:       dimse.CFindUnableToProcess,
				ErrorComment: resp.Err.Error(),
			}
			break
		}
		subCs, found := cs.parent.findOrCreateCommand(dimse.NewMessageID(), cs.cm, cs.context /*not used*/)
		vlog.Infof("C-GET: Sending %v using subcommand wl id:%d", resp.Path, subCs.messageID)
		if found {
			panic(subCs)
		}
		err := runCStoreOnAssociation(subCs.upcallCh, subCs.parent.downcallCh, subCs.cm, subCs.messageID, resp.DataSet)
		vlog.Infof("C-GET: Done sending %v using subcommand wl id:%d: %v", resp.Path, subCs.messageID, err)
		defer cs.parent.deleteCommand(subCs)
		if err != nil {
			vlog.Errorf("C-GET: C-store of %v failed: %v", resp.Path, err)
			numFailures++
		} else {
			vlog.Infof("C-GET: Sent %v", resp.Path)
			numSuccesses++
		}
		cs.sendMessage(&dimse.C_GET_RSP{
			AffectedSOPClassUID:            c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo:      c.MessageID,
			CommandDataSetType:             dimse.CommandDataSetTypeNull,
			NumberOfRemainingSuboperations: uint16(resp.Remaining),
			NumberOfCompletedSuboperations: numSuccesses,
			NumberOfFailedSuboperations:    numFailures,
			Status: dimse.Status{Status: dimse.StatusPending},
		}, nil)
	}
	cs.sendMessage(&dimse.C_GET_RSP{
		AffectedSOPClassUID:            c.AffectedSOPClassUID,
		MessageIDBeingRespondedTo:      c.MessageID,
		CommandDataSetType:             dimse.CommandDataSetTypeNull,
		NumberOfCompletedSuboperations: numSuccesses,
		NumberOfFailedSuboperations:    numFailures,
		Status: status}, nil)
	// Drain the responses in case of errors
	for _ = range responseCh {
	}
}

func (cs *providerCommandState) handleCEcho(c *dimse.C_ECHO_RQ) {
	status := dimse.Status{Status: dimse.StatusUnrecognizedOperation}
	if cs.parent.params.CEcho != nil {
		status = cs.parent.params.CEcho()
	}
	resp := &dimse.C_ECHO_RSP{
		MessageIDBeingRespondedTo: c.MessageID,
		CommandDataSetType:        dimse.CommandDataSetTypeNull,
		Status:                    status,
	}
	cs.sendMessage(resp, nil)
}

func (cs *providerCommandState) sendMessage(resp dimse.Message, data []byte) {
	vlog.VI(1).Infof("Sending PROVIDER message: %v %v", resp, cs.parent)
	payload := &stateEventDIMSEPayload{
		abstractSyntaxName: cs.context.abstractSyntaxUID,
		command:            resp,
		data:               data,
	}
	cs.parent.downcallCh <- stateEvent{
		event:        evt09,
		pdu:          nil,
		conn:         nil,
		dimsePayload: payload,
	}
}

type ServiceProviderParams struct {
	// The application-entity title of the server. Must be nonempty
	AETitle string

	// Names of remote AEs and their host:ports. Used only by C-MOVE. This
	// map should be nonempty iff the server supports CMove.
	RemoteAEs map[string]string

	// Called on C_ECHO request. If nil, a C-ECHO call will produce an error response.
	//
	// TODO(saito) Support a default C-ECHO callback?
	CEcho CEchoCallback

	// Called on C_FIND request.
	// If CFindCallback=nil, a C-FIND call will produce an error response.
	CFind CFindCallback

	// CMove is called on C_MOVE request.
	CMove CMoveCallback

	// CGet is called on C_GET request. The only difference between cmove
	// and cget is that cget uses the same connection to send images back to
	// the requester. Generally you shuold set the same function to CMove
	// and CGet.
	CGet CMoveCallback

	// If CStoreCallback=nil, a C-STORE call will produce an error response.
	CStore CStoreCallback
}

const DefaultMaxPDUSize = 4 << 20

// CStoreCallback is called C-STORE request.  sopInstanceUID are the IDs of the
// data.  sopClassUID is the data type requested
// (e.g.,"1.2.840.10008.5.1.4.1.1.1.2"), and transferSyntaxUID is the data
// encoding requested (e.g., "1.2.840.10008.1.2.1").  These args come from the
// request packat.
//
// "data" is the payload, i.e., a sequence of serialized
// dicom.DataElement objects.  Note that "data" usually does not contain
// metadata elements (elements whose tag.group=2 -- those include
// TransferSyntaxUID and MediaStorageSOPClassUID), since they are
// stripped by the requstor (two key metadata are passed as
// sop{Class,Instance)UID).
//
// The handler should store encode the sop{Class,InstanceUID} as the
//DICOM header, followed by data. It should return either 0 on success,
//or one of CStoreStatus* error codes.
type CStoreCallback func(
	transferSyntaxUID string,
	sopClassUID string,
	sopInstanceUID string,
	data []byte) dimse.Status

// CFindCallback implements a C-FIND handler.  sopClassUID is the data type
// requested (e.g.,"1.2.840.10008.5.1.4.1.1.1.2"), and transferSyntaxUID is the
// data encoding requested (e.g., "1.2.840.10008.1.2.1").  hese args come from
// the request packat.
//
// This function should create and return a
// channel that streams CFindResult objects. To report a matched DICOM dataset,
// the callback should send one CFindResult with nonempty Element field. To
// report multiple DICOM-dataset matches, the callback should send multiple
// CFindResult objects, one for each dataset.  The callback must close the
// channel after it produces all the responses.
type CFindCallback func(
	transferSyntaxUID string,
	sopClassUID string,
	filters []*dicom.Element) chan CFindResult

// CMoveCallback implements C-MOVE or C-GET handler.  sopClassUID is the data
// type requested (e.g.,"1.2.840.10008.5.1.4.1.1.1.2"), and transferSyntaxUID is
// the data encoding requested (e.g., "1.2.840.10008.1.2.1").  hese args come
// from the request packat.
//
// On return, it should return a channel that streams
// datasets to be sent to the remote client.  The callback must close the
// channel after it produces all the datasets.
type CMoveCallback func(
	transferSyntaxUID string,
	sopClassUID string,
	filters []*dicom.Element) chan CMoveResult

// CEchoCallback implements C-ECHO callback. It typically just returns
// dimse.Success.
type CEchoCallback func() dimse.Status

// ServiceProvider encapsulates the state for DICOM server (provider).
type ServiceProvider struct {
	params ServiceProviderParams
}

func writeElementsToBytes(elems []*dicom.Element, transferSyntaxUID string) ([]byte, error) {
	dataEncoder := dicomio.NewBytesEncoderWithTransferSyntax(transferSyntaxUID)
	for _, elem := range elems {
		dicom.WriteElement(dataEncoder, elem)
	}
	if err := dataEncoder.Error(); err != nil {
		return nil, err
	}
	return dataEncoder.Bytes(), nil
}

func readElementsInBytes(data []byte, transferSyntaxUID string) ([]*dicom.Element, error) {
	decoder := dicomio.NewBytesDecoderWithTransferSyntax(data, transferSyntaxUID)
	var elems []*dicom.Element
	for decoder.Len() > 0 {
		elem := dicom.ReadElement(decoder, dicom.ReadOptions{})
		vlog.VI(1).Infof("C-FIND: Read elem: %v, err %v", elem, decoder.Error())
		if decoder.Error() != nil {
			break
		}
		elems = append(elems, elem)
	}
	if decoder.Error() != nil {
		return nil, decoder.Error()
	}
	return elems, nil
}

func elementsString(elems []*dicom.Element) string {
	s := "["
	for i, elem := range elems {
		if i > 0 {
			s += ", "
		}
		s += elem.String()
	}
	return s + "]"
}

// Send "ds" to remoteHostPort using C-STORE. Called as part of C-MOVE.
func runCStoreOnNewAssociation(myAETitle, remoteAETitle, remoteHostPort string, ds *dicom.DataSet) error {
	params, err := NewServiceUserParams(remoteAETitle, myAETitle, sopclass.StorageClasses, nil)
	if err != nil {
		return err
	}
	su := NewServiceUser(params)
	defer su.Release()
	su.Connect(remoteHostPort)
	err = su.CStore(ds)
	vlog.VI(1).Infof("C-STORE subop done: %v", err)
	return err
}

func (dh *providerCommandDispatcher) handleEvent(event upcallEvent) {
	context, err := event.cm.lookupByContextID(event.contextID)
	if err != nil {
		vlog.Infof("Invalid context ID %d: %v", event.contextID, err)
		dh.downcallCh <- stateEvent{event: evt19, pdu: nil, err: err}
		return
	}
	messageID := event.command.GetMessageID()
	dc, found := dh.findOrCreateCommand(messageID, event.cm, context)
	if found {
		vlog.VI(1).Infof("Forwarding command to existing command: %+v", event.command, dc)
		dc.upcallCh <- event
		vlog.VI(1).Infof("Done forwarding command to existing command: %+v", event.command, dc)
		return
	}
	go func() {
		defer dh.deleteCommand(dc)
		switch c := event.command.(type) {
		case *dimse.C_STORE_RQ:
			dc.handleCStore(c, event.data)
		case *dimse.C_FIND_RQ:
			dc.handleCFind(c, event.data)
		case *dimse.C_MOVE_RQ:
			dc.handleCMove(c, event.data)
		case *dimse.C_GET_RQ:
			dc.handleCGet(c, event.data)
		case *dimse.C_ECHO_RQ:
			dc.handleCEcho(c)
		default:
			// TODO: handle errors properly.
			vlog.Fatalf("Unknown PROVIDER message type: %v", c)
		}
	}()
}

// NewServiceProvider creates a new DICOM server object. Run() will actually
// start running the service.
func NewServiceProvider(params ServiceProviderParams) *ServiceProvider {
	sp := &ServiceProvider{params: params}
	return sp
}

// RunProviderForConn starts threads for running a DICOM server on "conn". This
// function returns immediately; "conn" will be cleaned up in the background.
func RunProviderForConn(conn net.Conn, params ServiceProviderParams) {
	upcallCh := make(chan upcallEvent, 128)
	dc := providerCommandDispatcher{
		downcallCh:     make(chan stateEvent, 128),
		params:         params,
		activeCommands: make(map[uint16]*providerCommandState),
	}

	go runStateMachineForServiceProvider(conn, upcallCh, dc.downcallCh)
	handshakeCompleted := false
	for event := range upcallCh {
		if event.eventType == upcallEventHandshakeCompleted {
			doassert(!handshakeCompleted)
			handshakeCompleted = true
			continue
		}
		doassert(event.eventType == upcallEventData)
		doassert(event.command != nil)
		doassert(handshakeCompleted == true)
		dc.handleEvent(event)
	}
	vlog.VI(2).Info("Finished provider")
}

// Run listens to incoming connections, accepts them, and runs the DICOM
// protocol. This function never returns unless it fails to listen.
// "listenAddr" is the TCP address to listen to. E.g., ":1234" will listen to
// port 1234 at all the IP address that this machine can bind to.
func (sp *ServiceProvider) Run(listenAddr string) error {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			vlog.Errorf("Accept error: %v", err)
			continue
		}
		go func() { RunProviderForConn(conn, sp.params) }()
	}
}
