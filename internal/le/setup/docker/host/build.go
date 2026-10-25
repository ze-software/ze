// Design: docs/architecture/appliance/docker-host.md -- le builds the docker-host image
// Related: static.go -- the source builds in the pinned builder container
// Related: extrafiles.go -- the extra files tar the image carries
// Related: perm.go -- formats /perm, which gok leaves unformatted on darwin
// Related: pins.go -- the pins and the image key

package dockerhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/appliance"
	buildgokrazy "github.com/ze-software/ze/internal/le/build/gokrazy"
)

const (
	// imageFileName is the image's name inside its cache entry.
	imageFileName = "docker-host.img"
	// instanceName is the gokrazy instance the image is built from.
	instanceName = "docker-host"
	// imageSizeGiBDefault is the image size when the build names none: room
	// for the lab images on /perm. The image file is sparse, so the Mac pays
	// only for the blocks written (spec A-10).
	imageSizeGiBDefault = 16
	// imageSizeGiBMin holds the boot partitions with room for /perm.
	imageSizeGiBMin = 2
	// imageSizeGiBMax bounds a typing mistake.
	imageSizeGiBMax = 1024
)

// buildRequest is one image build: the repository it reads, the arch, the
// pins and the size in GiB, which parseBuildRequest checked.
type buildRequest struct {
	repo    string
	arch    string
	pins    *Pins
	sizeGiB int
}

// builtImage is an image in the docker-host cache.
type builtImage struct {
	// Path is the raw image file.
	Path string `json:"image"`
	// Key is the cache key the inputs give (ImageKey).
	Key string `json:"key"`
	// Built is false when the cache already held the image and gok did not run.
	Built bool `json:"built"`
}

// builder runs the build. Each slow or external step is a field, so a test
// replaces the kernel, the downloads, the container and gok.
type builder struct {
	// cacheDir is the docker-host cache root, ~/.cache/ze/docker-host.
	cacheDir string
	// kernelTree resolves the runtime kernel cache entry for an arch.
	kernelTree func(arch string) (string, error)
	// fetch answers the cached, sum-checked file for a pin.
	fetch func(ctx context.Context, pins *Pins, pin Pin) (string, error)
	// runContainer runs the source builds in the builder container.
	runContainer func(ctx context.Context, run containerRun) error
	// gok builds the raw image with gok's own arguments.
	gok func(arch string, args []string) error
	// formatPerm gives the image's /perm partition an ext4 file system.
	formatPerm func(image string) error
	// progress receives one line per step.
	progress io.Writer
}

// build answers the image for req, from the cache when the key's entry holds
// it. Otherwise it fetches every pinned input first, so a sha256 mismatch
// refuses before anything is written (spec AC-5), then builds the static
// binaries, writes the extra files tar, runs gok into a partial file, formats
// /perm and moves the image into place.
func (b *builder) build(ctx context.Context, req buildRequest) (builtImage, error) {
	tree, err := b.kernelTree(req.arch)
	if err != nil {
		return builtImage{}, fmt.Errorf("resolve the %s runtime kernel: %w", req.arch, err)
	}
	digest, err := sourceDigest(req.repo, req.arch, req.sizeGiB)
	if err != nil {
		return builtImage{}, err
	}
	key, err := ImageKey(req.pins, KeyInputs{Arch: req.arch, KernelKey: filepath.Base(tree), SourceDigest: digest})
	if err != nil {
		return builtImage{}, err
	}
	dir := filepath.Join(b.cacheDir, req.arch, key)
	image := filepath.Join(dir, imageFileName)
	if _, err := os.Stat(image); err == nil {
		return builtImage{Path: image, Key: key}, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return builtImage{}, fmt.Errorf("inspect %s: %w", image, err)
	}

	inputs, err := b.fetchInputs(ctx, req)
	if err != nil {
		return builtImage{}, err
	}
	b.say("static binaries for " + req.arch)
	staticDir, err := b.staticBinaries(ctx, req, inputs.sources)
	if err != nil {
		return builtImage{}, err
	}
	tarPath := filepath.Join(req.repo, "gokrazy", instanceName, "extrafiles_"+req.arch+".tar")
	b.say("extra files " + tarPath)
	busybox := filepath.Join(req.repo, "cmd", "ze-serial-shell", "_gokrazy", "extrafiles_"+req.arch+".tar")
	if err := writeExtraFiles(tarPath, extraFileInputs{busyboxTar: busybox, staticDir: staticDir, dockerTgz: inputs.docker}); err != nil {
		return builtImage{}, err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // a cache directory in the user's home
		return builtImage{}, fmt.Errorf("create %s: %w", dir, err)
	}
	partial := image + ".partial-" + strconv.Itoa(os.Getpid())
	defer os.Remove(partial) //nolint:errcheck // gone already once renamed into place

	b.say("gok image " + partial)
	if err := b.gok(req.arch, gokArgs(req.repo, partial, req.sizeGiB)); err != nil {
		return builtImage{}, err
	}
	b.say("format /perm in " + partial)
	if err := b.formatPerm(partial); err != nil {
		return builtImage{}, err
	}
	if err := os.Rename(partial, image); err != nil {
		return builtImage{}, fmt.Errorf("move the image into %s: %w", image, err)
	}
	return builtImage{Path: image, Key: key, Built: true}, nil
}

// fetchedInputs are the sum-checked downloads a build reads.
type fetchedInputs struct {
	docker  string
	sources map[string]string
}

// fetchInputs fetches the arch's docker tarball and every source tarball.
func (b *builder) fetchInputs(ctx context.Context, req buildRequest) (fetchedInputs, error) {
	pin, err := req.pins.Download("docker", req.arch)
	if err != nil {
		return fetchedInputs{}, err
	}
	b.say("fetch " + pin.URL)
	docker, err := b.fetch(ctx, req.pins, pin)
	if err != nil {
		return fetchedInputs{}, err
	}
	sources := make(map[string]string, len(req.pins.Sources()))
	for _, source := range req.pins.Sources() {
		b.say("fetch " + source.URL)
		path, err := b.fetch(ctx, req.pins, source)
		if err != nil {
			return fetchedInputs{}, err
		}
		sources[source.Name] = path
	}
	return fetchedInputs{docker: docker, sources: sources}, nil
}

// say writes one progress line.
func (b *builder) say(line string) {
	_, _ = io.WriteString(b.progress, "docker-host: "+line+"\n") //nolint:errcheck // progress output only
}

// gokArgs are gok's arguments for a full raw image of sizeGiB at image,
// built from the repository's docker-host instance.
func gokArgs(repo, image string, sizeGiB int) []string {
	return []string{
		"--parent_dir", filepath.Join(repo, "gokrazy"),
		"-i", instanceName,
		"overwrite",
		"--full", image,
		"--target_storage_bytes", strconv.Itoa(sizeGiB << 30),
	}
}

// runGok builds through `le build gokrazy`'s Run, which prepares the instance
// with the arch's runtime kernel and the checked-in module cache.
func runGok(arch string, args []string) error {
	return runGokWith(buildgokrazy.Run, arch, args)
}

// runGokWith is runGok with the gok entry point as a parameter, so a test
// drives it with a runner that changes the environment the way gok does.
//
// The build has to change this process's environment: Run and the Go builds
// gok spawns read the image arch from GOARCH, Run sets CGO_ENABLED, GOMODCACHE,
// GOFLAGS and GOPROXY, and gok applies the instance's Environment (GOOS=linux)
// with os.Setenv. le does not exit after the build: `./le test qemu docker-lab`
// and `./le setup docker host start` go on to run commands that inherit
// os.Environ(), and a GOOS=linux reaching the host le makes its launcher build
// a linux binary. So the environment is restored to the snapshot taken here
// before runGokWith returns, whether Run answered success or failure. A failed
// image pack exits the process from inside gok (packer.Main), so that path never returns.
func runGokWith(run func(args []string) int, arch string, args []string) (err error) {
	saved := os.Environ()
	defer func() {
		if restoreErr := restoreEnviron(saved); restoreErr != nil && err == nil {
			err = restoreErr
		}
	}()

	if err := os.Setenv("GOARCH", arch); err != nil {
		return fmt.Errorf("set GOARCH for gok: %w", err)
	}
	if code := run(args); code != 0 {
		return fmt.Errorf("gok exited %d building the docker-host image (its output is above). "+
			"Fix: correct what gok names, then run `./le setup docker host build` again", code)
	}
	return nil
}

// restoreEnviron makes the process environment equal saved, an os.Environ()
// snapshot: it unsets every variable the snapshot does not hold and sets every
// one whose value differs. Variables already equal are left untouched, so the
// environment is never cleared, even for a moment.
func restoreEnviron(saved []string) error {
	want := make(map[string]string, len(saved))
	for _, entry := range saved {
		name, value, _ := strings.Cut(entry, "=")
		want[name] = value
	}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if _, kept := want[name]; kept {
			continue
		}
		if err := os.Unsetenv(name); err != nil {
			return fmt.Errorf("unset %s after the gok build: %w", name, err)
		}
	}
	for name, value := range want {
		if current, ok := os.LookupEnv(name); ok && current == value {
			continue
		}
		if err := os.Setenv(name, value); err != nil {
			return fmt.Errorf("restore %s after the gok build: %w", name, err)
		}
	}
	return nil
}

// digestRoots are the repository paths whose content shapes the image
// besides the pins and the kernel: the guest program and the instance, this
// package (which assembles the extra files, their modes and the static
// recipe), the instance preparation it calls (internal/appliance/instance)
// and the gok run that writes the image (internal/le/build/gokrazy).
var digestRoots = []string{
	"cmd/ze-docker-host",
	"internal/appliance/dockerhost",
	"gokrazy/docker-host",
	"internal/le/setup/docker/host",
	"internal/appliance/instance",
	"internal/le/build/gokrazy",
}

// sourceDigest digests every file under digestRoots (the extra files tar
// excepted, being an output, and test files and testdata, which no image
// carries), the arch's busybox tar, the builder recipe and the image size, so
// a change to any of them gives a new image key.
func sourceDigest(repo, arch string, sizeGiB int) (string, error) {
	var files []string
	for _, root := range digestRoots {
		err := filepath.WalkDir(filepath.Join(repo, root), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			if entry.IsDir() {
				return nil
			}
			if strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			if strings.HasPrefix(entry.Name(), "extrafiles_") {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("digest %s: %w", root, err)
		}
	}
	files = append(files, filepath.Join(repo, "cmd", "ze-serial-shell", "_gokrazy", "extrafiles_"+arch+".tar"))
	slices.Sort(files)

	hash := sha256.New()
	_, _ = io.WriteString(hash, "recipe\x00"+staticRecipe+"\x00size\x00"+strconv.Itoa(sizeGiB)+"\x00") //nolint:errcheck // a hash write never fails
	for _, path := range files {
		sum, err := fileSHA256(path)
		if err != nil {
			return "", fmt.Errorf("digest the image inputs: %w", err)
		}
		relative, err := filepath.Rel(repo, path)
		if err != nil {
			return "", err
		}
		_, _ = io.WriteString(hash, relative+"\x00"+sum+"\x00") //nolint:errcheck // a hash write never fails
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// dockerHostRuntimeKernel resolves the runtime kernel for an image of arch.
// Its refusal on a host that cannot build that arch names this image build,
// on an arch host or for this host's arch, never a bare kernel build, which
// would fill another host's cache.
func dockerHostRuntimeKernel(arch string) (string, error) {
	return appliance.ImageRuntimeKernelTree(arch, func(arch string) string {
		return "./le setup docker host build arch " + arch
	})
}

// systemBuilder is the builder that runs the real steps.
func systemBuilder(cacheDir string, fetcher Fetcher) *builder {
	return &builder{
		cacheDir:     cacheDir,
		kernelTree:   dockerHostRuntimeKernel,
		fetch:        fetcher.Fetch,
		runContainer: runBuilderContainer,
		gok:          runGok,
		formatPerm:   formatPerm,
		progress:     os.Stderr,
	}
}
