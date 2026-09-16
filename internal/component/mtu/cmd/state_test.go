// Design: docs/architecture/diagnostics/path-mtu.md -- the local state tests
// Related: state.go -- readFragmentationCounters, tcpMTUProbing, pathMTUCacheFinding
//
// VALIDATES: the fragmentation counters are read as absolute values with the
// ported tool's severities (AC-20), tcp_mtu_probing parses into a closed set,
// and the cache note distinguishes a stale cached PMTU from an agreeing one
// (AC-11).
// PREVENTS: a missing counter reading as 0, an out-of-range sysctl value
// passing for a setting, and a cached value that disagrees with the wire
// going unmentioned.

package cmd

import (
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// snmpFixture writes a proc tree holding the IPv4 and IPv6 SNMP tables with
// the named counters, in the layout /proc/self/net/{snmp,snmp6} takes.
func snmpFixture(t *testing.T, v4, v6 map[string]string) string {
	t.Helper()
	root := t.TempDir()
	// procfs resolves "self" through a readlink, as the kernel does, so the
	// fixture makes it a link to a pid directory.
	dir := filepath.Join(root, "1", "net")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("1", filepath.Join(root, "self")); err != nil {
		t.Fatal(err)
	}
	header := "Ip: Forwarding DefaultTTL ReasmReqds ReasmFails FragOKs FragFails FragCreates"
	values := []string{"Ip:", "1", "64", v4["ReasmReqds"], v4["ReasmFails"], v4["FragOKs"], v4["FragFails"], v4["FragCreates"]}
	snmp := header + "\n" + strings.Join(values, " ") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "snmp"), []byte(snmp), 0o600); err != nil {
		t.Fatal(err)
	}
	var snmp6 strings.Builder
	for name, value := range v6 {
		snmp6.WriteString(name + "\t" + value + "\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "snmp6"), []byte(snmp6.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestFragmentationCountersAreAbsoluteWithSeverities reads a fixture with
// every counter set and checks the values are the file's, not a rate, and
// that each non-zero counter earns the severity the ported tool gave it.
func TestFragmentationCountersAreAbsoluteWithSeverities(t *testing.T) {
	root := snmpFixture(t,
		map[string]string{"ReasmReqds": "40", "ReasmFails": "3", "FragOKs": "120", "FragFails": "7", "FragCreates": "360"},
		map[string]string{"Ip6FragOKs": "5", "Ip6ReasmFails": "1"})
	got, err := readFragmentationCounters(root)
	if err != nil {
		t.Fatalf("readFragmentationCounters: %v", err)
	}
	want := fragmentationCounters{ipFragOKs: 120, ip6FragOKs: 5, ipFragCreates: 360, ipFragFails: 7, ipReasmReqds: 40, ipReasmFails: 3, ip6ReasmFails: 1}
	if got != want {
		t.Fatalf("counters = %+v, want %+v", got, want)
	}
	notes := got.findings()
	if len(notes) != 3 {
		t.Fatalf("findings = %d notes, want 3: %+v", len(notes), notes)
	}
	wantSeverity := []noteSeverity{noteSeverityCaution, noteSeverityFault, noteSeverityCaution}
	wantText := []string{"IpFragOKs 120", "IpFragFails 7", "IpReasmFails 3 of IpReasmReqds 40"}
	for i, n := range notes {
		if n.severity != wantSeverity[i] {
			t.Errorf("note %d: severity %s, want %s", i, n.severity, wantSeverity[i])
		}
		if !strings.Contains(n.text, wantText[i]) {
			t.Errorf("note %d: %q does not name %q", i, n.text, wantText[i])
		}
	}
	// A second read answers the same absolute values, never a delta.
	again, err := readFragmentationCounters(root)
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if again != want {
		t.Errorf("second read = %+v, want the same absolute values", again)
	}
}

// TestFragmentationCountersZeroIsInformation proves all-zero counters earn
// one information note, so the reader can tell "read and zero" from "not
// read".
func TestFragmentationCountersZeroIsInformation(t *testing.T) {
	root := snmpFixture(t,
		map[string]string{"ReasmReqds": "0", "ReasmFails": "0", "FragOKs": "0", "FragFails": "0", "FragCreates": "0"},
		map[string]string{"Ip6FragOKs": "0", "Ip6ReasmFails": "0"})
	got, err := readFragmentationCounters(root)
	if err != nil {
		t.Fatalf("readFragmentationCounters: %v", err)
	}
	notes := got.findings()
	if len(notes) != 1 {
		t.Fatalf("findings = %d notes, want 1: %+v", len(notes), notes)
	}
	if notes[0].severity != noteSeverityInfo {
		t.Errorf("severity %s, want info", notes[0].severity)
	}
	// Only IPv6 fragmenting is still a caution.
	got.ip6FragOKs = 2
	notes = got.findings()
	if len(notes) != 1 || notes[0].severity != noteSeverityCaution {
		t.Errorf("ip6 fragmentation alone: %+v, want one caution", notes)
	}
}

// TestFragmentationCountersAbsentIsAnError proves a counter the kernel does
// not report, and a table that cannot be read, each answer an error rather
// than a zero.
func TestFragmentationCountersAbsentIsAnError(t *testing.T) {
	root := snmpFixture(t,
		map[string]string{"ReasmReqds": "0", "ReasmFails": "0", "FragOKs": "0", "FragFails": "0", "FragCreates": "0"},
		map[string]string{"Ip6ReasmFails": "0"})
	if _, err := readFragmentationCounters(root); !errors.Is(err, errCounterAbsent) {
		t.Errorf("missing Ip6FragOKs: err = %v, want errCounterAbsent", err)
	}
	if _, err := readFragmentationCounters(t.TempDir()); err == nil {
		t.Error("an empty proc tree answered counters")
	}
}

// TestTCPMTUProbingParsesTheClosedSet pins the three kernel values and
// refuses anything else.
func TestTCPMTUProbingParsesTheClosedSet(t *testing.T) {
	for raw, want := range map[string]tcpMTUProbing{"0": tcpMTUProbingDisabled, "1": tcpMTUProbingOnBlackhole, "2": tcpMTUProbingAlways} {
		got, err := parseTCPMTUProbing(raw)
		if err != nil {
			t.Errorf("parse %q: %v", raw, err)
			continue
		}
		if got != want {
			t.Errorf("parse %q = %s, want %s", raw, got, want)
		}
	}
	for _, raw := range []string{"3", "-1", "", "on", "0\n"} {
		if got, err := parseTCPMTUProbing(raw); err == nil {
			t.Errorf("parse %q = %s with no error", raw, got)
		}
	}
	note, has := tcpMTUProbingDisabled.finding()
	if !has || note.severity != noteSeverityInfo {
		t.Errorf("disabled: finding = %+v, %v; want one info note", note, has)
	}
	if _, has := tcpMTUProbingAlways.finding(); has {
		t.Error("always earned a note")
	}
}

// TestPathMTUCacheFinding pins the cache note: stale is a caution naming the
// expiry when known, agreeing is information unless exhaustive, none is no note.
func TestPathMTUCacheFinding(t *testing.T) {
	target := netip.MustParseAddr("192.0.2.9")
	stale := pathMTUCache{cached: 1400, hasCached: true}
	note, has := pathMTUCacheFinding(target, &stale, 1500, false, 600)
	if !has || note.severity != noteSeverityCaution {
		t.Fatalf("stale: %+v, %v; want a caution", note, has)
	}
	for _, want := range []string{"192.0.2.9", "1400", "1500", "(600s)"} {
		if !strings.Contains(note.text, want) {
			t.Errorf("stale note %q does not name %q", note.text, want)
		}
	}
	note, _ = pathMTUCacheFinding(target, &stale, 1500, false, 0)
	if strings.Contains(note.text, "s)") {
		t.Errorf("stale note with no expiry named a duration: %q", note.text)
	}
	agree := pathMTUCache{cached: 1492, hasCached: true}
	note, has = pathMTUCacheFinding(target, &agree, 1492, false, 600)
	if !has || note.severity != noteSeverityInfo {
		t.Errorf("agreeing: %+v, %v; want information", note, has)
	}
	if _, has := pathMTUCacheFinding(target, &agree, 1492, true, 600); has {
		t.Error("agreeing under exhaustive earned a note")
	}
	if _, has := pathMTUCacheFinding(target, &pathMTUCache{}, 1500, false, 600); has {
		t.Error("no cached value earned a note")
	}
}
