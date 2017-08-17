package netdicom

import (
	"log"
	"net"
)

type ServiceProvider struct {
	listenAddr string
	listener net.Listener
	callbacks StateCallbacks
}

type ServiceProviderSession struct {
	sp *ServiceProvider
	sm* StateMachine
}

func onAssociateRequest(pdu A_ASSOCIATE) ([]SubItem, bool) {
	return pdu.Items, true
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
