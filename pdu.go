package netdicom

import (
	"net"
	"encoding/binary"
	"io"
)

type PDU interface {
	Encode(*Encoder)
}

type VariableItem interface {
	Encode(*Encoder)
}

type Decoder struct {
	byteOrder binary.ByteOrder
	implicit  bool
	in        io.Reader

	Type   byte
	Length uint32
	Err    error
}

func NewDecoder(
	byteOrder binary.ByteOrder,
	implicit bool,
	in io.Reader) *Decoder {
	d := &Decoder{}

	d.byteOrder = byteOrder
	d.implicit = implicit
	d.Err = binary.Read(in, d.byteOrder, &d.Type)
	if d.Err != nil {
		return d
	}
	d.Err = binary.Read(in, d.byteOrder, &d.Length)
	d.in = io.LimitReader(in, int64(d.Length))
	return d
}

func (d *Decoder) TryDecodeUint32() (uint32, bool) {
	return 0, false
}

func (d *Decoder) DecodeByte() byte {
	return 0
}

func (d *Decoder) DecodeUint32() uint32 {
	return 0
}

func (d *Decoder) DecodeUint16() uint16 {
	return 0
}

func (d *Decoder) DecodeString(length int32) string {
	return ""
}

func (d *Decoder) DecodeBytes(length int32) []byte {
	return nil
}

func (d *Decoder) skip(bytes int) {
}

type Encoder struct {
	pduType byte
	Err     error
}

func (e *Encoder) SetType(t byte) {
	e.pduType = t
}

func (e *Encoder) EncodeByte(v byte) {
}

func (e *Encoder) EncodeUint16(v uint16) {
}

func (e *Encoder) EncodeUint32(v uint32) {
}

func (e *Encoder) EncodeString(v string) {
}

func (e *Encoder) EncodeZeros(bytes int) {
}

func (e *Encoder) EncodeBytes(v []byte) {
	panic("aoue")
}

type PresentationDataValueItem struct {
	Length    uint32
	ContextID byte
	Value     []byte
}

func NewPresentationDataValueItem(contextID byte, value[]byte) PresentationDataValueItem {
	return PresentationDataValueItem{
		Length: uint32(1 + len(value)),
		ContextID: contextID,
		Value: value,
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
	item.Value = d.DecodeBytes(int32(item.Length))
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
	d := NewDecoder(binary.LittleEndian, true, in)
	if d.Err != nil {
		return nil, d.Err
	}

	switch d.Type {
	case type_A_ASSOCIATE_RQ:
		pdu := decodeA_ASSOCIATE_RQ(d)
		if d.Err != nil {
			return nil, d.Err
		}
		return pdu, nil
	// case type_A_ASSOCIATE_AC:
	// 	pdu := decodeu_ASSOCIATE_AC(d)
	// 	if d.Err != nil { return nil, d.Err }
	// 	return pdu
	case type_A_ASSOCIATE_RJ:
		pdu := decodeA_ASSOCIATE_RJ(d)
		if d.Err != nil {
			return nil, d.Err
		}
		return pdu, nil
	}
	// type_P_DATA_TF      = 4
	// type_A_RELEASE_RQ   = 5
	// type_A_RELEASE_RP   = 6
	// type_A_ABORT        = 7
	panic("aoeu")
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
	d.skip(4)
}

func (pdu *A_RELEASE_RQ) Encode(e *Encoder) {
	e.SetType(type_A_RELEASE_RQ)
	e.EncodeZeros(4)
}

type A_RELEASE_RP struct {
}

func New_A_RELEASE_RP() *A_RELEASE_RP {
	return &A_RELEASE_RP{}
}

func (pdu *A_RELEASE_RP) Decode(d *Decoder) {
	d.skip(4)
}

func (pdu *A_RELEASE_RP) Encode(e *Encoder) {
	e.SetType(type_A_RELEASE_RP)
	e.EncodeZeros(4)
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
	d.skip(2) // Reserved
	pdu.CalledAeTitle = d.DecodeString(16)
	pdu.CallingAeTitle = d.DecodeString(16)
	d.skip(8 * 4)
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

type ApplicationContextItem struct {
	Type byte
	// 1 byte reserved
	Length uint16
	Name   string
}

func (item *ApplicationContextItem) Encode(e *Encoder) {
	e.EncodeByte(item.Type)
	e.EncodeZeros(1)

	// TODO: handle unicode properly
	e.EncodeUint16(uint16(len(item.Name)))
	e.EncodeBytes([]byte(item.Name))
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
	d.skip(2) // Reserved
	pdu.ReservedAET = d.DecodeString(16)
	pdu.ReservedAEC = d.DecodeString(16)
	d.skip(8 * 4)
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
	d.skip(1) // reserved
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
	d.skip(2) // two bytes reserved
	pdu.Source = d.DecodeByte()
	pdu.ReasonDiagnostic = d.DecodeByte()
}

func (pdu *A_ABORT) Encode(e *Encoder) {
	e.SetType(type_A_ABORT)
	e.EncodeZeros(2)
	e.EncodeByte(pdu.Source)
	e.EncodeByte(pdu.ReasonDiagnostic)
}

type P_DATA_TF struct {
	Items []PresentationDataValueItem
}

func New_P_DATA_TF(items []PresentationDataValueItem) *P_DATA_TF {
	return &P_DATA_TF{Items: items}
}

func (pdu *P_DATA_TF) Decode(d *Decoder) {
	for d.Err == nil {
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

func (pdu *networkConnectedPDU) Encode(e*Encoder) {panic("Not implemented")}

type networkDisconnectedPDU struct {
	Err error
}

func (pdu *networkDisconnectedPDU) Encode(e*Encoder) {panic("Not implemented")}

type malformedPDU struct {
	Err error
}

func (pdu *malformedPDU) Encode(e*Encoder) {panic("Not implemented")}
