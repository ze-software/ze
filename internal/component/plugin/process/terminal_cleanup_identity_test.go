// Design: docs/architecture/api/process-protocol.md -- original process diagnostic identity
// Related: terminal_cleanup_test.go -- actual registered runner and terminal join
package process

import (
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// A valid internal label can contain 64 Unicode characters. Preserve that
// exact immutable identity in detail even when the display text is bounded.
func TestExitCleanupInternalKeepsCompleteUnicodeProcessIdentity(t *testing.T) {
	exitCleanupReset(t)
	const implementation = "terminal-unicode-binding"
	exitCleanupRegister(t, implementation, true, func(conn net.Conn) int {
		drainUntilClosed(conn)
		exitCleanupRecord(t, conn, exitCleanupCause)
		return 1
	})
	name := strings.Repeat("é", 64)
	proc := NewProcess(plugin.PluginConfig{Name: name, Run: "ze." + implementation, Internal: true, Encoder: "json"})
	if err := proc.StartWithContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	proc.SetRegistration(&plugin.PluginRegistration{ExitConfirmsCleanup: true})
	t.Cleanup(func() { proc.Stop(); exitCleanupJoin(t, proc) })
	proc.Stop()
	exitCleanupJoin(t, proc)
	issue := exitCleanupIssue(t, exitCleanupCause)
	if got := issue.Detail["process"]; got != name {
		t.Fatalf("original full process identity changed: got=%q want=%q", got, name)
	}
}
