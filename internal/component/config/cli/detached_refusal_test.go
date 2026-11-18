// Design: docs/architecture/api/commands.md -- native detached set refusals.
// Related: cmd_set_refusal_test.go -- the exact native enum and union wording.

package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestDetachedSetRefusalMatchesTheNativeCommand compares the actual root
// writer and daemon command, including the validator's single complete fix.
func TestDetachedSetRefusalMatchesTheNativeCommand(t *testing.T) {
	server := utilityServer(t)
	for _, path := range [][]string{
		{"bgp", "peer", "peer1", "session", "capability", "route-refresh", "requre"},
		{"bgp", "peer", "peer1", "connection", "local", "ip", "autoo"},
	} {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			file := writeTestConfig(t, setSecretConfig)
			code, diagnostic := captureStderr(t, func() int {
				return cmdSet(append([]string{file}, path...))
			})
			require.Equal(t, exitError, code)
			pair := suppliedConfigPair(t, setSecretConfig, "set "+strings.Join(path, " "))
			response, err := utilityCall(t, server, t.Context(), "show config supplied set"+utilityContent(pair))
			var refused *plugin.RefusalError
			require.ErrorAs(t, err, &refused)
			require.NotNil(t, response)
			require.Equal(t, plugin.StatusError, response.Status)
			require.Empty(t, response.Data)
			line, found := plugin.RefusalLine(err)
			require.True(t, found)
			require.Equal(t, strings.TrimSuffix(diagnostic, "\n"), line)
			require.Equal(t, 1, strings.Count(line, "\nfix: "))
			require.Equal(t, strings.TrimPrefix(line, "error: "), response.Error)
			unchanged, readErr := os.ReadFile(file)
			require.NoError(t, readErr)
			require.Equal(t, setSecretConfig, string(unchanged))
		})
	}
}

// TestDetachedSetRefusalMasksEveryNativePart puts the refused private operand
// inside the schema's accepted values, so the native why and fix need masking
// as well as the problem. A valid enum control must still produce a document.
func TestDetachedSetRefusalMasksEveryNativePart(t *testing.T) {
	t.Cleanup(configyang.RegisterModuleForTest("ze-detached-refusal-probe-conf.yang", `module ze-detached-refusal-probe-conf {
  yang-version 1.1;
  namespace "urn:ze:detached-refusal-probe";
  prefix probe;
  import ze-extensions { prefix ze; }
  container detached-refusal-probe {
    leaf credential {
      type enumeration { enum private-operand-approved; enum other; }
      ze:sensitive;
    }
  }
}`))
	const original = validConfig + "\ndetached-refusal-probe { credential other; }\n"
	const operand = "private-operand"
	file := writeTestConfig(t, original)
	code, diagnostic := captureStderr(t, func() int {
		return cmdSet([]string{file, "detached-refusal-probe", "credential", operand})
	})
	require.Equal(t, exitError, code)
	server := utilityServer(t)
	pair := suppliedConfigPair(t, original, "set detached-refusal-probe credential "+operand)
	response, err := utilityCall(t, server, t.Context(), "show config supplied set"+utilityContent(pair))
	var refused *plugin.RefusalError
	require.ErrorAs(t, err, &refused)
	require.NotNil(t, response)
	require.Equal(t, plugin.StatusError, response.Status)
	require.Empty(t, response.Data)
	line, found := plugin.RefusalLine(err)
	require.True(t, found)
	require.Equal(t, strings.TrimSuffix(diagnostic, "\n"), line)
	require.Equal(t, 1, strings.Count(line, "\nfix: "))
	for _, text := range []string{diagnostic, line, response.Error, err.Error()} {
		require.NotContains(t, text, operand)
		require.Contains(t, text, config.SecretDataPlaceholder)
		require.Contains(t, text, "credential accepts only these values:")
		require.Contains(t, text, "other")
	}
	unchanged, readErr := os.ReadFile(file)
	require.NoError(t, readErr)
	require.Equal(t, original, string(unchanged))
	pair = suppliedConfigPair(t, original, "set detached-refusal-probe credential other")
	response, err = utilityCall(t, server, t.Context(), "show config supplied set"+utilityContent(pair))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	require.NotNil(t, response.Data)
}
