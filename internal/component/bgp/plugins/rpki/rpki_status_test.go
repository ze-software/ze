package rpki

import (
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestActionString verifies the action-constant to keyword mapping.
//
// VALIDATES: actionString renders reject/log-only/accept.
// PREVENTS: status display drifting from config keywords.
func TestActionString(t *testing.T) {
	assert.Equal(t, "reject", actionString(ASPAPolicyReject))
	assert.Equal(t, "log-only", actionString(ASPAPolicyLogOnly))
	assert.Equal(t, "accept", actionString(ASPAPolicyAccept))
}

// TestStatusCommand_GlobalActions verifies the effective global actions object (AC-9).
//
// VALIDATES: appendGlobalActions emits the four effective actions from the atomics.
// PREVENTS: status omitting global actions or reading a different source than enforcement.
func TestStatusCommand_GlobalActions(t *testing.T) {
	rp := &rPKIPlugin{}
	rp.originInvalidAction.Store(uint32(ASPAPolicyReject))
	rp.originNotFoundAction.Store(uint32(ASPAPolicyAccept))
	rp.aspaInvalidAction.Store(uint32(ASPAPolicyLogOnly))
	rp.aspaUnknownAction.Store(uint32(ASPAPolicyAccept))

	b := textbuf.Get()
	defer b.Release()
	rp.appendGlobalActions(b)

	assert.Equal(t,
		`,"actions":{"invalid":"reject","not-found":"accept","aspa-invalid":"log-only","aspa-unknown":"accept"}`,
		b.String())
}

// TestStatusCommand_PerPeerActions verifies the per-peer resolved actions array with per-leaf
// source (AC-10).
//
// VALIDATES: appendPeerActions emits one entry per peer with action + source per leaf, sorted.
// PREVENTS: status hiding which config level supplied each action.
func TestStatusCommand_PerPeerActions(t *testing.T) {
	rp := &rPKIPlugin{}
	m := map[configjson.PeerConfigKey]peerActionSet{
		{ID: "192.0.2.1"}: {
			OriginInvalid:  resolvedAction{Action: ASPAPolicyAccept, Source: sourcePeer},
			OriginNotFound: resolvedAction{Action: ASPAPolicyAccept, Source: sourceGlobal},
			ASPAInvalid:    resolvedAction{Action: ASPAPolicyLogOnly, Source: sourceGlobal},
			ASPAUnknown:    resolvedAction{Action: ASPAPolicyAccept, Source: sourceGlobal},
		},
		{ID: "198.51.100.7"}: {
			OriginInvalid:  resolvedAction{Action: ASPAPolicyReject, Source: sourceGroup},
			OriginNotFound: resolvedAction{Action: ASPAPolicyAccept, Source: sourceGlobal},
			ASPAInvalid:    resolvedAction{Action: ASPAPolicyLogOnly, Source: sourceGlobal},
			ASPAUnknown:    resolvedAction{Action: ASPAPolicyAccept, Source: sourceGlobal},
		},
	}
	rp.perPeerActions.Store(&m)

	b := textbuf.Get()
	defer b.Release()
	rp.appendPeerActions(b)
	out := b.String()

	// Sorted by IP: 192.0.2.1 before 198.51.100.7.
	assert.Equal(t,
		`,"peer-actions":[`+
			`{"peer":"192.0.2.1","invalid":{"action":"accept","source":"peer"},`+
			`"not-found":{"action":"accept","source":"global"},`+
			`"aspa-invalid":{"action":"log-only","source":"global"},`+
			`"aspa-unknown":{"action":"accept","source":"global"}},`+
			`{"peer":"198.51.100.7","invalid":{"action":"reject","source":"group"},`+
			`"not-found":{"action":"accept","source":"global"},`+
			`"aspa-invalid":{"action":"log-only","source":"global"},`+
			`"aspa-unknown":{"action":"accept","source":"global"}}]`,
		out)
}

// TestStatusCommand_PerPeerActions_NamesAGroupAsAGroup verifies a dynamic group's
// template is rendered for what it is.
//
// VALIDATES: appendPeerActions writes `"group":"<name>"` for a template entry and
// `"peer":"<ip>"` for a configured peer, with peers first.
// PREVENTS: `"peer":"ix"` in operator output, which sends the reader looking for a
// peer that does not exist. The key holds a GROUP's name, and a listen-range group
// is where an IXP states the actions every session it accepts inherits.
func TestStatusCommand_PerPeerActions_NamesAGroupAsAGroup(t *testing.T) {
	rp := &rPKIPlugin{}
	leaves := peerActionSet{
		OriginInvalid:  resolvedAction{Action: ASPAPolicyReject, Source: sourceGroup},
		OriginNotFound: resolvedAction{Action: ASPAPolicyAccept, Source: sourceGlobal},
		ASPAInvalid:    resolvedAction{Action: ASPAPolicyLogOnly, Source: sourceGlobal},
		ASPAUnknown:    resolvedAction{Action: ASPAPolicyAccept, Source: sourceGlobal},
	}
	m := map[configjson.PeerConfigKey]peerActionSet{
		configjson.GroupKey("ix"): leaves,
		{ID: "192.0.2.1"}:         leaves,
	}
	rp.perPeerActions.Store(&m)

	b := textbuf.Get()
	defer b.Release()
	rp.appendPeerActions(b)
	out := b.String()

	assert.Contains(t, out, `{"group":"ix","invalid":{"action":"reject","source":"group"}`)
	assert.Contains(t, out, `{"peer":"192.0.2.1","invalid":{"action":"reject","source":"group"}`)
	assert.NotContains(t, out, `"peer":"ix"`)
	assert.Less(t, strings.Index(out, `"peer":"192.0.2.1"`), strings.Index(out, `"group":"ix"`),
		"peers sort before groups")
}

// TestStatusReportsSyncSeparatelyFromConfiguration verifies an operator can tell a configured
// cache server that has never delivered data from one that has.
//
// VALIDATES: statusCommand reports "synced":false and "sessions-synced":0 while the configured
// session has completed no sync, and flips both once an End of Data PDU lands. rp.active stays
// true across both, so the two facts are reported separately rather than as one.
// PREVENTS: A router that reports RPKI as running while it holds no VRP set, which makes every
// prefix read not-found and be accepted by the default not-found action.
func TestStatusReportsSyncSeparatelyFromConfiguration(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)

	rp := &rPKIPlugin{cache: newROACache(), aspaCache: newASPACache()}
	sess := newRTRSession("192.0.2.1", 3323, 100, "", rp.cache, rp.aspaCache, stopCh)
	rp.sessions = append(rp.sessions, sess)
	rp.active.Store(true) // a cache server is configured

	status := func() string {
		t.Helper()
		_, payload, err := rp.statusCommand()
		require.NoError(t, err)
		raw, ok := payload.(json.RawMessage)
		require.True(t, ok, "statusCommand answers with a JSON payload")
		return string(raw)
	}

	// The top-level fields are asserted as one anchored run. A bare `"synced":false` also
	// matches the per-server field inside "cache-servers", so it would pass even if the
	// top-level flag reported the configuration instead of the sync.
	out := status()
	assert.Contains(t, out, `"vrp-count-ipv4":0,"vrp-count-ipv6":0`, "no VRP has arrived")
	assert.Contains(t, out, `"sessions":1,"sessions-synced":0,"synced":false,"aspa-enabled"`)
	assert.Contains(t, out, `"state":"idle","synced":false`, "the configured server has delivered nothing")
	assert.True(t, rp.active.Load(), "configured stays true while nothing has synced")

	// One End of Data completes a sync on that session.
	buf := make([]byte, pduEndOfDataLen)
	buf[1] = pduEndOfData
	binary.BigEndian.PutUint32(buf[4:8], pduEndOfDataLen)
	done, err := sess.handlePDU(rTRHeader{Type: pduEndOfData, Length: pduEndOfDataLen}, buf)
	require.NoError(t, err)
	require.True(t, done)

	_, payload, err := rp.statusCommand()
	require.NoError(t, err)
	raw, ok := payload.(json.RawMessage)
	require.True(t, ok, "statusCommand answers with a JSON payload")
	out = string(raw)
	assert.Contains(t, out, `"sessions":1,"sessions-synced":1,"synced":true,"aspa-enabled"`)
	assert.Contains(t, out, `"state":"idle","synced":true`, "a synced server reports the sync, not the RTR state")
	assert.NotContains(t, out, `"synced":false`, "nothing still reads as unsynced")
	assert.True(t, rp.active.Load(), "configured is unchanged by the sync")
}

// TestStatusCommand_PerPeerActions_Empty verifies an empty array when no overrides exist.
//
// VALIDATES: appendPeerActions emits an empty array (not null) with a nil map.
// PREVENTS: JSON shape drift breaking consumers on the no-override case.
func TestStatusCommand_PerPeerActions_Empty(t *testing.T) {
	rp := &rPKIPlugin{}
	b := textbuf.Get()
	defer b.Release()
	rp.appendPeerActions(b)
	assert.Equal(t, `,"peer-actions":[]`, b.String())
}
