// Design: docs/architecture/wire/nlri.md — NLRI encoding and decoding
// RFC: rfc/short/rfc4760.md — AFI/SAFI constants
//
// Package nlri provides NLRI (Network Layer Reachability Information) types.
//
// This file contains error variables for specialized BGP address families.
// SAFI numeric constants live in internal/core/family. The NLRI type
// implementations live in their respective plugin packages
// (bgp-mvpn, bgp-vpls, bgp-rtc, bgp-mup, etc.).
package nlri

import "errors"

// Errors for specialized NLRI parsing.
var (
	ErrMVPNTruncated  = errors.New("mvpn: truncated data")
	ErrVPLSTruncated  = errors.New("vpls: truncated data")
	ErrRTCTruncated   = errors.New("rtc: truncated data")
	ErrMUPTruncated   = errors.New("mup: truncated data")
	ErrMUPInvalidType = errors.New("mup: invalid route type")
)
