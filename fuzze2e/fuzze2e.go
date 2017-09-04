package fuzze2e

import (
	"flag"
	"github.com/yasushi-saito/go-netdicom"
	"github.com/yasushi-saito/go-netdicom/sopclass"
	"io/ioutil"
	"log"
	"net"
)

func startServer(faults *netdicom.FaultInjector) net.Listener {
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
				break
			}
			log.Printf("Accepted connection %v", conn)
			netdicom.RunProviderForConn(conn, params, callbacks)
		}
	}()
	return listener
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
		"dontcare", "testclient", sopclass.StorageClasses,
		[]string{transferSyntaxUID})
	su := netdicom.NewServiceUser(params)
	su.Connect(serverAddr)
	err = su.CStore(data)
	log.Printf("Store done with status: %v", err)
	su.Release()
}

func init() {
	flag.Parse()
}

func Fuzz(data []byte) int {
	listener := startServer(netdicom.NewFaultInjector(data))
	runClient(listener.Addr().String(), netdicom.NewFaultInjector(data))
	listener.Close()
	return 0
}
