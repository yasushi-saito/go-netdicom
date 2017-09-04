package dimse

//go:generate ./generate_dimse_messages.py

// Implements message types defined in P3.7.
//
// http://dicom.nema.org/medical/dicom/current/output/pdf/part07.pdf

import (
	"encoding/binary"
	"fmt"
	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-dicom/dicomio"
	"github.com/yasushi-saito/go-netdicom/pdu"
	"v.io/x/lib/vlog"
)

// Common interface for all C-XXX message types.
type DIMSEMessage interface {
	fmt.Stringer // Print human-readable description for debugging.
	Encode(*dicomio.Encoder)
	HasData() bool // Do we expact data P_DATA_TF packets after the command packets?
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
			vlog.VI(2).Infof("Return %v for %s", elem, tag.String())
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
func encodeField(e *dicomio.Encoder, tag dicom.Tag, v interface{}) {
	elem := dicom.DicomElement{
		Tag:   tag,
		Vr:    "", // autodetect
		Vl:    1,
		Value: []interface{}{v},
	}
	dicom.EncodeDataElement(e, &elem)
}

const CommandDataSetTypeNull uint16 = 0x101

// C_STORE_RSP status codes.
// P3.4 GG4-1
const (
	CStoreStatusOutOfResources              uint16 = 0xa700
	CStoreStatusDataSetDoesNotMatchSOPClass uint16 = 0xa900
	CStoreStatusCannotUnderstand            uint16 = 0xc000
)

func ReadDIMSEMessage(d *dicomio.Decoder) DIMSEMessage {
	// A DIMSE message is a sequence of DicomElements, encoded in implicit
	// LE.
	//
	// TODO(saito) make sure that's the case. Where the ref?
	var elems []*dicom.DicomElement
	d.PushTransferSyntax(binary.LittleEndian, dicomio.ImplicitVR)
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
	v := decodeMessageForType(&dd, commandField)
	if dd.err != nil {
		d.SetError(dd.err)
		return nil
	}
	return v
}

func EncodeDIMSEMessage(e *dicomio.Encoder, v DIMSEMessage) {
	// DIMSE messages are always encoded Implicit+LE. See P3.7 6.3.1.
	subEncoder := dicomio.NewEncoder(binary.LittleEndian, dicomio.ImplicitVR)
	v.Encode(subEncoder)
	bytes, err := subEncoder.Finish()
	if err != nil {
		e.SetError(err)
		return
	}
	e.PushTransferSyntax(binary.LittleEndian, dicomio.ImplicitVR)
	defer e.PopTransferSyntax()
	encodeField(e, dicom.TagCommandGroupLength, uint32(len(bytes)))
	e.WriteBytes(bytes)
}

// Helper class for assembling a DIMSE command message and data payload from a
// sequence of P_DATA_TF PDUs.
type CommandAssembler struct {
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
func (a *CommandAssembler) AddDataPDU(pdu *pdu.P_DATA_TF) (byte, DIMSEMessage, []byte, error) {
	for _, item := range pdu.Items {
		if a.contextID == 0 {
			a.contextID = item.ContextID
		} else if a.contextID != item.ContextID {
			return 0, nil, nil, fmt.Errorf("Mixed context: %d %d", a.contextID, item.ContextID)
		}
		if item.Command {
			a.commandBytes = append(a.commandBytes, item.Value...)
			if item.Last {
				if a.readAllCommand {
					return 0, nil, nil, fmt.Errorf("P_DATA_TF: found >1 command chunks with the Last bit set")
				}
				a.readAllCommand = true
			}
		} else {
			a.dataBytes = append(a.dataBytes, item.Value...)
			if item.Last {
				if a.readAllData {
					return 0, nil, nil, fmt.Errorf("P_DATA_TF: found >1 data chunks with the Last bit set")
				}
				a.readAllData = true
			}
		}
	}
	if !a.readAllCommand {
		return 0, nil, nil, nil
	}
	if a.command == nil {
		d := dicomio.NewBytesDecoder(a.commandBytes, nil, dicomio.UnknownVR)
		a.command = ReadDIMSEMessage(d)
		if err := d.Finish(); err != nil {
			return 0, nil, nil, err
		}
	}
	if a.command.HasData() && !a.readAllData {
		return 0, nil, nil, nil
	}
	contextID := a.contextID
	command := a.command
	dataBytes := a.dataBytes
	*a = CommandAssembler{}
	return contextID, command, dataBytes, nil
	// TODO(saito) Verify that there's no unread items after the last command&data.
}
