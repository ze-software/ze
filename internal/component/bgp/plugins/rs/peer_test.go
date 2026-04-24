package rs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// --- PeerState.HasCapability ---

// VALIDATES: HasCapability returns false when Capabilities map is nil.
// PREVENTS: Nil-map panic on peer that never received OPEN.
func TestHasCapability_NilMap(t *testing.T) {
	p := &PeerState{Capabilities: nil}
	assert.False(t, p.HasCapability("route-refresh"))
}

// VALIDATES: HasCapability returns true for a capability that is present and true.
// PREVENTS: Capability lookup failing on populated map.
func TestHasCapability_Present(t *testing.T) {
	p := &PeerState{Capabilities: map[string]bool{"route-refresh": true}}
	assert.True(t, p.HasCapability("route-refresh"))
}

// VALIDATES: HasCapability returns false for an absent key.
// PREVENTS: False positive on capabilities never negotiated.
func TestHasCapability_Absent(t *testing.T) {
	p := &PeerState{Capabilities: map[string]bool{"route-refresh": true}}
	assert.False(t, p.HasCapability("add-path"))
}

// VALIDATES: HasCapability returns false when the capability is explicitly false.
// PREVENTS: Treating explicitly-disabled capability as present.
func TestHasCapability_ExplicitlyFalse(t *testing.T) {
	p := &PeerState{Capabilities: map[string]bool{"route-refresh": false}}
	assert.False(t, p.HasCapability("route-refresh"))
}

// --- PeerState.SupportsFamily ---

// VALIDATES: SupportsFamily returns true when Families map is nil (accept-all).
// PREVENTS: Dropping routes during the window between state-up and OPEN processing.
func TestSupportsFamily_NilMapAcceptsAll(t *testing.T) {
	p := &PeerState{Families: nil}
	assert.True(t, p.SupportsFamily(family.IPv4Unicast))
	assert.True(t, p.SupportsFamily(family.IPv6Unicast))
}

// VALIDATES: SupportsFamily returns true for a supported family.
// PREVENTS: Route filtering on families that were negotiated.
func TestSupportsFamily_Supported(t *testing.T) {
	p := &PeerState{Families: map[family.Family]bool{family.IPv4Unicast: true, family.IPv6Unicast: true}}
	assert.True(t, p.SupportsFamily(family.IPv4Unicast))
}

// VALIDATES: SupportsFamily returns false for an unsupported family.
// PREVENTS: Sending updates for families the peer cannot process.
func TestSupportsFamily_Unsupported(t *testing.T) {
	p := &PeerState{Families: map[family.Family]bool{family.IPv4Unicast: true}}
	assert.False(t, p.SupportsFamily(family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}))
}

// --- capabilityPresent ---

// VALIDATES: capabilityPresent returns true when a matching capability code exists.
// PREVENTS: OPEN processing missing a negotiated capability.
func TestCapabilityPresent_Found(t *testing.T) {
	caps := []CapabilityInfo{
		{Code: 2, Name: "route-refresh", Value: ""},
		{Code: 65, Name: "four-octet-as", Value: "65001"},
	}
	assert.True(t, capabilityPresent(caps, 2))  // route-refresh
	assert.True(t, capabilityPresent(caps, 65)) // four-octet-as
}

// VALIDATES: capabilityPresent returns false when the capability is absent.
// PREVENTS: False positives on capabilities not in OPEN.
func TestCapabilityPresent_NotFound(t *testing.T) {
	caps := []CapabilityInfo{
		{Code: 2, Name: "route-refresh", Value: ""},
	}
	assert.False(t, capabilityPresent(caps, 69)) // add-path
}

// VALIDATES: capabilityPresent handles empty and nil slices.
// PREVENTS: Panic on OPEN with no capabilities.
func TestCapabilityPresent_EmptyAndNil(t *testing.T) {
	assert.False(t, capabilityPresent(nil, 2))
	assert.False(t, capabilityPresent([]CapabilityInfo{}, 2))
}

// --- buildNLRIEntries ---

// VALIDATES: buildNLRIEntries returns nil for empty token list.
// PREVENTS: Spurious empty NLRI entries on empty input.
func TestBuildNLRIEntries_Empty(t *testing.T) {
	result := buildNLRIEntries(nil)
	assert.Nil(t, result)

	result = buildNLRIEntries([]string{})
	assert.Nil(t, result)
}

// VALIDATES: buildNLRIEntries splits comma-separated values with type prefix.
// PREVENTS: Multi-NLRI announces losing individual prefixes.
func TestBuildNLRIEntries_CommaSplit(t *testing.T) {
	tokens := []string{"prefix", "10.0.0.0/24,10.0.1.0/24"}
	result := buildNLRIEntries(tokens)
	require.Len(t, result, 2)
	assert.Equal(t, "prefix 10.0.0.0/24", result[0])
	assert.Equal(t, "prefix 10.0.1.0/24", result[1])
}

// VALIDATES: buildNLRIEntries handles comma in first token (no type prefix).
// PREVENTS: Crash when comma token is at position 0.
func TestBuildNLRIEntries_CommaNoPrefix(t *testing.T) {
	tokens := []string{"10.0.0.0/24,10.0.1.0/24"}
	result := buildNLRIEntries(tokens)
	require.Len(t, result, 2)
	assert.Equal(t, "10.0.0.0/24", result[0])
	assert.Equal(t, "10.0.1.0/24", result[1])
}

// VALIDATES: buildNLRIEntries splits on keyword boundary when token[0] is an NLRI type keyword.
// PREVENTS: Repeated NLRI type keywords being merged into a single entry.
func TestBuildNLRIEntries_KeywordBoundary(t *testing.T) {
	tokens := []string{"prefix", "10.0.0.0/24", "prefix", "10.0.1.0/24"}
	result := buildNLRIEntries(tokens)
	require.Len(t, result, 2)
	assert.Equal(t, "prefix 10.0.0.0/24", result[0])
	assert.Equal(t, "prefix 10.0.1.0/24", result[1])
}

// VALIDATES: buildNLRIEntries joins all tokens into a single entry for non-keyword, non-comma input.
// PREVENTS: Single complex NLRI being split into fragments.
func TestBuildNLRIEntries_SingleComplex(t *testing.T) {
	tokens := []string{"some", "complex", "nlri", "value"}
	result := buildNLRIEntries(tokens)
	require.Len(t, result, 1)
	assert.Equal(t, "some complex nlri value", result[0])
}

// VALIDATES: buildNLRIEntries handles a single token that is an NLRI keyword.
// PREVENTS: Keyword-boundary path producing empty entries when only one keyword group exists.
func TestBuildNLRIEntries_SingleKeyword(t *testing.T) {
	tokens := []string{"prefix", "192.168.0.0/16"}
	result := buildNLRIEntries(tokens)
	require.Len(t, result, 1)
	assert.Equal(t, "prefix 192.168.0.0/16", result[0])
}

// --- nlriKey ---

// VALIDATES: nlriKey strips "prefix " prefix from unicast NLRIs.
// PREVENTS: Withdrawal map keys containing redundant type keyword.
func TestNlriKey_StripsPrefixKeyword(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "ipv4 prefix", input: "prefix 10.0.0.0/24", want: "10.0.0.0/24"},
		{name: "ipv6 prefix", input: "prefix 2001:db8::/32", want: "2001:db8::/32"},
		{name: "non-prefix", input: "rd 65001:1 10.0.0.0/24", want: "rd 65001:1 10.0.0.0/24"},
		{name: "empty", input: "", want: ""},
		{name: "prefix only", input: "prefix ", want: ""},
		{name: "prefix without space", input: "prefixfoo", want: "prefixfoo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, nlriKey(tt.input))
		})
	}
}

// --- isUnicast ---

// VALIDATES: isUnicast returns true for IPv4Unicast and IPv6Unicast only.
// PREVENTS: Zero-alloc NLRI path being used for non-unicast families.
func TestIsUnicast(t *testing.T) {
	tests := []struct {
		name string
		fam  family.Family
		want bool
	}{
		{name: "ipv4/unicast", fam: family.IPv4Unicast, want: true},
		{name: "ipv6/unicast", fam: family.IPv6Unicast, want: true},
		{name: "ipv4/multicast", fam: family.IPv4Multicast, want: false},
		{name: "ipv6/multicast", fam: family.IPv6Multicast, want: false},
		{name: "zero family", fam: family.Family{}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isUnicast(tt.fam))
		})
	}
}
