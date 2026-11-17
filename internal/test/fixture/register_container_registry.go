// Design: docs/architecture/container/image-store.md -- the registry the fetch's functional tests reach
// Related: ../ociregistry/registry.go -- the in-process test registry it serves

package fixture

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/test/ociregistry"
)

// The images the fixture serves, under the names the hub completes the short
// names hello, probe and redis to, so a test fetches each as an operator
// would.
const (
	// containerRegistryHelloRepository is the image of one text file, for a
	// test that needs a fetch and no container.
	containerRegistryHelloRepository = "library/hello"
	containerRegistryTag             = "latest"
	containerRegistryFile            = "hello"
	containerRegistryContent         = "hello from the container registry fixture\n"
	// containerRegistryRedisRepository is the published Redis image, served
	// from the seeded layout and never built here.
	containerRegistryRedisRepository = "library/redis"
	// containerRegistryRedisWord is the second word of the fixture's command
	// line, written by a test that fetches the published Redis image.
	containerRegistryRedisWord = "redis"
	// containerRegistrySeedFix is how every refusal about the seed ends: the
	// reason the copy matters, and the command that writes it.
	containerRegistrySeedFix = ": the test fetches the published image from this registry, and the machine holds no usable copy of it. Run: " + ociregistry.SeedCommand
	// containerRegistryProbeRepository is the image whose entrypoint is the
	// container test probe, so a container of it runs and reports.
	containerRegistryProbeRepository = "library/probe"
	// containerRegistryProbeBinary is the probe program, found on the PATH
	// of the test step, where the harness puts it beside ze.
	containerRegistryProbeBinary = "ze-containerprobe"
	// containerRegistryReady is the line the test's ready= waits for.
	containerRegistryReady = "REGISTRY: listening on "
)

// containerRegistryLayerTime is the modification time of every layer entry,
// fixed so the same program always makes the same layer digest.
var containerRegistryLayerTime = time.Date(2026, time.November, 17, 0, 0, 0, 0, time.UTC)

func init() {
	Register("container/registry", containerRegistry)
}

// containerRegistry serves its images from the in-process test registry on
// 127.0.0.1:<port>, with the hub's anonymous bearer challenge, until the test
// ends it: library/hello:latest, whose one layer holds a text file,
// library/probe:latest, whose one layer holds the container test probe as its
// entrypoint, and library/redis:latest, the published Redis image, from the
// layout the seed action wrote. The daemon reaches it as the mirror of every
// registry through ze.test.container.registry.port, over plain HTTP. The probe
// program is found on the step's PATH, and a fixture that cannot find it
// refuses to start rather than serve some of its images.
//
// The Redis image is served when the checkout holds its seed. A test that
// fetches it writes the word redis after the port: the fixture then refuses to
// start without the seed, naming the command that writes it, so the test fails
// rather than fetch from a registry that does not hold the image.
func containerRegistry(ctx context.Context, args []string) error {
	redisRequired, err := containerRegistryWords(args)
	if err != nil {
		return err
	}
	port, err := strconv.ParseUint(args[0], 10, 16)
	if err != nil {
		return fmt.Errorf("container/registry: port %q: %w", args[0], err)
	}
	if port == 0 {
		return errors.New("container/registry: port 0 names no port the daemon can be told")
	}
	server, err := ociregistry.Start(ociregistry.Options{
		Auth:   ociregistry.AuthBearer,
		Listen: net.JoinHostPort("127.0.0.1", args[0]),
	})
	if err != nil {
		return fmt.Errorf("container/registry: %w", err)
	}
	defer server.Close()
	if err := containerRegistryImage(server); err != nil {
		return err
	}
	if err := containerRegistryProbeImage(server); err != nil {
		return err
	}
	if err := containerRegistryRedisImage(server, redisRequired); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(os.Stdout, containerRegistryReady+server.Host()); err != nil {
		return fmt.Errorf("container/registry: ready line: %w", err)
	}
	<-ctx.Done()
	return nil
}

// containerRegistryWords reads the fixture's command line, <port> or
// <port> redis, and answers whether the test requires the Redis image.
func containerRegistryWords(args []string) (bool, error) {
	usage := errors.New("container/registry: usage: le test fixture container/registry <port> [" + containerRegistryRedisWord + "]")
	if len(args) == 1 {
		return false, nil
	}
	if len(args) != 2 {
		return false, usage
	}
	if args[1] != containerRegistryRedisWord {
		return false, usage
	}
	return true, nil
}

// containerRegistryImage stores library/hello:latest: one text file, and a
// command that names it.
func containerRegistryImage(server *ociregistry.Registry) error {
	file := containerRegistryLayerFile{name: containerRegistryFile, mode: 0o644, content: []byte(containerRegistryContent)}
	process := map[string]any{"Cmd": []string{"/" + containerRegistryFile}}
	return containerRegistryPut(server, containerRegistryHelloRepository, file, process)
}

// containerRegistryRedisImage serves library/redis:latest from the layout the
// seed action wrote behind the checkout's cache link, which the QEMU guest
// reaches under its own mount of the checkout. The layers are streamed from
// that folder.
//
// With no seed the image is not served, and that is an error when the test
// required it. A seed that is there and cannot be served is an error either
// way: a layout that does not load, or one without the image of the
// architecture the fixture runs on.
func containerRegistryRedisImage(server *ociregistry.Registry, required bool) error {
	root := os.Getenv(envRepoRoot)
	if root == "" {
		return errors.New("container/registry: " + envRepoRoot + " is not set, so the seeded Redis image cannot be looked for: the functional runner sets it for every step")
	}
	layout := ociregistry.SeedLayoutDir(root)
	_, err := os.Stat(filepath.Join(layout, ociregistry.LayoutIndexFile))
	if errors.Is(err, fs.ErrNotExist) {
		if required {
			return errors.New("container/registry: the Redis image is not seeded at " + layout + containerRegistrySeedFix)
		}
		// Not required and not seeded: the fixture serves its other images.
		return nil
	}
	if err != nil {
		return fmt.Errorf("container/registry: the seeded Redis image: %w", err)
	}
	_, platforms, err := server.PutLayout(containerRegistryRedisRepository, containerRegistryTag, layout)
	if err != nil {
		return fmt.Errorf("container/registry: the seeded Redis image does not load (%w)%s", err, containerRegistrySeedFix)
	}
	held := slices.ContainsFunc(platforms, func(platform ociregistry.Platform) bool {
		return platform.Architecture == runtime.GOARCH
	})
	if !held {
		return errors.New("container/registry: the seeded Redis image at " + layout + " holds no " + runtime.GOARCH + " image" + containerRegistrySeedFix)
	}
	return nil
}

// containerRegistryProbeImage stores library/probe:latest: the probe program
// at /ze-containerprobe, executable, and the process configuration the
// container test store hands a container of its own fixture image, so a
// container of this image runs the probe's run mode and writes its report in
// its writable layer. The program is static (every build has CGO off), so the
// image needs nothing else.
func containerRegistryProbeImage(server *ociregistry.Registry) error {
	path, err := exec.LookPath(containerRegistryProbeBinary)
	if err != nil {
		return fmt.Errorf("container/registry: the probe image needs %s on the PATH, built beside ze by the functional harness: %w",
			containerRegistryProbeBinary, err)
	}
	program, err := os.ReadFile(path) //nolint:gosec // the probe the harness built
	if err != nil {
		return fmt.Errorf("container/registry: probe program: %w", err)
	}
	file := containerRegistryLayerFile{name: containerRegistryProbeBinary, mode: 0o755, content: program}
	process := map[string]any{
		"Entrypoint": []string{"/" + containerRegistryProbeBinary},
		"Cmd":        []string{"run"},
		"Env":        []string{"PATH=/"},
		"WorkingDir": "/",
	}
	return containerRegistryPut(server, containerRegistryProbeRepository, file, process)
}

// containerRegistryLayerFile is the one file of an image's one layer.
type containerRegistryLayerFile struct {
	name    string
	mode    int64
	content []byte
}

// containerRegistryPut stores repository:latest: one uncompressed layer
// holding file at the root, owned by the user the fixture runs as, which is
// the daemon's, and a configuration for the architecture the fixture was
// built for, with process as its process configuration.
func containerRegistryPut(server *ociregistry.Registry, repository string, file containerRegistryLayerFile, process map[string]any) error {
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	header := &tar.Header{
		Name:     file.name,
		Typeflag: tar.TypeReg,
		Mode:     file.mode,
		Size:     int64(len(file.content)),
		Uid:      os.Getuid(),
		Gid:      os.Getgid(),
		ModTime:  containerRegistryLayerTime,
		Format:   tar.FormatPAX,
	}
	if err := writer.WriteHeader(header); err != nil {
		return fmt.Errorf("container/registry: %s layer: %w", repository, err)
	}
	if _, err := writer.Write(file.content); err != nil {
		return fmt.Errorf("container/registry: %s layer: %w", repository, err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("container/registry: %s layer: %w", repository, err)
	}
	sum := sha256.Sum256(archive.Bytes())
	config, err := json.Marshal(map[string]any{
		"architecture": runtime.GOARCH,
		"os":           runtime.GOOS,
		"config":       process,
		"rootfs":       map[string]any{"type": "layers", "diff_ids": []string{"sha256:" + hex.EncodeToString(sum[:])}},
	})
	if err != nil {
		return fmt.Errorf("container/registry: %s configuration: %w", repository, err)
	}
	_, err = server.PutImage(repository, containerRegistryTag, ociregistry.Image{
		Architecture: runtime.GOARCH,
		Layers:       []ociregistry.Layer{{MediaType: ociregistry.MediaOCILayerTar, Content: archive.Bytes()}},
		Config:       config,
	})
	if err != nil {
		return fmt.Errorf("container/registry: %s image: %w", repository, err)
	}
	return nil
}
