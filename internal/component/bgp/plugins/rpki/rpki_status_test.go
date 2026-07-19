package rpki

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
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
	rp := &RPKIPlugin{}
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
	rp := &RPKIPlugin{}
	m := map[string]peerActionSet{
		"192.0.2.1": {
			OriginInvalid:  resolvedAction{Action: ASPAPolicyAccept, Source: sourcePeer},
			OriginNotFound: resolvedAction{Action: ASPAPolicyAccept, Source: sourceGlobal},
			ASPAInvalid:    resolvedAction{Action: ASPAPolicyLogOnly, Source: sourceGlobal},
			ASPAUnknown:    resolvedAction{Action: ASPAPolicyAccept, Source: sourceGlobal},
		},
		"198.51.100.7": {
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

// TestStatusCommand_PerPeerActions_Empty verifies an empty array when no overrides exist.
//
// VALIDATES: appendPeerActions emits an empty array (not null) with a nil map.
// PREVENTS: JSON shape drift breaking consumers on the no-override case.
func TestStatusCommand_PerPeerActions_Empty(t *testing.T) {
	rp := &RPKIPlugin{}
	b := textbuf.Get()
	defer b.Release()
	rp.appendPeerActions(b)
	assert.Equal(t, `,"peer-actions":[]`, b.String())
}
