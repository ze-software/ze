package bgp

import (
	"net/netip"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestScenarioFRRImagePinsItsRelease validates that a scenario's frr-image
// file starts its FRR container from the pinned release, which the suite pulls,
// while a scenario without one keeps the suite's FRR.
// Method: two fixture scenarios through scenarioPeers and pinnedFRRImages.
// PREVENTS: a pin read by nothing, so the scenario runs the suite's FRR and a
// capability only the pinned release speaks is never negotiated.
func TestScenarioFRRImagePinsItsRelease(t *testing.T) {
	const reference = "quay.io/frrouting/frr:10.4.1"
	producer := t.TempDir()
	pinned := t.TempDir()
	plain := t.TempDir()
	for _, scenario := range []string{pinned, plain} {
		writeFixture(t, filepath.Join(scenario, "ze.conf"), "bgp {}\n")
		writeFixture(t, filepath.Join(scenario, "frr.conf"), "router bgp 65002\n")
	}
	writeFixture(t, filepath.Join(pinned, frrImageFile), reference+"\n")
	network := interoplab.Network{Name: "lab", IPv4: netip.MustParsePrefix("172.31.22.0/24")}

	for scenario, want := range map[string]string{pinned: frrImageName(reference), plain: peerFRR} {
		peers, err := scenarioPeers(producer, scenario, "fixture", network)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, peer := range peers {
			if peer.Name != peerFRR {
				continue
			}
			found = true
			if peer.Image != want {
				t.Errorf("FRR image = %q, want %q", peer.Image, want)
			}
		}
		if !found {
			t.Fatalf("scenario %s started no FRR container", scenario)
		}
	}

	images, err := pinnedFRRImages([]interoplab.ScenarioSource{{Directory: pinned}, {Directory: plain}, {Directory: pinned}})
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 1 {
		t.Fatalf("pinned images = %+v, want one pull of %s", images, reference)
	}
	if images[0].Name != frrImageName(reference) || images[0].Tag != reference || !images[0].Pull {
		t.Fatalf("pinned image = %+v, want a pull of %s under %s", images[0], reference, frrImageName(reference))
	}
}

// TestScenarioFRRImageRefusesAnEmptyPin validates that a frr-image file naming
// no image, or two, is an error rather than a silent fall back to the suite's
// FRR.
func TestScenarioFRRImageRefusesAnEmptyPin(t *testing.T) {
	for _, content := range []string{"\n", "a:1 b:2\n"} {
		scenario := t.TempDir()
		writeFixture(t, filepath.Join(scenario, frrImageFile), content)
		if image, err := scenarioFRRImage(scenario); err == nil {
			t.Errorf("frr-image %q answered %q, want an error", content, image)
		}
	}
}
