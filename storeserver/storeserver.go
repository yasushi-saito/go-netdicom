package main

import (
	"flag"
	"io/ioutil"
	"fmt"
	"encoding/binary"
	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-netdicom"
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
	e.EncodeBytes(data)
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
