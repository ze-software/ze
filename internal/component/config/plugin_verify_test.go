package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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

// TestVerifyPluginConfigSendsReadRoots proves a root the plugin only reads
// (Registration.ConfigReads) reaches its verifier beside the root it owns.
// PREVENTS: bgp-rpki validating an RTR TLS certificate name without the pki
// store that defines it, once `pki` left its ConfigRoots.
func TestVerifyPluginConfigSendsReadRoots(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	var roots []string
	requireNoRegisterError(t, registry.Register(registry.Registration{
		Name:        "verify-reads-test",
		Description: "test verifier",
		ConfigRoots: []string{"bgp"},
		ConfigReads: []string{"pki"},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		InProcessConfigVerifier: func(sections []rpc.ConfigSection) error {
			for _, section := range sections {
				roots = append(roots, section.Root)
			}
			return nil
		},
	}))

	errs := VerifyPluginConfigMap(map[string]any{
		"bgp": map[string]any{"router-id": "1.2.3.4"},
		"pki": map[string]any{"ca": map[string]any{}},
	})
	if len(errs) != 0 {
		t.Fatalf("errors = %v, want none", errs)
	}
	if strings.Join(roots, ",") != "bgp,pki" {
		t.Fatalf("verifier roots = %v, want [bgp pki]", roots)
	}
}

// TestVerifyPluginConfigRefusesAbsentProvider proves AC-4 at the verify door
// that ze config validate, editor pre-commit and API commit pass through: the
// same refusal LoadConfig runs is among the verify errors, and the config with
// the leaf at another value verifies clean.
//
// VALIDATES: VerifyPluginConfigMapTransition calls refuseAbsentOptionalProviders.
// PREVENTS: validate or commit accepting what boot refuses.
func TestVerifyPluginConfigRefusesAbsentProvider(t *testing.T) {
	isolateRegistry(t)
	registerNeedOwner(t, []string{"system", "dns", "dnssec-validation"}, "permissive", "bgp-rib")

	errs := VerifyPluginConfig(mustParsePluginVerifyConfig(t, "system { dns { dnssec-validation permissive; } }\n"))
	if len(errs) != 1 {
		t.Fatalf("errors = %v, want exactly the absent-provider refusal", errs)
	}
	if !errors.Is(errs[0], ErrAbsentOptionalProvider) {
		t.Fatalf("error = %v, want ErrAbsentOptionalProvider", errs[0])
	}
	if !strings.Contains(errs[0].Error(), "\n  fix: rebuild ze with build tag ze_bgp") {
		t.Fatalf("error does not carry the fix line naming ze_bgp:\n%s", errs[0])
	}

	if errs := VerifyPluginConfig(mustParsePluginVerifyConfig(t, "system { dns { dnssec-validation off; } }\n")); len(errs) != 0 {
		t.Fatalf("the need does not hold at another value: errors = %v, want none", errs)
	}
}

// registerRootOwner registers a test plugin that owns root and hard-depends on
// dependency, the shape of the exabgp bridge (root exabgp, dependency bgp).
func registerRootOwner(t *testing.T, name, root, dependency string) {
	t.Helper()
	err := registry.Register(registry.Registration{
		Name:         name,
		Description:  "test root owner",
		ConfigRoots:  []string{root},
		Dependencies: []string{dependency},
		RunEngine:    func(net.Conn) int { return 0 },
		CLIHandler:   func([]string) int { return 0 },
	})
	if err != nil {
		t.Fatalf("register %s: %v", name, err)
	}
}

// TestVerifyPluginConfigRefusesMissingHardDependency proves the validate and
// commit doors refuse what boot refuses when a plugin the config's roots select
// hard-depends on a plugin this build lacks. Method: an isolated registry holds
// a plugin owning `system` that depends on the gated `bgp`, unregistered here,
// as in a ze_core ze_exabgp build. The door's error must be the
// *registry.MissingDependencyError boot's resolution answers for the same set,
// word for word, naming ze_bgp. An `external` block for the owner takes it out
// of the set (boot does not auto-load a configured plugin), and registering
// bgp makes the same config valid.
func TestVerifyPluginConfigRefusesMissingHardDependency(t *testing.T) {
	isolateRegistry(t)
	registerRootOwner(t, "root-owner", "system", "bgp")
	const config = "system { dns { dnssec-validation off; } }\n"

	errs := VerifyPluginConfig(mustParsePluginVerifyConfig(t, config))
	if len(errs) != 1 {
		t.Fatalf("errors = %v, want exactly the missing-dependency refusal", errs)
	}
	missing, ok := errors.AsType[*registry.MissingDependencyError](errs[0])
	if !ok {
		t.Fatalf("error = %v, want *registry.MissingDependencyError", errs[0])
	}
	if missing.Plugin != "root-owner" || missing.Dependency != "bgp" {
		t.Errorf("refusal names plugin %q dependency %q, want root-owner and bgp", missing.Plugin, missing.Dependency)
	}
	_, boot := registry.ResolveDependencies([]string{"root-owner"}, nil, func(string, registry.ConfigNeed) bool { return false })
	if boot == nil || errs[0].Error() != boot.Error() {
		t.Errorf("door message differs from boot's:\ndoor: %v\nboot: %v", errs[0], boot)
	}
	if !strings.Contains(errs[0].Error(), "ze_bgp") {
		t.Errorf("refusal does not name ze_bgp:\n%s", errs[0])
	}

	if errs := VerifyPluginConfigTransition(nil, mustParsePluginVerifyConfig(t, config)); len(errs) != 1 {
		t.Errorf("transition door errors = %v, want the same refusal", errs)
	}

	external := config + "plugin { external root-owner { run \"/bin/true\"; } }\n"
	if errs := VerifyPluginConfig(mustParsePluginVerifyConfig(t, external)); len(errs) != 0 {
		t.Errorf("an external block runs its own program, so boot does not resolve the registration: errors = %v, want none", errs)
	}

	registerProvider(t, "bgp")
	if errs := VerifyPluginConfig(mustParsePluginVerifyConfig(t, config)); len(errs) != 0 {
		t.Errorf("with bgp in the build the config is valid: errors = %v, want none", errs)
	}
}
