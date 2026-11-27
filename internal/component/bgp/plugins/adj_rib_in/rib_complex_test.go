// Design: docs/architecture/plugin/rib-storage-design.md -- complex NLRI key error reporting and counting
// Related: rib_complex.go -- installComplexNLRIs and removeComplexNLRIs
// Related: register_doctor_codes.go -- codeDoctorBGPAdjRIBInNLRIKeyFailed

package adj_rib_in

import (
	"bytes"
	"context"
	"log/slog"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

type complexTestPublisher struct {
	mu     sync.Mutex
	raises []sdk.Issue
}

func (p *complexTestPublisher) RaiseIssue(_ context.Context, issue sdk.Issue) error {
	p.mu.Lock()
	p.raises = append(p.raises, issue)
	p.mu.Unlock()
	return nil
}

func (p *complexTestPublisher) ClearIssue(_ context.Context, _, _ string) error {
	return nil
}

type complexSyncWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *complexSyncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *complexSyncWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

type complexCounter struct {
	mu  sync.Mutex
	val float64
}

func (c *complexCounter) Inc() {
	c.mu.Lock()
	c.val++
	c.mu.Unlock()
}

func (c *complexCounter) Add(v float64) {
	c.mu.Lock()
	c.val += v
	c.mu.Unlock()
}

type complexCounterVec struct {
	mu     sync.Mutex
	counts map[string]*complexCounter
}

func (v *complexCounterVec) With(labels ...string) metrics.Counter {
	key := strings.Join(labels, "|")
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.counts == nil {
		v.counts = make(map[string]*complexCounter)
	}
	c, ok := v.counts[key]
	if !ok {
		c = &complexCounter{}
		v.counts[key] = c
	}
	return c
}

func (v *complexCounterVec) Delete(labels ...string) bool {
	key := strings.Join(labels, "|")
	v.mu.Lock()
	defer v.mu.Unlock()
	_, ok := v.counts[key]
	delete(v.counts, key)
	return ok
}

func (v *complexCounterVec) value(labels ...string) float64 {
	key := strings.Join(labels, "|")
	v.mu.Lock()
	defer v.mu.Unlock()
	c, ok := v.counts[key]
	if !ok {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.val
}

type complexTestMetrics struct {
	metrics.NopRegistry
	mu   sync.Mutex
	vecs map[string]*complexCounterVec
}

func (m *complexTestMetrics) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.vecs == nil {
		m.vecs = make(map[string]*complexCounterVec)
	}
	vec := &complexCounterVec{}
	m.vecs[name] = vec
	return vec
}

// TestComplexNLRIKeyErrorWarnsCountsAndRaisesDoctorCode verifies that complex
// NLRI key derivation errors on both install and withdraw paths increment the
// Prometheus counter, emit one rate-limited WARN with the registered doctor
// code and fix command, and enqueue a report-issue RPC via issuequeue.
//
// VALIDATES: AC-2, AC-4 (rib_complex.go installComplexNLRIs and removeComplexNLRIs).
// PREVENTS: unkeyable complex NLRIs disappearing silently at Debug with no metric or doctor event.
func TestComplexNLRIKeyErrorWarnsCountsAndRaisesDoctorCode(t *testing.T) {
	reg := &complexTestMetrics{}
	SetMetricsRegistry(reg)
	t.Cleanup(func() { SetMetricsRegistry(nil) })

	logs := &complexSyncWriter{}
	prevLogger := logger()
	setLogger(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { setLogger(prevLogger) })

	pub := &complexTestPublisher{}
	q := issuequeue.Start(pub)
	t.Cleanup(q.Stop)

	r := newTestManager(t)
	r.issues = q
	source := netip.MustParseAddr("192.0.2.1")

	// Labeled IPv4 NLRI with total bit length 64 (24-bit BOS label + 40-bit prefix):
	// nlrisplit frames it cleanly, then routeKeyFromNLRI rejects prefix length 40 > 32.
	badAnnounce := append([]byte{64}, labelOneBottom...)
	badAnnounce = append(badAnnounce, 10, 0, 1, 2, 3)

	// Feed twice to verify counter increments twice while WARN and issuequeue rate-limit to once.
	feedStructured(t, r, source, mpAnnounce(ipv4Labeled, []byte{10, 0, 0, 1}, badAnnounce))
	feedStructured(t, r, source, mpAnnounce(ipv4Labeled, []byte{10, 0, 0, 1}, badAnnounce))

	// Labeled IPv4 withdrawal with 24-bit compatibility field + 40-bit prefix (> 32).
	badWithdraw := append([]byte{64}, compatibility...)
	badWithdraw = append(badWithdraw, 10, 0, 1, 2, 3)
	feedStructured(t, r, source, mpWithdraw(ipv4Labeled, badWithdraw))
	feedStructured(t, r, source, mpWithdraw(ipv4Labeled, badWithdraw))

	q.Flush()

	vec := reg.vecs[metricNLRIKeyErrors]
	require.NotNil(t, vec, "ze_adjribin_nlri_key_errors_total must be registered")
	assert.Equal(t, float64(2), vec.value(source.String(), ipv4Labeled.String(), "install"), "install key errors must be counted twice")
	assert.Equal(t, float64(2), vec.value(source.String(), ipv4Labeled.String(), "remove"), "remove key errors must be counted twice")

	out := strings.TrimSpace(logs.String())
	require.NotEmpty(t, out, "key derivation errors must emit WARN logs")
	lines := strings.Split(out, "\n")
	require.Len(t, lines, 2, "expected 1 rate-limited WARN for install and 1 for remove:\n%s", out)
	for _, line := range lines {
		assert.Contains(t, line, "level=WARN")
		assert.Contains(t, line, "code="+codeDoctorBGPAdjRIBInNLRIKeyFailed)
		assert.Contains(t, line, fixShowAdjRIBIn)
		assert.Contains(t, line, "report=queued")
	}

	pub.mu.Lock()
	raises := append([]sdk.Issue(nil), pub.raises...)
	pub.mu.Unlock()
	require.Len(t, raises, 2, "expected 1 rate-limited issuequeue raise per operation")
	for _, issue := range raises {
		assert.Equal(t, codeDoctorBGPAdjRIBInNLRIKeyFailed, issue.Code)
		assert.Equal(t, rpc.ReportError, issue.Severity)
		assert.Equal(t, fixShowAdjRIBIn, issue.Fix)
	}
}
