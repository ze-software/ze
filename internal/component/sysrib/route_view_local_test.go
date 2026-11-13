// Design: docs/architecture/rib/unified-locrib.md -- direct local reader lease.
// Related: route_view.go -- authenticated reader negotiation.

package sysrib

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A local lease reads the authority directly. Selected-feed bounds and expiry
// cannot fence it when no selected stream was requested or retained.
func TestRouteViewLocalAuthorityLease(t *testing.T) {
	defaults, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	publishDistances(defaults)
	t.Cleanup(func() { publishDistances(defaults) })
	static := redistevents.RegisterProtocol("static")
	rib := locrib.NewRIB()
	var authority routeViewAuthority
	authority.scan = func(context.Context, *locrib.RIB, routeViewVisitor) error {
		t.Fatal("local lease scanned a serialized selected view")
		return nil
	}
	owner := routeViewOwner()
	opened, err := authority.open(context.Background(), owner, rib, rpc.RouteViewOpenInput{Reader: rpc.RouteViewReaderLocalAuthority})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := authority.cleanup(owner); err != nil {
			t.Error(err)
		}
	})
	for index := range 4097 {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(index >> 8), byte(index), 0}), 24)
		rib.Insert(family.IPv4Unicast, prefix, locrib.Path{Source: static, RouteType: routetype.Blackhole})
	}
	err = authority.withSession(context.Background(), owner, opened.Session, func(session *routeViewSession) error {
		if !session.ready || session.capture != nil || session.snapshot != nil || session.journal != nil {
			t.Fatalf("local lease retained a selected feed: ready=%v capture=%v snapshot=%d journal=%d", session.ready, session.capture != nil, len(session.snapshot), len(session.journal))
		}
		cursor, err := session.synchronize(context.Background())
		if err != nil || cursor != 0 {
			t.Fatalf("direct reader invented a stream barrier: cursor=%d err=%v", cursor, err)
		}
		return session.validateConsumed(0)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !authority.active.maintain(time.Now().Add(3 * time.Minute)) {
		t.Fatal("idle local maintenance did not run")
	}
	ack, err := authority.applyDistances(context.Background(), owner, routeViewDistanceInput(t, opened.Session, 1, map[string]int{"static": 5}))
	if err != nil || ack.Cursor != 0 {
		t.Fatalf("direct distance acknowledgment was not current: %+v %v", ack, err)
	}
	path, ok := rib.Best(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"))
	if !ok || path.AdminDistance != 5 {
		t.Fatalf("direct reader did not observe acknowledged reselection: %+v %v", path, ok)
	}
	if _, err := authority.read(context.Background(), owner, rpc.RouteViewReadInput{Session: opened.Session, Cursor: 0, Limit: 1}); err == nil {
		t.Fatal("local-only lease silently served selected-feed data")
	}
	bus := newTestEventBus()
	input := routeViewPublication{Session: opened.Session, Cursor: 0, Operation: 1, Batch: routeViewOutputBatch()}
	if _, err := authority.publish(context.Background(), owner, input, bus); err != nil {
		t.Fatalf("local lease refused current output after expiry and reselection: %v", err)
	}
}

// Refusing an unavailable optimization cannot become an external placement
// restriction. The same external process can always open the normal feed.
func TestRouteViewExternalLocalReaderRefusalPreservesFeed(t *testing.T) {
	owner := process.NewProcess(plugin.PluginConfig{Name: "forked-system-rib", Run: "rib"})
	rib := locrib.NewRIB()
	var authority routeViewAuthority
	if _, err := authority.open(context.Background(), owner, rib, rpc.RouteViewOpenInput{Reader: rpc.RouteViewReaderLocalAuthority}); err == nil {
		t.Fatal("external process received a local authority lease")
	}
	opened, err := authority.open(context.Background(), owner, rib, rpc.RouteViewOpenInput{})
	if err != nil {
		t.Fatalf("local optimization refusal disabled external operation: %v", err)
	}
	t.Cleanup(func() {
		if err := authority.cleanup(owner); err != nil {
			t.Error(err)
		}
	})
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	if !page.Complete {
		t.Fatal("external default feed never became ready")
	}
}

func TestRouteViewOpenReaderInput(t *testing.T) {
	defaultInput := rpc.RouteViewOpenInput{Reader: rpc.RouteViewReaderLocalAuthority}
	if err := json.Unmarshal([]byte(`{}`), &defaultInput); err != nil {
		t.Fatalf("absent reader refused the default: %v", err)
	}
	if defaultInput.Reader != "" {
		t.Fatalf("absent reader retained a previous optimization: %+v", defaultInput)
	}
	for _, reader := range []string{rpc.RouteViewReaderSelectedFeed, rpc.RouteViewReaderLocalAuthority} {
		var input rpc.RouteViewOpenInput
		if err := json.Unmarshal([]byte(`{"reader":"`+reader+`"}`), &input); err != nil {
			t.Fatalf("declared reader %q failed public decoding: %v", reader, err)
		}
		if input.Reader != reader {
			t.Fatalf("reader lost at public boundary: %+v", input)
		}
	}
	for _, raw := range []string{`{"reader":null}`, `{"reader":""}`, `{"reader":"unknown"}`, `{"reader":1}`, `{"reader":"selected-feed","owner":"rib"}`, `{"reader":"local-authority"} {}`} {
		input := rpc.RouteViewOpenInput{Reader: rpc.RouteViewReaderSelectedFeed}
		if err := json.Unmarshal([]byte(raw), &input); err == nil {
			t.Fatalf("invalid reader input succeeded: %s", raw)
		}
		if input.Reader != rpc.RouteViewReaderSelectedFeed {
			t.Fatal("invalid reader changed current input")
		}
	}
}
