package l2tpauthradius

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

func TestBindRADIUSMetrics_Registration(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	bindRADIUSMetrics(reg)
	m := radiusMetricsPtr.Load()
	if m == nil {
		t.Fatal("expected RADIUS metrics to be bound")
	}
	defer radiusMetricsPtr.Store(nil)

	if m.up == nil {
		t.Error("up gauge not registered")
	}
	if m.authSent == nil {
		t.Error("authSent counter not registered")
	}
	if m.acctSent == nil {
		t.Error("acctSent counter not registered")
	}
	if m.interimSent == nil {
		t.Error("interimSent counter not registered")
	}
}

func TestBindRADIUSMetrics_NilRegistry(t *testing.T) {
	old := radiusMetricsPtr.Load()
	radiusMetricsPtr.Store(nil)
	defer radiusMetricsPtr.Store(old)

	bindRADIUSMetrics(nil)
	if radiusMetricsPtr.Load() != nil {
		t.Error("nil registry should not bind metrics")
	}
}

func TestRADIUSMetrics_IncAuthSent(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	bindRADIUSMetrics(reg)
	defer radiusMetricsPtr.Store(nil)

	incAuthSent("srv1", "10.0.0.1:1812")
	incAuthSent("srv1", "10.0.0.1:1812")
}

func TestRADIUSMetrics_IncAcctSent(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	bindRADIUSMetrics(reg)
	defer radiusMetricsPtr.Store(nil)

	incAcctSent("srv1", "10.0.0.1:1813")
}

func TestRADIUSMetrics_IncInterimSent(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	bindRADIUSMetrics(reg)
	defer radiusMetricsPtr.Store(nil)

	incInterimSent("srv1", "10.0.0.1:1813")
}

func TestRADIUSMetrics_SetRadiusUp(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	bindRADIUSMetrics(reg)
	defer radiusMetricsPtr.Store(nil)

	setRadiusUp("srv1", "10.0.0.1:1812", true)
	setRadiusUp("srv1", "10.0.0.1:1812", false)
}

func TestRADIUSMetrics_NilSafe(t *testing.T) {
	old := radiusMetricsPtr.Load()
	radiusMetricsPtr.Store(nil)
	defer radiusMetricsPtr.Store(old)

	incAuthSent("", "")
	incAcctSent("", "")
	incInterimSent("", "")
	setRadiusUp("", "", true)
}
