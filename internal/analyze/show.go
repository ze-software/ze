// Design: docs/architecture/mrt.md -- human-readable MRT dump (like bgpdump)

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

	handler := &mrt.Handler{
		OnPeerIndex: func(_ mrt.Header, pit *mrt.PeerIndexTable) error {
			peerIndex = pit
			os.Stdout.WriteString("PEER_INDEX_TABLE: " + textbuf.StringUint(uint64(len(pit.Peers))) + " peers\n") //nolint:errcheck // output
			for i, p := range pit.Peers {
				os.Stdout.WriteString("  [" + textbuf.StringUint(uint64(i)) + "] " + net.IP(p.IP).String() + " AS" + textbuf.StringUint32(p.ASN) + "\n") //nolint:errcheck // output
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
				nh := mrt.ExtractNextHop(attrs)
				nhStr := "-"
				if nh.IsValid() {
					nhStr = nh.String()
				}
				aspathStr := formatASPathFromAttrs(attrs)
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
			parsed, parseErr := mrt.ParseBGPMessage(m.BGPMessage)
			if parseErr != nil {
				os.Stdout.WriteString(ts.UTC().Format("15:04:05") + " " + peer + " [parse error]\n") //nolint:errcheck // output
				return nil                                                                           //nolint:nilerr // skip unparseable records, continue iteration
			}
			showParsedMessage(ts, peer, m.PeerAS, parsed)
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
		os.Stderr.WriteString(tb.Str("show: ").Err(err).Byte('\n').Slice()) //nolint:errcheck // error output
		return 1
	}
	return 0
}

func showParsedMessage(ts time.Time, peer string, peerAS uint32, parsed *mrt.ParsedMessage) {
	var tb textbuf.Buffer
	tb.Str(ts.UTC().Format("15:04:05")).Byte(' ').Str(peer).Str(" AS").Uint32(peerAS).Byte(' ')
	prefix := tb.String()
	switch parsed.Type {
	case 1:
		o := parsed.Open
		tb.Reset().Str(prefix).Str("OPEN v").Uint8(o.Version).Str(" AS").Uint32(o.ASN).Str(" hold=").Uint16(o.HoldTime).Byte('\n')
		os.Stdout.WriteString(tb.Slice()) //nolint:errcheck // output
	case 2:
		u := parsed.Update
		var lb textbuf.Buffer
		if len(u.WithdrawnPrefixes) > 0 {
			lb.Str("W=").Uint(uint64(len(u.WithdrawnPrefixes)))
		}
		if len(u.AnnouncedPrefixes) > 0 {
			if lb.Len() > 0 {
				lb.Byte(' ')
			}
			lb.Str("A=").Uint(uint64(len(u.AnnouncedPrefixes)))
		}
		aspathStr := formatASPathFromAttrs(u.Attributes)
		if aspathStr != "" {
			lb.Str(" path=").Str(aspathStr)
		}
		nh := mrt.ExtractNextHop(u.Attributes)
		if nh.IsValid() {
			lb.Str(" nh=").Addr(nh)
		}
		tb.Reset().Str(prefix).Str("UPDATE ").Str(lb.String()).Byte('\n')
		os.Stdout.WriteString(tb.Slice()) //nolint:errcheck // output
	case 3:
		n := parsed.Notification
		tb.Reset().Str(prefix).Str("NOTIFICATION code=").Uint8(n.Code).Byte('/').Uint8(n.Subcode).Byte('\n')
		os.Stdout.WriteString(tb.Slice()) //nolint:errcheck // output
	case 4:
		tb.Reset().Str(prefix).Str("KEEPALIVE\n")
		os.Stdout.WriteString(tb.Slice()) //nolint:errcheck // output
	}
}

func formatASPathFromAttrs(attrs []mrt.PathAttribute) string {
	a := mrt.FindAttribute(attrs, 2)
	if a == nil {
		return ""
	}
	segs, err := mrt.ParseASPath(a.Value, true)
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
