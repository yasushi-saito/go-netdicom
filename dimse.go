package netdicom

// Implements message types defined in P3.7.
//
// http://dicom.nema.org/medical/dicom/current/output/pdf/part07.pdf

import (
	"encoding/binary"
	"fmt"
	"github.com/yasushi-saito/go-dicom"
	"v.io/x/lib/vlog"
)

// Common interface for all C-XXX message types.
type DIMSEMessage interface {
	Encode(*dicom.Encoder)
	HasData() bool  // Do we expact data P_DATA_TF packets after the command packets?
	String() string // Produce human-readable description.
}

// Helper class for extracting values from a list of DicomElement.
type dimseDecoder struct {
	elems []*dicom.DicomElement
	err   error
}

type isOptionalElement int

const (
	RequiredElement isOptionalElement = iota
	OptionalElement
)

func (d *dimseDecoder) setError(err error) {
	if d.err == nil {
		d.err = err
	}
}

// Find an element with the given tag. If optional==OptionalElement, returns nil
// if not found.  If optional==RequiredElement, sets d.err and return nil if not found.
func (d *dimseDecoder) findElement(tag dicom.Tag, optional isOptionalElement) *dicom.DicomElement {
	for _, elem := range d.elems {
		if elem.Tag == tag {
			vlog.VI(1).Infof("Return %v for %s", elem, tag.String())
			return elem
		}
	}
	if optional == RequiredElement {
		d.setError(fmt.Errorf("Element %s not found during DIMSE decoding", dicom.TagString(tag)))
	}
	return nil
}

// Find an element with "tag", and extract a string value from it. Errors are reported in d.err.
func (d *dimseDecoder) getString(tag dicom.Tag, optional isOptionalElement) string {
	e := d.findElement(tag, optional)
	if e == nil {
		return ""
	}
	v, err := e.GetString()
	if err != nil {
		d.setError(err)
	}
	return v
}

// Find an element with "tag", and extract a uint32 from it. Errors are reported in d.err.
func (d *dimseDecoder) getUInt32(tag dicom.Tag, optional isOptionalElement) uint32 {
	e := d.findElement(tag, optional)
	if e == nil {
		return 0
	}
	v, err := e.GetUInt32()
	if err != nil {
		d.setError(err)
	}
	return v
}

// Find an element with "tag", and extract a uint16 from it. Errors are reported in d.err.
func (d *dimseDecoder) getUInt16(tag dicom.Tag, optional isOptionalElement) uint16 {
	e := d.findElement(tag, optional)
	if e == nil {
		return 0
	}
	v, err := e.GetUInt16()
	if err != nil {
		d.setError(err)
	}
	return v
}

// Encode a DIMSE field with the given tag, given value "v"
func encodeDIMSEField(e *dicom.Encoder, tag dicom.Tag, v interface{}) {
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
	if v.CommandDataSetType == CommandDataSetTypeNull {
		vlog.Error("Bogus C_STORE_RQ without dataset")
	}
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v *C_STORE_RQ) Encode(e *dicom.Encoder) {
	encodeDIMSEField(e, dicom.TagCommandField, uint16(1))
	encodeDIMSEField(e, dicom.TagAffectedSOPClassUID, v.AffectedSOPClassUID)
	encodeDIMSEField(e, dicom.TagMessageID, v.MessageID)
	encodeDIMSEField(e, dicom.TagPriority, v.Priority)
	encodeDIMSEField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	encodeDIMSEField(e, dicom.TagAffectedSOPInstanceUID, v.AffectedSOPInstanceUID)
	if v.MoveOriginatorApplicationEntityTitle != "" {
		encodeDIMSEField(e,
			dicom.TagMoveOriginatorApplicationEntityTitle,
			v.MoveOriginatorApplicationEntityTitle)
	}
	if v.MoveOriginatorMessageID != 0 {
		encodeDIMSEField(e, dicom.TagMoveOriginatorMessageID, v.MoveOriginatorMessageID)
	}
}

// Decode C_STORE_RQ object. Errors are reported in dd.err.
func decodeC_STORE_RQ(dd *dimseDecoder) *C_STORE_RQ {
	v := C_STORE_RQ{}
	v.AffectedSOPClassUID = dd.getString(dicom.TagAffectedSOPClassUID, RequiredElement)
	v.MessageID = dd.getUInt16(dicom.TagMessageID, RequiredElement)
	v.Priority = dd.getUInt16(dicom.TagPriority, RequiredElement)
	v.CommandDataSetType = dd.getUInt16(dicom.TagCommandDataSetType, RequiredElement)
	v.AffectedSOPInstanceUID = dd.getString(dicom.TagAffectedSOPInstanceUID, RequiredElement)
	v.MoveOriginatorApplicationEntityTitle = dd.getString(dicom.TagMoveOriginatorApplicationEntityTitle, OptionalElement)
	v.MoveOriginatorMessageID = dd.getUInt16(dicom.TagMoveOriginatorMessageID, OptionalElement)
	return &v
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

// Decode C_STORE_RSP object. Errors are reported in dd.err.
// See P3.7 C.
func decodeC_STORE_RSP(dd *dimseDecoder) *C_STORE_RSP {
	v := &C_STORE_RSP{}
	v.AffectedSOPClassUID = dd.getString(dicom.TagAffectedSOPClassUID, RequiredElement)
	v.MessageIDBeingRespondedTo = dd.getUInt16(dicom.TagMessageIDBeingRespondedTo, RequiredElement)
	v.AffectedSOPInstanceUID = dd.getString(dicom.TagAffectedSOPInstanceUID, RequiredElement)
	v.Status = dd.getUInt16(dicom.TagStatus, RequiredElement)
	v.CommandDataSetType = dd.getUInt16(dicom.TagCommandDataSetType, RequiredElement)
	return v
}

func (v *C_STORE_RSP) Encode(e *dicom.Encoder) {
	doassert(v.CommandDataSetType == 0x101)
	encodeDIMSEField(e, dicom.TagCommandField, uint16(0x8001))
	encodeDIMSEField(e, dicom.TagAffectedSOPClassUID, v.AffectedSOPClassUID)
	encodeDIMSEField(e, dicom.TagMessageIDBeingRespondedTo, v.MessageIDBeingRespondedTo)
	encodeDIMSEField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	encodeDIMSEField(e, dicom.TagAffectedSOPInstanceUID, v.AffectedSOPInstanceUID)
	encodeDIMSEField(e, dicom.TagStatus, v.Status)
}

func (v *C_STORE_RSP) HasData() bool {
	if v.CommandDataSetType != CommandDataSetTypeNull {
		vlog.Error("Bogus C_STORE_RSP with dataset")
	}
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
	encodeDIMSEField(e, dicom.TagCommandField, uint16(0x30))
	encodeDIMSEField(e, dicom.TagMessageID, v.MessageID)
	encodeDIMSEField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
}

// Decode C_ECHO_RQ object. Errors are reported in dd.err.
func decodeC_ECHO_RQ(dd *dimseDecoder) *C_ECHO_RQ {
	v := C_ECHO_RQ{}
	v.MessageID = dd.getUInt16(dicom.TagMessageID, RequiredElement)
	v.CommandDataSetType = dd.getUInt16(dicom.TagCommandDataSetType, RequiredElement)
	return &v
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

// Decode C_ECHO_RSP object. Errors are reported in dd.err.
func (v *C_ECHO_RSP) HasData() bool {
	if v.CommandDataSetType != CommandDataSetTypeNull {
		vlog.Error("Bogus C_ECHO_RSP with dataset")
	}
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func (v *C_ECHO_RSP) String() string {
	return fmt.Sprintf("echorsp{messageid:%v cmddatasettype: %v status: %v}", v.MessageIDBeingRespondedTo, v.CommandDataSetType, v.Status)
}

func (v *C_ECHO_RSP) Encode(e *dicom.Encoder) {
	encodeDIMSEField(e, dicom.TagCommandField, uint16(0x8030))
	encodeDIMSEField(e, dicom.TagMessageIDBeingRespondedTo, v.MessageIDBeingRespondedTo)
	encodeDIMSEField(e, dicom.TagCommandDataSetType, v.CommandDataSetType)
	encodeDIMSEField(e, dicom.TagStatus, v.Status)
}

func decodeC_ECHO_RSP(dd *dimseDecoder) *C_ECHO_RSP {
	v := C_ECHO_RSP{}
	v.MessageIDBeingRespondedTo = dd.getUInt16(dicom.TagMessageIDBeingRespondedTo, RequiredElement)
	v.CommandDataSetType = dd.getUInt16(dicom.TagCommandDataSetType, RequiredElement)
	v.Status = dd.getUInt16(dicom.TagStatus, RequiredElement)
	return &v
}

func (v *C_ECHO_RQ) HasData() bool {
	if v.CommandDataSetType != CommandDataSetTypeNull {
		vlog.Error("Bogus C_ECHO_RQ with dataset")
	}
	return v.CommandDataSetType != CommandDataSetTypeNull
}

func ReadDIMSEMessage(d *dicom.Decoder) DIMSEMessage {
	// A DIMSE message is a sequence of DicomElements, encoded in implicit
	// LE.
	//
	// TODO(saito) make sure that's the case. Where the ref?
	var elems []*dicom.DicomElement
	d.PushTransferSyntax(binary.LittleEndian, dicom.ImplicitVR)
	defer d.PopTransferSyntax()
	for d.Len() > 0 {
		elem := dicom.ReadDataElement(d)
		if d.Error() != nil {
			break
		}
		elems = append(elems, elem)
	}

	// Convert elems[] into a golang struct.
	dd := dimseDecoder{elems: elems, err: nil}
	commandField := dd.getUInt16(dicom.TagCommandField, RequiredElement)
	if dd.err != nil {
		d.SetError(dd.err)
		return nil
	}
	var v DIMSEMessage
	switch commandField {
	case 1:
		v = decodeC_STORE_RQ(&dd)
	case 0x8001:
		v = decodeC_STORE_RSP(&dd)
	case 0x30:
		v = decodeC_ECHO_RQ(&dd)
	case 0x8030:
		v = decodeC_ECHO_RSP(&dd)
	default:
		dd.setError(fmt.Errorf("Unknown DIMSE command 0x%x", commandField))
	}
	if dd.err != nil {
		d.SetError(dd.err)
		return nil
	}
	return v
}

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
	encodeDIMSEField(e, dicom.TagCommandGroupLength, uint32(len(bytes)))
	e.WriteBytes(bytes)
}

// Helper class for assembling a DIMSE command message and data payload from a
// sequence of P_DATA_TF PDUs.
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
	vlog.VI(1).Infof("Read all data for syntax %s, command [%v], data %d bytes, err%v",
		dicom.UIDString(context.abstractSyntaxUID),
		command.String(), len(a.dataBytes), err)
	*a = dimseCommandAssembler{}
	return context.abstractSyntaxUID, context.transferSyntaxUID, command, dataBytes, nil
	// TODO(saito) Verify that there's no unread items after the last command&data.
}
