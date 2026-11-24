// Design: docs/architecture/api/process-protocol.md -- every engine operation declares its bus message or exemption reason (AC-32, Q-T1(a)).
// Related: ../server/bus_op_gate.go -- ValidateAllEngineOps and ValidateEngineOpEntry.
// Related: all_test.go -- composition-root invariants over compiled-in registrations.

package all_test

import (
	"strings"
	"testing"

	_ "github.com/ze-software/ze/internal/component/plugin/all"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestEveryEngineOpDeclaresItsMessage enforces AC-32 and Q-T1(a) across every
// built-in and owner-registered engine operation linked into the composition
// root.
//
// VALIDATES: pluginserver.ValidateAllEngineOps passes with every compiled-in
// component and owner registered, and pluginserver.ValidateEngineOpEntry
// refuses an entry with neither BusMessage nor BusExemptReason (naming the
// offending operation) as well as an entry declaring both.
// PREVENTS: A built-in or owner-registered engine operation shipping without
// either a bus send gate message or an explicit written exemption reason.
func TestEveryEngineOpDeclaresItsMessage(t *testing.T) {
	t.Run("all-compiled-engine-ops", func(t *testing.T) {
		if err := pluginserver.ValidateAllEngineOps(); err != nil {
			t.Fatalf("ValidateAllEngineOps failed: %v", err)
		}
	})

	t.Run("missing-declaration-refused", func(t *testing.T) {
		const opName = "ze-test:missing-declaration"
		err := pluginserver.ValidateEngineOpEntry(opName, "", "")
		if err == nil {
			t.Fatalf("ValidateEngineOpEntry(%q, \"\", \"\") = nil, want error", opName)
		}
		if !strings.Contains(err.Error(), opName) {
			t.Errorf("ValidateEngineOpEntry error %q does not name %q", err.Error(), opName)
		}
	})

	t.Run("both-set-refused", func(t *testing.T) {
		const opName = "ze-test:both-set"
		err := pluginserver.ValidateEngineOpEntry(opName, "bgp:update", "redundant")
		if err == nil {
			t.Fatalf("ValidateEngineOpEntry(%q, \"bgp:update\", \"redundant\") = nil, want error", opName)
		}
		if !strings.Contains(err.Error(), opName) {
			t.Errorf("ValidateEngineOpEntry error %q does not name %q", err.Error(), opName)
		}
	})
}
