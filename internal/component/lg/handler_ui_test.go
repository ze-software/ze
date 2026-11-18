package lg

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// testServer returns a minimal LGServer for unit tests (no decorator).
func testServer() *LGServer {
	return &LGServer{}
}

// testServerWithDecorator returns an LGServer with an ASN decorator for testing.
func testServerWithDecorator(fn ASNDecorator) *LGServer {
	return &LGServer{decorateASN: fn}
}

// errTestEngineDown is what the stubbed dispatcher below fails with.
var errTestEngineDown = errors.New("engine is not answering")

// testServerWithDispatch returns an LGServer whose engine answers as fn says.
func testServerWithDispatch(fn CommandDispatcher) *LGServer {
	return &LGServer{dispatch: fn, logger: slogutil.DiscardLogger()}
}

// TestEngineUnavailableReachesTheBrowser drives the producer of one stream
// message for both of its answers.
//
// VALIDATES: AC-12 -- an engine that cannot answer is reported to a watching
// operator, in the ONE unnamed message the stream carries. The browser end of
// the same path is test/web/lg-stream-error.wb, which reads the row out of a
// real DOM; this pins what the server puts on the wire.
// PREVENTS: the silent half of the htmx 4 cutover. The reason used to travel as
// a second NAMED event (`peer-error`) that nothing consumed, and a dispatch
// error took neither branch: it parses as JSON, so the stream pushed an empty
// table and said nothing at all.
func TestEngineUnavailableReachesTheBrowser(t *testing.T) {
	t.Parallel()

	t.Run("dispatch fails", func(t *testing.T) {
		t.Parallel()

		srv := testServerWithDispatch(func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
			return nil, errTestEngineDown
		})

		body, err := srv.peerStreamBody()
		if err != nil {
			t.Fatalf("peerStreamBody: %v", err)
		}

		if !strings.Contains(body, `class="stream-error"`) {
			t.Errorf("body carries no error row: %s", body)
		}

		if want := cmdBGPOverview + ": " + failureEngineError; !strings.Contains(body, want) {
			t.Errorf("body does not name the failure %q: %s", want, body)
		}

		// The stream is public: the dispatcher's own error stays in the log.
		if strings.Contains(body, errTestEngineDown.Error()) {
			t.Errorf("body carries the dispatcher's own error: %s", body)
		}

		var out strings.Builder
		if writeErr := writeStreamMessage(&out, body); writeErr != nil {
			t.Fatalf("writeStreamMessage: %v", writeErr)
		}

		// An event NAME is the failure this test exists for: htmx 4 dispatches
		// a named message as a DOM event and swaps nothing.
		if strings.Contains(out.String(), "event:") {
			t.Errorf("the message carries an event name: %q", out.String())
		}

		if !strings.HasPrefix(out.String(), "data: ") || !strings.HasSuffix(out.String(), "\n\n") {
			t.Errorf("the message is not one framed SSE message: %q", out.String())
		}
	})

	t.Run("engine answers", func(t *testing.T) {
		t.Parallel()

		srv := testServerWithDispatch(func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"peers":[{"address":"192.0.2.1","state":"established"}]}`)), nil
		})

		body, err := srv.peerStreamBody()
		if err != nil {
			t.Fatalf("peerStreamBody: %v", err)
		}

		if strings.Contains(body, `class="stream-error"`) {
			t.Errorf("a healthy engine produced an error row: %s", body)
		}

		if !strings.Contains(body, "192.0.2.1") {
			t.Errorf("body carries no peer row: %s", body)
		}
	})
}

func TestExtractPeers(t *testing.T) {
	// VALIDATES: peer data extraction from engine JSON.
	// PREVENTS: nil panic or missing fields in template data.
	srv := testServer()

	t.Run("nil input", func(t *testing.T) {
		if got := srv.extractPeers(nil); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("empty peers array", func(t *testing.T) {
		ze := map[string]any{"peers": []any{}}
		got := srv.extractPeers(ze)
		if len(got) != 0 {
			t.Errorf("expected 0 peers, got %d", len(got))
		}
	})

	t.Run("valid peer", func(t *testing.T) {
		ze := map[string]any{
			"peers": []any{
				map[string]any{
					"peer-address":    "10.0.0.1",
					"remote-as":       "65001",
					"state":           "established",
					"uptime":          "3600",
					"routes-received": "100",
					"routes-accepted": "95",
					"routes-sent":     "50",
					"description":     "test",
					"name":            "peer1",
				},
			},
		}

		peers := srv.extractPeers(ze)
		if len(peers) != 1 {
			t.Fatalf("expected 1 peer, got %d", len(peers))
		}

		want := peerRow{
			Address:        "10.0.0.1",
			RemoteAS:       "65001",
			State:          "established",
			Uptime:         "3600",
			RoutesReceived: "100",
			RoutesAccepted: "95",
			RoutesSent:     "50",
			Description:    "test",
		}
		if peers[0] != want {
			t.Errorf("peer = %+v, want %+v", peers[0], want)
		}
	})

	t.Run("real summary record", func(t *testing.T) {
		// Real summary handler answers the aggregates and the rows as siblings.
		ze := map[string]any{
			"router-id":         "10.0.0.1",
			"local-as":          float64(65000),
			"peers-configured":  float64(1),
			"peers-established": float64(1),
			"peers": []any{
				map[string]any{
					"address":   "10.0.0.2",
					"remote-as": float64(65001),
					"state":     "established",
					"uptime":    "1h0m0s",
					"name":      "peer1",
				},
			},
		}
		peers := srv.extractPeers(ze)
		if len(peers) != 1 {
			t.Fatalf("expected 1 peer, got %d", len(peers))
		}
		if peers[0].Address != "10.0.0.2" {
			t.Errorf("Address = %q, want 10.0.0.2", peers[0].Address)
		}
		if peers[0].RemoteAS != "65001" {
			t.Errorf("RemoteAS = %q, want 65001", peers[0].RemoteAS)
		}
	})

	t.Run("non-map entry skipped", func(t *testing.T) {
		ze := map[string]any{
			"peers": []any{"not-a-map", map[string]any{"peer-address": "10.0.0.1"}},
		}
		peers := srv.extractPeers(ze)
		if len(peers) != 1 {
			t.Errorf("expected 1 peer (skip non-map), got %d", len(peers))
		}
	})
}

func TestExtractPeersWithASNNames(t *testing.T) {
	// VALIDATES: AC-2 -- ASN names populated from decorator.
	// PREVENTS: empty RemoteASName when decorator is wired.
	srv := testServerWithDecorator(func(asn string) string {
		if asn == "65001" {
			return "Test Org"
		}
		return ""
	})

	ze := map[string]any{
		"peers": []any{
			map[string]any{
				"peer-address": "10.0.0.1",
				"remote-as":    "65001",
				"state":        "established",
			},
		},
	}

	peers := srv.extractPeers(ze)
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}

	if peers[0].RemoteASName != "Test Org" {
		t.Errorf("RemoteASName = %q, want %q", peers[0].RemoteASName, "Test Org")
	}
}

func TestExtractRoutes(t *testing.T) {
	// VALIDATES: route extraction with routes/prefixes/adj-rib-in formats.
	t.Run("nil input", func(t *testing.T) {
		if got := extractRoutes(nil); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("routes key", func(t *testing.T) {
		ze := map[string]any{"routes": []any{"r1", "r2"}}
		got := extractRoutes(ze)
		if len(got) != 2 {
			t.Errorf("expected 2 routes, got %d", len(got))
		}
	})

	t.Run("prefixes fallback", func(t *testing.T) {
		ze := map[string]any{"prefixes": []any{"p1"}}
		got := extractRoutes(ze)
		if len(got) != 1 {
			t.Errorf("expected 1 route from prefixes fallback, got %d", len(got))
		}
	})

	t.Run("adj-rib-in format", func(t *testing.T) {
		ze := map[string]any{
			"adj-rib-in": map[string]any{
				"10.0.0.1": []any{
					map[string]any{"prefix": "192.168.0.0/24", "family": "ipv4/unicast"},
					map[string]any{"prefix": "192.168.1.0/24", "family": "ipv4/unicast"},
				},
			},
		}
		got := extractRoutes(ze)
		if len(got) != 2 {
			t.Fatalf("expected 2 routes, got %d", len(got))
		}
		r0, _ := got[0].(map[string]any)
		if r0["peer-address"] != "10.0.0.1" {
			t.Errorf("peer-address = %q, want 10.0.0.1", r0["peer-address"])
		}
	})

	t.Run("adj-rib-in and adj-rib-out combined", func(t *testing.T) {
		ze := map[string]any{
			"adj-rib-in": map[string]any{
				"10.0.0.1": []any{
					map[string]any{"prefix": "192.168.0.0/24"},
				},
			},
			"adj-rib-out": map[string]any{
				"10.0.0.1": []any{
					map[string]any{"prefix": "10.0.0.0/8"},
				},
			},
		}
		got := extractRoutes(ze)
		if len(got) != 2 {
			t.Errorf("expected 2 routes (in + out), got %d", len(got))
		}
	})

	t.Run("adj-rib-in preserves existing peer-address", func(t *testing.T) {
		ze := map[string]any{
			"adj-rib-in": map[string]any{
				"10.0.0.1": []any{
					map[string]any{"prefix": "192.168.0.0/24", "peer-address": "10.0.0.1"},
				},
			},
		}
		got := extractRoutes(ze)
		if len(got) != 1 {
			t.Fatalf("expected 1 route, got %d", len(got))
		}
		r0, _ := got[0].(map[string]any)
		if r0["peer-address"] != "10.0.0.1" {
			t.Errorf("peer-address = %q, want 10.0.0.1", r0["peer-address"])
		}
	})
}

func TestFindPeer(t *testing.T) {
	// VALIDATES: peer lookup by address and name.
	ze := map[string]any{
		"peers": []any{
			map[string]any{"peer-address": "10.0.0.1", "name": "peer1"},
			map[string]any{"peer-address": "10.0.0.2", "name": "peer2"},
		},
	}

	t.Run("nil input", func(t *testing.T) {
		if got := findPeer(nil, "10.0.0.1"); got != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("find by address", func(t *testing.T) {
		got := findPeer(ze, "10.0.0.2")
		if got == nil || getStr(got, "name") != "peer2" {
			t.Error("expected peer2")
		}
	})

	t.Run("find by name", func(t *testing.T) {
		got := findPeer(ze, "peer1")
		if got == nil || getStr(got, "peer-address") != "10.0.0.1" {
			t.Error("expected 10.0.0.1")
		}
	})

	t.Run("not found", func(t *testing.T) {
		if got := findPeer(ze, "nonexistent"); got != nil {
			t.Error("expected nil for missing peer")
		}
	})

	t.Run("real summary record with address field", func(t *testing.T) {
		ze := map[string]any{
			"peers": []any{
				map[string]any{"address": "10.0.0.3", "name": "peer3"},
			},
		}
		got := findPeer(ze, "10.0.0.3")
		if got == nil || getStr(got, "name") != "peer3" {
			t.Error("expected peer3")
		}
	})
}

// TestAsk proves ask tells each of the four non-answers apart from an answer,
// and from each other, by driving it over dispatchers that produce each one.
//
// VALIDATES: a failed dispatch is 502 "engine error", an error envelope is 502
// "engine error" marked as one and carrying its refusal, an empty reply is 503
// "engine unavailable", a reply that is not JSON is 503 "unreadable reply",
// each message is exactly the three-line operator error (the command and the
// class, the reason for that class, the fix naming the log) and never the
// dispatcher's own error, and an answer is returned with no failure.
// PREVENTS: a failure reaching a handler as data it then renders, and a
// transport error reaching an unauthenticated client.
func TestAsk(t *testing.T) {
	answering := func(out string) CommandDispatcher {
		return func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
			if out == "" {
				return plugin.NewResponse(plugin.StatusDone, nil), nil
			}
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(out)), nil
		}
	}
	failing := func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
		return nil, errTestEngineDown
	}

	// message is the whole operator error, written out from literals
	// (wantEngineRefusal), never from engineRefusal.
	engineError := wantEngineRefusal("show bgp", "engine error", wantWhyEngineError)
	tests := []struct {
		name     string
		dispatch CommandDispatcher
		status   int
		class    string
		envelope bool
		refusal  string
		message  string
	}{
		{"dispatch failure", failing, http.StatusBadGateway, failureEngineError, false, "", engineError},
		{"error envelope", answering(`{"error":"refused"}`), http.StatusBadGateway, failureEngineError, true, "refused", engineError},
		{
			"empty reply", answering(""), http.StatusServiceUnavailable, failureEngineUnavailable, false, "",
			wantEngineRefusal("show bgp", "engine unavailable", wantWhyEngineUnavailable),
		},
		// `null` is the one non-object reply the boundary still lets through
		// (plugin.RawJSON passes it for a command with no data); a scalar is
		// refused there, so it arrives as a failed dispatch.
		{
			"not an object", answering(`null`), http.StatusServiceUnavailable, failureUnreadableReply, false, "",
			wantEngineRefusal("show bgp", "unreadable reply", wantWhyUnreadableReply),
		},
		{"scalar refused at the boundary", answering(`42`), http.StatusBadGateway, failureEngineError, false, "", engineError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := testServerWithDispatch(tt.dispatch)
			ze, failure := srv.ask("show bgp")
			if ze != nil {
				t.Errorf("answer = %v, want none", ze)
			}
			if failure == nil {
				t.Fatal("failure = nil, want one")
			}
			if failure.status != tt.status || failure.class != tt.class {
				t.Errorf("failure = %+v, want status %d class %q", *failure, tt.status, tt.class)
			}
			if failure.envelope != tt.envelope || failure.refusal != tt.refusal {
				t.Errorf("failure = %+v, want envelope %v refusal %q", *failure, tt.envelope, tt.refusal)
			}
			if got, want := failure.message(), tt.message; got != want {
				t.Errorf("message = %q, want %q", got, want)
			}
		})
	}

	t.Run("answer", func(t *testing.T) {
		srv := testServerWithDispatch(answering(`{"router-id":"1.2.3.4"}`))
		ze, failure := srv.ask("show bgp")
		if failure != nil {
			t.Fatalf("failure = %+v, want none", *failure)
		}
		if getStr(ze, "router-id") != "1.2.3.4" {
			t.Errorf("answer = %v, want router-id 1.2.3.4", ze)
		}
		if got := failure.message(); got != "" {
			t.Errorf("message of no failure = %q, want empty", got)
		}
	})
}

func TestResolveASN(t *testing.T) {
	// VALIDATES: resolveASN handles nil decorator and empty input.
	t.Run("nil decorator", func(t *testing.T) {
		srv := testServer()
		if got := srv.resolveASN("65001"); got != "" {
			t.Errorf("nil decorator: got %q, want empty", got)
		}
	})

	t.Run("empty asn", func(t *testing.T) {
		srv := testServerWithDecorator(func(asn string) string { return "should not be called" })
		if got := srv.resolveASN(""); got != "" {
			t.Errorf("empty asn: got %q, want empty", got)
		}
	})

	t.Run("decorator returns name", func(t *testing.T) {
		srv := testServerWithDecorator(func(asn string) string {
			if asn == "65001" {
				return "Acme Corp"
			}
			return ""
		})
		if got := srv.resolveASN("65001"); got != "Acme Corp" {
			t.Errorf("got %q, want %q", got, "Acme Corp")
		}
	})

	t.Run("decorator returns empty", func(t *testing.T) {
		srv := testServerWithDecorator(func(string) string { return "" })
		if got := srv.resolveASN("99999"); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestDecorateGraphNodes(t *testing.T) {
	// VALIDATES: AC-6 -- graph node names populated from decorator.
	// PREVENTS: empty node labels when decorator is wired.
	srv := testServerWithDecorator(func(asn string) string {
		if asn == "65001" {
			return "Acme Corp"
		}
		return ""
	})

	g := &Graph{
		Nodes: []GraphNode{
			{ASN: 65001, Layer: 1},
			{ASN: 65002, Layer: 0},
		},
	}

	srv.decorateGraphNodes(g)

	if g.Nodes[0].Name != "Acme Corp" {
		t.Errorf("Node[0].Name = %q, want %q", g.Nodes[0].Name, "Acme Corp")
	}
	if g.Nodes[1].Name != "" {
		t.Errorf("Node[1].Name = %q, want empty", g.Nodes[1].Name)
	}
}

func TestExtractPeersSortOrder(t *testing.T) {
	// VALIDATES: peers sorted by IP address numerically, not lexicographically.
	srv := testServer()
	ze := map[string]any{
		"peers": []any{
			map[string]any{"peer-address": "10.0.0.10", "state": "established"},
			map[string]any{"peer-address": "10.0.0.2", "state": "established"},
			map[string]any{"peer-address": "10.0.0.1", "state": "established"},
		},
	}

	peers := srv.extractPeers(ze)
	if len(peers) != 3 {
		t.Fatalf("expected 3 peers, got %d", len(peers))
	}

	want := []string{"10.0.0.1", "10.0.0.2", "10.0.0.10"}
	for i, w := range want {
		if peers[i].Address != w {
			t.Errorf("peers[%d].Address = %q, want %q", i, peers[i].Address, w)
		}
	}
}

func TestRouteRowsBest(t *testing.T) {
	// VALIDATES: routeRows reads the "best" flag out of the decoded RIB, which
	// is what marks a row with the star and the best-route class.
	// PREVENTS: a route map with no "best" key rendering as the best path.
	tests := []struct {
		name string
		in   any
		want bool
	}{
		{"best true", map[string]any{"best": true, "prefix": "10.0.0.0/24"}, true},
		{"best false", map[string]any{"best": false, "prefix": "10.0.0.0/24"}, false},
		{"missing best", map[string]any{"prefix": "10.0.0.0/24"}, false},
		{"wrapped value", map[string]any{"best": map[string]any{"value": true}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := routeRows([]any{tt.in})
			if len(rows) != 1 {
				t.Fatalf("routeRows returned %d rows, want 1", len(rows))
			}
			if rows[0].Best != tt.want {
				t.Errorf("Best = %v, want %v", rows[0].Best, tt.want)
			}
		})
	}

	t.Run("non-map skipped", func(t *testing.T) {
		if rows := routeRows([]any{"not-a-map"}); len(rows) != 0 {
			t.Errorf("routeRows kept %d rows for a non-map entry, want 0", len(rows))
		}
	})
}

// largeASN is a 4-byte ASN from the private range (RFC 6996). Its float64 form
// prints in exponent notation under the %v verb. That makes it the
// discriminating input for both scalar renderers below.
const largeASN = 4200000000

// TestScalarStringRendersALargeASN pins the digits of a decoded JSON number.
//
// VALIDATES: scalarString prints 4200000000, not 4.2e+09. encoding/json decodes
// every number into a float64, and the %v verb renders a float64 with %g, which
// gives exponent form for this magnitude.
// PREVENTS: an operator reading "4.2e+09" where an AS number belongs. Every
// fixture in this package holds an ASN of 65002 or less. The two forms agree
// there, so no golden capture can see this.
func TestScalarStringRendersALargeASN(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"large ASN", float64(largeASN), "4200000000"},
		{"small ASN", float64(65002), "65002"},
		{"fractional", float64(1.5), "1.5"},
		{"string", "64500", "64500"},
		{"int", 64500, "64500"},
		{"bool", true, "true"},
		{"nil", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scalarString(tt.in); got != tt.want {
				t.Errorf("scalarString(%#v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestASPathRendersALargeASN pins the same digits on both AS-path paths.
//
// VALIDATES: the browser column (scalarList, through routeRows) and the CSV
// download (formatASPathPlain) render a 4-byte ASN identically.
// PREVENTS: one of the two paths keeping fmt.Sprint on the decoded float64.
// The CSV path did until 2026-08-14, after the browser path was fixed.
func TestASPathRendersALargeASN(t *testing.T) {
	route := map[string]any{
		"prefix":  "10.0.0.0/24",
		"as-path": []any{float64(largeASN), float64(65002)},
	}

	t.Run("browser column", func(t *testing.T) {
		rows := routeRows([]any{route})
		if len(rows) != 1 {
			t.Fatalf("routeRows returned %d rows, want 1", len(rows))
		}

		want := []string{"4200000000", "65002"}
		if len(rows[0].ASPath) != len(want) {
			t.Fatalf("ASPath = %v, want %v", rows[0].ASPath, want)
		}

		for i, w := range want {
			if rows[0].ASPath[i] != w {
				t.Errorf("ASPath[%d] = %q, want %q", i, rows[0].ASPath[i], w)
			}
		}
	})

	t.Run("csv download", func(t *testing.T) {
		if got, want := formatASPathPlain(route), "4200000000 65002"; got != want {
			t.Errorf("formatASPathPlain = %q, want %q", got, want)
		}
	})

	t.Run("csv keeps a string as it is", func(t *testing.T) {
		plain := map[string]any{"as-path": "64500 65002"}
		if got, want := formatASPathPlain(plain), "64500 65002"; got != want {
			t.Errorf("formatASPathPlain = %q, want %q", got, want)
		}
	})
}

// TestSearchSendsCoveringForBareAddress proves the search page asks bgp-rib
// the question its prefix field holds.
//
// VALIDATES: a bare address is sent as `covering <address>`, ahead of the
// other filters, and the page lists the route whose prefix contains it; a
// prefix with its length is sent as `exact <prefix>`, its host bits cleared,
// with the IPv4 length bound accepted at 32.
// PREVENTS: a host address sent as `prefix <address>`, a text match that
// finds only a /32 route and never the prefix covering it, and a prefix sent
// as that text match, which lists 192.0.0.0/24 for 192.0.0.0/2
// (spec-lg-search-matches-what-the-pages-promise AC-1, AC-2).
func TestSearchSendsCoveringForBareAddress(t *testing.T) {
	cases := []struct {
		name, form, command string
	}{
		{"bare address", "prefix=10.0.0.1", "show bgp rib received covering 10.0.0.1"},
		{"bare address with a path filter", "prefix=10.0.0.1&aspath=65001", "show bgp rib received covering 10.0.0.1 path 65001"},
		{"prefix with its length", "prefix=10.0.0.0/24", "show bgp rib received exact 10.0.0.0/24"},
		{"prefix with host bits set", "prefix=10.0.0.1/24", "show bgp rib received exact 10.0.0.0/24"},
		{"ipv4 last valid length", "prefix=10.0.0.0/32", "show bgp rib received exact 10.0.0.0/32"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, commands := lookupRequest(t, "/lg/search", tc.form)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
			}
			rib := ribCommands(commands)
			if len(rib) != 1 || rib[0] != tc.command {
				t.Fatalf("rib commands = %q, want [%q]", rib, tc.command)
			}
		})
	}

	// The fake answers covering with the plugin's meaning, so the route
	// 10.0.0.0/24 is listed for an address inside it.
	rec, _ := lookupRequest(t, "/lg/search", "prefix=10.0.0.1")
	if !strings.Contains(rec.Body.String(), "<td>10.0.0.0/24</td>") {
		t.Errorf("search for 10.0.0.1 does not list 10.0.0.0/24: %s", rec.Body.String())
	}
}

// TestSearchRefusesAnInvalidPrefix proves a prefix field that is neither an
// address nor a prefix is refused on the page and never reaches bgp-rib.
//
// VALIDATES: an IPv4 length of 33 and a word are each answered by the error
// banner naming the value, the reason and a valid form, and no `show bgp rib`
// command is sent.
func TestSearchRefusesAnInvalidPrefix(t *testing.T) {
	cases := []struct {
		name, form, value, why string
	}{
		{"ipv4 length past 32", "prefix=10.0.0.0/33", "10.0.0.0/33", "an IPv4 prefix length is 0 to 32"},
		{"not an address", "prefix=not-a-prefix", "not-a-prefix", "the field takes an address"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, commands := lookupRequest(t, "/lg/search", tc.form)
			if rib := ribCommands(commands); len(rib) != 0 {
				t.Fatalf("an invalid prefix reached bgp-rib: %q", rib)
			}
			body := rec.Body.String()
			if !strings.Contains(body, `class="error-banner">prefix `+tc.value) {
				t.Errorf("no banner naming %q: %s", tc.value, body)
			}
			if !strings.Contains(body, tc.why) {
				t.Errorf("banner does not say %q: %s", tc.why, body)
			}
		})
	}
}

// TestSearchSendsEnginePathGrammar proves the search page sends an AS path in
// the grammar bgp-rib reads, and that the page lists what that grammar
// matches.
//
// VALIDATES: AS numbers typed with spaces, with commas, or with both, are sent
// after `path` as the comma form matchASPath reads, a leading ^ kept; the
// fake, matching as matchASPath does, lists the route whose path is 65001
// then 65002 for the ordered run and for the anchored run, and leaves it out
// for the reverse order and for a run anchored at an AS later in the path;
// the largest AS number, 4294967295, is sent.
// PREVENTS: a spaced path sent as several tokens, which bgp-rib refuses as
// an unknown keyword (spec-lg-search-matches-what-the-pages-promise AC-8,
// AC-9).
func TestSearchSendsEnginePathGrammar(t *testing.T) {
	cases := []struct {
		name, aspath, command string
		listed                bool
	}{
		{"two AS numbers with a space", "65001 65002", "show bgp rib received path 65001,65002", true},
		{"anchored run", "^65001 65002", "show bgp rib received path ^65001,65002", true},
		{"anchored first AS", "^65001", "show bgp rib received path ^65001", true},
		{"commas and spaces", "65001 , 65002", "show bgp rib received path 65001,65002", true},
		{"reverse order", "65002 65001", "show bgp rib received path 65002,65001", false},
		{"anchored at a later AS", "^65002", "show bgp rib received path ^65002", false},
		{"last valid as number", "4294967295", "show bgp rib received path 4294967295", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, commands := lookupRequest(t, "/lg/search", url.Values{"aspath": {tc.aspath}}.Encode())
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
			}
			rib := ribCommands(commands)
			if len(rib) != 1 || rib[0] != tc.command {
				t.Fatalf("rib commands = %q, want [%q]", rib, tc.command)
			}
			body := rec.Body.String()
			if strings.Contains(body, `class="error-banner"`) {
				t.Fatalf("search for %q answered an error: %s", tc.aspath, body)
			}
			if listed := strings.Contains(body, "<td>10.0.0.0/24</td>"); listed != tc.listed {
				t.Errorf("search for %q lists the route of path 65001 65002: %v, want %v", tc.aspath, listed, tc.listed)
			}
		})
	}
}

// TestSearchRefusesUnsupportedPathCharacters proves the page refuses an AS
// path the engine cannot match, before anything reaches bgp-rib, and says
// what the field takes.
//
// VALIDATES: an end anchor, an alternation, a wildcard, a class, a
// repetition and an AS number past 4294967295 are each answered by the error
// banner naming the value, stating that the field takes AS numbers separated
// by spaces with an optional leading ^, and ending on a valid example; no
// `show bgp rib` command is sent.
// PREVENTS: a regular expression accepted by the page and refused by the
// engine as "invalid ASN", which names the wrong problem
// (spec-lg-search-matches-what-the-pages-promise AC-10).
func TestSearchRefusesUnsupportedPathCharacters(t *testing.T) {
	for _, aspath := range []string{"65001$", "(65001|65002)", "65001.*", "[0-9]", "65001+", "4294967296"} {
		t.Run(aspath, func(t *testing.T) {
			rec, commands := lookupRequest(t, "/lg/search", url.Values{"aspath": {aspath}}.Encode())
			if rib := ribCommands(commands); len(rib) != 0 {
				t.Fatalf("an unsupported AS path reached bgp-rib: %q", rib)
			}
			body := rec.Body.String()
			if !strings.Contains(body, `class="error-banner">AS path `+aspath) {
				t.Errorf("no banner naming %q: %s", aspath, body)
			}
			for _, want := range []string{"separated by spaces", "a leading ^ anchors the run", "fix: " + asPathPatternExample} {
				if !strings.Contains(body, want) {
					t.Errorf("banner does not say %q: %s", want, body)
				}
			}
		})
	}
}

// TestRouteDetailAsksForExactlyThePrefix proves the route detail shows the
// route of the prefix it names, and never a longer prefix whose text starts
// the same way.
//
// VALIDATES: the detail of 192.0.0.0/2 is asked as `exact 192.0.0.0/2`, and
// over a RIB that holds 192.0.0.0/24 from the requested peer and 192.0.0.0/2
// from another, it shows the 192.0.0.0/2 route; a bare address is refused
// with 400, sends nothing, and offers its host prefix.
// PREVENTS: the text filter `prefix 192.0.0.0/2`, which also answers
// 192.0.0.0/24, letting the peer match pick the /24 row for a /2 detail.
func TestRouteDetailAsksForExactlyThePrefix(t *testing.T) {
	const rib = `{"routes":[` +
		`{"peer":"10.0.0.1","direction":"received","family":"ipv4/unicast","prefix":"192.0.0.0/24",` +
		`"next-hop":"10.0.0.1","origin":"igp","as-path":[65024],"local-preference":100},` +
		`{"peer":"10.0.0.2","direction":"received","family":"ipv4/unicast","prefix":"192.0.0.0/2",` +
		`"next-hop":"10.0.0.2","origin":"igp","as-path":[65002],"local-preference":100}]}`

	var commands []string
	dispatch := func(_ context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
		commands = append(commands, cmd)
		query, refusal := fakeRIBPipeline(strings.Fields(strings.TrimPrefix(cmd, "show bgp rib")), false)
		if refusal != "" {
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"error":`+strconv.Quote(refusal)+`}`)), nil
		}
		answer := fakeSelectedAnswer(rib, "routes", "peer", query)
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(answer)), nil
	}
	srv, err := NewLGServer(LGConfig{ListenAddrs: []string{"127.0.0.1:0"}, Dispatch: dispatch})
	if err != nil {
		t.Fatalf("new lg server: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/lg/route/detail?prefix=192.0.0.0/2&peer=10.0.0.1", http.NoBody)
	rec := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if len(commands) != 1 || commands[0] != "show bgp rib received exact 192.0.0.0/2" {
		t.Fatalf("commands = %q, want [show bgp rib received exact 192.0.0.0/2]", commands)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<dt>Prefix</dt><dd>192.0.0.0/2</dd>") {
		t.Errorf("detail of 192.0.0.0/2 does not show that route: %s", body)
	}
	if strings.Contains(body, "192.0.0.0/24") {
		t.Errorf("detail of 192.0.0.0/2 shows the 192.0.0.0/24 route: %s", body)
	}

	commands = nil
	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/lg/route/detail?prefix=192.0.0.1&peer=10.0.0.1", http.NoBody)
	rec = httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("detail of a bare address: status = %d, want 400", rec.Code)
	}
	if len(commands) != 0 {
		t.Errorf("detail of a bare address reached bgp-rib: %q", commands)
	}
	if !strings.Contains(rec.Body.String(), "fix: 192.0.0.1/32") {
		t.Errorf("refusal does not offer the host prefix: %s", rec.Body.String())
	}
}
