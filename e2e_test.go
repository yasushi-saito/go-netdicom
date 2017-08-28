package netdicom_test

import (
	// "fmt"
	"encoding/binary"
	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-netdicom"
	"io/ioutil"
	"log"
	"net"
	"testing"
)

func onCStoreRequest(
	transferSyntaxUID string,
	sopClassUID string,
	sopInstanceUID string,
	data []byte) uint16 {

	e := dicom.NewEncoder(binary.LittleEndian, dicom.ExplicitVR)
	dicom.WriteFileHeader(e, transferSyntaxUID, sopClassUID, sopInstanceUID)
	e.EncodeBytes(data)
	_, err := e.Finish()
	if err != nil {
		log.Panic(err)
	}
	log.Print("Received C-STORE requset")
	return 0 // Success
}

var serverAddr string

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

func TestStoreSingleFile(t*testing.T) {
	data, err := ioutil.ReadFile("testdata/IM-0001-0003.dcm")
	if err != nil {
		log.Fatal(err)
	}
	decoder := dicom.NewBytesDecoder(data, binary.LittleEndian, dicom.ExplicitVR)
	meta := dicom.ParseFileHeader(decoder)
	if decoder.Error() != nil {
		log.Fatal(decoder.Error())
	}
	transferSyntaxUID, err := dicom.LookupElementByTag(meta, dicom.TagTransferSyntaxUID)
	if err != nil {
		log.Fatal(err)
	}
	params := netdicom.NewServiceUserParams(
		serverAddr, "dontcare", "testclient", netdicom.StorageClasses,
		[]string{dicom.MustGetString(*transferSyntaxUID)})
	su := netdicom.NewServiceUser(params)
	err = su.CStore(data)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Store done!!")
	su.Release()
}
