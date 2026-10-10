package client

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// VALIDATES: AC-22 for the offline fallback route (R7). The words after the
// fallback's path are judged against the leaves the model declares for it
// before the fallback runs, and a token at the bound reaches it.
// PREVENTS: a daemon-down fallback answering on a value the daemon would
// refuse.
// METHOD: a fallback registered at the real model path `show metrics name`
// (leaf name, length 1..128, mandatory), which ships no fallback of its own.
func TestOfflineFallbackJudgesArguments(t *testing.T) {
	var got []string
	ran := false
	if err := command.RegisterOfflineFallback("show metrics name", func(validated command.ValidatedArgs) int {
		args := validated.Tokens()
		ran, got = true, args
		return 0
	}); err != nil {
		t.Fatal(err)
	}
	if code, served := runOfflineFallback("show metrics name " + strings.Repeat("m", 129)); !served || code != 1 {
		t.Errorf("over-long name: code %d served %v, want 1 true", code, served)
	}
	if ran {
		t.Fatalf("an over-long name reached the fallback as %q", got)
	}
	atBound := strings.Repeat("m", 128)
	if code, _ := runOfflineFallback("show metrics name " + atBound); code != 0 {
		t.Errorf("name at the bound: exit %d, want 0", code)
	}
	if len(got) != 1 || got[0] != atBound {
		t.Errorf("fallback args = %q, want the name at the bound", got)
	}
}
