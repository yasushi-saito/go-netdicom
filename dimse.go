package netdicom

// Implements message types defined in P3.7.
//
// http://dicom.nema.org/medical/dicom/current/output/pdf/part07.pdf

import (
	"encoding/binary"
	"fmt"
	"github.com/golang/glog"
	"github.com/yasushi-saito/go-dicom"
)

// Common interface for all C-XXX message types.
type DIMSEMessage interface {
	Encode(*dicom.Encoder)
	HasData() bool
	String() string
}

func findElementWithTag(elems []*dicom.DicomElement, tag dicom.Tag) (*dicom.DicomElement, error) {
	for _, elem := range elems {
		if elem.Tag == tag {
			glog.V(1).Infof("Return %v for %s", elem, tag.String())
			return elem, nil
		}
	}
	return nil, fmt.Errorf("Element %s not found during DIMSE decoding", dicom.TagString(tag))
}

func getStringFromElements(elems []*dicom.DicomElement, tag dicom.Tag, errp *error) string {
	e, err := findElementWithTag(elems, tag)
	if err != nil {
		*errp = err
		return ""
	}
	v, err := e.GetString()
	if err != nil {
		*errp = err
		return ""
	}
	return v
}

func getUInt32FromElements(elems []*dicom.DicomElement, tag dicom.Tag, errp *error) uint32 {
	e, err := findElementWithTag(elems, tag)
	if err != nil {
		*errp = err
		return 0
	}
	v, err := e.GetUInt32()
	if err != nil {
		*errp = err
		return 0
	}
	return v
}

func getUInt16FromElements(elems []*dicom.DicomElement, tag dicom.Tag, errp *error) uint16 {
	e, err := findElementWithTag(elems, tag)
	if err != nil {
		*errp = err
		return 0
	}
	v, err := e.GetUInt16()
	if err != nil {
		*errp = err
		return 0
	}
	return v
}

func encodeDataElementWithSingleValue(e *dicom.Encoder, tag dicom.Tag, v interface{}) {
	elem := dicom.DicomElement{
		Tag:   tag,
		Vr:    "", // autodetect
		Vl:    1,
		Value: []interface{}{v},
	}
	dicom.EncodeDataElement(e, &elem)
}

// P3.7 9.3.1.1
type C_STORE_RQ struct {
	AffectedSOPClassUID                  string
	MessageID                            uint16
	Priority                             uint16
	CommandDataSetType                   uint16
	AffectedSOPInstanceUID               string
	MoveOriginatorApplicationEntityTitle string
	MoveOriginatorMessageID              uint16
}

func (v *C_STORE_RQ) HasData() bool {
	doassert(v.CommandDataSetType != CommandDataSetTypeNull) // TODO(saito)
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v *C_STORE_RQ) Encode(e *dicom.Encoder) {
	encodeDataElementWithSingleValue(e, dicom.TagCommandField, uint16(1))
	encodeDataElementWithSingleValue(e, dicom.TagAffectedSOPClassUID, v.AffectedSOPClassUID)
	encodeDataElementWithSingleValue(e, dicom.TagMessageID, v.MessageID)
	encodeDataElementWithSingleValue(e, dicom.TagPriority, v.Priority)
	encodeDataElementWithSingleValue(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	encodeDataElementWithSingleValue(e, dicom.TagAffectedSOPInstanceUID, v.AffectedSOPInstanceUID)
	if v.MoveOriginatorApplicationEntityTitle != "" {
		encodeDataElementWithSingleValue(e, dicom.Tag{0, 1030}, v.MoveOriginatorApplicationEntityTitle)
	}
	if v.MoveOriginatorMessageID != 0 {
		encodeDataElementWithSingleValue(e, dicom.Tag{0, 1031}, v.MoveOriginatorMessageID)
	}
}

func decodeC_STORE_RQ(elems []*dicom.DicomElement) (*C_STORE_RQ, error) {
	v := C_STORE_RQ{}
	var err error
	v.AffectedSOPClassUID = getStringFromElements(elems, dicom.TagAffectedSOPClassUID, &err)
	v.MessageID = getUInt16FromElements(elems, dicom.TagMessageID, &err)
	v.Priority = getUInt16FromElements(elems, dicom.TagPriority, &err)
	v.CommandDataSetType = getUInt16FromElements(elems, dicom.TagCommandDataSetType, &err)
	v.AffectedSOPInstanceUID = getStringFromElements(elems, dicom.TagAffectedSOPInstanceUID, &err)

	var err2 error
	v.MoveOriginatorApplicationEntityTitle = getStringFromElements(elems, dicom.TagMoveOriginatorApplicationEntityTitle, &err2)
	v.MoveOriginatorMessageID = getUInt16FromElements(elems, dicom.TagMoveOriginatorMessageID, &err2)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (v *C_STORE_RQ) String() string {
	return fmt.Sprintf("cstorerq{sopclass:%v messageid:%v pri: %v cmddatasettype: %v sopinstance: %v m0:%v m1:%v}",
		v.AffectedSOPClassUID, v.MessageID, v.Priority, v.CommandDataSetType, v.AffectedSOPInstanceUID,
		v.MoveOriginatorApplicationEntityTitle, v.MoveOriginatorMessageID)
}

const CommandDataSetTypeNull uint16 = 0x101

// P3.7 9.3.1.2
type C_STORE_RSP struct {
	AffectedSOPClassUID       string
	MessageIDBeingRespondedTo uint16
	// CommandDataSetType shall always be 0x0101; RSP has no dataset.
	CommandDataSetType     uint16
	AffectedSOPInstanceUID string
	Status                 uint16
}

// C_STORE_RSP status codes.
// P3.4 GG4-1
const (
	CStoreStatusOutOfResources              uint16 = 0xa700
	CStoreStatusDataSetDoesNotMatchSOPClass uint16 = 0xa900
	CStoreStatusCannotUnderstand            uint16 = 0xc000
)

// P3.7 C
func decodeC_STORE_RSP(elems []*dicom.DicomElement) (*C_STORE_RSP, error) {
	v := &C_STORE_RSP{}
	var err error
	v.AffectedSOPClassUID = getStringFromElements(elems, dicom.TagAffectedSOPClassUID, &err)
	v.MessageIDBeingRespondedTo = getUInt16FromElements(elems, dicom.TagMessageIDBeingRespondedTo, &err)
	v.Status = getUInt16FromElements(elems, dicom.TagStatus, &err)
	v.CommandDataSetType = getUInt16FromElements(elems, dicom.TagCommandDataSetType, &err)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (v *C_STORE_RSP) Encode(e *dicom.Encoder) {
	doassert(v.CommandDataSetType == 0x101)
	encodeDataElementWithSingleValue(e, dicom.TagCommandField, uint16(0x8001))
	encodeDataElementWithSingleValue(e, dicom.TagAffectedSOPClassUID, v.AffectedSOPClassUID)
	encodeDataElementWithSingleValue(e, dicom.TagMessageIDBeingRespondedTo, v.MessageIDBeingRespondedTo)
	encodeDataElementWithSingleValue(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	encodeDataElementWithSingleValue(e, dicom.TagAffectedSOPInstanceUID, v.AffectedSOPInstanceUID)
	encodeDataElementWithSingleValue(e, dicom.TagStatus, v.Status)
}

func (v *C_STORE_RSP) HasData() bool {
	doassert(v.CommandDataSetType == CommandDataSetTypeNull) // TODO(saito)
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v *C_STORE_RSP) String() string {
	return fmt.Sprintf("cstorersp{sopclass:%v messageid:%v cmddatasettype: %v sopinstance: %v status: 0x%v}",
		v.AffectedSOPClassUID, v.MessageIDBeingRespondedTo, v.CommandDataSetType, v.AffectedSOPInstanceUID,
		v.Status)
}

// P3.7 9.3.5
type C_ECHO_RQ struct {
	MessageID          uint16
	CommandDataSetType uint16
}

func (v *C_ECHO_RQ) Encode(e *dicom.Encoder) {
	encodeDataElementWithSingleValue(e, dicom.TagCommandField, uint16(0x30))
	encodeDataElementWithSingleValue(e, dicom.TagMessageID, v.MessageID)
	encodeDataElementWithSingleValue(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
}

func decodeC_ECHO_RQ(elems []*dicom.DicomElement) (*C_ECHO_RQ, error) {
	v := C_ECHO_RQ{}
	var err error
	v.MessageID = getUInt16FromElements(elems, dicom.TagMessageID, &err)
	v.CommandDataSetType = getUInt16FromElements(elems, dicom.TagCommandDataSetType, &err)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (v *C_ECHO_RQ) String() string {
	return fmt.Sprintf("echorsp{messageid:%v cmddatasettype: %v}",
		v.MessageID, v.CommandDataSetType)
}

type C_ECHO_RSP struct {
	MessageIDBeingRespondedTo uint16
	CommandDataSetType        uint16
	Status                    uint16
}

func (v *C_ECHO_RSP) HasData() bool {
	doassert(v.CommandDataSetType == CommandDataSetTypeNull) // TODO(saito)
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v *C_ECHO_RSP) String() string {
	return fmt.Sprintf("echorsp{messageid:%v cmddatasettype: %v status: %v}", v.MessageIDBeingRespondedTo, v.CommandDataSetType, v.Status)
}

func (v *C_ECHO_RSP) Encode(e *dicom.Encoder) {
	encodeDataElementWithSingleValue(e, dicom.TagCommandField, uint16(0x8030))
	encodeDataElementWithSingleValue(e, dicom.TagMessageIDBeingRespondedTo, v.MessageIDBeingRespondedTo)
	encodeDataElementWithSingleValue(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	encodeDataElementWithSingleValue(e, dicom.TagStatus, v.Status)
}

func decodeC_ECHO_RSP(elems []*dicom.DicomElement) (*C_ECHO_RSP, error) {
	v := C_ECHO_RSP{}
	var err error
	v.MessageIDBeingRespondedTo = getUInt16FromElements(elems, dicom.TagMessageIDBeingRespondedTo, &err)
	v.CommandDataSetType = getUInt16FromElements(elems, dicom.TagCommandDataSetType, &err)
	v.Status = getUInt16FromElements(elems, dicom.TagStatus, &err)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (v *C_ECHO_RQ) HasData() bool {
	doassert(v.CommandDataSetType == CommandDataSetTypeNull) // TODO(saito)
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func ReadDIMSEMessage(d *dicom.Decoder) DIMSEMessage {
	var elems []*dicom.DicomElement
	// Note: DIMSE elements are always implicit LE.
	//
	// TODO(saito) make sure that's the case. Where the ref?
	d.PushTransferSyntax(binary.LittleEndian, dicom.ImplicitVR)
	defer d.PopTransferSyntax()
	for d.Len() > 0 {
		elem := dicom.ReadDataElement(d)
		if d.Error() != nil {
			break
		}
		elems = append(elems, elem)
	}
	var err error
	commandField := getUInt16FromElements(elems, dicom.TagCommandField, &err)
	if err != nil {
		d.SetError(err)
		return nil
	}
	switch commandField {
	case 1:
		v, err := decodeC_STORE_RQ(elems)
		if err != nil {
			d.SetError(err)
		}
		return v
	case 0x8001:
		v, err := decodeC_STORE_RSP(elems)
		if err != nil {
			d.SetError(err)
		}
		return v
	case 0x30:
		v, err := decodeC_ECHO_RQ(elems)
		if err != nil {
			d.SetError(err)
		}
		return v
	case 0x8030:
		v, err := decodeC_ECHO_RSP(elems)
		if err != nil {
			d.SetError(err)
		}
		return v
	}

	d.SetError(fmt.Errorf("Unknown DIMSE command 0x%x", commandField))
	return nil
}

// func EncodeDIMSEMessage(v DIMSEMessage) ([]byte, error) {
func EncodeDIMSEMessage(e *dicom.Encoder, v DIMSEMessage) {
	// DIMSE messages are always encoded Implicit+LE. See P3.7 6.3.1.
	subEncoder := dicom.NewEncoder(binary.LittleEndian, dicom.ImplicitVR)
	v.Encode(subEncoder)
	bytes, err := subEncoder.Finish()
	if err != nil {
		e.SetError(err)
		return
	}
	e.PushTransferSyntax(binary.LittleEndian, dicom.ImplicitVR)
	defer e.PopTransferSyntax()
	encodeDataElementWithSingleValue(e, dicom.TagCommandGroupLength, uint32(len(bytes)))
	e.WriteBytes(bytes)
}

type dimseCommandAssembler struct {
	contextID      byte
	commandBytes   []byte
	command        DIMSEMessage
	dataBytes      []byte
	readAllCommand bool

	readAllData bool
}

// Add a P_DATA_TF fragment. If the final fragment is received, returns <SOPUID,
// TransferSyntaxUID, payload, nil>.  If it expects more fragments, it retutrns
// <"", "", nil, nil>.  On error, the final return value is non-nil.
func addPDataTF(a *dimseCommandAssembler, pdu *P_DATA_TF, contextManager *contextManager) (string, string, DIMSEMessage, []byte, error) {
	for _, item := range pdu.Items {
		if a.contextID == 0 {
			a.contextID = item.ContextID
		} else if a.contextID != item.ContextID {
			return "", "", nil, nil, fmt.Errorf("Mixed context: %d %d", a.contextID, item.ContextID)
		}
		if item.Command {
			a.commandBytes = append(a.commandBytes, item.Value...)
			if item.Last {
				if a.readAllCommand {
					return "", "", nil, nil, fmt.Errorf("P_DATA_TF: found >1 command chunks with the Last bit set")
				}
				a.readAllCommand = true
			}
		} else {
			a.dataBytes = append(a.dataBytes, item.Value...)
			if item.Last {
				if a.readAllData {
					return "", "", nil, nil, fmt.Errorf("P_DATA_TF: found >1 data chunks with the Last bit set")
				}
				a.readAllData = true
			}
		}
	}
	if !a.readAllCommand {
		return "", "", nil, nil, nil
	}
	if a.command == nil {
		d := dicom.NewBytesDecoder(a.commandBytes, nil, dicom.UnknownVR)
		a.command = ReadDIMSEMessage(d)
		if err := d.Finish(); err != nil {
			return "", "", nil, nil, err
		}
	}
	doassert(a.command != nil)
	if a.command.HasData() && !a.readAllData {
		return "", "", nil, nil, nil
	}
	context, err := contextManager.lookupByContextID(a.contextID)
	if err != nil {
		return "", "", nil, nil, err
	}
	command := a.command
	dataBytes := a.dataBytes
	glog.V(1).Infof("Read all data for syntax %s, command [%v], data %d bytes, err%v",
		dicom.UIDString(context.abstractSyntaxUID),
		command.String(), len(a.dataBytes), err)
	*a = dimseCommandAssembler{}
	return context.abstractSyntaxUID, context.transferSyntaxUID, command, dataBytes, nil
	// TODO(saito) Verify that there's no unread items after the last command&data.
}
