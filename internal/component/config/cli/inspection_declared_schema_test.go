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

// TestConfigInspectionExplainsUnavailableExternalSchema reaches local and
// supplied readers with no published external module. Each comparison puts
// the declaration and unknown section on either side, including stored history.
func TestConfigInspectionExplainsUnavailableExternalSchema(t *testing.T) {
	for _, path := range declaredInspectionPaths() {
		sides := []string{"before"}
		if path.comparison {
			sides = append(sides, "after")
		}
		for _, side := range sides {
			t.Run(path.name+"/"+side, func(t *testing.T) {
				t.Cleanup(config.ResetDeclaredSchemasForTest())
				run, marker := markingPlugin(t)
				before := monitorConfig(run, "acme-monitor { interval 5; }\n")
				after := "plugin {}\n"
				if side == "after" {
					before, after = after, before
				}
				answer := inspectDeclaredSchema(t, path.name, before, after)
				require.NotEqual(t, exitOK, answer.Code, "an unavailable section was accepted")
				require.Nil(t, answer.Payload, "a refused source produced an inspection payload")
				assert.Contains(t, answer.Diagnostic, "unknown top-level keyword: acme-monitor")
				assert.Contains(t, answer.Diagnostic, `external plugin "acme"`)
				assert.Contains(t, answer.Diagnostic, "starts no plugin")
				assert.Equal(t, 1, strings.Count(answer.Diagnostic, "fix: "), "one recovery command: %s", answer.Diagnostic)
				assert.True(t, strings.HasSuffix(answer.Diagnostic, "fix: ze config validate <config-file>\n"), "the content-validation recovery was lost: %s", answer.Diagnostic)
				assert.NotContains(t, answer.Diagnostic, "fix: ze cli")
				requireNotStarted(t, marker)
			})
		}
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
}

// declaredInspectionPaths names the concrete input routes that must preserve
// the same declared-schema boundary, including both current and history bytes.
func declaredInspectionPaths() []declaredInspectionPath {
	return []declaredInspectionPath{
		{name: "dump file"},
		{name: "dump supplied"},
		{name: "graph file"},
		{name: "graph supplied"},
		{name: "diff files", comparison: true},
		{name: "diff stored", comparison: true},
		{name: "diff history", comparison: true},
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
		diff, code := resolveDiff(store, []string{first, "after.conf"}, &diag)
		if code != exitOK {
			return command.LocalRefusal(code, diag.String())
		}
		return diffAnswer(diff)
	default:
		t.Fatalf("unknown declared-schema inspection route %q", path)
		return command.LocalAnswer{}
	}
}
