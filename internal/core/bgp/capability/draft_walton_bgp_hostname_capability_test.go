// Conformance tests for draft-walton-bgp-hostname-capability (FQDN Capability, code 73)
// at the session receive and negotiation boundary.
//
// The draft text is rfc/drafts/draft-walton-bgp-hostname-capability.txt and the
// extracted summary is rfc/short/draft-walton-bgp-hostname-capability.md.

package capability

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fqdnSessionCaps(fqdn *FQDN) []Capability {
	caps := []Capability{&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast}}
	if fqdn != nil {
		caps = append(caps, fqdn)
	}
	return append(caps,
		&Multiprotocol{AFI: AFIIPv6, SAFI: SAFIUnicast},
		&ASN4{ASN: 65002},
		&RouteRefresh{},
		&ExtendedMessage{},
	)
}

// RFC requirement: DRAFT-WALTON-BGP-HOSTNAME-CAPABILITY-4-1 negative -- the FQDN
// Capability is used only for displaying the hostname and/or domain name of a speaker
// for troubleshooting and makes no session or routing decision: an OPEN carrying code 73
// negotiates the same families in the same order, the same Identity and Encoding, the
// same protocol flags, and zero mismatches compared to the same OPEN without it.
func TestFQDNCapabilityDecidesNothing(t *testing.T) {
	t.Parallel()
	local := fqdnSessionCaps(nil)

	without := Negotiate(local, fqdnSessionCaps(nil), PeerIdentity{LocalASN: 65001, PeerASN: 65002})
	with := Negotiate(local, fqdnSessionCaps(&FQDN{Hostname: "spine-01", DomainName: "dc1.example"}), PeerIdentity{LocalASN: 65001, PeerASN: 65002})

	require.NotNil(t, without)
	require.NotNil(t, with)
	assert.Equal(t, without.Families(), with.Families(), "code 73 must not change negotiated families or their order")
	assert.Equal(t, without.Identity, with.Identity)
	assert.Equal(t, without.Encoding, with.Encoding)
	assert.Equal(t, without.ASN4, with.ASN4)
	assert.Equal(t, without.RouteRefresh, with.RouteRefresh)
	assert.Equal(t, without.ExtendedMessageRecv, with.ExtendedMessageRecv)
	assert.Equal(t, without.ExtendedMessageSend, with.ExtendedMessageSend)
	assert.Equal(t, without.EnhancedRouteRefresh, with.EnhancedRouteRefresh)
	assert.Equal(t, without.BFDStrictMode, with.BFDStrictMode)
	assert.Equal(t, without.LinkLocalNextHop, with.LinkLocalNextHop)
	assert.Equal(t, without.HoldTime, with.HoldTime)
	assert.Equal(t, without.GracefulRestart, with.GracefulRestart)
	assert.Empty(t, with.Mismatches, "unilateral code 73 must not record a capability mismatch")

	// Local advertising code 73 while peer does not must also produce zero mismatches.
	localOnly := Negotiate(
		fqdnSessionCaps(&FQDN{Hostname: "ze-local", DomainName: "lab.example"}),
		fqdnSessionCaps(nil),
		PeerIdentity{LocalASN: 65001, PeerASN: 65002},
	)
	assert.Empty(t, localOnly.Mismatches, "local-only code 73 must not record a mismatch")
	assert.Nil(t, localOnly.PeerFQDN, "local FQDN must never be stored as PeerFQDN")

	// The only differences on Negotiated are PeerFQDN, Session.PeerFQDN, and peerCodes[CodeFQDN].
	expectedCodes := make(map[Code]bool, len(without.peerCodes)+1)
	maps.Copy(expectedCodes, without.peerCodes)
	expectedCodes[CodeFQDN] = true
	assert.Equal(t, expectedCodes, with.peerCodes)

	withoutCleared, withCleared := *without, *with
	withoutCleared.peerCodes, withCleared.peerCodes = nil, nil
	withoutCleared.PeerFQDN, withCleared.PeerFQDN = nil, nil
	withoutSession, withSession := *without.Session, *with.Session
	withoutSession.PeerFQDN, withSession.PeerFQDN = nil, nil
	withoutCleared.Session, withCleared.Session = &withoutSession, &withSession
	assert.Equal(t, withoutCleared, withCleared, "code 73 must change nothing negotiated beyond PeerFQDN and peerCodes")
}

// RFC requirement: DRAFT-WALTON-BGP-HOSTNAME-CAPABILITY-4-1 positive -- a received FQDN
// capability (code 73) is preserved on Negotiated.PeerFQDN and Session.PeerFQDN for
// operator troubleshooting display, unilaterally even when the local speaker does not
// advertise code 73.
func TestFQDNCapabilityUnilateralReceiveAndDisplay(t *testing.T) {
	t.Parallel()
	id := PeerIdentity{LocalASN: 65001, PeerASN: 65002}

	tests := []struct {
		name           string
		remote         []Capability
		want           *FQDN
		wantAdvertised bool
	}{
		{
			name:           "hostname_and_domain",
			remote:         []Capability{&FQDN{Hostname: "r2", DomainName: "example.net"}},
			want:           &FQDN{Hostname: "r2", DomainName: "example.net"},
			wantAdvertised: true,
		},
		{
			name:           "hostname_only",
			remote:         []Capability{&FQDN{Hostname: "leaf-04"}},
			want:           &FQDN{Hostname: "leaf-04"},
			wantAdvertised: true,
		},
		{
			name:           "domain_only",
			remote:         []Capability{&FQDN{DomainName: "core.example"}},
			want:           &FQDN{DomainName: "core.example"},
			wantAdvertised: true,
		},
		{
			name:           "utf8_internationalized_names",
			remote:         []Capability{&FQDN{Hostname: "routeur-é1", DomainName: "réseau.example"}},
			want:           &FQDN{Hostname: "routeur-é1", DomainName: "réseau.example"},
			wantAdvertised: true,
		},
		{
			name:           "empty_both_omits_display_value",
			remote:         []Capability{&FQDN{Hostname: "", DomainName: ""}},
			want:           nil,
			wantAdvertised: true,
		},
		{
			name: "duplicate_last_wins_rfc5492",
			remote: []Capability{
				&FQDN{Hostname: "old-host", DomainName: "old.example"},
				&FQDN{Hostname: "new-host", DomainName: "new.example"},
			},
			want:           &FQDN{Hostname: "new-host", DomainName: "new.example"},
			wantAdvertised: true,
		},
		{
			name: "duplicate_empty_last_clears_previous",
			remote: []Capability{
				&FQDN{Hostname: "old-host", DomainName: "old.example"},
				&FQDN{},
			},
			want:           nil,
			wantAdvertised: true,
		},
		{
			name:           "absent",
			remote:         nil,
			want:           nil,
			wantAdvertised: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			neg := Negotiate(nil, tt.remote, id)
			assert.Equal(t, tt.want, neg.PeerFQDN)
			require.NotNil(t, neg.Session)
			assert.Equal(t, tt.want, neg.Session.PeerFQDN)
			assert.Equal(t, tt.wantAdvertised, neg.PeerAdvertised(CodeFQDN))
			if tt.wantAdvertised {
				assert.Empty(t, neg.CheckRequiredCodes([]Code{CodeFQDN}))
				assert.Equal(t, []Code{CodeFQDN}, neg.CheckRefusedCodes([]Code{CodeFQDN}))
			} else {
				assert.Equal(t, []Code{CodeFQDN}, neg.CheckRequiredCodes([]Code{CodeFQDN}))
				assert.Empty(t, neg.CheckRefusedCodes([]Code{CodeFQDN}))
			}
		})
	}
}

// TestFQDNCapabilitySanitizesUntrustedPeerStrings verifies that malformed UTF-8,
// terminal escape sequences, control characters, and Unicode format characters
// sent by a remote BGP peer cannot reach operator display surfaces.
func TestFQDNCapabilitySanitizesUntrustedPeerStrings(t *testing.T) {
	t.Parallel()
	id := PeerIdentity{LocalASN: 65001, PeerASN: 65002}

	tests := []struct {
		name     string
		hostname string
		domain   string
		want     *FQDN
	}{
		{
			name:     "ansi_escape_in_hostname_discarded_domain_preserved",
			hostname: "\x1b[31mevil\x1b[0m",
			domain:   "safe.example",
			want:     &FQDN{Hostname: "", DomainName: "safe.example"},
		},
		{
			name:     "c1_csi_escape_discarded",
			hostname: "r1\u009b2J",
			domain:   "safe.example",
			want:     &FQDN{Hostname: "", DomainName: "safe.example"},
		},
		{
			name:     "newline_and_cr_discarded",
			hostname: "r1\nfake-row",
			domain:   "d1\rd2",
			want:     nil,
		},
		{
			name:     "nul_and_del_discarded",
			hostname: "r1\x00",
			domain:   "d1\x7f",
			want:     nil,
		},
		{
			name:     "bidi_override_format_rune_discarded",
			hostname: "r1\u202esafe",
			domain:   "",
			want:     nil,
		},
		{
			name:     "unicode_line_and_paragraph_separators_discarded",
			hostname: "r1\u2028r2",
			domain:   "d1\u2029d2",
			want:     nil,
		},
		{
			name:     "invalid_utf8_discarded_valid_half_kept",
			hostname: string([]byte{0xff, 0xfe, 'a'}),
			domain:   "valid.example",
			want:     &FQDN{Hostname: "", DomainName: "valid.example"},
		},
		{
			name:     "both_invalid_utf8_returns_nil",
			hostname: string([]byte{0x80}),
			domain:   string([]byte{0xc0, 0xaf}),
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := &FQDN{Hostname: tt.hostname, DomainName: tt.domain}
			neg := Negotiate(nil, []Capability{in}, id)
			assert.Equal(t, tt.want, neg.PeerFQDN)
			assert.True(t, neg.PeerAdvertised(CodeFQDN), "wire capability presence stays recorded even when display text is rejected")
		})
	}

	// Mutating the input FQDN after Negotiate must not alias Negotiated.PeerFQDN.
	src := &FQDN{Hostname: "edge1", DomainName: "example.net"}
	neg := Negotiate(nil, []Capability{src}, id)
	src.Hostname = "mutated"
	require.NotNil(t, neg.PeerFQDN)
	assert.Equal(t, "edge1", neg.PeerFQDN.Hostname)
}
