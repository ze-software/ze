//go:build !ze_bgp

// Design: docs/architecture/testing/interop.md -- readiness cannot skip a compiled-out resolver.

package bgp

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// TestZePassiveReadinessWithoutBGP distinguishes a genuinely BGP-free config
// from one whose listener query cannot be answered by this build.
func TestZePassiveReadinessWithoutBGP(t *testing.T) {
	tree := config.NewTree()
	ports, err := zePassiveListenPorts(tree, "ze.conf")
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 0 {
		t.Fatalf("BGP-free config reported listeners: %v", ports)
	}
	tree.SetContainer("bgp", config.NewTree())
	ports, err = zePassiveListenPorts(tree, "ze.conf")
	if err == nil {
		t.Fatal("BGP config was reported ready without its resolver")
	}
	if !strings.Contains(err.Error(), "without ze_bgp") {
		t.Fatalf("missing-feature diagnostic: %v", err)
	}
	if len(ports) != 0 {
		t.Fatalf("failed resolution reported listeners: %v", ports)
	}
}
