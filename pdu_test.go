package netdicom_test

import (
	"encoding/binary"
	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-netdicom"
	"testing"
)

func testDIMSE(t *testing.T, v netdicom.DIMSEMessage) {
	e := dicom.NewEncoder(binary.LittleEndian, dicom.ImplicitVR)
	netdicom.EncodeDIMSEMessage(e, v)
	bytes, err := e.Finish()
	if err != nil {
		t.Fatal(err)
	}
	d := dicom.NewBytesDecoder(bytes, binary.LittleEndian, dicom.ImplicitVR)
	v2 := netdicom.ReadDIMSEMessage(d)
	err = d.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if v.String() != v2.String() {
		t.Errorf("%v <-> %v", v, v2)
	}
}

func TestCStoreRq(t *testing.T) {
	testDIMSE(t, &netdicom.C_STORE_RQ{
		"1.2.3",
		0x1234,
		0x2345,
		1,
		"3.4.5",
		"foohah",
		0x3456})
}

func TestCStoreRsp(t *testing.T) {
	testDIMSE(t, &netdicom.C_STORE_RSP{
		"1.2.3",
		0x1234,
		netdicom.CommandDataSetTypeNull,
		"3.4.5",
		0x3456})
}

func TestCEchoRq(t *testing.T) {
	testDIMSE(t, &netdicom.C_ECHO_RQ{0x1234, 1})
}

func TestCEchoRsp(t *testing.T) {
	testDIMSE(t, &netdicom.C_ECHO_RSP{0x1234, 1, 0x2345})
}
