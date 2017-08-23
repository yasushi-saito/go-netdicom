package netdicom

// Implements message types defined in P3.7.
//
// http://dicom.nema.org/medical/dicom/current/output/pdf/part07.pdf

import (
	"encoding/binary"
	"fmt"
	"github.com/yasushi-saito/go-dicom"
	"io"
	"log"
)

type DIMSEMessage interface {
}

func findElementWithTag(elems []*dicom.DicomElement, group, element uint16) (*dicom.DicomElement, error) {
	for _, elem := range elems {
		if elem.Tag.Group == group && elem.Tag.Element == element {
			log.Printf("Return %v for %d,%d", elem, group, element)
			return elem, nil
		}
	}

	return nil, fmt.Errorf("Element (0x%04x,0x%04x) not found during DIMSE decoding", group, element)
}

func getStringFromElements(elems []*dicom.DicomElement, group, element uint16) (string, error) {
	e, err := findElementWithTag(elems, group, element)
	if err != nil {
		return "", err
	}
	return dicom.GetString(*e)
}

func getUInt32FromElements(elems []*dicom.DicomElement, group, element uint16) (uint32, error) {
	e, err := findElementWithTag(elems, group, element)
	if err != nil {
		return 0, err
	}
	return dicom.GetUInt32(*e)
}

func getUInt16FromElements(elems []*dicom.DicomElement, group, element uint16) (uint16, error) {
	e, err := findElementWithTag(elems, group, element)
	if err != nil {
		return 0, err
	}
	return dicom.GetUInt16(*e)
}

// Fields common to all DIMSE messages.
type DIMSEMessageHeader struct {
	CommandGroupLength  uint32 // (0000,0000)
	AffectedSOPClassUID string // (0000,0002)
	CommandField        uint16 // (0000,0100)
}

func decodeDIMSEMessageHeader(elems []*dicom.DicomElement) (DIMSEMessageHeader, error) {
	var h DIMSEMessageHeader
	var err error
	h.CommandGroupLength, err = getUInt32FromElements(elems, 0, 0)
	if err != nil {
		return h, err
	}
	h.AffectedSOPClassUID, err = getStringFromElements(elems, 0, 2)
	if err != nil {
		return h, err
	}
	h.CommandField, err = getUInt16FromElements(elems, 0, 0x100)
	if err != nil {
		return h, err
	}
	return h, nil
}

func encodeDataElementWithSingleValue(e *dicom.Encoder, tag dicom.Tag, v interface{}) {
	values := []interface{}{v}
	dicom.EncodeDataElement(e, tag, values)
}

func encodeDIMSEMessageHeader(e *dicom.Encoder, v DIMSEMessageHeader) {
	encodeDataElementWithSingleValue(e, dicom.Tag{0, 0}, v.CommandGroupLength)
	encodeDataElementWithSingleValue(e, dicom.Tag{0, 2}, v.AffectedSOPClassUID)
	encodeDataElementWithSingleValue(e, dicom.Tag{0, 0x100}, v.CommandField)
}

// P3.7 9.3.1.1
type C_STORE_RQ struct {
	Header                               DIMSEMessageHeader
	MessageID                            uint16 // (0000,0110)
	Priority                             uint16 // (0000,0700)
	CommandDataSetType                   uint16 // (0000,0800)
	AffectedSOPInstanceUID               string // (0000,1000)
	MoveOriginatorApplicationEntityTitle string // (0000,1030)
	MoveOriginatorMessageID              uint16 // (0000,1031)
}

func (v *C_STORE_RQ) Encode(e *dicom.Encoder) {
	encodeDIMSEMessageHeader(e, v.Header)
	encodeDataElementWithSingleValue(e, dicom.Tag{0, 0x110}, v.MessageID)
	encodeDataElementWithSingleValue(e, dicom.Tag{0, 0x700}, v.Priority)
	encodeDataElementWithSingleValue(e, dicom.Tag{0, 0x800}, v.CommandDataSetType)
	encodeDataElementWithSingleValue(e, dicom.Tag{0, 0x1000}, v.AffectedSOPInstanceUID)
	if v.MoveOriginatorApplicationEntityTitle!="" {
		encodeDataElementWithSingleValue(e, dicom.Tag{0, 1030}, v.MoveOriginatorApplicationEntityTitle)
	}
	if v.MoveOriginatorMessageID != 0 {
		encodeDataElementWithSingleValue(e, dicom.Tag{0, 1031}, v.MoveOriginatorMessageID)
	}
}

func decodeC_STORE_RQ(header DIMSEMessageHeader, elems []*dicom.DicomElement) (*C_STORE_RQ, error) {
	v := C_STORE_RQ{Header: header}
	var err error
	v.MessageID, err = getUInt16FromElements(elems, 0, 0x110)
	if err != nil {
		return nil, err
	}
	v.Priority, err = getUInt16FromElements(elems, 0, 0x700)
	if err != nil {
		return nil, err
	}
	v.CommandDataSetType, err = getUInt16FromElements(elems, 0, 0x800)
	if err != nil {
		return nil, err
	}
	v.AffectedSOPInstanceUID, err = getStringFromElements(elems, 0, 0x1000)
	if err != nil {
		return nil, err
	}
	v.MoveOriginatorApplicationEntityTitle, _ = getStringFromElements(elems, 0, 0x1030)
	v.MoveOriginatorMessageID, _ = getUInt16FromElements(elems, 0, 0x1031)
	return &v, nil
}

// P3.7 9.3.1.2
type C_STORE_RSP struct {
	Header                    DIMSEMessageHeader
	MessageIDBeingRespondedTo uint16 // (0000,0120)
	CommandDataSetType        uint16 // (0000, 0800)
	Status                    uint16 // (0000,0900)
	AffectedSOPInstanceUID    string // (0000,1000)
}

func decodeC_STORE_RSP(header DIMSEMessageHeader, elems []*dicom.DicomElement) (*C_STORE_RSP, error) {
	v := C_STORE_RSP{Header: header}
	var err error
	v.MessageIDBeingRespondedTo, err = getUInt16FromElements(elems, 0, 0x120)
	if err != nil {
		return nil, err
	}
	v.CommandDataSetType, err = getUInt16FromElements(elems, 0, 0x800)
	if err != nil {
		return nil, err
	}
	v.Status, err = getUInt16FromElements(elems, 0, 0x900)
	if err != nil {
		return nil, err
	}
	v.AffectedSOPInstanceUID, err = getStringFromElements(elems, 0, 0x1000)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (v *C_STORE_RSP) Encode(e *dicom.Encoder) {
	encodeDIMSEMessageHeader(e, v.Header)
	encodeDataElementWithSingleValue(e, dicom.Tag{0, 0x120}, v.MessageIDBeingRespondedTo)
	encodeDataElementWithSingleValue(e, dicom.Tag{0, 0x800}, v.CommandDataSetType)
	encodeDataElementWithSingleValue(e, dicom.Tag{0, 0x900}, v.Status)
	encodeDataElementWithSingleValue(e, dicom.Tag{0, 0x1000}, v.AffectedSOPInstanceUID)
}

func DecodeDIMSEMessage(io io.Reader, limit int64) (DIMSEMessage, error) {
	var elems []*dicom.DicomElement
	d := dicom.NewDecoder(io, limit, binary.LittleEndian, true /*implicit*/) // TODO(saito) pass decoding params??
	for d.Len() > 0 && d.Error() == nil {
		elem := dicom.ReadDataElement(d)
		elems = append(elems, elem)
	}
	if err := d.Finish(); err != nil {
		return nil, err
	}
	header, err := decodeDIMSEMessageHeader(elems)
	if err != nil {
		return nil, err
	}
	switch header.CommandField {
	case 1:
		return decodeC_STORE_RQ(header, elems)
	case 0x8001:
		return decodeC_STORE_RSP(header, elems)
	}
	panic(fmt.Sprintf("Unknown DIMSE command 0x%x", header.CommandField))
}
