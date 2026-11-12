// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the ICMP processing icmp-error-check turns on
// Related: icmp_overlap.go -- refuseICMPErrorBypass and refuseICMPCheckDiscard, the refusals under test
// RFC: rfc/short/rfc4301.md -- Section 6.2, ICMP error processing

package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

// icmpOverlapPeer is the site-to-site peer "site", whose single traffic selector
// carries 10.1.0.0/24 to 10.2.0.0/24, or both IPv6 halves when v6 is set. skipCheck is
// icmp-error-check false.
func icmpOverlapPeer(t *testing.T, v6, skipCheck bool) ipsec.SiteToSitePeer {
	t.Helper()
	local, remote := "10.1.0.0/24", "10.2.0.0/24"
	if v6 {
		local, remote = "2001:db8:1::/64", "2001:db8:2::/64"
	}
	return ipsec.SiteToSitePeer{
		Name:               "site",
		SkipICMPErrorCheck: skipCheck,
		TrafficSelectors: []ipsec.TrafficSelectorPolicy{{
			Number:       "1",
			LocalPrefix:  mustCIDR(t, local),
			LocalPort:    ipsec.AnyPort(),
			RemotePrefix: mustCIDR(t, remote),
			RemotePort:   ipsec.AnyPort(),
		}},
	}
}

// icmpOverlapVTIPeer is icmpOverlapPeer bound to an xfrm interface (`vti bind`), with
// icmp-error-check true: a route-based peer, whose policies carry the interface's
// if_id and so never meet an operator entry in the kernel.
func icmpOverlapVTIPeer(t *testing.T) ipsec.SiteToSitePeer {
	t.Helper()
	peer := icmpOverlapPeer(t, false, false)
	peer.VTIBind = "vpn-xfrm"
	return peer
}

// icmpOverlapPolicy is one operator SPD entry. icmpType below zero is `icmp-type any`.
func icmpOverlapPolicy(t *testing.T, name string, action dataplane.SPAction, dir ipsec.SPDDirection,
	protocol uint8, icmpType int, local, remote string,
) ipsec.SPDPolicy {
	t.Helper()
	p := ipsec.SPDPolicy{
		Name:         name,
		Action:       action,
		Order:        3000,
		Direction:    dir,
		Protocol:     protocol,
		LocalPrefix:  mustCIDR(t, local),
		LocalPort:    ipsec.AnyPort(),
		RemotePrefix: mustCIDR(t, remote),
		RemotePort:   ipsec.AnyPort(),
	}
	if icmpType >= 0 {
		p.ICMP = ipsec.ICMPSelector{HasType: true, Type: uint8(icmpType)}
	}
	return p
}

// icmpOverlapConfig holds one peer and the entries given, under the catch-all given.
func icmpOverlapConfig(peer ipsec.SiteToSitePeer, unmatched dataplane.SPAction, policies ...ipsec.SPDPolicy) *ipsec.IPsecConfig {
	cfg := &ipsec.IPsecConfig{
		Peers:     map[string]ipsec.SiteToSitePeer{peer.Name: peer},
		Policies:  map[string]ipsec.SPDPolicy{},
		Unmatched: unmatched,
	}
	for i := range policies {
		cfg.Policies[policies[i].Name] = policies[i]
	}
	return cfg
}

// requireICMPOverlapRefusal fails unless err is an operator error naming every want.
func requireICMPOverlapRefusal(t *testing.T, err error, wants ...string) {
	t.Helper()
	refusal := icmpOverlapOperatorError(t, err)
	for _, want := range wants {
		if !strings.Contains(refusal.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%s", want, refusal.Error())
		}
	}
}

// icmpOverlapOperatorError fails unless err is a refusal that errors.As finds as an
// operator error, and answers that operator error.
func icmpOverlapOperatorError(t *testing.T, err error) *opererr.Error {
	t.Helper()
	if err == nil {
		t.Fatal("the configuration was accepted, want it refused")
	}
	var refusal *opererr.Error
	if !errors.As(err, &refusal) {
		t.Fatalf("the refusal is %T, want an operator error: %v", err, err)
	}
	return refusal
}

// requireICMPOverlapFixOmits fails unless err is an operator error whose fix names none
// of absent.
func requireICMPOverlapFixOmits(t *testing.T, err error, absent ...string) {
	t.Helper()
	refusal := icmpOverlapOperatorError(t, err)
	for _, unwanted := range absent {
		if strings.Contains(refusal.Fix(), unwanted) {
			t.Errorf("the fix names %q, which does not apply here:\n%s", unwanted, refusal.Fix())
		}
	}
}

// VALIDATES: `vpn ipsec unmatched bypass` beside a peer with icmp-error-check true is
// refused at verify, naming the peer, the catch-all and both fixes; the same catch-all
// beside a peer with icmp-error-check false, beside a `vti bind` peer, and a peer under
// `unmatched discard`, pass.
// PREVENTS: a commit whose bypass catch-all would carry an ICMP error in clear while
// Linux sends that error into the peer's Child SA (icmp_route_lookup).
// RFC requirement: RFC4301-6.2-3 negative -- a configuration holding `vpn ipsec unmatched bypass` and a site-to-site peer with icmp-error-check true and no `vti bind` is refused by verifyICMPOverlap with an operator error naming the peer, the catch-all, `icmp-error-check false` and `unmatched discard`; the same catch-all with the peer's icmp-error-check false or with the peer bound by `vti bind`, and the peer under `unmatched discard`, are accepted.
func TestICMPOverlapBypassCatchAll(t *testing.T) {
	refused := icmpOverlapConfig(icmpOverlapPeer(t, false, false), dataplane.SPActionBypass)
	requireICMPOverlapRefusal(t, verifyICMPOverlap(refused),
		`ipsec peer "site"`, "`vpn ipsec unmatched bypass`",
		"set vpn ipsec site-to-site peer site icmp-error-check false", "set vpn ipsec unmatched discard")

	unchecked := icmpOverlapConfig(icmpOverlapPeer(t, false, true), dataplane.SPActionBypass)
	if err := verifyICMPOverlap(unchecked); err != nil {
		t.Errorf("unmatched bypass beside a peer with icmp-error-check false was refused: %v", err)
	}
	routeBased := icmpOverlapConfig(icmpOverlapVTIPeer(t), dataplane.SPActionBypass)
	if err := verifyICMPOverlap(routeBased); err != nil {
		t.Errorf("unmatched bypass beside a vti bind peer was refused: %v", err)
	}
	discard := icmpOverlapConfig(icmpOverlapPeer(t, false, false), dataplane.SPActionDiscard)
	if err := verifyICMPOverlap(discard); err != nil {
		t.Errorf("unmatched discard beside a peer with icmp-error-check true was refused: %v", err)
	}
}

// VALIDATES: a bypass entry that would carry an outbound ICMP error to a peer's remote
// selectors is refused at verify when the peer has icmp-error-check true: protocol 1 out,
// protocol 0 both, ICMPv6 type 1 out, and any prefix beside a peer with no selector. The
// fix names the entry's remote prefix with the selectors it must stay outside, unless
// the peer has none, and then the entry's icmp-type under protocol 1 or 58 or its
// protocol under protocol 0.
// PREVENTS: an operator bypass for ICMP that Linux silently replaces with the Child SA,
// and a fix that names a command the entry cannot take.
// RFC requirement: RFC4301-6.2-3 negative -- under `vpn ipsec unmatched discard`, a configuration holding a `vpn ipsec policy` bypass entry, direction out or both, protocol 0, 1 or 58 with icmp-type any or an error type, whose remote prefix overlaps the remote selectors of a site-to-site peer with icmp-error-check true and no `vti bind` (or any prefix, for such a peer with no traffic selector), is refused by verifyICMPOverlap with an operator error naming the peer, the entry and `icmp-error-check false`.
func TestICMPOverlapBypassEntryRefused(t *testing.T) {
	bypass, out, both := dataplane.SPActionBypass, ipsec.SPDDirOut, ipsec.SPDDirBoth
	wide := ipsec.SiteToSitePeer{Name: "site"}
	const (
		typeFix     = "set vpn ipsec policy icmp4 icmp-type <type> with a type that is not an ICMP error"
		protocolFix = "set vpn ipsec policy icmp4 protocol <protocol>"
		prefixFix   = "set vpn ipsec policy icmp4 remote prefix <prefix> with a prefix that does not overlap "
	)
	cases := []struct {
		name   string
		peer   ipsec.SiteToSitePeer
		policy ipsec.SPDPolicy
		wants  []string
		absent []string
	}{
		{"icmp out", icmpOverlapPeer(t, false, false),
			icmpOverlapPolicy(t, "icmp4", bypass, out, 1, -1, "0.0.0.0/0", "0.0.0.0/0"),
			[]string{prefixFix + "10.2.0.0/24", typeFix + " (3, 4, 5, 11, 12)"},
			[]string{protocolFix}},
		{"any protocol both", icmpOverlapPeer(t, false, false),
			icmpOverlapPolicy(t, "icmp4", bypass, both, 0, -1, "192.0.2.0/24", "10.2.0.128/25"),
			[]string{prefixFix + "10.2.0.0/24", protocolFix},
			[]string{"icmp-type"}},
		{"icmpv6 destination unreachable out", icmpOverlapPeer(t, true, false),
			icmpOverlapPolicy(t, "icmp4", bypass, out, 58, 1, "::/0", "2001:db8::/32"),
			[]string{prefixFix + "2001:db8:2::/64", typeFix + " (1, 2, 3, 4)"},
			[]string{protocolFix}},
		{"peer with no selector", wide,
			icmpOverlapPolicy(t, "icmp4", bypass, out, 1, 3, "198.51.100.0/24", "203.0.113.0/24"),
			[]string{typeFix},
			[]string{"remote prefix", protocolFix}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := icmpOverlapConfig(tc.peer, dataplane.SPActionDiscard, tc.policy)
			wants := append([]string{`ipsec peer "site"`, "`vpn ipsec policy icmp4`",
				"set vpn ipsec site-to-site peer site icmp-error-check false"}, tc.wants...)
			err := verifyICMPOverlap(cfg)
			requireICMPOverlapRefusal(t, err, wants...)
			requireICMPOverlapFixOmits(t, err, tc.absent...)
		})
	}
}

// VALIDATES: a bypass entry that cannot carry an ICMP error to the peer's remote
// selectors passes verify: a disjoint remote prefix, an echo-request type, an in-only
// entry, a TCP entry, a peer with icmp-error-check false, an entry of the other family,
// and a `vti bind` peer.
// PREVENTS: the refusal reaching an entry the kernel's mapping never overrides.
// RFC requirement: RFC4301-6.2-3 positive -- under `vpn ipsec unmatched discard`, a `vpn ipsec policy` bypass entry whose remote prefix is disjoint from the peer's remote selectors, whose icmp-type is 8, whose direction is in, whose protocol is 6, whose family the peer's selectors do not carry, or beside a peer with icmp-error-check false or with `vti bind`, is accepted by verifyICMPOverlap.
func TestICMPOverlapBypassEntryAccepted(t *testing.T) {
	bypass, out, in := dataplane.SPActionBypass, ipsec.SPDDirOut, ipsec.SPDDirIn
	cases := []struct {
		name   string
		peer   ipsec.SiteToSitePeer
		policy ipsec.SPDPolicy
	}{
		{"remote prefix disjoint", icmpOverlapPeer(t, false, false),
			icmpOverlapPolicy(t, "icmp4", bypass, out, 1, -1, "0.0.0.0/0", "10.3.0.0/16")},
		{"echo request", icmpOverlapPeer(t, false, false),
			icmpOverlapPolicy(t, "icmp4", bypass, out, 1, 8, "0.0.0.0/0", "0.0.0.0/0")},
		{"direction in", icmpOverlapPeer(t, false, false),
			icmpOverlapPolicy(t, "icmp4", bypass, in, 1, -1, "0.0.0.0/0", "0.0.0.0/0")},
		{"tcp", icmpOverlapPeer(t, false, false),
			icmpOverlapPolicy(t, "icmp4", bypass, out, 6, -1, "0.0.0.0/0", "0.0.0.0/0")},
		{"icmp-error-check false", icmpOverlapPeer(t, false, true),
			icmpOverlapPolicy(t, "icmp4", bypass, out, 1, -1, "0.0.0.0/0", "0.0.0.0/0")},
		{"family mismatch", icmpOverlapPeer(t, true, false),
			icmpOverlapPolicy(t, "icmp4", bypass, out, 1, -1, "0.0.0.0/0", "0.0.0.0/0")},
		{"vti bind", icmpOverlapVTIPeer(t),
			icmpOverlapPolicy(t, "icmp4", bypass, out, 1, -1, "0.0.0.0/0", "0.0.0.0/0")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := icmpOverlapConfig(tc.peer, dataplane.SPActionDiscard, tc.policy)
			if err := verifyICMPOverlap(cfg); err != nil {
				t.Errorf("an entry that carries no ICMP error to the peer's remote selectors was refused: %v", err)
			}
		})
	}
}

// VALIDATES: every ICMP error type the kernel maps into a Child SA (IPv4 3, 4, 5, 11 and
// 12; IPv6 1, 2, 3 and 4) draws refusal A on a bypass entry of its family's protocol,
// and one informational type per family (IPv4 8 echo request, IPv6 128 echo request)
// passes.
// PREVENTS: an error type dropped from icmpErrorTypesV4 or icmpErrorTypesV6, or a type
// judged against the other family's list, without a red test.
func TestICMPOverlapBypassErrorTypes(t *testing.T) {
	bypass, out := dataplane.SPActionBypass, ipsec.SPDDirOut
	families := []struct {
		name     string
		v6       bool
		protocol uint8
		prefix   string
		errors   []int
		info     int
	}{
		{"icmp", false, 1, "0.0.0.0/0", []int{3, 4, 5, 11, 12}, 8},
		{"icmpv6", true, 58, "::/0", []int{1, 2, 3, 4}, 128},
	}
	for _, family := range families {
		for _, icmpType := range family.errors {
			t.Run(family.name+" error type "+strconv.Itoa(icmpType), func(t *testing.T) {
				cfg := icmpOverlapConfig(icmpOverlapPeer(t, family.v6, false), dataplane.SPActionDiscard,
					icmpOverlapPolicy(t, "icmp4", bypass, out, family.protocol, icmpType, family.prefix, family.prefix))
				requireICMPOverlapRefusal(t, verifyICMPOverlap(cfg), "`vpn ipsec policy icmp4`")
			})
		}
		t.Run(family.name+" informational type "+strconv.Itoa(family.info), func(t *testing.T) {
			cfg := icmpOverlapConfig(icmpOverlapPeer(t, family.v6, false), dataplane.SPActionDiscard,
				icmpOverlapPolicy(t, "icmp4", bypass, out, family.protocol, family.info, family.prefix, family.prefix))
			if err := verifyICMPOverlap(cfg); err != nil {
				t.Errorf("an entry for an informational ICMP type was refused: %v", err)
			}
		})
	}
}

// VALIDATES: an entry whose protocol matches ICMP only in the other family draws neither
// refusal beside a peer of the entry's address family: protocol 1 (ICMP, IPv4 only) with
// IPv6 prefixes beside an IPv6 peer, and protocol 58 (ICMPv6) with IPv4 prefixes beside
// an IPv4 peer, each as a bypass out entry and as a discard in entry.
// PREVENTS: icmpSpans keeping a span its protocol does not match ICMP in, which would
// refuse an entry no ICMP message reaches.
func TestICMPOverlapProtocolFamily(t *testing.T) {
	cases := []struct {
		name     string
		v6       bool
		protocol uint8
		prefix   string
	}{
		{"protocol 1 with ipv6 prefixes", true, 1, "::/0"},
		{"protocol 58 with ipv4 prefixes", false, 58, "0.0.0.0/0"},
	}
	for _, tc := range cases {
		t.Run(tc.name+" bypass", func(t *testing.T) {
			cfg := icmpOverlapConfig(icmpOverlapPeer(t, tc.v6, false), dataplane.SPActionDiscard,
				icmpOverlapPolicy(t, "icmp4", dataplane.SPActionBypass, ipsec.SPDDirOut, tc.protocol, -1, tc.prefix, tc.prefix))
			if err := verifyICMPOverlap(cfg); err != nil {
				t.Errorf("a bypass entry that matches no ICMP of the peer's family was refused: %v", err)
			}
		})
		t.Run(tc.name+" discard", func(t *testing.T) {
			cfg := icmpOverlapConfig(icmpOverlapPeer(t, tc.v6, false), dataplane.SPActionDiscard,
				icmpOverlapPolicy(t, "drop", dataplane.SPActionDiscard, ipsec.SPDDirIn, tc.protocol, -1, tc.prefix, tc.prefix))
			if err := verifyICMPOverlap(cfg); err != nil {
				t.Errorf("a discard entry that matches no ICMP of the peer's family was refused: %v", err)
			}
		})
	}
}

// VALIDATES: an inbound discard entry that matches ICMP is refused at verify beside a
// peer with icmp-error-check true, whatever its prefixes and its ICMP type: discard in
// protocol 1, discard both protocol 0, and discard in icmp-type 8 (echo request). The
// fix names the leaf and deleting the entry, and no change of its direction.
// PREVENTS: an operator discard entry that Linux icmp_rcv overrides for ICMP arriving on
// the peer's Child SA, an echo request included, and a fix that turns the entry into
// one with another meaning.
func TestICMPOverlapDiscardRefused(t *testing.T) {
	discard, in, both := dataplane.SPActionDiscard, ipsec.SPDDirIn, ipsec.SPDDirBoth
	cases := []struct {
		name   string
		policy ipsec.SPDPolicy
	}{
		{"icmp in", icmpOverlapPolicy(t, "drop", discard, in, 1, -1, "192.0.2.0/24", "198.51.100.0/24")},
		{"any protocol both", icmpOverlapPolicy(t, "drop", discard, both, 0, -1, "0.0.0.0/0", "0.0.0.0/0")},
		{"echo request in", icmpOverlapPolicy(t, "drop", discard, in, 1, 8, "0.0.0.0/0", "0.0.0.0/0")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := icmpOverlapConfig(icmpOverlapPeer(t, false, false), dataplane.SPActionDiscard, tc.policy)
			err := verifyICMPOverlap(cfg)
			requireICMPOverlapRefusal(t, err,
				`ipsec peer "site"`, "`vpn ipsec policy drop`",
				"set vpn ipsec site-to-site peer site icmp-error-check false",
				"delete vpn ipsec policy drop")
			requireICMPOverlapFixOmits(t, err, "direction")
		})
	}
}

// VALIDATES: a discard entry the kernel's inbound ICMP re-check cannot override passes
// verify: an out-only entry, a UDP entry, a peer with icmp-error-check false, an entry of
// a family the peer's selectors do not carry, `unmatched discard` with no entry, and an
// entry beside a `vti bind` peer.
// PREVENTS: the refusal reaching the catch-all, whose override for a consistent error
// is the Section 6.2 processing itself, an entry that matches no ICMP, or a route-based
// peer, whose inbound ICMP is policy-checked at its interface's if_id.
func TestICMPOverlapDiscardAccepted(t *testing.T) {
	discard, out, in := dataplane.SPActionDiscard, ipsec.SPDDirOut, ipsec.SPDDirIn
	cases := []struct {
		name     string
		peer     ipsec.SiteToSitePeer
		policies []ipsec.SPDPolicy
	}{
		{"direction out", icmpOverlapPeer(t, false, false),
			[]ipsec.SPDPolicy{icmpOverlapPolicy(t, "drop", discard, out, 1, -1, "0.0.0.0/0", "0.0.0.0/0")}},
		{"udp", icmpOverlapPeer(t, false, false),
			[]ipsec.SPDPolicy{icmpOverlapPolicy(t, "drop", discard, in, 17, -1, "0.0.0.0/0", "0.0.0.0/0")}},
		{"icmp-error-check false", icmpOverlapPeer(t, false, true),
			[]ipsec.SPDPolicy{icmpOverlapPolicy(t, "drop", discard, in, 1, -1, "0.0.0.0/0", "0.0.0.0/0")}},
		{"family mismatch", icmpOverlapPeer(t, true, false),
			[]ipsec.SPDPolicy{icmpOverlapPolicy(t, "drop", discard, in, 1, -1, "0.0.0.0/0", "0.0.0.0/0")}},
		{"unmatched discard alone", icmpOverlapPeer(t, false, false), nil},
		{"vti bind", icmpOverlapVTIPeer(t),
			[]ipsec.SPDPolicy{icmpOverlapPolicy(t, "drop", discard, in, 1, -1, "0.0.0.0/0", "0.0.0.0/0")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := icmpOverlapConfig(tc.peer, dataplane.SPActionDiscard, tc.policies...)
			if err := verifyICMPOverlap(cfg); err != nil {
				t.Errorf("a discard entry the kernel cannot override was refused: %v", err)
			}
		})
	}
}

// VALIDATES: a started configuration holding both conflicts logs each at Error level
// with its fix and the code `ze doctor` reports it under (config-plugin-verify), and one
// holding none logs nothing.
// PREVENTS: a box that boots a conflicting file, which no commit refused, running with
// it in silence.
func TestICMPOverlapSaidAtStart(t *testing.T) {
	var logged bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logged, nil))
	cfg := icmpOverlapConfig(icmpOverlapPeer(t, false, false), dataplane.SPActionBypass,
		icmpOverlapPolicy(t, "drop", dataplane.SPActionDiscard, ipsec.SPDDirIn, 1, -1, "0.0.0.0/0", "0.0.0.0/0"))
	warnICMPOverlapAtStart(cfg, log)
	text := logged.String()
	if got := strings.Count(text, "level=ERROR"); got != 2 {
		t.Fatalf("the start logged %d Error lines, want 2 (the bypass catch-all and the discard entry):\n%s", got, text)
	}
	for _, want := range []string{"`vpn ipsec unmatched bypass`", "`vpn ipsec policy drop`",
		"code=config-plugin-verify", "set vpn ipsec site-to-site peer site icmp-error-check false"} {
		if !strings.Contains(text, want) {
			t.Errorf("the start log does not carry %q:\n%s", want, text)
		}
	}

	logged.Reset()
	warnICMPOverlapAtStart(icmpOverlapConfig(icmpOverlapPeer(t, false, true), dataplane.SPActionBypass), log)
	if logged.Len() != 0 {
		t.Errorf("a configuration with no conflict logged at start:\n%s", logged.String())
	}
}

// icmpOverlapFixtures answers dualFamilyCPEConfig under `unmatched bypass`, as the
// route-based peer it is and as the same peer with its `vti bind` removed.
func icmpOverlapFixtures(t *testing.T) (routeBased, policyBased string) {
	t.Helper()
	routeBased = strings.Replace(dualFamilyCPEConfig, `"interface": "eth0",`, `"interface": "eth0", "unmatched": "bypass",`, 1)
	if routeBased == dualFamilyCPEConfig {
		t.Fatal("the fixture no longer holds the interface leaf this test writes unmatched beside")
	}
	policyBased = strings.Replace(routeBased, `"vti": {"bind": "vti0"},`, "", 1)
	if policyBased == routeBased {
		t.Fatal("the fixture no longer holds the vti bind this test removes")
	}
	return routeBased, policyBased
}

// VALIDATES: validateIPsecSections, the offline verifier that `ze config validate`, every
// commit door and OnConfigVerify run, refuses `unmatched bypass` beside a policy-based
// peer with icmp-error-check true, with an operator error errors.As finds, and accepts
// the same configuration while the peer is bound to an xfrm interface.
// PREVENTS: the refusal running only at the daemon's commit, so that `ze config validate`
// passes a configuration the commit refuses.
func TestICMPOverlapRefusedOffline(t *testing.T) {
	routeBased, policyBased := icmpOverlapFixtures(t)
	if err := validateIPsecSections(vpnSection(routeBased)); err != nil {
		t.Fatalf("unmatched bypass beside a vti bind peer failed config verify: %v", err)
	}
	requireICMPOverlapRefusal(t, validateIPsecSections(vpnSection(policyBased)),
		`ipsec peer "cpe"`, "`vpn ipsec unmatched bypass`",
		"set vpn ipsec site-to-site peer cpe icmp-error-check false")
}

// VALIDATES: `ze doctor`'s config-semantics check (config.ValidateSemantics, which runs
// every plugin's offline verifier over the running configuration) reports a policy-based
// peer with icmp-error-check true beside `unmatched bypass` once, under
// config-plugin-verify at error severity with the fix, and reports nothing about it for
// the same peer bound to an xfrm interface.
// PREVENTS: a conflict the box started with, which no commit refused, that `ze doctor`
// does not report.
func TestICMPOverlapDoctorReports(t *testing.T) {
	tree := func(body string) *config.Tree {
		var raw map[string]any
		if err := json.Unmarshal([]byte(body), &raw); err != nil {
			t.Fatalf("the fixture is not JSON: %v", err)
		}
		parsed, err := config.TreeFromPluginMap(raw)
		if err != nil {
			t.Fatalf("the fixture does not build a config tree: %v", err)
		}
		return parsed
	}
	overlap := func(body string) []diagnostic.Diagnostic {
		var found []diagnostic.Diagnostic
		for _, diag := range config.ValidateSemantics(tree(body)) {
			if strings.Contains(diag.Message, "icmp-error-check") {
				found = append(found, diag)
			}
		}
		return found
	}
	routeBased, policyBased := icmpOverlapFixtures(t)

	diags := overlap(policyBased)
	if len(diags) != 1 {
		t.Fatalf("the doctor answered %d icmp-error-check findings, want 1: %+v", len(diags), diags)
	}
	if diags[0].Code != diagnostic.CodeConfigPluginVerify || diags[0].Severity != diagnostic.SeverityError {
		t.Errorf("finding code %q severity %q, want %s error", diags[0].Code, diags[0].Severity, diagnostic.CodeConfigPluginVerify)
	}
	for _, want := range []string{"`vpn ipsec unmatched bypass`", "set vpn ipsec site-to-site peer cpe icmp-error-check false"} {
		if !strings.Contains(diags[0].Message, want) {
			t.Errorf("the finding does not carry %q: %q", want, diags[0].Message)
		}
	}
	if diags := overlap(routeBased); len(diags) != 0 {
		t.Errorf("a vti bind peer drew icmp-error-check findings: %+v", diags)
	}
}
