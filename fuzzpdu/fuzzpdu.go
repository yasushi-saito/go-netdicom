package fuzzpdu

import (
	"bytes"
	"flag"
	"github.com/yasushi-saito/go-netdicom"
)

func init() {
	flag.Parse()
}

func Fuzz(data []byte) int {
	in := bytes.NewBuffer(data)
	if len(data) == 0 || data[0] <= 0xc0 {
		netdicom.ReadPDU(in, 4<<20)
	} else {
		netdicom.ReadDIMSEMessage(in, int64(len(data)))
	}
	return 0
}
