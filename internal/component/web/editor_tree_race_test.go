// Design: docs/architecture/web-interface.md -- Per-User Editor lock safety.
// Related: editor.go -- ReadTree and snapshotTree synchronization.

package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestEditorManagerConcurrentReadAndMutation exercises concurrent same-user
// reads and mutations under -race across snapshotTree, ReadTree, and HTTP
// handlers that read the user's working tree.
//
// VALIDATES: concurrent reads and mutations for the same authenticated user
// serialize cleanly with zero data races and snapshotTree returns an isolated
// copy unaffected by later mutations.
// PREVENTS: returning a live unlocked *config.Tree pointer from EditorManager
// (the retired Tree method) that races with concurrent edits from another tab.
func TestEditorManagerConcurrentReadAndMutation(t *testing.T) {
	mgr := newTestEditorManager(t)
	schema := mgr.schema
	renderer, err := NewRenderer()
	require.NoError(t, err)

	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.1"))
	require.NoError(t, mgr.CreateEntry("alice", []string{"bgp", "group", "edge"}))

	// Snapshot isolation: mutating alice's session must not alter a prior snapshot.
	snapBefore := mgr.snapshotTree("alice")
	require.NotNil(t, snapBefore)
	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.2"))
	gotBefore, ok := snapBefore.GetContainer("bgp").Get("router-id")
	require.True(t, ok)
	assert.Equal(t, "10.0.0.1", gotBefore, "snapshot must stay isolated from subsequent SetValue calls")

	modeHandler := HandleCLIModeToggle(mgr, schema, renderer)
	addFormHandler := HandleConfigAddForm(mgr, schema, renderer)

	const iterations = 40
	var wg sync.WaitGroup
	wg.Add(4)

	// Writer: mutates alice's working tree (leaves and list entries).
	go func() {
		defer wg.Done()
		var tb textbuf.Buffer
		for i := range iterations {
			rid := tb.Reset().Str("10.0.1.").Int(int64(i%200 + 1)).String()
			_ = mgr.SetValue("alice", []string{"bgp"}, "router-id", rid)
			desc := tb.Reset().Str("desc-").Int(int64(i)).String()
			_ = mgr.SetValue("alice", []string{"bgp", "group", "edge"}, "description", desc)
		}
	}()

	// Reader 1: clones via snapshotTree and serializes / walks the snapshot.
	go func() {
		defer wg.Done()
		for range iterations {
			tree := mgr.snapshotTree("alice")
			if tree != nil {
				_ = config.Serialize(tree, schema)
				_ = walkTree(tree, schema, []string{"bgp", "group", "edge"})
			}
		}
	}()

	// Reader 2: reads under ReadTree without copying.
	go func() {
		defer wg.Done()
		for range iterations {
			mgr.ReadTree("alice", func(tree *config.Tree) {
				if tree != nil {
					_ = config.Serialize(tree, schema)
				}
			})
		}
	}()

	// Reader 3: exercises HTTP handlers that previously read mgr.Tree directly.
	go func() {
		defer wg.Done()
		for range iterations {
			form := url.Values{"mode": {"integrated"}, "path": {"bgp"}}
			modeReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/cli/mode", strings.NewReader(form.Encode()))
			modeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			modeReq = modeReq.WithContext(withUsername(modeReq.Context(), "alice"))
			modeRec := httptest.NewRecorder()
			modeHandler(modeRec, modeReq)

			addReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/config/add-form/bgp/peer/", http.NoBody)
			addReq = addReq.WithContext(withUsername(addReq.Context(), "alice"))
			addRec := httptest.NewRecorder()
			addFormHandler(addRec, addReq)
		}
	}()

	wg.Wait()

	final := mgr.snapshotTree("alice")
	require.NotNil(t, final)
	_, ok = final.GetContainer("bgp").Get("router-id")
	assert.True(t, ok)
}
