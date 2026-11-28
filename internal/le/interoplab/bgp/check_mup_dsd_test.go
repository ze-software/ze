// Design: docs/architecture/testing/interop.md -- pure predicate tests for bgp-mup-dsd-gobgp.
// Related: check_mup_dsd.go -- predicates under test.
package bgp

import "testing"

func TestMUPDSDAdjRIBIn(t *testing.T) {
	t.Parallel()
	const peer = "172.30.0.5"
	valid := `{"adj-rib-in":{"172.30.0.5":[` +
		`{"family":"ipv4/unicast","nlri-hex":"180a1400","filtered":false},` +
		`{"family":"ipv4/mup","nlri-hex":"0100020C0000FDEA00000064C0000201","filtered":false}` +
		`]}}`
	if err := requireMUPDSDAdjRIBIn(valid, peer); err != nil {
		t.Fatalf("valid Adj-RIB-In rejected: %v", err)
	}

	missingDSD := `{"adj-rib-in":{"172.30.0.5":[{"family":"ipv4/unicast","nlri-hex":"180a1400","filtered":false}]}}`
	if err := requireMUPDSDAdjRIBIn(missingDSD, peer); err == nil {
		t.Fatal("missing DSD route unexpectedly accepted")
	}

	missingControl := `{"adj-rib-in":{"172.30.0.5":[{"family":"ipv4/mup","nlri-hex":"0100020c0000fdea00000064c0000201","filtered":false}]}}`
	if err := requireMUPDSDAdjRIBIn(missingControl, peer); err == nil {
		t.Fatal("missing control route unexpectedly accepted")
	}

	filteredDSD := `{"adj-rib-in":{"172.30.0.5":[` +
		`{"family":"ipv4/unicast","nlri-hex":"180a1400","filtered":false},` +
		`{"family":"ipv4/mup","nlri-hex":"0100020c0000fdea00000064c0000201","filtered":true}` +
		`]}}`
	if err := requireMUPDSDAdjRIBIn(filteredDSD, peer); err == nil {
		t.Fatal("filtered DSD route unexpectedly accepted")
	}

	if err := requireMUPDSDAdjRIBIn("not-json", peer); err == nil {
		t.Fatal("malformed JSON unexpectedly accepted")
	}
}

func TestMUPDSDUnresolvedWarning(t *testing.T) {
	t.Parallel()
	const peer = "172.30.0.5"
	valid := `{"warnings":[{"source":"bgp-nlri-mup","code":"doctor-bgp-mup-locator-unresolved","severity":"warning","subject":"172.30.0.5 no-active-source","message":"kept unverified","fix":"ze show isis database"}],"count":1}`
	if err := requireMUPDSDUnresolvedWarning(valid, peer); err != nil {
		t.Fatalf("valid warning rejected: %v", err)
	}

	empty := `{"warnings":[],"count":0}`
	if err := requireMUPDSDUnresolvedWarning(empty, peer); err == nil {
		t.Fatal("empty warnings unexpectedly accepted")
	}

	wrongSource := `{"warnings":[{"source":"other","code":"doctor-bgp-mup-locator-unresolved","subject":"172.30.0.5 no-active-source","fix":"ze show isis database"}],"count":1}`
	if err := requireMUPDSDUnresolvedWarning(wrongSource, peer); err == nil {
		t.Fatal("wrong warning source unexpectedly accepted")
	}

	wrongSubject := `{"warnings":[{"source":"bgp-nlri-mup","code":"doctor-bgp-mup-locator-unresolved","subject":"172.30.0.9 no-active-source","fix":"ze show isis database"}],"count":1}`
	if err := requireMUPDSDUnresolvedWarning(wrongSubject, peer); err == nil {
		t.Fatal("wrong warning subject unexpectedly accepted")
	}

	wrongFix := `{"warnings":[{"source":"bgp-nlri-mup","code":"doctor-bgp-mup-locator-unresolved","subject":"172.30.0.5 no-active-source","fix":"other"}],"count":1}`
	if err := requireMUPDSDUnresolvedWarning(wrongFix, peer); err == nil {
		t.Fatal("wrong warning fix unexpectedly accepted")
	}
}

func TestMUPDSDUnresolvedLog(t *testing.T) {
	t.Parallel()
	const peer = "172.30.0.5"
	good := `WARN BGP MUP segment discovery locator originator could not be resolved code=doctor-bgp-mup-locator-unresolved peer=172.30.0.5 sid=2001:db8:: fix="ze show isis database"`
	if err := requireMUPDSDUnresolvedLog(good, peer); err != nil {
		t.Fatalf("valid log line rejected: %v", err)
	}
	if err := requireMUPDSDUnresolvedLog("WARN unrelated peer=172.30.0.5 sid=2001:db8::", peer); err == nil {
		t.Fatal("log missing warning code unexpectedly accepted")
	}
	if err := requireMUPDSDUnresolvedLog("code=doctor-bgp-mup-locator-unresolved sid=2001:db8::", peer); err == nil {
		t.Fatal("log missing peer address unexpectedly accepted")
	}
	if err := requireMUPDSDUnresolvedLog("code=doctor-bgp-mup-locator-unresolved peer=172.30.0.5", peer); err == nil {
		t.Fatal("log missing SID unexpectedly accepted")
	}
}

func TestMUPDSDSessionStable(t *testing.T) {
	t.Parallel()
	good := `{"peers":{"172.30.0.5":{"state":"established","connections-established":1,"connections-dropped":0}}}`
	if err := requireMUPDSDSessionStable(good); err != nil {
		t.Fatalf("stable session detail rejected: %v", err)
	}
	dropped := `{"peers":{"172.30.0.5":{"state":"established","connections-established":2,"connections-dropped":1}}}`
	if err := requireMUPDSDSessionStable(dropped); err == nil {
		t.Fatal("dropped session detail unexpectedly accepted")
	}
}
