package engine

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// cookieTree builds a vpn ipsec section carrying cookie-threshold and a peer list.
// respondPeers peers are set to respond, and initiatePeers peers are set to start.
func cookieTree(threshold string, respondPeers, initiatePeers int) *config.Tree {
	ipsecRoot := config.NewTree()
	ipsecRoot.Set("interface", "eth0")
	if threshold != "" {
		ipsecRoot.Set("cookie-threshold", threshold)
	}
	sts := ipsecRoot.GetOrCreateContainer("site-to-site")
	add := func(name, connType string) {
		peer := config.NewTree()
		peer.Set("connection-type", connType)
		peer.Set("remote-address", "203.0.113.7")
		sts.AddListEntry("peer", name, peer)
	}
	for i := range respondPeers {
		add("responder"+string(rune('a'+i)), "respond")
	}
	for i := range initiatePeers {
		add("initiator"+string(rune('a'+i)), "initiate")
	}
	vpnRoot := config.NewTree()
	vpnRoot.SetContainer("ipsec", ipsecRoot)
	root := config.NewTree()
	root.SetContainer("vpn", vpnRoot)
	return root
}

// VALIDATES: a cookie-threshold above the number of responding peers is reported.
// halfOpenResponderCount cannot climb past that number, so the challenge of RFC 7296
// Section 2.6 never fires.
// PREVENTS: an operator raising the threshold, seeing the config commit, and running
// with the state-exhaustion defense silently off.
func TestCookieThresholdDoctorReportsUnreachable(t *testing.T) {
	diags := checkIPsecCookieThreshold(registry.DoctorCheckContext{
		Tree: cookieTree("5", 2, 3),
	})
	if len(diags) != 1 {
		t.Fatalf("an unreachable cookie-threshold produced %d diagnostics, want 1", len(diags))
	}
	if diags[0].Code != "doctor-ipsec-cookie-threshold" {
		t.Errorf("code is %q, want doctor-ipsec-cookie-threshold", diags[0].Code)
	}
	if diags[0].Severity != "warning" {
		t.Errorf("severity is %q; the value becomes reachable when peers are added, "+
			"so it is not an error", diags[0].Severity)
	}
	// The message must carry both numbers. Without them the operator cannot tell what
	// to lower the threshold to (ai/rules/error-messages.md).
	for _, want := range []string{"5", "2", "cookie-threshold"} {
		if !strings.Contains(diags[0].Message, want) {
			t.Errorf("the message does not name %q: %s", want, diags[0].Message)
		}
	}
	// An initiating peer never takes a responder slot, so counting it would report a
	// reachable ceiling that does not exist.
	if strings.Contains(diags[0].Message, " 5 half-open") {
		t.Error("the ceiling counted initiating peers, which hold no responder slot")
	}
}

// VALIDATES: a threshold the responder can reach, and the default of zero, are silent.
// PREVENTS: a check that warns on every configuration, which trains an operator to
// ignore it.
func TestCookieThresholdDoctorSilentWhenReachable(t *testing.T) {
	for _, tc := range []struct {
		name string
		tree *config.Tree
	}{
		{"threshold equals the responder count", cookieTree("3", 3, 0)},
		{"threshold below the responder count", cookieTree("1", 3, 0)},
		{"default zero challenges everything", cookieTree("0", 0, 0)},
		{"leaf absent", cookieTree("", 0, 2)},
		{"no vpn section", config.NewTree()},
		{"nil tree", (*config.Tree)(nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if diags := checkIPsecCookieThreshold(registry.DoctorCheckContext{Tree: tc.tree}); len(diags) != 0 {
				t.Errorf("a reachable threshold produced %d diagnostics: %+v", len(diags), diags)
			}
		})
	}
	if diags := checkIPsecCookieThreshold(registry.DoctorCheckContext{Tree: "not a tree"}); len(diags) != 0 {
		t.Errorf("a tree of the wrong type produced %d diagnostics", len(diags))
	}
}
