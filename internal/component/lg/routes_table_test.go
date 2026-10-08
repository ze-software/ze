// Design: docs/architecture/api/birdwatcher-compat.md -- routes/table and routes/count
// Related: ribfake_test.go -- the bgp-rib answers these tests are fed

package lg

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// refusingDispatch answers every command with the error envelope a plugin
// returns for a command it refuses.
func refusingDispatch() CommandDispatcher {
	return func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"error":"unknown keyword: x"}`)), nil
	}
}

// TestAPIRoutesTableAnswersTheBestPaths drives routes/table through a dispatcher
// that answers as bgp-rib does.
//
// The table asked for `show bgp rib best ipv4/unicast`, which the plugin refuses,
// and read the refusal as an empty table. It also could not read the best-path
// envelope the plugin answers when asked correctly.
//
// VALIDATES: the table holds the best path, learnt from its best peer, primary.
// PREVENTS: an empty best table over a full Loc-RIB.
func TestAPIRoutesTableAnswersTheBestPaths(t *testing.T) {
	base, client := startPagServer(t, mockDispatch())
	env := getRoutes(t, client, base+"/api/looking-glass/routes/table/ipv4%2Funicast")

	routes, _ := env["routes"].([]any)
	if len(routes) != 1 {
		t.Fatalf("routes = %v, want the one best path", env["routes"])
	}
	route, _ := routes[0].(map[string]any)
	if got := getStr(route, "network"); got != "10.0.0.0/24" {
		t.Errorf("network = %q, want 10.0.0.0/24", got)
	}
	if got := getStr(route, "gateway"); got != "10.0.0.1" {
		t.Errorf("gateway = %q, want 10.0.0.1", got)
	}
	if got := getStr(route, "learnt_from"); got != "10.0.0.1" {
		t.Errorf("learnt_from = %q, want the best peer 10.0.0.1", got)
	}
	if route["primary"] != true {
		t.Errorf("primary = %v; a row of the best table is the best path", route["primary"])
	}
}

// TestAPIRoutesTableRefusesAnEngineError proves an engine refusal is not
// rendered as an empty table.
//
// VALIDATES: an error envelope answers 502 naming the command.
// PREVENTS: a refused command reading as "this family holds no route".
func TestAPIRoutesTableRefusesAnEngineError(t *testing.T) {
	base, client := startPagServer(t, refusingDispatch())
	resp := doGet(t, client, base+"/api/looking-glass/routes/table/ipv4%2Funicast")
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
}

// TestAPIRoutesCountAnswersTheStoredCount drives routes/count through a
// dispatcher that refuses what bgp-rib refuses.
//
// The endpoint asked for `show bgp rib count peer <name>`. The plugin refuses a
// filter after a terminal, and the endpoint read the refusal as a count of 0.
//
// The fake answers 100 for the `received` scope and 150 with no scope keyword,
// the plugin's default `sent-received`, so the endpoint that counted the routes
// Ze sent the peer as learned from it answers 150 here.
//
// VALIDATES: the received count the plugin answers reaches the client.
// PREVENTS: a count of 0 over a peer whose routes are stored, and a count that
// adds the routes Ze sent the peer.
func TestAPIRoutesCountAnswersTheStoredCount(t *testing.T) {
	base, client := startPagServer(t, mockDispatch())
	env := getRoutes(t, client, base+"/api/looking-glass/routes/count/protocol/peer1")

	if got := getNum(env, "routes"); got != 100 {
		t.Fatalf("routes = %v, want the plugin's count 100", env["routes"])
	}
}

// TestAPIRoutesCountRefusesAnEngineError proves an engine refusal is not
// rendered as a count of 0.
//
// VALIDATES: an error envelope answers 502 naming the command.
// PREVENTS: a refused command reading as "this peer holds no route".
func TestAPIRoutesCountRefusesAnEngineError(t *testing.T) {
	base, client := startPagServer(t, refusingDispatch())
	resp := doGet(t, client, base+"/api/looking-glass/routes/count/protocol/peer1")
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
}

// TestUIPeerPageReadsTheHistogram proves the peer page asks for the histogram in
// a spelling bgp-rib accepts.
//
// The page asked for `show bgp rib histogram peer <address>`, which the plugin
// refuses as a filter after a terminal, so the page showed an error and no
// histogram for every peer.
//
// VALIDATES: the histogram the plugin answers reaches the page.
// PREVENTS: an empty histogram and an error banner over a populated peer.
func TestUIPeerPageReadsTheHistogram(t *testing.T) {
	base, client := startPagServer(t, mockDispatch())
	resp := doGet(t, client, base+"/lg/peer/10.0.0.1")
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(raw)
	if strings.Contains(body, "filter after terminal") {
		t.Fatalf("the peer page shows the plugin's refusal:\n%s", body)
	}
	if !strings.Contains(body, "10.0.0.0/24") {
		t.Fatalf("the peer page lists no route:\n%s", body)
	}
}
