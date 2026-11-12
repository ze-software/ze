// Design: docs/architecture/testing/qemu-integration.md -- OTA daemon health.
// Related: install_ota_protocol_test.go -- authenticated orchestration fixture.

package testqemu

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/yang"
)

const otaHealthyResponse = `{"status":"healthy","components":[{"name":"iface","status":"healthy"},{"name":"plugins","status":"healthy"}],"count":2,"checked-at":"2026-11-12T12:00:00Z"}`

// TestApplianceOTAHealthChecksBothBoots proves that successful update and
// recovery each ask the authenticated daemon for its live health report.
func TestApplianceOTAHealthChecksBothBoots(t *testing.T) {
	fixture := newOTAProtocolFixture(t, false)
	fixture.run(t)
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if !slices.Equal(fixture.healthBoots, []string{"updated", "original"}) {
		t.Fatalf("health queried on runtimes %v, want updated then original", fixture.healthBoots)
	}
}

// TestApplianceOTAHealthWaitsForServiceReadiness makes the first updated-boot
// probe unhealthy, then makes the deterministic poll seam publish readiness.
func TestApplianceOTAHealthWaitsForServiceReadiness(t *testing.T) {
	for _, boot := range []string{"updated", "original"} {
		t.Run(boot, func(t *testing.T) {
			fixture := newOTAProtocolFixture(t, false)
			fixture.healthOutput = map[string]string{boot: `{"status":"degraded","components":[{"name":"iface","status":"degraded","reason":"interface error check timed out"},{"name":"plugins","status":"healthy"}],"count":2,"checked-at":"2026-11-12T12:00:00Z"}`}
			fixture.installer.ops.Sleep = func(context.Context, time.Duration) error {
				fixture.mu.Lock()
				fixture.healthOutput[boot] = otaHealthyResponse
				fixture.mu.Unlock()
				return nil
			}
			fixture.run(t)
			fixture.mu.Lock()
			defer fixture.mu.Unlock()
			want := []string{"updated", "updated", "original"}
			if boot == "original" {
				want = []string{"updated", "original", "original"}
			}
			if !slices.Equal(fixture.healthBoots, want) {
				t.Fatalf("health queried on runtimes %v, want %v", fixture.healthBoots, want)
			}
		})
	}
}

// stopOTAHealthPolling gives permanent-unhealthy fixtures a deterministic
// deadline outcome instead of waiting for their real boot timeout.
func stopOTAHealthPolling(fixture *otaProtocolFixture) *int {
	polls := 0
	fixture.installer.ops.Sleep = func(context.Context, time.Duration) error {
		polls++
		return context.DeadlineExceeded
	}
	return &polls
}

// TestApplianceOTAHealthRejectsUnusableAnswers drives bad daemon answers through
// the complete update and recovery orchestration, separately for each boot.
func TestApplianceOTAHealthRejectsUnusableAnswers(t *testing.T) {
	cases := []struct {
		name      string
		output    string
		refused   bool
		immediate bool
	}{
		{name: "command-refused", output: otaHealthyResponse, refused: true, immediate: true},
		{name: "malformed-json", output: `{"status":`, immediate: true},
		{name: "missing-report", output: `{}`},
		{name: "missing-timestamp", output: strings.ReplaceAll(otaHealthyResponse, `,"checked-at":"2026-11-12T12:00:00Z"`, "")},
		{name: "invalid-timestamp", output: strings.ReplaceAll(otaHealthyResponse, "2026-11-12T12:00:00Z", "yesterday")},
		{name: "unknown-overall", output: strings.Replace(otaHealthyResponse, `"status":"healthy"`, `"status":"unknown"`, 1)},
		{name: "inconsistent-overall", output: strings.Replace(otaHealthyResponse, `"status":"healthy"`, `"status":"down"`, 1)},
		{name: "missing-components", output: `{"status":"healthy","components":[],"count":0,"checked-at":"2026-11-12T12:00:00Z"}`},
		{name: "missing-plugins", output: strings.ReplaceAll(otaHealthyResponse, `,{"name":"plugins","status":"healthy"}`, "")},
		{name: "missing-iface", output: strings.ReplaceAll(otaHealthyResponse, `{"name":"iface","status":"healthy"},`, "")},
		{name: "duplicate-component", output: strings.ReplaceAll(otaHealthyResponse, `],"count":2`, `,{"name":"plugins","status":"healthy"}],"count":3`)},
		{name: "plugins-down", output: strings.ReplaceAll(otaHealthyResponse, `"name":"plugins","status":"healthy"`, `"name":"plugins","status":"down","reason":"configured plugin stopped"`)},
		{name: "iface-degraded", output: strings.ReplaceAll(otaHealthyResponse, `"name":"iface","status":"healthy"`, `"name":"iface","status":"degraded","reason":"interface error check timed out"`)},
		{name: "component-status-absent", output: strings.ReplaceAll(otaHealthyResponse, `"name":"plugins","status":"healthy"`, `"name":"plugins"`)},
		{name: "component-name-absent", output: strings.ReplaceAll(otaHealthyResponse, `"name":"plugins",`, "")},
		{name: "unexpected-degraded", output: strings.ReplaceAll(otaHealthyResponse, `],"count":2`, `,{"name":"firewall","status":"degraded","reason":"firewall audit could not run"}],"count":3`)},
		{name: "unexpected-down", output: strings.ReplaceAll(otaHealthyResponse, `],"count":2`, `,{"name":"pki","status":"down","reason":"certificate expired"}],"count":3`)},
		{name: "inactive-l2tp-other-reason", output: strings.ReplaceAll(otaHealthyResponse, `],"count":2`, `,{"name":"l2tp","status":"degraded","reason":"kernel data plane not programming sessions"}],"count":3`)},
		{name: "inactive-ipsec-other-reason", output: strings.ReplaceAll(otaHealthyResponse, `],"count":2`, `,{"name":"ipsec","status":"down","reason":"dataplane could not be read"}],"count":3`)},
		{name: "trailing-document", output: otaHealthyResponse + `{}`},
	}
	for _, boot := range []string{"updated", "original"} {
		for _, test := range cases {
			t.Run(boot+"/"+test.name, func(t *testing.T) {
				fixture := newOTAProtocolFixture(t, false)
				polls := stopOTAHealthPolling(fixture)
				fixture.healthOutput = map[string]string{boot: test.output}
				fixture.healthError = map[string]bool{boot: test.refused}
				report, err := fixture.installer.executeApplianceOTA(t.Context())
				if err == nil {
					t.Fatalf("OTA accepted unusable health answer on %s boot: report=%+v", boot, report)
				}
				if !strings.Contains(err.Error(), "health") {
					t.Fatalf("OTA failed outside the health assertion: %v", err)
				}
				if report.Verdict != InstallVerdictFail {
					t.Fatalf("unusable health answer returned verdict %q", report.Verdict)
				}
				if test.immediate {
					if *polls != 0 {
						t.Fatalf("unusable command/JSON answer entered %d readiness polls", *polls)
					}
				}
			})
		}
	}
}

// TestApplianceOTAHealthAllowsDormantProtocols proves the seed's management
// health is independent of unconfigured L2TP and IPsec subsystem health rows.
func TestApplianceOTAHealthAllowsDormantProtocols(t *testing.T) {
	fixture := newOTAProtocolFixture(t, false)
	output := `{"status":"down","components":[{"name":"iface","status":"healthy"},{"name":"plugins","status":"healthy"},{"name":"l2tp","status":"degraded","reason":"subsystem not running"},{"name":"ipsec","status":"down","reason":"ike engine not running"}],"count":4,"checked-at":"2026-11-12T12:00:00Z"}`
	fixture.healthOutput = map[string]string{"updated": output, "original": output}
	fixture.run(t)
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if !slices.Equal(fixture.healthBoots, []string{"updated", "original"}) {
		t.Fatalf("health queried on runtimes %v, want both boots", fixture.healthBoots)
	}
}

// TestApplianceOTAHealthConfiguredProtocolsCannotBeDormant proves that a seed
// enabling a protocol cannot use that protocol's inactive-state exception.
func TestApplianceOTAHealthConfiguredProtocolsCannotBeDormant(t *testing.T) {
	for _, test := range []struct {
		name string
		seed string
	}{
		{name: "l2tp", seed: "set l2tp enabled true\n"},
		{name: "ipsec", seed: "set vpn ipsec interface eth0\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newOTAProtocolFixture(t, false)
			polls := stopOTAHealthPolling(fixture)
			seed := "set environment ssh enabled true\n" + test.seed
			path := filepath.Join(fixture.installer.Tree, "gokrazy", "ze", "ze.conf")
			if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
				t.Fatal(err)
			}
			output := `{"status":"down","components":[{"name":"iface","status":"healthy"},{"name":"plugins","status":"healthy"},{"name":"l2tp","status":"degraded","reason":"subsystem not running"},{"name":"ipsec","status":"down","reason":"ike engine not running"}],"count":4,"checked-at":"2026-11-12T12:00:00Z"}`
			fixture.healthOutput = map[string]string{"updated": output}
			report, err := fixture.installer.executeApplianceOTA(t.Context())
			if err == nil {
				t.Fatalf("OTA accepted inactive configured %s: report=%+v", test.name, report)
			}
			if !strings.Contains(err.Error(), "health") {
				t.Fatalf("OTA failed outside health assertion: %v", err)
			}
			if *polls != 1 {
				t.Fatalf("configured inactive protocol entered %d polls, want one bounded wait", *polls)
			}
		})
	}
}

// registerOTAHealthSchema supplies only the fixture seed's syntax. The real
// launcher links the owning components' schemas; this leaf package does not.
func registerOTAHealthSchema(t *testing.T) {
	t.Helper()
	t.Cleanup(yang.RegisterModuleForTest("ze-ota-health-fixture-conf.yang", `module ze-ota-health-fixture-conf {
 namespace "urn:ze:ota-health-fixture";
 prefix ota;
 container environment { container ssh { leaf enabled { type boolean; } } }
 container system { leaf host { type string; } }
 container l2tp { leaf enabled { type boolean; } }
 container vpn { container ipsec { leaf interface { type string; } } }
}`))
}
