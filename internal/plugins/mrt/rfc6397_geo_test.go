// Design: docs/architecture/mrt.md -- RFC 6397 GEO_PEER_TABLE end-to-end dump and validation
// Related: dump.go -- writePeerIndexTable, writeGeoPeerTable
// Related: register.go -- ParseConfig, parseGeoConfig
// Related: internal/mrt/geo.go -- ValidateGeoCoordPair, WriteGeoPeerTable, BuildGeoPeerTableForIndex, ValidateGeoPeerTableAgainstIndex
// RFC 6397 Sections 4.1 and 6 -- see rfc/short/rfc6397.md

package mrt

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	mrtfmt "github.com/ze-software/ze/internal/mrt"
)

type stubGeoRIBDumper struct {
	peers []struct {
		addr   string
		asn    uint32
		bgpID  [4]byte
		isIPv6 bool
	}
}

func (s *stubGeoRIBDumper) DumpRIB(v registry.RIBDumpVisitor) {
	for _, p := range s.peers {
		v.OnPeer(p.addr, p.asn, p.bgpID, p.isIPv6)
	}
	if len(s.peers) > 0 {
		v.OnRoute(0, mrtfmt.AFIIPv4, 1, 24, []byte{198, 51, 100}, []byte{0x40, 1, 1, 0})
	}
}

func buildMRTPeerAndGeoStream(t *testing.T, pit *mrtfmt.PeerIndexTable, gpt *mrtfmt.GeoPeerTable) []byte {
	t.Helper()
	var stream bytes.Buffer
	now := time.Unix(1700000000, 0)
	hdrSize := mrtfmt.CommonHeaderLen

	pitSize := 4 + 2 + 2 + len(pit.ViewName)
	for i := range pit.Peers {
		pitSize += 5 + len(pit.Peers[i].IP) + 4
	}
	pitBuf := make([]byte, hdrSize+pitSize)
	pitN, err := mrtfmt.WritePeerIndexTable(pitBuf, hdrSize, pit.CollectorBGPID, pit.ViewName, pit.Peers)
	if err != nil {
		t.Fatalf("WritePeerIndexTable: %v", err)
	}
	writeHeader(pitBuf, false, now, mrtfmt.TypeTableDumpV2, mrtfmt.TDV2PeerIndexTable, pitN)
	stream.Write(pitBuf[:hdrSize+pitN])

	gptBuf := make([]byte, hdrSize+mrtfmt.GeoPeerTableSize(len(gpt.Peers)))
	gptN, err := mrtfmt.WriteGeoPeerTable(gptBuf, hdrSize, gpt)
	if err != nil {
		t.Fatalf("WriteGeoPeerTable: %v", err)
	}
	writeHeader(gptBuf, false, now, mrtfmt.TypeTableDumpV2, mrtfmt.TDV2GeoPeerTable, gptN)
	stream.Write(gptBuf[:hdrSize+gptN])
	return stream.Bytes()
}

// TestRFC6397GeoPeerTableCoordsOrderAndPrivacy proves all four RFC 6397 MUST
// obligations end-to-end through ParseConfig, Component.DumpRIB, wire encoding,
// and mrt.ReadFile / mrt.ReadFrom / DecodeGeoPeerTable.
//
// VALIDATES:
//   - RFC6397-4.1-1: Collector Latitude and Longitude reject mixed WGS84/NaN
//     values in both directions at config parse, wire encode, and wire decode,
//     while accepting both paired WGS84 and paired private (NaN, NaN) end-to-end.
//   - RFC6397-4.1-2: Peer Latitude and Longitude reject mixed WGS84/NaN values
//     for a single peer at config parse, wire encode, and wire decode, while
//     accepting paired WGS84 and paired NaN (including independent peers).
//   - RFC6397-4.1-3: GEO_PEER_TABLE emitted by DumpRIB matches PEER_INDEX_TABLE
//     in exact peer count, order, Peer Type, and Peer BGP ID, and mrt.ReadFrom
//     rejects streams whose GEO_PEER_TABLE has reordered Peer Type, reordered
//     BGP ID (same Peer Type), or mismatched peer count.
//   - RFC6397-6-1: Every restrictive GEOPRIV privacy rule (retransmission-allowed=false,
//     retention-years<100, non-empty ruleset-reference, non-empty note-well)
//     independently excludes coordinates (NaN, NaN) end-to-end through ParseConfig
//     -> DumpRIB -> ReadFile when non-technical-measures=false, while default rules
//     and non-technical-measures=true preserve WGS84 coordinates.
//
// RFC requirement: RFC6397-4.1-1 positive -- mixed Collector Latitude/Longitude (WGS84 + NaN, and NaN + WGS84) is rejected by ParseConfig, WriteGeoPeerTable, and DecodeGeoPeerTable with ErrGeoMixedCoords.
// RFC requirement: RFC6397-4.1-1 negative -- paired Collector WGS84 coordinates (48.8566, 2.3522) and paired private NaN coordinates (NaN, NaN) are both accepted end-to-end through ParseConfig -> DumpRIB -> ReadFile and round-trip cleanly.
// RFC requirement: RFC6397-4.1-2 positive -- mixed Peer Latitude/Longitude (WGS84 + NaN, and NaN + WGS84) for a single peer is rejected by ParseConfig, WriteGeoPeerTable, and DecodeGeoPeerTable with ErrGeoMixedCoords.
// RFC requirement: RFC6397-4.1-2 negative -- paired Peer WGS84 coordinates and paired Peer NaN coordinates (including one WGS84 peer alongside a NaN peer) are accepted end-to-end and round-trip cleanly.
// RFC requirement: RFC6397-4.1-3 positive -- DumpRIB emits GEO_PEER_TABLE whose Peer Entries match PEER_INDEX_TABLE in exact count, order, Type, and BGP ID, accepted by mrt.ReadFile and ValidateGeoPeerTableAgainstIndex.
// RFC requirement: RFC6397-4.1-3 negative -- mrt.ReadFrom rejects a GEO_PEER_TABLE stream whose peer entries swap Peer Type, swap same-Type BGP IDs, or differ in peer count, returning ErrGeoPeerTableMismatch.
// RFC requirement: RFC6397-6-1 positive -- each of the four restrictive GEOPRIV privacy rules (retransmission-allowed=false, retention-years<100, non-empty ruleset-reference, non-empty note-well) independently excludes coordinates (emitted as NaN, NaN) end-to-end through ParseConfig -> DumpRIB -> ReadFile when non-technical-measures is false.
// RFC requirement: RFC6397-6-1 negative -- location data with default RFC 6397 privacy rules, or restrictive rules accompanied by non-technical-measures=true, is included with its WGS84 coordinates in the emitted GEO_PEER_TABLE.
func TestRFC6397GeoPeerTableCoordsOrderAndPrivacy(t *testing.T) {
	// RFC6397-4.1-1 positive: ParseConfig, WriteGeoPeerTable, and DecodeGeoPeerTable reject mixed collector WGS84/NaN in both directions.
	for _, tc := range []struct {
		name string
		lat  string
		lon  string
	}{
		{"lat-wgs84-lon-nan", "48.8566", "NaN"},
		{"lat-nan-lon-wgs84", "NaN", "2.3522"},
	} {
		rawCfg, _ := json.Marshal(map[string]any{
			"mrt": map[string]any{
				"routes": map[string]any{
					"file":     filepath.Join(t.TempDir(), "rib.mrt"),
					"interval": 3600,
					"geo": map[string]any{
						"enabled": true,
						"collector": map[string]any{
							"latitude":  tc.lat,
							"longitude": tc.lon,
						},
					},
				},
			},
		})
		if _, err := ParseConfig(rawCfg); !errors.Is(err, mrtfmt.ErrGeoMixedCoords) {
			t.Fatalf("ParseConfig(%s) err = %v, want ErrGeoMixedCoords", tc.name, err)
		}
	}

	nan := float32(math.NaN())
	buf := make([]byte, 128)
	for _, tc := range []struct {
		name string
		lat  float32
		lon  float32
	}{
		{"wgs84-nan", 48.8566, nan},
		{"nan-wgs84", nan, 2.3522},
	} {
		if _, err := mrtfmt.WriteGeoPeerTable(buf, 0, &mrtfmt.GeoPeerTable{
			CollectorLat: tc.lat,
			CollectorLon: tc.lon,
		}); !errors.Is(err, mrtfmt.ErrGeoMixedCoords) {
			t.Fatalf("WriteGeoPeerTable(mixed collector %s) err = %v, want ErrGeoMixedCoords", tc.name, err)
		}
	}
	// Wire decode of mixed collector bytes in both directions must fail.
	badColLatNan := make([]byte, 14)
	badColLatNan[4], badColLatNan[5], badColLatNan[6], badColLatNan[7] = 0x7f, 0xc0, 0x00, 0x00
	badColLatNan[8], badColLatNan[9], badColLatNan[10], badColLatNan[11] = 0x40, 0x16, 0x8a, 0x72
	if _, err := mrtfmt.DecodeGeoPeerTable(badColLatNan); !errors.Is(err, mrtfmt.ErrGeoMixedCoords) {
		t.Fatalf("DecodeGeoPeerTable(mixed collector NaN/WGS84) err = %v, want ErrGeoMixedCoords", err)
	}
	badColLonNan := make([]byte, 14)
	badColLonNan[4], badColLonNan[5], badColLonNan[6], badColLonNan[7] = 0x42, 0x43, 0x6d, 0x28
	badColLonNan[8], badColLonNan[9], badColLonNan[10], badColLonNan[11] = 0x7f, 0xc0, 0x00, 0x00
	if _, err := mrtfmt.DecodeGeoPeerTable(badColLonNan); !errors.Is(err, mrtfmt.ErrGeoMixedCoords) {
		t.Fatalf("DecodeGeoPeerTable(mixed collector WGS84/NaN) err = %v, want ErrGeoMixedCoords", err)
	}

	// RFC6397-4.1-2 positive: mixed peer WGS84/NaN rejected by ParseConfig, WriteGeoPeerTable, and DecodeGeoPeerTable.
	for _, tc := range []struct {
		name string
		lat  string
		lon  string
	}{
		{"peer-lat-wgs84-lon-nan", "51.5074", "NaN"},
		{"peer-lat-nan-lon-wgs84", "NaN", "-0.1278"},
	} {
		rawCfg, _ := json.Marshal(map[string]any{
			"mrt": map[string]any{
				"routes": map[string]any{
					"file":     filepath.Join(t.TempDir(), "rib.mrt"),
					"interval": 3600,
					"geo": map[string]any{
						"enabled": true,
						"peer": map[string]any{
							"192.0.2.1": map[string]any{
								"address":   "192.0.2.1",
								"latitude":  tc.lat,
								"longitude": tc.lon,
							},
						},
					},
				},
			},
		})
		if _, err := ParseConfig(rawCfg); !errors.Is(err, mrtfmt.ErrGeoMixedCoords) {
			t.Fatalf("ParseConfig(%s) err = %v, want ErrGeoMixedCoords", tc.name, err)
		}
	}
	if _, err := mrtfmt.WriteGeoPeerTable(buf, 0, &mrtfmt.GeoPeerTable{
		CollectorLat: nan,
		CollectorLon: nan,
		Peers: []mrtfmt.GeoPeerEntry{
			{Type: mrtfmt.PeerAS4, BGPID: [4]byte{1, 1, 1, 1}, Latitude: nan, Longitude: 10.0},
		},
	}); !errors.Is(err, mrtfmt.ErrGeoMixedCoords) {
		t.Fatalf("WriteGeoPeerTable(mixed peer) err = %v, want ErrGeoMixedCoords", err)
	}
	badPeerWire := make([]byte, 14+13)
	badPeerWire[4], badPeerWire[5] = 0x7f, 0xc0
	badPeerWire[8], badPeerWire[9] = 0x7f, 0xc0
	badPeerWire[13] = 1
	badPeerWire[19], badPeerWire[20] = 0x7f, 0xc0
	if _, err := mrtfmt.DecodeGeoPeerTable(badPeerWire); !errors.Is(err, mrtfmt.ErrGeoMixedCoords) {
		t.Fatalf("DecodeGeoPeerTable(mixed peer) err = %v, want ErrGeoMixedCoords", err)
	}

	// RFC6397-4.1-1 negative (paired NaN collector): end-to-end ParseConfig -> DumpRIB -> ReadFile
	// proving paired private (NaN, NaN) collector coordinates are accepted and round-trip cleanly.
	nanColFile := filepath.Join(t.TempDir(), "geo-nan-collector.mrt")
	nanColCfgRaw, err := json.Marshal(map[string]any{
		"mrt": map[string]any{
			"routes": map[string]any{
				"file":     nanColFile,
				"interval": 3600,
				"geo": map[string]any{
					"enabled": "true",
					"collector": map[string]any{
						"latitude":  "NaN",
						"longitude": "NaN",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal nan collector config: %v", err)
	}
	nanColCfg, err := ParseConfig(nanColCfgRaw)
	if err != nil {
		t.Fatalf("ParseConfig(paired NaN collector): %v", err)
	}
	nanColComp := New(*nanColCfg, slog.New(slog.DiscardHandler))
	nanColComp.Start(nil)
	nanColComp.ribDumper = &stubGeoRIBDumper{
		peers: []struct {
			addr   string
			asn    uint32
			bgpID  [4]byte
			isIPv6 bool
		}{
			{"192.0.2.1", 65001, [4]byte{10, 0, 0, 1}, false},
		},
	}
	nanColComp.DumpRIB()
	nanColComp.Stop()

	var nanColGPT *mrtfmt.GeoPeerTable
	if err := mrtfmt.ReadFile(nanColFile, &mrtfmt.Handler{
		OnGeoPeer: func(_ mrtfmt.Header, g *mrtfmt.GeoPeerTable) error {
			nanColGPT = g
			return nil
		},
	}); err != nil {
		t.Fatalf("ReadFile(paired NaN collector): %v", err)
	}
	if nanColGPT == nil {
		t.Fatalf("expected GEO_PEER_TABLE for paired NaN collector dump")
	}
	if !math.IsNaN(float64(nanColGPT.CollectorLat)) || !math.IsNaN(float64(nanColGPT.CollectorLon)) {
		t.Fatalf("paired NaN collector decoded as (%v, %v), want (NaN, NaN)", nanColGPT.CollectorLat, nanColGPT.CollectorLon)
	}

	// End-to-end DumpRIB test proving:
	// - RFC6397-4.1-1 negative (collector WGS84 48.8566, 2.3522)
	// - RFC6397-4.1-2 negative (peer 0 WGS84 alongside peer 1 private NaN)
	// - RFC6397-4.1-3 positive (7 peers in exact PEER_INDEX_TABLE order, IPv4 + IPv6 mixed)
	// - RFC6397-6-1 positive (all 4 restrictive GEOPRIV rules tested independently on peers 2..5)
	// - RFC6397-6-1 negative (default privacy on peer 0, and restrictive rules + non-technical-measures=true on peer 6)
	outFile := filepath.Join(t.TempDir(), "geo-rib.mrt")
	rawCfg, err := json.Marshal(map[string]any{
		"mrt": map[string]any{
			"routes": map[string]any{
				"file":     outFile,
				"interval": 3600,
				"geo": map[string]any{
					"enabled": "true",
					"collector": map[string]any{
						"latitude":  "48.8566",
						"longitude": "2.3522",
					},
					"peer": map[string]any{
						"192.0.2.1": map[string]any{
							"address":   "192.0.2.1",
							"latitude":  "51.5074",
							"longitude": "-0.1278",
						},
						"2001:db8::2": map[string]any{
							"address":   "2001:db8::2",
							"latitude":  "NaN",
							"longitude": "NaN",
						},
						"192.0.2.3": map[string]any{
							"address":                "192.0.2.3",
							"latitude":               "37.7749",
							"longitude":              "-122.4194",
							"retransmission-allowed": "false",
							"non-technical-measures": "false",
						},
						"192.0.2.4": map[string]any{
							"address":                "192.0.2.4",
							"latitude":               "40.7128",
							"longitude":              "-74.0060",
							"retention-years":        "99",
							"non-technical-measures": "false",
						},
						"192.0.2.5": map[string]any{
							"address":                "192.0.2.5",
							"latitude":               "52.5200",
							"longitude":              "13.4050",
							"ruleset-reference":      "https://example.net/geo-policy",
							"non-technical-measures": "false",
						},
						"192.0.2.6": map[string]any{
							"address":                "192.0.2.6",
							"latitude":               "47.3769",
							"longitude":              "8.5417",
							"note-well":              "Do not redistribute",
							"non-technical-measures": "false",
						},
						"192.0.2.7": map[string]any{
							"address":                "192.0.2.7",
							"latitude":               "35.6762",
							"longitude":              "139.6503",
							"retransmission-allowed": "false",
							"retention-years":        "10",
							"ruleset-reference":      "https://example.net/geo-policy",
							"note-well":              "Contract allows archive",
							"non-technical-measures": "true",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal geo config: %v", err)
	}
	cfg, err := ParseConfig(rawCfg)
	if err != nil {
		t.Fatalf("ParseConfig(valid geo): %v", err)
	}

	comp := New(*cfg, slog.New(slog.DiscardHandler))
	comp.Start(nil)
	comp.ribDumper = &stubGeoRIBDumper{
		peers: []struct {
			addr   string
			asn    uint32
			bgpID  [4]byte
			isIPv6 bool
		}{
			{"192.0.2.1", 65001, [4]byte{10, 0, 0, 1}, false},
			{"2001:db8::2", 65002, [4]byte{10, 0, 0, 2}, true},
			{"192.0.2.3", 65003, [4]byte{10, 0, 0, 3}, false},
			{"192.0.2.4", 65004, [4]byte{10, 0, 0, 4}, false},
			{"192.0.2.5", 65005, [4]byte{10, 0, 0, 5}, false},
			{"192.0.2.6", 65006, [4]byte{10, 0, 0, 6}, false},
			{"192.0.2.7", 65007, [4]byte{10, 0, 0, 7}, false},
		},
	}
	comp.DumpRIB()
	comp.Stop()

	var pit *mrtfmt.PeerIndexTable
	var gpt *mrtfmt.GeoPeerTable
	if err := mrtfmt.ReadFile(outFile, &mrtfmt.Handler{
		OnPeerIndex: func(_ mrtfmt.Header, p *mrtfmt.PeerIndexTable) error {
			pit = p
			return nil
		},
		OnGeoPeer: func(_ mrtfmt.Header, g *mrtfmt.GeoPeerTable) error {
			gpt = g
			return nil
		},
	}); err != nil {
		t.Fatalf("ReadFile(%s): %v", outFile, err)
	}
	if pit == nil || gpt == nil {
		t.Fatalf("expected both PEER_INDEX_TABLE and GEO_PEER_TABLE, got pit=%v gpt=%v", pit != nil, gpt != nil)
	}

	// RFC6397-4.1-3 positive: exact count, order, Peer Type, and BGP ID match.
	if err := mrtfmt.ValidateGeoPeerTableAgainstIndex(pit, gpt); err != nil {
		t.Fatalf("ValidateGeoPeerTableAgainstIndex: %v", err)
	}
	// RFC6397-4.1-1 negative: collector WGS84 round-tripped.
	if math.Abs(float64(gpt.CollectorLat)-48.8566) > 1e-3 || math.Abs(float64(gpt.CollectorLon)-2.3522) > 1e-3 {
		t.Fatalf("collector coords = (%v, %v), want (48.8566, 2.3522)", gpt.CollectorLat, gpt.CollectorLon)
	}
	// Peer 0 (default privacy, WGS84): included.
	if math.Abs(float64(gpt.Peers[0].Latitude)-51.5074) > 1e-3 || math.Abs(float64(gpt.Peers[0].Longitude)+0.1278) > 1e-3 {
		t.Fatalf("peer 0 coords = (%v, %v), want (51.5074, -0.1278)", gpt.Peers[0].Latitude, gpt.Peers[0].Longitude)
	}
	// Peer 1 (private NaN): both NaN.
	if !math.IsNaN(float64(gpt.Peers[1].Latitude)) || !math.IsNaN(float64(gpt.Peers[1].Longitude)) {
		t.Fatalf("peer 1 private coords = (%v, %v), want (NaN, NaN)", gpt.Peers[1].Latitude, gpt.Peers[1].Longitude)
	}
	// RFC6397-6-1 positive: all 4 restrictive GEOPRIV rules independently exclude peer coords (NaN, NaN).
	for idx, rule := range map[int]string{
		2: "retransmission-allowed=false",
		3: "retention-years=99 (<100)",
		4: "ruleset-reference non-empty",
		5: "note-well non-empty",
	} {
		if !math.IsNaN(float64(gpt.Peers[idx].Latitude)) || !math.IsNaN(float64(gpt.Peers[idx].Longitude)) {
			t.Fatalf("peer %d (%s) leaked coords (%v, %v), want (NaN, NaN)",
				idx, rule, gpt.Peers[idx].Latitude, gpt.Peers[idx].Longitude)
		}
	}
	// RFC6397-6-1 negative: Peer 6 (all restrictive rules + non-technical-measures=true) IS included.
	if math.Abs(float64(gpt.Peers[6].Latitude)-35.6762) > 1e-3 || math.Abs(float64(gpt.Peers[6].Longitude)-139.6503) > 1e-3 {
		t.Fatalf("peer 6 coords with non-technical-measures = (%v, %v), want (35.6762, 139.6503)", gpt.Peers[6].Latitude, gpt.Peers[6].Longitude)
	}

	// RFC6397-6-1 positive on collector end-to-end: collector note-well without non-technical-measures excludes collector coords.
	colNoteFile := filepath.Join(t.TempDir(), "geo-restricted-collector.mrt")
	colNoteCfgRaw, err := json.Marshal(map[string]any{
		"mrt": map[string]any{
			"routes": map[string]any{
				"file":     colNoteFile,
				"interval": 3600,
				"geo": map[string]any{
					"enabled": "true",
					"collector": map[string]any{
						"latitude":               "48.8566",
						"longitude":              "2.3522",
						"note-well":              "Do not redistribute",
						"non-technical-measures": "false",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal restricted collector config: %v", err)
	}
	colNoteCfg, err := ParseConfig(colNoteCfgRaw)
	if err != nil {
		t.Fatalf("ParseConfig(restricted collector): %v", err)
	}
	colNoteComp := New(*colNoteCfg, slog.New(slog.DiscardHandler))
	colNoteComp.Start(nil)
	colNoteComp.ribDumper = &stubGeoRIBDumper{
		peers: []struct {
			addr   string
			asn    uint32
			bgpID  [4]byte
			isIPv6 bool
		}{
			{"192.0.2.1", 65001, [4]byte{10, 0, 0, 1}, false},
		},
	}
	colNoteComp.DumpRIB()
	colNoteComp.Stop()

	var colNoteGPT *mrtfmt.GeoPeerTable
	if err := mrtfmt.ReadFile(colNoteFile, &mrtfmt.Handler{
		OnGeoPeer: func(_ mrtfmt.Header, g *mrtfmt.GeoPeerTable) error {
			colNoteGPT = g
			return nil
		},
	}); err != nil {
		t.Fatalf("ReadFile(restricted collector): %v", err)
	}
	if colNoteGPT == nil || !math.IsNaN(float64(colNoteGPT.CollectorLat)) || !math.IsNaN(float64(colNoteGPT.CollectorLon)) {
		t.Fatalf("restricted collector coords leaked (%v), want (NaN, NaN)", colNoteGPT)
	}

	// RFC6397-4.1-3 negative through real mrt.ReadFrom stream validation:
	// 1) swapped Peer Type (peer 0 IPv4 vs peer 1 IPv6),
	// 2) swapped BGP ID with identical Peer Type (peer 2 IPv4 vs peer 3 IPv4),
	// 3) truncated peer count.
	swappedType := *gpt
	swappedType.Peers = append([]mrtfmt.GeoPeerEntry(nil), gpt.Peers...)
	swappedType.Peers[0], swappedType.Peers[1] = swappedType.Peers[1], swappedType.Peers[0]
	if err := mrtfmt.ReadFrom(bytes.NewReader(buildMRTPeerAndGeoStream(t, pit, &swappedType)), &mrtfmt.Handler{}); !errors.Is(err, mrtfmt.ErrGeoPeerTableMismatch) {
		t.Fatalf("ReadFrom(swapped Peer Type) = %v, want ErrGeoPeerTableMismatch", err)
	}

	swappedBGPID := *gpt
	swappedBGPID.Peers = append([]mrtfmt.GeoPeerEntry(nil), gpt.Peers...)
	swappedBGPID.Peers[2], swappedBGPID.Peers[3] = swappedBGPID.Peers[3], swappedBGPID.Peers[2]
	if err := mrtfmt.ReadFrom(bytes.NewReader(buildMRTPeerAndGeoStream(t, pit, &swappedBGPID)), &mrtfmt.Handler{}); !errors.Is(err, mrtfmt.ErrGeoPeerTableMismatch) {
		t.Fatalf("ReadFrom(swapped same-Type BGP ID) = %v, want ErrGeoPeerTableMismatch", err)
	}

	truncated := *gpt
	truncated.Peers = gpt.Peers[:len(gpt.Peers)-1]
	if err := mrtfmt.ReadFrom(bytes.NewReader(buildMRTPeerAndGeoStream(t, pit, &truncated)), &mrtfmt.Handler{}); !errors.Is(err, mrtfmt.ErrGeoPeerTableMismatch) {
		t.Fatalf("ReadFrom(truncated peers) = %v, want ErrGeoPeerTableMismatch", err)
	}
}
