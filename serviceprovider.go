package netdicom

import (
	"github.com/yasushi-saito/go-dicom"
	"log"
	"net"
)

type ServiceProviderParams struct {
	// The max PDU size, in bytes, that this instance is willing to receive.
	// If the value is <=0, DefaultMaxPDUSize is used.
	MaxPDUSize int
}

const DefaultMaxPDUSize int = 4 << 20

type CStoreCallback func(
	transferSyntaxUID string,
	sopClassUID string,
	sopInstanceUID string,
	data []byte) uint16

type CEchoCallback func() uint16

type ServiceProviderCallbacks struct {
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

	// Called on C_ECHO request.
	CEcho CEchoCallback
}

type ServiceProvider struct {
	params    ServiceProviderParams
	callbacks ServiceProviderCallbacks
}

func onDIMSECommand(downcallCh chan stateEvent, abstractSyntaxUID string,
	transferSyntaxUID string,
	msg DIMSEMessage, data []byte, callbacks ServiceProviderCallbacks) {
	doassert(transferSyntaxUID != "")
	switch c := msg.(type) {
	case *C_STORE_RQ:
		status := CStoreStatusCannotUnderstand
		if callbacks.CStore != nil {
			status = callbacks.CStore(
				transferSyntaxUID,
				c.AffectedSOPClassUID,
				c.AffectedSOPInstanceUID,
				data)
		}
		resp := &C_STORE_RSP{
			AffectedSOPClassUID:       c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        CommandDataSetTypeNull,
			AffectedSOPInstanceUID:    c.AffectedSOPInstanceUID,
			Status:                    status,
		}
		e := dicom.NewEncoder(nil, dicom.UnknownVR)
		EncodeDIMSEMessage(e, resp)
		bytes, err := e.Finish()
		if err != nil {
			panic(err) // TODO(saito)
		}
		downcallCh <- stateEvent{
			event: evt09,
			pdu:   nil,
			conn:  nil,
			dataPayload: &stateEventDataPayload{
				abstractSyntaxName: abstractSyntaxUID,
				command:            true,
				data:               bytes},
		}
	case *C_ECHO_RQ:
		status := CStoreStatusCannotUnderstand
		if callbacks.CEcho != nil {
			status = callbacks.CEcho()
		}
		resp := &C_ECHO_RSP{
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        CommandDataSetTypeNull,
			Status:                    status,
		}
		e := dicom.NewEncoder(nil, dicom.UnknownVR)
		EncodeDIMSEMessage(e, resp)
		bytes, err := e.Finish()
		if err != nil {
			panic(err) // TODO(saito)
		}
		downcallCh <- stateEvent{
			event: evt09,
			pdu:   nil,
			conn:  nil,
			dataPayload: &stateEventDataPayload{
				abstractSyntaxName: abstractSyntaxUID,
				command:            true,
				data:               bytes},
		}
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
			log.Printf("handshake completed")
			continue
		}
		doassert(event.eventType == upcallEventData)
		doassert(event.command != nil)
		doassert(handshakeCompleted == true)
		onDIMSECommand(downcallCh, event.abstractSyntaxUID,
			event.transferSyntaxUID,
			event.command, event.data, callbacks)
	}
	log.Printf("Finished upper layer service!")
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
	log.Print("Finished the provider")
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
			log.Printf("Accept error: %v", err)
			continue
		}
		go func() { RunProviderForConn(conn, sp.params, sp.callbacks) }()
	}
}
