// Design: plan/learned/956-ospf-2-wire.md -- offline decode JSON rendering
// RFC: rfc/short/rfc7684.md -- Extended Prefix (Opaque Type 7) / Extended Link (Opaque Type 8)
// body decode into the opaque JSON view (spec-ospf-ext-4).

package packet

import (
	"encoding/hex"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// JSONView is a JSON-serializable rendering of a decoded OSPFv2 packet. It is a
// cold CLI/diagnostic path; runtime code consumes typed views directly.
type JSONView struct {
	Type          string `json:"type"`
	RouterID      string `json:"router-id"`
	AreaID        string `json:"area-id"`
	Checksum      uint16 `json:"checksum"`
	ChecksumValid bool   `json:"checksum-valid"`
	// InstanceID is the RFC 6549 OSPFv2 Instance ID (header offset 14), rendered as a
	// field distinct from AuType (offset 15) so a decoded multi-instance packet shows both.
	InstanceID uint8  `json:"instance-id"`
	AuType     uint16 `json:"auth-type"`
	Auth       string `json:"auth"`

	Hello    *helloJSON    `json:"hello,omitempty"`
	DBDesc   *dbDescJSON   `json:"dbdesc,omitempty"`
	LSReq    *lsReqJSON    `json:"ls-request,omitempty"`
	LSUpdate *lsUpdateJSON `json:"ls-update,omitempty"`
	LSAck    *lsAckJSON    `json:"ls-ack,omitempty"`
}

type helloJSON struct {
	NetworkMask   string   `json:"network-mask"`
	HelloInterval uint16   `json:"hello-interval"`
	Options       string   `json:"options"`
	Priority      uint8    `json:"priority"`
	DeadInterval  uint32   `json:"dead-interval"`
	DR            string   `json:"dr"`
	BDR           string   `json:"bdr"`
	Neighbors     []string `json:"neighbors"`
}

type dbDescJSON struct {
	InterfaceMTU uint16          `json:"interface-mtu"`
	Options      string          `json:"options"`
	Flags        uint8           `json:"flags"`
	DDSequence   uint32          `json:"dd-sequence"`
	Headers      []lsaHeaderJSON `json:"headers"`
}

type lsReqJSON struct {
	Requests []lsRequestJSON `json:"requests"`
}

type lsRequestJSON struct {
	Type              string `json:"type"`
	LinkStateID       string `json:"link-state-id"`
	AdvertisingRouter string `json:"advertising-router"`
}

type lsUpdateJSON struct {
	LSAs []lsaJSON `json:"lsas"`
}

type lsAckJSON struct {
	Headers []lsaHeaderJSON `json:"headers"`
}

type lsaHeaderJSON struct {
	Age               uint16 `json:"age"`
	Options           string `json:"options"`
	Type              string `json:"type"`
	LinkStateID       string `json:"link-state-id"`
	AdvertisingRouter string `json:"advertising-router"`
	Sequence          uint32 `json:"sequence"`
	Checksum          uint16 `json:"checksum"`
	Length            uint16 `json:"length"`
}

type lsaJSON struct {
	Header        lsaHeaderJSON    `json:"header"`
	ChecksumValid bool             `json:"checksum-valid"`
	Router        *routerLSAJSON   `json:"router,omitempty"`
	Network       *networkLSAJSON  `json:"network,omitempty"`
	Summary       *summaryLSAJSON  `json:"summary,omitempty"`
	External      *externalLSAJSON `json:"external,omitempty"`
	Opaque        *opaqueLSAJSON   `json:"opaque,omitempty"`
}

type routerLSAJSON struct {
	Flags uint8            `json:"flags"`
	Links []routerLinkJSON `json:"links"`
}

type routerLinkJSON struct {
	LinkID   string `json:"link-id"`
	LinkData string `json:"link-data"`
	Type     uint8  `json:"type"`
	Metric   uint16 `json:"metric"`
}

type networkLSAJSON struct {
	NetworkMask     string   `json:"network-mask"`
	AttachedRouters []string `json:"attached-routers"`
}

type summaryLSAJSON struct {
	NetworkMask string `json:"network-mask"`
	TOS         uint8  `json:"tos"`
	Metric      uint32 `json:"metric"`
}

type externalLSAJSON struct {
	NetworkMask      string `json:"network-mask"`
	ExternalType2    bool   `json:"external-type-2"`
	Metric           uint32 `json:"metric"`
	ForwardingAddr   string `json:"forwarding-address"`
	ExternalRouteTag uint32 `json:"external-route-tag"`
}

type opaqueLSAJSON struct {
	Data string `json:"data"`
	// ExtendedPrefix / ExtendedLink decode the RFC 7684 Opaque Type 7 / 8 bodies inline
	// (spec-ospf-ext-4) so `ze` decode shows structured fields, not just hex.
	ExtendedPrefix []extPrefixJSON `json:"extended-prefix,omitempty"`
	ExtendedLink   *extLinkJSON    `json:"extended-link,omitempty"`
}

type extSubTLVJSON struct {
	Type   uint16 `json:"type"`
	Length int    `json:"length"`
	Data   string `json:"data"`
}

type extPrefixJSON struct {
	RouteType     uint8           `json:"route-type"`
	PrefixLength  uint8           `json:"prefix-length"`
	AF            uint8           `json:"af"`
	Flags         uint8           `json:"flags"`
	AddressPrefix string          `json:"address-prefix"`
	SubTLVs       []extSubTLVJSON `json:"sub-tlvs,omitempty"`
}

type extLinkJSON struct {
	LinkType uint8           `json:"link-type"`
	LinkID   string          `json:"link-id"`
	LinkData string          `json:"link-data"`
	SubTLVs  []extSubTLVJSON `json:"sub-tlvs,omitempty"`
}

// extSubTLVsToJSON renders decoded RFC 7684 sub-TLVs as type/length/hex rows.
func extSubTLVsToJSON(subs []ExtSubTLV) []extSubTLVJSON {
	if len(subs) == 0 {
		return nil
	}
	out := make([]extSubTLVJSON, 0, len(subs))
	for _, s := range subs {
		out = append(out, extSubTLVJSON{Type: s.Type, Length: len(s.Value), Data: hex.EncodeToString(s.Value)})
	}
	return out
}

// opaqueBodyToJSON decodes a stored opaque body into its structured extended view when the
// Opaque Type is 7 (Extended Prefix) or 8 (Extended Link); otherwise the body is left as hex
// only. A malformed body (RFC 7684 sec 5) leaves the extended fields empty and keeps the hex.
func opaqueBodyToJSON(lsa LSA, out *opaqueLSAJSON) {
	switch lsa.OpaqueType() {
	case ExtPrefixOpaqueType:
		if body, err := DecodeExtPrefixLSA(lsa.Body); err == nil {
			for _, p := range body.Prefixes {
				out.ExtendedPrefix = append(out.ExtendedPrefix, extPrefixJSON{
					RouteType: p.RouteType, PrefixLength: p.PrefixLength, AF: p.AF, Flags: p.Flags,
					AddressPrefix: ipv4String(p.AddressPrefix), SubTLVs: extSubTLVsToJSON(p.SubTLVs),
				})
			}
		}
	case ExtLinkOpaqueType:
		if body, err := DecodeExtLinkLSA(lsa.Body); err == nil && body.HasLink {
			out.ExtendedLink = &extLinkJSON{
				LinkType: body.Link.LinkType, LinkID: ipv4String(body.Link.LinkID),
				LinkData: ipv4String(body.Link.LinkData), SubTLVs: extSubTLVsToJSON(body.Link.SubTLVs),
			}
		}
	}
}

// ToJSON renders a decoded packet to a stable JSON view.
func (p Packet) ToJSON() JSONView {
	v := JSONView{
		Type:          p.Header.Type.String(),
		RouterID:      p.Header.RouterID.String(),
		AreaID:        p.Header.AreaID.String(),
		Checksum:      p.Header.Checksum,
		ChecksumValid: p.VerifyChecksum(),
		InstanceID:    p.Header.InstanceID,
		AuType:        uint16(p.Header.AuType),
		Auth:          hex.EncodeToString(p.Header.Auth[:]),
	}
	switch {
	case p.Hello != nil:
		v.Hello = helloToJSON(*p.Hello)
	case p.DBDesc != nil:
		v.DBDesc = dbDescToJSON(*p.DBDesc)
	case p.LSReq != nil:
		v.LSReq = lsReqToJSON(*p.LSReq)
	case p.LSUpdate != nil:
		v.LSUpdate = lsUpdateToJSON(*p.LSUpdate)
	case p.LSAck != nil:
		v.LSAck = lsAckToJSON(*p.LSAck)
	}
	return v
}

func helloToJSON(h Hello) *helloJSON {
	neighbors := make([]string, 0, len(h.Neighbors))
	for _, n := range h.Neighbors {
		neighbors = append(neighbors, n.String())
	}
	return &helloJSON{
		NetworkMask:   ipv4String(h.NetworkMask),
		HelloInterval: h.HelloInterval,
		Options:       h.Options.String(),
		Priority:      h.Priority,
		DeadInterval:  h.DeadInterval,
		DR:            ipv4String(h.DR),
		BDR:           ipv4String(h.BDR),
		Neighbors:     neighbors,
	}
}

func dbDescToJSON(d DBDesc) *dbDescJSON {
	headers := make([]lsaHeaderJSON, 0, len(d.Headers))
	for _, h := range d.Headers {
		headers = append(headers, lsaHeaderToJSON(h))
	}
	return &dbDescJSON{InterfaceMTU: d.InterfaceMTU, Options: d.Options.String(), Flags: d.Flags, DDSequence: d.DDSequence, Headers: headers}
}

func lsReqToJSON(r LSReq) *lsReqJSON {
	requests := make([]lsRequestJSON, 0, len(r.Requests))
	for _, req := range r.Requests {
		requests = append(requests, lsRequestJSON{Type: req.Type.String(), LinkStateID: req.LinkStateID.String(), AdvertisingRouter: req.AdvertisingRouter.String()})
	}
	return &lsReqJSON{Requests: requests}
}

func lsUpdateToJSON(u LSUpdate) *lsUpdateJSON {
	lsas := make([]lsaJSON, 0, len(u.LSAs))
	for _, lsa := range u.LSAs {
		lsas = append(lsas, lsaToJSON(lsa))
	}
	return &lsUpdateJSON{LSAs: lsas}
}

func lsAckToJSON(a LSAck) *lsAckJSON {
	headers := make([]lsaHeaderJSON, 0, len(a.Headers))
	for _, h := range a.Headers {
		headers = append(headers, lsaHeaderToJSON(h))
	}
	return &lsAckJSON{Headers: headers}
}

func lsaHeaderToJSON(h LSAHeader) lsaHeaderJSON {
	return lsaHeaderJSON{Age: h.Age.Age(), Options: h.Options.String(), Type: h.Type.String(), LinkStateID: h.LinkStateID.String(), AdvertisingRouter: h.AdvertisingRouter.String(), Sequence: uint32(h.Sequence), Checksum: h.Checksum, Length: h.Length}
}

func lsaToJSON(lsa LSA) lsaJSON {
	out := lsaJSON{Header: lsaHeaderToJSON(lsa.Header), ChecksumValid: lsa.VerifyChecksum()}
	switch lsa.Header.Type {
	case types.LSTypeRouter:
		if body, err := lsa.DecodeRouter(); err == nil {
			links := make([]routerLinkJSON, 0, len(body.Links))
			for _, link := range body.Links {
				links = append(links, routerLinkJSON{LinkID: link.LinkID.String(), LinkData: ipv4String(link.LinkData), Type: link.Type, Metric: uint16(link.Metric)})
			}
			out.Router = &routerLSAJSON{Flags: body.Flags, Links: links}
		}
	case types.LSTypeNetwork:
		if body, err := lsa.DecodeNetwork(); err == nil {
			routers := make([]string, 0, len(body.AttachedRouters))
			for _, router := range body.AttachedRouters {
				routers = append(routers, router.String())
			}
			out.Network = &networkLSAJSON{NetworkMask: ipv4String(body.NetworkMask), AttachedRouters: routers}
		}
	case types.LSTypeSummaryNetwork, types.LSTypeSummaryASBR:
		if body, err := lsa.DecodeSummary(); err == nil {
			out.Summary = &summaryLSAJSON{NetworkMask: ipv4String(body.NetworkMask), TOS: body.TOS, Metric: body.Metric}
		}
	case types.LSTypeASExternal, types.LSTypeNSSA:
		if body, err := lsa.DecodeExternal(); err == nil {
			out.External = &externalLSAJSON{NetworkMask: ipv4String(body.NetworkMask), ExternalType2: body.ExternalType2, Metric: body.Metric, ForwardingAddr: ipv4String(body.ForwardingAddr), ExternalRouteTag: body.ExternalRouteTag}
		}
	case types.LSTypeOpaqueLink, types.LSTypeOpaqueArea, types.LSTypeOpaqueAS:
		out.Opaque = &opaqueLSAJSON{Data: hex.EncodeToString(lsa.Body)}
		// RFC 7684 (spec-ospf-ext-4): decode Extended Prefix (7) / Extended Link (8) bodies.
		opaqueBodyToJSON(lsa, out.Opaque)
	default:
		return out
	}
	return out
}
