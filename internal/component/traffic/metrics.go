// Design: docs/architecture/decisions/001-pull-model-metrics.md -- 1s push poller for traffic qdisc and class statistics
// Related: backend.go -- ReadAllQdiscs readback seam
// Related: model.go -- QdiscStats, Qdisc, TrafficClass

package traffic

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/report"
)

const (
	qdiscMetricsCollectorName               = "traffic-qdisc-stats"
	qdiscMetricsInterval                    = time.Second
	qdiscMetricsWarnInterval                = 60 * time.Second
	trafficReportSource                     = "traffic"
	codeDoctorTrafficQdiscMetricsReadFailed = "doctor-traffic-qdisc-metrics-read-failed"
	qdiscMetricsReadFix                     = "ze show traffic control"
)

// statsGaugeSet holds the seven GaugeVec series shared by both qdisc-level and
// class-level traffic control statistics. Raw cumulative kernel counters carry
// the _total suffix as GaugeVecs because Linux tc counters reset on qdisc
// replacement and wrap at uint32.
type statsGaugeSet struct {
	bytes      metrics.GaugeVec
	packets    metrics.GaugeVec
	qlen       metrics.GaugeVec
	backlog    metrics.GaugeVec
	drops      metrics.GaugeVec
	overlimits metrics.GaugeVec
	requeues   metrics.GaugeVec
}

type qdiscMetricKey struct {
	iface  string
	qdisc  string
	handle string
}

type classMetricKey struct {
	iface  string
	qdisc  string
	class  string
	handle string
}

// qdiscMetricsPoller polls traffic control state on a fixed interval and pushes
// live qdisc and class statistics into a Prometheus Registry, removing stale
// label tuples on each tick.
//
// Goroutine confinement: after the synchronous initial collect() in
// startQdiscMetricsPollerWith, prevQdiscs, currQdiscs, prevClasses, and
// currClasses are accessed exclusively by the single worker goroutine.
//
// Caller MUST invoke stop() when the owning registry closes; stop() is safe
// for concurrent and repeated use and blocks until the worker goroutine exits.
type qdiscMetricsPoller struct {
	readAll     func() ([]InterfaceQoS, error)
	log         *slog.Logger
	clk         clock.Clock
	qdisc       statsGaugeSet
	class       statsGaugeSet
	prevQdiscs  map[qdiscMetricKey]struct{}
	currQdiscs  map[qdiscMetricKey]struct{}
	prevClasses map[classMetricKey]struct{}
	currClasses map[classMetricKey]struct{}
	readFailed  bool
	lastWarnAt  time.Time
	stopCh      chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup
}

func registerQdiscMetricsCollector() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        codeDoctorTrafficQdiscMetricsReadFailed,
		Title:       "Traffic qdisc metrics read failed",
		Description: "The 1-second traffic qdisc metrics push collector could not read live qdisc or class statistics from the active backend or kernel netlink reader.",
		Examples:    []string{qdiscMetricsReadFix, "ze show diagnostic " + codeDoctorTrafficQdiscMetricsReadFailed},
	}); err != nil {
		panic("BUG: traffic: register qdisc metrics diagnostic code: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorTrafficQdiscMetricsReadFailed}); err != nil {
		panic("BUG: traffic: register qdisc metrics event code: " + err.Error())
	}
	if err := metrics.RegisterPushCollector(qdiscMetricsCollectorName, startQdiscMetricsCollector); err != nil {
		panic("BUG: traffic: register qdisc metrics push collector: " + err.Error())
	}
}

func newStatsGaugeSet(reg metrics.Registry, prefix, subject string, labels []string) statsGaugeSet {
	return statsGaugeSet{
		bytes:      reg.GaugeVec(prefix+"bytes_total", subject+" transmitted bytes (raw kernel counter)", labels),
		packets:    reg.GaugeVec(prefix+"packets_total", subject+" transmitted packets (raw kernel counter)", labels),
		qlen:       reg.GaugeVec(prefix+"qlen", subject+" instantaneous queued packets", labels),
		backlog:    reg.GaugeVec(prefix+"backlog_bytes", subject+" instantaneous queued bytes", labels),
		drops:      reg.GaugeVec(prefix+"drops_total", subject+" dropped packets (raw kernel counter)", labels),
		overlimits: reg.GaugeVec(prefix+"overlimits_total", subject+" rate-limit overlimit events (raw kernel counter)", labels),
		requeues:   reg.GaugeVec(prefix+"requeues_total", subject+" requeued packets (raw kernel counter)", labels),
	}
}

func (s *statsGaugeSet) set(stats *QdiscStats, labelValues ...string) {
	s.bytes.With(labelValues...).Set(float64(stats.Bytes))
	s.packets.With(labelValues...).Set(float64(stats.Packets))
	s.qlen.With(labelValues...).Set(float64(stats.Qlen))
	s.backlog.With(labelValues...).Set(float64(stats.Backlog))
	s.drops.With(labelValues...).Set(float64(stats.Drops))
	s.overlimits.With(labelValues...).Set(float64(stats.Overlimits))
	s.requeues.With(labelValues...).Set(float64(stats.Requeues))
}

func (s *statsGaugeSet) delete(labelValues ...string) {
	s.bytes.Delete(labelValues...)
	s.packets.Delete(labelValues...)
	s.qlen.Delete(labelValues...)
	s.backlog.Delete(labelValues...)
	s.drops.Delete(labelValues...)
	s.overlimits.Delete(labelValues...)
	s.requeues.Delete(labelValues...)
}

func startQdiscMetricsCollector(reg metrics.Registry, log *slog.Logger) func() {
	if reg == nil {
		return nil
	}
	return startQdiscMetricsPollerWith(reg, log, clock.RealClock{}, ReadAllQdiscs)
}

func newQdiscMetricsPoller(
	reg metrics.Registry,
	log *slog.Logger,
	clk clock.Clock,
	readAll func() ([]InterfaceQoS, error),
) *qdiscMetricsPoller {
	return &qdiscMetricsPoller{
		readAll: readAll,
		log:     log,
		clk:     clk,
		qdisc: newStatsGaugeSet(
			reg,
			"ze_traffic_qdisc_",
			"Traffic control qdisc",
			[]string{"interface", "qdisc", "handle"},
		),
		class: newStatsGaugeSet(
			reg,
			"ze_traffic_class_",
			"Traffic control class",
			[]string{"interface", "qdisc", "class", "handle"},
		),
		prevQdiscs:  make(map[qdiscMetricKey]struct{}),
		currQdiscs:  make(map[qdiscMetricKey]struct{}),
		prevClasses: make(map[classMetricKey]struct{}),
		currClasses: make(map[classMetricKey]struct{}),
		stopCh:      make(chan struct{}),
	}
}

func startQdiscMetricsPollerWith(
	reg metrics.Registry,
	log *slog.Logger,
	clk clock.Clock,
	readAll func() ([]InterfaceQoS, error),
) func() {
	p := newQdiscMetricsPoller(reg, log, clk, readAll)

	// Sample synchronously before launching the worker so /metrics immediately
	// reflects live kernel state after exporter startup without racing with the
	// background ticker goroutine.
	p.collect()

	ticker := p.clk.NewTicker(qdiscMetricsInterval)
	p.wg.Add(1)
	go p.run(ticker)

	return p.stop
}

func (p *qdiscMetricsPoller) run(ticker clock.Ticker) {
	defer p.wg.Done()
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C():
			p.collect()
		case <-p.stopCh:
			return
		}
	}
}

func (p *qdiscMetricsPoller) collect() {
	list, err := p.readAll()
	if err != nil {
		if errors.Is(err, ErrNoBackendAvailable) {
			p.clearReadFailure()
			clear(p.currQdiscs)
			clear(p.currClasses)
			p.deleteStaleAndSwap()
			return
		}
		p.recordReadFailure(err)
		return
	}

	p.clearReadFailure()

	clear(p.currQdiscs)
	clear(p.currClasses)

	for i := range list {
		qos := &list[i]
		qdiscName := qos.Qdisc.KindName()
		if qos.Qdisc.Stats != nil {
			key := qdiscMetricKey{
				iface:  qos.Interface,
				qdisc:  qdiscName,
				handle: qos.Qdisc.Handle,
			}
			p.currQdiscs[key] = struct{}{}
			p.qdisc.set(qos.Qdisc.Stats, key.iface, key.qdisc, key.handle)
		}
		for j := range qos.Qdisc.Classes {
			cls := &qos.Qdisc.Classes[j]
			if cls.Stats == nil {
				continue
			}
			key := classMetricKey{
				iface:  qos.Interface,
				qdisc:  qdiscName,
				class:  cls.Name,
				handle: cls.Handle,
			}
			p.currClasses[key] = struct{}{}
			p.class.set(cls.Stats, key.iface, key.qdisc, key.class, key.handle)
		}
	}

	p.deleteStaleAndSwap()
}

func (p *qdiscMetricsPoller) recordReadFailure(err error) {
	now := p.clk.Now()
	if p.readFailed && now.Sub(p.lastWarnAt) < qdiscMetricsWarnInterval {
		return
	}
	if p.log != nil {
		p.log.Warn("traffic qdisc metrics read failed",
			"code", codeDoctorTrafficQdiscMetricsReadFailed,
			"err", err,
		)
	}
	report.RaiseWarningFix(
		trafficReportSource,
		codeDoctorTrafficQdiscMetricsReadFailed,
		qdiscMetricsCollectorName,
		report.FitMessage("traffic qdisc metrics read failed: "+err.Error()),
		qdiscMetricsReadFix,
	)
	p.readFailed = true
	p.lastWarnAt = now
}

func (p *qdiscMetricsPoller) clearReadFailure() {
	if !p.readFailed {
		return
	}
	report.ClearWarning(trafficReportSource, codeDoctorTrafficQdiscMetricsReadFailed, qdiscMetricsCollectorName)
	p.readFailed = false
	p.lastWarnAt = time.Time{}
}

func (p *qdiscMetricsPoller) deleteStaleAndSwap() {
	for key := range p.prevQdiscs {
		if _, alive := p.currQdiscs[key]; !alive {
			p.qdisc.delete(key.iface, key.qdisc, key.handle)
		}
	}
	for key := range p.prevClasses {
		if _, alive := p.currClasses[key]; !alive {
			p.class.delete(key.iface, key.qdisc, key.class, key.handle)
		}
	}

	clear(p.prevQdiscs)
	clear(p.prevClasses)
	p.prevQdiscs, p.currQdiscs = p.currQdiscs, p.prevQdiscs
	p.prevClasses, p.currClasses = p.currClasses, p.prevClasses
}

// stop signals the worker goroutine to exit and waits for it to finish.
// Safe for concurrent and repeated calls.
func (p *qdiscMetricsPoller) stop() {
	p.stopOnce.Do(func() {
		close(p.stopCh)
	})
	p.wg.Wait()
}
