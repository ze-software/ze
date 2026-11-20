// Design: docs/labs/pppoe-interop.md -- unit discrimination of product observations.
// Related: product_state.go -- these fixtures judge the lab, not the data plane.
package testvpppppoe

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func productSessionFixture() productSession {
	return productSession{SID: 17, MAC: "02:00:00:40:00:01", Access: "host-eth0", Service: "internet", State: "session", Unit: -1, Name: "pppoe_session0", Index: 7}
}

const productNativeFixture = `Number of PPPoE sessions: 1
[0] sw-if-index 7 client-ip4 10.20.0.2 client-ip6 ::/0 session-id 17 encap-if-index 1 decap-fib-index 0
    local-mac 02:00:00:00:00:01  client-mac 02:00:00:40:00:01
`

func TestProductRequiresRealDaemonNativeIdentity(t *testing.T) {
	row := productSessionFixture()
	if err := validateProductSessions([]productSession{row}, row.Access, 1); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*productSession)
	}{
		{"kernel unit", func(row *productSession) { row.Unit = 0 }},
		{"kernel name", func(row *productSession) { row.Name = "ppp0" }},
		{"zero index", func(row *productSession) { row.Index = 0 }},
		{"missing SID", func(row *productSession) { row.SID = 0 }},
		{"other access", func(row *productSession) { row.Access = "other" }},
		{"not admitted", func(row *productSession) { row.State = "discovery" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			broken := row
			test.mutate(&broken)
			if validateProductSessions([]productSession{broken}, row.Access, 1) == nil {
				t.Fatal("accepted a non-native or wrong access identity")
			}
		})
	}
	if validateProductSessions(nil, row.Access, 0) == nil {
		t.Fatal("null was treated as an empty live table")
	}
	if validateProductSessions([]productSession{row, row}, row.Access, 2) == nil {
		t.Fatal("accepted reused native identity")
	}
}

// TestProductNativeAcceptsVPPCLILineEndings proves the native session dump
// parses as vppctl prints it, every line ending in CR LF, with the same rows
// as the LF form. A lab run read "Number of PPPoE sessions: 1\r" and refused
// a correct dump because the count did not parse.
func TestProductNativeAcceptsVPPCLILineEndings(t *testing.T) {
	want, err := parseProductNative(productNativeFixture)
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseProductNative(strings.ReplaceAll(productNativeFixture, "\n", "\r\n"))
	if err != nil {
		t.Fatalf("refused the CR LF dump: %v", err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("CR LF rows = %+v, want %+v", got, want)
	}
}

func TestProductNativeRequiresMatchingNegotiatedOwner(t *testing.T) {
	rows, err := parseProductNative(productNativeFixture)
	if err != nil {
		t.Fatal(err)
	}
	session := productSessionFixture()
	if err := productPopulation([]productSession{session}, rows, []productLink{{Name: "ppp0", Address: "10.20.0.2"}}, 1); err != nil {
		t.Fatal(err)
	}
	for _, replacement := range []struct{ old, next string }{{"session-id 17", "session-id 18"}, {"sw-if-index 7", "sw-if-index 8"}, {"client-mac 02:00:00:40:00:01", "client-mac 02:00:00:40:00:02"}} {
		broken, err := parseProductNative(strings.Replace(productNativeFixture, replacement.old, replacement.next, 1))
		if err != nil {
			t.Fatal(err)
		}
		if matchProductNative([]productSession{session}, broken) == nil {
			t.Fatalf("accepted foreign or unnegotiated owner: %s", replacement.next)
		}
	}
	if _, err := parseProductNative(strings.Replace(productNativeFixture, "sessions: 1", "sessions: 2", 1)); err == nil {
		t.Fatal("accepted incomplete native dump")
	}
	for _, output := range []string{"", "usage: show pppoe session", "No pppoe sessions configured...\nERROR"} {
		if productNativeEmpty(output) {
			t.Fatalf("absence accepted incomplete native answer %q", output)
		}
	}
	if !productNativeEmpty("No pppoe sessions configured...\n") {
		t.Fatal("refused authoritative native empty answer")
	}
	// Owned host routes carry negotiated IPCP state. The creation field can
	// remain zero and must not be presented as the negotiated address.
	created, err := parseProductNative(strings.Replace(productNativeFixture, "client-ip4 10.20.0.2", "client-ip4 0.0.0.0", 1))
	if err != nil {
		t.Fatal(err)
	}
	if err := matchProductNative([]productSession{session}, created); err != nil {
		t.Fatal(err)
	}
	fib := productOwnedPathFixture
	if _, err := productOwner(fib, "10.20.0.2", []productSession{session}); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{"", strings.Replace(fib, "ze-pppoe", "api", 1), strings.Replace(fib, "pppoe_session0", "pppoe_session01", 1), strings.Replace(fib, "10.20.0.2/32", "10.20.0.3/32", 1)} {
		if _, err := productOwner(output, "10.20.0.2", []productSession{session}); err == nil {
			t.Fatal("accepted foreign, missing or substring-only FIB owner")
		}
	}
}

func TestProductCaptureRequiresJoinedCompleteZero(t *testing.T) {
	positive := "listening on cp0, link-type EN10MB\n0 packets captured\n0 packets received by filter\n0 packets dropped by kernel\n"
	if err := productCaptureEmpty(positive); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{"", "listening on cp0", "0 packets captured", strings.Replace(positive, "0 packets captured", "1 packet captured", 1), strings.Replace(positive, "0 packets captured", "2 packets captured", 1), strings.Replace(positive, "0 packets dropped", "1 packets dropped", 1), positive + "1 packets captured\n"} {
		if productCaptureEmpty(output) == nil {
			t.Fatalf("accepted incomplete or leaking capture %q", output)
		}
	}
}

func TestProductAuthenticationSeparatesAcceptanceAndRefusal(t *testing.T) {
	base := "rcvd [LCP ConfReq <auth chap MD5>]\nsent [LCP ConfAck]\nsent [LCP ConfReq]\nrcvd [LCP ConfAck]\nrcvd [CHAP Challenge]\nsent [CHAP Response name = \"alice\"]\n"
	accepted, refused := base+"rcvd [CHAP Success]\n", base+"rcvd [CHAP Failure]\n"
	if err := productAuth(accepted, true); err != nil {
		t.Fatal(err)
	}
	if err := productAuth(refused, false); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{"", refused, strings.Replace(accepted, "name = \"alice\"", "name = \"bob\"", 1), strings.Replace(accepted, "<auth chap MD5>", "<auth pap>", 1)} {
		if productAuth(output, true) == nil {
			t.Fatal("accepted missing or refused CHAP evidence")
		}
	}
	for _, output := range []string{"", accepted, refused + "rcvd [IPCP ConfAck]\n"} {
		if productAuth(output, false) == nil {
			t.Fatal("accepted missing refusal or authenticated IPCP")
		}
	}
}

func TestProductHundredPlanHasDistinctPhysicalPeers(t *testing.T) {
	seen := make(map[string]bool)
	for index := range 101 {
		name, address := productClient(index)
		if seen[name] || seen[address] {
			t.Fatal("physical client identity reused")
		}
		seen[name], seen[address] = true, true
		mac, err := net.ParseMAC(address)
		if err != nil || len(mac) != 6 || mac[0]&3 != 2 {
			t.Fatalf("not a locally administered unicast MAC: %s", address)
		}
	}
	plan := productPlan("checkout", "/owned", "unit", modePlain, 100)
	if plan.Source.Name != "pppoe-vpp-ze-ac-pppd-client-hundred" || plan.Source.Checker == nil {
		t.Fatal("missing registered product checker")
	}
	if len(plan.Peers) != 3 {
		t.Fatal("product introduced a separate lab framework or one container per subscriber")
	}
	server := plan.Peers[0]
	if server.Name != vppPeer || slices.Contains(server.Arguments, "/dev/ppp") || slices.Contains(server.Arguments, "--privileged") {
		t.Fatal("Ze/VPP server has a kernel PPP device or broad privilege")
	}
	staged := false
	for _, mount := range server.Mounts {
		if mount.Target == "/product/ze" && mount.Source == filepath.Join(string(filepath.Separator), "owned", "ze") && mount.ReadOnly {
			staged = true
		}
	}
	if !staged {
		t.Fatal("server does not run the freshly staged real daemon")
	}
	config := productConfig("host-eth0", "")
	// The access interface is declared: Ze refuses PPPoE on any other name.
	// product_config_test.go runs every product config through validation.
	for _, required := range []string{"backend vpp", "ethernet host-eth0 { }", "interface host-eth0 { }", "external true", "max-sessions 100", "end 10.20.0.102", "auth-method chap-md5"} {
		if !strings.Contains(config, required) {
			t.Fatalf("configuration missing %s", required)
		}
	}
}

func TestProductLimitPreservesEveryExistingOwner(t *testing.T) {
	before := make([]productSession, 100)
	for index := range before {
		before[index] = productSessionFixture()
		before[index].SID = uint16(index + 1)
		before[index].Index = uint32(index + 1)
		before[index].Name = fmt.Sprintf("pppoe_session%d", index)
		_, before[index].MAC = productClient(index)
	}
	after := slices.Clone(before)
	slices.Reverse(after)
	if err := sameProductOwners(before, after); err != nil {
		t.Fatal(err)
	}
	after[0].SID = 500
	if sameProductOwners(before, after) == nil {
		t.Fatal("limit refusal was allowed to displace a live subscriber")
	}
	if productPopulation(before, nil, nil, 100) == nil {
		t.Fatal("hundred daemon rows alone proved usable native subscribers")
	}
}

type productHTTPTransport func(*http.Request) (*http.Response, error)

func (transport productHTTPTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestProductObserverRefusesUnavailableOrIncompleteState(t *testing.T) {
	for _, test := range []struct {
		name, body string
		code       int
		accepted   bool
	}{
		{"valid empty live table", `{"status":"done","data":[]}`, 200, true},
		{"no data", `{"status":"done"}`, 200, false},
		{"null", `{"status":"done","data":null}`, 200, false},
		{"error", `{"status":"error","data":[],"error":"not running"}`, 200, false},
		{"warning", `{"status":"warning","data":[]}`, 200, false},
		{"non JSON", "usage: show pppoe", 200, false},
		{"HTTP failure", `{"status":"done","data":[]}`, 503, false},
		{"oversize", strings.Repeat(" ", productReplyLimit+1), 200, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: productHTTPTransport(func(request *http.Request) (*http.Response, error) {
				if request.URL.Host != "127.0.0.1:9099" || request.Header.Get("Authorization") != "Bearer "+productToken {
					t.Fatal("observer changed its scoped authenticated destination")
				}
				return &http.Response{StatusCode: test.code, Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})}
			_, err := observeProduct(context.Background(), client, "show pppoe sessions")
			if (err == nil) != test.accepted {
				t.Fatalf("acceptance=%v, expected=%v, err=%v", err == nil, test.accepted, err)
			}
		})
	}
	if _, err := observeProduct(context.Background(), http.DefaultClient, "clear pppoe session 17"); err == nil {
		t.Fatal("observer permits a mutating command")
	}
}

// TestProductPlanDeclaresTheDialLog proves a failed product run prints the
// logs the product dials write, not the mechanism lab's file: a run that
// stopped before IPCP showed no PPP negotiation because the client declared
// a file nothing wrote. The 101st dial's log and the capacity capture log
// answer a refused-capacity failure in the hundred lab.
func TestProductPlanDeclaresTheDialLog(t *testing.T) {
	plan := productPlan("checkout", "/owned", "unit", modePlain, 100)
	want := []string{productLog(0), productLog(100), "/tmp/product-capacity.log"}
	for _, peer := range plan.Peers {
		if peer.Name != clientPeer {
			continue
		}
		if !slices.Equal(peer.LogFiles, want) {
			t.Fatalf("client log files = %q, want %q", peer.LogFiles, want)
		}
		if slices.Contains(peer.LogFiles, "/var/log/ppp/mechanism.log") {
			t.Fatal("the product client still declares the mechanism lab's log")
		}
		return
	}
	t.Fatal("product plan has no client peer")
}

// TestProductLinksReadIPRoute2PeerAddress proves the IPCP link parser reads
// the peer where iproute2 prints it. The input is `ip -j -4 address show`
// from iproute2 6.1 for 10.20.0.2 peer 10.20.0.1/32: the peer is the
// "address" key, and a parser that read a "peer" key found no link on a
// client whose ppp0 was up.
func TestProductLinksReadIPRoute2PeerAddress(t *testing.T) {
	const output = `[{"ifindex":10,"ifname":"ppp0","flags":["POINTOPOINT","MULTICAST","NOARP","UP","LOWER_UP"],"mtu":1492,"qdisc":"noqueue","operstate":"UNKNOWN","group":"default","txqlen":3,"addr_info":[{"family":"inet","local":"10.20.0.2","address":"10.20.0.1","prefixlen":32,"scope":"global","label":"ppp0","valid_life_time":4294967295,"preferred_life_time":4294967295}]}]`
	rows, err := parseProductLinks(output)
	if err != nil {
		t.Fatal(err)
	}
	if want := []productLink{{Name: "ppp0", Address: "10.20.0.2"}}; !slices.Equal(rows, want) {
		t.Fatalf("links = %+v, want %+v", rows, want)
	}
	other := strings.Replace(output, `"address":"10.20.0.1"`, `"address":"10.20.0.9"`, 1)
	if rows, err := parseProductLinks(other); err != nil || len(rows) != 0 {
		t.Fatalf("a link to another peer was counted: %+v, %v", rows, err)
	}
}
