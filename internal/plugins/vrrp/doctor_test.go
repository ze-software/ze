// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- doctor config-sanity tests
//
// VALIDATES: AC-10 -- `ze doctor` reports exactly what a commit would reject,
// including the VPP-backend rejection, and stays silent on a healthy config.
// PREVENTS: a doctor check that drifts from the verifier (two rule sets, two
// answers) and a check that fires when no vrrp is configured at all.

package vrrp

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestDoctorSilentWithoutVRRP proves the check says nothing when interfaces are
// configured but no vrrp group is -- the plugin auto-loads with the `interface`
// root, so a noisy check would fire on every ze deployment.
func TestDoctorSilentWithoutVRRP(t *testing.T) {
	tree := map[string]any{
		"ethernet": map[string]any{
			"eth0": map[string]any{
				"unit": map[string]any{"0": map[string]any{"ipv4": map[string]any{"address": vips("10.0.0.2/24")}}},
			},
		},
	}
	if diags := vrrpConfigDiagnostics(t, tree, backendNetlink); len(diags) != 0 {
		t.Fatalf("no vrrp configured must produce no diagnostics, got %+v", diags)
	}
}

// TestDoctorSilentOnHealthyConfig proves a valid group produces no findings.
func TestDoctorSilentOnHealthyConfig(t *testing.T) {
	tree := oneGroup(familyIPv4, "10", map[string]any{"virtual-address": vips("192.0.2.1")})
	if diags := vrrpConfigDiagnostics(t, tree, backendNetlink); len(diags) != 0 {
		t.Fatalf("healthy config must produce no diagnostics, got %+v", diags)
	}
}

// TestDoctorReportsInvalidConfig proves a cross-leaf violation surfaces with the
// config-invalid code and an actionable message.
func TestDoctorReportsInvalidConfig(t *testing.T) {
	// accept-mode with version 2: a VRRPv3-only feature (RFC 9568 Section 6.1).
	tree := oneGroup(familyIPv4, "10", map[string]any{
		"virtual-address": vips("192.0.2.1"),
		"version":         "2",
		"accept-mode":     true,
	})
	diags := vrrpConfigDiagnostics(t, tree, backendNetlink)
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %d, want 1", len(diags))
	}
	if diags[0].Code != codeVRRPConfigInvalid {
		t.Errorf("code = %q, want %q", diags[0].Code, codeVRRPConfigInvalid)
	}
	if !strings.Contains(diags[0].Message, "accept-mode") {
		t.Errorf("message must name the offending leaf, got %q", diags[0].Message)
	}
}

// TestDoctorReportsVPPBackend proves vrrp on a VPP-backed tree reports the
// backend code, not the generic one: the fix is different (change the backend or
// drop the config), so the code must be too.
func TestDoctorReportsVPPBackend(t *testing.T) {
	tree := oneGroup(familyIPv4, "10", map[string]any{"virtual-address": vips("192.0.2.1")})
	diags := vrrpConfigDiagnostics(t, tree, backendVPP)
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %d, want 1", len(diags))
	}
	if diags[0].Code != codeVRRPBackendUnusable {
		t.Errorf("code = %q, want %q", diags[0].Code, codeVRRPBackendUnusable)
	}
	if !strings.Contains(diags[0].Message, "backend") {
		t.Errorf("message must name the backend, got %q", diags[0].Message)
	}
}

// TestDoctorCodesAreExplainable proves every code this plugin can emit is
// registered with metadata, so `ze explain <code>` resolves it.
func TestDoctorCodesAreExplainable(t *testing.T) {
	registerVRRPDiagnosticCodes()
	for _, code := range []string{codeVRRPConfigInvalid, codeVRRPBackendUnusable} {
		meta := diagnostic.Lookup(code)
		if meta == nil {
			t.Errorf("code %q is not registered; `ze explain %s` would fail", code, code)
			continue
		}
		if meta.Title == "" || meta.Description == "" {
			t.Errorf("code %q has empty metadata: %+v", code, meta)
		}
	}
}

// vrrpConfigDiagnostics runs the check's rule path over a tree, bypassing the
// config.Tree wrapper the runner supplies (the wrapper is exercised by the .ci
// functional test through `ze doctor` itself).
func vrrpConfigDiagnostics(t *testing.T, ifaceTree map[string]any, backend string) []diagnostic.Diagnostic {
	t.Helper()
	if backend != "" {
		ifaceTree["backend"] = backend
	}
	sections := []configSection{mkSection(t, ifaceTree)}
	return diagnoseSections(sections)
}
