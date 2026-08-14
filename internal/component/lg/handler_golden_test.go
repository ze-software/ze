// Related: server.go -- registerRoutes wires every route this capture requests
// Related: render.go -- renderPage and renderFragment, the wrappers a request reaches
// Related: golden_test.go -- the TEMPLATE capture, which bypasses those wrappers

package lg

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/golden"
)

// lgHandlerFixtures is where the captured responses live.
const lgHandlerFixtures = "handler"

// lgToken gates the second server this capture builds, so the bearer-token path
// is captured rather than described.
const lgToken = "golden-token"

// lgHandlerCase is one HTTP request and the response bytes it must keep
// producing.
type lgHandlerCase struct {
	// Name is the fixture stem.
	Name string
	// Method and Target are the request line.
	Method string
	Target string
	// Form, when set, is the urlencoded body.
	Form string
	// Auth, when set, is the Authorization header value. It selects the
	// token-gated server, so the capture covers what a gated deployment serves.
	Auth string
	// Gated requests the token-gated server without an Authorization header.
	Gated bool
	// Stream marks a route that writes until the client goes away. The request
	// carries a canceled context, so the capture holds what the handler emits
	// before it returns rather than blocking on a ticker.
	Stream bool
	// Rewrites normalize a volatile span. Each one names its producer.
	Rewrites []golden.Rewrite
}

// lgServerTimeRewrite drops the two clock reads in handleAPIStatus
// (handler_api.go: current_server and server_time). They are the only values in
// the looking glass that a second run changes.
var lgServerTimeRewrite = golden.Rewrite{
	Pattern:     regexp.MustCompile(`("(?:current_server|server_time)": ")[^"]*"`),
	Replacement: `${1}<TIME>"`,
}

// lgHandlerGoldenCases requests every route registerRoutes wires. The route
// list is NOT written here: TestLGHandlerGoldenOutput reads it from server.go
// and fails naming any route no case below reaches.
var lgHandlerGoldenCases = []lgHandlerCase{
	// Static assets. One real file proves the route serves its embedded tree;
	// the miss proves the 404 an operator's browser receives.
	{Name: "assets-ze-svg", Method: http.MethodGet, Target: "/lg/assets/ze.svg"},
	{Name: "assets-missing", Method: http.MethodGet, Target: "/lg/assets/missing.js"},

	// Birdwatcher-compatible JSON API.
	{Name: "api-status", Method: http.MethodGet, Target: "/api/looking-glass/status",
		Rewrites: []golden.Rewrite{lgServerTimeRewrite}},
	{Name: "api-protocols-bgp", Method: http.MethodGet, Target: "/api/looking-glass/protocols/bgp"},
	{Name: "api-protocols-short", Method: http.MethodGet, Target: "/api/looking-glass/protocols/short"},
	{Name: "api-routes-protocol", Method: http.MethodGet, Target: "/api/looking-glass/routes/protocol/peer1"},
	{Name: "api-routes-peer", Method: http.MethodGet, Target: "/api/looking-glass/routes/peer/10.0.0.1"},
	{Name: "api-routes-table", Method: http.MethodGet, Target: "/api/looking-glass/routes/table/ipv4"},
	{Name: "api-routes-filtered", Method: http.MethodGet, Target: "/api/looking-glass/routes/filtered/peer1"},
	{Name: "api-routes-export", Method: http.MethodGet, Target: "/api/looking-glass/routes/export/peer1"},
	{Name: "api-routes-noexport", Method: http.MethodGet, Target: "/api/looking-glass/routes/noexport/peer1"},
	{Name: "api-routes-count", Method: http.MethodGet, Target: "/api/looking-glass/routes/count/protocol/peer1"},
	{Name: "api-routes-prefix", Method: http.MethodGet, Target: "/api/looking-glass/routes/prefix?prefix=10.0.0.0%2F24"},
	{Name: "api-routes-prefix-invalid", Method: http.MethodGet, Target: "/api/looking-glass/routes/prefix"},
	{Name: "api-routes-search", Method: http.MethodGet, Target: "/api/looking-glass/routes/search?prefix=10.0.0.0%2F24"},
	{Name: "api-protocols-bmp", Method: http.MethodGet, Target: "/api/looking-glass/protocols/bmp"},
	{Name: "api-routes-bmp", Method: http.MethodGet, Target: "/api/looking-glass/routes/bmp/peer1"},
	{Name: "api-unknown", Method: http.MethodGet, Target: "/api/looking-glass/nothing-here"},

	// HTML pages. These are the bytes the templ port must keep.
	{Name: "ui-peers", Method: http.MethodGet, Target: "/lg/peers"},
	{Name: "ui-search-form", Method: http.MethodGet, Target: "/lg/search"},
	{Name: "ui-search-result", Method: http.MethodPost, Target: "/lg/search", Form: "prefix=10.0.0.0%2F24&family=ipv4"},
	{Name: "ui-search-empty", Method: http.MethodPost, Target: "/lg/search", Form: "family=ipv4"},
	{Name: "ui-search-invalid", Method: http.MethodPost, Target: "/lg/search", Form: "prefix=not-a-prefix"},
	{Name: "ui-lookup-redirect", Method: http.MethodGet, Target: "/lg/lookup"},
	{Name: "ui-peer-routes", Method: http.MethodGet, Target: "/lg/peer/10.0.0.1"},
	{Name: "ui-peer-download", Method: http.MethodGet, Target: "/lg/peer/10.0.0.1/download"},
	{Name: "ui-route-detail", Method: http.MethodGet, Target: "/lg/route/detail?prefix=10.0.0.0%2F24&peer=10.0.0.1"},
	{Name: "ui-events", Method: http.MethodGet, Target: "/lg/events", Stream: true},
	{Name: "ui-graph-aspath", Method: http.MethodGet, Target: "/lg/graph?prefix=10.0.0.0%2F24"},
	{Name: "ui-graph-nexthop", Method: http.MethodGet, Target: "/lg/graph?prefix=10.0.0.0%2F24&mode=nexthop"},
	{Name: "ui-help", Method: http.MethodGet, Target: "/lg/help"},
	{Name: "ui-root-redirect", Method: http.MethodGet, Target: "/lg/"},
	{Name: "ui-root-unknown", Method: http.MethodGet, Target: "/lg/nothing-here"},
	{Name: "ui-bare-redirect", Method: http.MethodGet, Target: "/lg"},
	{Name: "site-root-redirect", Method: http.MethodGet, Target: "/"},

	// The bearer token gate. bearerAuth sits between the headers and the mux,
	// so both answers are response bytes an operator receives.
	{Name: "gated-peers-authorized", Method: http.MethodGet, Target: "/lg/peers", Auth: "Bearer " + lgToken},
	{Name: "gated-peers-unauthorized", Method: http.MethodGet, Target: "/lg/peers", Gated: true},
}

// TestLGHandlerGoldenOutput captures the response bytes of a real request to
// every looking-glass route.
//
// It is not the template capture with extra steps. TestLGGoldenOutput executes
// one parsed template against data the test writes. This one goes through
// renderPage and renderFragment (render.go) with the view data the handler
// builds, so it covers the composition the templ port rewrites. A port that
// renders each template identically from a differently built page model keeps
// the template capture green and turns this one red.
//
// VALIDATES: every byte the looking glass answers a request with.
// PREVENTS: a rendering change reaching an operator with every test green.
func TestLGHandlerGoldenOutput(t *testing.T) {
	// One server answers which route a request reaches. bearerAuth wraps the
	// mux rather than sitting inside it, so the pattern table is the same with
	// and without a token. Every SERVE builds its own server: see lgGoldenServe.
	routes := newLGGoldenServer(t, "")

	// The route list comes from the source that registers it, so a route added
	// later fails here rather than passing uncaptured.
	live, dynamic := golden.RoutePatterns(t, golden.RepoFile(t, filepath.Join("internal", "component", "lg", "server.go")))
	for _, d := range dynamic {
		t.Errorf("%s registers a route whose pattern is %s, which this capture cannot name", d.Pos, d.Expr)
	}

	names := make([]string, 0, len(lgHandlerGoldenCases))
	for _, c := range lgHandlerGoldenCases {
		names = append(names, c.Name)
	}

	golden.AssertUniqueNames(t, "fixture", "lgHandlerGoldenCases", names)

	reached := make([]string, 0, len(lgHandlerGoldenCases))
	written := make([]string, 0, len(lgHandlerGoldenCases))
	root := filepath.Join("testdata", lgHandlerFixtures)

	for _, c := range lgHandlerGoldenCases {
		_, pattern := routes.mux.Handler(lgGoldenRequest(t, c))
		if pattern == "" {
			t.Errorf("case %q requests %s %s, which no route serves", c.Name, c.Method, c.Target)

			continue
		}

		reached = append(reached, pattern)

		fixture := filepath.Join(root, c.Name+".txt")
		written = append(written, fixture)

		t.Run(c.Name, func(t *testing.T) {
			got := lgGoldenServe(t, c)

			// Each serve runs against its own server, so a second serve of the
			// same request answers what the first one did. Two answers that
			// differ mean a clock, a map order or state this capture cannot
			// pin.
			if again := lgGoldenServe(t, c); !bytes.Equal(got, again) {
				t.Fatalf("%s %s answers two different bodies on two identical requests; the capture cannot pin it",
					c.Method, c.Target)
			}

			golden.Compare(t, fixture, got)
		})
	}

	golden.AssertCoversNames(t, "route", "lgHandlerGoldenCases", live, reached)
	golden.AssertCoversDir(t, root, "lgHandlerGoldenCases", written)
}

// newLGGoldenServer builds a looking glass over the fixed dispatcher. It binds
// no listener: the capture serves through the server's own handler chain, which
// is what carries securityHeaders and bearerAuth.
func newLGGoldenServer(t *testing.T, token string) *LGServer {
	t.Helper()

	srv, err := NewLGServer(LGConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		Dispatch:    mockDispatch(),
		Token:       token,
	})
	if err != nil {
		t.Fatalf("new lg server: %v", err)
	}

	return srv
}

// lgGoldenRequest builds one case's request.
func lgGoldenRequest(t *testing.T, c lgHandlerCase) *http.Request {
	t.Helper()

	var body io.Reader = http.NoBody
	if c.Form != "" {
		body = strings.NewReader(c.Form)
	}

	req := httptest.NewRequest(c.Method, c.Target, body)
	if c.Form != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	if c.Auth != "" {
		req.Header.Set("Authorization", c.Auth)
	}

	if c.Stream {
		// Cancel before the handler runs. A stream writes its headers and its
		// first payload, then returns on ctx.Done instead of holding the test
		// for a tick.
		ctx, cancel := context.WithCancel(req.Context())
		cancel()
		req = req.WithContext(ctx)
	}

	return req
}

// lgGoldenServe builds a server, issues one request and returns the fixture
// bytes. Each call gets its own server. A case therefore reads no state an
// earlier case left, and a filtered run answers what a full run answers.
func lgGoldenServe(t *testing.T, c lgHandlerCase) []byte {
	t.Helper()

	token := ""
	if c.Auth != "" || c.Gated {
		token = lgToken
	}

	srv := newLGGoldenServer(t, token)

	rec := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rec, lgGoldenRequest(t, c))

	// securityHeaders puts the build's version header on every response. The
	// rewrite that drops it therefore belongs to every case, not to a list each
	// case must remember.
	return golden.Response(rec, append([]golden.Rewrite{golden.VersionHeader}, c.Rewrites...))
}
