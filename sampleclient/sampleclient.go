// A sample program for sending a DICOM file to a remote provider using C-STORE protocol.
package main

import (
	"encoding/binary"
	"flag"
	"io/ioutil"

	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-dicom/dicomio"
	"github.com/yasushi-saito/go-dicom/dicomuid"
	"github.com/yasushi-saito/go-netdicom"
	"github.com/yasushi-saito/go-netdicom/sopclass"
	"v.io/x/lib/vlog"
)

var (
	serverFlag = flag.String("server", "localhost:10000", "host:port of the remote application entity")
	storeFlag  = flag.String("store", "", "If set, issue C-STORE to copy this file to the remote server")
	findFlag   = flag.String("find", "", "blah")
)

func cStore(server, inPath string) {
	data, err := ioutil.ReadFile(inPath)
	if err != nil {
		vlog.Fatalf("%s: %v", inPath, err)
	}
	decoder := dicomio.NewBytesDecoder(data, binary.LittleEndian, dicomio.ExplicitVR)
	meta := dicom.ParseFileHeader(decoder)
	if decoder.Error() != nil {
		vlog.Fatalf("%s: failed to parse as DICOM: %v", inPath, decoder.Error())
	}
	transferSyntaxUID, err := dicom.LookupElementByTag(meta, dicom.TagTransferSyntaxUID)
	if err != nil {
		vlog.Fatal(err)
	}
	params, err := netdicom.NewServiceUserParams(
		"dontcare", "testclient", sopclass.StorageClasses,
		[]string{transferSyntaxUID.MustGetString()})
	if err != nil {
		vlog.Fatal(err)
	}
	su := netdicom.NewServiceUser(params)
	defer su.Release()
	su.Connect(server)

	err = su.CStoreRaw(data)
	if err != nil {
		vlog.Fatalf("%s: cstore failed: %v", inPath, err)
	}
	vlog.Infof("C-STORE done!!")
}

func cFind(server, argStr string) {
	params, err := netdicom.NewServiceUserParams(
		"dontcare", "testclient", sopclass.QRFindClasses,
		[]string{dicomuid.ExplicitVRLittleEndian})
	if err != nil {
		vlog.Fatal(err)
	}
	su := netdicom.NewServiceUser(params)
	defer su.Release()
	vlog.Infof("Connecting to %s", server)
	su.Connect(server)
	args := []*dicom.Element{
		dicom.MustNewElement(dicom.TagSpecificCharacterSet, "ISO_IR 100"),
		dicom.MustNewElement(dicom.TagAccessionNumber, ""),
		dicom.MustNewElement(dicom.TagReferringPhysicianName, ""),
		dicom.MustNewElement(dicom.TagPatientName, ""),
		dicom.MustNewElement(dicom.TagPatientID, ""),
		dicom.MustNewElement(dicom.TagPatientBirthDate, ""),
		dicom.MustNewElement(dicom.TagPatientSex, ""),
		dicom.MustNewElement(dicom.TagStudyInstanceUID, ""),
		dicom.MustNewElement(dicom.TagRequestedProcedureDescription, ""),
		dicom.MustNewElement(dicom.TagScheduledProcedureStepSequence,
			dicom.MustNewElement(dicom.TagItem,
				dicom.MustNewElement(dicom.TagModality, ""),
				dicom.MustNewElement(dicom.TagScheduledProcedureStepStartDate, ""),
				dicom.MustNewElement(dicom.TagScheduledProcedureStepStartTime, ""),
				dicom.MustNewElement(dicom.TagScheduledPerformingPhysicianName, ""),
				dicom.MustNewElement(dicom.TagScheduledProcedureStepStatus, ""))),
	}
	for result := range su.CFind(netdicom.CFindStudyQRLevel, args) {
		if result.Err != nil {
			vlog.Errorf("C-FIND error: %v", result.Err)
			continue
		}
		vlog.Errorf("Got response with %d elems", len(result.Elements))
		for _, elem := range result.Elements {
			vlog.Errorf("Got elem: %v", elem.String())
		}
	}
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
