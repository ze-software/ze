// Design: third_party/vpp-patches/README.md -- the native API generation action.
// Related: api.go -- the bounded generation lifecycle and test process seam.
package buildvpp

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ze-software/ze/internal/core/opererr"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/interoplab"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/test/runner"
)

// The factory seam is replaced only in an isolated test subprocess.
var newAPIGeneration = generateAPIAtRoot

func answerGenerate() (any, int) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	root, err := lepath.Root()
	if err == nil {
		var report APIReport
		report, err = newAPIGeneration(ctx, root)
		if err == nil {
			return report, 0
		}
	}
	leaction.ReportError(opererr.Wrap(err, "the VPP PPPoE and Feature bindings could not be generated",
		"The PPPoE backend needs both APIs from the same pinned VPP source. Repair the reported source, Docker, generator or output ownership failure, then retry.",
		"./le build vpp update"))
	return nil, 1
}

func generateAPIAtRoot(ctx context.Context, root string) (APIReport, error) {
	toolchain, err := gotoolchain.New(root)
	if err != nil {
		return APIReport{}, err
	}
	paths, err := lepath.ResolveSession(root, true)
	if err != nil {
		return APIReport{}, err
	}
	scratch := paths.Scratch
	if !filepath.IsAbs(scratch) {
		scratch = filepath.Join(root, scratch)
	}
	return generateAPI(ctx, root, scratch, interoplab.NewBuildToolDocker(), apiHostProcess{},
		toolchain.EnvironmentContext(ctx, gotoolchain.EnvOptions{}))
}

type apiHostProcess struct{}

// Run uses the existing process owner so cancellation, pipe holders and
// descendants share the same bounded cleanup as other native build actions.
func (apiHostProcess) Run(ctx context.Context, plan interoplab.ProcessCommand) (result interoplab.ProcessResult, retErr error) {
	bounded, stop := context.WithTimeout(ctx, plan.Timeout)
	defer stop()
	command := exec.CommandContext(bounded, plan.Arguments[0], plan.Arguments[1:]...) //nolint:gosec // The generator plan owns argv.
	command.Dir, command.Env = plan.Directory, plan.Environment
	command.Stdin, command.Stdout, command.Stderr = plan.Stdin, plan.Stdout, plan.Stderr
	owner, err := runner.StartServerProcess(command)
	if err != nil {
		return result, errors.Join(bounded.Err(), err)
	}
	defer func() {
		retErr = errors.Join(retErr, owner.Close())
		if command.ProcessState != nil {
			result.ExitCode = command.ProcessState.ExitCode()
		}
	}()
	<-owner.Done()
	return result, bounded.Err()
}
