// Design: docs/guide/l2tp.md -- MRU floors for enabled network protocols.
// Related: manager.go, ncp.go -- admission and negotiated IPv6 service.

package ppp

import "fmt"

const (
	MinIPv4MRU uint16 = 68
	MinIPv6MRU uint16 = 1280
)

// ValidateNetworkMRU checks the requested Information-field limit after the
// caller has resolved its default MRU and enabled network protocols.
func ValidateNetworkMRU(mru uint16, ipv6Enabled bool) error {
	// RFC 791 Section 3.2: "Every internet module must be able to forward a datagram of 68
	// octets without further fragmentation."
	if mru < MinIPv4MRU || mru > MaxFrameLen {
		return fmt.Errorf("PPP MRU %d is outside the supported range %d..%d", mru, MinIPv4MRU, MaxFrameLen)
	}
	// RFC 5072 Section 2: "PPP links supporting IPv6 MUST allow the
	// information field to be at least as large as the minimum link MTU
	// size required for IPv6 [2]."
	if ipv6Enabled && mru < MinIPv6MRU {
		return fmt.Errorf("PPP MRU %d is below the IPv6 minimum %d", mru, MinIPv6MRU)
	}
	return nil
}

// ipv6MRUSupported checks both directions before any IPv6 address request.
// Negotiation can reduce a peer's receive MRU even when admission accepted a
// sufficiently large local cap. The caller MUST keep IPv6 disabled then.
//
// RFC 5072 Section 2: "PPP links supporting IPv6 MUST allow the
// information field to be at least as large as the minimum link MTU
// size required for IPv6 [2]."
func (s *pppSession) ipv6MRUSupported() bool {
	peerMRU := s.negotiatedMRU
	if peerMRU == 0 {
		peerMRU = MaxFrameLen
	}
	localMRU := s.localMRU
	if localMRU == 0 || s.lcpRejectedMRU {
		localMRU = MaxFrameLen
	}
	cap := s.maxMRU
	if cap == 0 {
		cap = MaxFrameLen
	}
	return min(peerMRU, localMRU, cap) >= MinIPv6MRU
}
