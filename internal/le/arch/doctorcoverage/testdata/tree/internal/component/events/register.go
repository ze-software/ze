package events

import "github.com/ze-software/ze/internal/core/diagnostic"

// declaringCheck declares the event codes, as the recent-events check does.
var declaringCheck = diagnostic.DoctorCheck{
	Name:               "fixture-events-check",
	Codes:              []string{"doctor-fixture-events"},
	DeclaresEventCodes: true,
}

// computedFlagCheck sets the flag from a call, which the source does not fix,
// so it declares only its own code.
var computedFlagCheck = diagnostic.DoctorCheck{
	Name:               "fixture-flag-computed-check",
	Codes:              []string{"doctor-fixture-flag-computed"},
	DeclaresEventCodes: computedFlag(),
}

func computedFlag() bool { return true }

// ownerEvents registers an event code as an owner does, an elided element
// of a slice literal.
var ownerEvents = []diagnostic.EventCode{{Code: "doctor-fixture-event"}}

// runtimeEvent names its code through a parameter the source does not fix,
// so no code is read from it.
func runtimeEvent(code string) diagnostic.EventCode {
	return diagnostic.EventCode{Code: code}
}
