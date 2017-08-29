package netdicom

import (
	"github.com/yasushi-saito/go-dicom"
)

// Parse the beginning of "bytes" as a DICOM file and extract its
// TransferSyntaxUID.
func GetTransferSyntaxUIDInBytes(bytes []byte) (string, error) {
	decoder := dicom.NewBytesDecoder(bytes, nil, dicom.UnknownVR)
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
