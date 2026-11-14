// Design: docs/architecture/testing/interop.md -- the deployment proofs and their actions
// Overview: compose.go -- the proof this action runs

package testdeployment

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/le/interoplab"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// composeActionName is the verb: `./le test deployment docker-compose`.
const composeActionName = "docker-compose"

// composeRunCommand is the command a refusal names to run the proof again.
const composeRunCommand = "./le test deployment " + composeActionName

// The action's keywords.
const (
	composeImageKeyword = "image"
	composeBreakKeyword = "break-volume"
)

func composeParameters() []leaction.Parameter {
	return []leaction.Parameter{
		{Keyword: composeImageKeyword, Kind: leaction.KindString, Value: "reference", Requirement: leaction.Optional,
			Help: "run this loaded image instead of a tree build"},
		{Keyword: composeBreakKeyword, Kind: leaction.KindSwitch,
			Help: "counterfactual: mount no volume, so the run must fail"},
	}
}

// runComposeHere runs the proof over the checkout under one signal root.
func runComposeHere(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "the Compose persistence proof cannot find the Ze checkout",
			"it builds docker/Dockerfile and reads docker/compose.yaml and docker/ze.conf from the checkout",
			"change to the Ze checkout, then run `"+composeRunCommand+"`"))
		return nil, 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	proof, err := newComposeProof(root, interoplab.NewDocker(), args.One(composeImageKeyword),
		args.Has(composeBreakKeyword), os.Stderr)
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "the Compose persistence proof could not name its run",
			"its project name and operator password come from the system random source, which did not answer",
			"check that the host's random source is readable, then run `"+composeRunCommand+"` again"))
		return nil, 1
	}
	return runCompose(ctx, proof)
}

// runCompose answers one run: an error is a step the proof could not start,
// a disproved step or anything left behind is the verdict, and both answer 1.
func runCompose(ctx context.Context, run proofRun[ComposeReport]) (any, int) {
	report, err := run.Run(ctx)
	if err != nil {
		// Run errors only before the first step, when nothing exists: the
		// refusal is the whole answer.
		leaction.ReportError(opererr.Wrap(err, "the Compose persistence proof did not start",
			"it stopped before its first step, so it created nothing on the Docker host",
			"resolve the cause above as its own fix line says, then run `"+composeRunCommand+"` again"))
		return nil, 1
	}
	if !report.Proven {
		return report, 1
	}
	return report, 0
}
