package netdicom

// Implements message types defined in P3.7.
//
// http://dicom.nema.org/medical/dicom/current/output/pdf/part07.pdf

// Fields common to all DIMSE messages.
type DIMSEMessageHeader struct {
	CommandGroupLength  uint32 // (0000,0000)
	AffectedSOPClassUID string // (0000,0002)
	CommandField        uint16 // (0000,0100)
}

// P3.7 9.3.1.1
type C_STORE_RQ struct {
	DIMSEMessageHeader
	MessageID                            uint16 // (0000,0110)
	Priority                             uint16 // (0000,0110)
	CommandDataSetType                   uint16 // (0000, 0800)
	AffectedSOPInstanceUID               string // (0000,1000)
	MoveOriginatorApplicationEntityTitle string // (0000,1030)
	MoveOriginatorMessageID              uint16 // (0000,1031)
}

// P3.7 9.3.1.2
type C_STORE_RSP struct {
	DIMSEMessageHeader
	MessageIDBeingRespondedTo uint16 // (0000,0120)
	CommandDataSetType        uint16 // (0000, 0800)
	Status                    uint16 // (0000,0900)
	AffectedSOPInstanceUID    string // (0000,1000)
}

type DIMSEParser struct {
}

func AddCommandItem(p *DIMSEParser, data []byte, last bool) {

}
