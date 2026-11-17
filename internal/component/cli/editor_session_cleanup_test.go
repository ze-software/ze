// Design: docs/guide/config-editor.md -- per-session candidate publication.
// Related: editor_draft.go -- SaveDraft consumes only the saving session.
// Related: editor_commit.go -- direct and accepted-candidate cleanup.

package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestSaveDraftPreservesOtherSessionIntent proves saving one session does
// not consume a previous session's leaf or copy operation in the same user's
// change file. Different fixed timestamps keep two web identities distinct.
func TestSaveDraftPreservesOtherSessionIntent(t *testing.T) {
	fixture := newSessionCleanupFixture(t, "web")
	committed, err := fixture.store.ReadFile(fixture.path)
	require.NoError(t, err)
	require.NoError(t, fixture.current.SaveDraft())
	fixture.checkOther(t)
	metadata, _, err := fixture.readChanges()
	require.NoError(t, err)
	assert.Empty(t, metadata.SessionEntries(fixture.current.schema, fixture.current.SessionID()))
	unchanged, err := fixture.store.ReadFile(fixture.path)
	require.NoError(t, err)
	assert.Equal(t, committed, unchanged, "saving a draft does not publish")
}

// TestCommitSessionPreservesOtherOriginIntent drives both existing commit
// paths with a current web leaf and a same-user SSH leaf plus structural copy.
// A successful commit applies only the web change and retains the SSH intent
// for its original owner, including its original metadata and operation.
func TestCommitSessionPreservesOtherOriginIntent(t *testing.T) {
	for _, route := range []string{"direct", "candidate"} {
		t.Run(route, func(t *testing.T) {
			fixture := newSessionCleanupFixture(t, "ssh")
			if route == "direct" {
				result, err := fixture.current.CommitSession()
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Empty(t, result.Conflicts)
				assert.Equal(t, 1, result.Applied)
			} else {
				result, content, err := fixture.current.CommitSessionCandidate(time.Now())
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Empty(t, result.Conflicts)
				assert.Equal(t, 1, result.Applied)
				require.NoError(t, storage.PromoteCandidate(fixture.store, fixture.path))
				require.NoError(t, fixture.current.MarkCommittedContent(content))
			}
			fixture.checkOther(t)
			committed, err := fixture.store.ReadFile(fixture.path)
			require.NoError(t, err)
			tree, _, err := parseConfigWithFormat(string(committed), fixture.current.schema)
			require.NoError(t, err)
			system := tree.GetContainer("system")
			require.NotNil(t, system)
			host, exists := system.Get("host")
			require.True(t, exists)
			assert.Equal(t, "web-committed", host)
			groups := tree.GetContainer("bgp")
			require.NotNil(t, groups)
			assert.Equal(t, []string{"other-source"}, groups.ListKeys("group"))
			original := groups.GetList("group")["other-source"]
			require.NotNil(t, original)
			value, exists := original.Get("description")
			require.True(t, exists)
			assert.Equal(t, "committed-original", value)
		})
	}
}

type sessionCleanupFixture struct {
	store        storage.Storage
	path         string
	current      *Editor
	otherSession string
	otherEntries []config.SessionEntry
	otherOps     []config.StructuralOp
	otherBytes   []byte
}

func newSessionCleanupFixture(t *testing.T, origin string) *sessionCleanupFixture {
	t.Helper()
	path := writeTestConfig(t, "system { host original; }\n"+
		"bgp { router-id 192.0.2.1; session { asn { local 65000; } } "+
		"group other-source { description committed-original; } }\n")
	fixture := &sessionCleanupFixture{path: path, store: newTestTreeStore(t, path)}
	other := fixture.owner(t, origin, 1)
	require.NoError(t, other.SetValue([]string{"bgp", "group", "other-source"}, "description", "other-pending"))
	require.NoError(t, other.CopyListEntry([]string{"bgp"}, "group", "other-source", []string{"other-copy"}))
	fixture.otherSession = other.SessionID()
	otherBytes, err := fixture.store.ReadFile(ChangePath(path, "alice"))
	require.NoError(t, err)
	fixture.otherBytes = otherBytes
	fixture.current = fixture.owner(t, "web", 2)
	require.NoError(t, fixture.current.SetValue([]string{"system"}, "host", "web-committed"))
	metadata, ops, err := fixture.readChanges()
	require.NoError(t, err)
	fixture.otherEntries = metadata.SessionEntries(fixture.current.schema, fixture.otherSession)
	fixture.otherOps = filterStructuralOps(ops, fixture.otherSession)
	require.NotEmpty(t, fixture.otherEntries)
	require.Len(t, fixture.otherOps, 1)
	return fixture
}

func (f *sessionCleanupFixture) owner(t *testing.T, origin string, second int) *Editor {
	t.Helper()
	editor, err := NewEditorWithStorage(f.store, f.path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, editor.Close()) })
	started := time.Date(2000, 1, 1, 0, 0, second, 0, time.UTC)
	editor.SetSession(&EditSession{User: "alice", Origin: origin, StartTime: started,
		ID: "alice@" + origin + "%" + started.Format(time.RFC3339)})
	return editor
}

func (f *sessionCleanupFixture) checkOther(t *testing.T) {
	t.Helper()
	metadata, ops, err := f.readChanges()
	require.NoError(t, err, "the other session's durable intent must remain readable")
	assert.ElementsMatch(t, f.otherEntries, metadata.SessionEntries(f.current.schema, f.otherSession))
	assert.Equal(t, f.otherOps, filterStructuralOps(ops, f.otherSession))
	data, err := f.store.ReadFile(ChangePath(f.path, "alice"))
	require.NoError(t, err)
	assert.Equal(t, f.otherBytes, data, "only the consumed session leaves the per-user file")
}

func (f *sessionCleanupFixture) readChanges() (*config.MetaTree, []config.StructuralOp, error) {
	data, err := f.store.ReadFile(ChangePath(f.path, "alice"))
	if err != nil {
		return nil, nil, err
	}
	_, meta, ops, err := config.ParseCandidateChangeFile(string(data), f.current.schema)
	return meta, ops, err
}
