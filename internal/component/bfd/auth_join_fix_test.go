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
// why, carries the fix for the case it met, says when a fix takes effect, and
// never carries a fix that case cannot apply. The OSPF cases are the reason
// the fix is per case: with OSPF on either side, "give every client the same
// auth block" cannot be done, so the refusal names dropping the auth block or
// disabling bfd on the OSPF interface. The edited-profile cases are the
// reason the refusal names the release: a live session keeps the
// authentication it was built with, so naming the profile it was built from
// cannot help once that profile changed.
func TestAuthJoinRefusalNamesApplicableFix(t *testing.T) {
	const impossibleForOSPF = "on this client, or give"
	const release = "a live session keeps the authentication it was built with until every client has released it"
	const retry = "a refused client asks again when it next starts BFD"
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
				"the live session runs no authentication but this client's bfd profile \"secure\" carries an auth block",
				"to fix it, remove the auth block from this client's bfd profile \"secure\"",
				"on every OSPF interface to this neighbor, set bfd { enabled false; } or delete its bfd block",
			},
			without: impossibleForOSPF,
		},
		{
			name:   "authenticated bgp holds the session, ospf joins",
			first:  bgpPeerSessionRequest("secure"),
			second: ospfShapedRequest(),
			want: []string{
				"the live session runs authentication but this client names no bfd profile",
				"to fix it, remove the auth block from the other clients' bfd profiles for this neighbor",
				"an OSPF interface, whose bfd block carries no authentication, can instead set bfd { enabled false; } or delete its bfd block",
			},
			without: impossibleForOSPF,
		},
		{
			name:   "authenticated bgp holds the session, pinned session with no profile joins",
			first:  bgpPeerSessionRequest("secure"),
			second: pinnedNoProfileRequest(),
			want: []string{
				"the live session runs authentication but this client names no bfd profile",
				"to fix it, remove the auth block from the other clients' bfd profiles for this neighbor",
				"a pinned single-hop-session or multi-hop-session can instead name a bfd profile with the same auth { type; key-id; secret; } block",
			},
		},
		{
			name:   "authenticated bgp holds the session, unauthenticated profile joins",
			first:  bgpPeerSessionRequest("secure"),
			second: bgpPeerSessionRequest("open"),
			want: []string{
				"this client's bfd profile \"open\" carries no auth block",
				"to fix it, give bfd profile \"open\" the same auth { type; key-id; secret; } block",
			},
		},
		{
			name:   "two authenticated profiles differ",
			first:  bgpPeerSessionRequest("secure"),
			second: bgpPeerSessionRequest("rekeyed"),
			want: []string{
				"this client's bfd profile \"rekeyed\" carries a different auth block from the live session's, which was built from bfd profile \"secure\"",
				"to fix it, name bfd profile \"secure\" " + impossibleForOSPF,
			},
		},
		{
			name:   "the profile the session was built from is edited, then asked for again",
			first:  bgpPeerSessionRequest("secure"),
			edit:   authJoinEditedSection,
			second: bgpPeerSessionRequest("secure"),
			want: []string{
				"this client's bfd profile \"secure\" is the profile the live session's authentication was built from, and its auth block has changed since",
				"to fix it, apply the edit by releasing the session from every client",
				"run 'request peer <address> teardown' for each BGP peer",
			},
			without: impossibleForOSPF,
		},
		{
			name:   "an unauthenticated profile the session was built from gains an auth block",
			first:  bgpPeerSessionRequest("open"),
			edit:   authJoinEditedSection,
			second: bgpPeerSessionRequest("open"),
			want: []string{
				"this client's bfd profile \"open\" is the profile the live session's authentication was built from, and its auth block has changed since",
				"to fix it, apply the edit by releasing the session from every client",
			},
			without: "remove the auth block from this client's",
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
				"every client of one remote system shares one BFD session (RFC 5882 Section 4.4)",
				release,
				retry,
			}, tc.want...)
			for _, w := range want {
				if !strings.Contains(text, w) {
					t.Errorf("refusal %q\ndoes not contain %q", text, w)
				}
			}
			if tc.without != "" && strings.Contains(text, tc.without) {
				t.Errorf("refusal %q\nnames %q, which this case cannot apply", text, tc.without)
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
