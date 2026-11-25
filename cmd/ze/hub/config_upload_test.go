// Design: docs/architecture/appliance/remote-operations.md -- SSH upload validation receipt fix line.
// Related: config_upload.go -- configMutationOwner.Upload and uploadValidationFix.

package hub

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestUploadReceiptNamesTheCommandLinePlugins proves the SSH upload validation
// refusal's fix line carries the recorded global --plugin arguments before
// `config validate <file>`, preserving record order, and stays the exact
// `ze config validate <file>` line when no command-line plugin was recorded.
//
// VALIDATES: AC-18: configMutationOwner.Upload includes each recorded --plugin
// entry in its validation refusal's fix line, leaves problem and why text
// unchanged, and keeps `ze config validate <file>` with an empty record.
// PREVENTS: An operator hitting a daemon-only --plugin refusal on SSH upload
// and being sent to `ze config validate <file>` without the daemon's --plugin
// flags, which would validate against a different plugin set.
func TestUploadReceiptNamesTheCommandLinePlugins(t *testing.T) {
	saved := zeconfig.CommandLinePlugins()
	t.Cleanup(func() { zeconfig.RecordCommandLinePlugins(saved) })

	const path = "router.conf"
	store := storage.BindConfigSource(storage.CreateMemory(), path, storage.ConfigSourceStored)
	t.Cleanup(func() { _ = store.Close() })
	owner := &configMutationOwner{}
	t.Cleanup(owner.close)
	reloads := 0
	require.NoError(t, owner.bind(store, path, nil, func(context.Context) error {
		reloads++
		return nil
	}))

	invalidCandidate := []byte("not-a-valid-root {\n}\n")

	cases := []struct {
		name    string
		plugins []string
		wantFix string
	}{
		{
			name:    "single command-line plugin",
			plugins: []string{"ze.flowspec-firewall"},
			wantFix: "ze --plugin ze.flowspec-firewall config validate <file>",
		},
		{
			name:    "multiple command-line plugins in record order",
			plugins: []string{"ze.flowspec-firewall", "ze.hostname"},
			wantFix: "ze --plugin ze.flowspec-firewall --plugin ze.hostname config validate <file>",
		},
		{
			name:    "no command-line plugins",
			plugins: nil,
			wantFix: "ze config validate <file>",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			zeconfig.RecordCommandLinePlugins(tc.plugins)

			err := owner.Upload(t.Context(), invalidCandidate)
			require.Error(t, err)

			opErr, ok := errors.AsType[*opererr.Error](err)
			require.True(t, ok, "upload validation refusal must be an *opererr.Error: %v", err)
			assert.Equal(t, "configuration upload failed validation", opErr.Problem())
			assert.Equal(t, "The configuration was refused before publication; validate the source locally to inspect its errors", opErr.Why())
			assert.Equal(t, tc.wantFix, opErr.Fix())

			lines := strings.Split(strings.TrimRight(err.Error(), "\n"), "\n")
			assert.Equal(t, "fix: "+tc.wantFix, lines[len(lines)-1])
		})
	}
	assert.Zero(t, reloads, "refused uploads must not invoke reload")
}
