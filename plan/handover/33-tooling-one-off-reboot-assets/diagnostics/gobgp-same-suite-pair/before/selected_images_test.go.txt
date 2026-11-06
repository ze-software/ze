// Design: docs/architecture/testing/interop.md -- native selected scenario images.
package bgp

import (
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestBGPSuiteSelectedImagesUsePreparedDependencies checks explicit and
// environment selectors through suiteFor and the actual dynamic preparer.
// Default and version-pinned FRR must resolve through their own declarations;
// relays use Ze, and an unselected full suite retains eager preparation.
func TestBGPSuiteSelectedImagesUsePreparedDependencies(t *testing.T) {
	const plain = "ospf-opaque-frr"
	const pinned = "bgp-extended-message-bilateral-frr"
	const reference = "quay.io/frrouting/frr:10.4.1"
	root := t.TempDir()
	for _, name := range []string{plain, pinned} {
		directory := filepath.Join(root, "test", "interop", "scenarios", name)
		if err := os.MkdirAll(directory, 0o750); err != nil {
			t.Fatal(err)
		}
		writeFixture(t, filepath.Join(directory, "ze.conf"), "bgp {}\n")
		writeFixture(t, filepath.Join(directory, "frr.conf"), "router bgp 65002\n")
	}
	pinnedDirectory := filepath.Join(root, "test", "interop", "scenarios", pinned)
	writeFixture(t, filepath.Join(pinnedDirectory, frrImageFile), reference+"\n")
	writeFixture(t, filepath.Join(pinnedDirectory, speakerArgsFile), "--hold-time 90\n")
	for _, row := range []struct {
		name, explicit, environment string
		selected                    bool
		want                        []string
	}{
		{name: "explicit", explicit: plain, selected: true, want: []string{plain}},
		{name: "environment pin", environment: pinned, selected: true, want: []string{pinned}},
		{name: "explicit wins", explicit: plain, environment: pinned, selected: true, want: []string{plain}},
		{name: "full suite", want: []string{pinned, plain}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Setenv("INTEROP_SCENARIO", row.environment)
			t.Setenv("FRR_IMAGE", "default-frr:fixture")
			suite, err := suiteFor(root, Options{Scenario: row.explicit, Suffix: "image-closure"})
			if err != nil {
				t.Fatal(err)
			}
			if suite.ImagesOnDemand != row.selected {
				t.Fatalf("ImagesOnDemand=%v, want %v", suite.ImagesOnDemand, row.selected)
			}
			var names []string
			for _, plan := range suite.Scenarios {
				names = append(names, plan.Source.Name)
				prepared, err := plan.Prepare(t.Context(), interoplab.PrepareContext{
					Source:  plan.Source,
					Network: interoplab.Network{Name: plan.Network.Name, IPv4: netip.MustParsePrefix("172.30.9.0/24")},
				})
				if prepared.Cleanup != nil {
					if cleanupErr := prepared.Cleanup(); cleanupErr != nil {
						t.Fatal(cleanupErr)
					}
				}
				if err != nil {
					t.Fatal(err)
				}
				images := make(map[string]string)
				for _, peer := range prepared.Peers {
					found := false
					for _, declaration := range suite.Images {
						if declaration.Name == peer.Image {
							images[peer.Name] = declaration.Tag
							found = true
							break
						}
					}
					if !found {
						t.Fatalf("prepared peer %q has undeclared image %q", peer.Name, peer.Image)
					}
				}
				want := map[string]string{"ze": "ze-interop", peerFRR: "default-frr:fixture"}
				if plan.Source.Name == pinned {
					want[peerFRR] = reference
					want[peerSpeaker] = "ze-interop"
				}
				if !reflect.DeepEqual(images, want) {
					t.Fatalf("%s prepared dependencies=%v, want %v", plan.Source.Name, images, want)
				}
			}
			if !reflect.DeepEqual(names, row.want) {
				t.Fatalf("selected=%v, want %v", names, row.want)
			}
		})
	}
	t.Setenv("INTEROP_SCENARIO", "")
	if _, err := suiteFor(root, Options{Scenario: "ospf-opaque"}); err == nil {
		t.Fatal("unknown partial selector succeeded")
	}
}
