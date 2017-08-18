package netdicom

import (
	"log"
	"net"
)

type ServiceProvider struct {
	listenAddr string
	listener   net.Listener
	callbacks  StateCallbacks
}

type ServiceProviderSession struct {
	sp *ServiceProvider
	sm *StateMachine
}

func onAssociateRequest(pdu A_ASSOCIATE) ([]SubItem, bool) {
	responses := []SubItem{
		&SubItemWithName{
			Type: ItemTypeApplicationContext,
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
			var syntaxItem = SubItemWithName{
				Type: ItemTypeTransferSyntax,
				Name: ImplicitVRLittleEndian,
			}
			responses = append(responses,
				&PresentationContextItem{
					Type: ItemTypePresentationContextAC,
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

func NewServiceProvider(listenAddr string) *ServiceProvider {
	sp := &ServiceProvider{
		listenAddr: listenAddr,
	}
	sp.callbacks.OnAssociateRequest = onAssociateRequest
	return sp
}

func (sp *ServiceProvider) Run() error {
	if sp.listener != nil {
		panic("Run called twice")
	}
	listener, err := net.Listen("tcp", sp.listenAddr)
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
		go RunStateMachineForServiceProvider(conn, sp.callbacks)
	}
}
