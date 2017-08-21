package netdicom

import (
	"log"
	"net"
)

type ServiceProviderParams struct {
	ListenAddr string
	MaximumPDUSize uint32
	// Called on receiving a P_DATA_TF message. If one message contains
	// items for multiple application contexts (very unlikely, but the spec
	// allows for it), this callback is run for each context ID.
	OnDataCallback func(context string, value [][]byte)

	// A_ASSOCIATE_RQ arrived from a client. STA3
	onAssociateRequest func(A_ASSOCIATE) ([]SubItem, bool)
}

func NewServiceProviderParams(listenAddr string) ServiceProviderParams {
	return ServiceProviderParams{
		ListenAddr: listenAddr,
		MaximumPDUSize: 1 << 20,
	}
}

type ServiceProvider struct {
	params ServiceProviderParams
	listener   net.Listener
}

type ServiceProviderSession struct {
	sp *ServiceProvider
	sm *StateMachine
}

func onAssociateRequest(pdu A_ASSOCIATE) ([]SubItem, bool) {
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
				Name: ImplicitVRLittleEndian,
			}
			responses = append(responses,
				&PresentationContextItem{
					Type: ItemTypePresentationContextResponse,
					ContextID: n.ContextID,
					Result:    0, // accepted
					Items:     []SubItem{&syntaxItem}})
		}
	}
	// TODO(saito) Set the PDU size more properly.
	responses = append(responses,
		&UserInformationItem{
			Items: []SubItem{&UserInformationMaximumLengthItem{MaximumLengthReceived: 1 << 20}}})
	return responses, true
}

func NewServiceProvider(params ServiceProviderParams) *ServiceProvider {
	// TODO: move OnAssociateRequest outside the params
	params.onAssociateRequest = onAssociateRequest
	sp := &ServiceProvider{
		params: params,
	}
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
		go RunStateMachineForServiceProvider(conn, sp.params)
	}
}
