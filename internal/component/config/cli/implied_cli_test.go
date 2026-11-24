// Design: docs/architecture/api/architecture.md -- CLI and RPC tests for `| implied` across `show config running`, `show config source`, `show config dump`, and `ze config show` stderr notice.
// Related: implied.go -- shared `| implied` pipe helpers and `{config, implied}` text renderer.
// Related: running_rpc.go -- `show config running | implied`.
// Related: config_supplied_tools.go -- `show config source | implied`.
// Related: config_data.go -- `show config dump | implied`.
// Related: cmd_show.go -- `ze config show` stderr notice.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/config"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestImpliedPipeFilterRegistration verifies AC-23: `implied` is registered as
// a command-declared ArgNone pipe filter on `show config running`,
// `show config source`, and `show config dump`, and rejects unexpected
// arguments.
//
// VALIDATES: command.PipeFiltersForCommand registration on all three config
// view commands and ArgNone argument enforcement.
// PREVENTS: `| implied` added to the global pipe catalog or accepting an
// ignored argument.
func TestImpliedPipeFilterRegistration(t *testing.T) {
	paths := []string{
		"show config running",
		"show config source",
		"show config dump",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			filters := command.PipeFiltersForCommand(path)
			found := false
			for _, f := range filters {
				if f.Name == pipeOpImplied {
					found = true
					assert.Equal(t, command.ArgNone, f.Argument, "%s implied filter must take ArgNone", path)
					assert.NotEmpty(t, f.Description)
				}
			}
			require.True(t, found, "%s must declare pipe filter %q, got %+v", path, pipeOpImplied, filters)

			_, _, errMsg := command.ProcessPipesDefaultFormatChecked(path+" | implied extra", "")
			assert.NotEmpty(t, errMsg, "%s | implied extra must be refused by ArgNone", path)
		})
	}
}

// TestShowConfigRunningImplied verifies AC-23 and AC-38 on `show config
// running`: without `| implied` the answer stays the plain masked config map
// with no `implied` key and no human notice; with `| implied` the data is the
// `{config, implied}` envelope (keeping written config byte-pure and masked),
// `| json` and `| yaml` emit zero human notice, default / `| text` renders
// `# implied by:` / `# implied:` config text, and `| match` filters it.
//
// VALIDATES: `show config running` parity without `| implied`, `{config,
// implied}` envelope with `| implied`, `# implied by:` text rendering, and
// `| match` composition.
// PREVENTS: Human notices leaking into JSON/YAML/RPC or `| implied` altering
// plain `show config running` output.
func TestShowConfigRunningImplied(t *testing.T) {
	restoreProviders := config.ResetImpliedProvidersForTest()
	t.Cleanup(restoreProviders)

	config.RegisterImpliedBusProvider(func(_ *config.Tree, _ config.ImpliedOptions) ([]config.ImpliedEndpoint, []config.ImpliedParticipant, error) {
		eps := []config.ImpliedEndpoint{{
			Name:           "acme-agent",
			CompiledPlugin: "acme-agent",
			ReasonKind:     "config-root",
			ReasonSource:   "ddos",
			ImpliedBy:      "config section ddos",
		}}
		parts := []config.ImpliedParticipant{{
			Path:         "/bus/endpoint/acme-agent",
			EndpointName: "acme-agent",
			Send: []config.ImpliedDirectionRule{{
				Message:          "acme:alert",
				Allowed:          true,
				Origin:           config.ImpliedOriginWritten,
				Targets:          []string{"/acme/peer/edge1"},
				ReplacedDefaults: []string{"/acme/peer/*"},
			}},
		}}
		return eps, parts, nil
	})
	config.RegisterImpliedFilterProvider("acme-fixture", func(_ *config.Tree) ([]config.ImpliedFilterMember, error) {
		return []config.ImpliedFilterMember{{
			TargetPath: "/acme/peer/edge1",
			Direction:  "export",
			Position:   "tail",
			Ref:        "acme:default-step",
			ImpliedBy:  "built-in export rule",
		}}, nil
	})

	runningMap := map[string]any{
		"ddos": map[string]any{
			"flowspec": map[string]any{
				"response-level": "alert",
				"action":         "discard",
			},
		},
		"system": map[string]any{
			"authentication": map[string]any{
				"user": map[string]any{
					"ci": map[string]any{
						"password": "$2a$10$abcdefghijklmnopqrstuv",
						"profile":  []any{"admin"},
					},
				},
			},
		},
	}

	coordinator := zePlugin.NewCoordinator(runningMap)
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, coordinator)
	require.NoError(t, err)

	// Without | implied: pure written map, no implied key, no notice.
	plainResp, err := srv.Dispatcher().Dispatch(&pluginserver.CommandContext{Server: srv}, "show config running")
	require.NoError(t, err)
	require.Equal(t, zePlugin.StatusDone, plainResp.Status, plainResp.Error)
	plainMap, ok := plainResp.Data.(zePlugin.Map)
	require.True(t, ok, "plain show config running must return plugin.Map, got %T", plainResp.Data)
	_, hasImpliedKey := plainMap[impliedKeyImplied]
	assert.False(t, hasImpliedKey, "plain show config running must not contain an implied key")
	assert.Empty(t, plainResp.Warnings, "plain show config running must carry no notice warning")

	// With | implied on RPC/REST/gRPC path (RendersResidualChain: true keeps raw envelope).
	impResp, err := srv.Dispatcher().Dispatch(&pluginserver.CommandContext{
		Server:               srv,
		RendersResidualChain: true,
	}, "show config running | implied")
	require.NoError(t, err)
	require.Equal(t, zePlugin.StatusDone, impResp.Status, impResp.Error)
	assert.Empty(t, impResp.Warnings, "structured implied response must carry no human notice")

	rawJSON, err := zePlugin.ResponseJSON(impResp, nil)
	require.NoError(t, err)
	assert.NotContains(t, rawJSON, "implied item(s) active", "JSON payload must never carry human notice")
	assert.NotContains(t, rawJSON, "$2a$10$abcdefghijklmnopqrstuv", "secrets must stay masked")

	var decoded struct {
		Config  map[string]any       `json:"config"`
		Implied config.ImpliedReport `json:"implied"`
	}
	require.NoError(t, json.Unmarshal([]byte(rawJSON), &decoded))
	assert.Contains(t, decoded.Config, "ddos")
	assert.Len(t, decoded.Implied.Endpoints, 1)
	assert.Equal(t, "acme-agent", decoded.Implied.Endpoints[0].Name)
	assert.Len(t, decoded.Implied.Participants, 1)
	assert.Equal(t, []string{"/acme/peer/*"}, decoded.Implied.Participants[0].Send[0].ReplacedDefaults)
	assert.Len(t, decoded.Implied.Filters, 1)

	// | implied | json emits pure {config, implied} JSON without any human notice.
	_, fmtJSON, errMsg := command.ProcessPipesDefaultFormatChecked("show config running | implied | json", "")
	require.Empty(t, errMsg)
	jsonOut := fmtJSON(rawJSON)
	assert.Contains(t, jsonOut, `"config"`)
	assert.Contains(t, jsonOut, `"implied"`)
	assert.NotContains(t, jsonOut, "implied item(s) active")

	// | implied | yaml emits config: + implied: YAML without any human notice.
	_, fmtYAML, errMsg := command.ProcessPipesDefaultFormatChecked("show config running | implied | yaml", "")
	require.Empty(t, errMsg)
	yamlOut := fmtYAML(rawJSON)
	assert.Contains(t, yamlOut, "config:")
	assert.Contains(t, yamlOut, "implied:")
	assert.NotContains(t, yamlOut, "implied item(s) active")

	// Default text and explicit | text render # implied by: and # implied: comments.
	for _, line := range []string{"show config running | implied", "show config running | implied | text"} {
		_, fmtText, msg := command.ProcessPipesDefaultFormatChecked(line, "text")
		require.Empty(t, msg, line)
		textOut := fmtText(rawJSON)
		assert.Contains(t, textOut, "response-level alert", line)
		assert.Contains(t, textOut, "# implied by: config section ddos", line)
		assert.Contains(t, textOut, "internal acme-agent;", line)
		assert.Contains(t, textOut, "replaced registered default(s)", line)
		assert.Contains(t, textOut, "# implied: export tail acme:default-step (built-in export rule)", line)
		assert.NotEqual(t, config.FormatSetMeta, config.DetectFormat(textOut), "rendered implied text must never look like FormatSetMeta")
	}

	// | implied | match <pat> and | implied | text | match <pat> filter rendered lines.
	for _, line := range []string{
		"show config running | implied | match acme-agent",
		"show config running | implied | text | match acme-agent",
	} {
		_, fmtMatch, msg := command.ProcessPipesDefaultFormatChecked(line, "text")
		require.Empty(t, msg, line)
		matchOut := fmtMatch(rawJSON)
		assert.Contains(t, matchOut, "acme-agent", line)
		assert.NotContains(t, matchOut, "response-level alert", line)
	}
}

// TestImpliedReceptionPrintsAsWrittenBlock verifies AC-38: an implied bus
// endpoint block emitted by `show config running | implied` parses back as
// valid written configuration through config.Parser without error.
//
// VALIDATES: AC-38 round-trip of rendered `# implied by:` blocks into
// config.Parser.
// PREVENTS: `| implied` printing pseudo-syntax that operators cannot copy and
// commit.
func TestImpliedReceptionPrintsAsWrittenBlock(t *testing.T) {
	restoreProviders := config.ResetImpliedProvidersForTest()
	t.Cleanup(restoreProviders)

	config.RegisterImpliedBusProvider(func(_ *config.Tree, _ config.ImpliedOptions) ([]config.ImpliedEndpoint, []config.ImpliedParticipant, error) {
		return []config.ImpliedEndpoint{{
			Name:           "acme-agent",
			CompiledPlugin: "acme-agent",
			ReasonKind:     "config-root",
			ImpliedBy:      "config section ddos",
		}}, nil, nil
	})

	coordinator := zePlugin.NewCoordinator(map[string]any{
		"ddos": map[string]any{
			"flowspec": map[string]any{"response-level": "alert"},
		},
	})
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, coordinator)
	require.NoError(t, err)

	resp, err := srv.Dispatcher().Dispatch(&pluginserver.CommandContext{
		Server:               srv,
		RendersResidualChain: true,
	}, "show config running | implied")
	require.NoError(t, err)
	require.Equal(t, zePlugin.StatusDone, resp.Status, resp.Error)

	rawJSON, err := zePlugin.ResponseJSON(resp, nil)
	require.NoError(t, err)
	_, fmtText, errMsg := command.ProcessPipesDefaultFormatChecked("show config running | implied", "text")
	require.Empty(t, errMsg)
	rendered := fmtText(rawJSON)

	require.Contains(t, rendered, "# implied by: config section ddos")
	require.NotEqual(t, config.FormatSetMeta, config.DetectFormat(rendered))

	schema, err := config.YANGSchema()
	require.NoError(t, err)
	parsed, err := config.NewParser(schema).Parse(rendered)
	require.NoError(t, err, "rendered implied text must parse cleanly:\n%s", rendered)
	busNode := parsed.GetContainer("bus")
	require.NotNil(t, busNode, "parsed config must contain the bus container")
	epNode := busNode.GetList("endpoint")["acme-agent"]
	require.NotNil(t, epNode, "parsed config must contain endpoint acme-agent")
	internalVal, ok := epNode.Get("internal")
	require.True(t, ok)
	assert.Equal(t, "acme-agent", internalVal)
}

// TestImpliedShowsInsertedFilterMembers verifies AC-46: `show config running |
// implied` reads inserted filter members strictly from registered
// ImpliedFilterProvider seams without hardcoding any filter names. Removing a
// member from the seam removes it from the view.
//
// VALIDATES: AC-46 seam-driven implied filter display and negative removal.
// PREVENTS: config/cli hardcoding filter member names instead of reading the
// registered seam.
func TestImpliedShowsInsertedFilterMembers(t *testing.T) {
	restoreProviders := config.ResetImpliedProvidersForTest()
	t.Cleanup(restoreProviders)

	members := []config.ImpliedFilterMember{
		{TargetPath: "/acme/peer/p1", Direction: "import", Position: "head", Ref: "acme:head-check", ImpliedBy: "built-in import rule"},
		{TargetPath: "/acme/peer/p1", Direction: "export", Position: "tail", Ref: "acme:tail-step", ImpliedBy: "built-in export rule"},
		{TargetPath: "/acme/peer/p1", Direction: "export", Position: "head", Ref: "acme:any-peer-guard", ImpliedBy: "/acme/peer/p2 role", Required: true, Reason: "any-peer policy"},
		{TargetPath: "/acme/peer/p2", Direction: "export", Position: "head", Ref: "acme:any-peer-guard", ImpliedBy: "/acme/peer/p2 role", Required: true, Reason: "any-peer policy"},
	}
	config.RegisterImpliedFilterProvider("acme-filters", func(_ *config.Tree) ([]config.ImpliedFilterMember, error) {
		return members, nil
	})

	coordinator := zePlugin.NewCoordinator(map[string]any{
		"ddos": map[string]any{"flowspec": map[string]any{"response-level": "alert"}},
	})
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, coordinator)
	require.NoError(t, err)

	dispatchText := func() string {
		resp, err := srv.Dispatcher().Dispatch(&pluginserver.CommandContext{
			Server:               srv,
			RendersResidualChain: true,
		}, "show config running | implied")
		require.NoError(t, err)
		require.Equal(t, zePlugin.StatusDone, resp.Status, resp.Error)
		rawJSON, err := zePlugin.ResponseJSON(resp, nil)
		require.NoError(t, err)
		_, fmtText, errMsg := command.ProcessPipesDefaultFormatChecked("show config running | implied", "text")
		require.Empty(t, errMsg)
		return fmtText(rawJSON)
	}

	first := dispatchText()
	assert.Contains(t, first, "import head acme:head-check (built-in import rule)")
	assert.Contains(t, first, "export tail acme:tail-step (built-in export rule)")
	assert.Contains(t, first, "export head acme:any-peer-guard (/acme/peer/p2 role, required: any-peer policy)")

	// Negative case: drop the any-peer entries from the fixture seam; they must leave the view.
	members = members[:2]
	second := dispatchText()
	assert.Contains(t, second, "acme:head-check")
	assert.NotContains(t, second, "acme:any-peer-guard")
}

// TestShowConfigSourceAndDumpImpliedOffline verifies AC-24: offline `show
// config source <file> | implied` and `show config dump <file> | implied`
// query external endpoints in declare mode without starting their run loop,
// and refuse with an operator error naming endpoint, run line, cause, and
// conformance command (with zero partial output) when a program is missing.
//
// VALIDATES: AC-24 offline declare-mode success and fail-closed refusal for
// both `show config source | implied` and `show config dump | implied`.
// PREVENTS: Offline `| implied` starting a plugin run loop or returning a
// partial report when an external endpoint cannot be queried.
func TestShowConfigSourceAndDumpImpliedOffline(t *testing.T) {
	t.Run("query_mode_succeeds_without_run_loop", func(t *testing.T) {
		restoreDeclared := config.ResetDeclaredSchemasForTest()
		t.Cleanup(restoreDeclared)
		restoreProviders := config.ResetImpliedProvidersForTest()
		t.Cleanup(restoreProviders)

		dir := t.TempDir()
		marker := filepath.Join(dir, "run-loop.marker")
		scriptPath := filepath.Join(dir, "acme-declare.sh")
		script := "#!/bin/sh\n" +
			"if [ \"$ZE_PLUGIN_MODE\" = \"declare\" ]; then\n" +
			"  printf '#1 ze-plugin-engine:declare-registration {\"protocol-version\":1,\"sends\":[{\"message\":\"acme:ping\"}],\"receives\":[{\"message\":\"acme:pong\"}]}\\n'\n" +
			"  exit 0\n" +
			"fi\n" +
			"echo \"started\" > \"" + marker + "\"\n" +
			"exit 1\n"
		require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o755)) //nolint:gosec // test fixture script must be executable

		config.RegisterImpliedBusProvider(func(_ *config.Tree, _ config.ImpliedOptions) ([]config.ImpliedEndpoint, []config.ImpliedParticipant, error) {
			return nil, []config.ImpliedParticipant{{
				Path:         "/bus/endpoint/acme-ext",
				EndpointName: "acme-ext",
				Receive: []config.ImpliedDirectionRule{{
					Message: "acme:pong",
					Allowed: true,
					Origin:  config.ImpliedOriginRegistered,
					Source:  "registration of acme-ext",
				}},
			}}, nil
		})

		cfgText := "bgp {\n    router-id 1.2.3.4;\n}\nbus {\n    endpoint acme-ext {\n        external {\n            run \"" + scriptPath + "\";\n        }\n    }\n}\n"
		cfgPath := filepath.Join(dir, "site.conf")
		require.NoError(t, os.WriteFile(cfgPath, []byte(cfgText), 0o600))

		// show config source <file> | implied
		srcArgs := commandtest.Args(cfgPath).WithPipeOperations([]command.PipeOperation{{Name: pipeOpImplied}})
		srcAns := command.AnswerLocal(dataSource, srcArgs)
		require.Equal(t, exitOK, srcAns.Code, srcAns.Diagnostic)
		srcEnv, ok := srcAns.Payload.(config.ImpliedEnvelope)
		require.True(t, ok, "show config source | implied must answer config.ImpliedEnvelope, got %T", srcAns.Payload)
		assert.Equal(t, 1, srcEnv.Implied.Count())

		srcRaw, err := json.Marshal(srcAns.Payload)
		require.NoError(t, err)
		_, fmtSrcText, errMsg := command.ProcessPipesDefaultFormatChecked("show config source "+cfgPath+" | implied", "")
		require.Empty(t, errMsg)
		assert.Contains(t, fmtSrcText(string(srcRaw)), "# implied by: registration of acme-ext")

		_, fmtSrcMatch, errMsg := command.ProcessPipesDefaultFormatChecked("show config source "+cfgPath+" | implied | text | match acme:pong", "")
		require.Empty(t, errMsg)
		assert.Contains(t, fmtSrcMatch(string(srcRaw)), "acme:pong")

		// show config dump <file> | implied
		dumpArgs := commandtest.Args(cfgPath).WithPipeOperations([]command.PipeOperation{{Name: pipeOpImplied}})
		dumpAns := command.AnswerLocal(dataDump, dumpArgs)
		require.Equal(t, exitOK, dumpAns.Code, dumpAns.Diagnostic)
		dumpEnv, ok := dumpAns.Payload.(config.ImpliedEnvelope)
		require.True(t, ok, "show config dump | implied must answer config.ImpliedEnvelope, got %T", dumpAns.Payload)
		assert.Equal(t, 1, dumpEnv.Implied.Count())

		dumpRaw, err := json.Marshal(dumpAns.Payload)
		require.NoError(t, err)
		_, fmtDumpText, errMsg := command.ProcessPipesDefaultFormatChecked("show config dump "+cfgPath+" | implied", "text")
		require.Empty(t, errMsg)
		assert.Contains(t, fmtDumpText(string(dumpRaw)), "# implied by: registration of acme-ext")

		_, fmtDumpMatch, errMsg := command.ProcessPipesDefaultFormatChecked("show config dump "+cfgPath+" | implied | text | match acme:pong", "")
		require.Empty(t, errMsg)
		assert.Contains(t, fmtDumpMatch(string(dumpRaw)), "acme:pong")

		_, statErr := os.Stat(marker)
		assert.True(t, os.IsNotExist(statErr), "offline declare-mode query must never start the program run loop")
	})

	t.Run("missing_binary_refuses_with_zero_partial_output", func(t *testing.T) {
		restoreDeclared := config.ResetDeclaredSchemasForTest()
		t.Cleanup(restoreDeclared)
		restoreProviders := config.ResetImpliedProvidersForTest()
		t.Cleanup(restoreProviders)

		config.RegisterImpliedBusProvider(func(_ *config.Tree, _ config.ImpliedOptions) ([]config.ImpliedEndpoint, []config.ImpliedParticipant, error) {
			return []config.ImpliedEndpoint{{
				Name:      "must-not-leak",
				ImpliedBy: "partial leak",
			}}, nil, nil
		})

		const badRun = "/nonexistent/ze-missing-implied-plugin --flag"
		cfgText := "bgp {\n    router-id 1.2.3.4;\n}\nbus {\n    endpoint bad-ext {\n        external {\n            run \"" + badRun + "\";\n        }\n    }\n}\n"
		cfgPath := writeTestConfig(t, cfgText)

		for _, tc := range []struct {
			name    string
			handler command.LocalDataHandler
		}{
			{name: "source", handler: dataSource},
			{name: "dump", handler: dataDump},
		} {
			t.Run(tc.name, func(t *testing.T) {
				args := commandtest.Args(cfgPath).WithPipeOperations([]command.PipeOperation{{Name: pipeOpImplied}})
				ans := command.AnswerLocal(tc.handler, args)
				assert.NotEqual(t, exitOK, ans.Code, "missing external binary must fail closed")
				assert.Nil(t, ans.Payload, "refused offline implied must return zero payload, never partial data")
				assert.Contains(t, ans.Diagnostic, "bad-ext")
				assert.Contains(t, ans.Diagnostic, badRun)
				assert.Contains(t, ans.Diagnostic, `ze show plugin conformance run "`+badRun+`"`)
			})
		}
	})
}

// TestConfigShowStderrNotice verifies AC-23: `ze config show` writes byte-pure
// written configuration to stdout and writes the plain-display notice to
// stderr only when implied items exist (and nothing to stderr when empty).
//
// VALIDATES: stdout byte-purity and stderr ImpliedPlainNotice of showConfigTo.
// PREVENTS: human notices polluting `ze config show` stdout or appearing when
// no implied items are active.
func TestConfigShowStderrNotice(t *testing.T) {
	cfgPath := writeTestConfig(t, showTestConfig)

	t.Run("empty_implied_writes_no_stderr_notice", func(t *testing.T) {
		restoreProviders := config.ResetImpliedProvidersForTest()
		t.Cleanup(restoreProviders)

		var stdout, stderr bytes.Buffer
		rc := showConfigTo(&stdout, &stderr, []string{cfgPath})
		require.Equal(t, exitOK, rc, stderr.String())
		assert.Contains(t, stdout.String(), "router-id")
		assert.NotContains(t, stdout.String(), "implied")
		assert.Empty(t, stderr.String(), "empty implied report must write nothing to stderr")
	})

	t.Run("active_implied_writes_notice_only_to_stderr", func(t *testing.T) {
		restoreProviders := config.ResetImpliedProvidersForTest()
		t.Cleanup(restoreProviders)

		config.RegisterImpliedBusProvider(func(_ *config.Tree, _ config.ImpliedOptions) ([]config.ImpliedEndpoint, []config.ImpliedParticipant, error) {
			return []config.ImpliedEndpoint{{
				Name:      "acme-agent",
				ImpliedBy: "config section bgp",
			}}, nil, nil
		})

		var stdout, stderr bytes.Buffer
		rc := showConfigTo(&stdout, &stderr, []string{cfgPath})
		require.Equal(t, exitOK, rc, stderr.String())
		assert.Contains(t, stdout.String(), "router-id")
		assert.NotContains(t, stdout.String(), "implied", "stdout must stay byte-pure written config")
		assert.Equal(t, "1 implied item(s) active — use | implied to view\n", strings.TrimLeft(stderr.String(), "\r"))
	})
}
