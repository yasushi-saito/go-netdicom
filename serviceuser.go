package netdicom

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/golang/glog"
	"github.com/yasushi-saito/go-dicom"
	"sync/atomic"
)

type serviceUserStatus int

const (
	serviceUserInitial = iota
	serviceUserAssociationActive
	serviceUserClosed
)

type ServiceUser struct {
	status serviceUserStatus
	// sm *StateMachine
	// associationActive bool
	downcallCh    chan stateEvent
	upcallCh      chan upcallEvent
	nextMessageID int32
}

type ServiceUserParams struct {
	CalledAETitle  string // Must be nonempty
	CallingAETitle string // Must be nonempty

	// List of SOPUIDs wanted by the user.
	RequiredServices []SOPUID

	// List of Transfer syntaxes supported by the user.  If you know the
	// transer syntax of the file you are going to copy, set that here.
	// Otherwise, you'll need to re-encode the data w/ the given transfer
	// syntax yourself.
	//
	// TODO(saito) Support reencoding internally on C_STORE, etc. The DICOM
	// spec is particularly moronic here, since we could just have specified
	// the transfer syntax per data sent.
	SupportedTransferSyntaxes []string

	// Max size of a message chunk (PDU) that the client can receiuve.  If
	// <= 0, DefaultMaxPDUSize is used.
	MaxPDUSize int
}

// If transferSyntaxUIDs is empty, the standard list of syntax is used.
func NewServiceUserParams(
	calledAETitle string,
	callingAETitle string,
	requiredServices []SOPUID,
	transferSyntaxUIDs []string) ServiceUserParams {
	if len(transferSyntaxUIDs) == 0 {
		transferSyntaxUIDs = dicom.StandardTransferSyntaxes
	} else {
		canonical := make([]string, len(transferSyntaxUIDs))
		for i, uid := range transferSyntaxUIDs {
			var err error
			canonical[i], err = dicom.CanonicalTransferSyntaxUID(uid)
			if err != nil {
				glog.Fatal(err) // TODO(saito)
			}
		}
		transferSyntaxUIDs = canonical
	}
	return ServiceUserParams{
		CalledAETitle:             calledAETitle,
		CallingAETitle:            callingAETitle,
		RequiredServices:          requiredServices,
		SupportedTransferSyntaxes: transferSyntaxUIDs,
		MaxPDUSize:                1 << 20,
	}
}

func NewServiceUser(serverAddr string, params ServiceUserParams) *ServiceUser {
	su := &ServiceUser{
		status: serviceUserInitial,
		// sm: NewStateMachineForServiceUser(params, nil, nil),
		downcallCh:    make(chan stateEvent, 128),
		upcallCh:      make(chan upcallEvent, 128),
		nextMessageID: 123, // any value != 0 suffices.
	}
	go runStateMachineForServiceUser(serverAddr, params, su.upcallCh, su.downcallCh)
	return su
}

func waitAssociationEstablishment(su *ServiceUser) error {
	for su.status < serviceUserAssociationActive {
		event, ok := <-su.upcallCh
		if !ok {
			su.status = serviceUserClosed
			break
		}
		if event.eventType == upcallEventHandshakeCompleted {
			su.status = serviceUserAssociationActive
			break
		}
		glog.Fatalf("Illegal upcall event during handshake: %v", event)
	}
	if su.status != serviceUserAssociationActive {
		return fmt.Errorf("Connection failed")
	}
	return nil
}

// Generate a new message ID that's unique within the "su".
func newMessageID(su *ServiceUser) uint16 {
	id := atomic.AddInt32(&su.nextMessageID, 1)
	return uint16(id % 0x10000)
}

// Issue a C-STORE request; blocks until the server responds, or an error
// happens. "data" is a DICOM file. Its transfer syntax must match the one
// established in during DICOM A_ASSOCIATE handshake.
//
// TODO(saito) Re-encode the data using the valid transfer syntax.
func (su *ServiceUser) CStore(data []byte) error {
	// Parse the beginning of file, extract syntax UIDs to fill in the
	// C-STORE request.
	decoder := dicom.NewDecoder(
		bytes.NewBuffer(data),
		int64(len(data)),
		binary.LittleEndian,
		dicom.ExplicitVR)
	meta := dicom.ParseFileHeader(decoder)
	if decoder.Error() != nil {
		return decoder.Error()
	}
	var getElement = func(meta []dicom.DicomElement, tag dicom.Tag) (string, error) {
		elem, err := dicom.LookupElementByTag(meta, tag)
		if err != nil {
			return "", fmt.Errorf("C-STORE data lacks %s: %v", tag.String(), err)
		}
		s, err := elem.GetString()
		if err != nil {
			return "", err
		}
		return s, nil
	}
	sopInstanceUID, err := getElement(meta, dicom.TagMediaStorageSOPInstanceUID)
	if err != nil {
		return fmt.Errorf("C-STORE data lacks SOPInstanceUID: %v", err)
	}
	transferSyntaxUID, err := getElement(meta, dicom.TagTransferSyntaxUID)
	if err != nil {
		return fmt.Errorf("C-STORE data lacks TransferSyntaxUID: %v", err)
	}
	sopClassUID, err := getElement(meta, dicom.TagMediaStorageSOPClassUID)
	if err != nil {
		return fmt.Errorf("C-STORE data lacks MediaStorageSOPClassUID: %v", err)
	}
	glog.V(1).Infof("DICOM transfersyntax:%s, abstractsyntax: %s, sopinstance: %s",
		transferSyntaxUID, sopClassUID, sopInstanceUID)

	// The remainder of the file becomes the actual C-STORE payload.
	body := decoder.ReadBytes(int(decoder.Len()))
	if decoder.Error() != nil {
		return decoder.Error()
	}
	err = waitAssociationEstablishment(su)
	if err != nil {
		return err
	}
	e := dicom.NewEncoder(nil, dicom.UnknownVR)
	EncodeDIMSEMessage(e, &C_STORE_RQ{
		AffectedSOPClassUID:    sopClassUID,
		MessageID:              newMessageID(su),
		CommandDataSetType:     1, // anything other than 0x101 suffices.
		AffectedSOPInstanceUID: sopInstanceUID,
	})
	req, err := e.Finish()
	if err != nil {
		return err
	}
	su.downcallCh <- stateEvent{
		event: evt09,
		dataPayload: &stateEventDataPayload{abstractSyntaxName: sopClassUID,
			command: true,
			data:    req}}
	su.downcallCh <- stateEvent{
		event: evt09,
		dataPayload: &stateEventDataPayload{abstractSyntaxName: sopClassUID,
			command: false,
			data:    body}}
	for {
		event, ok := <-su.upcallCh
		if !ok {
			su.status = serviceUserClosed
			return fmt.Errorf("Connection closed while waiting for cstore response")
		}
		doassert(event.eventType == upcallEventData)
		doassert(event.command != nil)
		resp, ok := event.command.(*C_STORE_RSP)
		doassert(ok) // TODO(saito)
		if resp.Status != 0 {
			return fmt.Errorf("C_STORE failed: %v", resp.String())
		}
		return nil
	}
	panic("should not reach here")
}

func (su *ServiceUser) Release() {
	err := waitAssociationEstablishment(su)
	if err != nil {
		return
	}
	su.downcallCh <- stateEvent{event: evt11}
	for {
		event, ok := <-su.upcallCh
		if !ok {
			su.status = serviceUserClosed
			break
		}
		glog.Fatalf("No event expected after release, but received %v", event)
	}
}
