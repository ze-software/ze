package wireu

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
)

// benchPayload builds a realistic IPv4 UPDATE payload for benchmarking.
// Contains: ORIGIN + AS_PATH (4 hops) + NEXT_HOP + LOCAL_PREF + N /24 prefixes.
func benchPayload(nlriCount int) []byte {
	// ORIGIN: flags=0x40 (transitive), code=1, len=1, value=0 (IGP)
	origin := buildOriginAttr()

	// AS_PATH: AS_SEQUENCE [64512, 64513, 64514, 64515] (4-byte ASN)
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512, 64513, 64514, 64515}},
	}, true)

	// NEXT_HOP: flags=0x40 (transitive), code=3, len=4, value=192.168.1.1
	nextHop := []byte{0x40, 0x03, 0x04, 192, 168, 1, 1}

	// LOCAL_PREF: flags=0x40 (transitive), code=5, len=4, value=100
	localPref := []byte{0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64}

	attrs := concatAttrs(origin, aspath, nextHop, localPref)

	// NLRI: N IPv4 /24 prefixes (each 4 bytes: prefixLen + 3 octets)
	// 10.0.0.0/24, 10.0.1.0/24, ...
	nlriData := make([]byte, 0, nlriCount*4)
	for i := range nlriCount {
		nlriData = append(nlriData, 24, 10, byte(i>>8), byte(i)) //nolint:gosec // bench data
	}

	return buildPayload(nil, attrs, nlriData)
}

// BenchmarkNewWireUpdate measures the cost of creating a WireUpdate from payload bytes.
// This is called once per received UPDATE in the reactor hot path.
func BenchmarkNewWireUpdate(b *testing.B) {
	payload := benchPayload(10)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		wu := NewWireUpdate(payload, bgpctx.ContextID(1))
		_ = wu
	}
}

// BenchmarkWireUpdateEnsureParsed measures the first-access parse cost.
// This triggers ParseUpdateSections to compute section offsets from wire bytes.
func BenchmarkWireUpdateEnsureParsed(b *testing.B) {
	payload := benchPayload(10)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		wu := NewWireUpdate(payload, bgpctx.ContextID(1))
		wu.ensureParsed()
	}
}

// BenchmarkWireUpdateAttrs measures the Attrs() accessor cost.
// This allocates an AttributesWire wrapper (slice header, no data copy).
func BenchmarkWireUpdateAttrs(b *testing.B) {
	payload := benchPayload(10)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		wu := NewWireUpdate(payload, bgpctx.ContextID(1))
		attrs, err := wu.Attrs()
		if err != nil {
			b.Fatal(err)
		}
		_ = attrs
	}
}

// BenchmarkWireUpdatePayload measures the Payload() accessor cost.
// This should be zero-cost (returns stored slice).
func BenchmarkWireUpdatePayload(b *testing.B) {
	payload := benchPayload(10)
	wu := NewWireUpdate(payload, bgpctx.ContextID(1))

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		p := wu.Payload()
		_ = p
	}
}

// BenchmarkRewriteASPath measures the EBGP AS-PATH prepend cost.
// Called once per UPDATE per EBGP destination peer.
func BenchmarkRewriteASPath(b *testing.B) {
	benchmarks := []struct {
		name    string
		srcASN4 bool
		dstASN4 bool
	}{
		{"ASN4_to_ASN4", true, true},
		{"ASN4_to_ASN2", true, false},
		{"ASN2_to_ASN4", false, true},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Build payload with matching source ASN encoding
			origin := buildOriginAttr()
			aspath := buildASPathAttr([]attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{64512, 64513, 64514, 64515}},
			}, bm.srcASN4)
			nextHop := []byte{0x40, 0x03, 0x04, 192, 168, 1, 1}
			localPref := []byte{0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64}
			attrs := concatAttrs(origin, aspath, nextHop, localPref)
			nlriData := []byte{24, 10, 0, 1, 24, 10, 0, 2, 24, 10, 0, 3}
			payload := buildPayload(nil, attrs, nlriData)

			dst := make([]byte, len(payload)+64)

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				n, err := RewriteASPath(dst, payload, 65000, bm.srcASN4, bm.dstASN4)
				if err != nil {
					b.Fatal(err)
				}
				_ = n
			}
		})
	}
}

// BenchmarkRewriteASPath_NoExisting measures AS-PATH insertion when none exists.
func BenchmarkRewriteASPath_NoExisting(b *testing.B) {
	origin := buildOriginAttr()
	nextHop := []byte{0x40, 0x03, 0x04, 192, 168, 1, 1}
	attrs := concatAttrs(origin, nextHop)
	payload := buildPayload(nil, attrs, []byte{24, 10, 0, 1})

	dst := make([]byte, len(payload)+64)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		n, err := RewriteASPath(dst, payload, 65000, true, true)
		if err != nil {
			b.Fatal(err)
		}
		_ = n
	}
}

// BenchmarkSplitWireUpdate measures UPDATE splitting cost.
// Benchmarked with payloads that do and do not require splitting.
func BenchmarkSplitWireUpdate(b *testing.B) {
	noAddPath := bgpctx.EncodingContextForASN4(true)

	b.Run("NoSplit", func(b *testing.B) {
		// Small payload that fits within max
		payload := benchPayload(5) // ~50 bytes total

		wu := NewWireUpdate(payload, bgpctx.ContextID(1))

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			chunks, err := SplitWireUpdate(wu, 4096, noAddPath)
			if err != nil {
				b.Fatal(err)
			}
			_ = chunks
		}
	})

	b.Run("Split_20_prefixes", func(b *testing.B) {
		// 20 /24 prefixes = 80 bytes NLRI + ~30 bytes attrs + 4 overhead = ~114 bytes
		// Force split with maxBodySize=60
		payload := benchPayload(20)

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			wu := NewWireUpdate(payload, bgpctx.ContextID(1))
			chunks, err := SplitWireUpdate(wu, 60, noAddPath)
			if err != nil {
				b.Fatal(err)
			}
			_ = chunks
		}
	})

	b.Run("Split_100_prefixes", func(b *testing.B) {
		// 100 /24 prefixes = 400 bytes NLRI
		// Force split with maxBodySize=100
		payload := benchPayload(100)

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			wu := NewWireUpdate(payload, bgpctx.ContextID(1))
			chunks, err := SplitWireUpdate(wu, 100, noAddPath)
			if err != nil {
				b.Fatal(err)
			}
			_ = chunks
		}
	})
}

// BenchmarkWireUpdateNLRI measures the NLRI() accessor cost.
func BenchmarkWireUpdateNLRI(b *testing.B) {
	payload := benchPayload(10)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		wu := NewWireUpdate(payload, bgpctx.ContextID(1))
		nlriBytes, err := wu.NLRI()
		if err != nil {
			b.Fatal(err)
		}
		_ = nlriBytes
	}
}

// BenchmarkWireUpdateWithdrawn measures the Withdrawn() accessor cost.
func BenchmarkWireUpdateWithdrawn(b *testing.B) {
	// Build payload with some withdrawn routes
	withdrawn := make([]byte, 0, 40)
	for i := range 10 {
		withdrawn = append(withdrawn, 24, 10, byte(i), 0) //nolint:gosec // bench data
	}
	payload := buildPayload(withdrawn, buildOriginAttr(), nil)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		wu := NewWireUpdate(payload, bgpctx.ContextID(1))
		wd, err := wu.Withdrawn()
		if err != nil {
			b.Fatal(err)
		}
		_ = wd
	}
}

// BenchmarkWireUpdateIsEOR measures End-of-RIB detection cost.
func BenchmarkWireUpdateIsEOR(b *testing.B) {
	b.Run("NotEOR", func(b *testing.B) {
		payload := benchPayload(5)

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			wu := NewWireUpdate(payload, bgpctx.ContextID(1))
			_, isEOR := wu.IsEOR()
			_ = isEOR
		}
	})

	b.Run("IPv4EOR", func(b *testing.B) {
		// Empty UPDATE: wdLen=0, attrLen=0, no NLRI
		payload := []byte{0x00, 0x00, 0x00, 0x00}

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			wu := NewWireUpdate(payload, bgpctx.ContextID(1))
			_, isEOR := wu.IsEOR()
			_ = isEOR
		}
	})
}

// BenchmarkWireUpdateAttrIterator measures the AttrIterator() creation cost.
func BenchmarkWireUpdateAttrIterator(b *testing.B) {
	payload := benchPayload(10)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		wu := NewWireUpdate(payload, bgpctx.ContextID(1))
		iter, err := wu.AttrIterator()
		if err != nil {
			b.Fatal(err)
		}
		_ = iter
	}
}
