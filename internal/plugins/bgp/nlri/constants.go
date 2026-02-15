// Package nlri provides NLRI (Network Layer Reachability Information) types.
//
// This file contains SAFI constants, Family variables, and error variables
// for specialized BGP address families. The NLRI type implementations live
// in their respective plugin packages (bgp-mvpn, bgp-vpls, bgp-rtc, bgp-mup, etc.).
package nlri

import "errors"

// Additional SAFI values for specialized NLRI types.
//
// RFC 4760 Section 3 defines the SAFI (Subsequent Address Family Identifier)
// as a one-octet field that provides additional information about the type
// of NLRI being carried.
//
// SAFI allocations are maintained by IANA in the "Subsequent Address Family
// Identifiers (SAFI) Parameters" registry.
const (
	SAFIMVPN        SAFI = 5   // Multicast VPN - RFC 6514
	SAFIVPLS        SAFI = 65  // VPLS - RFC 4761 Section 3.2.2
	SAFIMUP         SAFI = 85  // Mobile User Plane - draft-mpmz-bess-mup-safi
	SAFIRTC         SAFI = 132 // Route Target Constraint - RFC 4684 Section 4
	SAFIFlowSpecVPN SAFI = 134 // FlowSpec VPN - RFC 8955 (obsoletes RFC 5575)
)

// Common address families for specialized NLRI types.
//
// These combine AFI and SAFI values to identify specific BGP address families:
//   - MVPN uses AFI 1 (IPv4) or 2 (IPv6) with SAFI 5 (RFC 6514)
//   - VPLS uses AFI 25 (L2VPN) with SAFI 65 (RFC 4761)
//   - RTC uses AFI 1 (IPv4) with SAFI 132 (RFC 4684)
//   - MUP uses AFI 1 or 2 with SAFI 85 (draft-mpmz-bess-mup-safi)
//   - FlowSpec VPN uses AFI 1 or 2 with SAFI 134 (RFC 8955)
var (
	IPv4MVPN        = Family{AFI: AFIIPv4, SAFI: SAFIMVPN}
	IPv6MVPN        = Family{AFI: AFIIPv6, SAFI: SAFIMVPN}
	L2VPNVPLS       = Family{AFI: AFIL2VPN, SAFI: SAFIVPLS}
	IPv4RTC         = Family{AFI: AFIIPv4, SAFI: SAFIRTC}
	IPv4MUP         = Family{AFI: AFIIPv4, SAFI: SAFIMUP}
	IPv6MUP         = Family{AFI: AFIIPv6, SAFI: SAFIMUP}
	IPv4FlowSpecVPN = Family{AFI: AFIIPv4, SAFI: SAFIFlowSpecVPN}
	IPv6FlowSpecVPN = Family{AFI: AFIIPv6, SAFI: SAFIFlowSpecVPN}
)

// Errors for specialized NLRI parsing.
var (
	ErrMVPNTruncated  = errors.New("mvpn: truncated data")
	ErrVPLSTruncated  = errors.New("vpls: truncated data")
	ErrRTCTruncated   = errors.New("rtc: truncated data")
	ErrMUPTruncated   = errors.New("mup: truncated data")
	ErrMUPInvalidType = errors.New("mup: invalid route type")
)
