// Design: docs/architecture/container/runtime.md -- the created call, the ended call and the description contribution
// Related: supervise.go -- the supervisor, which makes the calls around a container's start and end
// Related: storeseam.go -- the other seam of this package

package container

import "context"

// A participant is a sibling package of this plugin that takes part in a
// container's start and end: the network package attaches the container's
// link at the created call and removes it at the ended call. It registers
// from its own init(), and this package never imports it.

// FileMount is a read-only file a participant contributes to a container's
// description. The participant owns the file and its directory: the runtime
// mounts it read-only and removes nothing it did not create.
type FileMount struct {
	// HostPath is the absolute path of the file on the host.
	HostPath string
	// ContainerPath is the absolute path of the file inside the container.
	ContainerPath string
}

// CreatedCall is what a participant learns at the created call. It carries a
// process id and a path, never an open descriptor into the container.
type CreatedCall struct {
	// Instance is the container's name.
	Instance string
	// InitProcessID is the process id of the container's init, on the host.
	InitProcessID int
	// NetworkNamespacePath names the container's own network namespace.
	NetworkNamespacePath string
}

// Participant takes part in the start and the end of every container. An
// implementation MUST be safe for concurrent use: containers start and end
// on their own supervisor workers.
type Participant interface {
	// Name is the participant's name in a container's state when one of its
	// calls fails.
	Name() string
	// Contribute answers the read-only file mounts this participant adds to
	// the container's description, before the container is created. It may
	// contribute mounts and nothing else.
	Contribute(ctx context.Context, instance string) ([]FileMount, error)
	// Created is called once, after the container is created and before its
	// entrypoint runs. The start continues only when it returns nil: an error
	// destroys the container and is recorded with Name in its state. It is
	// not called for a container that shares the host's network namespace,
	// which has no network namespace of its own to attach. It MUST return
	// when ctx ends.
	Created(ctx context.Context, call CreatedCall) error
	// Ended is called once for every container the runtime created, after
	// its processes are gone, and for each leftover the start-up sweep
	// removes. It is called whether or not Created was.
	Ended(ctx context.Context, instance string)
}

// participants is written only by RegisterParticipant, from package init
// functions, and read by the engine entry.
var participants []Participant

// RegisterParticipant adds a participant to every container's start and end.
// It MUST be called from a package init function. A nil participant, an
// empty name and a name registered twice are Ze defects and panic.
func RegisterParticipant(participant Participant) {
	if participant == nil {
		panic("BUG: container participant registered as nil")
	}
	name := participant.Name()
	if name == "" {
		panic("BUG: container participant registered with an empty name")
	}
	for _, registered := range participants {
		if registered.Name() == name {
			panic("BUG: container participant " + name + " registered twice")
		}
	}
	participants = append(participants, participant)
}
