// VALIDATES: the commit check that guards the discard default of `vpn ipsec
// unmatched`: a commit that would turn the discard catch-all on while the leaf is
// unset is refused, with a message naming what stays open and the commands that fix
// it; every other combination passes.
// PREVENTS: an operator configuring `vpn ipsec` over SSH and cutting the router's own
// control plane with a default nobody wrote, and the check widening into a refusal of
// the default itself (a box already discarding, or a written leaf).

package engine

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// implicitDiscardConfig parses `vpn ipsec` with the given `unmatched` value (absent
// when empty) and the given number of bypass policy entries.
func implicitDiscardConfig(t *testing.T, value string, bypassEntries int) *ipsec.IPsecConfig {
	t.Helper()
	tree := config.NewTree()
	vpn := tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")
	if value != "" {
		vpn.Set("unmatched", value)
	}
	names := []string{"ssh", "bgp", "dns"}
	for i := range bypassEntries {
		entry := config.NewTree()
		entry.Set("action", "bypass")
		entry.GetOrCreateContainer("local").Set("prefix", "192.0.2.0/24")
		entry.GetOrCreateContainer("remote").Set("prefix", "198.51.100.0/24")
		vpn.AddListEntry("policy", names[i], entry)
	}
	cfg, err := ipsec.ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig(unmatched %q): %v", value, err)
	}
	return cfg
}

// TestImplicitDiscardRefusedAtCommit drives refuseImplicitDiscard over the parsed
// configuration, the value OnConfigVerify hands it. Method: one case per input that
// decides the answer; the refusal's text must name the leaf, the RFC section, what
// is cut, what stays open (with the operator's bypass count), and both commands.
func TestImplicitDiscardRefusedAtCommit(t *testing.T) {
	cases := []struct {
		name           string
		value          string
		discardInForce bool
		refused        bool
	}{
		{"leaf unset, bypass in force: refused", "", false, true},
		{"leaf unset, discard already in force: accepted", "", true, false},
		{"written discard: accepted", "discard", false, false},
		{"written bypass: accepted", "bypass", false, false},
	}
	for _, c := range cases {
		err := refuseImplicitDiscard(implicitDiscardConfig(t, c.value, 0), c.discardInForce)
		if c.refused != (err != nil) {
			t.Fatalf("%s: refused = %v (%v), want %v", c.name, err != nil, err, c.refused)
		}
	}

	err := refuseImplicitDiscard(implicitDiscardConfig(t, "", 2), false)
	if err == nil {
		t.Fatal("leaf unset with two bypass entries: accepted, want the refusal")
	}
	for _, want := range []string{
		"`vpn ipsec unmatched` is not set",
		"RFC 4301 Section 4.4.1",
		"BGP, SSH, DNS and NTP",
		"its ICMP (ping, traceroute, `show mtu` probes",
		"its IPv6 neighbor discovery",
		"`set vpn ipsec policy icmp4 protocol 1`",
		"`icmp6` entry with protocol 58 and prefix ::/0",
		"UDP 500 and 4500",
		"the 2 `vpn ipsec policy` bypass entries",
		"set vpn ipsec unmatched discard",
		"set vpn ipsec unmatched bypass",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not say %q: %v", want, err)
		}
	}
}

// TestImplicitDiscardReadsWhatTheLastApplyInstalled covers the input the commit check
// takes from the running engine: discardInForce, which applyConfig stores after the
// catch-all is installed and OnConfigVerify reads. Method: an engine over a backend
// that holds the catch-all applies a configuration, then a leaf-unset configuration
// goes through refuseImplicitDiscard with the engine's discardInForce, as
// OnConfigVerify passes it. After a discard apply it is accepted, because the box
// already discards; after a bypass apply, and on an engine that applied nothing, it is
// refused. A store that never runs, or one that records the wrong disposition, turns
// one of the three red.
func TestImplicitDiscardReadsWhatTheLastApplyInstalled(t *testing.T) {
	loadFailClosedBackend(t, &catchAllDP{})
	unset := implicitDiscardConfig(t, "", 0)

	fresh := applyTestState(t)
	if err := refuseImplicitDiscard(unset, fresh.discardInForce.Load()); err == nil {
		t.Fatal("no apply yet: a leaf-unset commit was accepted, want the refusal")
	}

	cases := []struct {
		name    string
		applied dataplane.SPAction
		refused bool
	}{
		{"after a discard apply: accepted", dataplane.SPActionDiscard, false},
		{"after a bypass apply: refused", dataplane.SPActionBypass, true},
	}
	for _, c := range cases {
		state := applyTestState(t)
		cfg := testIPsecConfig(applyTestPeer("127.0.0.1"))
		cfg.Unmatched = c.applied
		if err := state.applyConfig(cfg, applyReload); err != nil {
			t.Fatalf("%s: apply: %v", c.name, err)
		}
		err := refuseImplicitDiscard(unset, state.discardInForce.Load())
		if c.refused != (err != nil) {
			t.Fatalf("%s: refused = %v (%v), want %v", c.name, err != nil, err, c.refused)
		}
	}
}

// TestImplicitDiscardICMPExampleIsAccepted proves the ICMP bypass the refusal and the
// start log print (implicitDiscardICMPExample) is a configuration Ze accepts and
// installs as written. Method: the icmp4 and icmp6 entries, with the leaf values the
// example names, go through ParseIPsecConfig and ValidateSPDPolicies; each must parse
// as a bypass of its protocol over the wildcard of its family at order 3000.
func TestImplicitDiscardICMPExampleIsAccepted(t *testing.T) {
	tree := config.NewTree()
	vpn := tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")
	vpn.Set("unmatched", "discard")
	examples := []struct {
		name     string
		protocol string
		number   uint8
		prefix   string
	}{
		{"icmp4", "1", 1, "0.0.0.0/0"},
		{"icmp6", "58", 58, "::/0"},
	}
	for _, e := range examples {
		if !strings.Contains(implicitDiscardICMPExample, e.name) {
			t.Fatalf("the printed example does not name the %s entry: %s", e.name, implicitDiscardICMPExample)
		}
		entry := config.NewTree()
		entry.Set("action", "bypass")
		entry.Set("protocol", e.protocol)
		entry.Set("order", "3000")
		entry.GetOrCreateContainer("local").Set("prefix", e.prefix)
		entry.GetOrCreateContainer("remote").Set("prefix", e.prefix)
		vpn.AddListEntry("policy", e.name, entry)
	}
	cfg, err := ipsec.ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: the printed ICMP example is refused: %v", err)
	}
	if err := cfg.ValidateSPDPolicies(); err != nil {
		t.Fatalf("ValidateSPDPolicies: the printed ICMP example is refused: %v", err)
	}
	for _, e := range examples {
		p, ok := cfg.Policies[e.name]
		if !ok {
			t.Fatalf("%s: not parsed", e.name)
		}
		if p.Action != dataplane.SPActionBypass || p.Order != 3000 || p.Protocol != e.number {
			t.Errorf("%s: action %d order %d protocol %d, want bypass at 3000 of protocol %s",
				e.name, p.Action, p.Order, p.Protocol, e.protocol)
		}
		if p.LocalPrefix.String() != e.prefix || p.RemotePrefix.String() != e.prefix {
			t.Errorf("%s: prefixes %s and %s, want %s", e.name, p.LocalPrefix, p.RemotePrefix, e.prefix)
		}
	}
}

// TestNoVPNBlockNeverRefused proves the guard's zero value: a delivery with no `vpn
// ipsec` block (EmptyConfig, bypass) is never refused, whatever is in force.
func TestNoVPNBlockNeverRefused(t *testing.T) {
	for _, inForce := range []bool{false, true} {
		if err := refuseImplicitDiscard(ipsec.EmptyConfig(), inForce); err != nil {
			t.Fatalf("no vpn ipsec block, discard in force %v: refused: %v", inForce, err)
		}
	}
}

// TestImplicitDiscardSaidAtStart covers the configuration the daemon starts with,
// which no commit verifies: warnImplicitDiscardAtStart must log the discard default
// at Error level with the commands that settle it, and stay silent for a written
// leaf and for a box with no `vpn ipsec` block. Method: a text handler over a buffer.
func TestImplicitDiscardSaidAtStart(t *testing.T) {
	cases := []struct {
		name  string
		cfg   *ipsec.IPsecConfig
		wants bool
	}{
		{"leaf unset: said", implicitDiscardConfig(t, "", 1), true},
		{"written discard: silent", implicitDiscardConfig(t, "discard", 0), false},
		{"written bypass: silent", implicitDiscardConfig(t, "bypass", 0), false},
		{"no vpn ipsec block: silent", ipsec.EmptyConfig(), false},
	}
	for _, c := range cases {
		var out bytes.Buffer
		warnImplicitDiscardAtStart(c.cfg, slog.New(slog.NewTextHandler(&out, nil)))
		if c.wants != (out.Len() > 0) {
			t.Fatalf("%s: logged %q, want logged = %v", c.name, out.String(), c.wants)
		}
		if !c.wants {
			continue
		}
		for _, want := range []string{
			"level=ERROR",
			"`vpn ipsec unmatched` is not set",
			"RFC 4301 Section 4.4.1",
			"BGP, SSH, DNS and NTP",
			"its ICMP (ping, traceroute, `show mtu` probes",
			"its IPv6 neighbor discovery",
			"`set vpn ipsec policy icmp4 protocol 1`",
			"`icmp6` entry with protocol 58 and prefix ::/0",
			"set vpn ipsec unmatched discard",
			"set vpn ipsec unmatched bypass",
			"bypass-entries=1",
		} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("%s: start log does not say %q: %s", c.name, want, out.String())
			}
		}
	}
}
