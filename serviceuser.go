package netdicom

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"

	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-dicom/dicomio"
	"github.com/yasushi-saito/go-dicom/dicomuid"
	"github.com/yasushi-saito/go-netdicom/dimse"
	"github.com/yasushi-saito/go-netdicom/sopclass"
	"v.io/x/lib/vlog"
)

type serviceUserStatus int

const (
	serviceUserInitial = iota
	serviceUserHandshaking
	serviceUserAssociationActive
	serviceUserClosed
)

// Encapsulates the state for DICOM client (user).
type ServiceUser struct {
	status     serviceUserStatus
	downcallCh chan stateEvent
	upcallCh   chan upcallEvent
	cm         *contextManager // Set only after the handshake completes.
}

type ServiceUserParams struct {
	CalledAETitle  string // Must be nonempty
	CallingAETitle string // Must be nonempty

	// List of SOPUIDs wanted by the user.
	RequiredServices []sopclass.SOPUID

	// List of Transfer syntaxes supported by the user.  If you know the
	// transer syntax of the file you are going to copy, set that here.
	// Otherwise, you'll need to re-encode the data w/ the given transfer
	// syntax yourself.
	//
	// TODO(saito) Support reencoding internally on C_STORE, etc. The DICOM
	// spec is particularly moronic here, since we could just have specified
	// the transfer syntax per data sent.
	SupportedTransferSyntaxes []string
}

// If transferSyntaxUIDs is empty, the standard list of syntax is used.
func NewServiceUserParams(
	calledAETitle string,
	callingAETitle string,
	requiredServices []sopclass.SOPUID,
	transferSyntaxUIDs []string) (ServiceUserParams, error) {
	if calledAETitle == "" {
		return ServiceUserParams{}, errors.New("NewServiceUSerParams: Empty calledAETitle")
	}
	if callingAETitle == "" {
		return ServiceUserParams{}, errors.New("NewServiceUSerParams: Empty callingAETitle")
	}
	if len(transferSyntaxUIDs) == 0 {
		transferSyntaxUIDs = dicomio.StandardTransferSyntaxes
	} else {
		for i, uid := range transferSyntaxUIDs {
			canonicalUID, err := dicomio.CanonicalTransferSyntaxUID(uid)
			if err != nil {
				return ServiceUserParams{}, err
			}
			transferSyntaxUIDs[i] = canonicalUID
		}
	}
	return ServiceUserParams{
		CalledAETitle:             calledAETitle,
		CallingAETitle:            callingAETitle,
		RequiredServices:          requiredServices,
		SupportedTransferSyntaxes: transferSyntaxUIDs,
	}, nil
}

func NewServiceUser(params ServiceUserParams) *ServiceUser {
	su := &ServiceUser{
		status: serviceUserInitial,
		// sm: NewStateMachineForServiceUser(params, nil, nil),
		downcallCh: make(chan stateEvent, 128),
		upcallCh:   make(chan upcallEvent, 128),
	}
	go runStateMachineForServiceUser(params, su.upcallCh, su.downcallCh)
	return su
}

func waitAssociationEstablishment(su *ServiceUser) error {
	if su.status < serviceUserHandshaking {
		vlog.Fatal("ServiceUser.Start() not yet called")
	}
	for su.status < serviceUserAssociationActive {
		event, ok := <-su.upcallCh
		if !ok {
			su.status = serviceUserClosed
			su.cm = nil
			break
		}
		if event.eventType == upcallEventHandshakeCompleted {
			su.status = serviceUserAssociationActive
			su.cm = event.cm
			doassert(su.cm != nil)
			break
		}
		vlog.Fatalf("Illegal upcall event during handshake: %v", event)
	}
	if su.status != serviceUserAssociationActive {
		return fmt.Errorf("Connection failed")
	}
	return nil
}

// Connect to the server at the given "host:port". Either Connect or SetConn
// must be before calling CStore, etc.
func (su *ServiceUser) Connect(serverAddr string) {
	doassert(su.status == serviceUserInitial)
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		vlog.Infof("Connect(%s): %v", serverAddr, err)
		su.downcallCh <- stateEvent{event: evt17, pdu: nil, err: err}
		close(su.downcallCh)
	} else {
		su.downcallCh <- stateEvent{event: evt02, pdu: nil, err: nil, conn: conn}
	}
	su.status = serviceUserHandshaking
}

// Use the given connection to talk to the server. Either Connect or SetConn
// must be before calling CStore, etc.
func (su *ServiceUser) SetConn(conn net.Conn) {
	doassert(su.status == serviceUserInitial)
	su.downcallCh <- stateEvent{event: evt02, pdu: nil, err: nil, conn: conn}
	su.status = serviceUserHandshaking
}

// Issue a C-STORE request; blocks until the server responds, or an error
// happens. "data" is a DICOM file. Its transfer syntax must match the one
// established in during DICOM A_ASSOCIATE handshake.
//
// TODO(saito) Re-encode the data using the valid transfer syntax.
//
//
// TODO(saito) Remove this function. Use CStore instead.
func (su *ServiceUser) CStoreRaw(data []byte) error {
	// Parse the beginning of file, extract syntax UIDs to fill in the
	// C-STORE request.
	decoder := dicomio.NewDecoder(
		bytes.NewBuffer(data),
		int64(len(data)),
		binary.LittleEndian,
		dicomio.ExplicitVR)
	meta := dicom.ParseFileHeader(decoder)
	if decoder.Error() != nil {
		return decoder.Error()
	}
	var getElement = func(meta []*dicom.Element, tag dicom.Tag) (string, error) {
		elem, err := dicom.FindElementByTag(meta, tag)
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
	vlog.VI(1).Infof("DICOM transfersyntax:%s, abstractsyntax: %s, sopinstance: %s",
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
	e := dicomio.NewBytesEncoder(nil, dicomio.UnknownVR)
	dimse.EncodeMessage(e, &dimse.C_STORE_RQ{
		AffectedSOPClassUID:    sopClassUID,
		MessageID:              dimse.NewMessageID(),
		CommandDataSetType:     dimse.CommandDataSetTypeNonNull,
		AffectedSOPInstanceUID: sopInstanceUID,
	})
	if err := e.Error(); err != nil {
		return err
	}
	su.downcallCh <- stateEvent{
		event: evt09,
		dimsePayload: &stateEventDIMSEPayload{
			abstractSyntaxName: sopClassUID,
			command:            e.Bytes(),
			data:               body}}
	for {
		event, ok := <-su.upcallCh
		if !ok {
			su.status = serviceUserClosed
			return fmt.Errorf("Connection closed while waiting for C-STORE response")
		}
		doassert(event.eventType == upcallEventData)
		doassert(event.command != nil)
		resp, ok := event.command.(*dimse.C_STORE_RSP)
		doassert(ok) // TODO(saito)
		vlog.VI(1).Infof("C-STORE: got resp: %v", resp)
		if resp.Status.Status != 0 {
			return fmt.Errorf("C_STORE failed: %v", resp.String())
		}
		return nil
	}
}

func (su *ServiceUser) CStore(ds *dicom.DataSet) error {
	return runCStoreOnAssociation(su.upcallCh, su.downcallCh, su.cm, dimse.NewMessageID(), ds)
}

type CFindQRLevel int

const (
	CFindPatientQRLevel CFindQRLevel = iota
	CFindStudyQRLevel
)

type CFindResult struct {
	// Exactly one of Err or Elements is set.
	Err      error
	Elements []*dicom.Element // Elements belonging to one dataset.
}

type CMoveResult struct {
	Remaining int // Number of files remaining to be sent. Set -1 if unknown.
	Err       error
	Path      string         // Path name of the DICOM file being copied. Used only for reporting errors.
	DataSet   *dicom.DataSet // Contents of the file.
}

// CFind issues a C-FIND request. Returns a channel that streams sequence of
// either an error or a dataset found. The caller MUST read all responses from
// the channel before issuing any other DIMSE command (C-FIND, C-STORE, etc).
//
// The param sopClassUID is one of the UIDs defined in sopclass.QRFindClasses.
// filter is the list of elements to match and retrieve.
func (su *ServiceUser) CFind(qrLevel CFindQRLevel, filter []*dicom.Element) chan CFindResult {
	ch := make(chan CFindResult, 128)
	err := waitAssociationEstablishment(su)
	if err != nil {
		ch <- CFindResult{Err: err}
		close(ch)
		return ch
	}
	// Translate qrLevel to the sopclass and QRLevel elem.
	var sopClassUID string
	var qrLevelString string
	switch qrLevel {
	case CFindPatientQRLevel:
		sopClassUID = dicomuid.PatientRootQRFind
		qrLevelString = "PATIENT"
	case CFindStudyQRLevel:
		sopClassUID = dicomuid.StudyRootQRFind
		qrLevelString = "STUDY"
	default:
		ch <- CFindResult{Err: fmt.Errorf("Invalid C-FIND QR lever: %d", qrLevel)}
		close(ch)
		return ch
	}

	// Encode the C-FIND DIMSE command.
	cmdEncoder := dicomio.NewBytesEncoder(nil, dicomio.UnknownVR)
	context, err := su.cm.lookupByAbstractSyntaxUID(sopClassUID)
	if err != nil {
		// This happens when the user passed a wrong sopclass list in
		// A-ASSOCIATE handshake.
		vlog.Errorf("Failed to lookup sopclass %v: %v", sopClassUID, err)
		ch <- CFindResult{Err: err}
		close(ch)
		return ch
	}
	dimse.EncodeMessage(cmdEncoder, &dimse.C_FIND_RQ{
		AffectedSOPClassUID: sopClassUID,
		MessageID:           dimse.NewMessageID(),
		CommandDataSetType:  dimse.CommandDataSetTypeNonNull,
	})
	if err := cmdEncoder.Error(); err != nil {
		ch <- CFindResult{Err: err}
		close(ch)
		return ch
	}

	// Encode the data payload containing the filtering conditions.
	dataEncoder := dicomio.NewBytesEncoderWithTransferSyntax(context.transferSyntaxUID)
	dicom.WriteElement(dataEncoder, dicom.MustNewElement(dicom.TagQueryRetrieveLevel, qrLevelString))
	for _, elem := range filter {
		if elem.Tag == dicom.TagQueryRetrieveLevel {
			// This tag is auto-computed from qrlevel.
			ch <- CFindResult{Err: fmt.Errorf("%v: tag must not be in the C-FIND payload (it is derived from qrLevel)", elem.Tag)}
			close(ch)
			return ch
		}
		dicom.WriteElement(dataEncoder, elem)
	}
	if err := dataEncoder.Error(); err != nil {
		ch <- CFindResult{Err: err}
		close(ch)
		return ch
	}
	go func() {
		defer close(ch)
		su.downcallCh <- stateEvent{
			event: evt09,
			dimsePayload: &stateEventDIMSEPayload{
				abstractSyntaxName: sopClassUID,
				command:            cmdEncoder.Bytes(),
				data:               dataEncoder.Bytes()}}
		for {
			event, ok := <-su.upcallCh
			if !ok {
				su.status = serviceUserClosed
				ch <- CFindResult{Err: fmt.Errorf("Connection closed while waiting for C-FIND response")}
				break
			}
			doassert(event.eventType == upcallEventData)
			doassert(event.command != nil)
			resp, ok := event.command.(*dimse.C_FIND_RSP)
			if !ok {
				ch <- CFindResult{Err: fmt.Errorf("Found wrong response for C-FIND: %v", event.command)}
				break
			}
			elems, err := readElementsInBytes(event.data, context.transferSyntaxUID)
			if err != nil {
				vlog.Errorf("Failed to decode C-FIND response: %v %v", resp.String(), err)
				ch <- CFindResult{Err: err}
			} else {
				ch <- CFindResult{Elements: elems}
			}
			if resp.Status.Status != dimse.StatusPending {
				if resp.Status.Status != 0 {
					// TODO: report error if status!= 0
					panic(resp)
				}
				break
			}
		}
	}()
	return ch
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
		vlog.Fatalf("No event expected after release, but received %v", event)
	}
}
