// Design: docs/contributing/terminal-demos.md -- one authored browser journey.
// Related: demos/terminal/web-config/demo.cjs -- presentation and CDP WebP capture.
package siteterminaldemo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	webtesting "github.com/ze-software/ze/internal/component/web/testing"
	"github.com/ze-software/ze/internal/test/runner"
)

const recordedWebConfigJourney = "test/web/recorded-config-save.wb"

// runWebConfigJourney is shared by validation and recording. Recording adds only
// the capture of the final page; it cannot replace or bypass authored steps.
func runWebConfigJourney(record bool) error {
	content, err := os.ReadFile(filepath.Join(demoRoot(), recordedWebConfigJourney))
	if err != nil {
		return fmt.Errorf("read shared browser journey %s: %w", recordedWebConfigJourney, err)
	}
	journey, err := webtesting.ParseWBFile(string(content))
	if err != nil {
		return fmt.Errorf("parse shared browser journey %s: %w", recordedWebConfigJourney, err)
	}
	return runParsedWebConfigJourney(journey, record)
}

// runParsedWebConfigJourney keeps setup, authored execution and capture on one
// path even when an integration proof varies parsed configuration operands.
func runParsedWebConfigJourney(journey *webtesting.WBTestCase, record bool) (err error) {
	if journey.SkipReason != "" {
		return fmt.Errorf("shared browser journey %s is skipped: %s; restore its runnable steps before validating or recording", recordedWebConfigJourney, journey.SkipReason)
	}
	root, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel, err := webtesting.CaseContext(root, journey.Timeout, 1)
	if err != nil {
		return err
	}
	defer cancel()
	workspace, err := newWebConfigWorkspace(ctx)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, workspace.Close()) }()
	server, err := workspace.start(ctx)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, server.Stop(syscall.SIGTERM, runner.TeardownGraceTimeout, 0)) }()
	if err := context.Cause(ctx); err != nil {
		return err
	}
	var capture webtesting.WBCapture
	if record {
		capture = captureWebConfig
	}
	result := webtesting.RunWBCaseWithCapture(ctx, journey, "https://127.0.0.1:8443", "", capture)
	if !result.Passed {
		return fmt.Errorf("shared browser journey %s failed: %s", recordedWebConfigJourney, result.Error)
	}
	return nil
}

// captureWebConfig borrows only this runner-owned browser. The existing
// Playwright helper captures WebP directly through CDP, without an encoder or
// another configuration workflow. The helper MUST return before owned Close.
func captureWebConfig(ctx context.Context, endpoint string) error {
	process := exec.CommandContext(ctx, "node", filepath.Join(demoDir("web-config"), "demo.cjs"), "capture", endpoint) //nolint:gosec // The script belongs to the closed demo table; endpoint comes from the owned browser.
	process.WaitDelay = 2 * time.Second
	process.Stdout = os.Stdout
	process.Stderr = os.Stderr
	return errors.Join(process.Run(), context.Cause(ctx))
}
