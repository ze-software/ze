// Design: docs/functional-tests.md -- decisions of the real-entry BGP diagnostic fixture.
package fixture

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func TestBGPPolicyDiagnosticsReportOracleKeepsExternalControl(t *testing.T) {
	c := diagnostic.Diagnostic{Code: diagnostic.CodeDoctorBGPLearnedExportNoPolicy,
		Severity: diagnostic.SeverityWarning,
		Message:  "peer C: no routes learned from other peers. Local announcements remain permitted",
		Fix:      "set bgp peer C filter export final accept"}
	e := diagnostic.Diagnostic{Code: diagnostic.CodeDoctorBGPRFC8212NoPolicy,
		Severity: diagnostic.SeverityWarning, Message: "peer E: RFC 8212: no export policy",
		Fix: "set bgp peer E filter export final accept"}
	report := diagnostic.NewDoctorResult(false, []diagnostic.Diagnostic{c, e})
	require.NoError(t, bgpPolicyDiagnosticsDoctor(report, true))
	require.Error(t, bgpPolicyDiagnosticsDoctor(diagnostic.NewDoctorResult(true, nil), false), "clearing C cannot hide absence of E's positive control")
	report.Diagnostics = []diagnostic.Diagnostic{e}
	require.NoError(t, bgpPolicyDiagnosticsDoctor(report, false))
	require.Error(t, bgpPolicyDiagnosticsDoctor(report, true))
	c.Message = strings.Replace(c.Message, bgpPolicyDiagnosticsPeerC, "peer D:", 1)
	report.Diagnostics = []diagnostic.Diagnostic{c, e}
	require.Error(t, bgpPolicyDiagnosticsDoctor(report, true), "a genuine role exemption cannot be replaced by another internal finding")
	c.Message = strings.Replace(c.Message, "peer D:", bgpPolicyDiagnosticsPeerC, 1)
	c.Fix = ""
	report.Diagnostics = []diagnostic.Diagnostic{c, e}
	require.Error(t, bgpPolicyDiagnosticsDoctor(report, true))
	c.Code = "wrong-code"
	report.Diagnostics = []diagnostic.Diagnostic{c, e}
	require.Error(t, bgpPolicyDiagnosticsDoctor(report, false), "a differently coded C absence has not cleared")
	validation := "configuration valid: -\n" +
		"warning: peer C: no routes learned from other peers. Local announcements remain permitted; set bgp peer C filter export final accept\n" +
		"warning: peer E: RFC 8212: no export policy; set bgp peer E filter export final accept\n"
	require.NoError(t, bgpPolicyDiagnosticsValidate(validation, true))
	require.Error(t, bgpPolicyDiagnosticsValidate(validation, false))
	require.Error(t, bgpPolicyDiagnosticsValidate(strings.Replace(validation, bgpPolicyDiagnosticsPeerC, "peer D:", 1), true))
	require.Error(t, bgpPolicyDiagnosticsValidate(strings.Replace(validation, "set bgp peer C filter export final accept", "", 1), true))
}

func TestBGPPolicyDiagnosticsWireOracleDistinguishesActualReflection(t *testing.T) {
	prefix := []byte{24, 10, 254, 1}
	base := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 1}
	plain := &message.Update{NLRI: prefix, PathAttributes: base}
	require.NoError(t, bgpPolicyDiagnosticsRouteValue(plain, prefix, nil, false))
	require.Error(t, bgpPolicyDiagnosticsRouteValue(plain, prefix, nil, true), "an unadorned route cannot prove the RR path")
	reflected := &message.Update{NLRI: prefix, PathAttributes: append(append([]byte(nil), base...),
		0x80, 9, 4, 127, 0, 0, 7, 0x80, 10, 4, 10, 0, 0, 1)}
	require.NoError(t, bgpPolicyDiagnosticsRouteValue(reflected, prefix, nil, true))
	require.Error(t, bgpPolicyDiagnosticsRouteValue(reflected, prefix, nil, false), "an external source is not iBGP reflection")
	require.Error(t, bgpPolicyDiagnosticsRouteValue(reflected, []byte{24, 10, 254, 2}, nil, true))
	require.Error(t, bgpPolicyDiagnosticsRouteValue(reflected, prefix, []byte{2, 1, 0xfd, 0xe9}, true))
	wrongOrigin := append([]byte(nil), reflected.PathAttributes...)
	wrongOrigin[len(base)+3] = 126
	require.Error(t, bgpPolicyDiagnosticsRouteValue(&message.Update{NLRI: prefix, PathAttributes: wrongOrigin}, prefix, nil, true))
	require.Error(t, bgpPolicyDiagnosticsRouteValue(nil, prefix, nil, true))
}
