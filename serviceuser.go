// This file implements the ServiceUser (i.e., a DICOM DIMSE client) class.
package netdicom

import (
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
// methods - say CStore and CFind requests - concurrently from two goroutines.
// You must wait for CStore to finish before issuing CFind.
type ServiceUser struct {
	// downcallCh chan stateEvent
	upcallCh chan upcallEvent

	mu   *sync.Mutex
	cond *sync.Cond // Broadcast when status changes.

	disp *serviceDispatcher

	// Following fields are guarded by mu.
	status serviceUserStatus
	cm     *contextManager // Set only after the handshake completes.
	// activeCommands map[uint16]*userCommandState // List of commands running
}

type ServiceUserParams struct {
	// Application-entity title of the peer. If empty, set to "unknown-called-ae"
	CalledAETitle string
	// Application-entity title of the client. If empty, set to
	// "unknown-calling-ae"
	CallingAETitle string

	// List of SOPUIDs wanted by the client. The value is typically one of
	// the constants listed in sopclass package.
	SOPClasses []sopclass.SOPUID

	// List of Transfer syntaxes supported by the user.  If you know the
	// transer syntax of the file you are going to copy, set that here.
	// Otherwise, you'll need to re-encode the data w/ the given transfer
	// syntax yourself.
	//
	// TODO(saito) Support reencoding internally on C_STORE, etc. The DICOM
	// spec is particularly moronic here, since we could just have specified
	// the transfer syntax per data sent.
	TransferSyntaxes []string
}

func validateServiceUserParams(params *ServiceUserParams) error {
	if params.CalledAETitle == "" {
		params.CalledAETitle = "unknown-called-ae"
	}
	if params.CallingAETitle == "" {
		params.CallingAETitle = "unknown-calling-ae"
	}
	if len(params.SOPClasses) == 0 {
		return fmt.Errorf("Empty ServiceUserParams.SOPClasses")
	}
	if len(params.TransferSyntaxes) == 0 {
		params.TransferSyntaxes = dicomio.StandardTransferSyntaxes
	} else {
		for i, uid := range params.TransferSyntaxes {
			canonicalUID, err := dicomio.CanonicalTransferSyntaxUID(uid)
			if err != nil {
				return err
			}
			params.TransferSyntaxes[i] = canonicalUID
		}
	}
	return nil
}

// NewServiceUser creates a new ServiceUser. The caller must call either
// Connect() or SetConn() before calling any other method, such as Cstore.
func NewServiceUser(params ServiceUserParams) (*ServiceUser, error) {
	if err := validateServiceUserParams(&params); err != nil {
		return nil, err
	}
	mu := &sync.Mutex{}
	su := &ServiceUser{
		// sm: NewStateMachineForServiceUser(params, nil, nil),
		// downcallCh: make(chan stateEvent, 128),
		upcallCh: make(chan upcallEvent, 128),
		disp:     newServiceDispatcher(),
		mu:       mu,
		cond:     sync.NewCond(mu),
		status:   serviceUserInitial,
	}
	go runStateMachineForServiceUser(params, su.upcallCh, su.disp.downcallCh)
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
			su.disp.handleEvent(event)
		}
		vlog.VI(1).Infof("Service user dispatcher finished")
		su.mu.Lock()
		su.cond.Broadcast()
		su.status = serviceUserClosed
		su.mu.Unlock()
	}()
	return su, nil
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
		su.disp.downcallCh <- stateEvent{event: evt17, pdu: nil, err: err}
	} else {
		su.disp.downcallCh <- stateEvent{event: evt02, pdu: nil, err: nil, conn: conn}
	}
}

// SetConn instructs ServiceUser to use the given network connection to talk to
// the server. Either Connect or SetConn must be before calling CStore, etc.
func (su *ServiceUser) SetConn(conn net.Conn) {
	doassert(su.status == serviceUserInitial)
	su.disp.downcallCh <- stateEvent{event: evt02, pdu: nil, err: nil, conn: conn}
}

// Send a C-ECHO request to the remote AE. Returns nil iff the remote AE
// responds ok.
func (su *ServiceUser) CEcho() error {
	err := su.waitUntilReady()
	if err != nil {
		return err
	}
	context, err := su.cm.lookupByAbstractSyntaxUID(dicomuid.VerificationSOPClass)
	if err != nil {
		return err
	}
	cs, found := su.disp.findOrCreateCommand(dimse.NewMessageID(), su.cm, context)
	doassert(!found)
	defer su.disp.deleteCommand(cs)
	cs.sendMessage(
		&dimse.C_ECHO_RQ{MessageID: cs.messageID,
			CommandDataSetType: dimse.CommandDataSetTypeNull,
		}, nil)
	event, ok := <-cs.upcallCh
	if !ok {
		return fmt.Errorf("Failed to receive C-ECHO response")
	}
	resp, ok := event.command.(*dimse.C_ECHO_RSP)
	if !ok {
		return fmt.Errorf("Invalid response for C-ECHO: %v", event.command)
	}
	if resp.Status.Status != dimse.StatusSuccess {
		err = fmt.Errorf("Non-OK status in C-ECHO response: %+v", resp.Status)
	}
	return err
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

	var sopClassUID string
	if sopClassUIDElem, err := ds.FindElementByTag(dicom.TagMediaStorageSOPClassUID); err != nil {
		return err
	} else if sopClassUID, err = sopClassUIDElem.GetString(); err != nil {
		return err
	}
	context, err := su.cm.lookupByAbstractSyntaxUID(sopClassUID)
	if err != nil {
		return err
	}
	cs, found := su.disp.findOrCreateCommand(dimse.NewMessageID(), su.cm, context)
	doassert(!found)
	if err != nil {
		vlog.Errorf("C-STORE: sop class %v not found in context %v", sopClassUID, err)
		return err
	}
	defer su.disp.deleteCommand(cs)
	return runCStoreOnAssociation(cs.upcallCh, su.disp.downcallCh, su.cm, cs.messageID, ds)
}

type QRLevel int
type qrOpType int

const (
	QRLevelPatient QRLevel = iota
	QRLevelStudy

	qrOpCFind qrOpType = iota
	qrOpCGet
	qrOpCMove
)

type CFindResult struct {
	// Exactly one of Err or Elements is set.
	Err      error
	Elements []*dicom.Element // Elements belonging to one dataset.
}

func encodeQRPayload(opType qrOpType, qrLevel QRLevel, filter []*dicom.Element, cm *contextManager) (contextManagerEntry, []byte, error) {
	var sopClassUID string
	var qrLevelString string
	switch qrLevel {
	case QRLevelPatient:
		switch opType {
		case qrOpCFind:
			sopClassUID = dicomuid.PatientRootQRFind
		case qrOpCGet:
			sopClassUID = dicomuid.PatientRootQRGet
		case qrOpCMove:
			sopClassUID = dicomuid.PatientRootQRMove
		}
		qrLevelString = "PATIENT"
	case QRLevelStudy:
		switch opType {
		case qrOpCFind:
			sopClassUID = dicomuid.StudyRootQRFind
		case qrOpCGet:
			sopClassUID = dicomuid.StudyRootQRGet
		case qrOpCMove:
			sopClassUID = dicomuid.StudyRootQRMove
		}
		qrLevelString = "STUDY"
	default:
		return contextManagerEntry{}, nil, fmt.Errorf("Invalid C-FIND QR lever: %d", qrLevel)
	}

	// Translate qrLevel to the sopclass and QRLevel elem.
	// Encode the C-FIND DIMSE command.
	context, err := cm.lookupByAbstractSyntaxUID(sopClassUID)
	if err != nil {
		// This happens when the user passed a wrong sopclass list in
		// A-ASSOCIATE handshake.
		return context, nil, err
	}

	// Encode the data payload containing the filtering conditions.
	dataEncoder := dicomio.NewBytesEncoderWithTransferSyntax(context.transferSyntaxUID)
	dicom.WriteElement(dataEncoder, dicom.MustNewElement(dicom.TagQueryRetrieveLevel, qrLevelString))
	for _, elem := range filter {
		if elem.Tag == dicom.TagQueryRetrieveLevel {
			// This tag is auto-computed from qrlevel.
			return context, nil, fmt.Errorf("%v: tag must not be in the C-FIND payload (it is derived from qrLevel)", elem.Tag)
		}
		dicom.WriteElement(dataEncoder, elem)
	}
	if err := dataEncoder.Error(); err != nil {
		return context, nil, err
	}
	return context, dataEncoder.Bytes(), err
}

// CFind issues a C-FIND request. Returns a channel that streams sequence of
// either an error or a dataset found. The caller MUST read all responses from
// the channel before issuing any other DIMSE command (C-FIND, C-STORE, etc).
//
// The param sopClassUID is one of the UIDs defined in sopclass.QRFindClasses.
// filter is the list of elements to match and retrieve.
//
// REQUIRES: Connect() or SetConn has been called.
func (su *ServiceUser) CFind(qrLevel QRLevel, filter []*dicom.Element) chan CFindResult {
	ch := make(chan CFindResult, 128)
	err := su.waitUntilReady()
	if err != nil {
		ch <- CFindResult{Err: err}
		close(ch)
		return ch
	}
	context, payload, err := encodeQRPayload(qrOpCFind, qrLevel, filter, su.cm)
	if err != nil {
		ch <- CFindResult{Err: err}
		close(ch)
		return ch
	}
	cs, found := su.disp.findOrCreateCommand(dimse.NewMessageID(), su.cm, context)
	doassert(!found)
	go func() {
		defer close(ch)
		defer su.disp.deleteCommand(cs)
		cs.sendMessage(
			&dimse.C_FIND_RQ{
				AffectedSOPClassUID: context.abstractSyntaxUID,
				MessageID:           cs.messageID,
				CommandDataSetType:  dimse.CommandDataSetTypeNonNull,
			},
			payload)
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

// CGet runs a C-GET command. It calls "cb" for every dataset received. "cb"
// should return dimse.Success iff the data was successfully and stably
// written. This function blocks until it receives all datasets from the server.
func (su *ServiceUser) CGet(qrLevel QRLevel, filter []*dicom.Element,
	cb func(transferSyntaxUID, SOPClassUID, sopInstanceUID string, data []byte) dimse.Status) error {
	err := su.waitUntilReady()
	if err != nil {
		return err
	}
	context, payload, err := encodeQRPayload(qrOpCGet, qrLevel, filter, su.cm)
	if err != nil {
		return err
	}
	cs, found := su.disp.findOrCreateCommand(dimse.NewMessageID(), su.cm, context)
	doassert(!found)
	defer su.disp.deleteCommand(cs)

	handleCStore := func(msg dimse.Message, data []byte, cs *serviceCommandState) {
		c := msg.(*dimse.C_STORE_RQ)
		status := cb(
			context.transferSyntaxUID,
			c.AffectedSOPClassUID,
			c.AffectedSOPInstanceUID,
			data)
		resp := &dimse.C_STORE_RSP{
			AffectedSOPClassUID:       c.AffectedSOPClassUID,
			MessageIDBeingRespondedTo: c.MessageID,
			CommandDataSetType:        dimse.CommandDataSetTypeNull,
			AffectedSOPInstanceUID:    c.AffectedSOPInstanceUID,
			Status:                    status,
		}
		cs.sendMessage(resp, nil)
	}
	su.disp.registerCallback(dimse.CommandFieldC_STORE_RQ, handleCStore)
	defer su.disp.unregisterCallback(dimse.CommandFieldC_STORE_RQ)
	cs.sendMessage(
		&dimse.C_GET_RQ{
			AffectedSOPClassUID: context.abstractSyntaxUID,
			MessageID:           cs.messageID,
			CommandDataSetType:  dimse.CommandDataSetTypeNonNull,
		},
		payload)
	for {
		event, ok := <-cs.upcallCh
		if !ok {
			su.status = serviceUserClosed
			return fmt.Errorf("Connection closed while waiting for C-GET response")
		}
		doassert(event.eventType == upcallEventData)
		doassert(event.command != nil)
		resp, ok := event.command.(*dimse.C_GET_RSP)
		if !ok {
			return fmt.Errorf("Found wrong response for C-GET: %v", event.command)
		}
		if resp.Status.Status != dimse.StatusPending {
			if resp.Status.Status != 0 {
				// TODO: report error if status!= 0
				panic(resp)
			}
			break
		}
	}
	return nil
}

// Release shuts down the connection. It must be called exactly once.  After
// Release(), no other operation can be performed on the ServiceUser object.
func (su *ServiceUser) Release() {
	su.waitUntilReady()
	su.disp.downcallCh <- stateEvent{event: evt11}

	su.mu.Lock()
	defer su.mu.Unlock()
	su.status = serviceUserClosed
	su.cond.Broadcast()
	su.disp.close()
}
