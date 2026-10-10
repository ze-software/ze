// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: daemonbuild.go -- the daemon this check probes with
// Related: l2tp.go -- the L2TP proof that calls it
// Related: vppiface.go -- the VPP interface proof that calls it
// Related: vppevidence.go -- the VPP evidence proof that calls it
//
// daemonkernel.go is the one step every proof that cross-compiles its own
// daemon takes between the build and its first container: the Docker daemon's
// kernel must carry every feature Ze enrolls (owner D-4: a wrong kernel "should
// not be possible - fail"). The interop suites take the same check through
// Suite.Run; these proofs do not run a Suite, so they call it here, once.

package testdeployment

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// daemonKernelTimeout bounds the whole check: the release query and the probe
// container, whose own bound is the interoplab probe timeout.
const daemonKernelTimeout = 3 * time.Minute

// buildCheckedDaemon runs build, then refuses a Docker daemon whose kernel lacks
// an enrolled feature, probing with the daemon that build writes for goarch.
//
// NO_BUILD=1 skips build and reuses the daemon already at that path, as the
// interop suites do (interoplab.ReadEnvironment). A reuse with no daemon there
// is refused by name: probing with nothing would read as a broken kernel.
func buildCheckedDaemon(tree, goarch string, build func() error) error {
	daemon := filepath.Join(tree, daemonRel(goarch))
	if interoplab.ReadEnvironment(interoplab.EnvironmentOptions{}).NoBuild {
		if _, err := os.Stat(daemon); err != nil {
			var tb textbuf.Buffer
			return errors.New(tb.Str("NO_BUILD=1 reuses the daemon at ").Str(daemonRel(goarch)).
				Str(", and there is none: ").Err(err).String())
		}
	} else if err := build(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), daemonKernelTimeout)
	defer cancel()

	return interoplab.DockerKernel(daemon)(ctx, interoplab.NewDocker())
}
