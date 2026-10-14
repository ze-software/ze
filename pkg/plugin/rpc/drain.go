// Design: docs/architecture/api/process-protocol.md — the stop drain RPC
// Related: types.go — Stage 1 registration carrying DrainBudget

package rpc

import (
	"errors"
	"fmt"
	"time"
)

// MethodDrain is the engine-to-plugin callback that asks a plugin to make its
// protocol unattractive before the daemon stops (the stop drain). The engine
// sends it once per stop, in parallel, to every plugin whose Stage 1
// registration declared a DrainBudget, after config changes are fenced and
// before any plugin connection closes.
const MethodDrain = "ze-plugin-callback:drain"

// The drain ceiling and the teardown margin are declared once, here. The
// daemon bounds the whole drain by DrainCeilingSeconds counted from the start
// of the stop, Stage 1 refuses a DrainBudget above it, and the systemd unit's
// stop timeout is derived from the sum of the two, so the supervisor never
// kills ze while it drains or tears down. Owner decision 2026-10-13 (Q-1 a).
const (
	DrainCeilingSeconds        = 60
	DrainTeardownMarginSeconds = 30
)

// DrainKind is how the daemon stops, which each plugin reads to decide what
// its protocol does. Zero is DrainKindUnspecified, which no stop sends.
type DrainKind uint8

const (
	DrainKindUnspecified DrainKind = iota
	// DrainKindStop is a stop with no return expected: drain, then leave.
	DrainKindStop
	// DrainKindRestart is a restart or reboot under graceful restart: a
	// protocol that holds its forwarding state across the restart (BGP,
	// RFC 4724 Section 5) answers at once and drains nothing.
	DrainKindRestart
)

// String returns the wire name of the kind.
func (k DrainKind) String() string {
	switch k {
	case DrainKindStop:
		return "stop"
	case DrainKindRestart:
		return "restart"
	case DrainKindUnspecified:
		return "unspecified"
	}
	// The value arrives from outside on decode only through UnmarshalText,
	// which refuses an unknown name, so this is a value no decode produced.
	return "unspecified"
}

// MarshalText encodes the kind as its wire name. Unspecified is refused, so
// the engine never sends a drain without a kind.
func (k DrainKind) MarshalText() ([]byte, error) {
	switch k {
	case DrainKindStop, DrainKindRestart:
		return []byte(k.String()), nil
	case DrainKindUnspecified:
		return nil, errors.New("drain kind unspecified")
	}
	return nil, fmt.Errorf("drain kind %d unknown", uint8(k))
}

// UnmarshalText decodes a wire name. The set is open on the wire, so an
// unknown name is an error, never a guessed kind.
func (k *DrainKind) UnmarshalText(text []byte) error {
	switch string(text) {
	case "stop":
		*k = DrainKindStop
	case "restart":
		*k = DrainKindRestart
	default:
		return fmt.Errorf("drain kind %q unknown (want stop or restart)", text)
	}
	return nil
}

// DrainRestartCause names why a restart-kind drain restarts, so a protocol
// that announces its restart reason (OSPF Grace-LSA, RFC 3623 Appendix A) can
// choose it. It is set only on DrainKindRestart.
type DrainRestartCause string

const (
	// DrainRestartInPlace is a restart of the daemon on the same host.
	DrainRestartInPlace DrainRestartCause = "in-place"
	// DrainRestartReboot is a restart through a host reboot.
	DrainRestartReboot DrainRestartCause = "reboot"
)

// DrainInput is the input for MethodDrain. It carries only what the stop
// drain reads: the kind, the restart cause on a restart, and the deadline
// after which the engine stops waiting for the answer.
type DrainInput struct {
	Kind         DrainKind         `json:"kind"`
	RestartCause DrainRestartCause `json:"restart-cause,omitempty"`
	Deadline     time.Time         `json:"deadline"`
}

// DrainOutput is the answer to MethodDrain. A plugin answers once its
// protocol drained or the deadline passed; an error answer is a failed drain.
type DrainOutput struct{}
