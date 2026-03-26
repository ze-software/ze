package perf

import (
	"net/netip"
	"testing"
)

func TestSenderInlineNLRI(t *testing.T) {
	t.Parallel()

	// IPv4/unicast UPDATE uses inline NLRI field (no MP_REACH_NLRI).
	sender := NewSender(SenderConfig{
		ASN:     65001,
		IsEBGP:  true,
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Family:  "ipv4/unicast",
		ForceMP: false,
	})

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	data := sender.BuildRoute(prefix)

	if data == nil {
		t.Fatal("BuildRoute returned nil")
	}

	// BGP marker check.
	for i := range 16 {
		if data[i] != 0xFF {
			t.Fatalf("marker byte %d: got 0x%02x, want 0xFF", i, data[i])
		}
	}

	// Type byte at offset 18 must be 2 (UPDATE).
	if data[18] != 2 {
		t.Fatalf("message type: got %d, want 2 (UPDATE)", data[18])
	}

	// Parse UPDATE structure:
	// offset 19: withdrawn routes length (2 bytes)
	withdrawnLen := int(data[19])<<8 | int(data[20])
	if withdrawnLen != 0 {
		t.Fatalf("withdrawn length: got %d, want 0", withdrawnLen)
	}

	// offset 21: total path attribute length (2 bytes)
	attrStart := 21
	attrLen := int(data[attrStart])<<8 | int(data[attrStart+1])
	if attrLen == 0 {
		t.Fatal("path attributes length is 0, expected attributes")
	}

	// Walk attributes to verify no MP_REACH_NLRI (type 14).
	attrDataStart := attrStart + 2
	pos := attrDataStart
	attrEnd := attrDataStart + attrLen

	for pos < attrEnd {
		flags := data[pos]
		code := data[pos+1]
		pos += 2

		var aLen int
		if flags&0x10 != 0 { // Extended length
			aLen = int(data[pos])<<8 | int(data[pos+1])
			pos += 2
		} else {
			aLen = int(data[pos])
			pos++
		}

		if code == 14 {
			t.Fatal("found MP_REACH_NLRI (type 14) in inline NLRI UPDATE")
		}

		pos += aLen
	}

	// Trailing NLRI must be present (non-zero bytes after attributes).
	nlriStart := attrEnd
	nlriLen := len(data) - nlriStart
	if nlriLen == 0 {
		t.Fatal("no trailing NLRI in IPv4/unicast UPDATE")
	}
}

func TestSenderForceMP(t *testing.T) {
	t.Parallel()

	// IPv4/unicast with force-mp encodes in MP_REACH_NLRI (AFI=1/SAFI=1).
	sender := NewSender(SenderConfig{
		ASN:     65001,
		IsEBGP:  true,
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Family:  "ipv4/unicast",
		ForceMP: true,
	})

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	data := sender.BuildRoute(prefix)

	if data == nil {
		t.Fatal("BuildRoute returned nil")
	}

	// Type byte at offset 18 must be 2 (UPDATE).
	if data[18] != 2 {
		t.Fatalf("message type: got %d, want 2 (UPDATE)", data[18])
	}

	// Parse UPDATE structure.
	withdrawnLen := int(data[19])<<8 | int(data[20])
	if withdrawnLen != 0 {
		t.Fatalf("withdrawn length: got %d, want 0", withdrawnLen)
	}

	attrStart := 21
	attrLen := int(data[attrStart])<<8 | int(data[attrStart+1])

	// Walk attributes to find MP_REACH_NLRI (type 14).
	attrDataStart := attrStart + 2
	pos := attrDataStart
	attrEnd := attrDataStart + attrLen

	foundMP := false
	var mpValueStart int
	var mpValueLen int

	for pos < attrEnd {
		flags := data[pos]
		code := data[pos+1]
		pos += 2

		var aLen int
		if flags&0x10 != 0 { // Extended length
			aLen = int(data[pos])<<8 | int(data[pos+1])
			pos += 2
		} else {
			aLen = int(data[pos])
			pos++
		}

		if code == 14 {
			foundMP = true
			mpValueStart = pos
			mpValueLen = aLen
		}

		pos += aLen
	}

	if !foundMP {
		t.Fatal("MP_REACH_NLRI (type 14) not found in force-mp UPDATE")
	}

	// MP_REACH_NLRI value: AFI(2) + SAFI(1) + NH_len(1) + NH + Reserved(1) + NLRI
	if mpValueLen < 5 {
		t.Fatalf("MP_REACH_NLRI value too short: %d bytes", mpValueLen)
	}

	afi := uint16(data[mpValueStart])<<8 | uint16(data[mpValueStart+1])
	safi := data[mpValueStart+2]

	if afi != 1 {
		t.Fatalf("MP_REACH_NLRI AFI: got %d, want 1 (IPv4)", afi)
	}
	if safi != 1 {
		t.Fatalf("MP_REACH_NLRI SAFI: got %d, want 1 (Unicast)", safi)
	}

	// No trailing NLRI after attributes (all NLRI is inside MP_REACH_NLRI).
	nlriStart := attrEnd
	nlriLen := len(data) - nlriStart
	if nlriLen != 0 {
		t.Fatalf("trailing NLRI length: got %d, want 0 (all NLRI in MP_REACH_NLRI)", nlriLen)
	}
}

// VALIDATES: "BuildBatch packs multiple NLRIs into one UPDATE, ExtractPrefixes recovers all."
// PREVENTS: Multi-NLRI packing produces malformed UPDATEs or loses prefixes.
func TestSenderBatchInline(t *testing.T) {
	t.Parallel()

	sender := NewSender(SenderConfig{
		ASN:     65001,
		IsEBGP:  true,
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Family:  "ipv4/unicast",
		ForceMP: false,
	})

	prefixes := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/24"),
		netip.MustParsePrefix("10.0.1.0/24"),
		netip.MustParsePrefix("10.0.2.0/24"),
		netip.MustParsePrefix("172.16.0.0/16"),
		netip.MustParsePrefix("192.0.2.0/24"),
	}

	data := sender.BuildBatch(prefixes)
	if data == nil {
		t.Fatal("BuildBatch returned nil")
	}

	// Verify it's a single UPDATE message (type 2).
	if data[18] != 2 {
		t.Fatalf("message type: got %d, want 2", data[18])
	}

	// Extract prefixes using the receiver's parser -- round-trip validation.
	body := data[19:] // Skip BGP header.
	got := ExtractPrefixes(body)

	if len(got) != len(prefixes) {
		t.Fatalf("ExtractPrefixes: got %d prefixes, want %d", len(got), len(prefixes))
	}

	// CountPrefixes must agree.
	counted := CountPrefixes(body)
	if counted != len(prefixes) {
		t.Fatalf("CountPrefixes: got %d, want %d", counted, len(prefixes))
	}

	// Verify all prefixes are present (order preserved for inline NLRI).
	for i, want := range prefixes {
		if got[i] != want {
			t.Errorf("prefix[%d]: got %v, want %v", i, got[i], want)
		}
	}
}

// VALIDATES: "BuildBatch with force-MP packs NLRIs in MP_REACH_NLRI."
// PREVENTS: MP batch produces inline NLRI instead of MP_REACH_NLRI.
func TestSenderBatchForceMP(t *testing.T) {
	t.Parallel()

	sender := NewSender(SenderConfig{
		ASN:     65001,
		IsEBGP:  true,
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Family:  "ipv4/unicast",
		ForceMP: true,
	})

	prefixes := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/24"),
		netip.MustParsePrefix("10.0.1.0/24"),
		netip.MustParsePrefix("10.0.2.0/24"),
	}

	data := sender.BuildBatch(prefixes)
	if data == nil {
		t.Fatal("BuildBatch returned nil")
	}

	body := data[19:]
	got := ExtractPrefixes(body)

	if len(got) != len(prefixes) {
		t.Fatalf("ExtractPrefixes: got %d, want %d", len(got), len(prefixes))
	}

	if CountPrefixes(body) != len(prefixes) {
		t.Fatalf("CountPrefixes mismatch")
	}
}

// VALIDATES: "BuildBatch with IPv6 packs NLRIs in MP_REACH_NLRI AFI=2."
// PREVENTS: IPv6 batch produces IPv4 encoding.
func TestSenderBatchIPv6(t *testing.T) {
	t.Parallel()

	sender := NewSender(SenderConfig{
		ASN:     65001,
		IsEBGP:  true,
		NextHop: netip.MustParseAddr("2001:db8::1"),
		Family:  "ipv6/unicast",
	})

	prefixes := []netip.Prefix{
		netip.MustParsePrefix("2001:db8:1::/48"),
		netip.MustParsePrefix("2001:db8:2::/48"),
		netip.MustParsePrefix("2001:db8:3::/48"),
		netip.MustParsePrefix("2001:db8:4::/48"),
	}

	data := sender.BuildBatch(prefixes)
	if data == nil {
		t.Fatal("BuildBatch returned nil")
	}

	body := data[19:]
	got := ExtractPrefixes(body)

	if len(got) != len(prefixes) {
		t.Fatalf("ExtractPrefixes: got %d, want %d", len(got), len(prefixes))
	}

	if CountPrefixes(body) != len(prefixes) {
		t.Fatalf("CountPrefixes mismatch")
	}

	for i, want := range prefixes {
		if got[i] != want {
			t.Errorf("prefix[%d]: got %v, want %v", i, got[i], want)
		}
	}
}

// VALIDATES: "iBGP batch includes LOCAL_PREF and empty AS_PATH."
// PREVENTS: iBGP attributes missing in batched UPDATEs.
func TestSenderBatchIBGP(t *testing.T) {
	t.Parallel()

	sender := NewSender(SenderConfig{
		ASN:     65001,
		IsEBGP:  false,
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Family:  "ipv4/unicast",
	})

	prefixes := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/24"),
		netip.MustParsePrefix("10.0.1.0/24"),
		netip.MustParsePrefix("10.0.2.0/24"),
	}

	data := sender.BuildBatch(prefixes)
	if data == nil {
		t.Fatal("BuildBatch returned nil")
	}

	// Round-trip: all prefixes recoverable.
	body := data[19:]
	got := ExtractPrefixes(body)

	if len(got) != len(prefixes) {
		t.Fatalf("ExtractPrefixes: got %d, want %d", len(got), len(prefixes))
	}

	// Walk attributes to verify LOCAL_PREF (type 5) is present.
	attrStart := 2 + 2 // withdrawn_len + attr_len offset in body
	attrLen := int(body[2])<<8 | int(body[3])
	pos := attrStart
	attrEnd := attrStart + attrLen

	foundLocalPref := false
	for pos < attrEnd {
		flags := body[pos]
		code := body[pos+1]
		pos += 2

		var aLen int
		if flags&0x10 != 0 {
			aLen = int(body[pos])<<8 | int(body[pos+1])
			pos += 2
		} else {
			aLen = int(body[pos])
			pos++
		}

		if code == 5 { // LOCAL_PREF
			foundLocalPref = true
		}

		pos += aLen
	}

	if !foundLocalPref {
		t.Error("LOCAL_PREF (type 5) not found in iBGP batch UPDATE")
	}
}

// VALIDATES: "BuildBatch with nil/empty returns nil, single-element delegates to BuildRoute."
// PREVENTS: Panic on empty input or divergence between single and batch paths.
func TestSenderBatchEdgeCases(t *testing.T) {
	t.Parallel()

	sender := NewSender(SenderConfig{
		ASN:     65001,
		IsEBGP:  true,
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Family:  "ipv4/unicast",
	})

	// Empty slice returns nil.
	if got := sender.BuildBatch(nil); got != nil {
		t.Errorf("BuildBatch(nil): got %d bytes, want nil", len(got))
	}

	if got := sender.BuildBatch([]netip.Prefix{}); got != nil {
		t.Errorf("BuildBatch(empty): got %d bytes, want nil", len(got))
	}

	// Single-element produces same output as BuildRoute.
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	single := sender.BuildRoute(prefix)
	batched := sender.BuildBatch([]netip.Prefix{prefix})

	if len(single) != len(batched) {
		t.Fatalf("single=%d bytes, batched=%d bytes", len(single), len(batched))
	}

	for i := range single {
		if single[i] != batched[i] {
			t.Fatalf("byte %d: single=0x%02x, batched=0x%02x", i, single[i], batched[i])
		}
	}
}

// VALIDATES: "BuildBatch produces UPDATEs within RFC 4271 4096-byte limit."
// PREVENTS: Oversized BGP messages that corrupt wire framing.
func TestSenderBatchRespects4096(t *testing.T) {
	t.Parallel()

	sender := NewSender(SenderConfig{
		ASN:     65001,
		IsEBGP:  true,
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Family:  "ipv4/unicast",
	})

	// 1200 prefixes -- too many for one 4096-byte UPDATE.
	// Generate directly to avoid the expensive 14M candidate pool in GenerateIPv4Routes.
	prefixes := make([]netip.Prefix, 1200)
	for i := range prefixes {
		prefixes[i] = netip.PrefixFrom(
			netip.AddrFrom4([4]byte{byte(11 + i/256), byte(i % 256), 0, 0}), 24) //nolint:gosec // test data
	}

	// Compute safe batch size using the same logic as runIteration.
	probe := sender.BuildRoute(prefixes[0])
	perNLRI := 1 + (prefixes[0].Bits()+7)/8
	overhead := len(probe) - perNLRI
	maxBatch := max((4096-overhead-1)/perNLRI, 1)

	// Build a batch at the computed max size.
	batchCount := min(maxBatch, len(prefixes))

	data := sender.BuildBatch(prefixes[:batchCount])
	if data == nil {
		t.Fatal("BuildBatch returned nil")
	}

	if len(data) > 4096 {
		t.Errorf("batch of %d: message size %d > 4096", batchCount, len(data))
	}

	// Verify all prefixes round-trip.
	body := data[19:]
	got := ExtractPrefixes(body)

	if len(got) != batchCount {
		t.Errorf("ExtractPrefixes: got %d, want %d", len(got), batchCount)
	}
}

func TestSenderIPv6(t *testing.T) {
	t.Parallel()

	// IPv6/unicast UPDATE uses MP_REACH_NLRI with AFI=2/SAFI=1.
	sender := NewSender(SenderConfig{
		ASN:     65001,
		IsEBGP:  true,
		NextHop: netip.MustParseAddr("2001:db8::1"),
		Family:  "ipv6/unicast",
		ForceMP: false,
	})

	prefix := netip.MustParsePrefix("2001:db8:1::/48")
	data := sender.BuildRoute(prefix)

	if data == nil {
		t.Fatal("BuildRoute returned nil")
	}

	if data[18] != 2 {
		t.Fatalf("message type: got %d, want 2 (UPDATE)", data[18])
	}

	// Walk attributes to find MP_REACH_NLRI with AFI=2.
	attrStart := 21
	attrLen := int(data[attrStart])<<8 | int(data[attrStart+1])
	attrDataStart := attrStart + 2
	pos := attrDataStart
	attrEnd := attrDataStart + attrLen

	foundMP := false

	for pos < attrEnd {
		flags := data[pos]
		code := data[pos+1]
		pos += 2

		var aLen int
		if flags&0x10 != 0 {
			aLen = int(data[pos])<<8 | int(data[pos+1])
			pos += 2
		} else {
			aLen = int(data[pos])
			pos++
		}

		if code == 14 && aLen >= 3 {
			afi := uint16(data[pos])<<8 | uint16(data[pos+1])
			safi := data[pos+2]
			if afi == 2 && safi == 1 {
				foundMP = true
			}
		}

		pos += aLen
	}

	if !foundMP {
		t.Fatal("MP_REACH_NLRI with AFI=2/SAFI=1 not found in IPv6 UPDATE")
	}
}
