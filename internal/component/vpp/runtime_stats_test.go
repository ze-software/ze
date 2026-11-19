// Design: docs/research/vpp-deployment-reference.md -- runtime counters independent of telemetry

package vpp

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/doctor"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

type runtimeStatsTestBus struct{ emit func(string, string, any) }

func (b runtimeStatsTestBus) Emit(namespace, event string, payload any) (int, error) {
	b.emit(namespace, event, payload)
	return 0, nil
}
func (runtimeStatsTestBus) Subscribe(string, string, func(any)) func() { return func() {} }

func runtimeStatsTestState(t *testing.T) {
	t.Helper()
	oldProvider := GetActiveStatsProvider()
	oldBus := getEventBus()
	oldMetrics := metricsRegPtr.Load()
	setActiveStatsProvider(nil)
	setVPPEventBus(nil)
	metricsRegPtr.Store(nil)
	t.Cleanup(func() { setActiveStatsProvider(oldProvider); setVPPEventBus(oldBus); metricsRegPtr.Store(oldMetrics) })
}

// The daemon's actual runtime doctor dispatcher reaches the registered stats
// finding and carries the retained source error and recovery command.
func TestVPPRuntimeStatsDoctorEntry(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	diagnostic.RegisterBuiltinCodes()
	if diagnostic.Lookup(diagnostic.CodeDoctorVPPStats) == nil {
		t.Fatal("stats code has no explanation")
	}
	reportRuntimeStatsFailure("connection", errors.New("stats socket denied"))
	for _, finding := range doctor.RunRuntimeChecks(nil, nil) {
		if finding.Code != diagnostic.CodeDoctorVPPStats {
			continue
		}
		if !strings.Contains(finding.Message, "stats socket denied") {
			t.Fatalf("cause lost: %+v", finding)
		}
		if finding.Fix != runtimeStatsFix {
			t.Fatalf("fix lost: %+v", finding)
		}
		return
	}
	t.Fatal("registered runtime doctor omitted failed stats")
}
