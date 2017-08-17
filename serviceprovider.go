package netdicom

import (
	"log"
	"net"
// "github.com/yasushi-saito/go-dicom"
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

func NewServiceProvider(listenAddr string) *ServiceProvider {
	return &ServiceProvider{listenAddr: listenAddr}
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
