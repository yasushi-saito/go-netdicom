package netdicom

import (
	_ "sync"
)

type FaultInjector struct {
	fuzz  []byte
	steps int
}

var userFaults, providerFaults *FaultInjector

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

func (f *FaultInjector) shouldContinue() bool {
	if len(f.fuzz) == 0 {
		return true
	}
	f.steps++
	action := f.fuzz[f.steps % len(f.fuzz)]
	if action < byte(0x70) {
		return true
	} else {
		return false
	}
}
