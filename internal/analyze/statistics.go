// Design: docs/architecture/mrt.md — MRT file statistics

package analyze

import (
	"encoding/json"
	"net"
	"os"
	"sort"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/mrt"
)

type mrtStats struct {
	Files         []string          `json:"files"`
	TypeCounts    map[string]uint64 `json:"type-counts"`
	SubtypeCounts map[string]uint64 `json:"subtype-counts"`
	AFICounts     map[string]uint64 `json:"afi-counts"`
	PeerCount     int               `json:"peer-count"`
	Peers         map[string]uint64 `json:"peers"`
	BGPMsgTypes   map[string]uint64 `json:"bgp-message-types"`
	AddPathCount  uint64            `json:"add-path-records"`
	RIBEntries    uint64            `json:"rib-entries"`
	MinTimestamp  uint32            `json:"min-timestamp"`
	MaxTimestamp  uint32            `json:"max-timestamp"`
	TotalRecords  uint64            `json:"total-records"`
}

const statisticsUsage = `ze-analyze statistics -- MRT file statistics

Reads MRT files and reports record counts by type/subtype, AFI breakdown,
peer summary, timestamp range, and BGP message type distribution.

Usage:
  ze-analyze statistics <file.mrt[.gz|.bz2]> [file2...]

Output:
  JSON to stdout, human summary to stderr.
`

func runStatistics(args []string) int {
	if len(args) == 0 {
		os.Stderr.WriteString(statisticsUsage) //nolint:errcheck // usage output
		return 1
	}

	st := &mrtStats{
		Files:         args,
		TypeCounts:    make(map[string]uint64),
		SubtypeCounts: make(map[string]uint64),
		AFICounts:     make(map[string]uint64),
		Peers:         make(map[string]uint64),
		BGPMsgTypes:   make(map[string]uint64),
		MinTimestamp:  ^uint32(0),
	}

	for _, fname := range args {
		if err := mrt.ReadFile(fname, &mrt.Handler{
			OnHeader: func(h mrt.Header, _ uint32, _ []byte) error {
				st.TotalRecords++
				if h.Timestamp < st.MinTimestamp {
					st.MinTimestamp = h.Timestamp
				}
				if h.Timestamp > st.MaxTimestamp {
					st.MaxTimestamp = h.Timestamp
				}
				tn := typeName(h.Type)
				st.TypeCounts[tn]++
				st.SubtypeCounts[tn+"/"+textbuf.StringUint16(h.Subtype)]++
				if mrt.IsAddPathRIBSubtype(h.Subtype) || mrt.IsAddPathBGP4MPSubtype(h.Subtype) {
					st.AddPathCount++
				}
				return nil
			},
			OnPeerIndex: func(_ mrt.Header, pit *mrt.PeerIndexTable) error {
				st.PeerCount = len(pit.Peers)
				for _, p := range pit.Peers {
					key := net.IP(p.IP).String()
					st.Peers[key] = uint64(p.ASN)
				}
				return nil
			},
			OnRIB: func(h mrt.Header, r *mrt.RIBRecord) error {
				afi := mrt.RIBSubtypeAFI(h.Subtype)
				if afi == mrt.AFIIPv4 {
					st.AFICounts["ipv4"] += uint64(len(r.Entries))
				} else {
					st.AFICounts["ipv6"] += uint64(len(r.Entries))
				}
				st.RIBEntries += uint64(len(r.Entries))
				return nil
			},
			OnRIBGeneric: func(_ mrt.Header, r *mrt.RIBGenericRecord) error {
				var tb textbuf.Buffer
				key := tb.Str("afi-").Uint16(r.AFI).Str("-safi-").Uint(uint64(r.SAFI)).String()
				st.AFICounts[key] += uint64(len(r.Entries))
				st.RIBEntries += uint64(len(r.Entries))
				return nil
			},
			OnMessage: func(_ mrt.Header, _ uint32, m *mrt.MessageRecord) error {
				peerKey := net.IP(m.PeerIP).String()
				st.Peers[peerKey] = uint64(m.PeerAS)
				if len(m.BGPMessage) >= 19 {
					bgpType := m.BGPMessage[18]
					st.BGPMsgTypes[bgpMsgTypeName(bgpType)]++
				}
				return nil
			},
			OnStateChange: func(_ mrt.Header, _ uint32, _ *mrt.StateChangeRecord) error {
				st.BGPMsgTypes["state-change"]++
				return nil
			},
			OnTableDump: func(_ mrt.Header, _ *mrt.TableDumpRecord) error {
				st.RIBEntries++
				return nil
			},
		}); err != nil {
			wf(os.Stderr, "error processing %s: %v\n", fname, err)
			return 1
		}
	}

	if st.TotalRecords == 0 {
		st.MinTimestamp = 0
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(st); err != nil {
		wf(os.Stderr, "json encode: %v\n", err)
		return 1
	}

	printStatsSummary(st)
	return 0
}

func printStatsSummary(st *mrtStats) {
	w := os.Stderr
	wf(w, "\nMRT Statistics Summary\n")
	wf(w, "=====================\n\n")
	wf(w, "Total records: %s\n", formatNumber(st.TotalRecords))
	wf(w, "RIB entries:   %s\n", formatNumber(st.RIBEntries))
	wf(w, "Peers:         %d\n", st.PeerCount)
	wf(w, "Add-path:      %s\n", formatNumber(st.AddPathCount))
	wf(w, "Timestamp range: %d - %d\n\n", st.MinTimestamp, st.MaxTimestamp)

	wf(w, "Records by type:\n")
	for _, k := range sortedKeys(st.TypeCounts) {
		wf(w, "  %-20s %s\n", k, formatNumber(st.TypeCounts[k]))
	}

	wf(w, "\nAFI breakdown:\n")
	for _, k := range sortedKeys(st.AFICounts) {
		wf(w, "  %-20s %s\n", k, formatNumber(st.AFICounts[k]))
	}

	if len(st.BGPMsgTypes) > 0 {
		wf(w, "\nBGP message types:\n")
		for _, k := range sortedKeys(st.BGPMsgTypes) {
			wf(w, "  %-20s %s\n", k, formatNumber(st.BGPMsgTypes[k]))
		}
	}
}

func sortedKeys(m map[string]uint64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func typeName(t uint16) string {
	switch t {
	case mrt.TypeTableDump:
		return "table-dump"
	case mrt.TypeTableDumpV2:
		return "table-dump-v2"
	case mrt.TypeBGP4MP:
		return "bgp4mp"
	case mrt.TypeBGP4MPET:
		return "bgp4mp-et"
	case mrt.TypeISIS:
		return "isis"
	case mrt.TypeOSPFv2:
		return "ospfv2"
	case mrt.TypeOSPFv3:
		return "ospfv3"
	}
	var tb textbuf.Buffer
	return tb.Str("type-").Uint16(t).String()
}

func bgpMsgTypeName(t uint8) string {
	switch t {
	case 1:
		return "open"
	case 2:
		return "update"
	case 3:
		return "notification"
	case 4:
		return "keepalive"
	case 5:
		return "route-refresh"
	}
	var tb textbuf.Buffer
	return tb.Str("type-").Uint(uint64(t)).String()
}
