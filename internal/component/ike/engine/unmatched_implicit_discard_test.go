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
