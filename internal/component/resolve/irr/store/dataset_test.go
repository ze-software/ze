// Design: docs/architecture/irr-owner.md -- the stored IRR dataset value
package store

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/zefs"
)

// encodeDataset is the owner's encode step: one buffer sized from EncodedLen,
// filled by WriteTo, whose count must equal EncodedLen.
func encodeDataset(t *testing.T, d Dataset) []byte {
	t.Helper()
	buf := make([]byte, d.EncodedLen())
	if written := d.WriteTo(buf, 0); written != len(buf) {
		t.Fatalf("WriteTo wrote %d bytes, EncodedLen is %d", written, len(buf))
	}
	return buf
}

func mustDataset(t *testing.T, in DatasetInput) Dataset {
	t.Helper()
	d, err := NewDataset(in)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func sampleInput() DatasetInput {
	return DatasetInput{
		ASSet:       "RIPE::AS-TEST",
		Server:      "whois.radb.net",
		IPv4:        []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0"), netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("198.51.100.128/25"), netip.MustParsePrefix("203.0.113.7/32")},
		IPv6:        []netip.Prefix{netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2001:db8:1:2::/63"), netip.MustParsePrefix("2001:db8::1/128")},
		RefreshedAt: time.Date(2026, 11, 15, 1, 2, 3, 0, time.UTC),
		CheckedAt:   time.Date(2026, 11, 15, 4, 5, 6, 0, time.UTC),
	}
}

// TestDatasetRoundTrip proves decode reproduces what was encoded: both prefix
// lists in order, every timestamp, the AS-SET and the normalised server, with
// an unset stale-since read back as unset and a set one read back exact.
//
// Method: encode a dataset holding /0, /24, /25 and /32 IPv4 and /32, /63 and
// /128 IPv6 prefixes, decode it under its own key, compare every field; then
// the same with stale-since set and a write at a non-zero offset.
func TestDatasetRoundTrip(t *testing.T) {
	in := sampleInput()
	original := mustDataset(t, in)
	decoded, err := DecodeDataset(original.Key(), encodeDataset(t, original))
	if err != nil {
		t.Fatal(err)
	}
	requireSameDataset(t, decoded, in)
	if !decoded.StaleSince().IsZero() {
		t.Fatalf("unset stale-since read back as %s", decoded.StaleSince())
	}

	in.StaleSince = time.Date(2026, 11, 15, 7, 8, 9, 0, time.UTC)
	stale := mustDataset(t, in)
	const offset = 7
	buf := make([]byte, offset+stale.EncodedLen())
	if written := stale.WriteTo(buf, offset); written != stale.EncodedLen() {
		t.Fatalf("WriteTo at offset wrote %d, want %d", written, stale.EncodedLen())
	}
	decoded, err = DecodeDataset(stale.Key(), buf[offset:])
	if err != nil {
		t.Fatal(err)
	}
	requireSameDataset(t, decoded, in)
	if !decoded.StaleSince().Equal(in.StaleSince) {
		t.Fatalf("stale-since = %s, want %s", decoded.StaleSince(), in.StaleSince)
	}
}

func requireSameDataset(t *testing.T, got Dataset, want DatasetInput) {
	t.Helper()
	if got.ASSet() != want.ASSet || got.Server() != "whois.radb.net" {
		t.Fatalf("identity = %s from %s, want %s from whois.radb.net", got.ASSet(), got.Server(), want.ASSet)
	}
	if !slices.Equal(got.IPv4(), want.IPv4) {
		t.Fatalf("IPv4 = %v, want %v", got.IPv4(), want.IPv4)
	}
	if !slices.Equal(got.IPv6(), want.IPv6) {
		t.Fatalf("IPv6 = %v, want %v", got.IPv6(), want.IPv6)
	}
	if !got.RefreshedAt().Equal(want.RefreshedAt) || !got.CheckedAt().Equal(want.CheckedAt) {
		t.Fatalf("times = %s, %s, want %s, %s", got.RefreshedAt(), got.CheckedAt(), want.RefreshedAt, want.CheckedAt)
	}
}

// TestDatasetLargeFitsOneKey proves the largest dataset any consumer accepts,
// DatasetFamilyMax prefixes of each family at their widest (/32 and /128),
// fits the declared bound of the dataset key and reads back identical.
//
// Method: build 500,000 distinct /32 and 500,000 distinct /128 prefixes,
// encode, check the size against zefs.IRRDatasetValueMax, decode and compare.
func TestDatasetLargeFitsOneKey(t *testing.T) {
	in := DatasetInput{ASSet: "AS-LARGE", Server: "whois.radb.net", CheckedAt: time.Unix(1_800_000_000, 0)}
	in.IPv4 = make([]netip.Prefix, DatasetFamilyMax)
	in.IPv6 = make([]netip.Prefix, DatasetFamilyMax)
	for i := range DatasetFamilyMax {
		var v4 [4]byte
		binary.BigEndian.PutUint32(v4[:], uint32(0x0a000000+i))
		in.IPv4[i] = netip.PrefixFrom(netip.AddrFrom4(v4), 32)
		var v6 [16]byte
		v6[0], v6[1] = 0x20, 0x01
		binary.BigEndian.PutUint32(v6[12:], uint32(i))
		in.IPv6[i] = netip.PrefixFrom(netip.AddrFrom16(v6), 128)
	}
	d := mustDataset(t, in)
	value := encodeDataset(t, d)
	if len(value) > zefs.IRRDatasetValueMax {
		t.Fatalf("largest dataset is %d bytes, over the declared %d", len(value), zefs.IRRDatasetValueMax)
	}
	decoded, err := DecodeDataset(d.Key(), value)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(decoded.IPv4(), in.IPv4) || !slices.Equal(decoded.IPv6(), in.IPv6) {
		t.Fatal("largest dataset did not read back identical")
	}
}

// TestDatasetOverCapsRefused proves a list over DatasetFamilyMax is refused
// whole with an error naming both counts (AC-12), never stored truncated.
//
// Method: one IPv4 prefix more than DatasetFamilyMax; NewDataset answers
// ErrDatasetTooLarge and the message holds 500001 and the IPv6 count.
func TestDatasetOverCapsRefused(t *testing.T) {
	in := DatasetInput{ASSet: "AS-HUGE", Server: "whois.radb.net"}
	in.IPv4 = make([]netip.Prefix, DatasetFamilyMax+1)
	for i := range in.IPv4 {
		in.IPv4[i] = netip.MustParsePrefix("192.0.2.0/24")
	}
	in.IPv6 = []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}
	_, err := NewDataset(in)
	if !errors.Is(err, ErrDatasetTooLarge) {
		t.Fatalf("error = %v, want ErrDatasetTooLarge", err)
	}
	if !strings.Contains(err.Error(), "500001 IPv4 and 1 IPv6") {
		t.Fatalf("error %q does not name the counts", err)
	}
}

// TestDatasetDigestRefused proves a value whose content no longer matches its
// digest is refused with ErrDatasetDigest, for a flipped byte in every region:
// the times, a name length's partner byte, a prefix and the digest itself.
//
// Method: flip one bit at several offsets of an encoded dataset and decode
// each copy; only an intact copy decodes.
func TestDatasetDigestRefused(t *testing.T) {
	d := mustDataset(t, sampleInput())
	value := encodeDataset(t, d)
	for _, offset := range []int{3, 30, len(value) - 40, len(value) - 1} {
		corrupt := slices.Clone(value)
		corrupt[offset] ^= 0x01
		if _, err := DecodeDataset(d.Key(), corrupt); !errors.Is(err, ErrDatasetDigest) {
			t.Fatalf("byte %d flipped: error = %v, want ErrDatasetDigest", offset, err)
		}
	}
	if _, err := DecodeDataset(d.Key(), value); err != nil {
		t.Fatalf("intact value refused: %v", err)
	}
}

// TestDatasetTruncatedRefused proves every truncation of a value is refused
// with a named error and never read as a shorter list: below the fixed size it
// is ErrDatasetTruncated, and above it the lost tail fails the structure or
// the digest.
//
// Method: decode every prefix of an encoded dataset shorter than the whole.
func TestDatasetTruncatedRefused(t *testing.T) {
	d := mustDataset(t, sampleInput())
	value := encodeDataset(t, d)
	for size := range value {
		_, err := DecodeDataset(d.Key(), value[:size])
		if err == nil {
			t.Fatalf("value cut to %d of %d bytes decoded", size, len(value))
		}
		if size < datasetFixedLen && !errors.Is(err, ErrDatasetTruncated) {
			t.Fatalf("value cut to %d bytes: error = %v, want ErrDatasetTruncated", size, err)
		}
	}
}

// TestDatasetIdentityRefused proves a value read under another dataset's key
// is refused: the key and the value must name the same AS-SET and server.
//
// Method: decode one encoded dataset under the key of another AS-SET and of
// another server.
func TestDatasetIdentityRefused(t *testing.T) {
	d := mustDataset(t, sampleInput())
	value := encodeDataset(t, d)
	for _, key := range []string{
		zefs.KeyIRRDataset.Key("whois.radb.net", "AS-OTHER"),
		zefs.KeyIRRDataset.Key("rr.ntt.net", "RIPE::AS-TEST"),
	} {
		if _, err := DecodeDataset(key, value); !errors.Is(err, ErrDatasetIdentity) {
			t.Fatalf("read under %s: error = %v, want ErrDatasetIdentity", key, err)
		}
	}
}

// TestDatasetValueSizeBoundary proves the reader's own size bound: a value of
// exactly zefs.IRRDatasetValueMax bytes is judged on its content, one byte more
// is refused as too large before any parsing.
//
// Method: two zero-filled values, at the bound and one over; the first fails
// on its version byte, the second on its size.
func TestDatasetValueSizeBoundary(t *testing.T) {
	key := zefs.KeyIRRDataset.Key("whois.radb.net", "AS-TEST")
	if _, err := DecodeDataset(key, make([]byte, zefs.IRRDatasetValueMax)); !errors.Is(err, ErrDatasetVersion) {
		t.Fatalf("value at the bound: error = %v, want ErrDatasetVersion (judged on content)", err)
	}
	if _, err := DecodeDataset(key, make([]byte, zefs.IRRDatasetValueMax+1)); !errors.Is(err, ErrDatasetTooLarge) {
		t.Fatalf("value over the bound: error = %v, want ErrDatasetTooLarge", err)
	}
}

// TestServerIdentityNormalised proves every spelling of one IRR server names
// one dataset key, and a different port names another.
//
// Method: whois.radb.net, upper case, a trailing dot and the explicit whois
// port 43 all give the key of whois.radb.net; port 4343 keeps its port; an
// IPv6 literal at port 43 and without a port agree, and one at another port
// carries it after an underscore; a host a key cannot carry is refused.
func TestServerIdentityNormalised(t *testing.T) {
	want, err := DatasetKey("whois.radb.net", "AS-TEST")
	if err != nil {
		t.Fatal(err)
	}
	for _, server := range []string{"WHOIS.RADB.NET", "whois.radb.net:43", "Whois.Radb.Net.:043", " whois.radb.net "} {
		got, err := DatasetKey(server, "AS-TEST")
		if err != nil || got != want {
			t.Fatalf("DatasetKey(%q) = %q, %v, want %q", server, got, err, want)
		}
	}
	other, err := DatasetKey("whois.radb.net:4343", "AS-TEST")
	if err != nil || other != zefs.KeyIRRDataset.Key("whois.radb.net:4343", "AS-TEST") {
		t.Fatalf("port 4343: %q, %v", other, err)
	}
	for server, wantForm := range map[string]string{
		"2001:DB8::1":        "2001:db8::1",
		"[2001:db8::1]:43":   "2001:db8::1",
		"[2001:db8::1]:4343": "2001:db8::1_4343",
		"127.0.0.1:4343":     "127.0.0.1:4343",
	} {
		got, err := DatasetServer(server)
		if err != nil || got != wantForm {
			t.Fatalf("DatasetServer(%q) = %q, %v, want %q", server, got, err, wantForm)
		}
	}
	for _, server := range []string{"", "bad/host", "host:0", "host:99999", "a..b"} {
		if got, err := DatasetServer(server); !errors.Is(err, ErrDatasetServer) {
			t.Fatalf("DatasetServer(%q) = %q, %v, want ErrDatasetServer", server, got, err)
		}
	}
}
