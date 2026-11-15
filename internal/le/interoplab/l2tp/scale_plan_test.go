// Design: docs/labs/l2tp-interop.md -- many real PPP sessions in each L2TP tunnel.
// Related: register_scale.go -- the scenario's complete plan declaration.
package l2tp

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestScaleScenarioPlan follows scenario discovery through preparation and
// checks that every LAC, relay and independent observer belongs to cleanup.
func TestScaleScenarioPlan(t *testing.T) {
	const name = "l2tp-many-sessions-kl2tpd"
	root := t.TempDir()
	directory := filepath.Join(root, filepath.FromSlash(scenarioDirectory), name)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	environment := testEnvironment()
	plans, needsKernel, err := plansAt(root, name, environment)
	if err != nil {
		t.Fatalf("discover scale scenario: %v", err)
	}
	if !needsKernel {
		t.Fatal("the scale scenario bypassed the PPPoL2TP prerequisite")
	}
	if len(plans) != 1 {
		t.Fatalf("selected %d scenarios, want only %s", len(plans), name)
	}
	plan := plans[0]
	prepared, err := plan.Prepare(context.Background(), interoplab.PrepareContext{
		Source: plan.Source,
		Network: interoplab.Network{
			Name: "scale-test", IPv4: netip.MustParsePrefix("172.29.0.0/24"),
		},
	})
	if err != nil {
		t.Fatalf("prepare scale scenario: %v", err)
	}
	if prepared.Cleanup != nil {
		t.Cleanup(func() {
			if err := prepared.Cleanup(); err != nil {
				t.Error(err)
			}
		})
	}
	if len(prepared.Peers) != 9 {
		t.Fatalf("prepared %d peers, want Ze, FRR, RADIUS, three LACs and three relays", len(prepared.Peers))
	}
	if len(plan.Containers) != len(prepared.Peers) {
		t.Fatalf("cleanup declares %d containers for %d peers", len(plan.Containers), len(prepared.Peers))
	}
	hosts := make(map[uint8]string, len(prepared.Peers))
	for _, peer := range prepared.Peers {
		if !slices.Contains(plan.Containers, peer.Container) {
			t.Errorf("cleanup omits peer %s container %s", peer.Name, peer.Container)
		}
		if other, exists := hosts[peer.Host]; exists {
			t.Errorf("peers %s and %s share host %d", other, peer.Name, peer.Host)
		}
		hosts[peer.Host] = peer.Name
	}
	for _, peer := range []string{"ze", "frr", "radius", "kl2tpd-1", "kl2tpd-2", "kl2tpd-3", "scale-relay-1", "scale-relay-2", "scale-relay-3"} {
		peerByName(t, prepared.Peers, peer)
	}
	builds := imageBuilds(root, plans, environment.Image)
	for _, peer := range prepared.Peers {
		if !slices.ContainsFunc(builds, func(build interoplab.ImageBuild) bool {
			return build.Name == peer.Image && build.Required
		}) {
			t.Errorf("peer %s image %s is not a required build", peer.Name, peer.Image)
		}
	}
}
