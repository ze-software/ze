// Design: docs/guide/bfd.md -- Session sharing
// Related: rfc5882_auth_join_test.go -- the fixture these tests reuse
// Related: engine/auth_join.go -- authMismatchCase builds the refusal
//
// A client refused because its BFD authentication differs from the live
// session's logs the refusal as its error, so the refusal has to name a fix
// the operator can apply. An OSPF interface's bfd block carries no profile and
// no auth block, so a fix that asks every client for the same auth block is
// impossible when OSPF shares the session. These tests open the first session
// through the real pluginService.EnsureSession, with an OSPF-shaped request
// (no profile, no authentication, timers only) or a BGP-shaped one naming a
// profile, then ask again for the same remote system and read the refusal.
package bfd

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/engine"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// ospfShapedRequest is the request an OSPF interface's bfd block produces
// (ospf bfdRequestForNeighbor): the neighbor, the interface address, single
// hop and the block's timers, with no profile and no authentication.
func ospfShapedRequest() api.SessionRequest {
	return api.SessionRequest{
		Peer:                  bgpPeerAddr,
		Local:                 bgpLocalAddr,
		Mode:                  api.SingleHop,
		DesiredMinTxInterval:  300000,
		RequiredMinRxInterval: 300000,
		DetectMult:            3,
	}
}

// pinnedNoProfileRequest is the request a pinned single-hop-session with no
// profile leaf produces: the neighbor, the local address and single hop, with
// no profile and so no authentication.
func pinnedNoProfileRequest() api.SessionRequest {
	return api.SessionRequest{Peer: bgpPeerAddr, Local: bgpLocalAddr, Mode: api.SingleHop}
}

// authJoinEditedSection is authJoinSection after an operator edits two
// profiles: "secure" moves to key-id 9, and "open" gains an auth block.
const authJoinEditedSection = `{"bfd":{"profile":{` +
	`"secure":{"auth":{"type":"keyed-sha1","key-id":"9","secret":"JOIN-SECRET"}},` +
	`"open":{"auth":{"type":"keyed-sha1","key-id":"7","secret":"JOIN-SECRET"}}}}}`

// TestAuthJoinRefusalNamesApplicableFix opens a session with the first
// request, applies the edit config when the case has one, asks again with the
// second, and checks the refusal: it matches engine.ErrAuthMismatch, says
// why, carries the one fix for the case it met, never carries a fix that case
// cannot apply, and stays short enough to read in a log line. The OSPF cases
// are the reason the fix is per case: with OSPF on either side, "give every
// client the same auth block" cannot be done. The edited-profile cases are
// the reason the refusal names the release and the guide: a live session
// keeps the authentication it was built with, so naming the profile it was
// built from cannot help once that profile changed. Every other case fixes
// the refused client only, so it points nowhere else.
func TestAuthJoinRefusalNamesApplicableFix(t *testing.T) {
	const impossibleForOSPF = "or copy its auth block into"
	const editingLive = `(docs/guide/bfd.md, "Editing live")`
	const refusalCharsMax = 400
	cases := []struct {
		name    string
		first   api.SessionRequest
		edit    string
		second  api.SessionRequest
		want    []string
		without string
	}{
		{
			name:   "ospf holds the session, authenticated bgp joins",
			first:  ospfShapedRequest(),
			second: bgpPeerSessionRequest("secure"),
			want: []string{
				"the session runs none, bfd profile \"secure\" has an auth block",
				"to fix it, remove the auth block from bfd profile \"secure\"",
			},
			without: impossibleForOSPF,
		},
		{
			name:   "authenticated bgp holds the session, ospf joins",
			first:  bgpPeerSessionRequest("secure"),
			second: ospfShapedRequest(),
			want: []string{
				"the session authenticates, this client names no bfd profile",
				"to fix it, name bfd profile \"secure\" on this client, or set " +
					"bfd { enabled false; } if it is an OSPF interface",
			},
			without: impossibleForOSPF,
		},
		{
			name:   "authenticated bgp holds the session, pinned session with no profile joins",
			first:  bgpPeerSessionRequest("secure"),
			second: pinnedNoProfileRequest(),
			want: []string{
				"the session authenticates, this client names no bfd profile",
				"to fix it, name bfd profile \"secure\" on this client",
			},
		},
		{
			name:   "authenticated bgp holds the session, unauthenticated profile joins",
			first:  bgpPeerSessionRequest("secure"),
			second: bgpPeerSessionRequest("open"),
			want: []string{
				"the session authenticates, bfd profile \"open\" has no auth block",
				"to fix it, give bfd profile \"open\" the auth block of bfd profile \"secure\"",
			},
		},
		{
			name:   "two authenticated profiles differ",
			first:  bgpPeerSessionRequest("secure"),
			second: bgpPeerSessionRequest("rekeyed"),
			want: []string{
				"the session was built from bfd profile \"secure\", this client names \"rekeyed\"",
				"to fix it, name bfd profile \"secure\" on this client, or copy its auth block into \"rekeyed\"",
			},
		},
		{
			name:   "the profile the session was built from is edited, then asked for again",
			first:  bgpPeerSessionRequest("secure"),
			edit:   authJoinEditedSection,
			second: bgpPeerSessionRequest("secure"),
			want: []string{
				"bfd profile \"secure\" changed after the session was built from it",
				"to fix it, release the session from every client to apply the edit " + editingLive,
			},
			without: impossibleForOSPF,
		},
		{
			name:   "an unauthenticated profile the session was built from gains an auth block",
			first:  bgpPeerSessionRequest("open"),
			edit:   authJoinEditedSection,
			second: bgpPeerSessionRequest("open"),
			want: []string{
				"bfd profile \"open\" changed after the session was built from it",
				"to fix it, release the session from every client to apply the edit " + editingLive,
			},
			without: "remove the auth block from",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _ := authJoinFixture(t)
			if _, err := svc.EnsureSession(tc.first); err != nil {
				t.Fatalf("EnsureSession(first): %v", err)
			}
			if tc.edit != "" {
				applyAuthJoinEdit(t, svc, tc.edit)
			}
			h, err := svc.EnsureSession(tc.second)
			if err == nil {
				t.Fatalf("EnsureSession(second) joined session %v, want refusal", h.Key())
			}
			if !errors.Is(err, engine.ErrAuthMismatch) {
				t.Fatalf("EnsureSession(second) error = %v, want engine.ErrAuthMismatch", err)
			}
			text := err.Error()
			want := append([]string{
				engine.ErrAuthMismatch.Error(),
				"one session per neighbor runs one authentication (RFC 5882 Section 4.4), " +
					"so this client runs without BFD",
			}, tc.want...)
			for _, w := range want {
				if !strings.Contains(text, w) {
					t.Errorf("refusal %q\ndoes not contain %q", text, w)
				}
			}
			if tc.without != "" && strings.Contains(text, tc.without) {
				t.Errorf("refusal %q\nnames %q, which this case cannot apply", text, tc.without)
			}
			if tc.edit == "" && strings.Contains(text, editingLive) {
				t.Errorf("refusal %q\npoints at the release guide, but its fix changes this client only", text)
			}
			if len(text) > refusalCharsMax {
				t.Errorf("refusal is %d characters, want at most %d: %q", len(text), refusalCharsMax, text)
			}
			if strings.Contains(text, "interface \"\"") {
				t.Errorf("refusal %q\nnames an empty interface", text)
			}
		})
	}
}

// applyAuthJoinEdit commits section to svc the way a config reload does
// (runtimeState.applyConfig), so the next request resolves its profile from
// the edited config.
func applyAuthJoinEdit(t *testing.T, svc *pluginService, section string) {
	t.Helper()

	cfg, err := parseSections([]sdk.ConfigSection{{Root: configRoot, Data: section}})
	if err != nil {
		t.Fatalf("parseSections(edit): %v", err)
	}
	runtimeStateGuard.Lock()
	err = svc.state.applyConfig(cfg)
	runtimeStateGuard.Unlock()
	if err != nil {
		t.Fatalf("applyConfig(edit): %v", err)
	}
}
