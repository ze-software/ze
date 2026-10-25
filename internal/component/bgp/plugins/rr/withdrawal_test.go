package rr

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// TestPrefixBytesToKey_IPv4 verifies IPv4 prefix conversion from wire bytes to string key.
//
// VALIDATES: walkUnicastNLRIs produces correct route keys for IPv4.
// PREVENTS: Withdrawal map keyed on corrupted prefix strings.
func TestPrefixBytesToKey_IPv4(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		prefix []byte
		want   string
	}{
		{"24-bit", []byte{24, 10, 0, 0}, "10.0.0.0/24"},
		{"16-bit", []byte{16, 172, 16}, "172.16.0.0/16"},
		{"32-bit", []byte{32, 192, 168, 1, 1}, "192.168.1.1/32"},
		{"0-bit", []byte{0}, "0.0.0.0/0"},
		{"empty", []byte{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := prefixBytesToKey(tt.prefix, false)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestPrefixBytesToKey_IPv6 verifies IPv6 prefix conversion from wire bytes to string key.
//
// VALIDATES: walkUnicastNLRIs produces correct route keys for IPv6.
// PREVENTS: IPv6 prefix bytes misinterpreted as IPv4.
func TestPrefixBytesToKey_IPv6(t *testing.T) {
	t.Parallel()
	// 2001:db8:1::/48 = bitLen=48, bytes=20 01 0d b8 00 01
	prefix := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}
	got := prefixBytesToKey(prefix, true)
	assert.Equal(t, "2001:db8:1::/48", got)
}

// TestNlriKey verifies prefix keyword stripping for route key compaction.
//
// VALIDATES: nlriKey strips "prefix " for unicast, preserves other NLRI types.
// PREVENTS: Withdrawal map key mismatch between add and del for same route.
func TestNlriKey(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "10.0.0.0/24", nlriKey("prefix 10.0.0.0/24"))
	assert.Equal(t, "rd 65000:1 prefix 10.0.0.0/24", nlriKey("rd 65000:1 prefix 10.0.0.0/24"))
	assert.Equal(t, "2001:db8::/32", nlriKey("prefix 2001:db8::/32"))
}

// TestHandleStateDown_BatchesByFamily takes down a source holding 501 IPv4
// and two IPv6 unicast routes, through handleStateDown and the departure
// worker, and reads the commands the engine received.
//
// VALIDATES: the departure is batched per family, at most withdrawalBatchSize
// routes per command (two IPv4 commands, one IPv6), and every route carries
// its own del keyword, so the engine parses each one as a prefix
// (nlriDelCmd); the withdrawal map is cleared for the downed peer.
// PREVENTS: one RPC per route on peer-down with a large table, and the
// comma-joined form ("del prefix 10.0.0.0/24,prefix 10.0.0.1/32") the engine
// refused whole as one invalid prefix, so a client kept every unicast route
// of a departed source that held more than one.
func TestHandleStateDown_BatchesByFamily(t *testing.T) {
	const source = "10.0.0.1"
	rr, requests := newPeerDownReflector(t)
	routes := make(map[string]withdrawalInfo)
	ipv4 := make(map[string]bool)
	for i := range withdrawalBatchSize + 1 {
		prefix := fmt.Sprintf("prefix 10.%d.%d.0/24", i/256, i%256)
		routes["ipv4/unicast|"+prefix] = withdrawalInfo{Family: family.IPv4Unicast, Prefix: prefix}
		ipv4[prefix] = true
	}
	routes["ipv6/unicast|2001:db8::/32"] = withdrawalInfo{Family: family.IPv6Unicast, Prefix: "prefix 2001:db8::/32"}
	routes["ipv6/unicast|2001:db8:1::/48"] = withdrawalInfo{Family: family.IPv6Unicast, Prefix: "prefix 2001:db8:1::/48"}
	rr.withdrawals[source] = routes

	rr.handleStateDown(source)
	rr.stopDepartures()
	close(requests)

	var batchSizes []int
	var ipv6Commands []string
	for request := range requests {
		if rest, ok := strings.CutPrefix(request.command, "update text nlri"); ok {
			words := strings.Split(rest, " del ")
			require.Empty(t, words[0], "IPv4 command %q has text before its first del", request.command)
			for _, prefix := range words[1:] {
				require.True(t, ipv4[prefix], "IPv4 command names %q, which is not one route the source held", prefix)
				delete(ipv4, prefix)
			}
			batchSizes = append(batchSizes, len(words)-1)
			continue
		}
		ipv6Commands = append(ipv6Commands, request.command)
	}
	assert.Empty(t, ipv4, "IPv4 routes no command withdrew")
	slices.Sort(batchSizes)
	assert.Equal(t, []int{1, withdrawalBatchSize}, batchSizes, "IPv4 routes per command")
	assert.Equal(t, []string{"update text mp ipv6/unicast del prefix 2001:db8:1::/48 del prefix 2001:db8::/32"}, ipv6Commands)
	assert.Nil(t, rr.withdrawals[source], "withdrawal map cleared for downed peer")
}
