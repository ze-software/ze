// Design: docs/architecture/testing/qemu-integration.md -- VPN kernel evidence on Ze's system.
// Related: ../qemu/vpnproof.go -- native-architecture docker-host guest entry point.
// Related: wireguardproof.go -- received WireGuard traffic and host module-loader absence.
package testdeployment

import (
	"context"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	interopipsec "github.com/ze-software/ze/internal/le/interoplab/ipsec"
)

// vpnKernelReport carries each live scenario result. Guest image and accelerator
// provenance belong to the outer QEMU report, not to a deployment-only invocation.
type vpnKernelReport struct {
	IPsec     []interopipsec.Report  `json:"ipsec"`
	WireGuard interoplab.SuiteReport `json:"wireguard"`
}

func (r vpnKernelReport) Text() string {
	var out textbuf.Buffer
	for _, report := range r.IPsec {
		out.Str(report.Text()).Byte('\n')
	}
	out.Str("WireGuard kernel ping and no /sbin/modprobe: ")
	if r.WireGuard.Code == 0 {
		out.Str("PASS\n")
	} else {
		out.Str("FAIL\n").Str(r.WireGuard.Text())
	}
	out.Str("Guest provenance and arm64/HVF coverage must be read from the outer QEMU report.\n")
	return out.String()
}

func runVPNKernelAt(ctx context.Context, root string) (any, int) {
	report := vpnKernelReport{}
	code := 0
	for _, scenario := range []string{
		"psk-site-to-site", "esp-aes-cbc-sha256", "esp-chacha20poly1305",
		"esp-3des-sha1", "esp-integrity-only-null",
	} {
		result, status := interopipsec.RunAt(ctx, root, scenario)
		requireVPNCleanup(&result.SuiteReport)
		report.IPsec = append(report.IPsec, result)
		if status != 0 || result.Code != 0 {
			code = 1
		}
	}
	report.WireGuard = runWireGuardProof(ctx, root)
	requireVPNCleanup(&report.WireGuard)
	if report.WireGuard.Code != 0 {
		code = 1
	}
	return report, code
}

// requireVPNCleanup makes complete resource removal part of this proof's
// verdict, including when the protocol checker itself passed.
func requireVPNCleanup(report *interoplab.SuiteReport) {
	if len(report.CleanupErrors) != 0 {
		report.Code = 1
	}
	for index := range report.Scenarios {
		scenario := &report.Scenarios[index]
		if len(scenario.CleanupErrors) == 0 {
			continue
		}
		report.Code = 1
		if !scenario.Passed {
			continue
		}
		scenario.Passed = false
		scenario.Error = "protocol checks passed, but resource cleanup failed"
		report.Passed--
		report.Failed++
		report.FailedNames = append(report.FailedNames, scenario.Name)
	}
}
