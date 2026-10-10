package siteterminaldemo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestShowcaseLabFixturesReportReadyOnOutput checks the declaration the
// showcase lab starts from, without root or a network namespace: every
// fixture, including the customer that `announce` starts, has a unique name, a
// start function and at least one readiness observation of a known kind, so
// the lab never waits on a sleep. Starting the fixtures for real is
// validateShowcaseLab, run inside the renderer container.
func TestShowcaseLabFixturesReportReadyOnOutput(t *testing.T) {
	fixtures := append(showcaseLabFixtures(), showcaseCustomer())
	seen := map[string]bool{}
	for _, fixture := range fixtures {
		if fixture.name == "" {
			t.Fatal("a showcase lab fixture has no name")
		}
		if seen[fixture.name] {
			t.Errorf("fixture %s is declared twice", fixture.name)
		}
		seen[fixture.name] = true
		if fixture.start == nil {
			t.Errorf("fixture %s has no start function", fixture.name)
		}
		if len(fixture.ready) == 0 {
			t.Errorf("fixture %s declares no readiness observation", fixture.name)
		}
		for _, ready := range fixture.ready {
			if ready.target == "" {
				t.Errorf("fixture %s: a readiness observation has no target", fixture.name)
			}
			switch ready.kind {
			case readinessListen:
				if len(ready.argv) != 0 {
					t.Errorf("fixture %s: a listen observation carries a command %v", fixture.name, ready.argv)
				}
			case readinessAnswer:
				if len(ready.argv) == 0 {
					t.Errorf("fixture %s: an answer observation has no command", fixture.name)
				}
			case readinessUnspecified:
				t.Errorf("fixture %s: a readiness observation has no kind", fixture.name)
			}
		}
	}
}

// TestShowcaseLabFilesMatchItsFixtures reads the lab directory the runner
// loads: each file a fixture names exists, and frr.conf configures every FRR
// daemon the lab starts with the router-id the ospfd readiness waits for. A
// renamed file or a dropped section would otherwise fail only inside the
// container.
func TestShowcaseLabFilesMatchItsFixtures(t *testing.T) {
	lab := filepath.Join(repoDemoRoot, showcaseLabID)
	for _, name := range []string{"ze.conf", "frr.conf", "transit.msg", "customer.msg"} {
		if _, err := os.Stat(filepath.Join(lab, name)); err != nil {
			t.Errorf("showcase lab file %s: %v", name, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(lab, "frr.conf"))
	if err != nil {
		t.Fatalf("read frr.conf: %v", err)
	}
	frr := string(data)
	for _, section := range []string{"router bgp 65002", "\nbfd\n", "router ospf", "ospf router-id " + showcaseFRRRouterID, "neighbor 172.29.0.1 remote-as 65000"} {
		if !strings.Contains(frr, section) {
			t.Errorf("frr.conf lacks %q, which the lab's FRR fixture relies on", section)
		}
	}
}
