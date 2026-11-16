// Design: docs/labs/l2tp-interop.md -- exact scale topology and victim selection.
// Related: scale_observe.go -- independent full-population assertions.
package l2tp

import (
	"encoding/json"
	"fmt"
	"testing"
)

func scaleFullTestPopulation(t *testing.T) scalePopulation {
	t.Helper()
	rows := make([]scaleTestSession, 0, scaleLACCount*scaleSessionsPerLAC)
	for lac := 1; lac <= scaleLACCount; lac++ {
		for session := 1; session <= scaleSessionsPerLAC; session++ {
			rows = append(rows, scaleTestSession{
				Tunnel: uint16(lac * 101), Local: uint16(scaleSessionsPerLAC + 1 - session), Remote: uint16(session),
				Username: scaleUsername(lac, session), Address: fmt.Sprintf("10.100.0.%d", 1+(lac-1)*scaleSessionsPerLAC+session), State: "established",
			})
		}
	}
	return scaleTestPopulation(t, rows)
}

func TestScaleTopologyEvidence(t *testing.T) {
	population := scaleFullTestPopulation(t)
	if err := requireScaleTopology(population); err != nil {
		t.Fatal(err)
	}
	user := scaleUsername(2, 1)
	session := population.byUser[user]
	session.tunnel = 101
	population.byUser[user] = session
	if err := requireScaleTopology(population); err == nil {
		t.Fatal("one LAC spanning mixed tunnel identities accepted")
	}
}

func TestScaleConfiguredPoolEvidence(t *testing.T) {
	rows := scaleTestSessions()[:1]
	rows[0].Address = "192.0.2.1"
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readScaleSessions(raw, []string{rows[0].Username}); err == nil {
		t.Fatal("non-pool IPv4 address accepted")
	}
}

func TestScaleCollisionTargets(t *testing.T) {
	population := scaleFullTestPopulation(t)
	first, second, err := scaleCollisionTargets(population)
	if err != nil {
		t.Fatal(err)
	}
	if first != "scale-1-16" || second != "scale-2-16" {
		t.Fatalf("selected %s/%s, want actual colliding Ze SID1 sessions", first, second)
	}
	if population.byUser[first].remote == 1 {
		t.Fatal("fixture no longer distinguishes peer SID from Ze SID")
	}
}
