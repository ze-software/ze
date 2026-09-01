// Design: docs/architecture/mrt.md -- human-readable MRT dump (like bgpdump)
// RFC: rfc/short/rfc6396.md -- per-record-type AS width, RIB-entry MP_REACH truncation
// Related: routes.go -- prefix table extraction from the same records

package analyze

import (
	"net"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/mrt"
)

const showUsage = `ze-analyze show -- human-readable MRT record dump

Reads MRT records and prints a human-readable summary of each record,
similar to bgpdump. BGP messages are parsed to show UPDATE contents,
OPEN parameters, and NOTIFICATION details.

Usage:
  ze-analyze show <file.mrt[.gz|.bz2]> [--limit <n>]
`

func runShow(args []string) int {
	if len(args) == 0 {
		os.Stderr.WriteString(showUsage) //nolint:errcheck // usage output
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
					os.Stderr.WriteString(showUsage) //nolint:errcheck // usage output
					return 1
				}
				v = v*10 + int(c-'0')
			}
			limit = v
		}
	}

	var count int
	var peerIndex *mrt.PeerIndexTable
	// show renders one line per record, so a damaged record is easy to scroll
	// past. The tally at the end is what tells the operator the dump they just
	// read was not the whole file.
	var damaged malformedCounter
	defer damaged.report(os.Stderr)

	handler := &mrt.Handler{
		OnPeerIndex: func(_ mrt.Header, pit *mrt.PeerIndexTable) error {
			peerIndex = pit
			var tb textbuf.Buffer
			tb.Str("PEER_INDEX_TABLE: ").Uint(uint64(len(pit.Peers))).Str(" peers\n").StdOut() //nolint:errcheck // output
			for i, p := range pit.Peers {
				tb.Reset().Str("  [").Uint(uint64(i)).Str("] ").Str(net.IP(p.IP).String()).
					Str(" AS").Uint32(p.ASN).Byte('\n').StdOut() //nolint:errcheck // output
			}
			return nil
		},
		OnRIB: func(h mrt.Header, r *mrt.RIBRecord) error {
			if limit > 0 && count >= limit {
				return nil
			}
			count++
			pfx := formatPrefix(h.Subtype, r.PrefixLength, r.Prefix)
			os.Stdout.WriteString("RIB " + pfx + " (" + textbuf.StringUint(uint64(len(r.Entries))) + " entries)\n") //nolint:errcheck // output
			for _, e := range r.Entries {
				peerName := peerLabel(peerIndex, e.PeerIndex)
				attrs := mrt.ParseAttributes(e.Attributes)
				// RIB entries carry the abbreviated MP_REACH_NLRI
				// (RFC 6396 Section 4.3.4), not the full form.
				nh := mrt.ExtractNextHopRIB(attrs)
				nhStr := "-"
				if nh.IsValid() {
					nhStr = nh.String()
				}
				aspathStr := formatASPathFromAttrs(attrs, mrt.ASPathIsFourByte(h.Type, h.Subtype))
				os.Stdout.WriteString("  peer=" + peerName + " nh=" + nhStr + " path=" + aspathStr + "\n") //nolint:errcheck // output
			}
			return nil
		},
		OnMessage: func(h mrt.Header, usec uint32, m *mrt.MessageRecord) error {
			if limit > 0 && count >= limit {
				return nil
			}
			count++
			ts := time.Unix(int64(h.Timestamp), int64(usec)*1000)
			peer := net.IP(m.PeerIP).String()
			// ParseBGPMessage salvages: a damaged UPDATE still returns what
			// decoded, alongside the error. Render that rather than collapsing
			// the whole record to "[parse error]", which threw away every
			// readable field and told the operator nothing about WHAT was wrong.
			parsed, parseErr := mrt.ParseBGPMessage(m.BGPMessage)
			if parseErr != nil {
				damaged.note(parseErr)
			}
			if parsed == nil {
				var tb textbuf.Buffer
				tb.Str(ts.UTC().Format("15:04:05")).Byte(' ').Str(peer).
					Str(" [unparseable: ").Str(damageTag(parseErr)).Str("] ").Err(parseErr).Byte('\n')
				tb.StdOut() //nolint:errcheck // output
				return nil  //nolint:nilerr // skip unparseable records, continue iteration
			}
			showParsedMessage(ts, peer, m.PeerAS, parsed, mrt.ASPathIsFourByte(h.Type, h.Subtype))
			return nil
		},
		OnStateChange: func(h mrt.Header, usec uint32, s *mrt.StateChangeRecord) error {
			if limit > 0 && count >= limit {
				return nil
			}
			count++
			ts := time.Unix(int64(h.Timestamp), int64(usec)*1000)
			peer := net.IP(s.PeerIP).String()
			os.Stdout.WriteString(ts.UTC().Format("15:04:05") + " " + peer + " STATE " + fsmName(s.OldState) + " -> " + fsmName(s.NewState) + "\n") //nolint:errcheck // output
			return nil
		},
	}

	if err := mrt.ReadFile(inputFile, handler); err != nil {
		var tb textbuf.Buffer
		tb.Str("show: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // error output
		return 1
	}
	return 0
}

func showParsedMessage(ts time.Time, peer string, peerAS uint32, parsed *mrt.ParsedMessage, fourByteAS bool) {
	var tb textbuf.Buffer
	tb.Str(ts.UTC().Format("15:04:05")).Byte(' ').Str(peer).Str(" AS").Uint32(peerAS).Byte(' ')
	prefix := tb.String()
	switch parsed.Type {
	case 1:
		o := parsed.Open
		tb.Reset().Str(prefix).Str("OPEN v").Uint8(o.Version).Str(" AS").Uint32(o.ASN).Str(" hold=").Uint16(o.HoldTime).Byte('\n')
		tb.StdOut() //nolint:errcheck // output
	case 2:
		u := parsed.Update
		var lb textbuf.Buffer
		// The UPDATE's own withdrawn/NLRI fields are IPv4 only; every other
		// family travels in MP_UNREACH/MP_REACH, so both must be counted or an
		// IPv6 UPDATE renders with no prefix counts at all.
		mpW, wOK := mpUnreachCount(u.Attributes)
		mpA, aOK := mpReachCount(u.Attributes)
		withdrawn := len(u.WithdrawnPrefixes) + mpW
		announced := len(u.AnnouncedPrefixes) + mpA
		// A partial count is printed with a trailing '+' so it can never be read
		// as an exact one: "A=3+" says at least 3 and the rest is unreadable.
		// Printed even at zero when damaged -- "no A= field" is precisely the
		// ambiguity this exists to remove.
		if withdrawn > 0 || !wOK {
			lb.Str("W=").Uint(uint64(withdrawn))
			if !wOK {
				lb.Byte('+')
			}
		}
		if announced > 0 || !aOK {
			if lb.Len() > 0 {
				lb.Byte(' ')
			}
			lb.Str("A=").Uint(uint64(announced))
			if !aOK {
				lb.Byte('+')
			}
		}
		aspathStr := formatASPathFromAttrs(u.Attributes, fourByteAS)
		if aspathStr != "" {
			lb.Str(" path=").Str(aspathStr)
		}
		nh := mrt.ExtractNextHop(u.Attributes)
		if nh.IsValid() {
			lb.Str(" nh=").Addr(nh)
		}
		tb.Reset().Str(prefix).Str("UPDATE ").Str(lb.String()).Byte('\n')
		tb.StdOut() //nolint:errcheck // output
	case 3:
		n := parsed.Notification
		tb.Reset().Str(prefix).Str("NOTIFICATION code=").Uint8(n.Code).Byte('/').Uint8(n.Subcode).Byte('\n')
		tb.StdOut() //nolint:errcheck // output
	case 4:
		tb.Reset().Str(prefix).Str("KEEPALIVE\n")
		tb.StdOut() //nolint:errcheck // output
	}
}

// mpReachCount returns the number of prefixes announced via MP_REACH_NLRI
// (RFC 4760 Section 3) and whether that count is COMPLETE.
//
// A malformed attribute still contributes the prefixes that decoded before the
// damage rather than aborting the record: show renders one line per record and
// a damaged attribute must not hide the rest of it. But it MUST NOT contribute
// them as if they were the whole story -- returning a bare 0, as this did,
// printed a cut IPv6 MP_REACH carrying 40 prefixes with no A= field at all,
// indistinguishable from an UPDATE that announced nothing
// (ai/rules/evidence.md: a guard that neither denies nor speaks does
// not exist). ok=false is what the caller renders the damage marker from.
func mpReachCount(attrs []mrt.PathAttribute) (count int, ok bool) {
	a := mrt.FindAttribute(attrs, mrt.AttrMPReachNLRI)
	if a == nil {
		return 0, true
	}
	mp, err := mrt.ParseMPReach(a.Value)
	if mp != nil {
		count = len(mp.Prefixes)
	}
	return count, err == nil
}

// mpUnreachCount returns the number of prefixes withdrawn via MP_UNREACH_NLRI
// (RFC 4760 Section 4) and whether that count is complete. Same contract as
// mpReachCount.
func mpUnreachCount(attrs []mrt.PathAttribute) (count int, ok bool) {
	a := mrt.FindAttribute(attrs, mrt.AttrMPUnreachNLRI)
	if a == nil {
		return 0, true
	}
	mp, err := mrt.ParseMPUnreach(a.Value)
	if mp != nil {
		count = len(mp.Prefixes)
	}
	return count, err == nil
}

// formatASPathFromAttrs renders AS_PATH for display. fourByte MUST come from
// the enclosing MRT record type via mrt.ASPathIsFourByte: RFC 6396 fixes the AS
// width per record type and it cannot be inferred from the attribute bytes.
func formatASPathFromAttrs(attrs []mrt.PathAttribute, fourByte bool) string {
	a := mrt.FindAttribute(attrs, mrt.AttrASPath)
	if a == nil {
		return ""
	}
	segs, err := mrt.ParseASPath(a.Value, fourByte)
	if err != nil {
		return "?"
	}
	var result []byte
	for _, seg := range segs {
		if seg.Type == 1 {
			result = append(result, '{')
		}
		for i, asn := range seg.ASNs {
			if i > 0 || (seg.Type != 1 && len(result) > 0) {
				result = append(result, ' ')
			}
			result = appendUint32Str(result, asn)
		}
		if seg.Type == 1 {
			result = append(result, '}')
		}
	}
	return string(result)
}

func appendUint32Str(buf []byte, n uint32) []byte {
	if n == 0 {
		return append(buf, '0')
	}
	var digits [10]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return append(buf, digits[i:]...)
}

func formatPrefix(subtype uint16, prefixLen uint8, prefix []byte) string {
	var tb textbuf.Buffer
	afi := mrt.RIBSubtypeAFI(subtype)
	switch afi {
	case mrt.AFIIPv4:
		var ip [4]byte
		copy(ip[:], prefix)
		addr := net.IP(ip[:]).String()
		return tb.Str(addr).Byte('/').Uint8(prefixLen).String()
	case mrt.AFIIPv6:
		var ip [16]byte
		copy(ip[:], prefix)
		addr := net.IP(ip[:]).String()
		return tb.Str(addr).Byte('/').Uint8(prefixLen).String()
	}
	return tb.Uint8(prefixLen).Byte('/').Uint(uint64(len(prefix))).Str("bytes").String()
}

func peerLabel(pit *mrt.PeerIndexTable, idx uint16) string {
	if pit == nil || int(idx) >= len(pit.Peers) {
		var tb textbuf.Buffer
		return tb.Byte('[').Uint16(idx).Byte(']').String()
	}
	p := &pit.Peers[idx]
	return net.IP(p.IP).String()
}

func fsmName(state uint16) string {
	switch state {
	case 1:
		return "Idle"
	case 2:
		return "Connect"
	case 3:
		return "Active"
	case 4:
		return "OpenSent"
	case 5:
		return "OpenConfirm"
	case 6:
		return "Established"
	}
	return textbuf.StringUint16(state)
}
