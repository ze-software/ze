// Design: docs/architecture/api/birdwatcher-compat.md -- received and sent scopes
// Related: ribfake_test.go -- the bgp-rib answers these tests are fed, per scope

package lg

import (
	"compress/gzip"
	"io"
	"strings"
	"testing"
)

// The fake answers each bgp-rib scope apart (ribfake_test.go): the `received`
// rows hold 10.0.0.0/24 and count 100, the `sent` rows hold 192.0.2.0/24 and
// count 50, and a command with no scope keyword answers both, as the plugin's
// default `sent-received` scope does. These tests ask what each surface shows
// a peer that both sent Ze routes and was sent routes by Ze.

// receivedPrefix and sentPrefix are the one route the fake holds in each scope.
const (
	receivedPrefix = "10.0.0.0/24"
	sentPrefix     = "192.0.2.0/24"
)

// routeNetworks returns the `network` of every route in a birdwatcher envelope.
func routeNetworks(env map[string]any) []string {
	routes, _ := env["routes"].([]any)
	networks := make([]string, 0, len(routes))
	for _, r := range routes {
		route, _ := r.(map[string]any)
		networks = append(networks, getStr(route, "network"))
	}
	return networks
}

// TestAPIRoutesLearnedFromAPeerAreOnlyTheReceived proves routes/protocol/{name}
// and routes/peer/{peer} list what the peer sent Ze, and nothing Ze sent it.
//
// Both asked `show bgp rib peer <name>` with no scope, which bgp-rib answers
// with its Adj-RIB-In and Adj-RIB-Out together, so a route Ze sent the peer was
// listed as learned from it.
//
// VALIDATES: the received route is listed, the sent route is not.
// PREVENTS: a route Ze advertised reading as one the peer advertised.
func TestAPIRoutesLearnedFromAPeerAreOnlyTheReceived(t *testing.T) {
	base, client := startPagServer(t, mockDispatch())
	for _, path := range []string{"routes/protocol/10.0.0.1", "routes/peer/10.0.0.1"} {
		env := getRoutes(t, client, base+"/api/looking-glass/"+path)
		networks := routeNetworks(env)
		if len(networks) != 1 || networks[0] != receivedPrefix {
			t.Errorf("%s: networks = %v, want only the received %s", path, networks, receivedPrefix)
		}
	}
}

// TestAPIRoutesExportIsOnlyTheSent proves routes/export/{name} lists what Ze
// sent the peer, and nothing the peer sent Ze.
//
// VALIDATES: the sent route is listed, the received route is not.
// PREVENTS: the export view drifting to the default two-direction scope.
func TestAPIRoutesExportIsOnlyTheSent(t *testing.T) {
	base, client := startPagServer(t, mockDispatch())
	env := getRoutes(t, client, base+"/api/looking-glass/routes/export/10.0.0.1")
	networks := routeNetworks(env)
	if len(networks) != 1 || networks[0] != sentPrefix {
		t.Fatalf("networks = %v, want only the sent %s", networks, sentPrefix)
	}
}

// TestUIPeerPageShowsReceivedAndSentApart proves the peer page counts what the
// peer sent Ze beside what Ze sent it, and lists only the received routes.
//
// VALIDATES: the counts row reads 100 received and 50 sent; the route rows hold
// the received prefix and not the sent one.
// PREVENTS: one mixed count, and a sent route listed under "Routes from".
func TestUIPeerPageShowsReceivedAndSentApart(t *testing.T) {
	base, client := startPagServer(t, mockDispatch())
	resp := doGet(t, client, base+"/lg/peer/10.0.0.1")
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, "<tr><td>100</td><td>50</td></tr>") {
		t.Errorf("the peer page does not count 100 received beside 50 sent:\n%s", body)
	}
	if !strings.Contains(body, receivedPrefix) {
		t.Errorf("the peer page does not list the received %s:\n%s", receivedPrefix, body)
	}
	if strings.Contains(body, sentPrefix) {
		t.Errorf("the peer page lists the sent %s as a route from the peer:\n%s", sentPrefix, body)
	}
}

// TestUIPeerDownloadIsOnlyTheReceived proves the peer page's CSV download holds
// the routes the peer sent Ze, the set the page lists.
//
// VALIDATES: the CSV holds the received prefix and not the sent one.
// PREVENTS: the download adding the routes Ze sent the peer.
func TestUIPeerDownloadIsOnlyTheReceived(t *testing.T) {
	base, client := startPagServer(t, mockDispatch())
	resp := doGet(t, client, base+"/lg/peer/10.0.0.1/download")
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	raw, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	csv := string(raw)
	if !strings.Contains(csv, receivedPrefix) {
		t.Errorf("the download does not hold the received %s:\n%s", receivedPrefix, csv)
	}
	if strings.Contains(csv, sentPrefix) {
		t.Errorf("the download holds the sent %s:\n%s", sentPrefix, csv)
	}
}
