// Design: docs/architecture/mrt.md -- prefix table extraction from MRT

package analyze

import (
	"encoding/json"
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
	Prefix      string   `json:"prefix"`
	NextHop     string   `json:"next-hop"`
	ASPath      []uint32 `json:"as-path"`
	Origin      string   `json:"origin"`
	LocalPref   uint32   `json:"local-pref,omitempty"`
	MED         uint32   `json:"med,omitempty"`
	Communities []string `json:"communities,omitempty"`
	PeerIP      string   `json:"peer-ip"`
	PeerASN     uint32   `json:"peer-asn"`
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

	handler := &mrt.Handler{
		OnPeerIndex: func(_ mrt.Header, pit *mrt.PeerIndexTable) error {
			peerIndex = pit
			return nil
		},
		OnRIB: func(h mrt.Header, r *mrt.RIBRecord) error {
			pfx := formatPrefix(h.Subtype, r.PrefixLength, r.Prefix)
			for _, e := range r.Entries {
				if limit > 0 && count >= limit {
					return nil
				}
				count++
				attrs := mrt.ParseAttributes(e.Attributes)
				rec := buildRouteRecord(pfx, attrs, peerIndex, e.PeerIndex)
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

func buildRouteRecord(prefix string, attrs []mrt.PathAttribute, pit *mrt.PeerIndexTable, peerIdx uint16) routeRecord {
	rec := routeRecord{Prefix: prefix}

	nh := mrt.ExtractNextHop(attrs)
	if nh.IsValid() {
		rec.NextHop = nh.String()
	}

	if a := mrt.FindAttribute(attrs, 2); a != nil {
		segs, err := mrt.ParseASPath(a.Value, true)
		if err == nil {
			for _, seg := range segs {
				rec.ASPath = append(rec.ASPath, seg.ASNs...)
			}
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

	if pit != nil && int(peerIdx) < len(pit.Peers) {
		p := &pit.Peers[peerIdx]
		rec.PeerIP = net.IP(p.IP).String()
		rec.PeerASN = p.ASN
	}

	return rec
}
