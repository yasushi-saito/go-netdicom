// A sample program for sending a DICOM file to a remote provider using C-STORE protocol.
package main

import (
	"encoding/binary"
	"flag"
	"io/ioutil"
	"log"

	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-dicom/dicomio"
	"github.com/yasushi-saito/go-netdicom"
	"github.com/yasushi-saito/go-netdicom/sopclass"
	"v.io/x/lib/vlog"
)

var (
	serverFlag = flag.String("server", "localhost:10000", "host:port of the remote application entity")
	storeFlag  = flag.String("store", "", "If set, issue C-STORE to copy this file to the remote server")
	findFlag   = flag.String("find", "aoeu", "blah")
)

func cStore(server, inPath string) {
	data, err := ioutil.ReadFile(inPath)
	if err != nil {
		log.Fatalf("%s: %v", inPath, err)
	}
	decoder := dicomio.NewBytesDecoder(data, binary.LittleEndian, dicomio.ExplicitVR)
	meta := dicom.ParseFileHeader(decoder)
	if decoder.Error() != nil {
		log.Fatalf("%s: failed to parse as DICOM: %v", inPath, decoder.Error())
	}
	transferSyntaxUID, err := dicom.LookupElementByTag(meta, dicom.TagTransferSyntaxUID)
	if err != nil {
		log.Fatal(err)
	}
	params := netdicom.NewServiceUserParams(
		"dontcare", "testclient", sopclass.StorageClasses,
		[]string{transferSyntaxUID.MustGetString()})
	su := netdicom.NewServiceUser(params)
	su.Connect(server)

	err = su.CStore(data)
	if err != nil {
		log.Fatalf("%s: cstore failed: %v", inPath, err)
	}
	log.Printf("C-STORE done!!")
	su.Release()
}

func cFind(server, argStr string) {
	params := netdicom.NewServiceUserParams(
		"dontcare", "testclient", sopclass.StorageClasses,
		[]string{dicom.ExplicitVRLittleEndian})
	su := netdicom.NewServiceUser(params)
	su.Connect(server)

	var args []*dicom.Element
	args = append(args,
		dicom.NewElement(dicom.TagQueryRetrievalLevel, "PATIENT"))
	args = append(args,
		dicom.NewElement(dicom.TagPatientName, "*"))
	_, err := su.CFind(args)
	if err != nil {
		log.Fatalf("C-FIND '%s' failed: %v", argStr, err)
	}
	log.Printf("C-FIND done!!")
	su.Release()
}

func main() {
	flag.Parse()
	vlog.ConfigureLibraryLoggerFromFlags()

	if *storeFlag != "" {
		cStore(*serverFlag, *storeFlag)
	} else if *findFlag != "" {
		cFind(*serverFlag, *findFlag)
	} else {
		vlog.Fatal("Either -store or -find must be set")
	}
}
