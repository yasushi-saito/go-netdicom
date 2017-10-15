// This file implements the ServiceUser (i.e., a DICOM DIMSE client) class.
package netdicom

import (
	"errors"
	"fmt"
	"net"
	"sync"

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
	serviceUserAssociationActive
	serviceUserClosed
)

// ServiceUser encapsulates implements the client side of DICOM network protocol.
//
//  params, err := netdicom.NewServiceUserParams(
//     "dontcare" /*remote app-entity title*/,
//     "testclient" /*this app-entity title*/,
//     sopclass.QRFindClasses, /* SOP classes to use in the requests*/
//     nil /* transfer syntaxes to use; unually nil suffices */)
//  user := netdicom.NewServiceUser(params)
//  // Connect to server 1.2.3.4, port 8888
//  user.Connect("1.2.3.4:8888")
//  // Send test.dcm to the server
//  ds, err := dicom.ReadDataSetFromFile("test.dcm", dicom.ReadOptions{})
//  err := user.CStore(ds)
//  // Disconnect
//  user.Release()
//
// The ServiceUser class is thread compatible. That is, you cannot call C*
// methods concurrently - say two CStore requests - from two goroutines.  You
// must wait for one CStore to finish before issuing another one.
type ServiceUser struct {
	downcallCh chan stateEvent
	upcallCh   chan upcallEvent

	mu   *sync.Mutex
	cond *sync.Cond // Broadcast when status changes.

	// Following fields are guarded by mu.
	status         serviceUserStatus
	cm             *contextManager              // Set only after the handshake completes.
	activeCommands map[uint16]*userCommandState // List of commands running
}

func (su *ServiceUser) createCommand(messageID uint16) *userCommandState {
	su.mu.Lock()
	defer su.mu.Unlock()
	if _, ok := su.activeCommands[messageID]; ok {
		panic(messageID)
	}
	cs := &userCommandState{
		parent:    su,
		messageID: messageID,
		upcallCh:  make(chan upcallEvent, 128),
	}
	su.activeCommands[messageID] = cs
	return cs
}

func (su *ServiceUser) findCommand(messageID uint16) *userCommandState {
	su.mu.Lock()
	defer su.mu.Unlock()
	if cs, ok := su.activeCommands[messageID]; ok {
		return cs
	}
	return nil
}

func (su *ServiceUser) deleteCommand(cs *userCommandState) {
	su.mu.Lock()
	if _, ok := su.activeCommands[cs.messageID]; !ok {
		panic(fmt.Sprintf("cs %+v", cs))
	}
	delete(su.activeCommands, cs.messageID)
	su.mu.Unlock()
	close(cs.upcallCh)
}

// Per-command-invocation state.
type userCommandState struct {
	parent    *ServiceUser // parent dispatcher
	messageID uint16       // PROVIDER MessageID
	// context   contextManagerEntry // the transfersyntax/sopclass for this command.

	// upcallCh streams PROVIDER command+data for the given messageID.
	upcallCh chan upcallEvent
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

// NewServiceUserParams creates a ServiceUserParams.  requiredServices is the
// abstract syntaxes (SOP classes) that the client wishes to use in the
// requests.  It's usually one of the lists defined in the sopclass package.  If
// transferSyntaxUIDs is empty, the exhaustive list of syntaxes defined in the
// DICOM standard is used.
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

func (su *ServiceUser) handleEvent(event upcallEvent) {
	messageID := event.command.GetMessageID()
	cs := su.findCommand(messageID)
	if cs == nil {
		vlog.Errorf("Dropping message for non-existent ID: %v", event.command)
		return
	}
	cs.upcallCh <- event
}

// NewServiceUser creates a new ServiceUser. The caller must call either
// Connect() or SetConn() before calling any other method, such as Cstore.
func NewServiceUser(params ServiceUserParams) *ServiceUser {
	mu := &sync.Mutex{}
	su := &ServiceUser{
		// sm: NewStateMachineForServiceUser(params, nil, nil),
		downcallCh: make(chan stateEvent, 128),
		upcallCh:   make(chan upcallEvent, 128),

		mu:             mu,
		cond:           sync.NewCond(mu),
		status:         serviceUserInitial,
		activeCommands: make(map[uint16]*userCommandState),
	}
	go runStateMachineForServiceUser(params, su.upcallCh, su.downcallCh)
	go func() {
		for event := range su.upcallCh {
			if event.eventType == upcallEventHandshakeCompleted {
				su.mu.Lock()
				doassert(su.cm == nil)
				su.status = serviceUserAssociationActive
				su.cond.Broadcast()
				su.cm = event.cm
				doassert(su.cm != nil)
				su.mu.Unlock()
				continue
			}
			doassert(event.eventType == upcallEventData)
			su.handleEvent(event)
		}
		vlog.Infof("Service user dispatcher finished")
		su.mu.Lock()
		su.cond.Broadcast()
		su.status = serviceUserClosed
		su.mu.Unlock()
	}()
	return su
}

func (su *ServiceUser) waitUntilReady() error {
	su.mu.Lock()
	defer su.mu.Unlock()
	for su.status <= serviceUserInitial {
		su.cond.Wait()
	}
	if su.status != serviceUserAssociationActive {
		// Will get an error when waiting for a response.
		vlog.Errorf("Connection failed")
		return fmt.Errorf("Connection failed")
	}
	return nil
}

// Connect connects to the server at the given "host:port". Either Connect or
// SetConn must be before calling CStore, etc.
func (su *ServiceUser) Connect(serverAddr string) {
	doassert(su.status == serviceUserInitial)
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		vlog.Infof("Connect(%s): %v", serverAddr, err)
		su.downcallCh <- stateEvent{event: evt17, pdu: nil, err: err}
	} else {
		su.downcallCh <- stateEvent{event: evt02, pdu: nil, err: nil, conn: conn}
	}
}

// SetConn instructs ServiceUser to use the given network connection to talk to
// the server. Either Connect or SetConn must be before calling CStore, etc.
func (su *ServiceUser) SetConn(conn net.Conn) {
	doassert(su.status == serviceUserInitial)
	su.downcallCh <- stateEvent{event: evt02, pdu: nil, err: nil, conn: conn}
}

// Send a C-ECHO request to the remote AE. Returns nil iff the remote AE
// responds ok.
func (su *ServiceUser) CEcho() error {
	err := su.waitUntilReady()
	if err != nil {
		return err
	}
	cs := su.createCommand(dimse.NewMessageID())
	defer su.deleteCommand(cs)
	su.downcallCh <- stateEvent{
		event: evt09,
		dimsePayload: &stateEventDIMSEPayload{
			abstractSyntaxName: dicomuid.VerificationSOPClass,
			command: &dimse.C_ECHO_RQ{
				MessageID:          cs.messageID,
				CommandDataSetType: dimse.CommandDataSetTypeNull,
			},
			data: nil}}
	event, ok := <-cs.upcallCh
	if !ok {
		return fmt.Errorf("Failed to receive C-ECHO response")
	}
	resp, ok := event.command.(*dimse.C_ECHO_RSP)
	if !ok {
		return fmt.Errorf("Invalid response for C-ECHO: %v", event.command)
	}
	if resp.Status.Status != dimse.StatusSuccess {
		err = fmt.Errorf("Non-OK status in C-ECHO response: %v", resp.Status)
	}
	return nil
}

// CStore issues a C-STORE request to transfer "ds" in remove peer.  It blocks
// until the operation finishes.
//
// REQUIRES: Connect() or SetConn has been called.
func (su *ServiceUser) CStore(ds *dicom.DataSet) error {
	err := su.waitUntilReady()
	if err != nil {
		return err
	}
	doassert(su.cm != nil)
	cs := su.createCommand(dimse.NewMessageID())
	return runCStoreOnAssociation(cs.upcallCh, su.downcallCh, su.cm, cs.messageID, ds)
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
//
// REQUIRES: Connect() or SetConn has been called.
func (su *ServiceUser) CFind(qrLevel CFindQRLevel, filter []*dicom.Element) chan CFindResult {
	ch := make(chan CFindResult, 128)
	err := su.waitUntilReady()
	if err != nil {
		ch <- CFindResult{Err: err}
		close(ch)
		return ch
	}
	cs := su.createCommand(dimse.NewMessageID())
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
	context, err := su.cm.lookupByAbstractSyntaxUID(sopClassUID)
	if err != nil {
		// This happens when the user passed a wrong sopclass list in
		// A-ASSOCIATE handshake.
		vlog.Errorf("Failed to lookup sopclass %v: %v", sopClassUID, err)
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
		defer su.deleteCommand(cs)
		su.downcallCh <- stateEvent{
			event: evt09,
			dimsePayload: &stateEventDIMSEPayload{
				abstractSyntaxName: sopClassUID,
				command: &dimse.C_FIND_RQ{
					AffectedSOPClassUID: sopClassUID,
					MessageID:           cs.messageID,
					CommandDataSetType:  dimse.CommandDataSetTypeNonNull,
				},
				data: dataEncoder.Bytes()}}
		for {
			event, ok := <-cs.upcallCh
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

// Release shuts down the connection. It must be called exactly once.  After
// Release(), no other operation can be performed on the ServiceUser object.
func (su *ServiceUser) Release() {
	su.waitUntilReady()
	su.downcallCh <- stateEvent{event: evt11}

	su.mu.Lock()
	defer su.mu.Unlock()
	su.status = serviceUserClosed
	su.cond.Broadcast()
	for _, cs := range su.activeCommands {
		close(cs.upcallCh)
	}
}
