// Design: docs/architecture/ospf/ospfv3-5-nssa-redist.md -- OSPFv3 NSSA Type-7 redistribution.
// RFC: rfc/short/rfc3101.md (sec 2.3/2.4 P-bit and forwarding address), rfc/short/rfc5340.md (App A.4.8 NSSA-LSA)

package ospf

import (
	"sort"

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
	return e.externalScopeV6For(cfg, running, nil)
}

// externalScopeV6For is the OSPFv3 counterpart of externalScopeFor: it enumerates the NSSA
// areas this router originates NSSA-LSAs into, each with the intra-NSSA forwarding address
// RFC 3101 Section 2.4 requires on a P-set LSA, and reports whether the router may originate
// an AS-wide AS-External-LSA. A nil activeIfaces admits every running interface; a non-nil
// one admits only the named interfaces, so a link with no advertised adjacency contributes
// neither an attachment nor a forwarding address.
//
// The result is deterministic: running is ordered by interface name before the walk, so an
// NSSA reached over several interfaces always takes the first name's address. A later
// interface in the same area upgrades an area whose first interface had no usable address.
func (e *engine) externalScopeV6For(cfg ospfConfig, running []interfaceConfig, activeIfaces map[string]bool) (nssas []nssaAttachmentV6, canType5 bool) {
	sort.Slice(running, func(i, j int) bool { return running[i].Name < running[j].Name })
	attachedNormal := false
	seen := make(map[types.AreaID]int, len(running))
	for _, ic := range running {
		if activeIfaces != nil && !activeIfaces[ic.Name] {
			continue
		}
		switch areaTypeFor(cfg, ic.AreaID) {
		case areaTypeNSSA:
			fa, ok := e.forwardingAddressForAF(ic.Name)
			if idx, dup := seen[ic.AreaID]; dup {
				if !nssas[idx].hasFA && ok {
					nssas[idx].fa, nssas[idx].hasFA = fa, true
				}
				continue
			}
			seen[ic.AreaID] = len(nssas)
			nssas = append(nssas, nssaAttachmentV6{area: ic.AreaID, fa: fa, hasFA: ok})
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

// v6NSSADefaultLSID is the Link State ID of this router's OSPFv3 NSSA default LSA. RFC 5340
// Section 4.4.3.7 strips the Link State ID of every addressing semantic ("The Link State ID
// of an NSSA-LSA has lost all of its addressing semantics and simply serves to distinguish
// multiple NSSA-LSAs that are originated by the same router in the same area"), so the value
// only has to be unique among this router's NSSA-LSAs. Redistribution allocates from
// redistV6Next, which pre-increments before its first use (v6InjectExternal), so 0 is never
// handed out there and the default destination can own it.
var v6NSSADefaultLSID = types.LinkStateID{}

// v6OriginateNSSADefault originates this router's OSPFv3 NSSA default LSA into area.
// RFC 5340 Appendix A.4.1 gives a zero PrefixLength no Address Prefix words, so the default
// destination is the prefix length alone and the encoding is the same for an IPv6 and an
// IPv4 (RFC 5838) address family. It reports whether the area store changed.
func (e *engine) v6OriginateNSSADefault(area types.AreaID, router types.RouterID, metric uint32, fa [16]byte, hasFA, propagate bool) bool {
	return e.v6OriginateNSSALSA(area, router, v6NSSADefaultLSID, ospfv3packet.Prefix{}, false, metric, fa, hasFA, 0, propagate)
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
