// Design: plan/learned/1030-ospf-ext-2-traffic-engineering.md -- `show ospf te-database` render +
// the opaque-area TE decode hook.
// RFC: rfc/short/rfc3630.md (Router-Address + Link TLV), rfc/short/rfc5392.md (inter-AS).
//
// `show ospf te-database` renders the TED (the received TE topology). The opaque-area TE
// decode hook decodes a stored TE opaque LSA body inline (Router-Address or Link TLV with
// sub-TLVs) so `show ospf database opaque-area` shows structured TE rather than raw hex.
// IPv4 rendering goes through netip (no fmt/Sprintf on the render path).

package ospf

import (
	"net/netip"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
)

// teDatabaseView is the `show ospf te-database` payload: the router addresses and links of
// the TED, with IPv4 values rendered as dotted-quad strings for a readable JSON view.
type teDatabaseView struct {
	RouterAddresses []teRouterAddressRow `json:"router-addresses"`
	Links           []teLinkRow          `json:"links"`
}

type teRouterAddressRow struct {
	Router  string `json:"router"`
	Address string `json:"address"`
}

// teLinkRow is one TED link entry rendered for display. Optional attributes use pointers so
// an absent sub-TLV is omitted rather than shown as a spurious zero.
type teLinkRow struct {
	AdvertisingRouter string    `json:"advertising-router"`
	Area              string    `json:"area"`
	Scope             string    `json:"scope"`
	Instance          uint32    `json:"instance"`
	Usable            bool      `json:"usable"`
	LinkType          string    `json:"link-type,omitempty"`
	LinkID            string    `json:"link-id,omitempty"`
	LocalAddresses    []string  `json:"local-addresses,omitempty"`
	RemoteAddresses   []string  `json:"remote-addresses,omitempty"`
	TEMetric          *uint32   `json:"te-metric,omitempty"`
	MaxBandwidth      *float64  `json:"max-bandwidth,omitempty"`
	MaxReservable     *float64  `json:"max-reservable-bandwidth,omitempty"`
	Unreserved        []float64 `json:"unreserved-bandwidth,omitempty"`
	AdminGroup        *uint32   `json:"admin-group,omitempty"`
	AdminGroups       []uint    `json:"admin-groups,omitempty"`
	RemoteAS          *uint32   `json:"remote-as,omitempty"`
	RemoteASBRv4      string    `json:"remote-asbr-ipv4,omitempty"`
	RemoteASBRv6      string    `json:"remote-asbr-ipv6,omitempty"`
}

// teDecodedLSA is one opaque-area TE LSA decoded inline for `show ospf database
// opaque-area`: either a Router-Address value or a decoded Link (embedded teLinkRow).
type teDecodedLSA struct {
	AdvertisingRouter string `json:"advertising-router"`
	Scope             string `json:"scope"`
	Instance          uint32 `json:"instance"`
	RouterAddress     string `json:"router-address,omitempty"`
	teLinkRow
}

// teDatabaseSnapshot renders the TED for `show ospf te-database` (AC-15). It returns the
// single-element wrapping the other `show ospf ...` commands use.
func (e *engine) teDatabaseSnapshot() []any {
	view := teDatabaseView{}
	if e.ted == nil {
		return []any{view}
	}
	snap := e.ted.Snapshot()
	for _, ra := range snap.RouterAddresses {
		view.RouterAddresses = append(view.RouterAddresses, teRouterAddressRow{
			Router: ra.Router.String(), Address: ipv4Str(ra.Address),
		})
	}
	for i := range snap.Links {
		l := &snap.Links[i]
		row := teLinkRowFromLink(l.Link)
		row.AdvertisingRouter = l.AdvertisingRouter.String()
		row.Area = l.Area.String()
		row.Scope = l.Scope.String()
		row.Instance = l.OpaqueID
		row.Usable = l.Usable
		view.Links = append(view.Links, row)
	}
	return []any{view}
}

// teDecodeOpaqueLSA decodes one stored TE opaque LSA body for inline display (AC-16). ok is
// false when the body is not a valid TE LSA (it is then left to the generic hex view).
func teDecodeOpaqueLSA(v ospflsdb.OpaqueLSAView) (teDecodedLSA, bool) {
	lsa, err := packet.DecodeTELSA(v.Body)
	if err != nil {
		return teDecodedLSA{}, false
	}
	out := teDecodedLSA{
		AdvertisingRouter: v.AdvertisingRouter.String(),
		Scope:             OpaqueScope(v.Scope).String(),
		Instance:          v.OpaqueID,
	}
	switch {
	case lsa.IsRouterAddress:
		out.RouterAddress = ipv4Str(lsa.RouterAddress)
	case lsa.IsLink:
		out.teLinkRow = teLinkRowFromLink(lsa.Link)
	default:
		return teDecodedLSA{}, false
	}
	return out, true
}

// teDecodedDatabase wraps the inline TE decode appended to a `show ospf database
// opaque-area` / `opaque-as` response so TE bodies are shown structured, not as raw hex.
type teDecodedDatabase struct {
	TE []teDecodedLSA `json:"te"`
}

// databaseOpaqueWithTEDecode returns the generic opaque-scope database snapshot with the
// TE-decoded bodies appended for the given scope (AC-16). When no TE LSAs are present it is
// identical to the plain subview.
func (e *engine) databaseOpaqueWithTEDecode(lsType string, scope OpaqueScope) []any {
	out := e.databaseSnapshotByType(lsType)
	if decoded := e.teOpaqueAreaDecode(scope); len(decoded) > 0 {
		out = append(out, teDecodedDatabase{TE: decoded})
	}
	return out
}

// teOpaqueAreaDecode returns the TE-decoded view of every stored TE opaque LSA of the given
// scope, for enriching `show ospf database opaque-area` / `opaque-as` (AC-16).
func (e *engine) teOpaqueAreaDecode(scope OpaqueScope) []teDecodedLSA {
	if e.lsdb == nil {
		return nil
	}
	var out []teDecodedLSA
	for _, opaqueType := range []uint8{packet.TEOpaqueType, packet.InterAsTEOpaqueType} {
		for _, v := range e.lsdb.OpaqueLSAsByType(opaqueType) {
			if OpaqueScope(v.Scope) != scope {
				continue
			}
			if decoded, ok := teDecodeOpaqueLSA(v); ok {
				out = append(out, decoded)
			}
		}
	}
	return out
}

// teLinkRowFromLink flattens a decoded packet.TELink into a display row.
func teLinkRowFromLink(l packet.TELink) teLinkRow {
	row := teLinkRow{}
	if l.HasLinkType {
		row.LinkType = teLinkTypeString(l.LinkType)
	}
	if l.HasLinkID {
		row.LinkID = ipv4Str(l.LinkID)
	}
	for _, ip := range l.LocalIPs {
		row.LocalAddresses = append(row.LocalAddresses, ipv4Str(ip))
	}
	for _, ip := range l.RemoteIPs {
		row.RemoteAddresses = append(row.RemoteAddresses, ipv4Str(ip))
	}
	if l.HasTEMetric {
		m := l.TEMetric
		row.TEMetric = &m
	}
	if l.HasMaxBandwidth {
		b := l.MaxBandwidth
		row.MaxBandwidth = &b
	}
	if l.HasMaxReservable {
		b := l.MaxReservable
		row.MaxReservable = &b
	}
	if l.HasUnreserved {
		row.Unreserved = append(row.Unreserved, l.Unreserved[:]...)
	}
	if l.HasAdminGroup {
		g := l.AdminGroup
		row.AdminGroup = &g
		// RFC 3630 sec 2.5.9: decode the mask into the set administrative group numbers
		// (LSB = group 0) for a readable view.
		for n := range uint(32) {
			if packet.TEAdminGroupHasGroup(g, n) {
				row.AdminGroups = append(row.AdminGroups, n)
			}
		}
	}
	if l.HasRemoteAS {
		as := l.RemoteAS
		row.RemoteAS = &as
	}
	if l.HasRemoteASBRv4 {
		row.RemoteASBRv4 = ipv4Str(l.RemoteASBRv4)
	}
	if l.HasRemoteASBRv6 {
		row.RemoteASBRv6 = netip.AddrFrom16(l.RemoteASBRv6).String()
	}
	return row
}

func teLinkTypeString(t uint8) string {
	switch t {
	case packet.TELinkTypePointToPoint:
		return networkPointToPoint
	case packet.TELinkTypeMultiAccess:
		return "multi-access"
	default:
		return "unknown"
	}
}

// ipv4Str renders a 4-octet address as a dotted quad without fmt.
func ipv4Str(ip [4]byte) string { return netip.AddrFrom4(ip).String() }
