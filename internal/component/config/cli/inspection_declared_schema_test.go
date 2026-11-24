// Design: docs/architecture/config/yang-config-design.md -- every config inspection uses the process's declared schema.
// Related: cmd_validate_declared_test.go -- shared external-module and no-exec fixtures.
// Related: supplied_config_test.go -- registered supplied-content routes and envelope.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestConfigInspectionExplainsUnavailableExternalSchema verifies S-3 / AC-18
// across every local and supplied inspection route (dump, graph, diff): when no
// daemon answers, a non-declaring external program is refused naming endpoint,
// run line, cause, and `fix: ze show plugin conformance run "<run>"`, while a
// declare-mode external program succeeds and produces the expected payload
// without ever starting its run loop.
//
// VALIDATES: S-3 / AC-18 — all 8 inspection paths fail closed on non-declaring
// external programs with the conformance fix, and succeed via declare mode
// without touching the run-loop marker.
// PREVENTS: offline config dump/graph/diff starting external run loops or
// losing the conformance fix on declaration query refusal.
func TestConfigInspectionExplainsUnavailableExternalSchema(t *testing.T) {
	for _, path := range declaredInspectionPaths() {
		sides := []string{"before"}
		if path.comparison {
			sides = append(sides, "after")
		}
		for _, side := range sides {
			t.Run(path.name+"/"+side+"/non_declaring_refused", func(t *testing.T) {
				t.Cleanup(config.ResetDeclaredSchemasForTest())
				run, marker := silentPlugin(t)
				before := monitorConfig(run, "acme-monitor { interval 5; }\n")
				after := "bus {}\n"
				if side == "after" {
					before, after = after, before
				}
				answer := inspectDeclaredSchema(t, path.name, before, after)
				require.NotEqual(t, exitOK, answer.Code, "a non-declaring external program was accepted")
				require.Nil(t, answer.Payload, "a refused source produced an inspection payload")
				assert.Contains(t, answer.Diagnostic, `bus endpoint "acme"`)
				assert.Contains(t, answer.Diagnostic, run)
				assert.Contains(t, answer.Diagnostic, "without writing a Stage 1 declaration")
				assert.Equal(t, 1, strings.Count(answer.Diagnostic, "fix: "), "one recovery command: %s", answer.Diagnostic)
				assert.True(t, strings.HasSuffix(answer.Diagnostic, `fix: ze show plugin conformance run "`+run+"\"\n"),
					"conformance fix missing or not last: %s", answer.Diagnostic)
				requireNotStarted(t, marker)
			})
		}

		t.Run(path.name+"/declaring_program_succeeds", func(t *testing.T) {
			t.Cleanup(config.ResetDeclaredSchemasForTest())
			run, marker := declaringPlugin(t, monitorSchemaDecl())
			content := monitorConfig(run, "acme-monitor { interval 5; }\n") + validConfig
			answer := inspectDeclaredSchema(t, path.name, content, content)
			require.Equal(t, exitOK, answer.Code, answer.Diagnostic)
			require.NotNil(t, answer.Payload)
			switch {
			case strings.HasPrefix(path.name, "dump"):
				document, ok := answer.Payload.(map[string]any)
				require.True(t, ok, "dump answered %T", answer.Payload)
				section, ok := document["acme-monitor"].(map[string]any)
				require.True(t, ok, "dump lost the queried external section: %#v", document)
				assert.Equal(t, "5", section["interval"])
			case strings.HasPrefix(path.name, "graph"):
				document, ok := answer.Payload.(command.Blob)
				require.True(t, ok, "graph answered %T", answer.Payload)
				var graph config.Graph
				require.NoError(t, json.Unmarshal([]byte(document), &graph))
				assert.Contains(t, graph.Nodes, config.GraphNode{
					ID: "section/acme-monitor", Kind: config.NodeSection, Name: "acme-monitor",
				})
			default:
				document, ok := answer.Payload.(map[string]any)
				require.True(t, ok, "diff answered %T", answer.Payload)
				require.Len(t, document, 3, "the comparison envelope changed")
				for _, name := range []string{"added", "removed", "changed"} {
					require.Contains(t, document, name)
					assert.Empty(t, document[name], "identical declared sources differed")
				}
			}
			requireNotStarted(t, marker)
		})
	}
}

// TestConfigInspectionUsesPublishedExternalSchema supplies the same module
// a daemon publishes at boot and observes each producer's existing wire shape.
func TestConfigInspectionUsesPublishedExternalSchema(t *testing.T) {
	for _, path := range declaredInspectionPaths() {
		t.Run(path.name, func(t *testing.T) {
			t.Cleanup(config.ResetDeclaredSchemasForTest())
			run, marker := markingPlugin(t)
			require.NoError(t, config.PublishExternalModules([]configyang.ExternalModule{{
				Plugin: "acme", Module: "acme-monitor-conf", Namespace: "urn:acme:monitor", Text: monitorModule,
			}}))
			content := monitorConfig(run, "acme-monitor { interval 5; }\n") + validConfig
			answer := inspectDeclaredSchema(t, path.name, content, content)
			require.Equal(t, exitOK, answer.Code, answer.Diagnostic)
			require.NotNil(t, answer.Payload)
			assert.NotContains(t, answer.Diagnostic, "external plugin")
			switch {
			case strings.HasPrefix(path.name, "dump"):
				document, ok := answer.Payload.(map[string]any)
				require.True(t, ok, "dump answered %T", answer.Payload)
				section, ok := document["acme-monitor"].(map[string]any)
				require.True(t, ok, "dump lost the actual external section: %#v", document)
				assert.Equal(t, "5", section["interval"])
			case strings.HasPrefix(path.name, "graph"):
				document, ok := answer.Payload.(command.Blob)
				require.True(t, ok, "graph answered %T", answer.Payload)
				var graph config.Graph
				require.NoError(t, json.Unmarshal([]byte(document), &graph))
				assert.Contains(t, graph.Nodes, config.GraphNode{
					ID: "section/acme-monitor", Kind: config.NodeSection, Name: "acme-monitor",
				})
			default:
				document, ok := answer.Payload.(map[string]any)
				require.True(t, ok, "diff answered %T", answer.Payload)
				require.Len(t, document, 3, "the comparison envelope changed")
				for _, name := range []string{"added", "removed", "changed"} {
					require.Contains(t, document, name)
					assert.Empty(t, document[name], "identical declared sources differed")
				}
			}
			requireNotStarted(t, marker)
		})
	}
}

// TestConfigInspectionPreservesPublishedRangeRefusals makes the accepted
// module's interval invalid on each comparison side. A known range refusal
// must stay a range refusal, rather than become an unavailable-schema claim.
func TestConfigInspectionPreservesPublishedRangeRefusals(t *testing.T) {
	for _, path := range declaredInspectionPaths() {
		sides := []string{"before"}
		if path.comparison {
			sides = append(sides, "after")
		}
		for _, side := range sides {
			t.Run(path.name+"/"+side, func(t *testing.T) {
				t.Cleanup(config.ResetDeclaredSchemasForTest())
				run, marker := markingPlugin(t)
				require.NoError(t, config.PublishExternalModules([]configyang.ExternalModule{{
					Plugin: "acme", Module: "acme-monitor-conf", Namespace: "urn:acme:monitor", Text: monitorModule,
				}}))
				before := monitorConfig(run, "acme-monitor { interval 11; }\n")
				after := monitorConfig(run, "acme-monitor { interval 5; }\n")
				if side == "after" {
					before, after = after, before
				}
				answer := inspectDeclaredSchema(t, path.name, before, after)
				require.NotEqual(t, exitOK, answer.Code, "an out-of-range source was accepted")
				require.Nil(t, answer.Payload)
				assert.Contains(t, answer.Diagnostic, "interval")
				assert.NotContains(t, answer.Diagnostic, "unknown top-level keyword")
				assert.NotContains(t, answer.Diagnostic, "external plugin")
				assert.NotContains(t, answer.Diagnostic, "fix: ze cli")
				assert.NotContains(t, answer.Diagnostic, "fix: ze config validate <config-file>")
				requireNotStarted(t, marker)
			})
		}
	}
}

type declaredInspectionPath struct {
	name       string
	comparison bool
	// asked is true for a route that asks the running ze about a config that
	// declares an external plugin: the operator's own file and store forms.
	// The supplied forms are the daemon's producers, which ask nobody.
	asked bool
}

// declaredInspectionPaths names the concrete input routes that must preserve
// the same declared-schema boundary, including both current and history bytes.
func declaredInspectionPaths() []declaredInspectionPath {
	return []declaredInspectionPath{
		{name: "dump file", asked: true},
		{name: "dump supplied"},
		{name: "graph file", asked: true},
		{name: "graph supplied"},
		{name: "diff files", comparison: true, asked: true},
		{name: "diff stored", comparison: true, asked: true},
		{name: "diff history", comparison: true, asked: true},
		{name: "diff supplied", comparison: true},
	}
}

func inspectDeclaredSchema(t *testing.T, path, before, after string) command.LocalAnswer {
	t.Helper()
	switch path {
	case "dump file":
		return answerOn(t, "show config dump", []string{writeTestConfig(t, before)}, false)
	case "dump supplied":
		return suppliedConfigAnswer(t, "dump", suppliedConfigTokens(before))
	case "graph file":
		return answerOn(t, "show config graph", []string{writeTestConfig(t, before)}, false)
	case "graph supplied":
		return suppliedConfigAnswer(t, "graph", suppliedConfigTokens(before))
	case "diff files":
		return answerOn(t, "show config diff", []string{writeTestConfig(t, before), writeTestConfig(t, after)}, false)
	case "diff supplied":
		return suppliedConfigAnswer(t, "diff", suppliedConfigTokens(suppliedConfigPair(t, before, after)))
	case "diff stored", "diff history":
		store, err := storage.Create(t.TempDir())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, store.Close()) })
		require.NoError(t, store.WriteFile("before.conf", []byte(before), 0o600))
		require.NoError(t, store.WriteFile("after.conf", []byte(after), 0o600))
		first := "before.conf"
		if path == "diff history" {
			require.NoError(t, store.WriteVersion("after.conf", []byte(before), time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)))
			first = "1"
		}
		var diag textbuf.Buffer
		diff, code := diffAsking(store, []string{first, "after.conf"}, first+" after.conf", command.RouteLocal, "", routeAsk, &diag)
		if code != exitOK {
			return command.LocalRefusal(code, diag.String())
		}
		return diffAnswer(diff)
	default:
		t.Fatalf("unknown declared-schema inspection route %q", path)
		return command.LocalAnswer{}
	}
}
