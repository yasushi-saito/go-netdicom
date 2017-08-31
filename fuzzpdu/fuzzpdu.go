package fuzzpdu

import (
	"bytes"
	"github.com/yasushi-saito/go-netdicom"
)

func Fuzz(data []byte) int {
	in := bytes.NewBuffer(data)
	netdicom.ReadPDU(in, 4 << 20)
	return 0
}
