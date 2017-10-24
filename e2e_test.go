package netdicom_test

import (
	"errors"
	"flag"
	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-dicom/dicomio"
	"github.com/yasushi-saito/go-dicom/dicomuid"
	"github.com/yasushi-saito/go-netdicom"
	"github.com/yasushi-saito/go-netdicom/dimse"
	"github.com/yasushi-saito/go-netdicom/sopclass"
	"sync"
	"testing"
	"v.io/x/lib/vlog"
)

var provider *netdicom.ServiceProvider
var cstoreData []byte            // data received by the cstore handler
var cstoreStatus = dimse.Success // status returned by the cstore handler
var nEchoRequests int

var once sync.Once

func initTest() {
	once.Do(func() {
		flag.Parse()
		vlog.ConfigureLibraryLoggerFromFlags()
		var err error
		provider, err = netdicom.NewServiceProvider(netdicom.ServiceProviderParams{
			CEcho:  onCEchoRequest,
			CStore: onCStoreRequest,
			CFind:  onCFindRequest,
			CGet:   onCGetRequest,
		}, ":0")
		if err != nil {
			vlog.Fatal(err)
		}
		go provider.Run()
	})
}

func onCEchoRequest() dimse.Status {
	nEchoRequests++
	return dimse.Success
}

func onCStoreRequest(
	transferSyntaxUID string,
	sopClassUID string,
	sopInstanceUID string,
	data []byte) dimse.Status {
	vlog.Infof("Start C-STORE handler, transfersyntax=%s, sopclass=%s, sopinstance=%s",
		dicomuid.UIDString(transferSyntaxUID),
		dicomuid.UIDString(sopClassUID),
		dicomuid.UIDString(sopInstanceUID))
	e := dicomio.NewBytesEncoder(nil, dicomio.UnknownVR)
	dicom.WriteFileHeader(e,
		[]*dicom.Element{
			dicom.MustNewElement(dicom.TagTransferSyntaxUID, transferSyntaxUID),
			dicom.MustNewElement(dicom.TagMediaStorageSOPClassUID, sopClassUID),
			dicom.MustNewElement(dicom.TagMediaStorageSOPInstanceUID, sopInstanceUID),
		})
	e.WriteBytes(data)
	cstoreData = e.Bytes()
	vlog.Infof("Received C-STORE request, %d bytes", len(cstoreData))
	return dimse.Success
}

func onCFindRequest(
	transferSyntaxUID string,
	sopClassUID string,
	filters []*dicom.Element,
	ch chan netdicom.CFindResult) {
	vlog.Infof("Received cfind request")
	found := 0
	for _, elem := range filters {
		vlog.Infof("Filter %v", elem)
		if elem.Tag == dicom.TagQueryRetrieveLevel {
			if elem.MustGetString() != "PATIENT" {
				vlog.Fatalf("Wrong QR level: %v", elem)
			}
			found++
		}
		if elem.Tag == dicom.TagPatientName {
			if elem.MustGetString() != "foohah" {
				vlog.Fatalf("Wrong patient name: %v", elem)
			}
			found++
		}
	}
	if found != 2 {
		vlog.Fatalf("Didn't find expected filters: %v", filters)
	}
	ch <- netdicom.CFindResult{
		Elements: []*dicom.Element{dicom.MustNewElement(dicom.TagPatientName, "johndoe")},
	}
	ch <- netdicom.CFindResult{
		Elements: []*dicom.Element{dicom.MustNewElement(dicom.TagPatientName, "johndoe2")},
	}
	close(ch)
}

func onCGetRequest(
	transferSyntaxUID string,
	sopClassUID string,
	filters []*dicom.Element,
	ch chan netdicom.CMoveResult) {
	vlog.Infof("Received cget request")
	path := "testdata/IM-0001-0003.dcm"
	dataset := readDICOMFile(path)
	ch <- netdicom.CMoveResult{
		Remaining: -1,
		Path:      path,
		DataSet:   dataset,
	}
	close(ch)
}

// Check that two datasets, "in" and "out" are the same, except for metadata
// elements.
func checkFileBodiesEqual(t *testing.T, in, out *dicom.DataSet) {
	var removeMetaElems = func(f *dicom.DataSet) []*dicom.Element {
		var elems []*dicom.Element
		for _, elem := range f.Elements {
			if elem.Tag.Group != dicom.TagMetadataGroup {
				elems = append(elems, elem)
			}
		}
		return elems
	}

	inElems := removeMetaElems(in)
	outElems := removeMetaElems(out)
	if len(inElems) != len(outElems) {
		t.Errorf("Wrong # of elems: in %d, out %d", len(inElems), len(outElems))
	}
	for i := 0; i < len(inElems); i++ {
		ins := inElems[i].String()
		outs := outElems[i].String()
		if ins != outs {
			t.Errorf("%dth element mismatch: %v <-> %v", i, ins, outs)
		}
	}
}

func getCStoreData() (*dicom.DataSet, error) {
	if cstoreData == nil {
		return nil, errors.New("Did not receive C-STORE data")
	}
	f, err := dicom.ReadDataSetInBytes(cstoreData, dicom.ReadOptions{})
	if err != nil {
		return nil, err
	}
	return f, nil
}

func readDICOMFile(path string) *dicom.DataSet {
	dataset, err := dicom.ReadDataSetFromFile(path, dicom.ReadOptions{})
	if err != nil {
		vlog.Fatal(err)
	}
	return dataset
}

func newServiceUser(t *testing.T, sopClasses []string) *netdicom.ServiceUser {
	initTest()
	su, err := netdicom.NewServiceUser(netdicom.ServiceUserParams{SOPClasses: sopClasses})
	if err != nil {
		t.Fatal(err)
	}
	vlog.Infof("Connecting to %v", provider.ListenAddr().String())
	su.Connect(provider.ListenAddr().String())
	return su
}

func TestStoreSingleFile(t *testing.T) {
	initTest()
	dataset := readDICOMFile("testdata/IM-0001-0003.dcm")
	su := newServiceUser(t, sopclass.StorageClasses)
	defer su.Release()
	err := su.CStore(dataset)
	if err != nil {
		vlog.Fatal(err)
	}
	vlog.Infof("Store done!!")

	out, err := getCStoreData()
	if err != nil {
		vlog.Fatal(err)
	}
	checkFileBodiesEqual(t, dataset, out)
}

func TestEcho(t *testing.T) {
	su := newServiceUser(t, sopclass.VerificationClasses)
	defer su.Release()
	oldCount := nEchoRequests
	if err := su.CEcho(); err != nil {
		vlog.Fatal(err)
	}
	if nEchoRequests != oldCount+1 {
		vlog.Fatal("cecho handler did not run")
	}
}

func TestFind(t *testing.T) {
	su := newServiceUser(t, sopclass.QRFindClasses)
	defer su.Release()
	filter := []*dicom.Element{
		dicom.MustNewElement(dicom.TagPatientName, "foohah"),
	}
	var namesFound []string

	for result := range su.CFind(netdicom.QRLevelPatient, filter) {
		vlog.Errorf("Got result: %v", result)
		if result.Err != nil {
			t.Error(result.Err)
			continue
		}
		for _, elem := range result.Elements {
			if elem.Tag != dicom.TagPatientName {
				t.Error(elem)
			}
			namesFound = append(namesFound, elem.MustGetString())
		}
	}
	if len(namesFound) != 2 || namesFound[0] != "johndoe" || namesFound[1] != "johndoe2" {
		t.Error(namesFound)
	}
}

func TestCGet(t *testing.T) {
	su := newServiceUser(t, sopclass.QRGetClasses)
	defer su.Release()
	filter := []*dicom.Element{
		dicom.MustNewElement(dicom.TagPatientName, "foohah"),
	}

	var cgetData []byte

	err := su.CGet(netdicom.QRLevelPatient, filter,
		func(transferSyntaxUID, sopClassUID, sopInstanceUID string, data []byte) dimse.Status {
			vlog.Infof("Got data: %v %v %v %d bytes", transferSyntaxUID, sopClassUID, sopInstanceUID, len(data))
			if len(cgetData) > 0 {
				t.Fatal("Received multiple C-GET responses")
			}
			e := dicomio.NewBytesEncoder(nil, dicomio.UnknownVR)
			dicom.WriteFileHeader(e,
				[]*dicom.Element{
					dicom.MustNewElement(dicom.TagTransferSyntaxUID, transferSyntaxUID),
					dicom.MustNewElement(dicom.TagMediaStorageSOPClassUID, sopClassUID),
					dicom.MustNewElement(dicom.TagMediaStorageSOPInstanceUID, sopInstanceUID),
				})
			e.WriteBytes(data)
			cgetData = e.Bytes()
			return dimse.Success
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(cgetData) == 0 {
		t.Fatal("No data received")
	}
	ds, err := dicom.ReadDataSetInBytes(cgetData, dicom.ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	expected := readDICOMFile("testdata/IM-0001-0003.dcm")
	checkFileBodiesEqual(t, expected, ds)
}

func TestNonexistentServer(t *testing.T) {
	initTest()
	dataset := readDICOMFile("testdata/IM-0001-0003.dcm")
	su, err := netdicom.NewServiceUser(netdicom.ServiceUserParams{
		SOPClasses: sopclass.StorageClasses})
	if err != nil {
		t.Fatal(err)
	}
	su.Connect(":99999")
	err = su.CStore(dataset)
	if err == nil || err.Error() != "Connection failed" {
		vlog.Fatalf("Expect CStore to fail: %v", err)
	}
	su.Release()
}

// TODO(saito) Test that the state machine shuts down propelry.
