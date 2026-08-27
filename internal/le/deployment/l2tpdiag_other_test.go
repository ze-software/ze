//go:build !linux

package deployment

import (
	"strings"
	"testing"
)

func TestL2TPDiagnosticsRemainRegisteredAndRefuseTheOperatingSystem(t *testing.T) {
	cases := []struct {
		name string
		run  func() (L2TPDiagnosticReport, error)
	}{
		{name: l2tpPPPoXDiagnosticName, run: func() (L2TPDiagnosticReport, error) {
			return executeL2TPPPoXDiagnostic(defaultPPPoXDiagnosticOptions())
		}},
		{name: l2tpTunnelDiagnosticName, run: func() (L2TPDiagnosticReport, error) {
			return executeL2TPTunnelDiagnostic(defaultTunnelDiagnosticOptions())
		}},
	}
	for _, test := range cases {
		report, err := test.run()
		if err == nil || !strings.Contains(err.Error(), "requires Linux") {
			t.Fatalf("%s error = %v", test.name, err)
		}
		if report.Diagnostic != test.name || report.Verdict != "" {
			t.Fatalf("%s report = %#v", test.name, report)
		}
	}
}
