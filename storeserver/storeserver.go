package main

import (
	"flag"
	"fmt"
	"github.com/yasushi-saito/go-netdicom"
	"io/ioutil"
	"log"
	"strings"
	"sync/atomic"
)

var (
	portFlag = flag.String("port", "10000", "TCP port to listen to")
)

var pathSeq int32

func onCStoreRequest(data []byte) uint16 {
	path := fmt.Sprintf("image%04d.dcm", atomic.AddInt32(&pathSeq, 1))
	log.Printf("Writing %s", path)
	err := ioutil.WriteFile(path, data, 0644)
	if err != nil {
		log.Printf("%s: failed to write: %v", path, err)
		return netdicom.CStoreStatusOutOfResources
	}
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
