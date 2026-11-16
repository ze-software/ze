//go:build zetest

// Design: docs/architecture/container/runtime.md -- the test store and its fixture image
// Related: storeseam.go -- the seam this store fills
// Related: ../../test/containerprobe/main.go -- the program the fixture image holds

package container

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The test store exists only in a zetest build, the functional-test daemon
// (internal/le/test/functional/binaries.go adds the tag), so no shipped ze
// registers or reads its key. It answers every container with one fixture
// image: a folder holding the probe program, built beside the ze binary by
// the functional harness. Nothing is fetched.
const testStoreKey = "ze.test.container.store"

// The fixture image: the probe program at the root, and its arguments.
const (
	probeBinary   = "ze-containerprobe"
	probeArgument = "run"
	testDigest    = "sha256:7a65000000000000000000000000000000000000000000000000000000000001"
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         testStoreKey,
	Type:        "string",
	Description: "Test DUT builds only: the directory of the container test store, whose one image holds the probe program (test infrastructure)",
	Private:     true,
})

//nolint:gochecknoinits // registration: the test build fills the store seam the image store package fills in a shipped build
func init() {
	SetStoreFactory(func(StoreNotices) Store { return testStore{} })
}

// testStore answers Ready for every container, from the directory
// testStoreKey names: the image in image/, and each container's writable
// layer in layer/NAME. Safe for concurrent use: it holds no state, and a
// fixture install of one container never races another's, because the copy
// is renamed into place.
type testStore struct{}

// Resolve answers the fixture image for request. An unset key is a failure to
// answer, never an absent image: no fetch would ever bring one.
func (testStore) Resolve(_ context.Context, request ResolveRequest) (Resolution, error) {
	directory := env.Get(testStoreKey)
	if directory == "" {
		return nil, errors.New(testStoreKey + " is not set: the test build's container store has no directory")
	}
	directory, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	image := filepath.Join(directory, "image")
	if err := installProbe(image); err != nil {
		return nil, err
	}
	return Ready{
		Digest:           testDigest,
		ImageFolder:      image,
		WritableFolder:   filepath.Join(directory, "layer", request.Instance),
		Entrypoint:       []string{"/" + probeBinary},
		Command:          []string{probeArgument},
		Environment:      []string{"PATH=/"},
		WorkingDirectory: "/",
	}, nil
}

// Hold holds nothing: the fixture image is never removed while a test runs.
func (testStore) Hold(string, Ready) func() { return func() {} }

// installProbe puts the probe program at the root of the image folder, copied
// from beside the running ze binary, once.
func installProbe(image string) error {
	target := filepath.Join(image, probeBinary)
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	source := filepath.Join(filepath.Dir(executable), probeBinary)
	// The image root is owner and group only: the probe runs as root, and the
	// container's / takes this mode (mountRoot).
	if err := os.MkdirAll(image, 0o750); err != nil {
		return err
	}
	in, err := os.Open(source) //nolint:gosec // the probe built beside the test daemon
	if err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str("the test store's fixture image needs ").Str(source).Str(", built beside ze by the functional harness: ").Str(err.Error()).String())
	}
	defer in.Close() //nolint:errcheck // a read-only file
	staging, err := os.CreateTemp(image, "."+probeBinary+"-")
	if err != nil {
		return err
	}
	if _, err := io.Copy(staging, in); err != nil {
		return errors.Join(err, staging.Close(), os.Remove(staging.Name()))
	}
	if err := staging.Chmod(0o755); err != nil {
		return errors.Join(err, staging.Close(), os.Remove(staging.Name()))
	}
	if err := staging.Close(); err != nil {
		return errors.Join(err, os.Remove(staging.Name()))
	}
	return os.Rename(staging.Name(), target)
}
