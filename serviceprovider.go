package netdicom

import (
	"log"
	"net"
)

type ServiceProviderParams struct {
	// TCP address to listen to. E.g., ":1234" will listen to port 1234 at
	// all the IP address that this machine can bind to.
	ListenAddr string

	// The max PDU size, in bytes, that this instance is willing to receive.
	// If the value is <=0, DefaultMaxPDUSize is used.
	MaxPDUSize uint32

	// Called on receiving a C_STORE_RQ message. "data" is the payload,
	// usually a raw DICOM file. The handler should store the data and
	// return either 0 on success, or one of CStoreStatus* error codes.
	OnCStoreRequest func(data []byte) uint16
}

const DefaultMaxPDUSize uint32 = 4 << 20

type ServiceProvider struct {
	params   ServiceProviderParams
}

func onDataRequest(downcallCh chan StateEvent, pdu *P_DATA_TF, contextIDMap *contextIDMap,
	assembler *dimseCommandAssembler, params ServiceProviderParams) {
	abstractSyntaxUID, msg, data, err := addPDataTF(assembler, pdu, contextIDMap)
	if err != nil {
		log.Panic(err) // TODO(saito)
	}
	if msg == nil {
		return
	}
	switch c := msg.(type) {
	case *C_STORE_RQ:
		status := CStoreStatusCannotUnderstand
		if params.OnCStoreRequest != nil {
			status = params.OnCStoreRequest(data)
		}
		resp := &C_STORE_RSP{
			AffectedSOPClassUID:       c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        CommandDataSetTypeNull,
			AffectedSOPInstanceUID:    c.AffectedSOPInstanceUID,
			Status:                    status,
		}
		bytes, err := EncodeDIMSEMessage(resp)
		if err != nil {
			panic(err) // TODO(saito)
		}
		downcallCh <- StateEvent{
			event:              Evt9,
			pdu:                nil,
			conn:               nil,
			abstractSyntaxName: abstractSyntaxUID,
			command:            true,
			data:               bytes}
	default:
		panic("aoeu")
	}
}

func NewServiceProvider(params ServiceProviderParams) *ServiceProvider {
	doassert(params.ListenAddr != "")
	if params.MaxPDUSize <= 0 {
		params.MaxPDUSize = DefaultMaxPDUSize
	}
	sp := &ServiceProvider{params: params}
	return sp
}

// Run a thread that listens to events from the DUL statemachine (DICOM spec P3.8).
func runUpperLayerForServiceProvider(params ServiceProviderParams, upcallCh chan UpcallEvent, downcallCh chan StateEvent) {
	assembler := &dimseCommandAssembler{}
	handshakeCompleted := false
	for event := range upcallCh {
		if event.eventType == upcallEventHandshakeCompleted {
			doassert(!handshakeCompleted)
			handshakeCompleted = true
			log.Printf("handshake completed")
			continue
		}
		doassert(event.eventType == upcallEventData)
		doassert(event.pdu != nil)
		doassert(handshakeCompleted == true)
		if pdata, ok := event.pdu.(*P_DATA_TF); ok {
			onDataRequest(downcallCh, pdata, event.contextIDMap, assembler, params)
			continue
		}
		log.Panicf("Unknown upcall event: %v", event.pdu) // TODO
	}
	log.Printf("Finished upper layer service!")
}

// Start threads for handling "conn". This function returns immediately; "conn"
// will be cleaned up in the background.
func runProviderForConn(conn net.Conn, spParams ServiceProviderParams) {
	downcallCh := make(chan StateEvent, 128)
	upcallCh := make(chan UpcallEvent, 128)
	smParams := StateMachineParams{
		verbose:    true,
		maxPDUSize: spParams.MaxPDUSize,
		// // onAssociateRequest: onAssociateRequest,
		// onDataRequest: func(sm *StateMachine, pdu P_DATA_TF, contextIDMap contextIDMap) {
		// 	onDataRequest(sm, pdu, contextIDMap, dataState, sp.params)
		// },
	}
	go runStateMachineForServiceProvider(conn, smParams, upcallCh, downcallCh)
	go runUpperLayerForServiceProvider(spParams, upcallCh, downcallCh)
}

// Listen to incoming connections, accept them, and run the DICOM protocol. This
// function never returns unless it fails to listen.
func (sp *ServiceProvider) Run() error {
	listener, err := net.Listen("tcp", sp.params.ListenAddr)
	if err != nil {
		return err
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		runProviderForConn(conn, sp.params)
	}
}
