// Design: docs/architecture/mrt.md -- RFC 6397 GEO_PEER_TABLE encoding and validation
// RFC: rfc/short/rfc6397.md -- MRT BGP Routing Information Export Format with Geo-Location Extensions
// Related: decode.go -- GeoPeerTable, GeoPeerEntry, DecodeGeoPeerTable
// Related: encode.go -- WritePeerIndexTable

package mrt

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"
)

var (
	// ErrGeoMixedCoords reports a latitude/longitude pair mixing WGS84 coordinates with NaN.
	ErrGeoMixedCoords = errors.New("mrt: mixed WGS84 coordinate and NaN value")
	// ErrGeoCoordRange reports a WGS84 latitude outside [-90, 90], longitude outside [-180, 180], or infinity.
	ErrGeoCoordRange = errors.New("mrt: WGS84 coordinate out of range")
	// ErrGeoPeerTableMismatch reports a GEO_PEER_TABLE whose peer entries do not match PEER_INDEX_TABLE.
	ErrGeoPeerTableMismatch = errors.New("mrt: GEO_PEER_TABLE peer entries do not match PEER_INDEX_TABLE")
)

// GeoPrivacyRules captures the RFC 6397 Section 6 / RFC 4119 GEOPRIV privacy
// rules attached to a location object, compared against RFC 6397's default
// rules (retransmission-allowed: true, retention-expires: 100 years,
// ruleset-reference: empty, note-well: empty).
type GeoPrivacyRules struct {
	RetransmissionAllowed *bool
	RetentionYears        *int
	RulesetReference      string
	NoteWell              string
	NonTechnicalMeasures  bool
}

// IsMoreRestrictive reports whether the location object carries privacy rules
// stricter than RFC 6397 Section 6's default MRT geo-location rules.
func (p GeoPrivacyRules) IsMoreRestrictive() bool {
	if p.RetransmissionAllowed != nil && !*p.RetransmissionAllowed {
		return true
	}
	if p.RetentionYears != nil && *p.RetentionYears < 100 {
		return true
	}
	if p.RulesetReference != "" {
		return true
	}
	return p.NoteWell != ""
}

// AllowInclude reports whether location coordinates governed by these privacy
// rules may be included in an MRT geo-location record.
//
// RFC 6397 Section 6: "Location information derived from a location object with
// more restrictive privacy rules MUST NOT be included in an MRT geo-location
// record unless there are non-technical measures in place that enforce and
// communicate the constraints on the use of the location information in the MRT
// geo-location format (e.g., contractual agreements between peers)."
func (p GeoPrivacyRules) AllowInclude() bool {
	if !p.IsMoreRestrictive() {
		return true
	}
	return p.NonTechnicalMeasures
}

// GeoLocation holds one entity's configured WGS84 decimal-degree coordinates
// (or NaN, NaN when private) together with its GEOPRIV privacy metadata.
type GeoLocation struct {
	Latitude  float32
	Longitude float32
	Privacy   GeoPrivacyRules
}

// PrivateGeoLocation returns a GeoLocation with both coordinates set to IEEE 754 NaN.
func PrivateGeoLocation() GeoLocation {
	nan := float32(math.NaN())
	return GeoLocation{Latitude: nan, Longitude: nan}
}

// ValidateGeoCoordPair verifies that lat and lon are either both IEEE 754 NaN
// or both valid finite WGS84 decimal degrees (latitude in [-90, 90], longitude
// in [-180, 180]).
//
// RFC 6397 Section 4.1: "The Collector Latitude and Collector Longitude MUST
// NOT be a mix of WGS84 [WGS-84] datum coordinates and NAN values."
//
// RFC 6397 Section 4.1: "The Peer Latitude and Peer Longitude MUST NOT be a mix
// of WGS84 [WGS-84] datum coordinates and NAN values for a single peer."
func ValidateGeoCoordPair(lat, lon float32) error {
	latNaN := math.IsNaN(float64(lat))
	lonNaN := math.IsNaN(float64(lon))
	if latNaN != lonNaN {
		return ErrGeoMixedCoords
	}
	if latNaN {
		return nil
	}
	if math.IsInf(float64(lat), 0) {
		return ErrGeoCoordRange
	}
	if math.IsInf(float64(lon), 0) {
		return ErrGeoCoordRange
	}
	if lat < -90 {
		return ErrGeoCoordRange
	}
	if lat > 90 {
		return ErrGeoCoordRange
	}
	if lon < -180 {
		return ErrGeoCoordRange
	}
	if lon > 180 {
		return ErrGeoCoordRange
	}
	return nil
}

// GeoPeerTableSize returns the encoded byte length of a GEO_PEER_TABLE body
// for peerCount peers: CollectorBGPID(4) + Lat(4) + Lon(4) + PeerCount(2) +
// peerCount * (Type(1) + BGPID(4) + Lat(4) + Lon(4)).
func GeoPeerTableSize(peerCount int) int {
	return 14 + peerCount*13
}

// WriteGeoPeerTable encodes a TABLE_DUMP_V2 GEO_PEER_TABLE record body into
// buf starting at off, rejecting mixed WGS84/NaN coordinates before writing.
//
// RFC 6397 Section 4.1: "The Collector Latitude and Collector Longitude MUST
// NOT be a mix of WGS84 [WGS-84] datum coordinates and NAN values."
//
// RFC 6397 Section 4.1: "The Peer Latitude and Peer Longitude MUST NOT be a mix
// of WGS84 [WGS-84] datum coordinates and NAN values for a single peer."
func WriteGeoPeerTable(buf []byte, off int, table *GeoPeerTable) (int, error) {
	if table == nil {
		return 0, fmt.Errorf("geo peer table: %w", ErrShortData)
	}
	if err := ValidateGeoCoordPair(table.CollectorLat, table.CollectorLon); err != nil {
		return 0, fmt.Errorf("collector coordinates: %w", err)
	}
	if len(table.Peers) > math.MaxUint16 {
		return 0, fmt.Errorf("geo peer table: peer count %d exceeds uint16", len(table.Peers))
	}
	for i := range table.Peers {
		if err := ValidateGeoCoordPair(table.Peers[i].Latitude, table.Peers[i].Longitude); err != nil {
			return 0, fmt.Errorf("peer %d coordinates: %w", i, err)
		}
	}
	needed := GeoPeerTableSize(len(table.Peers))
	if off+needed > len(buf) {
		return 0, fmt.Errorf("geo peer table buffer: %w", ErrShortData)
	}

	start := off
	copy(buf[off:], table.CollectorBGPID[:])
	off += 4
	binary.BigEndian.PutUint32(buf[off:], math.Float32bits(table.CollectorLat))
	off += 4
	binary.BigEndian.PutUint32(buf[off:], math.Float32bits(table.CollectorLon))
	off += 4
	binary.BigEndian.PutUint16(buf[off:], uint16(len(table.Peers)))
	off += 2

	for i := range table.Peers {
		p := &table.Peers[i]
		buf[off] = p.Type
		off++
		copy(buf[off:], p.BGPID[:])
		off += 4
		binary.BigEndian.PutUint32(buf[off:], math.Float32bits(p.Latitude))
		off += 4
		binary.BigEndian.PutUint32(buf[off:], math.Float32bits(p.Longitude))
		off += 4
	}
	return off - start, nil
}

// BuildGeoPeerTableForIndex constructs a GEO_PEER_TABLE companion to a
// PEER_INDEX_TABLE, preserving exact peer count, order, Type, and BGP ID while
// enforcing coordinate validity and RFC 6397 Section 6 privacy filtering.
//
// RFC 6397 Section 4.1: "The order of the Peer Entries in the GEO_PEER_TABLE
// MUST match the order and number as existing in the PEER_INDEX_TABLE."
//
// RFC 6397 Section 6: "Location information derived from a location object with
// more restrictive privacy rules MUST NOT be included in an MRT geo-location
// record unless there are non-technical measures in place that enforce and
// communicate the constraints on the use of the location information in the MRT
// geo-location format (e.g., contractual agreements between peers)."
func BuildGeoPeerTableForIndex(collectorBGPID [4]byte, collector GeoLocation, peers []PeerEntry, peerLocations map[string]GeoLocation) (*GeoPeerTable, error) {
	if err := ValidateGeoCoordPair(collector.Latitude, collector.Longitude); err != nil {
		return nil, fmt.Errorf("collector coordinates: %w", err)
	}
	nan := float32(math.NaN())
	colLat, colLon := collector.Latitude, collector.Longitude
	if !collector.Privacy.AllowInclude() {
		colLat, colLon = nan, nan
	}

	geoPeers := make([]GeoPeerEntry, len(peers))
	for i := range peers {
		lat, lon := nan, nan
		ipKey := net.IP(peers[i].IP).String()
		if loc, found := peerLocations[ipKey]; found {
			if err := ValidateGeoCoordPair(loc.Latitude, loc.Longitude); err != nil {
				return nil, fmt.Errorf("peer %s coordinates: %w", ipKey, err)
			}
			if loc.Privacy.AllowInclude() {
				lat, lon = loc.Latitude, loc.Longitude
			}
		}
		geoPeers[i] = GeoPeerEntry{
			Type:      peers[i].Type,
			BGPID:     peers[i].BGPID,
			Latitude:  lat,
			Longitude: lon,
		}
	}

	return &GeoPeerTable{
		CollectorBGPID: collectorBGPID,
		CollectorLat:   colLat,
		CollectorLon:   colLon,
		Peers:          geoPeers,
	}, nil
}

// ValidateGeoPeerTableAgainstIndex verifies that a decoded GEO_PEER_TABLE
// matches its companion PEER_INDEX_TABLE in peer count, order, Peer Type, and
// Peer BGP ID.
//
// RFC 6397 Section 4.1: "The order of the Peer Entries in the GEO_PEER_TABLE
// MUST match the order and number as existing in the PEER_INDEX_TABLE."
func ValidateGeoPeerTableAgainstIndex(pit *PeerIndexTable, gpt *GeoPeerTable) error {
	if pit == nil {
		return ErrGeoPeerTableMismatch
	}
	if gpt == nil {
		return ErrGeoPeerTableMismatch
	}
	if len(pit.Peers) != len(gpt.Peers) {
		return fmt.Errorf("%w: peer count %d != %d", ErrGeoPeerTableMismatch, len(gpt.Peers), len(pit.Peers))
	}
	for i := range pit.Peers {
		if pit.Peers[i].Type != gpt.Peers[i].Type {
			return fmt.Errorf("%w: peer %d type 0x%02x != 0x%02x", ErrGeoPeerTableMismatch, i, gpt.Peers[i].Type, pit.Peers[i].Type)
		}
		if pit.Peers[i].BGPID != gpt.Peers[i].BGPID {
			return fmt.Errorf("%w: peer %d bgp id %v != %v", ErrGeoPeerTableMismatch, i, gpt.Peers[i].BGPID, pit.Peers[i].BGPID)
		}
	}
	return nil
}
