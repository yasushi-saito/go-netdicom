package netdicom

import (
	"bytes"
	"github.com/yasushi-saito/go-dicom"
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

	// Called on receiving a P_DATA_TF message. If one message contains
	// items for multiple application contexts (very unlikely, but the spec
	// allows for it), this callback is run for each context ID.
	// OnDataCallback func(context string, value [][]byte)

	// A_ASSOCIATE_RQ arrived from a client. STA3
	// onAssociateRequest func(A_ASSOCIATE) ([]SubItem, bool)

	// Called on receiving C_STORE_RQ message. The handler should store the
	// given data and return either 0 on success, or one of CStoreStatus*
	// error codes.
	OnCStoreRequest func(data []byte) uint16
}

const DefaultMaxPDUSize uint32 = 4 << 20

type ServiceProvider struct {
	params   ServiceProviderParams
	listener net.Listener
}

type ServiceProviderSession struct {
	sp *ServiceProvider
	sm *StateMachine
}

// func onAssociateRequest(pdu A_ASSOCIATE) ([]SubItem, bool) {
// 	responses := []SubItem{
// 		&ApplicationContextItem{
// 			Name: DefaultApplicationContextItemName,
// 		},
// 	}

// 	for _, item := range pdu.Items {
// 		if n, ok := item.(*PresentationContextItem); ok {
// 			// TODO(saito) Need to pick the syntax preferred by us.
// 			// For now, just hardcode the syntax, ignoring the list
// 			// in RQ.
// 			//
// 			// var syntaxItem SubItem
// 			// for _, subitem := range(n.Items) {
// 			// 	log.Printf("Received PresentaionContext(%x): %v", n.ContextID, subitem.DebugString())
// 			// 	if n, ok := subitem.(*SubItemWithName); ok && n.Type == ItemTypeTransferSyntax {
// 			// 		syntaxItem = n
// 			// 		break
// 			// 	}
// 			// }
// 			// doassert(syntaxItem != nil)
// 			var syntaxItem = TransferSyntaxSubItem{
// 				Name: dicom.ImplicitVRLittleEndian,
// 			}
// 			responses = append(responses,
// 				&PresentationContextItem{
// 					Type:      ItemTypePresentationContextResponse,
// 					ContextID: n.ContextID,
// 					Result:    0, // accepted
// 					Items:     []SubItem{&syntaxItem}})
// 		}
// 	}
// 	// TODO(saito) Set the PDU size more properly.
// 	responses = append(responses,
// 		&UserInformationItem{
// 			Items: []SubItem{&UserInformationMaximumLengthItem{MaximumLengthReceived: 1 << 20}}})
// 	return responses, true
// }

type dataRequestState struct {
	contextID      byte
	command        []byte
	data           []byte
	readAllCommand bool
	readAllData    bool
}

func onDataRequest(sm *StateMachine, pdu P_DATA_TF, contextIDMap contextIDMap,
	state *dataRequestState, params ServiceProviderParams) {
	for _, item := range pdu.Items {
		if state.contextID == 0 {
			state.contextID = item.ContextID
		} else if state.contextID != item.ContextID {
			log.Panicf("Mixed context: %d %d", state.contextID, item.ContextID)
		}
		if item.Command {
			state.command = append(state.command, item.Value...)
			if item.Last {
				doassert(!state.readAllCommand)
				state.readAllCommand = true
			}
		} else {
			state.data = append(state.data, item.Value...)
			if item.Last {
				doassert(!state.readAllData)
				state.readAllData = true
			}
		}
		if !state.readAllCommand || !state.readAllData {
			return
		}
		syntaxName, err := contextIDToAbstractSyntaxName(&contextIDMap, state.contextID)
		command, err := DecodeDIMSEMessage(bytes.NewBuffer(state.command), int64(len(state.command)))
		log.Printf("Read all data for syntax %s, command [%v], data %d bytes, err%v",
			dicom.UIDDebugString(syntaxName), command, len(state.data), err)

		switch c := command.(type) {
		case *C_STORE_RQ:
			status := CStoreStatusCannotUnderstand
			if params.OnCStoreRequest != nil {
				status = params.OnCStoreRequest(state.data)
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
			sendData(sm, syntaxName, true /*command*/, bytes)
		default:
			panic("aoeu")
		}
	}
}

func NewServiceProvider(params ServiceProviderParams) *ServiceProvider {
	doassert(params.ListenAddr != "")
	if params.MaxPDUSize <= 0 {
		params.MaxPDUSize = DefaultMaxPDUSize
	}
	// doassert(params.OnCStoreRequest != nil)
	// TODO: move OnAssociateRequest outside the params
	//params.onAssociateRequest = onAssociateRequest

	//params.onDataRequest = func(pdu P_DATA_TF, contextIDMap contextIDMap) {
	//onDataRequest(dataState, pdu, contextIDMap)
	//}
	sp := &ServiceProvider{params: params}
	return sp
}

func (sp *ServiceProvider) Run() error {
	if sp.listener != nil {
		panic("Run called twice")
	}
	listener, err := net.Listen("tcp", sp.params.ListenAddr)
	if err != nil {
		return err
	}
	sp.listener = listener
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		log.Printf("Accept connection")
		dataState := &dataRequestState{}
		smParams := StateMachineParams{
			verbose:    true,
			maxPDUSize: sp.params.MaxPDUSize,
			// onAssociateRequest: onAssociateRequest,
			onDataRequest: func(sm *StateMachine, pdu P_DATA_TF, contextIDMap contextIDMap) {
				onDataRequest(sm, pdu, contextIDMap, dataState, sp.params)
			},
		}
		go RunStateMachineForServiceProvider(conn, smParams)
	}
}
