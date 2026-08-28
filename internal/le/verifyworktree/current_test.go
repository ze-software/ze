// VALIDATES: current-checkout full and changed entry points select distinct
// native populations, preserve mode certificates, and publish reader artifacts.
package verifyworktree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/verify"
)

func TestRunCurrentFullAndChangedModes(t *testing.T) {
	for _, test := range []struct {
		mode        string
		wantMode    string
		firstAction string
		omits       string
	}{
		{mode: "full", wantMode: verify.Mode, firstAction: "verify-lint/run"},
		{mode: "changed", wantMode: verify.ChangedMode, firstAction: "verify-lint/run", omits: "verify-deps/alloc"},
	} {
		t.Run(test.mode, func(t *testing.T) {
			repo := newFixtureRepo(t)
			repo.commit(t, "fixture", "one")
			var called []string
			runner := func(_ context.Context, _ string, identity verify.Identity) verify.ActionResult {
				called = append(called, identity.Name)
				return verify.ActionResult{Identity: identity, Registered: true, Completed: true, Output: identity.Name + " ok\n"}
			}
			report := runCurrent(context.Background(), repo.root, test.mode, runner)
			if report.Code != 0 || !report.Completed || report.Mode != test.wantMode {
				t.Fatalf("report = %#v", report)
			}
			if len(called) == 0 || called[0] != test.firstAction {
				t.Fatalf("called stages = %q", called)
			}
			if test.omits != "" && strings.Contains(strings.Join(called, " "), test.omits) {
				t.Fatalf("changed mode called full-only %s", test.omits)
			}
			certificate, err := verify.ReadCertificate(repo.root)
			if err != nil || certificate.Mode != test.wantMode {
				t.Fatalf("certificate = %#v, err %v", certificate, err)
			}
			for _, rel := range []string{verify.CombinedLogPath, verify.FailuresLogPath, verify.FailuresJSONPath} {
				if _, err := os.Stat(filepath.Join(repo.root, filepath.FromSlash(rel))); err != nil {
					t.Errorf("artifact %s: %v", rel, err)
				}
			}
			_, fullErr := os.Stat(filepath.Join(repo.root, filepath.FromSlash(verify.FullJSONPath)))
			if test.mode == "full" && fullErr != nil {
				t.Errorf("full index: %v", fullErr)
			}
			if test.mode == "changed" && !os.IsNotExist(fullErr) {
				t.Errorf("changed mode wrote full index: %v", fullErr)
			}
		})
	}
}

func TestListCurrentAndModeGrammarFailClosed(t *testing.T) {
	full, err := listCurrent("")
	if err != nil || full.Mode != verify.Mode || full.Stages[0].Name != "verify-lint/run" {
		t.Fatalf("full list = %#v, err %v", full, err)
	}
	changed, err := listCurrent("changed")
	if err != nil || changed.Mode != verify.ChangedMode || changed.Stages[0].Name != "verify-lint/run" {
		t.Fatalf("changed list = %#v, err %v", changed, err)
	}
	if _, err := listCurrent("chnaged"); err == nil {
		t.Fatal("unknown mode was accepted")
	}
}
