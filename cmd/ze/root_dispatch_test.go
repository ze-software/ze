//go:build ze_core

package main

import (
	"testing"

	"github.com/ze-software/ze/internal/component/aihelp"
	"github.com/ze-software/ze/internal/component/command/registry"
)

// These tests register sentinel root names rather than calling
// registry.ResetForTest(): the cmd/ze test binary shares one process-wide
// registry populated by every imported package's init(), and a reset would
// wipe the real roots other tests (e.g. setup_features_stripped_test.go) depend
// on. Sentinel names cannot collide with real roots.

// TestRootDispatchUsesRegisteredOwnerHandler proves cmd/ze dispatches an
// owner-backed root through the registry handler, with arguments passed in the
// same order, and that an unregistered root falls through to the legacy switch.
func TestRootDispatchUsesRegisteredOwnerHandler(t *testing.T) {
	const name = "ztest-owner-root"

	called := false
	var gotArgs []string
	registry.MustRegisterRootHandler(name, func(_ *registry.RuntimeContext, args []string) int {
		called = true
		gotArgs = append([]string(nil), args...)
		return 7
	}, registry.Meta{Description: "sentinel owner root", Mode: "offline"})

	code, handled := dispatchRegisteredRoot(name, &registry.RuntimeContext{}, []string{"sub", "x", "y"})
	if !handled {
		t.Fatal("registry should have handled the owner-backed root")
	}
	if !called {
		t.Fatal("owner handler was not invoked")
	}
	if code != 7 {
		t.Errorf("exit code = %d, want 7", code)
	}
	want := []string{"sub", "x", "y"}
	if len(gotArgs) != len(want) {
		t.Fatalf("handler received %d args, want %d (%v)", len(gotArgs), len(want), gotArgs)
	}
	for i := range want {
		if gotArgs[i] != want[i] {
			t.Errorf("arg %d = %q, want %q", i, gotArgs[i], want[i])
		}
	}

	// An unregistered root must NOT be handled by the registry; control falls
	// through to the legacy static switch in main().
	if _, handled := dispatchRegisteredRoot("ztest-definitely-unregistered", &registry.RuntimeContext{}, nil); handled {
		t.Error("unregistered root must not be handled by the registry")
	}
}

// TestRootDispatchPassesRuntimeContext proves main() builds an explicit runtime
// context (storage resolver, plugin list, version printer, config override,
// web/MCP flags) and hands that exact context to the owner handler. No
// dependency is read or opened during context construction.
func TestRootDispatchPassesRuntimeContext(t *testing.T) {
	zeFlags = zeGlobalFlags{
		plugins:      []string{"p1", "p2"},
		fileOverride: "/tmp/override.conf",
		webPort:      "8443",
		insecureWeb:  true,
		mcpAddr:      "127.0.0.1:9000",
		mcpToken:     "tok",
		chaosSeed:    42,
		chaosRate:    0.25,
	}
	rctx := newZeRuntimeContext()

	if rctx.ResolveStorage == nil {
		t.Error("ResolveStorage not wired")
	}
	if rctx.PrintVersion == nil {
		t.Error("PrintVersion not wired")
	}
	if len(rctx.Plugins) != 2 || rctx.Plugins[0] != "p1" || rctx.Plugins[1] != "p2" {
		t.Errorf("Plugins = %v, want [p1 p2]", rctx.Plugins)
	}
	if rctx.ConfigOverride != "/tmp/override.conf" {
		t.Errorf("ConfigOverride = %q, want /tmp/override.conf", rctx.ConfigOverride)
	}
	if rctx.WebPort != "8443" || !rctx.InsecureWeb {
		t.Errorf("web flags = (%q, %v), want (8443, true)", rctx.WebPort, rctx.InsecureWeb)
	}
	if rctx.MCPAddr != "127.0.0.1:9000" || rctx.MCPToken != "tok" {
		t.Errorf("mcp flags = (%q, %q), want (127.0.0.1:9000, tok)", rctx.MCPAddr, rctx.MCPToken)
	}
	if rctx.ChaosSeed != 42 {
		t.Errorf("ChaosSeed = %d, want 42", rctx.ChaosSeed)
	}
	if rctx.ChaosRate != 0.25 {
		t.Errorf("ChaosRate = %f, want 0.25", rctx.ChaosRate)
	}

	// The handler receives the identical context instance built by main().
	const name = "ztest-ctx-root"
	var seen *registry.RuntimeContext
	registry.MustRegisterRootHandler(name, func(c *registry.RuntimeContext, _ []string) int {
		seen = c
		return 0
	}, registry.Meta{})

	dispatchRegisteredRoot(name, rctx, nil)
	if seen != rctx {
		t.Error("owner handler did not receive the runtime context built by main()")
	}
}

// TestHelpAIUsesOwnerRegistry proves the CLI help/AI inventory is derived from
// the same registry, so an owner-backed root registered with a handler appears
// in `ze help --ai` exactly like a metadata-only root.
func TestHelpAIUsesOwnerRegistry(t *testing.T) {
	const name = "ztest-help-root"
	const desc = "sentinel owner root for help inventory"

	registry.MustRegisterRootHandler(name, func(_ *registry.RuntimeContext, _ []string) int {
		return 0
	}, registry.Meta{Description: desc, Mode: "offline", Subs: "alpha, beta"})

	found := false
	for _, c := range aihelp.CLISubcommands() {
		if c.Name == name {
			found = true
			if c.Description != desc {
				t.Errorf("help desc = %q, want %q", c.Description, desc)
			}
			if c.Subs != "alpha, beta" {
				t.Errorf("help subs = %q, want %q", c.Subs, "alpha, beta")
			}
			break
		}
	}
	if !found {
		t.Errorf("owner-backed root %q absent from aihelp.CLISubcommands(); help is not registry-derived", name)
	}
}
