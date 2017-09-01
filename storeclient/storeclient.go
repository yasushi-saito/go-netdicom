// A sample program for sending a DICOM file to a remote provider using C-STORE protocol.
package main

import (
	"encoding/binary"
	"flag"
	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-netdicom"
	"io/ioutil"
	"log"
)

func main() {
	flag.Parse()
	vlog.ConfigureLibraryLoggerFromFlags()
	if len(flag.Args()) != 2 {
		log.Fatal("Usage: storeclient <serverhost:port> <file>")
	}
	server, inPath := flag.Arg(0), flag.Arg(1)

	data, err := ioutil.ReadFile(inPath)
	if err != nil {
		log.Fatalf("%s: %v", inPath, err)
	}
	decoder := dicom.NewBytesDecoder(data, binary.LittleEndian, dicom.ExplicitVR)
	meta := dicom.ParseFileHeader(decoder)
	if decoder.Error() != nil {
		log.Fatalf("%s: failed to parse as DICOM: %v", inPath, decoder.Error())
	}
	transferSyntaxUID, err := dicom.LookupElementByTag(meta, dicom.TagTransferSyntaxUID)
	if err != nil {
		log.Fatal(err)
	}
	params := netdicom.NewServiceUserParams(
		"dontcare", "testclient", netdicom.StorageClasses,
		[]string{transferSyntaxUID.MustGetString()})
	su := netdicom.NewServiceUser(server, params)

	err = su.CStore(data)
	if err != nil {
		log.Fatalf("%s: cstore failed: %v", inPath, err)
	}
	log.Printf("Store done!!")
	su.Release()
}
