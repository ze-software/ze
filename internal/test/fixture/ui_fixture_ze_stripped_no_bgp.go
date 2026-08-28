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

	_, err = fmt.Fprintln(os.Stdout, "ze-stripped: non-BGP surface valid, bgp rejected")
	return err
}

func validateZEStrippedConfig(ctx context.Context, path string) (validationResult, error) {
	cmd := exec.CommandContext(ctx, "ze-stripped", "config", "validate", path)
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

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.exitCode = exitErr.ExitCode()
		return result, nil
	}

	return validationResult{}, fmt.Errorf("start ze-stripped config validate %q: %w", path, err)
}
