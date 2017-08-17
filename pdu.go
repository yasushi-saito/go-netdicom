package netdicom

import (
	"fmt"
	"io"
	"log"
	"net"
)

type PDU interface {
	Encode(*Encoder)
	DebugString() string
}

type VariableItem interface {
	Encode(*Encoder)
}

type SubItem struct {
	Type byte // 30H
	// 1 byte reserved
	Length uint16
	Name   string
}

func decodeSubItem(d *Decoder) SubItem {
	v := SubItem{}
	v.Type = d.DecodeByte()
	d.Skip(1)
	v.Length = d.DecodeUint16()
	v.Name = d.DecodeString(int(v.Length))
	return v
}

func (item *SubItem) Encode(e *Encoder) {
	e.EncodeByte(item.Type)
	e.EncodeZeros(1)
	// TODO: handle unicode properly
	e.EncodeUint16(uint16(len(item.Name)))
	e.EncodeBytes([]byte(item.Name))
}

// P3.8 9.3.2.1
type ApplicationContextItem SubItem // Type==10H

// P3.8 9.3.2.2
type PresentationContextItem struct {
	Type byte // 20H
	// 2 bytes reserved
	Length    uint16
	ContextID byte
	// 3 bytes reserved
	Items []SubItem // List of {Abstract,Transfer}SyntaxSubItem
}

func decodePresentationContextItem(d *Decoder) PresentationContextItem {
	v := PresentationContextItem{}
	v.Type = d.DecodeByte()
	d.Skip(2)
	v.Length = d.DecodeUint16()
	v.ContextID = d.DecodeByte()
	return v
}

// P3.8 9.3.2.2.1 & 9.3.2.2.2
type AbstractSyntaxSubItem SubItem // Type=30H
type TransferSyntaxSubItem SubItem // Type=40H

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

func DecodePresentationDataValueItem(d *Decoder) *PresentationDataValueItem {
	item := &PresentationDataValueItem{}
	var ok bool
	item.Length, ok = d.TryDecodeUint32()
	if !ok {
		return nil
	}
	item.ContextID = d.DecodeByte()
	item.Value = d.DecodeBytes(int(item.Length))
	return item
}

func (item *PresentationDataValueItem) Encode(e *Encoder) {
	e.EncodeUint32(item.Length)
	e.EncodeByte(item.ContextID)
	e.EncodeBytes(item.Value)
}

func decodeVariableItems(d *Decoder) []VariableItem {
	// for {
	// 	item := tryDecodeVariableItem(d);
	// 	if item == nil {break}
	// 	pdu.VariableItems = append(pdu.VariableItems, item)
	// }
	return nil
}

type PDUHeader struct {
	Type byte
	// Reserved0 byte
	Length uint32
}

func DecodePDU(in io.Reader) (PDU, error) {
	d := NewDecoder(in)
	if d.err != nil {
		return nil, d.err
	}
	var pdu PDU = nil
	switch d.Type {
	case type_A_ASSOCIATE_RQ:
		pdu = decodeA_ASSOCIATE_RQ(d)
		if d.err != nil {
			return nil, d.err
		}
	case type_A_ASSOCIATE_AC:
		pdu = decodeA_ASSOCIATE_AC(d)
		if d.err != nil {
			return nil, d.err
		}
	case type_A_ASSOCIATE_RJ:
		pdu = decodeA_ASSOCIATE_RJ(d)
		if d.err != nil {
			return nil, d.err
		}
	}
	if pdu == nil {
		// type_P_DATA_TF      = 4
		// type_A_RELEASE_RQ   = 5
		// type_A_RELEASE_RP   = 6
		// type_A_ABORT        = 7
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

// func decodeHeader(d *Decoder, pduType byte) (PDUHeader, length uint32) {
// 	h := PDUHeader{}
// 	h.Type = pduType
// 	d.DecodeByte() // Reserved
// 	h.Length = d.DecodeUint32()
// 	return h
// }

func encodeHeader(header PDUHeader, e *Encoder) {
	e.EncodeByte(header.Type)
	e.EncodeZeros(1) // Reserved
	e.EncodeUint32(header.Length)
}

const (
	type_A_ASSOCIATE_RQ = 1
	type_A_ASSOCIATE_AC = 2
	type_A_ASSOCIATE_RJ = 3
	type_P_DATA_TF      = 4
	type_A_RELEASE_RQ   = 5
	type_A_RELEASE_RP   = 6
	type_A_ABORT        = 7
)

type A_RELEASE_RQ struct {
}

func New_A_RELEASE_RQ() *A_RELEASE_RQ {
	return &A_RELEASE_RQ{}
}

func (pdu *A_RELEASE_RQ) Decode(d *Decoder) {
	d.Skip(4)
}

func (pdu *A_RELEASE_RQ) Encode(e *Encoder) {
	e.SetType(type_A_RELEASE_RQ)
	e.EncodeZeros(4)
}

func (pdu *A_RELEASE_RQ) DebugString() string {
	return fmt.Sprintf("A_RELEASE_RQ(%v)", *pdu)
}

type A_RELEASE_RP struct {
}

func New_A_RELEASE_RP() *A_RELEASE_RP {
	return &A_RELEASE_RP{}
}

func (pdu *A_RELEASE_RP) Decode(d *Decoder) {
	d.Skip(4)
}

func (pdu *A_RELEASE_RP) Encode(e *Encoder) {
	e.SetType(type_A_RELEASE_RP)
	e.EncodeZeros(4)
}

func (pdu *A_RELEASE_RP) DebugString() string {
	return fmt.Sprintf("A_RELEASE_RP(%v)", *pdu)
}

type A_ASSOCIATE_RQ struct {
	ProtocolVersion uint16
	// Reserved1 uint16
	CalledAeTitle  string
	CallingAeTitle string
	VariableItems  []VariableItem
}

func New_A_ASSOCIATE_RQ(params SessionParams) *A_ASSOCIATE_RQ {
	pdu := A_ASSOCIATE_RQ{}
	return &pdu
}

func decodeA_ASSOCIATE_RQ(d *Decoder) *A_ASSOCIATE_RQ {
	pdu := &A_ASSOCIATE_RQ{}
	pdu.ProtocolVersion = d.DecodeUint16()
	d.Skip(2) // Reserved
	pdu.CalledAeTitle = d.DecodeString(16)
	pdu.CallingAeTitle = d.DecodeString(16)
	d.Skip(8 * 4)
	pdu.VariableItems = decodeVariableItems(d)
	return pdu
}

func (pdu *A_ASSOCIATE_RQ) Encode(e *Encoder) {
	e.SetType(type_A_ASSOCIATE_RQ)
	e.EncodeUint16(pdu.ProtocolVersion)
	e.EncodeZeros(2) // Reserved
	e.EncodeString(fillString(pdu.CalledAeTitle, 16))
	e.EncodeString(fillString(pdu.CallingAeTitle, 16))
	e.EncodeZeros(8 * 4)
	e.EncodeUint32(0) // TODO
}

func (pdu *A_ASSOCIATE_RQ) DebugString() string {
	return fmt.Sprintf("A_ASSOCIATE_RQ(%v)", *pdu)
}

type A_ASSOCIATE_AC struct {
	ProtocolVersion uint16
	// Reserved1 uint16
	ReservedAET   string
	ReservedAEC   string
	VariableItems []VariableItem
}

func decodeA_ASSOCIATE_AC(d *Decoder) *A_ASSOCIATE_AC {
	pdu := &A_ASSOCIATE_AC{}
	pdu.ProtocolVersion = d.DecodeUint16()
	d.Skip(2) // Reserved
	pdu.ReservedAET = d.DecodeString(16)
	pdu.ReservedAEC = d.DecodeString(16)
	d.Skip(8 * 4)
	pdu.VariableItems = decodeVariableItems(d)
	return pdu
}

func (pdu *A_ASSOCIATE_AC) Encode(e *Encoder) {
	e.SetType(type_A_ASSOCIATE_RQ)
	e.EncodeUint16(pdu.ProtocolVersion)
	e.EncodeZeros(2) // Reserved
	e.EncodeString(fillString(pdu.ReservedAET, 16))
	e.EncodeString(fillString(pdu.ReservedAEC, 16))
	e.EncodeZeros(8 * 4)
	for _, item := range pdu.VariableItems {
		item.Encode(e)
	}
}

func (pdu *A_ASSOCIATE_AC) DebugString() string {
	return "A_ASSOCIATE_AC"
}

type A_ASSOCIATE_RJ struct {
	Header           PDUHeader
	Result           byte
	Source           byte
	ReasonDiagnostic byte
}

func New_A_ASSOCIATE_RJ(params SessionParams) *A_ASSOCIATE_RJ {
	pdu := A_ASSOCIATE_RJ{}
	return &pdu
}

func decodeA_ASSOCIATE_RJ(d *Decoder) *A_ASSOCIATE_RJ {
	pdu := &A_ASSOCIATE_RJ{}
	d.Skip(1) // reserved
	pdu.Result = d.DecodeByte()
	pdu.Source = d.DecodeByte()
	pdu.ReasonDiagnostic = d.DecodeByte()
	return pdu
}

func (pdu *A_ASSOCIATE_RJ) Encode(e *Encoder) {
	e.SetType(type_A_ASSOCIATE_RJ)
	e.EncodeZeros(1)
	e.EncodeByte(pdu.Result)
	e.EncodeByte(pdu.Source)
	e.EncodeByte(pdu.ReasonDiagnostic)
}

func (pdu *A_ASSOCIATE_RJ) DebugString() string {
	return "A_ASSOCIATE_RJ"
}

type A_ABORT struct {
	Source           byte
	ReasonDiagnostic byte
}

func New_A_ABORT(source, reasonDiagnostic byte) *A_ABORT {
	pdu := A_ABORT{
		Source:           source,
		ReasonDiagnostic: reasonDiagnostic}
	return &pdu
}

func (pdu *A_ABORT) Decode(d *Decoder) {
	d.Skip(2) // two bytes reserved
	pdu.Source = d.DecodeByte()
	pdu.ReasonDiagnostic = d.DecodeByte()
}

func (pdu *A_ABORT) Encode(e *Encoder) {
	e.SetType(type_A_ABORT)
	e.EncodeZeros(2)
	e.EncodeByte(pdu.Source)
	e.EncodeByte(pdu.ReasonDiagnostic)
}

func (pdu *A_ABORT) DebugString() string {
	return "A_ABORT"
}

type P_DATA_TF struct {
	Items []PresentationDataValueItem
}

func New_P_DATA_TF(items []PresentationDataValueItem) *P_DATA_TF {
	return &P_DATA_TF{Items: items}
}

func (pdu *P_DATA_TF) Decode(d *Decoder) {
	for d.err == nil {
		item := DecodePresentationDataValueItem(d)
		if item == nil {
			break
		}
		pdu.Items = append(pdu.Items, *item)
	}
}

func (pdu *P_DATA_TF) Encode(e *Encoder) {
	e.SetType(type_P_DATA_TF)
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

type networkConnectedPDU struct {
	Conn net.Conn
}

func (pdu *networkConnectedPDU) Encode(e *Encoder) { panic("Not implemented") }

type networkDisconnectedPDU struct {
	Err error
}

func (pdu *networkDisconnectedPDU) Encode(e *Encoder) { panic("Not implemented") }

type malformedPDU struct {
	Err error
}

func (pdu *malformedPDU) Encode(e *Encoder) { panic("Not implemented") }
