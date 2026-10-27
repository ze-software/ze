// Design: docs/architecture/testing/qemu-integration.md -- cleanup is part of the VPN verdict.
// Related: vpnproof.go -- protocol success cannot hide failed resource removal.
package testdeployment

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	interopipsec "github.com/ze-software/ze/internal/le/interoplab/ipsec"
)

// TestVPNCleanupFailureRefusesPassingTraffic covers both protocol reports and
// the image-cleanup boundary. A prior checker failure must not be counted twice.
func TestVPNCleanupFailureRefusesPassingTraffic(t *testing.T) {
	for _, checkerPassed := range []bool{true, false} {
		report := interoplab.SuiteReport{
			Scenarios: []interoplab.ScenarioResult{{
				Name: "traffic", Passed: checkerPassed,
				CleanupErrors: []string{"docker rm peer: permission denied"},
			}},
		}
		if checkerPassed {
			report.Passed = 1
		} else {
			report.Failed = 1
			report.FailedNames = []string{"traffic"}
			report.Scenarios[0].Error = "receiver counter did not advance"
		}
		requireVPNCleanup(&report)
		if report.Code != 1 || report.Passed != 0 || report.Failed != 1 || report.Scenarios[0].Passed {
			t.Fatalf("checker passed=%t: cleanup verdict = %#v", checkerPassed, report)
		}
		if len(report.FailedNames) != 1 || report.FailedNames[0] != "traffic" {
			t.Fatalf("checker passed=%t: failed names = %v", checkerPassed, report.FailedNames)
		}
		if !checkerPassed && report.Scenarios[0].Error != "receiver counter did not advance" {
			t.Fatal("cleanup erased the protocol failure")
		}
		combined := vpnKernelReport{
			IPsec: []interopipsec.Report{{SuiteReport: report}}, WireGuard: report,
		}
		if got := combined.Text(); strings.Count(got, "docker rm peer: permission denied") != 2 || !strings.Contains(got, "WireGuard kernel ping and no /sbin/modprobe: FAIL") {
			t.Fatalf("both protocol reports must expose cleanup failure: %s", got)
		}
	}
	imageFailure := interoplab.SuiteReport{CleanupErrors: []string{"docker image rm proof: image is in use"}}
	requireVPNCleanup(&imageFailure)
	if imageFailure.Code != 1 || !strings.Contains(imageFailure.Text(), "image is in use") {
		t.Fatalf("image cleanup failure was lost: %#v", imageFailure)
	}
}
