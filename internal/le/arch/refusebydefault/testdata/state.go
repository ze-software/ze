// Fixture for RuleLiteralState (AC-4).
package fixture

import "errors"

type Status int

const (
	StatusUnknown Status = iota
	StatusDegraded
	StatusHealthy
)

func lookupService() *int { return nil }

// checkHealth mirrors l2tp: registered reads as healthy, and nothing measures.
func checkHealth() (Status, string) {
	if lookupService() == nil {
		return StatusDegraded, "subsystem not running"
	}
	return StatusHealthy, "" // want: state-from-literal
}

func readSessions() (int, error) {
	return 0, errors.New("not read")
}

// measuredHealth reads the state it reports: no finding.
func measuredHealth() (Status, string) {
	count, err := readSessions()
	if err != nil {
		return StatusUnknown, err.Error()
	}
	if count == 0 {
		return StatusDegraded, "no sessions"
	}
	return StatusHealthy, ""
}

// supported mirrors trafficusage ebpfSupported: a check that cannot fail.
func supported() error { return nil } // want: state-from-literal

// PeerState is a state answer.
type PeerState struct {
	Running bool
	Name    string
}

func state(name string) PeerState {
	return PeerState{
		Running: true, // want: state-from-literal
		Name:    name,
	}
}
