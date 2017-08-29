package netdicom_test

import (
	"errors"
	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-netdicom"
	"io/ioutil"
	"log"
	"net"
	"testing"
)

var serverAddr string
var cstoreData []byte

func onCStoreRequest(
	transferSyntaxUID string,
	sopClassUID string,
	sopInstanceUID string,
	data []byte) uint16 {
	log.Printf("Start C-STORE handler, transfersyntax=%s, sopclass=%s, sopinstance=%s",
		dicom.UIDString(transferSyntaxUID),
		dicom.UIDString(sopClassUID),
		dicom.UIDString(sopInstanceUID))

	// endian, implicit, err := dicom.ParseTransferSyntaxUID(transferSyntaxUID)
	// if err != nil {
	// 	log.Panic(err)
	// }

	//implicit = dicom.ExplicitVR
	e := dicom.NewEncoder(nil, dicom.UnknownVR)
	dicom.WriteFileHeader(e, transferSyntaxUID, sopClassUID, sopInstanceUID)
	e.WriteBytes(data)

	if cstoreData != nil {
		log.Panic("Received C-STORE data twice")
	}
	var err error
	cstoreData, err = e.Finish()
	if err != nil {
		log.Panic(err)
	}
	log.Print("Received C-STORE requset")
	return 0 // Success
}

func checkFileBodiesEqual(t *testing.T, in, out *dicom.DicomFile) {
	var removeMetaElems = func(f *dicom.DicomFile) []*dicom.DicomElement {
		var elems []*dicom.DicomElement
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

func getCStoreData() (*dicom.DicomFile, error) {
	if cstoreData == nil {
		return nil, errors.New("Did not receive C-STORE data")
	}
	f, err := dicom.ParseBytes(cstoreData)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func init() {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Panic(err)
	}
	go func() {
		params := netdicom.ServiceProviderParams{
			// TODO(saito) test w/ small PDU.
			MaxPDUSize:      4096000,
			OnCStoreRequest: onCStoreRequest}

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Accept error: %v", err)
				continue
			}
			log.Printf("Accepted connection %v", conn)
			netdicom.RunProviderForConn(conn, params)
		}
	}()
	serverAddr = listener.Addr().String()
}

func TestStoreSingleFile(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/IM-0001-0003.dcm")
	if err != nil {
		log.Fatal(err)
	}
	transferSyntaxUID, err := netdicom.GetTransferSyntaxUIDInBytes(data)
	if err != nil {
		log.Fatal(err)
	}
	params := netdicom.NewServiceUserParams(
		serverAddr, "dontcare", "testclient", netdicom.StorageClasses,
		[]string{transferSyntaxUID})
	su := netdicom.NewServiceUser(params)
	err = su.CStore(data)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Store done!!")
	su.Release()

	out, err := getCStoreData()
	if err != nil {
		log.Fatal(err)
	}
	in, err := dicom.ParseBytes(data)
	if err != nil {
		log.Fatal(err)
	}
	checkFileBodiesEqual(t, in, out)
}
