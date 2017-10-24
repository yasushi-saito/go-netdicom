// This file defines ServiceProvider (i.e., a DICOM server).

package netdicom

import (
	"fmt"
	"net"

	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-dicom/dicomio"
	"github.com/yasushi-saito/go-netdicom/dimse"
	"github.com/yasushi-saito/go-netdicom/sopclass"
	"v.io/x/lib/vlog"
)

func handleCStore(
	cb CStoreCallback,
	c *dimse.C_STORE_RQ, data []byte,
	cs *serviceCommandState) {
	status := dimse.Status{Status: dimse.StatusUnrecognizedOperation}
	if cb != nil {
		status = cb(
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

func handleCFind(
	params ServiceProviderParams,
	c *dimse.C_FIND_RQ, data []byte,
	cs *serviceCommandState) {
	if params.CFind == nil {
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
	responseCh := make(chan CFindResult, 128)
	go func() {
		params.CFind(cs.context.transferSyntaxUID, c.AffectedSOPClassUID, elems, responseCh)
	}()
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

func handleCMove(
	params ServiceProviderParams,
	c *dimse.C_MOVE_RQ, data []byte,
	cs *serviceCommandState) {
	sendError := func(err error) {
		cs.sendMessage(&dimse.C_MOVE_RSP{
			AffectedSOPClassUID:       c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        dimse.CommandDataSetTypeNull,
			Status:                    dimse.Status{Status: dimse.StatusUnrecognizedOperation, ErrorComment: err.Error()},
		}, nil)
	}
	if params.CMove == nil {
		cs.sendMessage(&dimse.C_MOVE_RSP{
			AffectedSOPClassUID:       c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        dimse.CommandDataSetTypeNull,
			Status:                    dimse.Status{Status: dimse.StatusUnrecognizedOperation, ErrorComment: "No callback found for C-MOVE"},
		}, nil)
		return
	}
	remoteHostPort, ok := params.RemoteAEs[c.MoveDestination]
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
	responseCh := make(chan CMoveResult, 128)
	go func() {
		params.CMove(cs.context.transferSyntaxUID, c.AffectedSOPClassUID, elems, responseCh)
	}()
	// responseCh :=
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
		err := runCStoreOnNewAssociation(params.AETitle, c.MoveDestination, remoteHostPort, resp.DataSet)
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

func handleCGet(
	params ServiceProviderParams,
	c *dimse.C_GET_RQ, data []byte, cs *serviceCommandState) {
	sendError := func(err error) {
		cs.sendMessage(&dimse.C_GET_RSP{
			AffectedSOPClassUID:       c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        dimse.CommandDataSetTypeNull,
			Status:                    dimse.Status{Status: dimse.StatusUnrecognizedOperation, ErrorComment: err.Error()},
		}, nil)
	}
	if params.CGet == nil {
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
	responseCh := make(chan CMoveResult, 128)
	go func() {
		params.CGet(cs.context.transferSyntaxUID, c.AffectedSOPClassUID, elems, responseCh)
	}()
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
		subCs, found := cs.disp.findOrCreateCommand(dimse.NewMessageID(), cs.cm, cs.context /*not used*/)
		vlog.Infof("C-GET: Sending %v using subcommand wl id:%d", resp.Path, subCs.messageID)
		if found {
			panic(subCs)
		}
		err := runCStoreOnAssociation(subCs.upcallCh, subCs.disp.downcallCh, subCs.cm, subCs.messageID, resp.DataSet)
		vlog.Infof("C-GET: Done sending %v using subcommand wl id:%d: %v", resp.Path, subCs.messageID, err)
		defer cs.disp.deleteCommand(subCs)
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

func handleCEcho(
	params ServiceProviderParams,
	c *dimse.C_ECHO_RQ, data []byte,
	cs *serviceCommandState) {
	status := dimse.Status{Status: dimse.StatusUnrecognizedOperation}
	if params.CEcho != nil {
		status = params.CEcho()
	}
	vlog.Infof("Received E-ECHO: context: %+v, status: %+v", cs.context, status)
	resp := &dimse.C_ECHO_RSP{
		MessageIDBeingRespondedTo: c.MessageID,
		CommandDataSetType:        dimse.CommandDataSetTypeNull,
		Status:                    status,
	}
	cs.sendMessage(resp, nil)
}

// ServiceProviderParams defines parameters for ServiceProvider.
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

// DefaultMaxPDUSize is the the PDU size advertized by go-netdicom.
const DefaultMaxPDUSize = 4 << 20

// CStoreCallback is called C-STORE request.  sopInstanceUID is the UID of the
// data.  sopClassUID is the data type requested
// (e.g.,"1.2.840.10008.5.1.4.1.1.1.2"), and transferSyntaxUID is the encoding
// of the data (e.g., "1.2.840.10008.1.2.1").  These args are extracted from the
// request packet.
//
// "data" is the payload, i.e., a sequence of serialized dicom.DataElement
// objects in transferSyntaxUID.  "data" does not contain metadata elements
// (elements whose Tag.Group=2 -- e.g., TransferSyntaxUID and
// MediaStorageSOPClassUID), since they are stripped by the requster (two key
// metadata are passed as sop{Class,Instance)UID).
//
// The function should store encode the sop{Class,InstanceUID} as the DICOM
// header, followed by data. It should return either dimse.Success0 on success,
// or one of CStoreStatus* error codes on errors.
type CStoreCallback func(
	transferSyntaxUID string,
	sopClassUID string,
	sopInstanceUID string,
	data []byte) dimse.Status

// CFindCallback implements a C-FIND handler.  sopClassUID is the data type
// requested (e.g.,"1.2.840.10008.5.1.4.1.1.1.2"), and transferSyntaxUID is the
// data encoding requested (e.g., "1.2.840.10008.1.2.1").  These args are
// extracted from the request packet.
//
// This function should stream CFindResult objects through "ch". The function
// may block.  To report a matched DICOM dataset, the function should send one
// CFindResult with a nonempty Element field. To report multiple DICOM-dataset
// matches, the callback should send multiple CFindResult objects, one for each
// dataset.  The callback must close the channel after it produces all the
// responses.
type CFindCallback func(
	transferSyntaxUID string,
	sopClassUID string,
	filters []*dicom.Element,
	ch chan CFindResult)

// CMoveCallback implements C-MOVE or C-GET handler.  sopClassUID is the data
// type requested (e.g.,"1.2.840.10008.5.1.4.1.1.1.2"), and transferSyntaxUID is
// the data encoding requested (e.g., "1.2.840.10008.1.2.1").  These args are
// extracted from the request packet.
//
// The callback must stream datasets or error to "ch". The callback may
// block. The callback must close the channel after it produces all the
// datasets.
type CMoveCallback func(
	transferSyntaxUID string,
	sopClassUID string,
	filters []*dicom.Element,
	ch chan CMoveResult)

// CEchoCallback implements C-ECHO callback. It typically just returns
// dimse.Success.
type CEchoCallback func() dimse.Status

// ServiceProvider encapsulates the state for DICOM server (provider).
type ServiceProvider struct {
	params   ServiceProviderParams
	listener net.Listener
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
	su, err := NewServiceUser(ServiceUserParams{
		CalledAETitle:  remoteAETitle,
		CallingAETitle: myAETitle,
		SOPClasses:     sopclass.StorageClasses})
	if err != nil {
		return err
	}
	defer su.Release()
	su.Connect(remoteHostPort)
	err = su.CStore(ds)
	vlog.VI(1).Infof("C-STORE subop done: %v", err)
	return err
}

// NewServiceProvider creates a new DICOM server object.  "listenAddr" is the
// TCP address to listen to. E.g., ":1234" will listen to port 1234 at all the
// IP address that this machine can bind to.  Run() will actually start running
// the service.
func NewServiceProvider(params ServiceProviderParams, port string) (*ServiceProvider, error) {
	sp := &ServiceProvider{params: params}
	var err error
	sp.listener, err = net.Listen("tcp", port)
	if err != nil {
		return nil, err
	}
	return sp, nil
}

// RunProviderForConn starts threads for running a DICOM server on "conn". This
// function returns immediately; "conn" will be cleaned up in the background.
func RunProviderForConn(conn net.Conn, params ServiceProviderParams) {
	upcallCh := make(chan upcallEvent, 128)
	disp := newServiceDispatcher()
	disp.registerCallback(dimse.CommandFieldC_STORE_RQ,
		func(msg dimse.Message, data []byte, cs *serviceCommandState) {
			handleCStore(params.CStore, msg.(*dimse.C_STORE_RQ), data, cs)
		})
	disp.registerCallback(dimse.CommandFieldC_FIND_RQ,
		func(msg dimse.Message, data []byte, cs *serviceCommandState) {
			handleCFind(params, msg.(*dimse.C_FIND_RQ), data, cs)
		})
	disp.registerCallback(dimse.CommandFieldC_MOVE_RQ,
		func(msg dimse.Message, data []byte, cs *serviceCommandState) {
			handleCMove(params, msg.(*dimse.C_MOVE_RQ), data, cs)
		})
	disp.registerCallback(dimse.CommandFieldC_GET_RQ,
		func(msg dimse.Message, data []byte, cs *serviceCommandState) {
			handleCGet(params, msg.(*dimse.C_GET_RQ), data, cs)
		})
	disp.registerCallback(dimse.CommandFieldC_ECHO_RQ,
		func(msg dimse.Message, data []byte, cs *serviceCommandState) {
			handleCEcho(params, msg.(*dimse.C_ECHO_RQ), data, cs)
		})
	go runStateMachineForServiceProvider(conn, upcallCh, disp.downcallCh)
	for event := range upcallCh {
		disp.handleEvent(event)
	}
	vlog.VI(2).Info("Finished provider")
}

// Run listens to incoming connections, accepts them, and runs the DICOM
// protocol. This function never returns.
func (sp *ServiceProvider) Run() {
	for {
		conn, err := sp.listener.Accept()
		if err != nil {
			vlog.Errorf("Accept error: %v", err)
			continue
		}
		go func() { RunProviderForConn(conn, sp.params) }()
	}
}

// ListenAddr returns the TCP address that the server is listening on. It is the
// address passed to the NewServiceProvider(), except that if value was of form
// <name>:0, the ":0" part is replaced by the actual port numwber.
func (sp *ServiceProvider) ListenAddr() net.Addr {
	return sp.listener.Addr()
}
