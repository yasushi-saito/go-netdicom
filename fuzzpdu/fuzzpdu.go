package fuzzpdu

import (
	"bytes"
	"encoding/binary"
	"flag"
	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-netdicom/dimse"
	"github.com/yasushi-saito/go-netdicom/pdu"
)

func init() {
	flag.Parse()
}

func Fuzz(data []byte) int {
	in := bytes.NewBuffer(data)
	if len(data) == 0 || data[0] <= 0xc0 {
		pdu.ReadPDU(in, 4<<20)
	} else {
		d := dicom.NewDecoder(in, int64(len(data)), binary.LittleEndian, dicom.ExplicitVR)
		dimse.ReadDIMSEMessage(d)
	}
	return 0
}
