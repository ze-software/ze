package managed

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

// testChainMetrics is a concurrency-safe metrics.Registry test double for
// verifying client chain mismatch and resync counter increments.
type testChainMetrics struct {
	mu       sync.Mutex
	counters map[string]float64
}

func newTestChainMetrics() *testChainMetrics {
	return &testChainMetrics{counters: make(map[string]float64)}
}

func (m *testChainMetrics) count(key string) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.counters[key]
}

func (m *testChainMetrics) Counter(name, _ string) metrics.Counter {
	return testCounter{m: m, key: name}
}

func (m *testChainMetrics) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	return testCounterVec{m: m, name: name}
}

func (m *testChainMetrics) Gauge(_, _ string) metrics.Gauge {
	return metrics.NopRegistry{}.Gauge("", "")
}

func (m *testChainMetrics) GaugeVec(_, _ string, _ []string) metrics.GaugeVec {
	return metrics.NopRegistry{}.GaugeVec("", "", nil)
}

func (m *testChainMetrics) Histogram(_, _ string, _ []float64) metrics.Histogram {
	return metrics.NopRegistry{}.Histogram("", "", nil)
}

func (m *testChainMetrics) HistogramVec(_, _ string, _ []float64, _ []string) metrics.HistogramVec {
	return metrics.NopRegistry{}.HistogramVec("", "", nil, nil)
}

type testCounter struct {
	m   *testChainMetrics
	key string
}

func (c testCounter) Inc() { c.Add(1) }
func (c testCounter) Add(v float64) {
	c.m.mu.Lock()
	defer c.m.mu.Unlock()
	c.m.counters[c.key] += v
}

type testCounterVec struct {
	m    *testChainMetrics
	name string
}

func (cv testCounterVec) With(labelValues ...string) metrics.Counter {
	return testCounter{m: cv.m, key: cv.name + ":" + strings.Join(labelValues, ",")}
}

func (cv testCounterVec) Delete(...string) bool { return true }

// TestClientDiffApplyAndChainPersistence proves that a managed client applies
// a Mode=="diff" config-fetch reply, verifies VersionHash and FoldDiff
// ChainHash, validates through Handler.Validate, commits via CommitOver,
// persists meta/managed/chain-state, and sends config-ack carrying the folded
// chain hash.
//
// VALIDATES: spec-fleet-10 AC-4 and AC-12 (client diff apply, CommitOver guard,
// chain persistence, and client resync metrics).
// PREVENTS: a client skipping chain verification, writing chain state before
// CommitOver succeeds, or omitting chain-hash from config-ack.
func TestClientDiffApplyAndChainPersistence(t *testing.T) {
	t.Parallel()

	const baseBlock = "system {\n\thost edge-01;\n\tdomain example.net;\n}\n" +
		"interfaces {\n\tinterface eth0 {\n\t\tdescription \"uplink transit primary\";\n\t\tmtu 1500;\n\t}\n" +
		"\tinterface eth1 {\n\t\tdescription \"uplink transit backup\";\n\t\tmtu 1500;\n\t}\n}\n"
	v1Bytes := []byte(baseBlock + "bgp {\n\trouter-id 192.0.2.1;\n\tas 65001;\n}\n")
	v2Bytes := []byte(baseBlock + "bgp {\n\trouter-id 192.0.2.2;\n\tas 65001;\n}\n")
	v1 := fleet.VersionHash(v1Bytes)
	v2 := fleet.VersionHash(v2Bytes)
	c1 := fleet.FoldFull(v1Bytes)

	delta, worth, err := fleet.ComputeDiff(v1Bytes, v2Bytes)
	require.NoError(t, err)
	require.True(t, worth)
	c2, err := fleet.FoldDiff(c1, v1, v2, delta)
	require.NoError(t, err)

	store := holdDevice(t, string(v1Bytes), string(v1Bytes))
	chainAccess := NewChainAccess(store)
	require.NoError(t, chainAccess.Write(fleet.ChainState{Version: v1, Chain: c1}))

	reg := newTestChainMetrics()
	var validatedBytes []byte
	var committedBytes []byte
	var lastStatus ChainStatus
	access := NewBaselineAccess(store)

	cfg := ClientConfig{
		Name:     "edge-01",
		Baseline: access,
		Chain:    chainAccess,
		Metrics:  reg,
		OnChainState: func(st ChainStatus) {
			lastStatus = st
		},
		Handler: &Handler{
			Validate: func(data []byte) error {
				validatedBytes = append([]byte(nil), data...)
				return nil
			},
		},
		CommitOver: stagedOver(access, func(data []byte) error {
			committedBytes = append([]byte(nil), data...)
			require.NoError(t, store.WriteFile(fleetStatusConfigName, data, 0))
			_, baseErr := RecordActiveAsBaseline(store)
			return baseErr
		}),
	}

	mc, seen := holdPipe(t, func(req *rpc.Request) any {
		switch req.Method {
		case fleet.VerbConfigFetch:
			var fetch fleet.ConfigFetchRequest
			require.NoError(t, json.Unmarshal(req.Params, &fetch))
			assert.Equal(t, v1, fetch.Version)
			assert.Equal(t, c1, fetch.ChainHash)
			return fleet.ConfigFetchResponse{
				Mode:        fleet.FetchModeDiff,
				BaseVersion: v1,
				Version:     v2,
				Diff:        base64.StdEncoding.EncodeToString(delta),
				ChainHash:   c2,
			}
		case fleet.VerbConfigAck:
			return fleet.ManagedReply{ChainHash: c2}
		default:
			return fleet.ManagedReply{}
		}
	})

	require.NoError(t, fetchAndProcess(context.Background(), mc, &cfg))

	require.Equal(t, fleet.VerbConfigFetch, nextRequest(t, seen).Method)
	ack := ackOf(t, seen)
	assert.True(t, ack.OK)
	assert.Equal(t, v2, ack.Version)
	assert.Equal(t, c2, ack.ChainHash)
	assert.Empty(t, ack.RejectKind)

	assert.Equal(t, v2Bytes, validatedBytes, "diff-reconstructed bytes must pass Handler.Validate")
	assert.Equal(t, v2Bytes, committedBytes, "diff-reconstructed bytes must commit through CommitOver")

	persisted, present, err := chainAccess.Read()
	require.NoError(t, err)
	require.True(t, present)
	assert.Equal(t, fleet.ChainState{Version: v2, Chain: c2}, persisted)

	assert.Equal(t, c2, lastStatus.ChainHash)
	assert.Equal(t, ChainStateInSync, lastStatus.ChainState)
	assert.Equal(t, LastResyncDiff, lastStatus.LastResync)
	assert.Equal(t, 1.0, reg.count("ze_managed_client_resyncs_total:diff,applied"))
	assert.Zero(t, reg.count("ze_managed_client_resyncs_total:diff,fallback"))
}

// TestChainAccessStrictDecodeAndHealing proves ChainAccess fails closed on
// corrupt, unknown-field, trailing-data, oversized, or invalid-hash values
// rather than treating them as absent, skips redundant writes when unchanged,
// and overwrites a damaged key on a valid Write.
//
// VALIDATES: spec-fleet-10 AC-12 and R-9 (strict decode and heal on overwrite).
// PREVENTS: corrupt meta/managed/chain-state silently treated as empty.
func TestChainAccessStrictDecodeAndHealing(t *testing.T) {
	t.Parallel()

	t.Run("absent key returns not present with no error", func(t *testing.T) {
		t.Parallel()
		store, err := storage.Create(t.TempDir())
		require.NoError(t, err)
		t.Cleanup(func() { _ = store.Close() })

		st, present, err := NewChainAccess(store).Read()
		require.NoError(t, err)
		assert.False(t, present)
		assert.Equal(t, fleet.ChainState{}, st)
	})

	validChain := fleet.FoldFull([]byte("system { host r1; }\n"))
	validVer := fleet.VersionHash([]byte("system { host r1; }\n"))

	badCases := map[string]string{
		"empty":           "",
		"corrupt json":    "{not json",
		"unknown field":   `{"version":"` + validVer + `","chain":"` + validChain + `","extra":1}`,
		"trailing json":   `{"version":"` + validVer + `","chain":"` + validChain + `"}{}`,
		"empty version":   `{"version":"","chain":"` + validChain + `"}`,
		"short chain":     `{"version":"` + validVer + `","chain":"deadbeef"}`,
		"uppercase chain": `{"version":"` + validVer + `","chain":"` + strings.ToUpper(validChain) + `"}`,
		"oversized":       strings.Repeat("a", zefs.ManagedChainStateValueMax+1),
	}
	for name, raw := range badCases {
		t.Run("rejects "+name, func(t *testing.T) {
			t.Parallel()
			access := &ChainAccess{
				read:  func() ([]byte, error) { return []byte(raw), nil },
				write: func([]byte) error { return nil },
			}
			_, present, err := access.Read()
			require.Error(t, err)
			assert.False(t, present)
		})
	}

	t.Run("write is idempotent and heals corrupt key", func(t *testing.T) {
		t.Parallel()
		var stored []byte
		writes := 0
		access := &ChainAccess{
			read: func() ([]byte, error) { return stored, nil },
			write: func(data []byte) error {
				writes++
				stored = append([]byte(nil), data...)
				return nil
			},
		}
		stored = []byte("{corrupt")
		_, _, err := access.Read()
		require.Error(t, err)

		want := fleet.ChainState{Version: validVer, Chain: validChain}
		require.NoError(t, access.Write(want))
		assert.Equal(t, 1, writes, "valid write must heal corrupt key")

		got, present, err := access.Read()
		require.NoError(t, err)
		require.True(t, present)
		assert.Equal(t, want, got)

		require.NoError(t, access.Write(want))
		assert.Equal(t, 1, writes, "unchanged chain state must skip redundant store write")
	})

	t.Run("write failure returns error", func(t *testing.T) {
		t.Parallel()
		access := &ChainAccess{
			read:  func() ([]byte, error) { return nil, errors.New("read failed") },
			write: func([]byte) error { return errors.New("disk full") },
		}
		err := access.Write(fleet.ChainState{Version: validVer, Chain: validChain})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "disk full")
	})
}

// TestRecordActiveChainAnchor proves RecordActiveChainAnchor hashes the store's
// active configuration bytes and persists meta/managed/chain-state.
//
// VALIDATES: spec-fleet-10 AC-3 (active config anchor helper).
// PREVENTS: chain anchor diverging from the stored active configuration bytes.
func TestRecordActiveChainAnchor(t *testing.T) {
	t.Parallel()

	cfgBytes := []byte("system { host anchor-test; }\n")
	store := holdDevice(t, string(cfgBytes), "")

	recorded, err := RecordActiveChainAnchor(store)
	require.NoError(t, err)
	assert.Equal(t, fleet.VersionHash(cfgBytes), recorded.Version)
	assert.Equal(t, fleet.FoldFull(cfgBytes), recorded.Chain)

	got, present, err := ReadChainState(store)
	require.NoError(t, err)
	require.True(t, present)
	assert.Equal(t, recorded, got)
}

// TestPersistChainStatePreservesAndClearsWriteError proves that a store write
// failure during persistChainState is NOT clobbered by markInSync, fires
// OnChainState once with consistent WriteError/UnreadableError, and clears both
// fields on a subsequent successful write.
//
// VALIDATES: spec-fleet-10 AC-12 (client chain-state write failures survive in
// ChainStatus so runtime doctor reports doctor-managed-chain-unreadable).
// PREVENTS: markInSync wiping a write error immediately after Chain.Write fails.
func TestPersistChainStatePreservesAndClearsWriteError(t *testing.T) {
	t.Parallel()

	v1Bytes := []byte("system { host write-err-1; }\n")
	v2Bytes := []byte("system { host write-err-2; }\n")
	v1 := fleet.VersionHash(v1Bytes)
	c1 := fleet.FoldFull(v1Bytes)
	v2 := fleet.VersionHash(v2Bytes)
	c2 := fleet.FoldFull(v2Bytes)

	var stored []byte
	failWrite := true
	access := &ChainAccess{
		read: func() ([]byte, error) {
			if stored == nil {
				return nil, errors.New("not initialized")
			}
			return stored, nil
		},
		write: func(data []byte) error {
			if failWrite {
				return errors.New("disk full")
			}
			stored = append([]byte(nil), data...)
			return nil
		},
	}

	var notifications []ChainStatus
	cfg := ClientConfig{
		Name:  "edge-write-err",
		Chain: access,
		OnChainState: func(st ChainStatus) {
			notifications = append(notifications, st)
		},
	}

	// Lazily initialize chainRuntime first so we only count persistChainState notifications.
	cfg.chain()
	notifications = nil

	cfg.persistChainState(v1, c1, LastResyncFullReplay)
	require.Len(t, notifications, 1, "persistChainState must notify OnChainState exactly once")
	assert.Equal(t, ChainStateInSync, notifications[0].ChainState)
	assert.Equal(t, c1, notifications[0].ChainHash)
	assert.Equal(t, LastResyncFullReplay, notifications[0].LastResync)
	assert.Contains(t, notifications[0].WriteError, "disk full")
	assert.Contains(t, notifications[0].UnreadableError, "disk full")

	snap := cfg.chain().snapshot()
	assert.Contains(t, snap.WriteError, "disk full")
	assert.Contains(t, snap.UnreadableError, "disk full")

	// Next write succeeds: both WriteError and UnreadableError must clear.
	failWrite = false
	cfg.persistChainState(v2, c2, LastResyncDiff)
	require.Len(t, notifications, 2)
	assert.Equal(t, ChainStateInSync, notifications[1].ChainState)
	assert.Equal(t, c2, notifications[1].ChainHash)
	assert.Equal(t, LastResyncDiff, notifications[1].LastResync)
	assert.Empty(t, notifications[1].WriteError)
	assert.Empty(t, notifications[1].UnreadableError)
}
