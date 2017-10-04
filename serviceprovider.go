package netdicom

// This file defines ServiceProvider (i.e., a DICOM server).

import (
	"fmt"
	"net"

	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-dicom/dicomio"
	"github.com/yasushi-saito/go-netdicom/dimse"
	"github.com/yasushi-saito/go-netdicom/sopclass"
	"v.io/x/lib/vlog"
)

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

func onDIMSECommand(
	upcallCh chan upcallEvent,
	downcallCh chan stateEvent,
	cm *contextManager,
	contextID byte,
	msg dimse.Message,
	data []byte,
	params ServiceProviderParams) {
	context, err := cm.lookupByContextID(contextID)
	if err != nil {
		vlog.Infof("Invalid context ID %d: %v", contextID, err)
		downcallCh <- stateEvent{event: evt19, pdu: nil, err: err}
		return
	}
	var sendResponse = func(resp dimse.Message) {
		vlog.VI(1).Infof("DIMSE resp: %v", resp)
		e := dicomio.NewBytesEncoder(nil, dicomio.UnknownVR)
		dimse.EncodeMessage(e, resp)
		bytes := e.Bytes()
		downcallCh <- stateEvent{
			event: evt09,
			pdu:   nil,
			conn:  nil,
			dataPayload: &stateEventDataPayload{
				abstractSyntaxName: context.abstractSyntaxUID,
				command:            true,
				data:               bytes},
		}
	}
	var sendData = func(bytes []byte) {
		downcallCh <- stateEvent{
			event: evt09,
			pdu:   nil,
			conn:  nil,
			dataPayload: &stateEventDataPayload{
				abstractSyntaxName: context.abstractSyntaxUID,
				command:            false,
				data:               bytes},
		}
	}

	vlog.VI(1).Infof("DIMSE request: %s data %d bytes context %+v", msg.String(), len(data), context)
	switch c := msg.(type) {
	case *dimse.C_STORE_RQ:
		status := dimse.Status{Status: dimse.StatusUnrecognizedOperation}
		if params.CStore != nil {
			status = params.CStore(
				context.transferSyntaxUID,
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
		sendResponse(resp)
	case *dimse.C_FIND_RQ:
		if params.CFind == nil {
			sendResponse(&dimse.C_FIND_RSP{
				AffectedSOPClassUID:       c.AffectedSOPClassUID,
				MessageIDBeingRespondedTo: c.MessageID,
				CommandDataSetType:        dimse.CommandDataSetTypeNull,
				Status:                    dimse.Status{Status: dimse.StatusUnrecognizedOperation, ErrorComment: "No callback found for C-FIND"},
			})
			break
		}
		elems, err := readElementsInBytes(data, context.transferSyntaxUID)
		if err != nil {
			sendResponse(&dimse.C_FIND_RSP{
				AffectedSOPClassUID:       c.AffectedSOPClassUID,
				MessageIDBeingRespondedTo: c.MessageID,
				CommandDataSetType:        dimse.CommandDataSetTypeNull,
				Status:                    dimse.Status{Status: dimse.StatusUnrecognizedOperation, ErrorComment: err.Error()},
			})
			break
		}
		vlog.VI(1).Infof("C-FIND-RQ payload: %s", elementsString(elems))

		status := dimse.Status{Status: dimse.StatusSuccess}
		responseCh := params.CFind(context.transferSyntaxUID, c.AffectedSOPClassUID, elems)
		for resp := range responseCh {
			if resp.Err != nil {
				status = dimse.Status{
					Status:       dimse.CFindUnableToProcess,
					ErrorComment: resp.Err.Error(),
				}
				break
			}
			vlog.VI(1).Infof("C-FIND-RSP: %s", elementsString(resp.Elements))
			payload, err := writeElementsToBytes(resp.Elements, context.transferSyntaxUID)
			if err != nil {
				vlog.Errorf("C-FIND: encode error %v", err)
				status = dimse.Status{
					Status:       dimse.CFindUnableToProcess,
					ErrorComment: err.Error(),
				}
				break
			}
			sendResponse(&dimse.C_FIND_RSP{
				AffectedSOPClassUID:       c.AffectedSOPClassUID,
				MessageIDBeingRespondedTo: c.MessageID,
				CommandDataSetType:        dimse.CommandDataSetTypeNonNull,
				Status:                    dimse.Status{Status: dimse.StatusPending},
			})
			sendData(payload)
		}
		sendResponse(&dimse.C_FIND_RSP{
			AffectedSOPClassUID:       c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        dimse.CommandDataSetTypeNull,
			Status:                    status})
		// Drain the responses in case of errors
		for _ = range responseCh {
		}

	case *dimse.C_MOVE_RQ:
		sendError := func(err error) {
			sendResponse(&dimse.C_MOVE_RSP{
				AffectedSOPClassUID:       c.AffectedSOPClassUID,
				MessageIDBeingRespondedTo: c.MessageID,
				CommandDataSetType:        dimse.CommandDataSetTypeNull,
				Status:                    dimse.Status{Status: dimse.StatusUnrecognizedOperation, ErrorComment: err.Error()},
			})
		}
		if params.CMove == nil {
			sendResponse(&dimse.C_MOVE_RSP{
				AffectedSOPClassUID:       c.AffectedSOPClassUID,
				MessageIDBeingRespondedTo: c.MessageID,
				CommandDataSetType:        dimse.CommandDataSetTypeNull,
				Status:                    dimse.Status{Status: dimse.StatusUnrecognizedOperation, ErrorComment: "No callback found for C-MOVE"},
			})
			break
		}
		remoteHostPort, ok := params.RemoteAEs[c.MoveDestination]
		if !ok {
			sendError(fmt.Errorf("C-MOVE destination '%v' not registered in the server", c.MoveDestination))
			break
		}
		elems, err := readElementsInBytes(data, context.transferSyntaxUID)
		if err != nil {
			sendError(err)
			break
		}
		vlog.VI(1).Infof("C-MOVE-RQ payload: %s", elementsString(elems))
		responseCh := params.CMove(context.transferSyntaxUID, c.AffectedSOPClassUID, elems)
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
			sendResponse(&dimse.C_MOVE_RSP{
				AffectedSOPClassUID:            c.AffectedSOPClassUID,
				MessageIDBeingRespondedTo:      c.MessageID,
				CommandDataSetType:             dimse.CommandDataSetTypeNull,
				NumberOfRemainingSuboperations: uint16(resp.Remaining),
				NumberOfCompletedSuboperations: numSuccesses,
				NumberOfFailedSuboperations:    numFailures,
				Status: dimse.Status{Status: dimse.StatusPending},
			})
		}
		sendResponse(&dimse.C_MOVE_RSP{
			AffectedSOPClassUID:            c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo:      c.MessageID,
			CommandDataSetType:             dimse.CommandDataSetTypeNull,
			NumberOfCompletedSuboperations: numSuccesses,
			NumberOfFailedSuboperations:    numFailures,
			Status: status})
		// Drain the responses in case of errors
		for _ = range responseCh {
		}
	case *dimse.C_GET_RQ:
		sendError := func(err error) {
			sendResponse(&dimse.C_GET_RSP{
				AffectedSOPClassUID:       c.AffectedSOPClassUID,
				MessageIDBeingRespondedTo: c.MessageID,
				CommandDataSetType:        dimse.CommandDataSetTypeNull,
				Status:                    dimse.Status{Status: dimse.StatusUnrecognizedOperation, ErrorComment: err.Error()},
			})
		}
		if params.CGet == nil {
			sendResponse(&dimse.C_GET_RSP{
				AffectedSOPClassUID:       c.AffectedSOPClassUID,
				MessageIDBeingRespondedTo: c.MessageID,
				CommandDataSetType:        dimse.CommandDataSetTypeNull,
				Status:                    dimse.Status{Status: dimse.StatusUnrecognizedOperation, ErrorComment: "No callback found for C-GET"},
			})
			break
		}
		elems, err := readElementsInBytes(data, context.transferSyntaxUID)
		if err != nil {
			sendError(err)
			break
		}
		vlog.VI(1).Infof("C-GET-RQ payload: %s", elementsString(elems))
		responseCh := params.CGet(context.transferSyntaxUID, c.AffectedSOPClassUID, elems)
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
			vlog.Infof("C-GET: Sending %v", resp.Path)
			err := runCStoreOnAssociation(upcallCh, downcallCh, cm, dimse.NewMessageID(), resp.DataSet)
			if err != nil {
				vlog.Errorf("C-GET: C-store of %v failed: %v", resp.Path, err)
				numFailures++
			} else {
				vlog.Infof("C-GET: Sent %v", resp.Path)
				numSuccesses++
			}
			sendResponse(&dimse.C_GET_RSP{
				AffectedSOPClassUID:            c.AffectedSOPClassUID,
				MessageIDBeingRespondedTo:      c.MessageID,
				CommandDataSetType:             dimse.CommandDataSetTypeNull,
				NumberOfRemainingSuboperations: uint16(resp.Remaining),
				NumberOfCompletedSuboperations: numSuccesses,
				NumberOfFailedSuboperations:    numFailures,
				Status: dimse.Status{Status: dimse.StatusPending},
			})
		}
		sendResponse(&dimse.C_GET_RSP{
			AffectedSOPClassUID:            c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo:      c.MessageID,
			CommandDataSetType:             dimse.CommandDataSetTypeNull,
			NumberOfCompletedSuboperations: numSuccesses,
			NumberOfFailedSuboperations:    numFailures,
			Status: status})
		// Drain the responses in case of errors
		for _ = range responseCh {
		}
	case *dimse.C_ECHO_RQ:
		status := dimse.Status{Status: dimse.StatusUnrecognizedOperation}
		if params.CEcho != nil {
			status = params.CEcho()
		}
		resp := &dimse.C_ECHO_RSP{
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        dimse.CommandDataSetTypeNull,
			Status:                    status,
		}
		sendResponse(resp)
	default:
		vlog.Fatalf("UNknown DIMSE message type: %v", c)
	}
}

func NewServiceProvider(params ServiceProviderParams) *ServiceProvider {
	sp := &ServiceProvider{params: params}
	return sp
}

// Start threads for handling "conn". This function returns immediately; "conn"
// will be cleaned up in the background.
func RunProviderForConn(conn net.Conn, params ServiceProviderParams) {
	downcallCh := make(chan stateEvent, 128)
	upcallCh := make(chan upcallEvent, 128)
	go runStateMachineForServiceProvider(conn, upcallCh, downcallCh)

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
		onDIMSECommand(upcallCh, downcallCh, event.cm, event.contextID,
			event.command, event.data, params)
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
