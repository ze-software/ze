// Design: docs/architecture/appliance/on-device-installer.md -- the ext4 project-quota leaf
// Detail: quota_linux.go -- the kernel calls
// Detail: quota_other.go -- the answers on a system that is not Linux

// Package projquota gives a folder a size cap that the kernel enforces: an
// ext4 project quota.
//
// It is a core leaf with no registry and no config lifecycle. A consumer asks
// whether the filesystem under a path enforces project quota, gives a folder a
// project id, sets a hard limit for that id and reads its usage. Every call is
// a system call of the caller's own process, which must run as root.
//
// The package is a wiring stub. Probe answers StateUnknown with the reason,
// every other operation answers ErrNotImplemented, and the kernel-facts phase
// of plan/post-release/features/spec-appliance-perm-grows-at-install.md
// replaces each one.
package projquota

import "errors"

// ErrNotImplemented is what every operation of this package answers until the
// quota code is written. A caller MUST treat it as a failure: no folder was
// marked and no limit was set.
var ErrNotImplemented = errors.New("projquota: the project-quota code is not implemented yet")

// ErrUnsupported is what every operation answers on a system that is not Linux.
var ErrUnsupported = errors.New("projquota: an ext4 project quota exists on Linux only")

// State says whether the filesystem under a path enforces project quota.
type State uint8

const (
	// StateUnspecified is the zero value and is never an answer.
	StateUnspecified State = iota
	// StateEnforced means the kernel refuses a write past a project's limit.
	StateEnforced
	// StateNotEnforced means the filesystem has the feature and enforcement
	// is off.
	StateNotEnforced
	// StateNoFeature means the filesystem was made without the feature.
	StateNoFeature
	// StateUnknown means the state could not be read. A consumer MUST NOT
	// read it as enforced.
	StateUnknown
)

// Status is the answer of Probe.
type Status struct {
	State State
	// Reason says why the state could not be read. It is set when State is
	// StateUnknown and empty otherwise.
	Reason string
}

// Usage is what a project uses and may use.
type Usage struct {
	// UsedBytes is the space the project's files hold.
	UsedBytes uint64
	// LimitBytes is the hard limit of the project.
	LimitBytes uint64
}
