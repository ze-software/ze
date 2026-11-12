// Design: docs/research/vpp-deployment-reference.md -- VPP MPLS FIB programming
// Related: backend.go -- vppBackend interface extended with MPLS methods
// Related: fibvpp.go -- processEvent dispatches to MPLS when labels present
// Related: mplsentry.go -- (mpls-fib, entry) consumer that calls the label routes

package fibvpp

import (
	"errors"
	"fmt"
	"net/netip"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/fib_types"
	"go.fd.io/govpp/binapi/mpls"

	"github.com/ze-software/ze/internal/core/mplsfib"
)

const (
	// mplsTableZero is the MPLS label space every MPLS-enabled VPP interface
	// looks up (MPLS_FIB_DEFAULT_TABLE_ID). It is not fib.vpp.table-id, which
	// names an IP table.
	mplsTableZero = 0

	// implicitNullLabel is the reserved label value 3, which VPP reads as a pop
	// when it is the whole path label stack (fib_path_ext_is_imp_null).
	implicitNullLabel = 3

	// noInterface is VPP's "no sw_if_index" (~0). A path that carries 0 instead
	// is attached to local0, the interface VPP reserves for itself.
	noInterface = ^uint32(0)
)

// mplsBackend extends vppBackend with MPLS operations.
type mplsBackend interface {
	addMPLSRoute(prefix netip.Prefix, nextHop netip.Addr, labels []uint32) error
	delMPLSRoute(prefix netip.Prefix, labels []uint32) error
	// createMPLSTable creates MPLS table 0. VPP refuses a label route and an
	// interface enable until it exists; creating it again is harmless.
	createMPLSTable() error
	// addLabelRoutes installs both stack positions of one swap or pop entry.
	addLabelRoutes(r labelRoute) error
	// delLabelRoutes removes both stack positions of inLabel.
	delLabelRoutes(inLabel uint32) error
	// dumpLabelRoutes lists the unicast label routes MPLS table 0 holds whose
	// in-label is outside the reserved range, one per stack position.
	dumpLabelRoutes() ([]labelPosition, error)
	// delLabelRoute removes the label route of one stack position.
	delLabelRoute(p labelPosition) error
}

// labelPosition names one VPP label route: an in-label at one value of the
// S bit. One Ze swap or pop entry is two of them.
type labelPosition struct {
	inLabel uint32
	eos     uint8
}

// labelRoute is one swap or pop entry as VPP programs it. outLabels empty is a
// pop; nextHop invalid is an egress pop, which looks the payload up in the IP
// table of fec's family.
type labelRoute struct {
	inLabel   uint32
	outLabels []uint32
	nextHop   netip.Addr
	fec       netip.Prefix
	ipTableID uint32
}

// uniformLabels writes labels into path in order, each in uniform mode.
//
// RFC 3032 Section 2.4.2: "When a labeled packet is forwarded, the TTL field of the label stack entry at the top of the label stack MUST be set to the outgoing TTL value."
// RFC 3032 Section 2.4.3: "When an IP packet is first labeled, the TTL field of the label stack entry MUST BE set to the value of the IP TTL field."
// Uniform mode (FIB_MPLS_LSP_MODE_UNIFORM, from is_uniform in fib_api_path_decode)
// copies the TTL from the packet, so no fixed TTL is written.
func uniformLabels(path *fib_types.FibPath, labels []uint32) {
	path.NLabels = uint8(len(labels)) //nolint:gosec // callers validate <= mplsfib.MaxLabelStack
	for i, l := range labels {
		path.LabelStack[i] = fib_types.FibMplsLabel{IsUniform: 1, Label: l}
	}
}

// govppMPLSBackend implements mplsBackend using GoVPP binary API.
// A labeled push is an IP route, so it goes through source like every other
// IP route fib-vpp installs (routesource.go).
type govppMPLSBackend struct {
	ch      api.Channel
	tableID uint32
	source  *routeSource
}

// newGovppMPLSBackend binds the MPLS backend to ch. source MUST be the one
// the writer's IP backend uses, so the stale sweep sees pushes and plain
// routes as one set.
func newGovppMPLSBackend(ch api.Channel, tableID uint32, source *routeSource) *govppMPLSBackend {
	return &govppMPLSBackend{ch: ch, tableID: tableID, source: source}
}

// addMPLSRoute programs a label push: IP route with MPLS label stack on the FibPath.
func (b *govppMPLSBackend) addMPLSRoute(prefix netip.Prefix, nextHop netip.Addr, labels []uint32) error {
	if err := mplsfib.ValidateOutLabels(labels); err != nil {
		return err
	}
	path, err := gatewayPath(nextHop, prefix)
	if err != nil {
		return err
	}
	// RFC 3032 Section 2.4.3
	uniformLabels(&path, labels)
	key := ipRouteKey{table: b.tableID, prefix: prefix}
	return b.source.sendRoute("mpls ip_route_add_del_v2", true, key, []fib_types.FibPath{path})
}

// delMPLSRoute removes a labeled IP route.
func (b *govppMPLSBackend) delMPLSRoute(prefix netip.Prefix, _ []uint32) error {
	key := ipRouteKey{table: b.tableID, prefix: prefix}
	return b.source.sendRoute("mpls del ip_route_add_del_v2", false, key, nil)
}

// createMPLSTable creates MPLS table 0. mpls_table_create holds one API lock
// on the table however often it is called, so a repeat creates nothing.
func (b *govppMPLSBackend) createMPLSTable() error {
	req := &mpls.MplsTableAddDel{
		MtIsAdd: true,
		MtTable: mpls.MplsTable{MtTableID: mplsTableZero},
	}
	reply := &mpls.MplsTableAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("mpls MplsTableAddDel table 0: %w: %w", errVPPMutationUncertain, err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("mpls MplsTableAddDel table 0 retval=%d", reply.Retval)
	}
	return nil
}

// addLabelRoutes installs inLabel once per value of the S bit.
//
// RFC 3032 Section 2.1: "This bit is set to one for the last entry in the label stack (i.e., for the bottom of the stack), and zero for all other label stack entries."
// A packet whose in-label is the bottom of its stack and one that carries more
// labels below it look up different VPP entries (fp_eos), so one Ze entry is
// two VPP routes. vl_api_mpls_route_add_del_t_handler takes mr_eos_proto as
// the payload protocol of the S=1 route and MPLS for the S=0 one.
func (b *govppMPLSBackend) addLabelRoutes(r labelRoute) error {
	for _, eos := range [2]uint8{1, 0} {
		route, err := labelRouteFor(r, eos)
		if err != nil {
			return err
		}
		if err := b.sendLabelRoute(true, route); err != nil {
			return err
		}
	}
	return nil
}

// delLabelRoutes removes both stack positions of inLabel, attempting the
// second even when the first fails so neither is left behind.
func (b *govppMPLSBackend) delLabelRoutes(inLabel uint32) error {
	var result error
	for _, eos := range [2]uint8{1, 0} {
		route := mpls.MplsRoute{MrTableID: mplsTableZero, MrLabel: inLabel, MrEos: eos}
		result = errors.Join(result, b.sendLabelRoute(false, route))
	}
	return result
}

// reservedLabelsMax is the highest reserved label value. VPP installs its own
// entries for reserved labels when MPLS table 0 is created, and no Ze label
// allocator hands one out as an in-label.
//
// RFC 3032 Section 6: "Label values 0-15 inclusive have special meaning, as specified in this document, or as further assigned by IANA."
const reservedLabelsMax = 15

// dumpLabelRoutes reads every route of MPLS table 0 (mpls_route_dump) and
// keeps the unicast ones whose in-label is not reserved: those are the routes
// a Ze writer can have installed. A multicast route is never one, because
// fib-vpp installs none.
func (b *govppMPLSBackend) dumpLabelRoutes() ([]labelPosition, error) {
	dump := b.ch.SendMultiRequest(&mpls.MplsRouteDump{Table: mpls.MplsTable{MtTableID: mplsTableZero}})
	var positions []labelPosition
	for {
		details := &mpls.MplsRouteDetails{}
		last, err := dump.ReceiveReply(details)
		if err != nil {
			return nil, fmt.Errorf("mpls MplsRouteDump table 0: %w", err)
		}
		if last {
			return positions, nil
		}
		route := details.MrRoute
		if route.MrIsMulticast {
			continue
		}
		if route.MrLabel <= reservedLabelsMax {
			continue
		}
		positions = append(positions, labelPosition{inLabel: route.MrLabel, eos: route.MrEos})
	}
}

// delLabelRoute removes the label route of one stack position.
func (b *govppMPLSBackend) delLabelRoute(p labelPosition) error {
	return b.sendLabelRoute(false, mpls.MplsRoute{MrTableID: mplsTableZero, MrLabel: p.inLabel, MrEos: p.eos})
}

func (b *govppMPLSBackend) sendLabelRoute(add bool, route mpls.MplsRoute) error {
	req := &mpls.MplsRouteAddDel{MrIsAdd: add, MrRoute: route}
	reply := &mpls.MplsRouteAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("mpls MplsRouteAddDel label %d eos %d: %w: %w",
			route.MrLabel, route.MrEos, errVPPMutationUncertain, err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("mpls MplsRouteAddDel label %d eos %d retval=%d",
			route.MrLabel, route.MrEos, reply.Retval)
	}
	return nil
}

// labelRouteFor builds the VPP route of r at one stack position.
func labelRouteFor(r labelRoute, eos uint8) (mpls.MplsRoute, error) {
	path, payload, err := labelPath(r, eos)
	if err != nil {
		return mpls.MplsRoute{}, err
	}
	return mpls.MplsRoute{
		MrTableID:  mplsTableZero,
		MrLabel:    r.inLabel,
		MrEos:      eos,
		MrEosProto: uint8(payload), //nolint:gosec // FibPathNhProto values are 0 to 5
		MrNPaths:   1,
		MrPaths:    []fib_types.FibPath{path},
	}, nil
}

// labelPath builds the one path of r at stack position eos and the payload
// protocol of an S=1 route (VPP ignores it at S=0).
func labelPath(r labelRoute, eos uint8) (fib_types.FibPath, fib_types.FibPathNhProto, error) {
	path := fib_types.FibPath{Weight: defaultWeight(), SwIfIndex: noInterface}
	if r.nextHop.IsValid() {
		path.Proto, path.Nh.Address = nextHopAddress(r.nextHop)
		uniformLabels(&path, swapOrPopStack(r.outLabels))
		return path, path.Proto, nil
	}
	// RFC 3032 Section 2.2: "The LSR which pops the last label off the stack must therefore be able to identify the packet's network layer protocol."
	// An egress pop names no next hop, so the payload family comes from the FEC.
	if !r.fec.IsValid() {
		return fib_types.FibPath{}, 0, fmt.Errorf("mpls-fib: egress pop of in-label %d names neither a next hop "+
			"nor a FEC, so the payload's network layer protocol is unknown (RFC 3032 Section 2.2)", r.inLabel)
	}
	payload := fib_types.FIB_API_PATH_NH_PROTO_IP6
	if r.fec.Addr().Is4() {
		payload = fib_types.FIB_API_PATH_NH_PROTO_IP4
	}
	// RFC 3032 Section 2.1: "When an LSR would otherwise replace the label at the top of the stack with a new label, but the new label is "Implicit NULL", the LSR will pop the stack instead of doing the replacement."
	uniformLabels(&path, []uint32{implicitNullLabel})
	if eos == 1 {
		// A path with no address and no interface in a named table is a
		// lookup (DEAG): the popped IP packet is routed in that table.
		path.Proto = payload
		path.TableID = r.ipTableID
		return path, payload, nil
	}
	// More labels remain below: look the next one up in MPLS table 0.
	path.Proto = fib_types.FIB_API_PATH_NH_PROTO_MPLS
	path.TableID = mplsTableZero
	return path, payload, nil
}

// swapOrPopStack returns the labels a swap imposes, or the Implicit NULL label
// that makes VPP pop toward the next hop.
func swapOrPopStack(outLabels []uint32) []uint32 {
	if len(outLabels) > 0 {
		return outLabels
	}
	// RFC 3032 Section 2.1
	return []uint32{implicitNullLabel}
}

// mockMPLSBackend is a test double for MPLS operations.
type mockMPLSBackend struct {
	pushes      []mplsPushOp
	delPushes   []netip.Prefix
	tables      int
	labelAdds   []labelRoute
	labelDels   []uint32
	err         error
	labelAddErr error
	labelDelErr error
	// dumped is what dumpLabelRoutes answers, or dumpErr; positionDels
	// records each delLabelRoute, which fails with labelDelErr.
	dumped       []labelPosition
	dumpErr      error
	positionDels []labelPosition
}

type mplsPushOp struct {
	prefix  netip.Prefix
	nextHop netip.Addr
	labels  []uint32
}

func (m *mockMPLSBackend) addMPLSRoute(prefix netip.Prefix, nextHop netip.Addr, labels []uint32) error {
	if m.err != nil {
		return m.err
	}
	if err := mplsfib.ValidateOutLabels(labels); err != nil {
		return err
	}
	cp := make([]uint32, len(labels))
	copy(cp, labels)
	m.pushes = append(m.pushes, mplsPushOp{prefix, nextHop, cp})
	return nil
}

func (m *mockMPLSBackend) delMPLSRoute(prefix netip.Prefix, _ []uint32) error {
	if m.err != nil {
		return m.err
	}
	m.delPushes = append(m.delPushes, prefix)
	return nil
}

func (m *mockMPLSBackend) createMPLSTable() error {
	if m.err != nil {
		return m.err
	}
	m.tables++
	return nil
}

func (m *mockMPLSBackend) addLabelRoutes(r labelRoute) error {
	if m.err != nil {
		return m.err
	}
	if m.labelAddErr != nil {
		return m.labelAddErr
	}
	r.outLabels = append([]uint32(nil), r.outLabels...)
	m.labelAdds = append(m.labelAdds, r)
	return nil
}

func (m *mockMPLSBackend) delLabelRoutes(inLabel uint32) error {
	if m.err != nil {
		return m.err
	}
	if m.labelDelErr != nil {
		return m.labelDelErr
	}
	m.labelDels = append(m.labelDels, inLabel)
	return nil
}

func (m *mockMPLSBackend) dumpLabelRoutes() ([]labelPosition, error) {
	if m.dumpErr != nil {
		return nil, m.dumpErr
	}
	return append([]labelPosition(nil), m.dumped...), nil
}

func (m *mockMPLSBackend) delLabelRoute(p labelPosition) error {
	m.positionDels = append(m.positionDels, p)
	if m.labelDelErr != nil {
		return m.labelDelErr
	}
	return nil
}
