package netdicom

import (
// "github.com/yasushi-saito/go-dicom"
)

type ServiceProvider struct {
	sm *StateMachine
}

func NewServiceProvider(port int) *ServiceProvider {
	return &ServiceProvider{sm: NewStateMachineForServiceProvider(port)}
}
