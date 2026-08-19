// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- packet sampling lifecycle
// Related: sampling/tc_linux.go -- SetupSampling / RemoveSampling (tc sample action)
// Related: sampling/psample_linux.go -- PsampleReader (generic netlink reception)

package flowexport

import (
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/flowexport/sampling"
)

const (
	// maxConsecutiveReadErrors is how many back-to-back psample read errors
	// are tolerated before the reader backs off. A sustained stream of
	// unparseable kernel messages would otherwise spin the CPU at 100%.
	maxConsecutiveReadErrors = 10
	// psampleErrorBackoff is the pause inserted after the error threshold.
	psampleErrorBackoff = 100 * time.Millisecond
)

// samplingWorker installs tc sample actions on the configured interfaces and
// runs a single long-lived goroutine that reads sampled packets from the
// kernel psample group and dispatches them to the exporter's sFlow
// collectors as flow samples.
//
// Platform specifics live in the sampling package: on non-Linux hosts
// SetupSampling and NewPsampleReader return errors and the worker degrades
// to a no-op (logged once).
type samplingWorker struct {
	exp    *exporter
	cfgs   []SamplingConfig
	reader *sampling.PsampleReader

	idxToName map[uint32]string
	stopped   atomic.Bool
	doneCh    chan struct{}
}

func newSamplingWorker(exp *exporter, cfgs []SamplingConfig) *samplingWorker {
	return &samplingWorker{
		exp:       exp,
		cfgs:      cfgs,
		idxToName: make(map[uint32]string),
		doneCh:    make(chan struct{}),
	}
}

// Start installs sampling on each configured interface and launches the
// reader goroutine. Failures are logged; a failed setup leaves the worker
// idle rather than aborting the whole exporter.
func (w *samplingWorker) Start() {
	log := loggerPtr.Load()

	for i := range w.cfgs {
		c := &w.cfgs[i]
		if err := sampling.SetupSampling(c.Interface, c.Rate, c.Group, c.TruncSize); err != nil {
			log.Warn("flow-export: tc sample setup failed",
				"interface", c.Interface, "error", err)
			continue
		}
		if b, err := iface.Resolve(c.Interface); err == nil {
			w.idxToName[uint32(b.Ifindex)] = c.Interface //nolint:gosec // ifindex is a small positive kernel value
		}
		log.Info("flow-export: packet sampling enabled",
			"interface", c.Interface, "rate", c.Rate, "group", c.Group)
	}

	reader, err := sampling.NewPsampleReader()
	if err != nil {
		log.Warn("flow-export: psample reader unavailable; sampling idle", "error", err)
		close(w.doneCh)
		return
	}
	w.reader = reader

	go w.run()
}

func (w *samplingWorker) run() {
	defer close(w.doneCh)
	consecutiveErrs := 0
	for {
		pkt, err := w.reader.Read()
		if err != nil {
			// A closed socket (Stop) and transient parse errors both surface
			// here. Exit on stop; otherwise keep reading, but back off if the
			// errors are sustained so a stream of unparseable kernel messages
			// cannot spin the CPU.
			if w.stopped.Load() {
				return
			}
			consecutiveErrs++
			if consecutiveErrs == maxConsecutiveReadErrors {
				loggerPtr.Load().Warn("flow-export: repeated psample read errors, backing off", "error", err)
			}
			if consecutiveErrs >= maxConsecutiveReadErrors {
				time.Sleep(psampleErrorBackoff)
			}
			continue
		}
		consecutiveErrs = 0

		// Samples do NOT arrive only for the interfaces ze set up. The reader
		// joins the psample multicast group, which carries every producer on
		// this host, and parsePsampleMessage (sampling/psample.go) does not read
		// the sample-group attribute, so the group ze configures is write-only:
		// it reaches the kernel through the tc action and is never compared on
		// the way back.
		//
		// A sample from another producer is exported anyway, and that is a
		// decision rather than an oversight (owner, 2026-08-18): filtering on
		// the group would make ze export nothing, and say nothing, if the
		// configured group and the installed tc action ever drifted apart. The
		// metric carries a generic label when the index is not one ze set up.
		name := w.idxToName[pkt.IfIndex]
		if name == "" {
			name = "unknown"
		}
		incSamples(name)

		w.exp.exportFlowSample(FlowSample{
			IfIndex:  pkt.IfIndex,
			Rate:     pkt.Rate,
			OrigSize: pkt.OrigSize,
			Header:   pkt.Header,
		})
	}
}

// Stop removes the tc sample actions, closes the reader (unblocking the
// goroutine), and waits for it to exit.
func (w *samplingWorker) Stop() {
	w.stopped.Store(true)
	if w.reader != nil {
		_ = w.reader.Close()
	}
	for i := range w.cfgs {
		if err := sampling.RemoveSampling(w.cfgs[i].Interface); err != nil {
			loggerPtr.Load().Debug("flow-export: tc sample teardown",
				"interface", w.cfgs[i].Interface, "error", err)
		}
	}
	<-w.doneCh
}
