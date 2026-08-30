// Design: docs/architecture/core-design.md -- health check, report bus, and Prometheus metrics for PKI certificate expiry

package pki

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	expiryWarnDays = 30

	// Label names on both pki gauges. A Prometheus label is a separate contract
	// from the payload keys of the same spelling in types.go.
	metricLabelName = "name"
	metricLabelType = "type"

	reportSource      = "pki"
	reportCodeExpiry  = "cert-expiry"
	reportCodeExpired = "cert-expired"
)

type pkiMetrics struct {
	expirySeconds metrics.GaugeVec
	nearExpiry    metrics.GaugeVec
}

var pkiMetricsPtr atomic.Pointer[pkiMetrics]

// registerHealth registers the PKI certificate expiry health check.
// Called once at startup from the component wiring path.
func registerHealth() {
	health.Register("pki", checkHealth)
}

func ensureMetrics() {
	if pkiMetricsPtr.Load() != nil {
		return
	}
	reg := registry.GetMetricsRegistry()
	if reg == nil {
		return
	}
	m := &pkiMetrics{
		expirySeconds: reg.GaugeVec("ze_pki_certificate_expiry_seconds", "Seconds until certificate expires", []string{metricLabelName, metricLabelType}),
		nearExpiry:    reg.GaugeVec("ze_pki_certificate_near_expiry", "1 if certificate expires within 30 days, 0 otherwise", []string{metricLabelName, metricLabelType}),
	}
	pkiMetricsPtr.Store(m)
}

func checkHealth() (health.Status, string) {
	s := get()
	if len(s.caCerts) == 0 && len(s.certificates) == 0 {
		return health.StatusHealthy, "no certificates loaded"
	}

	now := time.Now()
	threshold := now.Add(expiryWarnDays * 24 * time.Hour)

	var earliestName string
	var earliestLabel string
	var earliestExpiry time.Time

	var tb textbuf.Buffer
	for name, ca := range s.caCerts {
		if now.After(ca.Certificate.NotAfter) {
			return health.StatusDown, tb.Str("ca ").Str(name).Str(" expired").String()
		}
		if ca.Certificate.NotAfter.Before(threshold) {
			if earliestExpiry.IsZero() || ca.Certificate.NotAfter.Before(earliestExpiry) {
				earliestName = name
				earliestLabel = "ca"
				earliestExpiry = ca.Certificate.NotAfter
			}
		}
	}

	for name, entry := range s.certificates {
		if now.After(entry.Certificate.NotAfter) {
			return health.StatusDown, tb.Reset().Str("certificate ").Str(name).Str(" expired").String()
		}
		if entry.Certificate.NotAfter.Before(threshold) {
			if earliestExpiry.IsZero() || entry.Certificate.NotAfter.Before(earliestExpiry) {
				earliestName = name
				earliestLabel = "certificate"
				earliestExpiry = entry.Certificate.NotAfter
			}
		}
	}

	if earliestExpiry.IsZero() {
		return health.StatusHealthy, ""
	}
	return health.StatusDegraded, tb.Reset().Str(earliestLabel).Byte(' ').Str(earliestName).Str(" expires in ").Int(int64(daysUntil(now, earliestExpiry))).Str(" days").String()
}

// RaiseExpiryWarnings checks all loaded certificates and raises report bus
// warnings for any approaching expiry. Call after Load on config reload.
func RaiseExpiryWarnings() {
	s := get()
	now := time.Now()
	warn := now.Add(expiryWarnDays * 24 * time.Hour)

	for name, ca := range s.caCerts {
		raiseOrClearExpiry(now, warn, "ca", name, ca.Certificate.NotAfter)
	}
	for name, entry := range s.certificates {
		raiseOrClearExpiry(now, warn, "cert", name, entry.Certificate.NotAfter)
	}

	updateMetrics(s, now, warn)
}

func raiseOrClearExpiry(now, warn time.Time, kind, name string, notAfter time.Time) {
	var tb textbuf.Buffer
	subject := tb.Str(kind).Byte('/').Str(name).String()
	var label string
	if kind == "ca" {
		label = tb.Reset().Str("CA certificate ").Str(name).String()
	} else {
		label = tb.Reset().Str("Certificate ").Str(name).String()
	}

	if now.After(notAfter) {
		report.RaiseWarning(reportSource, reportCodeExpired, subject,
			label+" has expired (expired "+notAfter.UTC().Format("2006-01-02")+")",
			map[string]any{fieldNotAfter: notAfter.UTC().Format(time.RFC3339)})
		return
	}
	if notAfter.Before(warn) {
		days := daysUntil(now, notAfter)
		report.RaiseWarning(reportSource, reportCodeExpiry, subject,
			label+" expires in "+strconv.Itoa(days)+" days",
			map[string]any{fieldNotAfter: notAfter.UTC().Format(time.RFC3339), "days-remaining": days})
		return
	}
	report.ClearWarning(reportSource, reportCodeExpiry, subject)
	report.ClearWarning(reportSource, reportCodeExpired, subject)
}

func updateMetrics(s *storeState, now, warn time.Time) {
	ensureMetrics()
	m := pkiMetricsPtr.Load()
	if m == nil {
		return
	}
	for name, ca := range s.caCerts {
		remaining := ca.Certificate.NotAfter.Sub(now).Seconds()
		m.expirySeconds.With(name, "ca").Set(remaining)
		if ca.Certificate.NotAfter.Before(warn) {
			m.nearExpiry.With(name, "ca").Set(1)
		} else {
			m.nearExpiry.With(name, "ca").Set(0)
		}
	}
	for name, entry := range s.certificates {
		remaining := entry.Certificate.NotAfter.Sub(now).Seconds()
		m.expirySeconds.With(name, "device").Set(remaining)
		if entry.Certificate.NotAfter.Before(warn) {
			m.nearExpiry.With(name, "device").Set(1)
		} else {
			m.nearExpiry.With(name, "device").Set(0)
		}
	}
}

func daysUntil(now, expiry time.Time) int {
	d := expiry.Sub(now)
	if d < 0 {
		return 0
	}
	return int(d.Hours() / 24)
}
