package lg

import (
	"context"
	"errors"
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

		if !strings.Contains(body, errTestEngineDown.Error()) {
			t.Errorf("body does not name the reason %q: %s", errTestEngineDown, body)
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
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"summary":{"peers":[{"address":"192.0.2.1","state":"established"}]}}`)), nil
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

	t.Run("summary envelope", func(t *testing.T) {
		// Real summary handler returns {"summary": {"peers": [...]}}.
		ze := map[string]any{
			"summary": map[string]any{
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

	t.Run("summary envelope with address field", func(t *testing.T) {
		ze := map[string]any{
			"summary": map[string]any{
				"peers": []any{
					map[string]any{"address": "10.0.0.3", "name": "peer3"},
				},
			},
		}
		got := findPeer(ze, "10.0.0.3")
		if got == nil || getStr(got, "name") != "peer3" {
			t.Error("expected peer3")
		}
	})
}

func TestEngineError(t *testing.T) {
	// VALIDATES: error message for nil vs non-nil engine data.
	if got := engineError(nil); got != "BGP engine unavailable" {
		t.Errorf("nil: got %q, want BGP engine unavailable", got)
	}
	if got := engineError(map[string]any{}); got != "" {
		t.Errorf("non-nil: got %q, want empty", got)
	}
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
