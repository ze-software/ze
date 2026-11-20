// Design: docs/architecture/testing/interop.md -- the persistent image cache.
// Related: docker.go -- Build consults this cache for a build that sets Cache.
// Related: docker_build_context.go -- the context walk the key hashes.
// Related: host_cpu.go -- the CPU identity a host-CPU-specific key carries.
package interoplab

import (
	"archive/tar"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/moby/patternmatcher"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/scratch"
)

// The persistent image cache keeps an image whose build is expensive (the VPP
// lab image compiles VPP for about twenty minutes) across runs, because the
// Docker host that built it may not keep it: `./le test qemu docker-lab` opens
// its guest's disk with snapshot=on, so the guest's image store is discarded
// at every shutdown. The owner's rule is that a lab never builds a reusable
// image when a build of the same inputs exists, that a build reaches the cache
// the moment it completes, and that two runs needing one build share it.
//
// The cache lives on the machine `le` runs on, under the per-user cache
// (scratch.CacheTarget, so XDG_CACHE_HOME moves it), in imageCacheDirName:
// one `docker image save` archive per key. The docker CLI writes and reads the
// archive on its own side, so a Docker daemon reached through a forwarded
// socket still saves to and loads from this machine.
//
// A hit loads the archive and then builds nothing but a one-line Dockerfile
// FROM the loaded image, carrying this run's owner labels and the build's two
// tags. The loaded image carries the labels of the run that built it, whose
// process is gone, so the orphan sweep would judge a run-owned tag on it
// abandoned and remove it from under the run that loaded it (orphans.go:
// sweepOne reads a tag's labels from its image). The relabel build compiles
// nothing and makes the result exactly what a build would have answered.

// imageCacheDirName is the cache's directory under the per-user cache.
const imageCacheDirName = "docker-image"

// imageCacheFormat opens every key, so a change to what the key hashes or to
// the archive's form starts new entries instead of loading old ones.
const imageCacheFormat = "ze-interop-image-cache-v1"

// The suffixes of one key's files in the cache directory.
const (
	imageCacheEntrySuffix   = ".tar"
	imageCacheLockSuffix    = ".lock"
	imageCachePartialSuffix = ".partial"
)

// imageCacheLoadRepository names the temporary tag a hit puts on the loaded
// image so the relabel Dockerfile can name it in FROM. It is not the
// run-owned repository, so the orphan sweep never judges it by the loaded
// image's stale labels; relabelCachedImage removes it after the relabel build.
const imageCacheLoadRepository = "ze-interop-cache-load"

// imageCacheLockPoll is how often a run waiting for another run's build of
// the same key tries the lock again.
const imageCacheLockPoll = 2 * time.Second

// loadedImagePrefix opens the line `docker image load` prints for an archive
// saved by image ID.
const loadedImagePrefix = "Loaded image ID: "

// errImageCacheMiss says the cache holds no entry for the key.
var errImageCacheMiss = errors.New("the image cache holds no entry for this key")

// imageCacheEntry names one key's files.
type imageCacheEntry struct {
	dir  string
	key  string
	path string
}

// hostImageCacheDir answers the cache directory of the machine le runs on.
func hostImageCacheDir() (string, error) {
	root, err := scratch.CacheTarget(os.Environ())
	if err != nil {
		return "", err
	}
	return filepath.Join(root, imageCacheDirName), nil
}

// fixtureImageCacheDir is the test double's answer: no cache, so a build that
// opts in is built and says why.
func fixtureImageCacheDir() (string, error) {
	return "", errors.New("the fixture Docker client keeps no image cache")
}

// fixtureHostCPU is the test double's fixed CPU identity.
func fixtureHostCPU() (string, error) {
	return "fixture-cpu", nil
}

// buildCached answers build from the cache when an entry for its key loads,
// and otherwise builds it and saves the result. A cache that cannot be used
// never fails the build: the build runs without it and one progress line
// says why. Only the build's own failure fails it.
func (d *Docker) buildCached(ctx context.Context, build ImageBuild, timeout time.Duration) (ImageResult, error) {
	entry, err := d.imageCacheEntry(ctx, build)
	if err != nil {
		imageCacheNotice(build, "is not looked up in the image cache: "+err.Error()+"; building it")
		return d.buildImage(ctx, build, timeout)
	}
	lock, err := entry.lock(ctx, timeout, func() {
		imageCacheNotice(build, "waits for another run that builds the same image (key "+entry.key+")")
	})
	if err != nil {
		if ctx.Err() != nil {
			return ImageResult{Name: build.Name, Tag: build.Tag}, err
		}
		imageCacheNotice(build, "is not looked up in the image cache: "+err.Error()+"; building it")
		return d.buildImage(ctx, build, timeout)
	}
	defer lock.Close() //nolint:errcheck // Closing drops the hold; nothing was written through it.
	entry.removeStaleFiles(build)

	image, err := d.loadCachedImage(ctx, build, entry, timeout)
	if err == nil {
		imageCacheNotice(build, "loaded from the image cache, key "+entry.key+" ("+image.Reference+")")
		return image, nil
	}
	if ctx.Err() != nil {
		return image, err
	}
	if errors.Is(err, errImageCacheMiss) {
		imageCacheNotice(build, "is not in the image cache (key "+entry.key+"); building it")
	} else {
		imageCacheNotice(build, "is built: "+err.Error())
	}
	image, err = d.buildImage(ctx, build, timeout)
	if err != nil {
		return image, err
	}
	d.saveCachedImage(ctx, build, entry, image)
	return image, nil
}

// imageCacheEntry derives build's key and names its files.
func (d *Docker) imageCacheEntry(ctx context.Context, build ImageBuild) (imageCacheEntry, error) {
	dir, err := d.imageCacheDir()
	if err != nil {
		return imageCacheEntry{}, err
	}
	key, err := d.imageCacheKey(ctx, build)
	if err != nil {
		return imageCacheEntry{}, err
	}
	return imageCacheEntry{dir: dir, key: key, path: filepath.Join(dir, key+imageCacheEntrySuffix)}, nil
}

// imageCacheKey is the SHA-256 of everything that decides the image: the
// Dockerfile's place and the main context exactly as Build streams it (its
// ignore file applied), each named context whole, the target, the build
// arguments in order, the architecture the Docker daemon builds for, and,
// for a host-CPU-specific image, this machine's CPU identity. Each value is
// written with its label and length, so no two lists hash alike. The owner
// labels and the tags are not inputs: they differ per run, and a hit applies
// this run's.
func (d *Docker) imageCacheKey(ctx context.Context, build ImageBuild) (string, error) {
	hash := sha256.New()
	writeKeyField(hash, "format", imageCacheFormat)
	source, err := prepareBuildContext(build.Context, build.Dockerfile)
	if err != nil {
		return "", err
	}
	writeKeyField(hash, "dockerfile", source.dockerfile)
	if err := hashBuildContext(hash, "context", source); err != nil {
		return "", err
	}
	for _, named := range build.NamedContexts {
		writeKeyField(hash, "named-context", named.Name)
		whole, err := wholeBuildContext(named.Path)
		if err != nil {
			return "", err
		}
		if err := hashBuildContext(hash, "named-context-entry", whole); err != nil {
			return "", err
		}
	}
	writeKeyField(hash, "target", build.Target)
	for _, argument := range build.BuildArgs {
		writeKeyField(hash, "build-arg", argument)
	}
	architecture, err := d.ServerArchitecture(ctx)
	if err != nil {
		return "", err
	}
	writeKeyField(hash, "server-architecture", architecture)
	if build.HostCPUSpecific {
		cpu, err := d.dockerHostCPU(ctx, architecture)
		if err != nil {
			return "", err
		}
		writeKeyField(hash, "host-cpu", cpu)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// dockerHostCPU answers the CPU identity a host-CPU-specific image is keyed
// by: the CPU of the machine le runs on. That machine's CPU stands for the
// Docker host's because the daemon a lab reaches is local: either on this
// machine, or in a VM this machine runs with its own CPU passed through
// (`./le test qemu docker-lab` starts its guest with `-cpu max` under KVM or
// HVF, Colima's VM likewise), so `-march=native` inside the build sees this
// CPU's model and flags. Two cases break that, and each refuses the cache
// rather than guess: a daemon of another architecture, which runs emulated,
// and an endpoint that is not a local socket or loopback address, which names
// another machine.
func (d *Docker) dockerHostCPU(ctx context.Context, architecture string) (string, error) {
	if architecture != runtime.GOARCH {
		return "", errors.New("the Docker daemon builds for " + architecture + " on this " + runtime.GOARCH +
			" machine, so it runs emulated and this machine's CPU is not the one the image is compiled for")
	}
	endpoint, err := d.dockerEndpoint(ctx)
	if err != nil {
		return "", err
	}
	if !localDockerEndpoint(endpoint) {
		return "", errors.New("the Docker endpoint " + endpoint + " is not a local socket or a loopback address, " +
			"so this machine's CPU is not known to be the Docker host's")
	}
	cpu, err := d.hostCPU()
	if err != nil {
		return "", fmt.Errorf("this machine's CPU identity cannot be read (%w), and a CPU-specific image is never reused unjudged", err)
	}
	return cpu, nil
}

// dockerEndpoint answers the daemon address the docker CLI uses: DOCKER_HOST
// in the client's environment, else the current context's endpoint.
func (d *Docker) dockerEndpoint(ctx context.Context) (string, error) {
	if endpoint := environmentValue(d.environ, "DOCKER_HOST"); endpoint != "" {
		return endpoint, nil
	}
	result, err := d.command(ctx, dockerCommandTimeout, DockerExecutable, "context", dockerInspectVerb, dockerFormatFlag, "{{.Endpoints.docker.Host}}")
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimSpace(result.Stdout)
	if endpoint == "" {
		return "", errors.New("`docker context inspect` named no Docker endpoint")
	}
	return endpoint, nil
}

// environmentValue answers name's value in environ, os.Environ syntax, the
// last assignment winning as it does for a process.
func environmentValue(environ []string, name string) string {
	value := ""
	for _, assignment := range environ {
		found, ok := strings.CutPrefix(assignment, name+"=")
		if ok {
			value = found
		}
	}
	return value
}

// localDockerEndpoint reports whether endpoint names a daemon reached through
// a socket on this machine or a loopback address.
func localDockerEndpoint(endpoint string) bool {
	if strings.HasPrefix(endpoint, "unix://") {
		return true
	}
	address, ok := strings.CutPrefix(endpoint, "tcp://")
	if !ok {
		return false
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	if host == "localhost" {
		return true
	}
	parsed, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return parsed.IsLoopback()
}

// writeKeyField writes one labeled, length-prefixed value into the key.
func writeKeyField(hash io.Writer, label, value string) {
	field := make([]byte, 0, len(label)+1+8+len(value))
	field = append(field, label...)
	field = append(field, 0)
	field = binary.BigEndian.AppendUint64(field, uint64(len(value)))
	field = append(field, value...)
	hash.Write(field) //nolint:errcheck,gosec // A hash.Hash never returns an error from Write.
}

// writeKeyNumber writes one labeled number into the key, as eight bytes.
func writeKeyNumber(hash io.Writer, label string, value int64) {
	field := make([]byte, 0, len(label)+1+8)
	field = append(field, label...)
	field = append(field, 0)
	field = binary.BigEndian.AppendUint64(field, uint64(value)) //nolint:gosec // The bits are hashed, never read back as a number.
	hash.Write(field)                                           //nolint:errcheck,gosec // A hash.Hash never returns an error from Write.
}

// hashBuildContext hashes source as Build streams it, through the same walk
// (streamBuildContext): each entry's name, type, link, mode, size and bytes, in
// walk order. The modification time is left out: a checkout sets it when it
// writes the file, and it changes nothing the image does.
func hashBuildContext(hash io.Writer, label string, source buildContext) error {
	_, err := streamBuildContext(source, func(archive io.Reader) (ProcessResult, error) {
		return ProcessResult{}, hashContextArchive(hash, label, archive)
	})
	return err
}

// hashContextArchive reads one context archive into the key. The loop is
// bounded by the archive, which the context walk wrote from a local directory.
func hashContextArchive(hash io.Writer, label string, archive io.Reader) error {
	reader := tar.NewReader(archive)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read the build context for the image cache key: %w", err)
		}
		writeKeyField(hash, label, header.Name)
		writeKeyField(hash, "type", string([]byte{header.Typeflag}))
		writeKeyField(hash, "link", header.Linkname)
		writeKeyNumber(hash, "mode", header.Mode)
		writeKeyNumber(hash, "size", header.Size)
		// The archive is the context Ze itself streamed, and each entry is
		// read only up to the size its header declares.
		if _, err := io.CopyN(hash, reader, header.Size); err != nil {
			return fmt.Errorf("read %s for the image cache key: %w", header.Name, err)
		}
	}
}

// wholeBuildContext prepares a named context to hash every entry of. Docker
// is handed a named context as a path, so no ignore file Ze reads decides
// what it may copy: the whole directory is hashed.
func wholeBuildContext(path string) (buildContext, error) {
	if path == "" {
		return buildContext{}, errors.New("Docker build context path is empty")
	}
	named, err := filepath.Abs(path)
	if err != nil {
		return buildContext{}, fmt.Errorf("resolve Docker build context %s: %w", path, err)
	}
	root, err := filepath.EvalSymlinks(named)
	if err != nil {
		return buildContext{}, fmt.Errorf("read Docker build context: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return buildContext{}, fmt.Errorf("read Docker build context: %w", err)
	}
	if !info.IsDir() {
		return buildContext{}, errors.New("Docker build context " + root + " is not a directory")
	}
	matcher, err := patternmatcher.New(nil)
	if err != nil {
		return buildContext{}, err
	}
	return buildContext{root: root, matcher: matcher}, nil
}

// staleFilePrefix opens the name of every temporary file or directory a run
// writes for this key while it holds the key's lock: a partial save and a
// relabel context.
func (e imageCacheEntry) staleFilePrefix() string {
	return "." + e.key + "-"
}

// removeStaleFiles removes the temporaries a killed run left for this key. The
// caller MUST hold the key's lock: every run writes them only under it, so any
// that exist belong to a run that is gone. A partial save can be gigabytes,
// and the cache has no other trim. A removal that fails is reported and
// changes nothing else.
func (e imageCacheEntry) removeStaleFiles(build ImageBuild) {
	entries, err := os.ReadDir(e.dir)
	if err != nil {
		imageCacheNotice(build, "warning: the image cache "+e.dir+" cannot be listed for a killed run's files: "+err.Error())
		return
	}
	for _, stale := range entries {
		if !strings.HasPrefix(stale.Name(), e.staleFilePrefix()) {
			continue
		}
		path := filepath.Join(e.dir, stale.Name())
		if err := os.RemoveAll(path); err != nil {
			imageCacheNotice(build, "warning: a killed run's file "+path+" was not removed: "+err.Error()+". Fix: rm -r "+path)
		}
	}
}

// lock takes the key's lock, waiting at most bound for another run that holds
// it, and calls waiting once when it has to wait. The holder builds and saves
// the same image, so the waiter then loads it rather than building it again.
// The caller MUST close the answered file to release the lock.
func (e imageCacheEntry) lock(ctx context.Context, bound time.Duration, waiting func()) (*os.File, error) {
	if err := os.MkdirAll(e.dir, 0o750); err != nil {
		return nil, fmt.Errorf("create the image cache %s: %w", e.dir, err)
	}
	path := filepath.Join(e.dir, e.key+imageCacheLockSuffix)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600) //nolint:gosec // the key's lock file in the image cache
	if err != nil {
		return nil, fmt.Errorf("open the image cache lock %s: %w", path, err)
	}
	deadline := time.NewTimer(bound)
	defer deadline.Stop()
	announced := false
	// Bounded by deadline: the holder's build runs under the same bound.
	for {
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) //nolint:gosec // a file descriptor fits an int
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			file.Close() //nolint:errcheck,gosec // The lock error is the answer.
			return nil, fmt.Errorf("lock %s: %w", path, err)
		}
		if !announced {
			announced = true
			waiting()
		}
		select {
		case <-ctx.Done():
			file.Close() //nolint:errcheck,gosec // The cancellation is the answer.
			return nil, ctx.Err()
		case <-deadline.C:
			file.Close() //nolint:errcheck,gosec // The timeout is the answer.
			return nil, errors.New("another run held " + path + " for longer than the build bound " + bound.String())
		case <-time.After(imageCacheLockPoll):
		}
	}
}

// loadCachedImage loads the key's archive and relabels it for this run. It
// answers errImageCacheMiss when no entry exists. An entry Docker refuses to
// load is removed, and the error says so; an entry that failed for another
// reason (a timeout, a cancel) is kept.
func (d *Docker) loadCachedImage(ctx context.Context, build ImageBuild, entry imageCacheEntry, timeout time.Duration) (ImageResult, error) {
	image := ImageResult{Name: build.Name, Tag: build.Tag}
	if _, err := os.Stat(entry.path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return image, errImageCacheMiss
		}
		return image, fmt.Errorf("the image cache entry %s cannot be read: %w", entry.path, err)
	}
	result, err := d.command(ctx, dockerImageArchiveTimeout, DockerExecutable, dockerImageObject, "load", "--quiet", "--input", entry.path)
	if err != nil {
		if !dockerRefused(err) {
			return image, fmt.Errorf("the image cache entry %s was not loaded: %w", entry.path, err)
		}
		return image, removeUnloadableEntry(entry, err)
	}
	loaded, found := loadedImageID(result.Stdout)
	if !found {
		return image, removeUnloadableEntry(entry, errors.New("`docker image load` printed no image ID"))
	}
	return d.relabelCachedImage(ctx, build, entry, loaded, timeout)
}

// dockerRefused reports whether err is docker exiting with a failure status,
// as opposed to the command not running or not finishing.
func dockerRefused(err error) bool {
	commandErr, ok := errors.AsType[*commandError](err)
	if !ok {
		return false
	}
	return commandErr.Cause == nil && commandErr.ExitCode != 0
}

// loadedImageID answers the image ID `docker image load` printed.
func loadedImageID(stdout string) (string, bool) {
	for line := range strings.SplitSeq(stdout, "\n") {
		id, found := strings.CutPrefix(strings.TrimSpace(line), loadedImagePrefix)
		if found && immutableImageID(id) {
			return id, true
		}
	}
	return "", false
}

// removeUnloadableEntry removes an entry Docker would not load, so the next
// save writes a good one, and answers the error that says so.
func removeUnloadableEntry(entry imageCacheEntry, cause error) error {
	err := os.Remove(entry.path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("the image cache entry %s did not load (%w) and could not be removed (%w); fix: rm %s", entry.path, cause, err, entry.path)
	}
	return fmt.Errorf("the image cache entry %s did not load (%w), so it was removed", entry.path, cause)
}

// relabelCachedImage builds FROM the loaded image with this run's owner labels,
// the build's Tag and a new run-owned tag, so the answer is what buildImage
// answers. It compiles nothing. The loaded image keeps no tag afterwards and
// stays as the base whose layers the relabeled image shares.
func (d *Docker) relabelCachedImage(ctx context.Context, build ImageBuild, entry imageCacheEntry, loaded string, timeout time.Duration) (ImageResult, error) {
	image := ImageResult{Name: build.Name, Tag: build.Tag}
	loadTag := imageCacheLoadRepository + ":" + strings.ToLower(rand.Text())
	if _, err := d.command(ctx, dockerCommandTimeout, DockerExecutable, "tag", loaded, loadTag); err != nil {
		return image, fmt.Errorf("the cached image %s was loaded but could not be tagged: %w", loaded, err)
	}
	relabeled, err := d.buildRelabel(ctx, build, entry, loadTag, timeout)
	if removeErr := d.removeImageTag(context.WithoutCancel(ctx), loadTag); removeErr != nil {
		err = errors.Join(err, fmt.Errorf("remove the cached image's temporary tag %s: %w", loadTag, removeErr))
	}
	if err != nil {
		if relabeled.Built {
			if releaseErr := d.ReleaseImage(context.WithoutCancel(ctx), relabeled); releaseErr != nil {
				err = errors.Join(err, releaseErr)
			}
		}
		return image, fmt.Errorf("the cached image %s was loaded but not tagged for this run: %w", loaded, err)
	}
	relabeled.Cached = true
	return relabeled, nil
}

// buildRelabel writes the one-line Dockerfile in a directory of its own and
// builds it through buildImage, which applies the labels and both tags.
func (d *Docker) buildRelabel(ctx context.Context, build ImageBuild, entry imageCacheEntry, base string, timeout time.Duration) (ImageResult, error) {
	relabel, err := os.MkdirTemp(entry.dir, entry.staleFilePrefix()+"relabel-")
	if err != nil {
		return ImageResult{}, fmt.Errorf("create the relabel context: %w", err)
	}
	defer os.RemoveAll(relabel) //nolint:errcheck // A leftover directory holds one line and is never read again.
	dockerfile := filepath.Join(relabel, buildContextDefaultDockerfile)
	if err := os.WriteFile(dockerfile, []byte("FROM "+base+"\n"), 0o600); err != nil {
		return ImageResult{}, fmt.Errorf("write the relabel Dockerfile: %w", err)
	}
	return d.buildImage(ctx, ImageBuild{Name: build.Name, Tag: build.Tag, Dockerfile: dockerfile, Context: relabel, Progress: build.Progress}, timeout)
}

// saveCachedImage saves a fresh build under its key: `docker image save` to a
// partial file in the cache directory, then one rename into place, so an entry
// that exists is whole and two runs never write one file. It runs without the
// caller's cancellation, because a build reaches the cache the moment it
// completes. A failure leaves the build answered and prints a warning.
func (d *Docker) saveCachedImage(ctx context.Context, build ImageBuild, entry imageCacheEntry, image ImageResult) {
	partial := filepath.Join(entry.dir, entry.staleFilePrefix()+strings.ToLower(rand.Text())+imageCachePartialSuffix)
	err := d.SaveImage(context.WithoutCancel(ctx), image.Reference, partial)
	if err == nil {
		err = os.Rename(partial, entry.path)
	}
	if err != nil {
		os.Remove(partial) //nolint:errcheck,gosec // An absent partial file is the goal.
		var tb textbuf.Buffer
		imageCacheNotice(build, tb.Str("warning: was built but not saved to the image cache: ").Str(err.Error()).
			Str(". The next run builds it again. Fix: free space under ").Str(entry.dir).
			Str(" (df -h ").Str(entry.dir).Str(") or make it writable, then run again").String())
		return
	}
	imageCacheNotice(build, "saved to the image cache, key "+entry.key)
}

// imageCacheNotice prints one line about the cache: on the build's Progress
// when it has one, which no build process writes to at that moment, and on
// stderr otherwise.
func imageCacheNotice(build ImageBuild, text string) {
	output := build.Progress
	if output == nil {
		output = os.Stderr
	}
	var tb textbuf.Buffer
	tb.Str("interop: image ").Str(build.Name).Byte(' ').Str(text).Byte('\n')
	output.Write(tb.Bytes()) //nolint:errcheck,gosec // A progress line that cannot be written changes no result.
}
