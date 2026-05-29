// Design: plan/spec-mpls-1-kernel.md -- `show mpls forwarding` handler tests
package show

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

func TestShowMPLSForwardingArgs(t *testing.T) {
	t.Run("no args returns done envelope", func(t *testing.T) {
		resp, err := handleShowMPLSForwarding(nil, nil)
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusDone, resp.Status)
	})

	t.Run("--limit without value rejects", func(t *testing.T) {
		resp, err := handleShowMPLSForwarding(nil, []string{"--limit"})
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "--limit requires a value")
	})

	t.Run("non-numeric --limit rejects", func(t *testing.T) {
		resp, err := handleShowMPLSForwarding(nil, []string{"--limit", "abc"})
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "invalid --limit")
	})

	t.Run("zero --limit rejects", func(t *testing.T) {
		resp, err := handleShowMPLSForwarding(nil, []string{"--limit", "0"})
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
	})

	t.Run("unexpected positional rejects with usage", func(t *testing.T) {
		resp, err := handleShowMPLSForwarding(nil, []string{"bogus"})
		require.NoError(t, err)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "show mpls forwarding")
	})
}
