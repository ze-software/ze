// Design: docs/architecture/testing/interop.md -- fail-closed native read receipts.
package bgp

import (
	"context"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestGoBGPReloadReadReceiptsStrict refuses stale, wrong-file, overflow, partial
// and write-close events. These unit discriminators are not a native reload proof.
func TestGoBGPReloadReadReceiptsStrict(t *testing.T) {
	if err := requireGoBGPReloadReadEvent("/run/gobgp.toml|CLOSE_NOWRITE,CLOSE\n"); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{
		"", "/run/other.toml|CLOSE_NOWRITE,CLOSE\n", "/run/gobgp.toml|CLOSE_WRITE,CLOSE\n",
		"/run/gobgp.toml|IN_Q_OVERFLOW\n", "/run/gobgp.toml|CLOSE_NOWRITE\n",
		"/run/gobgp.toml|CLOSE_NOWRITE,CLOSE\n/run/gobgp.toml|CLOSE_NOWRITE,CLOSE\n",
		"/run/gobgp.toml|", "CLOSE_NOWRITE,CLOSE", "/run/gobgp.toml|CLOSE_NOWRITE,CLOSE,IGNORED\n",
	} {
		if requireGoBGPReloadReadEvent(output) == nil {
			t.Fatalf("invalid native read receipt passed: %q", output)
		}
	}
	if !gobgpReloadReadWatchReady("Setting up watches.\nWatches established.\n") {
		t.Fatal("native armed watch was not recognized")
	}
	for _, output := range []string{"", "Setting up watches.\n", "not Watches established.\n", "Watches established later.\n"} {
		if gobgpReloadReadWatchReady(output) {
			t.Fatalf("unarmed native watch passed: %q", output)
		}
	}
}

type gobgpReloadCommandLab struct {
	interoplab.CheckerLab
	result  interoplab.CommandResult
	failure error
}

func (lab *gobgpReloadCommandLab) Exec(context.Context, string, []string, []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	return lab.result, lab.failure
}

// TestGoBGPReloadCommandsKeepExitTruth checks the native command/observation
// boundary: successful empty stdout is normal for cp/pkill, never for a query.
func TestGoBGPReloadCommandsKeepExitTruth(t *testing.T) {
	lab := &gobgpReloadCommandLab{}
	proof := gobgpReloadProof{check: &interoplab.CheckContext{Lab: lab}, evidence: make(map[string]string)}
	if err := proof.exec(t.Context(), "copy", []string{"cp", "source", "target"}); err != nil {
		t.Fatal(err)
	}
	if proof.evidence["copy-exit"] != "0" {
		t.Fatal("successful native command exit was not retained")
	}
	lab.result.ExitCode = 1
	if proof.exec(t.Context(), "copy", []string{"cp", "source", "target"}) == nil {
		t.Fatal("failed native copy passed")
	}
	lab.result.ExitCode = 0
	lab.failure = errors.New("native exec unavailable")
	if err := proof.exec(t.Context(), "copy", []string{"cp", "source", "target"}); !errors.Is(err, lab.failure) {
		t.Fatalf("native execution failure lost: %v", err)
	}
}
