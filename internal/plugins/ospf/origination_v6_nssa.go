// Design: plan/learned/975-ospfv3-5-nssa-redist.md -- OSPFv3 NSSA Type-7 redistribution.
// RFC: rfc/short/rfc3101.md (sec 2.3/2.4 P-bit and forwarding address), rfc/short/rfc5340.md (App A.4.8 NSSA-LSA)

package ospf

import (
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

type nssaAttachmentV6 struct {
	area  types.AreaID
	fa    [16]byte
	hasFA bool
}

// forwardingAddressForAF returns interface name's NSSA/AS-External forwarding address at this
// engine's address family (RFC 5838 §2.7): an IPv4 AF carries the interface's IPv4 address in
// the leading 4 octets of the 128-bit field (symmetric with v6ExternalReader's read), while an
// IPv6 AF carries a global IPv6 forwarding address. ok is false when no usable address exists.
func (e *engine) forwardingAddressForAF(name string) ([16]byte, bool) {
	if e.af.isIPv4() {
		v4 := interfaceIPv4Address(name)
		if v4 == ([4]byte{}) {
			return [16]byte{}, false
		}
		var fa [16]byte
		copy(fa[:4], v4[:])
		return fa, true
	}
	return interfaceIPv6ForwardingAddress(name)
}

func (e *engine) externalScopeV6() (nssas []nssaAttachmentV6, canType5 bool) {
	e.mu.Lock()
	cfg := e.cfg
	running := make([]interfaceConfig, 0, len(e.running))
	for _, ic := range e.running {
		running = append(running, ic)
	}
	e.mu.Unlock()
	attachedNormal := false
	seen := make(map[types.AreaID]bool, len(running))
	for _, ic := range running {
		switch areaTypeFor(cfg, ic.AreaID) {
		case areaTypeNSSA:
			if !seen[ic.AreaID] {
				seen[ic.AreaID] = true
				fa, ok := e.forwardingAddressForAF(ic.Name)
				nssas = append(nssas, nssaAttachmentV6{area: ic.AreaID, fa: fa, hasFA: ok})
			}
		case areaTypeStub:
			// stub areas carry no externals
		default:
			attachedNormal = true
		}
	}
	return nssas, attachedNormal || len(nssas) == 0
}

func v6NSSAKey(router types.RouterID, lsid types.LinkStateID) types.LSAKey {
	return types.LSAKey{Type: types.LSType(ospfv3types.LSTypeNSSA), LinkStateID: lsid, AdvertisingRouter: router}
}

func (e *engine) v6OriginateNSSALSA(area types.AreaID, router types.RouterID, lsid types.LinkStateID, prefix ospfv3packet.Prefix, type2 bool, metric uint32, fa [16]byte, hasFA bool, tag uint32, propagate bool) bool {
	if router == (types.RouterID{}) || area == types.BackboneArea {
		return false
	}
	if fa == ([16]byte{}) {
		hasFA = false
	}
	// RFC 3101 §2.3/§2.4, mapped to OSPFv3 by RFC 5340 App A.4.8: enforce the
	// Type-7 P-bit at the origination boundary. The P-bit lives in PrefixOptions
	// for OSPFv3, requires a non-zero Forwarding Address, and is cleared when a
	// local Type-5 twin already advertises this LSID into the AS-wide store.
	if propagate {
		if !hasFA {
			propagate = false
		} else if lsa, ok := e.lsdb.LookupLSA(types.BackboneArea, v6ExternalKey(router, lsid)); ok && !lsa.Header.Age.IsMaxAge() {
			propagate = false
		}
	}
	bodyPrefix := prefix
	if propagate {
		bodyPrefix.Options |= ospfv3types.OptPrefixP
	} else {
		bodyPrefix.Options &^= ospfv3types.OptPrefixP
	}
	body := ospfv3packet.ExternalLSA{
		ExternalType2:     type2,
		Metric:            metric & packet.ExternalMetricMax,
		Prefix:            bodyPrefix,
		ForwardingAddr:    fa,
		HasForwardingAddr: hasFA,
		ExternalRouteTag:  tag,
		HasRouteTag:       tag != 0,
	}
	bodyBytes := make([]byte, body.EncodedLen())
	body.WriteTo(bodyBytes, 0)
	key := v6NSSAKey(router, lsid)
	id := lsid
	b := body
	_, ok := e.lsdb.OriginateSelf(area, key, bodyBytes, func(seq types.LSSequenceNumber, purge bool) packet.LSA {
		return v6SelfLSA(ospfv3packet.LSA{
			Header:   v6OriginHeader(ospfv3types.LSTypeNSSA, ospfv3types.LinkStateID(id), router, seq, purge),
			External: &b,
		})
	})
	return ok
}
