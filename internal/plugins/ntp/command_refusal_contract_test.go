// Design: docs/architecture/cli/error-surface.md -- every command refusal carries its recovery.
// Related: register_commands.go -- the runtime command callback owns these refusals.

package ntp

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestNTPRuntimeRefusalsCarryOperatorRecovery checks the errors before they
// cross SDK serialization: every branch retains its cause and a usable command.
func TestNTPRuntimeRefusalsCarryOperatorRecovery(t *testing.T) {
	previous := loadState()
	t.Cleanup(func() { storeState(previous) })
	storeState(nil)
	for _, current := range []struct {
		name    string
		command string
		args    []string
		cause   string
	}{
		{name: "arguments", command: commandNTPStatus, args: []string{"unexpected"}, cause: "takes no arguments"},
		{name: "unknown command", command: "show system unknown", cause: "unknown NTP command"},
		{name: "unpublished status", command: commandNTPStatus, cause: "has not published"},
		{name: "unpublished peers", command: commandNTPPeers, cause: "has not published"},
	} {
		t.Run(current.name, func(t *testing.T) {
			status, data, err := executeNTPCommand("1", current.command, current.args, "")
			if status != plugin.StatusError || data != nil || err == nil {
				t.Fatalf("runtime refusal status=%q data=%v error=%v", status, data, err)
			}
			var refusal *opererr.Error
			if !errors.As(err, &refusal) {
				t.Fatalf("runtime refusal has type %T, want an operator error: %v", err, err)
			}
			if refusal.Fix() != "ze show system ntp" {
				t.Errorf("runtime recovery=%q, want ze show system ntp", refusal.Fix())
			}
			if !strings.Contains(err.Error(), current.cause) {
				t.Errorf("runtime refusal lost cause %q: %v", current.cause, err)
			}
		})
	}
}
