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
	v2 := netdicom.ReadDIMSEMessage2(d)
	err = d.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if v.String() != v2.String() {
		t.Errorf("%v <-> %v", v, v2)
	}
}

// Encode and decode C_ECHO_RQ.
func TestCEchoRq(t *testing.T) {
	testDIMSE(t, &netdicom.C_ECHO_RQ{0x1234, 1})
}

// Encode and decode C_ECHO_RSP.
func TestCEchoRsp(t *testing.T) {
	testDIMSE(t, &netdicom.C_ECHO_RSP{0x1234, 1, 0x2345})
}
