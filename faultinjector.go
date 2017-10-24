package netdicom

import (
	"fmt"
	"math"
)

type faultInjectorAction int

const (
	faultInjectorContinue = iota
	faultInjectorDisconnect
)

type faultInjectorStateTransition struct {
	state  stateType
	event  *stateEvent
	action *stateAction
}

type FaultInjector interface {
	fmt.Stringer
	// Called when an "event" happens when at "oldState" and transitions to
	// "newState"
	onStateTransition(oldState stateType, event *stateEvent, action *stateAction, newState stateType)
	onSend(data []byte) faultInjectorAction
}

func SetUserFaultInjector(f FaultInjector) {
	userFaults = f
}
func SetProviderFaultInjector(f FaultInjector) {
	providerFaults = f
}

func getUserFaultInjector() FaultInjector {
	return userFaults
}
func getProviderFaultInjector() FaultInjector {
	return providerFaults
}

var userFaults, providerFaults FaultInjector

// FuzzFaultInjector is used by fuzz tests to inject faults somewhat
// deterministically.
type FuzzFaultInjector struct {
	fuzz  []byte
	steps int

	stateHistory []faultInjectorStateTransition
}

func fuzzByte(f *FuzzFaultInjector) byte {
	doassert(len(f.fuzz) > 0)
	v := f.fuzz[f.steps]
	f.steps++
	if f.steps >= len(f.fuzz) {
		f.steps = 0
	}
	return v
}

func fuzzUInt16(f *FuzzFaultInjector) uint16 {
	return (uint16(fuzzByte(f)) << 8) |
		uint16(fuzzByte(f))
}

func fuzzUInt32(f *FuzzFaultInjector) uint32 {
	return (uint32(fuzzByte(f)) << 24) |
		(uint32(fuzzByte(f)) << 16) |
		(uint32(fuzzByte(f)) << 8) |
		uint32(fuzzByte(f))
}

func fuzzExponentialInRange(f *FuzzFaultInjector, max int) int {
	// Generate a uniform number in range [0,1]
	r := float64(fuzzUInt16(f)) / float64(0xffff)
	doassert(r >= 0 && r <= 1.0)
	// Convert to exponential distribution with mean of 1.
	exp := -math.Log(r)
	v := int(exp * float64(max))
	if v < 0 {
		v = 0
	}
	if v >= max {
		v = max - 1
	}
	return v
}

func NewFuzzFaultInjector(fuzz []byte) FaultInjector {
	return &FuzzFaultInjector{fuzz: fuzz}
}

func (f *FuzzFaultInjector) onStateTransition(oldState stateType, event *stateEvent, action *stateAction, newState stateType) {
	f.stateHistory = append(f.stateHistory, faultInjectorStateTransition{oldState, event, action})
}

func (f *FuzzFaultInjector) onSend(data []byte) faultInjectorAction {
	if len(f.fuzz) == 0 {
		return faultInjectorContinue
	}
	op := fuzzByte(f)
	if op >= 0xe8 {
		return faultInjectorDisconnect
	}
	if op >= 0xc0 {
		// Mutate a byte.
		offset := fuzzExponentialInRange(f, len(data))
		data[offset] = fuzzByte(f)
	}
	return faultInjectorContinue
}

func (f *FuzzFaultInjector) String() string {
	s := "statehistory:{"
	for i, e := range f.stateHistory {
		if i > 0 {
			s += ","
		}
		s += fmt.Sprintf("{state:%v, event:%v, action:%v}\n",
			e.state.String(), e.event.String(), e.action.String())
	}
	return s + "}"
}
