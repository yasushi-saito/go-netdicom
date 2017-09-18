
// Auto-generated from generate_dimse_messages.py. DO NOT EDIT.
package dimse
import (
	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-dicom/dicomio"
        "fmt"
)

type C_STORE_RQ struct  {
	AffectedSOPClassUID string
	MessageID uint16
	Priority uint16
	CommandDataSetType uint16
	AffectedSOPInstanceUID string
	MoveOriginatorApplicationEntityTitle string
	MoveOriginatorMessageID uint16
	Extra []*dicom.Element
}

func (v* C_STORE_RQ) Encode(e *dicomio.Encoder) {
	encodeField(e, dicom.TagCommandField, uint16(1))
	encodeField(e, dicom.TagAffectedSOPClassUID, v.AffectedSOPClassUID)
	encodeField(e, dicom.TagMessageID, v.MessageID)
	encodeField(e, dicom.TagPriority, v.Priority)
	encodeField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	encodeField(e, dicom.TagAffectedSOPInstanceUID, v.AffectedSOPInstanceUID)
	if v.MoveOriginatorApplicationEntityTitle != "" {
		encodeField(e, dicom.TagMoveOriginatorApplicationEntityTitle, v.MoveOriginatorApplicationEntityTitle)
	}
	if v.MoveOriginatorMessageID != 0 {
		encodeField(e, dicom.TagMoveOriginatorMessageID, v.MoveOriginatorMessageID)
	}
	for _, elem := range v.Extra {
		dicom.EncodeDataElement(e, elem)
	}
}

func (v* C_STORE_RQ) HasData() bool {
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v* C_STORE_RQ) String() string {
	return fmt.Sprintf("C_STORE_RQ{AffectedSOPClassUID:%v MessageID:%v Priority:%v CommandDataSetType:%v AffectedSOPInstanceUID:%v MoveOriginatorApplicationEntityTitle:%v MoveOriginatorMessageID:%v", v.AffectedSOPClassUID, v.MessageID, v.Priority, v.CommandDataSetType, v.AffectedSOPInstanceUID, v.MoveOriginatorApplicationEntityTitle, v.MoveOriginatorMessageID)
}

func decodeC_STORE_RQ(d *dimseDecoder) *C_STORE_RQ {
	v := &C_STORE_RQ{}
	v.AffectedSOPClassUID = d.getString(dicom.TagAffectedSOPClassUID, RequiredElement)
	v.MessageID = d.getUInt16(dicom.TagMessageID, RequiredElement)
	v.Priority = d.getUInt16(dicom.TagPriority, RequiredElement)
	v.CommandDataSetType = d.getUInt16(dicom.TagCommandDataSetType, RequiredElement)
	v.AffectedSOPInstanceUID = d.getString(dicom.TagAffectedSOPInstanceUID, RequiredElement)
	v.MoveOriginatorApplicationEntityTitle = d.getString(dicom.TagMoveOriginatorApplicationEntityTitle, OptionalElement)
	v.MoveOriginatorMessageID = d.getUInt16(dicom.TagMoveOriginatorMessageID, OptionalElement)
	v.Extra = d.unparsedElements()
	return v
}
type C_STORE_RSP struct  {
	AffectedSOPClassUID string
	MessageIDBeingRespondedTo uint16
	CommandDataSetType uint16
	AffectedSOPInstanceUID string
	Status Status
	Extra []*dicom.Element
}

func (v* C_STORE_RSP) Encode(e *dicomio.Encoder) {
	encodeField(e, dicom.TagCommandField, uint16(32769))
	encodeField(e, dicom.TagAffectedSOPClassUID, v.AffectedSOPClassUID)
	encodeField(e, dicom.TagMessageIDBeingRespondedTo, v.MessageIDBeingRespondedTo)
	encodeField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	encodeField(e, dicom.TagAffectedSOPInstanceUID, v.AffectedSOPInstanceUID)
	encodeStatus(e, v.Status)
	for _, elem := range v.Extra {
		dicom.EncodeDataElement(e, elem)
	}
}

func (v* C_STORE_RSP) HasData() bool {
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v* C_STORE_RSP) String() string {
	return fmt.Sprintf("C_STORE_RSP{AffectedSOPClassUID:%v MessageIDBeingRespondedTo:%v CommandDataSetType:%v AffectedSOPInstanceUID:%v Status:%v", v.AffectedSOPClassUID, v.MessageIDBeingRespondedTo, v.CommandDataSetType, v.AffectedSOPInstanceUID, v.Status)
}

func decodeC_STORE_RSP(d *dimseDecoder) *C_STORE_RSP {
	v := &C_STORE_RSP{}
	v.AffectedSOPClassUID = d.getString(dicom.TagAffectedSOPClassUID, RequiredElement)
	v.MessageIDBeingRespondedTo = d.getUInt16(dicom.TagMessageIDBeingRespondedTo, RequiredElement)
	v.CommandDataSetType = d.getUInt16(dicom.TagCommandDataSetType, RequiredElement)
	v.AffectedSOPInstanceUID = d.getString(dicom.TagAffectedSOPInstanceUID, RequiredElement)
	v.Status = d.getStatus()
	v.Extra = d.unparsedElements()
	return v
}
type C_FIND_RQ struct  {
	AffectedSOPClassUID string
	MessageID uint16
	Priority uint16
	CommandDataSetType uint16
	Extra []*dicom.Element
}

func (v* C_FIND_RQ) Encode(e *dicomio.Encoder) {
	encodeField(e, dicom.TagCommandField, uint16(32))
	encodeField(e, dicom.TagAffectedSOPClassUID, v.AffectedSOPClassUID)
	encodeField(e, dicom.TagMessageID, v.MessageID)
	encodeField(e, dicom.TagPriority, v.Priority)
	encodeField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	for _, elem := range v.Extra {
		dicom.EncodeDataElement(e, elem)
	}
}

func (v* C_FIND_RQ) HasData() bool {
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v* C_FIND_RQ) String() string {
	return fmt.Sprintf("C_FIND_RQ{AffectedSOPClassUID:%v MessageID:%v Priority:%v CommandDataSetType:%v", v.AffectedSOPClassUID, v.MessageID, v.Priority, v.CommandDataSetType)
}

func decodeC_FIND_RQ(d *dimseDecoder) *C_FIND_RQ {
	v := &C_FIND_RQ{}
	v.AffectedSOPClassUID = d.getString(dicom.TagAffectedSOPClassUID, RequiredElement)
	v.MessageID = d.getUInt16(dicom.TagMessageID, RequiredElement)
	v.Priority = d.getUInt16(dicom.TagPriority, RequiredElement)
	v.CommandDataSetType = d.getUInt16(dicom.TagCommandDataSetType, RequiredElement)
	v.Extra = d.unparsedElements()
	return v
}
type C_FIND_RSP struct  {
	AffectedSOPClassUID string
	MessageIDBeingRespondedTo uint16
	CommandDataSetType uint16
	Status Status
	Extra []*dicom.Element
}

func (v* C_FIND_RSP) Encode(e *dicomio.Encoder) {
	encodeField(e, dicom.TagCommandField, uint16(32800))
	encodeField(e, dicom.TagAffectedSOPClassUID, v.AffectedSOPClassUID)
	encodeField(e, dicom.TagMessageIDBeingRespondedTo, v.MessageIDBeingRespondedTo)
	encodeField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	encodeStatus(e, v.Status)
	for _, elem := range v.Extra {
		dicom.EncodeDataElement(e, elem)
	}
}

func (v* C_FIND_RSP) HasData() bool {
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v* C_FIND_RSP) String() string {
	return fmt.Sprintf("C_FIND_RSP{AffectedSOPClassUID:%v MessageIDBeingRespondedTo:%v CommandDataSetType:%v Status:%v", v.AffectedSOPClassUID, v.MessageIDBeingRespondedTo, v.CommandDataSetType, v.Status)
}

func decodeC_FIND_RSP(d *dimseDecoder) *C_FIND_RSP {
	v := &C_FIND_RSP{}
	v.AffectedSOPClassUID = d.getString(dicom.TagAffectedSOPClassUID, RequiredElement)
	v.MessageIDBeingRespondedTo = d.getUInt16(dicom.TagMessageIDBeingRespondedTo, RequiredElement)
	v.CommandDataSetType = d.getUInt16(dicom.TagCommandDataSetType, RequiredElement)
	v.Status = d.getStatus()
	v.Extra = d.unparsedElements()
	return v
}
type C_ECHO_RQ struct  {
	MessageID uint16
	CommandDataSetType uint16
	Extra []*dicom.Element
}

func (v* C_ECHO_RQ) Encode(e *dicomio.Encoder) {
	encodeField(e, dicom.TagCommandField, uint16(48))
	encodeField(e, dicom.TagMessageID, v.MessageID)
	encodeField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	for _, elem := range v.Extra {
		dicom.EncodeDataElement(e, elem)
	}
}

func (v* C_ECHO_RQ) HasData() bool {
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v* C_ECHO_RQ) String() string {
	return fmt.Sprintf("C_ECHO_RQ{MessageID:%v CommandDataSetType:%v", v.MessageID, v.CommandDataSetType)
}

func decodeC_ECHO_RQ(d *dimseDecoder) *C_ECHO_RQ {
	v := &C_ECHO_RQ{}
	v.MessageID = d.getUInt16(dicom.TagMessageID, RequiredElement)
	v.CommandDataSetType = d.getUInt16(dicom.TagCommandDataSetType, RequiredElement)
	v.Extra = d.unparsedElements()
	return v
}
type C_ECHO_RSP struct  {
	MessageIDBeingRespondedTo uint16
	CommandDataSetType uint16
	Status Status
	Extra []*dicom.Element
}

func (v* C_ECHO_RSP) Encode(e *dicomio.Encoder) {
	encodeField(e, dicom.TagCommandField, uint16(32816))
	encodeField(e, dicom.TagMessageIDBeingRespondedTo, v.MessageIDBeingRespondedTo)
	encodeField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	encodeStatus(e, v.Status)
	for _, elem := range v.Extra {
		dicom.EncodeDataElement(e, elem)
	}
}

func (v* C_ECHO_RSP) HasData() bool {
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v* C_ECHO_RSP) String() string {
	return fmt.Sprintf("C_ECHO_RSP{MessageIDBeingRespondedTo:%v CommandDataSetType:%v Status:%v", v.MessageIDBeingRespondedTo, v.CommandDataSetType, v.Status)
}

func decodeC_ECHO_RSP(d *dimseDecoder) *C_ECHO_RSP {
	v := &C_ECHO_RSP{}
	v.MessageIDBeingRespondedTo = d.getUInt16(dicom.TagMessageIDBeingRespondedTo, RequiredElement)
	v.CommandDataSetType = d.getUInt16(dicom.TagCommandDataSetType, RequiredElement)
	v.Status = d.getStatus()
	v.Extra = d.unparsedElements()
	return v
}
func decodeMessageForType(d* dimseDecoder, commandField uint16) Message {
	switch commandField {
	case 0x1:
		return decodeC_STORE_RQ(d)
	case 0x8001:
		return decodeC_STORE_RSP(d)
	case 0x20:
		return decodeC_FIND_RQ(d)
	case 0x8020:
		return decodeC_FIND_RSP(d)
	case 0x30:
		return decodeC_ECHO_RQ(d)
	case 0x8030:
		return decodeC_ECHO_RSP(d)
	default:
		d.setError(fmt.Errorf("Unknown DIMSE command 0x%x", commandField))
		return nil
	}
}
