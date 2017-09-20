package dimse_test

import (
	"encoding/binary"
	"github.com/yasushi-saito/go-dicom/dicomio"
	"github.com/yasushi-saito/go-netdicom/dimse"
	"testing"
)

func testDIMSE(t *testing.T, v dimse.Message) {
	e := dicomio.NewBytesEncoder(binary.LittleEndian, dicomio.ImplicitVR)
	dimse.EncodeMessage(e, v)
	bytes := e.Bytes()
	d := dicomio.NewBytesDecoder(bytes, binary.LittleEndian, dicomio.ImplicitVR)
	v2 := dimse.ReadMessage(d)
	err := d.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if v.String() != v2.String() {
		t.Errorf("%v <-> %v", v, v2)
	}
}

func TestCStoreRq(t *testing.T) {
	testDIMSE(t, &dimse.C_STORE_RQ{
		"1.2.3",
		0x1234,
		0x2345,
		1,
		"3.4.5",
		"foohah",
		0x3456, nil})
}

func TestCStoreRsp(t *testing.T) {
	testDIMSE(t, &dimse.C_STORE_RSP{
		"1.2.3",
		0x1234,
		dimse.CommandDataSetTypeNull,
		"3.4.5",
		dimse.Status{Status: dimse.StatusCode(0x3456)},
		nil})
}

func TestCEchoRq(t *testing.T) {
	testDIMSE(t, &dimse.C_ECHO_RQ{0x1234, 1, nil})
}

func TestCEchoRsp(t *testing.T) {
	testDIMSE(t, &dimse.C_ECHO_RSP{0x1234, 1,
		dimse.Status{Status: dimse.StatusCode(0x2345)},
		nil})
}
