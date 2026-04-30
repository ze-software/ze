package config

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

func TestVerifyPluginConfigRunsRegisteredVerifier(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	called := false
	if err := registry.Register(registry.Registration{
		Name:        "verify-test",
		Description: "test verifier",
		ConfigRoots: []string{"bgp"},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		InProcessConfigVerifier: func(sections []rpc.ConfigSection) error {
			called = true
			if len(sections) != 1 {
				t.Fatalf("sections = %d, want 1", len(sections))
			}
			if sections[0].Root != "bgp" {
				t.Fatalf("root = %q, want bgp", sections[0].Root)
			}
			var payload map[string]any
			if err := json.Unmarshal([]byte(sections[0].Data), &payload); err != nil {
				t.Fatalf("unmarshal section: %v", err)
			}
			if _, ok := payload["bgp"]; !ok {
				t.Fatalf("section payload missing wrapped bgp root: %s", sections[0].Data)
			}
			return fmt.Errorf("rejected by verifier")
		},
	}); err != nil {
		t.Fatalf("register verifier: %v", err)
	}

	tree := mustParsePluginVerifyConfig(t, `bgp { router-id 1.2.3.4; }`)
	errs := VerifyPluginConfig(tree)
	if !called {
		t.Fatal("verifier was not called")
	}
	if len(errs) != 1 {
		t.Fatalf("errors = %d, want 1", len(errs))
	}
	if !strings.Contains(errs[0].Error(), "verify-test: rejected by verifier") {
		t.Fatalf("error = %q", errs[0].Error())
	}
}

func TestVerifyPluginConfigSkipsAbsentRoots(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	called := false
	if err := registry.Register(registry.Registration{
		Name:        "verify-test",
		Description: "test verifier",
		ConfigRoots: []string{"interface"},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		InProcessConfigVerifier: func([]rpc.ConfigSection) error {
			called = true
			return nil
		},
	}); err != nil {
		t.Fatalf("register verifier: %v", err)
	}

	tree := mustParsePluginVerifyConfig(t, `bgp { router-id 1.2.3.4; }`)
	if errs := VerifyPluginConfig(tree); len(errs) != 0 {
		t.Fatalf("errors = %v, want none", errs)
	}
	if called {
		t.Fatal("verifier called for absent root")
	}
}

func TestVerifyPluginConfigTransitionSendsEmptyDeletedRoot(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	called := false
	if err := registry.Register(registry.Registration{
		Name:        "verify-test",
		Description: "test verifier",
		ConfigRoots: []string{"interface"},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		InProcessConfigVerifier: func(sections []rpc.ConfigSection) error {
			called = true
			if len(sections) != 1 {
				t.Fatalf("sections = %d, want 1", len(sections))
			}
			if sections[0].Root != "interface" {
				t.Fatalf("root = %q, want interface", sections[0].Root)
			}
			if sections[0].Data != "{}" {
				t.Fatalf("data = %q, want {}", sections[0].Data)
			}
			return nil
		},
	}); err != nil {
		t.Fatalf("register verifier: %v", err)
	}

	previous := map[string]any{"interface": map[string]any{"backend": "netlink"}}
	candidate := map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}
	if errs := VerifyPluginConfigMapTransition(previous, candidate); len(errs) != 0 {
		t.Fatalf("errors = %v, want none", errs)
	}
	if !called {
		t.Fatal("verifier was not called for deleted root")
	}
}

func TestVerifyPluginConfigContentTransitionIgnoresInvalidPrevious(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	requireNoRegisterError(t, registry.Register(registry.Registration{
		Name:        "verify-test",
		Description: "test verifier",
		ConfigRoots: []string{"bgp"},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		InProcessConfigVerifier: func([]rpc.ConfigSection) error {
			return nil
		},
	}))

	err := VerifyPluginConfigContentTransition(`not valid config`, `bgp { router-id 1.2.3.4; }`)
	if err != nil {
		t.Fatalf("VerifyPluginConfigContentTransition returned error: %v", err)
	}
}

func mustParsePluginVerifyConfig(t *testing.T, input string) *Tree {
	t.Helper()
	schema, err := YANGSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	tree, err := NewParser(schema).Parse(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return tree
}

func requireNoRegisterError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("register verifier: %v", err)
	}
}
