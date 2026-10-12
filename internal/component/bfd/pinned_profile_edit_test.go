// Design: docs/guide/bfd.md -- Editing live
// Related: bfd.go -- applyPinned
// Related: engine/shared_session.go -- Loop.UpdateSession
//
// VALIDATES: spec-bgp-live-receive-binding-and-bfd-keep-session AC-25 at the
// plugin's apply: a reload that edits the profile of a pinned session reaches
// the live session in place. A timer edit keeps the session and its local
// discriminator and changes what it advertises; an auth block added to the
// profile lets a client asking for that authentication join the one session it
// was refused before the edit.
// PREVENTS: applyPinned skipping a key it already holds, so a profile edit to a
// pinned session changes nothing until the daemon restarts.
package bfd

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/engine"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	"github.com/ze-software/ze/internal/core/clock"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// pinnedEditPeer is the remote system of the pinned multi-hop session.
const pinnedEditPeer = "203.0.113.9"

// pinnedEditSection is a config with profile "p" (300ms x 3, its auth block
// given by pAuth), profile "secure" with a Keyed SHA1 auth block, and a pinned
// multi-hop session to pinnedEditPeer under profile "p".
func pinnedEditSection(pTimers, pAuth string) string {
	return `{"bfd":{"profile":{` +
		`"p":{` + pTimers + pAuth + `},` +
		`"secure":{"auth":{"type":"keyed-sha1","key-id":"7","secret":"EDIT-SECRET"}}},` +
		`"multi-hop-session":{"` + pinnedEditPeer + `":{"local":"172.30.9.2","profile":"p"}}}}`
}

const (
	pinnedEditSlow = `"detect-multiplier":"3","desired-min-tx-us":"300000","required-min-rx-us":"300000"`
	pinnedEditFast = `"detect-multiplier":"5","desired-min-tx-us":"100000","required-min-rx-us":"100000"`
	pinnedEditAuth = `,"auth":{"type":"keyed-sha1","key-id":"7","secret":"EDIT-SECRET"}`
)

// pinnedEditService applies section with the multi-hop loop already in
// place, as multiHopServiceIn does, so applyPinned runs end to end without
// binding the RFC 5883 port. The loop is never started.
func pinnedEditService(t *testing.T, section string) (*pluginService, *engine.Loop) {
	t.Helper()
	tr, _ := transport.Pair(api.MultiHop, netip.MustParseAddr("172.30.9.2"), netip.MustParseAddr(pinnedEditPeer))
	loop := engine.NewLoop(tr, clock.RealClock{})
	state := newRuntimeState()
	key := loopKey{vrf: api.DefaultVRF, mode: api.MultiHop}
	state.loops[key] = loop
	state.loopDevices[key] = loopDeviceFor(api.SessionRequest{VRF: api.DefaultVRF})
	svc := &pluginService{state: state}
	applyPinnedEdit(t, svc, section)
	return svc, loop
}

// applyPinnedEdit commits section the way a config reload does.
func applyPinnedEdit(t *testing.T, svc *pluginService, section string) {
	t.Helper()
	cfg, err := parseSections([]sdk.ConfigSection{{Root: configRoot, Data: section}})
	if err != nil {
		t.Fatalf("parseSections: %v", err)
	}
	runtimeStateGuard.Lock()
	err = svc.state.applyConfig(cfg)
	runtimeStateGuard.Unlock()
	if err != nil {
		t.Fatalf("applyConfig: %v", err)
	}
}

// onlySession answers the loop's one session, failing on any other count.
func onlySession(t *testing.T, loop *engine.Loop) api.SessionState {
	t.Helper()
	snap := loop.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("the loop holds %d sessions, want 1", len(snap))
	}
	return snap[0]
}

// TestPinnedProfileTimerEditInPlace edits profile "p" from 300ms x 3 to
// 100ms x 5 by reload. The pinned session keeps its local discriminator and
// now runs Detect Mult 5.
// MUTATION: applyPinned skipping UpdateSession for a key it holds keeps
// Detect Mult 3, so this test goes red.
func TestPinnedProfileTimerEditInPlace(t *testing.T) {
	svc, loop := pinnedEditService(t, pinnedEditSection(pinnedEditSlow, ""))
	before := onlySession(t, loop)
	if before.DetectMult != 3 {
		t.Fatalf("precondition: Detect Mult %d, want 3", before.DetectMult)
	}

	applyPinnedEdit(t, svc, pinnedEditSection(pinnedEditFast, ""))

	after := onlySession(t, loop)
	if after.LocalDiscr != before.LocalDiscr {
		t.Fatalf("the edit replaced the session: discriminator %d, want %d", after.LocalDiscr, before.LocalDiscr)
	}
	if after.DetectMult != 5 {
		t.Fatalf("after the edit: Detect Mult %d, want the edited 5", after.DetectMult)
	}
}

// TestPinnedProfileAuthEditJoinsAgain asks for the pinned session's remote
// system under profile "secure" (Keyed SHA1) and is refused, since "p" has no
// auth block. A reload gives "p" the same auth block, and the same request
// joins the one session, now at refcount 2.
func TestPinnedProfileAuthEditJoinsAgain(t *testing.T) {
	svc, loop := pinnedEditService(t, pinnedEditSection(pinnedEditSlow, ""))
	joiner := api.SessionRequest{Peer: netip.MustParseAddr(pinnedEditPeer), Mode: api.MultiHop, Profile: "secure"}
	if _, err := svc.EnsureSession(joiner); !errors.Is(err, engine.ErrAuthMismatch) {
		t.Fatalf("join before the edit: err %v, want ErrAuthMismatch", err)
	}
	before := onlySession(t, loop)

	applyPinnedEdit(t, svc, pinnedEditSection(pinnedEditSlow, pinnedEditAuth))

	if _, err := svc.EnsureSession(joiner); err != nil {
		t.Fatalf("join after the edit: %v", err)
	}
	after := onlySession(t, loop)
	if after.Refcount != 2 || after.LocalDiscr != before.LocalDiscr {
		t.Fatalf("after the join: refcount %d discriminator %d, want 2 on discriminator %d", after.Refcount, after.LocalDiscr, before.LocalDiscr)
	}
}
