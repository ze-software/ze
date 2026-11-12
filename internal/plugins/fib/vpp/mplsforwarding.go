// Design: docs/architecture/mpls/mpls-kernel.md -- `show mpls forwarding` on the VPP data plane
// Related: register_mpls_forwarding.go -- registers this reader for the "vpp" data plane
// Related: mpls.go -- the label routes and labeled IP routes this reader reads back
// Related: mplsstale.go -- the same MPLS table 0 dump, for the restart sweep
//
// `show mpls forwarding` reads the data plane the config selects. On VPP the
// swap and pop entries fib-vpp writes live in MPLS table 0, and the label
// pushes are IP routes whose path carries a label stack, in whichever IP
// table fib.vpp.table-id names. This reader dumps both through its own GoVPP
// channel, so it answers from what VPP holds, as the kernel reader answers
// from AF_MPLS, and it needs nothing of the fib-vpp writer's in-memory state:
// it answers the same whether fib-vpp runs in the daemon or forked.
//
// VPP carries no owner mark on a route, so every unicast label route with an
// unreserved in-label, and every IP route with a label stack, is shown: on
// the VPP Ze supervises fib-vpp is the only writer of either (mplsstale.go
// makes the same reading for its sweep).

package fibvpp

import (
	"net/netip"
	"slices"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/fib_types"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/mpls"

	vppcomp "github.com/ze-software/ze/internal/component/vpp"
	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/opererr"
)

// forwardingFix is the command that shows why VPP does not answer.
const forwardingFix = "run 'ze doctor' to see why VPP is not answering, then retry 'ze show mpls forwarding'"

// readMPLSForwarding is the "vpp" data plane's forwarding reader. It opens a
// channel on the vpp component's connection and refuses when there is none:
// with VPP unreachable the labels it holds are unknown, not absent.
func readMPLSForwarding(limit int) ([]mplsfib.ForwardingEntry, error) {
	connector := vppcomp.GetActiveConnector()
	if connector == nil {
		return nil, opererr.New("the VPP MPLS forwarding table cannot be read: the vpp component is not running",
			"the config selects the vpp data plane, but there is no GoVPP connection to read MPLS table 0 through, "+
				"so the labels VPP holds are unknown",
			"configure 'vpp { }', "+forwardingFix)
	}
	ch, err := connector.NewChannel()
	if err != nil {
		return nil, opererr.Wrap(err, "the VPP MPLS forwarding table cannot be read: no GoVPP channel opens",
			"VPP did not accept an API channel, so the labels it holds are unknown", forwardingFix)
	}
	defer ch.Close()
	return readVPPForwarding(ch, limit)
}

// vppForwardingRow is one row before its egress interface is named.
type vppForwardingRow struct {
	entry     mplsfib.ForwardingEntry
	swIfIndex uint32
}

// readVPPForwarding reads MPLS table 0 and the labeled routes of every IP
// table through ch, at most limit rows (limit <= 0: no cap). Each dump is
// read to its end even past the cap, so the channel carries no stale reply.
func readVPPForwarding(ch api.Channel, limit int) ([]mplsfib.ForwardingEntry, error) {
	rows, err := dumpLabelForwarding(ch)
	if err != nil {
		return nil, err
	}
	pushes, err := dumpPushForwarding(ch)
	if err != nil {
		return nil, err
	}
	rows = append(rows, pushes...)
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	names, err := interfaceNames(ch, rows)
	if err != nil {
		return nil, err
	}
	out := make([]mplsfib.ForwardingEntry, 0, len(rows))
	for i := range rows {
		rows[i].entry.Device = names[rows[i].swIfIndex]
		out = append(out, rows[i].entry)
	}
	return out, nil
}

// dumpLabelForwarding reads MPLS table 0 into one row per in-label, sorted by
// label. VPP holds one Ze entry as two routes, one per value of the S bit
// (addLabelRoutes); the S=1 route is the one shown, because it is the one an
// IP payload meets, and the S=0 route stands in only when it is alone.
func dumpLabelForwarding(ch api.Channel) ([]vppForwardingRow, error) {
	dump := ch.SendMultiRequest(&mpls.MplsRouteDump{Table: mpls.MplsTable{MtTableID: mplsTableZero}})
	byLabel := make(map[uint32]vppForwardingRow)
	bottom := make(map[uint32]bool)
	for {
		details := &mpls.MplsRouteDetails{}
		last, err := dump.ReceiveReply(details)
		if err != nil {
			return nil, opererr.Wrap(err, "the VPP MPLS forwarding table cannot be read: the MPLS table 0 dump failed",
				"VPP refused or dropped mpls_route_dump, so the labels it holds are unknown", forwardingFix)
		}
		if last {
			break
		}
		route := details.MrRoute
		if route.MrIsMulticast {
			continue
		}
		// RFC 3032 Section 6: the reserved labels 0-15 are VPP's own entries.
		if route.MrLabel <= reservedLabelsMax {
			continue
		}
		if bottom[route.MrLabel] {
			continue
		}
		row := vppForwardingRow{entry: mplsfib.ForwardingEntry{InLabel: int(route.MrLabel)}, swIfIndex: noInterface}
		if len(route.MrPaths) > 0 {
			row.entry.OutLabels, row.entry.NextHop = pathLabelsAndNextHop(&route.MrPaths[0])
			row.swIfIndex = route.MrPaths[0].SwIfIndex
		}
		row.entry.Operation = mplsfib.LabelOperation(row.entry.OutLabels)
		if row.entry.Operation == "pop" {
			// fib-vpp writes a pop as the single label Implicit NULL, which is
			// VPP's encoding and not an outgoing label: the kernel reader
			// shows a pop with no out-label, so this one does too.
			row.entry.OutLabels = nil
		}
		byLabel[route.MrLabel] = row
		bottom[route.MrLabel] = route.MrEos == 1
	}
	labels := make([]uint32, 0, len(byLabel))
	for label := range byLabel {
		labels = append(labels, label)
	}
	slices.Sort(labels)
	rows := make([]vppForwardingRow, 0, len(labels))
	for _, label := range labels {
		rows = append(rows, byLabel[label])
	}
	return rows, nil
}

// dumpPushForwarding reads every IP table VPP holds and keeps the routes
// whose first path carries a label stack: the label pushes addMPLSRoute
// installs. All tables are read because the table fib.vpp.table-id names is
// fib-vpp's config, which this reader does not hold.
func dumpPushForwarding(ch api.Channel) ([]vppForwardingRow, error) {
	tables, err := dumpIPTables(ch)
	if err != nil {
		return nil, err
	}
	var rows []vppForwardingRow
	for _, table := range tables {
		dump := ch.SendMultiRequest(&ip.IPRouteDump{Table: table})
		for {
			details := &ip.IPRouteDetails{}
			last, err := dump.ReceiveReply(details)
			if err != nil {
				return nil, opererr.Wrap(err, "the VPP MPLS forwarding table cannot be read: an IP route dump failed",
					"VPP refused or dropped ip_route_dump, so the label pushes it holds are unknown", forwardingFix)
			}
			if last {
				break
			}
			route := details.Route
			if len(route.Paths) == 0 || route.Paths[0].NLabels == 0 {
				continue
			}
			prefix, ok := prefixFromVPP(route.Prefix)
			if !ok {
				continue
			}
			row := vppForwardingRow{entry: mplsfib.ForwardingEntry{FEC: prefix.String(), Operation: "push"}}
			row.entry.OutLabels, row.entry.NextHop = pathLabelsAndNextHop(&route.Paths[0])
			row.swIfIndex = route.Paths[0].SwIfIndex
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// dumpIPTables lists the IPv4 and IPv6 tables VPP holds.
func dumpIPTables(ch api.Channel) ([]ip.IPTable, error) {
	dump := ch.SendMultiRequest(&ip.IPTableDump{})
	var tables []ip.IPTable
	for {
		details := &ip.IPTableDetails{}
		last, err := dump.ReceiveReply(details)
		if err != nil {
			return nil, opererr.Wrap(err, "the VPP MPLS forwarding table cannot be read: the IP table dump failed",
				"VPP refused or dropped ip_table_dump, so the label pushes it holds are unknown", forwardingFix)
		}
		if last {
			return tables, nil
		}
		tables = append(tables, details.Table)
	}
}

// interfaceNames names the egress interface of every row whose path names
// one, from sw_interface_dump. A path fib-vpp writes names no interface
// (noInterface: VPP resolves the next hop), so the dump runs only when some
// row needs it, and such a row's Device stays empty, as a kernel route with
// no output device does.
func interfaceNames(ch api.Channel, rows []vppForwardingRow) (map[uint32]string, error) {
	names := map[uint32]string{noInterface: ""}
	if !slices.ContainsFunc(rows, func(r vppForwardingRow) bool { return r.swIfIndex != noInterface }) {
		return names, nil
	}
	dump := ch.SendMultiRequest(&interfaces.SwInterfaceDump{SwIfIndex: interface_types.InterfaceIndex(noInterface)})
	for {
		details := &interfaces.SwInterfaceDetails{}
		last, err := dump.ReceiveReply(details)
		if err != nil {
			return nil, opererr.Wrap(err, "the VPP MPLS forwarding table cannot be read: the interface dump failed",
				"VPP refused or dropped sw_interface_dump, so the egress interfaces cannot be named", forwardingFix)
		}
		if last {
			return names, nil
		}
		names[uint32(details.SwIfIndex)] = details.InterfaceName
	}
}

// pathLabelsAndNextHop reads the label stack and the next hop of one path. A
// path with no address (an egress pop's table lookup) has no next hop.
func pathLabelsAndNextHop(path *fib_types.FibPath) ([]int, string) {
	count := min(int(path.NLabels), len(path.LabelStack))
	var labels []int
	for i := range count {
		labels = append(labels, int(path.LabelStack[i].Label))
	}
	var nextHop netip.Addr
	switch path.Proto {
	case fib_types.FIB_API_PATH_NH_PROTO_IP4:
		nextHop = netip.AddrFrom4(path.Nh.Address.GetIP4())
	case fib_types.FIB_API_PATH_NH_PROTO_IP6:
		nextHop = netip.AddrFrom16(path.Nh.Address.GetIP6())
	case fib_types.FIB_API_PATH_NH_PROTO_MPLS, fib_types.FIB_API_PATH_NH_PROTO_ETHERNET,
		fib_types.FIB_API_PATH_NH_PROTO_BIER:
		// A lookup in another MPLS table or a non-IP next hop: no IP address.
		return labels, ""
	default:
		// VPP's protocol set is open; an unknown one names no IP next hop.
		return labels, ""
	}
	if nextHop.IsUnspecified() {
		return labels, ""
	}
	return labels, nextHop.String()
}

// prefixFromVPP converts a VPP prefix to netip, false for an address family
// VPP added after this code.
func prefixFromVPP(p ip_types.Prefix) (netip.Prefix, bool) {
	switch p.Address.Af {
	case ip_types.ADDRESS_IP4:
		return netip.PrefixFrom(netip.AddrFrom4(p.Address.Un.GetIP4()), int(p.Len)), true
	case ip_types.ADDRESS_IP6:
		return netip.PrefixFrom(netip.AddrFrom16(p.Address.Un.GetIP6()), int(p.Len)), true
	default:
		// VPP's address-family set is open.
		return netip.Prefix{}, false
	}
}
