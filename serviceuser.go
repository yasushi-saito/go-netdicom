package netdicom

import (
// "github.com/yasushi-saito/go-dicom"
)

type ServiceUser struct {
	sm *StateMachine
}

func NewServiceUser(server string) *ServiceUser {
	return &ServiceUser{sm: NewStateMachineForServiceUser(server)}
}

func (su *ServiceUser) CStore(data []byte) {
	di := NewPresentationDataValueItem(0 /*todo*/, data)
	SendData(su.sm, []PresentationDataValueItem{di})
}

func (su *ServiceUser) Release() error {
	StartRelease(su.sm)
	return nil
}
