// VALIDATES: the old-4.2 and old-5.0 populations: their own etc directory and
// the `option=allowed:` verdict.
// PREVENTS: a ported case reading the api population's config of the same
// name, and an allowance that outlives the difference it names.

package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/runner"
)

// TestExaBGPConfigDirPrefersThePopulationsOwn proves a population with an etc
// of its own names configs there, and one without uses the shared etc.
// Method: a temporary tree with one population of each kind, and a third whose
// etc is a file, which must be refused rather than read as absent.
func TestExaBGPConfigDirPrefersThePopulationsOwn(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "old-5.0", "etc"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "broken"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "broken", "etc"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	own, err := exabgpConfigDir(root, "old-5.0")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "old-5.0", "etc"); own != want {
		t.Errorf("old-5.0 config dir = %q, want %q", own, want)
	}
	shared, err := exabgpConfigDir(root, "api")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "etc"); shared != want {
		t.Errorf("api config dir = %q, want %q", shared, want)
	}
	if _, err := exabgpConfigDir(root, "broken"); err == nil {
		t.Error("an etc that is a file was accepted")
	}
}

// TestParseExaBGPCIAllowed proves `option=allowed:<reason>` and
// `option=allowed-error:<text>` are read as the case's allowance, configs
// resolve under the directory handed in, and an allowance with no reason, or
// either half of the pair without the other, is refused.
func TestParseExaBGPCIAllowed(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	tests := runner.NewTests()
	configDir := filepath.Join(dir, "configs")

	good := write("good.ci", "option=file:a.conf\noption=allowed:RFC 4271 6.1 asks for the type\n"+
		"option=allowed-error:unexpected EOF\n1:raw:FF\n")
	entry, err := parseExaBGPCI(configDir, tests.Add("good"), good)
	if err != nil {
		t.Fatal(err)
	}
	if entry.allowed != "RFC 4271 6.1 asks for the type" {
		t.Errorf("allowed = %q", entry.allowed)
	}
	if entry.allowedError != "unexpected EOF" {
		t.Errorf("allowedError = %q", entry.allowedError)
	}
	if len(entry.configs) != 1 || entry.configs[0] != filepath.Join(configDir, "a.conf") {
		t.Errorf("configs = %v, want [%s/a.conf]", entry.configs, configDir)
	}

	empty := write("empty.ci", "option=file:a.conf\noption=allowed:\n")
	if _, err := parseExaBGPCI(configDir, tests.Add("empty"), empty); err == nil {
		t.Error("an allowance with no reason was accepted")
	} else if !strings.Contains(err.Error(), "names no reason") {
		t.Errorf("error = %v, want it to say the allowance names no reason", err)
	}

	for _, half := range []struct{ name, body, missing string }{
		{"reason-only", "option=file:a.conf\noption=allowed:a reason\n", exabgpAllowedErrorOption},
		{"text-only", "option=file:a.conf\noption=allowed-error:some text\n", exabgpAllowedOption},
	} {
		path := write(half.name+".ci", half.body)
		_, err := parseExaBGPCI(configDir, tests.Add(half.name), path)
		if err == nil {
			t.Errorf("%s: half an allowance was accepted", half.name)
			continue
		}
		if !strings.Contains(err.Error(), "but no "+half.missing) {
			t.Errorf("%s: error = %v, want it to name the missing %s", half.name, err, half.missing)
		}
	}
}

// TestJudgeExaBGPAllowed proves the verdict table: a plain case passes when it
// ran green; an allowed case that fails with its expected text is a SKIP
// naming its reason and lets the run pass; an allowed case that passes fails
// the run, because its allowance would hide the next difference; an allowed
// case that fails WITHOUT its text (a timeout, a crash) fails the run and keeps
// its own state; and under --dut exabgp the allowance is not read at all.
func TestJudgeExaBGPAllowed(t *testing.T) {
	const reason = "a reason"
	const expected = "unexpected EOF in neighbor block"
	matching := errors.New("migrate conf-split.conf: exit status 1: line 22: " + expected)
	cases := []struct {
		name      string
		allowed   string
		passed    bool
		failure   error
		detail    exabgpRunDetail
		state     runner.State
		dut       exabgpDUT
		wantOK    bool
		wantState runner.State
	}{
		{"plain green", "", true, nil, exabgpRunDetail{}, runner.StateSuccess, exabgpDUTZe, true, runner.StateSuccess},
		{"plain red", "", false, matching, exabgpRunDetail{}, runner.StateFail, exabgpDUTZe, false, runner.StateFail},
		{"allowed red with text", reason, false, matching, exabgpRunDetail{}, runner.StateFail, exabgpDUTZe, true, runner.StateSkip},
		{"allowed red with text in output", reason, false, errors.New("exit status 1"),
			exabgpRunDetail{clientStderr: expected}, runner.StateFail, exabgpDUTZe, true, runner.StateSkip},
		{"allowed green", reason, true, nil, exabgpRunDetail{}, runner.StateSuccess, exabgpDUTZe, false, runner.StateFail},
		{"allowed timeout", reason, false, context.DeadlineExceeded, exabgpRunDetail{}, runner.StateTimeout, exabgpDUTZe, false, runner.StateTimeout},
		{"allowed crash", reason, false, errors.New("ze exited before the mock BGP server finished its script: signal: segmentation fault"),
			exabgpRunDetail{}, runner.StateFail, exabgpDUTZe, false, runner.StateFail},
		{"allowed green under exabgp", reason, true, nil, exabgpRunDetail{}, runner.StateSuccess, exabgpDUTExaBGP, true, runner.StateSuccess},
		{"allowed red under exabgp", reason, false, matching, exabgpRunDetail{}, runner.StateFail, exabgpDUTExaBGP, false, runner.StateFail},
	}
	tests := runner.NewTests()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := tests.Add(tc.name)
			rec.SetState(tc.state)
			rec.Error = tc.failure
			entry := &exabgpTestEntry{record: rec, ciFile: tc.name + ".ci", allowed: tc.allowed}
			if tc.allowed != "" {
				entry.allowedError = expected
			}
			if got := judgeExaBGPAllowed(entry, tc.passed, tc.detail, tc.dut); got != tc.wantOK {
				t.Errorf("judgeExaBGPAllowed = %v, want %v", got, tc.wantOK)
			}
			if rec.State != tc.wantState {
				t.Errorf("state = %v, want %v", rec.State, tc.wantState)
			}
			if tc.wantState == runner.StateSkip && rec.SkipReason != "allowed: "+reason {
				t.Errorf("skip reason = %q", rec.SkipReason)
			}
			if tc.allowed != "" && !tc.passed && !tc.wantOK && tc.dut == exabgpDUTZe {
				if rec.Error == nil || !strings.Contains(rec.Error.Error(), "not with the allowed text") ||
					!strings.Contains(rec.Error.Error(), tc.failure.Error()) {
					t.Errorf("error = %v, want it to say the allowed text is missing and keep the cause", rec.Error)
				}
			}
		})
	}
}

// TestDiscoverExaBGPOldPopulations proves the two ported populations load
// from the real tree: every case parses, and every config it names exists
// under the population's own etc, not the shared one.
func TestDiscoverExaBGPOldPopulations(t *testing.T) {
	baseDir, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, suiteName := range []string{exabgpSuiteOld42, exabgpSuiteOld50} {
		suite, err := discoverExaBGPSuite(baseDir, suiteName)
		if err != nil {
			t.Fatalf("%s: %v", suiteName, err)
		}
		if suite.tests.Count() == 0 {
			t.Fatalf("%s: no cases", suiteName)
		}
		ownEtc := filepath.Join(baseDir, "test", predecessorTestDir, suiteName, exabgpConfigDirName)
		for nick, entry := range suite.byNick {
			for _, config := range entry.configs {
				if filepath.Dir(config) != ownEtc {
					t.Errorf("%s %s: config %s is not under %s", suiteName, nick, config, ownEtc)
				}
				if _, err := os.Stat(config); err != nil {
					t.Errorf("%s %s: %v", suiteName, nick, err)
				}
			}
		}
	}
}
