// Design: docs/architecture/testing/interop.md -- a package test's own server container (labtest)
// Related: labtest.go -- StartContainer, which answers a Container and runs these steps.

package labtest

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// moduleFile names the file that marks the module root, where a
// Spec.Dockerfile path starts.
const moduleFile = "go.mod"

// Container is one started server container: the host address it serves at,
// and the commands a test runs inside it. labtest removes the container in
// t.Cleanup, after which Exec fails. Safe for concurrent use.
type Container struct {
	// Address is where the server serves on the host.
	Address netip.AddrPort
	name    string
	docker  *interoplab.Docker
}

// Exec runs argv in the container and answers its output. A non-zero exit
// is an error carrying the command's stderr.
func (c Container) Exec(ctx context.Context, argv ...string) (interoplab.CommandResult, error) {
	if c.docker == nil {
		return interoplab.CommandResult{}, errors.New("labtest: Exec on a Container that StartContainer did not answer")
	}
	return c.docker.Exec(ctx, c.name, argv, nil)
}

// build makes image from dockerfile, a path from the module root, with the
// Dockerfile's directory as the build context, and registers the release of
// the tag this run owns. It fails the test and answers false when the build
// fails. The release runs after the container's removal, because t.Cleanup
// runs last in first out and start registers the removal after this.
func (h host) build(t tester, name, image, dockerfile string) bool {
	t.Helper()
	root, err := h.moduleRoot()
	if err != nil {
		t.Fatalf("labtest %s: %v. Fix: run the test from inside the Ze module", name, err)
		return false
	}
	path := filepath.Join(root, filepath.FromSlash(dockerfile))
	built, err := h.docker.Build(t.Context(), interoplab.ImageBuild{Name: name, Tag: image,
		Dockerfile: path, Context: filepath.Dir(path), Required: true})
	if err != nil {
		t.Fatalf("labtest %s: cannot build %s from %s: %v. Fix: `docker build -f %s %s` shows the failing step",
			name, image, dockerfile, err, path, filepath.Dir(path))
		return false
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), releaseTimeout)
		defer cancel()
		if err := h.docker.ReleaseImage(ctx, built); err != nil {
			t.Errorf("labtest: cannot release the image tag this run built for %s: %v. Fix: `./le test"+
				" integration clean` removes it once this process has exited", image, err)
		}
	})
	return true
}

// readyInContainer is the ready probe for a Spec.ReadyInContainer command:
// nil once argv exits 0 inside the container.
func (h host) readyInContainer(name string, argv []string) func(context.Context, netip.AddrPort) error {
	return func(ctx context.Context, _ netip.AddrPort) error {
		_, err := h.docker.Exec(ctx, name, argv, nil)
		return err
	}
}

// moduleRoot answers the nearest directory at or above the working
// directory that holds go.mod. A package test runs in its package's
// directory, so the walk ends at the Ze module root. The walk is bounded by
// the depth of the path: filepath.Dir of the root is the root.
func moduleRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, moduleFile)); err == nil {
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", errors.New("no " + moduleFile + " at or above the working directory")
		}
		directory = parent
	}
}
