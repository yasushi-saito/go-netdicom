package netdicom

import (
	"fmt"
	"bytes"
	"net"
	"encoding/binary"
	"io"
)

type PDU interface {
	Encode(*Encoder)
	DebugString() string
}

type VariableItem interface {
	Encode(*Encoder)
}

type Decoder struct {
	byteOrder binary.ByteOrder
	implicit  bool
	in        io.Reader
	err    error

	Type   byte
	// 1 byte reserved
	Length uint32
}

func NewDecoder(
	byteOrder binary.ByteOrder,
	implicit bool,
	in io.Reader) *Decoder {
	d := &Decoder{}
	d.byteOrder = byteOrder
	d.implicit = implicit
	d.err = binary.Read(in, d.byteOrder, &d.Type)
	if d.err != nil {
		return d
	}
	var skip byte
	d.err = binary.Read(in, d.byteOrder, &skip)
	if d.err != nil {
		return d
	}
	d.err = binary.Read(in, d.byteOrder, &d.Length)
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

func (d *Decoder) Skip(bytes int) {
	junk := make([]byte, bytes)
	n, err := d.in.Read(junk)
	if err != nil {
		d.err = err
		return
	}
	if n != bytes {
		d.err = fmt.Errorf("Failed to skip %d bytes (read %d bytes instead)", bytes, n)
		return
	}
}

type Encoder struct {
	byteOrder binary.ByteOrder
	implicit  bool

	pduType byte
	err     error
	buf *bytes.Buffer
}

func NewEncoder() *Encoder {
	e := &Encoder{}
	e.byteOrder = binary.LittleEndian
	e.buf = &bytes.Buffer{}
	return e
}

func (e *Encoder) Finish() ([]byte, error) {
	if e.pduType == 0 {
		panic("pduType not set")
	}
	// Reserve the header bytes. It will be filled in Finish.
	header := make([]byte, 6) // First 6 bytes of buf.
	header[0] = e.pduType
	header[1] = 0  // Reserved.
	e.byteOrder.PutUint32(header[2:6], uint32(e.buf.Len() - 6))

	data := append(header, e.buf.Bytes()...)
	return data, e.err
}

func (e *Encoder) SetType(t byte) {
	e.pduType = t
}

func (e *Encoder) EncodeByte(v byte) {
	binary.Write(e.buf, e.byteOrder, &v)
}

func (e *Encoder) EncodeUint16(v uint16) {
	binary.Write(e.buf, e.byteOrder, &v)
}

func (e *Encoder) EncodeUint32(v uint32) {
	binary.Write(e.buf, e.byteOrder, &v)
}

func (e *Encoder) EncodeString(v string) {
	e.buf.Write([]byte(v))
}

func (e *Encoder) EncodeZeros(bytes int) {
	e.buf.Next(bytes)
}

func (e *Encoder) EncodeBytes(v []byte) {
	e.buf.Write(v)
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
	if d.err != nil {
		return nil, d.err
	}

	switch d.Type {
	case type_A_ASSOCIATE_RQ:
		pdu := decodeA_ASSOCIATE_RQ(d)
		if d.err != nil {
			return nil, d.err
		}
		return pdu, nil
	case type_A_ASSOCIATE_AC:
		pdu := decodeA_ASSOCIATE_AC(d)
		if d.err != nil { return nil, d.err }
		return pdu, nil
	case type_A_ASSOCIATE_RJ:
		pdu := decodeA_ASSOCIATE_RJ(d)
		if d.err != nil {
			return nil, d.err
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
	d.Skip(4)
}

func (pdu *A_RELEASE_RQ) Encode(e *Encoder) {
	e.SetType(type_A_RELEASE_RQ)
	e.EncodeZeros(4)
}

func (pdu *A_RELEASE_RQ) DebugString() string {
	return "A_RELEASE_RQ"
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
	return "A_RELEASE_RP"
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
	return "A_ASSOCIATE_RQ"
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

func (pdu *networkConnectedPDU) Encode(e*Encoder) {panic("Not implemented")}

type networkDisconnectedPDU struct {
	Err error
}

func (pdu *networkDisconnectedPDU) Encode(e*Encoder) {panic("Not implemented")}

type malformedPDU struct {
	Err error
}

func (pdu *malformedPDU) Encode(e*Encoder) {panic("Not implemented")}
