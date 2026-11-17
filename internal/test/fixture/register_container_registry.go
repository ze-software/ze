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
	"runtime"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/test/ociregistry"
)

// The image the fixture serves, under the name the hub completes the short
// name redis to, so a test fetches it as an operator would.
const (
	containerRegistryRepository = "library/redis"
	containerRegistryTag        = "latest"
	containerRegistryFile       = "hello"
	containerRegistryContent    = "hello from the container registry fixture\n"
	// containerRegistryReady is the line the test's ready= waits for.
	containerRegistryReady = "REGISTRY: listening on "
)

func init() {
	Register("container/registry", containerRegistry)
}

// containerRegistry serves one small image from the in-process test registry
// on 127.0.0.1:<port>, with the hub's anonymous bearer challenge, until the
// test ends it. The daemon reaches it as the mirror of every registry
// through ze.test.container.registry.port, over plain HTTP.
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
	if _, err := fmt.Fprintln(os.Stdout, containerRegistryReady+server.Host()); err != nil {
		return fmt.Errorf("container/registry: ready line: %w", err)
	}
	<-ctx.Done()
	return nil
}

// containerRegistryImage stores the image: one uncompressed layer holding
// one file owned by the user the fixture runs as, which is the daemon's, and
// a configuration for the architecture the fixture was built for.
func containerRegistryImage(server *ociregistry.Registry) error {
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	header := &tar.Header{
		Name:     containerRegistryFile,
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     int64(len(containerRegistryContent)),
		Uid:      os.Getuid(),
		Gid:      os.Getgid(),
		ModTime:  time.Date(2026, time.November, 17, 0, 0, 0, 0, time.UTC),
		Format:   tar.FormatPAX,
	}
	if err := writer.WriteHeader(header); err != nil {
		return fmt.Errorf("container/registry: layer: %w", err)
	}
	if _, err := writer.Write([]byte(containerRegistryContent)); err != nil {
		return fmt.Errorf("container/registry: layer: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("container/registry: layer: %w", err)
	}
	sum := sha256.Sum256(archive.Bytes())
	config, err := json.Marshal(map[string]any{
		"architecture": runtime.GOARCH,
		"os":           "linux",
		"config":       map[string]any{"Cmd": []string{"/" + containerRegistryFile}},
		"rootfs":       map[string]any{"type": "layers", "diff_ids": []string{"sha256:" + hex.EncodeToString(sum[:])}},
	})
	if err != nil {
		return fmt.Errorf("container/registry: configuration: %w", err)
	}
	_, err = server.PutImage(containerRegistryRepository, containerRegistryTag, ociregistry.Image{
		Architecture: runtime.GOARCH,
		Layers:       []ociregistry.Layer{{MediaType: ociregistry.MediaOCILayerTar, Content: archive.Bytes()}},
		Config:       config,
	})
	if err != nil {
		return fmt.Errorf("container/registry: image: %w", err)
	}
	return nil
}
