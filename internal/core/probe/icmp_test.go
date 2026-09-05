package probe

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"testing"
)

// TestBuildICMPEchoChecksum verifies the built packet carries the type, id, and
// sequence in the right offsets and a checksum that makes the full packet sum to
// zero (one's-complement), the standard ICMP validity property.
func TestBuildICMPEchoChecksum(t *testing.T) {
	pkt := BuildICMPEcho(8, 0x1234, 0x0007, []byte("ze-ping"))
	if pkt[0] != 8 {
		t.Fatalf("type = %d, want 8", pkt[0])
	}
	if got := uint16(pkt[4])<<8 | uint16(pkt[5]); got != 0x1234 {
		t.Errorf("id = %#x, want 0x1234", got)
	}
	if got := uint16(pkt[6])<<8 | uint16(pkt[7]); got != 0x0007 {
		t.Errorf("seq = %#x, want 0x0007", got)
	}
	// Verifying checksum: summing all 16-bit words (including the checksum
	// field) in one's complement must yield 0xffff.
	var sum uint32
	for i := 0; i+1 < len(pkt); i += 2 {
		sum += uint32(pkt[i])<<8 | uint32(pkt[i+1])
	}
	if len(pkt)%2 == 1 {
		sum += uint32(pkt[len(pkt)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum >> 16) + (sum & 0xffff)
	}
	if uint16(sum) != 0xffff {
		t.Errorf("checksum folded sum = %#x, want 0xffff", uint16(sum))
	}
}

// TestResolveTargetLiteral verifies an IP literal resolves to itself without DNS.
func TestResolveTargetLiteral(t *testing.T) {
	for _, lit := range []string{"192.0.2.1", "2001:db8::1"} {
		got, err := ResolveTarget(lit, FamilyAny)
		if err != nil {
			t.Fatalf("ResolveTarget(%q): %v", lit, err)
		}
		want := netip.MustParseAddr(lit)
		if got != want {
			t.Errorf("ResolveTarget(%q) = %v, want %v", lit, got, want)
		}
	}
}

// TestFamilyOf verifies the mapping from a source address to the family that
// constrains resolution: the zero address constrains nothing, and an
// IPv4-mapped IPv6 address is IPv4 because that is the socket family it binds.
func TestFamilyOf(t *testing.T) {
	cases := []struct {
		addr netip.Addr
		want Family
	}{
		{netip.Addr{}, FamilyAny},
		{netip.MustParseAddr("192.0.2.1"), FamilyIPv4},
		{netip.MustParseAddr("2001:db8::1"), FamilyIPv6},
		{netip.MustParseAddr("::ffff:192.0.2.1"), FamilyIPv4},
		{netip.MustParseAddr("fe80::1%eth0"), FamilyIPv6},
	}
	for _, tc := range cases {
		if got := FamilyOf(tc.addr); got != tc.want {
			t.Errorf("FamilyOf(%v) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}

// TestFamilyNetwork verifies each family names the net.Resolver network that
// holds a lookup to it. The string is what reaches LookupNetIP, so a wrong one
// silently drops the constraint.
func TestFamilyNetwork(t *testing.T) {
	cases := map[Family]string{FamilyAny: "ip", FamilyIPv4: "ip4", FamilyIPv6: "ip6"}
	for family, want := range cases {
		if got := family.network(); got != want {
			t.Errorf("%v.network() = %q, want %q", family, got, want)
		}
	}
}

// TestResolveTargetLiteralFamilyMismatch verifies a literal target of the wrong
// family is refused with ErrFamilyMismatch, so the conflict is named where the
// operator can see both arguments rather than at the socket bind.
func TestResolveTargetLiteralFamilyMismatch(t *testing.T) {
	cases := []struct {
		target string
		family Family
	}{
		{"192.0.2.1", FamilyIPv6},
		{"2001:db8::1", FamilyIPv4},
	}
	for _, tc := range cases {
		got, err := ResolveTarget(tc.target, tc.family)
		if !errors.Is(err, ErrFamilyMismatch) {
			t.Errorf("ResolveTarget(%q, %v) = %v, %v; want ErrFamilyMismatch", tc.target, tc.family, got, err)
		}
	}
}

// TestResolveTargetUnmapsLiteral verifies an IPv4-mapped IPv6 literal comes back
// as IPv4. The caller picks the socket family from the returned address, and a
// mapped address reports Is6, which would open an ICMPv6 socket for an IPv4
// destination.
func TestResolveTargetUnmapsLiteral(t *testing.T) {
	got, err := ResolveTarget("::ffff:192.0.2.1", FamilyIPv4)
	if err != nil {
		t.Fatalf("ResolveTarget: %v", err)
	}
	if !got.Is4() {
		t.Errorf("ResolveTarget(\"::ffff:192.0.2.1\") = %v, want the unmapped IPv4 form", got)
	}
}

// TestResolveTargetFamilyHint verifies a family-constrained hostname lookup
// never answers with an address of the other family. The invariant holds in
// every environment: a host whose "localhost" carries both families answers in
// the asked family, and a host that carries only one answers ErrFamilyMismatch
// for the other. Answering with the other family is the defect this constrains,
// because the caller then binds a source the socket cannot carry.
func TestResolveTargetFamilyHint(t *testing.T) {
	for _, family := range []Family{FamilyIPv4, FamilyIPv6} {
		got, err := ResolveTarget("localhost", family)
		if err != nil {
			if !errors.Is(err, ErrFamilyMismatch) {
				t.Errorf("ResolveTarget(localhost, %v): %v; want an address or ErrFamilyMismatch", family, err)
			}
			continue
		}
		if FamilyOf(got) != family {
			t.Errorf("ResolveTarget(localhost, %v) = %v, which is %v", family, got, FamilyOf(got))
		}
		// LookupNetIP answers an IPv4 address in the IPv4-mapped IPv6 form,
		// which reports Is6. The caller reads the socket family off this
		// address, so a mapped answer opens an ICMPv6 socket for IPv4.
		if got != got.Unmap() {
			t.Errorf("ResolveTarget(localhost, %v) = %v, want the unmapped form %v", family, got, got.Unmap())
		}
	}
}

// TestFamilyAbsent verifies the classifier that decides whether a
// family-constrained lookup failure is a family conflict or a broken resolver.
// It is the guard between "your source and your target disagree" and a DNS
// failure the source had nothing to do with, and only the first may be blamed
// on the source address.
func TestFamilyAbsent(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		// The shape LookupNetIP returns for a name whose records are all of
		// the other family, which is AC-3: an A-only name asked for in IPv6.
		{"no suitable address", &net.AddrError{Err: "no suitable address found", Addr: "v4only.example"}, true},
		{"not found", &net.DNSError{Err: "no such host", Name: "v4only.example", IsNotFound: true}, true},
		{"timeout", &net.DNSError{Err: "i/o timeout", Name: "slow.example", IsTimeout: true}, false},
		{"server failure", &net.DNSError{Err: "server misbehaving", Name: "broken.example"}, false},
		{"unrelated", errors.New("connection refused"), false},
		{"wrapped not found", fmt.Errorf("lookup: %w", &net.DNSError{Err: "no such host", IsNotFound: true}), true},
	}
	for _, tc := range cases {
		if got := familyAbsent(tc.err); got != tc.want {
			t.Errorf("familyAbsent(%s) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// checksumOnesFold sums b as 16-bit big-endian words with end-around carry and
// returns the folded 16-bit value. A message whose checksum field is correct
// folds to 0xffff (the RFC 1071 verification property), so this is the mirror of
// the encoder's icmpChecksum used to assert RFC 792's checksum obligation.
func checksumOnesFold(b []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(b); i += 2 {
		sum += uint32(b[i])<<8 | uint32(b[i+1])
	}
	if len(b)%2 == 1 {
		sum += uint32(b[len(b)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum >> 16) + (sum & 0xffff)
	}
	return uint16(sum)
}

// RFC requirement: RFC792-Echo-1 positive -- a ze-built ICMP echo request carries Type 8.
func TestRFC792EchoRequestType(t *testing.T) {
	if got := BuildICMPEcho(8, 0x1234, 7, []byte("ze-ping"))[0]; got != 8 {
		t.Errorf("echo request Type = %d, want 8", got)
	}
}

// RFC requirement: RFC792-Echo-2 positive -- a ze-built ICMP echo request carries Code 0.
func TestRFC792EchoRequestCode(t *testing.T) {
	if got := BuildICMPEcho(8, 0x1234, 7, []byte("ze-ping"))[1]; got != 0 {
		t.Errorf("echo request Code = %d, want 0", got)
	}
}

// RFC requirement: RFC792-Echo-3 positive -- the built echo carries a checksum that makes
// the one's-complement sum over the whole message fold to 0xffff.
// RFC requirement: RFC1071-1-5 positive -- summing every 16-bit word including the checksum field folds to 0xffff, the RFC 1071 receive test; icmpChecksum makes it hold (probe/icmp.go:34,39-49).
func TestRFC792ChecksumValid(t *testing.T) {
	pkt := BuildICMPEcho(8, 0x1234, 7, []byte("ze-ping"))
	if got := checksumOnesFold(pkt); got != 0xffff {
		t.Errorf("checksum fold = %#x, want 0xffff", got)
	}
}

// RFC requirement: RFC792-Echo-3 negative -- altering a message byte after the checksum is
// computed breaks the property, so a corrupted echo does not carry a valid checksum.
// RFC requirement: RFC1071-1-5 negative -- flipping a payload byte makes the whole-message one's-complement sum no longer fold to 0xffff, so the RFC 1071 verify test rejects it (invariant established by icmpChecksum, probe/icmp.go:34).
func TestRFC792ChecksumRejectsCorruption(t *testing.T) {
	pkt := BuildICMPEcho(8, 0x1234, 7, []byte("ze-ping"))
	pkt[9] ^= 0xff // flip a payload byte; leave the checksum field intact
	if got := checksumOnesFold(pkt); got == 0xffff {
		t.Errorf("corrupted echo still folds to 0xffff; checksum does not detect the change")
	}
}

// RFC requirement: RFC792-Echo-4 positive -- an odd total length is padded with one zero
// octet for the computation, so an odd-length payload still yields a valid checksum.
// RFC requirement: RFC1071-1-4 positive -- the packet stays odd length (the pad is not transmitted) yet its one's-complement sum with a single implicit zero-octet pad folds to 0xffff (icmpChecksum odd-byte path, probe/icmp.go:44-45).
func TestRFC792ChecksumOddLength(t *testing.T) {
	pkt := BuildICMPEcho(8, 0x1234, 7, []byte("odd")) // 8 + 3 = 11 bytes, odd
	if len(pkt)%2 == 0 {
		t.Fatalf("want an odd-length packet to exercise padding, got len %d", len(pkt))
	}
	if got := checksumOnesFold(pkt); got != 0xffff {
		t.Errorf("odd-length checksum fold = %#x, want 0xffff", got)
	}
}
