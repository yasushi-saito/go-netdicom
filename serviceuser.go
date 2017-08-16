package netdicom

import (
	// "github.com/yasushi-saito/go-dicom"
)

type ServiceUser struct {
	sm *StateMachine
}

func NewServiceUser() *ServiceUser {
	return &ServiceUser{sm: NewStateMachineForServiceUser()}
}

func CStore(su *ServiceUser, data []byte) {
	di := NewPresentationDataValueItem(0/*todo*/, data)
	SendData(su.sm, []PresentationDataValueItem{di})
}

func Release(su *ServiceUser) {
	StartRelease(su.sm)
}
