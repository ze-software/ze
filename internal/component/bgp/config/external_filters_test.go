package bgpconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// declareRuntime installs a runtime declaration seam answering decls for
// pluginName, as the plugin server does once the plugin passed Stage 1, and
// clears the pending references the test leaves.
func declareRuntime(t *testing.T, pluginName string, filters []rpc.FilterDecl) {
	t.Helper()
	declared, err := registry.CompileFilterDeclarations(pluginName, filters)
	if err != nil {
		t.Fatalf("compile declarations: %v", err)
	}
	registry.SetRuntimeFilterDeclarations(func(process string) ([]registry.DeclaredFilter, bool) {
		if process != pluginName {
			return nil, false
		}
		return declared, true
	})
	t.Cleanup(func() { registry.SetRuntimeFilterDeclarations(nil) })
}

func clearPending(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		pendingExternal.mu.Lock()
		pendingExternal.refs = nil
		pendingExternal.mu.Unlock()
	})
}

func emptyPolicyRegistry(t *testing.T) *policy.Registry {
	t.Helper()
	reg, err := policy.NewRegistry()
	if err != nil {
		t.Fatalf("policy registry: %v", err)
	}
	return reg
}

func requireRefusal(t *testing.T, err error, want ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want a refusal naming %q, got none", want)
	}
	for _, w := range want {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("refusal %q does not name %q", err, w)
		}
	}
}

// TestCheckChainExternalDeclaration proves AC-5 for an external filter: a
// chain at bgp export naming a filter whose Stage 1 declaration reads a field
// the attachment does not carry, or sets one it does not allow, fails config
// load naming the filter, the field and the attachment. The same judgement
// runs whether the plugin declared before the load (a reload) or after it
// (startup, through the Stage 1 judge).
func TestCheckChainExternalDeclaration(t *testing.T) {
	clearPending(t)
	reg := emptyPolicyRegistry(t)
	const ctx = "peer 192.0.2.1 filter export"
	filters := []rpc.FilterDecl{
		{Name: "reads-tag", Direction: rpc.FilterExport, Reads: []string{"prefix", "tag"}},
		{Name: "sets-prefix", Direction: rpc.FilterExport, Reads: []string{"prefix"}, Sets: []string{"prefix"}},
		{Name: "fits", Direction: rpc.FilterExport, Reads: []string{"prefix", rpc.FilterFieldRaw}, Sets: []string{"local-preference"}},
	}

	t.Run("declared before load", func(t *testing.T) {
		declareRuntime(t, "ext", filters)
		err := validateChainNames(reg, []policy.Ref{{Name: "ext:reads-tag"}}, "bgp export", ctx)
		requireRefusal(t, err, ctx, "ext:reads-tag", "tag", "bgp export")
		err = validateChainNames(reg, []policy.Ref{{Name: "ext:sets-prefix"}}, "bgp export", ctx)
		requireRefusal(t, err, "ext:sets-prefix", "sets prefix", "bgp export")
		if err := validateChainNames(reg, []policy.Ref{{Name: "ext:fits"}}, "bgp export", ctx); err != nil {
			t.Fatalf("a filter whose declaration fits bgp export was refused: %v", err)
		}
	})

	t.Run("declared after load", func(t *testing.T) {
		if err := validateChainNames(reg, []policy.Ref{{Name: "late:reads-tag"}}, "bgp export", ctx); err != nil {
			t.Fatalf("a reference to a plugin that has not declared must wait for Stage 1, got %v", err)
		}
		declared, err := registry.CompileFilterDeclarations("late", filters)
		if err != nil {
			t.Fatal(err)
		}
		err = checkFilterDeclarations("late", declared)
		requireRefusal(t, err, ctx, "late:reads-tag", "tag", "bgp export")
		// The refusal judges the loaded config, so it fails daemon startup
		// rather than the plugin alone (plugin/server refuseStartupOnLoadedChain).
		if !errors.Is(err, registry.ErrLoadedChainRefused) {
			t.Fatalf("a refused loaded chain must be marked registry.ErrLoadedChainRefused, got %v", err)
		}
	})
}

// TestCheckChainExternalNamePattern proves AC-21: a reference matching a
// declared name pattern is judged against that pattern's declared fields like
// a named instance, and a reference matching no declared name or pattern fails
// naming the reference and every name and pattern the plugin declares.
func TestCheckChainExternalNamePattern(t *testing.T) {
	clearPending(t)
	reg := emptyPolicyRegistry(t)
	const ctx = "peer 192.0.2.1 filter import"
	declareRuntime(t, "irr", []rpc.FilterDecl{
		{NamePattern: "[0-9]+", Direction: rpc.FilterImport, Reads: []string{"prefix", "tag"}},
		{Name: "status", Direction: rpc.FilterImport, Reads: []string{"prefix"}},
	})

	err := validateChainNames(reg, []policy.Ref{{Name: "irr:65001"}}, "bgp import", ctx)
	requireRefusal(t, err, "irr:65001", "tag", "bgp import")

	err = validateChainNames(reg, []policy.Ref{{Name: "irr:as-set"}}, "bgp import", ctx)
	requireRefusal(t, err, ctx, "irr:as-set", `plugin "irr"`, "pattern [0-9]+", "status")

	if err := validateChainNames(reg, []policy.Ref{{Name: "irr:status"}}, "bgp import", ctx); err != nil {
		t.Fatalf("an exactly declared name that fits was refused: %v", err)
	}
}

// TestFilterDeclarationVocabulary proves the Stage 1 judge refuses a read or
// set the policy vocabulary does not hold, and raw declared as a set.
func TestFilterDeclarationVocabulary(t *testing.T) {
	clearPending(t)
	for _, tc := range []struct {
		name   string
		filter rpc.FilterDecl
		want   string
	}{
		{"unknown read", rpc.FilterDecl{Name: "f", Reads: []string{"communities"}}, `"communities"`},
		{"unknown set", rpc.FilterDecl{Name: "f", Sets: []string{"localpref"}}, `"localpref"`},
		{"raw set", rpc.FilterDecl{Name: "f", Sets: []string{rpc.FilterFieldRaw}}, "raw is read only"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			declared, err := registry.CompileFilterDeclarations("p", []rpc.FilterDecl{tc.filter})
			if err != nil {
				t.Fatal(err)
			}
			err = checkFilterDeclarations("p", declared)
			requireRefusal(t, err, `plugin "p"`, tc.want)
			// A refused declaration holds no loaded chain: the plugin's failure
			// policy decides, so it is not marked as a refused config.
			if errors.Is(err, registry.ErrLoadedChainRefused) {
				t.Fatalf("a refused declaration must not be marked as a refused loaded chain: %v", err)
			}
		})
	}
}
