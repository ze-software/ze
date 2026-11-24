// Design: docs/architecture/storage-backends.md — fail-closed stored editor parsing.
// Related: editor.go.

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewEditorWithStorageRejectsCorruptConfig verifies that storage-backed
// editor constructors return the parse error on corrupt configuration input
// instead of silently substituting an empty tree with a nil error.
//
// VALIDATES: NewEditorWithStorage, NewIsolatedEditorWithStorage, and
// NewOfflineSessionEditor fail closed when stored config cannot parse.
// PREVENTS: Silent zero-value empty tree hiding a corrupt stored configuration.
func TestNewEditorWithStorageRejectsCorruptConfig(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{
			name:    "unclosed block",
			content: "bgp {\n\trouter-id 1.2.3.4;\n",
		},
		{
			name:    "invalid numeric peer key",
			content: "bgp {\n\trouter-id 1.2.3.4;\n\tpeer 192.0.2.7 {\n\t\tsession { asn { local 65000; remote 65001; } }\n\t}\n}\n",
		},
		{
			name:    "garbled syntax",
			content: "this is not { valid } config syntax !!!\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTestConfig(t, tc.content)
			store := newTestTreeStore(t, path)

			ed, err := NewEditorWithStorage(store, path)
			require.Error(t, err, "NewEditorWithStorage must return parse error")
			assert.Nil(t, ed)

			isolated, err := NewIsolatedEditorWithStorage(store, path)
			require.Error(t, err, "NewIsolatedEditorWithStorage must return parse error")
			assert.Nil(t, isolated)

			offline, err := NewOfflineSessionEditor(store, path, "alice")
			require.Error(t, err, "NewOfflineSessionEditor must return parse error")
			assert.Nil(t, offline)
		})
	}

	// Positive control: a valid stored configuration still opens with a populated tree.
	t.Run("valid config opens", func(t *testing.T) {
		path := writeTestConfig(t, validBGPConfig)
		store := newTestTreeStore(t, path)

		ed, err := NewEditorWithStorage(store, path)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, ed.Close()) })
		require.NotNil(t, ed.Tree().GetContainer("bgp"))
		assert.True(t, ed.treeValid)
	})
}
