package netdicom

import (
	_ "sync"
)

type faultInjectorAction int
const (
	faultInjectorContinue = iota
	faultInjectorDisconnect
)

type FaultInjector struct {
	fuzz  []byte
	steps int
}

var userFaults, providerFaults *FaultInjector

func nextFuzzByte(f *FaultInjector) byte {
	doassert(len(f.fuzz) > 0)
	v := f.fuzz[f.steps]
	f.steps++
	if f.steps >= len(f.fuzz) {
		f.steps = 0
	}
	return v
}

func nextFuzzUInt32(f *FaultInjector) uint32 {
	return (uint32(nextFuzzByte(f)) << 24) |
		(uint32(nextFuzzByte(f)) << 16) |
		(uint32(nextFuzzByte(f)) << 8) |
		uint32(nextFuzzByte(f))
}

func NewFaultInjector(fuzz []byte) *FaultInjector {
	return &FaultInjector{fuzz: fuzz}
}

func SetUserFaultInjector(f *FaultInjector) {
	userFaults = f
}
func SetProviderFaultInjector(f *FaultInjector) {
	providerFaults = f
}

func GetUserFaultInjector() *FaultInjector {
	return userFaults
}
func GetProviderFaultInjector() *FaultInjector {
	return providerFaults
}

func (f *FaultInjector) onSend(data []byte) faultInjectorAction {
	if len(f.fuzz) == 0 {
		return faultInjectorContinue
	}
	op := nextFuzzByte(f)
	if op >= 0xe8 {
		return faultInjectorDisconnect
	}
	return faultInjectorContinue
}
