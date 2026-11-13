// Design: docs/architecture/testing/interop.md -- OSPF redistribution producer setup.
// Related: prepare_ospf_redistribute_default.go -- native prestart registration.
package bgp

import (
	"net/netip"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

func TestOSPFRedistributeDummyPreparedBeforeFRR(t *testing.T) {
	source := filepath.Join("..", "..", "..", "..", "test", "interop", "scenarios", ospfRedistributeDefaultScenario)
	prepared, err := prepareScenario(t.TempDir(), t.TempDir(), "fixture", interoplab.PrepareContext{
		Source:  interoplab.ScenarioSource{Name: ospfRedistributeDefaultScenario, Directory: source},
		Network: interoplab.Network{IPv4: netip.MustParsePrefix("172.30.44.0/24")},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := prepared.Cleanup(); err != nil {
			t.Error(err)
		}
	})
	found := false
	for _, peer := range prepared.Peers {
		if peer.Name != peerFRR {
			if strings.Contains(peer.Prestart, "ze-redist0") {
				t.Fatalf("producer setup leaked into peer %s", peer.Name)
			}
			continue
		}
		found = true
		composed := interoplab.ComposeStart(peer, nil, frrImageStart)
		if len(composed.Command) < 3 {
			t.Fatalf("FRR start has no native wrapper: %q", composed.Command)
		}
		script := composed.Command[1]
		previous := -1
		for _, step := range []string{"ip link add ze-redist0 type dummy\n", "ip link set dev ze-redist0 up\n", wrapperExec} {
			at := strings.Index(script, step)
			if at < 0 {
				t.Fatalf("FRR prestart lacks %q: %s", step, script)
			}
			if at <= previous {
				t.Fatalf("FRR prestart step %q is out of order: %s", step, script)
			}
			previous = at
		}
		if !slices.Equal(composed.Command[len(composed.Command)-3:], []string{"/sbin/tini", "--", "/usr/lib/frr/docker-start"}) {
			t.Fatalf("FRR image argv changed: %q", composed.Command)
		}
	}
	if !found {
		t.Fatal("native preparation omitted FRR")
	}
}

func TestOSPFRedistributeDummyPreparationRequiresOneFRR(t *testing.T) {
	for _, peers := range [][]interoplab.PeerConfig{
		{{Name: "ze"}},
		{{Name: peerFRR}, {Name: peerFRR}},
	} {
		if _, err := prepareOSPFRedistributeDefaultPeers(peers, t.TempDir()); err == nil {
			t.Fatalf("accepted invalid producer topology: %+v", peers)
		}
	}
	peers, err := prepareOSPFRedistributeDefaultPeers([]interoplab.PeerConfig{{Name: peerFRR, Prestart: "existing-setup\n"}}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(peers[0].Prestart, "existing-setup\n") {
		t.Fatalf("preparer lost existing setup: %q", peers[0].Prestart)
	}
}
