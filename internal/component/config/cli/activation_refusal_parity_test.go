// Design: docs/architecture/api/commands.md -- shared activation refusal producers.
package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestDetachedActivationRefusalKeepsProducerContext checks the real root and
// supplied daemon routes for parse and activation failures. The parser's line
// and load context, or the activation verb, must survive beside one native fix.
func TestDetachedActivationRefusalKeepsProducerContext(t *testing.T) {
	server := utilityServer(t)
	for _, activate := range []bool{false, true} {
		verb := "deactivate"
		if activate {
			verb = "activate"
		}
		for _, test := range []struct {
			name    string
			source  string
			path    []string
			context string
			fix     string
		}{
			{name: "unfinished source", source: "bgp {\n", path: []string{"bgp"}, context: "load:"},
			{name: "native parser correction", source: "}\n", path: []string{"bgp"},
				context: "load: line 1: this '}' closes no section", fix: "delete this '}'"},
			{name: "missing node", source: validConfig, path: []string{"bgp", "no-such-node"}, context: verb + " failed:"},
		} {
			t.Run(verb+"/"+test.name, func(t *testing.T) {
				file := writeTestConfig(t, test.source)
				code, diagnostic := captureStderr(t, func() int {
					return runDeactivateLike(nil, append([]string{file}, test.path...), activate)
				})
				require.Equal(t, exitError, code)
				pair := suppliedConfigPair(t, test.source, verb+" "+strings.Join(test.path, " "))
				response, err := utilityCall(t, server, t.Context(), "show config supplied "+verb+utilityContent(pair))
				var refused *plugin.RefusalError
				require.ErrorAs(t, err, &refused)
				require.NotNil(t, response)
				require.Equal(t, plugin.StatusError, response.Status)
				require.Empty(t, response.Data)
				line, found := plugin.RefusalLine(err)
				require.True(t, found)
				require.Equal(t, strings.TrimSuffix(diagnostic, "\n"), line)
				require.Contains(t, line, test.context)
				require.Equal(t, 1, strings.Count(line, "\nfix: "))
				if test.fix != "" {
					require.True(t, strings.HasSuffix(line, "\nfix: "+test.fix), line)
				}
				unchanged, readErr := os.ReadFile(file)
				require.NoError(t, readErr)
				require.Equal(t, test.source, string(unchanged))
			})
		}
		before := validConfig
		if activate {
			before = "inactive: " + before
		}
		pair := suppliedConfigPair(t, before, verb+" bgp")
		response, err := utilityCall(t, server, t.Context(), "show config supplied "+verb+utilityContent(pair))
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, response.Status)
		require.NotEmpty(t, response.Data)
	}
}
