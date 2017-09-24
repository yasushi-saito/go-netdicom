package netdicom

import (
	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-dicom/dicomio"
)

// Parse the beginning of "bytes" as a DICOM file and extract its
// TransferSyntaxUID.
func GetTransferSyntaxUIDInBytes(bytes []byte) (string, error) {
	decoder := dicomio.NewBytesDecoder(bytes, nil, dicomio.UnknownVR)
	meta := dicom.ParseFileHeader(decoder)
	if decoder.Error() != nil {
		return "", decoder.Error()
	}
	transferSyntaxUID, err := dicom.LookupElementByTag(meta, dicom.TagTransferSyntaxUID)
	if err != nil {
		return "", err
	}
	s, err := transferSyntaxUID.GetString()
	if err != nil {
		return "", err
	}
	return s, nil
}

func doassert(cond bool, values... interface{}) {
	if !cond {
		panic(values)
	}
}
