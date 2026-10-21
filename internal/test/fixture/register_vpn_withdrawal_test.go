package fixture

import (
	"slices"
	"strings"
	"testing"
)

// TestVPNWithdrawalPeerArgs checks the `held <peer address>` and `release
// <peer address>` pairs the inventory fixture reads after its role and final
// UPDATE count.
//
// VALIDATES: each well-formed pair yields its address in its own list, in
// argument order, and no pair yields neither.
// PREVENTS: a malformed pair, an unknown keyword or a non-address being read as
// a peer the fixture waits for or releases, which would hang the test until its
// timeout instead of refusing.
func TestVPNWithdrawalPeerArgs(t *testing.T) {
	accepted := []struct {
		args    []string
		held    []string
		release []string
	}{
		{args: []string{"rr"}},
		{args: []string{"rr", "3"}},
		{args: []string{"rr", "3", "held", "127.0.0.5"}, held: []string{"127.0.0.5"}},
		{args: []string{"rs", "4", "held", "127.0.0.5", "held", "::1"}, held: []string{"127.0.0.5", "::1"}},
		{
			args:    []string{"rr", "3", "release", "127.0.0.5", "release", "127.0.0.6", "held", "127.0.0.5"},
			held:    []string{"127.0.0.5"},
			release: []string{"127.0.0.5", "127.0.0.6"},
		},
	}
	for _, tc := range accepted {
		got, err := vpnWithdrawalPeerArgs(tc.args)
		if err != nil {
			t.Fatalf("vpnWithdrawalPeerArgs(%q): %v", tc.args, err)
		}
		if !slices.Equal(got.held, tc.held) {
			t.Fatalf("vpnWithdrawalPeerArgs(%q) held = %q, want %q", tc.args, got.held, tc.held)
		}
		if !slices.Equal(got.release, tc.release) {
			t.Fatalf("vpnWithdrawalPeerArgs(%q) release = %q, want %q", tc.args, got.release, tc.release)
		}
	}

	refused := []struct {
		args []string
		want string
	}{
		{args: []string{"rr", "3", "held"}, want: "`release <peer address>` pairs"},
		{args: []string{"rr", "3", "release"}, want: "`release <peer address>` pairs"},
		{args: []string{"rr", "3", "keep", "127.0.0.5"}, want: "`held <peer address>` or"},
		{args: []string{"rr", "3", "held", "sourceb"}, want: "held peer \"sourceb\" is not an IP address"},
		{args: []string{"rr", "3", "release", "sourceb"}, want: "release peer \"sourceb\" is not an IP address"},
	}
	for _, tc := range refused {
		_, err := vpnWithdrawalPeerArgs(tc.args)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("vpnWithdrawalPeerArgs(%q) error = %v, want one containing %q", tc.args, err, tc.want)
		}
	}
}
