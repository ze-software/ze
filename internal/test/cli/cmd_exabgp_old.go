// Design: docs/functional-tests.md -- ExaBGP compatibility populations
// Related: cmd_exabgp.go -- the runner these helpers serve

package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"

	"github.com/ze-software/ze/internal/test/runner"
)

// exabgpConfigDir answers the directory a suite's `option=file:` lines name
// their configs in. A population copied from an ExaBGP release (old-4.2,
// old-5.0) carries that release's configurations and helpers in an etc of its
// own, because they share names with the ones the api population holds and
// differ from them. Every other population uses the etc they share.
func exabgpConfigDir(root, suiteName string) (string, error) {
	own := filepath.Join(root, suiteName, exabgpConfigDirName)
	info, err := os.Stat(own)
	if errors.Is(err, fs.ErrNotExist) {
		return filepath.Join(root, exabgpConfigDirName), nil
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str(own).
			Str(" is not a directory: a population's etc holds the configs its cases name, so move the file away").String())
	}
	return own, nil
}

// checkExaBGPAllowedPair refuses a case that carries only one half of an
// allowance. The reason says why the case differs, and the expected text is how
// the runner tells that difference from any other failure, so either one alone
// cannot be judged.
func checkExaBGPAllowedPair(ciFile string, test *exabgpTestEntry) error {
	if test.allowed != "" && test.allowedError == "" {
		var tb textbuf.Buffer
		return errors.New(tb.Str(ciFile).Str(" carries ").Str(exabgpAllowedOption).
			Str(" but no ").Str(exabgpAllowedErrorOption).
			Str(": without the text its failure prints, a timeout or a crash would be counted as the allowed difference. Add ").
			Str(exabgpAllowedErrorOption).Str("<words the failure prints>").String())
	}
	if test.allowed == "" && test.allowedError != "" {
		var tb textbuf.Buffer
		return errors.New(tb.Str(ciFile).Str(" carries ").Str(exabgpAllowedErrorOption).
			Str(" but no ").Str(exabgpAllowedOption).
			Str(": the expected failure text is read only for an allowed case. Add ").
			Str(exabgpAllowedOption).Str("<reason>, or delete the line").String())
	}
	return nil
}

// judgeExaBGPAllowed turns a case's run into its verdict, and answers whether
// the verdict lets the run pass.
//
// A case with no allowance passes when it ran green. A case carrying
// `option=allowed:<reason>` is held to FAIL with the text its
// `option=allowed-error:` line names: that failure is recorded as a skip
// naming the reason. A green run of it fails the suite, because a difference
// that is gone leaves an allowance that would hide the next one. A failure
// without the expected text fails the suite too, because it is some other
// failure (a timeout, a crash) that the allowance does not cover. ExaBGP keeps
// its ALLOWED list short for the same reason.
//
// The allowance describes how ze differs from the release, so it applies only
// when ze is the daemon. In proof mode the release itself runs the case, and
// the case is judged on its own run.
func judgeExaBGPAllowed(test *exabgpTestEntry, passed bool, detail exabgpRunDetail, dut exabgpDUT) bool {
	if test.allowed == "" {
		return passed
	}
	if dut != exabgpDUTZe {
		return passed
	}
	rec := test.record
	var tb textbuf.Buffer
	if passed {
		rec.SetState(runner.StateFail)
		rec.Error = errors.New(tb.Str(test.ciFile).
			Str(" passes, yet carries ").Str(exabgpAllowedOption).Str(test.allowed).
			Str(": the difference is gone, so delete that line").String())
		return false
	}
	if !exabgpFailureCarries(rec.Error, detail, test.allowedError) {
		// The record keeps its own state (FAIL or TIMEOUT) and its error, so the
		// failure output still shows what really happened.
		cause := "no error"
		if rec.Error != nil {
			cause = rec.Error.Error()
		}
		rec.Error = errors.New(tb.Str(test.ciFile).
			Str(" failed, but not with the allowed text \"").Str(test.allowedError).
			Str("\", so this is another failure the allowance does not cover: ").Str(cause).String())
		return false
	}
	// The display prints SkipReason on the case's SKIP line, so the reason is
	// read where the verdict is.
	rec.SetState(runner.StateSkip)
	rec.Error = nil
	rec.SkipReason = tb.Str("allowed: ").Str(test.allowed).String()
	return true
}

// exabgpFailureCarries answers whether a failed run printed the expected text,
// in its verdict or in the output of either process.
func exabgpFailureCarries(failure error, detail exabgpRunDetail, text string) bool {
	if failure != nil && strings.Contains(failure.Error(), text) {
		return true
	}
	for _, output := range []string{detail.serverStdout, detail.serverStderr, detail.clientStdout, detail.clientStderr} {
		if strings.Contains(output, text) {
			return true
		}
	}
	return false
}
