// Design: docs/architecture/cli/error-surface.md -- recorded-window recovery refusal.
package confirm_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestStoppedWindowRefusalCarriesItsNativeRecovery keeps the original window
// identity while exposing the shared operator error and a copyable start
// command, including a configuration path that requires quoting.
func TestStoppedWindowRefusalCarriesItsNativeRecovery(t *testing.T) {
	const path = "saved router/router.conf"
	stopped := &confirm.StoppedWindowError{Owner: "alice", ConfigPath: path}
	wrapped := fmt.Errorf("stored rollback: %w", stopped)
	refusal, typed := errors.AsType[*opererr.Error](wrapped)
	if !typed {
		t.Fatalf("recorded-window refusal has no operator error: %v", wrapped)
	}
	if !strings.Contains(refusal.Problem(), "alice") {
		t.Fatalf("refusal lost the unconfirmed change's owner: %v", wrapped)
	}
	if !strings.Contains(refusal.Why(), "when the daemon starts") {
		t.Fatalf("refusal lost the boot-recovery requirement: %v", wrapped)
	}
	words, _, err := command.ScanExecution(refusal.Fix())
	if err != nil {
		t.Fatalf("the recovery command cannot be parsed: %v", err)
	}
	if !slices.Equal(words, []string{"ze", "start", path}) {
		t.Fatalf("recovery command words = %q, want the selected configuration", words)
	}
	if !strings.HasSuffix(stopped.Error(), "\nfix: "+refusal.Fix()) {
		t.Fatalf("refusal does not end with its exact recovery command: %v", stopped)
	}
	if strings.Count(stopped.Error(), "\nfix: ") != 1 {
		t.Fatalf("refusal has more than one final fix: %v", stopped)
	}
	if !errors.Is(wrapped, stopped) {
		t.Fatal("native presentation discarded the original window error identity")
	}
	selected, found := errors.AsType[*confirm.StoppedWindowError](wrapped)
	if !found {
		t.Fatal("native presentation discarded the recorded-window error type")
	}
	if selected != stopped {
		t.Fatal("native presentation replaced the original recorded-window error")
	}
}
