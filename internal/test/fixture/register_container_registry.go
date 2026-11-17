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
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/test/ociregistry"
)

// The images the fixture serves, under the names the hub completes the short
// names redis and probe to, so a test fetches each as an operator would.
const (
	containerRegistryRepository = "library/redis"
	containerRegistryTag        = "latest"
	containerRegistryFile       = "hello"
	containerRegistryContent    = "hello from the container registry fixture\n"
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

// containerRegistry serves two images from the in-process test registry on
// 127.0.0.1:<port>, with the hub's anonymous bearer challenge, until the test
// ends it: library/redis:latest, whose one layer holds a text file, and
// library/probe:latest, whose one layer holds the container test probe as its
// entrypoint. The daemon reaches it as the mirror of every registry through
// ze.test.container.registry.port, over plain HTTP. The probe program is
// found on the step's PATH, and a fixture that cannot find it refuses to
// start rather than serve one image of the two.
func containerRegistry(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("container/registry: usage: le test fixture container/registry <port>")
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
	if _, err := fmt.Fprintln(os.Stdout, containerRegistryReady+server.Host()); err != nil {
		return fmt.Errorf("container/registry: ready line: %w", err)
	}
	<-ctx.Done()
	return nil
}

// containerRegistryImage stores library/redis:latest: one text file, and a
// command that names it.
func containerRegistryImage(server *ociregistry.Registry) error {
	file := containerRegistryLayerFile{name: containerRegistryFile, mode: 0o644, content: []byte(containerRegistryContent)}
	process := map[string]any{"Cmd": []string{"/" + containerRegistryFile}}
	return containerRegistryPut(server, containerRegistryRepository, file, process)
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
