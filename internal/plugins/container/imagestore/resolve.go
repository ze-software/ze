// Design: docs/architecture/container/image-store.md -- resolve: the store's answer to the runtime
// Related: state.go -- the binding and the generation record resolve reads
// Related: imageconfig.go -- the image configuration the ready answer carries
// Related: publish.go -- the completion mark that proves an image folder whole

package imagestore

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/internal/plugins/container/registry"
)

// pathForbidden holds the characters the runtime refuses in a folder it
// mounts, because the overlay's option syntax splits on them. A container
// name becomes a folder name, so it carries none of them.
const pathForbidden = ",:\\"

// Resolve answers Ready or Absent for one container.
//
// The digest a container runs is decided once: at its first start on an
// image of the store, the digest its reference's binding names is recorded
// as its current generation, and from then on that record decides, not the
// binding, so a re-fetch of a tag that moved changes nothing until an update
// runs. A config that names another reference starts a new generation on the
// digest that reference is bound to.
//
// An image no binding names is Absent with what the fetch queue knows of it.
// A digest whose folder the store no longer holds, or holds without its
// completion mark, is Absent too, never an error: a fetch brings it back.
//
// The request's storage size is not read: capped storage is a later phase of
// the store, and a request that declares volumes is refused until it lands.
func (s *Store) Resolve(ctx context.Context, request container.ResolveRequest) (container.Resolution, error) {
	if err := checkInstance(request.Instance); err != nil {
		return nil, err
	}
	if len(request.Volumes) != 0 {
		return nil, volumesNotImplemented(request.Instance)
	}
	reference, err := requestReference(request)
	if err != nil {
		return nil, err
	}
	engine, err := s.engine.engine()
	if err != nil {
		return nil, err
	}
	generation, recorded, err := readGeneration(ctx, engine, request.Instance)
	if err != nil {
		return nil, err
	}
	if recorded {
		// A record of another reference is a config that names another
		// image: the new reference's binding decides, as at a first start.
		if generation.reference == reference.String() {
			return s.resolveRecorded(request.Instance, reference, generation.current)
		}
	}
	return s.resolveBound(ctx, engine, request.Instance, reference)
}

// resolveRecorded answers the generation the container's record decides. A
// recorded digest the store lost is fetched back by digest: the tag may name
// another digest by now, and the record, not the tag, decides.
func (s *Store) resolveRecorded(instance string, reference registry.Reference, digest registry.Digest) (container.Resolution, error) {
	ready, present, err := s.readyAnswer(instance, digest)
	if err != nil {
		return nil, err
	}
	if present {
		return ready, nil
	}
	pinned := reference.Registry() + "/" + reference.Repository() + "@" + digest.String()
	return container.Absent{
		Reference: reference.String(),
		Reason:    container.AbsentImageLost,
		Detail: "container " + instance + " runs image " + digest.String() +
			" by its generation record, and the store no longer holds it whole: " + container.FixFetch(pinned),
	}, nil
}

// resolveBound answers the digest the reference's binding names, and records
// it as the container's current generation before the container starts.
func (s *Store) resolveBound(ctx context.Context, engine container.EngineClient, instance string, reference registry.Reference) (container.Resolution, error) {
	digest, bound, err := readBinding(ctx, engine, reference)
	if err != nil {
		return nil, err
	}
	if !bound {
		return s.unbound(reference), nil
	}
	ready, present, err := s.readyAnswer(instance, digest)
	if err != nil {
		return nil, err
	}
	if !present {
		return container.Absent{
			Reference: reference.String(),
			Reason:    container.AbsentImageLost,
			Detail: "image " + digest.String() + ", which " + reference.String() +
				" is bound to, is no longer whole in the store: " + container.FixFetch(reference.String()),
		}, nil
	}
	if err := writeGeneration(ctx, engine, instance, reference, digest); err != nil {
		return nil, err
	}
	return ready, nil
}

// unbound answers the absence of an image no binding names: its fetch is
// running or waiting, its last fetch failed, or no fetch of it was asked for.
func (s *Store) unbound(reference registry.Reference) container.Absent {
	if absent, queued := s.fetches.standing(reference); queued {
		return absent
	}
	return container.Absent{Reference: reference.String(), Reason: container.AbsentNeverFetched}
}

// readyAnswer answers the ready answer of instance on digest, after it
// created the writable folder, and false when the image folder of digest is
// absent or holds no completion mark.
func (s *Store) readyAnswer(instance string, digest registry.Digest) (container.Ready, bool, error) {
	architecture, err := registry.RunningArchitecture()
	if err != nil {
		return container.Ready{}, false, err
	}
	var ready container.Ready
	present := false
	err = s.withRoot(func(root *storeRoot) error {
		_, markErr := root.readMark(digest)
		if errors.Is(markErr, errNoMark) {
			return nil
		}
		if markErr != nil {
			return markErr
		}
		configured, configErr := root.readImageConfiguration(digest, architecture)
		if configErr != nil {
			return configErr
		}
		writable, folderErr := root.makeWritableFolder(instance, digest)
		if folderErr != nil {
			return folderErr
		}
		ready = configured
		ready.Digest = digest.String()
		ready.ImageFolder = filepath.Join(root.path, imagesFolder, digestFolder(digest), rootfsFolder)
		ready.WritableFolder = writable
		present = true
		return nil
	})
	if err != nil {
		return container.Ready{}, false, err
	}
	return ready, present, nil
}

// makeWritableFolder creates layers/<instance>/sha256-<hex>/ where it is
// absent and answers its absolute path. The runtime creates the overlay's
// upper and work folders inside it.
//
// It is created by the resolve, not by the fetch worker: the worker writes
// images, blobs and staging only, and a resolve that waited behind a fetch of
// minutes would hold its container's start for as long.
func (r *storeRoot) makeWritableFolder(instance string, digest registry.Digest) (string, error) {
	parent := filepath.Join(layersFolder, instance)
	layer := filepath.Join(parent, digestFolder(digest))
	if err := r.makeFolder(parent, "the writable layers of container "+instance); err != nil {
		return "", err
	}
	if err := r.makeFolder(layer, "the writable layer of container "+instance+" on "+digest.String()); err != nil {
		return "", err
	}
	return filepath.Join(r.path, layer), nil
}

// makeFolder creates the folder name beneath the store root with the store's
// mode, and refuses a name that holds anything but a folder. purpose says
// what the store keeps there.
func (r *storeRoot) makeFolder(name, purpose string) error {
	err := r.root.Mkdir(name, storeMode)
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrExist) {
		return storeCannotOpen(r.path, err)
	}
	info, err := r.root.Lstat(name)
	if err != nil {
		return storeCannotOpen(r.path, err)
	}
	if !info.IsDir() {
		folder := filepath.Join(r.path, name)
		return opererr.New(
			"the container store's "+folder+" is not a folder",
			"the store keeps "+purpose+" there",
			"rm "+folder+" && mkdir -m 0700 "+folder,
		)
	}
	return nil
}

// checkInstance refuses a container name that cannot name one folder of the
// store and one state key. The config's pattern admits none of these, so a
// refusal here is a name that reached the store unchecked.
func checkInstance(instance string) error {
	switch instance {
	case "", ".":
		return instanceRefused(instance, "it names no folder")
	}
	if filepath.Base(instance) != instance {
		return instanceRefused(instance, "it holds a path separator")
	}
	if strings.Contains(instance, "..") {
		return instanceRefused(instance, "it holds two dots in a row")
	}
	if strings.ContainsAny(instance, pathForbidden) {
		return instanceRefused(instance, "it holds a comma, a colon or a backslash, which the overlay mount cannot carry")
	}
	return nil
}

// instanceRefused is the refusal of a container name the store cannot use.
func instanceRefused(instance, why string) error {
	return opererr.New(
		"the image store cannot resolve container "+instance+": the name cannot name its writable layer",
		why,
		"rename the container instance in the configuration, then commit",
	)
}

// requestReference completes the reference the request names. An empty one
// is a config with no image, never a short name of the hub.
func requestReference(request container.ResolveRequest) (registry.Reference, error) {
	fix := "set container instance " + request.Instance + " image REFERENCE"
	if request.Reference == "" {
		return registry.Reference{}, opererr.New(
			"container "+request.Instance+" was not started: its configuration names no image",
			"the store resolves a container by the image reference its configuration names",
			fix,
		)
	}
	reference, err := registry.ParseReference(request.Reference)
	if err != nil {
		return registry.Reference{}, opererr.Wrap(err,
			"container "+request.Instance+" was not started: its image reference is not one the store can complete",
			"a reference is an optional registry host, a repository in lower case, and a tag or a digest",
			fix,
		)
	}
	return reference, nil
}

// volumesNotImplemented refuses a request that declares volumes: a volume is
// a folder with a size cap the kernel enforces, and capped storage is the
// store's phase 6, which waits on the appliance's partition format.
func volumesNotImplemented(instance string) error {
	return opererr.New(
		"container "+instance+" was not started: it declares volumes, and volumes are not implemented in this build",
		"a volume is a folder the image store creates with a size cap the kernel enforces, which arrives with the store's capped-storage phase",
		container.FixShowDoctor,
	)
}
