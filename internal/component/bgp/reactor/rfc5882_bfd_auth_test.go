// Design: docs/guide/bfd.md -- Enabling BFD on a BGP peer, and Authentication
// Related: config_bfd_profile_commit_test.go -- the commit check this test runs
// Related: rfc5882_peer_bfd_test.go -- fakeBFDService, the Service stand-in
//
// RFC 5882 Section 10.2 asks that an EBGP session advised by BFD use BFD
// authentication. Ze's route to it runs through two components: the BGP peer
// names a bfd profile, and the BFD plugin resolves that name into the
// profile's timers, role and authentication (bfd.resolveProfile). This file
// proves the BGP half from the operator's config; the BFD half, from the same
// request to the Control packets on the wire, is
// internal/component/bfd/rfc5882_bgp_auth_test.go.
package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
)

// rfc5882AuthSection is a bfd section whose "secure" profile carries a Keyed
// SHA1 auth block.
const rfc5882AuthSection = `{"bfd":{"profile":{"secure":{` +
	`"auth":{"type":"keyed-sha1","key-id":"7","secret":"EBGP-SECRET"}}}}}`

// RFC requirement: RFC5882-10.2-1 positive -- "BFD authentication SHOULD be
// used", the BGP half: an EBGP peer (local AS 65000, remote AS 65001) whose
// `connection bfd` block names the profile "secure" passes the commit check
// against a bfd section where "secure" carries a Keyed SHA1 auth block, and
// opens its BFD session with a request that names "secure" and carries no
// authentication of its own, so the profile's auth block is what the BFD
// plugin applies to the session.
//
// Goal and method: the peer's settings come from PeersFromTree over the
// operator's bgp tree, verifyPeerBFDProfiles runs the real bfd profile check,
// and startBFDClient hands its request to a recording Service.
func TestRFC5882EBGPPeerNamesAuthenticatedProfile(t *testing.T) {
	tree := bfdStrictTree(map[string]any{"enabled": "true", "profile": "secure"})
	if err := verifyPeerBFDProfiles(tree, rfc5882AuthSection); err != nil {
		t.Fatalf("commit refused a peer naming an authenticated profile: %v", err)
	}

	peers, err := PeersFromTree(tree)
	if err != nil {
		t.Fatalf("PeersFromTree: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("PeersFromTree returned %d peers, want 1", len(peers))
	}

	var got api.SessionRequest
	svc := &fakeBFDService{}
	svc.ensureFn = func(req api.SessionRequest) (api.SessionHandle, error) {
		got = req
		svc.handle = &fakeBFDHandle{key: req.Key(), ch: make(chan api.StateChange, 4)}
		return svc.handle, nil
	}
	api.SetService(svc)
	defer api.SetService(nil)

	p := NewPeer(peers[0])
	p.startBFDClient()
	defer p.stopBFDClient()

	if svc.ensure.Load() != 1 {
		t.Fatalf("EnsureSession calls = %d, want 1", svc.ensure.Load())
	}
	if got.Profile != "secure" {
		t.Errorf("request profile = %q, want %q: the BFD plugin would not apply the auth block", got.Profile, "secure")
	}
	if got.Auth != nil {
		t.Errorf("request carries its own auth %+v, want none: the profile decides the session's authentication", got.Auth)
	}
}
