// Design: docs/architecture/diagnostics/production-diagnostics.md -- bounded reload transaction history

package server

import (
	"context"
	"slices"
	"sync"
	"time"
)

const (
	maxReloadHistory      = 32
	maxReloadChangedRoots = 32
)

// ReloadPluginStep records one plugin's timing and outcome during a reload transaction.
type ReloadPluginStep struct {
	Plugin     string `json:"plugin"`
	VerifyMS   int64  `json:"verify-ms,omitempty"`
	ApplyMS    int64  `json:"apply-ms,omitempty"`
	RollbackMS int64  `json:"rollback-ms,omitempty"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

// ReloadRecord is one entry in the bounded reload history ring.
type ReloadRecord struct {
	Generation   uint64             `json:"generation"`
	OpID         string             `json:"operation-id,omitempty"`
	Trigger      string             `json:"trigger"`
	Outcome      string             `json:"outcome"`
	StartedAt    time.Time          `json:"started-at"`
	FinishedAt   time.Time          `json:"finished-at"`
	DurationMS   int64              `json:"duration-ms"`
	ChangedRoots []string           `json:"changed-roots,omitempty"`
	TxState      string             `json:"tx-state,omitempty"`
	Plugins      []ReloadPluginStep `json:"plugins,omitempty"`
	Error        string             `json:"error,omitempty"`
}

type reloadTraceContextKey struct{}

// ReloadTrace collects changed roots, transaction state, and per-plugin steps
// during one config reload so doReloadContext can publish a ReloadRecord under
// reloadGen.mu without diagnostic readers ever taking Server.txLock.
type ReloadTrace struct {
	mu           sync.Mutex
	changedRoots []string
	txState      string
	plugins      []ReloadPluginStep
}

// WithReloadTrace attaches trace to ctx so plugin-server reload steps record
// changed roots and transaction step timings into it.
func WithReloadTrace(ctx context.Context, trace *ReloadTrace) context.Context {
	if trace == nil {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, reloadTraceContextKey{}, trace)
}

func reloadTraceFrom(ctx context.Context) *ReloadTrace {
	if ctx == nil {
		return nil
	}
	trace, _ := ctx.Value(reloadTraceContextKey{}).(*ReloadTrace)
	return trace
}

func (t *ReloadTrace) recordRoots(roots []string) {
	if t == nil {
		return
	}
	cleaned := make([]string, 0, len(roots))
	for _, r := range roots {
		if r != "" {
			cleaned = append(cleaned, r)
		}
	}
	slices.Sort(cleaned)
	cleaned = slices.Compact(cleaned)
	if len(cleaned) > maxReloadChangedRoots {
		cleaned = cleaned[:maxReloadChangedRoots]
	}
	t.mu.Lock()
	t.changedRoots = cleaned
	t.mu.Unlock()
}

func (t *ReloadTrace) recordTx(state string, steps []ReloadPluginStep) {
	if t == nil {
		return
	}
	copied := slices.Clone(steps)
	t.mu.Lock()
	t.txState = state
	t.plugins = copied
	t.mu.Unlock()
}

// Snapshot returns copies of the recorded changed roots, transaction state,
// and per-plugin step results. Safe for concurrent use.
func (t *ReloadTrace) Snapshot() ([]string, string, []ReloadPluginStep) {
	if t == nil {
		return nil, "", nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return slices.Clone(t.changedRoots), t.txState, slices.Clone(t.plugins)
}

func cloneReloadRecord(rec ReloadRecord) ReloadRecord {
	rec.ChangedRoots = slices.Clone(rec.ChangedRoots)
	rec.Plugins = slices.Clone(rec.Plugins)
	return rec
}
