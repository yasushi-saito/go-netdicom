package netdicom

import (
	"fmt"
	"github.com/yasushi-saito/go-dicom"
	"log"
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
	downcallCh    chan StateEvent
	upcallCh      chan UpcallEvent
	nextMessageID int32
}

type ServiceUserParams struct {
	Provider         string // server "host:port"
	CalledAETitle    string
	CallingAETitle   string
	RequiredServices []SOPUID

	// List of Transfer syntaxes supported by the user.
	// The value is most often StandardTransferSyntaxes.
	SupportedTransferSyntaxes []string
	MaxPDUSize                uint32
}

func NewServiceUserParams(
	provider string,
	calledAETitle string,
	callingAETitle string,
	requiredServices []SOPUID) ServiceUserParams {
	return ServiceUserParams{
		Provider:                  provider,
		CalledAETitle:             calledAETitle,
		CallingAETitle:            callingAETitle,
		RequiredServices:          requiredServices,
		SupportedTransferSyntaxes: dicom.StandardTransferSyntaxes,
		MaxPDUSize:                1 << 20,
	}
}

func NewServiceUser(params ServiceUserParams) *ServiceUser {
	return &ServiceUser{
		status: serviceUserInitial,
		// sm: NewStateMachineForServiceUser(params, nil, nil),
		downcallCh:    make(chan StateEvent, 128),
		upcallCh:      make(chan UpcallEvent, 128),
		nextMessageID: 123, // any value != 0 suffices.
	}
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
		log.Panicf("Illegal upcall event during handshake: %v", event)
	}
	if su.status != serviceUserAssociationActive {
		return fmt.Errorf("Connection failed")
	}
	return nil
}

func newMessageID(su *ServiceUser) uint16 {
	id := atomic.AddInt32(&su.nextMessageID, 1)
	return uint16(id % 0x10000)
}

func (su *ServiceUser) CStore(
	abstractSyntaxUID string,
	sopInstanceUID string,
	data []byte) error {
	err := waitAssociationEstablishment(su)
	if err != nil {
		return err
	}
	req, err := EncodeDIMSEMessage(&C_STORE_RQ{
		AffectedSOPClassUID:    abstractSyntaxUID,
		MessageID:              newMessageID(su),
		CommandDataSetType:     1, // anything other than nul suffices.
		AffectedSOPInstanceUID: sopInstanceUID,
	})
	if err != nil {
		return err
	}
	// 2017/08/23 20:40:17 VRead all data for syntax 1.2.840.10008.5.1.4.1.1.12.1[X-Ray Angiographic Image Storage], command [cstorerq{sopclass:1.2.840.10008.5.1.4.1.1.12.1 messageid:1 pri: 2 cmddatasettype: 1 sopinstance: 1.2.840.113857.1626.160635.1727.151424.1.1 m0: m1:0}], data 2752272 bytes, err<nil>
	su.downcallCh <- StateEvent{
		event:              Evt9,
		abstractSyntaxName: abstractSyntaxUID,
		command:            true,
		data:               req}
	su.downcallCh <- StateEvent{
		event:              Evt9,
		abstractSyntaxName: abstractSyntaxUID,
		command:            false,
		data:               data}
	for {
		event, ok := <-su.upcallCh
		if !ok {
			su.status = serviceUserClosed
			return fmt.Errorf("Connection closed while waiting for cstore response")
		}
		doassert(event.eventType == upcallEventData)
		// pdu, ok := event.pdu.(*P_DATA_TF)
	}
	panic("aoue not implemented")
}

func (su *ServiceUser) Release() {
	err := waitAssociationEstablishment(su)
	if err != nil {
		return
	}
	su.downcallCh <- StateEvent{event: Evt11}
	for {
		event, ok := <-su.upcallCh
		if !ok {
			su.status = serviceUserClosed
			break
		}
		log.Panicf("No event expected after release, but received %v", event)
	}
}
