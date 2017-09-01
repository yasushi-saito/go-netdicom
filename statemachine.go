package netdicom

// Implements the network statemachine, as defined in P3.8 9.2.3.
// http://dicom.nema.org/medical/dicom/current/output/pdf/part08.pdf

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/yasushi-saito/go-dicom"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

type stateType struct {
	Name        string
	Description string
}

func (s *stateType) String() string {
	return fmt.Sprintf("%s(%s)", s.Name, s.Description)
}

var smSeq int32 = 32 // for assignign unique stateMachine.name

var (
	sta01 = &stateType{"Sta01", "Idle"}
	sta02 = &stateType{"Sta02", "Transport connection open (Awaiting A-ASSOCIATE-RQ PDU)"}
	sta03 = &stateType{"Sta03", "Awaiting local A-ASSOCIATE response primitive (from local user)"}
	sta04 = &stateType{"Sta04", "Awaiting transport connection opening to complete (from local transport service)"}
	sta05 = &stateType{"Sta05", "Awaiting A-ASSOCIATE-AC or A-ASSOCIATE-RJ PDU"}
	sta06 = &stateType{"Sta06", "Association established and ready for data transfer"}
	sta07 = &stateType{"Sta07", "Awaiting A-RELEASE-RP PDU"}
	sta08 = &stateType{"Sta08", "Awaiting local A-RELEASE response primitive (from local user)"}
	sta09 = &stateType{"Sta09", "Release collision requestor side; awaiting A-RELEASE response (from local user)"}
	sta10 = &stateType{"Sta10", "Release collision acceptor side; awaiting A-RELEASE-RP PDU"}
	sta11 = &stateType{"Sta11", "Release collision requestor side; awaiting A-RELEASE-RP PDU"}
	sta12 = &stateType{"Sta12", "Release collision acceptor side; awaiting A-RELEASE response primitive (from local user)"}
	sta13 = &stateType{"Sta13", "Awaiting Transport Connection Close Indication (Association no longer exists)"}
)

type eventType struct {
	Event       int
	Description string
}

var (
	evt01 = eventType{1, "A-ASSOCIATE request (local user)"}
	evt02 = eventType{2, "Connection established (for service user)"}
	evt03 = eventType{3, "A-ASSOCIATE-AC PDU (received on transport connection)"}
	evt04 = eventType{4, "A-ASSOCIATE-RJ PDU (received on transport connection)"}
	evt05 = eventType{5, "Connection accepted (for service provider)"}
	evt06 = eventType{6, "A-ASSOCIATE-RQ PDU (on tranport connection)"}
	evt07 = eventType{7, "A-ASSOCIATE response primitive (accept)"}
	evt08 = eventType{8, "A-ASSOCIATE response primitive (reject)"}
	evt09 = eventType{9, "P-DATA request primitive"}
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

type stateAction struct {
	Name        string
	Description string
	Callback    func(sm *stateMachine, event stateEvent) *stateType
}

func (s *stateAction) String() string {
	return fmt.Sprintf("%s(%s)", s.Name, s.Description)
}

var actionAe1 = &stateAction{"AE-1",
	"Issue TRANSPORT CONNECT request primitive to local transport service",
	func(sm *stateMachine, event stateEvent) *stateType {
		if event.conn == nil && event.serverAddr == "" {
			glog.Fatalf("%s: illegal event %v", sm.name, event)
		}
		go func(ch chan stateEvent, serverHostPort string) {
			conn, err := net.Dial("tcp", serverHostPort)
			if err != nil {
				glog.Infof("%s: Failed to connect to %s: %v", sm.name, serverHostPort, err)
				ch <- stateEvent{event: evt17, pdu: nil, err: err}
				close(ch)
				return
			}
			ch <- stateEvent{event: evt02, pdu: nil, err: nil, conn: conn}
			networkReaderThread(ch, conn, sm.userParams.MaxPDUSize, sm.name)
		}(sm.netCh, event.serverAddr)
		return sta04
	}}

// Generate an item list to be embedded in an A_REQUEST_RQ PDU. The PDU is sent
// when running as a service user.
func buildAssociateRequestItems(m *contextManager, params ServiceUserParams) []SubItem {
	items := []SubItem{
		&ApplicationContextItem{
			Name: DefaultApplicationContextItemName,
		}}
	for _, item := range m.generateAssociateRequest(
		params.RequiredServices,
		params.SupportedTransferSyntaxes) {
		items = append(items, item)
	}
	items = append(items,
		&UserInformationItem{
			Items: []SubItem{
				&UserInformationMaximumLengthItem{uint32(params.MaxPDUSize)},
				&ImplementationClassUIDSubItem{dicom.DefaultImplementationClassUID},
				&ImplementationVersionNameSubItem{dicom.DefaultImplementationVersionName}}})
	return items
}

var actionAe2 = &stateAction{"AE-2", "Send A-ASSOCIATE-RQ-PDU",
	func(sm *stateMachine, event stateEvent) *stateType {
		items := buildAssociateRequestItems(sm.contextManager, sm.userParams)
		pdu := &A_ASSOCIATE{
			Type:            PDUTypeA_ASSOCIATE_RQ,
			ProtocolVersion: CurrentProtocolVersion,
			CalledAETitle:   sm.userParams.CalledAETitle,
			CallingAETitle:  sm.userParams.CallingAETitle,
			Items:           items,
		}
		sendPDU(sm, pdu)
		startTimer(sm)
		return sta05
	}}

var actionAe3 = &stateAction{"AE-3", "Issue A-ASSOCIATE confirmation (accept) primitive",
	func(sm *stateMachine, event stateEvent) *stateType {
		stopTimer(sm)
		pdu := event.pdu.(*A_ASSOCIATE)
		doassert(pdu.Type == PDUTypeA_ASSOCIATE_AC)
		var items []*PresentationContextItem
		for _, item := range pdu.Items {
			if n, ok := item.(*PresentationContextItem); ok {
				items = append(items, n)
			}
		}
		err := sm.contextManager.onAssociateResponse(items)
		if err == nil {
			sm.upcallCh <- upcallEvent{eventType: upcallEventHandshakeCompleted}
			sm.maxPDUSize = sm.userParams.MaxPDUSize // TODO(saito) Extract from response!
			doassert(sm.maxPDUSize > 0)
			return sta06
		} else {
			glog.Error(err)
			return actionAa8.Callback(sm, event)
		}
	}}

var actionAe4 = &stateAction{"AE-4", "Issue A-ASSOCIATE confirmation (reject) primitive and close transport connection",
	func(sm *stateMachine, event stateEvent) *stateType {
		closeConnection(sm)
		return sta01
	}}

var actionAe5 = &stateAction{"AE-5", "Issue Transport connection response primitive; start ARTIM timer",
	func(sm *stateMachine, event stateEvent) *stateType {
		doassert(event.conn != nil)
		startTimer(sm)
		go func(ch chan stateEvent, conn net.Conn) {
			networkReaderThread(ch, conn, sm.providerParams.MaxPDUSize, sm.name)
		}(sm.netCh, event.conn)
		return sta02
	}}

func extractPresentationContextItems(items []SubItem) []*PresentationContextItem {
	var contextItems []*PresentationContextItem
	for _, item := range items {
		if n, ok := item.(*PresentationContextItem); ok {
			contextItems = append(contextItems, n)
		}
	}
	return contextItems
}

var actionAe6 = &stateAction{"AE-6", `Stop ARTIM timer and if A-ASSOCIATE-RQ acceptable by "
service-dul: issue A-ASSOCIATE indication primitive
otherwise issue A-ASSOCIATE-RJ-PDU and start ARTIM timer`,
	func(sm *stateMachine, event stateEvent) *stateType {
		stopTimer(sm)
		pdu := event.pdu.(*A_ASSOCIATE)
		if pdu.ProtocolVersion != 0x0001 {
			glog.Infof("%s: Wrong remote protocol version 0x%x", sm.name, pdu.ProtocolVersion)
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
		items, err := sm.contextManager.onAssociateRequest(extractPresentationContextItems(pdu.Items))
		if err != nil {
			// TODO(saito) set proper error code.
			sm.downcallCh <- stateEvent{
				event: evt08,
				pdu: &A_ASSOCIATE_RJ{
					Result: ResultRejectedPermanent,
					Source: SourceULServiceProviderACSE,
					Reason: 1,
				},
			}
		} else {
			for _, item := range items {
				responses = append(responses, item)
			}
			// TODO(saito) Set the PDU size more properly.
			responses = append(responses,
				&UserInformationItem{
					Items: []SubItem{&UserInformationMaximumLengthItem{MaximumLengthReceived: uint32(sm.providerParams.MaxPDUSize)}}})
			// TODO(saito) extract the user params.
			sm.maxPDUSize = sm.providerParams.MaxPDUSize
			doassert(sm.maxPDUSize > 0)
			doassert(len(responses) > 0)
			doassert(pdu.CalledAETitle != "")
			doassert(pdu.CallingAETitle != "")
			sm.downcallCh <- stateEvent{
				event: evt07,
				pdu: &A_ASSOCIATE{
					Type:            PDUTypeA_ASSOCIATE_AC,
					ProtocolVersion: CurrentProtocolVersion,
					CalledAETitle:   pdu.CalledAETitle,
					CallingAETitle:  pdu.CallingAETitle,
					Items:           responses,
				},
			}
		}
		return sta03
	}}
var actionAe7 = &stateAction{"AE-7", "Send A-ASSOCIATE-AC PDU",
	func(sm *stateMachine, event stateEvent) *stateType {
		sendPDU(sm, event.pdu.(*A_ASSOCIATE))
		sm.upcallCh <- upcallEvent{eventType: upcallEventHandshakeCompleted}
		return sta06
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
		// TODO(saito) Don't crash here.
		glog.Fatalf("%s: Illegal syntax name %s: %s", sm.name, dicom.UIDString(abstractSyntaxName), err)
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
	return pdus
}

// Data transfer related actions
var actionDt1 = &stateAction{"DT-1", "Send P-DATA-TF PDU",
	func(sm *stateMachine, event stateEvent) *stateType {
		doassert(event.dataPayload != nil)
		pdus := splitDataIntoPDUs(sm, event.dataPayload.abstractSyntaxName, event.dataPayload.command, event.dataPayload.data)
		for _, pdu := range pdus {
			sendPDU(sm, &pdu)
		}
		return sta06
	}}

var actionDt2 = &stateAction{"DT-2", "Send P-DATA indication primitive",
	func(sm *stateMachine, event stateEvent) *stateType {
		abstractSyntaxUID, transferSyntaxUID, command, data, err := addPDataTF(&sm.commandAssembler, event.pdu.(*P_DATA_TF), sm.contextManager)
		if err == nil {
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
			return sta06
		} else {
			glog.Infof("%s: Failed to assemble data: %v", sm.name, err) // TODO(saito)
			return actionAa8.Callback(sm, event)
		}
	}}

// Assocation Release related actions
var actionAr1 = &stateAction{"AR-1", "Send A-RELEASE-RQ PDU",
	func(sm *stateMachine, event stateEvent) *stateType {
		sendPDU(sm, &A_RELEASE_RQ{})
		return sta07
	}}
var actionAr2 = &stateAction{"AR-2", "Issue A-RELEASE indication primitive",
	func(sm *stateMachine, event stateEvent) *stateType {
		// TODO(saito) Do RELEASE callback here.
		sm.downcallCh <- stateEvent{event: evt14}
		return sta08
	}}

var actionAr3 = &stateAction{"AR-3", "Issue A-RELEASE confirmation primitive and close transport connection",
	func(sm *stateMachine, event stateEvent) *stateType {
		sendPDU(sm, &A_RELEASE_RP{})
		closeConnection(sm)
		return sta01
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
		return sta01
	}}

var actionAr6 = &stateAction{"AR-6", "Issue P-DATA indication",
	func(sm *stateMachine, event stateEvent) *stateType {
		return sta07
	}}

var actionAr7 = &stateAction{"AR-7", "Issue P-DATA-TF PDU",
	func(sm *stateMachine, event stateEvent) *stateType {
		doassert(event.dataPayload != nil)
		pdus := splitDataIntoPDUs(sm, event.dataPayload.abstractSyntaxName, event.dataPayload.command, event.dataPayload.data)
		for _, pdu := range pdus {
			sendPDU(sm, &pdu)
		}
		sm.downcallCh <- stateEvent{event: evt14}
		return sta08
	}}

var actionAr8 = &stateAction{"AR-8", "Issue A-RELEASE indication (release collision): if association-requestor, next state is Sta09, if not next state is Sta10",
	func(sm *stateMachine, event stateEvent) *stateType {
		if sm.isUser {
			return sta09
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
		if sm.currentState == sta02 {
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
		return sta01
	}}

var actionAa3 = &stateAction{"AA-3", "If (service-user initiated abort): issue A-ABORT indication and close transport connection, otherwise (service-dul initiated abort): issue A-P-ABORT indication and close transport connection",
	func(sm *stateMachine, event stateEvent) *stateType {
		closeConnection(sm)
		return sta01
	}}

var actionAa4 = &stateAction{"AA-4", "Issue A-P-ABORT indication primitive",
	func(sm *stateMachine, event stateEvent) *stateType {
		return sta01
	}}

var actionAa5 = &stateAction{"AA-5", "Stop ARTIM timer",
	func(sm *stateMachine, event stateEvent) *stateType {
		stopTimer(sm)
		return sta01
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

type stateEventDebugInfo struct {
	state *stateType // the state the system was in when timer was created.
}

type stateEvent struct {
	event eventType
	pdu   PDU
	err   error
	conn  net.Conn

	serverAddr  string                 // host:port to connect to. Set only for evt01
	dataPayload *stateEventDataPayload // set iff event==evt09.
	debug       *stateEventDebugInfo
}

func (e *stateEvent) String() string {
	debug := ""
	if e.debug != nil {
		debug = e.debug.state.String()
	}
	return fmt.Sprintf("type:%d(%s) err:%v debug:%v pdu:%v", e.event.Event, e.event.Description, e.err, debug, e.pdu)
}

type stateTransition struct {
	current *stateType
	event   eventType
	action  *stateAction
}

var stateTransitions = []stateTransition{
	stateTransition{sta01, evt01, actionAe1},
	stateTransition{sta01, evt05, actionAe5},
	stateTransition{sta02, evt03, actionAa1},
	stateTransition{sta02, evt04, actionAa1},
	stateTransition{sta02, evt06, actionAe6},
	stateTransition{sta02, evt10, actionAa1},
	stateTransition{sta02, evt12, actionAa1},
	stateTransition{sta02, evt13, actionAa1},
	stateTransition{sta02, evt16, actionAa2},
	stateTransition{sta02, evt17, actionAa5},
	stateTransition{sta02, evt18, actionAa2},
	stateTransition{sta02, evt19, actionAa1},
	stateTransition{sta03, evt03, actionAa8},
	stateTransition{sta03, evt04, actionAa8},
	stateTransition{sta03, evt06, actionAa8},
	stateTransition{sta03, evt07, actionAe7},
	stateTransition{sta03, evt08, actionAe8},
	stateTransition{sta03, evt10, actionAa8},
	stateTransition{sta03, evt12, actionAa8},
	stateTransition{sta03, evt13, actionAa8},
	stateTransition{sta03, evt15, actionAa1},
	stateTransition{sta03, evt16, actionAa3},
	stateTransition{sta03, evt17, actionAa4},
	stateTransition{sta03, evt19, actionAa8},
	stateTransition{sta04, evt02, actionAe2},
	stateTransition{sta04, evt15, actionAa2},
	stateTransition{sta04, evt17, actionAa4},
	stateTransition{sta05, evt03, actionAe3},
	stateTransition{sta05, evt04, actionAe4},
	stateTransition{sta05, evt06, actionAa8},
	stateTransition{sta05, evt10, actionAa8},
	stateTransition{sta05, evt12, actionAa8},
	stateTransition{sta05, evt13, actionAa8},
	stateTransition{sta05, evt15, actionAa1},
	stateTransition{sta05, evt16, actionAa3},
	stateTransition{sta05, evt17, actionAa4},
	stateTransition{sta05, evt18, actionAa8},
	stateTransition{sta05, evt19, actionAa8},

	stateTransition{sta06, evt03, actionAa8},
	stateTransition{sta06, evt04, actionAa8},
	stateTransition{sta06, evt06, actionAa8},
	stateTransition{sta06, evt09, actionDt1},
	stateTransition{sta06, evt10, actionDt2},
	stateTransition{sta06, evt11, actionAr1},
	stateTransition{sta06, evt12, actionAr2},
	stateTransition{sta06, evt13, actionAa8},
	stateTransition{sta06, evt15, actionAa1},
	stateTransition{sta06, evt16, actionAa3},
	stateTransition{sta06, evt17, actionAa4},
	stateTransition{sta06, evt19, actionAa8},
	stateTransition{sta07, evt03, actionAa8},
	stateTransition{sta07, evt04, actionAa8},
	stateTransition{sta07, evt06, actionAa8},
	stateTransition{sta07, evt10, actionAr6},
	stateTransition{sta07, evt12, actionAr8},
	stateTransition{sta07, evt13, actionAr3},
	stateTransition{sta07, evt15, actionAa1},
	stateTransition{sta07, evt16, actionAa3},
	stateTransition{sta07, evt17, actionAa4},
	stateTransition{sta07, evt19, actionAa8},
	stateTransition{sta08, evt03, actionAa8},
	stateTransition{sta08, evt04, actionAa8},
	stateTransition{sta08, evt06, actionAa8},
	stateTransition{sta08, evt09, actionAr7},
	stateTransition{sta08, evt10, actionAa8},
	stateTransition{sta08, evt12, actionAa8},
	stateTransition{sta08, evt13, actionAa8},
	stateTransition{sta08, evt14, actionAr4},
	stateTransition{sta08, evt15, actionAa1},
	stateTransition{sta08, evt16, actionAa3},
	stateTransition{sta08, evt17, actionAa4},
	stateTransition{sta08, evt19, actionAa8},
	stateTransition{sta09, evt03, actionAa8},
	stateTransition{sta09, evt04, actionAa8},
	stateTransition{sta09, evt06, actionAa8},
	stateTransition{sta09, evt10, actionAa8},
	stateTransition{sta09, evt12, actionAa8},
	stateTransition{sta09, evt13, actionAa8},
	stateTransition{sta09, evt14, actionAr9},
	stateTransition{sta09, evt15, actionAa1},
	stateTransition{sta09, evt16, actionAa3},
	stateTransition{sta09, evt17, actionAa4},
	stateTransition{sta09, evt19, actionAa8},
	stateTransition{sta10, evt03, actionAa8},
	stateTransition{sta10, evt04, actionAa8},
	stateTransition{sta10, evt06, actionAa8},
	stateTransition{sta10, evt10, actionAa8},
	stateTransition{sta10, evt12, actionAa8},
	stateTransition{sta10, evt13, actionAr10},
	stateTransition{sta10, evt15, actionAa1},
	stateTransition{sta10, evt16, actionAa3},
	stateTransition{sta10, evt17, actionAa4},
	stateTransition{sta10, evt19, actionAa8},
	stateTransition{sta11, evt03, actionAa8},
	stateTransition{sta11, evt04, actionAa8},
	stateTransition{sta11, evt06, actionAa8},
	stateTransition{sta11, evt10, actionAa8},
	stateTransition{sta11, evt12, actionAa8},
	stateTransition{sta11, evt13, actionAr3},
	stateTransition{sta11, evt15, actionAa1},
	stateTransition{sta11, evt16, actionAa3},
	stateTransition{sta11, evt17, actionAa4},
	stateTransition{sta11, evt19, actionAa8},
	stateTransition{sta12, evt03, actionAa8},
	stateTransition{sta12, evt04, actionAa8},
	stateTransition{sta12, evt06, actionAa8},
	stateTransition{sta12, evt10, actionAa8},
	stateTransition{sta12, evt12, actionAa8},
	stateTransition{sta12, evt13, actionAa8},
	stateTransition{sta12, evt14, actionAr4},
	stateTransition{sta12, evt15, actionAa1},
	stateTransition{sta12, evt16, actionAa3},
	stateTransition{sta12, evt17, actionAa4},
	stateTransition{sta12, evt19, actionAa8},

	stateTransition{sta13, evt03, actionAa6},
	stateTransition{sta13, evt04, actionAa6},
	stateTransition{sta13, evt06, actionAa7},
	stateTransition{sta13, evt07, actionAa7},
	stateTransition{sta13, evt08, actionAa7},
	stateTransition{sta13, evt09, actionAa7},
	stateTransition{sta13, evt10, actionAa6},
	stateTransition{sta13, evt11, actionAa6},
	stateTransition{sta13, evt12, actionAa6},
	stateTransition{sta13, evt13, actionAa6},
	stateTransition{sta13, evt14, actionAa6},
	stateTransition{sta13, evt15, actionAa2},
	stateTransition{sta13, evt16, actionAa2},
	stateTransition{sta13, evt17, actionAr5},
	stateTransition{sta13, evt18, actionAa2},
	stateTransition{sta13, evt19, actionAa7},
}

const (
	Idle = iota
	Connecting
	Connected
	ReadingPDU
)

type stateMachine struct {
	name           string // For logging only
	isUser         bool   // true if service user, false if provider
	userParams     ServiceUserParams
	providerParams ServiceProviderParams

	// abstractSyntaxMap maps a contextID (an odd integer) to an abstract
	// syntax string such as 1.2.840.10008.5.1.4.1.1.1.2.  This field is set
	// on receiving A_ASSOCIATE_RQ message. Thus, it is set only on the
	// provider side (not the user).
	contextManager *contextManager
	//contextIDToAbstractSyntaxNameMap map[byte]string
	//abstractSyntaxNameToContextIDMap map[string]byte

	// For receiving PDU and network status events.
	// Owned by networkReaderThread.
	netCh chan stateEvent

	// For reporting errors to this event.  Owned by the statemachine.
	errorCh chan stateEvent

	// For receiving commands from the upper layer
	// Owned by the upper layer.
	downcallCh chan stateEvent

	// For sending indications to the the upper layer. Owned by the
	// statemachine.
	upcallCh chan upcallEvent

	// For Timer expiration event
	timerCh      chan stateEvent
	conn         net.Conn
	currentState *stateType

	// The negotiated PDU size.
	maxPDUSize int

	commandAssembler dimseCommandAssembler
	faults           *FaultInjector
}

func closeConnection(sm *stateMachine) {
	close(sm.upcallCh)
	glog.Infof("%s: Closing connection %v", sm.name, sm.conn)
	sm.conn.Close()
}

func sendPDU(sm *stateMachine, pdu PDU) {
	doassert(sm.conn != nil)
	data, err := EncodePDU(pdu)
	if err != nil {
		glog.Infof("%s: Failed to encode: %v; closing connection %v", sm.name, err, sm.conn)
		sm.conn.Close()
		sm.errorCh <- stateEvent{event: evt17, err: err}
		return
	}
	if sm.faults != nil {
		action := sm.faults.onSend(data)
		if action == faultInjectorDisconnect {
			glog.Infof("%s: FAULT: closing connection for test", sm.name)
			sm.conn.Close()
		}
	}
	n, err := sm.conn.Write(data)
	if n != len(data) || err != nil {
		glog.Infof("%s: Failed to write %d bytes. Actual %d bytes : %v; closing connection %v", sm.name, len(data), n, err, sm.conn)
		sm.conn.Close()
		sm.errorCh <- stateEvent{event: evt17, err: err}
		return
	}
	// glog.Infof("%s: sendPDU: %v", sm.name, pdu.String())
}

func startTimer(sm *stateMachine) {
	ch := make(chan stateEvent, 1)
	sm.timerCh = ch
	currentState := sm.currentState
	time.AfterFunc(time.Duration(10)*time.Second,
		func() {
			ch <- stateEvent{event: evt18, debug: &stateEventDebugInfo{currentState}}
			close(ch)
		})
}

func restartTimer(sm *stateMachine) {
	startTimer(sm)
}

func stopTimer(sm *stateMachine) {
	sm.timerCh = make(chan stateEvent, 1)
}

func networkReaderThread(ch chan stateEvent, conn net.Conn, maxPDUSize int, smName string) {
	glog.V(1).Infof("%s: Starting network reader for %v, maxPDU %d", smName, conn, maxPDUSize)
	doassert(maxPDUSize > 16*1024)
	for {
		pdu, err := ReadPDU(conn, maxPDUSize)
		if err != nil {
			glog.Infof("%s: Failed to read PDU: %v", err, smName)
			if err == io.EOF {
				ch <- stateEvent{event: evt17, pdu: nil, err: nil}
			} else {
				ch <- stateEvent{event: evt19, pdu: nil, err: err}
			}
			close(ch)
			break
		}
		doassert(pdu != nil)
		switch n := pdu.(type) {
			case *A_ASSOCIATE:
			if n.Type == PDUTypeA_ASSOCIATE_RQ {
				ch <- stateEvent{event: evt06, pdu: n, err: nil}
			} else {
				doassert(n.Type == PDUTypeA_ASSOCIATE_AC)
				ch <- stateEvent{event: evt03, pdu: n, err: nil}
			}
			continue
		case *A_ASSOCIATE_RJ:
			ch <- stateEvent{event: evt04, pdu: n, err: nil}
			continue
		case *P_DATA_TF:
			ch <- stateEvent{event: evt10, pdu: n, err: nil}
			continue
		case *A_RELEASE_RQ:
			ch <- stateEvent{event: evt12, pdu: n, err: nil}
			continue
		case *A_RELEASE_RP:
			ch <- stateEvent{event: evt13, pdu: n, err: nil}
			continue
		case *A_ABORT:
			ch <- stateEvent{event: evt16, pdu: n, err: nil}
			continue
		default:
			err := fmt.Errorf("%s: Unknown PDU type: %v", pdu.String(), smName)
			ch <- stateEvent{event: evt19, pdu: pdu, err: err}
			glog.Error(err)
			continue
		}
	}
	glog.V(1).Infof("%s: Exiting network reader for %v", conn, smName)
}

func getNextEvent(sm *stateMachine) stateEvent {
	var ok bool
	var event stateEvent
	var channel string
	for event.event.Event == 0 {
		select {
		case event, ok = <-sm.netCh:
			channel = "net"
			if !ok {
				sm.netCh = nil
			}
		case event = <-sm.errorCh:
			channel = "error"
			// this channel shall never close.
		case event, ok = <-sm.timerCh:
			channel = "timer"
			if !ok {
				sm.timerCh = nil
			}
		case event, ok = <-sm.downcallCh:
			channel = "downcall"
			if !ok {
				sm.downcallCh = nil
			}
		}
	}
	if event.event.Event == 0 {
		glog.Fatalf("%s: received null event from channel '%s', sm: %v",
			sm.name, channel, sm)
	}
	switch event.event {
	case evt02:
		doassert(event.conn != nil)
		sm.conn = event.conn
	case evt17:
		close(sm.upcallCh)
		sm.conn = nil
	}
	return event
}

func findAction(currentState *stateType, event *stateEvent, smName string) *stateAction {
	for _, t := range stateTransitions {
		if t.current == currentState && t.event == event.event {
			return t.action
		}
	}
	return nil
}

const DefaultMaximiumPDUSize = uint32(1 << 20)

func runOneStep(sm *stateMachine) {
	event := getNextEvent(sm)
	glog.V(1).Infof("%s: Current state: %v, Event %v", sm.name, sm.currentState, event)
	action := findAction(sm.currentState, &event, sm.name)
	if action == nil {
		msg := fmt.Sprintf("%s: No action found for state %v, event %v", sm.name, sm.currentState, event.String())
		if sm.faults != nil {
			msg += " FIhistory: " + sm.faults.String()
		}
		glog.Infof("Unknown state transition:")
		for _, s := range strings.Split(msg, "\n") {
			glog.Infof(s)
		}
		glog.Fatalf(msg)
	}
	if sm.faults != nil {
		sm.faults.onStateTransition(sm.currentState, &event, action)
	}
	glog.V(1).Infof("%s: Running action %v", sm.name, action)
	sm.currentState = action.Callback(sm, event)
	glog.V(1).Infof("Next state: %v", sm.currentState)
}

func runStateMachineForServiceUser(
	serverAddr string,
	params ServiceUserParams,
	upcallCh chan upcallEvent,
	downcallCh chan stateEvent) {
	doassert(serverAddr != "")
	doassert(params.CallingAETitle != "")
	doassert(len(params.RequiredServices) > 0)
	doassert(len(params.SupportedTransferSyntaxes) > 0)
	sm := &stateMachine{
		name:           fmt.Sprintf("sm(u)-%d", atomic.AddInt32(&smSeq, 1)),
		isUser:         true,
		contextManager: newContextManager(),
		userParams:     params,
		netCh:          make(chan stateEvent, 128),
		errorCh:        make(chan stateEvent, 128),
		downcallCh:     downcallCh,
		upcallCh:       upcallCh,
		faults:         GetUserFaultInjector(),
	}
	event := stateEvent{event: evt01, serverAddr: serverAddr}
	action := findAction(sta01, &event, sm.name)
	sm.currentState = action.Callback(sm, event)
	for sm.currentState != sta01 {
		runOneStep(sm)
	}
	glog.V(1).Info("Connection shutdown")
}

func runStateMachineForServiceProvider(
	conn net.Conn,
	params ServiceProviderParams,
	upcallCh chan upcallEvent,
	downcallCh chan stateEvent) {
	sm := &stateMachine{
		name:           fmt.Sprintf("sm(p)-%d", atomic.AddInt32(&smSeq, 1)),
		isUser:         false,
		providerParams: params,
		contextManager: newContextManager(),
		conn:           conn,
		netCh:          make(chan stateEvent, 128),
		errorCh:        make(chan stateEvent, 128),
		downcallCh:     downcallCh,
		upcallCh:       upcallCh,
		faults:         GetProviderFaultInjector(),
	}
	event := stateEvent{event: evt05, conn: conn}
	action := findAction(sta01, &event, sm.name)
	sm.currentState = action.Callback(sm, event)
	for sm.currentState != sta01 {
		runOneStep(sm)
	}
	glog.V(1).Info("Connection shutdown")
}
