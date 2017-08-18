package netdicom

import (
	"bytes"
	"fmt"
	"io"
	"log"
)

type PDU interface {
	EncodePayload(*Encoder)
	DebugString() string
}

// Possible Type field for PDUs.
const (
	PDUTypeA_ASSOCIATE_RQ = 1
	PDUTypeA_ASSOCIATE_AC = 2
	PDUTypeA_ASSOCIATE_RJ = 3
	PDUTypeP_DATA_TF      = 4
	PDUTypeA_RELEASE_RQ   = 5
	PDUTypeA_RELEASE_RP   = 6
	PDUTypeA_ABORT        = 7
)

type SubItem interface {
	Encode(*Encoder)
	DebugString() string
}

// Possible Type field values for SubItem.
const (
	ItemTypeApplicationContext           = 0x10
	ItemTypePresentationContextRQ        = 0x20
	ItemTypePresentationContextAC        = 0x21
	ItemTypeAbstractSyntax               = 0x30
	ItemTypeTransferSyntax               = 0x40
	ItemTypeUserInformation              = 0x50
	ItemTypeUserInformationMaximumLength = 0x51
	ItemTypeUnsupported52                = 0x52
)

func decodeSubItem(d *Decoder) SubItem {
	itemType := d.DecodeByte()
	d.Skip(1)
	length := d.DecodeUint16()
	// log.Printf("DecodeSubItem: item=0x%x length=%v, err=%v", itemType, length, d.Error())
	if itemType == ItemTypeApplicationContext ||
		itemType == ItemTypeAbstractSyntax ||
		itemType == ItemTypeTransferSyntax {
		return decodeSubItemWithName(d, itemType, length)
	}
	if itemType == ItemTypePresentationContextRQ ||
		itemType == ItemTypePresentationContextAC {
		return decodePresentationContextItem(d, itemType, length)
	}
	if itemType == ItemTypeUserInformation {
		return decodeUserInformationItem(d, length)
	}
	if itemType == ItemTypeUserInformationMaximumLength {
		return decodeUserInformationMaximumLengthItem(d, length)
	}
	if itemType == 0x52 || itemType == 0x55 {
		return decodeSubItemUnsupported(d, itemType, length)
	}
	panic(fmt.Sprintf("Unknown item type: 0x%x", itemType))
}

func encodeSubItemHeader(e *Encoder, itemType byte, length uint16) {
	e.EncodeByte(itemType)
	e.EncodeZeros(1)
	e.EncodeUint16(length)
}

// P3.8 9.3.2.3
type UserInformationItem struct {
	Items []SubItem // P3.8, Annex D.
	// Data []byte
}

func (v *UserInformationItem) Encode(e *Encoder) {
	itemEncoder := NewEncoder()
	for _, s := range v.Items {
		s.Encode(itemEncoder)
	}
	itemBytes, err := itemEncoder.Finish()
	if err != nil {
		e.SetError(err)
		return
	}
	encodeSubItemHeader(e, ItemTypeUserInformation, uint16(len(itemBytes)))
	e.EncodeBytes(itemBytes)
}

func decodeUserInformationItem(d *Decoder, length uint16) *UserInformationItem {
	v := &UserInformationItem{}
	d.PushLimit(int(length))
	defer d.PopLimit()
	for d.Available() > 0 && d.Error() == nil {
		v.Items = append(v.Items, decodeSubItem(d))
	}
	return v
}

func (v *UserInformationItem) DebugString() string {
	return fmt.Sprintf("userinformationitem{items: %s}",
		subItemListDebugString(v.Items))
}

// P3.8 D.1
type UserInformationMaximumLengthItem struct {
	MaximumLengthReceived uint32
}

func (v *UserInformationMaximumLengthItem) Encode(e *Encoder) {
	encodeSubItemHeader(e, ItemTypeUserInformationMaximumLength, 4)
	e.EncodeUint32(v.MaximumLengthReceived)
}

func decodeUserInformationMaximumLengthItem(d *Decoder, length uint16) *UserInformationMaximumLengthItem {
	doassert(length == 4) //TODO
	return &UserInformationMaximumLengthItem{MaximumLengthReceived: d.DecodeUint32()}
}

func (item *UserInformationMaximumLengthItem) DebugString() string {
	return fmt.Sprintf("userinformationmaximumlengthitem{%d}",
		item.MaximumLengthReceived)
}

// Container for subitems that this package doesnt' support
type SubItemUnsupported struct {
	Type byte
	Data []byte
}

func (item *SubItemUnsupported) Encode(e *Encoder) {
	encodeSubItemHeader(e, item.Type, uint16(len(item.Data)))
	// TODO: handle unicode properly
	e.EncodeBytes(item.Data)
}

func (item *SubItemUnsupported) DebugString() string {
	return fmt.Sprintf("subitemunsupported{type: 0x%0x data: %dbytes}",
		item.Type, len(item.Data))
}

func decodeSubItemUnsupported(
	d *Decoder, itemType byte, length uint16) *SubItemUnsupported {
	v := &SubItemUnsupported{}
	v.Type = itemType
	v.Data = d.DecodeBytes(int(length))
	return v
}

type SubItemWithName struct {
	Type byte
	Name string
}

func (item *SubItemWithName) Encode(e *Encoder) {
	encodeSubItemHeader(e, item.Type, uint16(len(item.Name)))
	// TODO: handle unicode properly
	e.EncodeBytes([]byte(item.Name))
}

func (item *SubItemWithName) DebugString() string {
	return fmt.Sprintf("subitem{type: 0x%0x name: \"%s\"}", item.Type, item.Name)
}

func decodeSubItemWithName(d *Decoder, itemType byte, length uint16) *SubItemWithName {
	v := &SubItemWithName{}
	v.Type = itemType
	v.Name = d.DecodeString(int(length))
	return v
}

// The name that should be used for ApplicationContextItem.Name.
const DefaultApplicationContextItemName = "1.2.840.10008.3.1.1.1"

// P3.8 9.3.2.2, 9.3.3.2
type PresentationContextItem struct {
	Type      byte // ItemTypePresentationContext*
	ContextID byte
	// 1 byte reserved
	Result byte // Used iff type=0x21, zero else.
	// 1 byte reserved
	Items []SubItem // List of {Abstract,Transfer}SyntaxSubItem
}

func decodePresentationContextItem(d *Decoder, itemType byte, length uint16) *PresentationContextItem {
	v := &PresentationContextItem{Type: itemType}
	d.PushLimit(int(length))
	defer d.PopLimit()
	v.ContextID = d.DecodeByte()
	d.Skip(1)
	v.Result = d.DecodeByte()
	d.Skip(1)
	for d.Available() > 0 && d.Error() == nil {
		v.Items = append(v.Items, decodeSubItem(d))
	}
	if v.ContextID%2 != 1 {
		d.SetError(fmt.Errorf("PresentationContextItem ID must be odd, but found %x", v.ContextID))
	}
	return v
}

func (v *PresentationContextItem) Encode(e *Encoder) {
	doassert(v.Type == ItemTypePresentationContextRQ || v.Type == ItemTypePresentationContextAC)

	itemEncoder := NewEncoder()
	for _, s := range v.Items {
		s.Encode(itemEncoder)
	}
	itemBytes, err := itemEncoder.Finish()
	if err != nil {
		e.SetError(err)
		return
	}
	encodeSubItemHeader(e, v.Type, uint16(8+len(itemBytes)))
	e.EncodeByte(v.ContextID)
	e.EncodeZeros(3)
	e.EncodeBytes(itemBytes)
}

func (item *PresentationContextItem) DebugString() string {
	return fmt.Sprintf("presentationcontext{id: %d items:%s}",
		item.ContextID, subItemListDebugString(item.Items))
}

// P3.8 9.3.2.2.1 & 9.3.2.2.2
type AbstractSyntaxSubItem SubItem // Type=30H
//type TransferSyntaxSubItem SubItem // Type=40H

type PresentationDataValueItem struct {
	Length    uint32
	ContextID byte
	Value     []byte
}

func NewPresentationDataValueItem(contextID byte, value []byte) PresentationDataValueItem {
	return PresentationDataValueItem{
		Length:    uint32(1 + len(value)),
		ContextID: contextID,
		Value:     value,
	}
}

func DecodePresentationDataValueItem(d *Decoder) PresentationDataValueItem {
	item := PresentationDataValueItem{}
	item.Length = d.DecodeUint32()
	item.ContextID = d.DecodeByte()
	item.Value = d.DecodeBytes(int(item.Length))
	return item
}

func (item *PresentationDataValueItem) Encode(e *Encoder) {
	e.EncodeUint32(item.Length)
	e.EncodeByte(item.ContextID)
	e.EncodeBytes(item.Value)
}

func EncodePDU(pdu PDU) ([]byte, error) {
	var pduType byte
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
		panic(fmt.Sprintf("Unknown PDU %v", pdu))
	}

	e := NewEncoder()
	pdu.EncodePayload(e)
	payload, err := e.Finish()
	if err != nil {
		return nil, err
	}

	// Reserve the header bytes. It will be filled in Finish.
	header := make([]byte, 6) // First 6 bytes of buf.
	header[0] = pduType
	header[1] = 0 // Reserved.
	e.byteOrder.PutUint32(header[2:6], uint32(len(payload)))
	return append(header, payload...), nil
}

func DecodePDU(in io.Reader) (PDU, error) {
	d := NewDecoder(in)
	if d.err != nil {
		return nil, d.err
	}
	var pdu PDU = nil
	switch d.Type {
	case PDUTypeA_ASSOCIATE_RQ:
		fallthrough
	case PDUTypeA_ASSOCIATE_AC:
		pdu = decodeA_ASSOCIATE(d, d.Type)
		if d.err != nil {
			return nil, d.err
		}
	case PDUTypeA_ASSOCIATE_RJ:
		pdu = decodeA_ASSOCIATE_RJ(d)
		if d.err != nil {
			return nil, d.err
		}
	case PDUTypeA_ABORT:
		pdu = decodeA_ABORT(d)
		if d.err != nil {
			return nil, d.err
		}
	}
	if pdu == nil {
		// PDUTypeP_DATA_TF      = 4
		// PDUTypeA_RELEASE_RQ   = 5
		// PDUTypeA_RELEASE_RP   = 6
		// PDUTypeA_ABORT        = 7
		err := fmt.Errorf("DecodePDU: unknown message type %d", d.Type)
		log.Panicf("%v", err)
		return nil, err
	}
	if err := d.Finish(); err != nil {
		log.Panicf("DecodePDU: %v", err)
		return nil, err
	}
	return pdu, nil
}

type A_RELEASE_RQ struct {
}

func New_A_RELEASE_RQ() *A_RELEASE_RQ {
	return &A_RELEASE_RQ{}
}

func DecodeA_RELEASE_RQ(d *Decoder) *A_RELEASE_RQ {
	pdu := &A_RELEASE_RQ{}
	d.Skip(4)
	return pdu
}

func (pdu *A_RELEASE_RQ) EncodePayload(e *Encoder) {
	e.EncodeZeros(4)
}

func (pdu *A_RELEASE_RQ) DebugString() string {
	buf := &bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("A_RELEASE_RQ(%v)", *pdu))
	return buf.String()
}

type A_RELEASE_RP struct {
}

func New_A_RELEASE_RP() *A_RELEASE_RP {
	return &A_RELEASE_RP{}
}

func (pdu *A_RELEASE_RP) Decode(d *Decoder) {
	d.Skip(4)
}

func (pdu *A_RELEASE_RP) EncodePayload(e *Encoder) {
	e.EncodeZeros(4)
}

func (pdu *A_RELEASE_RP) DebugString() string {
	return fmt.Sprintf("A_RELEASE_RP(%v)", *pdu)
}

// type A_ASSOCIATE_RQ struct {
// 	ProtocolVersion uint16
// 	// Reserved uint16
// 	CalledAETitle  string
// 	CallingAETitle string
// 	Items          []SubItem
// }

// func New_A_ASSOCIATE_RQ(params SessionParams) *A_ASSOCIATE_RQ {
// 	pdu := A_ASSOCIATE_RQ{}
// 	pdu.ProtocolVersion = 1
// 	return &pdu
// }

// func decodeA_ASSOCIATE_RQ(d *Decoder) *A_ASSOCIATE_RQ {
// 	pdu := &A_ASSOCIATE_RQ{}
// 	pdu.ProtocolVersion = d.DecodeUint16()
// 	d.Skip(2) // Reserved
// 	pdu.CalledAETitle = d.DecodeString(16)
// 	pdu.CallingAETitle = d.DecodeString(16)
// 	d.Skip(8 * 4)
// 	for d.Available() > 0 && d.Error() == nil {
// 		pdu.Items = append(pdu.Items, decodeSubItem(d))
// 	}
// 	return pdu
// }

// func (pdu *A_ASSOCIATE_RQ) Encode(e *Encoder) {
// 	e.SetType(PDUTypeA_ASSOCIATE_RQ)
// 	e.EncodeUint16(pdu.ProtocolVersion)
// 	e.EncodeZeros(2) // Reserved
// 	e.EncodeString(fillString(pdu.CalledAETitle, 16))
// 	e.EncodeString(fillString(pdu.CallingAETitle, 16))
// 	e.EncodeZeros(8 * 4)
// 	e.EncodeUint32(0) // TODO
// }
//func (pdu *A_ASSOCIATE_RQ) DebugString() string {
//	return fmt.Sprintf(
//		"A_ASSOCIATE_RQ{version:%v calledaet:%s callingaet:%s items: %s",
//		pdu.ProtocolVersion, pdu.CalledAETitle, pdu.CallingAETitle,
//		subItemListDebugString(pdu.Items))
//}

func subItemListDebugString(items []SubItem) string {
	buf := bytes.Buffer{}
	buf.WriteString("[")
	for i, subitem := range items {
		if i > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(subitem.DebugString())
	}
	buf.WriteString("]")
	return buf.String()
}

const (
	CurrentProtocolVersion uint16 = 1
)

// Defines A_ASSOCIATE_{RQ,AC}. P3.8 9.3.2 and 9.3.3
type A_ASSOCIATE struct {
	Type            byte // One of {PDUTypeA_Associate_RQ,PDUTypeA_Associate_AC}
	ProtocolVersion uint16
	// Reserved uint16
	CalledAETitle  string // For .._AC, the value is copied from A_ASSOCIATE_RQ
	CallingAETitle string // For .._AC, the value is copied from A_ASSOCIATE_RQ
	Items          []SubItem
}

func decodeA_ASSOCIATE(d *Decoder, pduType byte) *A_ASSOCIATE {
	pdu := &A_ASSOCIATE{}
	pdu.Type = pduType
	pdu.ProtocolVersion = d.DecodeUint16()
	d.Skip(2) // Reserved
	pdu.CalledAETitle = d.DecodeString(16)
	pdu.CallingAETitle = d.DecodeString(16)
	d.Skip(8 * 4)
	for d.Available() > 0 && d.Error() == nil {
		pdu.Items = append(pdu.Items, decodeSubItem(d))
	}
	return pdu
}

func (pdu *A_ASSOCIATE) EncodePayload(e *Encoder) {
	doassert(pdu.Type != 0)
	e.EncodeUint16(pdu.ProtocolVersion)
	e.EncodeZeros(2) // Reserved
	e.EncodeString(fillString(pdu.CalledAETitle, 16))
	e.EncodeString(fillString(pdu.CallingAETitle, 16))
	e.EncodeZeros(8 * 4)
	for _, item := range pdu.Items {
		item.Encode(e)
	}
}

func (pdu *A_ASSOCIATE) DebugString() string {
	name := "AC"
	if pdu.Type == PDUTypeA_ASSOCIATE_RQ {
		name = "RQ"
	}
	return fmt.Sprintf("A_ASSOCIATE_%s{version:%v called:'%v' calling:'%v' items:%s}",
		name, pdu.ProtocolVersion,
		pdu.CalledAETitle, pdu.CallingAETitle, subItemListDebugString(pdu.Items))
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

func New_A_ASSOCIATE_RJ(result, source, reason byte) *A_ASSOCIATE_RJ {
	pdu := A_ASSOCIATE_RJ{
		Result: result, Source: source, Reason: reason,
	}
	return &pdu
}

func decodeA_ASSOCIATE_RJ(d *Decoder) *A_ASSOCIATE_RJ {
	pdu := &A_ASSOCIATE_RJ{}
	d.Skip(1) // reserved
	pdu.Result = d.DecodeByte()
	pdu.Source = d.DecodeByte()
	pdu.Reason = d.DecodeByte()
	return pdu
}

func (pdu *A_ASSOCIATE_RJ) EncodePayload(e *Encoder) {
	e.EncodeZeros(1)
	e.EncodeByte(pdu.Result)
	e.EncodeByte(pdu.Source)
	e.EncodeByte(pdu.Reason)
}

func (pdu *A_ASSOCIATE_RJ) DebugString() string {
	return "A_ASSOCIATE_RJ"
}

type A_ABORT struct {
	Source byte
	Reason byte
}

func New_A_ABORT(source, reason byte) *A_ABORT {
	pdu := A_ABORT{
		Source: source,
		Reason: reason}
	return &pdu
}

func decodeA_ABORT(d *Decoder) *A_ABORT {
	pdu := &A_ABORT{}
	d.Skip(2)
	pdu.Source = d.DecodeByte()
	pdu.Reason = d.DecodeByte()
	return pdu
}

func (pdu *A_ABORT) EncodePayload(e *Encoder) {
	e.EncodeZeros(2)
	e.EncodeByte(pdu.Source)
	e.EncodeByte(pdu.Reason)
}

func (pdu *A_ABORT) DebugString() string {
	return fmt.Sprintf("A_ABORT{source:%d reason:%d}", pdu.Source, pdu.Reason)
}

type P_DATA_TF struct {
	Items []PresentationDataValueItem
}

func New_P_DATA_TF(items []PresentationDataValueItem) *P_DATA_TF {
	return &P_DATA_TF{Items: items}
}

func (pdu *P_DATA_TF) Decode(d *Decoder) {
	for d.Available() > 0 && d.Error() == nil {
		pdu.Items = append(pdu.Items, DecodePresentationDataValueItem(d))
	}
}

func (pdu *P_DATA_TF) EncodePayload(e *Encoder) {
	for _, item := range pdu.Items {
		item.Encode(e)
	}
}

func (pdu *P_DATA_TF) DebugString() string {
	return "P_DATA_TF"
}

func fillString(v string, length int) string {
	if len(v) > length {
		return v[:16]
	}
	for len(v) < length {
		v += " "
	}
	return v
}
