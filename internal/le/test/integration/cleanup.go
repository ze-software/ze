// Design: docs/architecture/testing/interop.md -- Crash-safe teardown: the explicit cleanup verb
// Related: ../../interoplab/orphans.go -- Docker.CleanupOrphans, the verdict and the report.
// Related: gates.go -- the cleanup row.

package testintegration

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// cleanupVerb is the explicit orphan cleanup, `./le test integration clean [dry]`.
const cleanupVerb = "clean"

// labCleaner is the part of interoplab.Docker the cleanup verb drives.
type labCleaner interface {
	Probe(context.Context) error
	CleanupOrphans(context.Context, bool) (interoplab.OrphanReport, error)
}

func runLabCleanup(ctx context.Context, _ string, args leaction.Arguments) (any, int) {
	return labCleanup(ctx, interoplab.NewDocker(), cleanupDry(args))
}

func cleanupDry(args leaction.Arguments) bool { return args.Has("dry") }

// labCleanup probes Docker, then removes the lab resources whose owner is
// proven gone, or in a dry run lists them. A daemon that does not answer is
// an error naming the fix; a skipped resource never is.
func labCleanup(ctx context.Context, cleaner labCleaner, dry bool) (any, int) {
	var problem textbuf.Buffer
	if err := cleaner.Probe(ctx); err != nil {
		leaction.ReportError(errors.New(problem.Str("cleanup: Docker does not answer (").Err(err).
			Str("), so no lab resource can be listed. Fix: start Docker, or point DOCKER_HOST at the").
			Str(" daemon the labs use, then run `./le test integration clean` again").String()))
		return nil, 1
	}
	report, err := cleaner.CleanupOrphans(ctx, dry)
	if err != nil {
		leaction.ReportError(errors.New(problem.Str("cleanup: the labeled lab resources could not be listed (").
			Err(err).Str("). Fix: run the failing docker command by hand to see why the daemon refuses it,").
			Str(" then run `./le test integration clean` again").String()))
		return nil, 1
	}
	return report, report.Code()
}
