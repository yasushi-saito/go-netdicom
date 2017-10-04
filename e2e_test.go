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
	"io/ioutil"
	"net"
	"sync"
	"testing"
	"v.io/x/lib/vlog"
)

var serverAddr string
var cstoreData []byte

var once sync.Once

func initTest() {
	once.Do(func() {
		flag.Parse()
		vlog.ConfigureLibraryLoggerFromFlags()
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			vlog.Fatal(err)
		}
		go func() {
			params := netdicom.ServiceProviderParams{
				CStore: onCStoreRequest,
				CFind:  onCFindRequest,
			}
			for {
				conn, err := listener.Accept()
				if err != nil {
					vlog.Infof("Accept error: %v", err)
					continue
				}
				vlog.Infof("Accepted connection %v", conn)
				netdicom.RunProviderForConn(conn, params)
			}
		}()
		serverAddr = listener.Addr().String()
	})
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

	if cstoreData != nil {
		vlog.Fatal("Received C-STORE data twice")
	}
	cstoreData = e.Bytes()
	vlog.Infof("Received C-STORE request")
	return dimse.Status{Status: dimse.StatusSuccess}
}

func onCFindRequest(
	transferSyntaxUID string,
	sopClassUID string,
	filters []*dicom.Element) chan netdicom.CFindResult {
	ch := make(chan netdicom.CFindResult, 128)
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
	return ch
}

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

func readDICOMFile(path string) ([]byte, string) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		vlog.Fatal(err)
	}
	transferSyntaxUID, err := netdicom.GetTransferSyntaxUIDInBytes(data)
	if err != nil {
		vlog.Fatal(err)
	}
	return data, transferSyntaxUID
}

func TestStoreSingleFile(t *testing.T) {
	initTest()
	data, transferSyntaxUID := readDICOMFile("testdata/IM-0001-0003.dcm")
	params, err := netdicom.NewServiceUserParams(
		"dontcare", "testclient", sopclass.StorageClasses,
		[]string{transferSyntaxUID})
	if err != nil {
		vlog.Fatal(err)
	}
	su := netdicom.NewServiceUser(params)
	su.Connect(serverAddr)
	err = su.CStoreRaw(data)
	if err != nil {
		vlog.Fatal(err)
	}
	vlog.Infof("Store done!!")
	su.Release()

	out, err := getCStoreData()
	if err != nil {
		vlog.Fatal(err)
	}
	in, err := dicom.ReadDataSetInBytes(data, dicom.ReadOptions{})
	if err != nil {
		vlog.Fatal(err)
	}
	checkFileBodiesEqual(t, in, out)
}

func TestFind(t *testing.T) {
	initTest()
	params, err := netdicom.NewServiceUserParams(
		"dontcare", "testclient", sopclass.QRFindClasses,
		dicomio.StandardTransferSyntaxes)
	su := netdicom.NewServiceUser(params)
	if err != nil {
		vlog.Fatal(err)
	}
	su.Connect(serverAddr)
	filter := []*dicom.Element{
		dicom.MustNewElement(dicom.TagPatientName, "foohah"),
	}
	var namesFound []string

	for result := range su.CFind(netdicom.CFindPatientQRLevel, filter) {
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

func TestNonexistentServer(t *testing.T) {
	initTest()
	data, transferSyntaxUID := readDICOMFile("testdata/IM-0001-0003.dcm")
	params, err := netdicom.NewServiceUserParams(
		"dontcare", "testclient", sopclass.StorageClasses,
		[]string{transferSyntaxUID})
	if err != nil {
		t.Fatal(err)
	}
	su := netdicom.NewServiceUser(params)
	su.Connect(":99999")
	err = su.CStoreRaw(data)
	if err == nil || err.Error() != "Connection failed" {
		vlog.Fatalf("Expect CStore to fail: %v", err)
	}
	su.Release()
}

// TODO(saito) Test that the state machine shuts down propelry.
