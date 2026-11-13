// Design: docs/architecture/rib/unified-locrib.md -- the end of the consumer's engine session is a normal stop.
// Related: route_view_forked_stop_test.go -- the forked consumer harness these tests reuse.

package sysrib

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Goal: a long-poll read that the daemon's stop ends returns that
// cancellation as its cause. The runtime dispatch leaves exactly such a
// failure unanswered (pluginserver engineSessionEnded), so the connection
// close answers the forked consumer; a read that dropped the cause would be
// answered as a refusal again. Method: the read enters its bounded wait under
// a context standing for the server's lifetime context, which the runtime
// dispatch passes; the wait seam holds until that context is canceled, so
// the wait's timer cannot win.
func TestRouteViewReadEndedByServerStopCarriesCancellation(t *testing.T) {
	var authority routeViewAuthority
	server, stop := context.WithCancel(context.Background())
	defer stop()
	waiting := make(chan struct{})
	authority.beforeWait = func() {
		close(waiting)
		<-server.Done()
	}
	owner, opened := routeViewOpenTest(t, &authority, locrib.NewRIB())
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	done := make(chan error, 1)
	go func() {
		_, err := authority.read(server, owner, rpc.RouteViewReadInput{Session: opened.Session, Cursor: page.Cursor, Limit: 1, WaitMillis: rpc.RouteViewWaitMillisMax})
		done <- err
	}()
	select {
	case <-waiting:
	case <-time.After(5 * time.Second):
		t.Fatal("read did not enter its bounded wait")
	}
	stop()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("read ended by the server's stop lost its cancellation cause, so the dispatch would answer it as a refusal: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server stop did not end the waiting read")
	}
}

// Goal: a live owner's refusal of a read carries no cancellation, so the
// runtime dispatch still answers it. Method: with the server context live,
// read at a cursor the session never returned and with a token it never
// issued.
func TestRouteViewLiveReadRefusalCarriesNoCancellation(t *testing.T) {
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, locrib.NewRIB())
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	for name, input := range map[string]rpc.RouteViewReadInput{
		"stale cursor":  {Session: opened.Session, Cursor: page.Cursor + 1, Limit: 1},
		"foreign token": {Session: opened.Session + "-foreign", Cursor: page.Cursor, Limit: 1},
	} {
		_, err := authority.read(context.Background(), owner, input)
		if err == nil {
			t.Fatalf("%s: read succeeded, want a refusal", name)
		}
		if errors.Is(err, context.Canceled) {
			t.Fatalf("%s: live refusal carries a cancellation, so the dispatch would withhold it: %v", name, err)
		}
	}
}

// Goal: a live owner's refusal that reads exactly like the stop answer the
// daemon used to send stays a retained failure. The fix is that the daemon no
// longer sends it at a stop; the consumer never forgives a refusal by its
// text. Method: the text is produced by routeViewStopped itself for the
// observed owner name, the engine answers the second read with it as an error
// line and keeps the connection open.
func TestRouteViewForkedLiveReadRefusalNamingStopRemainsFatal(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	owner := process.NewProcess(plugin.PluginConfig{Name: "sysrib-owner"})
	stopped := routeViewStopped(context.Canceled, owner)
	text := stopped.Error()
	feed := newRouteViewForkedFeed()
	feed.readFault = errors.New(text)
	h := newRouteViewForkedEngine(t, ctx, feed.answer)
	client := h.startClient(t)
	h.awaitUseful(t, feed, client)
	h.awaitStopped(t, client)
	var refusal *rpc.RPCCallError
	if !errors.As(client.Failure(), &refusal) {
		t.Fatalf("live refusal naming a stop was not retained as an RPC refusal: %T %v", client.Failure(), client.Failure())
	}
	if refusal.Message != text {
		t.Fatalf("live refusal lost its published cause: %q, want %q", refusal.Message, text)
	}
	findings, err := client.health.findings(routeViewDoctorName)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("live refusal findings=%+v, want one actual failed state", findings)
	}
	if findings[0].Severity != "error" {
		t.Fatalf("live refusal naming a stop became normal-stop health: %+v", findings[0])
	}
	if findings[0].Code != routeViewDoctorCode {
		t.Fatalf("live refusal doctor code=%q, want %q", findings[0].Code, routeViewDoctorCode)
	}
	if !strings.Contains(findings[0].Message, "Selected routing request stopped before completion") {
		t.Fatalf("failed doctor state omitted the live cause: %+v", findings[0])
	}
}
