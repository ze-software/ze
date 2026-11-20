// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions, transit traffic
// Related: spd_forward_bypass.go -- verifyForwardBypass, warnForwardBypassAtStart and the doctor check under test

package engine

import (
	"bytes"
	"errors"
	"log/slog"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

// forwardBypassProtocol is TCP, the protocol of the entries these tests write.
const forwardBypassProtocol = 6

// zeHostAddress is the address the tests' host holds (heldAddressesFn).
var zeHostAddress = netip.MustParseAddr("192.0.2.1")

// holdAddresses makes heldAddressesFn answer addrs for the life of the test.
func holdAddresses(t *testing.T, addrs ...netip.Addr) {
	t.Helper()
	saved := heldAddressesFn
	t.Cleanup(func() { heldAddressesFn = saved })
	heldAddressesFn = func() (map[netip.Addr]bool, error) {
		held := make(map[netip.Addr]bool, len(addrs))
		for _, a := range addrs {
			held[a] = true
		}
		return held, nil
	}
}

// forwardBypassEntry is an operator entry from local to 198.51.100.0/24 over TCP, with
// local port 22 when port is true and both ports any when it is false.
func forwardBypassEntry(t *testing.T, name string, action dataplane.SPAction, local string, port bool) ipsec.SPDPolicy {
	t.Helper()
	p := ipsec.SPDPolicy{
		Name:         name,
		Action:       action,
		Order:        1000,
		Direction:    ipsec.SPDDirBoth,
		Protocol:     forwardBypassProtocol,
		LocalPrefix:  mustCIDR(t, local),
		LocalPort:    ipsec.AnyPort(),
		RemotePrefix: mustCIDR(t, "198.51.100.0/24"),
		RemotePort:   ipsec.AnyPort(),
	}
	if port {
		p.LocalPort = ipsec.PortSelector{Form: ipsec.PortSingle, Port: 22}
	}
	return p
}

// VALIDATES: owner decision of 2026-11-20 on open decision 2 of the RFC 4301
// architecture spec ("refusal until done"): a bypass with a port that forwarded traffic
// can match is refused at commit on a dataplane with a forward lookup, naming the entry,
// why, and the any-port form that is accepted. RFC 4301 Section 7.4: "All
// implementations MUST support stateful fragment checking to accommodate BYPASS traffic
// for which a non-trivial port range is specified."
// METHOD: verifyForwardBypass on a forward-lookup dataplane (XFRM's shape), over a
// port-scoped bypass whose local side is a protected subnet; then the cases just outside
// the refusal: the same entry with both ports any, the same entry as a discard, the same
// entry on a dataplane without a forward lookup (VPP's shape), no dataplane at all, and
// a port-scoped bypass whose local prefix is the one address the host holds.
// PREVENTS: a commit that installs a bypass forwarded traffic never meets
// (spdEntryHasTransit projects none), and a refusal so wide it removes a bypass that
// works: one naming only Ze's own traffic, or one on a backend that decides forwarded
// traffic in its inbound SPD.
func TestRFC4301PortScopedForwardBypassRefusedAtCommit(t *testing.T) {
	holdAddresses(t, zeHostAddress)
	forward := &forwardPolicyDP{spdDP: newSPDDP()}
	commit := func(dp dataplane.Dataplane, p ipsec.SPDPolicy) error {
		return verifyForwardBypass(dp, &ipsec.IPsecConfig{Policies: map[string]ipsec.SPDPolicy{p.Name: p}})
	}

	err := commit(forward, forwardBypassEntry(t, "ssh-hosts", dataplane.SPActionBypass, "10.1.0.0/16", true))
	if err == nil {
		t.Fatal("a port-scoped bypass for a protected subnet committed on a dataplane with a forward lookup; " +
			"forwarded traffic on its port would meet the next entry instead")
	}
	var refusal *opererr.Error
	if !errors.As(err, &refusal) {
		t.Fatalf("the refusal is %T, want an operator error: %v", err, err)
	}
	if !strings.Contains(refusal.Problem(), "ssh-hosts") {
		t.Errorf("the refusal does not name the entry: %q", refusal.Problem())
	}
	if !strings.Contains(refusal.Why(), "RFC 4301 Section 7.4") || !strings.Contains(refusal.Why(), "forwarded") {
		t.Errorf("the refusal does not say why: %q", refusal.Why())
	}
	if want := "set vpn ipsec policy ssh-hosts local port any"; refusal.Fix() != want {
		t.Errorf("the refusal's fix is %q, want %q", refusal.Fix(), want)
	}

	accepted := []struct {
		name  string
		dp    dataplane.Dataplane
		entry ipsec.SPDPolicy
	}{
		{"any-port bypass", forward, forwardBypassEntry(t, "all-hosts", dataplane.SPActionBypass, "10.1.0.0/16", false)},
		{"port-scoped discard", forward, forwardBypassEntry(t, "drop-ssh", dataplane.SPActionDiscard, "10.1.0.0/16", true)},
		{"no forward lookup", newSPDDP(), forwardBypassEntry(t, "ssh-hosts", dataplane.SPActionBypass, "10.1.0.0/16", true)},
		{"no dataplane", nil, forwardBypassEntry(t, "ssh-hosts", dataplane.SPActionBypass, "10.1.0.0/16", true)},
		{"local prefix held by the host", forward, forwardBypassEntry(t, "ssh-ze", dataplane.SPActionBypass, "192.0.2.1/32", true)},
	}
	for _, tc := range accepted {
		if err := commit(tc.dp, tc.entry); err != nil {
			t.Errorf("%s: refused, want accepted: %v", tc.name, err)
		}
	}

	// A host prefix the host does not hold names a forwarded host, so it is refused.
	if err := commit(forward, forwardBypassEntry(t, "ssh-server", dataplane.SPActionBypass, "192.0.2.9/32", true)); err == nil {
		t.Error("a port-scoped bypass for a host address this node does not hold committed; traffic forwarded to it would miss the entry")
	}
}

// VALIDATES: a port-scoped bypass that names a port on both sides is refused with the
// local side's fix and told to clear the remote port too, and a host whose addresses
// cannot be read refuses rather than exempts.
// METHOD: verifyForwardBypass over an entry with both ports, then over an entry for the
// host's own address while heldAddressesFn fails.
// PREVENTS: a fix that leaves the entry still refused, and an unreadable address set
// read as "this is Ze's own address".
func TestPortScopedForwardBypassRefusalCoversBothPortsAndUnreadableAddresses(t *testing.T) {
	forward := &forwardPolicyDP{spdDP: newSPDDP()}
	holdAddresses(t, zeHostAddress)
	both := forwardBypassEntry(t, "web", dataplane.SPActionBypass, "10.1.0.0/16", true)
	both.RemotePort = ipsec.PortSelector{Form: ipsec.PortSingle, Port: 443}
	err := verifyForwardBypass(forward, &ipsec.IPsecConfig{Policies: map[string]ipsec.SPDPolicy{"web": both}})
	var refusal *opererr.Error
	if !errors.As(err, &refusal) {
		t.Fatalf("an entry with both ports was not refused with an operator error: %v", err)
	}
	if !strings.Contains(refusal.Why(), "Set remote port any as well") {
		t.Errorf("the refusal does not name the second port to clear: %q", refusal.Why())
	}

	errUnreadable := errors.New("netlink dump refused by test")
	heldAddressesFn = func() (map[netip.Addr]bool, error) { return nil, errUnreadable }
	own := forwardBypassEntry(t, "ssh-ze", dataplane.SPActionBypass, "192.0.2.1/32", true)
	err = verifyForwardBypass(forward, &ipsec.IPsecConfig{Policies: map[string]ipsec.SPDPolicy{"ssh-ze": own}})
	if !errors.Is(err, errUnreadable) {
		t.Fatalf("with the host's addresses unreadable the entry was not refused with the read error: %v", err)
	}
}

// VALIDATES: the start half of the ruling, under owner decision 3 (a) of 2026-11-19:
// a start never refuses the IPsec block, so it installs a port-scoped bypass that
// forwarded traffic can match, logs it at Error under its doctor code, and the
// ipsec-spd-forward-bypass doctor check reports it with the any-port fix; an entry
// naming the host's own address is neither logged nor reported.
// METHOD: start on a forward-lookup backend with both entries, read the log and the
// check, then remove the host's address and read the check again.
// PREVENTS: the commit refusal running at start (the access-security door trap "A
// refusal added for reload also ran at startup"), and an entry the commit would refuse
// going unreported on a box that booted with it.
func TestPortScopedForwardBypassAtStartIsLoggedAndReportedByDoctor(t *testing.T) {
	resetSPDLeft(t)
	holdAddresses(t, zeHostAddress)
	dp := &forwardPolicyDP{spdDP: newSPDDP()}
	loadFailClosedBackend(t, dp)
	var logged bytes.Buffer
	state := applyTestState(t)
	state.log = slog.New(slog.NewTextHandler(&logged, nil))
	transit := forwardBypassEntry(t, "ssh-hosts", dataplane.SPActionBypass, "10.1.0.0/16", true)
	own := forwardBypassEntry(t, "ssh-ze", dataplane.SPActionBypass, "192.0.2.1/32", true)
	cfg := withPolicies(lifecycleConfig(t), map[string]ipsec.SPDPolicy{"ssh-hosts": transit, "ssh-ze": own})
	cfg.Unmatched = dataplane.SPActionBypass
	// forwardPolicyDP cannot refuse per-socket IPsec policies, so the default deny would
	// refuse the start (applyApplicationOverride); allow keeps that guard out of the subject.
	cfg.ApplicationOverride = ipsec.ApplicationOverrideAllow

	if err := state.applyConfig(cfg, applyStartup); err != nil {
		t.Fatalf("the start refused the IPsec block: %v", err)
	}
	out := logged.String()
	if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "vpn ipsec policy ssh-hosts is a bypass with a port") {
		t.Fatalf("the start did not log the port-scoped bypass at Error:\n%s", out)
	}
	if !strings.Contains(out, diagnostic.CodeDoctorIPsecSPDForwardBypass) {
		t.Fatalf("the start's log line carries no doctor code %s:\n%s", diagnostic.CodeDoctorIPsecSPDForwardBypass, out)
	}
	if strings.Contains(out, "vpn ipsec policy ssh-ze is a bypass") {
		t.Fatalf("the start logged an entry that names only the host's own address:\n%s", out)
	}

	findings := answerSPDForwardBypass(state)
	if len(findings) != 1 {
		t.Fatalf("the doctor check answered %d findings, want 1 for ssh-hosts: %+v", len(findings), findings)
	}
	finding := findings[0]
	if finding.Code != diagnostic.CodeDoctorIPsecSPDForwardBypass || finding.Severity != severityError {
		t.Fatalf("the finding is %s/%s, want %s/%s", finding.Severity, finding.Code, severityError, diagnostic.CodeDoctorIPsecSPDForwardBypass)
	}
	if !strings.Contains(finding.Message, "ssh-hosts") || finding.Fix != "set vpn ipsec policy ssh-hosts local port any" {
		t.Fatalf("the finding does not name the entry and its fix: %+v", finding)
	}

	holdAddresses(t)
	if findings := answerSPDForwardBypass(state); len(findings) != 2 {
		t.Fatalf("with the host's address gone the check answered %d findings, want 2: %+v", len(findings), findings)
	}
}
