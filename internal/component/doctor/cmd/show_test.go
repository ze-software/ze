package cmd

import (
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
