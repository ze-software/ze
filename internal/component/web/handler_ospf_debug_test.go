// VALIDATES: spec-ospf-ext-14 AC-22, A-7, R-6 -- the read-only opaque (IPv4) and OSPFv3
// (IPv6) database web views dispatch the right read-only `show` command and render JSON; NO
// exported OSPF web handler dispatches an inject command (injection is CLI + authz only).
// PREVENTS: a remote LSDB-write path via the web, or a database view that dispatches the
// wrong command.
package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestOSPFOpaqueWebView(t *testing.T) {
	var got string
	h := &OSPFHandlers{Dispatch: fakeOSPFDispatch(`[{"areas":[]}]`, &got)}
	req := httptest.NewRequestWithContext(t.Context(), "GET", "/ospf/database/opaque?format=json", http.NoBody)
	rec := httptest.NewRecorder()
	h.HandleOSPFOpaqueDatabase()(rec, req)

	if got != "show ospf database opaque-area" {
		t.Errorf("dispatched %q, want 'show ospf database opaque-area'", got)
	}
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestOSPFv3DatabaseWebView(t *testing.T) {
	var got string
	h := &OSPFHandlers{Dispatch: fakeOSPFDispatch(`[{"address-family":"ipv6-unicast","lsas":[]}]`, &got)}
	req := httptest.NewRequestWithContext(t.Context(), "GET", "/ospfv3/database?format=json", http.NoBody)
	rec := httptest.NewRecorder()
	h.HandleOSPFv3Database()(rec, req)

	if got != "show ospf ipv6 database" {
		t.Errorf("dispatched %q, want 'show ospf ipv6 database'", got)
	}
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// TestNoInjectWebRoute / TestNoV3InjectWebRoute: no exported OSPF web handler is an
// inject/origination route. The assertion reflects over EVERY exported method of
// *OSPFHandlers (the route-registration surface used by cmd/ze/hub/service_web.go) so a
// future inject handler added to the struct is caught automatically -- it is not a
// hand-maintained allow-list that a new method could slip past. (The route table wires only
// these read-only handlers.)
func TestNoInjectWebRoute(t *testing.T) {
	assertNoInjectDispatch(t)
}

func TestNoV3InjectWebRoute(t *testing.T) {
	assertNoInjectDispatch(t)
}

// assertNoInjectDispatch enumerates every exported method of *OSPFHandlers by reflection and
// asserts (1) no method NAME looks like an inject/origination route, and (2) every
// route-builder method (zero args, returns http.HandlerFunc) dispatches a command that does
// not contain "inject". A pre-canceled request context makes the SSE stream handlers push
// their single initial snapshot and then return instead of streaming forever.
func assertNoInjectDispatch(t *testing.T) {
	t.Helper()
	var got string
	h := &OSPFHandlers{Dispatch: fakeOSPFDispatch(`[]`, &got)}

	handlerFuncType := reflect.TypeFor[http.HandlerFunc]()
	hv := reflect.ValueOf(h)
	ht := hv.Type()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	drove := 0
	for i := range hv.NumMethod() {
		name := ht.Method(i).Name
		lname := strings.ToLower(name)
		// Durability: injection/origination must never be exposed as a web handler, whether
		// or not it is driveable below.
		if strings.Contains(lname, "inject") || strings.Contains(lname, "originate") {
			t.Fatalf("OSPFHandlers exposes method %q -- inject/origination must never be a web route", name)
		}

		m := hv.Method(i)
		mt := m.Type()
		// Route builders take no args and return an http.HandlerFunc. Drive those; skip any
		// other method shape (there are none today, but the guard keeps the reflection safe).
		if mt.NumIn() != 0 || mt.NumOut() != 1 || mt.Out(0) != handlerFuncType {
			continue
		}
		fn, ok := reflect.TypeAssert[http.HandlerFunc](m.Call(nil)[0])
		if !ok {
			continue
		}

		got = ""
		req := httptest.NewRequestWithContext(t.Context(), "GET", "/x?format=json", http.NoBody).WithContext(ctx)
		fn(httptest.NewRecorder(), req)
		if strings.Contains(got, "inject") {
			t.Fatalf("handler %q dispatched an inject command: %q", name, got)
		}
		drove++
	}
	// Guard against the reflection silently matching nothing (e.g. the Handle* methods are
	// renamed or their signature changes): at least the known read-only views must be driven.
	if drove == 0 {
		t.Fatal("no OSPFHandlers route-builder methods were enumerated; reflection matched nothing")
	}
}
