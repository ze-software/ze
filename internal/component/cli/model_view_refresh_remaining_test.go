// Design: docs/architecture/cli/error-surface.md -- editor views and feedback.
// Related: model_view_refresh_test.go -- the original incidental command cases.

package cli

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// incidentalViewProbe gives the peer context both kinds of structural edit:
// a multi-key list, whose complete address reaches createListEntry, and an
// ordered leaf-list. No protocol behavior depends on the probe's contents.
const incidentalViewProbe = `module ze-incidental-view-probe-conf {
  yang-version 1.1;
  namespace "urn:ze:incidental-view-probe";
  prefix probe;
  import ze-bgp-conf { prefix bgp; }
  augment "/bgp:bgp/bgp:peer" {
    list probe {
      key "first second";
      leaf first { type string; }
      leaf second { type string; }
    }
    leaf-list probe-members { type string; ordered-by user; }
  }
}`

// incidentalStoredModel has real config history and per-user change files.
// The normal commit path decides whether the fixture needs a reload callback.
func incidentalStoredModel(t *testing.T, session bool) Model {
	t.Helper()
	f := newWindowFixture(t)
	ed, err := NewEditorWithStorage(f.store, f.configPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ed.Close()) })
	if session {
		ed.SetSession(NewEditSession("alice", "ssh"))
	}
	m, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 14})
	m, ok := sized.(Model)
	require.True(t, ok)
	t.Cleanup(func() { require.NoError(t, m.Close()) })
	m.setViewportData(*m.configViewAtPath(nil))
	return m
}

func setIncidentalTimer(t *testing.T, m *Model, value string) {
	t.Helper()
	require.NoError(t, m.editor.SetValue([]string{"bgp", "peer", "peer1", "timer"}, "receive-hold-time", value))
}

func incidentalPeerLine(m *Model, verb, args string) string {
	if len(m.contextPath) == 0 {
		return verb + " bgp peer peer1 " + args
	}
	return verb + " " + args
}

// assertIncidentalView compares the applied answer with the old copy-built
// view at the position the command ran at. The whole viewport includes its
// validation marks and diff gutter; feedback and history are checked too.
func assertIncidentalView(t *testing.T, before, out Model, result commandResult, line, status string) {
	t.Helper()
	require.NoError(t, out.err, line)
	require.True(t, result.refreshConfig, "the producer must request the current-position view")
	require.Nil(t, result.configView, "the producer must not return a copy-built view")
	assert.Equal(t, before.contextPath, out.contextPath, "the prompt's position")
	assert.Contains(t, out.statusMessage, status)
	assert.Equal(t, result.statusMessage, out.statusMessage)
	if line != "" {
		entries := out.history.Entries()
		require.NotEmpty(t, entries)
		assert.Equal(t, line, entries[len(entries)-1], "history retains the typed command")
	}

	legacy := out
	legacy.setViewportData(*legacy.configViewAtPath(before.contextPath))
	assert.Equal(t, legacy.viewportContent, out.viewportContent, "content, validation marks and diff gutter")
	assert.Equal(t, legacy.viewport.YOffset(), out.viewport.YOffset(), "scroll position")
	assert.Equal(t, legacy.showingConfig, out.showingConfig)
	assert.Equal(t, legacy.feedbackLine(), out.feedbackLine(), "status row")
}

// TestRemainingIncidentalViewsWithoutTop covers every additional synchronous
// refresh producer: structural edits, rollback, both commit modes and reload
// outcomes, discard paths, session visibility and file-mode confirm commands.
// Each runs as a typed line at the root and inside edit bgp peer peer1.
func TestRemainingIncidentalViewsWithoutTop(t *testing.T) {
	t.Cleanup(configyang.RegisterModuleForTest("ze-incidental-view-probe-conf.yang", incidentalViewProbe))
	type scenario struct {
		name    string
		session bool
		prepare func(*testing.T, *Model)
		line    func(*Model) string
		status  string
		check   func(*testing.T, *Model)
	}
	changeTimer := func(t *testing.T, m *Model) { setIncidentalTimer(t, m, "60") }
	commitLine := func(*Model) string { return "commit now" }
	prepareProbe := func(t *testing.T, m *Model) {
		_, err := m.dispatchCommand("set bgp peer peer1 probe old second original")
		require.NoError(t, err)
	}
	setReload := func(fail bool) func(*testing.T, *Model) {
		return func(t *testing.T, m *Model) {
			changeTimer(t, m)
			m.editor.SetReloadNotifier(func() error {
				if fail {
					return errors.New("fixture reload refused")
				}
				return storage.PromoteCandidate(m.editor.store, m.editor.originalPath)
			})
		}
	}
	timerIs := func(value string) func(*testing.T, *Model) {
		return func(t *testing.T, m *Model) {
			peer := m.editor.tree.GetContainer("bgp").GetList("peer")["peer1"]
			require.NotNil(t, peer)
			timer := peer.GetContainer("timer")
			if value == "" {
				if timer != nil {
					_, present := timer.Get("receive-hold-time")
					assert.False(t, present)
				}
				return
			}
			require.NotNil(t, timer)
			got, present := timer.Get("receive-hold-time")
			assert.True(t, present)
			assert.Equal(t, value, got)
		}
	}
	otherSession := func(t *testing.T, m *Model) {
		ed, err := NewEditorWithStorage(m.editor.store, m.editor.originalPath)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, ed.Close()) })
		ed.SetSession(NewEditSession("bob", "ssh"))
		require.NoError(t, ed.SetValue([]string{"bgp", "peer", "peer1", "timer"}, "receive-hold-time", "60"))
		require.Len(t, m.editor.ActiveSessions(), 1)
	}
	confirmed := func(t *testing.T, m *Model) {
		changeTimer(t, m)
		result, err := m.cmdCommitConfirmed(60, false)
		require.NoError(t, err)
		require.True(t, result.refreshConfig)
		m.applyResult(result)
		require.True(t, m.confirmTimerActive)
	}
	cases := []scenario{
		{name: "multi-key entry creation", line: func(m *Model) string {
			return incidentalPeerLine(m, "set", "probe alpha second beta")
		}, status: "created probe", check: func(t *testing.T, m *Model) {
			peer := m.editor.tree.GetContainer("bgp").GetList("peer")["peer1"]
			entries := peer.GetList("probe")
			require.Len(t, entries, 1)
			entry := entries["alpha second beta"]
			require.NotNil(t, entry, "the complete address identifies the created entry")
			second, present := entry.Get("second")
			assert.True(t, present)
			assert.Equal(t, "beta", second)
		}},
		{name: "insert", prepare: func(t *testing.T, m *Model) {
			require.NoError(t, m.editor.SetValue(topPeerContext, "probe-members", "zero"))
		}, line: func(m *Model) string { return incidentalPeerLine(m, "insert", "probe-members one last") }, status: "Inserted one", check: func(t *testing.T, m *Model) {
			peer := m.editor.tree.GetContainer("bgp").GetList("peer")["peer1"]
			assert.Equal(t, []string{"zero", "one"}, peer.GetMultiValues("probe-members"))
		}},
		{name: "rename", prepare: prepareProbe, line: func(m *Model) string {
			return incidentalPeerLine(m, "rename", "probe old second original to new second original")
		}, status: "Renamed probe", check: func(t *testing.T, m *Model) {
			peer := m.editor.tree.GetContainer("bgp").GetList("peer")["peer1"]
			entries := peer.GetList("probe")
			require.Len(t, entries, 1)
			assert.NotContains(t, entries, "old second original")
			entry := entries["new second original"]
			require.NotNil(t, entry, "rename moves the first key and preserves the other key")
			second, present := entry.Get("second")
			assert.True(t, present)
			assert.Equal(t, "original", second)
		}},
		{name: "copy", prepare: prepareProbe, line: func(m *Model) string {
			return incidentalPeerLine(m, "copy", "probe old second original to new second original")
		}, status: "Copied probe", check: func(t *testing.T, m *Model) {
			peer := m.editor.tree.GetContainer("bgp").GetList("peer")["peer1"]
			entries := peer.GetList("probe")
			require.Len(t, entries, 2)
			for _, address := range []string{"old second original", "new second original"} {
				entry := entries[address]
				require.NotNil(t, entry, "copy preserves both complete addresses")
				second, present := entry.Get("second")
				assert.True(t, present)
				assert.Equal(t, "original", second)
			}
		}},
		{name: "file commit without daemon", prepare: changeTimer, line: commitLine, status: "Configuration committed", check: timerIs("60")},
		{name: "file reload accepted", prepare: setReload(false), line: commitLine, status: "Configuration committed and", check: timerIs("60")},
		{name: "file reload refused", prepare: setReload(true), line: commitLine, status: "commit failed: fixture reload refused", check: timerIs("60")},
		{name: "session commit without daemon", session: true, prepare: changeTimer, line: commitLine, status: "Session committed:", check: timerIs("60")},
		{name: "session reload accepted", session: true, prepare: setReload(false), line: commitLine, status: "Session committed:", check: timerIs("60")},
		{name: "session reload refused", session: true, prepare: setReload(true), line: commitLine, status: "commit failed: fixture reload refused", check: timerIs("60")},
		{name: "rollback", prepare: func(t *testing.T, m *Model) {
			setIncidentalTimer(t, m, "60")
			_, err := m.editor.Save()
			require.NoError(t, err)
			setIncidentalTimer(t, m, "90")
			_, err = m.editor.Save()
			require.NoError(t, err)
		}, line: func(*Model) string { return "rollback 1" }, status: "Rolled back to", check: timerIs("60")},
		{name: "discard file path", prepare: changeTimer, line: func(*Model) string {
			return "discard bgp peer peer1 timer receive-hold-time"
		}, status: "Discarded:", check: timerIs("")},
		{name: "discard session path", session: true, prepare: changeTimer, line: func(*Model) string {
			return "discard bgp peer peer1 timer receive-hold-time"
		}, status: "Discarded:", check: timerIs("")},
		{name: "no own changes", session: true, line: func(*Model) string { return "show | changes" }, status: "No pending changes"},
		{name: "no session changes", session: true, line: func(*Model) string { return "show | changes all" }, status: "No pending changes"},
		{name: "other session changes", session: true, prepare: otherSession, line: func(*Model) string { return "show | changes all" }, status: "1 pending change across 1 sessions"},
		{name: "disconnect other session", session: true, prepare: otherSession, line: func(m *Model) string {
			return "disconnect " + m.editor.ActiveSessions()[0]
		}, status: "Disconnected session:", check: func(t *testing.T, m *Model) { assert.Empty(t, m.editor.ActiveSessions()) }},
		{name: "confirmed file commit", prepare: changeTimer, line: func(*Model) string { return "commit confirmed 60" }, status: "Committed", check: func(t *testing.T, m *Model) { assert.True(t, m.confirmTimerActive) }},
		{name: "accept file commit", prepare: confirmed, line: func(*Model) string { return "commit accept" }, status: "accepted", check: func(t *testing.T, m *Model) { assert.False(t, m.confirmTimerActive) }},
		{name: "abort file commit", prepare: confirmed, line: func(*Model) string { return "commit abort" }, status: "rolled back", check: func(t *testing.T, m *Model) {
			assert.False(t, m.confirmTimerActive)
			timerIs("")(t, m)
		}},
	}
	for _, tc := range cases {
		for _, inPeer := range []bool{false, true} {
			name := tc.name + "/root"
			if inPeer {
				name = tc.name + "/peer"
			}
			t.Run(name, func(t *testing.T) {
				m := incidentalStoredModel(t, tc.session)
				if tc.prepare != nil {
					tc.prepare(t, &m)
				}
				if inPeer {
					m = enterLine(t, m, "edit bgp peer peer1")
					require.NoError(t, m.err)
					require.Equal(t, topPeerContext, m.contextPath)
				}
				m.showHints = true
				m.editor.diffGutter = true
				m.setViewportData(*m.configViewAtPath(m.contextPath))
				m.viewport.SetYOffset(2)
				line := tc.line(&m)
				out, result := enterLineResult(t, m, line)
				assertIncidentalView(t, m, out, result, line, tc.status)
				if tc.check != nil {
					tc.check(t, &out)
				}
			})
		}
	}
}

// TestWindowRevertedRefreshesThePosition covers the asynchronous refresh
// producer and both result appliers over the configuration actually restored
// in the store, at root and peer positions.
func TestWindowRevertedRefreshesThePosition(t *testing.T) {
	for _, position := range [][]string{nil, topPeerContext} {
		for _, applier := range []string{"command result", "direct result"} {
			t.Run(strings.Join(position, "/")+"/"+applier, func(t *testing.T) {
				m := incidentalStoredModel(t, true)
				m.contextPath = slices.Clone(position)
				m.setViewportData(*m.configViewAtPath(position))
				changed := strings.Replace(testValidBGPConfigSimplePeer, "1.2.3.4", "9.9.9.9", 1)
				_, err := storage.WriteCandidateVersion(m.editor.store, m.editor.originalPath, []byte(changed), time.Now())
				require.NoError(t, err)
				require.NoError(t, storage.PromoteCandidate(m.editor.store, m.editor.originalPath))
				result := m.windowReverted("trial restored")
				out := m
				if applier == "direct result" {
					out.applyResult(result)
				} else {
					applied, _ := m.handleCommandResult(commandResultMsg{result: result})
					var ok bool
					out, ok = applied.(Model)
					require.True(t, ok)
				}
				assertIncidentalView(t, m, out, result, "", "trial restored")
				assert.Contains(t, out.editor.WorkingContent(), "9.9.9.9")
				assert.NotContains(t, out.statusMessage, "view not refreshed")
			})
		}
	}
}

// TestValidCommitVerifyKeepsItsViewport pins the status-only answer: unlike
// the issue report, a valid verification does not replace the current view.
func TestValidCommitVerifyKeepsItsViewport(t *testing.T) {
	for _, position := range [][]string{nil, topPeerContext} {
		t.Run(strings.Join(position, "/"), func(t *testing.T) {
			m := incidentalStoredModel(t, false)
			m.contextPath = slices.Clone(position)
			m.setViewportData(*m.configViewAtPath(position))
			before := m.viewportContent
			out, result := enterLineResult(t, m, "commit verify")
			require.NoError(t, out.err)
			assert.False(t, result.refreshConfig)
			assert.Nil(t, result.configView)
			assert.Empty(t, result.output)
			assert.Equal(t, before, out.viewportContent)
			assert.Contains(t, out.statusMessage, "the candidate is valid; nothing was applied")
			assert.Equal(t, position, out.contextPath)
		})
	}
}
