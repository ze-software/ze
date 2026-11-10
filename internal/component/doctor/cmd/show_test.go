package cmd

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/doctor"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestPlatformMatchesEmpty(t *testing.T) {
	t.Parallel()
	assert.True(t, platformMatches(nil, nil), "nil platforms = match any")
	assert.True(t, platformMatches([]string{}, nil), "empty platforms = match any")
}

func TestPlatformMatchesAny(t *testing.T) {
	t.Parallel()
	assert.True(t, platformMatches([]string{"any"}, nil))
}

func TestPlatformMatchesSpecific(t *testing.T) {
	t.Parallel()
	info := &host.PlatformInfo{Type: host.PlatformDarwin}
	assert.True(t, platformMatches([]string{"darwin"}, info))
	assert.False(t, platformMatches([]string{"gokrazy"}, info))
}

func TestPlatformMatchesNilInfo(t *testing.T) {
	t.Parallel()
	assert.False(t, platformMatches([]string{"darwin"}, nil))
}

func TestNormalizeSeverity(t *testing.T) {
	t.Parallel()
	assert.Equal(t, diagnostic.SeverityError, normalizeSeverity("error"))
	assert.Equal(t, diagnostic.SeverityWarning, normalizeSeverity("warning"))
	assert.Equal(t, diagnostic.SeverityWarning, normalizeSeverity("unknown"))
	assert.Equal(t, diagnostic.SeverityWarning, normalizeSeverity(""))
}

// TestDoctorOneProducer proves AC-14 for doctor: `ze doctor` (the local-data
// handler registered at doctor), and the daemon's doctor and show doctor (one
// RPC handler, HandleDoctor) answer through one producer.
//
// Method: run the registered local handler on the local route and HandleDoctor
// on the daemon route with the same words, and compare the encoded payloads.
// The kernel-capabilities mode is used because its probe answers the same on
// two calls, where the readiness checks report live disk and descriptor
// figures. A refused word must be refused alike on both routes, and the daemon
// error must be the local refusal's first line minus its "error: " prefix.
func TestDoctorOneProducer(t *testing.T) {
	handler, rest := command.LookupLocalData([]string{"doctor"})
	require.NotNil(t, handler, "doctor is not registered as a local-data command")
	require.Empty(t, rest)

	local := command.AnswerLocal(handler, commandtest.Args("kernel-capabilities"))
	require.Equal(t, 0, local.Code, "local diagnostic: %s", local.Diagnostic)
	want, err := json.Marshal(local.Payload)
	require.NoError(t, err)

	resp, err := HandleDoctor(&pluginserver.CommandContext{}, commandtest.Args("kernel-capabilities"))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	got, ok := resp.Data.(plugin.RawJSON)
	require.True(t, ok, "daemon data is %T, want the encoded producer answer", resp.Data)
	assert.JSONEq(t, string(want), string(got), "daemon and local answers differ")
	assert.Contains(t, string(got), `"capabilities"`)

	refusedLocal := command.AnswerLocal(handler, commandtest.Args("bogus"))
	require.Equal(t, 1, refusedLocal.Code)
	firstLine, _, _ := strings.Cut(refusedLocal.Diagnostic, "\n")
	_, err = HandleDoctor(&pluginserver.CommandContext{}, commandtest.Args("bogus"))
	require.Error(t, err)
	reason := strings.TrimPrefix(firstLine, "error: ")
	assert.True(t, strings.HasPrefix(err.Error(), reason), "daemon refusal %q does not open with the local reason %q", err.Error(), reason)
}

// TestDoctorDaemonRouteCarriesPluginChecks proves the daemon form adds the
// plugin checks the producer is handed, and the local route adds none.
//
// Method: call the producer with a runtime source holding one error, then
// with none, over a config file that does not exist (both report the missing
// config the same way), and compare what each answers.
func TestDoctorDaemonRouteCarriesPluginChecks(t *testing.T) {
	runtime := func() []diagnostic.Diagnostic {
		return []diagnostic.Diagnostic{{Code: "fixture-plugin-check", Severity: diagnostic.SeverityError, Message: "plugin fixture is unhappy"}}
	}
	withChecks := doctor.Answer(commandtest.Args(), runtime)
	encoded, err := json.Marshal(withChecks.Payload)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"fixture-plugin-check"`)
	assert.Contains(t, string(encoded), `"ready":false`)

	without := doctor.Answer(commandtest.Args(), nil)
	encoded, err = json.Marshal(without.Payload)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), `"fixture-plugin-check"`)
}

// fakeDoctorCheckSource is a running-plugin set for collectPluginDoctorChecks:
// one plugin declaring one check, answering a fixed diagnostic.
type fakeDoctorCheckSource struct {
	answer rpc.DoctorCheckOutput
}

func (fakeDoctorCheckSource) DoctorCheckPlugins() map[string][]plugin.DoctorCheckRegistration {
	return map[string][]plugin.DoctorCheckRegistration{
		"fixture-plugin": {{Name: "fixture-check", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"}}},
	}
}

func (f fakeDoctorCheckSource) CallDoctorCheck(context.Context, string, string) (*rpc.DoctorCheckOutput, error) {
	return &f.answer, nil
}

// TestCollectPluginDoctorChecksCarriesFix asks a running plugin for its
// doctor check and reads what the daemon route builds from the answer.
//
// VALIDATES: AC-3 on the daemon route, the fix a plugin's doctor-check answer
// carries reaches the doctor diagnostic unchanged.
// PREVENTS: the callback bridge rebuilding a diagnostic from code, severity
// and message alone and dropping the fix command.
func TestCollectPluginDoctorChecksCarriesFix(t *testing.T) {
	source := fakeDoctorCheckSource{answer: rpc.DoctorCheckOutput{Diagnostics: []rpc.DoctorCheckDiagnostic{{
		Code: "doctor-fixture-degraded", Severity: "error", Message: "fixture state is degraded", Fix: "show fixture state",
	}}}}
	diags := collectDoctorChecksFrom(source, "")
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-fixture-degraded", diags[0].Code)
	assert.Equal(t, diagnostic.SeverityError, diags[0].Severity)
	assert.Equal(t, "fixture state is degraded", diags[0].Message)
	assert.Equal(t, "show fixture state", diags[0].Fix)
}

// TestDoctorRuntimePhaseRunsOnDaemonRoute registers a runtime-phase check and
// runs show doctor's handler.
//
// VALIDATES: AC-1, the daemon route runs a registered runtime check and its
// diagnostic reaches the answer with code, severity, message and fix.
// PREVENTS: a runtime check that no route runs.
func TestDoctorRuntimePhaseRunsOnDaemonRoute(t *testing.T) {
	const name = "test-daemon-runtime"
	check := diagnostic.DoctorCheck{
		Name: name, Phase: diagnostic.DoctorPhaseRuntime, Component: "test",
		Dependencies: []string{"x"}, Platforms: []string{diagnostic.DoctorPlatformAny},
		Codes: []string{"doctor-test-daemon-runtime"},
		Check: func(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
			return []diagnostic.Diagnostic{{
				Code: "doctor-test-daemon-runtime", Severity: diagnostic.SeverityWarning,
				Message: "daemon runtime state is degraded", Fix: "show test runtime",
			}}
		},
	}
	require.NoError(t, diagnostic.RegisterDoctorCheck(check))
	t.Cleanup(func() { diagnostic.UnregisterDoctorCheckForTest(name) })

	resp, err := HandleDoctor(&pluginserver.CommandContext{}, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	got, ok := resp.Data.(plugin.RawJSON)
	require.True(t, ok, "daemon data is %T, want the encoded producer answer", resp.Data)

	var result diagnostic.DoctorResult
	require.NoError(t, json.Unmarshal([]byte(got), &result))
	var found *diagnostic.Diagnostic
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Code == "doctor-test-daemon-runtime" {
			found = &result.Diagnostics[i]
		}
	}
	require.NotNil(t, found, "show doctor did not run the runtime check: %s", got)
	assert.Equal(t, diagnostic.SeverityWarning, found.Severity)
	assert.Equal(t, "daemon runtime state is degraded", found.Message)
	assert.Equal(t, "show test runtime", found.Fix)
}
