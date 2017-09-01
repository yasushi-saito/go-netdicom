package netdicom

// Implements message types defined in P3.8. It sits below the DIMSE layer.
//
// http://dicom.nema.org/medical/dicom/current/output/pdf/part08.pdf
import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/golang/glog"
	"github.com/yasushi-saito/go-dicom"
	"io"
)

type PDU interface {
	// Encode the PDU payload. The "payload" here excludes the first 6 bytes
	// that are common to all PDU types - they are encoded in EncodePDU separately.
	WritePayload(*dicom.Encoder)
	// Print human-readable description for debugging.
	String() string
}

// Possible Type field for PDUs.
type PDUType byte

const (
	PDUTypeA_ASSOCIATE_RQ PDUType = 1
	PDUTypeA_ASSOCIATE_AC         = 2
	PDUTypeA_ASSOCIATE_RJ         = 3
	PDUTypeP_DATA_TF              = 4
	PDUTypeA_RELEASE_RQ           = 5
	PDUTypeA_RELEASE_RP           = 6
	PDUTypeA_ABORT                = 7
)

type SubItem interface {
	Write(*dicom.Encoder)
	String() string
}

// Possible Type field values for SubItem.
const (
	ItemTypeApplicationContext           = 0x10
	ItemTypePresentationContextRequest   = 0x20
	ItemTypePresentationContextResponse  = 0x21
	ItemTypeAbstractSyntax               = 0x30
	ItemTypeTransferSyntax               = 0x40
	ItemTypeUserInformation              = 0x50
	ItemTypeUserInformationMaximumLength = 0x51
	ItemTypeImplementationClassUID       = 0x52
	ItemTypeAsynchronousOperationsWindow = 0x53
	ItemTypeImplementationVersionName    = 0x55
)

func decodeSubItem(d *dicom.Decoder) SubItem {
	itemType := d.ReadByte()
	d.Skip(1)
	length := d.ReadUInt16()
	if itemType == ItemTypeApplicationContext {
		return decodeApplicationContextItem(d, length)
	}
	if itemType == ItemTypeAbstractSyntax {
		return decodeAbstractSyntaxSubItem(d, length)
	}
	if itemType == ItemTypeTransferSyntax {
		return decodeTransferSyntaxSubItem(d, length)
	}
	if itemType == ItemTypePresentationContextRequest {
		return decodePresentationContextItem(d, itemType, length)
	}
	if itemType == ItemTypePresentationContextResponse {
		return decodePresentationContextItem(d, itemType, length)
	}
	if itemType == ItemTypeUserInformation {
		return decodeUserInformationItem(d, length)
	}
	if itemType == ItemTypeUserInformationMaximumLength {
		return decodeUserInformationMaximumLengthItem(d, length)
	}
	if itemType == ItemTypeImplementationClassUID {
		return decodeImplementationClassUIDSubItem(d, length)
	}
	if itemType == ItemTypeAsynchronousOperationsWindow {
		return decodeAsynchronousOperationsWindowSubItem(d, length)
	}
	if itemType == ItemTypeImplementationVersionName {
		return decodeImplementationVersionNameSubItem(d, length)
	}
	d.SetError(fmt.Errorf("Unknown item type: 0x%x", itemType))
	return nil
}

func encodeSubItemHeader(e *dicom.Encoder, itemType byte, length uint16) {
	e.WriteByte(itemType)
	e.WriteZeros(1)
	e.WriteUInt16(length)
}

// P3.8 9.3.2.3
type UserInformationItem struct {
	Items []SubItem // P3.8, Annex D.
	// Data []byte
}

func (v *UserInformationItem) Write(e *dicom.Encoder) {
	itemEncoder := dicom.NewEncoder(binary.BigEndian, dicom.UnknownVR)
	for _, s := range v.Items {
		s.Write(itemEncoder)
	}
	itemBytes, err := itemEncoder.Finish()
	if err != nil {
		e.SetError(err)
		return
	}
	encodeSubItemHeader(e, ItemTypeUserInformation, uint16(len(itemBytes)))
	e.WriteBytes(itemBytes)
}

func decodeUserInformationItem(d *dicom.Decoder, length uint16) *UserInformationItem {
	v := &UserInformationItem{}
	d.PushLimit(int64(length))
	defer d.PopLimit()
	for d.Len() > 0 {
		item := decodeSubItem(d)
		if d.Error() != nil {
			break
		}
		v.Items = append(v.Items, item)
	}
	return v
}

func (v *UserInformationItem) String() string {
	return fmt.Sprintf("userinformationitem{items: %s}",
		subItemListString(v.Items))
}

// P3.8 D.1
type UserInformationMaximumLengthItem struct {
	MaximumLengthReceived uint32
}

func (v *UserInformationMaximumLengthItem) Write(e *dicom.Encoder) {
	encodeSubItemHeader(e, ItemTypeUserInformationMaximumLength, 4)
	e.WriteUInt32(v.MaximumLengthReceived)
}

func decodeUserInformationMaximumLengthItem(d *dicom.Decoder, length uint16) *UserInformationMaximumLengthItem {
	if length != 4 {
		d.SetError(fmt.Errorf("UserInformationMaximumLengthItem must be 4 bytes, but found %dB", length))
	}
	return &UserInformationMaximumLengthItem{MaximumLengthReceived: d.ReadUInt32()}
}

func (item *UserInformationMaximumLengthItem) String() string {
	return fmt.Sprintf("userinformationmaximumlengthitem{%d}",
		item.MaximumLengthReceived)
}

// PS3.7 Annex D.3.3.2.1
type ImplementationClassUIDSubItem subItemWithName

func decodeImplementationClassUIDSubItem(d *dicom.Decoder, length uint16) *ImplementationClassUIDSubItem {
	return &ImplementationClassUIDSubItem{Name: decodeSubItemWithName(d, length)}
}

func (v *ImplementationClassUIDSubItem) Write(e *dicom.Encoder) {
	encodeSubItemWithName(e, ItemTypeImplementationClassUID, v.Name)
}

func (v *ImplementationClassUIDSubItem) String() string {
	return fmt.Sprintf("implementationclassuid{name: \"%s\"}", v.Name)
}

// PS3.7 Annex D.3.3.3.1
type AsynchronousOperationsWindowSubItem struct {
	MaxOpsInvoked   uint16
	MaxOpsPerformed uint16
}

func decodeAsynchronousOperationsWindowSubItem(d *dicom.Decoder, length uint16) *AsynchronousOperationsWindowSubItem {
	return &AsynchronousOperationsWindowSubItem{
		MaxOpsInvoked:   d.ReadUInt16(),
		MaxOpsPerformed: d.ReadUInt16(),
	}
}

func (v *AsynchronousOperationsWindowSubItem) Write(e *dicom.Encoder) {
	encodeSubItemHeader(e, ItemTypeAsynchronousOperationsWindow, 2*2)
	e.WriteUInt16(v.MaxOpsInvoked)
	e.WriteUInt16(v.MaxOpsPerformed)
}

func (v *AsynchronousOperationsWindowSubItem) String() string {
	return fmt.Sprintf("asynchronousopswindow{invoked: %d performed: %d}",
		v.MaxOpsInvoked, v.MaxOpsPerformed)
}

// PS3.7 Annex D.3.3.2.3
type ImplementationVersionNameSubItem subItemWithName

func decodeImplementationVersionNameSubItem(d *dicom.Decoder, length uint16) *ImplementationVersionNameSubItem {
	return &ImplementationVersionNameSubItem{Name: decodeSubItemWithName(d, length)}
}

func (v *ImplementationVersionNameSubItem) Write(e *dicom.Encoder) {
	encodeSubItemWithName(e, ItemTypeImplementationVersionName, v.Name)
}

func (v *ImplementationVersionNameSubItem) String() string {
	return fmt.Sprintf("implementationversionname{name: \"%s\"}", v.Name)
}

// Container for subitems that this package doesnt' support
type SubItemUnsupported struct {
	Type byte
	Data []byte
}

func (item *SubItemUnsupported) Write(e *dicom.Encoder) {
	encodeSubItemHeader(e, item.Type, uint16(len(item.Data)))
	// TODO: handle unicode properly
	e.WriteBytes(item.Data)
}

func (item *SubItemUnsupported) String() string {
	return fmt.Sprintf("subitemunsupported{type: 0x%0x data: %dbytes}",
		item.Type, len(item.Data))
}

func decodeSubItemUnsupported(
	d *dicom.Decoder, itemType byte, length uint16) *SubItemUnsupported {
	v := &SubItemUnsupported{}
	v.Type = itemType
	v.Data = d.ReadBytes(int(length))
	return v
}

type subItemWithName struct {
	// Type byte
	Name string
}

func encodeSubItemWithName(e *dicom.Encoder, itemType byte, name string) {
	encodeSubItemHeader(e, itemType, uint16(len(name)))
	// TODO: handle unicode properly
	e.WriteBytes([]byte(name))
}

func decodeSubItemWithName(d *dicom.Decoder, length uint16) string {
	return d.ReadString(int(length))
}

//func (item *SubItemWithName) String() string {
//	return fmt.Sprintf("subitem{type: 0x%0x name: \"%s\"}", item.Type, item.Name)
//}

type ApplicationContextItem subItemWithName

const DefaultApplicationContextItemName = "1.2.840.10008.3.1.1.1"

func decodeApplicationContextItem(d *dicom.Decoder, length uint16) *ApplicationContextItem {
	return &ApplicationContextItem{Name: decodeSubItemWithName(d, length)}
}

func (v *ApplicationContextItem) Write(e *dicom.Encoder) {
	encodeSubItemWithName(e, ItemTypeApplicationContext, v.Name)
}

func (v *ApplicationContextItem) String() string {
	return fmt.Sprintf("applicationcontext{name: \"%s\"}", v.Name)
}

type AbstractSyntaxSubItem subItemWithName

func decodeAbstractSyntaxSubItem(d *dicom.Decoder, length uint16) *AbstractSyntaxSubItem {
	return &AbstractSyntaxSubItem{Name: decodeSubItemWithName(d, length)}
}

func (v *AbstractSyntaxSubItem) Write(e *dicom.Encoder) {
	encodeSubItemWithName(e, ItemTypeAbstractSyntax, v.Name)
}

func (v *AbstractSyntaxSubItem) String() string {
	return fmt.Sprintf("abstractsyntax{name: \"%s\"}", v.Name)
}

type TransferSyntaxSubItem subItemWithName

func decodeTransferSyntaxSubItem(d *dicom.Decoder, length uint16) *TransferSyntaxSubItem {
	return &TransferSyntaxSubItem{Name: decodeSubItemWithName(d, length)}
}

func (v *TransferSyntaxSubItem) Write(e *dicom.Encoder) {
	encodeSubItemWithName(e, ItemTypeTransferSyntax, v.Name)
}

func (v *TransferSyntaxSubItem) String() string {
	return fmt.Sprintf("transfersyntax{name: \"%s\"}", v.Name)
}

// P3.8 9.3.2.2, 9.3.3.2
type PresentationContextItem struct {
	Type      byte // ItemTypePresentationContext*
	ContextID byte
	// 1 byte reserved
	Result byte // Used iff type=0x21, zero else.
	// 1 byte reserved
	Items []SubItem // List of {Abstract,Transfer}SyntaxSubItem
}

func decodePresentationContextItem(d *dicom.Decoder, itemType byte, length uint16) *PresentationContextItem {
	v := &PresentationContextItem{Type: itemType}
	d.PushLimit(int64(length))
	defer d.PopLimit()
	v.ContextID = d.ReadByte()
	d.Skip(1)
	v.Result = d.ReadByte()
	d.Skip(1)
	for d.Len() > 0 {
		item := decodeSubItem(d)
		if d.Error() != nil {
			break
		}
		v.Items = append(v.Items, item)
	}
	if v.ContextID%2 != 1 {
		d.SetError(fmt.Errorf("PresentationContextItem ID must be odd, but found %x", v.ContextID))
	}
	return v
}

func (v *PresentationContextItem) Write(e *dicom.Encoder) {
	doassert(v.Type == ItemTypePresentationContextRequest ||
		v.Type == ItemTypePresentationContextResponse)

	itemEncoder := dicom.NewEncoder(binary.BigEndian, dicom.UnknownVR)
	for _, s := range v.Items {
		s.Write(itemEncoder)
	}
	itemBytes, err := itemEncoder.Finish()
	if err != nil {
		e.SetError(err)
		return
	}
	encodeSubItemHeader(e, v.Type, uint16(4+len(itemBytes)))
	e.WriteByte(v.ContextID)
	e.WriteZeros(3)
	e.WriteBytes(itemBytes)
}

func (v *PresentationContextItem) String() string {
	itemType := "rq"
	if v.Type == ItemTypePresentationContextResponse {
		itemType = "ac"
	}
	return fmt.Sprintf("presentationcontext%s{id: %d items:%s}",
		itemType, v.ContextID, subItemListString(v.Items))
}

// P3.8 9.3.2.2.1 & 9.3.2.2.2
type PresentationDataValueItem struct {
	// Length: 2 + len(Value)
	ContextID byte

	// P3.8, E.2: the following two fields encode a single byte.
	Command bool // Bit 7 (LSB): 1 means command 0 means data
	Last    bool // Bit 6: 1 means last fragment. 0 means not last fragment.

	// Payload, either command or data
	Value []byte
}

func ReadPresentationDataValueItem(d *dicom.Decoder) PresentationDataValueItem {
	item := PresentationDataValueItem{}
	length := d.ReadUInt32()
	item.ContextID = d.ReadByte()
	header := d.ReadByte()
	item.Command = (header&1 != 0)
	item.Last = (header&2 != 0)
	item.Value = d.ReadBytes(int(length - 2)) // remove contextID and header
	if header&0xfc != 0 {
		d.SetError(fmt.Errorf("PresentationDataValueItem: illegal header byte %x", header))
	}
	return item
}

func (v *PresentationDataValueItem) Write(e *dicom.Encoder) {
	var header byte = 0
	if v.Command {
		header |= 1
	}
	if v.Last {
		header |= 2
	}
	e.WriteUInt32(uint32(2 + len(v.Value)))
	e.WriteByte(v.ContextID)
	e.WriteByte(header)
	e.WriteBytes(v.Value)
}

func (v *PresentationDataValueItem) String() string {
	return fmt.Sprintf("presentationdatavalue{context: %d, cmd:%v last:%v value: %d bytes}", v.ContextID, v.Command, v.Last, len(v.Value))
}

func EncodePDU(pdu PDU) ([]byte, error) {
	var pduType PDUType
	if n, ok := pdu.(*A_ASSOCIATE); ok {
		pduType = n.Type
	} else if _, ok := pdu.(*A_ASSOCIATE_RJ); ok {
		pduType = PDUTypeA_ASSOCIATE_RJ
	} else if _, ok := pdu.(*P_DATA_TF); ok {
		pduType = PDUTypeP_DATA_TF
	} else if _, ok := pdu.(*A_RELEASE_RQ); ok {
		pduType = PDUTypeA_RELEASE_RQ
	} else if _, ok := pdu.(*A_RELEASE_RP); ok {
		pduType = PDUTypeA_RELEASE_RP
	} else if _, ok := pdu.(*A_ABORT); ok {
		pduType = PDUTypeA_ABORT
	} else {
		glog.Fatalf("Unknown PDU %v", pdu)
	}

	e := dicom.NewEncoder(binary.BigEndian, dicom.UnknownVR)
	pdu.WritePayload(e)
	payload, err := e.Finish()
	if err != nil {
		return nil, err
	}

	// Reserve the header bytes. It will be filled in Finish.
	header := make([]byte, 6) // First 6 bytes of buf.
	header[0] = byte(pduType)
	header[1] = 0 // Reserved.
	binary.BigEndian.PutUint32(header[2:6], uint32(len(payload)))
	return append(header, payload...), nil
}

func ReadPDU(in io.Reader, maxPDUSize int) (PDU, error) {
	var pduType PDUType
	var skip byte
	var length uint32
	err := binary.Read(in, binary.BigEndian, &pduType)
	if err != nil {
		return nil, err
	}
	err = binary.Read(in, binary.BigEndian, &skip)
	if err != nil {
		return nil, err
	}
	err = binary.Read(in, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}
	if length >= uint32(maxPDUSize)*2 {
		// Avoid using too much memory. *2 is just an arbitrary slack.
		return nil, fmt.Errorf("Invalid length %d; it's much larger than max PDU size of %d", length, maxPDUSize)
	}
	d := dicom.NewDecoder(in, int64(length),
		binary.BigEndian, // PDU is always big endian
		dicom.UnknownVR)  // irrelevant for PDU parsing
	var pdu PDU = nil
	switch pduType {
	case PDUTypeA_ASSOCIATE_RQ:
		fallthrough
	case PDUTypeA_ASSOCIATE_AC:
		pdu = decodeA_ASSOCIATE(d, pduType)
	case PDUTypeA_ASSOCIATE_RJ:
		pdu = decodeA_ASSOCIATE_RJ(d)
	case PDUTypeA_ABORT:
		pdu = decodeA_ABORT(d)
	case PDUTypeP_DATA_TF:
		pdu = decodeP_DATA_TF(d)
	case PDUTypeA_RELEASE_RQ:
		pdu = decodeA_RELEASE_RQ(d)
	case PDUTypeA_RELEASE_RP:
		pdu = decodeA_RELEASE_RP(d)
	}
	if pdu == nil {
		err := fmt.Errorf("ReadPDU: unknown message type %d", pduType)
		return nil, err
	}
	if err := d.Finish(); err != nil {
		return nil, err
	}
	return pdu, nil
}

type A_RELEASE_RQ struct {
}

func decodeA_RELEASE_RQ(d *dicom.Decoder) *A_RELEASE_RQ {
	pdu := &A_RELEASE_RQ{}
	d.Skip(4)
	return pdu
}

func (pdu *A_RELEASE_RQ) WritePayload(e *dicom.Encoder) {
	e.WriteZeros(4)
}

func (pdu *A_RELEASE_RQ) String() string {
	return fmt.Sprintf("A_RELEASE_RQ(%v)", *pdu)
}

type A_RELEASE_RP struct {
}

func decodeA_RELEASE_RP(d *dicom.Decoder) *A_RELEASE_RP {
	pdu := &A_RELEASE_RP{}
	d.Skip(4)
	return pdu
}

func (pdu *A_RELEASE_RP) WritePayload(e *dicom.Encoder) {
	e.WriteZeros(4)
}

func (pdu *A_RELEASE_RP) String() string {
	return fmt.Sprintf("A_RELEASE_RP(%v)", *pdu)
}

func subItemListString(items []SubItem) string {
	buf := bytes.Buffer{}
	buf.WriteString("[")
	for i, subitem := range items {
		if i > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(subitem.String())
	}
	buf.WriteString("]")
	return buf.String()
}

const CurrentProtocolVersion uint16 = 1

// Defines A_ASSOCIATE_{RQ,AC}. P3.8 9.3.2 and 9.3.3
type A_ASSOCIATE struct {
	Type            PDUType // One of {PDUTypeA_Associate_RQ,PDUTypeA_Associate_AC}
	ProtocolVersion uint16
	// Reserved uint16
	CalledAETitle  string // For .._AC, the value is copied from A_ASSOCIATE_RQ
	CallingAETitle string // For .._AC, the value is copied from A_ASSOCIATE_RQ
	Items          []SubItem
}

func decodeA_ASSOCIATE(d *dicom.Decoder, pduType PDUType) *A_ASSOCIATE {
	pdu := &A_ASSOCIATE{}
	pdu.Type = pduType
	pdu.ProtocolVersion = d.ReadUInt16()
	d.Skip(2) // Reserved
	pdu.CalledAETitle = d.ReadString(16)
	pdu.CallingAETitle = d.ReadString(16)
	d.Skip(8 * 4)
	for d.Len() > 0 {
		item := decodeSubItem(d)
		if d.Error() != nil {
			break
		}
		pdu.Items = append(pdu.Items, item)
	}
	if pdu.CalledAETitle == "" || pdu.CallingAETitle == "" {
		d.SetError(fmt.Errorf("A_ASSOCIATE.{Called,Calling}AETitle must not be empty, in %v", pdu.String()))
	}
	return pdu
}

func (pdu *A_ASSOCIATE) WritePayload(e *dicom.Encoder) {
	doassert(pdu.Type != 0)
	doassert(pdu.CalledAETitle != "")
	doassert(pdu.CallingAETitle != "")
	e.WriteUInt16(pdu.ProtocolVersion)
	e.WriteZeros(2) // Reserved
	e.WriteString(fillString(pdu.CalledAETitle, 16))
	e.WriteString(fillString(pdu.CallingAETitle, 16))
	e.WriteZeros(8 * 4)
	for _, item := range pdu.Items {
		item.Write(e)
	}
}

func (pdu *A_ASSOCIATE) String() string {
	name := "AC"
	if pdu.Type == PDUTypeA_ASSOCIATE_RQ {
		name = "RQ"
	}
	return fmt.Sprintf("A_ASSOCIATE_%s{version:%v called:'%v' calling:'%v' items:%s}",
		name, pdu.ProtocolVersion,
		pdu.CalledAETitle, pdu.CallingAETitle, subItemListString(pdu.Items))
}

// P3.8 9.3.4
type A_ASSOCIATE_RJ struct {
	Result byte
	Source byte
	Reason byte
}

// Possible values for A_ASSOCIATE_RJ.Result
const (
	ResultRejectedPermanent = 1
	ResultRejectedTransient = 2
)

// Possible values for A_ASSOCIATE_RJ.Source
const (
	SourceULServiceUser                 = 1
	SourceULServiceProviderACSE         = 2
	SourceULServiceProviderPresentation = 3
)

// Possible values for A_ASSOCIATE_RJ.Reason
const (
	ReasonNone                               = 1
	ReasonApplicationContextNameNotSupported = 2
)

func decodeA_ASSOCIATE_RJ(d *dicom.Decoder) *A_ASSOCIATE_RJ {
	pdu := &A_ASSOCIATE_RJ{}
	d.Skip(1) // reserved
	pdu.Result = d.ReadByte()
	pdu.Source = d.ReadByte()
	pdu.Reason = d.ReadByte()
	return pdu
}

func (pdu *A_ASSOCIATE_RJ) WritePayload(e *dicom.Encoder) {
	e.WriteZeros(1)
	e.WriteByte(pdu.Result)
	e.WriteByte(pdu.Source)
	e.WriteByte(pdu.Reason)
}

func (pdu *A_ASSOCIATE_RJ) String() string {
	return "A_ASSOCIATE_RJ"
}

type A_ABORT struct {
	Source byte
	Reason byte
}

func decodeA_ABORT(d *dicom.Decoder) *A_ABORT {
	pdu := &A_ABORT{}
	d.Skip(2)
	pdu.Source = d.ReadByte()
	pdu.Reason = d.ReadByte()
	return pdu
}

func (pdu *A_ABORT) WritePayload(e *dicom.Encoder) {
	e.WriteZeros(2)
	e.WriteByte(pdu.Source)
	e.WriteByte(pdu.Reason)
}

func (pdu *A_ABORT) String() string {
	return fmt.Sprintf("A_ABORT{source:%d reason:%d}", pdu.Source, pdu.Reason)
}

type P_DATA_TF struct {
	Items []PresentationDataValueItem
}

func decodeP_DATA_TF(d *dicom.Decoder) *P_DATA_TF {
	pdu := &P_DATA_TF{}
	for d.Len() > 0 {
		item := ReadPresentationDataValueItem(d)
		if d.Error() != nil {
			break
		}
		pdu.Items = append(pdu.Items, item)
	}
	return pdu
}

func (pdu *P_DATA_TF) WritePayload(e *dicom.Encoder) {
	for _, item := range pdu.Items {
		item.Write(e)
	}
}

func (pdu *P_DATA_TF) String() string {
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("P_DATA_TF{items: ["))

	for i, item := range pdu.Items {
		if i > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(item.String())
	}
	buf.WriteString("]}")
	return buf.String()
}

// fillString pads the string with " " up to the given length.
func fillString(v string, length int) string {
	if len(v) > length {
		return v[:16]
	}
	for len(v) < length {
		v += " "
	}
	return v
}
