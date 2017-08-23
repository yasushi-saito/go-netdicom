package netdicom

import (
	"fmt"
	"github.com/yasushi-saito/go-dicom"
	"log"
	"net"
	"time"
)

type StateType struct {
	Name        string
	Description string
}

var Sta1 = &StateType{"Sta1", "Idle"}
var Sta2 = &StateType{"Sta2", "Transport connection open (Awaiting A-ASSOCIATE-RQ PDU)"}
var Sta3 = &StateType{"Sta3", "Awaiting local A-ASSOCIATE response primitive (from local user)"}
var Sta4 = &StateType{"Sta4", "Awaiting transport connection opening to complete (from local transport service)"}
var Sta5 = &StateType{"Sta5", "Awaiting A-ASSOCIATE-AC or A-ASSOCIATE-RJ PDU"}
var Sta6 = &StateType{"Sta6", "Association established and ready for data transfer"}
var Sta7 = &StateType{"Sta7", "Awaiting A-RELEASE-RP PDU"}
var Sta8 = &StateType{"Sta8", "Awaiting local A-RELEASE response primitive (from local user)"}
var Sta9 = &StateType{"Sta9", "Release collision requestor side; awaiting A-RELEASE response (from local user)"}
var Sta10 = &StateType{"Sta10", "Release collision acceptor side; awaiting A-RELEASE-RP PDU"}
var Sta11 = &StateType{"Sta11", "Release collision requestor side; awaiting A-RELEASE-RP PDU"}
var Sta12 = &StateType{"Sta12", "Release collision acceptor side; awaiting A-RELEASE response primitive (from local user)"}
var Sta13 = &StateType{"Sta13", "Awaiting Transport Connection Close Indication (Association no longer exists)"}

type StateAction struct {
	Name        string
	Description string
	Callback    func(sm *StateMachine, event StateEvent) *StateType
}

var Ae1 = &StateAction{"AE-1",
	"Issue TRANSPORT CONNECT request primitive to local transport service",
	func(sm *StateMachine, event StateEvent) *StateType {
		go func(ch chan StateEvent, serverHostPort string) {
			conn, err := net.Dial("tcp", serverHostPort)
			if err != nil {
				log.Printf("Failed to connect to %s: %v", serverHostPort, err)
				ch <- StateEvent{event: Evt17, pdu: nil, err: err}
				close(ch)
				return
			}
			ch <- StateEvent{event: Evt2, pdu: nil, err: nil, conn: conn}
			networkReaderThread(ch, conn)
		}(sm.netCh, sm.serviceUserParams.Provider)
		return Sta4
	}}

// Generate an item list to be embedded in an A_REQUEST_RQ PDU. The PDU is sent
// when running as a service user.
func buildAssociateRequestItems(params ServiceUserParams) (*contextIDMap, []SubItem) {
	contextIDMap := newContextIDMap()
	items := []SubItem{
		&ApplicationContextItem{
			Name: DefaultApplicationContextItemName,
		}}
	var contextID byte = 1
	for _, sop := range params.RequiredServices {
		addContextIDToAbstractSyntaxNameMap(contextIDMap, sop.UID, contextID)
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
				&ImplementationClassUIDSubItem{DefaultImplementationClassUID},
				&ImplementationVersionNameSubItem{DefaultImplementationVersionName}}})
	return contextIDMap, items
}

var Ae2 = &StateAction{"AE-2", "Send A-ASSOCIATE-RQ-PDU",
	func(sm *StateMachine, event StateEvent) *StateType {
		newContextIDMap, items := buildAssociateRequestItems(sm.serviceUserParams)
		sendPDU(sm, &A_ASSOCIATE{
			Type:            PDUTypeA_ASSOCIATE_RQ,
			ProtocolVersion: CurrentProtocolVersion,
			CalledAETitle:   sm.serviceUserParams.CalledAETitle,
			CallingAETitle:  sm.serviceUserParams.CallingAETitle,
			Items:           items,
		})
		sm.contextIDMap = newContextIDMap
		startTimer(sm)
		return Sta5
	}}

var Ae3 = &StateAction{"AE-3", "Issue A-ASSOCIATE confirmation (accept) primitive",
	func(sm *StateMachine, event StateEvent) *StateType {
		return Sta6
	}}

var Ae4 = &StateAction{"AE-4", "Issue A-ASSOCIATE confirmation (reject) primitive and close transport connection",
	func(sm *StateMachine, event StateEvent) *StateType {
		closeConnection(sm)
		return Sta1
	}}

var Ae5 = &StateAction{"AE-5", "Issue Transport connection response primitive; start ARTIM timer",
	func(sm *StateMachine, event StateEvent) *StateType {
		doassert(event.conn != nil)
		startTimer(sm)
		go func(ch chan StateEvent, conn net.Conn) {
			networkReaderThread(ch, conn)
		}(sm.netCh, event.conn)
		return Sta2
	}}

var Ae6 = &StateAction{"AE-6", `Stop ARTIM timer and if A-ASSOCIATE-RQ acceptable by "
service-dul: issue A-ASSOCIATE indication primitive
otherwise issue A-ASSOCIATE-RJ-PDU and start ARTIM timer`,
	func(sm *StateMachine, event StateEvent) *StateType {
		stopTimer(sm)
		newContextIDMap := newContextIDMap()
		pdu := event.pdu.(*A_ASSOCIATE)
		if pdu.ProtocolVersion != 0x0001 {
			log.Printf("Wrong remote protocol version 0x%x", pdu.ProtocolVersion)
			rj := A_ASSOCIATE_RJ{Result: 1, Source: 2, Reason: 2}
			sendPDU(sm, &rj)
			startTimer(sm)
			return Sta13
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
				var syntaxItem = TransferSyntaxSubItem{
					Name: dicom.ImplicitVRLittleEndian,
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
						addContextIDToAbstractSyntaxNameMap(newContextIDMap, aitem.Name, n.ContextID)
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
			sm.upperLayerCh <- StateEvent{
				event: Evt7,
				pdu: &A_ASSOCIATE{
					Type:            PDUTypeA_ASSOCIATE_AC,
					ProtocolVersion: CurrentProtocolVersion,
					CalledAETitle:   pdu.CalledAETitle,
					CallingAETitle:  pdu.CallingAETitle,
					Items:           responses,
				},
			}
			sm.contextIDMap = newContextIDMap
		} else {
			sm.upperLayerCh <- StateEvent{
				event: Evt8,
				pdu: &A_ASSOCIATE_RJ{
					Result: ResultRejectedPermanent,
					Source: SourceULServiceProviderACSE,
					Reason: 1,
				},
			}
		}
		return Sta3
	}}
var Ae7 = &StateAction{"AE-7", "Send A-ASSOCIATE-AC PDU",
	func(sm *StateMachine, event StateEvent) *StateType {
		sendPDU(sm, event.pdu.(*A_ASSOCIATE))
		return Sta6
	}}

var Ae8 = &StateAction{"AE-8", "Send A-ASSOCIATE-RJ PDU and start ARTIM timer",
	func(sm *StateMachine, event StateEvent) *StateType {
		sendPDU(sm, event.pdu.(*A_ASSOCIATE_RJ))
		startTimer(sm)
		return Sta13
	}}

// Produce a list of P_DATA_TF PDUs that collective store "data".
func splitDataIntoPDUs(sm *StateMachine, abstractSyntaxName string, command bool, data []byte) []P_DATA_TF {
	doassert(sm.maxPDUSize > 0)
	doassert(len(data) > 0)

	contextID, err := abstractSyntaxNameToContextID(sm.contextIDMap, abstractSyntaxName)
	doassert(err == nil)
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
				ContextID: contextID,
				Command:   command,
				Last:      len(data) <= maxChunkSize,
				Value:     chunk,
			}}})
	}
	log.Printf("Created %d data pdus", len(pdus))
	return pdus
}

// Data transfer related actions
var Dt1 = &StateAction{"DT-1", "Send P-DATA-TF PDU",
	func(sm *StateMachine, event StateEvent) *StateType {
		doassert(event.data != nil)
		pdus := splitDataIntoPDUs(sm, event.abstractSyntaxName, event.command, event.data)
		log.Printf("Sending %d data pdus", len(pdus))
		for _, pdu := range pdus {
			sendPDU(sm, &pdu)
		}
		log.Printf("Finished sending %d data pdus", len(pdus))
		return Sta6
	}}

// func abstractSyntaxNameToContextID(sm *StateMachine, name string) byte {
// 	id, ok := sm.abstractSyntaxNameToContextIDMap[name]
// 	if !ok {
// 		log.Printf("Unknown syntax %s", name)
// 		return 255
// 	}
// 	return id
// }

// func contextIDToAbstractSyntaxName(sm *StateMachine, contextID byte) string {
// 	name, ok := sm.contextIDToAbstractSyntaxNameMap[contextID]
// 	if !ok {
// 		log.Printf("Unknown context %d sent from user", contextID)
// 		name = fmt.Sprintf("unknown-syntax-%d", contextID)
// 	}
// 	return name
// }

var Dt2 = &StateAction{"DT-2", "Send P-DATA indication primitive",
	func(sm *StateMachine, event StateEvent) *StateType {
		sm.params.onDataRequest(*event.pdu.(*P_DATA_TF), *sm.contextIDMap)
		// var mergedData [][]byte
		// var currentContext byte = 0

		// maybeUpcall := func() {
		// 	if mergedData != nil {
		// 		if sm.serviceProviderParams.OnDataCallback==nil{
		// 			log.Print("OnDataCallback is not set")
		// 		} else {
		// 			syntaxName, err := contextIDToAbstractSyntaxName(sm.contextIDMap, currentContext)
		// 			doassert(err == nil)
		// 			sm.serviceProviderParams.OnDataCallback(syntaxName, mergedData)
		// 		}
		// 		mergedData = nil
		// 	}
		// }

		// for _, item := range pdu.Items {
		// 	if currentContext != item.ContextID {
		// 		maybeUpcall()
		// 		currentContext = item.ContextID
		// 	}
		// 	mergedData = append(mergedData, item.Value)
		// }
		// maybeUpcall()
		return Sta6
	}}

// Assocation Release related actions
var Ar1 = &StateAction{"AR-1", "Send A-RELEASE-RQ PDU",
	func(sm *StateMachine, event StateEvent) *StateType {
		sendPDU(sm, &A_RELEASE_RQ{})
		return Sta7
	}}
var Ar2 = &StateAction{"AR-2", "Issue A-RELEASE indication primitive",
	func(sm *StateMachine, event StateEvent) *StateType {
		return Sta8
	}}

var Ar3 = &StateAction{"AR-3", "Issue A-RELEASE confirmation primitive and close transport connection",
	func(sm *StateMachine, event StateEvent) *StateType {
		sendPDU(sm, &A_RELEASE_RP{})
		closeConnection(sm)
		return Sta1
	}}
var Ar4 = &StateAction{"AR-4", "Issue A-RELEASE-RP PDU and start ARTIM timer",
	func(sm *StateMachine, event StateEvent) *StateType {
		sendPDU(sm, &A_RELEASE_RP{})
		startTimer(sm)
		return Sta13
	}}

var Ar5 = &StateAction{"AR-5", "Stop ARTIM timer",
	func(sm *StateMachine, event StateEvent) *StateType {
		stopTimer(sm)
		return Sta1
	}}

var Ar6 = &StateAction{"AR-6", "Issue P-DATA indication",
	func(sm *StateMachine, event StateEvent) *StateType {
		return Sta7
	}}

var Ar7 = &StateAction{"AR-7", "Issue P-DATA-TF PDU",
	func(sm *StateMachine, event StateEvent) *StateType {
		pdus := splitDataIntoPDUs(sm, event.abstractSyntaxName, event.command, event.data)
		for _, pdu := range pdus {
			sendPDU(sm, &pdu)
		}
		// sendPDU(sm, &P_DATA_TF{Items: event.data})
		return Sta8
	}}

var Ar8 = &StateAction{"AR-8", "Issue A-RELEASE indication (release collision): if association-requestor, next state is Sta9, if not next state is Sta10",
	func(sm *StateMachine, event StateEvent) *StateType {
		panic("AR8")
		if sm.requestor == 1 {
			return Sta9
		} else {
			return Sta10
		}
	}}

var Ar9 = &StateAction{"AR-9", "Send A-RELEASE-RP PDU",
	func(sm *StateMachine, event StateEvent) *StateType {
		sendPDU(sm, &A_RELEASE_RP{})
		return Sta11
	}}

var Ar10 = &StateAction{"AR-10", "Issue A-RELEASE confimation primitive",
	func(sm *StateMachine, event StateEvent) *StateType {
		return Sta12
	}}

// Association abort related actions
var Aa1 = &StateAction{"AA-1", "Send A-ABORT PDU (service-user source) and start (or restart if already started) ARTIM timer",
	func(sm *StateMachine, event StateEvent) *StateType {
		diagnostic := byte(0)
		if sm.currentState == Sta2 {
			diagnostic = 2
		}
		sendPDU(sm, &A_ABORT{Source: 0, Reason: diagnostic})
		restartTimer(sm)
		return Sta13
	}}

var Aa2 = &StateAction{"AA-2", "Stop ARTIM timer if running. Close transport connection",
	func(sm *StateMachine, event StateEvent) *StateType {
		stopTimer(sm)
		closeConnection(sm)
		return Sta1
	}}

var Aa3 = &StateAction{"AA-3", "If (service-user initiated abort): issue A-ABORT indication and close transport connection, otherwise (service-dul initiated abort): issue A-P-ABORT indication and close transport connection",
	func(sm *StateMachine, event StateEvent) *StateType {
		closeConnection(sm)
		return Sta1
	}}

var Aa4 = &StateAction{"AA-4", "Issue A-P-ABORT indication primitive",
	func(sm *StateMachine, event StateEvent) *StateType {
		return Sta1
	}}

var Aa5 = &StateAction{"AA-5", "Stop ARTIM timer",
	func(sm *StateMachine, event StateEvent) *StateType {
		stopTimer(sm)
		return Sta1
	}}

var Aa6 = &StateAction{"AA-6", "Ignore PDU",
	func(sm *StateMachine, event StateEvent) *StateType {
		return Sta13
	}}

var Aa7 = &StateAction{"AA-7", "Send A-ABORT PDU",
	func(sm *StateMachine, event StateEvent) *StateType {
		sendPDU(sm, &A_ABORT{Source: 0, Reason: 0})
		return Sta13
	}}

var Aa8 = &StateAction{"AA-8", "Send A-ABORT PDU (service-dul source), issue an A-P-ABORT indication and start ARTIM timer",
	func(sm *StateMachine, event StateEvent) *StateType {
		sendPDU(sm, &A_ABORT{Source: 2, Reason: 0})
		startTimer(sm)
		return Sta13
	}}

//type StateTransitionEvent struct {
//	Name        string
//	Description string
//}

type EventType int

const (
	Evt1  = EventType(1)  // A-ASSOCIATE request (local user)
	Evt2  = EventType(2)  // Connection established (for service user)
	Evt3  = EventType(3)  // A-ASSOCIATE-AC PDU (received on transport connection)
	Evt4  = EventType(4)  // A-ASSOCIATE-RJ PDU (received on transport connection)
	Evt5  = EventType(5)  // Connection accepted (for service provider)
	Evt6  = EventType(6)  // A-ASSOCIATE-RQ PDU (on tranport connection)
	Evt7  = EventType(7)  // A-ASSOCIATE response primitive (accept)
	Evt8  = EventType(8)  // A-ASSOCIATE response primitive (reject)
	Evt9  = EventType(9)  // P-DATA request primitive
	Evt10 = EventType(10) // P-DATA-TF PDU (on transport connection)
	Evt11 = EventType(11) // A-RELEASE request primitive
	Evt12 = EventType(12) // A-RELEASE-RQ PDU (on transport)
	Evt13 = EventType(13) // A-RELEASE-RP PDU (on transport)
	Evt14 = EventType(14) // A-RELEASE response primitive
	Evt15 = EventType(15) // A-ABORT request primitive
	Evt16 = EventType(16) // A-ABORT PDU (on transport)
	Evt17 = EventType(17) // Transport connection closed indication (local transport service)
	Evt18 = EventType(18) // ARTIM timer expired (Association reject/release timer)
	Evt19 = EventType(19) // Unrecognized or invalid PDU received
)

type StateEvent struct {
	event EventType
	pdu   PDU
	err   error
	conn  net.Conn

	// The following four fields are set iff event==Evt9.

	// The syntax UID of the data to be sent.
	abstractSyntaxName string

	// Is the data command or data? E.g., true for C_STORE, false for C_FIND.
	command bool

	// Data to send. len(data) may exceed the max PDU size, in which case it
	// will be split into multiple PresentationDataValueItems.
	data []byte
}

//func PDUReceivedEvent(event EventType, pdu PDU) StateEvent{
//	return StateEvent{event: event, pdu: pdu, err: nil, conn: nil, data: nil}
//}

type StateTransition struct {
	event   EventType
	current *StateType
	action  *StateAction
}

var stateTransitions = []StateTransition{
	StateTransition{Evt1, Sta1, Ae1},
	StateTransition{Evt2, Sta4, Ae2},
	StateTransition{Evt3, Sta2, Aa1},
	StateTransition{Evt3, Sta3, Aa8},
	StateTransition{Evt3, Sta5, Ae3},
	StateTransition{Evt3, Sta6, Aa8},
	StateTransition{Evt3, Sta7, Aa8},
	StateTransition{Evt3, Sta8, Aa8},
	StateTransition{Evt3, Sta9, Aa8},
	StateTransition{Evt3, Sta10, Aa8},
	StateTransition{Evt3, Sta11, Aa8},
	StateTransition{Evt3, Sta12, Aa8},
	StateTransition{Evt3, Sta13, Aa6},
	StateTransition{Evt4, Sta2, Aa1},
	StateTransition{Evt4, Sta3, Aa8},
	StateTransition{Evt4, Sta5, Ae4},
	StateTransition{Evt4, Sta6, Aa8},
	StateTransition{Evt4, Sta7, Aa8},
	StateTransition{Evt4, Sta8, Aa8},
	StateTransition{Evt4, Sta9, Aa8},
	StateTransition{Evt4, Sta10, Aa8},
	StateTransition{Evt4, Sta11, Aa8},
	StateTransition{Evt4, Sta12, Aa8},
	StateTransition{Evt4, Sta13, Aa6},
	StateTransition{Evt5, Sta1, Ae5},
	StateTransition{Evt6, Sta2, Ae6},
	StateTransition{Evt6, Sta3, Aa8},
	StateTransition{Evt6, Sta5, Aa8},
	StateTransition{Evt6, Sta6, Aa8},
	StateTransition{Evt6, Sta7, Aa8},
	StateTransition{Evt6, Sta8, Aa8},
	StateTransition{Evt6, Sta9, Aa8},
	StateTransition{Evt6, Sta10, Aa8},
	StateTransition{Evt6, Sta11, Aa8},
	StateTransition{Evt6, Sta12, Aa8},
	StateTransition{Evt6, Sta13, Aa7},
	StateTransition{Evt7, Sta3, Ae7},
	StateTransition{Evt8, Sta3, Ae8},
	StateTransition{Evt9, Sta6, Dt1},
	StateTransition{Evt9, Sta8, Ar7},
	StateTransition{Evt10, Sta2, Aa1},
	StateTransition{Evt10, Sta3, Aa8},
	StateTransition{Evt10, Sta5, Aa8},
	StateTransition{Evt10, Sta6, Dt2},
	StateTransition{Evt10, Sta7, Ar6},
	StateTransition{Evt10, Sta8, Aa8},
	StateTransition{Evt10, Sta9, Aa8},
	StateTransition{Evt10, Sta10, Aa8},
	StateTransition{Evt10, Sta11, Aa8},
	StateTransition{Evt10, Sta12, Aa8},
	StateTransition{Evt10, Sta13, Aa6},
	StateTransition{Evt11, Sta6, Ar1},
	StateTransition{Evt12, Sta2, Aa1},
	StateTransition{Evt12, Sta3, Aa8},
	StateTransition{Evt12, Sta5, Aa8},
	StateTransition{Evt12, Sta6, Ar2},
	StateTransition{Evt12, Sta7, Ar8},
	StateTransition{Evt12, Sta8, Aa8},
	StateTransition{Evt12, Sta9, Aa8},
	StateTransition{Evt12, Sta10, Aa8},
	StateTransition{Evt12, Sta11, Aa8},
	StateTransition{Evt12, Sta12, Aa8},
	StateTransition{Evt12, Sta13, Aa6},
	StateTransition{Evt13, Sta2, Aa1},
	StateTransition{Evt13, Sta3, Aa8},
	StateTransition{Evt13, Sta5, Aa8},
	StateTransition{Evt13, Sta6, Aa8},
	StateTransition{Evt13, Sta7, Ar3},
	StateTransition{Evt13, Sta8, Aa8},
	StateTransition{Evt13, Sta9, Aa8},
	StateTransition{Evt13, Sta10, Ar10},
	StateTransition{Evt13, Sta11, Ar3},
	StateTransition{Evt13, Sta12, Aa8},
	StateTransition{Evt13, Sta13, Aa6},
	StateTransition{Evt14, Sta8, Ar4},
	StateTransition{Evt14, Sta9, Ar9},
	StateTransition{Evt14, Sta12, Ar4},
	StateTransition{Evt15, Sta3, Aa1},
	StateTransition{Evt15, Sta4, Aa2},
	StateTransition{Evt15, Sta5, Aa1},
	StateTransition{Evt15, Sta6, Aa1},
	StateTransition{Evt15, Sta7, Aa1},
	StateTransition{Evt15, Sta8, Aa1},
	StateTransition{Evt15, Sta9, Aa1},
	StateTransition{Evt15, Sta10, Aa1},
	StateTransition{Evt15, Sta11, Aa1},
	StateTransition{Evt15, Sta12, Aa1},
	StateTransition{Evt16, Sta2, Aa2},
	StateTransition{Evt16, Sta3, Aa3},
	StateTransition{Evt16, Sta5, Aa3},
	StateTransition{Evt16, Sta6, Aa3},
	StateTransition{Evt16, Sta7, Aa3},
	StateTransition{Evt16, Sta8, Aa3},
	StateTransition{Evt16, Sta9, Aa3},
	StateTransition{Evt16, Sta10, Aa3},
	StateTransition{Evt16, Sta11, Aa3},
	StateTransition{Evt16, Sta12, Aa3},
	StateTransition{Evt16, Sta13, Aa2},
	StateTransition{Evt17, Sta2, Aa5},
	StateTransition{Evt17, Sta3, Aa4},
	StateTransition{Evt17, Sta4, Aa4},
	StateTransition{Evt17, Sta5, Aa4},
	StateTransition{Evt17, Sta6, Aa4},
	StateTransition{Evt17, Sta7, Aa4},
	StateTransition{Evt17, Sta8, Aa4},
	StateTransition{Evt17, Sta9, Aa4},
	StateTransition{Evt17, Sta10, Aa4},
	StateTransition{Evt17, Sta11, Aa4},
	StateTransition{Evt17, Sta12, Aa4},
	StateTransition{Evt17, Sta13, Ar5},
	StateTransition{Evt18, Sta2, Aa2},
	StateTransition{Evt18, Sta13, Aa2},
	StateTransition{Evt19, Sta2, Aa1},
	StateTransition{Evt19, Sta3, Aa8},
	StateTransition{Evt19, Sta5, Aa8},
	StateTransition{Evt19, Sta6, Aa8},
	StateTransition{Evt19, Sta7, Aa8},
	StateTransition{Evt19, Sta8, Aa8},
	StateTransition{Evt19, Sta9, Aa8},
	StateTransition{Evt19, Sta10, Aa8},
	StateTransition{Evt19, Sta11, Aa8},
	StateTransition{Evt19, Sta12, Aa8},
	StateTransition{Evt19, Sta13, Aa7},
}

const (
	Idle = iota
	Connecting
	Connected
	ReadingPDU
)

type StateMachine struct {
	serviceUserParams     ServiceUserParams
	params StateMachineParams
	requestor             int32

	// abstractSyntaxMap maps a contextID (an odd integer) to an abstract
	// syntax string such as 1.2.840.10008.5.1.4.1.1.1.2.  This field is set
	// on receiving A_ASSOCIATE_RQ message. Thus, it is set only on the
	// provider side (not the user).
	contextIDMap *contextIDMap
	//contextIDToAbstractSyntaxNameMap map[byte]string
	//abstractSyntaxNameToContextIDMap map[string]byte

	// For receiving PDU and network status events.
	netCh chan StateEvent
	// For receiving commands from the upper layer (C_STORE, etc)
	upperLayerCh chan StateEvent
	// For Timer expiration event
	timerCh      chan StateEvent
	conn         net.Conn
	currentState *StateType

	// The negotiated PDU size.
	maxPDUSize int
}

func doassert(x bool) {
	if !x {
		panic("doassert")
	}
}

func closeConnection(sm *StateMachine) {
	sm.conn.Close()
}

func sendPDU(sm *StateMachine, pdu PDU) {
	doassert(sm.conn != nil)
	data, err := EncodePDU(pdu)
	if err != nil {
		log.Printf("Failed to encode: %v", err)
		sm.conn.Close()
		sm.netCh <- StateEvent{event: Evt17, err: err}
		return
	}
	n, err := sm.conn.Write(data)
	if n != len(data) || err != nil {
		log.Printf("Failed to write %d bytes. Actual %d bytes : %v", len(data), n, err)
		sm.conn.Close()
		sm.netCh <- StateEvent{event: Evt17, err: err}
		return
	}
	log.Printf("sendPDU: %v", pdu.DebugString())
}

func startTimer(sm *StateMachine) {
	ch := make(chan StateEvent)
	sm.timerCh = ch
	time.AfterFunc(time.Duration(10)*time.Second,
		func() {
			ch <- StateEvent{event: Evt18}
			close(ch)
		})
}

func restartTimer(sm *StateMachine) {
	startTimer(sm)
}

func stopTimer(sm *StateMachine) {
	sm.timerCh = make(chan StateEvent)
}

func networkReaderThread(ch chan StateEvent, conn net.Conn) {
	log.Printf("Starting network reader for %v", conn)
	for {
		pdu, err := DecodePDU(conn)
		if err != nil {
			log.Printf("Failed to read PDU: %v", err)
			ch <- StateEvent{event: Evt19, pdu: nil, err: err}
			break
		}
		if pdu == nil {
			log.Printf("Empty PDU")
			break
		}
		log.Printf("Read PDU: %v", pdu.DebugString())
		if n, ok := pdu.(*A_ASSOCIATE); ok {
			if n.Type == PDUTypeA_ASSOCIATE_RQ {
				ch <- StateEvent{event: Evt6, pdu: n, err: nil}
			} else {
				doassert(n.Type == PDUTypeA_ASSOCIATE_AC)
				ch <- StateEvent{event: Evt3, pdu: n, err: nil}
			}
			continue
		}
		if n, ok := pdu.(*A_ASSOCIATE_RJ); ok {
			ch <- StateEvent{event: Evt4, pdu: n, err: nil}
			continue
		}
		if n, ok := pdu.(*P_DATA_TF); ok {
			ch <- StateEvent{event: Evt10, pdu: n, err: nil}
			continue
		}
		if n, ok := pdu.(*A_RELEASE_RQ); ok {
			ch <- StateEvent{event: Evt12, pdu: n, err: nil}
			continue
		}
		if n, ok := pdu.(*A_RELEASE_RP); ok {
			ch <- StateEvent{event: Evt13, pdu: n, err: nil}
			continue
		}
		if n, ok := pdu.(*A_ABORT); ok {
			ch <- StateEvent{event: Evt16, pdu: n, err: nil}
			continue
		}
		panic(fmt.Sprintf("Unknown PDU type: %v", pdu.DebugString()))
	}
	log.Print("The peer closed the connection")
	ch <- StateEvent{event: Evt17, pdu: nil, err: nil}
	close(ch)
}

func getNextEvent(sm *StateMachine) StateEvent {
	var event StateEvent
	select {
	case event = <-sm.netCh:
	case event = <-sm.timerCh:
	case event = <-sm.upperLayerCh:
	}
	switch event.event {
	case Evt2:
		doassert(event.conn != nil)
		sm.conn = event.conn
	case Evt17:
		sm.conn = nil
	}
	return event
}

func findAction(currentState *StateType, event EventType) *StateAction {
	for _, t := range stateTransitions {
		if t.current == currentState && t.event == event {
			return t.action
		}
	}
	log.Panicf("No action found for state %v, event %v", *currentState, event)
	return nil
}

type StateMachineParams struct {
	verbose        bool
	// listenAddr     string // Set only when running as provider
	maxPDUSize uint32

	// onAssociateRequest func(A_ASSOCIATE) ([]SubItem, bool)
	onDataRequest      func(P_DATA_TF, contextIDMap)
}

const DefaultMaximiumPDUSize = uint32(1 << 20)

func NewStateMachineForServiceUser(params ServiceUserParams) *StateMachine {
	doassert(params.Provider != "")
	doassert(params.CallingAETitle != "")
	doassert(len(params.RequiredServices) > 0)
	doassert(len(params.SupportedTransferSyntaxes) > 0)
	sm := &StateMachine{}
	sm.contextIDMap = newContextIDMap()
	sm.serviceUserParams = params
	sm.netCh = make(chan StateEvent, 128)
	sm.upperLayerCh = make(chan StateEvent, 128)
	sm.maxPDUSize = 1 << 20 // TODO(saito)
	event := StateEvent{event: Evt1}
	action := findAction(Sta1, event.event)
	sm.currentState = action.Callback(sm, event)
	RunStateMachineUntilQuiescent(sm)
	return sm
}

func RunStateMachineForServiceProvider(conn net.Conn, params StateMachineParams) {
	sm := &StateMachine{params: params}
	sm.contextIDMap = newContextIDMap()
	sm.conn = conn
	sm.netCh = make(chan StateEvent, 128)
	sm.maxPDUSize = 1 << 20 // TODO(saito)
	sm.upperLayerCh = make(chan StateEvent, 128)
	event := StateEvent{event: Evt5, conn: conn}
	action := findAction(Sta1, event.event)
	sm.currentState = action.Callback(sm, event)
	for sm.currentState != Sta1 {
		event := getNextEvent(sm)
		action := findAction(sm.currentState, event.event)
		log.Printf("Running action %v", action.Name)
		sm.currentState = action.Callback(sm, event)
		log.Printf("Got event:%v action:%v, next:%s",
			event, action, sm.currentState)
	}
	log.Print("Connection shutdown")
}

func SendData(sm *StateMachine,
	abstractSyntaxUID string,
	data []byte) {
	log.Printf("Send data")
	sm.upperLayerCh <- StateEvent{
		event:              Evt9,
		pdu:                nil,
		conn:               nil,
		abstractSyntaxName: abstractSyntaxUID,
		command:            false,
		data:               data}
	for sm.currentState != Sta1 {
		event := getNextEvent(sm)
		action := findAction(sm.currentState, event.event)
		log.Printf("Running action %v", action.Name)
		sm.currentState = action.Callback(sm, event)
		log.Printf("Got event:%v action:%v, next:%s",
			event, action, sm.currentState)
	}
	log.Print("Connection shutdown")
}

func StartRelease(sm *StateMachine) {
	log.Printf("Release")
	sm.upperLayerCh <- StateEvent{event: Evt11}
	RunStateMachineUntilQuiescent(sm)
}

func RunStateMachineUntilQuiescent(sm *StateMachine) {
	log.Printf("Start SM: current:%s", sm.currentState)
	for sm.currentState != Sta6 && sm.currentState != Sta1 {
		event := getNextEvent(sm)
		action := findAction(sm.currentState, event.event)
		log.Printf("Running action %v", action.Name)
		sm.currentState = action.Callback(sm, event)
		log.Printf("Got event:%v action:%v, next:%s",
			event, action, sm.currentState)
	}
	log.Printf("Finish SM: current:%s", sm.currentState)
}
