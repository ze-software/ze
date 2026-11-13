// Design: docs/architecture/isis/isis-13-cli-diag-interop.md -- actionable plugin command refusals.
// Related: register.go -- the real OnExecuteCommand handler under test.
package isis

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestISISUnknownCommandReportsRepair sends an unknown request through the real
// SDK command callback in socket and direct modes. Startup and a later valid
// callback are positive controls: a broken transport cannot satisfy the refusal.
func TestISISUnknownCommandReportsRepair(t *testing.T) {
	const (
		initial = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l2","interfaces":{"interface":{"isis-on-a":{"circuit-type":"point-to-point","hello-interval":"3600"}}}}}`
		unknown = "show isis no-such-view"
		why     = "The IS-IS plugin does not provide this command."
		fix     = "ze help command isis"
	)
	for _, bridged := range []bool{false, true} {
		t.Run(fmt.Sprintf("bridge=%t", bridged), func(t *testing.T) {
			useOnLinkBackend(t)
			eng, call := startISISRuntimeRollbackPluginWithBackend(t, bridged, initial, &fakeBackend{})
			assertISISRollbackRuntime(t, eng, initial, "startup")
			// executeCommandAnswer returns a handler error before record-answer
			// admission, so the callback helper reads this refusal on either path.
			err := call("ze-plugin-callback:execute-command", rpc.ExecuteCommandInput{Serial: "unknown-isis-command", Command: unknown})
			if err == nil {
				t.Fatal("unknown IS-IS command was accepted")
			}
			message := err.Error()
			if !strings.Contains(message, "unknown command: "+unknown) {
				t.Errorf("refusal lost the requested command: %q", message)
			}
			if !strings.Contains(message, "\n"+why+"\n") {
				t.Errorf("refusal omitted the operator explanation: %q", message)
			}
			if !strings.HasSuffix(message, "\nfix: "+fix) {
				t.Errorf("refusal must end with its executable repair command: %q", message)
			}
			if bridged {
				var operatorError *opererr.Error
				if !errors.As(err, &operatorError) {
					t.Errorf("direct callback returned %T instead of an operator error", err)
				}
			}
			if err := call("ze-plugin-callback:config-verify", rpc.ConfigVerifyInput{Sections: []rpc.ConfigSection{{Root: configRoot, Data: initial}}}); err != nil {
				t.Fatalf("valid callback after command refusal: %v", err)
			}
			assertISISRollbackRuntime(t, eng, initial, "after command refusal")
		})
	}
}
