package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDiscardPathWithoutSessionKeepsOtherChanges proves `discard <path>` in the
// editor without a session drops only the changes under that path. The command
// once ignored its path there and reverted every pending change, while the
// help offered `discard <path>` as a separate form.
//
// Method: two changes are made, one inside and one outside the discarded path,
// the command goes through the model's dispatcher as typed, and the tree is read
// back: the change inside is the committed value again, the change outside
// stays, and the editor still holds unsaved changes.
func TestDiscardPathWithoutSessionKeepsOtherChanges(t *testing.T) {
	// The inside leaf is changed and must read its committed value after the
	// discard.
	tests := []struct {
		name            string
		command         string
		insidePath      []string
		insideKey       string
		insideNew       string
		insideCommitted string
	}{
		{
			name:            "a leaf",
			command:         "discard bgp router-id",
			insidePath:      []string{"bgp"},
			insideKey:       "router-id",
			insideNew:       "9.9.9.9",
			insideCommitted: "1.2.3.4",
		},
		{
			name:            "a container",
			command:         "discard bgp session",
			insidePath:      []string{"bgp", "session", "asn"},
			insideKey:       "local",
			insideNew:       "65009",
			insideCommitted: "65000",
		},
		{
			name:            "a list entry",
			command:         "discard bgp peer peer1",
			insidePath:      []string{"bgp", "peer", "peer1", "connection", "remote"},
			insideKey:       "ip",
			insideNew:       "2.2.2.2",
			insideCommitted: "1.1.1.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := writeTestConfig(t, validBGPConfig)
			ed, err := NewLooseFileEditor(nil, configPath)
			require.NoError(t, err)
			defer ed.Close() //nolint:errcheck // test cleanup
			require.False(t, ed.HasSession(), "the case is the editor without a session")

			require.NoError(t, ed.SetValue(tt.insidePath, tt.insideKey, tt.insideNew))
			outsidePath, outsideKey, outsideNew := []string{"bgp"}, "router-id", "5.5.5.5"
			if tt.insideKey == "router-id" {
				outsidePath, outsideKey, outsideNew = []string{"bgp", "session", "asn"}, "local", "65009"
			}
			require.NoError(t, ed.SetValue(outsidePath, outsideKey, outsideNew))

			m := Model{editor: ed}
			result, err := m.dispatchCommand(tt.command)
			require.NoError(t, err)
			assert.Contains(t, result.statusMessage, "Discarded: ")

			inside := ed.WalkPath(tt.insidePath)
			require.NotNil(t, inside, "the committed element is back in the tree")
			got, ok := inside.Get(tt.insideKey)
			assert.True(t, ok)
			assert.Equal(t, tt.insideCommitted, got, "the change under the discarded path is dropped")

			outside := ed.WalkPath(outsidePath)
			require.NotNil(t, outside)
			kept, ok := outside.Get(outsideKey)
			assert.True(t, ok)
			assert.Equal(t, outsideNew, kept, "the change outside the discarded path stays")
			assert.True(t, ed.Dirty(), "a change is still pending")
		})
	}
}

// TestDiscardPathWithoutSessionOfTheOnlyChangeIsClean proves the dirty flag
// reports what remains: discarding the path of the only pending change leaves
// nothing to save, as the session form's `discard <path>` does.
func TestDiscardPathWithoutSessionOfTheOnlyChangeIsClean(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewLooseFileEditor(nil, configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	require.NoError(t, ed.SetValue([]string{"bgp", "session", "asn"}, "local", "65009"))
	require.True(t, ed.Dirty())

	m := Model{editor: ed}
	_, err = m.dispatchCommand("discard bgp session asn local")
	require.NoError(t, err)

	asn, _ := ed.WalkPath([]string{"bgp", "session", "asn"}).Get("local")
	assert.Equal(t, "65000", asn)
	assert.False(t, ed.Dirty(), "no change remains after the only one is discarded")
}

// TestDiscardAllWithoutSessionDropsEveryChange proves the other form still
// drops everything, so the fix above did not move `discard all`.
func TestDiscardAllWithoutSessionDropsEveryChange(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	ed, err := NewLooseFileEditor(nil, configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))
	require.NoError(t, ed.SetValue([]string{"bgp", "session", "asn"}, "local", "65009"))

	m := Model{editor: ed}
	_, err = m.dispatchCommand("discard all")
	require.NoError(t, err)

	rid, _ := ed.WalkPath([]string{"bgp"}).Get("router-id")
	assert.Equal(t, "1.2.3.4", rid)
	asn, _ := ed.WalkPath([]string{"bgp", "session", "asn"}).Get("local")
	assert.Equal(t, "65000", asn)
	assert.False(t, ed.Dirty())
}
