package netdicom_test

import (
	"bytes"
	"github.com/yasushi-saito/go-netdicom"
	"io"
	"testing"
)

func TestBasic(t *testing.T) {
	e := netdicom.NewEncoder()
	e.SetType(1)
	e.EncodeByte(10)
	e.EncodeByte(11)
	e.EncodeUint16(0x123)
	e.EncodeUint32(0x234)
	e.EncodeZeros(12)
	e.EncodeString("abcde")

	encoded, err := e.Finish()
	if err != nil {
		t.Error(encoded)
	}
	d := netdicom.NewDecoder(bytes.NewBuffer(encoded))
	if d.Type != 1 {
		t.Errorf("Header %v", d.Type)
	}
	if v := d.DecodeByte(); v != 10 {
		t.Errorf("DecodeByte %v", v)
	}
	if v := d.DecodeByte(); v != 11 {
		t.Errorf("DecodeByte %v", v)
	}
	if v := d.DecodeUint16(); v != 0x123 {
		t.Errorf("DecodeUint16 %v", v)
	}
	if v := d.DecodeUint32(); v != 0x234 {
		t.Errorf("DecodeUint32 %v", v)
	}
	d.Skip(12)
	if v := d.DecodeString(5); v != "abcde" {
		t.Errorf("DecodeString %v", v)
	}
	if d.Available() != 0 {
		t.Errorf("Available %d", d.Available())
	}
	if d.Error() != nil {
		t.Errorf("!Error %v", d.Error())
	}
	// Read past the buffer. It should flag an error
	if _ = d.DecodeByte(); d.Error() == nil {
		t.Errorf("Error %v %v", d.Error())
	}
}

func TestLimit(t *testing.T) {
	e := netdicom.NewEncoder()
	e.SetType(1)
	e.EncodeByte(10)
	e.EncodeByte(11)
	e.EncodeByte(12)

	encoded, err := e.Finish()
	if err != nil {
		t.Error(encoded)
	}

	// Allow reading the first two bytes
	d := netdicom.NewDecoder(bytes.NewBuffer(encoded))
	if d.Available() != 3 {
		t.Errorf("Available %d", d.Available())
	}
	d.PushLimit(2)
	if d.Available() != 2 {
		t.Errorf("Available %d", d.Available())
	}
	v0, v1 := d.DecodeByte(), d.DecodeByte()
	if d.Available() != 0 {
		t.Errorf("Available %d", d.Available())
	}
	_ = d.DecodeByte()
	if v0 != 10 || v1 != 11 || d.Error() != io.EOF {
		t.Error("Limit: %v %v %v", v0, v1, d.Error())
	}

}
