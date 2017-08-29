package netdicom

import (
	"github.com/yasushi-saito/go-dicom"
	"io"
	"log"
	"net"
	"time"
)

type stateType struct {
	Name        string
	Description string
}

var sta1 = &stateType{"Sta1", "Idle"}
var sta2 = &stateType{"Sta2", "Transport connection open (Awaiting A-ASSOCIATE-RQ PDU)"}
var sta3 = &stateType{"Sta3", "Awaiting local A-ASSOCIATE response primitive (from local user)"}
var sta4 = &stateType{"Sta4", "Awaiting transport connection opening to complete (from local transport service)"}
var sta5 = &stateType{"Sta5", "Awaiting A-ASSOCIATE-AC or A-ASSOCIATE-RJ PDU"}
var sta6 = &stateType{"Sta6", "Association established and ready for data transfer"}
var sta7 = &stateType{"Sta7", "Awaiting A-RELEASE-RP PDU"}
var sta8 = &stateType{"Sta8", "Awaiting local A-RELEASE response primitive (from local user)"}
var sta9 = &stateType{"Sta9", "Release collision requestor side; awaiting A-RELEASE response (from local user)"}
var sta10 = &stateType{"Sta10", "Release collision acceptor side; awaiting A-RELEASE-RP PDU"}
var sta11 = &stateType{"Sta11", "Release collision requestor side; awaiting A-RELEASE-RP PDU"}
var sta12 = &stateType{"Sta12", "Release collision acceptor side; awaiting A-RELEASE response primitive (from local user)"}
var sta13 = &stateType{"Sta13", "Awaiting Transport Connection Close Indication (Association no longer exists)"}

type stateAction struct {
	Name        string
	Description string
	Callback    func(sm *stateMachine, event stateEvent) *stateType
}

var actionAe1 = &stateAction{"AE-1",
	"Issue TRANSPORT CONNECT request primitive to local transport service",
	func(sm *stateMachine, event stateEvent) *stateType {
		go func(ch chan stateEvent, serverHostPort string) {
			conn, err := net.Dial("tcp", serverHostPort)
			if err != nil {
				log.Printf("Failed to connect to %s: %v", serverHostPort, err)
				ch <- stateEvent{event: evt17, pdu: nil, err: err}
				close(ch)
				return
			}
			ch <- stateEvent{event: evt2, pdu: nil, err: nil, conn: conn}
			networkReaderThread(ch, conn)
		}(sm.netCh, sm.serviceUserParams.Provider)
		return sta4
	}}

// Generate an item list to be embedded in an A_REQUEST_RQ PDU. The PDU is sent
// when running as a service user.
func buildAssociateRequestItems(params ServiceUserParams) (*contextManager, []SubItem) {
	contextManager := newContextIDMap()
	items := []SubItem{
		&ApplicationContextItem{
			Name: DefaultApplicationContextItemName,
		}}
	var contextID byte = 1
	for _, sop := range params.RequiredServices {
		// TODO(saito) Fix translation uid.
		contextManager.addMapping(sop.UID, "", contextID)
		syntaxItems := []SubItem{
			&AbstractSyntaxSubItem{Name: sop.UID},
		}
		for _, syntaxUID := range params.SupportedTransferSyntaxes {
			syntaxItems = append(syntaxItems,
				&TransferSyntaxSubItem{Name: syntaxUID},
			)
		}
		items = append(items,
			&PresentationContextItem{
				Type:      ItemTypePresentationContextRequest,
				ContextID: contextID,
				Result:    0, // must be zero for request
				Items:     syntaxItems,
			})
		contextID += 2 // must be odd.
	}
	items = append(items,
		&UserInformationItem{
			Items: []SubItem{
				&UserInformationMaximumLengthItem{params.MaxPDUSize},
				&ImplementationClassUIDSubItem{dicom.DefaultImplementationClassUID},
				&ImplementationVersionNameSubItem{dicom.DefaultImplementationVersionName}}})
	return contextManager, items
}

var actionAe2 = &stateAction{"AE-2", "Send A-ASSOCIATE-RQ-PDU",
	func(sm *stateMachine, event stateEvent) *stateType {
		newContextIDMap, items := buildAssociateRequestItems(sm.serviceUserParams)
		pdu := &A_ASSOCIATE{
			Type:            PDUTypeA_ASSOCIATE_RQ,
			ProtocolVersion: CurrentProtocolVersion,
			CalledAETitle:   sm.serviceUserParams.CalledAETitle,
			CallingAETitle:  sm.serviceUserParams.CallingAETitle,
			Items:           items,
		}
		sendPDU(sm, pdu)
		sm.contextManager = newContextIDMap
		startTimer(sm)
		return sta5
	}}

var actionAe3 = &stateAction{"AE-3", "Issue A-ASSOCIATE confirmation (accept) primitive",
	func(sm *stateMachine, event stateEvent) *stateType {
		// TODO(saito) Set the context ID map here!
		sm.upcallCh <- upcallEvent{eventType: upcallEventHandshakeCompleted}
		return sta6
	}}

var actionAe4 = &stateAction{"AE-4", "Issue A-ASSOCIATE confirmation (reject) primitive and close transport connection",
	func(sm *stateMachine, event stateEvent) *stateType {
		closeConnection(sm)
		return sta1
	}}

var actionAe5 = &stateAction{"AE-5", "Issue Transport connection response primitive; start ARTIM timer",
	func(sm *stateMachine, event stateEvent) *stateType {
		doassert(event.conn != nil)
		startTimer(sm)
		go func(ch chan stateEvent, conn net.Conn) {
			networkReaderThread(ch, conn)
		}(sm.netCh, event.conn)
		return sta2
	}}

var actionAe6 = &stateAction{"AE-6", `Stop ARTIM timer and if A-ASSOCIATE-RQ acceptable by "
service-dul: issue A-ASSOCIATE indication primitive
otherwise issue A-ASSOCIATE-RJ-PDU and start ARTIM timer`,
	func(sm *stateMachine, event stateEvent) *stateType {
		stopTimer(sm)
		newContextIDMap := newContextIDMap()
		pdu := event.pdu.(*A_ASSOCIATE)
		if pdu.ProtocolVersion != 0x0001 {
			log.Printf("Wrong remote protocol version 0x%x", pdu.ProtocolVersion)
			rj := A_ASSOCIATE_RJ{Result: 1, Source: 2, Reason: 2}
			sendPDU(sm, &rj)
			startTimer(sm)
			return sta13
		}
		responses := []SubItem{
			&ApplicationContextItem{
				Name: DefaultApplicationContextItemName,
			},
		}
		for _, item := range pdu.Items {
			if n, ok := item.(*PresentationContextItem); ok {
				// TODO(saito) Need to pick the syntax preferred by us.
				// For now, just hardcode the syntax, ignoring the list
				// in RQ.
				//
				// var syntaxItem SubItem
				// for _, subitem := range(n.Items) {
				// 	log.Printf("Received PresentaionContext(%x): %v", n.ContextID, subitem.DebugString())
				// 	if n, ok := subitem.(*SubItemWithName); ok && n.Type == ItemTypeTransferSyntax {
				// 		syntaxItem = n
				// 		break
				// 	}
				// }
				// doassert(syntaxItem != nil)
				transferSyntaxUID := dicom.ImplicitVRLittleEndian
				var syntaxItem = TransferSyntaxSubItem{
					Name: transferSyntaxUID,
				}
				responses = append(responses,
					&PresentationContextItem{
						Type:      ItemTypePresentationContextResponse,
						ContextID: n.ContextID,
						Result:    0, // accepted
						Items:     []SubItem{&syntaxItem}})
				for _, aitem := range n.Items {
					if aitem, ok := aitem.(*AbstractSyntaxSubItem); ok {
						log.Printf("Map context %d -> %s", n.ContextID, aitem.Name)
						newContextIDMap.addMapping(
							aitem.Name,
							transferSyntaxUID,
							n.ContextID)
					}
					// TODO(saito) CHeck that each item has exactly one aitem.
				}
			}
		}
		// TODO(saito) Set the PDU size more properly.
		responses = append(responses,
			&UserInformationItem{
				Items: []SubItem{&UserInformationMaximumLengthItem{MaximumLengthReceived: 1 << 20}}})
		ok := true
		// items, ok := sm.serviceProviderParams.onAssociateRequest(*pdu)
		if ok {
			doassert(len(responses) > 0)
			doassert(pdu.CalledAETitle != "")
			doassert(pdu.CallingAETitle != "")
			sm.downcallCh <- stateEvent{
				event: evt7,
				pdu: &A_ASSOCIATE{
					Type:            PDUTypeA_ASSOCIATE_AC,
					ProtocolVersion: CurrentProtocolVersion,
					CalledAETitle:   pdu.CalledAETitle,
					CallingAETitle:  pdu.CallingAETitle,
					Items:           responses,
				},
			}
			sm.contextManager = newContextIDMap
		} else {
			sm.downcallCh <- stateEvent{
				event: evt8,
				pdu: &A_ASSOCIATE_RJ{
					Result: ResultRejectedPermanent,
					Source: SourceULServiceProviderACSE,
					Reason: 1,
				},
			}
		}
		return sta3
	}}
var actionAe7 = &stateAction{"AE-7", "Send A-ASSOCIATE-AC PDU",
	func(sm *stateMachine, event stateEvent) *stateType {
		sendPDU(sm, event.pdu.(*A_ASSOCIATE))
		sm.upcallCh <- upcallEvent{eventType: upcallEventHandshakeCompleted}
		return sta6
	}}

var actionAe8 = &stateAction{"AE-8", "Send A-ASSOCIATE-RJ PDU and start ARTIM timer",
	func(sm *stateMachine, event stateEvent) *stateType {
		sendPDU(sm, event.pdu.(*A_ASSOCIATE_RJ))
		startTimer(sm)
		return sta13
	}}

// Produce a list of P_DATA_TF PDUs that collective store "data".
func splitDataIntoPDUs(sm *stateMachine, abstractSyntaxName string, command bool, data []byte) []P_DATA_TF {
	doassert(sm.maxPDUSize > 0)
	doassert(len(data) > 0)

	context, err := sm.contextManager.lookupByAbstractSyntaxUID(abstractSyntaxName)
	if err != nil {
		log.Panicf("Illegal syntax name %s: %s", dicom.UIDDebugString(abstractSyntaxName), err)
	}
	var pdus []P_DATA_TF
	// two byte header overhead.
	//
	// TODO(saito) move the magic number elsewhere.
	var maxChunkSize = sm.maxPDUSize - 2
	for len(data) > 0 {
		chunkSize := len(data)
		if chunkSize > maxChunkSize {
			chunkSize = sm.maxPDUSize
		}
		chunk := data[0:chunkSize]
		data = data[chunkSize:]
		pdus = append(pdus, P_DATA_TF{Items: []PresentationDataValueItem{
			PresentationDataValueItem{
				ContextID: context.contextID,
				Command:   command,
				Last:      false, // Set later.
				Value:     chunk,
			}}})
	}
	if len(pdus) > 0 {
		pdus[len(pdus)-1].Items[0].Last = true
	}
	log.Printf("Created %d data pdus", len(pdus))
	return pdus
}

// Data transfer related actions
var actionDt1 = &stateAction{"DT-1", "Send P-DATA-TF PDU",
	func(sm *stateMachine, event stateEvent) *stateType {
		doassert(event.dataPayload != nil)
		pdus := splitDataIntoPDUs(sm, event.dataPayload.abstractSyntaxName, event.dataPayload.command, event.dataPayload.data)
		log.Printf("Sending %d data pdus", len(pdus))
		for _, pdu := range pdus {
			sendPDU(sm, &pdu)
		}
		log.Printf("Finished sending %d data pdus", len(pdus))
		return sta6
	}}

var actionDt2 = &stateAction{"DT-2", "Send P-DATA indication primitive",
	func(sm *stateMachine, event stateEvent) *stateType {
		abstractSyntaxUID, transferSyntaxUID, command, data, err := addPDataTF(&sm.commandAssembler, event.pdu.(*P_DATA_TF), sm.contextManager)
		if err != nil {
			log.Panic(err) // TODO(saito)
		}
		if command != nil {
			sm.upcallCh <- upcallEvent{
				eventType:         upcallEventData,
				abstractSyntaxUID: abstractSyntaxUID,
				transferSyntaxUID: transferSyntaxUID,
				command:           command,
				data:              data}
		} else {
			// Not all fragments received yet
		}
		return sta6
	}}

// Assocation Release related actions
var actionAr1 = &stateAction{"AR-1", "Send A-RELEASE-RQ PDU",
	func(sm *stateMachine, event stateEvent) *stateType {
		sendPDU(sm, &A_RELEASE_RQ{})
		return sta7
	}}
var actionAr2 = &stateAction{"AR-2", "Issue A-RELEASE indication primitive",
	func(sm *stateMachine, event stateEvent) *stateType {
		// TODO(saito) Do RELEASE callback here.
		sm.downcallCh <- stateEvent{event: evt14}
		return sta8
	}}

var actionAr3 = &stateAction{"AR-3", "Issue A-RELEASE confirmation primitive and close transport connection",
	func(sm *stateMachine, event stateEvent) *stateType {
		sendPDU(sm, &A_RELEASE_RP{})
		closeConnection(sm)
		return sta1
	}}
var actionAr4 = &stateAction{"AR-4", "Issue A-RELEASE-RP PDU and start ARTIM timer",
	func(sm *stateMachine, event stateEvent) *stateType {
		sendPDU(sm, &A_RELEASE_RP{})
		startTimer(sm)
		return sta13
	}}

var actionAr5 = &stateAction{"AR-5", "Stop ARTIM timer",
	func(sm *stateMachine, event stateEvent) *stateType {
		stopTimer(sm)
		return sta1
	}}

var actionAr6 = &stateAction{"AR-6", "Issue P-DATA indication",
	func(sm *stateMachine, event stateEvent) *stateType {
		return sta7
	}}

var actionAr7 = &stateAction{"AR-7", "Issue P-DATA-TF PDU",
	func(sm *stateMachine, event stateEvent) *stateType {
		doassert(event.dataPayload != nil)
		pdus := splitDataIntoPDUs(sm, event.dataPayload.abstractSyntaxName, event.dataPayload.command, event.dataPayload.data)
		for _, pdu := range pdus {
			sendPDU(sm, &pdu)
		}
		sm.downcallCh <- stateEvent{event: evt14}
		return sta8
	}}

var actionAr8 = &stateAction{"AR-8", "Issue A-RELEASE indication (release collision): if association-requestor, next state is Sta9, if not next state is Sta10",
	func(sm *stateMachine, event stateEvent) *stateType {
		if sm.isUser {
			return sta9
		} else {
			return sta10
		}
	}}

var actionAr9 = &stateAction{"AR-9", "Send A-RELEASE-RP PDU",
	func(sm *stateMachine, event stateEvent) *stateType {
		sendPDU(sm, &A_RELEASE_RP{})
		return sta11
	}}

var actionAr10 = &stateAction{"AR-10", "Issue A-RELEASE confimation primitive",
	func(sm *stateMachine, event stateEvent) *stateType {
		return sta12
	}}

// Association abort related actions
var actionAa1 = &stateAction{"AA-1", "Send A-ABORT PDU (service-user source) and start (or restart if already started) ARTIM timer",
	func(sm *stateMachine, event stateEvent) *stateType {
		diagnostic := byte(0)
		if sm.currentState == sta2 {
			diagnostic = 2
		}
		sendPDU(sm, &A_ABORT{Source: 0, Reason: diagnostic})
		restartTimer(sm)
		return sta13
	}}

var actionAa2 = &stateAction{"AA-2", "Stop ARTIM timer if running. Close transport connection",
	func(sm *stateMachine, event stateEvent) *stateType {
		stopTimer(sm)
		closeConnection(sm)
		return sta1
	}}

var actionAa3 = &stateAction{"AA-3", "If (service-user initiated abort): issue A-ABORT indication and close transport connection, otherwise (service-dul initiated abort): issue A-P-ABORT indication and close transport connection",
	func(sm *stateMachine, event stateEvent) *stateType {
		closeConnection(sm)
		return sta1
	}}

var actionAa4 = &stateAction{"AA-4", "Issue A-P-ABORT indication primitive",
	func(sm *stateMachine, event stateEvent) *stateType {
		return sta1
	}}

var actionAa5 = &stateAction{"AA-5", "Stop ARTIM timer",
	func(sm *stateMachine, event stateEvent) *stateType {
		stopTimer(sm)
		return sta1
	}}

var actionAa6 = &stateAction{"AA-6", "Ignore PDU",
	func(sm *stateMachine, event stateEvent) *stateType {
		return sta13
	}}

var actionAa7 = &stateAction{"AA-7", "Send A-ABORT PDU",
	func(sm *stateMachine, event stateEvent) *stateType {
		sendPDU(sm, &A_ABORT{Source: 0, Reason: 0})
		return sta13
	}}

var actionAa8 = &stateAction{"AA-8", "Send A-ABORT PDU (service-dul source), issue an A-P-ABORT indication and start ARTIM timer",
	func(sm *stateMachine, event stateEvent) *stateType {
		sendPDU(sm, &A_ABORT{Source: 2, Reason: 0})
		startTimer(sm)
		return sta13
	}}

type eventType struct {
	Event       int
	Description string
}

var (
	evt1  = eventType{1, "A-ASSOCIATE request (local user)"}
	evt2  = eventType{2, "Connection established (for service user)"}
	evt3  = eventType{3, "A-ASSOCIATE-AC PDU (received on transport connection)"}
	evt4  = eventType{4, "A-ASSOCIATE-RJ PDU (received on transport connection)"}
	evt5  = eventType{5, "Connection accepted (for service provider)"}
	evt6  = eventType{6, "A-ASSOCIATE-RQ PDU (on tranport connection)"}
	evt7  = eventType{7, "A-ASSOCIATE response primitive (accept)"}
	evt8  = eventType{8, "A-ASSOCIATE response primitive (reject)"}
	evt9  = eventType{9, "P-DATA request primitive"}
	evt10 = eventType{10, "P-DATA-TF PDU (on transport connection)"}
	evt11 = eventType{11, "A-RELEASE request primitive"}
	evt12 = eventType{12, "A-RELEASE-RQ PDU (on transport)"}
	evt13 = eventType{13, "A-RELEASE-RP PDU (on transport)"}
	evt14 = eventType{14, "A-RELEASE response primitive"}
	evt15 = eventType{15, "A-ABORT request primitive"}
	evt16 = eventType{16, "A-ABORT PDU (on transport)"}
	evt17 = eventType{17, "Transport connection closed indication (local transport service)"}
	evt18 = eventType{18, "ARTIM timer expired (Association reject/release timer)"}
	evt19 = eventType{19, "Unrecognized or invalid PDU received"}
)

var (
	upcallEventHandshakeCompleted = eventType{100, "Handshake completed"}
	upcallEventData               = eventType{101, "P_DATA_TF PDU received"}
	// Note: connection shutdown and any error will result in channel
	// closure, so they don't have event types.
)

type upcallEvent struct {
	eventType eventType // upcallEvent*

	// abstractSyntaxUID is extracted from the P_DATA_TF packet.
	// transferSyntaxUID is the value agreed on for the abstractSyntaxUID
	// during protocol handshake. Both are nonempty iff
	// eventType==upcallEventData.
	abstractSyntaxUID string
	transferSyntaxUID string

	command DIMSEMessage
	data    []byte
}

type stateEventDataPayload struct {
	// The syntax UID of the data to be sent.
	abstractSyntaxName string

	// Is the data command or data? E.g., true for C_STORE, false for C_FIND.
	command bool

	// Data to send. len(data) may exceed the max PDU size, in which case it
	// will be split into multiple PresentationDataValueItems.
	data []byte
}

type stateEvent struct {
	event eventType
	pdu   PDU
	err   error
	conn  net.Conn

	dataPayload *stateEventDataPayload // set iff event==evt9.
}

//func PDUReceivedEvent(event eventType, pdu PDU) stateEvent{
//	return stateEvent{event: event, pdu: pdu, err: nil, conn: nil, data: nil}
//}

type stateTransition struct {
	event   eventType
	current *stateType
	action  *stateAction
}

var stateTransitions = []stateTransition{
	stateTransition{evt1, sta1, actionAe1},
	stateTransition{evt2, sta4, actionAe2},
	stateTransition{evt3, sta2, actionAa1},
	stateTransition{evt3, sta3, actionAa8},
	stateTransition{evt3, sta5, actionAe3},
	stateTransition{evt3, sta6, actionAa8},
	stateTransition{evt3, sta7, actionAa8},
	stateTransition{evt3, sta8, actionAa8},
	stateTransition{evt3, sta9, actionAa8},
	stateTransition{evt3, sta10, actionAa8},
	stateTransition{evt3, sta11, actionAa8},
	stateTransition{evt3, sta12, actionAa8},
	stateTransition{evt3, sta13, actionAa6},
	stateTransition{evt4, sta2, actionAa1},
	stateTransition{evt4, sta3, actionAa8},
	stateTransition{evt4, sta5, actionAe4},
	stateTransition{evt4, sta6, actionAa8},
	stateTransition{evt4, sta7, actionAa8},
	stateTransition{evt4, sta8, actionAa8},
	stateTransition{evt4, sta9, actionAa8},
	stateTransition{evt4, sta10, actionAa8},
	stateTransition{evt4, sta11, actionAa8},
	stateTransition{evt4, sta12, actionAa8},
	stateTransition{evt4, sta13, actionAa6},
	stateTransition{evt5, sta1, actionAe5},
	stateTransition{evt6, sta2, actionAe6},
	stateTransition{evt6, sta3, actionAa8},
	stateTransition{evt6, sta5, actionAa8},
	stateTransition{evt6, sta6, actionAa8},
	stateTransition{evt6, sta7, actionAa8},
	stateTransition{evt6, sta8, actionAa8},
	stateTransition{evt6, sta9, actionAa8},
	stateTransition{evt6, sta10, actionAa8},
	stateTransition{evt6, sta11, actionAa8},
	stateTransition{evt6, sta12, actionAa8},
	stateTransition{evt6, sta13, actionAa7},
	stateTransition{evt7, sta3, actionAe7},
	stateTransition{evt8, sta3, actionAe8},
	stateTransition{evt9, sta6, actionDt1},
	stateTransition{evt9, sta8, actionAr7},
	stateTransition{evt10, sta2, actionAa1},
	stateTransition{evt10, sta3, actionAa8},
	stateTransition{evt10, sta5, actionAa8},
	stateTransition{evt10, sta6, actionDt2},
	stateTransition{evt10, sta7, actionAr6},
	stateTransition{evt10, sta8, actionAa8},
	stateTransition{evt10, sta9, actionAa8},
	stateTransition{evt10, sta10, actionAa8},
	stateTransition{evt10, sta11, actionAa8},
	stateTransition{evt10, sta12, actionAa8},
	stateTransition{evt10, sta13, actionAa6},
	stateTransition{evt11, sta6, actionAr1},
	stateTransition{evt12, sta2, actionAa1},
	stateTransition{evt12, sta3, actionAa8},
	stateTransition{evt12, sta5, actionAa8},
	stateTransition{evt12, sta6, actionAr2},
	stateTransition{evt12, sta7, actionAr8},
	stateTransition{evt12, sta8, actionAa8},
	stateTransition{evt12, sta9, actionAa8},
	stateTransition{evt12, sta10, actionAa8},
	stateTransition{evt12, sta11, actionAa8},
	stateTransition{evt12, sta12, actionAa8},
	stateTransition{evt12, sta13, actionAa6},
	stateTransition{evt13, sta2, actionAa1},
	stateTransition{evt13, sta3, actionAa8},
	stateTransition{evt13, sta5, actionAa8},
	stateTransition{evt13, sta6, actionAa8},
	stateTransition{evt13, sta7, actionAr3},
	stateTransition{evt13, sta8, actionAa8},
	stateTransition{evt13, sta9, actionAa8},
	stateTransition{evt13, sta10, actionAr10},
	stateTransition{evt13, sta11, actionAr3},
	stateTransition{evt13, sta12, actionAa8},
	stateTransition{evt13, sta13, actionAa6},
	stateTransition{evt14, sta8, actionAr4},
	stateTransition{evt14, sta9, actionAr9},
	stateTransition{evt14, sta12, actionAr4},
	stateTransition{evt15, sta3, actionAa1},
	stateTransition{evt15, sta4, actionAa2},
	stateTransition{evt15, sta5, actionAa1},
	stateTransition{evt15, sta6, actionAa1},
	stateTransition{evt15, sta7, actionAa1},
	stateTransition{evt15, sta8, actionAa1},
	stateTransition{evt15, sta9, actionAa1},
	stateTransition{evt15, sta10, actionAa1},
	stateTransition{evt15, sta11, actionAa1},
	stateTransition{evt15, sta12, actionAa1},
	stateTransition{evt16, sta2, actionAa2},
	stateTransition{evt16, sta3, actionAa3},
	stateTransition{evt16, sta5, actionAa3},
	stateTransition{evt16, sta6, actionAa3},
	stateTransition{evt16, sta7, actionAa3},
	stateTransition{evt16, sta8, actionAa3},
	stateTransition{evt16, sta9, actionAa3},
	stateTransition{evt16, sta10, actionAa3},
	stateTransition{evt16, sta11, actionAa3},
	stateTransition{evt16, sta12, actionAa3},
	stateTransition{evt16, sta13, actionAa2},
	stateTransition{evt17, sta2, actionAa5},
	stateTransition{evt17, sta3, actionAa4},
	stateTransition{evt17, sta4, actionAa4},
	stateTransition{evt17, sta5, actionAa4},
	stateTransition{evt17, sta6, actionAa4},
	stateTransition{evt17, sta7, actionAa4},
	stateTransition{evt17, sta8, actionAa4},
	stateTransition{evt17, sta9, actionAa4},
	stateTransition{evt17, sta10, actionAa4},
	stateTransition{evt17, sta11, actionAa4},
	stateTransition{evt17, sta12, actionAa4},
	stateTransition{evt17, sta13, actionAr5},
	stateTransition{evt18, sta2, actionAa2},
	stateTransition{evt18, sta13, actionAa2},
	stateTransition{evt19, sta2, actionAa1},
	stateTransition{evt19, sta3, actionAa8},
	stateTransition{evt19, sta5, actionAa8},
	stateTransition{evt19, sta6, actionAa8},
	stateTransition{evt19, sta7, actionAa8},
	stateTransition{evt19, sta8, actionAa8},
	stateTransition{evt19, sta9, actionAa8},
	stateTransition{evt19, sta10, actionAa8},
	stateTransition{evt19, sta11, actionAa8},
	stateTransition{evt19, sta12, actionAa8},
	stateTransition{evt19, sta13, actionAa7},
}

const (
	Idle = iota
	Connecting
	Connected
	ReadingPDU
)

type stateMachine struct {
	isUser            bool // true if service user, false if provider
	serviceUserParams ServiceUserParams
	params            stateMachineParams

	// abstractSyntaxMap maps a contextID (an odd integer) to an abstract
	// syntax string such as 1.2.840.10008.5.1.4.1.1.1.2.  This field is set
	// on receiving A_ASSOCIATE_RQ message. Thus, it is set only on the
	// provider side (not the user).
	contextManager *contextManager
	//contextIDToAbstractSyntaxNameMap map[byte]string
	//abstractSyntaxNameToContextIDMap map[string]byte

	// For receiving PDU and network status events.
	netCh chan stateEvent

	// For receiving commands from the upper layer
	downcallCh chan stateEvent
	// For sending indications to the the upper layer
	upcallCh chan upcallEvent

	// For Timer expiration event
	timerCh      chan stateEvent
	conn         net.Conn
	currentState *stateType

	// The negotiated PDU size.
	maxPDUSize int

	commandAssembler dimseCommandAssembler
}

func doassert(x bool) {
	if !x {
		panic("doassert")
	}
}

func closeConnection(sm *stateMachine) {
	close(sm.upcallCh)
	log.Printf("Closing connection %v", sm.conn)
	sm.conn.Close()
}

func sendPDU(sm *stateMachine, pdu PDU) {
	doassert(sm.conn != nil)
	data, err := EncodePDU(pdu)
	if err != nil {
		log.Printf("Failed to encode: %v; closing connection %v", err, sm.conn)
		sm.conn.Close()
		sm.netCh <- stateEvent{event: evt17, err: err}
		return
	}
	n, err := sm.conn.Write(data)
	if n != len(data) || err != nil {
		log.Printf("Failed to write %d bytes. Actual %d bytes : %v; closing connection %v", len(data), n, err, sm.conn)
		sm.conn.Close()
		sm.netCh <- stateEvent{event: evt17, err: err}
		return
	}
	log.Printf("sendPDU: %v", pdu.DebugString())
}

func startTimer(sm *stateMachine) {
	ch := make(chan stateEvent)
	sm.timerCh = ch
	time.AfterFunc(time.Duration(10)*time.Second,
		func() {
			ch <- stateEvent{event: evt18}
			close(ch)
		})
}

func restartTimer(sm *stateMachine) {
	startTimer(sm)
}

func stopTimer(sm *stateMachine) {
	sm.timerCh = make(chan stateEvent)
}

func networkReaderThread(ch chan stateEvent, conn net.Conn) {
	log.Printf("Starting network reader for %v", conn)
	for {
		pdu, err := DecodePDU(conn)
		if err != nil {
			log.Printf("Failed to read PDU: %v", err)
			if err == io.EOF {
				ch <- stateEvent{event: evt17, pdu: nil, err: nil}
			} else {
				ch <- stateEvent{event: evt19, pdu: nil, err: err}
			}
			close(ch)
			break
		}
		doassert(pdu != nil)
		log.Printf("Read PDU: %v", pdu.DebugString())
		if n, ok := pdu.(*A_ASSOCIATE); ok {
			if n.Type == PDUTypeA_ASSOCIATE_RQ {
				ch <- stateEvent{event: evt6, pdu: n, err: nil}
			} else {
				doassert(n.Type == PDUTypeA_ASSOCIATE_AC)
				ch <- stateEvent{event: evt3, pdu: n, err: nil}
			}
			continue
		}
		// TODO(saito) use type switches
		if n, ok := pdu.(*A_ASSOCIATE_RJ); ok {
			ch <- stateEvent{event: evt4, pdu: n, err: nil}
			continue
		}
		if n, ok := pdu.(*P_DATA_TF); ok {
			ch <- stateEvent{event: evt10, pdu: n, err: nil}
			continue
		}
		if n, ok := pdu.(*A_RELEASE_RQ); ok {
			ch <- stateEvent{event: evt12, pdu: n, err: nil}
			continue
		}
		if n, ok := pdu.(*A_RELEASE_RP); ok {
			ch <- stateEvent{event: evt13, pdu: n, err: nil}
			continue
		}
		if n, ok := pdu.(*A_ABORT); ok {
			ch <- stateEvent{event: evt16, pdu: n, err: nil}
			continue
		}
		log.Panicf("Unknown PDU type: %v", pdu.DebugString())
	}
	log.Printf("Exiting network reader for %v", conn)
}

func getNextEvent(sm *stateMachine) stateEvent {
	var event stateEvent
	select {
	case event = <-sm.netCh:
	case event = <-sm.timerCh:
	case event = <-sm.downcallCh:
	}
	switch event.event {
	case evt2:
		doassert(event.conn != nil)
		sm.conn = event.conn
	case evt17:
		close(sm.upcallCh)
		sm.conn = nil
	}
	return event
}

func findAction(currentState *stateType, event eventType) *stateAction {
	for _, t := range stateTransitions {
		if t.current == currentState && t.event == event {
			return t.action
		}
	}
	log.Panicf("No action found for state %v, event %v", *currentState, event)
	return nil
}

type stateMachineParams struct {
	verbose bool
	// listenAddr     string // Set only when running as provider
	maxPDUSize uint32

	// onAssociateRequest func(A_ASSOCIATE) ([]SubItem, bool)
	// onDataRequest func(*stateMachine, P_DATA_TF, contextManager)
}

const DefaultMaximiumPDUSize = uint32(1 << 20)

func runOneStep(sm *stateMachine) {
	event := getNextEvent(sm)
	log.Printf("Current state: %v, Event %v", sm.currentState, event)
	action := findAction(sm.currentState, event.event)
	log.Printf("Running action %v", action)
	sm.currentState = action.Callback(sm, event)
	log.Printf("Next state: %v", sm.currentState)
}

func runStateMachineForServiceUser(
	params ServiceUserParams,
	upcallCh chan upcallEvent,
	downcallCh chan stateEvent) {
	doassert(params.Provider != "")
	doassert(params.CallingAETitle != "")
	doassert(len(params.RequiredServices) > 0)
	doassert(len(params.SupportedTransferSyntaxes) > 0)
	sm := &stateMachine{
		isUser:            true,
		contextManager:      newContextIDMap(),
		serviceUserParams: params,
		netCh:             make(chan stateEvent, 128),
		downcallCh:        downcallCh,
		upcallCh:          upcallCh,
		maxPDUSize:        1 << 20, // TODO(saito)
	}
	event := stateEvent{event: evt1}
	action := findAction(sta1, event.event)
	sm.currentState = action.Callback(sm, event)
	for sm.currentState != sta1 {
		runOneStep(sm)
	}
	log.Print("Connection shutdown")
}

func runStateMachineForServiceProvider(
	conn net.Conn,
	params stateMachineParams,
	upcallCh chan upcallEvent,
	downcallCh chan stateEvent) {
	sm := &stateMachine{
		isUser:       false,
		params:       params,
		contextManager: newContextIDMap(),
		conn:         conn,
		netCh:        make(chan stateEvent, 128),
		maxPDUSize:   1 << 20, // TODO(saito)
		downcallCh:   downcallCh,
		upcallCh:     upcallCh,
	}
	event := stateEvent{event: evt5, conn: conn}
	action := findAction(sta1, event.event)
	sm.currentState = action.Callback(sm, event)
	for sm.currentState != sta1 {
		runOneStep(sm)
	}
	log.Print("Connection shutdown")
}
