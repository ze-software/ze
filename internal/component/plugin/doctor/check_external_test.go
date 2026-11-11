package doctor

import (
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestExternalPluginNameCheck is AC-23's local half: a config already on disk
// with a name the connect-back refuses gets doctor-plugin-connect-back-refused
// at the parsed-config phase, naming the block, the allowed form and a fix; a
// valid name gets nothing.
func TestExternalPluginNameCheck(t *testing.T) {
	tree, err := config.ParseTreeForValidation("plugin {\n\texternal a_b {\n\t\trun \"/bin/true\"\n\t}\n\texternal good-one {\n\t\trun \"/bin/true\"\n\t}\n}\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rows := checkExternalPluginNames(diagnostic.DoctorCheckContext{Tree: tree})
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want one for a_b", rows)
	}
	row := rows[0]
	if row.Code != diagnostic.CodeDoctorPluginConnectBackRefused || row.Severity != diagnostic.SeverityError {
		t.Errorf("row = %+v", row)
	}
	if !strings.Contains(row.Message, `"a_b"`) || row.Fix == "" {
		t.Errorf("row does not name the block or carries no fix: %+v", row)
	}

	// A run that hands the check no parsed tree gets a row saying the names
	// were not checked, never the empty answer a clean config gets.
	unread := checkExternalPluginNames(diagnostic.DoctorCheckContext{})
	if len(unread) != 1 || !strings.Contains(unread[0].Message, "were not checked") {
		t.Errorf("no tree: rows = %+v, want one saying the names were not checked", unread)
	}
}

// TestExternalUnsupportedCheckReadsField is AC-24: an external block running a
// registration that declares RunsInProcessOnly gets
// doctor-plugin-external-unsupported with the fix "move it to an internal
// block"; the same block as internal, and a plugin without the field, get
// nothing. The check reads the field, so the fixture registration alone makes
// it fire.
func TestExternalUnsupportedCheckReadsField(t *testing.T) {
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	for _, reg := range []registry.Registration{
		{Name: "in-process-only-fixture", Description: "test", RunEngine: func(_ net.Conn) int { return 0 }, CLIHandler: func([]string) int { return 0 }, RunsInProcessOnly: true},
		{Name: "forkable-fixture", Description: "test", RunEngine: func(_ net.Conn) int { return 0 }, CLIHandler: func([]string) int { return 0 }},
	} {
		if err := registry.Register(reg); err != nil {
			t.Fatalf("register %s: %v", reg.Name, err)
		}
	}

	rows := checkExternalInProcess(diagnostic.DoctorCheckContext{Plugins: []zeplugin.PluginConfig{
		{Name: "in-process-only-fixture", Run: "ze plugin in-process-only-fixture"},
		{Name: "forkable-fixture", Run: "ze plugin forkable-fixture"},
		{Name: "as-internal", Internal: true, Run: "in-process-only-fixture"},
	}})
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want one", rows)
	}
	if rows[0].Code != diagnostic.CodeDoctorPluginExternalUnsupported {
		t.Errorf("code = %q", rows[0].Code)
	}
	if !strings.HasPrefix(rows[0].Fix, "move it to an internal block") {
		t.Errorf("fix = %q", rows[0].Fix)
	}
}

// TestManagedChecksAnswerNotServing is AC-25 and AC-26 with no managed server:
// no listener was asked for and no client refused, so both checks answer no
// row rather than failing. The rows themselves are TestManagedClientRejectedRow
// and the server package's TestManagedListenerUnboundRecorded.
func TestManagedChecksAnswerNotServing(t *testing.T) {
	if _, err := server.ManagedDoctorSnapshot(); err == nil {
		t.Skip("a managed server is published in this process")
	}
	if rows := checkManagedListenerUnbound(diagnostic.DoctorCheckContext{}); len(rows) != 0 {
		t.Errorf("unbound rows = %+v", rows)
	}
	if rows := checkManagedClientRejected(diagnostic.DoctorCheckContext{}); len(rows) != 0 {
		t.Errorf("rejected rows = %+v", rows)
	}
}

// TestManagedClientRejectedRow is AC-26's row: each cause names the client,
// the detail and its own fix, at warning severity.
func TestManagedClientRejectedRow(t *testing.T) {
	causes := []server.ManagedRejectCause{
		server.ManagedRejectNameConnected, server.ManagedRejectConfigRejected,
	}
	for _, cause := range causes {
		row := managedRejectedRow(server.ManagedClientRejection{Client: "edge-1", Cause: cause, Detail: "the detail"})
		if row.Code != diagnostic.CodeDoctorManagedClientRejected || row.Severity != diagnostic.SeverityWarning {
			t.Errorf("cause %d: row = %+v", cause, row)
		}
		if !strings.Contains(row.Message, `"edge-1"`) || !strings.Contains(row.Message, "the detail") || row.Fix == "" {
			t.Errorf("cause %d: row = %+v", cause, row)
		}
	}
}
