package fuzzpdu

import (
	"bytes"
	"flag"
	"github.com/yasushi-saito/go-netdicom/pdu"
	"github.com/yasushi-saito/go-netdicom/dimse"
)

func init() {
	flag.Parse()
}

func Fuzz(data []byte) int {
	in := bytes.NewBuffer(data)
	if len(data) == 0 || data[0] <= 0xc0 {
		pdu.ReadPDU(in, 4<<20)
	} else {
		dimse.ReadDIMSEMessage(in, int64(len(data)))
	}
	return 0
}
