// Design: docs/architecture/mrt.md -- prefix table extraction from MRT
// RFC: rfc/short/rfc6396.md -- per-record-type AS width, RIB-entry MP_REACH truncation
// Related: show.go -- human-readable dump of the same records

package analyze

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/mrt"
)

const routesUsage = `ze-analyze routes -- extract prefix table from MRT

Reads TABLE_DUMP_V2 RIB entries and outputs one JSON object per route
with prefix, next-hop, AS path, origin, and communities.

Usage:
  ze-analyze routes <file.mrt[.gz|.bz2]> [--limit <n>]
`

type routeRecord struct {
	Prefix              string   `json:"prefix"`
	NextHop             string   `json:"next-hop"`
	ASPath              []uint32 `json:"as-path"`
	Origin              string   `json:"origin"`
	LocalPref           uint32   `json:"local-pref,omitempty"`
	MED                 uint32   `json:"med,omitempty"`
	Communities         []string `json:"communities,omitempty"`
	LargeCommunities    []string `json:"large-communities,omitempty"`
	ExtendedCommunities []string `json:"extended-communities,omitempty"`
	Aggregator          string   `json:"aggregator,omitempty"`
	AtomicAggregate     bool     `json:"atomic-aggregate,omitempty"`
	OnlyToCustomer      uint32   `json:"only-to-customer,omitempty"`
	PeerIP              string   `json:"peer-ip"`
	PeerASN             uint32   `json:"peer-asn"`
}

func runRoutes(args []string) int {
	if len(args) == 0 {
		os.Stderr.WriteString(routesUsage) //nolint:errcheck // usage output
		return 1
	}

	var limit int
	inputFile := args[0]
	for i := 1; i < len(args); i++ {
		if args[i] == "--limit" && i+1 < len(args) {
			i++
			v := 0
			for _, c := range args[i] {
				if c < '0' || c > '9' {
					os.Stderr.WriteString(routesUsage) //nolint:errcheck // usage output
					return 1
				}
				v = v*10 + int(c-'0')
			}
			limit = v
		}
	}

	enc := json.NewEncoder(os.Stdout)
	var count int
	var peerIndex *mrt.PeerIndexTable
	// routes emits one JSON object per route to stdout. A route whose AS_PATH
	// failed to decode is emitted with an EMPTY as-path, which a consumer reads
	// as "originated locally" rather than "unreadable", so the count of damaged
	// records has to reach the operator on stderr.
	var damaged malformedCounter
	defer damaged.report(os.Stderr)

	handler := &mrt.Handler{
		OnPeerIndex: func(_ mrt.Header, pit *mrt.PeerIndexTable) error {
			peerIndex = pit
			return nil
		},
		OnRIB: func(h mrt.Header, r *mrt.RIBRecord) error {
			pfx := formatPrefix(h.Subtype, r.PrefixLength, r.Prefix)
			fourByteAS := mrt.ASPathIsFourByte(h.Type, h.Subtype)
			for _, e := range r.Entries {
				if limit > 0 && count >= limit {
					return nil
				}
				count++
				attrs := mrt.ParseAttributes(e.Attributes)
				rec, recErr := buildRouteRecord(pfx, attrs, peerIndex, e.PeerIndex, fourByteAS)
				damaged.note(recErr)
				_ = enc.Encode(rec)
			}
			return nil
		},
	}

	if err := mrt.ReadFile(inputFile, handler); err != nil {
		os.Stderr.WriteString("routes: " + err.Error() + "\n") //nolint:errcheck // error output
		return 1
	}
	return 0
}

// buildRouteRecord assembles one output row. fourByteAS MUST come from the
// enclosing MRT record type via mrt.ASPathIsFourByte (RFC 6396 fixes the AS
// width per record type; it is not inferable from the attribute bytes).
//
// The returned error reports a field that could not be decoded. The record is
// still returned and still emitted -- the prefix and next-hop are usable even
// when the AS_PATH is not -- but the caller MUST count the error, because an
// undecodable AS_PATH leaves as-path empty in the JSON and an empty as-path
// means "locally originated" to every consumer.
func buildRouteRecord(prefix string, attrs []mrt.PathAttribute, pit *mrt.PeerIndexTable, peerIdx uint16, fourByteAS bool) (routeRecord, error) {
	rec := routeRecord{Prefix: prefix}
	var damage error

	// RIB entries carry the abbreviated MP_REACH_NLRI (RFC 6396 Section 4.3.4).
	nh := mrt.ExtractNextHopRIB(attrs)
	if nh.IsValid() {
		rec.NextHop = nh.String()
	}

	if a := mrt.FindAttribute(attrs, mrt.AttrASPath); a != nil {
		segs, err := mrt.ParseASPath(a.Value, fourByteAS)
		if err != nil {
			damage = fmt.Errorf("AS_PATH for %s: %w", prefix, err)
		}
		for _, seg := range segs {
			rec.ASPath = append(rec.ASPath, seg.ASNs...)
		}
	}

	if origin, ok := mrt.ExtractOrigin(attrs); ok {
		switch origin {
		case 0:
			rec.Origin = "igp"
		case 1:
			rec.Origin = "egp"
		default:
			rec.Origin = "incomplete"
		}
	}

	if lp, ok := mrt.ExtractLocalPref(attrs); ok {
		rec.LocalPref = lp
	}
	if med, ok := mrt.ExtractMED(attrs); ok {
		rec.MED = med
	}

	comms := mrt.ExtractCommunities(attrs)
	for _, c := range comms {
		tb := textbuf.Get()
		tb.Reset().Uint32(c >> 16).Byte(':').Uint32(c & 0xffff)
		rec.Communities = append(rec.Communities, tb.String())
		tb.Release()
	}

	// RFC 8092 large communities: "global:local1:local2".
	for _, lc := range mrt.ExtractLargeCommunities(attrs) {
		tb := textbuf.Get()
		tb.Reset().Uint32(lc[0]).Byte(':').Uint32(lc[1]).Byte(':').Uint32(lc[2])
		rec.LargeCommunities = append(rec.LargeCommunities, tb.String())
		tb.Release()
	}

	// RFC 4360 extended communities, rendered as "type:subtype:hex-value".
	for _, ec := range mrt.ExtractExtendedCommunities(attrs) {
		tb := textbuf.Get()
		tb.Reset().Uint8(ec.Type).Byte(':').Uint8(ec.Subtype).Byte(':').Hex(ec.Value[:])
		rec.ExtendedCommunities = append(rec.ExtendedCommunities, tb.String())
		tb.Release()
	}

	if agg, ok := mrt.ExtractAggregator(attrs); ok {
		tb := textbuf.Get()
		tb.Reset().Str("AS").Uint32(agg.ASN).Byte(':').Addr(agg.Address)
		rec.Aggregator = tb.String()
		tb.Release()
	}

	rec.AtomicAggregate = mrt.HasAtomicAggregate(attrs)

	// RFC 9234 Only-to-Customer: the value is the AS that set it. Its presence
	// marks a route that must not be re-advertised to a peer or provider, so a
	// leak shows up as an OTC-carrying route arriving from the wrong direction.
	if otc := mrt.FindAttribute(attrs, mrt.AttrOTC); otc != nil && len(otc.Value) == 4 {
		rec.OnlyToCustomer = binary.BigEndian.Uint32(otc.Value)
	}

	if pit != nil && int(peerIdx) < len(pit.Peers) {
		p := &pit.Peers[peerIdx]
		rec.PeerIP = net.IP(p.IP).String()
		rec.PeerASN = p.ASN
	}

	return rec, damage
}
