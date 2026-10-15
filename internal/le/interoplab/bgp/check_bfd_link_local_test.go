// Design: docs/architecture/testing/interop.md -- BFD over link-local against FRR.
// Related: check_bfd_link_local.go -- zeLinkLocalSession, requireCrossedDiscriminators
package bgp

import (
	"net/netip"
	"slices"
	"strings"
	"testing"
)

// VALIDATES: the bfd-link-local checkers' three decisions, each in both
// polarities: which links ze's sessions to the peer run on, which ze session
// a link's check reads, and whether the discriminators crossed. Method: feed hand-written `show bfd sessions` answers and
// discriminator pairs, and require the accept or the named refusal.
// PREVENTS: a checker that passes on a session to the right address on the
// wrong link, on a session not yet Up, or on one direction of traffic alone.
func TestBFDLinkLocalCheckerBranches(t *testing.T) {
	peer := netip.MustParseAddr("fe80::fd00:1e:2c:3")
	t.Run("session on the named link is read", func(t *testing.T) {
		session, err := zeLinkLocalSession(`[{"peer":"fe80::fd00:1e:2c:3","interface":"eth1","state":"up","local-discriminator":9,"remote-discriminator":8},`+
			`{"peer":"fe80::fd00:1e:2c:3","interface":"eth0","state":"up","local-discriminator":1,"remote-discriminator":2}]`, peer, "eth0")
		if err != nil {
			t.Fatal(err)
		}
		if session.Interface != "eth0" || *session.LocalDiscr != 1 {
			t.Fatalf("read %+v, want the eth0 session with local discriminator 1", session)
		}
	})
	for _, refused := range []struct {
		name, output, want string
	}{
		{"only another link", `[{"peer":"fe80::fd00:1e:2c:3","interface":"eth1","state":"up"}]`, "no BFD session"},
		{"not up", `[{"peer":"fe80::fd00:1e:2c:3","interface":"eth0","state":"down"}]`, `"down", want up`},
		{"not JSON", `error: no service`, "decode ze BFD sessions JSON"},
	} {
		t.Run(refused.name, func(t *testing.T) {
			_, err := zeLinkLocalSession(refused.output, peer, "eth0")
			if err == nil || !strings.Contains(err.Error(), refused.want) {
				t.Fatalf("error = %v, want it to contain %q", err, refused.want)
			}
		})
	}

	t.Run("links of the sessions to the peer, whatever their state", func(t *testing.T) {
		links, err := zeLinkLocalLinks(`[{"peer":"fe80::fd00:1e:2c:3","interface":"eth2","state":"down"},`+
			`{"peer":"fe80::fd00:1e:2c:9","interface":"eth9","state":"up"},`+
			`{"peer":"fe80::fd00:1e:2c:3%eth1","interface":"eth1","state":"up"},`+
			`{"peer":"fe80::fd00:1e:2c:3","interface":"eth2","state":"down"}]`, peer)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(links, []string{"eth1", "eth2"}) {
			t.Fatalf("links = %q, want [eth1 eth2]: sorted, distinct, the other peer's eth9 left out", links)
		}
	})
	for _, refused := range []struct {
		name, output, want string
	}{
		{"not JSON", `error: no service`, "decode ze BFD sessions JSON"},
		{"peer not an address", `[{"peer":"zbl0","interface":"eth1","state":"up"}]`, "is not an address"},
	} {
		t.Run("links "+refused.name, func(t *testing.T) {
			_, err := zeLinkLocalLinks(refused.output, peer)
			if err == nil || !strings.Contains(err.Error(), refused.want) {
				t.Fatalf("error = %v, want it to contain %q", err, refused.want)
			}
		})
	}

	discriminator := func(value uint64) *uint64 { return &value }
	t.Run("crossed discriminators accepted", func(t *testing.T) {
		if err := requireCrossedDiscriminators(7, 1, zeBFDSession{LocalDiscr: discriminator(1), RemoteDiscr: discriminator(7)}); err != nil {
			t.Fatal(err)
		}
	})
	for _, refused := range []struct {
		name               string
		frrID, frrRemoteID uint64
		ze                 zeBFDSession
		want               string
	}{
		{"FRR never heard ze", 7, 0, zeBFDSession{LocalDiscr: discriminator(1), RemoteDiscr: discriminator(7)}, "FRR did not learn"},
		{"ze never heard FRR", 7, 1, zeBFDSession{LocalDiscr: discriminator(1), RemoteDiscr: discriminator(0)}, "ze did not learn"},
		{"ze printed no remote", 7, 1, zeBFDSession{LocalDiscr: discriminator(1)}, "no remote-discriminator"},
		{"zero local", 7, 0, zeBFDSession{LocalDiscr: discriminator(0), RemoteDiscr: discriminator(7)}, "ze's local discriminator is 0"},
	} {
		t.Run(refused.name, func(t *testing.T) {
			err := requireCrossedDiscriminators(refused.frrID, refused.frrRemoteID, refused.ze)
			if err == nil || !strings.Contains(err.Error(), refused.want) {
				t.Fatalf("error = %v, want it to contain %q", err, refused.want)
			}
		})
	}
}
