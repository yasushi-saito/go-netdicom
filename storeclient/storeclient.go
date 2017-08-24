// A sample program for sending a DICOM file to a remote provider using C-STORE protocol.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-netdicom"
	"io/ioutil"
	"log"
)

var (
//	fileFlag   = flag.String("file", "", "the DICOM file you want to parse")
//	serverFlag = flag.String("server", "", "host:port of the DICOM service provider")
)

func main() {
	flag.Parse()
	if len(flag.Args()) != 2 {
		log.Fatal("Usage: storeclient <serverhost:port> <file>")
	}
	server, inPath := flag.Arg(0), flag.Arg(1)

	data, err := ioutil.ReadFile(inPath)
	if err != nil {
		log.Fatalf("%s: %v", inPath, err)
	}

	decoder := dicom.NewDecoder(
		bytes.NewBuffer(data),
		int64(len(data)),
		binary.LittleEndian,
		dicom.ExplicitVR)
	meta := dicom.ParseFileHeader(decoder)
	if decoder.Error() != nil {
		log.Fatalf("%s: failed to parse as DICOM: %v", inPath, decoder.Error())
	}
	sopInstanceUID, err := dicom.LookupElement(meta, "MediaSOPInstanceUID")
	if err != nil {
		log.Fatalf("%s: file does not contain SOPInstanceUID: %v", inPath, err)
	}

	transferSyntaxUID, err := dicom.LookupElement(meta, "TransferSyntaxUID")
	if err != nil {
		log.Fatalf("%s: file does not contain TransferSyntaxUID: %v", inPath, err)
	}
	sopClassUID, err := dicom.LookupElement(meta, "MediaSOPClassUID")
	if err != nil {
		log.Fatalf("%s: file does not contain AbstractSyntaxUID: %v", inPath, err)
	}
	log.Printf("%s: DICOM transfersyntax:%s, abstractsyntax: %s, sopinstance: %s",
		inPath, transferSyntaxUID, sopClassUID, sopInstanceUID)

	params := netdicom.NewServiceUserParams(
		server, "dontcare", "testclient", netdicom.StorageClasses,
		[]string{dicom.MustGetString(*transferSyntaxUID)})
	su := netdicom.NewServiceUser(params)

	body := decoder.DecodeBytes(int(decoder.Len()))
	if decoder.Error() != nil {
		log.Panic("read")
	}
	err = su.CStore(dicom.MustGetString(*sopClassUID), dicom.MustGetString(*sopInstanceUID), body)
	if err != nil {
		log.Fatalf("%s: cstore failed: %v", inPath, err)
	}
	su.Release()
}
