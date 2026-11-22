// Design: docs/research/vpp-deployment-reference.md -- process ownership doctor
package vpp

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// Probe the actual configured capability path: a supported host is silent,
// absence reports its cause, and a disabled service never probes the host.
func TestVPPProcessDoctorReportsUnsupportedHost(t *testing.T) {
	tree := config.NewTree()
	tree.GetOrCreateContainer(componentVPP).Set("enabled", yangTrue)
	cause := errors.New("SO_PEERPIDFD is unavailable")
	calls := 0
	probe := func() error { calls++; return cause }
	findings := checkProcessPeerWithProbe(diagnostic.DoctorCheckContext{Tree: tree}, probe)
	if len(findings) != 1 {
		t.Fatalf("host findings=%d", len(findings))
	}
	if findings[0].Code != codeDoctorVPPProcess {
		t.Fatal("wrong physical-process doctor code")
	}
	if !strings.Contains(findings[0].Message, cause.Error()) {
		t.Fatal("host capability cause was lost")
	}
	if findings[0].Fix != processReportFix {
		t.Fatal("host capability recovery command was lost")
	}
	if findings := checkProcessPeerWithProbe(diagnostic.DoctorCheckContext{Tree: tree}, func() error { return nil }); len(findings) != 0 {
		t.Fatal("supported host refused")
	}
	tree.GetOrCreateContainer(componentVPP).Set("enabled", "false")
	if findings := checkProcessPeerWithProbe(diagnostic.DoctorCheckContext{Tree: tree}, probe); len(findings) != 0 {
		t.Fatal("disabled VPP probed its host")
	}
	if calls != 1 {
		t.Fatalf("host probes=%d", calls)
	}
}

// An observation/close warning reaches the real registered report reader with
// its exact cause; clearing another process cannot hide the unresolved owner.
func TestVPPProcessDoctorRetainsExactRuntimeFailure(t *testing.T) {
	first, _ := newPeerTestWitness(t, 301)
	second, _ := newPeerTestWitness(t, 302)
	cause := errors.New("pidfd poll refused")
	reportProcessFailure(first, "observation", cause)
	t.Cleanup(func() { clearProcessObservationFailure(first) })
	clearProcessObservationFailure(second)
	findings := checkProcessPeer(diagnostic.DoctorCheckContext{})
	found := false
	for _, finding := range findings {
		if !strings.Contains(finding.Message, cause.Error()) {
			continue
		}
		found = true
		if finding.Code != codeDoctorVPPProcess {
			t.Fatal("wrong retained-process doctor code")
		}
		if finding.Fix != processReportFix {
			t.Fatal("runtime recovery command was lost")
		}
	}
	if !found {
		t.Fatal("runtime observation was left only in a log")
	}
	clearProcessObservationFailure(first)
	for _, issue := range report.Warnings() {
		if issue.Source != processReportSource {
			continue
		}
		if issue.Subject == processFailureKey(first, "observation") {
			t.Fatal("confirmed observation retained its warning")
		}
	}
}
