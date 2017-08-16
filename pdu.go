package netdicom

type PDU interface {
	Encode(*Encoder)
	Decode(*Decoder)
}

type Decoder struct {
	err error
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

func (d *Decoder) DecodeString16() string {
	return ""
}

func (d *Decoder) skip(bytes int) {
}

type Encoder struct {
	err error
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

type PDUHeader struct {
	Type byte
	// Reserved0 byte
	Length       uint32
}

func decodeHeader(d *Decoder, pduType byte) PDUHeader {
	h := PDUHeader{}
	h.Type = pduType
	d.DecodeByte() // Reserved
	h.Length = d.DecodeUint32()
	return h
}

func encodeHeader(header PDUHeader, e *Encoder) {
	e.EncodeByte(header.Type)
	e.EncodeZeros(1) // Reserved
	e.EncodeUint32(header.Length)
}

type A_ASSOCIATE_RQ struct {
	Header         PDUHeader
	ProtocolVersion uint16
	// Reserved1 uint16
	CalledAeTitle  string
	CallingAeTitle string
	VariableItems  uint32
}

func New_A_ASSOCIATE_RQ(params SessionParams) *A_ASSOCIATE_RQ {
	pdu := A_ASSOCIATE_RQ{}
	return &pdu
}

func (pdu *A_ASSOCIATE_RQ) Decode(d *Decoder) {
	pdu.Header = decodeHeader(d, 1)
	pdu.ProtocolVersion = d.DecodeUint16()
	d.DecodeUint16() // Reserved
	pdu.CalledAeTitle = d.DecodeString16()
	pdu.CallingAeTitle = d.DecodeString16()
	d.skip(8 * 4)
	pdu.VariableItems = d.DecodeUint32()
}

func (pdu *A_ASSOCIATE_RQ) Encode(e *Encoder) {
	encodeHeader(pdu.Header, e)
	e.EncodeUint16(pdu.ProtocolVersion)
	e.EncodeZeros(2) // Reserved
	e.EncodeString(fillString(pdu.CalledAeTitle, 16))
	e.EncodeString(fillString(pdu.CallingAeTitle, 16))
	e.EncodeZeros(8 * 4)
	e.EncodeUint32(pdu.VariableItems)
}

type A_ASSOCIATE_RJ struct {
	Header         PDUHeader
	Result byte
	Source byte
	ReasonDiagnostic byte
}

func New_A_ASSOCIATE_RJ(params SessionParams) *A_ASSOCIATE_RJ {
	pdu := A_ASSOCIATE_RJ{}
	return &pdu
}

func (pdu *A_ASSOCIATE_RJ) Decode(d *Decoder) {
	pdu.Header = decodeHeader(d, 3)
	d.skip(1)  // reserved
	pdu.Result = d.DecodeByte()
	pdu.Source = d.DecodeByte()
	pdu.ReasonDiagnostic = d.DecodeByte()
}

func (pdu *A_ASSOCIATE_RJ) Encode(e *Encoder) {
	encodeHeader(pdu.Header, e)
	e.EncodeZeros(1)
	e.EncodeByte(pdu.Result)
	e.EncodeByte(pdu.Source)
	e.EncodeByte(pdu.ReasonDiagnostic)
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
