// Design: docs/architecture/container/runtime.md -- the seam between the runtime and the image store
// Related: supervise.go -- the supervisor, which asks the store and receives its notices
// Related: register.go -- where a sibling package's registration is read

package container

import "context"

// The image store is a sibling package of this plugin and runs in its
// process, so the seam is a Go boundary inside the plugin tree. The store
// package registers a factory from its own init(); this package never imports
// it, and a build without it starts no container and says why.

// VolumeRequest is one volume the config declares for a container.
type VolumeRequest struct {
	// Name is the volume's name in the container's config.
	Name string
	// Destination is the absolute path the volume is mounted at inside the
	// container.
	Destination string
	// SizeMebibytes is the cap of the volume.
	SizeMebibytes uint32
}

// ResolveRequest asks the store to resolve one container: the image it runs
// and the folders it writes to.
type ResolveRequest struct {
	// Instance is the container's name.
	Instance string
	// Reference is the image reference the config names, as written.
	Reference string
	// StorageMebibytes is the cap of the container's writable layer.
	StorageMebibytes uint32
	// Volumes holds one entry for each volume the config declares.
	Volumes []VolumeRequest
}

// VolumeFolder is one volume of a resolved container: the folder the store
// created and capped for it.
type VolumeFolder struct {
	Name        string
	Destination string
	// Folder is the absolute path of the volume on the host.
	Folder string
}

// Resolution is the store's answer to a ResolveRequest: Ready or Absent, and
// nothing else. A consumer switches on the two types with no default.
type Resolution interface {
	resolution()
}

// Ready is the answer for an image the store holds. It carries the image's
// configuration as fetched: the store resolves nothing, and the runtime
// merges it with the container's config.
type Ready struct {
	// Digest is the image digest the container runs.
	Digest string
	// ImageFolder is the unpacked image, the read-only lower layer of the
	// container's root. An absolute path.
	ImageFolder string
	// WritableFolder is the one directory of this container and this digest,
	// holding the upper and work directories of the overlay. An absolute path.
	WritableFolder string
	// Volumes holds one folder for each volume of the request.
	Volumes []VolumeFolder
	// Entrypoint is the image's entrypoint.
	Entrypoint []string
	// Command is the image's default command. It is a list of its own, never
	// joined to Entrypoint.
	Command []string
	// Environment holds the variables the image declares, each "NAME=value".
	Environment []string
	// WorkingDirectory is the directory the entrypoint starts in.
	WorkingDirectory string
	// User is the image's user as written: a name, a number, either with a
	// group, or empty.
	User string
	// StopSignal is the signal the image declares for a stop, empty when it
	// declares none.
	StopSignal string
	// ExposedPorts holds the ports the image declares, for display only.
	ExposedPorts []string
	// DeclaredVolumes holds the paths the image declares as volumes. The store
	// makes no folder for them.
	DeclaredVolumes []string
}

func (Ready) resolution() {}

// AbsentReason says why the store does not hold an image. Zero is
// AbsentUnspecified, which no answer carries.
type AbsentReason uint8

const (
	AbsentUnspecified AbsentReason = iota
	// AbsentNeverFetched is an image no fetch was asked for.
	AbsentNeverFetched
	// AbsentFetchRunning is an image whose fetch has not ended.
	AbsentFetchRunning
	// AbsentFetchFailed is an image whose last fetch failed. Absent.Detail
	// says why.
	AbsentFetchFailed
	// AbsentImageLost is an image a binding or a generation record names,
	// whose folder the store no longer holds whole. Absent.Detail names the
	// digest and the fetch that brings it back.
	AbsentImageLost
)

// Absent is the answer for an image the store does not hold. It is never an
// empty value: it names the reference and the reason.
type Absent struct {
	// Reference is the whole reference the config's name completed to. The
	// store names the same reference when it later reports the image present.
	Reference string
	Reason    AbsentReason
	// Detail is why the last fetch failed, for AbsentFetchFailed, and the
	// digest with the fetch that brings it back, for AbsentImageLost.
	Detail string
}

func (Absent) resolution() {}

// Store is the image store's side of the seam: what the runtime asks of it.
// An implementation MUST be safe for concurrent use, because each container
// is resolved by its own supervisor worker.
type Store interface {
	// Resolve answers Ready or Absent for one container. An error is a
	// failure to answer, never an absent image.
	Resolve(ctx context.Context, request ResolveRequest) (Resolution, error)
	// Hold keeps the image and the folders of a Ready answer in place: the
	// store refuses to remove what is held. The caller MUST call the returned
	// release once, after the container's cleanup ended.
	Hold(instance string, resolved Ready) (release func())
}

// StoreNotices is the runtime's side of the seam: what the store tells it.
// The supervisor implements it, and the store receives it when it is built.
type StoreNotices interface {
	// ImagePresent reports that the image of reference is now in the store.
	// The runtime starts each container that waited for it and answers their
	// names.
	ImagePresent(ctx context.Context, reference string) (started []string)
	// RestartOnNewGeneration stops the container, resolves it again and
	// starts it. It answers when the container runs, or why it does not.
	RestartOnNewGeneration(ctx context.Context, instance string) error
	// StopThenStart stops the container, then starts it again, for the
	// store's clear command. It answers as RestartOnNewGeneration does.
	StopThenStart(ctx context.Context, instance string) error
}

// StoreFactory builds the store for one run of the plugin. It receives the
// notices the store sends, and it MUST start nothing: the plugin calls it
// from its engine entry, never from a declaration query.
type StoreFactory func(notices StoreNotices) Store

// storeFactory is written once, from the store package's init(), and read by
// the engine entry.
var storeFactory StoreFactory

// SetStoreFactory installs the image store of this build. The store package
// MUST call it from a package init function, once: a nil factory and a second
// call are Ze defects and panic.
func SetStoreFactory(factory StoreFactory) {
	if factory == nil {
		panic("BUG: container image store registered with no factory")
	}
	if storeFactory != nil {
		panic("BUG: container image store registered twice")
	}
	storeFactory = factory
}
