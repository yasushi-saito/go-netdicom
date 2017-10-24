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

// FaultInjector is a unittest helper. It's used by the statemachine to inject
// faults.
type FaultInjector struct {
	fuzz  []byte
	steps int

	stateHistory []faultInjectorStateTransition
}

var userFaults, providerFaults *FaultInjector

func fuzzByte(f *FaultInjector) byte {
	doassert(len(f.fuzz) > 0)
	v := f.fuzz[f.steps]
	f.steps++
	if f.steps >= len(f.fuzz) {
		f.steps = 0
	}
	return v
}

func fuzzUInt16(f *FaultInjector) uint16 {
	return (uint16(fuzzByte(f)) << 8) |
		uint16(fuzzByte(f))
}

func fuzzUInt32(f *FaultInjector) uint32 {
	return (uint32(fuzzByte(f)) << 24) |
		(uint32(fuzzByte(f)) << 16) |
		(uint32(fuzzByte(f)) << 8) |
		uint32(fuzzByte(f))
}

func fuzzExponentialInRange(f *FaultInjector, max int) int {
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

// NewFuzzFaultInjector creates a new fuzzing fault injector
func NewFuzzFaultInjector(fuzz []byte) *FaultInjector {
	return &FaultInjector{fuzz: fuzz}
}

// SetUserFaultInjector sets a singleton fault injector to be used by the user
// (client)-side statemachines.
func SetUserFaultInjector(f *FaultInjector) {
	userFaults = f
}

// SetProviderFaultInjector sets a singleton fault injector to be used by the
// provider (server)-side statemachines.
func SetProviderFaultInjector(f *FaultInjector) {
	providerFaults = f
}

func getUserFaultInjector() *FaultInjector {
	return userFaults
}
func getProviderFaultInjector() *FaultInjector {
	return providerFaults
}

// Called when an "event" happens when at "state".
func (f *FaultInjector) onStateTransition(state stateType, event *stateEvent, action *stateAction) {
	f.stateHistory = append(f.stateHistory, faultInjectorStateTransition{state, event, action})
}

func (f *FaultInjector) onSend(data []byte) faultInjectorAction {
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

func (f *FaultInjector) String() string {
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
