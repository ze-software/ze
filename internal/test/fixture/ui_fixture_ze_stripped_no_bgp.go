package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func init() {
	Register("ui/ze-stripped-no-bgp", uiDriver(runZEStrippedNoBGP))
}

type validationResult struct {
	exitCode int
	output   string
}

func runZEStrippedNoBGP(ctx context.Context) error {
	// 1. The non-BGP surface still parses on a binary with BGP compiled out.
	ok, err := validateZEStrippedConfig(ctx, "fib-static.conf")
	if err != nil {
		return err
	}
	if ok.exitCode != 0 {
		return fmt.Errorf("fib+static config rejected by a BGP-free binary: rc=%d out=%q", ok.exitCode, ok.output)
	}
	if !strings.Contains(ok.output, "valid") {
		return fmt.Errorf("expected a validity report, got %q", ok.output)
	}

	// 2. And BGP really is absent. Without this, step 1 would pass just as well on a
	// binary that still carried BGP, which is exactly the vacuity that made the
	// pre-existing parse test unable to detect anything.
	bad, err := validateZEStrippedConfig(ctx, "with-bgp.conf")
	if err != nil {
		return err
	}
	if bad.exitCode == 0 {
		return fmt.Errorf("a bgp block was ACCEPTED by a binary built without ze_bgp: out=%q", bad.output)
	}

	// 3. The refusal names the build tag that carries the root, read from the
	// generated gated-plugin table (config.TopLevelKeywordHint), so the operator
	// learns why the keyword is unknown and how to get it. A plain "unknown
	// top-level keyword" with no tag fails this step.
	if !strings.Contains(bad.output, "build tag ze_bgp") {
		return fmt.Errorf("the bgp refusal does not name the ze_bgp build tag: out=%q", bad.output)
	}

	_, err = fmt.Fprintln(os.Stdout, "ze-stripped: non-BGP surface valid, bgp rejected")
	return err
}

// validateZEStrippedConfig runs `ze-stripped config validate path`.
func validateZEStrippedConfig(ctx context.Context, path string) (validationResult, error) {
	return validateStrippedConfig(ctx, "ze-stripped", path)
}

// validateStrippedConfig runs `<binary> config validate path` and answers its
// exit code and combined output. binary is one of the stripped builds the
// functional set holds (internal/le/test/functional/binaries.go).
func validateStrippedConfig(ctx context.Context, binary, path string) (validationResult, error) {
	cmd := exec.CommandContext(ctx, binary, "config", "validate", path) //nolint:gosec // the fixture chooses the program and its arguments
	output, err := cmd.CombinedOutput()
	result := validationResult{
		exitCode: 0,
		output:   string(output),
	}
	if err == nil {
		return result, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return validationResult{}, ctxErr
	}

	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		result.exitCode = exitErr.ExitCode()
		return result, nil
	}

	return validationResult{}, fmt.Errorf("start %s config validate %q: %w", binary, path, err)
}
