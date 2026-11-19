// Design: docs/contributing/feature-maturity.md -- the recorded green run Supported requires
// Related: recordrun.go -- runInGuest, the guest route every boot of this build takes
// Related: ../test/qemu/dockerlab_command.go -- the guest-le and image words that skip the boot's own builds

package feature

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/linuxle"
	setupdockerhost "github.com/ze-software/ze/internal/le/setup/docker/host"
	testfunctional "github.com/ze-software/ze/internal/le/test/functional"
	testqemu "github.com/ze-software/ze/internal/le/test/qemu"
)

// guestBuild is what one feature run builds for its guest route, once, before
// its first test, and names to every guest boot after it. A boot of `docker-lab
// harness` would otherwise cross-build the guest le again from the working
// tree, which other sessions edit at the same time: one half-edited package
// anywhere then voids the whole run, and the tests of one run would judge as
// many builds as it has guest tests.
type guestBuild struct {
	binaries string // checkout-relative directory of the guest daemons
	le       string // checkout-relative guest le
	leDigest string // SHA-256 of le, read right after this run built it
	image    string // the docker-host image every boot opens
	kernel   string // the runtime vmlinuz that image boots
}

// guestSteps are the host steps that build a guest route's inputs: each is
// minutes of work or needs a kernel cache, so a test replaces them.
type guestSteps struct {
	daemons func(dir, goarch string) error
	le      func(rel, goarch string) error
	image   func(goarch string) (string, error)
	kernel  func(goarch string) (string, error)
}

// hostGuestSteps answers the real steps over r's tree and toolchain.
func (r *repoRunner) hostGuestSteps() guestSteps {
	return guestSteps{
		daemons: func(dir, goarch string) error { return testfunctional.PrepareGuest(r.toolchain, dir, goarch) },
		le: func(rel, goarch string) error {
			return testqemu.BuildGuestLeAt(context.Background(), r.tree, goarch, rel)
		},
		image: func(goarch string) (string, error) {
			return setupdockerhost.CachedImage(r.tree, goarch)
		},
		kernel: func(goarch string) (string, error) { return testqemu.GuestRuntimeKernel(r.tree, goarch) },
	}
}

// guestReady builds the guest route's inputs on its first call and answers that
// one build on every later call. Not safe for concurrent use.
func (r *repoRunner) guestReady() (*guestBuild, error) {
	if r.guest != nil {
		return r.guest, nil
	}
	steps := r.guestSteps
	if steps == nil {
		host := r.hostGuestSteps()
		steps = &host
	}
	build, err := buildGuestRoute(r.tree, testqemu.GuestArch(), steps)
	if err != nil {
		return nil, err
	}
	r.guest = &build
	return r.guest, nil
}

// buildGuestRoute runs steps once for goarch: the daemons and the le into the
// feature run's own directory under the checkout, which the guest mounts, then
// the image and its kernel. The le's digest is read last, after its build
// answered success, so it names the bytes this run built.
func buildGuestRoute(tree, goarch string, steps *guestSteps) (guestBuild, error) {
	dir := filepath.Join(guestBinariesRel, "linux-"+goarch)
	build := guestBuild{binaries: filepath.Join(dir, "bin"), le: filepath.Join(dir, linuxle.Name)}
	if err := steps.daemons(filepath.Join(tree, build.binaries), goarch); err != nil {
		return guestBuild{}, errors.New("cannot cross-build the guest ze for linux/" + goarch + ": " + err.Error())
	}
	if err := steps.le(build.le, goarch); err != nil {
		return guestBuild{}, errors.New("cannot cross-build the guest le for linux/" + goarch + ": " + err.Error())
	}
	digest, err := fileDigest(filepath.Join(tree, build.le))
	if err != nil {
		return guestBuild{}, errors.New("cannot read the guest le this run built at " + build.le + ": " + err.Error())
	}
	build.leDigest = digest
	image, err := steps.image(goarch)
	if err != nil {
		return guestBuild{}, errors.New("cannot resolve the docker-host image Ze's guest boots: " + err.Error())
	}
	build.image = image
	kernel, err := steps.kernel(goarch)
	if err != nil {
		return guestBuild{}, errors.New("cannot resolve the runtime kernel Ze's guest boots: " + err.Error())
	}
	build.kernel = kernel
	return build, nil
}

// unchanged refuses a guest le that is no longer the one this run built: gone,
// or holding other bytes. Its age proves nothing, because `go build` leaves an
// up-to-date output untouched, so the check is the content digest. Another
// `le feature run` builds into the same directory, so that is the usual writer.
func (b *guestBuild) unchanged(tree string) error {
	digest, err := fileDigest(filepath.Join(tree, b.le))
	if err != nil {
		return errors.New("the guest le this run built at its start, " + b.le + ", cannot be read now (" +
			err.Error() + "), so the next guest test would run no build of this run. Fix: run one " +
			"`./le feature run` at a time, since each builds into " + guestBinariesRel + ", then run it again")
	}
	if digest != b.leDigest {
		return errors.New("the guest le this run built at its start, " + b.le + ", holds other bytes now, " +
			"so the next guest test would judge another build of the tree. Fix: run one `./le feature run` " +
			"at a time, since each builds into " + guestBinariesRel + ", then run it again")
	}
	return nil
}

// fileDigest answers the hex SHA-256 of the file at path, streamed.
func fileDigest(path string) (string, error) {
	file, err := os.Open(path) //nolint:gosec // a path this run built under the checkout
	if err != nil {
		return "", err
	}
	defer file.Close() //nolint:errcheck // read-only

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
