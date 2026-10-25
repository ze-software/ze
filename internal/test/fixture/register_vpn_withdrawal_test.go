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

// TestVPNWithdrawalGatesPair checks the `gates <n>` pair that sets how many
// source actions the inventory fixture releases.
//
// VALIDATES: no pair gives the default of three gates, and 3 to 6 (the number
// of gate prefixes) are read as given, beside the peer pairs.
// PREVENTS: a gate count the fixture has no prefix for, or a malformed one,
// being read as a count, which would index past the gate prefixes or wait for
// a gate no peer script expects.
func TestVPNWithdrawalGatesPair(t *testing.T) {
	accepted := []struct {
		args  []string
		gates int
	}{
		{args: []string{"rr"}, gates: 3},
		{args: []string{"rr", "5", "release", "127.0.0.5"}, gates: 3},
		{args: []string{"rr", "5", "gates", "3"}, gates: 3},
		{args: []string{"rr", "5", "gates", "4", "release", "127.0.0.5"}, gates: 4},
		{args: []string{"rr", "6", "gates", "5"}, gates: 5},
		{args: []string{"rr", "7", "gates", "6"}, gates: 6},
	}
	for _, tc := range accepted {
		got, err := vpnWithdrawalPeerArgs(tc.args)
		if err != nil {
			t.Fatalf("vpnWithdrawalPeerArgs(%q): %v", tc.args, err)
		}
		if got.gates != tc.gates {
			t.Fatalf("vpnWithdrawalPeerArgs(%q) gates = %d, want %d", tc.args, got.gates, tc.gates)
		}
	}

	refused := []struct {
		args []string
		want string
	}{
		{args: []string{"rr", "5", "gates", "2"}, want: "gates 2 is below 3"},
		{args: []string{"rr", "5", "gates", "7"}, want: "gates 7 is above 6"},
		{args: []string{"rr", "5", "gates", "four"}, want: "gates \"four\" is not a number"},
		{args: []string{"rr", "5", "gates"}, want: "`gates <n>`"},
	}
	for _, tc := range refused {
		_, err := vpnWithdrawalPeerArgs(tc.args)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("vpnWithdrawalPeerArgs(%q) error = %v, want one containing %q", tc.args, err, tc.want)
		}
	}
}
