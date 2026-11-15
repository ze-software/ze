// Design: docs/architecture/testing/interop.md -- fail-closed live capability reload proof.
// Related: check_role_reload.go -- the checker driven through a controlled foreign peer.
package bgp

import (
	"context"
	"fmt"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestRoleReloadCheckerSequence proves the checker observes both directions of
// capability replacement. Method: a controlled lab changes the foreign peer's
// received Role and session generation only when it sees a real reload signal.
func TestRoleReloadCheckerSequence(t *testing.T) {
	lab := &roleReloadLab{generation: 4, role: "undefined", outcome: "applied"}
	if err := checkRoleReload(t.Context(), roleReloadCheck(lab)); err != nil {
		t.Fatal(err)
	}
	if lab.signals != 2 {
		t.Fatalf("signals = %d, want two reloads", lab.signals)
	}
	want := []string{zeMountedReloadConfig, zeMountedConfig}
	if !slices.Equal(lab.configs, want) {
		t.Fatalf("reload configs = %v, want %v", lab.configs, want)
	}
	if lab.generation != 6 {
		t.Fatalf("established generation = %d, want 6", lab.generation)
	}
}

// TestRoleReloadCheckerRefusesMissingEvidence proves neither a successful
// reload alone nor the previous session can satisfy the interop assertion.
// Method: each controlled peer omits one required transition; a short context
// ends the normal state wait without replacing its production timeout.
func TestRoleReloadCheckerRefusesMissingEvidence(t *testing.T) {
	for _, test := range []struct {
		name     string
		onReload func(*roleReloadLab)
	}{
		{name: "unchanged Role", onReload: func(lab *roleReloadLab) { lab.role = "undefined" }},
		{name: "missing Role", onReload: func(lab *roleReloadLab) { lab.role = "" }},
		{name: "unchanged session", onReload: func(lab *roleReloadLab) { lab.generation-- }},
		{name: "refused reload", onReload: func(lab *roleReloadLab) { lab.outcome = "refused" }},
		{name: "removal retained Role", onReload: func(lab *roleReloadLab) {
			if lab.signals == 2 {
				lab.role = "provider"
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
			defer cancel()
			lab := &roleReloadLab{generation: 4, role: "undefined", outcome: "applied", onReload: test.onReload}
			if err := checkRoleReload(ctx, roleReloadCheck(lab)); err == nil {
				t.Fatal("checker accepted missing reload evidence")
			}
		})
	}
}

// roleReloadLab serves the existing native command boundaries used by the
// checker. It inherits irrelevant lab operations and diagnostics from the
// package's recordingLab; no container or local server is started.
type roleReloadLab struct {
	recordingLab
	generation       uint64
	reloadGeneration uint64
	role             string
	outcome          string
	configs          []string
	signals          int
	onReload         func(*roleReloadLab)
}

func roleReloadCheck(lab *roleReloadLab) *interoplab.CheckContext {
	return &interoplab.CheckContext{
		Lab:     lab,
		Source:  interoplab.ScenarioSource{Name: roleReloadScenario},
		Network: interoplab.Network{IPv4: netip.MustParsePrefix("172.30.0.0/24")},
	}
}

func (lab *roleReloadLab) Query(_ context.Context, peer string, _ []string, _ []interoplab.EnvironmentVariable) (string, error) {
	if peer == peerFRR {
		return fmt.Sprintf(`{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":%d,"localRole":"customer","remoteRole":%q}}`, lab.generation, lab.role), nil
	}
	if peer == "ze" {
		return fmt.Sprintf(`{"generation":%d,"last-outcome":%q}`, lab.reloadGeneration, lab.outcome), nil
	}
	return "", fmt.Errorf("unexpected query peer %s", peer)
}

func (lab *roleReloadLab) Exec(_ context.Context, peer string, command []string, _ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	if peer != "ze" {
		return interoplab.CommandResult{}, fmt.Errorf("unexpected exec peer %s", peer)
	}
	if len(command) != 3 {
		return interoplab.CommandResult{}, fmt.Errorf("unexpected command %v", command)
	}
	if command[0] != "cp" {
		return interoplab.CommandResult{}, fmt.Errorf("unexpected command %v", command)
	}
	lab.configs = append(lab.configs, command[1])
	return interoplab.CommandResult{}, nil
}

func (lab *roleReloadLab) Signal(_ context.Context, peer, signal string) error {
	if peer != "ze" {
		return fmt.Errorf("unexpected signal peer %s", peer)
	}
	if signal != signalHUP {
		return fmt.Errorf("unexpected signal %s", signal)
	}
	lab.signals++
	lab.reloadGeneration++
	lab.generation++
	lab.role = "undefined"
	if lab.configs[len(lab.configs)-1] == zeMountedReloadConfig {
		lab.role = "provider"
	}
	if lab.onReload != nil {
		lab.onReload(lab)
	}
	return nil
}
