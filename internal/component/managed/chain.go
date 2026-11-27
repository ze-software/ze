// Design: docs/architecture/fleet-config.md — managed client SHA-1 configuration hash chain state and metrics
// Related: client.go — uses ChainAccess, chainRuntime, and clientChainMetrics during sync and ping
// Related: baseline.go — BaselineAccess, the hold comparison that takes precedence over chain resync
// Related: cmd_fleet.go — reports ChainStatus in show fleet status
// Related: doctor.go — reports unreadable chain store state and active chain drift

package managed

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

// Client hash-chain status words, resync method labels, and timing bounds.
const (
	// ChainStateInSync reports that client and hub hash chains agree.
	ChainStateInSync = "in-sync"
	// ChainStateMismatched reports that client and hub hash chains disagree or resync failed.
	ChainStateMismatched = "mismatched"
	// ChainStateUninitialized reports that no hash chain state has been anchored yet.
	ChainStateUninitialized = "uninitialized"
	// ChainStateNoDaemon reports that no running daemon answered fleet status.
	ChainStateNoDaemon = "no-daemon"

	// LastResyncNone reports that no resync has applied in this daemon run.
	LastResyncNone = "none"
	// LastResyncDiff reports that the last applied change used diff mode.
	LastResyncDiff = "diff"
	// LastResyncFullReplay reports that the last applied change used full replay.
	LastResyncFullReplay = "full-replay"

	// ChainHashNone is reported in show fleet status when no chain hash is recorded.
	ChainHashNone = "none"

	// pingTimeout bounds one client-initiated VerbPing round-trip (< heartbeatInterval = 30s).
	pingTimeout = 10 * time.Second

	chainMetricMethodDiff     = "diff"
	chainMetricMethodFull     = "full"
	chainMetricResultApplied  = "applied"
	chainMetricResultFallback = "fallback"
	chainMetricResultFailed   = "failed"

	chainWarningSubject = "chain"
	chainDriftFix       = "ze show fleet status"
)

// ChainStatus is the runtime SHA-1 configuration hash chain status reported by
// a running managed client to its owner (show fleet status and runtime doctor).
type ChainStatus struct {
	ChainHash       string
	ChainState      string
	LastResync      string
	DriftError      string
	UnreadableError string
	WriteError      string
}

// ChainAccess reads and writes the managed client's persisted SHA-1
// configuration hash chain state under meta/managed/chain-state.
// The zero value is not usable: build one with NewChainAccess.
// Safe for concurrent use when the underlying store is safe for concurrent use.
type ChainAccess struct {
	read  func() ([]byte, error)
	write func([]byte) error
}

// NewChainAccess binds chain state reads and writes to store under
// zefs.KeyManagedChainState. store is borrowed: the caller MUST keep it open
// until the client holding the result has returned.
func NewChainAccess(store storage.Storage) *ChainAccess {
	key := zefs.KeyManagedChainState.Pattern
	return &ChainAccess{
		read:  func() ([]byte, error) { return store.ReadFile(key) },
		write: func(data []byte) error { return store.WriteKey(key, data) },
	}
}

// Read reads and strictly decodes the persisted client chain state from
// meta/managed/chain-state. It returns (zero, false, nil) ONLY when the key is
// absent (fs.ErrNotExist); corrupt JSON, unknown fields, trailing data,
// oversized values, or invalid chain hashes return an actionable error.
func (c *ChainAccess) Read() (fleet.ChainState, bool, error) {
	if c == nil {
		return fleet.ChainState{}, false, errors.New("managed chain access is not configured")
	}
	if c.read == nil {
		return fleet.ChainState{}, false, errors.New("managed chain access is not configured")
	}
	raw, err := c.read()
	if errors.Is(err, fs.ErrNotExist) {
		return fleet.ChainState{}, false, nil
	}
	if err != nil {
		return fleet.ChainState{}, false, fmt.Errorf("read %s: %w", zefs.KeyManagedChainState.Pattern, err)
	}
	state, err := decodeChainState(raw)
	if err != nil {
		return fleet.ChainState{}, false, fmt.Errorf("decode %s: %w", zefs.KeyManagedChainState.Pattern, err)
	}
	return state, true, nil
}

// Write validates state and writes compact JSON to meta/managed/chain-state
// only when the persisted value differs or cannot be decoded. Write failures
// are logged at Warn under doctor-managed-chain-unreadable.
func (c *ChainAccess) Write(state fleet.ChainState) error {
	if c == nil {
		return errors.New("managed chain access is not configured")
	}
	if c.write == nil {
		return errors.New("managed chain access is not configured")
	}
	if err := validateChainState(state); err != nil {
		return err
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", zefs.KeyManagedChainState.Pattern, err)
	}
	if len(encoded) > zefs.ManagedChainStateValueMax {
		return fmt.Errorf("encoded %s size %d exceeds max %d", zefs.KeyManagedChainState.Pattern, len(encoded), zefs.ManagedChainStateValueMax)
	}
	if c.read != nil {
		existing, present, readErr := c.Read()
		if readErr == nil && present && existing == state {
			return nil
		}
	}
	if err := c.write(encoded); err != nil {
		logger().Warn("the managed configuration hash chain state could not be written to the store",
			"code", pluginserver.CodeDoctorManagedChainUnreadable,
			"key", zefs.KeyManagedChainState.Pattern,
			"fix", fixCheckStore,
			"error", err)
		return fmt.Errorf("write %s: %w", zefs.KeyManagedChainState.Pattern, err)
	}
	return nil
}

// ReadChainState reads and strictly decodes meta/managed/chain-state from store.
func ReadChainState(store storage.Storage) (fleet.ChainState, bool, error) {
	return NewChainAccess(store).Read()
}

// RecordActiveChainAnchor reads the device's active configuration from store,
// computes its VersionHash and FoldFull SHA-1 chain anchor, and persists the
// resulting ChainState to meta/managed/chain-state.
func RecordActiveChainAnchor(store storage.Storage) (fleet.ChainState, error) {
	content, stored, err := activeConfig(store)
	if err != nil {
		return fleet.ChainState{}, fmt.Errorf("read active configuration: %w", err)
	}
	if !stored {
		return fleet.ChainState{}, errNoActiveConfig
	}
	state := fleet.ChainState{
		Version: fleet.VersionHash(content),
		Chain:   fleet.FoldFull(content),
	}
	if err := NewChainAccess(store).Write(state); err != nil {
		return fleet.ChainState{}, err
	}
	return state, nil
}

// decodeChainState strictly decodes raw JSON bytes into a validated ChainState.
func decodeChainState(raw []byte) (fleet.ChainState, error) {
	if len(raw) == 0 {
		return fleet.ChainState{}, errors.New("empty chain state value")
	}
	if len(raw) > zefs.ManagedChainStateValueMax {
		return fleet.ChainState{}, fmt.Errorf("chain state size %d exceeds max %d", len(raw), zefs.ManagedChainStateValueMax)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var state fleet.ChainState
	if err := dec.Decode(&state); err != nil {
		return fleet.ChainState{}, err
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fleet.ChainState{}, errors.New("unexpected trailing JSON value")
		}
		return fleet.ChainState{}, fmt.Errorf("unexpected trailing data: %w", err)
	}
	if err := validateChainState(state); err != nil {
		return fleet.ChainState{}, err
	}
	return state, nil
}

// validateChainState verifies that state carries a non-empty version hash and
// a valid 40-character lowercase hexadecimal SHA-1 chain hash.
func validateChainState(state fleet.ChainState) error {
	if state.Version == "" {
		return errors.New("chain state version is empty")
	}
	if !fleet.ValidChainHash(state.Chain) {
		return fmt.Errorf("invalid chain hash %q", state.Chain)
	}
	return nil
}

// clientChainMetrics holds the managed client's Prometheus hash-chain counters.
// All methods are nil-safe when no metrics.Registry is configured.
type clientChainMetrics struct {
	mismatches metrics.Counter
	resyncs    metrics.CounterVec
}

// newClientChainMetrics registers the client chain counters on reg, or returns
// nil when reg is nil.
func newClientChainMetrics(reg metrics.Registry) *clientChainMetrics {
	if reg == nil {
		return nil
	}
	return &clientChainMetrics{
		mismatches: reg.Counter(
			"ze_managed_client_chain_mismatches_total",
			"Total configuration hash chain mismatches detected by the managed client.",
		),
		resyncs: reg.CounterVec(
			"ze_managed_client_resyncs_total",
			"Total configuration resync attempts performed by the managed client.",
			[]string{"method", "result"},
		),
	}
}

func (m *clientChainMetrics) incMismatch() {
	if m == nil {
		return
	}
	if m.mismatches == nil {
		return
	}
	m.mismatches.Inc()
}

func (m *clientChainMetrics) incResync(method, result string) {
	if m == nil {
		return
	}
	if m.resyncs == nil {
		return
	}
	m.resyncs.With(method, result).Inc()
}

// chainRuntime holds the concurrency-safe in-memory hash-chain snapshot,
// failed-sync latch, and metrics handles shared between sendPing and
// notificationLoop within one client run.
// Safe for concurrent use.
type chainRuntime struct {
	mu                 sync.RWMutex
	status             ChainStatus
	failedVersion      string
	failedChain        string
	failedTriggerChain string
	failedReplyChain   string
	metrics            *clientChainMetrics
	onState            func(ChainStatus)
}

func newChainRuntime(reg metrics.Registry, onState func(ChainStatus)) *chainRuntime {
	return &chainRuntime{
		status: ChainStatus{
			ChainState: ChainStateUninitialized,
			LastResync: LastResyncNone,
		},
		metrics: newClientChainMetrics(reg),
		onState: onState,
	}
}

// loadFromStore initializes the in-memory chain snapshot from access without
// failing startup when the stored key is absent or unreadable.
func (r *chainRuntime) loadFromStore(access *ChainAccess) {
	if access == nil {
		return
	}
	state, present, err := access.Read()
	var snap ChainStatus
	r.mu.Lock()
	if err != nil {
		r.status.ChainHash = ""
		r.status.ChainState = ChainStateUninitialized
		r.status.UnreadableError = err.Error()
		snap = r.status
		r.mu.Unlock()
		logger().Warn("the stored managed configuration hash chain could not be read; waiting for full replay to heal",
			"code", pluginserver.CodeDoctorManagedChainUnreadable,
			"key", zefs.KeyManagedChainState.Pattern,
			"fix", fixCheckStore,
			"error", err)
		r.notify(snap)
		return
	}
	r.status.UnreadableError = ""
	if present {
		r.status.ChainHash = state.Chain
		if r.status.ChainState == "" {
			r.status.ChainState = ChainStateUninitialized
		}
	} else {
		r.status.ChainHash = ""
		r.status.ChainState = ChainStateUninitialized
	}
	snap = r.status
	r.mu.Unlock()
	r.notify(snap)
}

// currentChain returns the current 40-hex chain head (or empty when unanchored).
func (r *chainRuntime) currentChain() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status.ChainHash
}

// snapshot returns a copy of the current runtime ChainStatus.
func (r *chainRuntime) snapshot() ChainStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

// markInSync records that the client's hash chain converged to chainHash and
// clears any active failed-sync latch or drift error. When lastResync is
// non-empty it replaces LastResync. When writeErr is non-nil it preserves the
// runtime persistence failure on ChainStatus so doctor reports it; when nil it
// clears any prior write/unreadable error once chainHash is anchored.
func (r *chainRuntime) markInSync(chainHash, lastResync string, writeErr error) {
	r.mu.Lock()
	if chainHash != "" {
		r.status.ChainHash = chainHash
	}
	if writeErr != nil {
		r.status.WriteError = writeErr.Error()
		r.status.UnreadableError = writeErr.Error()
	} else {
		r.status.WriteError = ""
		if chainHash != "" {
			r.status.UnreadableError = ""
		}
	}
	r.status.ChainState = ChainStateInSync
	r.status.DriftError = ""
	if lastResync != "" {
		r.status.LastResync = lastResync
	}
	r.failedVersion = ""
	r.failedChain = ""
	r.failedTriggerChain = ""
	r.failedReplyChain = ""
	snap := r.status
	r.mu.Unlock()
	r.notify(snap)
}

// markFailed latches a failed resync attempt (including the hub's reply chain
// hash from the failure ack) so subsequent periodic pings advertising the same
// hub chain do not loop, and publishes ChainStateMismatched.
func (r *chainRuntime) markFailed(version, hubChain, triggerChain, replyChain, driftErr string) {
	r.mu.Lock()
	r.status.ChainState = ChainStateMismatched
	r.status.DriftError = driftErr
	r.failedVersion = version
	r.failedChain = hubChain
	r.failedTriggerChain = triggerChain
	r.failedReplyChain = replyChain
	snap := r.status
	r.mu.Unlock()
	r.notify(snap)
}

// resetSessionLatch clears the per-connection failed-sync latch on reconnect so
// a new connection retries convergence (AC-11).
func (r *chainRuntime) resetSessionLatch() {
	r.mu.Lock()
	r.failedVersion = ""
	r.failedChain = ""
	r.failedTriggerChain = ""
	r.failedReplyChain = ""
	r.mu.Unlock()
}

// isLatchedChain reports whether hubChain matches the failed resync latch for
// this connection.
func (r *chainRuntime) isLatchedChain(hubChain string) bool {
	if hubChain == "" {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.failedChain != "" && r.failedChain == hubChain {
		return true
	}
	if r.failedTriggerChain != "" && r.failedTriggerChain == hubChain {
		return true
	}
	if r.failedReplyChain != "" && r.failedReplyChain == hubChain {
		return true
	}
	return false
}

func (r *chainRuntime) recordMismatch() {
	r.metrics.incMismatch()
}

func (r *chainRuntime) recordResync(method, result string) {
	r.metrics.incResync(method, result)
}

func (r *chainRuntime) notify(snap ChainStatus) {
	if r.onState != nil {
		r.onState(snap)
	}
}

// activeBytes reads the raw bytes of the device's active configuration through
// access.held when configured.
func (access *BaselineAccess) activeBytes() ([]byte, error) {
	if access == nil {
		return nil, errors.New("active configuration bytes are unavailable")
	}
	if access.held == nil {
		return nil, errors.New("active configuration bytes are unavailable")
	}
	held, err := access.held()
	if err != nil {
		return nil, err
	}
	return held.content, nil
}

// reconstructAndVerifyDiff verifies a Mode=="diff" ConfigFetchResponse against
// activeBytes and currentChain, applies the line diff bounded by
// rpc.MaxMessageSize, verifies the reconstructed VersionHash, and runs full
// YANG validation through handler. Any error returns without staging anything
// and carries hashes/codes only (never configuration text).
func reconstructAndVerifyDiff(activeBytes []byte, currentChain string, resp fleet.ConfigFetchResponse, handler *Handler) ([]byte, error) {
	if resp.Diff == "" {
		return nil, errors.New("diff payload is empty")
	}
	if resp.BaseVersion == "" {
		return nil, errors.New("diff base-version is empty")
	}
	if resp.Version == "" {
		return nil, errors.New("diff target version is empty")
	}
	if !fleet.ValidChainHash(resp.ChainHash) {
		return nil, fmt.Errorf("diff response has invalid chain-hash %q", resp.ChainHash)
	}
	if gotBase := fleet.VersionHash(activeBytes); gotBase != resp.BaseVersion {
		return nil, fmt.Errorf("active version %s does not match diff base-version %s", gotBase, resp.BaseVersion)
	}
	delta, err := base64.StdEncoding.DecodeString(resp.Diff)
	if err != nil {
		return nil, fmt.Errorf("decode diff: %w", err)
	}
	prev := currentChain
	if resp.Reanchor {
		prev = fleet.FoldFull(activeBytes)
	}
	expectedChain, err := fleet.FoldDiff(prev, resp.BaseVersion, resp.Version, delta)
	if err != nil {
		return nil, fmt.Errorf("fold diff chain: %w", err)
	}
	if expectedChain != resp.ChainHash {
		return nil, fmt.Errorf("diff chain-hash mismatch: computed %s, hub declared %s", expectedChain, resp.ChainHash)
	}
	reconstructed, err := fleet.ApplyDiff(activeBytes, delta, rpc.MaxMessageSize)
	if err != nil {
		return nil, fmt.Errorf("apply diff: %w", err)
	}
	if gotTarget := fleet.VersionHash(reconstructed); gotTarget != resp.Version {
		return nil, fmt.Errorf("reconstructed version %s does not match target version %s", gotTarget, resp.Version)
	}
	if err := handler.ValidateRawConfig(reconstructed); err != nil {
		return nil, errors.New("reconstructed config failed validation")
	}
	return reconstructed, nil
}

// combineDiffAndFullError formats a bounded error string naming both the prior
// diff failure cause and the full-replay failure cause when both failed.
func combineDiffAndFullError(diffReason, fullReason string) string {
	if diffReason == "" {
		return fullReason
	}
	return "diff failed (" + diffReason + "); full replay failed (" + fullReason + ")"
}

// signalDrift performs a non-blocking send of hubChain onto driftCh so
// notificationLoop handles resync serially without blocking sendPing.
func signalDrift(driftCh chan<- string, hubChain string) {
	if driftCh == nil {
		return
	}
	if hubChain == "" {
		return
	}
	select {
	case driftCh <- hubChain:
	default:
	}
}

// initChainRuntime initializes or refreshes the per-connection chain runtime
// before any worker goroutine starts on cfg.
func (cfg *ClientConfig) initChainRuntime() {
	if cfg.chainState == nil {
		cfg.chainState = newChainRuntime(cfg.Metrics, cfg.OnChainState)
	}
	cfg.chainState.resetSessionLatch()
	cfg.chainState.loadFromStore(cfg.Chain)
}

// chain returns cfg's concurrency-safe chain runtime, lazily creating one for
// direct unit-test calls that bypass runConnection.
func (cfg *ClientConfig) chain() *chainRuntime {
	if cfg.chainState == nil {
		cfg.chainState = newChainRuntime(cfg.Metrics, cfg.OnChainState)
		cfg.chainState.loadFromStore(cfg.Chain)
	}
	return cfg.chainState
}

// currentChainHash returns the client's current in-memory 40-hex chain head
// without touching storage. Safe for concurrent use once initChainRuntime ran.
func (cfg *ClientConfig) currentChainHash() string {
	return cfg.chain().currentChain()
}

// pingBound returns the per-ping RPC timeout bound.
func (cfg *ClientConfig) pingBound() time.Duration {
	if cfg.pingTimeout > 0 {
		return cfg.pingTimeout
	}
	return pingTimeout
}

// pingTickInterval returns the periodic ping ticker interval.
func (cfg *ClientConfig) pingTickInterval() time.Duration {
	if cfg.pingInterval > 0 {
		return cfg.pingInterval
	}
	return heartbeatInterval
}

// checkReplyChain compares the hub's advertised chain hash from a ManagedReply
// against the client's current chain head and signals driftCh on mismatch.
func (cfg *ClientConfig) checkReplyChain(reply fleet.ManagedReply, driftCh chan<- string) {
	if reply.ChainHash == "" {
		return
	}
	rt := cfg.chain()
	if reply.ChainHash == rt.currentChain() {
		return
	}
	if rt.isLatchedChain(reply.ChainHash) {
		return
	}
	rt.recordMismatch()
	signalDrift(driftCh, reply.ChainHash)
}

// persistChainState writes the new chain anchor to storage (when ChainAccess is
// wired), updates the in-memory runtime status (preserving any write failure
// for runtime doctor checks), clears any active drift warning and failed-sync
// latch, and notifies OnChainState once with consistent state.
func (cfg *ClientConfig) persistChainState(version, chainHash, lastResync string) {
	var writeErr error
	if cfg.Chain != nil && version != "" && fleet.ValidChainHash(chainHash) {
		writeErr = cfg.Chain.Write(fleet.ChainState{Version: version, Chain: chainHash})
	}
	cfg.chain().markInSync(chainHash, lastResync, writeErr)
	report.ClearWarning(doctorComponent, pluginserver.CodeDoctorManagedChainDrift, chainWarningSubject)
}

// recordChainDriftFailure records a non-converging resync failure: increments
// the client resync failure metric, latches the failed hub target (including
// the hub's ack reply chain) so periodic pings do not loop, logs Warn under
// doctor-managed-chain-drift, and raises a transient report-bus warning with
// fix `ze show fleet status`.
func (cfg *ClientConfig) recordChainDriftFailure(version, hubChain, triggerChain, replyChain, method, cause string) {
	rt := cfg.chain()
	rt.recordResync(method, chainMetricResultFailed)
	rt.markFailed(version, hubChain, triggerChain, replyChain, cause)
	logger().Warn("managed configuration resync did not converge; keeping active configuration until hub config or local fault changes",
		"code", pluginserver.CodeDoctorManagedChainDrift,
		"name", cfg.Name,
		"version", version,
		"fix", chainDriftFix,
		"error", cause)
	report.RaiseWarningFix(
		doctorComponent,
		pluginserver.CodeDoctorManagedChainDrift,
		chainWarningSubject,
		report.FitMessage("managed configuration hash chain resync failed for version "+version+": "+cause),
		chainDriftFix,
	)
}
