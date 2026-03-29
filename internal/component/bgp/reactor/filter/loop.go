// Design: docs/architecture/core-design.md — route loop detection ingress filter
// RFC: rfc/short/rfc4271.md
// RFC: rfc/short/rfc4456.md
//
// Package filter implements protocol-mandated ingress filters for the BGP reactor.
// Filters are registered with the plugin registry at init() and called by the
// reactor's ingress filter chain for each received UPDATE.
package filter

import (
	"encoding/binary"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

var logger = slogutil.LazyLogger("bgp.filter")

// LoopIngress checks for route loops in received UPDATE messages.
// Returns accept=false if a loop is detected.
//
// Three checks:
//  1. AS loop: local ASN in AS_PATH (all sessions, RFC 4271 Section 9)
//  2. Originator-ID loop: ORIGINATOR_ID == local Router ID (iBGP only, RFC 4456 Section 8)
//  3. Cluster-list loop: local Router ID in CLUSTER_LIST (iBGP only, RFC 4456 Section 8)
func LoopIngress(src registry.PeerFilterInfo, payload []byte, _ map[string]any) (bool, []byte) {
	if len(payload) < 4 {
		return true, nil
	}

	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	offset := 2 + withdrawnLen
	if offset+2 > len(payload) {
		return true, nil
	}

	attrLen := int(binary.BigEndian.Uint16(payload[offset : offset+2]))
	offset += 2
	if offset+attrLen > len(payload) {
		return true, nil
	}

	pathAttrs := payload[offset : offset+attrLen]
	isIBGP := src.LocalAS == src.PeerAS
	localASN := src.LocalAS
	routerID := src.RouterID

	// Walk path attributes once, checking all three loop conditions.
	pos := 0
	for pos < len(pathAttrs) {
		if pos+2 > len(pathAttrs) {
			break
		}

		flags := pathAttrs[pos]
		code := attribute.AttributeCode(pathAttrs[pos+1])
		pos += 2

		var dataLen int
		if flags&0x10 != 0 { // Extended length
			if pos+2 > len(pathAttrs) {
				break
			}
			dataLen = int(binary.BigEndian.Uint16(pathAttrs[pos : pos+2]))
			pos += 2
		} else {
			if pos+1 > len(pathAttrs) {
				break
			}
			dataLen = int(pathAttrs[pos])
			pos++
		}

		if pos+dataLen > len(pathAttrs) {
			break
		}

		data := pathAttrs[pos : pos+dataLen]

		switch code { //nolint:exhaustive // only loop-relevant attributes checked
		case attribute.AttrASPath:
			// RFC 4271 Section 9: "If the local AS appears in the AS_PATH attribute,
			// the route MUST be excluded from the Phase 2 decision function."
			iter := attribute.NewASPathIterator(data, src.ASN4)
			for {
				_, asns, ok := iter.Next()
				if !ok {
					break
				}
				asnIter := attribute.NewASNIterator(asns, src.ASN4)
				for {
					asn, ok := asnIter.Next()
					if !ok {
						break
					}
					if asn == localASN {
						logger().Debug("AS loop detected", "peer", src.Address, "local-asn", localASN)
						return false, nil
					}
				}
			}

		case attribute.AttrOriginatorID:
			// RFC 4456 Section 8: "A router that recognizes the ORIGINATOR_ID attribute
			// SHOULD ignore a route received with its BGP Identifier as the ORIGINATOR_ID."
			if isIBGP && dataLen == 4 {
				originatorID := binary.BigEndian.Uint32(data)
				if originatorID == routerID {
					logger().Debug("ORIGINATOR_ID loop detected", "peer", src.Address, "originator-id", originatorID)
					return false, nil
				}
			}

		case attribute.AttrClusterList:
			// RFC 4456 Section 8: "If the local CLUSTER_ID is found in the CLUSTER_LIST,
			// the advertisement received SHOULD be ignored."
			if isIBGP && dataLen%4 == 0 {
				for i := 0; i < dataLen; i += 4 {
					clusterID := binary.BigEndian.Uint32(data[i:])
					if clusterID == routerID {
						logger().Debug("CLUSTER_LIST loop detected", "peer", src.Address, "cluster-id", clusterID)
						return false, nil
					}
				}
			}
		}

		pos += dataLen
	}

	return true, nil
}
