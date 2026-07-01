// Design: plan/learned/818-flow-export-1-counter-export.md -- Exporter lifecycle
// Related: flowtypes.go -- FlowSample / ConntrackFlow dispatched by ExportFlowSample / ExportFlows

package flowexport

import (
	"net/netip"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/observation"
	"codeberg.org/thomas-mangin/ze/internal/plugins/flowexport/enrich"
)

// ProtocolEncoder encodes counter snapshots into wire-format datagrams
// and sends them via the provided Sender.
type ProtocolEncoder interface {
	// Encode writes one or more datagrams for the snapshot into pooled
	// buffers and sends them. Returns the number of data records exported
	// (used for IPFIX sequence counting).
	Encode(snap CounterSnapshot, sender *Sender) (dataRecords int, err error)

	// EncodeTemplate writes template datagrams (NetFlow v9, IPFIX).
	// No-op for sFlow which has no template concept.
	EncodeTemplate(sender *Sender) error
}

// collectorState tracks per-collector runtime state.
type collectorState struct {
	cfg    CollectorConfig
	sender *Sender

	encoder    ProtocolEncoder
	flowSample FlowSampleEncoder
	flowRecord FlowRecordEncoder

	lastPoll         time.Time
	lastTemplate     time.Time
	flowTemplateLast time.Time
	sequence         uint32
}

// Exporter manages flow export to all configured collectors.
type Exporter struct {
	mu         sync.Mutex
	collectors []collectorState
	enricher   *enrich.Enricher
	stoppers   []func() // worker/builder teardown, run outside e.mu on Stop
	stopCh     chan struct{}
	stopped    bool
	startTime  time.Time
	// recent is a bounded ring of recently exported conntrack flows, fed from
	// ExportFlows and read by `show flow-recent` so on-box consumers (the DDoS
	// characterizer) can inspect the current flow mix without a packet capture.
	// nil when conntrack export is disabled (nothing would feed it).
	recent *recentRing
}

// AddStopper registers a teardown function (sampling/conntrack worker, BGP
// enrich builder) to run when the exporter stops. Stoppers run outside the
// exporter mutex because worker goroutines call ExportFlow* which take it.
func (e *Exporter) AddStopper(fn func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stoppers = append(e.stoppers, fn)
}

// SetEnricher assigns the BGP enricher used to annotate per-flow records.
// May be nil (enrichment disabled).
func (e *Exporter) SetEnricher(en *enrich.Enricher) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.enricher = en
}

// NewExporter creates an exporter from config. Opens UDP sockets
// to all collectors. Call Stop() to release resources.
func NewExporter(cfg *Config) (*Exporter, error) {
	e := &Exporter{
		stopCh:    make(chan struct{}),
		startTime: time.Now(),
	}
	// The recent-flow ring is only fed by the conntrack export path, so allocate
	// it only when conntrack export is enabled -- otherwise it would sit empty.
	if cfg.Conntrack.Enabled {
		e.recent = newRecentRing(cfg.Conntrack.RecentRing)
	}

	for i := range cfg.Collectors {
		cc := &cfg.Collectors[i]
		sender, err := NewSender(cc.Address, cc.Port)
		if err != nil {
			e.Stop()
			return nil, err
		}
		cs := collectorState{
			cfg:    *cc,
			sender: sender,
		}
		e.collectors = append(e.collectors, cs)
	}

	return e, nil
}

// SetEncoder assigns a protocol encoder to the named collector.
func (e *Exporter) SetEncoder(collectorName string, enc ProtocolEncoder) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.collectors {
		if e.collectors[i].cfg.Name == collectorName {
			e.collectors[i].encoder = enc
			return
		}
	}
}

// SetFlowSampleEncoder assigns an sFlow flow-sample encoder to the named
// collector (spec 2 packet sampling).
func (e *Exporter) SetFlowSampleEncoder(collectorName string, enc FlowSampleEncoder) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.collectors {
		if e.collectors[i].cfg.Name == collectorName {
			e.collectors[i].flowSample = enc
			return
		}
	}
}

// SetFlowRecordEncoder assigns a per-flow record encoder to the named
// collector (spec 2 conntrack flow records).
func (e *Exporter) SetFlowRecordEncoder(collectorName string, enc FlowRecordEncoder) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.collectors {
		if e.collectors[i].cfg.Name == collectorName {
			e.collectors[i].flowRecord = enc
			return
		}
	}
}

// NotifySnapshot is called from the iface rate tracker with fresh
// counter data. Non-blocking: if the exporter is busy, the snapshot
// is dropped (counter export is best-effort like the protocols it
// implements).
func (e *Exporter) NotifySnapshot(snap CounterSnapshot) {
	if !e.mu.TryLock() {
		return
	}
	defer e.mu.Unlock()

	if e.stopped {
		return
	}

	now := snap.Time
	log := loggerPtr.Load()

	for i := range e.collectors {
		cs := &e.collectors[i]
		if cs.encoder == nil {
			continue
		}

		pollInterval := time.Duration(cs.cfg.PollingInterval) * time.Second
		if !cs.lastPoll.IsZero() && now.Sub(cs.lastPoll) < pollInterval {
			continue
		}

		// Template refresh (NetFlow v9, IPFIX).
		refreshInterval := time.Duration(cs.cfg.TemplateRefresh) * time.Second
		if cs.lastTemplate.IsZero() || now.Sub(cs.lastTemplate) >= refreshInterval {
			if err := cs.encoder.EncodeTemplate(cs.sender); err != nil {
				log.Warn("flow-export: template send failed",
					"collector", cs.cfg.Name, "error", err)
				incErrors(cs.cfg.Name, cs.cfg.Protocol)
			} else {
				cs.lastTemplate = now
			}
		}

		dgBefore, bytesBefore, _ := cs.sender.Stats()
		records, err := cs.encoder.Encode(snap, cs.sender)
		if err != nil {
			log.Warn("flow-export: encode/send failed",
				"collector", cs.cfg.Name, "error", err)
			incErrors(cs.cfg.Name, cs.cfg.Protocol)
			continue
		}
		dgAfter, bytesAfter, _ := cs.sender.Stats()

		cs.lastPoll = now
		cs.sequence += uint32(records)
		// One Encode call may emit multiple datagrams (sFlow batches
		// counter samples and spills overflow into additional datagrams).
		// Count each datagram the sender actually transmitted.
		for range dgAfter - dgBefore {
			incDatagrams(cs.cfg.Name, cs.cfg.Protocol)
		}
		addBytes(cs.cfg.Name, cs.cfg.Protocol, float64(bytesAfter-bytesBefore))
	}
}

// ExportFlowSample dispatches one sampled packet to every sFlow collector
// that has a flow-sample encoder. Called from the sampling worker goroutine.
// Non-blocking with respect to the snapshot path: takes the same mutex, so a
// sample is encoded between counter ticks.
func (e *Exporter) ExportFlowSample(sample FlowSample) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopped {
		return
	}
	log := loggerPtr.Load()
	for i := range e.collectors {
		cs := &e.collectors[i]
		if cs.flowSample == nil {
			continue
		}
		dgBefore, bytesBefore, _ := cs.sender.Stats()
		if err := cs.flowSample.EncodeFlowSample(sample, cs.sender); err != nil {
			log.Warn("flow-export: flow-sample send failed",
				"collector", cs.cfg.Name, "error", err)
			incErrors(cs.cfg.Name, cs.cfg.Protocol)
			continue
		}
		dgAfter, bytesAfter, _ := cs.sender.Stats()
		for range dgAfter - dgBefore {
			incDatagrams(cs.cfg.Name, cs.cfg.Protocol)
		}
		addBytes(cs.cfg.Name, cs.cfg.Protocol, float64(bytesAfter-bytesBefore))
	}
}

// ExportFlows enriches and dispatches per-flow records to every NetFlow v9 /
// IPFIX collector that has a flow-record encoder. Called from both conntrack
// worker goroutines -- the periodic dump and the destroy-event listener -- so
// the e.mu lock below is what serializes their concurrent fan-out. Enrichment
// (BGP next-hop / AS) is applied once per flow before fan-out to collectors.
func (e *Exporter) ExportFlows(flows []ConntrackFlow) {
	if len(flows) == 0 {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopped {
		return
	}

	if e.enricher != nil {
		for i := range flows {
			enr := e.enricher.Enrich(flows[i].SrcAddr, flows[i].DstAddr)
			flows[i].SrcAS = enr.SrcAS
			flows[i].DstAS = enr.DstAS
			flows[i].NextHop = enr.NextHop
		}
	}

	// Tap the (enriched) flows into the recent-flow ring for `show flow-recent`.
	// Independent of collector fan-out below: the DDoS characterizer needs the
	// ring even when no NetFlow/IPFIX collector is configured. No-op when the
	// ring is disabled (conntrack off).
	e.recent.append(flows)

	now := time.Now()
	log := loggerPtr.Load()
	for i := range e.collectors {
		cs := &e.collectors[i]
		if cs.flowRecord == nil {
			continue
		}

		// Flow template refresh shares the collector's template-refresh interval.
		refreshInterval := time.Duration(cs.cfg.TemplateRefresh) * time.Second
		if cs.flowTemplateLast.IsZero() || now.Sub(cs.flowTemplateLast) >= refreshInterval {
			if err := cs.flowRecord.EncodeFlowTemplate(cs.sender); err != nil {
				log.Warn("flow-export: flow-template send failed",
					"collector", cs.cfg.Name, "error", err)
				incErrors(cs.cfg.Name, cs.cfg.Protocol)
			} else {
				cs.flowTemplateLast = now
			}
		}

		dgBefore, bytesBefore, _ := cs.sender.Stats()
		records, err := cs.flowRecord.EncodeFlows(flows, cs.sender)
		if err != nil {
			log.Warn("flow-export: flow-record send failed",
				"collector", cs.cfg.Name, "error", err)
			incErrors(cs.cfg.Name, cs.cfg.Protocol)
			continue
		}
		dgAfter, bytesAfter, _ := cs.sender.Stats()
		cs.sequence += uint32(records)
		for range dgAfter - dgBefore {
			incDatagrams(cs.cfg.Name, cs.cfg.Protocol)
		}
		addBytes(cs.cfg.Name, cs.cfg.Protocol, float64(bytesAfter-bytesBefore))
		addFlows(cs.cfg.Name, float64(records))
	}

	feed := observation.Global()
	for i := range flows {
		f := &flows[i]
		obs := observation.Observation{
			Kind:    observation.KindFlow,
			Feature: observation.FeatureFlowBytes,
			Value:   float64(f.Bytes),
			At:      now,
		}
		obs.Flow.Src = f.SrcAddr
		obs.Flow.Dst = f.DstAddr
		obs.Flow.SrcPort = f.SrcPort
		obs.Flow.DstPort = f.DstPort
		obs.Flow.Proto = f.Protocol
		feed.Publish(obs)
	}
}

// RecentFlows returns a snapshot of the recent-flow ring in oldest-to-newest
// order, optionally filtered to flows destined into dst (zero-value prefix =
// all). Empty when conntrack export (and thus the ring) is disabled. Reads only
// the ring's own mutex, so it never contends with the exporter fan-out lock.
func (e *Exporter) RecentFlows(dst netip.Prefix) []ConntrackFlow {
	return e.recent.snapshot(dst)
}

// RecentDrops returns the cumulative number of recent-flow-ring entries
// overwritten before being read. Exposed for the recent-ring-drops metric.
func (e *Exporter) RecentDrops() uint64 {
	return e.recent.dropCount()
}

// Uptime returns milliseconds since the exporter started.
func (e *Exporter) Uptime() uint32 {
	return uint32(time.Since(e.startTime).Milliseconds())
}

// Status returns per-collector export statistics.
func (e *Exporter) Status() []map[string]any {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := make([]map[string]any, 0, len(e.collectors))
	for i := range e.collectors {
		cs := &e.collectors[i]
		datagrams, bytes, errors := cs.sender.Stats()
		m := map[string]any{
			"name":           cs.cfg.Name,
			"address":        cs.cfg.Address,
			"port":           cs.cfg.Port,
			"protocol":       cs.cfg.Protocol,
			"datagrams-sent": datagrams,
			"bytes-sent":     bytes,
			"errors":         errors,
			"sequence":       cs.sequence,
		}
		if !cs.lastPoll.IsZero() {
			m["last-export-time"] = cs.lastPoll.Unix()
		}
		result = append(result, m)
	}
	return result
}

// Stop tears down workers, closes all collector senders, and marks the
// exporter stopped.
func (e *Exporter) Stop() {
	e.mu.Lock()
	if e.stopped {
		e.mu.Unlock()
		return
	}
	e.stopped = true
	close(e.stopCh)
	stoppers := e.stoppers
	e.stoppers = nil
	e.mu.Unlock()

	// Run worker/builder teardown WITHOUT the lock: their goroutines call
	// ExportFlow*, which take e.mu and now observe stopped==true and return.
	for _, fn := range stoppers {
		fn()
	}

	e.mu.Lock()
	for i := range e.collectors {
		if e.collectors[i].sender != nil {
			_ = e.collectors[i].sender.Close()
		}
	}
	e.mu.Unlock()
}
