// VALIDATES: spec-session-editor-file-mode-parity AC-1, AC-3, AC-4, AC-5 at the
// editor: a session load records the difference as per-leaf change entries,
// deletes for a replace, and nothing for refused input.
// PREVENTS: a session load that bypasses write-through, records unchanged
// leaves, drops the deletes a replace implies, or writes half a load.

package cli

import (
	"bytes"
	"errors"
	"io/fs"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// loadSessionTwoPeers is validBGPConfig with a second peer, so a replace has a
// list entry to delete.
const loadSessionTwoPeers = `bgp {
	router-id 1.2.3.4
	session {
		asn {
			local 65000
		}
	}
	peer peer1 {
		connection {
			remote {
				ip 1.1.1.1
			}
		}
		session {
			asn {
				remote 65001
			}
		}
		timer { receive-hold-time 90; }
	}
	peer peer2 {
		connection {
			remote {
				ip 2.2.2.2
			}
		}
		session {
			asn {
				remote 65002
			}
		}
	}
}
`

// newLoadSessionEditor opens a session editor over content for user thomas.
func newLoadSessionEditor(t *testing.T, content string) (*Editor, *EditSession, string) {
	t.Helper()
	configPath := writeTestConfig(t, content)
	store := newTestTreeStore(t, configPath)
	ed, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck,gosec // Best effort cleanup
	session := NewEditSession("thomas", "ssh")
	ed.SetSession(session)
	return ed, session, configPath
}

// parseLoadInput parses hierarchical input rooted at the schema root.
func parseLoadInput(t *testing.T, ed *Editor, input string) *config.Tree {
	t.Helper()
	tree, err := config.NewParser(ed.schema).Parse(input)
	require.NoError(t, err)
	return tree
}

// committedLoadTree reopens the committed config.
func committedLoadTree(t *testing.T, ed *Editor, configPath string) *config.Tree {
	t.Helper()
	committed, err := NewEditorWithStorage(ed.store, configPath)
	require.NoError(t, err)
	t.Cleanup(func() { committed.Close() }) //nolint:errcheck,gosec // Best effort cleanup
	return committed.tree
}

// TestSessionLoadMergeEmitsPerLeafEntries verifies AC-1 and R-6 at the editor:
// a merge records one set per added or changed leaf, none for a leaf equal to
// the session value, and the commit applies them.
func TestSessionLoadMergeEmitsPerLeafEntries(t *testing.T) {
	ed, session, configPath := newLoadSessionEditor(t, validBGPConfig)
	loaded := parseLoadInput(t, ed, `bgp {
	router-id 5.6.7.8
	session { asn { local 65000; } }
	peer peer1 { timer { receive-hold-time 120; } }
}`)

	require.NoError(t, ed.LoadMerge(nil, loaded))

	assert.ElementsMatch(t, []string{
		"set bgp router-id",
		"set bgp peer peer1 timer receive-hold-time",
	}, pendingKinds(ed, session.ID))

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)
	bgp := committedLoadTree(t, ed, configPath).GetContainer("bgp")
	routerID, _ := bgp.Get("router-id")
	assert.Equal(t, "5.6.7.8", routerID)
	hold, _ := bgp.GetList("peer")["peer1"].GetContainer("timer").Get("receive-hold-time")
	assert.Equal(t, "120", hold)
}

// TestSessionLoadReplaceEmitsDeletes verifies AC-3 and AC-4 at the editor: an
// absolute replace records a delete for an omitted leaf and a delete-entry for
// an omitted list entry; a replace at a context path touches nothing outside it.
func TestSessionLoadReplaceEmitsDeletes(t *testing.T) {
	ed, session, configPath := newLoadSessionEditor(t, loadSessionTwoPeers)
	loaded := parseLoadInput(t, ed, validBGPConfig)
	peer1 := loaded.GetContainer("bgp").GetList("peer")["peer1"]
	peer1.RemoveContainer("timer")

	require.NoError(t, ed.LoadReplace(nil, loaded))

	assert.ElementsMatch(t, []string{
		"delete bgp peer peer1 timer",
		"delete bgp peer peer2",
	}, pendingKinds(ed, session.ID))

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)
	peers := committedLoadTree(t, ed, configPath).GetContainer("bgp").GetList("peer")
	assert.Nil(t, peers["peer2"])
	assert.Nil(t, peers["peer1"].GetContainer("timer"), "the omitted timer container is deleted")

	// AC-4: replace at bgp peer peer1 with a body that changes the remote ip
	// and omits the session container; router-id outside it is untouched.
	context := []string{"bgp", "peer", "peer1"}
	body := parseLoadInput(t, ed, `bgp { peer peer1 { connection { remote { ip 9.9.9.9; } } } }`).
		GetContainer("bgp").GetList("peer")["peer1"]

	require.NoError(t, ed.LoadReplace(context, body))

	assert.ElementsMatch(t, []string{
		"set bgp peer peer1 connection remote ip",
		"delete bgp peer peer1 session",
	}, pendingKinds(ed, session.ID))
	routerID, _ := ed.tree.GetContainer("bgp").Get("router-id")
	assert.Equal(t, "1.2.3.4", routerID)
}

// TestSessionLoadRefusesBadInputAtomically verifies AC-5 and R-4 at the editor:
// a load whose tree the schema refuses at the context writes nothing.
func TestSessionLoadRefusesBadInputAtomically(t *testing.T) {
	ed, session, configPath := newLoadSessionEditor(t, validBGPConfig)
	before, readErr := ed.store.ReadFile(ChangePath(configPath, session.User))
	_ = readErr //nolint:errcheck // no change file yet is the expected state
	loaded := parseLoadInput(t, ed, `bgp { router-id 5.6.7.8; }`)

	err := ed.LoadMerge([]string{"bgp", "nosuch"}, loaded)
	require.Error(t, err)

	after, _ := ed.store.ReadFile(ChangePath(configPath, session.User)) //nolint:errcheck // compared below
	assert.Equal(t, string(before), string(after))
	assert.Empty(t, pendingKinds(ed, session.ID))
	routerID, _ := ed.tree.GetContainer("bgp").Get("router-id")
	assert.Equal(t, "1.2.3.4", routerID)
}

// TestParseAtContext verifies config.Parser.ParseAt, the parse a relative
// load runs: a fragment is checked against the context node's children and
// answered rooted at the context; a keyword the context does not hold and a
// context the schema does not know are refused by name.
func TestParseAtContext(t *testing.T) {
	ed, _, _ := newLoadSessionEditor(t, validBGPConfig)
	context := []string{"bgp", "peer", "peer1"}

	body, err := config.NewParser(ed.schema).ParseAt(`connection { remote { ip 9.9.9.9; } }`, context)
	require.NoError(t, err)
	ip, ok := body.GetContainer("connection").GetContainer("remote").Get("ip")
	require.True(t, ok)
	assert.Equal(t, "9.9.9.9", ip)

	_, err = config.NewParser(ed.schema).ParseAt(`router-id 5.6.7.8;`, context)
	require.Error(t, err, "router-id is a bgp leaf, not a peer leaf")
	assert.Contains(t, err.Error(), "unknown keyword: router-id")

	_, err = config.NewParser(ed.schema).ParseAt(`ip 1.1.1.1;`, []string{"bgp", "nosuch"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown context path: bgp nosuch")
}

// loadTwoEdits is a merge that changes two leaves at two depths, so a load
// that applies them one at a time has a middle to fail in: router-id first,
// because loadTree applies a node's leaves before it descends.
const loadTwoEdits = `bgp { router-id 5.6.7.8; peer peer1 { connection { remote { ip 7.7.7.7; } } } }`

// loadState is what a load must leave unchanged when it fails: the session's
// change file on disk, the editor's tree, and its pending changes.
type loadState struct {
	changeFile string
	tree       string
	pending    []string
	dirty      bool
}

func captureLoadState(t *testing.T, ed *Editor, configPath string, session *EditSession) loadState {
	t.Helper()
	state := loadState{tree: config.Serialize(ed.tree, ed.schema), dirty: ed.Dirty()}
	if session == nil {
		return state
	}
	data, err := ed.store.ReadFile(ChangePath(configPath, session.User))
	if err != nil {
		require.ErrorIs(t, err, fs.ErrNotExist)
	}
	state.changeFile = string(data)
	state.pending = pendingKinds(ed, session.ID)
	return state
}

// errChangeMedium is the write failure failingChangeWriteStore injects.
var errChangeMedium = errors.New("change file medium failure")

// failingChangeWriteStore fails every locked write whose bytes hold marker,
// so a test can fail the write that carries one particular edit of a load.
type failingChangeWriteStore struct {
	storage.Storage
	marker []byte
}

func (s *failingChangeWriteStore) AcquireLock(path string) (storage.WriteGuard, error) {
	guard, err := s.Storage.AcquireLock(path)
	if err != nil {
		return nil, err
	}
	return &failingChangeWriteGuard{WriteGuard: guard, marker: s.marker}, nil
}

type failingChangeWriteGuard struct {
	storage.WriteGuard
	marker []byte
}

func (g *failingChangeWriteGuard) WriteFile(name string, data []byte, mode fs.FileMode) error {
	if bytes.Contains(data, g.marker) {
		return errChangeMedium
	}
	return g.WriteGuard.WriteFile(name, data, mode)
}

// TestLoadRefusedMidwayLeavesCandidateUnchanged verifies that a load is all
// or nothing (R-4): the input carries a valid router-id change, then a remote
// ip holding two values, which load refuses after router-id is applied. The
// session change file, the tree and the pending changes stay as they were, in
// session and in file mode alike, and the error says the candidate is unchanged.
func TestLoadRefusedMidwayLeavesCandidateUnchanged(t *testing.T) {
	for _, mode := range []string{"session", "file"} {
		t.Run(mode, func(t *testing.T) {
			ed, session, configPath := newLoadSessionEditor(t, validBGPConfig)
			if mode == "file" {
				ed.SetSession(nil)
				session = nil
			}
			require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"), "a pending edit the failed load must keep")
			before := captureLoadState(t, ed, configPath, session)

			loaded := parseLoadInput(t, ed, loadTwoEdits)
			remote := loaded.GetContainer("bgp").GetList("peer")["peer1"].GetContainer("connection").GetContainer("remote")
			remote.AddMultiValueMember("ip", "8.8.8.8")

			err := ed.LoadMerge(nil, loaded)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "candidate unchanged")
			assert.Equal(t, before, captureLoadState(t, ed, configPath, session))
		})
	}
}

// TestLoadWriteFailureLeavesCandidateUnchanged verifies that an I/O failure
// part-way through a session load leaves the candidate as it was: the write
// carrying the second edit (the remote ip) fails, and neither the first edit
// nor the second reaches the change file or the tree.
func TestLoadWriteFailureLeavesCandidateUnchanged(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	store := &failingChangeWriteStore{Storage: newTestTreeStore(t, configPath), marker: []byte("7.7.7.7")}
	ed, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck,gosec // Best effort cleanup
	session := NewEditSession("thomas", "ssh")
	ed.SetSession(session)
	before := captureLoadState(t, ed, configPath, session)

	err = ed.LoadMerge(nil, parseLoadInput(t, ed, loadTwoEdits))
	require.ErrorIs(t, err, errChangeMedium)
	assert.Contains(t, err.Error(), "candidate unchanged")
	assert.Equal(t, before, captureLoadState(t, ed, configPath, session))
}

// countingWriteStore counts the locked writes of one name.
type countingWriteStore struct {
	storage.Storage
	name   string
	writes int
}

func (s *countingWriteStore) AcquireLock(path string) (storage.WriteGuard, error) {
	guard, err := s.Storage.AcquireLock(path)
	if err != nil {
		return nil, err
	}
	return &countingWriteGuard{WriteGuard: guard, store: s}, nil
}

type countingWriteGuard struct {
	storage.WriteGuard
	store *countingWriteStore
}

func (g *countingWriteGuard) WriteFile(name string, data []byte, mode fs.FileMode) error {
	if name == g.store.name {
		g.store.writes++
	}
	return g.WriteGuard.WriteFile(name, data, mode)
}

// TestSessionLoadLargeInputOneWrite verifies the write half of A-3: a session
// load of 100 new peers, 400 leaves, records every leaf as its own change
// entry and writes the change file exactly once, under the one lock the load
// holds. The time half is logged, not asserted: each step still re-reads and
// re-serializes the staged change file, so the cost grows with the square of
// the load (1000 peers took about 50s when this was written).
func TestSessionLoadLargeInputOneWrite(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	session := NewEditSession("thomas", "ssh")
	store := &countingWriteStore{Storage: newTestTreeStore(t, configPath), name: ChangePath(configPath, session.User)}
	ed, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck,gosec // Best effort cleanup
	ed.SetSession(session)

	const peers = 100
	var input strings.Builder
	input.WriteString("bgp {\n")
	for i := range peers {
		octet := strconv.Itoa(i%250 + 1)
		third := strconv.Itoa(i/250 + 1)
		input.WriteString("peer big" + strconv.Itoa(i) + " { connection { remote { ip 10." + third + "." + octet + ".1; } local { ip 10." + third + "." + octet + ".2; } } session { asn { remote 65001; } } timer { receive-hold-time 90; } }\n")
	}
	input.WriteString("}\n")

	start := time.Now()
	require.NoError(t, ed.LoadMerge(nil, parseLoadInput(t, ed, input.String())))
	t.Logf("load of %d peers took %s", peers, time.Since(start))

	assert.Equal(t, 1, store.writes, "the change file is written once per load")
	assert.Len(t, ed.PendingChanges(session.ID), peers*4, "one change entry per loaded leaf")
}
