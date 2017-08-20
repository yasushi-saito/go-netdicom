package netdicom

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
)

type Encoder struct {
	err     error
	buf     *bytes.Buffer
}

func NewEncoder() *Encoder {
	e := &Encoder{}
	e.buf = &bytes.Buffer{}
	return e
}

func (e *Encoder) SetError(err error) {
	if e.err == nil {
		e.err = err
	}
}

func (e *Encoder) Finish() ([]byte, error) {
	return e.buf.Bytes(), e.err
}

func (e *Encoder) EncodeByte(v byte) {
	binary.Write(e.buf, binary.BigEndian, &v)
}

func (e *Encoder) EncodeUint16(v uint16) {
	binary.Write(e.buf, binary.BigEndian, &v)
}

func (e *Encoder) EncodeUint32(v uint32) {
	binary.Write(e.buf, binary.BigEndian, &v)
}

func (e *Encoder) EncodeString(v string) {
	e.buf.Write([]byte(v))
}

func (e *Encoder) EncodeZeros(len int) {
	// TODO(saito) reuse the buffer!
	zeros := make([]byte, len)
	e.buf.Write(zeros)
}

func (e *Encoder) EncodeBytes(v []byte) {
	e.buf.Write(v)
}

type Decoder struct {
	in        io.Reader
	err       error

	Type byte
	// 1 byte reserved
	Length uint32

	// Cumulative # bytes read.
	pos    int
	// Max bytes to read. PushLimit() will add a new limit, and PopLimit()
	// will restore the old limit. The newest limit is at the end.
	//
	// INVARIANT: limits[] store values in decreasing order.
	limits []int
}

func (e *Decoder) SetError(err error) {
	if e.err == nil {
		e.err = err
	}
}

func (d *Decoder) PushLimit(limit int) {
	d.limits = append(d.limits, d.pos + limit)
}

func (d *Decoder) PopLimit() {
	d.limits = d.limits[:len(d.limits)-1]
}

func NewDecoder(in io.Reader) *Decoder {
	d := &Decoder{}
	d.err = binary.Read(in, binary.BigEndian, &d.Type)
	if d.err != nil {
		return d
	}
	var skip byte
	d.err = binary.Read(in, binary.BigEndian, &skip)
	if d.err != nil {
		return d
	}
	d.err = binary.Read(in, binary.BigEndian, &d.Length)
	log.Printf("Header: %v %v", d.Type, d.Length)
	d.in = in
	d.PushLimit(int(d.Length))
	log.Printf("NewDecoder: type=%d, length=%d", d.Type, d.Length)
	return d
}

func (d *Decoder) Error() error { return d.err }

func (d *Decoder) Finish() error {
	if d.err != nil {
		return d.err
	}
	if d.Available() != 0 {
		return fmt.Errorf("Decoder found junk (%d bytes remaining)", d.Available())
	}
	return nil
}

// io.Reader implementation
func (d *Decoder) Read(p []byte) (int, error) {
	desired := d.Available()
	var eof error
	if desired < len(p) {
		p = p[:desired]
		desired = len(p)
		// Read less than requested, so this call should result at least
		// in EOF error. Remember that fact.
		eof = io.EOF
	}
	n, err := d.in.Read(p)
	if err == nil {
		d.pos += n
		err = eof
	}
	return n, err
}

func (d *Decoder) Available() int {
	return d.limits[len(d.limits)-1] - d.pos
}

func (d *Decoder) DecodeByte() (v byte) {
	err := binary.Read(d, binary.BigEndian, &v)
	if err != nil {
		d.err = err
		return 0
	}
	return v
}

func (d *Decoder) DecodeUint32() (v uint32) {
	err := binary.Read(d, binary.BigEndian, &v)
	if err != nil {
		d.err = err
	}
	return v
}

func (d *Decoder) DecodeUint16() (v uint16) {
	err := binary.Read(d, binary.BigEndian, &v)
	if err != nil {
		d.err = err
	}
	return v
}

func (d *Decoder) DecodeString(length int) string {
	return string(d.DecodeBytes(length))
}

func (d *Decoder) DecodeBytes(length int) []byte {
	v := make([]byte, length)
	n, err := d.Read(v)
	if err != nil {
		d.err = err
	}
	if n != length {
		panic("XXXXXXXXZZZ")
		d.err = fmt.Errorf("DecodeBytes: %d <-> %d", n, length)
	}
	return v
}

func (d *Decoder) Skip(bytes int) {
	junk := make([]byte, bytes)
	n, err := d.Read(junk)
	if err != nil {
		d.err = err
		return
	}
	if n != bytes {
		d.err = fmt.Errorf("Failed to skip %d bytes (read %d bytes instead)", bytes, n)
		return
	}
}
