package pdufuzz

import (
	"bytes"
	"github.com/yasushi-saito/go-netdicom"
)

func Fuzz(data []byte) int {
	in := bytes.NewBuffer(data)
	netdicom.ReadPDU(in)
	return 0
}
