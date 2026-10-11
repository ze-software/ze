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

// TestAuthJoinRefusalNamesApplicableFix opens a session with the first
// request, asks again with the second, and checks the refusal: it matches
// engine.ErrAuthMismatch, says why, carries the fix for the case it met, and
// never carries a fix that case cannot apply. The OSPF cases are the reason
// the fix is per case: with OSPF on either side, "give every client the same
// auth block" cannot be done, so the refusal names dropping the auth block or
// disabling bfd on the OSPF interface.
func TestAuthJoinRefusalNamesApplicableFix(t *testing.T) {
	const impossibleForOSPF = "name the same bfd profile on every client of this neighbor"
	cases := []struct {
		name    string
		first   api.SessionRequest
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
				"disable bfd on any OSPF interface to this neighbor",
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
				"an OSPF interface, whose bfd block carries no authentication, can instead disable bfd on that interface",
			},
			without: impossibleForOSPF,
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
				"this client's bfd profile \"rekeyed\" carries a different auth block",
				"to fix it, " + impossibleForOSPF,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _ := authJoinFixture(t)
			if _, err := svc.EnsureSession(tc.first); err != nil {
				t.Fatalf("EnsureSession(first): %v", err)
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
			}, tc.want...)
			for _, w := range want {
				if !strings.Contains(text, w) {
					t.Errorf("refusal %q\ndoes not contain %q", text, w)
				}
			}
			if tc.without != "" && strings.Contains(text, tc.without) {
				t.Errorf("refusal %q\nnames %q, which this case cannot apply", text, tc.without)
			}
		})
	}
}
