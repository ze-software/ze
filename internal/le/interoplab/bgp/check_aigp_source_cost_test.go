// Design: docs/architecture/testing/interop.md -- fail-closed independent AIGP proof.
package bgp

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func aigpSourceCostCheckerBranches(t *testing.T) {
	t.Run("foreign-attribute-identity", aigpForeignAttributeIdentity)
	t.Run("whole-route-absence", aigpWholeRouteAbsence)
	t.Run("source-traffic-fence", aigpSourceTrafficFence)
	t.Run("recipient-session-fence", aigpRecipientSessionFence)
}

// aigpForeignAttributeIdentity rejects values detached from their route,
// wrong provenance, missing attributes and substring-only metric matches.
func aigpForeignAttributeIdentity(t *testing.T) {
	const route = `{"prefix":"10.10.2.0/24","paths":[{"aigpMetric":107,"aspath":{"string":"65001 65004"},"peer":{"peerId":"172.31.22.2"},"nexthops":[{"ip":"172.31.22.2"}]}]}`
	if err := requireFRRAIGPRoute(route, aigpDirectPrefix, "172.31.22.2", 107); err != nil {
		t.Fatal(err)
	}
	for name, output := range map[string]string{
		"unanswered": "", "null": "null", "empty": "{}",
		"destination-cost": strings.Replace(route, "107", "143", 1),
		"double-count":     strings.Replace(route, "107", "114", 1),
		"no-attribute":     strings.Replace(route, `"aigpMetric":107,`, "", 1),
		"wrong-prefix":     strings.Replace(route, "10.10.2.0", "10.10.3.0", 1),
		"wrong-source":     strings.Replace(route, `"peerId":"172.31.22.2"`, `"peerId":"172.31.22.9"`, 1),
		"wrong-next-hop":   strings.Replace(route, `"ip":"172.31.22.2"`, `"ip":"172.31.22.9"`, 1),
		"wrong-path":       strings.Replace(route, "65001 65004", "65001 65005", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if requireFRRAIGPRoute(output, aigpDirectPrefix, "172.31.22.2", 107) == nil {
				t.Fatalf("accepted unrelated/malformed FRR attribute: %s", output)
			}
		})
	}
	const control = "*> 10.10.2.0/24 172.31.22.9 65001 65004 {Origin: i} {Aigp: [{Metric: 100}]}\n*> 10.10.3.0/24 172.31.22.77 65001 65004 {Origin: i} {Aigp: [{Metric: 100}]}\n"
	if err := requireGoBGPAIGPControl(control, "172.31.22.9", "172.31.22.77"); err != nil {
		t.Fatal(err)
	}
	for name, output := range map[string]string{
		"empty": "", "duplicate-route": control + control,
		"metric-prefix":  strings.ReplaceAll(control, "Metric: 100", "Metric: 1000"),
		"changed-metric": strings.Replace(control, "Metric: 100", "Metric: 107", 1),
		"missing-metric": strings.Replace(control, "{Aigp: [{Metric: 100}]}", "", 1),
		"wrong-hop":      strings.Replace(control, "172.31.22.77", "172.31.22.2", 1),
		"other-prefix":   strings.Replace(control, "10.10.3.0/24", "110.10.3.0/24", 1),
	} {
		t.Run("control/"+name, func(t *testing.T) {
			if requireGoBGPAIGPControl(output, "172.31.22.9", "172.31.22.77") == nil {
				t.Fatalf("accepted incorrect GoBGP control: %s", output)
			}
		})
	}
}

// aigpWholeRouteAbsence requires the retained original routes and the later
// FIFO sentinel; absence of only the AIGP attribute never establishes withdrawal.
func aigpWholeRouteAbsence(t *testing.T) {
	const withheld = `{"routes":{"10.10.0.0/24":[{}],"10.10.1.0/24":[{}],"10.10.2.0/24":[{}],"10.10.4.0/24":[{}]}}`
	const recovered = `{"routes":{"10.10.0.0/24":[{}],"10.10.1.0/24":[{}],"10.10.2.0/24":[{}],"10.10.3.0/24":[{}],"10.10.4.0/24":[{}]}}`
	if err := requireFRRAIGPInventory(withheld, false); err != nil {
		t.Fatal(err)
	}
	if err := requireFRRAIGPInventory(recovered, true); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{"", "{}", "null", `{"routes":{}}`, recovered,
		strings.Replace(withheld, `"10.10.4.0/24":[{}]`, `"10.10.4.0/24":[]`, 1)} {
		if requireFRRAIGPInventory(output, false) == nil {
			t.Fatalf("accepted unproven whole-route absence: %s", output)
		}
	}
	if requireFRRAIGPInventory(withheld, true) == nil {
		t.Fatal("absence passed as recovery")
	}
}

// aigpSourceTrafficFence refuses absent counters, a reset session and an
// unfinished source batch, and compares received UPDATEs across metric control.
func aigpSourceTrafficFence(t *testing.T) {
	const original = `{"peers":{"172.31.22.9":{"state":"established","updates-received":4,"eor-received":1,"connections-established":1,"connections-dropped":0}}}`
	before, err := readAIGPSourceFence(original, "172.31.22.9")
	if err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{"", "{}", "null",
		strings.Replace(original, `"connections-dropped":0`, `"unrelated":0`, 1),
		strings.Replace(original, `"connections-established":1`, `"connections-established":2`, 1),
		strings.Replace(original, `"eor-received":1`, `"eor-received":0`, 1),
		strings.Replace(original, `"updates-received":4`, `"updates-received":3`, 1),
		strings.Replace(original, `"state":"established"`, `"state":"idle"`, 1)} {
		if _, err := readAIGPSourceFence(output, "172.31.22.9"); err == nil {
			t.Fatalf("accepted source fence without live complete evidence: %s", output)
		}
	}
	after, err := readAIGPSourceFence(strings.Replace(original, `"updates-received":4`, `"updates-received":5`, 1), "172.31.22.9")
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("new source UPDATE did not invalidate the recovery fence")
	}
}

// TestAIGPScenarioSelectedNetwork renders the actual injector/config fixture,
// not a copy, and requires both received next hops to follow the chosen subnet.
func TestAIGPScenarioSelectedNetwork(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "test", "interop", "scenarios", scenarioAIGPSourceCostFRR)
	target := t.TempDir()
	network := renumberedNetwork()
	if err := renderScenario(root, target, network); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(target, "inject.msg"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "400304AC1E00") {
		t.Fatal("rendered injector retained a default-subnet next hop")
	}
	for _, host := range []uint8{9, 77} {
		address := network.IPv4.Addr().As4()
		address[3] = host
		// Read the rendered wire with the existing encoding, preserving every
		// source metric and NLRI while replacing only the next-hop address.
		want := strings.ToUpper(hex.EncodeToString(address[:]))
		if !strings.Contains(string(content), "400304"+want+"801A0B01000B0000000000000064") {
			t.Fatalf("rendered AIGP source lacks next hop %s", want)
		}
	}
}

// aigpRecipientSessionFence requires the same recipient connection even when
// its state is Established again and the route inventory looks correct.
func aigpRecipientSessionFence(t *testing.T) {
	const address = "172.31.22.2"
	const initial = `{"172.31.22.2":{"bgpState":"Established","connectionsEstablished":3,"connectionsDropped":2,"portLocal":179,"portForeign":42001}}`
	want, err := readFRRAIGPRecipientFence(initial, address)
	if err != nil {
		t.Fatal(err)
	}
	if err := requireFRRAIGPRecipientFence(initial, address, want); err != nil {
		t.Fatal(err)
	}
	for name, output := range map[string]string{
		"empty": "", "null": "null", "object": "{}",
		"wrong-neighbor":                 strings.Replace(initial, address, "172.31.22.9", 1),
		"missing-established":            strings.Replace(initial, `"connectionsEstablished":3,`, "", 1),
		"missing-dropped":                strings.Replace(initial, `"connectionsDropped":2,`, "", 1),
		"null-established":               strings.Replace(initial, `"connectionsEstablished":3`, `"connectionsEstablished":null`, 1),
		"null-dropped":                   strings.Replace(initial, `"connectionsDropped":2`, `"connectionsDropped":null`, 1),
		"missing-local-port":             strings.Replace(initial, `"portLocal":179,`, "", 1),
		"missing-foreign-port":           strings.Replace(initial, `"portForeign":42001`, `"unrelated":42001`, 1),
		"null-local-port":                strings.Replace(initial, `"portLocal":179`, `"portLocal":null`, 1),
		"null-foreign-port":              strings.Replace(initial, `"portForeign":42001`, `"portForeign":null`, 1),
		"zero-local-port":                strings.Replace(initial, `"portLocal":179`, `"portLocal":0`, 1),
		"invalid-foreign-port":           strings.Replace(initial, `"portForeign":42001`, `"portForeign":65536`, 1),
		"idle":                           strings.Replace(initial, `"bgpState":"Established"`, `"bgpState":"Idle"`, 1),
		"new-connection":                 strings.Replace(initial, `"connectionsEstablished":3`, `"connectionsEstablished":4`, 1),
		"dropped":                        strings.Replace(initial, `"connectionsDropped":2`, `"connectionsDropped":3`, 1),
		"counter-reset":                  strings.Replace(initial, `"connectionsEstablished":3`, `"connectionsEstablished":1`, 1),
		"drop-counter-reset":             strings.Replace(initial, `"connectionsDropped":2`, `"connectionsDropped":0`, 1),
		"same-counters-new-local-port":   strings.Replace(initial, `"portLocal":179`, `"portLocal":42002`, 1),
		"same-counters-new-foreign-port": strings.Replace(initial, `"portForeign":42001`, `"portForeign":42002`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if requireFRRAIGPRecipientFence(output, address, want) == nil {
				t.Fatalf("accepted missing or replacement recipient session: %s", output)
			}
		})
	}
}
