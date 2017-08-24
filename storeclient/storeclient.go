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
		log.Fatalf("%s: file does not contain SOPInstanceUID", *fileFlag)
	}

	syntaxUID, err := file.LookupElement("TransferSyntaxUID")
	if err != nil {
		log.Fatalf("%s: file does not contain TransferSyntaxUID", *fileFlag)
	}
	log.Printf("%s: DICOM transfer format: %s", *fileFlag, syntaxUID)

	params := netdicom.NewServiceUserParams(
		*serverFlag, "dontcare", "testclient", netdicom.StorageClasses)
	su := netdicom.NewServiceUser(params)

	// TODO(saito) Pick the syntax UID more properly.
	su.CStore(dicom.MustGetString(*syntaxUID),
		dicom.MustGetString(*sopInstanceUID), data)
	su.Release()
}
