package netdicom

// This file defines ServiceProvider (i.e., a DICOM server).

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
type dimseCommandDispatcher struct {
	downcallCh chan stateEvent // for sending PDUs to the statemachine.
	params     ServiceProviderParams

	mu             sync.Mutex
	activeCommands map[uint16]*dimseCommandState // guarded by mu
}

func (dc *dimseCommandDispatcher) findOrCreateCommand(
	messageID uint16,
	cm *contextManager,
	context contextManagerEntry) (*dimseCommandState, bool) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	if cs, ok := dc.activeCommands[messageID]; ok {
		return cs, true
	}
	cs := &dimseCommandState{
		parent:    dc,
		messageID: messageID,
		cm:        cm,
		params:    &dc.params,
		context:   context,
		upcallCh:  make(chan upcallEvent, 128),
	}
	dc.activeCommands[messageID] = cs
	vlog.VI(1).Infof("Start dimse command %v", messageID)
	return cs, false
}

func (dc *dimseCommandDispatcher) deleteCommand(cs *dimseCommandState) {
	dc.mu.Lock()
	vlog.VI(1).Infof("Finish dimse command %v", cs.messageID)
	if _, ok := dc.activeCommands[cs.messageID]; !ok {
		panic(fmt.Sprintf("cs %+v", cs))
	}
	delete(dc.activeCommands, cs.messageID)
	dc.mu.Unlock()
}

// Per-command-invocation state.
type dimseCommandState struct {
	parent *dimseCommandDispatcher // parent dispatcher

	messageID uint16 // DIMSE MessageID
	cm        *contextManager
	params    *ServiceProviderParams
	context   contextManagerEntry

	// upcallCh streams DIMSE command+data for the same messageID.
	upcallCh chan upcallEvent
}

func (cs *dimseCommandState) handleCStore(c *dimse.C_STORE_RQ, data []byte) {
	status := dimse.Status{Status: dimse.StatusUnrecognizedOperation}
	if cs.params.CStore != nil {
		status = cs.params.CStore(
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

func (cs *dimseCommandState) handleCFind(c *dimse.C_FIND_RQ, data []byte) {
	if cs.params.CFind == nil {
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
	responseCh := cs.params.CFind(cs.context.transferSyntaxUID, c.AffectedSOPClassUID, elems)
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

func (cs *dimseCommandState) handleCMove(c *dimse.C_MOVE_RQ, data []byte) {
	sendError := func(err error) {
		cs.sendMessage(&dimse.C_MOVE_RSP{
			AffectedSOPClassUID:       c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        dimse.CommandDataSetTypeNull,
			Status:                    dimse.Status{Status: dimse.StatusUnrecognizedOperation, ErrorComment: err.Error()},
		}, nil)
	}
	if cs.params.CMove == nil {
		cs.sendMessage(&dimse.C_MOVE_RSP{
			AffectedSOPClassUID:       c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        dimse.CommandDataSetTypeNull,
			Status:                    dimse.Status{Status: dimse.StatusUnrecognizedOperation, ErrorComment: "No callback found for C-MOVE"},
		}, nil)
		return
	}
	remoteHostPort, ok := cs.params.RemoteAEs[c.MoveDestination]
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
	responseCh := cs.params.CMove(cs.context.transferSyntaxUID, c.AffectedSOPClassUID, elems)
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
		err := runCStoreOnNewAssociation(cs.params.AETitle, c.MoveDestination, remoteHostPort, resp.DataSet)
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

func (cs *dimseCommandState) handleCGet(c *dimse.C_GET_RQ, data []byte) {
	sendError := func(err error) {
		cs.sendMessage(&dimse.C_GET_RSP{
			AffectedSOPClassUID:       c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        dimse.CommandDataSetTypeNull,
			Status:                    dimse.Status{Status: dimse.StatusUnrecognizedOperation, ErrorComment: err.Error()},
		}, nil)
	}
	if cs.params.CGet == nil {
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
	responseCh := cs.params.CGet(cs.context.transferSyntaxUID, c.AffectedSOPClassUID, elems)
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

func (cs *dimseCommandState) handleCEcho(c *dimse.C_ECHO_RQ) {
	status := dimse.Status{Status: dimse.StatusUnrecognizedOperation}
	if cs.params.CEcho != nil {
		status = cs.params.CEcho()
	}
	resp := &dimse.C_ECHO_RSP{
		MessageIDBeingRespondedTo: c.MessageID,
		CommandDataSetType:        dimse.CommandDataSetTypeNull,
		Status:                    status,
	}
	cs.sendMessage(resp, nil)
}

func (cs *dimseCommandState) sendMessage(resp dimse.Message, data []byte) {
	vlog.VI(1).Infof("Sending DIMSE message: %v %v", resp, cs.parent)
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
	AETitle   string            // The title of the provider. Must be nonempty
	RemoteAEs map[string]string // Names of remote AEs and their host:ports. Used by C-MOVE.

	// Called on C_ECHO request. If nil, a C-ECHO call will produce an error response.
	CEcho CEchoCallback

	// Called on C_FIND request. It should create and return a channel that
	// streams CFindResult objects. To report a matched DICOM dataset, the
	// callback should send one CFindResult with nonempty Element field. To
	// report multiple DICOM-dataset matches, the callback should send
	// multiple CFindResult objects, one for each dataset.  The callback
	// must close the channel after it produces all the responses.
	//
	// If CFindCallback=nil, a C-FIND call will produce an error response.
	CFind CFindCallback

	// CMove is called on C_MOVE request. On return:
	//
	// - numMatches should be either the total number of datasets to be
	// sent, or -1 when the number is unknown.
	//
	// - ch should be a channel that streams datasets to be sent to the
	// remoteAEHostPort.  The callback must close the channel after it
	// produces all the datasets.
	CMove CMoveCallback

	// CGet is called on C_GET request. The only difference between cmove
	// and cget is that cget uses the same connection to send images back to
	// the requester. Thus, it's ok to set the same callback for CMove and
	// CGet.
	CGet CMoveCallback

	// Called on receiving a C_STORE_RQ message.  sopClassUID and
	// sopInstanceUID are the IDs of the data. They are from the C-STORE
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
	// DICOM header, followed by data. It should return either 0 on success,
	// or one of CStoreStatus* error codes.
	//
	// If CStoreCallback=nil, a C-STORE call will produce an error response.
	CStore CStoreCallback

	// TODO(saito) Implement C-GET, etc.
}

const DefaultMaxPDUSize = 4 << 20

type CStoreCallback func(
	transferSyntaxUID string,
	sopClassUID string,
	sopInstanceUID string,
	data []byte) dimse.Status

type CFindCallback func(
	transferSyntaxUID string,
	sopClassUID string,
	filters []*dicom.Element) chan CFindResult

type CMoveCallback func(
	transferSyntaxUID string,
	sopClassUID string,
	filters []*dicom.Element) chan CMoveResult

type CEchoCallback func() dimse.Status

// Encapsulates the state for DICOM server (provider).
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
		vlog.Infof("C-FIND: Read elem: %v, err %v", elem, decoder.Error())
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
	vlog.Infof("C-STORE subop done: %v", err)
	return err
}

func (dh *dimseCommandDispatcher) handleEvent(event upcallEvent) {
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
	vlog.VI(1).Infof("Receive DIMSE command: %v", event.command)
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
			vlog.Fatalf("Unknown DIMSE message type: %v", c)
		}
		vlog.VI(1).Infof("Finished DIMSE command: %v", event.command)
	}()
}

func NewServiceProvider(params ServiceProviderParams) *ServiceProvider {
	sp := &ServiceProvider{params: params}
	return sp
}

// Start threads for handling "conn". This function returns immediately; "conn"
// will be cleaned up in the background.
func RunProviderForConn(conn net.Conn, params ServiceProviderParams) {
	upcallCh := make(chan upcallEvent, 128)
	dc := dimseCommandDispatcher{
		downcallCh:     make(chan stateEvent, 128),
		params:         params,
		activeCommands: make(map[uint16]*dimseCommandState),
	}

	go runStateMachineForServiceProvider(conn, upcallCh, dc.downcallCh)
	handshakeCompleted := false
	for event := range upcallCh {
		if event.eventType == upcallEventHandshakeCompleted {
			doassert(!handshakeCompleted)
			handshakeCompleted = true
			vlog.VI(2).Infof("handshake completed")
			continue
		}
		doassert(event.eventType == upcallEventData)
		doassert(event.command != nil)
		doassert(handshakeCompleted == true)
		vlog.Infof("Handle event: %v", event.command)
		dc.handleEvent(event)
		vlog.Infof("Finish handle event: %v", event.command)
	}
	vlog.VI(2).Info("Finished provider")
}

// Listen to incoming connections, accept them, and run the DICOM protocol. This
// function never returns unless it fails to listen.  "listenAddr" is the TCP
// address to listen to. E.g., ":1234" will listen to port 1234 at all the IP
// address that this machine can bind to.
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
