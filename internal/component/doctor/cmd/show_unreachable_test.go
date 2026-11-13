// Design: docs/architecture/doctor-and-health-checks.md -- unavailable plugin recovery.
package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type unreachableDoctorSource struct{ called string }

func (*unreachableDoctorSource) DoctorCheckPlugins() map[string][]plugin.DoctorCheckRegistration {
	return map[string][]plugin.DoctorCheckRegistration{"forwarding-west": {{Name: "fixture-check", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"}}}}
}

func (source *unreachableDoctorSource) CallDoctorCheck(_ context.Context, name, _ string) (*rpc.DoctorCheckOutput, error) {
	source.called = name
	return nil, errors.New("fixture-runtime-connection-lost")
}

func TestCollectUnreachablePluginDoctorCheckNamesRecovery(t *testing.T) {
	if healthy := collectDoctorChecksFrom(fakeDoctorCheckSource{}, ""); len(healthy) != 0 {
		t.Fatalf("healthy plugin findings = %v", healthy)
	}
	source := &unreachableDoctorSource{}
	findings := collectDoctorChecksFrom(source, "")
	if source.called != "forwarding-west" {
		t.Fatalf("callback identity = %q", source.called)
	}
	if len(findings) != 1 {
		t.Fatalf("unavailable plugin findings = %v", findings)
	}
	finding := findings[0]
	if finding.Code != "doctor-plugin-unreachable" {
		t.Fatalf("code = %q", finding.Code)
	}
	if finding.Severity != diagnostic.SeverityWarning {
		t.Fatalf("severity = %q", finding.Severity)
	}
	if !strings.Contains(finding.Message, "fixture-runtime-connection-lost") {
		t.Fatalf("cause was lost: %q", finding.Message)
	}
	if finding.Fix != "ze request plugin restart forwarding-west" {
		t.Fatalf("unreachable recovery = %q", finding.Fix)
	}
}
