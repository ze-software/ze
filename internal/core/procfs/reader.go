// Design: plan/learned/727-diag-core.md -- shared /proc reading infrastructure
// Related: reader_other.go -- non-Linux stub

package procfs

import (
	"encoding/hex"
	"errors"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var ErrUnsupported = errors.New("procfs: not available on this platform")

const (
	TCPEstablished = 1
	TCPSynSent     = 2
	TCPSynRecv     = 3
	TCPFinWait1    = 4
	TCPFinWait2    = 5
	TCPTimeWait    = 6
	TCPClose       = 7
	TCPCloseWait   = 8
	TCPLastAck     = 9
	TCPListen      = 10
	TCPClosing     = 11
)

var tcpStateNames = map[int]string{
	TCPEstablished: "ESTABLISHED",
	TCPSynSent:     "SYN_SENT",
	TCPSynRecv:     "SYN_RECV",
	TCPFinWait1:    "FIN_WAIT1",
	TCPFinWait2:    "FIN_WAIT2",
	TCPTimeWait:    "TIME_WAIT",
	TCPClose:       "CLOSE",
	TCPCloseWait:   "CLOSE_WAIT",
	TCPLastAck:     "LAST_ACK",
	TCPListen:      "LISTEN",
	TCPClosing:     "CLOSING",
}

func TCPStateString(state int) string {
	if s, ok := tcpStateNames[state]; ok {
		return s
	}
	return textbuf.StrIntStr("UNKNOWN(", int64(state), ")")
}

func ParseHexAddr(encoded string) string {
	if len(encoded) == 8 {
		return parseHexIPv4(encoded)
	}
	if len(encoded) == 32 {
		return parseHexIPv6(encoded)
	}
	return encoded
}

func parseHexIPv4(encoded string) string {
	b, err := strconv.ParseUint(encoded, 16, 32)
	if err != nil {
		return encoded
	}
	addr := netip.AddrFrom4([4]byte{byte(b), byte(b >> 8), byte(b >> 16), byte(b >> 24)})
	return textbuf.StringAddr(addr)
}

func parseHexIPv6(encoded string) string {
	if len(encoded) != 32 {
		return encoded
	}
	raw, err := hex.DecodeString(encoded)
	if err != nil || len(raw) != 16 {
		return encoded
	}
	// /proc/net/tcp6 stores each 32-bit word in host byte order (little-endian on x86/arm).
	var addr16 [16]byte
	for i := range 4 {
		addr16[i*4+0] = raw[i*4+3]
		addr16[i*4+1] = raw[i*4+2]
		addr16[i*4+2] = raw[i*4+1]
		addr16[i*4+3] = raw[i*4+0]
	}
	addr := netip.AddrFrom16(addr16)
	return textbuf.StringAddr(addr)
}

func ParseHexPort(encoded string) int {
	v, err := strconv.ParseUint(encoded, 16, 16)
	if err != nil {
		return 0
	}
	return int(v)
}
