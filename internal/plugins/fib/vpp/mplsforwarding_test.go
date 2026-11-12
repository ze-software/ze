// Design: docs/architecture/mpls/mpls-kernel.md -- VPP `show mpls forwarding` reader tests
package fibvpp

import (
	"errors"
	"fmt"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/fib_types"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/mpls"

	"github.com/ze-software/ze/internal/core/mplsfib"
)

// forwardingChannel is an api.Channel that answers the four dumps the
// forwarding reader sends from canned tables, in the shape of testChannel
// (apply_test.go). dumpErr fails the dump of the named message.
type forwardingChannel struct {
	labelRoutes []mpls.MplsRoute
	ipTables    []ip.IPTable
	ipRoutes    map[uint32][]ip.IPRoute // by table id
	ifaces      []interfaces.SwInterfaceDetails
	dumpErr     map[string]error
	sent        []string // message name of every dump, in order
}

var _ api.Channel = (*forwardingChannel)(nil)

func (c *forwardingChannel) SendRequest(msg api.Message) api.RequestCtx {
	return &forwardingRefusedCtx{name: msg.GetMessageName()}
}

func (c *forwardingChannel) SendMultiRequest(msg api.Message) api.MultiRequestCtx {
	c.sent = append(c.sent, msg.GetMessageName())
	var replies []api.Message
	switch req := msg.(type) {
	case *mpls.MplsRouteDump:
		for _, r := range c.labelRoutes {
			replies = append(replies, &mpls.MplsRouteDetails{MrRoute: r})
		}
	case *ip.IPTableDump:
		for _, t := range c.ipTables {
			replies = append(replies, &ip.IPTableDetails{Table: t})
		}
	case *ip.IPRouteDump:
		for _, r := range c.ipRoutes[req.Table.TableID] {
			replies = append(replies, &ip.IPRouteDetails{Route: r})
		}
	case *interfaces.SwInterfaceDump:
		for i := range c.ifaces {
			replies = append(replies, &c.ifaces[i])
		}
	}
	return &forwardingDumpCtx{replies: replies, err: c.dumpErr[msg.GetMessageName()], name: msg.GetMessageName()}
}

func (c *forwardingChannel) SubscribeNotification(chan api.Message, api.Message) (api.SubscriptionCtx, error) {
	return nil, nil //nolint:nilnil // test stub, never called
}
func (c *forwardingChannel) SetReplyTimeout(time.Duration)          {}
func (c *forwardingChannel) CheckCompatiblity(...api.Message) error { return nil }
func (c *forwardingChannel) Close()                                 {}

// forwardingRefusedCtx fails every plain request: the reader sends none.
type forwardingRefusedCtx struct{ name string }

func (r *forwardingRefusedCtx) ReceiveReply(api.Message) error {
	return fmt.Errorf("the forwarding reader sent a mutation %s", r.name)
}

// forwardingDumpCtx copies one canned reply per call, then ends the dump.
type forwardingDumpCtx struct {
	replies []api.Message
	err     error
	name    string
	index   int
}

func (r *forwardingDumpCtx) ReceiveReply(reply api.Message) (bool, error) {
	if r.err != nil {
		return false, r.err
	}
	if r.index == len(r.replies) {
		return true, nil
	}
	source := reflect.ValueOf(r.replies[r.index])
	target := reflect.ValueOf(reply)
	if source.Type() != target.Type() {
		return false, fmt.Errorf("%s reply decoded into %T, served %T", r.name, reply, r.replies[r.index])
	}
	target.Elem().Set(source.Elem())
	r.index++
	return false, nil
}

// vppPath builds one FIB path with an IPv4 next hop (invalid: none), the
// labels in order and the interface index.
func vppPath(nextHop string, swIfIndex uint32, labels ...uint32) fib_types.FibPath {
	path := fib_types.FibPath{SwIfIndex: swIfIndex}
	if nextHop != "" {
		path.Proto, path.Nh.Address = nextHopAddress(netip.MustParseAddr(nextHop))
	}
	uniformLabels(&path, labels)
	return path
}

// TestVPPForwardingReadsTableZeroAndPushes proves the VPP reader answers the
// rows `show mpls forwarding` shows on the kernel, in the kernel's shape: one
// swap or pop row per in-label though VPP holds two routes per label (S=1 and
// S=0), the reserved labels and multicast left out, a labeled IP route as a
// push row keyed by FEC, and an egress interface named from VPP.
//
// Method: a fake channel holds a swap (both S positions), an egress pop whose
// S=1 path is a table lookup, a reserved label, a multicast route, and two IP
// tables, one carrying a labeled route and an unlabeled one.
func TestVPPForwardingReadsTableZeroAndPushes(t *testing.T) {
	egressPop := labelRoute{inLabel: 300, fec: netip.MustParsePrefix("10.9.0.0/24")}
	popS1, err := labelRouteFor(egressPop, 1)
	if err != nil {
		t.Fatal(err)
	}
	popS0, err := labelRouteFor(egressPop, 0)
	if err != nil {
		t.Fatal(err)
	}
	ch := &forwardingChannel{
		labelRoutes: []mpls.MplsRoute{
			{MrLabel: 200, MrEos: 0, MrNPaths: 1, MrPaths: []fib_types.FibPath{vppPath("10.0.0.2", 5, 201)}},
			{MrLabel: 200, MrEos: 1, MrNPaths: 1, MrPaths: []fib_types.FibPath{vppPath("10.0.0.2", 5, 201)}},
			popS0,
			popS1,
			{MrLabel: 3, MrEos: 1},
			{MrLabel: 400, MrEos: 1, MrIsMulticast: true},
		},
		ipTables: []ip.IPTable{{TableID: 0}, {TableID: 7}},
		ipRoutes: map[uint32][]ip.IPRoute{
			7: {
				{TableID: 7, Prefix: toVPPPrefix(netip.MustParsePrefix("192.0.2.0/24")), NPaths: 1,
					Paths: []fib_types.FibPath{vppPath("10.0.0.3", noInterface)}},
				{TableID: 7, Prefix: toVPPPrefix(netip.MustParsePrefix("198.51.100.0/24")), NPaths: 1,
					Paths: []fib_types.FibPath{vppPath("10.0.0.3", noInterface, 500, 501)}},
			},
		},
		ifaces: []interfaces.SwInterfaceDetails{{SwIfIndex: interface_types.InterfaceIndex(5), InterfaceName: "GigabitEthernet0/8/0"}},
	}

	got, err := readVPPForwarding(ch, 0)
	if err != nil {
		t.Fatalf("readVPPForwarding: %v", err)
	}
	want := []mplsfib.ForwardingEntry{
		{InLabel: 200, Operation: "swap", OutLabels: []int{201}, NextHop: "10.0.0.2", Device: "GigabitEthernet0/8/0"},
		{InLabel: 300, Operation: "pop"},
		{FEC: "198.51.100.0/24", Operation: "push", OutLabels: []int{500, 501}, NextHop: "10.0.0.3"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rows:\n got %+v\nwant %+v", got, want)
	}
}

// TestVPPForwardingLimit proves the reader answers at most limit rows, sorted
// by in-label, and that the cap does not skip a dump: the IP tables are still
// read, so no dump is left half-received on the channel.
func TestVPPForwardingLimit(t *testing.T) {
	ch := &forwardingChannel{
		labelRoutes: []mpls.MplsRoute{
			{MrLabel: 102, MrEos: 1, MrPaths: []fib_types.FibPath{vppPath("10.0.0.2", noInterface, 103)}},
			{MrLabel: 100, MrEos: 1, MrPaths: []fib_types.FibPath{vppPath("10.0.0.2", noInterface, 101)}},
		},
	}
	got, err := readVPPForwarding(ch, 1)
	if err != nil {
		t.Fatalf("readVPPForwarding: %v", err)
	}
	if len(got) != 1 || got[0].InLabel != 100 {
		t.Fatalf("limit 1: got %+v", got)
	}
	if want := []string{"mpls_route_dump", "ip_table_dump"}; !reflect.DeepEqual(ch.sent, want) {
		t.Fatalf("dumps sent %v, want %v", ch.sent, want)
	}
}

// TestVPPForwardingDumpFailureIsAnError proves a dump VPP refuses is an error
// carrying the fix, never an empty table read as "no label is installed".
//
// Method: fail each of the three dumps the reader always sends in turn.
func TestVPPForwardingDumpFailureIsAnError(t *testing.T) {
	for _, name := range []string{"mpls_route_dump", "ip_table_dump", "ip_route_dump"} {
		t.Run(name, func(t *testing.T) {
			ch := &forwardingChannel{
				labelRoutes: []mpls.MplsRoute{{MrLabel: 100, MrEos: 1}},
				ipTables:    []ip.IPTable{{TableID: 0}},
				dumpErr:     map[string]error{name: errors.New("VPP API timeout")},
			}
			got, err := readVPPForwarding(ch, 0)
			if err == nil {
				t.Fatalf("a failed %s answered %+v and no error", name, got)
			}
			if got != nil {
				t.Errorf("a failed %s still answered rows %+v", name, got)
			}
			if !strings.Contains(err.Error(), "VPP API timeout") || !strings.Contains(err.Error(), "ze doctor") {
				t.Errorf("error lacks its cause or fix: %v", err)
			}
		})
	}
}

// TestVPPForwardingReaderIsRegistered proves the "vpp" data plane's reader is
// this package's, so `show mpls forwarding` on a VPP config reaches it.
func TestVPPForwardingReaderIsRegistered(t *testing.T) {
	reader, ok := mplsfib.ForwardingReaderFor(vppDataPlane)
	if !ok {
		t.Fatal("no forwarding reader registered for the vpp data plane")
	}
	if reflect.ValueOf(reader).Pointer() != reflect.ValueOf(readMPLSForwarding).Pointer() {
		t.Fatal("the vpp data plane's reader is not readMPLSForwarding")
	}
}
