package aihelp

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// buildRepeats is how many times the determinism test assembles the reference.
// Go randomizes every map range, so an order that leaks from a map differs
// between two runs in nearly every pair; twenty runs leave no realistic chance
// of twenty identical orders.
const buildRepeats = 20

// TestBuildIsDeterministic assembles the reference repeatedly in one process
// and compares the JSON byte for byte.
//
// VALIDATES: `ze help ai --json` and MCP ze_reference print the same bytes on
// every run (spec-le-cli-usability, journal reference-output-depends-on-map-order).
// PREVENTS: an RPC, notification or service order taken from a map range.
func TestBuildIsDeterministic(t *testing.T) {
	first := buildJSON(t)
	for range buildRepeats {
		if got := buildJSON(t); got != first {
			t.Fatalf("two Build runs answered different references:\n%s", firstDifference(first, got))
		}
	}
}

// TestServicesShowOnlyTheirOwnEnvVars checks every service the linked registry
// holds: each env var it lists is in the service's own group, ze.<service>.
//
// VALIDATES: a service shows its own env vars and never another's.
// PREVENTS: bmp listing gnmi's vars, and ntp listing ze.metrics.interval,
// because a leaf name (enabled, interval) also ends another group's key.
func TestServicesShowOnlyTheirOwnEnvVars(t *testing.T) {
	services, err := Services()
	if err != nil {
		t.Fatalf("Services: %v", err)
	}
	for _, svc := range services {
		for _, key := range svc.EnvVars {
			if envGroup(key) != svc.Name {
				t.Errorf("service %s lists %s, which belongs to group %s", svc.Name, key, envGroup(key))
			}
		}
	}
}

// TestServiceEnvVarsMatchesTheGroupOnly feeds the matcher keys whose leaf names
// collide with the service's own leaves, the shape that misattributed bmp.
//
// VALIDATES: a service with no env group of its own lists none, whatever leaf
// names it shares with other groups; a service with one lists exactly it.
// PREVENTS: a leaf-name heuristic picking a foreign group.
func TestServiceEnvVarsMatchesTheGroupOnly(t *testing.T) {
	keys := []string{
		"ze.web.enabled", "ze.gnmi.listen", "ze.gnmi.enabled",
		"ze.metrics.interval", "ze.log", "ze.log.backend", "ze.gnmi-extra.token",
	}
	if got := serviceEnvVars("bmp", keys); len(got) != 0 {
		t.Errorf("bmp, which has no env group, listed %v", got)
	}
	if got := serviceEnvVars("ntp", keys); len(got) != 0 {
		t.Errorf("ntp, which has no env group, listed %v", got)
	}
	if got := serviceEnvVars("gnmi", keys); !slices.Equal(got, []string{"ze.gnmi.enabled", "ze.gnmi.listen"}) {
		t.Errorf("gnmi listed %v, want its two keys sorted", got)
	}
	if got := serviceEnvVars("log", keys); !slices.Equal(got, []string{"ze.log", "ze.log.backend"}) {
		t.Errorf("log listed %v, want ze.log and ze.log.backend", got)
	}
}

// buildJSON answers one Build as JSON.
func buildJSON(t *testing.T) string {
	t.Helper()
	ref, err := Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return string(data)
}

// envGroup answers the group of an env key: "ze.web.listen" is in "web".
func envGroup(key string) string {
	parts := strings.SplitN(key, ".", 3) //nolint:mnd // ze.<group>.<leaf>
	if len(parts) < 2 {                  //nolint:mnd // ze.<group>
		return ""
	}
	return parts[1]
}

// firstDifference shows where two renderings part, so a failure names the
// entry whose order moved rather than printing two large documents.
func firstDifference(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	start := max(i-80, 0)
	return "first run:  ..." + a[start:min(i+80, len(a))] + "\nlater run:  ..." + b[start:min(i+80, len(b))]
}
