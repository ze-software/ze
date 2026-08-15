// Design: docs/architecture/chaos-web-dashboard.md -- web dashboard UI
// Related: handlers.go -- registerRoutes, the one caller

package web

import (
	"net/http"

	"github.com/ze-software/ze/internal/core/errorfragment"
)

// fragmentMux registers every dashboard route behind the error-fragment
// middleware, so the 30 http.Error sites in this package answer an htmx request
// with markup the browser can swap into the target rather than a bare status
// line.
//
// The wrap is per route rather than around the mux, because a chaos run started
// with --metrics-addr shares its ServeMux with the metrics endpoint
// (internal/chaos/orchestrator/run.go). A dashboard mounted on somebody else's
// mux owns no handler chain, so there is no single place to wrap; taking "/"
// over on that mux would claim every path the owner has not registered.
//
// It carries http.ServeMux's own two method signatures, so registerRoutes reads
// as a plain route list and the capture that derives the route set from that
// source keeps working (golden.RoutePatterns, internal/test/golden). This type
// lives in its own file for the same reason: the capture reads handlers.go, and
// a registration whose pattern is a parameter is one it must report.
type fragmentMux struct{ base *http.ServeMux }

// Handle registers a handler behind the middleware.
func (m fragmentMux) Handle(pattern string, handler http.Handler) {
	m.base.Handle(pattern, errorfragment.Middleware(handler))
}

// HandleFunc registers a handler function behind the middleware.
func (m fragmentMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	m.base.Handle(pattern, errorfragment.Middleware(http.HandlerFunc(handler)))
}
