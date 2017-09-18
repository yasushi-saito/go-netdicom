package netdicom_test

import (
	"errors"
	"flag"
	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-dicom/dicomio"
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
			// TODO(saito) test w/ small PDU.
			params := netdicom.ServiceProviderParams{MaxPDUSize: 4096000}
			callbacks := netdicom.ServiceProviderCallbacks{CStore: onCStoreRequest}
			for {
				conn, err := listener.Accept()
				if err != nil {
					vlog.Infof("Accept error: %v", err)
					continue
				}
				vlog.Infof("Accepted connection %v", conn)
				netdicom.RunProviderForConn(conn, params, callbacks)
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
		dicom.UIDString(transferSyntaxUID),
		dicom.UIDString(sopClassUID),
		dicom.UIDString(sopInstanceUID))
	e := dicomio.NewEncoder(nil, dicomio.UnknownVR)
	dicom.WriteFileHeader(e, transferSyntaxUID, sopClassUID, sopInstanceUID)
	e.WriteBytes(data)

	if cstoreData != nil {
		vlog.Fatal("Received C-STORE data twice")
	}
	var err error
	cstoreData, err = e.Finish()
	if err != nil {
		vlog.Fatal(err)
	}
	vlog.Infof("Received C-STORE requset")
	return dimse.Status{Status: dimse.StatusSuccess}
}

func checkFileBodiesEqual(t *testing.T, in, out *dicom.DataSet) {
	var removeMetaElems = func(f *dicom.DataSet) []*dicom.Element {
		var elems []*dicom.Element
		for _, elem := range f.Elements {
			if elem.Tag.Group != dicom.TagMetadataGroup {
				elems = append(elems, &elem)
			}
		}
		return elems
	}

	inElems := removeMetaElems(in)
	outElems := removeMetaElems(out)
	if len(inElems) != len(outElems) {
		t.Error("Wrong # of elems: in %d, out %d", len(inElems), len(outElems))
	}
	for i := 0; i < len(inElems); i++ {
		ins := inElems[i].String()
		outs := outElems[i].String()
		if ins != outs {
			t.Error("%dth element mismatch: %v <-> %v", i, ins, outs)
		}
	}
}

func getCStoreData() (*dicom.DataSet, error) {
	if cstoreData == nil {
		return nil, errors.New("Did not receive C-STORE data")
	}
	f, err := dicom.ParseBytes(cstoreData)
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
	params := netdicom.NewServiceUserParams(
		"dontcare", "testclient", sopclass.StorageClasses,
		[]string{transferSyntaxUID})
	su := netdicom.NewServiceUser(params)
	su.Connect(serverAddr)
	err := su.CStore(data)
	if err != nil {
		vlog.Fatal(err)
	}
	vlog.Infof("Store done!!")
	su.Release()

	out, err := getCStoreData()
	if err != nil {
		vlog.Fatal(err)
	}
	in, err := dicom.ParseBytes(data)
	if err != nil {
		vlog.Fatal(err)
	}
	checkFileBodiesEqual(t, in, out)
}

func TestNonexistentServer(t *testing.T) {
	initTest()
	data, transferSyntaxUID := readDICOMFile("testdata/IM-0001-0003.dcm")
	params := netdicom.NewServiceUserParams(
		"dontcare", "testclient", sopclass.StorageClasses,
		[]string{transferSyntaxUID})
	su := netdicom.NewServiceUser(params)
	su.Connect(":99999")
	err := su.CStore(data)
	if err == nil || err.Error() != "Connection failed" {
		vlog.Fatalf("Expect CStore to fail: %v", err)
	}
	su.Release()
}

// TODO(saito) Test that the state machine shuts down propelry.
