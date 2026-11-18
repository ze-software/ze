// Design: docs/architecture/web-interface.md -- browser review shares editor intent.
package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestEditorManagerComparePendingParity sends the default show comparison
// through the real TUI producer and compares it with graphical commit review.
// A second equal sibling and another user's edit must not enter Alice's diff.
func TestEditorManagerComparePendingParity(t *testing.T) {
	seed := `bgp {
    router-id 1.2.3.4
    session { asn { local 65000; } }
    peer alpha { connection { remote { ip 192.0.2.1; } } }
    peer beta { connection { remote { ip 192.0.2.1; } } }
}`
	path := filepath.Join(t.TempDir(), "test.conf")
	require.NoError(t, os.WriteFile(path, []byte(seed), 0o600))
	store := testConfigStore(t, path)
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	mgr := NewEditorManager(store, path, schema, testEditorFactory(), testEditSessionFactory())
	t.Cleanup(func() { require.NoError(t, mgr.Close()) })
	require.NoError(t, mgr.SetValue("alice", []string{"bgp", "peer", "alpha", "connection", "remote"}, "ip", "192.0.2.10"))
	require.NoError(t, mgr.SetValue("bob", []string{"bgp", "peer", "beta", "connection", "remote"}, "ip", "192.0.2.20"))

	want := "- bgp peer alpha connection remote ip 192.0.2.1\n+ bgp peer alpha connection remote ip 192.0.2.10\n"
	review, err := mgr.Diff("alice")
	require.NoError(t, err)
	assert.Equal(t, want, review, "graphical review must retain both addressed sides")
	answer, err := mgr.ConfigCommand("alice", contract.ConfigCommandRequest{Line: "show | compare"})
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(review), strings.TrimSpace(answer.Output), "SSH producer and graphical review must share the pending renderer")
	assert.NotContains(t, answer.Output, "peer beta")
	assert.NotContains(t, answer.Output, "192.0.2.20")
	absent, err := mgr.Diff("carol")
	require.NoError(t, err)
	assert.Empty(t, absent, "an absent session has no intent")
}

// TestEditorManagerPendingReviewUsesCurrentCommitted publishes a newer
// confirmed config without touching Alice's pending program. Both review
// surfaces must compare the candidate with that confirmed value and leave
// the original metadata available for stale-conflict detection.
func TestEditorManagerPendingReviewUsesCurrentCommitted(t *testing.T) {
	mgr := newTestEditorManager(t)
	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "192.0.2.10"))
	us, err := mgr.GetOrCreate("alice")
	require.NoError(t, err)
	before := us.editor.PendingChanges(us.editor.SessionID())
	require.Len(t, before, 1)
	require.Equal(t, "1.2.3.4", before[0].Previous)
	committed := mgr.Tree("alice").Clone()
	bgp := committed.GetContainer("bgp")
	require.NotNil(t, bgp)
	bgp.Set("router-id", "192.0.2.20")
	_, err = storage.WriteCandidateVersion(mgr.store, mgr.configPath, []byte(config.Serialize(committed, mgr.schema)), time.Now())
	require.NoError(t, err)
	require.NoError(t, storage.PromoteCandidate(mgr.store, mgr.configPath))

	want := "- bgp router-id 192.0.2.20\n+ bgp router-id 192.0.2.10\n"
	review, err := mgr.Diff("alice")
	require.NoError(t, err)
	assert.Equal(t, want, review)
	answer, err := mgr.ConfigCommand("alice", contract.ConfigCommandRequest{Line: "show | compare"})
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(want), strings.TrimSpace(answer.Output))
	assert.Equal(t, before, us.editor.PendingChanges(us.editor.SessionID()), "comparison must not rewrite conflict ownership")
}

// TestEditorManagerPendingSecretParity rotates an actual schema-marked token.
// Path-qualified review must remain useful even though masking makes its two
// values identical, and the SSH producer must publish the same safe rows.
func TestEditorManagerPendingSecretParity(t *testing.T) {
	fixture, _ := secretSchemaAndTree(true)
	mgr := secretTransportManager(t, registerSecretTransportSchema(t, fixture))
	require.NoError(t, mgr.SetValue(secretUser, []string{"web-render-secret", "api-server"}, "token", replacementSecret))
	review, err := mgr.Diff(secretUser)
	require.NoError(t, err)
	assert.Contains(t, review, "- web-render-secret api-server token "+config.SecretDataPlaceholder)
	assert.Contains(t, review, "+ web-render-secret api-server token "+config.SecretDataPlaceholder)
	answer, err := mgr.ConfigCommand(secretUser, contract.ConfigCommandRequest{Line: "show | compare"})
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(review), strings.TrimSpace(answer.Output))
	for _, value := range secretValues {
		assert.NotContains(t, review, value)
		assert.NotContains(t, answer.Output, value)
	}
}
