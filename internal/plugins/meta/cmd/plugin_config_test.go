// Design: docs/architecture/api/commands.md -- unit tests for bgp plugin format handler.
// Related: plugin_config.go -- handleBgpPluginFormat.

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestHandleBgpPluginFormatBase64 proves `bgp plugin format base64` sets the
// process format to `plugin.FormatBase64`, preserves the existing valid formats,
// and rejects missing or unknown formats while naming `base64` in the error.
//
// VALIDATES: AC-4 -- `bgp plugin format base64` sets `Process.Format() == plugin.FormatBase64`.
// PREVENTS: runtime `bgp plugin format` handler rejecting the `base64` format token
// or omitting `base64` from its valid-values error text.
func TestHandleBgpPluginFormatBase64(t *testing.T) {
	proc := process.NewProcess(plugin.PluginConfig{Name: "exabgp-test"})
	ctx := &pluginserver.CommandContext{Process: proc}

	for _, want := range []string{
		plugin.FormatBase64,
		plugin.FormatHex,
		plugin.FormatParsed,
		plugin.FormatFull,
	} {
		t.Run("accept-"+want, func(t *testing.T) {
			validated, err := command.ValidateArgs([]string{want}, nil, nil)
			require.NoError(t, err)

			resp, err := handleBgpPluginFormat(ctx, validated)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusDone, resp.Status)

			data, ok := resp.Data.(plugin.Map)
			require.True(t, ok, "response data is %T, want plugin.Map", resp.Data)
			assert.Equal(t, want, data["format"])
			assert.Equal(t, want, proc.Format())
		})
	}

	t.Run("reject-missing-format", func(t *testing.T) {
		validated, err := command.ValidateArgs(nil, nil, nil)
		require.NoError(t, err)

		_, err = handleBgpPluginFormat(ctx, validated)
		require.Error(t, err)
		assert.Contains(t, err.Error(), plugin.FormatBase64)
	})

	t.Run("reject-unknown-format", func(t *testing.T) {
		validated, err := command.ValidateArgs([]string{"unknown-fmt"}, nil, nil)
		require.NoError(t, err)

		_, err = handleBgpPluginFormat(ctx, validated)
		require.Error(t, err)
		assert.Contains(t, err.Error(), plugin.FormatBase64)
	})
}
