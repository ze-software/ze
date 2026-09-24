// Design: docs/architecture/wire/isis.md -- RFC 1195 narrow IPv4 reachability.
// Goal: retain prefix and metric meaning while clearing only reserved metric bits.
// Method: compare production TLV encoders and decoders against explicit wire entries.

package packet

import (
	"bytes"
	"errors"
	"net/netip"
	"testing"
)

// RFC requirement: RFC1195-5.3.4-3 positive -- encoding TLV128 clears bit7 of all
// three optional metrics while retaining their support bits and six-bit values.
func TestRFC1195NarrowMetricReservedTransmit(t *testing.T) {
	tlv := NarrowIPReachTLV{Entries: []NarrowIPReachEntry{{
		DefaultMetricValue: 19,
		DelayMetric:        0xff,
		ExpenseMetric:      0x6a,
		ErrorMetric:        0xc7,
		Prefix:             netip.MustParsePrefix("192.0.2.129/25"),
	}}}
	var buf [32]byte
	end := tlv.WriteTo(buf[:], 3)
	want := []byte{128, 12, 19, 0xbf, 0x2a, 0x87, 192, 0, 2, 128, 255, 255, 255, 128}
	if !bytes.Equal(buf[3:end], want) {
		t.Fatalf("encoded TLV = %x, want %x", buf[3:end], want)
	}
}

// RFC requirement: RFC1195-5.3.4-3 negative -- decoding TLV128 with optional
// reserved bits set produces the same metrics and prefix as the zero-bit form.
func TestRFC1195NarrowMetricReservedReceive(t *testing.T) {
	value := []byte{19, 0x3f, 0x80, 0x07, 192, 0, 2, 129, 255, 255, 255, 128}
	clean, err := DecodeNarrowIPReachTLV(value, false)
	if err != nil {
		t.Fatal(err)
	}
	value[1] |= 0x40
	value[2] |= 0x40
	value[3] |= 0x40
	dirty, err := DecodeNarrowIPReachTLV(value, false)
	if err != nil {
		t.Fatal(err)
	}
	want := NarrowIPReachEntry{
		DefaultMetricValue: 19,
		DelayMetric:        63,
		ExpenseMetric:      0x80,
		ErrorMetric:        7,
		Prefix:             netip.MustParsePrefix("192.0.2.128/25"),
	}
	if len(clean.Entries) != 1 {
		t.Fatalf("clean entries = %d, want 1", len(clean.Entries))
	}
	if clean.Entries[0] != want {
		t.Fatalf("clean entry = %+v, want %+v", clean.Entries[0], want)
	}
	if len(dirty.Entries) != 1 {
		t.Fatalf("reserved-bit entries = %d, want 1", len(dirty.Entries))
	}
	if dirty.Entries[0] != want {
		t.Fatalf("reserved bits changed entry: %+v, want %+v", dirty.Entries[0], want)
	}
}

// External route origin, metric type, and the RFC 2966 down bit are independent.
// Exercise the four default-metric flag combinations without changing the metric.
func TestNarrowIPReachExternalAndDown(t *testing.T) {
	for _, tc := range []struct {
		name           string
		externalMetric bool
		down           bool
		wireMetric     byte
	}{
		{name: "internal-up", wireMetric: 23},
		{name: "external-up", externalMetric: true, wireMetric: 0x40 | 23},
		{name: "internal-down", down: true, wireMetric: 0x80 | 23},
		{name: "external-down", externalMetric: true, down: true, wireMetric: 0xc0 | 23},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tlv := NarrowIPReachTLV{External: true, Entries: []NarrowIPReachEntry{{
				DefaultMetricValue: 23,
				ExternalMetric:     tc.externalMetric,
				UpDown:             tc.down,
				DelayMetric:        0xff,
				ExpenseMetric:      0xc1,
				ErrorMetric:        0x40,
				Prefix:             netip.MustParsePrefix("0.0.0.0/0"),
			}}}
			var buf [14]byte
			end := tlv.WriteTo(buf[:], 0)
			want := []byte{130, 12, tc.wireMetric, 0xbf, 0x81, 0, 0, 0, 0, 0, 0, 0, 0, 0}
			if !bytes.Equal(buf[:end], want) {
				t.Fatalf("encoded TLV = %x, want %x", buf[:end], want)
			}
			out, err := DecodeNarrowIPReachTLV(buf[2:end], true)
			if err != nil {
				t.Fatal(err)
			}
			if len(out.Entries) != 1 {
				t.Fatalf("entries = %d, want 1", len(out.Entries))
			}
			entry := out.Entries[0]
			if entry.DefaultMetricValue != 23 {
				t.Fatalf("metric = %d, want 23", entry.DefaultMetricValue)
			}
			if entry.ExternalMetric != tc.externalMetric {
				t.Fatalf("external metric = %v, want %v", entry.ExternalMetric, tc.externalMetric)
			}
			if entry.UpDown != tc.down {
				t.Fatalf("down = %v, want %v", entry.UpDown, tc.down)
			}
			if entry.Prefix != netip.MustParsePrefix("0.0.0.0/0") {
				t.Fatalf("prefix = %v, want default route", entry.Prefix)
			}
		})
	}
}

// A truncated final entry must not return a partially usable route list.
func TestNarrowIPReachTruncatedEntry(t *testing.T) {
	value := []byte{1, 0x80, 0x80, 0x80, 192, 0, 2, 1, 255, 255, 255, 255, 1}
	out, err := DecodeNarrowIPReachTLV(value, false)
	if !errors.Is(err, ErrLength) {
		t.Fatalf("partial entry error = %v, want ErrLength", err)
	}
	if len(out.Entries) != 0 {
		t.Fatalf("partial TLV returned %d entries", len(out.Entries))
	}
}

// TLV130's I/E bit is in the default metric only. Optional metric bit7 remains
// reserved, so it cannot change a prefix's external cost or support flags.
func TestNarrowIPReachExternalOptionalReservedReceive(t *testing.T) {
	value := []byte{0x40 | 9, 0xff, 0x6a, 0xc7, 198, 51, 100, 1, 255, 255, 255, 255}
	out, err := DecodeNarrowIPReachTLV(value, true)
	if err != nil {
		t.Fatal(err)
	}
	want := NarrowIPReachEntry{
		DefaultMetricValue: 9,
		ExternalMetric:     true,
		DelayMetric:        0xbf,
		ExpenseMetric:      0x2a,
		ErrorMetric:        0x87,
		Prefix:             netip.MustParsePrefix("198.51.100.1/32"),
	}
	if len(out.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(out.Entries))
	}
	if out.Entries[0] != want {
		t.Fatalf("entry = %+v, want %+v", out.Entries[0], want)
	}
}

// Decode must retain the invalid TLV128 external-metric bit so the SPF consumer
// can ignore the prefix under RFC2966 section3.3 instead of installing it as internal.
func TestNarrowIPReachInternalExternalMetricRetained(t *testing.T) {
	value := []byte{0x40 | 9, 0x80, 0x80, 0x80, 198, 51, 100, 1, 255, 255, 255, 255}
	out, err := DecodeNarrowIPReachTLV(value, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(out.Entries))
	}
	if !out.Entries[0].ExternalMetric {
		t.Fatal("invalid TLV128 external metric was converted to internal")
	}
	var buf [14]byte
	end := out.WriteTo(buf[:], 0)
	want := []byte{128, 12, 9, 0x80, 0x80, 0x80, 198, 51, 100, 1, 255, 255, 255, 255}
	if !bytes.Equal(buf[:end], want) {
		t.Fatalf("originated TLV128 = %x, want %x", buf[:end], want)
	}
}
