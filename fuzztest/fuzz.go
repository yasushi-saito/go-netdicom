package fuzztest

import (
	"github.com/yasushi-saito/go-netdicom"
	"io/ioutil"
	"log"
	"net"
)

func startServer(faults *netdicom.FaultInjector) string {
	netdicom.SetProviderFaultInjector(faults)
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Panic(err)
	}
	go func() {
		// TODO(saito) test w/ small PDU.
		params := netdicom.ServiceProviderParams{MaxPDUSize: 4096000}
		callbacks := netdicom.ServiceProviderCallbacks{
			CStore: func(transferSyntaxUID string,
				sopClassUID string,
				sopInstanceUID string,
				data []byte) uint16 {
				return 0
			},
		}

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Accept error: %v", err)
				continue
			}
			log.Printf("Accepted connection %v", conn)
			netdicom.RunProviderForConn(conn, params, callbacks)
		}
	}()
	return listener.Addr().String()
}

func runClient(serverAddr string, faults *netdicom.FaultInjector) {
	data, err := ioutil.ReadFile("../testdata/reportsi.dcm")
	if err != nil {
		log.Fatal(err)
	}
	transferSyntaxUID, err := netdicom.GetTransferSyntaxUIDInBytes(data)
	if err != nil {
		log.Fatal(err)
	}
	netdicom.SetUserFaultInjector(faults)

	params := netdicom.NewServiceUserParams(
		"dontcare", "testclient", netdicom.StorageClasses,
		[]string{transferSyntaxUID})
	su := netdicom.NewServiceUser(serverAddr, params)
	err = su.CStore(data)
	log.Printf("Store done with status: %v", err)
	su.Release()
}

func Fuzz(data []byte) int {
	serverAddr := startServer(netdicom.NewFaultInjector(data))
	runClient(serverAddr, netdicom.NewFaultInjector(data))
	return 0
}
