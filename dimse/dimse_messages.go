
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
	Extra []*dicom.Element  // Unparsed elements
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
		dicom.WriteElement(e, elem)
	}
}

func (v* C_STORE_RQ) HasData() bool {
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v* C_STORE_RQ) String() string {
	return fmt.Sprintf("C_STORE_RQ{AffectedSOPClassUID:%v MessageID:%v Priority:%v CommandDataSetType:%v AffectedSOPInstanceUID:%v MoveOriginatorApplicationEntityTitle:%v MoveOriginatorMessageID:%v", v.AffectedSOPClassUID, v.MessageID, v.Priority, v.CommandDataSetType, v.AffectedSOPInstanceUID, v.MoveOriginatorApplicationEntityTitle, v.MoveOriginatorMessageID)
}

func decodeC_STORE_RQ(d *messageDecoder) *C_STORE_RQ {
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
	Extra []*dicom.Element  // Unparsed elements
}

func (v* C_STORE_RSP) Encode(e *dicomio.Encoder) {
	encodeField(e, dicom.TagCommandField, uint16(32769))
	encodeField(e, dicom.TagAffectedSOPClassUID, v.AffectedSOPClassUID)
	encodeField(e, dicom.TagMessageIDBeingRespondedTo, v.MessageIDBeingRespondedTo)
	encodeField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	encodeField(e, dicom.TagAffectedSOPInstanceUID, v.AffectedSOPInstanceUID)
	encodeStatus(e, v.Status)
	for _, elem := range v.Extra {
		dicom.WriteElement(e, elem)
	}
}

func (v* C_STORE_RSP) HasData() bool {
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v* C_STORE_RSP) String() string {
	return fmt.Sprintf("C_STORE_RSP{AffectedSOPClassUID:%v MessageIDBeingRespondedTo:%v CommandDataSetType:%v AffectedSOPInstanceUID:%v Status:%v", v.AffectedSOPClassUID, v.MessageIDBeingRespondedTo, v.CommandDataSetType, v.AffectedSOPInstanceUID, v.Status)
}

func decodeC_STORE_RSP(d *messageDecoder) *C_STORE_RSP {
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
	Extra []*dicom.Element  // Unparsed elements
}

func (v* C_FIND_RQ) Encode(e *dicomio.Encoder) {
	encodeField(e, dicom.TagCommandField, uint16(32))
	encodeField(e, dicom.TagAffectedSOPClassUID, v.AffectedSOPClassUID)
	encodeField(e, dicom.TagMessageID, v.MessageID)
	encodeField(e, dicom.TagPriority, v.Priority)
	encodeField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	for _, elem := range v.Extra {
		dicom.WriteElement(e, elem)
	}
}

func (v* C_FIND_RQ) HasData() bool {
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v* C_FIND_RQ) String() string {
	return fmt.Sprintf("C_FIND_RQ{AffectedSOPClassUID:%v MessageID:%v Priority:%v CommandDataSetType:%v", v.AffectedSOPClassUID, v.MessageID, v.Priority, v.CommandDataSetType)
}

func decodeC_FIND_RQ(d *messageDecoder) *C_FIND_RQ {
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
	Extra []*dicom.Element  // Unparsed elements
}

func (v* C_FIND_RSP) Encode(e *dicomio.Encoder) {
	encodeField(e, dicom.TagCommandField, uint16(32800))
	encodeField(e, dicom.TagAffectedSOPClassUID, v.AffectedSOPClassUID)
	encodeField(e, dicom.TagMessageIDBeingRespondedTo, v.MessageIDBeingRespondedTo)
	encodeField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	encodeStatus(e, v.Status)
	for _, elem := range v.Extra {
		dicom.WriteElement(e, elem)
	}
}

func (v* C_FIND_RSP) HasData() bool {
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v* C_FIND_RSP) String() string {
	return fmt.Sprintf("C_FIND_RSP{AffectedSOPClassUID:%v MessageIDBeingRespondedTo:%v CommandDataSetType:%v Status:%v", v.AffectedSOPClassUID, v.MessageIDBeingRespondedTo, v.CommandDataSetType, v.Status)
}

func decodeC_FIND_RSP(d *messageDecoder) *C_FIND_RSP {
	v := &C_FIND_RSP{}
	v.AffectedSOPClassUID = d.getString(dicom.TagAffectedSOPClassUID, RequiredElement)
	v.MessageIDBeingRespondedTo = d.getUInt16(dicom.TagMessageIDBeingRespondedTo, RequiredElement)
	v.CommandDataSetType = d.getUInt16(dicom.TagCommandDataSetType, RequiredElement)
	v.Status = d.getStatus()
	v.Extra = d.unparsedElements()
	return v
}
type C_GET_RQ struct  {
	AffectedSOPClassUID string
	MessageID uint16
	Priority uint16
	CommandDataSetType uint16
	Extra []*dicom.Element  // Unparsed elements
}

func (v* C_GET_RQ) Encode(e *dicomio.Encoder) {
	encodeField(e, dicom.TagCommandField, uint16(16))
	encodeField(e, dicom.TagAffectedSOPClassUID, v.AffectedSOPClassUID)
	encodeField(e, dicom.TagMessageID, v.MessageID)
	encodeField(e, dicom.TagPriority, v.Priority)
	encodeField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	for _, elem := range v.Extra {
		dicom.WriteElement(e, elem)
	}
}

func (v* C_GET_RQ) HasData() bool {
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v* C_GET_RQ) String() string {
	return fmt.Sprintf("C_GET_RQ{AffectedSOPClassUID:%v MessageID:%v Priority:%v CommandDataSetType:%v", v.AffectedSOPClassUID, v.MessageID, v.Priority, v.CommandDataSetType)
}

func decodeC_GET_RQ(d *messageDecoder) *C_GET_RQ {
	v := &C_GET_RQ{}
	v.AffectedSOPClassUID = d.getString(dicom.TagAffectedSOPClassUID, RequiredElement)
	v.MessageID = d.getUInt16(dicom.TagMessageID, RequiredElement)
	v.Priority = d.getUInt16(dicom.TagPriority, RequiredElement)
	v.CommandDataSetType = d.getUInt16(dicom.TagCommandDataSetType, RequiredElement)
	v.Extra = d.unparsedElements()
	return v
}
type C_GET_RSP struct  {
	AffectedSOPClassUID string
	MessageIDBeingRespondedTo uint16
	CommandDataSetType uint16
	NumberOfRemainingSuboperations uint16
	NumberOfCompletedSuboperations uint16
	NumberOfFailedSuboperations uint16
	NumberOfWarningSuboperations uint16
	Status Status
	Extra []*dicom.Element  // Unparsed elements
}

func (v* C_GET_RSP) Encode(e *dicomio.Encoder) {
	encodeField(e, dicom.TagCommandField, uint16(32784))
	encodeField(e, dicom.TagAffectedSOPClassUID, v.AffectedSOPClassUID)
	encodeField(e, dicom.TagMessageIDBeingRespondedTo, v.MessageIDBeingRespondedTo)
	encodeField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	if v.NumberOfRemainingSuboperations != 0 {
		encodeField(e, dicom.TagNumberOfRemainingSuboperations, v.NumberOfRemainingSuboperations)
	}
	if v.NumberOfCompletedSuboperations != 0 {
		encodeField(e, dicom.TagNumberOfCompletedSuboperations, v.NumberOfCompletedSuboperations)
	}
	if v.NumberOfFailedSuboperations != 0 {
		encodeField(e, dicom.TagNumberOfFailedSuboperations, v.NumberOfFailedSuboperations)
	}
	if v.NumberOfWarningSuboperations != 0 {
		encodeField(e, dicom.TagNumberOfWarningSuboperations, v.NumberOfWarningSuboperations)
	}
	encodeStatus(e, v.Status)
	for _, elem := range v.Extra {
		dicom.WriteElement(e, elem)
	}
}

func (v* C_GET_RSP) HasData() bool {
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v* C_GET_RSP) String() string {
	return fmt.Sprintf("C_GET_RSP{AffectedSOPClassUID:%v MessageIDBeingRespondedTo:%v CommandDataSetType:%v NumberOfRemainingSuboperations:%v NumberOfCompletedSuboperations:%v NumberOfFailedSuboperations:%v NumberOfWarningSuboperations:%v Status:%v", v.AffectedSOPClassUID, v.MessageIDBeingRespondedTo, v.CommandDataSetType, v.NumberOfRemainingSuboperations, v.NumberOfCompletedSuboperations, v.NumberOfFailedSuboperations, v.NumberOfWarningSuboperations, v.Status)
}

func decodeC_GET_RSP(d *messageDecoder) *C_GET_RSP {
	v := &C_GET_RSP{}
	v.AffectedSOPClassUID = d.getString(dicom.TagAffectedSOPClassUID, RequiredElement)
	v.MessageIDBeingRespondedTo = d.getUInt16(dicom.TagMessageIDBeingRespondedTo, RequiredElement)
	v.CommandDataSetType = d.getUInt16(dicom.TagCommandDataSetType, RequiredElement)
	v.NumberOfRemainingSuboperations = d.getUInt16(dicom.TagNumberOfRemainingSuboperations, OptionalElement)
	v.NumberOfCompletedSuboperations = d.getUInt16(dicom.TagNumberOfCompletedSuboperations, OptionalElement)
	v.NumberOfFailedSuboperations = d.getUInt16(dicom.TagNumberOfFailedSuboperations, OptionalElement)
	v.NumberOfWarningSuboperations = d.getUInt16(dicom.TagNumberOfWarningSuboperations, OptionalElement)
	v.Status = d.getStatus()
	v.Extra = d.unparsedElements()
	return v
}
type C_MOVE_RQ struct  {
	AffectedSOPClassUID string
	MessageID uint16
	Priority uint16
	MoveDestination string
	CommandDataSetType uint16
	Extra []*dicom.Element  // Unparsed elements
}

func (v* C_MOVE_RQ) Encode(e *dicomio.Encoder) {
	encodeField(e, dicom.TagCommandField, uint16(33))
	encodeField(e, dicom.TagAffectedSOPClassUID, v.AffectedSOPClassUID)
	encodeField(e, dicom.TagMessageID, v.MessageID)
	encodeField(e, dicom.TagPriority, v.Priority)
	encodeField(e, dicom.TagMoveDestination, v.MoveDestination)
	encodeField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	for _, elem := range v.Extra {
		dicom.WriteElement(e, elem)
	}
}

func (v* C_MOVE_RQ) HasData() bool {
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v* C_MOVE_RQ) String() string {
	return fmt.Sprintf("C_MOVE_RQ{AffectedSOPClassUID:%v MessageID:%v Priority:%v MoveDestination:%v CommandDataSetType:%v", v.AffectedSOPClassUID, v.MessageID, v.Priority, v.MoveDestination, v.CommandDataSetType)
}

func decodeC_MOVE_RQ(d *messageDecoder) *C_MOVE_RQ {
	v := &C_MOVE_RQ{}
	v.AffectedSOPClassUID = d.getString(dicom.TagAffectedSOPClassUID, RequiredElement)
	v.MessageID = d.getUInt16(dicom.TagMessageID, RequiredElement)
	v.Priority = d.getUInt16(dicom.TagPriority, RequiredElement)
	v.MoveDestination = d.getString(dicom.TagMoveDestination, RequiredElement)
	v.CommandDataSetType = d.getUInt16(dicom.TagCommandDataSetType, RequiredElement)
	v.Extra = d.unparsedElements()
	return v
}
type C_MOVE_RSP struct  {
	AffectedSOPClassUID string
	MessageIDBeingRespondedTo uint16
	CommandDataSetType uint16
	NumberOfRemainingSuboperations uint16
	NumberOfCompletedSuboperations uint16
	NumberOfFailedSuboperations uint16
	NumberOfWarningSuboperations uint16
	Status Status
	Extra []*dicom.Element  // Unparsed elements
}

func (v* C_MOVE_RSP) Encode(e *dicomio.Encoder) {
	encodeField(e, dicom.TagCommandField, uint16(32801))
	encodeField(e, dicom.TagAffectedSOPClassUID, v.AffectedSOPClassUID)
	encodeField(e, dicom.TagMessageIDBeingRespondedTo, v.MessageIDBeingRespondedTo)
	encodeField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	if v.NumberOfRemainingSuboperations != 0 {
		encodeField(e, dicom.TagNumberOfRemainingSuboperations, v.NumberOfRemainingSuboperations)
	}
	if v.NumberOfCompletedSuboperations != 0 {
		encodeField(e, dicom.TagNumberOfCompletedSuboperations, v.NumberOfCompletedSuboperations)
	}
	if v.NumberOfFailedSuboperations != 0 {
		encodeField(e, dicom.TagNumberOfFailedSuboperations, v.NumberOfFailedSuboperations)
	}
	if v.NumberOfWarningSuboperations != 0 {
		encodeField(e, dicom.TagNumberOfWarningSuboperations, v.NumberOfWarningSuboperations)
	}
	encodeStatus(e, v.Status)
	for _, elem := range v.Extra {
		dicom.WriteElement(e, elem)
	}
}

func (v* C_MOVE_RSP) HasData() bool {
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v* C_MOVE_RSP) String() string {
	return fmt.Sprintf("C_MOVE_RSP{AffectedSOPClassUID:%v MessageIDBeingRespondedTo:%v CommandDataSetType:%v NumberOfRemainingSuboperations:%v NumberOfCompletedSuboperations:%v NumberOfFailedSuboperations:%v NumberOfWarningSuboperations:%v Status:%v", v.AffectedSOPClassUID, v.MessageIDBeingRespondedTo, v.CommandDataSetType, v.NumberOfRemainingSuboperations, v.NumberOfCompletedSuboperations, v.NumberOfFailedSuboperations, v.NumberOfWarningSuboperations, v.Status)
}

func decodeC_MOVE_RSP(d *messageDecoder) *C_MOVE_RSP {
	v := &C_MOVE_RSP{}
	v.AffectedSOPClassUID = d.getString(dicom.TagAffectedSOPClassUID, RequiredElement)
	v.MessageIDBeingRespondedTo = d.getUInt16(dicom.TagMessageIDBeingRespondedTo, RequiredElement)
	v.CommandDataSetType = d.getUInt16(dicom.TagCommandDataSetType, RequiredElement)
	v.NumberOfRemainingSuboperations = d.getUInt16(dicom.TagNumberOfRemainingSuboperations, OptionalElement)
	v.NumberOfCompletedSuboperations = d.getUInt16(dicom.TagNumberOfCompletedSuboperations, OptionalElement)
	v.NumberOfFailedSuboperations = d.getUInt16(dicom.TagNumberOfFailedSuboperations, OptionalElement)
	v.NumberOfWarningSuboperations = d.getUInt16(dicom.TagNumberOfWarningSuboperations, OptionalElement)
	v.Status = d.getStatus()
	v.Extra = d.unparsedElements()
	return v
}
type C_ECHO_RQ struct  {
	MessageID uint16
	CommandDataSetType uint16
	Extra []*dicom.Element  // Unparsed elements
}

func (v* C_ECHO_RQ) Encode(e *dicomio.Encoder) {
	encodeField(e, dicom.TagCommandField, uint16(48))
	encodeField(e, dicom.TagMessageID, v.MessageID)
	encodeField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	for _, elem := range v.Extra {
		dicom.WriteElement(e, elem)
	}
}

func (v* C_ECHO_RQ) HasData() bool {
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v* C_ECHO_RQ) String() string {
	return fmt.Sprintf("C_ECHO_RQ{MessageID:%v CommandDataSetType:%v", v.MessageID, v.CommandDataSetType)
}

func decodeC_ECHO_RQ(d *messageDecoder) *C_ECHO_RQ {
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
	Extra []*dicom.Element  // Unparsed elements
}

func (v* C_ECHO_RSP) Encode(e *dicomio.Encoder) {
	encodeField(e, dicom.TagCommandField, uint16(32816))
	encodeField(e, dicom.TagMessageIDBeingRespondedTo, v.MessageIDBeingRespondedTo)
	encodeField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	encodeStatus(e, v.Status)
	for _, elem := range v.Extra {
		dicom.WriteElement(e, elem)
	}
}

func (v* C_ECHO_RSP) HasData() bool {
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v* C_ECHO_RSP) String() string {
	return fmt.Sprintf("C_ECHO_RSP{MessageIDBeingRespondedTo:%v CommandDataSetType:%v Status:%v", v.MessageIDBeingRespondedTo, v.CommandDataSetType, v.Status)
}

func decodeC_ECHO_RSP(d *messageDecoder) *C_ECHO_RSP {
	v := &C_ECHO_RSP{}
	v.MessageIDBeingRespondedTo = d.getUInt16(dicom.TagMessageIDBeingRespondedTo, RequiredElement)
	v.CommandDataSetType = d.getUInt16(dicom.TagCommandDataSetType, RequiredElement)
	v.Status = d.getStatus()
	v.Extra = d.unparsedElements()
	return v
}
func decodeMessageForType(d* messageDecoder, commandField uint16) Message {
	switch commandField {
	case 0x1:
		return decodeC_STORE_RQ(d)
	case 0x8001:
		return decodeC_STORE_RSP(d)
	case 0x20:
		return decodeC_FIND_RQ(d)
	case 0x8020:
		return decodeC_FIND_RSP(d)
	case 0x10:
		return decodeC_GET_RQ(d)
	case 0x8010:
		return decodeC_GET_RSP(d)
	case 0x21:
		return decodeC_MOVE_RQ(d)
	case 0x8021:
		return decodeC_MOVE_RSP(d)
	case 0x30:
		return decodeC_ECHO_RQ(d)
	case 0x8030:
		return decodeC_ECHO_RSP(d)
	default:
		d.setError(fmt.Errorf("Unknown DIMSE command 0x%x", commandField))
		return nil
	}
}
