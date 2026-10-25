//go:build ze_core && ze_anomaly && ze_as112 && ze_bfd && ze_bgp && ze_bmp && ze_copp && ze_cos && ze_ddos && ze_dhcpserver && ze_exabgp && ze_flowexport && ze_geodns && ze_gnmi && ze_grpc && ze_ike && ze_isis && ze_l2tp && ze_ldp && ze_lg && ze_mcp && ze_mpls && ze_ntp && ze_ospf && ze_policyroute && ze_pxe && ze_radius && ze_rest && ze_rsvpte && ze_ssh && ze_tacacs && ze_telemetry && ze_trafficusage && ze_vpp && ze_vrrp && ze_web && !ze_le && !ze_distro && !ze_appliance && !ze_setup

// Related: help_ai.go -- the reference these goldens pin

package main

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"
)

// VALIDATES: AC-41 -- `ze help ai`, `ze help ai all` and `ze help ai --json`
// print what they printed before le gained its own `help ai`. The goldens
// were captured from HEAD under the full feature set this file builds with.
// The summary is compared byte for byte. `all` and `--json` are compared
// as their sorted lines, because at HEAD both list RPCs in a different order
// on each run (map iteration), so no byte golden of them can hold.
// PREVENTS: the shared reference type or the help renderer changing ze's
// reference in passing.
func TestZeHelpAiOutputUnchanged(t *testing.T) {
	for name, args := range map[string][]string{"summary": nil, "all": {"all"}, "json": {"--json"}} {
		want, err := os.ReadFile("testdata/help-ai-" + name + ".golden")
		if err != nil {
			t.Fatal(err)
		}
		var got bytes.Buffer
		if code := renderAIHelp(&got, args); code != 0 {
			t.Fatalf("%s: exit %d", name, code)
		}
		if name == "summary" {
			if !bytes.Equal(got.Bytes(), want) {
				t.Errorf("ze help ai differs from its golden:\n%s", got.String())
			}
			continue
		}
		gotLines, wantLines := sortedLines(got.String()), sortedLines(string(want))
		if !slices.Equal(gotLines, wantLines) {
			t.Errorf("ze help ai %s differs from its golden beyond line order: only now %q, only in the golden %q",
				name, firstMissing(wantLines, gotLines), firstMissing(gotLines, wantLines))
		}
	}
}

// firstMissing answers up to five lines of have that want does not hold.
func firstMissing(want, have []string) []string {
	var out []string
	for _, line := range have {
		if _, found := slices.BinarySearch(want, line); !found && len(out) < 5 {
			out = append(out, line)
		}
	}
	return out
}

// sortedLines answers text's lines, sorted, without the services' env-var
// lines: at HEAD a service whose leaves tie two env prefixes is listed with
// either one, by map order (plan/journal/reference-output-depends-on-map-order.md).
// The exclusion goes when that row is fixed.
func sortedLines(text string) []string {
	lines := slices.DeleteFunc(strings.Split(text, "\n"), func(line string) bool {
		return strings.HasPrefix(line, "    Env vars:  ")
	})
	slices.Sort(lines)
	return lines
}
