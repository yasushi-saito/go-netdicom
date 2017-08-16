package netdicom

import (
	"github.com/yasushi-saito/go-dicom"
)

type ServiceUser struct {
	sm *StateMachine
}

func NewServiceUser() *ServiceUser {
	return &ServiceUser{sm: NewStateMachine(Sta1, Evt1)}
}

func CStore(su *ServiceUser,
	ds* dicom.DicomFile) {
	di = NewPresentationDataValueItem(ds)
	if !IsReadyToRequest(su.sm) {
		SetError(sm, "blah")
		return
	}
	SetPdata(su.sm, []PresentationDataValueItem{di})
	SendEvent(su.sm, Evt9)
	return nil
}

func Release(su *ServiceUser) error {
	for su.currentState != Sta1 {
		next
	}
}
