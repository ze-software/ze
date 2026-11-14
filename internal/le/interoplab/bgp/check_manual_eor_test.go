// Design: docs/architecture/testing/interop.md -- requested End-of-RIB on a manual-eor peer.
package bgp

import (
	"net/netip"
	"strings"
	"testing"
)

const manualEORTestPeer = "172.30.0.2"

// manualEORTestLine wraps message in FRR 10.7.1's file-target envelope, the
// form noFamilyFRRLogMessage admits.
func manualEORTestLine(message string) string {
	return "2026/11/14 10:00:00 BGP: [ABCDE-12345] " + message
}

func manualEORTestRoute(prefix string) string {
	return manualEORTestLine(manualEORTestPeer + "(Unknown) rcvd " + prefix + " IPv4 unicast")
}

func manualEORTestMarker(peer string) string {
	return manualEORTestLine(manualEORMarkerMessage(peer))
}

func manualEORTestLog(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}

// manualEORCheckerBranches drives every predicate of the manual-eor checker in
// both polarities, each false case written against one wrong reading: an
// absence with no route proving the log was read, a marker from another
// neighbor or outside FRR's envelope counted as Ze's, a second or early
// marker accepted, a replaced log read as the second session, and a process
// record that proves nothing about the timing.
func manualEORCheckerBranches(t *testing.T) {
	first := manualEORTestRoute(manualEORFirstPrefix)
	second := manualEORTestRoute(manualEORSecondPrefix)
	marker := manualEORTestMarker(manualEORTestPeer)

	t.Run("first session", func(t *testing.T) {
		if err := requireNoManualEORMarker(manualEORTestLog(first), manualEORTestPeer, manualEORFirstPrefix); err != nil {
			t.Fatalf("a route and no marker was refused: %v", err)
		}
		other := manualEORTestMarker("172.30.0.22")
		embedded := "noise " + manualEORMarkerMessage(manualEORTestPeer)
		if err := requireNoManualEORMarker(manualEORTestLog(first, other, embedded), manualEORTestPeer, manualEORFirstPrefix); err != nil {
			t.Fatalf("another neighbor's marker or unenveloped text was counted as ze's: %v", err)
		}
		if err := requireNoManualEORMarker(manualEORTestLog(first, marker), manualEORTestPeer, manualEORFirstPrefix); err == nil {
			t.Fatal("an unrequested marker on the first session was accepted")
		}
		if err := requireNoManualEORMarker(manualEORTestLog(manualEORTestLine("bgpd started")), manualEORTestPeer, manualEORFirstPrefix); err == nil {
			t.Fatal("a log with no route from ze passed as ze withholding the marker")
		}
		longer := manualEORTestRoute("110.84.1.0/24")
		if err := requireNoManualEORMarker(manualEORTestLog(longer), manualEORTestPeer, manualEORFirstPrefix); err == nil {
			t.Fatal("a longer prefix passed as the first session's route")
		}
	})

	t.Run("second session", func(t *testing.T) {
		if err := requireOneManualEORMarkerAfterRoute(manualEORTestLog(second, marker), manualEORTestPeer, manualEORSecondPrefix); err != nil {
			t.Fatalf("a route then one marker was refused: %v", err)
		}
		if !manualEORRequestedReady(manualEORTestLog(second, marker), manualEORTestPeer, manualEORSecondPrefix) {
			t.Fatal("a route then a marker was not ready")
		}
		if err := requireOneManualEORMarkerAfterRoute(manualEORTestLog(second), manualEORTestPeer, manualEORSecondPrefix); err == nil {
			t.Fatal("a dropped marker was accepted")
		}
		if err := requireOneManualEORMarkerAfterRoute(manualEORTestLog(second, marker, marker), manualEORTestPeer, manualEORSecondPrefix); err == nil {
			t.Fatal("a second marker was accepted")
		}
		if err := requireOneManualEORMarkerAfterRoute(manualEORTestLog(marker, second), manualEORTestPeer, manualEORSecondPrefix); err == nil {
			t.Fatal("a marker before the route was accepted")
		}
		if manualEORRequestedReady(manualEORTestLog(marker, second), manualEORTestPeer, manualEORSecondPrefix) {
			t.Fatal("a marker before the route was ready")
		}
		if err := requireOneManualEORMarkerAfterRoute(manualEORTestLog(marker), manualEORTestPeer, manualEORSecondPrefix); err == nil {
			t.Fatal("a marker with no route was accepted")
		}
	})

	t.Run("log after the clear", func(t *testing.T) {
		before := manualEORTestLog(first)
		tail, err := manualEORLogAfter(before, before+manualEORTestLog(second, marker))
		if err != nil {
			t.Fatalf("an appended log was refused: %v", err)
		}
		if strings.Contains(tail, manualEORFirstPrefix) {
			t.Fatal("the tail holds a line written before the clear")
		}
		if _, err := manualEORLogAfter(before, manualEORTestLog(second, marker)); err == nil {
			t.Fatal("a replaced log was read as the second session")
		}
	})

	t.Run("process records", func(t *testing.T) {
		const one = `[{"session":1,"peer":"172.30.0.3","prefix":"10.84.1.0/24","end-of-rib":false,"elapsed-ms":12}]`
		const two = `[{"session":1,"peer":"172.30.0.3","prefix":"10.84.1.0/24","end-of-rib":false,"elapsed-ms":12},` +
			`{"session":2,"peer":"172.30.0.3","prefix":"10.84.2.0/24","end-of-rib":true,"elapsed-ms":30}]`
		if err := requireManualEORRequests(one, 1); err != nil {
			t.Fatalf("the first session's record was refused: %v", err)
		}
		if err := requireManualEORRequests(two, 2); err != nil {
			t.Fatalf("both sessions' records were refused: %v", err)
		}
		if err := requireManualEORRequests(one, 2); err == nil {
			t.Fatal("a missing second record was accepted")
		}
		refused := map[string]string{
			"no end-of-rib request": strings.Replace(two, `"end-of-rib":true`, `"end-of-rib":false`, 1),
			"request on session 1":  strings.Replace(two, `"end-of-rib":false`, `"end-of-rib":true`, 1),
			"past the bound":        strings.Replace(two, `"elapsed-ms":30`, `"elapsed-ms":4000`, 1),
			"recorded error":        strings.Replace(two, `"elapsed-ms":30`, `"elapsed-ms":30,"error":"timeout"`, 1),
			"wrong prefix":          strings.Replace(two, `"10.84.2.0/24"`, `"10.84.1.0/24"`, 1),
			"wrong session":         strings.Replace(two, `"session":2`, `"session":3`, 1),
			"not json":              "cat: no such file",
		}
		for name, output := range refused {
			if err := requireManualEORRequests(output, 2); err == nil {
				t.Fatalf("%s: the records were accepted", name)
			}
		}
	})

	t.Run("peer-up event", func(t *testing.T) {
		const up = `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"remote":{"address":"172.30.0.3"}},"state":"up"}}`
		peer, isUp, err := manualEORPeerUp(up)
		if err != nil {
			t.Fatalf("an up event did not decode: %v", err)
		}
		if !isUp {
			t.Fatal("an up event was not read as up")
		}
		if peer != netip.MustParseAddr("172.30.0.3") {
			t.Fatalf("an up event named %s", peer)
		}
		down := strings.Replace(up, `"state":"up"`, `"state":"down"`, 1)
		if _, isUp, err := manualEORPeerUp(down); err != nil || isUp {
			t.Fatalf("a down event read as up=%t err=%v", isUp, err)
		}
		if _, _, err := manualEORPeerUp("not json"); err == nil {
			t.Fatal("an undecodable event was read as no event")
		}
		noAddress := strings.Replace(up, `"172.30.0.3"`, `""`, 1)
		if _, _, err := manualEORPeerUp(noAddress); err == nil {
			t.Fatal("an up event with no address was accepted")
		}
	})
}
