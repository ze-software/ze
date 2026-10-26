// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the cross-peer inner-selector guard, transport mode
// Related: inner_selectors.go -- classifyInnerPair, which judges the whole pair a Child SA installs
// VALIDATES: a transport-mode pair the peer narrowed on this node's half only (TSi any,
// TSr TCP port 80) is the peer's choice, not operator-fixed, because the policy installs
// TCP to port 80 alone; and the refusal's selector fix names both halves at the
// connection's own host length, IPv6 included.
// PREVENTS: the operator-ordered exemption (RFC 4301 Section 4.4.1) admitting a policy
// the operator never wrote, because only the remote half was compared.

package engine

import (
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestRFC3948OrderedTransportLocalHalfNarrowedRefused: transport-mode peer b at
// policy-priority 1000 proposes TSi 198.18.0.9/32 protocol 0 any port and TSr
// 192.0.2.1/32 protocol 6 port 80, so its policy installs TCP from local port 80 only;
// full-tunnel peer a at 2000 proposes exactly its configured 0.0.0.0/0 to 0.0.0.0/0.
// b's operator wrote the addresses, not the protocol or the port, so the pair is b's
// choice and the priorities do not order it. Method: both orders, each peer through
// narrowChildSelectors and createFirstChildSA; the second must be answered
// TS_UNACCEPTABLE with nothing on its dataplane and the registry holding the first alone.
func TestRFC3948OrderedTransportLocalHalfNarrowedRefused(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- transport-mode peer b with remote address 198.18.0.9 and no traffic selector at policy-priority 1000 proposing TSi 198.18.0.9/32 protocol 0 any port, TSr 192.0.2.1/32 protocol 6 port 80, and tunnel peer a configured with traffic selector 0.0.0.0/0 to 0.0.0.0/0 at policy-priority 2000 proposing exactly it: whichever proposes second is answered TS_UNACCEPTABLE with no state or policy on its dataplane, the registry holding the first alone, because a pair narrowed on this node's half to one protocol and port is the peer's choice.
	for _, transportFirst := range []bool{true, false} {
		name := "tunnel first"
		if transportFirst {
			name = "transport first"
		}
		t.Run(name, func(t *testing.T) {
			reg := newInnerSelectorRegistry()
			a, b := orderedTransportPeers(t, reg, 2000, 1000)
			first, second := a, b
			if transportFirst {
				first, second = b, a
			}
			if err := localHalfChild(t, first, &mockDP{}); err != nil {
				t.Fatalf("peer %s alone: %v", first.PeerName, err)
			}
			dp := &mockDP{}
			err := localHalfChild(t, second, dp)
			mustInnerRefused(t, err, "peer "+second.PeerName)
			mustNamePeersAndFix(t, err, "a")
			if len(dp.sas) != 0 || len(dp.policies) != 0 {
				t.Fatalf("peer %s's dataplane got %d states and %d policies, want none",
					second.PeerName, len(dp.sas), len(dp.policies))
			}
			if got := reg.holdings(); got != 1 {
				t.Fatalf("registry holds %d Child SAs, want 1 (peer %s)", got, first.PeerName)
			}
		})
	}
}

// localHalfChild runs innerAuthChild's two steps for one peer of orderedTransportPeers:
// peer a proposes its full tunnel, peer b every protocol and port on its own half and
// TCP port 80 on this node's.
func localHalfChild(t *testing.T, sa *SA, dp *mockDP) error {
	t.Helper()
	if sa.PeerName == "a" {
		_, err := innerAuthChild(t, sa, dp, fullTunnelTS, fullTunnelTS)
		return err
	}
	tsi := &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSi, TrafficSelectors: []wire.TrafficSelector{
		wireSel(t, "198.18.0.9/32", 0, 65535, 0),
	}}
	tsr := &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSr, TrafficSelectors: []wire.TrafficSelector{
		wireSel(t, "192.0.2.1/32", 80, 80, 6),
	}}
	if err := narrowChildSelectors(sa, tsi, tsr, nil); err != nil {
		return err
	}
	ifID, err := resolveIfID(&sa.PeerCfg)
	if err != nil {
		t.Fatalf("resolveIfID: %v", err)
	}
	_, err = createFirstChildSA(sa, sa.ESPGroup, sa.PeerCfg.LocalAddress, sa.PeerCfg.RemoteAddress, ifID, dp,
		slogutil.DiscardLogger())
	return err
}

// TestTransportConflictFixNamesIPv6Hosts: the refusal of an IPv6 transport-mode
// connection names its traffic selector fix with /128 host prefixes on both halves, not
// /32. Method: build the two recorded pairs by hand and read innerSelectorTransportConflict.
func TestTransportConflictFixNamesIPv6Hosts(t *testing.T) {
	_, local, err := net.ParseCIDR("2001:db8::1/128")
	if err != nil {
		t.Fatal(err)
	}
	_, remote, err := net.ParseCIDR("2001:db8:9::9/128")
	if err != nil {
		t.Fatal(err)
	}
	_, any6, err := net.ParseCIDR("::/0")
	if err != nil {
		t.Fatal(err)
	}
	transport := innerSelectorPair{
		policy: dataplane.SPParams{Src: local, Dst: remote, Mode: modeTransport},
		text:   "local 2001:db8::1/128 remote 2001:db8:9::9/128",
	}
	tunnel := innerSelectorPair{
		policy: dataplane.SPParams{Src: any6, Dst: any6, Mode: modeTunnel},
		text:   "local ::/0 remote ::/0",
	}
	for _, c := range []struct {
		name   string
		reject error
	}{
		{"transport proposes", innerSelectorTransportConflict("b", transport, "a", tunnel, "a")},
		{"tunnel proposes", innerSelectorTransportConflict("a", tunnel, "b", transport, "a")},
	} {
		t.Run(c.name, func(t *testing.T) {
			msg := c.reject.Error()
			for _, want := range []string{
				"set vpn ipsec site-to-site peer b traffic-selector <n> local prefix 2001:db8::1/128",
				"set vpn ipsec site-to-site peer b traffic-selector <n> remote prefix 2001:db8:9::9/128",
			} {
				if !strings.Contains(msg, want) {
					t.Errorf("refusal %q does not name %q", msg, want)
				}
			}
			if strings.Contains(msg, "/32") {
				t.Errorf("refusal %q names an IPv4 host length for an IPv6 connection", msg)
			}
		})
	}
}
