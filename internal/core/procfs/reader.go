// Design: plan/spec-diag-core.md -- shared /proc reading infrastructure

package procfs

import (
	"encoding/hex"
	"errors"
	"net/netip"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
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

func ParseHexAddr(hexStr string) string {
	if len(hexStr) == 8 {
		return parseHexIPv4(hexStr)
	}
	if len(hexStr) == 32 {
		return parseHexIPv6(hexStr)
	}
	return hexStr
}

func parseHexIPv4(hexStr string) string {
	b, err := strconv.ParseUint(hexStr, 16, 32)
	if err != nil {
		return hexStr
	}
	addr := netip.AddrFrom4([4]byte{byte(b), byte(b >> 8), byte(b >> 16), byte(b >> 24)})
	return textbuf.Addr(addr)
}

func parseHexIPv6(hexStr string) string {
	if len(hexStr) != 32 {
		return hexStr
	}
	raw, err := hex.DecodeString(hexStr)
	if err != nil || len(raw) != 16 {
		return hexStr
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
	return textbuf.Addr(addr)
}

func ParseHexPort(hexStr string) int {
	v, err := strconv.ParseUint(hexStr, 16, 16)
	if err != nil {
		return 0
	}
	return int(v)
}
