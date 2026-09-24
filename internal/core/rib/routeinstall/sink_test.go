// VALIDATES: the forked RouteSink marshals a locrib.Path into a wire
// RouteInstallEntry carrying the protocol NAME (not the numeric ProtocolID),
// numeric AFI/SAFI, and string prefix/next-hop, and dispatches via the SDK client.
// PREVENTS: a forked OSPF/IS-IS route being shipped with a per-process numeric
// ProtocolID the engine would misresolve, or a marshaling error crashing SPF.

package routeinstall

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// captureClient records the batches passed to RouteInstall/RouteRemove and can
// return a forced error to exercise the failure path.
var errTransientFlush = errors.New("transient")

type captureClient struct {
	installed    []rpc.RouteInstallEntry
	removed      []rpc.RouteRemoveEntry
	installCalls int
	err          error
	failFirst    int // return a transient error on the first N RouteInstall calls
	closed       bool
}

func (c *captureClient) RouteInstall(_ context.Context, routes []rpc.RouteInstallEntry) (uint32, error) {
	c.installCalls++
	if c.failFirst > 0 {
		c.failFirst--
		return 0, errTransientFlush
	}
	if c.err != nil {
		return 0, c.err
	}
	c.installed = append(c.installed, routes...)
	return uint32(len(routes)), nil
}

func (c *captureClient) RouteRemove(_ context.Context, routes []rpc.RouteRemoveEntry) (uint32, error) {
	c.removed = append(c.removed, routes...)
	return uint32(len(routes)), c.err
}

func (c *captureClient) Close() error {
	c.closed = true
	return nil
}

func v4u() family.Family { return family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast} }

func TestSinkInsertForwardMarshalsEntry(t *testing.T) {
	cc := &captureClient{}
	sink := New(context.Background(), cc)
	src := redistevents.RegisterProtocol("routeinstall-test-ospf")
	sink.InsertForward(v4u(), netip.MustParsePrefix("10.9.0.0/24"), locrib.Path{
		Source:    src,
		Instance:  2,
		NextHop:   netip.MustParseAddr("192.0.2.9"),
		Interface: "eth9",
		OnLink:    true,
		ECMP: []locrib.NextHop{
			{Addr: netip.MustParseAddr("192.0.2.10"), Interface: "eth10", OnLink: true, Weight: 2},
			{Addr: netip.MustParseAddr("192.0.2.11"), Interface: "eth11", Weight: 1},
		},
		AdminDistance: 110,
		Metric:        7,
	})
	sink.Flush()
	if len(cc.installed) != 1 {
		t.Fatalf("installed batches = %d, want 1", len(cc.installed))
	}
	wire, err := json.Marshal(cc.installed[0])
	if err != nil {
		t.Fatal(err)
	}
	var e rpc.RouteInstallEntry
	if err := json.Unmarshal(wire, &e); err != nil {
		t.Fatal(err)
	}
	if e.Interface != "eth9" || !e.OnLink {
		t.Fatalf("primary forwarding target lost in RPC: %+v", e)
	}
	if len(e.ECMP) != 2 {
		t.Fatalf("RPC ECMP members = %d, want 2", len(e.ECMP))
	}
	if !e.ECMP[0].OnLink || e.ECMP[0].Interface != "eth10" || e.ECMP[0].Weight != 2 {
		t.Errorf("direct RPC member lost forwarding fields: %+v", e.ECMP[0])
	}
	if e.ECMP[1].OnLink || e.ECMP[1].Interface != "eth11" || e.ECMP[1].Weight != 1 {
		t.Errorf("recursive RPC member changed forwarding fields: %+v", e.ECMP[1])
	}
	if e.Protocol != "routeinstall-test-ospf" {
		t.Errorf("Protocol = %q, want the source's registered NAME", e.Protocol)
	}
	if e.AFI != uint16(family.AFIIPv4) || e.SAFI != uint8(family.SAFIUnicast) {
		t.Errorf("AFI/SAFI = %d/%d, want %d/%d", e.AFI, e.SAFI, family.AFIIPv4, family.SAFIUnicast)
	}
	if e.Prefix != "10.9.0.0/24" || e.NextHop != "192.0.2.9" {
		t.Errorf("prefix/next-hop = %q/%q", e.Prefix, e.NextHop)
	}
	if e.Instance != 2 || e.AdminDistance != 110 || e.Metric != 7 {
		t.Errorf("instance/admin/metric = %d/%d/%d, want 2/110/7", e.Instance, e.AdminDistance, e.Metric)
	}
}

func TestSinkRemoveMarshalsEntry(t *testing.T) {
	cc := &captureClient{}
	sink := New(context.Background(), cc)
	src := redistevents.RegisterProtocol("routeinstall-test-isis")
	sink.Remove(v4u(), netip.MustParsePrefix("10.10.0.0/24"), src, 3)
	sink.Flush()
	if len(cc.removed) != 1 {
		t.Fatalf("removed batches = %d, want 1", len(cc.removed))
	}
	e := cc.removed[0]
	if e.Protocol != "routeinstall-test-isis" || e.Prefix != "10.10.0.0/24" || e.Instance != 3 {
		t.Errorf("entry = %+v, want protocol/prefix/instance routeinstall-test-isis/10.10.0.0/24/3", e)
	}
}

func TestSinkEmptyNextHopOmitted(t *testing.T) {
	cc := &captureClient{}
	sink := New(context.Background(), cc)
	src := redistevents.RegisterProtocol("routeinstall-test-conn")
	sink.InsertForward(v4u(), netip.MustParsePrefix("10.11.0.0/24"), locrib.Path{
		Source: src, AdminDistance: 110, // no next-hop (directly connected)
	})
	sink.Flush()
	if len(cc.installed) != 1 {
		t.Fatalf("installed batches = %d, want 1", len(cc.installed))
	}
	if cc.installed[0].NextHop != "" {
		t.Errorf("NextHop = %q, want empty for zero Addr", cc.installed[0].NextHop)
	}
}

func TestSinkInsertForwardSurvivesClientError(t *testing.T) {
	cc := &captureClient{err: context.DeadlineExceeded}
	sink := New(context.Background(), cc)
	src := redistevents.RegisterProtocol("routeinstall-test-err")
	// A failed install closes the connection so the engine can withdraw all
	// paths held by the disconnected producer.
	sink.InsertForward(v4u(), netip.MustParsePrefix("10.12.0.0/24"), locrib.Path{Source: src, AdminDistance: 110})
	sink.Flush()
	if !cc.closed {
		t.Fatal("persistent install failure left the engine connection live")
	}
}

// TestSinkBatchesMultipleOps: R-1 -- several buffered ops go out in one RPC batch.
func TestSinkBatchesMultipleOps(t *testing.T) {
	cc := &captureClient{}
	sink := New(context.Background(), cc)
	src := redistevents.RegisterProtocol("routeinstall-test-batch")
	for _, pfx := range []string{"10.13.0.0/24", "10.14.0.0/24", "10.15.0.0/24"} {
		sink.InsertForward(v4u(), netip.MustParsePrefix(pfx), locrib.Path{Source: src, AdminDistance: 110})
	}
	sink.Flush()
	if cc.installCalls != 1 {
		t.Errorf("RouteInstall called %d times; want 1 batched call", cc.installCalls)
	}
	if len(cc.installed) != 3 {
		t.Errorf("installed %d entries; want 3 in the batch", len(cc.installed))
	}
}

// TestSinkFlushRetriesOnTransientError: ISSUE 2 -- a transient RouteInstall error
// is retried, so a brief mux hiccup does not lose the batch; the engine receives
// the routes once a retry succeeds.
func TestSinkFlushRetriesOnTransientError(t *testing.T) {
	cc := &captureClient{failFirst: 2} // fail twice, then succeed (within maxFlushAttempts=3)
	sink := New(context.Background(), cc)
	src := redistevents.RegisterProtocol("routeinstall-test-retry")
	sink.InsertForward(v4u(), netip.MustParsePrefix("10.16.0.0/24"), locrib.Path{Source: src, AdminDistance: 110})
	sink.Flush()
	if cc.installCalls != 3 {
		t.Errorf("RouteInstall attempted %d times; want 3 (2 transient failures + 1 success)", cc.installCalls)
	}
	if len(cc.installed) != 1 {
		t.Errorf("delivered %d entries after retry; want 1 (batch not lost)", len(cc.installed))
	}
	if cc.closed {
		t.Fatal("successful retry closed the engine connection")
	}
}

// A failed withdrawal needs the same disconnect cleanup as a failed install.
func TestSinkRemoveFailureClosesClient(t *testing.T) {
	cc := &captureClient{err: context.DeadlineExceeded}
	sink := New(context.Background(), cc)
	source := redistevents.RegisterProtocol("routeinstall-test-remove-error")
	sink.Remove(v4u(), netip.MustParsePrefix("198.51.100.0/24"), source, 0)
	sink.Flush()
	if !cc.closed {
		t.Fatal("failed withdrawal left stale routes on a live connection")
	}
}
