// Design: docs/architecture/cli/command-namespacing.md -- CLI command grammar gate
//
// Feeder 3 of the CLI grammar gate (ai/rules/cli.md, "Mechanical
// Enforcement"). Feeders 1 (static YANG tree, internal/le/cligrammar/register.go) and
// 2 (plugin registration, validateCommandName) already enforce grammar on 100% of
// This in-process test locks that coverage against regression from the runtime
// side, WITHOUT booting a daemon or maintaining an all-plugins config.
//
// TestRuntimeBuiltinSurfaceGrammar walks the exact inputs the server feeds to
// loadBuiltinsWithAliases (server.go:257) -- every registered builtin RPC mapped
// through yang.WireMethodToPaths -- and re-runs grammar.CheckName on each path,
// skipping category-exempt handlers via grammar.ExemptCategory(WireMethod). This
// is the one check the live `system command list` RPC could not do: the RPC
// payload strips the wire method (command_registry.go:115-120), so exemption by
// wire-method namespace is only possible in-process.
//
// Why not a daemon-boot audit: builtins are 100% YANG-derived (command.go:53-98
// skips any handler with no YANG path) so they are a strict subset of Feeder 1's
// tree, and plugin commands are rejected at registration by Feeder 2. The merged
// runtime surface can therefore contain only conforming commands by construction;
// a boot-and-dump audit adds no catch value.

package server_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/grammar"
	"github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	// Trigger every builtin RPC init() registration, matching the composition
	// root the running daemon assembles (mirrors all_import_test.go).
	_ "github.com/ze-software/ze/internal/component/plugin/all"
)

// TestRuntimeBuiltinSurfaceGrammar validates that every builtin command the daemon
// actually assembles into its dispatcher is verb-first grammar-clean.
//
// VALIDATES: Feeder 3 -- the runtime builtin assembly source (AllBuiltinRPCs x
//
//	WireMethodToPaths) has zero grammar violations, with category-exempt handlers
//	skipped by wire method (the check the RPC surface cannot perform).
//
// PREVENTS: a builtin RPC whose YANG-derived CLI path (or an alias path) drifts from
//
//	the grammar reaching the dispatcher unchecked; the exemption set silently
//	swallowing the whole surface.
func TestRuntimeBuiltinSurfaceGrammar(t *testing.T) {
	loader, err := yang.DefaultLoader()
	require.NoError(t, err, "load YANG")
	wireToPaths := yang.WireMethodToPaths(loader)

	var findings []grammar.Finding
	checked := 0
	exempt := map[string]int{}

	for _, reg := range pluginserver.AllBuiltinRPCs() {
		// Category-exempt handlers (bridge / wire-protocol / editor) are keyed on
		// the wire-method namespace -- available here, unavailable over the RPC.
		if cat, ok := grammar.ExemptCategory(reg.WireMethod); ok {
			exempt[cat]++
			continue
		}
		// Handlers with no YANG path are not CLI-dispatchable; loadBuiltinsWithAliases
		// skips them (command.go:80-84), so they carry no CLI grammar. Skip likewise.
		for _, path := range wireToPaths[reg.WireMethod] {
			checked++
			findings = append(findings, grammar.CheckName(path)...)
		}
	}

	for _, f := range findings {
		t.Errorf("[%s] %q: %s", f.Rule, f.Command, f.Message)
	}

	// Non-vacuous: we must have actually grammar-checked a large surface, not
	// skipped everything. The parent gate reports ~255 built-in nodes; the ~290
	// runtime figure includes aliases. A floor well below either catches a
	// wiring break (empty AllBuiltinRPCs / empty WireMethodToPaths) without being
	// brittle to the exact count.
	if checked < 100 {
		t.Fatalf("audited too few builtin command paths (%d); builtin registration or YANG map is broken", checked)
	}
	// The exemption branch must be exercised on real data: the fixed bridge surface
	// (announce/withdraw/peer-raw/peer-update/help) is intentionally not verb-first
	// and must be skipped, never flagged. At least announce+withdraw are always present.
	assert.GreaterOrEqual(t, exempt["bridge"], 2,
		"bridge-exempt builtins not found; exemption path not exercised")

	t.Logf("audited %d builtin command paths; exempt: %v", checked, exempt)
}
