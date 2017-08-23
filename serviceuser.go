package netdicom

import (
	"github.com/yasushi-saito/go-dicom"
)

type ServiceUser struct {
	sm *StateMachine
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
		sm: NewStateMachineForServiceUser(params),
	}
}

func (su *ServiceUser) CStore(abstractSyntaxUID string, data []byte) {
	sendData(su.sm, abstractSyntaxUID, false /*data*/, data)
	panic("aoue not implemented")
}

func (su *ServiceUser) Release() error {
	StartRelease(su.sm)
	return nil
}
