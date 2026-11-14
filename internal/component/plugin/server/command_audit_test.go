package server

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/redact"
)

// errAuditTestHandler is the failure the failing handler of these tests answers.
var errAuditTestHandler = errors.New("store write failed")

// auditedLine is a store mutation whose content must never reach the audit log.
const (
	auditedCommand = "update data key"
	auditedContent = "c2VjcmV0LWNyZWRlbnRpYWw="
	auditedLine    = auditedCommand + " meta/ddos-flowspec/announcement content " + auditedContent
)

// auditDispatcher answers a dispatcher holding auditedCommand, registered with
// action as its declared audit action and handler as its handler, and the
// memory log it records into.
func auditDispatcher(t *testing.T, action string, handler Handler) (*Dispatcher, *audit.Log) {
	t.Helper()
	d := NewDispatcher()
	if err := d.RegisterWithOptions(auditedCommand, handler, "write a key", RegisterOptions{AuditAction: action}); err != nil {
		t.Fatal(err)
	}
	log, err := audit.NewMemory(audit.MinMaxEntries)
	if err != nil {
		t.Fatal(err)
	}
	d.SetAuditRecorder(log)
	return d, log
}

// answering is a handler that returns resp and err.
func answering(resp *plugin.Response, err error) Handler {
	return func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		return resp, err
	}
}

// TestDeclaredAuditActionRecordsMaskedEntry proves a command whose registration
// declares an audit action records one entry when it succeeds, with the content
// replaced by the placeholder.
//
// VALIDATES: AC-8, audit half: the entry holds the declared action, the command
// path, the key and redact.Placeholder in place of the content, with the actor,
// remote address and surface of the caller. Method: dispatch the line through a
// dispatcher whose command declares audit.ActionDataKeyChange, then read the
// memory log.
// PREVENTS: a store mutation through the daemon leaving no audit entry, and a
// credential written with `update data key` read back in clear from `show audit`.
func TestDeclaredAuditActionRecordsMaskedEntry(t *testing.T) {
	d, log := auditDispatcher(t, audit.ActionDataKeyChange, answering(&plugin.Response{Status: plugin.StatusDone}, nil))

	if _, err := d.Dispatch(&CommandContext{Username: "alice", RemoteAddr: "192.0.2.10:2222", Surface: audit.SSH}, auditedLine); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	entries := log.Query(audit.Filter{})
	if len(entries) != 1 {
		t.Fatalf("got %d audit entries, want 1: %+v", len(entries), entries)
	}
	entry := entries[0]
	want := strings.TrimSuffix(auditedLine, auditedContent) + redact.Placeholder
	if entry.Detail != want {
		t.Errorf("detail %q, want %q", entry.Detail, want)
	}
	if strings.Contains(entry.Detail, auditedContent) {
		t.Errorf("detail %q holds the content", entry.Detail)
	}
	if entry.Action != audit.ActionDataKeyChange {
		t.Errorf("action %q, want %q", entry.Action, audit.ActionDataKeyChange)
	}
	if entry.Actor != "alice" {
		t.Errorf("actor %q, want alice", entry.Actor)
	}
	if entry.RemoteAddr != "192.0.2.10:2222" {
		t.Errorf("remote address %q, want 192.0.2.10:2222", entry.RemoteAddr)
	}
	if entry.Surface != audit.SSH {
		t.Errorf("surface %q, want %q", entry.Surface, audit.SSH)
	}
	if entry.Outcome != audit.OutcomeSuccess {
		t.Errorf("outcome %q, want %q", entry.Outcome, audit.OutcomeSuccess)
	}
}

// TestDeclaredAuditActionSkipsFailedRun proves a command that declares an audit
// action records nothing when its handler refuses or fails.
//
// VALIDATES: the log records what changed state: a refusal answer and a handler
// error each leave the log empty. Method: one dispatcher per outcome.
// PREVENTS: a refused store write read in `show audit` as a write that happened.
func TestDeclaredAuditActionSkipsFailedRun(t *testing.T) {
	cases := map[string]Handler{
		"refusal": answering(&plugin.Response{Status: plugin.StatusError, Error: "refused"}, nil),
		"error":   answering(nil, errAuditTestHandler),
	}
	for name, handler := range cases {
		t.Run(name, func(t *testing.T) {
			d, log := auditDispatcher(t, audit.ActionDataKeyChange, handler)
			// The answer is the handler's own: this test reads only the log.
			_, _ = d.Dispatch(&CommandContext{Username: "alice"}, auditedLine)
			if entries := log.Query(audit.Filter{}); len(entries) != 0 {
				t.Errorf("got %d audit entries, want 0: %+v", len(entries), entries)
			}
		})
	}
}

// TestUndeclaredAuditActionRecordsNothing proves a command that declares no
// audit action records no entry, whatever its text.
//
// VALIDATES: the declaration alone decides: the same successful line under a
// registration with no AuditAction leaves the log empty.
// PREVENTS: the dispatcher naming a command by its text to decide what is
// audited (ai/rules/principles.md, a central enumeration).
func TestUndeclaredAuditActionRecordsNothing(t *testing.T) {
	d, log := auditDispatcher(t, "", answering(&plugin.Response{Status: plugin.StatusDone}, nil))

	if _, err := d.Dispatch(&CommandContext{Username: "alice"}, auditedLine); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if entries := log.Query(audit.Filter{}); len(entries) != 0 {
		t.Errorf("got %d audit entries, want 0: %+v", len(entries), entries)
	}
}

// TestLoadersCarryTheDeclaredAuditAction proves both builtin loaders give the
// dispatched command the audit action its RPC registration declares, and that
// `request reload` declares audit.ActionDaemonReload.
//
// VALIDATES: the startup load (loadBuiltinsWithAliases over every linked
// registration and the YANG paths) and LoadBuiltins each serve `request reload`
// with audit.ActionDaemonReload, and loadBuiltinsWithAliases over a test
// registration on two paths leaves its AuditAction on both commands. Method:
// read the loaded commands; the reload handler itself is not run, because it
// needs a live server.
// PREVENTS: a loader that drops the field, so a declared command records
// nothing, and `request reload` losing its daemon-reload entry.
func TestLoadersCarryTheDeclaredAuditAction(t *testing.T) {
	t.Run("startup load request reload", func(t *testing.T) {
		loader, err := yang.DefaultLoader()
		if err != nil {
			t.Fatal(err)
		}
		d := NewDispatcher()
		if err := loadBuiltinsWithAliases(d, AllBuiltinRPCs(), yang.WireMethodToPaths(loader), yang.PathToDescription(loader),
			yang.PathToHelp(loader), yang.PathToArgDefs(loader), yang.BuildCommandTree(loader)); err != nil {
			t.Fatal(err)
		}
		cmd := d.Lookup("request reload")
		if cmd == nil {
			t.Fatal("request reload not served by the startup load")
		}
		if cmd.AuditAction != audit.ActionDaemonReload {
			t.Errorf("request reload audit action %q, want %q", cmd.AuditAction, audit.ActionDaemonReload)
		}
	})

	// Each load ends in loadLocalData, which refuses a linked local-data
	// command whose daemon path no YANG node declares, so every load here takes
	// the wire map of the YANG this binary links, never a hand-made subset.
	t.Run("LoadBuiltins request reload", func(t *testing.T) {
		d := NewDispatcher()
		wireToPath := internalBuildTestWireToPath(t)
		if err := LoadBuiltins(d, wireToPath, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
		// LoadBuiltins serves each wire method under one path, the shortest
		// alias, so the reload row is read under the path it was given.
		path := wireToPath["ze-plugin:system-daemon-reload"]
		if path == "" {
			t.Fatal("no YANG path declares ze-plugin:system-daemon-reload")
		}
		cmd := d.Lookup(path)
		if cmd == nil {
			t.Fatal("request reload not registered")
		}
		if cmd.AuditAction != audit.ActionDaemonReload {
			t.Errorf("request reload audit action %q, want %q", cmd.AuditAction, audit.ActionDaemonReload)
		}
	})

	t.Run("loadBuiltinsWithAliases", func(t *testing.T) {
		var ran bool
		regs := []RPCRegistration{
			{WireMethod: "ze-alpha:write", Handler: handlerMarking(&ran), AuditAction: audit.ActionDataKeyChange},
		}
		loader, err := yang.DefaultLoader()
		if err != nil {
			t.Fatal(err)
		}
		wireToPaths := yang.WireMethodToPaths(loader)
		alphaPaths := []string{"update alpha key", "set alpha key"}
		wireToPaths["ze-alpha:write"] = alphaPaths
		d := NewDispatcher()
		if err := loadBuiltinsWithAliases(d, regs, wireToPaths, nil, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
		for _, path := range alphaPaths {
			cmd := d.commands[path]
			if cmd == nil {
				t.Fatalf("%q not registered", path)
			}
			if cmd.AuditAction != audit.ActionDataKeyChange {
				t.Errorf("%q audit action %q, want %q", path, cmd.AuditAction, audit.ActionDataKeyChange)
			}
		}
	})
}
