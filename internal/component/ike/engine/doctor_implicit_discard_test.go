package engine

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// implicitDiscardTree builds a vpn ipsec section with the unmatched leaf set to the
// given value, or left unset when unmatched is empty.
func implicitDiscardTree(unmatched string) *config.Tree {
	ipsecRoot := config.NewTree()
	ipsecRoot.Set("interface", "eth0")
	if unmatched != "" {
		ipsecRoot.Set("unmatched", unmatched)
	}
	vpnRoot := config.NewTree()
	vpnRoot.SetContainer("ipsec", ipsecRoot)
	root := config.NewTree()
	root.SetContainer("vpn", vpnRoot)
	return root
}

// TestImplicitDiscardDoctorReports checks that `ze doctor` reports the discard
// catch-all in force only because `vpn ipsec unmatched` is unset, and only then.
//
// VALIDATES: checkIPsecImplicitDiscard answers one error, under the code the start log
// carries, with the facts and the confirming fix; a written discard or bypass answers
// nothing.
// PREVENTS: a box that booted an unverified configuration dropping its own clear
// traffic with nothing but a start log line long scrolled away to say why.
func TestImplicitDiscardDoctorReports(t *testing.T) {
	diags := checkIPsecImplicitDiscard(registry.DoctorCheckContext{Tree: implicitDiscardTree("")})
	if len(diags) != 1 {
		t.Fatalf("unmatched unset produced %d diagnostics, want 1", len(diags))
	}
	d := diags[0]
	if d.Code != "doctor-ipsec-implicit-discard" {
		t.Errorf("code is %q, want doctor-ipsec-implicit-discard", d.Code)
	}
	if d.Severity != "error" {
		t.Errorf("severity is %q, want error: the traffic is being dropped now", d.Severity)
	}
	if d.Fix != "set vpn ipsec unmatched discard" {
		t.Errorf("fix is %q, want the confirming command", d.Fix)
	}
	for _, want := range []string{
		"`vpn ipsec unmatched` is not set",
		"RFC 4301 Section 4.4.1",
		"set vpn ipsec unmatched bypass",
	} {
		if !strings.Contains(d.Message, want) {
			t.Errorf("message does not say %q: %s", want, d.Message)
		}
	}

	for _, written := range []string{"discard", "bypass"} {
		if got := checkIPsecImplicitDiscard(registry.DoctorCheckContext{Tree: implicitDiscardTree(written)}); len(got) != 0 {
			t.Errorf("unmatched %s written produced %d diagnostics, want none: %+v", written, len(got), got)
		}
	}
}

// TestImplicitDiscardDoctorWithoutTree checks that a check context holding no
// parsed configuration answers a warning saying the check cannot answer.
//
// VALIDATES: checkIPsecImplicitDiscard answers one warning under its own code, saying
// it holds no parsed configuration, with the command that prints the running one.
// PREVENTS: a check that cannot read the configuration answering nothing, which reads
// as "no discard in force".
func TestImplicitDiscardDoctorWithoutTree(t *testing.T) {
	diags := checkIPsecImplicitDiscard(registry.DoctorCheckContext{Tree: "not a tree"})
	if len(diags) != 1 {
		t.Fatalf("a context with no parsed tree produced %d diagnostics, want 1", len(diags))
	}
	d := diags[0]
	if d.Code != "doctor-ipsec-implicit-discard" {
		t.Errorf("code is %q, want doctor-ipsec-implicit-discard", d.Code)
	}
	if d.Severity != "warning" {
		t.Errorf("severity is %q, want warning: nothing is known to be dropped", d.Severity)
	}
	if !strings.Contains(d.Message, "holds no parsed configuration") {
		t.Errorf("message does not say the configuration could not be read: %s", d.Message)
	}
	if d.Fix != "show config running" {
		t.Errorf("fix is %q, want the command that prints the running configuration", d.Fix)
	}
}
