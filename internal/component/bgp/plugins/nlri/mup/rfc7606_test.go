package mup

import (
	"context"
	"log/slog"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
	"github.com/ze-software/ze/internal/core/locatororigin"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// mupWireNLRI frames one BGP-MUP NLRI: [arch:1][route-type:2][length:1][body]
// (draft-ietf-bess-mup-safi Section 3.1).
func mupWireNLRI(arch byte, routeType uint16, body ...byte) []byte {
	out := []byte{arch, byte(routeType >> 8), byte(routeType), byte(len(body))}
	return append(out, body...)
}

// VALIDATES: Implemented names exactly the route types String can name, under the
// one architecture type the draft defines. draft-ietf-bess-mup-safi Section 3.1
// enumerates route types 1..4, and String renders every other value as "type(N)".
// PREVENTS: Section 5.4's recognized set drifting from the rest of the package.
func TestImplementedMatchesRouteTypeNames(t *testing.T) {
	for rt := range 256 {
		routeType := MUPRouteType(rt)
		named := !strings.HasPrefix(routeType.String(), "type(")
		if named != routeType.Implemented(MUPArch3GPP5G) {
			t.Errorf("route type %d: String names it=%v but Implemented()=%v",
				rt, named, routeType.Implemented(MUPArch3GPP5G))
		}
	}
}

// VALIDATES: RecognizeNLRI accepts every route type the draft defines under
// architecture type 1.
// PREVENTS: an implemented type being discarded, which would drop working MUP routes.
func TestRecognizeNLRIAcceptsImplementedTypes(t *testing.T) {
	for _, rt := range []MUPRouteType{MUPISD, MUPDSD, MUPT1ST, MUPT2ST} {
		if RecognizeNLRI(mupWireNLRI(byte(MUPArch3GPP5G), uint16(rt), 0xaa, 0xbb), false) != nlritype.Keep {
			t.Errorf("route type %d is implemented and must be recognized", rt)
		}
	}
}

// VALIDATES: a route type the draft does not define is not recognized, and so is a
// route type carried under an architecture ze does not implement. Section 3.1 says
// the Route Type specific encoding depends on Architecture Type AND Route Type, so
// the pair is what names the NLRI type.
// PREVENTS: Section 5.4's discard never firing for BGP-MUP, and an architecture ze
// cannot decode being relayed because its route-type octets happen to read as 1.
//
// RFC requirement: RFC7606-5.4-1 positive -- a BGP-MUP architecture and route type pair ze does not implement is not recognized, so the Section 5.4 discard removes it.
func TestRecognizeNLRIRejectsUnimplementedTypes(t *testing.T) {
	for _, rt := range []uint16{0, 5, 6, 99, 0xffff} {
		if RecognizeNLRI(mupWireNLRI(byte(MUPArch3GPP5G), rt, 0xaa), false) != nlritype.Discard {
			t.Errorf("route type %d is not defined by the draft and must not be recognized", rt)
		}
	}
	for _, arch := range []byte{0, 2, 3, 255} {
		if RecognizeNLRI(mupWireNLRI(arch, uint16(MUPISD), 0xaa), false) != nlritype.Discard {
			t.Errorf("architecture type %d is not implemented and must not be recognized", arch)
		}
	}
}

// VALIDATES: the 4-byte RFC 7911 path identifier is skipped before the architecture
// and route type are read.
// PREVENTS: judging a path id byte as an architecture type.
func TestRecognizeNLRISkipsPathID(t *testing.T) {
	withPathID := append([]byte{0x00, 0x00, 0x00, 0x63},
		mupWireNLRI(byte(MUPArch3GPP5G), uint16(MUPDSD), 0xaa)...)
	if RecognizeNLRI(withPathID, true) != nlritype.Keep {
		t.Fatal("the pair after the path id is implemented and must be recognized")
	}
	if RecognizeNLRI(withPathID, false) != nlritype.Discard {
		t.Fatal("without add-path the leading byte is 0x00, not an implemented architecture")
	}
}

// VALIDATES: a slice too short to hold the architecture and route type is refused
// rather than read past.
// PREVENTS: an out-of-range read, and a truncated NLRI being relayed as recognized.
func TestRecognizeNLRIRefusesTruncated(t *testing.T) {
	if RecognizeNLRI(nil, false) != nlritype.Discard {
		t.Error("an empty NLRI carries no type")
	}
	if RecognizeNLRI([]byte{byte(MUPArch3GPP5G), 0x00}, false) != nlritype.Discard {
		t.Error("two bytes cannot hold a one-octet architecture and a two-octet route type")
	}
	if RecognizeNLRI([]byte{0x00, 0x00, 0x00, byte(MUPArch3GPP5G), 0x00}, true) != nlritype.Discard {
		t.Error("a slice shorter than the path id plus the type carries no type")
	}
}

// VALIDATES: the plugin's init actually installed the recognizer for both AFIs, so
// RFC 7606 Section 5.4 binds ipv4/mup and ipv6/mup in a real build.
// PREVENTS: the whole mechanism being present but unwired.
func TestRecognizerIsRegisteredForBothMUPFamilies(t *testing.T) {
	for _, fam := range []Family{IPv4MUP, IPv6MUP} {
		fn := nlritype.Get(fam)
		if fn == nil {
			t.Fatalf("the mup plugin must register its Section 5.4 recognizer for %s", fam)
		}
		// An implemented pair is never discarded. This one-octet body is malformed, so the
		// registered recognizer withdraws it (draft-ietf-bess-mup-safi Section 3.1.3.1).
		if got := fn(mupWireNLRI(byte(MUPArch3GPP5G), uint16(MUPT1ST), 0x00), false); got != nlritype.Withdraw {
			t.Errorf("%s: the registered recognizer must withdraw a malformed implemented pair, got %d", fam, got)
		}
		if fn(mupWireNLRI(byte(MUPArch3GPP5G), 99, 0x00), false) != nlritype.Discard {
			t.Errorf("%s: the registered recognizer must refuse an undefined route type", fam)
		}
	}
}

type mupLogRecord struct {
	level slog.Level
	msg   string
	attrs map[string]any
}

type mupLogCapture struct {
	mu      sync.Mutex
	records []mupLogRecord
}

func (c *mupLogCapture) Enabled(_ context.Context, _ slog.Level) bool { return true }

//nolint:gocritic // implements slog.Handler
func (c *mupLogCapture) Handle(_ context.Context, r slog.Record) error {
	attrs := make(map[string]any)
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})
	c.mu.Lock()
	c.records = append(c.records, mupLogRecord{level: r.Level, msg: r.Message, attrs: attrs})
	c.mu.Unlock()
	return nil
}

func (c *mupLogCapture) WithAttrs(_ []slog.Attr) slog.Handler { return c }
func (c *mupLogCapture) WithGroup(_ string) slog.Handler      { return c }

func (c *mupLogCapture) countByCode(code string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, rec := range c.records {
		if rec.level != slog.LevelWarn {
			continue
		}
		if rec.attrs["code"] == code {
			n++
		}
	}
	return n
}

func (c *mupLogCapture) lastByCode(code string) (mupLogRecord, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, rec := range slices.Backward(c.records) {
		if rec.attrs["code"] == code {
			return rec, true
		}
	}
	return mupLogRecord{}, false
}

func setupMUPOriginTestEnv(t *testing.T) (*mupLogCapture, *time.Time) {
	t.Helper()
	now := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
	capture := &mupLogCapture{}
	SetLogger(slog.New(capture))
	resetMUPWarnLimiterForTest(func() time.Time { return now })
	locatororigin.ResetForTest()
	report.ResetForTest()
	t.Cleanup(func() {
		SetLogger(slogutil.DiscardLogger())
		resetMUPWarnLimiterForTest(nil)
		locatororigin.ResetForTest()
		report.ResetForTest()
	})
	return capture, &now
}

func seedMUPOriginSnapshot(entries ...locatororigin.LocatorEntry) {
	locatororigin.ApplySnapshot(&locatororigin.Snapshot{
		Source:     "isis",
		Generation: 1,
		Active:     true,
		Entries:    entries,
	})
}

func mupTestUpdateAttrs(raw []byte, peer netip.Addr, nh []byte) nlritype.UpdateAttrs {
	attrs := nlritype.NewUpdateAttrs(raw)
	attrs.SetPeer(peer)
	attrs.SetMPReachNextHop(nh)
	return attrs
}

// TestMUPDSDNextHopLocatorMismatch verifies draft-ietf-bess-mup-safi Section 3.3.6
// originator matching for DSD routes across TLV 140, TLV 232, /128 TLV 236,
// covering IPv6 Source Router ID, and the complete MP_REACH_NLRI next-hop length
// matrix (16, 32, 4, and unsupported lengths).
//
// VALIDATES: matching 16-octet and 32-octet (global half) next hops are kept;
// mismatching 16-octet, 32-octet (link-local-only match), and 4-octet next hops
// against a known originator withdraw only the DSD route while keeping sibling
// routes; 4-octet next hops against an unresolved originator and unsupported
// next-hop lengths keep the route and raise doctor-bgp-mup-locator-unresolved.
// PREVENTS: accepting spoofed or misrouted DSD routes whose next hop does not
// belong to the Prefix-SID locator originator, or dropping routes on non-16-byte
// next hops when the originator is not proven.
func TestMUPDSDNextHopLocatorMismatch(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.10")
	// dsdAttrsPrefixSID carries SRv6 SID 2001:db8::1.
	locPrefix := netip.MustParsePrefix("2001:db8::/48")
	addrTLV140 := netip.MustParseAddr("2001:db8:140::1")
	addrTLV232 := netip.MustParseAddr("2001:db8:232::1")
	addrTLV236Host := netip.MustParseAddr("2001:db8:236::1")
	addrOther := netip.MustParseAddr("2001:db8:999::9")
	addrLinkLocal := netip.MustParseAddr("fe80::1")

	knownEntry := locatororigin.LocatorEntry{
		Prefix:     locPrefix,
		Known:      true,
		Originator: "0100.0000.0001",
		Addresses:  []netip.Addr{addrTLV140, addrTLV232, addrTLV236Host},
	}

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.6-1 positive -- a DSD route whose 16-octet MP_REACH_NLRI next hop or 32-octet global first half matches an address or covering IPv6 Source Router ID in the seeded locator-originator entry is kept
	t.Run("matching next hop across TLV 140 TLV 232 host TLV 236 32-octet global and Source Router ID", func(t *testing.T) {
		for _, fam := range []Family{IPv4MUP, IPv6MUP} {
			recognize, check := nlritype.Rulings(fam)
			require.NotNil(t, recognize)
			require.NotNil(t, check)
			dsd := dsdRoute(192, 0, 2, 1)
			sibling := t2stRoute(64, 192, 0, 2, 1, 0, 0, 0, 1)
			if fam == IPv6MUP {
				dsd = dsdRoute(ipv6Addr...)
				sibling = t2stRoute(160, append(append([]byte{}, ipv6Addr...), 0, 0, 0, 1)...)
			}
			section := concatNLRI(dsd, sibling)
			var scratch [nlrisplit.PrefixKeyScratchSize]byte

			nh32GlobalMatch := append(addrTLV140.AsSlice(), addrLinkLocal.AsSlice()...)
			positiveCases := []struct {
				name  string
				entry locatororigin.LocatorEntry
				nh    []byte
			}{
				{"16-octet matches TLV 140", knownEntry, addrTLV140.AsSlice()},
				{"16-octet matches TLV 232", knownEntry, addrTLV232.AsSlice()},
				{"16-octet matches host /128 TLV 236", knownEntry, addrTLV236Host.AsSlice()},
				{"32-octet global half matches originator", knownEntry, nh32GlobalMatch},
				{
					name: "16-octet matches re-advertised covering IPv6 Source Router ID",
					entry: locatororigin.LocatorEntry{
						Prefix:          locPrefix,
						Known:           false,
						Originator:      "0100.0000.0002",
						SourceRouterIDs: []netip.Addr{addrOther},
						ReasonClass:     locatororigin.ReasonReadvertised,
						Reason:          "covering locator 2001:db8::/48 re-advertised by 0100.0000.0002 (R or X set)",
					},
					nh: addrOther.AsSlice(),
				},
			}
			for _, tc := range positiveCases {
				t.Run(fam.String()+"/"+tc.name, func(t *testing.T) {
					capture, _ := setupMUPOriginTestEnv(t)
					seedMUPOriginSnapshot(tc.entry)
					attrs := mupTestUpdateAttrs(dsdAttrsPrefixSID, peer, tc.nh)
					r, err := nlritype.RetainUpdate(recognize, check, &attrs, fam, section, false, true, scratch[:])
					require.NoError(t, err)
					assert.Equal(t, section, r.Kept)
					assert.False(t, r.Removed())
					assert.Equal(t, 0, capture.countByCode(codeDoctorBGPMUPLocatorMismatch), "matching DSD next hop must not log mismatch")
					assert.Equal(t, 0, capture.countByCode(codeDoctorBGPMUPLocatorUnresolved), "matching DSD next hop (including Source Router ID) must not fall through to unresolved")
				})
			}
		}
	})

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.6-1 negative -- a DSD route whose MP_REACH_NLRI next hop does not belong to the proven Prefix-SID locator originator (mismatched 16-octet IPv6, 32-octet matching only link-local half, or 4-octet IPv4) is rejected
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.6-4 positive -- a DSD route failing the locator originator check is treated as withdrawn while its sibling routes in the same MP_REACH_NLRI remain kept
	t.Run("proven mismatch withdraws DSD while keeping sibling", func(t *testing.T) {
		nh32LinkLocalOnly := append(addrOther.AsSlice(), addrTLV140.AsSlice()...)
		nh4 := netip.MustParseAddr("192.0.2.1").AsSlice()
		mismatchCases := []struct {
			name string
			nh   []byte
		}{
			{"16-octet IPv6 mismatch", addrOther.AsSlice()},
			{"32-octet global mismatch even when second half matches", nh32LinkLocalOnly},
			{"4-octet IPv4 mismatch against known IPv6 originator", nh4},
		}
		for _, fam := range []Family{IPv4MUP, IPv6MUP} {
			recognize, check := nlritype.Rulings(fam)
			dsd := dsdRoute(192, 0, 2, 1)
			sibling := t2stRoute(64, 192, 0, 2, 1, 0, 0, 0, 1)
			if fam == IPv6MUP {
				dsd = dsdRoute(ipv6Addr...)
				sibling = t2stRoute(160, append(append([]byte{}, ipv6Addr...), 0, 0, 0, 1)...)
			}
			for _, addPath := range []bool{false, true} {
				frame := func(n []byte) []byte {
					if !addPath {
						return n
					}
					return concatNLRI([]byte{0, 0, 0, 9}, n)
				}
				framedDSD := frame(dsd)
				framedSib := frame(sibling)
				section := concatNLRI(framedDSD, framedSib)
				var scratch [nlrisplit.PrefixKeyScratchSize]byte

				for _, tc := range mismatchCases {
					name := fam.String() + "/" + tc.name
					if addPath {
						name += "/ADD-PATH"
					}
					t.Run(name, func(t *testing.T) {
						capture, _ := setupMUPOriginTestEnv(t)
						seedMUPOriginSnapshot(knownEntry)
						attrs := mupTestUpdateAttrs(dsdAttrsPrefixSID, peer, tc.nh)
						r, err := nlritype.RetainUpdate(recognize, check, &attrs, fam, section, addPath, true, scratch[:])
						require.NoError(t, err)
						assert.Equal(t, framedSib, r.Kept, "sibling must stay kept")
						assert.Equal(t, framedDSD, r.Withdrawn, "mismatched DSD must be treated as withdrawn")
						assert.Zero(t, r.Discarded)
						assert.Equal(t, 1, capture.countByCode(codeDoctorBGPMUPLocatorMismatch))
						rec, found := capture.lastByCode(codeDoctorBGPMUPLocatorMismatch)
						require.True(t, found)
						assert.Equal(t, "0100.0000.0001", rec.attrs["originator"])
						assert.Equal(t, peer.String(), rec.attrs["peer"])
					})
				}
			}
		}
	})

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.6-4 negative -- a DSD route passing the locator originator check or carrying an uncomparable next-hop length (0, 3, 5, 15, 17, 31, 33) or unresolved originator is kept rather than withdrawn
	t.Run("conforming DSD, 4-octet against unresolved originator, and unsupported next-hop lengths stay kept", func(t *testing.T) {
		recognize, check := nlritype.Rulings(IPv4MUP)
		dsd := dsdRoute(192, 0, 2, 1)
		var scratch [nlrisplit.PrefixKeyScratchSize]byte

		// Conforming DSD passing the locator originator check is kept without warning.
		capture, _ := setupMUPOriginTestEnv(t)
		seedMUPOriginSnapshot(knownEntry)
		attrsOK := mupTestUpdateAttrs(dsdAttrsPrefixSID, peer, addrTLV140.AsSlice())
		r, err := nlritype.RetainUpdate(recognize, check, &attrsOK, IPv4MUP, dsd, false, true, scratch[:])
		require.NoError(t, err)
		assert.Equal(t, dsd, r.Kept)
		assert.False(t, r.Removed())
		assert.Equal(t, 0, capture.countByCode(codeDoctorBGPMUPLocatorMismatch))
		assert.Equal(t, 0, capture.countByCode(codeDoctorBGPMUPLocatorUnresolved))

		// 4-octet next hop with no active source stays kept and warns.
		capture, _ = setupMUPOriginTestEnv(t)
		attrs4 := mupTestUpdateAttrs(dsdAttrsPrefixSID, peer, netip.MustParseAddr("192.0.2.1").AsSlice())
		r, err = nlritype.RetainUpdate(recognize, check, &attrs4, IPv4MUP, dsd, false, true, scratch[:])
		require.NoError(t, err)
		assert.Equal(t, dsd, r.Kept)
		assert.False(t, r.Removed())
		assert.Equal(t, 1, capture.countByCode(codeDoctorBGPMUPLocatorUnresolved))

		// Unsupported lengths 0, 3, 5, 15, 17, 31, 33 stay kept and warn even when originator is known.
		for _, nhLen := range []int{0, 3, 5, 15, 17, 31, 33} {
			capture, _ := setupMUPOriginTestEnv(t)
			seedMUPOriginSnapshot(knownEntry)
			nh := make([]byte, nhLen)
			attrs := mupTestUpdateAttrs(dsdAttrsPrefixSID, peer, nh)
			r, err := nlritype.RetainUpdate(recognize, check, &attrs, IPv4MUP, dsd, false, true, scratch[:])
			require.NoError(t, err)
			assert.Equal(t, dsd, r.Kept, "next-hop length %d must be kept", nhLen)
			assert.False(t, r.Removed(), "next-hop length %d must not withdraw", nhLen)
			assert.Equal(t, 1, capture.countByCode(codeDoctorBGPMUPLocatorUnresolved))
		}
	})
}

// TestMUPDSDUnresolvedOriginatorKept verifies that every unresolved locator
// outcome keeps the route, emits a rate-limited Warn carrying
// doctor-bgp-mup-locator-unresolved, and raises a bounded report-bus warning.
//
// VALIDATES: unresolved DSD routes are never withdrawn, repeat Warn logs within
// the throttle interval are suppressed and counted on the next window, and
// report.Warnings() exposes doctor-bgp-mup-locator-unresolved with a bounded
// subject and executable fix.
// PREVENTS: silent loss of routes when IS-IS is absent or ambiguous, and log
// flooding from hostile peers sending unresolvable DSD routes.
func TestMUPDSDUnresolvedOriginatorKept(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.20")
	locPrefix := netip.MustParsePrefix("2001:db8::/48")
	nh := netip.MustParseAddr("2001:db8:999::1").AsSlice()
	dsd := dsdRoute(192, 0, 2, 1)
	recognize, check := nlritype.Rulings(IPv4MUP)
	var scratch [nlrisplit.PrefixKeyScratchSize]byte

	cases := []struct {
		name        string
		seed        func()
		wantClass   locatororigin.ReasonClass
		wantSubject string
	}{
		{
			name:        "empty index",
			seed:        func() {},
			wantClass:   locatororigin.ReasonNoActiveSource,
			wantSubject: peer.String() + " " + locatororigin.ReasonNoActiveSource.String(),
		},
		{
			name: "no covering locator",
			seed: func() {
				seedMUPOriginSnapshot(locatororigin.LocatorEntry{
					Prefix:     netip.MustParsePrefix("fc00:9::/48"),
					Known:      true,
					Originator: "0100.0000.0001",
					Addresses:  []netip.Addr{netip.MustParseAddr("2001:db8:1::1")},
				})
			},
			wantClass:   locatororigin.ReasonNoCoveringLocator,
			wantSubject: peer.String() + " " + locatororigin.ReasonNoCoveringLocator.String(),
		},
		{
			name: "anycast locator",
			seed: func() {
				seedMUPOriginSnapshot(locatororigin.LocatorEntry{
					Prefix:      locPrefix,
					Known:       false,
					ReasonClass: locatororigin.ReasonAnycast,
					Reason:      "covering locator 2001:db8::/48 carries the anycast A-flag (RFC 9352 Section 6)",
				})
			},
			wantClass:   locatororigin.ReasonAnycast,
			wantSubject: peer.String() + " " + locatororigin.ReasonAnycast.String(),
		},
		{
			name: "re-advertised locator without matching Source Router ID",
			seed: func() {
				seedMUPOriginSnapshot(locatororigin.LocatorEntry{
					Prefix:      locPrefix,
					Known:       false,
					Originator:  "0100.0000.0002",
					ReasonClass: locatororigin.ReasonReadvertised,
					Reason:      "covering locator 2001:db8::/48 re-advertised by 0100.0000.0002 (R or X set)",
				})
			},
			wantClass:   locatororigin.ReasonReadvertised,
			wantSubject: peer.String() + " " + locatororigin.ReasonReadvertised.String(),
		},
		{
			name: "missing Prefix Attribute Flags",
			seed: func() {
				seedMUPOriginSnapshot(locatororigin.LocatorEntry{
					Prefix:      locPrefix,
					Known:       false,
					Originator:  "0100.0000.0003",
					ReasonClass: locatororigin.ReasonMissingFlags,
					Reason:      "covering locator 2001:db8::/48 from 0100.0000.0003 carries no Prefix Attribute Flags sub-TLV (RFC 9352 Section 7.1)",
				})
			},
			wantClass:   locatororigin.ReasonMissingFlags,
			wantSubject: peer.String() + " " + locatororigin.ReasonMissingFlags.String(),
		},
		{
			name: "undecoded TLV 237 on originator LSP",
			seed: func() {
				seedMUPOriginSnapshot(locatororigin.LocatorEntry{
					Prefix:             locPrefix,
					Known:              true,
					Originator:         "0100.0000.0004",
					Addresses:          []netip.Addr{netip.MustParseAddr("2001:db8:4::1")},
					HasUndecodedTLV237: true,
				})
			},
			wantClass:   locatororigin.ReasonUndecodedTLV237,
			wantSubject: peer.String() + " " + locatororigin.ReasonUndecodedTLV237.String(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			capture, now := setupMUPOriginTestEnv(t)
			tc.seed()
			attrs := mupTestUpdateAttrs(dsdAttrsPrefixSID, peer, nh)

			// First delivery logs one Warn and raises one report warning.
			r, err := nlritype.RetainUpdate(recognize, check, &attrs, IPv4MUP, dsd, false, true, scratch[:])
			require.NoError(t, err)
			assert.Equal(t, dsd, r.Kept)
			assert.False(t, r.Removed())
			assert.Equal(t, 1, capture.countByCode(codeDoctorBGPMUPLocatorUnresolved))

			// Immediate repeat within mupWarnInterval is suppressed in logs.
			r, err = nlritype.RetainUpdate(recognize, check, &attrs, IPv4MUP, dsd, false, true, scratch[:])
			require.NoError(t, err)
			assert.False(t, r.Removed())
			assert.Equal(t, 1, capture.countByCode(codeDoctorBGPMUPLocatorUnresolved))

			// Advancing clock past mupWarnInterval emits a second Warn carrying suppressed count.
			*now = now.Add(mupWarnInterval + time.Millisecond)
			r, err = nlritype.RetainUpdate(recognize, check, &attrs, IPv4MUP, dsd, false, true, scratch[:])
			require.NoError(t, err)
			assert.False(t, r.Removed())
			assert.Equal(t, 2, capture.countByCode(codeDoctorBGPMUPLocatorUnresolved))
			last, ok := capture.lastByCode(codeDoctorBGPMUPLocatorUnresolved)
			require.True(t, ok)
			assert.Equal(t, uint64(1), last.attrs["suppressed-since-last"])
			assert.Equal(t, fixBGPMUPLocatorUnresolved, last.attrs["fix"])

			warnings := report.Warnings()
			foundWarn := false
			for _, w := range warnings {
				if w.Code != codeDoctorBGPMUPLocatorUnresolved {
					continue
				}
				if w.Subject != tc.wantSubject {
					continue
				}
				foundWarn = true
				assert.Equal(t, reportSourceMUP, w.Source)
				assert.Equal(t, fixBGPMUPLocatorUnresolved, w.Fix)
			}
			assert.True(t, foundWarn, "report.Warnings() must carry %s for subject %q", codeDoctorBGPMUPLocatorUnresolved, tc.wantSubject)
		})
	}
}

// TestMUPISDWithoutPrefixSIDTreatAsWithdraw verifies draft-ietf-bess-mup-safi
// Section 3.3.3: an ISD route without a Prefix-SID attribute (or with Prefix-SID
// discarded from an EBGP peer) is treated as withdrawn while sibling T1ST/T2ST
// routes stay kept.
//
// VALIDATES: ISD without Prefix-SID and ISD with discarded Prefix-SID withdraw
// only the ISD route in both IPv4 and IPv6 MUP with and without ADD-PATH; ISD
// with a surviving Prefix-SID stays kept; MP_UNREACH_NLRI withdrawals need no
// Prefix-SID.
// PREVENTS: accepting an ISD route without a Prefix-SID attribute.
func TestMUPISDWithoutPrefixSIDTreatAsWithdraw(t *testing.T) {
	cases := []struct {
		name     string
		fam      Family
		isd      []byte
		siblings [][]byte
	}{
		{"IPv4", IPv4MUP, isdRoute(24, 10, 0, 0), [][]byte{
			t1stRoute(nil, teid1, endpoint4NoSource...), t2stRoute(64, 192, 0, 2, 1, 0, 0, 0, 1),
		}},
		{"IPv6", IPv6MUP, isdRoute(32, 0x20, 0x01, 0x0d, 0xb8), [][]byte{
			t2stRoute(160, append(append([]byte{}, ipv6Addr...), 0, 0, 0, 1)...),
		}},
	}
	for _, tc := range cases {
		for _, addPath := range []bool{false, true} {
			name := tc.name
			if addPath {
				name += " ADD-PATH"
			}
			t.Run(name, func(t *testing.T) {
				setupMUPOriginTestEnv(t)
				recognize, check := nlritype.Rulings(tc.fam)
				require.NotNil(t, recognize)
				require.NotNil(t, check)

				frame := func(nlri []byte) []byte {
					if !addPath {
						return nlri
					}
					return concatNLRI([]byte{0, 0, 0, 7}, nlri)
				}
				siblings := make([]byte, 0)
				for _, s := range tc.siblings {
					siblings = concatNLRI(siblings, frame(s))
				}
				isd := frame(tc.isd)
				section := concatNLRI(siblings[:0:0], isd, siblings)
				var scratch [nlrisplit.PrefixKeyScratchSize]byte

				// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3 positive -- an ISD route in an MP_REACH_NLRI without a Prefix-SID attribute (or with Prefix-SID discarded from an EBGP peer) is treated as withdrawn while its T1ST and T2ST siblings remain kept
				without := nlritype.NewUpdateAttrs(dsdAttrsNoPrefixSID)
				r, err := nlritype.RetainUpdate(recognize, check, &without, tc.fam, section, addPath, true, scratch[:])
				require.NoError(t, err)
				assert.Equal(t, siblings, r.Kept, "only the ISD route leaves the announcement")
				assert.Equal(t, isd, r.Withdrawn, "the ISD route is withdrawn, not discarded")
				assert.Zero(t, r.Discarded)

				discarded := nlritype.NewUpdateAttrs(dsdAttrsPrefixSID)
				discarded.Discarding(attribute.AttrPrefixSID)
				r, err = nlritype.RetainUpdate(recognize, check, &discarded, tc.fam, section, addPath, true, scratch[:])
				require.NoError(t, err)
				assert.Equal(t, siblings, r.Kept)
				assert.Equal(t, isd, r.Withdrawn, "a Prefix-SID being discarded leaves the ISD route without one")

				// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3 negative -- an ISD route whose UPDATE carries a surviving Prefix-SID attribute is kept, and an MP_UNREACH_NLRI withdrawal of an ISD route needs no Prefix-SID
				with := nlritype.NewUpdateAttrs(dsdAttrsPrefixSID)
				r, err = nlritype.RetainUpdate(recognize, check, &with, tc.fam, section, addPath, true, scratch[:])
				require.NoError(t, err)
				assert.Equal(t, section, r.Kept, "an ISD route with a Prefix-SID is kept")
				assert.False(t, r.Removed())

				r, err = nlritype.RetainUpdate(recognize, check, &without, tc.fam, section, addPath, false, nil)
				require.NoError(t, err)
				assert.False(t, r.Removed(), "a withdrawal needs no Prefix-SID")
			})
		}
	}
}

// TestMUPISDOriginatorMismatch verifies draft-ietf-bess-mup-safi Section 3.3.3
// originator checking of the ISD Address/Prefix field against the proven IGP
// locator originator while ignoring the MP_REACH_NLRI next hop.
//
// VALIDATES: matching IPv6 (/128 host and shorter prefix) and IPv4 (/32 host
// and shorter prefix) ISD routes are kept even when MP_REACH_NLRI next hop
// differs from the originator; mismatching IPv6 or IPv4 ISD Address withdraws
// only the ISD route while keeping siblings; an unresolved originator keeps the
// ISD route and raises a diagnostic warning.
// PREVENTS: accepting ISD routes whose Address field does not belong to the
// Prefix-SID locator originator, or falsely withdrawing conforming ISD routes
// whose MP_REACH_NLRI next hop differs or whose family is ipv4/mup.
func TestMUPISDOriginatorMismatch(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.30")
	locPrefix := netip.MustParsePrefix("2001:db8::/48")
	origAddr := netip.MustParseAddr("2001:db8:140::1")
	origPfx64 := netip.MustParsePrefix("2001:db8:236:10::/64")
	origV4Addr := netip.MustParseAddr("192.0.2.10")
	origV4Pfx24 := netip.MustParsePrefix("10.10.0.0/24")
	otherAddr := netip.MustParseAddr("2001:db8:999::9")

	knownEntry := locatororigin.LocatorEntry{
		Prefix:     locPrefix,
		Known:      true,
		Originator: "0100.0000.0001",
		Addresses:  []netip.Addr{origAddr, origV4Addr},
		Prefixes:   []netip.Prefix{netip.PrefixFrom(origAddr, 128), origPfx64, origV4Pfx24},
	}

	origAddrBytes := origAddr.AsSlice()
	otherAddrBytes := otherAddr.AsSlice()
	origPfx64Bytes := origPfx64.Addr().AsSlice()[:8]
	otherPfx64Bytes := netip.MustParseAddr("2001:db8:236:20::").AsSlice()[:8]

	v6Sibling := t2stRoute(160, append(append([]byte{}, ipv6Addr...), 0, 0, 0, 1)...)
	v4Sibling := t2stRoute(64, 192, 0, 2, 1, 0, 0, 0, 1)
	var scratch [nlrisplit.PrefixKeyScratchSize]byte

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1 positive -- an IPv6 or IPv4 ISD route whose NLRI Address belongs to the seeded Prefix-SID locator originator is kept, including when the MP_REACH_NLRI next hop differs (Section 3.3.3 ignores MP_REACH_NLRI next hop)
	t.Run("matching IPv6 and IPv4 ISD Address stays kept even with differing next hop", func(t *testing.T) {
		positiveCases := []struct {
			name    string
			fam     Family
			isd     []byte
			sibling []byte
			nh      []byte
		}{
			{
				name:    "IPv6 host /128 with matching next hop",
				fam:     IPv6MUP,
				isd:     isdRoute(128, origAddrBytes...),
				sibling: v6Sibling,
				nh:      origAddrBytes,
			},
			{
				name:    "IPv6 prefix /64 with matching next hop",
				fam:     IPv6MUP,
				isd:     isdRoute(64, origPfx64Bytes...),
				sibling: v6Sibling,
				nh:      origAddrBytes,
			},
			{
				name:    "IPv6 host /128 with differing MP_REACH next hop",
				fam:     IPv6MUP,
				isd:     isdRoute(128, origAddrBytes...),
				sibling: v6Sibling,
				nh:      otherAddrBytes,
			},
			{
				name:    "IPv4 host /32 in originator Addresses with differing next hop",
				fam:     IPv4MUP,
				isd:     isdRoute(32, 192, 0, 2, 10),
				sibling: v4Sibling,
				nh:      otherAddrBytes,
			},
			{
				name:    "IPv4 prefix /24 in originator Prefixes with differing next hop",
				fam:     IPv4MUP,
				isd:     isdRoute(24, 10, 10, 0),
				sibling: v4Sibling,
				nh:      otherAddrBytes,
			},
		}
		for _, tc := range positiveCases {
			t.Run(tc.name, func(t *testing.T) {
				capture, _ := setupMUPOriginTestEnv(t)
				seedMUPOriginSnapshot(knownEntry)
				recognize, check := nlritype.Rulings(tc.fam)
				section := concatNLRI(tc.isd, tc.sibling)
				attrs := mupTestUpdateAttrs(dsdAttrsPrefixSID, peer, tc.nh)
				r, err := nlritype.RetainUpdate(recognize, check, &attrs, tc.fam, section, false, true, scratch[:])
				require.NoError(t, err)
				assert.Equal(t, section, r.Kept)
				assert.False(t, r.Removed())
				assert.Equal(t, 0, capture.countByCode(codeDoctorBGPMUPLocatorMismatch))
				assert.Equal(t, 0, capture.countByCode(codeDoctorBGPMUPLocatorUnresolved))
			})
		}
	})

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1 negative -- an IPv6 or IPv4 ISD route whose NLRI Address does not belong to the proven Prefix-SID locator originator is rejected
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4 positive -- an ISD route failing the NLRI Address originator check is treated as withdrawn while its sibling routes remain kept
	t.Run("mismatching ISD Address withdraws ISD while keeping sibling", func(t *testing.T) {
		mismatchCases := []struct {
			name    string
			fam     Family
			isd     []byte
			sibling []byte
			nh      []byte
		}{
			{
				name:    "IPv6 /128 address mismatch with matching next hop",
				fam:     IPv6MUP,
				isd:     isdRoute(128, otherAddrBytes...),
				sibling: v6Sibling,
				nh:      origAddrBytes,
			},
			{
				name:    "IPv6 /64 prefix mismatch with matching next hop",
				fam:     IPv6MUP,
				isd:     isdRoute(64, otherPfx64Bytes...),
				sibling: v6Sibling,
				nh:      origAddrBytes,
			},
			{
				name:    "IPv4 /32 address mismatch against known originator",
				fam:     IPv4MUP,
				isd:     isdRoute(32, 192, 0, 2, 99),
				sibling: v4Sibling,
				nh:      origAddrBytes,
			},
			{
				name:    "IPv4 /24 prefix mismatch against known originator",
				fam:     IPv4MUP,
				isd:     isdRoute(24, 10, 99, 0),
				sibling: v4Sibling,
				nh:      origAddrBytes,
			},
		}
		for _, tc := range mismatchCases {
			t.Run(tc.name, func(t *testing.T) {
				capture, _ := setupMUPOriginTestEnv(t)
				seedMUPOriginSnapshot(knownEntry)
				recognize, check := nlritype.Rulings(tc.fam)
				section := concatNLRI(tc.isd, tc.sibling)
				attrs := mupTestUpdateAttrs(dsdAttrsPrefixSID, peer, tc.nh)
				r, err := nlritype.RetainUpdate(recognize, check, &attrs, tc.fam, section, false, true, scratch[:])
				require.NoError(t, err)
				assert.Equal(t, tc.sibling, r.Kept, "sibling must remain kept")
				assert.Equal(t, tc.isd, r.Withdrawn, "mismatched ISD must be treated as withdrawn")
				assert.Zero(t, r.Discarded)
				rec, ok := capture.lastByCode(codeDoctorBGPMUPLocatorMismatch)
				require.True(t, ok)
				assert.Equal(t, "address", rec.attrs["field"])
				assert.Equal(t, "0100.0000.0001", rec.attrs["originator"])
			})
		}
	})

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4 negative -- a conforming ISD route whose NLRI Address matches the known originator (even with a differing MP_REACH_NLRI next hop) is kept without warning, and an ISD route whose locator originator is unresolved (empty index, anycast, or undecoded TLV 237) is kept with an unresolved diagnostic warning rather than withdrawn
	t.Run("conforming ISD and unresolved originator ISD stay kept", func(t *testing.T) {
		recognize, check := nlritype.Rulings(IPv6MUP)

		// Conforming ISD with differing MP_REACH next hop stays kept without warning.
		capture, _ := setupMUPOriginTestEnv(t)
		seedMUPOriginSnapshot(knownEntry)
		okISD := isdRoute(128, origAddrBytes...)
		okSection := concatNLRI(okISD, v6Sibling)
		okAttrs := mupTestUpdateAttrs(dsdAttrsPrefixSID, peer, otherAddrBytes)
		r, err := nlritype.RetainUpdate(recognize, check, &okAttrs, IPv6MUP, okSection, false, true, scratch[:])
		require.NoError(t, err)
		assert.Equal(t, okSection, r.Kept)
		assert.False(t, r.Removed())
		assert.Equal(t, 0, capture.countByCode(codeDoctorBGPMUPLocatorMismatch))
		assert.Equal(t, 0, capture.countByCode(codeDoctorBGPMUPLocatorUnresolved))

		// Unresolved originator cases (empty index, anycast, undecoded TLV 237) stay kept with warning.
		unresolvedCases := []struct {
			name string
			seed func()
		}{
			{
				name: "empty index",
				seed: func() {},
			},
			{
				name: "anycast locator",
				seed: func() {
					seedMUPOriginSnapshot(locatororigin.LocatorEntry{
						Prefix:      locPrefix,
						Known:       false,
						ReasonClass: locatororigin.ReasonAnycast,
						Reason:      "covering locator 2001:db8::/48 carries the anycast A-flag (RFC 9352 Section 6)",
					})
				},
			},
			{
				name: "undecoded TLV 237 on originator LSP",
				seed: func() {
					seedMUPOriginSnapshot(locatororigin.LocatorEntry{
						Prefix:             locPrefix,
						Known:              true,
						Originator:         "0100.0000.0001",
						Addresses:          []netip.Addr{origAddr},
						HasUndecodedTLV237: true,
					})
				},
			},
		}
		badISD := isdRoute(128, otherAddrBytes...)
		badSection := concatNLRI(badISD, v6Sibling)
		for _, tc := range unresolvedCases {
			t.Run(tc.name, func(t *testing.T) {
				capture, _ := setupMUPOriginTestEnv(t)
				tc.seed()
				attrs := mupTestUpdateAttrs(dsdAttrsPrefixSID, peer, otherAddrBytes)
				r, err := nlritype.RetainUpdate(recognize, check, &attrs, IPv6MUP, badSection, false, true, scratch[:])
				require.NoError(t, err)
				assert.Equal(t, badSection, r.Kept)
				assert.False(t, r.Removed())
				assert.Equal(t, 1, capture.countByCode(codeDoctorBGPMUPLocatorUnresolved))
			})
		}
	})
}
