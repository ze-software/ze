// Design: rfc/short/rfc5880.md -- BFD observability surface
// Related: service.go -- Service interface extended with Snapshot/Profiles
// Related: events.go -- SessionRequest, Key, StateChange used by Snapshot
//
// Observability types the BFD plugin publishes to command handlers. The
// engine copies its live state into these structs at snapshot time so
// readers never touch session.Machine internals directly; every field is
// either an RFC 5880 state variable (Section 6.8.1) or an operational
// observation (counts, timestamps) computed from the express loop.
package api

import (
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
)

// SessionState is a point-in-time copy of one BFD session's identity,
// negotiated timers, and recent history. Returned by Service.Snapshot
// and Service.SessionDetail.
//
// All duration fields are Go time.Duration (rendered in microseconds or
// milliseconds by consumers). Discriminators carry their RFC 5880 wire
// values; RemoteDiscr is zero until the first packet is received.
//
// Transitions is a bounded ring of the most recent state changes,
// oldest first. An empty slice means the session has not yet crossed a
// state boundary since creation.
type SessionState struct {
	Peer              string             `json:"peer"`
	Local             string             `json:"local,omitempty"`
	Interface         string             `json:"interface,omitempty"`
	VRF               string             `json:"vrf"`
	Mode              string             `json:"mode"`
	State             string             `json:"state"`
	Diag              string             `json:"diag"`
	LocalDiscr        uint32             `json:"local-discriminator"`
	RemoteDiscr       uint32             `json:"remote-discriminator"`
	TxInterval        time.Duration      `json:"tx-interval"`
	RxInterval        time.Duration      `json:"rx-interval"`
	DetectionInterval time.Duration      `json:"detection-interval"`
	DetectMult        uint8              `json:"detect-multiplier"`
	LastReceived      time.Time          `json:"last-received,omitzero"`
	CreatedAt         time.Time          `json:"created-at"`
	Refcount          int                `json:"refcount"`
	Profile           string             `json:"profile,omitempty"`
	TxPackets         uint64             `json:"tx-packets"`
	RxPackets         uint64             `json:"rx-packets"`
	Transitions       []TransitionRecord `json:"transitions,omitempty"`
}

// TransitionRecord captures one historical state change on a session.
// The engine records up to TransitionHistoryDepth entries per session;
// older entries are evicted when new ones are appended.
type TransitionRecord struct {
	When time.Time `json:"when"`
	From string    `json:"from"`
	To   string    `json:"to"`
	Diag string    `json:"diag"`
}

// TransitionHistoryDepth is the number of per-session transitions the
// engine retains for `show bfd session <peer>`. Small and fixed because
// the detail view is for incident triage, not historical analysis.
const TransitionHistoryDepth = 8

// ProfileState is one entry returned by Service.Profiles. Values are
// the resolved (post-default) profile parameters an operator would see
// after `show bfd profile <name>`.
type ProfileState struct {
	Name            string `json:"name"`
	DetectMult      uint8  `json:"detect-multiplier"`
	DesiredMinTxUs  uint32 `json:"desired-min-tx-us"`
	RequiredMinRxUs uint32 `json:"required-min-rx-us"`
	Passive         bool   `json:"passive"`
}

// StateLabel returns the canonical string for a packet.State as used in
// SessionState.State, TransitionRecord.From / .To, and Prometheus
// label values. Exposed so consumers format states identically to the
// engine.
func StateLabel(s packet.State) string { return s.String() }

// DiagLabel returns the canonical string for a packet.Diag.
func DiagLabel(d packet.Diag) string { return d.String() }
