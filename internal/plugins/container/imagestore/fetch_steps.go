// Design: docs/architecture/container/image-store.md -- the fetch: resolve, the kept blobs, the free-space rule, application, publication
// Related: fetch.go -- the worker that runs these steps, one fetch at a time
// Related: fetch_apply_linux.go -- the application of the checked blobs
// Related: publish.go -- the publication the fetch ends with

package imagestore

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/internal/plugins/container/registry"
	"github.com/ze-software/ze/pkg/zefs"
)

// The free-space rule and the names a fetch uses in staging.
const (
	// reserveMin is the smallest free space a fetch leaves on the store's
	// filesystem, which the configuration also lives on.
	reserveMin = 64 << 20
	// reservePercent is the share of the filesystem the reserve grows to on
	// a filesystem larger than 1.25 GiB.
	reservePercent = 5
	// largestImageDefault is the default of store largest-image, the bound on
	// the expanded bytes of one image, until its config node lands.
	largestImageDefault = 1 << 30
	// blobDownloadPrefix names a blob being downloaded in staging. It takes
	// its digest as its name in blobs/ only once it was checked.
	blobDownloadPrefix = "blob-"
	// stagedImagePrefix names an image being applied in staging.
	stagedImagePrefix = "image-"
	// blobMode and fetchedMode are the modes of a kept blob and of the
	// manifest and configuration an image folder holds, byte for byte.
	blobMode    fs.FileMode = 0o600
	fetchedMode fs.FileMode = 0o400
)

// fetch runs one fetch on the store root, opened for it.
func (s *Store) fetch(job *fetchJob) (container.FetchAnswer, error) {
	var answer container.FetchAnswer
	err := s.withRoot(func(root *storeRoot) error {
		var fetchErr error
		answer, fetchErr = s.fetchInto(root, job)
		return fetchErr
	})
	return answer, err
}

// fetchInto resolves the job's reference and, unless the digest is already
// in the store, downloads the blobs it does not keep, applies them, and
// publishes the image.
func (s *Store) fetchInto(root *storeRoot, job *fetchJob) (container.FetchAnswer, error) {
	reference := job.reference
	config, err := s.clientConfig(reference)
	if err != nil {
		return container.FetchAnswer{}, err
	}
	client, err := registry.New(config)
	if err != nil {
		return container.FetchAnswer{}, &fetchError{code: container.CodeRegistryUnreachable, err: opererr.Wrap(err,
			"image "+reference.String()+" was not fetched: the client of registry "+reference.Registry()+" could not be built",
			"the store reaches a registry only through a client it checked",
			container.FixFetch(reference.String()),
		)}
	}
	defer client.Close()
	image, err := client.Resolve(job.ctx, reference)
	if err != nil {
		return container.FetchAnswer{}, registryFailed(reference, err)
	}
	digest := image.Digest()
	resolved := digest.String()
	job.resolved.Store(&resolved)
	mark, err := root.readMark(digest)
	if err == nil {
		return s.present(job, digest, mark.ExpandedBytes, false)
	}
	if !errors.Is(err, errNoMark) {
		return container.FetchAnswer{}, err
	}
	facts := imageFacts{
		Reference: reference.String(),
		Platform:  "linux/" + config.Architecture.String(),
		Registry:  reference.Registry(),
		Fetched:   time.Now().UTC(),
	}
	if err := s.download(root, job, client, image); err != nil {
		return container.FetchAnswer{}, err
	}
	staged := stagedImagePrefix + digestFolder(digest)
	if err := s.stage(root, staged, reference, image, &facts); err != nil {
		return container.FetchAnswer{}, errors.Join(err, root.removeStaged(staged))
	}
	if err := root.publish(staged, digest, facts); err != nil {
		return container.FetchAnswer{}, errors.Join(err, root.removeStaged(staged))
	}
	// The image is published, so the fetch succeeded: a blob left behind is
	// removed by the next fetch of another image, and is no failure of this
	// one.
	if err := root.removeBlobs(image.Layers()); err != nil {
		storeLog().Warn("container: the blobs of a published image could not be removed",
			"image", digest.String(), "error", err, "code", container.CodeStoreUnwritable)
	}
	return s.present(job, digest, facts.ExpandedBytes, true)
}

// present binds the job's reference to digest, whose image is in the store,
// then tells the runtime, which starts the containers that waited for it:
// their resolve reads the binding, so it is written first.
//
// A binding that cannot be written fails the fetch and keeps the image: the
// folder is whole and stays published, the listing shows it present beside
// the failed row of its reference, and no container is started, because its
// resolve would find no binding. The fix is the fetch again, which finds the
// digest present, downloads nothing and writes the binding.
func (s *Store) present(job *fetchJob, digest registry.Digest, size uint64, created bool) (container.FetchAnswer, error) {
	reference := job.reference.String()
	if err := s.bind(job, digest); err != nil {
		return container.FetchAnswer{}, err
	}
	started := s.notices.ImagePresent(job.ctx, reference)
	if started == nil {
		started = []string{}
	}
	return container.FetchAnswer{Reference: reference, Digest: digest.String(), Size: size, Created: created, Started: started}, nil
}

// bind writes the binding of the job's reference to digest. Its failure is
// a failed fetch under doctor-container-store-unwritable: the doctor table
// has no class of its own for a state write the engine refused.
func (s *Store) bind(job *fetchJob, digest registry.Digest) error {
	engine, err := s.engine.engine()
	if err != nil {
		return bindingNotRecorded(job.reference, digest, err)
	}
	if err := writeBinding(job.ctx, engine, job.reference, digest); err != nil {
		return bindingNotRecorded(job.reference, digest, err)
	}
	return nil
}

// bindingNotRecorded is the failure of a fetch whose image is published and
// whose binding was not written.
func bindingNotRecorded(reference registry.Reference, digest registry.Digest, cause error) error {
	whole := reference.String()
	return &fetchError{code: container.CodeStoreUnwritable, err: opererr.Wrap(cause,
		"image "+digest.String()+" is in the store, and its binding to "+whole+" was not recorded",
		"a container runs the digest its reference is bound to, so no container starts on the image until the binding is recorded",
		container.FixFetch(whole),
	)}
}

// download removes the blobs a stopped fetch of another image left, checks
// the free-space rule, and downloads each layer blob the store does not keep.
func (s *Store) download(root *storeRoot, job *fetchJob, client *registry.Client, image *registry.Image) error {
	layers := image.Layers()
	if err := root.removeLeftoverBlobs(layers); err != nil {
		return err
	}
	needed := uint64(len(image.Manifest()) + len(image.Config())) //nolint:gosec // G115: a length is never negative
	var missing []registry.Blob
	for _, layer := range layers {
		kept, err := root.blobKept(layer.Blob().Digest())
		if err != nil {
			return err
		}
		if kept {
			continue
		}
		missing = append(missing, layer.Blob())
		needed += uint64(layer.Blob().Size()) //nolint:gosec // G115: the registry package refuses a negative size
	}
	if err := s.checkSpace(root, job.reference, needed); err != nil {
		return err
	}
	for _, blob := range missing {
		if err := root.downloadBlob(job, client, blob); err != nil {
			return err
		}
	}
	return nil
}

// checkSpace refuses a fetch whose downloads would leave less than the
// reserve free on the store's filesystem. A filesystem whose space cannot be
// read refuses the fetch too: a fetch that does not know the space it may
// take could fill the filesystem the configuration lives on.
func (s *Store) checkSpace(root *storeRoot, reference registry.Reference, needed uint64) error {
	blocks, err := s.space(root.path)
	if err != nil {
		return &fetchError{code: container.CodeFetchNoSpace, err: opererr.Wrap(err,
			"image "+reference.String()+" was not fetched: the free space of the container store "+root.path+" cannot be read",
			"a fetch that does not know the space it takes could fill the filesystem the configuration lives on",
			container.FixShowDoctor,
		)}
	}
	free := blocks.Available * blocks.Size
	reserve := storeReserve(blocks.Total * blocks.Size)
	if free < reserve {
		return noSpace(reference, needed, free, reserve)
	}
	if free-reserve < needed {
		return noSpace(reference, needed, free, reserve)
	}
	return nil
}

// storeReserve answers the free space a fetch leaves on a filesystem of total
// bytes: the larger of 64 MiB and 5% of the filesystem.
func storeReserve(total uint64) uint64 {
	return max(uint64(reserveMin), total/100*reservePercent)
}

// noSpace is the refusal of a fetch that would cross the reserve.
func noSpace(reference registry.Reference, needed, free, reserve uint64) error {
	return &fetchError{code: container.CodeFetchNoSpace, err: opererr.New(
		"image "+reference.String()+" was not fetched: it needs "+textbuf.StringUint(needed)+
			" bytes, and the container store's filesystem has "+textbuf.StringUint(free)+
			" bytes free, of which "+textbuf.StringUint(reserve)+" are its reserve",
		"the reserve keeps the filesystem the configuration lives on writable, so no blob was downloaded",
		container.FixDeleteAllImages,
	)}
}

// blobPath answers the path, under the store root, of the kept blob digest
// names.
func blobPath(digest registry.Digest) string {
	return filepath.Join(blobsFolder, digestFolder(digest))
}

// blobKept answers whether the blob digest names is kept. It is trusted as
// it is: a blob takes its digest as its name only once it was checked.
func (r *storeRoot) blobKept(digest registry.Digest) (bool, error) {
	info, err := r.root.Lstat(blobPath(digest))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, storeCannotOpen(r.path, err)
	}
	if !info.Mode().IsRegular() {
		path := filepath.Join(r.path, blobPath(digest))
		return false, opererr.New(
			"the container store's "+path+" is not a file",
			"a kept blob is a file the store checked against its digest",
			"rm -rf "+path,
		)
	}
	return true, nil
}

// removeLeftoverBlobs removes every kept blob that is no layer of layers: the
// blobs of a stopped fetch of another image.
func (r *storeRoot) removeLeftoverBlobs(layers []registry.Layer) error {
	folder, err := r.root.Open(blobsFolder)
	if err != nil {
		return storeCannotOpen(r.path, err)
	}
	names, err := folder.Readdirnames(-1)
	if closeErr := folder.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return storeCannotOpen(r.path, err)
	}
	wanted := make([]string, 0, len(layers))
	for _, layer := range layers {
		wanted = append(wanted, digestFolder(layer.Blob().Digest()))
	}
	for _, name := range names {
		if slices.Contains(wanted, name) {
			continue
		}
		if err := r.root.RemoveAll(filepath.Join(blobsFolder, name)); err != nil {
			return storeCannotOpen(r.path, err)
		}
	}
	return nil
}

// removeBlobs removes the kept blobs of layers, once their image is
// published.
func (r *storeRoot) removeBlobs(layers []registry.Layer) error {
	var removeErr error
	for _, layer := range layers {
		err := r.root.Remove(blobPath(layer.Blob().Digest()))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		removeErr = errors.Join(removeErr, err)
	}
	return removeErr
}

// downloadBlob downloads blob into staging, counting its bytes into the
// job's received bytes, and gives it its digest as its name in blobs/ once
// the registry client checked it.
func (r *storeRoot) downloadBlob(job *fetchJob, client *registry.Client, blob registry.Blob) error {
	staging := filepath.Join(stagingFolder, blobDownloadPrefix+digestFolder(blob.Digest()))
	file, err := r.root.OpenFile(staging, os.O_RDWR|os.O_CREATE|os.O_EXCL, blobMode)
	if err != nil {
		return storeCannotOpen(r.path, err)
	}
	counted := &countedBlobFile{file: file, received: &job.received}
	if err := client.FetchBlob(job.ctx, job.reference, blob, counted); err != nil {
		return errors.Join(registryFailed(job.reference, err), file.Close(), r.root.Remove(staging))
	}
	if err := file.Sync(); err != nil {
		return errors.Join(storeCannotOpen(r.path, err), file.Close(), r.root.Remove(staging))
	}
	if err := file.Close(); err != nil {
		return errors.Join(storeCannotOpen(r.path, err), r.root.Remove(staging))
	}
	if err := zefs.RenameNoReplace(r.dir, staging, blobPath(blob.Digest())); err != nil {
		return errors.Join(storeCannotOpen(r.path, err), r.root.Remove(staging))
	}
	return nil
}

// countedBlobFile is the staging file of one blob, which counts the bytes
// written into it into the received bytes of its fetch. A restart of the
// copy, which empties the file, takes its bytes back out of the count.
type countedBlobFile struct {
	file     *os.File
	received *atomic.Uint64
	written  uint64
}

// Write writes into the file and counts what it wrote.
func (c *countedBlobFile) Write(p []byte) (int, error) {
	n, err := c.file.Write(p)
	c.written += uint64(n) //nolint:gosec // G115: a byte count is never negative
	c.received.Add(uint64(n))
	return n, err
}

// Seek seeks the file.
func (c *countedBlobFile) Seek(offset int64, whence int) (int64, error) {
	return c.file.Seek(offset, whence)
}

// Truncate cuts the file to size and takes the bytes cut off out of the
// count.
func (c *countedBlobFile) Truncate(size int64) error {
	if size < 0 {
		return errors.New("container image store: a blob cannot be cut to a negative size")
	}
	if err := c.file.Truncate(size); err != nil {
		return err
	}
	kept := uint64(size)
	if kept < c.written {
		c.received.Add(^(c.written - kept - 1))
		c.written = kept
	}
	return nil
}

// layerDigests is the part of an image configuration the store reads
// at fetch: the digest each decompressed layer must hash to. The names are
// the OCI image format's.
type layerDigests struct {
	RootFS struct {
		Type    string   `json:"type"`
		DiffIDs []string `json:"diff_ids"` //nolint:tagliatelle // the OCI image format's key
	} `json:"rootfs"`
}

// diffIDs answers the digest each of count layers must hash to once
// decompressed, from the image configuration config.
func diffIDs(reference registry.Reference, config []byte, count int) ([]registry.Digest, error) {
	var parsed layerDigests
	if err := json.Unmarshal(config, &parsed); err != nil {
		return nil, configRefused(reference, "the image configuration is not readable JSON", err)
	}
	if parsed.RootFS.Type != "layers" {
		return nil, configRefused(reference, "the image configuration's rootfs is not of type layers", nil)
	}
	if len(parsed.RootFS.DiffIDs) != count {
		return nil, configRefused(reference, "the image configuration declares "+textbuf.StringInt(int64(len(parsed.RootFS.DiffIDs)))+
			" layer digests for "+textbuf.StringInt(int64(count))+" layers", nil)
	}
	digests := make([]registry.Digest, 0, count)
	for _, text := range parsed.RootFS.DiffIDs {
		digest, err := registry.ParseDigest(text)
		if err != nil {
			return nil, configRefused(reference, "the image configuration declares a layer digest the store does not accept", err)
		}
		digests = append(digests, digest)
	}
	return digests, nil
}

// configRefused is the refusal of an image whose configuration does not say
// what its layers hash to.
func configRefused(reference registry.Reference, why string, cause error) error {
	return &fetchError{code: container.CodeImageRefused, err: opererr.Wrap(cause,
		"image "+reference.String()+" was refused",
		why+", so no layer can be checked once decompressed",
		container.FixFetch("REFERENCE"),
	)}
}

// stage applies the checked blobs of image under staging/staged and writes
// its manifest and configuration beside the root, byte for byte as fetched.
func (s *Store) stage(root *storeRoot, staged string, reference registry.Reference, image *registry.Image, facts *imageFacts) error {
	layers := image.Layers()
	digests, err := diffIDs(reference, image.Config(), len(layers))
	if err != nil {
		return err
	}
	if err := root.removeStaged(staged); err != nil {
		return err
	}
	if err := applyLayers(root, staged, reference, layers, digests, s.largestImage, facts); err != nil {
		return err
	}
	if err := root.writeFetched(staged, manifestName, image.Manifest()); err != nil {
		return err
	}
	return root.writeFetched(staged, configName, image.Config())
}

// writeFetched writes content, as fetched, under the staged folder.
func (r *storeRoot) writeFetched(staged, name string, content []byte) error {
	file, err := r.root.OpenFile(filepath.Join(stagingFolder, staged, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, fetchedMode)
	if err != nil {
		return storeCannotOpen(r.path, err)
	}
	if _, err := file.Write(content); err != nil {
		return errors.Join(storeCannotOpen(r.path, err), file.Close())
	}
	if err := file.Close(); err != nil {
		return storeCannotOpen(r.path, err)
	}
	return nil
}

// removeStaged removes the staged folder of an image that was not published.
func (r *storeRoot) removeStaged(staged string) error {
	if err := r.root.RemoveAll(filepath.Join(stagingFolder, staged)); err != nil {
		return storeCannotOpen(r.path, err)
	}
	return nil
}

// defaultClientConfig answers how the registry reference names is reached
// while no registry configuration exists: over verified TLS through the
// system roots, anonymously, with no mirror. The public hub needs no more. A
// private registry, its credentials, its trust anchor and its mirror wait
// for the registry list of the operator surface.
func defaultClientConfig(reference registry.Reference) (registry.Config, error) {
	architecture, err := registry.RunningArchitecture()
	if err != nil {
		return registry.Config{}, &fetchError{code: container.CodeImageRefused, err: opererr.Wrap(err,
			"image "+reference.String()+" was not fetched",
			"an image runs only on the architecture it was built for",
			container.FixShowDoctor,
		)}
	}
	return registry.Config{Registry: reference.Registry(), Transport: registry.TransportHTTPS, Architecture: architecture}, nil
}

// registryFailed maps a failed request of the registry client to the doctor
// code of its class and the fix that code's explanation names.
func registryFailed(reference registry.Reference, err error) error {
	text := reference.String()
	host := reference.Registry()
	failure, ok := errors.AsType[*registry.Error](err)
	if !ok {
		return &fetchError{code: container.CodeRegistryUnreachable, err: opererr.Wrap(err,
			"image "+text+" was not fetched", "the fetch stopped before registry "+host+" answered", container.FixFetch(text))}
	}
	var code, why, fix string
	switch failure.Failure() {
	case registry.FailureUnreachable:
		code, why, fix = container.CodeRegistryUnreachable, "registry "+host+" could not be reached, and the blobs checked so far stay for the next attempt", container.FixFetch(text)
	case registry.FailureAuthRefused:
		code, why, fix = container.CodeRegistryAuthRefused, "registry "+host+" refused the credentials or the access", container.FixRegistryCredentials(host)
	case registry.FailureNotFound:
		code, why, fix = container.CodeImageNotFound, "registry "+host+" holds no such repository, tag or digest", container.FixFetch("REFERENCE")
	case registry.FailureDigestMismatch:
		code, why, fix = container.CodeImageDigestMismatch, "what the registry served does not match its digest or its size, so none of it is stored", container.FixFetch(text)
	case registry.FailureRefused:
		code, why, fix = container.CodeImageRefused, "the image is one the store does not accept", container.FixFetch("REFERENCE")
	case registry.FailureUnspecified:
		panic("BUG: imagestore: a registry error with no failure class")
	}
	return &fetchError{code: code, err: opererr.Wrap(failure, "image "+text+" was not fetched", why, fix)}
}
