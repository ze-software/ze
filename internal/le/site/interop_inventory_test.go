// Design: docs/architecture/site-facts.md -- recorded partitions reach the site intact.
package site

import (
	"encoding/json"
	"strings"
	"testing"
)

// interopRecordedFixture holds deliberately different suite and protocol
// partitions of the same seventeen cases, independent of checkout contents.
func interopRecordedFixture() map[string]map[string]any {
	entries := make(map[string]map[string]any)
	for _, protocol := range []string{
		"bgp", "ospfv2", "ospfv3", "isis", "bfd", "bmp", "rpki", "vrrp", "dns",
		"dhcpv4", "dhcpv6", "ldp", "as112", "gtsm", "ipsec", "l2tp", "pppoe",
		"radius", "rsvpte", "topology", "other",
	} {
		entries["interop.protocols."+protocol] = map[string]any{"value": 0, "source": "recorded fixture"}
	}
	for key, count := range map[string]int{
		"interop.suite_count":    3,
		"interop.suites.interop": 14, "interop.suites.interop-radius": 2,
		"interop.suites.interop-radius-subscriber": 1,
		"interop.protocols.bgp":                    10, "interop.protocols.ospfv3": 4, "interop.protocols.radius": 3,
	} {
		entries[key] = map[string]any{"value": count, "source": "recorded fixture"}
	}
	return entries
}

// readInteropRecordedFixture supplies bytes rather than a source directory, so
// the fixture's sentinel values can only come from the committed inventory.
func readInteropRecordedFixture(t *testing.T, entries map[string]map[string]any) (siteFacts, error) {
	t.Helper()
	content, err := json.Marshal(map[string]any{"facts": entries})
	if err != nil {
		t.Fatal(err)
	}
	var facts siteFacts
	facts.Sources = make(map[string]string)
	facts.Interop.Scenarios = 17
	err = factsFromInteropInventory(content, &facts)
	return facts, err
}

// Protocol and suite partitions keep their recorded values and provenance;
// RADIUS is one protocol row spanning two independent suites.
func TestInteropSnapshotUsesRecordedPartitions(t *testing.T) {
	facts, err := readInteropRecordedFixture(t, interopRecordedFixture())
	if err != nil {
		t.Fatal(err)
	}
	if facts.Interop.SuiteCount != 3 || len(facts.Interop.Suites) != 3 {
		t.Errorf("suite inventory = %+v", facts.Interop)
	}
	if facts.Interop.Protocols["bgp"] != 10 || facts.Interop.Protocols["ospfv3"] != 4 || facts.Interop.Protocols["radius"] != 3 {
		t.Errorf("recorded protocol inventory = %v", facts.Interop.Protocols)
	}
	for _, key := range []string{"interop.suite_count", "interop.suites.interop-radius", "interop.protocols.bgp"} {
		if !strings.Contains(facts.Sources[key], "repo-facts.json: recorded fixture") {
			t.Errorf("%s provenance = %q", key, facts.Sources[key])
		}
	}
}

// Missing, null and negative measurements cannot masquerade as zero; mismatched
// partitions cannot publish a total that the visible breakdown does not explain.
func TestInteropSnapshotRejectsIncompleteOrInconsistentPartitions(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(map[string]map[string]any)
		want   string
	}{
		{"missing-suite-count", func(entries map[string]map[string]any) { delete(entries, "interop.suite_count") }, "interop.suite_count"},
		{"missing-protocol", func(entries map[string]map[string]any) { delete(entries, "interop.protocols.ospfv3") }, "interop.protocols.ospfv3"},
		{"null-protocol", func(entries map[string]map[string]any) { entries["interop.protocols.bgp"]["value"] = nil }, "interop.protocols.bgp"},
		{"negative-protocol", func(entries map[string]map[string]any) { entries["interop.protocols.bgp"]["value"] = -1 }, "interop.protocols.bgp"},
		{"protocol-sum", func(entries map[string]map[string]any) { entries["interop.protocols.bgp"]["value"] = 9 }, "interop.protocols. total"},
		{"unknown-protocol", func(entries map[string]map[string]any) {
			entries["interop.protocols.bgp"]["value"] = 9
			entries["interop.protocols.unreviewed"] = map[string]any{"value": 1}
		}, "unknown protocol key interop.protocols.unreviewed"},
		{"suite-sum", func(entries map[string]map[string]any) { entries["interop.suites.interop"]["value"] = 13 }, "interop.suites. total"},
		{"suite-count", func(entries map[string]map[string]any) { entries["interop.suite_count"]["value"] = 4 }, "interop.suite_count disagrees"},
		{"empty-suite", func(entries map[string]map[string]any) {
			entries["interop.suites.interop-unused"] = map[string]any{"value": 0}
		}, "scenario-free suite"},
	} {
		t.Run(test.name, func(t *testing.T) {
			entries := interopRecordedFixture()
			test.mutate(entries)
			_, err := readInteropRecordedFixture(t, entries)
			if err == nil {
				t.Fatal("accepted an incomplete or inconsistent inventory")
			}
			if !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), "./le site facts update") {
				t.Errorf("refusal = %v, want %s and regeneration command", err, test.want)
			}
		})
	}
}

// Newly recorded suites and an explicit Other area survive publication without
// a second, closed list in the site reader silently dropping their scenarios.
func TestInteropSnapshotRetainsFutureSuiteAndOtherArea(t *testing.T) {
	entries := interopRecordedFixture()
	entries["interop.suites.interop-radius"]["value"] = 1
	entries["interop.suites.interop-future"] = map[string]any{"value": 1, "source": "future fixture"}
	entries["interop.suite_count"]["value"] = 4
	entries["interop.protocols.radius"]["value"] = 2
	entries["interop.protocols.other"]["value"] = 1
	facts, err := readInteropRecordedFixture(t, entries)
	if err != nil {
		t.Fatal(err)
	}
	if facts.Interop.Suites["interop-future"] != 1 || facts.Interop.Protocols["other"] != 1 {
		t.Errorf("future cases were lost: %+v", facts.Interop)
	}
}
