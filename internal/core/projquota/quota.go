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
// Safe for concurrent use: no function holds state.
package projquota

import "errors"

// ErrUnsupported is what every operation answers on a system that is not Linux.
var ErrUnsupported = errors.New("projquota: an ext4 project quota exists on Linux only")

// ErrDefaultProject is what an operation answers for project id 0, the
// project every file belongs to until it is given one.
var ErrDefaultProject = errors.New("projquota: project id 0 is the default project and takes no limit")

// ErrLimitTooSmall is what SetLimit answers for a limit under LimitBytesMin.
// A limit of zero would mean no limit at all.
var ErrLimitTooSmall = errors.New("projquota: a hard limit is at least 4096 bytes")

// ErrLimitNotWhole is what SetLimit answers for a limit that is not a whole
// number of the kernel's 1024-byte quota blocks.
var ErrLimitNotWhole = errors.New("projquota: a hard limit is a whole number of 1024-byte quota blocks")

// ErrFolderNotEmpty is what AssignProject answers for a folder that holds an
// entry: only entries made after the assignment inherit the project, so the
// ones already there would escape the limit.
var ErrFolderNotEmpty = errors.New("projquota: a folder is given a project while it is empty")

// ErrNoFeature is what Enable answers on a filesystem without project quota
// accounting.
var ErrNoFeature = errors.New("projquota: the filesystem has no project quota accounting")

// LimitBytesMin is the smallest hard limit SetLimit accepts: one block of the
// persistent partition's format.
const LimitBytesMin = 4096

// State says whether the filesystem under a path enforces project quota.
type State uint8

const (
	// StateUnspecified is the zero value and is never an answer.
	StateUnspecified State = iota
	// StateEnforced means the kernel refuses a write past a project's limit.
	StateEnforced
	// StateNotEnforced means the filesystem accounts project usage and
	// enforcement is off.
	StateNotEnforced
	// StateNoFeature means the filesystem accounts no project usage: it was
	// made without the quota feature or without a project quota inode.
	StateNoFeature
	// StateUnknown means the state could not be read. A consumer MUST NOT
	// read it as enforced.
	StateUnknown
)

// String answers the state's word in `show host storage`.
func (s State) String() string {
	switch s {
	case StateEnforced:
		return "enforced"
	case StateNotEnforced:
		return "not-enforced"
	case StateNoFeature:
		return "no-feature"
	case StateUnknown:
		return "unknown"
	case StateUnspecified:
		return "unspecified"
	}
	return "invalid"
}

// Status is the answer of Probe.
type Status struct {
	State State
	// Reason says why the state could not be read. It is set when State is
	// StateUnknown and empty otherwise.
	Reason string
}

// Usage is what a project uses and may use.
type Usage struct {
	// UsedBytes is the space the project's files hold, space the kernel has
	// reserved for delayed allocation included.
	UsedBytes uint64
	// LimitBytes is the hard limit of the project; zero is no limit.
	LimitBytes uint64
}
