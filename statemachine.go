package netdicom

import (
	"net"
)

type StateType struct {
	Name        string
	Description string
}

var Sta1 = &StateType{"Sta1", "Idle"}
var Sta2 = &StateType{"Sta2", "Transport connection open (Awaiting A-ASSOCIATE-RQ PDU)"}
var Sta3 = &StateType{"Sta3", "Awaiting local A-ASSOCIATE response primitive (from local user)"}
var Sta4 = &StateType{"Sta4", "Awaiting transport connection opening to complete (from local transport service"}
var Sta5 = &StateType{"Sta5", "Awaiting A-ASSOCIATE-AC or A-ASSOCIATE-RJ PDU"}
var Sta6 = &StateType{"Sta6", "Association established and ready for data transfer"}
var Sta7 = &StateType{"Sta7", "Awaiting A-RELEASE-RP PDU"}
var Sta8 = &StateType{"Sta8", "Awaiting local A-RELEASE response primitive (from local user)"}
var Sta9 = &StateType{"Sta9", "Release collision requestor side; awaiting A-RELEASE response (from local user)"}
var Sta10 = &StateType{"Sta10", "Release collision acceptor side; awaiting A-RELEASE-RP PDU"}
var Sta11 = &StateType{"Sta11", "Release collision requestor side; awaiting A-RELEASE-RP PDU"}
var Sta12 = &StateType{"Sta12", "Release collision acceptor side; awaiting A-RELEASE response primitive (from local user)"}
var Sta13 = &StateType{"Sta13", "Awaiting Transport Connection Close Indication (Association no longer exists)"}Sta1

type StateAction struct {
	Name        string
	Description string
	Callback    func(sm *StateMachine) *StateType
}

var Ae1 = &StateAction{"AE-1",
	"Issue TRANSPORT CONNECT request primitive to local transport service",
	func(sm *StateMachine) (*StateType, *StateTransitionEvent) {
		assert(sm.conn == nil)
		conn, err := net.Dial("tcp", sm.Params.Peer)
		if err != nil {
			return Sta4, Evt17
		}
		sm.conn = conn
		return Sta4, Evt2
	}}

var Ae2 = &StateAction{"AE-2", "Send A-ASSOCIATE-RQ-PDU",
	func(sm *StateMachine) (*StateType, *StateTransitionEvent) {
		sendPdu(sm, New_A_ASSOCIATE_RQ(sm.Params))
		startTimer(sm)
		startReadingPdu(sm)
		return Sta5, nil
	}}

var Ae3 = &StateAction{"AE-3", "Issue A-ASSOCIATE confirmation (accept) primitive",
	func(sm *StateMachine) *StateType {
		return Sta6
	}}

var Ae4 = &StateAction{"AE-4", "Issue A-ASSOCIATE confirmation (reject) primitive and close transport connection",
	func(sm *StateMachine) *StateType {
		closeConnection(sm)
		return Sta1
	}}

var Ae5 = &StateAction{"AE-5", "Issue Transport connection response primitive; start ARTIM timer",
	func(sm *StateMachine) *StateType {
		startTimer(sm)
		return Sta2
	}}

var Ae6 = &StateAction{"AE-6", `Stop ARTIM timer and if A-ASSOCIATE-RQ acceptable by "
service-dul: issue A-ASSOCIATE indication primitive
otherwise issue A-ASSOCIATE-RJ-PDU and start ARTIM timer`,
	func(sm *StateMachine) *StateType {
		stopTimer(sm)
		return Sta3
	}}
var Ae7 = &StateAction{"AE-7", "Send A-ASSOCIATE-AC PDU",
	func(sm *StateMachine) *StateType {
		pdu := New_A_ASSOCIATE_RQ(sm.Params)
		sendPdu(sm, pdu)
		return Sta6
	}}

var Ae8 = &StateAction{"AE-8", "Send A-ASSOCIATE-RJ PDU and start ARTIM timer",
	func(sm *StateMachine) *StateType {
		pdu := New_A_ASSOCIATE_RJ(sm.Params)
		sendPdu(sm, pdu)
		startTimer(sm)
		return Sta13
	}}

// Data transfer related actions
var Dt1 = &StateAction{"DT-1", "Send P-DATA-TF PDU",
	func(sm *StateMachine) *StateType {
		pdu := New_P_DATA_TF(sm.PData)
		sendPdu(sm, pdu)
		return Sta6
	}}

var Dt2 = &StateAction{"DT-2", "Send P-DATA indication primitive",
	func(sm *StateMachine) *StateType {
		return Sta6
	}}

// Assocation Release related actions
var Ar1 = &StateAction{"AR-1", "Send A-RELEASE-RQ PDU",
	func(sm *StateMachine) *StateType {
		sendPdu(sm, New_A_RELEASE_RQ())
		return Sta7
	}}
var Ar2 = &StateAction{"AR-2", "Issue A-RELEASE indication primitive",
	func(sm *StateMachine) *StateType {
		return Sta8
	}}

var Ar3 = &StateAction{"AR-3", "Issue A-RELEASE confirmation primitive and close transport connection",
	func(sm *StateMachine) *StateType {
		sendPdu(sm, New_A_RELEASE_RP())
		closeConnection(sm)
		return Sta1
	}}
var Ar4 = &StateAction{"AR-4", "Issue A-RELEASE-RP PDU and start ARTIM timer",
	func(sm *StateMachine) *StateType {
		sendPdu(sm, New_A_RELEASE_RP())
		startTimer(sm)
		return Sta13
	}}

var Ar5 = &StateAction{"AR-5", "Stop ARTIM timer",
	func(sm *StateMachine) *StateType {
		stopTimer(sm)
		return Sta1
	}}

var Ar6 = &StateAction{"AR-6", "Issue P-DATA indication",
	func(sm *StateMachine) *StateType {
		return Sta7
	}}

var Ar7 = &StateAction{"AR-7", "Issue P-DATA-TF PDU",
	func(sm *StateMachine) *StateType {
		sendPdu(sm, New_P_DATA_TF(sm.PData))
		return Sta8
	}}

var Ar8 = &StateAction{"AR-8", "Issue A-RELEASE indication (release collision): if association-requestor, next state is Sta9, if not next state is Sta10",
	func(sm *StateMachine) *StateType {
		panic("aoeu")
		if sm.Requestor == 1 {
			return Sta9
		} else {
			return Sta10
		}
	}}

var Ar9 = &StateAction{"AR-9", "Send A-RELEASE-RP PDU",
	func(sm *StateMachine) *StateType {
		sendPdu(sm, New_A_RELEASE_RP())
		return Sta11
	}}

var Ar10 = &StateAction{"AR-10", "Issue A-RELEASE confimation primitive",
	func(sm *StateMachine) *StateType {
		return Sta12
	}}

// Association abort related actions
var Aa1 = &StateAction{"AA-1", "Send A-ABORT PDU (service-user source) and start (or restart if already started) ARTIM timer",
	func(sm *StateMachine) *StateType {
		diagnostic := byte(0)
		if sm.currentState == Sta2 {
			diagnostic = 2
		}
		sendPdu(sm, New_A_ABORT(0, diagnostic))
		restartTimer(sm)
		return Sta13
	}}

var Aa2 = &StateAction{"AA-2", "Stop ARTIM timer if running. Close transport connection",
	func(sm *StateMachine) *StateType {
		stopTimer(sm)
		closeConnection(sm)
		return Sta1
	}}

var Aa3 = &StateAction{"AA-3", "If (service-user initiated abort): issue A-ABORT indication and close transport connection, otherwise (service-dul initiated abort): issue A-P-ABORT indication and close transport connection",
	func(sm *StateMachine) *StateType {
		closeConnection(sm)
		return Sta1
	}}

var Aa4 = &StateAction{"AA-4", "Issue A-P-ABORT indication primitive",
	func(sm *StateMachine) *StateType {
		return Sta1
	}}

var Aa5 = &StateAction{"AA-5", "Stop ARTIM timer",
	func(sm *StateMachine) *StateType {
		stopTimer(sm)
		return Sta1
	}}

var Aa6 = &StateAction{"AA-6", "Ignore PDU",
	func(sm *StateMachine) *StateType {
		sm.PData = nil
		return Sta13
	}}

var Aa7 = &StateAction{"AA-7", "Send A-ABORT PDU",
	func(sm *StateMachine) *StateType {
		sendPdu(sm, New_A_ABORT(0, 0))
		return Sta13
	}}

var Aa8 = &StateAction{"AA-8", "Send A-ABORT PDU (service-dul source), issue an A-P-ABORT indication and start ARTIM timer",
	func(sm *StateMachine) *StateType {
		sendPdu(sm, New_A_ABORT(2, 0))
		startTimer(sm)
		return Sta13
	}}

type StateTransitionEvent struct {
	Name        string
	Description string
}

var Evt1 = &StateTransitionEvent{"Evt1", "A-ASSOCIATE request (local user)"}
var Evt2 = &StateTransitionEvent{"Evt2", "Transport connect confirmation (local transport service)"}
var Evt3 = &StateTransitionEvent{"Evt3", "A-ASSOCIATE-AC PDU (received on transport connection)"}
var Evt4 = &StateTransitionEvent{"Evt4", "A-ASSOCIATE-RJ PDU (received on transport connection)"}
var Evt5 = &StateTransitionEvent{"Evt5", "Transport connection indication (local transport service)"}
var Evt6 = &StateTransitionEvent{"Evt6", "A-ASSOCIATE-RQ PDU (on tranport connection)"}
var Evt7 = &StateTransitionEvent{"Evt7", "A-ASSOCIATE response primitive (accept)"}
var Evt8 = &StateTransitionEvent{"Evt8", "A-ASSOCIATE response primitive (reject)"}
var Evt9 = &StateTransitionEvent{"Evt9", "P-DATA request primitive"}
var Evt10 = &StateTransitionEvent{"Evt10", "P-DATA-TF PDU (on transport connection)"}
var Evt11 = &StateTransitionEvent{"Evt11", "A-RELEASE request primitive"}
var Evt12 = &StateTransitionEvent{"Evt12", "A-RELEASE-RQ PDU (on transport)"}
var Evt13 = &StateTransitionEvent{"Evt13", "A-RELEASE-RP PDU (on transport)"}
var Evt14 = &StateTransitionEvent{"Evt14", "A-RELEASE response primitive"}
var Evt15 = &StateTransitionEvent{"Evt15", "A-ABORT request primitive"}
var Evt16 = &StateTransitionEvent{"Evt16", "A-ABORT PDU (on transport)"}
var Evt17 = &StateTransitionEvent{"Evt17", "Transport connection closed indication (local transport service)"}
var Evt18 = &StateTransitionEvent{"Evt18", "ARTIM timer expired (Association reject/release timer)"}
var Evt19 = &StateTransitionEvent{"Evt19", "Unrecognized or invalid PDU received"}

type StateTransition struct {
	event   *StateTransitionEvent
	current *StateType
	action  *StateAction
}

var StateTransitions = []StateTransition{
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

type SessionParams struct {
	Peer           string // host:port
	CallingAeTitle string
	CalledAeTitle  string
}

type StateMachine struct {
	Params           SessionParams
	PData            []PresentationDataValueItem
	Requestor        int32
	connectionStatus int // Idle, Connecting, etc
	conn             net.Conn
	currentState     *StateType
}

func doassert(x bool) {
	if !x {
		panic("doassert")
	}
}

func startReadingPdu(sm *StateMachine) {
	doassert(sm.conn != nil)
	doassert(sm.connectionStatus == Connected)
	sm.connectionStatus = ReadingPDU
}

func startConnection(sm *StateMachine) {
	doassert(sm.conn == nil)
	doassert(sm.connectionStatus == Idle)
	sm.connectionStatus = Connecting
}

func closeConnection(sm *StateMachine) {
	doassert(sm.conn != nil)
	sm.conn.Close()
}

func sendPdu(sm *StateMachine, pdu PDU) {
}

func startTimer(sm *StateMachine)   { panic("startTimer") }
func restartTimer(sm *StateMachine) { panic("restartTimer") }
func stopTimer(sm *StateMachine)    { panic("stopTimer") }

func getNextEvent(sm *StateMachine) *StateTransitionEvent {
	if sm.connectionStatus == Idle {
		panic("blaoeu")
	}
	if sm.connectionStatus == Connecting {
		conn, err := net.Dial("tcp", sm.Params.Peer)
		if err != nil {
			return Evt17
		}
		sm.conn = conn
		sm.connectionStatus = Connected
		return Evt2
	}
	if sm.connectionStatus == ReadyToSend
	pdu, err := DecodePDU(sm.conn)
	if err != nil {
		return Evt19
	}
	if _, ok := pdu.(*A_ASSOCIATE_AC); ok {
		return Evt3
	}
	if _, ok := pdu.(*A_ASSOCIATE_RJ); ok {
		return Evt4
	}
	if _, ok := pdu.(*A_ASSOCIATE_RQ); ok {
		return Evt6
	}
	panic("oaue")
}

func findAction(
	event *StateTransitionEvent,
	currentState *StateType) *StateAction {
	panic("blah")
}

func runStep(sm *StateMachine) {
	event := getNextEvent(sm)
	action := findAction(sm.currentState, event)
	sm.currentState = action.Callback(sm)
}

func NewStateMachine(
	initialState* StateType,
	initialEvent* StateTransitionEvent) *StateMachine {
	sm := &StateMachine{}
	action := findAction(initialState, initialEvent)

	for {
		nextState, nextEvent := action.Callback(sm)
		sm.currentState = nextState
		if nextEvent == nil {
			break
		}
		action := findAction(sm.currentState, nextEvent)
	}
	return sm
}
