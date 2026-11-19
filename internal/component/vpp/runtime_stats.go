// Design: docs/research/vpp-deployment-reference.md -- runtime statistics ownership
// Related: vpp.go -- the API-connected lifecycle enters and closes this owner

package vpp

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"go.fd.io/govpp/api"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	vppevents "github.com/ze-software/ze/internal/core/vpp/events"
)

type runtimeStatsClient interface {
	statsProvider
	statsDisconnector
}

// runtimeStats owns one stats connection and its optional telemetry worker.
// The manager MUST Close it before closing its binary API connection. Close
// cancels and joins the poller, withdraws this publication, then disconnects.
// Reads and Close are safe for concurrent use; reads serialize access to the
// GoVPP client's cached directories and refuse a retired API generation.
type runtimeStats struct {
	mu         sync.Mutex
	owner      *Connector
	generation uint64
	client     runtimeStatsClient
	closed     bool
	stopOnce   sync.Once
	cancel     context.CancelFunc
	done       chan struct{}
}

// GetInterfaceStatsForGeneration reads only the statistics published for this
// exact binary API owner and generation. A session MUST retain that pair from
// creation; a fresh active connector or stats provider is not a substitute.
// out is usable only on success. Safe for concurrent use.
func GetInterfaceStatsForGeneration(owner *Connector, generation uint64, out *api.InterfaceStats) error {
	if owner == nil {
		return errors.New("vpp: session statistics require an API owner")
	}
	if out == nil {
		return errors.New("vpp: session statistics require an output snapshot")
	}
	reader, ok := GetActiveStatsProvider().(*runtimeStats)
	if !ok {
		return errors.New("vpp: no generation-bound interface statistics are available")
	}
	if reader.owner != owner {
		return errors.New("vpp: interface statistics belong to another API owner")
	}
	if reader.generation != generation {
		return errVppGenerationChanged
	}
	return reader.GetInterfaceStats(out)
}

// activateRuntimeStats is the manager's post-API activation step. Statistics
// are attempted regardless of telemetry, and success or failure is published
// before observers receive connected/reconnected. The API can remain useful
// when stats fail; consumers needing counters receive an explicit refusal.
func (m *VPPManager) activateRuntimeStats(ctx context.Context,
	connect func(string) (runtimeStatsClient, error)) (*runtimeStats, error) {
	generation, err := m.connector.ConnectionGeneration()
	if err != nil {
		return nil, err
	}
	return m.activateRuntimeStatsGeneration(ctx, generation, connect)
}

// activateRuntimeStatsGeneration retains a completed bootstrap's exact epoch
// through statistics, plugin checks and the original final readiness fence.
func (m *VPPManager) activateRuntimeStatsGeneration(ctx context.Context, generation uint64,
	connect func(string) (runtimeStatsClient, error)) (*runtimeStats, error) {
	lease, err := m.connectRuntimeStatsGeneration(ctx, generation, connect)
	// refuse-by-default: guards VPPManager.runOnce -- cancellation ends activation without publishing readiness or a failed-service warning.
	if ctx.Err() != nil {
		if lease != nil {
			lease.Close()
		}
		return nil, ctx.Err()
	}
	if err != nil {
		reportRuntimeStatsFailure("connection", err)
		if errors.Is(err, errVppStatsPeerMismatch) {
			return nil, err
		}
		if errors.Is(err, errVppGenerationChanged) {
			return nil, err
		}
		logger().Warn("vpp: statistics unavailable; consumers requiring counters will refuse",
			"code", diagnostic.CodeDoctorVPPStats, "fix", runtimeStatsFix, "error", err)
	}
	if err := m.settings.checkStartupRuntime(ctx, m.connector); err != nil {
		if lease != nil {
			lease.Close()
		}
		return nil, err
	}
	current, err := m.connector.ConnectionGeneration()
	if err == nil {
		if current != generation {
			err = errVppGenerationChanged
		}
	}
	if err == nil {
		err = m.connector.checkAPIProcess(generation)
	}
	if err != nil {
		if lease != nil {
			lease.Close()
		}
		return nil, err
	}
	if m.hasRunOnce {
		m.emitEvent(vppevents.EventReconnected)
	} else {
		m.emitEvent(vppevents.EventConnected)
		m.hasRunOnce = true
	}
	m.ready = true
	m.notifyStartup(nil)
	return lease, nil
}

func (m *VPPManager) connectRuntimeStats(ctx context.Context,
	connect func(string) (runtimeStatsClient, error)) (*runtimeStats, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	generation, err := m.connector.ConnectionGeneration()
	if err != nil {
		return nil, err
	}
	return m.connectRuntimeStatsGeneration(ctx, generation, connect)
}

// connectRuntimeStatsGeneration MUST retain the supplied original epoch rather
// than select a replacement between the caller's entry check and this helper.
func (m *VPPManager) connectRuntimeStatsGeneration(ctx context.Context, generation uint64,
	connect func(string) (runtimeStatsClient, error)) (*runtimeStats, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	current, err := m.connector.ConnectionGeneration()
	if err != nil {
		return nil, err
	}
	if current != generation {
		return nil, errVppGenerationChanged
	}
	client, err := connect(m.settings.Stats.SocketPath)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, errors.New("vpp: stats connection returned no client")
	}
	if err := m.connector.checkStatsProcess(generation, client); err != nil {
		client.Disconnect()
		return nil, err
	}
	lease := &runtimeStats{owner: m.connector, generation: generation, client: client}
	var initial api.InterfaceStats
	if err := lease.GetInterfaceStats(&initial); err != nil {
		lease.Close()
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		lease.Close()
		return nil, err
	}
	if err := lease.publish(ctx); err != nil {
		lease.Close()
		return nil, err
	}
	report.ClearWarning(runtimeStatsReportSource, diagnostic.CodeDoctorVPPStats, "connection")
	if reg := getMetricsRegistry(); reg != nil {
		pollerCtx, cancel := context.WithCancel(ctx)
		lease.cancel = cancel
		lease.done = make(chan struct{})
		interval := time.Duration(m.settings.Stats.PollInterval) * time.Second
		poller := newStatsPoller(lease, newVPPMetrics(reg), interval)
		go func() { defer close(lease.done); poller.run(pollerCtx) }()
		logger().Info("vpp: stats telemetry started", "interval", interval)
	}
	return lease, nil
}

// publish admits a fresh unpublished lease only while its original API epoch
// is live. The caller MUST own this newly constructed lease exclusively. The
// connector lock covers the final check and provider installation together;
// only bounded nonblocking pidfd observations run under it; no API or socket
// I/O runs under it. Readers release statsProviderMu before entering a lease,
// so the lock order is connector.mu then statsProviderMu, never the reverse.
func (s *runtimeStats) publish(ctx context.Context) error {
	s.owner.mu.Lock()
	defer s.owner.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if !s.owner.connected {
		return errVppNotConnected
	}
	if s.owner.conn == nil {
		return errVppNotConnected
	}
	if s.owner.generation != s.generation {
		return errVppGenerationChanged
	}
	if err := s.owner.checkStatsProcessLocked(s.generation, s.client); err != nil {
		return err
	}
	statsProviderMu.Lock()
	statsProviderRef = s
	statsProviderMu.Unlock()
	return nil
}

func (s *runtimeStats) GetInterfaceStats(out *api.InterfaceStats) error {
	return s.read("interfaces", func() error { return s.client.GetInterfaceStats(out) })
}
func (s *runtimeStats) GetNodeStats(out *api.NodeStats) error {
	return s.read("nodes", func() error { return s.client.GetNodeStats(out) })
}
func (s *runtimeStats) GetSystemStats(out *api.SystemStats) error {
	return s.read("system", func() error { return s.client.GetSystemStats(out) })
}

func (s *runtimeStats) read(subject string, read func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	if err := s.checkGeneration(); err != nil {
		return err
	}
	if err := read(); err != nil {
		reportRuntimeStatsFailure(subject, err)
		return err
	}
	if err := s.checkGeneration(); err != nil {
		return err
	}
	// This exact category was read successfully from the same live owner;
	// success from another category or API generation cannot clear it.
	report.ClearWarning(runtimeStatsReportSource, diagnostic.CodeDoctorVPPStats, subject)
	return nil
}

func (s *runtimeStats) checkGeneration() error {
	current, err := s.owner.ConnectionGeneration()
	if err != nil {
		return err
	}
	if current != s.generation {
		return errVppGenerationChanged
	}
	return s.owner.checkStatsProcess(s.generation, s.client)
}

func (s *runtimeStats) Close() {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		if s.done != nil {
			<-s.done
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.closed = true
		statsProviderMu.Lock()
		if statsProviderRef == s {
			statsProviderRef = nil
		}
		statsProviderMu.Unlock()
		s.client.Disconnect()
	})
}
