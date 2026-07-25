package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/plugin"
)

// VALIDATES: AC-12 -- show aaa accounting exposes TACACS+ accounting drop count.
// PREVENTS: Accounting queue drops being invisible to operators.
func TestHandleShowAAAAccounting(t *testing.T) {
	aaa.RegisterAAAAccountingProvider(func() map[string]any {
		return map[string]any{"dropped-records": uint64(7)}
	})
	t.Cleanup(func() { aaa.RegisterAAAAccountingProvider(nil) })

	resp, err := handleShowAAAAccounting(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, uint64(7), data["dropped-records"])
}
