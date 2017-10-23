// A sample program for sending a DICOM file to a remote provider using C-STORE protocol.
package main

import (
	"flag"

	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-dicom/dicomuid"
	"github.com/yasushi-saito/go-netdicom"
	"github.com/yasushi-saito/go-netdicom/sopclass"
	"v.io/x/lib/vlog"
)

var (
	serverFlag        = flag.String("server", "localhost:10000", "host:port of the remote application entity")
	storeFlag         = flag.String("store", "", "If set, issue C-STORE to copy this file to the remote server")
	aeTitleFlag       = flag.String("ae-title", "testclient", "AE title of the client")
	remoteAETitleFlag = flag.String("remote-ae-title", "testserver", "AE title of the server")
	findFlag          = flag.String("find", "", "blah")
)

func cStore(server, inPath string) {
	dataset, err := dicom.ReadDataSetFromFile(inPath, dicom.ReadOptions{})
	if err != nil {
		vlog.Fatalf("%s: %v", inPath, err)
	}
	su, err := netdicom.NewServiceUser(netdicom.ServiceUserParams{
		CalledAETitle:  *aeTitleFlag,
		CallingAETitle: *remoteAETitleFlag,
		SOPClasses:     sopclass.StorageClasses})
	if err != nil {
		vlog.Fatal(err)
	}
	defer su.Release()
	su.Connect(server)

	err = su.CStore(dataset)
	if err != nil {
		vlog.Fatalf("%s: cstore failed: %v", inPath, err)
	}
	vlog.Infof("C-STORE done!!")
}

func cFind(server, argStr string) {
	su, err := netdicom.NewServiceUser(netdicom.ServiceUserParams{
		CalledAETitle:    *aeTitleFlag,
		CallingAETitle:   *remoteAETitleFlag,
		SOPClasses:       sopclass.QRFindClasses,
		TransferSyntaxes: []string{dicomuid.ExplicitVRLittleEndian}, // for testing
	})
	if err != nil {
		vlog.Fatal(err)
	}

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
