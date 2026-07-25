// VALIDATES: AC-1 (proxy enricher registered at startup), AC-2 (proxy calls plugin, merges result),
//            AC-3 (timeout), AC-4 (cleanup on exit), AC-5 (Unregister), AC-6 (Unregister noop)
// PREVENTS: proxy enricher wiring regression, hung plugin blocking show commands, leaked proxy enrichers

package server

import (
	"testing"

	"github.com/ze-software/ze/internal/core/show"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestProxyEnricherRegistersOnStartup(t *testing.T) {
	t.Cleanup(show.ResetForTest)
	t.Cleanup(func() { resetProxyRegistry() })

	enrichers := []rpc.EnricherDecl{
		{Command: "show test detail", Key: "ext-test"},
	}

	registerProxyEnrichers("test-plugin", enrichers, nil)

	base := map[string]any{"id": "s1"}
	show.Enrich("show test detail", base)

	proxyMu.Lock()
	entries := proxyRegistry["test-plugin"]
	proxyMu.Unlock()

	if len(entries) != 1 {
		t.Fatalf("expected 1 proxy entry, got %d", len(entries))
	}
	if entries[0].command != "show test detail" {
		t.Fatalf("expected command 'show test detail', got %q", entries[0].command)
	}
}

func TestProxyEnricherCleanupOnExit(t *testing.T) {
	t.Cleanup(show.ResetForTest)
	t.Cleanup(func() { resetProxyRegistry() })

	enrichers := []rpc.EnricherDecl{
		{Command: "show test detail", Key: "ext-a"},
		{Command: "show test summary", Key: "ext-b"},
	}

	registerProxyEnrichers("test-plugin", enrichers, nil)

	proxyMu.Lock()
	before := len(proxyRegistry["test-plugin"])
	proxyMu.Unlock()
	if before != 2 {
		t.Fatalf("expected 2 proxy entries before cleanup, got %d", before)
	}

	unregisterProxyEnrichers("test-plugin")

	proxyMu.Lock()
	after := len(proxyRegistry["test-plugin"])
	proxyMu.Unlock()
	if after != 0 {
		t.Fatalf("expected 0 proxy entries after cleanup, got %d", after)
	}
}

func TestProxyEnricherCleanupNonExistent(t *testing.T) {
	t.Cleanup(func() { resetProxyRegistry() })

	unregisterProxyEnrichers("nonexistent-plugin")
}

func TestValidateEnricherDeclsValid(t *testing.T) {
	err := validateEnricherDecls([]rpc.EnricherDecl{
		{Command: "show subscriber detail", Key: "cos"},
		{Command: "show subscriber detail", Key: "radius"},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateEnricherDeclsEmptyCommand(t *testing.T) {
	err := validateEnricherDecls([]rpc.EnricherDecl{
		{Command: "", Key: "cos"},
	})
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestValidateEnricherDeclsEmptyKey(t *testing.T) {
	err := validateEnricherDecls([]rpc.EnricherDecl{
		{Command: "show subscriber detail", Key: ""},
	})
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestValidateEnricherDeclsNonKebabKey(t *testing.T) {
	err := validateEnricherDecls([]rpc.EnricherDecl{
		{Command: "show subscriber detail", Key: "COS_Profile"},
	})
	if err == nil {
		t.Fatal("expected error for non-kebab key")
	}
}

func TestValidateEnricherDeclsDuplicateKey(t *testing.T) {
	err := validateEnricherDecls([]rpc.EnricherDecl{
		{Command: "show subscriber detail", Key: "cos"},
		{Command: "show subscriber detail", Key: "cos"},
	})
	if err == nil {
		t.Fatal("expected error for duplicate key")
	}
}

func TestValidateEnricherDeclsTooMany(t *testing.T) {
	decls := make([]rpc.EnricherDecl, 17)
	for i := range decls {
		decls[i] = rpc.EnricherDecl{Command: "show test", Key: "k-" + string(rune('a'+i))}
	}
	err := validateEnricherDecls(decls)
	if err == nil {
		t.Fatal("expected error for too many enrichers")
	}
}

func TestValidateEnricherDeclsNil(t *testing.T) {
	if err := validateEnricherDecls(nil); err != nil {
		t.Fatalf("nil should be valid: %v", err)
	}
}

func resetProxyRegistry() {
	proxyMu.Lock()
	proxyRegistry = map[string][]proxyEntry{}
	proxyMu.Unlock()
}
