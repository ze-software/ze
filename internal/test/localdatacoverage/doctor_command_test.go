// Design: docs/architecture/doctor-and-health-checks.md -- Linux-only kernel enrollment.

package localdatacoverage

import "testing"

func doctorKernelReport(ready any, capabilities any) map[string]any {
	return map[string]any{"ready": ready, "capabilities": capabilities}
}

func doctorKernelRow(subsystem, kernel string, state any) map[string]any {
	return map[string]any{"subsystem": subsystem, "kernel": kernel, "state": state}
}

// TestDoctorKernelReportAcceptsPlatformContract checks real report semantics,
// including a non-ready empty enrollment only on a non-Linux build.
func TestDoctorKernelReportAcceptsPlatformContract(t *testing.T) {
	present := doctorKernelRow("alpha", "CONFIG_ALPHA", "present")
	for _, test := range []struct {
		name    string
		linux   bool
		payload any
	}{
		{"Linux all present", true, doctorKernelReport(true, []any{present})},
		{"Linux absent", true, doctorKernelReport(false, []any{doctorKernelRow("alpha", "CONFIG_ALPHA", "absent")})},
		{"Linux unknown", true, doctorKernelReport(false, []any{doctorKernelRow("alpha", "CONFIG_ALPHA", "unknown")})},
		{"Linux denied", true, doctorKernelReport(false, []any{doctorKernelRow("alpha", "CONFIG_ALPHA", "denied")})},
		{"Linux mixed", true, doctorKernelReport(false, []any{present, doctorKernelRow("bravo", "CONFIG_BRAVO", "unknown")})},
		{"off Linux unenrolled", false, doctorKernelReport(false, []any{})},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateDoctorKernelReport(test.payload, test.linux); err != nil {
				t.Fatalf("valid platform report refused: %v", err)
			}
		})
	}
}

// TestDoctorKernelReportRefusesInvalidReports prevents an empty Linux answer,
// malformed row or contradictory ready verdict from clearing the binary walk.
func TestDoctorKernelReportRefusesInvalidReports(t *testing.T) {
	present := doctorKernelRow("alpha", "CONFIG_ALPHA", "present")
	for _, test := range []struct {
		name    string
		linux   bool
		payload any
	}{
		{"Linux empty not ready", true, doctorKernelReport(false, []any{})},
		{"Linux empty ready", true, doctorKernelReport(true, []any{})},
		{"off Linux empty ready", false, doctorKernelReport(true, []any{})},
		{"root scalar", false, "not a report"},
		{"missing ready", false, map[string]any{"capabilities": []any{}}},
		{"missing capabilities", false, map[string]any{"ready": false}},
		{"extra root field", false, map[string]any{"ready": false, "capabilities": []any{}, "status": "ok"}},
		{"string ready", false, doctorKernelReport("false", []any{})},
		{"null capabilities", false, doctorKernelReport(false, nil)},
		{"scalar capabilities", true, doctorKernelReport(true, "capabilities")},
		{"null row", true, doctorKernelReport(false, []any{nil})},
		{"scalar row", true, doctorKernelReport(false, []any{"capability"})},
		{"missing subsystem", true, doctorKernelReport(true, []any{map[string]any{"kernel": "CONFIG_ALPHA", "state": "present"}})},
		{"wrong subsystem type", true, doctorKernelReport(true, []any{map[string]any{"subsystem": 7, "kernel": "CONFIG_ALPHA", "state": "present"}})},
		{"empty subsystem", true, doctorKernelReport(true, []any{doctorKernelRow("", "CONFIG_ALPHA", "present")})},
		{"missing kernel", true, doctorKernelReport(true, []any{map[string]any{"subsystem": "alpha", "state": "present"}})},
		{"wrong kernel type", true, doctorKernelReport(true, []any{map[string]any{"subsystem": "alpha", "kernel": 7, "state": "present"}})},
		{"empty kernel", true, doctorKernelReport(true, []any{doctorKernelRow("alpha", "", "present")})},
		{"missing state", true, doctorKernelReport(false, []any{map[string]any{"subsystem": "alpha", "kernel": "CONFIG_ALPHA"}})},
		{"wrong state type", true, doctorKernelReport(false, []any{doctorKernelRow("alpha", "CONFIG_ALPHA", 7)})},
		{"empty state", true, doctorKernelReport(false, []any{doctorKernelRow("alpha", "CONFIG_ALPHA", "")})},
		{"unspecified state", true, doctorKernelReport(false, []any{doctorKernelRow("alpha", "CONFIG_ALPHA", "unspecified")})},
		{"unrecognized state", true, doctorKernelReport(false, []any{doctorKernelRow("alpha", "CONFIG_ALPHA", "other")})},
		{"all present not ready", true, doctorKernelReport(false, []any{present})},
		{"absent ready", true, doctorKernelReport(true, []any{doctorKernelRow("alpha", "CONFIG_ALPHA", "absent")})},
		{"unknown ready", true, doctorKernelReport(true, []any{doctorKernelRow("alpha", "CONFIG_ALPHA", "unknown")})},
		{"denied ready", true, doctorKernelReport(true, []any{doctorKernelRow("alpha", "CONFIG_ALPHA", "denied")})},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateDoctorKernelReport(test.payload, test.linux); err == nil {
				t.Fatal("invalid report accepted")
			}
		})
	}
}
