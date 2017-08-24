// A sample program for sending a DICOM file to a remote provider using C-STORE protocol.
package main

import (
	"flag"
	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-netdicom"
	"io/ioutil"
	"log"
)

var (
	fileFlag   = flag.String("file", "", "the DICOM file you want to parse")
	serverFlag = flag.String("server", "", "host:port of the DICOM service provider")
)

func main() {
	flag.Parse()

	if *serverFlag == "" || *fileFlag == "" {
		log.Fatal("Both --server and --file must be set")
	}
	data, err := ioutil.ReadFile(*fileFlag)
	if err != nil {
		log.Fatalf("%s: %v", *fileFlag, err)
	}
	file, err := dicom.ParseBytes(data)
	if err != nil {
		log.Fatalf("%s: failed to parse as DICOM: %v", *fileFlag, err)
	}
	sopInstanceUID, err := file.LookupElement("SOPInstanceUID")
	if err != nil {
		log.Fatalf("%s: file does not contain SOPInstanceUID: %v", *fileFlag, err)
	}

	transferSyntaxUID, err := file.LookupElement("TransferSyntaxUID")
	if err != nil {
		log.Fatalf("%s: file does not contain TransferSyntaxUID: %v", *fileFlag, err)
	}
	sopClassUID, err := file.LookupElement("SOPClassUID")
	if err != nil {
		log.Fatalf("%s: file does not contain AbstractSyntaxUID: %v", *fileFlag, err)
	}
	log.Printf("%s: DICOM transfersyntax:%s, abstractsyntax: %s, sopinstance: %s",
		*fileFlag, transferSyntaxUID, sopClassUID, sopInstanceUID)

	params := netdicom.NewServiceUserParams(
		*serverFlag, "dontcare", "testclient", netdicom.StorageClasses,
		[]string{dicom.MustGetString(*transferSyntaxUID)})
	su := netdicom.NewServiceUser(params)
	err = su.CStore(dicom.MustGetString(*sopClassUID), dicom.MustGetString(*sopInstanceUID), data)
	if err != nil {
		log.Fatalf("%s: cstore failed: %v", *fileFlag, err)
	}
	su.Release()
}
