package bgp

import (
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// liveKeepSessionCheckerBranches drives both polarities of every predicate the
// bgp-attach-receive-only-live-frr and BFD enable checkers decide with, over
// FRR JSON shaped as FRR prints it, so no container is needed.
func liveKeepSessionCheckerBranches(t *testing.T) {
	const neighbor = "172.30.0.2"
	neighborJSON := func(state, generation, refresh string) string {
		return `{"` + neighbor + `":{"bgpState":"` + state + `","connectionsEstablished":` + generation +
			`,"messageStats":{"updatesRecv":3,"routeRefreshSent":0,"routeRefreshRecv":` + refresh + `}}}`
	}

	before, err := parseFRRSessionCounters(neighborJSON(stateEstablished, "1", "0"), neighbor)
	if err != nil || before.generation != 1 || before.refreshReceived != 0 {
		t.Fatalf("counters = %+v, %v; want generation 1, refresh 0", before, err)
	}
	after, err := parseFRRSessionCounters(neighborJSON(stateEstablished, "1", "1"), neighbor)
	if err != nil {
		t.Fatalf("established neighbor refused: %v", err)
	}
	if err := requireOneRefreshSameSession(before, after); err != nil {
		t.Fatalf("one refresh on the same session refused: %v", err)
	}
	if err := requireOneRefreshSameSession(before, before); err == nil {
		t.Fatal("zero ROUTE-REFRESH passed as the catch-up")
	}
	two := frrSessionCounters{generation: 1, refreshReceived: 2}
	if err := requireOneRefreshSameSession(before, two); err == nil {
		t.Fatal("two ROUTE-REFRESH passed for one negotiated family")
	}
	moved, err := parseFRRSessionCounters(neighborJSON(stateEstablished, "2", "1"), neighbor)
	if err != nil {
		t.Fatalf("re-established neighbor refused: %v", err)
	}
	if err := requireOneRefreshSameSession(before, moved); err == nil || !strings.Contains(err.Error(), "moved from 1 to 2") {
		t.Fatalf("a re-established session passed: %v", err)
	}
	if _, err := parseFRRSessionCounters(neighborJSON("Active", "1", "0"), neighbor); err == nil {
		t.Fatal("a neighbor that is not Established passed")
	}
	missing := `{"` + neighbor + `":{"bgpState":"Established","connectionsEstablished":1,"messageStats":{"updatesRecv":3}}}`
	if _, err := parseFRRSessionCounters(missing, neighbor); err == nil || !strings.Contains(err.Error(), "updatesRecv") {
		t.Fatalf("a missing routeRefreshRecv passed or did not list the keys: %v", err)
	}
	if _, err := parseFRRSessionCounters(neighborJSON(stateEstablished, "1", `"x"`), neighbor); err == nil {
		t.Fatal("a non-numeric routeRefreshRecv passed")
	}

	if err := requireSameBGPGeneration("the add", 3, 3); err != nil {
		t.Fatalf("same generation refused: %v", err)
	}
	if err := requireSameBGPGeneration("the add", 3, 4); err == nil || !strings.Contains(err.Error(), "the add") {
		t.Fatalf("moved generation passed: %v", err)
	}

	routes := []rpkiReloadRoute{{Key: "ipv4/unicast:10.0.0.0/24"}, {Key: attachLiveRouteKey}}
	if !adjRIBInHolds(routes, attachLiveRouteKey) {
		t.Fatal("held route not found")
	}
	if adjRIBInHolds(routes[:1], attachLiveRouteKey) || adjRIBInHolds(nil, attachLiveRouteKey) {
		t.Fatal("absent route reported held")
	}

	// The removal step: FRR's BFD peer as it reads once ze sent AdminDown,
	// against ze going silent and against a peer still Up.
	bfdPeer := func(status, diagnostic, remote string) string {
		return `{"peer":"172.30.0.2","id":3639475440,"remote-id":1,"status":"` + status +
			`","diagnostic":"` + diagnostic + `","remote-diagnostic":"` + remote + `","remote-transmit-interval":1000}`
	}
	reading, err := readFRRBFDDown(bfdPeer("down", "neighbor signaled session down", frrDiagAdminDown))
	if err != nil || !reading.adminDownFromRemote() {
		t.Fatalf("ze's AdminDown not recognized: %+v, %v", reading, err)
	}
	for _, refused := range []string{
		bfdPeer("down", "control detection time expired", "ok"),
		bfdPeer("up", "ok", "ok"),
		bfdPeer("up", "ok", frrDiagAdminDown),
		bfdPeer("init", "ok", frrDiagAdminDown),
	} {
		reading, err := readFRRBFDDown(refused)
		if err != nil {
			t.Fatalf("well-formed peer refused: %v", err)
		}
		if reading.adminDownFromRemote() {
			t.Fatalf("taken for ze's AdminDown: %s", refused)
		}
	}
	for _, malformed := range []string{
		`{"peer":"172.30.0.2","remote-diagnostic":"administratively down"}`,
		`{"peer":"172.30.0.2","status":"down"}`,
		`% Unknown command`,
	} {
		if _, err := readFRRBFDDown(malformed); err == nil {
			t.Fatalf("malformed peer passed: %s", malformed)
		}
	}

	// A step reload config gets the interop CLI user like ze.conf does, or the
	// checker loses SSH at that reload.
	if !isZeStepReloadConfig(bfdEnableEditConfig) {
		t.Fatalf("%s not recognized as a step reload config", bfdEnableEditConfig)
	}
	for _, other := range []string{zeConfigFile, zeReloadConfigFile, "frr.conf", "sub/ze-reload-edit.conf"} {
		if isZeStepReloadConfig(other) {
			t.Fatalf("%s taken for a step reload config", other)
		}
	}
	source, target := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(source, bfdEnableEditConfig), []byte("bgp {\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := renderScenario(source, target, interoplab.Network{IPv4: netip.MustParsePrefix("172.31.71.0/24")}); err != nil {
		t.Fatalf("render: %v", err)
	}
	rendered, err := os.ReadFile(filepath.Join(target, bfdEnableEditConfig))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rendered), "user interop") {
		t.Fatalf("%s rendered without the interop CLI user:\n%s", bfdEnableEditConfig, rendered)
	}
}
