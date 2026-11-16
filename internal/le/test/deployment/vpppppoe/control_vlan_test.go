// Design: docs/labs/pppoe-interop.md -- VLAN control must have a real paired VPP child.
package testvpppppoe

import (
	"context"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestControlVLANPairsBothSides sends setup through the Docker process seam;
// the read-back CP child, rather than a guessed suffix or the parent, must own the map.
func TestControlVLANPairsBothSides(t *testing.T) {
	paired := false
	mapped := false
	docker := interoplab.NewFixtureDocker(mechanismProcess(func(_ context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
		args := command.Arguments
		if slices.Contains(args, "show") {
			if slices.Contains(args, "lcp") {
				output := "itf-pair: [0] host-eth0 tap3 cp0 12 type tap\n"
				if paired {
					output += "itf-pair: [1] host-eth0.100 tap9.100 cp0.100 13 type tap\n"
				}
				return interoplab.ProcessResult{Stdout: output}, nil
			}
		}
		if slices.Contains(args, "host-if") {
			if slices.Contains(args, "cp0.100") {
				paired = true
			}
		}
		if slices.Contains(args, "add") {
			if slices.Contains(args, "vlan") {
				t.Error("Linux-only VLAN creation bypassed LCP pairing")
			}
		}
		if slices.Contains(args, "map") {
			mapped = true
			if !paired {
				t.Error("mapped before creating the VLAN pair")
			}
			if !slices.Contains(args, "tap9.100") {
				t.Errorf("map ignored actual CP child: %q", args)
			}
		}
		return interoplab.ProcessResult{}, nil
	}))
	device, namespace, err := prepareControlPair(t.Context(), mechanismProbeLab{docker: docker}, modeVLAN)
	if err != nil {
		t.Fatal(err)
	}
	if device != "cp0.100" {
		t.Fatalf("device = %q", device)
	}
	if namespace != "" {
		t.Fatalf("namespace = %q", namespace)
	}
	if !mapped {
		t.Fatal("VLAN map absent")
	}
}
