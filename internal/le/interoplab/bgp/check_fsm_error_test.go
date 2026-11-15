// Design: docs/architecture/testing/interop.md -- RFC 6608 capture and native-parser evidence.
package bgp

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"testing"
)

// fsmErrorCheckerBranches proves a closed wire interval and the foreign
// parser's evidence are both required. These are oracle fixtures, not interop
// results: the main lab must still run each registered scenario against FRR.
func fsmErrorCheckerBranches(t *testing.T) {
	for _, test := range fsmRelayCases {
		t.Run(test.personality, func(t *testing.T) {
			good := fsmErrorTestReport(t, test)
			if err := judgeFSMRelay(good, test); err != nil {
				t.Fatalf("valid capture: %v", err)
			}
			for _, mutation := range []struct {
				name string
				edit func(*fsmRelayReport)
			}{
				{"missing native OPEN", func(r *fsmRelayReport) { r.Frames = r.Frames[1:] }},
				{"incomplete", func(r *fsmRelayReport) { r.Finished = false }},
				{"transport error", func(r *fsmRelayReport) { r.Problem = "truncated native frame" }},
				{"wrong case", func(r *fsmRelayReport) { r.Personality = "unregistered" }},
			} {
				t.Run(mutation.name, func(t *testing.T) {
					bad := fsmErrorTestReport(t, test)
					mutation.edit(&bad)
					if err := judgeFSMRelay(bad, test); err == nil {
						t.Fatal("incomplete capture passed")
					}
				})
			}
			if test.subcode == 0 {
				bad := fsmErrorTestReport(t, test)
				bad.Frames = bad.Frames[:len(bad.Frames)-1]
				if err := judgeFSMRelay(bad, test); err == nil {
					t.Fatal("control passed without its Established route witness")
				}
				fsmControlSnapshotBranches(t, test)
				return
			}
			if test.subcode != 1 {
				bad := fsmErrorTestReport(t, test)
				bad.Frames = append(bad.Frames[:2], bad.Frames[3:]...)
				if err := judgeFSMRelay(bad, test); err == nil {
					t.Fatal("OPEN replay passed without Ze's OpenConfirm KEEPALIVE witness")
				}
				bad = fsmErrorTestReport(t, test)
				for index := range bad.Frames {
					if bad.Frames[index].Replay {
						bad.Frames[index].Frame = hex.EncodeToString(speakerMessage(bgpOpen, []byte{4, 1, 2}))
					}
				}
				if err := judgeFSMRelay(bad, test); err == nil {
					t.Fatal("replay bytes not produced by native peer passed")
				}
			}
			if test.subcode == 3 {
				bad := fsmErrorTestReport(t, test)
				bad.Frames = append(bad.Frames[:4], bad.Frames[5:]...)
				if err := judgeFSMRelay(bad, test); err == nil {
					t.Fatal("Established replay passed without configured-route witness")
				}
			}
			for _, mutation := range []struct {
				name string
				edit func(*fsmRelayReport)
			}{
				{"Ze did not close", func(r *fsmRelayReport) { r.ZeEOF = false }},
				{"FRR did not close", func(r *fsmRelayReport) { r.NativeEOF = false }},
				{"missing notification", func(r *fsmRelayReport) { r.Frames = r.Frames[:len(r.Frames)-1] }},
				{"wrong subcode", func(r *fsmRelayReport) {
					r.Frames[len(r.Frames)-1].Frame = hex.EncodeToString(speakerMessage(bgpNotification, []byte{5, 0, test.message}))
				}},
				{"extra Data", func(r *fsmRelayReport) {
					r.Frames[len(r.Frames)-1].Frame = hex.EncodeToString(speakerMessage(bgpNotification, []byte{5, test.subcode, test.message, 0}))
				}},
			} {
				t.Run(mutation.name, func(t *testing.T) {
					bad := fsmErrorTestReport(t, test)
					mutation.edit(&bad)
					if err := judgeFSMRelay(bad, test); err == nil {
						t.Fatal("bad fatal capture passed")
					}
				})
			}
			neighbor := "172.30.0.10"
			document := fmt.Sprintf(`{%q:{"lastErrorCodeSubcode":"05%02X","lastResetDueTo":"BGP Notification received","bgpState":"Idle"}}`, neighbor, test.subcode)
			parsed := fmt.Sprintf("%%NOTIFICATION: received from neighbor %s 5/%d (FSM Error) 1 bytes %02x\n", neighbor, test.subcode, test.message)
			if err := judgeFRRFSMNotification(document, parsed, neighbor, test); err != nil {
				t.Fatalf("native parsed evidence: %v", err)
			}
			for _, bad := range []struct{ document, log string }{
				{document, ""},
				{document, strings.Replace(parsed, "1 bytes", "0 bytes", 1)},
				{document, strings.Replace(parsed, "received from", "sent to", 1)},
			} {
				if err := judgeFRRFSMNotification(bad.document, bad.log, neighbor, test); err == nil {
					t.Fatal("missing foreign parsed notification evidence passed")
				}
			}
			if test.subcode == 1 {
				fsmOpenSentParsedLogBranches(t, test)
			} else {
				for _, bad := range []string{"{}", strings.Replace(document, "received", "sent", 1)} {
					if err := judgeFRRFSMNotification(bad, parsed, neighbor, test); err == nil {
						t.Fatal("missing configured-peer reset after FRR Established passed")
					}
				}
			}
		})
	}
}

// This is the actual pinned FRR 10.7.1 output retained in the first live
// OpenSent run's checker recording, not a Ze log or a relay-generated verdict.
func fsmOpenSentParsedLogBranches(t *testing.T, test fsmRelayCase) {
	t.Helper()
	const neighbor = "172.30.0.10"
	const parsed = "%NOTIFICATION: received from neighbor 172.30.0.10 5/1 (Neighbor Events Error/Receive Unexpected Message in OpenSent State) 1 bytes 04"
	const configured = `{"172.30.0.10":{"lastResetDueTo":"No path to specified Neighbor","lastResetCode":27,"bgpState":"Active"}}`
	if err := judgeFRRFSMNotification(configured, parsed, neighbor, test); err != nil {
		t.Fatalf("actual pre-transfer native parsed evidence: %v", err)
	}
	for _, bad := range []string{
		"",
		strings.Replace(parsed, "5/1", "5/0", 1),
		strings.Replace(parsed, "5/1", "4/1", 1),
		strings.Replace(parsed, "1 bytes", "0 bytes", 1),
		strings.Replace(parsed, "1 bytes", "2 bytes", 1),
		strings.Replace(parsed, "bytes 04", "bytes 01", 1),
		parsed + " 00",
		strings.Replace(parsed, "received from", "sent to", 1),
		strings.Replace(parsed, neighbor, "172.30.0.11", 1),
		strings.Replace(parsed, "%NOTIFICATION", "Ze notification sent", 1),
	} {
		if err := judgeFRRFSMNotification(configured, bad, neighbor, test); err == nil {
			t.Fatalf("incorrect parsed receive evidence passed: %q", bad)
		}
	}
}

func TestFSMQueryFailureRetainsSuccessfulEvidence(t *testing.T) {
	evidence := map[string]string{}
	retainFSMQuery(evidence, "frr-parsed-log", "native parsed receive", nil)
	retainFSMQuery(evidence, "frr-parsed-log", "", context.DeadlineExceeded)
	retainFSMQuery(evidence, "frr-parsed-log", "", nil)
	if evidence["frr-parsed-log"] != "native parsed receive" || evidence["frr-parsed-log-query-error"] != context.DeadlineExceeded.Error() {
		t.Fatalf("failed query erased native evidence or its error: %v", evidence)
	}
}

func fsmControlSnapshotBranches(t *testing.T, test fsmRelayCase) {
	t.Helper()
	const neighbor = "172.30.0.10"
	document := fmt.Sprintf(`{%q:{"bgpState":"Established"}}`, neighbor)
	routes := fmt.Sprintf(`{"prefix":%q,"paths":[{}]}`, fsmWitnessPrefix)
	initial := fsmErrorTestReport(t, test)
	if err := judgeFSMControlSnapshot(document, routes, neighbor, initial, initial, test); err != nil {
		t.Fatalf("final live native control: %v", err)
	}
	for _, mutation := range []struct {
		name string
		edit func(*string, *string, *fsmRelayReport)
	}{
		{"final Idle", func(document, _ *string, _ *fsmRelayReport) {
			*document = strings.Replace(*document, "Established", "Idle", 1)
		}},
		{"final missing neighbor", func(document, _ *string, _ *fsmRelayReport) { *document = `{}` }},
		{"final empty route", func(_, routes *string, _ *fsmRelayReport) { *routes = `{}` }},
		{"final route without paths", func(_, routes *string, _ *fsmRelayReport) {
			*routes = fmt.Sprintf(`{"prefix":%q,"paths":[]}`, fsmWitnessPrefix)
		}},
		{"final Ze EOF", func(_, _ *string, report *fsmRelayReport) { report.ZeEOF = true }},
		{"final native EOF", func(_, _ *string, report *fsmRelayReport) { report.NativeEOF = true }},
		{"final stopped tail", func(_, _ *string, report *fsmRelayReport) { report.Problem = "FSM control relay forwarding ended" }},
		{"final replacement endpoint", func(_, _ *string, report *fsmRelayReport) { report.NativeEndpoint = "172.30.0.4:179" }},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			final := fsmErrorTestReport(t, test)
			finalDocument, finalRoutes := document, routes
			mutation.edit(&finalDocument, &finalRoutes, &final)
			if err := judgeFSMControlSnapshot(finalDocument, finalRoutes, neighbor, initial, final, test); err == nil {
				t.Fatal("stale polling success concealed a bad final control snapshot")
			}
		})
	}
}

func fsmErrorTestReport(t *testing.T, test fsmRelayCase) fsmRelayReport {
	t.Helper()
	ze, err := speakerOpen(65001, 90, net.ParseIP("172.30.0.2"), nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	native, err := speakerOpen(65002, 90, net.ParseIP("172.30.0.3"), nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	report := fsmRelayReport{Personality: test.personality, Finished: true, ZeEndpoint: "172.30.0.2:179", NativeEndpoint: "172.30.0.3:179"}
	add := func(source string, frame []byte, delivered, replay bool) {
		report.Frames = append(report.Frames, fsmRelayFrame{Source: source, Frame: hex.EncodeToString(frame), Delivered: delivered, Replay: replay})
	}
	add(fsmNative, native, test.subcode != 1, false)
	add(fsmZe, ze, true, false)
	if test.subcode != 1 {
		add(fsmZe, speakerKeepalive(), true, false)
	}
	add(fsmNative, speakerKeepalive(), test.subcode != 2, false)
	if test.subcode == 0 || test.subcode == 3 {
		add(fsmZe, speakerMessage(bgpUpdate, buildSpeakerUpdate(nil, []byte{24, 10, 66, 8})), true, false)
	}
	if test.subcode != 0 {
		if test.subcode != 1 {
			add(fsmNative, native, true, true)
		}
		add(fsmZe, speakerMessage(bgpNotification, []byte{5, test.subcode, test.message}), true, false)
		report.ZeEOF, report.NativeEOF = true, true
	}
	return report
}

func TestFSMErrorRelayOracle(t *testing.T) { fsmErrorCheckerBranches(t) }
