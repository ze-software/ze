package detect

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/zefs"
)

func makeSamples(n int, v float64) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = v
	}
	return s
}

// zefsStateClient adapts a real storage.Storage tree to the SDK stateClient
// seam so persistence tests exercise real zefs storage without global state.
type zefsStateClient struct {
	store storage.Storage
}

func (c *zefsStateClient) StateGet(_ context.Context, key string) ([]byte, bool, error) {
	data, err := c.store.ReadFile(key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func (c *zefsStateClient) StatePut(_ context.Context, key string, data []byte) error {
	return c.store.WriteFile(key, data, 0o600)
}

// useBaselineStore creates a private zefs tree wrapped as a stateClient and
// closes it in t.Cleanup.
func useBaselineStore(t *testing.T) *zefsStateClient {
	t.Helper()
	bs, err := storage.Create(t.TempDir())
	if err != nil {
		t.Fatalf("storage.Create: %v", err)
	}
	t.Cleanup(func() {
		if err := bs.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	})
	return &zefsStateClient{store: bs}
}

// VALIDATES: a persisted snapshot round-trips through save + load unchanged, via zefs.
func TestSaveLoadBaselinesRoundTrip(t *testing.T) {
	store := useBaselineStore(t)
	pps := baselineState{Samples: makeSamples(300, 1000), Count: 300, P99Cache: 1000}
	bps := baselineState{Samples: makeSamples(300, 20000), Count: 300, P99Cache: 20000}
	if err := saveBaselines(context.Background(), store, pps, bps); err != nil {
		t.Fatalf("save: %v", err)
	}
	blob, ok := loadBaselines(context.Background(), store)
	if !ok {
		t.Fatal("load returned ok=false for a freshly saved snapshot")
	}
	if blob.Version != baselineStateVersion {
		t.Errorf("version = %d, want %d", blob.Version, baselineStateVersion)
	}
	if len(blob.Pps.Samples) != 300 || blob.Pps.P99Cache != 1000 {
		t.Errorf("pps state not preserved: %+v", blob.Pps)
	}
	if len(blob.Bps.Samples) != 300 || blob.Bps.P99Cache != 20000 {
		t.Errorf("bps state not preserved: %+v", blob.Bps)
	}
}

// VALIDATES: AC-6 -- a version mismatch is rejected (warm fresh, no crash).
func TestLoadBaselines_RejectsVersion(t *testing.T) {
	store := useBaselineStore(t)
	if err := store.StatePut(context.Background(), zefs.KeyDDoSDetectBaseline.Pattern,
		[]byte(`{"version":999,"pps":{"samples":[1,2,3]},"bps":{"samples":[]}}`)); err != nil {
		t.Fatal(err)
	}
	if _, ok := loadBaselines(context.Background(), store); ok {
		t.Error("expected ok=false for a version mismatch")
	}
}

// VALIDATES: AC-6 -- a missing store or corrupt blob is rejected without error.
func TestLoadBaselines_MissingAndCorrupt(t *testing.T) {
	// No state client attached: best-effort no-op restore.
	if _, ok := loadBaselines(context.Background(), nil); ok {
		t.Error("expected ok=false when no store is registered")
	}
	// Corrupt blob under the key in a registered store.
	store := useBaselineStore(t)
	if err := store.StatePut(context.Background(), zefs.KeyDDoSDetectBaseline.Pattern, []byte(`{not json`)); err != nil {
		t.Fatal(err)
	}
	if _, ok := loadBaselines(context.Background(), store); ok {
		t.Error("expected ok=false for corrupt JSON")
	}
}

// VALIDATES: saveBaselines and loadBaselines travel over sdk.Plugin.StatePut /
// StateGet using zefs.KeyDDoSDetectBaseline.Pattern.
func TestSaveLoadBaselinesOverSDKStateRPC(t *testing.T) {
	pluginSide, engineSide := net.Pipe()
	p := sdk.NewWithConn(Name, pluginSide)
	mux := rpc.NewMuxConn(rpc.NewConn(engineSide, engineSide))
	t.Cleanup(func() {
		_ = p.Close()
		_ = mux.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var stored []byte
	done := make(chan error, 1)
	go func() {
		for range 2 {
			req := <-mux.Requests()
			if req == nil {
				done <- errors.New("connection closed before both state RPCs completed")
				return
			}
			var in rpc.StateInput
			if err := json.Unmarshal(req.Params, &in); err != nil {
				done <- err
				return
			}
			if in.Key != zefs.KeyDDoSDetectBaseline.Pattern {
				done <- errors.New("unexpected state key: " + in.Key)
				return
			}
			switch req.Method {
			case rpc.MethodStatePut:
				stored = append([]byte(nil), in.Data...)
				if err := mux.SendResult(ctx, req.ID, rpc.StateOutput{Status: rpc.StateOK}); err != nil {
					done <- err
					return
				}
			case rpc.MethodStateGet:
				if err := mux.SendResult(ctx, req.ID, rpc.StateOutput{Status: rpc.StateOK, Data: stored}); err != nil {
					done <- err
					return
				}
			default:
				done <- errors.New("unexpected RPC method: " + req.Method)
				return
			}
		}
		done <- nil
	}()

	pps := baselineState{Samples: makeSamples(100, 500), Count: 100, P99Cache: 500}
	bps := baselineState{Samples: makeSamples(100, 10000), Count: 100, P99Cache: 10000}
	if err := saveBaselines(ctx, p, pps, bps); err != nil {
		t.Fatalf("saveBaselines over SDK: %v", err)
	}
	blob, ok := loadBaselines(ctx, p)
	if !ok {
		t.Fatal("loadBaselines over SDK returned ok=false")
	}
	if len(blob.Pps.Samples) != 100 || blob.Pps.P99Cache != 500 {
		t.Errorf("pps state not preserved: %+v", blob.Pps)
	}
	if len(blob.Bps.Samples) != 100 || blob.Bps.P99Cache != 10000 {
		t.Errorf("bps state not preserved: %+v", blob.Bps)
	}
	if err := <-done; err != nil {
		t.Fatalf("engine RPC handler: %v", err)
	}
}

// VALIDATES: AC-5 -- a warmed baseline snapshot restores into a fresh baseline,
// preserving readiness and p99 (the restart skips the window re-warm).
func TestBaselineSnapshotRestoreRoundTrip(t *testing.T) {
	src := newBaseline(300, 3.0, 5000)
	for range 300 {
		src.Add(1000, false)
	}
	st := src.snapshot()

	dst := newBaseline(300, 3.0, 5000)
	if !dst.restore(st) {
		t.Fatal("restore rejected a valid full snapshot")
	}
	if !dst.Ready() {
		t.Error("restored baseline should be Ready (no warm-up needed)")
	}
	if dst.P99() != src.P99() {
		t.Errorf("restored p99 = %v, want %v", dst.P99(), src.P99())
	}
}

// VALIDATES: AC-6 -- restore rejects too-few-samples, NaN, Inf, and negative values,
// leaving the baseline empty so it warms fresh.
func TestBaselineRestore_RejectsBadState(t *testing.T) {
	cases := []struct {
		name string
		st   baselineState
	}{
		{"too-few", baselineState{Samples: makeSamples(10, 1000), Count: 10, P99Cache: 1000}},
		{"nan-sample", baselineState{Samples: append(makeSamples(60, 1000), math.NaN()), Count: 61, P99Cache: 1000}},
		{"inf-sample", baselineState{Samples: append(makeSamples(60, 1000), math.Inf(1)), Count: 61, P99Cache: 1000}},
		{"negative-sample", baselineState{Samples: append(makeSamples(60, 1000), -5), Count: 61, P99Cache: 1000}},
		{"nan-p99", baselineState{Samples: makeSamples(60, 1000), Count: 60, P99Cache: math.NaN()}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := newBaseline(300, 3.0, 5000)
			if b.restore(tc.st) {
				t.Error("restore accepted invalid state")
			}
			if b.Ready() || len(b.samples) != 0 {
				t.Error("rejected restore should leave the baseline empty")
			}
		})
	}
}
