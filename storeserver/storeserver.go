package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/yasushi-saito/go-dicom"
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

func onCStoreRequest(
	transferSyntaxUID string,
	sopClassUID string,
	sopInstanceUID string,
	data []byte) uint16 {
	path := fmt.Sprintf("image%04d.dcm", atomic.AddInt32(&pathSeq, 1))

	log.Printf("Writing %s", path)
	e := dicom.NewEncoder(binary.LittleEndian, dicom.ExplicitVR)
	dicom.WriteFileHeader(e, transferSyntaxUID, sopClassUID, sopInstanceUID)
	e.WriteBytes(data)
	bytes, err := e.Finish()

	if err != nil {
		log.Printf("%s: failed to write: %v", path, err)
		return netdicom.CStoreStatusOutOfResources
	}
	err = ioutil.WriteFile(path, bytes, 0644)
	if err != nil {
		log.Printf("%s: %s", path, err)
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
	params := netdicom.ServiceProviderParams{}
	callbacks := netdicom.ServiceProviderCallbacks{CStore: onCStoreRequest}
	su := netdicom.NewServiceProvider(params, callbacks)
	err := su.Run(port)
	if err != nil {
		panic(err)
	}
}
