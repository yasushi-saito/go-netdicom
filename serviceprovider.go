package netdicom

// This file defines ServiceProvider (i.e., a DICOM server).

import (
	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-dicom/dicomio"
	"github.com/yasushi-saito/go-netdicom/dimse"
	"net"
	"v.io/x/lib/vlog"
)

type ServiceProviderParams struct {
	// Max size of a message chunk (PDU) that the provider can receiuve.  If
	// <= 0, DefaultMaxPDUSize is used.
	MaxPDUSize int
}

const DefaultMaxPDUSize int = 4 << 20

type CStoreCallback func(
	transferSyntaxUID string,
	sopClassUID string,
	sopInstanceUID string,
	data []byte) dimse.Status

type CFindCallback func(
	transferSyntaxUID string,
	sopClassUID string,
	filters []*dicom.Element) chan CFindResult

type CEchoCallback func() dimse.Status

type ServiceProviderCallbacks struct {
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

	// TODO(saito) Implement C-MOVE, C-GET, etc.
}

// Encapsulates the state for DICOM server (provider).
type ServiceProvider struct {
	params    ServiceProviderParams
	callbacks ServiceProviderCallbacks
}

func writeElementsToBytes(elems []*dicom.Element, transferSyntaxUID string) ([]byte, error) {
	dataEncoder := dicomio.NewBytesEncoderWithTransferSyntax(transferSyntaxUID)
	for _, elem := range elems {
		dicom.WriteDataElement(dataEncoder, elem)
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

func onDIMSECommand(downcallCh chan stateEvent,
	cm *contextManager,
	contextID byte,
	msg dimse.Message,
	data []byte,
	callbacks ServiceProviderCallbacks) {
	context, err := cm.lookupByContextID(contextID)
	if err != nil {
		vlog.Infof("Invalid context ID %d: %v", contextID, err)
		downcallCh <- stateEvent{event: evt19, pdu: nil, err: err}
		return
	}
	var sendResponse = func(resp dimse.Message) {
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

	vlog.VI(1).Infof("DIMSE request: %s", msg.String())
	switch c := msg.(type) {
	case *dimse.C_STORE_RQ:
		status := dimse.Status{Status: dimse.StatusUnrecognizedOperation}
		if callbacks.CStore != nil {
			status = callbacks.CStore(
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
		if callbacks.CFind == nil {
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
		responseCh := callbacks.CFind(context.transferSyntaxUID, c.AffectedSOPClassUID, elems)
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
	case *dimse.C_ECHO_RQ:
		status := dimse.Status{Status: dimse.StatusUnrecognizedOperation}
		if callbacks.CEcho != nil {
			status = callbacks.CEcho()
		}
		resp := &dimse.C_ECHO_RSP{
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        dimse.CommandDataSetTypeNull,
			Status:                    status,
		}
		sendResponse(resp)
	default:
		panic("aoeu")
	}
}

func NewServiceProvider(
	params ServiceProviderParams,
	callbacks ServiceProviderCallbacks) *ServiceProvider {
	if params.MaxPDUSize <= 0 {
		params.MaxPDUSize = DefaultMaxPDUSize
	}
	sp := &ServiceProvider{params: params, callbacks: callbacks}
	return sp
}

// Start threads for handling "conn". This function returns immediately; "conn"
// will be cleaned up in the background.
func RunProviderForConn(conn net.Conn,
	params ServiceProviderParams,
	callbacks ServiceProviderCallbacks) {
	downcallCh := make(chan stateEvent, 128)
	upcallCh := make(chan upcallEvent, 128)
	go runStateMachineForServiceProvider(conn, params, upcallCh, downcallCh)

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
		onDIMSECommand(downcallCh, event.cm, event.contextID,
			event.command, event.data, callbacks)
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
		go func() { RunProviderForConn(conn, sp.params, sp.callbacks) }()
	}
}
