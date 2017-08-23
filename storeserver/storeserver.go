package main

import (
	"flag"
	"github.com/yasushi-saito/go-netdicom"
	"log"
	"strings"
)

var (
	portFlag = flag.String("port", "10000", "TCP port to listen to")
)

func onCStoreRequest(data []byte) uint16 {
	log.Printf("ONCSTORE! %d bytes", len(data))
	return 0 // Success
}

func main() {
	flag.Parse()
	port := *portFlag
	if !strings.Contains(port, ":") {
		port = ":" + port
	}
	log.Printf("Listening on %s", port)
	params := netdicom.ServiceProviderParams{
		ListenAddr:      port,
		OnCStoreRequest: onCStoreRequest,
	}
	su := netdicom.NewServiceProvider(params)
	err := su.Run()
	if err != nil {
		panic(err)
	}
}
