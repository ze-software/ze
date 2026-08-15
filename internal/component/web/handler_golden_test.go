// Related: render.go -- the wrappers a request renders through
// Related: webroute.go -- RegisteredWebRoutes, half of this capture's route list
// Related: golden_test.go -- the TEMPLATE capture, which bypasses those wrappers

package web

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/test/golden"
)

// webHandlerFixtures is where the captured responses live.
const webHandlerFixtures = "handler"

// webGoldenUser is the operator every authenticated case signs in as.
const (
	webGoldenUser     = "operator"
	webGoldenPassword = "operator-password"
)

// webHandlerCase is one HTTP request and the response bytes it must keep
// producing.
type webHandlerCase struct {
	// Name is the fixture stem.
	Name string
	// Method and Target are the request line.
	Method string
	Target string
	// Form, when set, is the urlencoded body. A mutating request also needs an
	// Origin, which webGoldenRequest supplies.
	Form string
	// HTMX marks a request the browser makes through htmx, which several
	// handlers answer with a fragment instead of a page.
	HTMX bool
	// Cookies carries extra request cookies, the UI mode switch among them.
	Cookies map[string]string
	// Anonymous drops the session cookie, so the capture holds the login page
	// the middleware answers with.
	Anonymous bool
	// ReadOnly sends the request to the server built with an authorizer that
	// refuses configuration edits.
	ReadOnly bool
	// Stream marks a route that writes until the client goes away. The request
	// carries a canceled context, so the capture holds what the handler emits
	// before it returns rather than blocking on a ticker.
	Stream bool
	// Setup publishes whatever the handler looks up before the request runs,
	// and undoes it through t.Cleanup. A handler that answers 503 without a
	// running subsystem captures the 503 alone, and the page behind it stays
	// uncaptured. The data it publishes MUST be fixed: the capture serves the
	// same request twice and compares the two answers.
	Setup func(t *testing.T)
	// Rewrites normalize a volatile span. Each one names its producer.
	Rewrites []golden.Rewrite
}

// The rewrites below are the only values in the web surface that a second run
// changes. Each names the function that produces it. A rewrite blinds the
// capture to what it matches, so each one keeps the markup and drops the value
// alone.
var (
	// buildResourcesData (page_system.go) reads the clock, the goroutine count,
	// the heap, the CPU count and the build's version. writeKV renders each as
	// one table row, so the row label stays and only the cell it labels goes.
	webRuntimeRewrite = golden.Rewrite{
		Pattern: regexp.MustCompile(
			`(<td class="wb-detail-kv-key">(?:Uptime|Goroutines|Memory Allocated|Memory System|GC Runs|Current Time|CPU Cores|GOMAXPROCS|Version)` +
				`</td><td class="wb-detail-kv-val">)[^<]*`),
		Replacement: "${1}<RUNTIME>",
	}
	// buildDashboardSystemPanel (workbench_dashboard.go) reads the same clock
	// plus the host name and its memory. The dashboard renders each as a
	// labeled stat.
	webDashboardRewrite = golden.Rewrite{
		Pattern: regexp.MustCompile(
			`(<span class="wb-dashboard-stat-label">(?:Hostname|Uptime|Version|Built|CPU Count|CPU|Memory Available|Memory Used|Memory)` +
				`</span><span class="wb-dashboard-stat-value">)[^<]*`),
		Replacement: "${1}<RUNTIME>",
	}
	// buildHostHardwareData (page_system.go) reports what host.Detect finds: the
	// CPU model, every core, every NIC and every disk. The section list, the row
	// count, every key, every value and the class on a row all follow the
	// machine. A fixture holding them is red on the next machine, and it says
	// nothing about the rendering.
	//
	// The four rewrites below drop that content in stages, and each stage
	// requires the exact markup buildHostHardwareHTML and writeHardwareKV write
	// around it. A port that renders the panel differently matches none of them,
	// so the machine-derived bytes reach the comparison and this fixture fails.
	// What that markup IS stays pinned byte for byte by TestWebMarkupGoldenOutput
	// (markup_golden_test.go), which renders the same builder over a fixed
	// section list. Neither half covers the panel alone.
	webHardwareRowRewrite = golden.Rewrite{
		Pattern: regexp.MustCompile(
			`<tr(?: class="wb-hardware-[a-z]+")?><td class="wb-detail-kv-key">[^<]*</td>` +
				`<td class="wb-detail-kv-val">[^<]*</td></tr>`),
		Replacement: "<HARDWARE-ROW>",
	}
	webHardwareRowsRewrite = golden.Rewrite{
		Pattern:     regexp.MustCompile(`(?:<HARDWARE-ROW>)+`),
		Replacement: "<HARDWARE-ROWS>",
	}
	webHardwareSectionRewrite = golden.Rewrite{
		Pattern: regexp.MustCompile(
			`<div class="wb-hardware-section"><h3>[^<]*</h3>` +
				`<table class="wb-detail-kv"><HARDWARE-ROWS></table></div>`),
		Replacement: "<HARDWARE-SECTION>",
	}
	webHardwareCountRewrite = golden.Rewrite{
		Pattern:     regexp.MustCompile(`(?:<HARDWARE-SECTION>)+`),
		Replacement: "<HARDWARE-SECTIONS>",
	}
	// health.Registry.Check stamps every component with the time it ran.
	webHealthTimeRewrite = golden.Rewrite{
		Pattern:     regexp.MustCompile(`("checked-at": ")[^"]*"`),
		Replacement: `${1}<TIME>"`,
	}
	// SessionStore.CreateSession draws the token from crypto/rand.
	webSessionCookieRewrite = golden.Rewrite{
		Pattern:     regexp.MustCompile(`(?m)^(header: Set-Cookie: ze-session=)[^;]*`),
		Replacement: "${1}<SESSION>",
	}
)

// webHandlerGoldenCases requests every route the hub serves. The route list is
// NOT written here. TestWebHandlerGoldenOutput reads the registry at run time
// and the hub's own source for the rest. It then fails naming any route no case
// below reaches. webNavCases adds one case per workbench navigation entry, so a
// page added to sections() is captured without an edit here.
var webHandlerGoldenCases = []webHandlerCase{
	// Sign-in. Both answers are pages an operator sees.
	{Name: "post-login-ok", Method: http.MethodPost, Target: "/login",
		Form:     "username=" + webGoldenUser + "&password=" + webGoldenPassword,
		Rewrites: []golden.Rewrite{webSessionCookieRewrite}},
	{Name: "post-login-refused", Method: http.MethodPost, Target: "/login",
		Form: "username=" + webGoldenUser + "&password=wrong"},
	{Name: "get-show-anonymous", Method: http.MethodGet, Target: "/show/", Anonymous: true},
	{Name: "get-fragment-anonymous-htmx", Method: http.MethodGet, Target: "/fragment/detail?path=bgp",
		Anonymous: true, HTMX: true},

	// Static files.
	{Name: "get-assets-ze-svg", Method: http.MethodGet, Target: "/assets/ze.svg"},
	{Name: "get-assets-missing", Method: http.MethodGet, Target: "/assets/missing.js"},
	{Name: "get-favicon", Method: http.MethodGet, Target: "/favicon.ico"},

	// Server-sent events.
	{Name: "get-events", Method: http.MethodGet, Target: "/events", Stream: true},

	// Admin console.
	{Name: "get-admin", Method: http.MethodGet, Target: "/admin/"},
	{Name: "get-admin-subtree", Method: http.MethodGet, Target: "/admin/show/"},
	{Name: "get-admin-htmx", Method: http.MethodGet, Target: "/admin/show/", HTMX: true},
	{Name: "post-admin-execute", Method: http.MethodPost, Target: "/admin/show/version"},
	{Name: "get-admin-read-only", Method: http.MethodGet, Target: "/admin/", ReadOnly: true},

	// CLI surface.
	{Name: "get-cli", Method: http.MethodGet, Target: "/cli"},
	{Name: "post-cli", Method: http.MethodPost, Target: "/cli", Form: "command=show+bgp&path="},
	{Name: "get-cli-complete", Method: http.MethodGet, Target: "/cli/complete?input=sh&path=&mode=operational"},
	{Name: "post-cli-terminal", Method: http.MethodPost, Target: "/cli/terminal",
		Form: "command=show+bgp&path=&mode=operational"},
	{Name: "post-cli-mode", Method: http.MethodPost, Target: "/cli/mode", Form: "mode=configure&path="},

	// The HTMX fragment path, in both UI modes.
	{Name: "get-fragment-detail", Method: http.MethodGet, Target: "/fragment/detail?path=bgp", HTMX: true},
	{Name: "get-fragment-detail-page", Method: http.MethodGet, Target: "/fragment/detail?path=bgp"},

	// Configuration editing.
	{Name: "post-config-set", Method: http.MethodPost, Target: "/config/set/bgp/",
		Form: "leaf=router-id&value=9.9.9.9"},
	{Name: "post-config-form", Method: http.MethodPost, Target: "/config/form/bgp/",
		Form: "leaf=router-id&value=9.9.9.9"},
	{Name: "post-config-add", Method: http.MethodPost, Target: "/config/add/bgp/peer/", Form: "name=gamma"},
	{Name: "get-config-add-form", Method: http.MethodGet, Target: "/config/add-form/bgp/peer/"},
	{Name: "post-config-rename", Method: http.MethodPost, Target: "/config/rename/bgp/peer/alpha/",
		Form: "new-key=renamed"},
	{Name: "get-config-changes", Method: http.MethodGet, Target: "/config/changes"},
	{Name: "post-config-delete", Method: http.MethodPost, Target: "/config/delete/bgp/peer/", Form: "leaf=alpha"},
	{Name: "get-config-diff", Method: http.MethodGet, Target: "/config/diff"},
	{Name: "get-config-diff-close", Method: http.MethodGet, Target: "/config/diff-close"},
	{Name: "post-config-commit", Method: http.MethodPost, Target: "/config/commit"},
	{Name: "post-config-commit-path", Method: http.MethodPost, Target: "/config/commit/bgp/"},
	{Name: "get-config-download", Method: http.MethodGet, Target: "/config/download"},
	{Name: "get-config-download-read-only", Method: http.MethodGet, Target: "/config/download", ReadOnly: true},
	{Name: "post-config-upload", Method: http.MethodPost, Target: "/config/upload",
		Form: "config=" + webUploadedConfig},
	{Name: "post-config-discard", Method: http.MethodPost, Target: "/config/discard"},
	{Name: "post-config-discard-path", Method: http.MethodPost, Target: "/config/discard/bgp/"},

	// Workbench related-tool execution.
	{Name: "post-tools-related-run", Method: http.MethodPost, Target: "/tools/related/run",
		Form: "tool_id=ping&context_path=bgp"},

	// Portal and health.
	{Name: "get-portal", Method: http.MethodGet, Target: "/portal/"},
	{Name: "get-health", Method: http.MethodGet, Target: "/health",
		Rewrites: []golden.Rewrite{webHealthTimeRewrite}},

	// The catch-all's three branches: the root redirect, a /monitor/ path, and
	// a path that is neither /show/ nor /monitor/.
	{Name: "get-root", Method: http.MethodGet, Target: "/"},
	{Name: "get-monitor", Method: http.MethodGet, Target: "/monitor/",
		Rewrites: []golden.Rewrite{webDashboardRewrite}},
	{Name: "get-unrouted", Method: http.MethodGet, Target: "/nothing-here"},

	// The Finder shell, still reachable as a rollback.
	{Name: "get-show-finder", Method: http.MethodGet, Target: "/show/",
		Cookies: map[string]string{"ze-ui-mode": "finder"}},
	{Name: "get-show-bgp-finder", Method: http.MethodGet, Target: "/show/bgp/peer/",
		Cookies: map[string]string{"ze-ui-mode": "finder"}},

	// Feature routes, which register through webroute.go rather than the hub.
	{Name: "get-isis", Method: http.MethodGet, Target: "/isis"},
	{Name: "get-isis-neighbors", Method: http.MethodGet, Target: "/isis/neighbors"},
	{Name: "get-isis-neighbors-stream", Method: http.MethodGet, Target: "/isis/neighbors/stream", Stream: true},
	{Name: "get-isis-database", Method: http.MethodGet, Target: "/isis/database"},
	{Name: "get-isis-database-stream", Method: http.MethodGet, Target: "/isis/database/stream", Stream: true},
	{Name: "get-ospf", Method: http.MethodGet, Target: "/ospf"},
	{Name: "get-ospf-neighbors", Method: http.MethodGet, Target: "/ospf/neighbors"},
	{Name: "get-ospf-neighbors-stream", Method: http.MethodGet, Target: "/ospf/neighbors/stream", Stream: true},
	{Name: "get-ospf-database", Method: http.MethodGet, Target: "/ospf/database"},
	{Name: "get-ospf-database-stream", Method: http.MethodGet, Target: "/ospf/database/stream", Stream: true},
	{Name: "get-ospf-database-opaque", Method: http.MethodGet, Target: "/ospf/database/opaque"},
	{Name: "get-ospf-database-opaque-stream", Method: http.MethodGet, Target: "/ospf/database/opaque/stream",
		Stream: true},
	{Name: "get-ospfv3-database", Method: http.MethodGet, Target: "/ospfv3/database"},
	{Name: "get-ospfv3-database-stream", Method: http.MethodGet, Target: "/ospfv3/database/stream", Stream: true},
	{Name: "get-l2tp", Method: http.MethodGet, Target: "/l2tp"},
	{Name: "get-l2tp-session", Method: http.MethodGet, Target: "/l2tp/1"},
	{Name: "post-l2tp-disconnect", Method: http.MethodPost, Target: "/l2tp/1/disconnect"},
	{Name: "get-l2tp-samples", Method: http.MethodGet, Target: "/l2tp/user1/samples"},
	{Name: "get-l2tp-samples-csv", Method: http.MethodGet, Target: "/l2tp/user1/samples.csv"},
	{Name: "get-l2tp-samples-stream", Method: http.MethodGet, Target: "/l2tp/user1/samples/stream", Stream: true},
	{Name: "get-gokrazy", Method: http.MethodGet, Target: "/gokrazy/"},
}

// webNavRewrites names the navigation pages whose handler reads the machine
// rather than the configuration. The key is the page URL, so a rewrite cannot
// outlive the page it was written for. webNavCases fails when a key names a URL
// the navigation no longer offers.
var webNavRewrites = map[string][]golden.Rewrite{
	"/show/":                  {webDashboardRewrite},
	"/show/system/resources/": {webRuntimeRewrite},
	"/show/system/hardware/": {
		webHardwareRowRewrite, webHardwareRowsRewrite,
		webHardwareSectionRewrite, webHardwareCountRewrite,
	},
}

// webNavCases requests every page the workbench navigation offers. The list
// comes from sections() (workbench_sections.go), so a page added to the
// navigation is captured with no edit to this file.
func webNavCases(t *testing.T) []webHandlerCase {
	t.Helper()

	var cases []webHandlerCase

	offered := make(map[string]bool)

	for _, def := range sections() {
		for _, child := range def.children {
			offered[child.URL] = true

			cases = append(cases, webHandlerCase{
				Name:     webCaseName(child.URL),
				Method:   http.MethodGet,
				Target:   child.URL,
				Rewrites: webNavRewrites[child.URL],
			})
		}
	}

	for url := range webNavRewrites {
		if !offered[url] {
			t.Errorf("webNavRewrites normalizes %s, which the navigation no longer offers", url)
		}
	}

	return cases
}

// webCaseName turns a URL into a fixture stem.
var webCaseNameDrop = regexp.MustCompile(`[^a-z0-9]+`)

func webCaseName(target string) string {
	return "nav-" + strings.Trim(webCaseNameDrop.ReplaceAllString(strings.ToLower(target), "-"), "-")
}

// TestWebHandlerGoldenOutput captures the response bytes of a real request to
// every route the hub serves.
//
// It is not the component capture with extra steps. TestWebGoldenOutput renders
// one component against data the test writes. This one goes through
// RenderLayout, RenderWorkbench, RenderField and every page handler, with the
// view model the handler builds. It therefore covers the composition the other
// capture cannot see. A page that renders each component identically from a
// differently built view model keeps the component capture green and turns this
// one red.
//
// VALIDATES: every byte the web UI answers a request with.
// PREVENTS: a rendering change reaching an operator with every test green.
func TestWebHandlerGoldenOutput(t *testing.T) {
	cases := append(append([]webHandlerCase{}, webHandlerGoldenCases...), webNavCases(t)...)

	names := make([]string, 0, len(cases))
	for _, c := range cases {
		names = append(names, c.Name)
	}

	golden.AssertUniqueNames(t, "fixture", "webHandlerGoldenCases", names)

	env := newWebGoldenEnv(t, false)
	reached := make([]string, 0, len(cases))
	written := make([]string, 0, len(cases))
	root := filepath.Join("testdata", webHandlerFixtures)

	if !golden.Updating() {
		if _, err := os.Stat(root); err != nil {
			t.Fatalf("fixture directory %s is missing; capture it with -update-golden: %v", root, err)
		}
	}

	for _, c := range cases {
		_, pattern := env.mux.Handler(webGoldenRequest(t, env, c))
		if pattern == "" {
			t.Errorf("case %q requests %s %s, which no route serves", c.Name, c.Method, c.Target)

			continue
		}

		reached = append(reached, pattern)

		fixture := filepath.Join(root, c.Name+".txt")
		written = append(written, fixture)

		t.Run(c.Name, func(t *testing.T) {
			if c.Setup != nil {
				c.Setup(t)
			}

			got := webGoldenServe(t, c)

			// Each serve runs against its own editor and its own session, so a
			// second serve of a mutating request answers what the first one
			// did. Two answers that differ mean a clock, a map order or state
			// this capture cannot pin.
			if again := webGoldenServe(t, c); !bytes.Equal(got, again) {
				t.Fatalf("%s %s answers two different bodies on two identical requests; the capture cannot pin it",
					c.Method, c.Target)
			}

			// A byte comparison accepts an empty page, and a capture run
			// freezes it. The component capture has always refused a unit that
			// renders only whitespace: this is the same refusal, one layer up.
			golden.AssertResponseHasBody(t, c.Name, got, c.Stream)

			golden.Compare(t, fixture, got)
		})
	}

	golden.AssertCoversNames(t, "route", "webHandlerGoldenCases", webLiveRoutes(t), reached)
	golden.AssertCoversDir(t, root, "webHandlerGoldenCases", written)
}

// The one dynamic registration this capture accepts in the hub's source. Each
// is read as it is written there, the import alias included: the loop's
// subject, then the pattern it registers.
const (
	webRouteRegistry    = "zeweb.RegisteredWebRoutes()"
	webRoutePatternExpr = "route.Pattern"
)

// webLiveRoutes returns every route the hub serves. Two places decide that set.
// One is the registry the web package fills at init. The other is the literal
// patterns startWebServer registers. Neither is typed into this test, so a
// route added to either one fails the coverage check by name.
func webLiveRoutes(t *testing.T) []string {
	t.Helper()

	hub := golden.RepoFile(t, filepath.Join("cmd", "ze", "hub", "service_web.go"))

	live, dynamic := golden.RoutePatterns(t, hub)

	// startWebServer registers the feature routes from a loop over
	// RegisteredWebRoutes, so their patterns are not literals in that file.
	// That loop is the ONLY registration the source cannot name. Anything else
	// dynamic is a route this capture would miss.
	//
	// What is checked is the SET the loop reads, not the spelling of its pattern
	// expression. A loop repointed at another registry goes on writing
	// route.Pattern. This capture reads RegisteredWebRoutes below, so the two
	// must name one registry. Otherwise the mirror describes a set the hub no
	// longer serves.
	for _, d := range dynamic {
		if d.RangeOver == webRouteRegistry && d.Expr == webRoutePatternExpr {
			continue
		}

		t.Errorf("%s registers a route whose pattern is %s, from a loop over %q; "+
			"this capture mirrors %s and cannot name it", d.Pos, d.Expr, d.RangeOver, webRouteRegistry)
	}

	for _, route := range RegisteredWebRoutes() {
		live = append(live, route.Pattern)
	}

	return live
}

// webGoldenEnv is one server: the mux the hub builds, and a signed-in session.
//
// handler is what a request is served through, and it is not the mux. NewWebServer
// (server.go) puts SecurityHeaders over its mux, so the bytes a browser reads
// carry those headers on every route. mux stays because Handler(req) is how a
// case reports which route pattern it reached.
type webGoldenEnv struct {
	mux     *http.ServeMux
	handler http.Handler
	session string
}

// webGoldenShared holds what every case reuses. The renderer, the schema and
// the committed tree are read-only, so building them once costs one parse
// rather than one per case.
var (
	webGoldenOnce   sync.Once
	webGoldenShared struct {
		renderer *Renderer
		schema   *config.Schema
		tree     *config.Tree
		admin    map[string][]string
		err      error
	}
)

func webGoldenParts(t *testing.T) (*Renderer, *config.Schema, *config.Tree, map[string][]string) {
	t.Helper()

	webGoldenOnce.Do(func() {
		renderer, err := NewRenderer()
		if err != nil {
			webGoldenShared.err = err

			return
		}

		schema, err := config.YANGSchema()
		if err != nil {
			webGoldenShared.err = err

			return
		}

		tree, err := config.ParseTreeWithYANG(webGoldenConfig, nil)
		if err != nil {
			webGoldenShared.err = err

			return
		}

		webGoldenShared.renderer = renderer
		webGoldenShared.schema = schema
		webGoldenShared.tree = tree
		webGoldenShared.admin = AdminTreeFromYANG(webGoldenCommandTree())
	})

	if webGoldenShared.err != nil {
		t.Fatalf("build the shared capture parts: %v", webGoldenShared.err)
	}

	return webGoldenShared.renderer, webGoldenShared.schema, webGoldenShared.tree, webGoldenShared.admin
}

// webGoldenCommandTree is the operational command tree the admin console reads.
// The hub derives it from the merged YANG modules and the registered plugins,
// which no test binary holds. The tree is therefore the input. What the capture
// covers is the markup AdminTreeFromYANG and HandleAdminView build from it.
func webGoldenCommandTree() *command.Node {
	return &command.Node{
		Children: map[string]*command.Node{
			"show": {
				Name: "show",
				Children: map[string]*command.Node{
					"version": {Name: "version", WireMethod: "ze-show:version"},
					"bgp":     {Name: "bgp", WireMethod: "ze-bgp:summary"},
				},
			},
			"peer": {
				Name: "peer",
				Children: map[string]*command.Node{
					"detail": {Name: "detail", WireMethod: "ze-bgp:peer-detail"},
				},
			},
		},
	}
}

// newWebGoldenEnv builds the mux startWebServer builds, over fixed inputs.
//
// It mirrors that function rather than calling it: startWebServer opens TLS
// listeners, needs blob storage and a certificate store, and serves in a
// goroutine. The mirror is held honest by webLiveRoutes, which reads the route
// list from that file and fails when the two drift.
func newWebGoldenEnv(t *testing.T, readOnly bool) *webGoldenEnv {
	t.Helper()

	renderer, schema, tree, adminChildren := webGoldenParts(t)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "ze.conf")

	if err := os.WriteFile(configPath, []byte(webGoldenConfig), 0o600); err != nil {
		t.Fatalf("write the capture config: %v", err)
	}

	store := storage.NewFilesystem()
	editorMgr := NewEditorManager(store, configPath, schema, testEditorFactory(), testEditSessionFactory())
	broker := NewEventBroker(0)
	dispatch := webGoldenDispatch()

	users := []authz.UserConfig{{Name: webGoldenUser, Hash: webGoldenHash(t)}}
	localUsers := func() ([]authz.UserConfig, error) { return users, nil }
	sessionStore := NewSessionStore(localUsers)
	authenticator := &authz.LocalAuthenticator{UsersFunc: localUsers}

	loginRenderer := func(w http.ResponseWriter, r *http.Request) {
		data := LoginData{ReturnTo: r.URL.RequestURI(), Locale: LocaleFromRequest(r)}
		if err := renderer.RenderLogin(w, data); err != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
		}
	}

	var authorizer aaaAuthorizer
	if readOnly {
		authorizer = webGoldenDenyEdits{}
	}

	authWrap := func(h http.Handler) http.Handler {
		return AuthMiddlewareWithAudit(sessionStore, authenticator, loginRenderer, h, nil)
	}
	mutationWrap := func(h http.Handler) http.Handler { return authWrap(RequireSameOrigin(h)) }
	editWrap := func(h http.Handler) http.Handler { return authWrap(RequireEditAuthz(authorizer, h)) }
	editMutationWrap := func(h http.Handler) http.Handler {
		return authWrap(RequireEditAuthz(authorizer, RequireSameOrigin(h)))
	}

	// The startup UI mode is fixed rather than read from the environment, so a
	// developer who sets ze.web.ui-mode cannot rewrite every page fixture.
	const uiMode = UIModeWorkbench

	finderHandler := HandleFragment(renderer, schema, tree, editorMgr, false)
	workbenchHandler := HandleWorkbench(renderer, schema, tree, editorMgr, false,
		WithDispatch(dispatch), WithBroker(broker), WithPowerUsers([]string{webGoldenUser}),
		WithAuthorizer(authorizer))
	showHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ReadUIModeFromRequest(r, uiMode) == UIModeWorkbench {
			workbenchHandler(w, r)

			return
		}

		finderHandler(w, r)
	})

	mux := http.NewServeMux()
	mux.HandleFunc("POST /login", LoginHandlerWithAudit(sessionStore, authenticator, loginRenderer, nil))
	mux.Handle("/assets/", http.StripPrefix("/assets/", renderer.AssetHandler()))
	mux.Handle("GET /favicon.ico", renderer.FaviconHandler())
	mux.Handle("/events", authWrap(broker))
	mux.Handle("/admin/", editMutationWrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			HandleAdminExecute(renderer, dispatch)(w, r)

			return
		}

		HandleAdminView(renderer, adminChildren)(w, r)
	})))
	mux.Handle("GET /cli", authWrap(HandleCLIPageHTTP(renderer, false)))
	mux.Handle("POST /cli", mutationWrap(HandleCLICommandWithAuthorizer(editorMgr, schema, renderer, authorizer)))
	mux.Handle("/cli/complete", authWrap(HandleCLICompleteWithCommandCompleter(
		cli.NewCompleter(), cli.NewCommandCompleter(webGoldenCommandTree()), editorMgr, schema)))
	mux.Handle("POST /cli/terminal", mutationWrap(HandleCLITerminalWithDispatchAuthorizerAndAudit(
		editorMgr, schema, tree, dispatch, authorizer, nil)))
	mux.Handle("POST /cli/mode", mutationWrap(HandleCLIModeToggle(editorMgr, schema, renderer)))
	mux.Handle("/fragment/detail", authWrap(finderHandler))
	mux.Handle("POST /config/set/", mutationWrap(HandleConfigSetWithAuthorizer(editorMgr, schema, renderer, authorizer)))
	mux.Handle("POST /config/form/", mutationWrap(HandleConfigFormWithAuthorizer(editorMgr, schema, renderer, authorizer)))
	mux.Handle("POST /config/add/", mutationWrap(HandleConfigAddWithAuthorizer(editorMgr, schema, renderer, authorizer)))
	mux.Handle("GET /config/add-form/", editWrap(HandleConfigAddForm(editorMgr, schema, renderer)))
	mux.Handle("POST /config/rename/", mutationWrap(HandleConfigRenameWithAuthorizer(editorMgr, schema, authorizer)))
	mux.Handle("GET /config/changes", authWrap(HandleConfigChanges(editorMgr, renderer, authorizer)))
	mux.Handle("POST /config/delete/", mutationWrap(HandleConfigDeleteWithAuthorizer(editorMgr, authorizer)))
	mux.Handle("/config/diff", authWrap(webGoldenDiffHandler(renderer, editorMgr)))
	mux.Handle("/config/diff-close", authWrap(webGoldenDiffCloseHandler(renderer)))
	mux.Handle("/config/commit", mutationWrap(HandleConfigCommitWithAuthorizerAndAudit(
		editorMgr, renderer, broker, authorizer, nil)))
	mux.Handle("/config/commit/", mutationWrap(HandleConfigCommitWithAuthorizerAndAudit(
		editorMgr, renderer, broker, authorizer, nil)))
	mux.Handle("GET /config/download", editWrap(HandleConfigDownload(editorMgr, nil)))
	mux.Handle("POST /config/upload", editMutationWrap(HandleConfigUpload(
		editorMgr, webGoldenValidate, configPath, authorizer, nil)))
	mux.Handle("POST /config/discard", mutationWrap(HandleConfigDiscardWithAuthorizerAndAudit(editorMgr, authorizer, nil)))
	mux.Handle("POST /config/discard/", mutationWrap(HandleConfigDiscardWithAuthorizerAndAudit(editorMgr, authorizer, nil)))
	mux.Handle("POST /tools/related/run", mutationWrap(HandleRelatedToolRun(renderer, schema, tree, editorMgr, dispatch)))

	// Every registered route is wired, its Enabled gate included. The hub skips
	// a gated route. This capture must not: a route would leave the capture
	// with nothing failing.
	routeWraps := map[WrapKind]func(http.Handler) http.Handler{
		WrapAuth:     authWrap,
		WrapMutation: mutationWrap,
	}
	routeDeps := RouteDeps{Renderer: renderer, Dispatch: dispatch}

	for _, route := range RegisteredWebRoutes() {
		mux.Handle(route.Pattern, routeWraps[route.Wrap](route.Build(routeDeps)))
	}

	mux.Handle("/portal/", authWrap(HandlePortal(renderer, uiMode)))
	mux.Handle("GET /health", authWrap(webGoldenHealthHandler()))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/show/", http.StatusFound)

			return
		}

		if strings.HasPrefix(r.URL.Path, "/show/") || strings.HasPrefix(r.URL.Path, "/monitor/") {
			authWrap(showHandler).ServeHTTP(w, r)

			return
		}

		authWrap(finderHandler).ServeHTTP(w, r)
	})

	session, err := sessionStore.CreateSession(webGoldenUser, authz.AuthResult{Authenticated: true})
	if err != nil {
		t.Fatalf("create the capture session: %v", err)
	}

	return &webGoldenEnv{mux: mux, handler: SecurityHeaders(mux), session: session.Token}
}

// webGoldenRequest builds one case's request.
func webGoldenRequest(t *testing.T, env *webGoldenEnv, c webHandlerCase) *http.Request {
	t.Helper()

	var body io.Reader = http.NoBody
	if c.Form != "" {
		body = strings.NewReader(c.Form)
	}

	req := httptest.NewRequest(c.Method, c.Target, body)
	if c.Form != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	// RequireSameOrigin refuses a mutating request whose Origin does not match
	// the host, so every mutating case carries the one the browser sends.
	req.Header.Set("Origin", "https://"+req.Host)

	if c.HTMX {
		req.Header.Set("HX-Request", "true")
	}

	if !c.Anonymous {
		req.AddCookie(&http.Cookie{Name: "ze-session", Value: env.session})
	}

	for name, value := range c.Cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}

	if c.Stream {
		// Cancel before the handler runs, so it returns on ctx.Done instead of
		// holding the test for a tick. The capture then holds the headers and
		// whatever the handler wrote before it returned. For the three stream
		// routes here that is nothing, which is why Stream also exempts a case
		// from AssertResponseHasBody.
		ctx, cancel := context.WithCancel(req.Context())
		cancel()
		req = req.WithContext(ctx)
	}

	return req
}

// webGoldenServe builds a server, issues one request and returns the fixture
// bytes. Each call gets its own editor and its own session. A mutating case
// therefore answers on a fresh server, not on the state a previous case left.
func webGoldenServe(t *testing.T, c webHandlerCase) []byte {
	t.Helper()

	env := newWebGoldenEnv(t, c.ReadOnly)

	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, webGoldenRequest(t, env, c))

	// addSecurityHeaders puts the build's version header on every authenticated
	// response. The rewrite that drops it therefore belongs to every case, not
	// to a list each case must remember.
	return golden.Response(rec, append([]golden.Rewrite{golden.VersionHeader}, c.Rewrites...))
}
