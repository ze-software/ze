// Design: docs/architecture/api/architecture.md -- unit tests for ImpliedReport, round-trippable renderers, offline external refusal, plain-display notices, and graph edge origins.
// Related: implied.go -- ImpliedReport data types, provider seams, and BuildImpliedReport.
// Related: implied_render.go -- RenderImpliedText, RenderImpliedSet, and notice helpers.
// Related: graph.go -- BuildGraph and origin-annotated GraphEdge.

package config

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestImpliedEnvelopeKeepsConfigPure verifies AC-23: marshaling an
// ImpliedEnvelope to JSON and YAML leaves the written configuration under
// `config` byte-for-byte identical to standalone marshaling while placing
// endpoints, participants, and filters under `implied` with kebab-case keys.
//
// VALIDATES: AC-23 JSON and YAML envelope purity and kebab-case schema keys.
// PREVENTS: Implied items leaking into the written config payload or breaking
// kebab-case JSON/YAML keys.
func TestImpliedEnvelopeKeepsConfigPure(t *testing.T) {
	written := map[string]any{
		"bus": map[string]any{
			"endpoint": map[string]any{
				"collector": map[string]any{
					"internal": "test-implied-collector",
				},
			},
		},
	}

	report := ImpliedReport{
		Endpoints: []ImpliedEndpoint{{
			Name:           "auto-ep",
			CompiledPlugin: "test-implied-collector",
			ReasonKind:     "config-root",
			ReasonSource:   "/bus",
			ImpliedBy:      "config-root /bus",
		}},
		Participants: []ImpliedParticipant{{
			Path:               "/bus/endpoint/collector",
			EndpointName:       "collector",
			SendAllow:          []string{"testimplied:state"},
			SendAllowOrigin:    ImpliedOriginRegistered,
			SendAllowSource:    "registration of collector",
			ReceiveAllow:       []string{"testimplied:update"},
			ReceiveAllowOrigin: ImpliedOriginRegistered,
			ReceiveAllowSource: "registration of collector",
			Receive: []ImpliedDirectionRule{{
				Message:       "testimplied:update",
				Allowed:       true,
				Origin:        ImpliedOriginRegistered,
				Source:        "registration of collector",
				Targets:       []string{"/bus/endpoint/emitter"},
				Tags:          map[string][]string{"direction": {"received"}},
				Dormant:       true,
				DormantReason: "no sender loaded",
			}},
			Send: []ImpliedDirectionRule{{
				Message:          "testimplied:state",
				Allowed:          true,
				Origin:           ImpliedOriginWritten,
				Source:           "bus > endpoint collector > send > to",
				Targets:          []string{"/bus/endpoint/sink"},
				ReplacedDefaults: []string{"/bus/endpoint/default-sink"},
			}},
		}},
		Filters: []ImpliedFilterMember{{
			TargetPath: "/acme/node/edge1",
			Direction:  "import",
			Position:   "head",
			Ref:        "acme:loop-check",
			ImpliedBy:  "built-in loop-check step",
			Required:   true,
			Reason:     "mandatory safety check",
		}},
	}

	env := ImpliedEnvelope{
		Config:  written,
		Implied: report,
	}

	// JSON check: `config` must equal standalone json.Marshal(written) byte-for-byte.
	wantConfigJSON, err := json.Marshal(written)
	require.NoError(t, err)

	envJSON, err := json.Marshal(env)
	require.NoError(t, err)

	var rawEnv map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(envJSON, &rawEnv))
	require.Contains(t, rawEnv, "config")
	require.Contains(t, rawEnv, "implied")
	assert.Equal(t, wantConfigJSON, []byte(rawEnv["config"]), "envelope config JSON must stay byte-pure")

	var impliedJSON map[string]any
	require.NoError(t, json.Unmarshal(rawEnv["implied"], &impliedJSON))
	require.Contains(t, impliedJSON, "endpoints")
	require.Contains(t, impliedJSON, "participants")
	require.Contains(t, impliedJSON, "filters")
	assert.Contains(t, string(rawEnv["implied"]), `"compiled-plugin"`)
	assert.Contains(t, string(rawEnv["implied"]), `"reason-kind"`)
	assert.Contains(t, string(rawEnv["implied"]), `"implied-by"`)
	assert.Contains(t, string(rawEnv["implied"]), `"replaced-defaults"`)
	assert.Contains(t, string(rawEnv["implied"]), `"dormant-reason"`)
	assert.Contains(t, string(rawEnv["implied"]), `"target-path"`)

	// YAML check: `config` must equal standalone yaml.Marshal(written) byte-for-byte.
	wantConfigYAML, err := yaml.Marshal(written)
	require.NoError(t, err)

	envYAML, err := yaml.Marshal(env)
	require.NoError(t, err)

	var rawYAML map[string]yaml.Node
	require.NoError(t, yaml.Unmarshal(envYAML, &rawYAML))
	cfgNode, hasCfg := rawYAML["config"]
	require.True(t, hasCfg)
	gotConfigYAML, err := yaml.Marshal(&cfgNode)
	require.NoError(t, err)
	assert.Equal(t, wantConfigYAML, gotConfigYAML, "envelope config YAML must stay byte-pure")

	impliedNode, hasImplied := rawYAML["implied"]
	require.True(t, hasImplied)
	gotImpliedYAML, err := yaml.Marshal(&impliedNode)
	require.NoError(t, err)
	impliedYAMLText := string(gotImpliedYAML)
	assert.Contains(t, impliedYAMLText, "endpoints:")
	assert.Contains(t, impliedYAMLText, "participants:")
	assert.Contains(t, impliedYAMLText, "filters:")
	assert.Contains(t, impliedYAMLText, "compiled-plugin:")
	assert.Contains(t, impliedYAMLText, "replaced-defaults:")
	assert.Contains(t, impliedYAMLText, "target-path:")
}

// TestImpliedTextRoundTrips verifies AC-38 and S30: rendered implied text uses
// single-hash comments (never `##` at column 0), detects as FormatHierarchical
// (never FormatSetMeta), parses cleanly through Parser.Parse without clobbering
// written endpoints, round-trips in set format through SetParser.Parse, and
// emits no implied copy once the rules are written.
//
// VALIDATES: AC-38, S30 round-trip parsing of rendered `| implied` text and set
// output, absence of `##` markers, and suppression of implied copies for
// written rules.
// PREVENTS: `##` breaking DetectFormat into FormatSetMeta, invalid generated
// bus syntax, or duplicate implied blocks after pasting and committing.
func TestImpliedTextRoundTrips(t *testing.T) {
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })

	require.NoError(t, events.RegisterMessage("testimplied", "update",
		events.TagDef{Name: "direction", Values: []string{"received", "sent"}},
	))
	require.NoError(t, events.RegisterMessage("testimplied", "state"))

	noopEngine := func(net.Conn) int { return 0 }
	noopCLI := func([]string) int { return 0 }
	require.NoError(t, registry.Register(registry.Registration{
		Name:        "test-implied-collector",
		Description: "test collector for implied round-trip",
		RunEngine:   noopEngine,
		CLIHandler:  noopCLI,
		Sends:       []rpc.MessageSendDecl{{Message: "testimplied:state"}},
		Receives: []rpc.MessageReceiveDecl{{
			Message: "testimplied:update",
			Tags:    []rpc.MessageTagDecl{{Name: "direction", Values: []string{"received"}}},
		}},
	}))

	schema, err := YANGSchema()
	require.NoError(t, err)

	written := `bus {
    endpoint collector {
        internal test-implied-collector;
    }
}
`
	report := ImpliedReport{
		Endpoints: []ImpliedEndpoint{{
			Name:           "auto-collector",
			CompiledPlugin: "test-implied-collector",
			ReasonKind:     "config-root",
			ReasonSource:   "/bus",
			ImpliedBy:      "config-root /bus",
		}},
		Participants: []ImpliedParticipant{{
			Path:               "/bus/endpoint/collector",
			EndpointName:       "collector",
			ReceiveAllow:       []string{"testimplied:update"},
			ReceiveAllowOrigin: ImpliedOriginRegistered,
			ReceiveAllowSource: "registration of collector",
			Receive: []ImpliedDirectionRule{{
				Message: "testimplied:update",
				Allowed: true,
				Origin:  ImpliedOriginRegistered,
				Source:  "registration of collector",
				Targets: []string{"/bus/endpoint/auto-collector"},
				Tags:    map[string][]string{"direction": {"received"}},
			}},
			Send: []ImpliedDirectionRule{
				{
					Message:          "testimplied:state",
					Allowed:          true,
					Origin:           ImpliedOriginWritten,
					Source:           "bus > endpoint collector > send > to",
					Targets:          []string{"/bus/endpoint/auto-collector"},
					ReplacedDefaults: []string{"/bus/endpoint/default-sink"},
				},
				{
					Message:       "testimplied:update",
					Allowed:       true,
					Origin:        ImpliedOriginDormant,
					Dormant:       true,
					DormantReason: "no matching receiver loaded",
				},
			},
		}},
		Filters: []ImpliedFilterMember{{
			TargetPath: "/acme/node/edge1",
			Direction:  "import",
			Position:   "head",
			Ref:        "acme:loop-check",
			ImpliedBy:  "built-in loop-check step",
			Required:   true,
			Reason:     "mandatory safety check",
		}},
	}

	rendered := RenderImpliedText(written, report)

	assert.Equal(t, FormatHierarchical, DetectFormat(rendered), "rendered implied text must detect as hierarchical, never FormatSetMeta")
	for line := range strings.SplitSeq(rendered, "\n") {
		trimmed := strings.TrimSpace(line)
		assert.False(t, strings.HasPrefix(line, "##"), "line must never start with ## at column 0: %q", line)
		if len(trimmed) > 1 && trimmed[0] == '#' {
			assert.Equal(t, byte(' '), trimmed[1], "every comment line must use single '#' followed by space: %q", line)
		}
	}

	assert.Contains(t, rendered, "# implied by: registration of collector")
	assert.Contains(t, rendered, "# implied: replaced registered default(s) [ /bus/endpoint/default-sink ]")
	assert.Contains(t, rendered, "# implied: dormant (no matching receiver loaded)")
	assert.Contains(t, rendered, "# implied: import head acme:loop-check (built-in loop-check step, required: mandatory safety check)")

	parsed, parseErr := NewParser(schema).Parse(rendered)
	require.NoError(t, parseErr, "rendered implied text must parse cleanly:\n%s", rendered)
	busTree := parsed.GetContainer("bus")
	require.NotNil(t, busTree)
	epCollector := busTree.GetList("endpoint")["collector"]
	require.NotNil(t, epCollector, "written endpoint collector must remain intact after parsing")
	internalVal, ok := epCollector.Get("internal")
	require.True(t, ok)
	assert.Equal(t, "test-implied-collector", internalVal)
	epAuto := busTree.GetList("endpoint")["auto-collector"]
	require.NotNil(t, epAuto, "implied endpoint block must parse cleanly")
	pCollector := busTree.GetList("participant")["/bus/endpoint/collector"]
	require.NotNil(t, pCollector, "implied participant block must parse cleanly in short path form")
	require.NotNil(t, pCollector.GetContainer("receive"))

	// Set-command format must detect as FormatSet and parse cleanly via SetParser.
	writtenSet := "set bus endpoint collector internal test-implied-collector\n"
	renderedSet := RenderImpliedSet(writtenSet, report)
	assert.Equal(t, FormatSet, DetectFormat(renderedSet), "rendered implied set output must detect as FormatSet")
	for line := range strings.SplitSeq(renderedSet, "\n") {
		assert.False(t, strings.HasPrefix(line, "##"), "set line must never start with ##: %q", line)
	}
	parsedSet, setErr := NewSetParser(schema).Parse(renderedSet)
	require.NoError(t, setErr, "rendered implied set output must parse cleanly:\n%s", renderedSet)
	require.NotNil(t, parsedSet.GetContainer("bus"))

	// S30 second print: once rules are written with no replaced defaults or dormancy,
	// no implied copy is printed.
	writtenOnlyReport := ImpliedReport{
		Participants: []ImpliedParticipant{{
			Path:               "/bus/endpoint/collector",
			EndpointName:       "collector",
			ReceiveAllow:       []string{"testimplied:update"},
			ReceiveAllowOrigin: ImpliedOriginWritten,
			Receive: []ImpliedDirectionRule{{
				Message: "testimplied:update",
				Allowed: true,
				Origin:  ImpliedOriginWritten,
				Targets: []string{"/bus/endpoint/auto-collector"},
			}},
		}},
	}
	assert.Equal(t, written, RenderImpliedText(written, writtenOnlyReport), "written-only report must emit no implied copy")
}

// TestOfflineImpliedRefusesUnknownDeclaration verifies AC-24: when running
// offline (opts.Offline == true) against a config holding an external endpoint
// whose program cannot be queried, BuildImpliedReport refuses with an
// *opererr.Error naming the endpoint, its run line, the underlying cause, and
// the exact conformance fix command, and returns an empty ImpliedReport (never
// a partial set). When a program answers in declare mode, it succeeds without
// running its main loop.
//
// VALIDATES: AC-24 offline refusal on unqueryable external endpoint and
// declare-mode query success without starting the run loop.
// PREVENTS: Offline `| implied` silently omitting an external endpoint and
// presenting a partial implied set as complete.
func TestOfflineImpliedRefusesUnknownDeclaration(t *testing.T) {
	t.Run("missing_program_refuses_without_partial_report", func(t *testing.T) {
		restoreDeclared := ResetDeclaredSchemasForTest()
		t.Cleanup(restoreDeclared)
		restoreProviders := ResetImpliedProvidersForTest()
		t.Cleanup(restoreProviders)

		RegisterImpliedBusProvider(func(_ *Tree, _ ImpliedOptions) ([]ImpliedEndpoint, []ImpliedParticipant, error) {
			return []ImpliedEndpoint{{
				Name:       "should-not-appear",
				ReasonKind: "config-root",
				ImpliedBy:  "partial leak",
			}}, nil, nil
		})
		RegisterImpliedFilterProvider("fixture", func(_ *Tree) ([]ImpliedFilterMember, error) {
			return []ImpliedFilterMember{{
				TargetPath: "/acme/node/a",
				Direction:  "import",
				Position:   "head",
				Ref:        "acme:check",
				ImpliedBy:  "partial leak",
			}}, nil
		})

		const badRun = "/nonexistent/ze-missing-prog --flag"
		tree := NewTree()
		bus := NewTree()
		ep := NewTree()
		ext := NewTree()
		ext.Set("run", badRun)
		ep.SetContainer("external", ext)
		bus.AddListEntry("endpoint", "bad-ext", ep)
		tree.SetContainer("bus", bus)

		report, err := BuildImpliedReport(tree, ImpliedOptions{Offline: true})
		require.Error(t, err)
		assert.Equal(t, ImpliedReport{}, report, "refused offline query must return an empty report, never a partial set")

		opErr, ok := errors.AsType[*opererr.Error](err)
		require.True(t, ok, "offline query refusal must return *opererr.Error, got %T: %v", err, err)
		assert.Contains(t, opErr.Error(), "bad-ext")
		assert.Contains(t, opErr.Error(), badRun)
		assert.Contains(t, opErr.Error(), "not found")
		assert.Equal(t, `ze show plugin conformance run "`+badRun+`"`, opErr.Fix())
	})

	t.Run("valid_program_queries_in_declare_mode", func(t *testing.T) {
		restoreDeclared := ResetDeclaredSchemasForTest()
		t.Cleanup(restoreDeclared)
		restoreProviders := ResetImpliedProvidersForTest()
		t.Cleanup(restoreProviders)

		dir := t.TempDir()
		marker := filepath.Join(dir, "run-loop.marker")
		scriptPath := filepath.Join(dir, "acme-declare.sh")
		script := `#!/bin/sh
if [ "$ZE_PLUGIN_MODE" = "declare" ]; then
  printf '#1 ze-plugin-engine:declare-registration {"protocol_version":1,"sends":[{"message":"acme:ping"}],"receives":[{"message":"acme:pong"}]}\n'
  exit 0
fi
echo "started" > "` + marker + `"
exit 1
`
		require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o755)) //nolint:gosec // test fixture script must be executable

		providerSawDecl := false
		RegisterImpliedBusProvider(func(_ *Tree, _ ImpliedOptions) ([]ImpliedEndpoint, []ImpliedParticipant, error) {
			decl, ok := lookupExternalBusDecl("good-ext")
			providerSawDecl = ok && len(decl.Receives) == 1 && decl.Receives[0].Message == "acme:pong"
			return nil, []ImpliedParticipant{{
				Path:         "/bus/endpoint/good-ext",
				EndpointName: "good-ext",
				Receive: []ImpliedDirectionRule{{
					Message: "acme:pong",
					Allowed: true,
					Origin:  ImpliedOriginRegistered,
					Source:  "registration of good-ext",
				}},
			}}, nil
		})

		tree := NewTree()
		bus := NewTree()
		ep := NewTree()
		ext := NewTree()
		ext.Set("run", scriptPath)
		ep.SetContainer("external", ext)
		bus.AddListEntry("endpoint", "good-ext", ep)
		tree.SetContainer("bus", bus)

		report, err := BuildImpliedReport(tree, ImpliedOptions{
			Offline:    true,
			ConfigFile: filepath.Join(dir, "site.conf"),
		})
		require.NoError(t, err)
		assert.True(t, providerSawDecl, "offline query must record the external declaration before invoking bus provider")
		assert.Equal(t, 1, report.Count())

		_, statErr := os.Stat(marker)
		assert.True(t, os.IsNotExist(statErr), "offline query mode must never start the program run loop")
	})
}

// TestImpliedNoticeOmittedWhenEmpty verifies that empty (and written-only)
// ImpliedReports emit empty notice strings, while non-empty reports emit exact
// counts naming `| implied` without `##` markers.
//
// VALIDATES: ImpliedReport.IsEmpty, ImpliedReport.Count, ImpliedNoticeLine,
// and ImpliedPlainNotice across empty, written-only, and active reports.
// PREVENTS: Noisy plain-display notices when nothing is implied, or miscounted
// notices that include written rules.
func TestImpliedNoticeOmittedWhenEmpty(t *testing.T) {
	empty := ImpliedReport{}
	assert.True(t, empty.IsEmpty())
	assert.Equal(t, 0, empty.Count())
	assert.Empty(t, ImpliedNoticeLine(empty))
	assert.Empty(t, ImpliedPlainNotice(empty))

	writtenOnly := ImpliedReport{
		Participants: []ImpliedParticipant{{
			Path: "/bus/endpoint/collector",
			Receive: []ImpliedDirectionRule{{
				Message: "testimplied:update",
				Allowed: true,
				Origin:  ImpliedOriginWritten,
			}},
		}},
	}
	assert.True(t, writtenOnly.IsEmpty())
	assert.Equal(t, 0, writtenOnly.Count())
	assert.Empty(t, ImpliedNoticeLine(writtenOnly))
	assert.Empty(t, ImpliedPlainNotice(writtenOnly))

	active := ImpliedReport{
		Endpoints: []ImpliedEndpoint{{
			Name:       "auto-ep",
			ReasonKind: "config-root",
			ImpliedBy:  "config-root /bus",
		}},
		Participants: []ImpliedParticipant{{
			Path: "/bus/endpoint/collector",
			Receive: []ImpliedDirectionRule{
				{Message: "testimplied:update", Allowed: true, Origin: ImpliedOriginRegistered},
				{Message: "testimplied:state", Allowed: true, Origin: ImpliedOriginWritten},
			},
			Send: []ImpliedDirectionRule{
				{Message: "testimplied:state", Allowed: true, Origin: ImpliedOriginRequired},
			},
		}},
		Filters: []ImpliedFilterMember{{
			TargetPath: "/acme/node/edge1",
			Direction:  "import",
			Position:   "head",
			Ref:        "acme:loop-check",
			ImpliedBy:  "built-in step",
		}},
	}
	assert.False(t, active.IsEmpty())
	assert.Equal(t, 4, active.Count())

	noticeLine := ImpliedNoticeLine(active)
	assert.True(t, strings.HasPrefix(noticeLine, "# note: 4 "), "expected single-hash note line, got %q", noticeLine)
	assert.False(t, strings.HasPrefix(noticeLine, "##"))
	assert.Contains(t, noticeLine, "| implied")

	plainNotice := ImpliedPlainNotice(active)
	assert.True(t, strings.HasPrefix(plainNotice, "4 "), "expected count prefix, got %q", plainNotice)
	assert.Contains(t, plainNotice, "| implied")
}

// TestGraphBusEdgeProviderOrigins verifies AC-25: BuildGraph consumes the
// registered GraphBusEdgeProvider, retains GraphEdge.Origin across all five
// canonical origins, and never collapses edges that differ in Origin.
//
// VALIDATES: AC-25 GraphEdge.Origin propagation and origin-aware edge dedup.
// PREVENTS: Graph deduplication silently dropping a distinct edge origin.
func TestGraphBusEdgeProviderOrigins(t *testing.T) {
	restore := ResetImpliedProvidersForTest()
	t.Cleanup(restore)

	schema, err := YANGSchema()
	require.NoError(t, err)

	RegisterGraphBusEdgeProvider(func(_ *Tree) []GraphEdge {
		return []GraphEdge{
			{From: "plugin/emitter", To: "plugin/collector", Kind: EdgeProcessBinds, Origin: ImpliedOriginWritten},
			{From: "plugin/emitter", To: "plugin/collector", Kind: EdgeProcessBinds, Origin: ImpliedOriginRequired},
			{From: "plugin/emitter", To: "plugin/audit", Kind: EdgeProcessBinds, Origin: ImpliedOriginInherited},
			{From: "plugin/emitter", To: "plugin/metrics", Kind: EdgeProcessBinds, Origin: ImpliedOriginRegistered},
			{From: "plugin/emitter", To: "plugin/dormant-sink", Kind: EdgeProcessBinds, Origin: ImpliedOriginDormant},
		}
	})

	tree := NewTree()
	bus := NewTree()
	ep := NewTree()
	ep.Set("internal", "emitter")
	bus.AddListEntry("endpoint", "emitter", ep)
	tree.SetContainer("bus", bus)

	g := BuildGraph(tree, schema)

	assert.NotNil(t, findNode(g, "plugin/emitter"))
	assert.True(t, hasEdge(g, "section/bus", "plugin/emitter", EdgeContains))

	originsFound := map[string]int{}
	for _, e := range g.Edges {
		if e.From == "plugin/emitter" && e.Kind == EdgeProcessBinds {
			originsFound[e.Origin]++
		}
	}
	assert.Equal(t, 1, originsFound[ImpliedOriginWritten])
	assert.Equal(t, 1, originsFound[ImpliedOriginRequired])
	assert.Equal(t, 1, originsFound[ImpliedOriginInherited])
	assert.Equal(t, 1, originsFound[ImpliedOriginRegistered])
	assert.Equal(t, 1, originsFound[ImpliedOriginDormant])
}
