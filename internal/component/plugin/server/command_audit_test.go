package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
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

// TestUnmatchedLineLogsNoContent proves a line no command matches is logged
// with the content after the `content` keyword replaced by the placeholder.
//
// VALIDATES: AC-8, its log half: a mistyped `update data key` (here `update
// data kee`) reaches the dispatcher's debug line for an unmatched command,
// which records the line, and the content is absent from it. Method: capture
// the package logger at debug level, dispatch the line through a dispatcher
// that serves no such command, and read the captured text. The line is
// lowercased before matching, so the content is checked in that form.
// PREVENTS: the base64 of a credential written in clear to the daemon log
// because the operator mistyped the command around it.
func TestUnmatchedLineLogsNoContent(t *testing.T) {
	orig := logger
	t.Cleanup(func() { logger = orig })
	var buf bytes.Buffer
	captured := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger = func() *slog.Logger { return captured }

	line := "update data kee meta/ddos-flowspec/announcement content " + auditedContent
	_, err := NewDispatcher().Dispatch(&CommandContext{Username: "alice"}, line)
	if err == nil {
		t.Fatalf("dispatch of %q succeeded, want the unknown-command refusal", line)
	}
	text := buf.String()
	if !strings.Contains(text, "dispatchPlugin: no match") {
		t.Fatalf("the unmatched line wrote no debug line, so this test proves nothing:\n%s", text)
	}
	if strings.Contains(text, strings.ToLower(auditedContent)) {
		t.Errorf("the debug line holds the content:\n%s", text)
	}
	if !strings.Contains(text, "content "+redact.Placeholder) {
		t.Errorf("the debug line lacks the placeholder after content:\n%s", text)
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
		loader, err := yang.DefaultLoader()
		if err != nil {
			t.Fatal(err)
		}
		if err := LoadBuiltins(d, wireToPath, nil, nil, nil, yang.BuildCommandTree(loader)); err != nil {
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

const (
	explicitBundlePath       = "show pki certificate name bundle pem"
	explicitBundleLine       = "show pki certificate name dev-1 bundle pem confirm"
	explicitBundleUnconfLine = "show pki certificate name dev-1 bundle pem"
	explicitBundleSecretBody = "MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwTESTSECRETKEY"
	explicitBundleFakePEM    = "-----BEGIN CERTIFICATE-----\nCERTBODY\n-----END CERTIFICATE-----\n" +
		"-----BEGIN PRIVATE KEY-----\n" + explicitBundleSecretBody + "\n-----END PRIVATE KEY-----\n" // secret-scan: allow
)

func explicitAuthzTestStore() *authz.Store {
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "admin",
		Run:  authz.Section{Default: authz.Allow},
		Edit: authz.Section{Default: authz.Allow},
	})
	store.AddProfile(authz.Profile{
		Name: "pki-exporter",
		Run:  authz.Section{Default: authz.Deny},
		Edit: authz.Section{
			Default: authz.Deny,
			Entries: []authz.Entry{
				{Number: 10, Action: authz.Allow, Match: explicitBundlePath},
			},
		},
	})
	store.AssignProfiles("alice", []string{"admin"})
	store.AssignProfiles("exporter", []string{"pki-exporter"})
	return store
}

func explicitBundleDispatcher(t *testing.T, called *bool) (*Dispatcher, *audit.Log) {
	t.Helper()
	d := NewDispatcher()
	argDefs := []command.ArgDef{
		commandtest.Must(command.NewStringArg("name", nil, nil, command.ArgOptions{Mandatory: true})),
		commandtest.Must(command.NewFlagArg("confirm", command.ArgOptions{Mandatory: true})),
	}
	handler := func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		*called = true
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"pem": explicitBundleFakePEM},
		}, nil
	}
	if err := d.RegisterWithOptions(explicitBundlePath, handler, "export cert and private key", RegisterOptions{
		ReadOnly:             false,
		RequireExplicitAuthz: true,
		ArgDefs:              argDefs,
		AuditAction:          audit.ActionPKIPrivateKeyExport,
	}); err != nil {
		t.Fatal(err)
	}
	log, err := audit.NewMemory(audit.MinMaxEntries)
	if err != nil {
		t.Fatal(err)
	}
	d.SetAuditRecorder(log)
	return d, log
}

// TestDispatcherExplicitAuthzAndRedactedAudit proves a command registered with
// RequireExplicitAuthz fails closed across Dispatch, wrapHandler, and
// handleArgComplete, and records redacted OutcomeDenied/OutcomeSuccess audit
// entries with zero private-key bytes.
//
// VALIDATES: AC-3-bundle, AC-4, AC-6 at the daemon dispatcher chokepoint:
// RequireExplicitAuthz denies nil authorizer, default-action allow profiles,
// wrapHandler internalRPCIdentity, and handleArgComplete; missing confirm and
// authz refusals record OutcomeDenied; confirmed explicit runs record
// OutcomeSuccess with zero private-key bytes in audit entries.
// PREVENTS: private-key extraction bypassing RBAC through unconfigured authz,
// default-action allow, wire RPC wrapHandler, or argument completion, and
// private-key bytes leaking into audit logs.
func TestDispatcherExplicitAuthzAndRedactedAudit(t *testing.T) {
	t.Run("loader propagates RequireExplicitAuthz", func(t *testing.T) {
		var ran bool
		regs := []RPCRegistration{
			{
				WireMethod:           "ze-alpha:secret",
				Handler:              handlerMarking(&ran),
				NotReadOnly:          true,
				RequireExplicitAuthz: true,
				AuditAction:          audit.ActionPKIPrivateKeyExport,
			},
		}
		loader, err := yang.DefaultLoader()
		if err != nil {
			t.Fatal(err)
		}
		wireToPaths := yang.WireMethodToPaths(loader)
		const secretPath = "show alpha secret"
		wireToPaths["ze-alpha:secret"] = []string{secretPath}
		d := NewDispatcher()
		if err := loadBuiltinsWithAliases(d, regs, wireToPaths, nil, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
		cmd := d.Lookup(secretPath)
		if cmd == nil {
			t.Fatalf("%q not registered", secretPath)
		}
		if !cmd.RequireExplicitAuthz {
			t.Errorf("%q RequireExplicitAuthz = false, want true", secretPath)
		}
		if cmd.ReadOnly {
			t.Errorf("%q ReadOnly = true, want false", secretPath)
		}
		if cmd.AuditAction != audit.ActionPKIPrivateKeyExport {
			t.Errorf("%q AuditAction = %q, want %q", secretPath, cmd.AuditAction, audit.ActionPKIPrivateKeyExport)
		}
	})

	t.Run("nil authorizer denies and audits OutcomeDenied", func(t *testing.T) {
		var called bool
		d, log := explicitBundleDispatcher(t, &called)

		_, err := d.Dispatch(&CommandContext{
			Username:   "alice",
			RemoteAddr: "192.0.2.10:2222",
			Surface:    audit.SSH,
		}, explicitBundleLine)
		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("dispatch with nil authorizer err = %v, want ErrUnauthorized", err)
		}
		if called {
			t.Fatal("handler executed despite nil authorizer on RequireExplicitAuthz command")
		}
		entries := log.Query(audit.Filter{})
		if len(entries) != 1 {
			t.Fatalf("got %d audit entries, want 1: %+v", len(entries), entries)
		}
		entry := entries[0]
		if entry.Action != audit.ActionPKIPrivateKeyExport {
			t.Errorf("action = %q, want %q", entry.Action, audit.ActionPKIPrivateKeyExport)
		}
		if entry.Outcome != audit.OutcomeDenied {
			t.Errorf("outcome = %q, want %q", entry.Outcome, audit.OutcomeDenied)
		}
		if strings.Contains(entry.Detail, "PRIVATE KEY") || strings.Contains(entry.Detail, explicitBundleSecretBody) {
			t.Errorf("audit detail leaked private key material: %q", entry.Detail)
		}
	})

	t.Run("default-action allow denies and audits OutcomeDenied", func(t *testing.T) {
		var called bool
		d, log := explicitBundleDispatcher(t, &called)
		d.SetAuthorizer(authz.StoreAuthorizer{Store: explicitAuthzTestStore()})

		_, err := d.Dispatch(&CommandContext{
			Username:   "alice",
			RemoteAddr: "192.0.2.11:2222",
			Surface:    audit.SSH,
		}, explicitBundleLine)
		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("dispatch for default-action allow user err = %v, want ErrUnauthorized", err)
		}
		if called {
			t.Fatal("handler executed for default-action allow profile without explicit entry")
		}
		entries := log.Query(audit.Filter{})
		if len(entries) != 1 {
			t.Fatalf("got %d audit entries, want 1: %+v", len(entries), entries)
		}
		if entries[0].Outcome != audit.OutcomeDenied {
			t.Errorf("outcome = %q, want %q", entries[0].Outcome, audit.OutcomeDenied)
		}
		if entries[0].Action != audit.ActionPKIPrivateKeyExport {
			t.Errorf("action = %q, want %q", entries[0].Action, audit.ActionPKIPrivateKeyExport)
		}
	})

	t.Run("authorized missing confirm refuses and audits OutcomeDenied", func(t *testing.T) {
		var called bool
		d, log := explicitBundleDispatcher(t, &called)
		d.SetAuthorizer(authz.StoreAuthorizer{Store: explicitAuthzTestStore()})

		_, err := d.Dispatch(&CommandContext{
			Username:   "exporter",
			RemoteAddr: "192.0.2.12:2222",
			Surface:    audit.SSH,
		}, explicitBundleUnconfLine)
		if err == nil {
			t.Fatal("dispatch without confirm succeeded, want missing-argument refusal")
		}
		if errors.Is(err, ErrUnauthorized) {
			t.Fatalf("dispatch without confirm returned ErrUnauthorized, want missing-confirm refusal: %v", err)
		}
		if !strings.Contains(err.Error(), "confirm") || !strings.Contains(err.Error(), explicitBundleLine) {
			t.Errorf("missing-confirm refusal %q does not name confirm and %q", err.Error(), explicitBundleLine)
		}
		if called {
			t.Fatal("handler executed without confirm keyword")
		}
		entries := log.Query(audit.Filter{})
		if len(entries) != 1 {
			t.Fatalf("got %d audit entries, want 1: %+v", len(entries), entries)
		}
		if entries[0].Action != audit.ActionPKIPrivateKeyExport {
			t.Errorf("action = %q, want %q", entries[0].Action, audit.ActionPKIPrivateKeyExport)
		}
		if entries[0].Outcome != audit.OutcomeDenied {
			t.Errorf("outcome = %q, want %q", entries[0].Outcome, audit.OutcomeDenied)
		}
		if strings.Contains(entries[0].Detail, "PRIVATE KEY") || strings.Contains(entries[0].Detail, explicitBundleSecretBody) {
			t.Errorf("audit detail leaked private key material: %q", entries[0].Detail)
		}
	})

	t.Run("authorized confirmed run succeeds and audits OutcomeSuccess with no key bytes", func(t *testing.T) {
		var called bool
		d, log := explicitBundleDispatcher(t, &called)
		d.SetAuthorizer(authz.StoreAuthorizer{Store: explicitAuthzTestStore()})

		resp, err := d.Dispatch(&CommandContext{
			Username:   "exporter",
			RemoteAddr: "192.0.2.13:2222",
			Surface:    audit.SSH,
		}, explicitBundleLine)
		if err != nil {
			t.Fatalf("authorized confirmed dispatch returned error: %v", err)
		}
		if resp == nil || resp.Status != plugin.StatusDone {
			t.Fatalf("resp = %+v, want StatusDone", resp)
		}
		if !called {
			t.Fatal("handler was not invoked for authorized confirmed dispatch")
		}
		entries := log.Query(audit.Filter{})
		if len(entries) != 1 {
			t.Fatalf("got %d audit entries, want 1: %+v", len(entries), entries)
		}
		entry := entries[0]
		if entry.Action != audit.ActionPKIPrivateKeyExport {
			t.Errorf("action = %q, want %q", entry.Action, audit.ActionPKIPrivateKeyExport)
		}
		if entry.Outcome != audit.OutcomeSuccess {
			t.Errorf("outcome = %q, want %q", entry.Outcome, audit.OutcomeSuccess)
		}
		if entry.Actor != "exporter" {
			t.Errorf("actor = %q, want exporter", entry.Actor)
		}
		if strings.Contains(entry.Detail, "PRIVATE KEY") || strings.Contains(entry.Detail, explicitBundleSecretBody) {
			t.Errorf("audit detail leaked private key material: %q", entry.Detail)
		}
	})

	t.Run("authorized confirmed handler error audits OutcomeError with no key bytes", func(t *testing.T) {
		d := NewDispatcher()
		argDefs := []command.ArgDef{
			commandtest.Must(command.NewStringArg("name", nil, nil, command.ArgOptions{Mandatory: true})),
			commandtest.Must(command.NewFlagArg("confirm", command.ArgOptions{Mandatory: true})),
		}
		if err := d.RegisterWithOptions(explicitBundlePath, answering(&plugin.Response{
			Status: plugin.StatusError,
			Error:  "pki: bundle is only available for device certificates",
		}, nil), "export cert and private key", RegisterOptions{
			ReadOnly:             false,
			RequireExplicitAuthz: true,
			ArgDefs:              argDefs,
			AuditAction:          audit.ActionPKIPrivateKeyExport,
		}); err != nil {
			t.Fatal(err)
		}
		log, err := audit.NewMemory(audit.MinMaxEntries)
		if err != nil {
			t.Fatal(err)
		}
		d.SetAuditRecorder(log)
		d.SetAuthorizer(authz.StoreAuthorizer{Store: explicitAuthzTestStore()})

		resp, err := d.Dispatch(&CommandContext{
			Username:   "exporter",
			RemoteAddr: "192.0.2.14:2222",
			Surface:    audit.SSH,
		}, explicitBundleLine)
		if err != nil {
			t.Fatalf("unexpected Go error: %v", err)
		}
		if resp == nil || resp.Status != plugin.StatusError {
			t.Fatalf("resp = %+v, want StatusError", resp)
		}
		entries := log.Query(audit.Filter{})
		if len(entries) != 1 {
			t.Fatalf("got %d audit entries, want 1: %+v", len(entries), entries)
		}
		if entries[0].Action != audit.ActionPKIPrivateKeyExport {
			t.Errorf("action = %q, want %q", entries[0].Action, audit.ActionPKIPrivateKeyExport)
		}
		if entries[0].Outcome != audit.OutcomeError {
			t.Errorf("outcome = %q, want %q", entries[0].Outcome, audit.OutcomeError)
		}
		if strings.Contains(entries[0].Detail, "PRIVATE KEY") || strings.Contains(entries[0].Detail, explicitBundleSecretBody) {
			t.Errorf("audit detail leaked private key material: %q", entries[0].Detail)
		}
	})

	t.Run("wrapHandler denies internalRPCIdentity on RequireExplicitAuthz command", func(t *testing.T) {
		var called bool
		d, _ := explicitBundleDispatcher(t, &called)
		d.SetAuthorizer(authz.StoreAuthorizer{Store: explicitAuthzTestStore()})
		s := &Server{ctx: context.Background(), dispatcher: d}

		rpcHandler := s.wrapHandler(func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
			called = true
			return &plugin.Response{Status: plugin.StatusDone}, nil
		}, explicitBundlePath, false)

		_, err := rpcHandler("ze-pki:show-certificate-bundle-pem", json.RawMessage(`{"args":["dev-1","confirm"]}`))
		if err == nil {
			t.Fatal("wrapHandler allowed internalRPCIdentity on RequireExplicitAuthz command")
		}
		if !strings.Contains(err.Error(), plugin.UnauthorizedMessage) {
			t.Errorf("wrapHandler err = %q, want substring %q", err.Error(), plugin.UnauthorizedMessage)
		}
		if called {
			t.Fatal("wrapped handler executed for internalRPCIdentity")
		}
	})

	t.Run("handleArgComplete denies non-explicit caller on RequireExplicitAuthz command", func(t *testing.T) {
		var called bool
		d, _ := explicitBundleDispatcher(t, &called)
		d.SetAuthorizer(authz.StoreAuthorizer{Store: explicitAuthzTestStore()})
		s := &Server{ctx: context.Background(), dispatcher: d}

		_, err := handleArgComplete(&CommandContext{
			Server:         s,
			RequestContext: context.Background(),
			Username:       "alice",
		}, explicitBundlePath, []string{"dev-1"}, "")
		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("handleArgComplete for default-action allow user err = %v, want ErrUnauthorized", err)
		}
	})
}
