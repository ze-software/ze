// Design: docs/architecture/chaos-web-dashboard.md — multi-target config generation

package scenario

import "fmt"

// Target identifies the BGP daemon that ze-chaos generates config for and forks.
type Target string

const (
	TargetZe   Target = "ze"
	TargetFRR  Target = "frr"
	TargetBIRD Target = "bird"
)

// DefaultBinary returns the default binary name for a target, searched in PATH.
func (t Target) DefaultBinary() string {
	switch t {
	case TargetFRR:
		return "bgpd"
	case TargetBIRD:
		return "bird"
	default:
		return "ze"
	}
}

// SinglePort returns true when the target listens on a single BGP port
// for all peers (identified by source address), as opposed to Ze's
// per-peer port model.
func (t Target) SinglePort() bool {
	return t == TargetFRR || t == TargetBIRD
}

// ParseTarget parses a target string, returning an error for unknown values.
func ParseTarget(s string) (Target, error) {
	switch Target(s) {
	case TargetZe, TargetFRR, TargetBIRD:
		return Target(s), nil
	default:
		return "", fmt.Errorf("unknown target %q (valid: ze, frr, bird)", s)
	}
}
