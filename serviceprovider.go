package netdicom

import (
	"github.com/yasushi-saito/go-dicom/dicomio"
	"github.com/yasushi-saito/go-netdicom/dimse"
	"net"
	"v.io/x/lib/vlog"
)

type ServiceProviderParams struct {
	// Max size of a message chunk (PDU) that the client can receiuve.  If
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
	data []byte) dimse.Status

type CEchoCallback func() dimse.Status

type ServiceProviderCallbacks struct {
	// Called on C_ECHO request. It should return 0 on success
	CEcho CEchoCallback

	// Called on C_FIND request. It should return 0 on success
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
	CStore CStoreCallback
}

// Encapsulates the state for DICOM server (provider).
type ServiceProvider struct {
	params    ServiceProviderParams
	callbacks ServiceProviderCallbacks
}

func onDIMSECommand(downcallCh chan stateEvent,
	cm *contextManager,
	contextID byte,
	msg dimse.Message, data []byte, callbacks ServiceProviderCallbacks) {
	context, err := cm.lookupByContextID(contextID)
	if err != nil {
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
	status := dimse.Status{Status: dimse.StatusUnrecognizedOperation}
	switch c := msg.(type) {
	case *dimse.C_STORE_RQ:
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
		if callbacks.CFind != nil {
			status = callbacks.CFind(
				context.transferSyntaxUID,
				c.AffectedSOPClassUID,
				data)
		}
		resp := &dimse.C_FIND_RSP{
			AffectedSOPClassUID:       c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        dimse.CommandDataSetTypeNull,
			Status:                    status,
		}
		sendResponse(resp)
	case *dimse.C_ECHO_RQ:
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

// Run a thread that listens to events from the DUL statemachine (DICOM spec P3.8).
func runUpperLayerForServiceProvider(callbacks ServiceProviderCallbacks,
	upcallCh chan upcallEvent,
	downcallCh chan stateEvent) {
	handshakeCompleted := false
	for event := range upcallCh {
		if event.eventType == upcallEventHandshakeCompleted {
			doassert(!handshakeCompleted)
			handshakeCompleted = true
			vlog.VI(1).Infof("handshake completed")
			continue
		}
		doassert(event.eventType == upcallEventData)
		doassert(event.command != nil)
		doassert(handshakeCompleted == true)
		onDIMSECommand(downcallCh, event.cm, event.contextID,
			event.command, event.data, callbacks)
	}
	vlog.VI(1).Infof("Finished upper layer service!")
}

// Start threads for handling "conn". This function returns immediately; "conn"
// will be cleaned up in the background.
func RunProviderForConn(conn net.Conn,
	params ServiceProviderParams,
	callbacks ServiceProviderCallbacks) {
	downcallCh := make(chan stateEvent, 128)
	upcallCh := make(chan upcallEvent, 128)
	go runStateMachineForServiceProvider(conn, params, upcallCh, downcallCh)
	runUpperLayerForServiceProvider(callbacks, upcallCh, downcallCh)
	vlog.VI(1).Info("Finished the provider")
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
