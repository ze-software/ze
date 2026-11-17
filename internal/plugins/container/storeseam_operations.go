// Design: docs/architecture/container/image-store.md -- the image commands' side of the seam
// Related: storeseam.go -- the seam the runtime asks: Resolve and Hold
// Related: image_cmd.go -- the command handlers that call these operations

package container

import (
	"context"
	"errors"
	"time"
)

// PluginName is the plugin's name in the registry. The image store package
// names it when it grants the plugin its state keys.
const PluginName = pluginName

// ImageOperations is what the image commands ask of the store. A Store the
// factory built MAY also be an ImageOperations: the runtime's seam stays
// Resolve and Hold, so a store that answers no image command still runs
// containers. The image commands refuse, naming the reason, when the store of
// this build is not one. An implementation MUST be safe for concurrent use:
// the commands arrive from every surface at once.
//
// Every operation takes the request's context first. A fetch and an update
// run under it, so a caller that goes away stops them. Each answers a typed
// value or an error, never both, and an error is an operator error that names
// what is wrong, why, and the command that fixes it.
type ImageOperations interface {
	// FetchImage fetches reference and answers once, at the end.
	FetchImage(ctx context.Context, reference string) (FetchAnswer, error)
	// ListImages answers one row for each image, each fetch in flight and
	// each failed fetch.
	ListImages(ctx context.Context) ([]ImageRow, error)
	// StoreTotals answers the store's place, its space and its layers.
	StoreTotals(ctx context.Context) (StoreTotals, error)
	// ListVolumes answers one row for each stored volume.
	ListVolumes(ctx context.Context) ([]VolumeRow, error)
	// UpdateImage resolves the reference instance's config names again,
	// fetches it, and moves the container to it when the digest changed.
	UpdateImage(ctx context.Context, instance string) (UpdateAnswer, error)
	// RollbackImage swaps the current and the previous generation of
	// instance, and restarts it.
	RollbackImage(ctx context.Context, instance string) (RollbackAnswer, error)
	// DeleteImage deletes the one image digest names, unless a configured
	// container holds it.
	DeleteImage(ctx context.Context, digest string) (DeleteAnswer, error)
	// DeleteAllImages deletes every image no configured container holds, and
	// the leftover blobs.
	DeleteAllImages(ctx context.Context) (DeleteAnswer, error)
	// DeleteVolume deletes the volume of instance, unless the config still
	// declares it.
	DeleteVolume(ctx context.Context, instance, volume string) (VolumeDeleteAnswer, error)
	// ClearLayer empties the writable layer of instance, configured or not.
	ClearLayer(ctx context.Context, instance string) (ClearAnswer, error)
}

// StoreChecker is what the doctor check container-store asks of the store. A
// Store the factory built MAY also be one: a store with no folder of its own
// has nothing for the check to find.
type StoreChecker interface {
	// CheckStore answers nil when the store root can be used now, and an
	// operator error that names the folder, why it matters and the command
	// that fixes it when it cannot. It MUST change nothing, because doctor
	// only reads. Safe for concurrent use.
	CheckStore() error
}

// ImageState is the state of one row of the image listing. Zero is
// ImageStateUnspecified, which no row carries.
type ImageState uint8

const (
	ImageStateUnspecified ImageState = iota
	// ImageStatePresent is an image in the store, bound to its reference.
	ImageStatePresent
	// ImageStateFetching is a fetch that has not ended.
	ImageStateFetching
	// ImageStateFailed is the last fetch of a reference, which failed.
	ImageStateFailed
	// ImageStatePrevious is an image a container holds as its previous
	// generation.
	ImageStatePrevious
)

// errStateUnspecified refuses to write a state nobody set: a row that carries
// it is a store defect, and an empty or guessed word would read as data.
var errStateUnspecified = errors.New("container: a listing row carries no state")

// MarshalText answers the word the listing shows.
func (s ImageState) MarshalText() ([]byte, error) {
	switch s {
	case ImageStatePresent:
		return []byte("present"), nil
	case ImageStateFetching:
		return []byte("fetching"), nil
	case ImageStateFailed:
		return []byte("failed"), nil
	case ImageStatePrevious:
		return []byte("previous"), nil
	case ImageStateUnspecified:
		return nil, errStateUnspecified
	}
	return nil, errStateUnspecified
}

// VolumeState is the state of one row of the volume listing. Zero is
// VolumeStateUnspecified, which no row carries.
type VolumeState uint8

const (
	VolumeStateUnspecified VolumeState = iota
	// VolumeStateInUse is a volume the config declares.
	VolumeStateInUse
	// VolumeStateOrphan is a volume of a container or a volume the config no
	// longer holds.
	VolumeStateOrphan
)

// MarshalText answers the word the listing shows.
func (s VolumeState) MarshalText() ([]byte, error) {
	switch s {
	case VolumeStateInUse:
		return []byte("in-use"), nil
	case VolumeStateOrphan:
		return []byte("orphan"), nil
	case VolumeStateUnspecified:
		return nil, errStateUnspecified
	}
	return nil, errStateUnspecified
}

// FetchAnswer is the answer of create container image reference. Sizes are
// bytes.
type FetchAnswer struct {
	// Reference is the whole reference the operator's name completed to.
	Reference string `json:"reference"`
	Digest    string `json:"digest"`
	Size      uint64 `json:"size"`
	// Created is false when the digest was already in the store.
	Created bool `json:"created"`
	// Started names the containers that waited for this image and started.
	Started []string `json:"started"`
}

// ImageRow is one row of show container image list.
type ImageRow struct {
	Reference string     `json:"reference"`
	Digest    string     `json:"digest,omitempty"`
	Size      uint64     `json:"size"`
	Fetched   time.Time  `json:"fetched,omitzero"`
	State     ImageState `json:"state"`
	// Received is the bytes a fetch in flight received so far.
	Received uint64 `json:"received"`
	// UsedBy names the containers that hold the image.
	UsedBy []string `json:"used-by"`
	// Error and Code say why a failed fetch failed, and its doctor code.
	Error string `json:"error,omitempty"`
	Code  string `json:"code,omitempty"`
}

// StoreTotals is the answer of show container store. Sizes are bytes.
type StoreTotals struct {
	Path    string `json:"path"`
	Total   uint64 `json:"total"`
	Free    uint64 `json:"free"`
	Reserve uint64 `json:"reserve"`
	// CapsEnforced says whether the kernel enforces the size caps of the
	// layers and the volumes on the store's filesystem.
	CapsEnforced bool       `json:"caps-enforced"`
	Layers       []LayerRow `json:"layers"`
}

// LayerRow is one writable layer of the store: its cap and its use.
type LayerRow struct {
	Instance string `json:"instance"`
	Digest   string `json:"digest"`
	Size     uint64 `json:"size"`
	Used     uint64 `json:"used"`
}

// VolumeRow is one row of show container volume list. Sizes are bytes.
type VolumeRow struct {
	Instance string      `json:"instance"`
	Name     string      `json:"name"`
	Size     uint64      `json:"size"`
	Used     uint64      `json:"used"`
	State    VolumeState `json:"state"`
}

// UpdateAnswer is the answer of update container image name.
type UpdateAnswer struct {
	Name           string `json:"name"`
	PreviousDigest string `json:"previous-digest"`
	Digest         string `json:"digest"`
	// Changed is false when the reference resolved to the current digest.
	Changed   bool `json:"changed"`
	Restarted bool `json:"restarted"`
}

// RollbackAnswer is the answer of update container image rollback name.
type RollbackAnswer struct {
	Name           string `json:"name"`
	PreviousDigest string `json:"previous-digest"`
	Digest         string `json:"digest"`
	Restarted      bool   `json:"restarted"`
}

// DeleteAnswer is the answer of delete container image digest and delete
// container image all: what was removed, and what the delete rule kept.
type DeleteAnswer struct {
	Deleted []DeletedImage `json:"deleted"`
	Kept    []KeptImage    `json:"kept"`
}

// DeletedImage is one image a delete removed. Size is bytes.
type DeletedImage struct {
	Digest string `json:"digest"`
	Size   uint64 `json:"size"`
}

// KeptImage is one image the delete rule kept, and the container and
// generation that hold it.
type KeptImage struct {
	Digest string `json:"digest"`
	Reason string `json:"reason"`
}

// VolumeDeleteAnswer is the answer of delete container volume name. Size is
// bytes.
type VolumeDeleteAnswer struct {
	Instance string `json:"instance"`
	Name     string `json:"name"`
	Size     uint64 `json:"size"`
}

// ClearAnswer is the answer of clear container layer name.
type ClearAnswer struct {
	Name      string `json:"name"`
	Cleared   bool   `json:"cleared"`
	Restarted bool   `json:"restarted"`
}
