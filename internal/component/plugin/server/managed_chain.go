// Design: docs/architecture/fleet-config.md -- hub-side SHA-1 configuration hash chain store and historical base lookup
// Related: managed.go -- HandleConfigFetch uses managedChainStore and readBaseVersion
// Related: managed_serve.go -- handleRequest and handleConfigAck update chain state and resync failure records
// Related: managed_doctor.go -- ManagedRuntimeHealth reads chain drift and unreadable entries

package server

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	managedChainCheckFix     = "ze show data check"
	managedChainDoctorFix    = "ze show doctor"
	managedChainEntryPrefix  = "sha256:"
	managedChainKeySuffix    = "/chain"
	managedChainDefaultCause = "chain mismatch did not converge"
)

// ManagedResyncFailure records the bounded hub-side facts of a client's most
// recent non-verify failed resync attempt (where both diff resync and full
// replay failed). Alarm evaluators and doctor checks read value copies.
type ManagedResyncFailure struct {
	FailedAt      time.Time
	DiffReason    string
	FullReason    string
	ClientChain   string
	ExpectedChain string
}

// ManagedChainDoctorEntry reports one managed client whose SHA-1 configuration
// hash chain failed to converge after diff resync and full replay.
type ManagedChainDoctorEntry struct {
	Client        string
	Reason        string
	FailedAt      time.Time
	ClientChain   string
	ExpectedChain string
}

// ManagedChainUnreadableEntry reports one managed client whose persisted hub
// chain state key could not be read, decoded, or written.
type ManagedChainUnreadableEntry struct {
	Client string
	Err    string
}

// managedChainStore tracks per-client persisted SHA-1 chain heads, unreadable
// chain key diagnostics, and open resync failures for ManagedServer. Safe for
// concurrent use. Lock discipline: mu is NEVER held across storage.Storage I/O.
type managedChainStore struct {
	store storage.Storage

	mu             sync.Mutex
	loaded         map[string]bool
	states         map[string]fleet.ChainState
	unreadable     map[string]string
	resyncFailures map[string]*ManagedResyncFailure
}

// Clone returns an independent copy of f, or nil when f is nil.
func (f *ManagedResyncFailure) Clone() *ManagedResyncFailure {
	// refuse-by-default: guards buildEvalContextAt -- absent resync failure remains nil for converged devices
	if f == nil {
		return nil
	}
	cp := *f
	return &cp
}

// ManagedClientChainKey returns the registered ZeFS store key holding client
// name's hub-side persisted SHA-1 configuration hash chain state
// (meta/managed/client/<name>/chain). Its basename never matches client-*.conf,
// so writing it never triggers ClientNameFromConfigKey.
func ManagedClientChainKey(name string) string {
	return zefs.KeyManagedClientChain.Key(name)
}

// NewStoreBaseVersionReader returns a historical base configuration lookup over
// store that resolves raw bytes for (name, version) from dated version entries
// (file/<stamp>/client-<name>.conf -> sha256:<hex> -> object/<hex>), verifying
// fleet.VersionHash(bytes) == version before returning. Returns nil when store
// is nil.
func NewStoreBaseVersionReader(store storage.Storage) func(name, version string) ([]byte, bool, error) {
	// refuse-by-default: guards NewManagedServer -- an omitted store has no version history so callers fall back to full replay
	if store == nil {
		return nil
	}
	return func(name, version string) ([]byte, bool, error) {
		if name == "" {
			return nil, false, nil
		}
		if version == "" {
			return nil, false, nil
		}
		if !fleet.ValidDeviceKeyName(name) {
			return nil, false, nil
		}
		configKey := ClientConfigKey(name)
		versions, err := store.ListVersions(configKey)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil, false, nil
			}
			return nil, false, fmt.Errorf("list config versions for %s: %w", name, err)
		}
		wantPrefix := managedChainEntryPrefix + version
		for _, vi := range versions {
			val, readErr := store.ReadKey(vi.Path)
			if readErr != nil {
				if errors.Is(readErr, fs.ErrNotExist) {
					continue
				}
				return nil, false, fmt.Errorf("read config version %s for %s: %w", vi.Path, name, readErr)
			}
			if !strings.HasPrefix(string(val), wantPrefix) {
				continue
			}
			data, verErr := store.ReadVersion(configKey, vi.Stamp)
			if verErr != nil {
				if errors.Is(verErr, fs.ErrNotExist) {
					continue
				}
				return nil, false, fmt.Errorf("read config version %s for %s: %w", vi.Stamp, name, verErr)
			}
			if fleet.VersionHash(data) == version {
				return data, true, nil
			}
		}
		return nil, false, nil
	}
}

func newManagedChainStore(store storage.Storage) *managedChainStore {
	return &managedChainStore{
		store:          store,
		loaded:         make(map[string]bool),
		states:         make(map[string]fleet.ChainState),
		unreadable:     make(map[string]string),
		resyncFailures: make(map[string]*ManagedResyncFailure),
	}
}

// loadAll scans existing persisted meta/managed/client/*/chain keys at server
// startup so corrupt keys are immediately visible to runtime doctor checks and
// valid states are cached in memory before clients connect. Storage I/O runs
// outside c.mu.
func (c *managedChainStore) loadAll() {
	if c == nil {
		return
	}
	if c.store == nil {
		return
	}
	keys, err := c.store.ListKeys(zefs.ManagedClientChainPrefix)
	if err != nil {
		managedLogger().Warn("failed to list managed client hash chain keys on startup",
			"code", CodeDoctorManagedChainUnreadable, "prefix", zefs.ManagedClientChainPrefix,
			"error", err, "fix", managedChainCheckFix)
		return
	}
	for _, key := range keys {
		name, ok := clientNameFromChainKey(key)
		if !ok {
			continue
		}
		c.get(name)
	}
}

// get returns the cached or persisted ChainState for name. Storage reads run
// outside c.mu and never write to the store. Corrupt or unreadable keys are
// logged under CodeDoctorManagedChainUnreadable and recorded for doctor rather
// than silently ignored as absent.
func (c *managedChainStore) get(name string) (fleet.ChainState, bool) {
	if c == nil {
		return fleet.ChainState{}, false
	}
	if name == "" {
		return fleet.ChainState{}, false
	}
	c.mu.Lock()
	if st, ok := c.states[name]; ok {
		c.mu.Unlock()
		return st, true
	}
	alreadyLoaded := c.loaded[name]
	c.mu.Unlock()

	if alreadyLoaded {
		return fleet.ChainState{}, false
	}
	if c.store == nil {
		return fleet.ChainState{}, false
	}
	if !fleet.ValidDeviceKeyName(name) {
		return fleet.ChainState{}, false
	}

	key := ManagedClientChainKey(name)
	raw, err := c.store.ReadKey(key)
	if errors.Is(err, fs.ErrNotExist) {
		c.mu.Lock()
		c.loaded[name] = true
		c.mu.Unlock()
		return fleet.ChainState{}, false
	}
	if err != nil {
		c.recordUnreadable(name, key, err)
		return fleet.ChainState{}, false
	}
	st, decErr := decodeManagedChainState(key, raw)
	if decErr != nil {
		c.recordUnreadable(name, key, decErr)
		return fleet.ChainState{}, false
	}

	c.mu.Lock()
	if existing, ok := c.states[name]; ok {
		c.loaded[name] = true
		c.mu.Unlock()
		return existing, true
	}
	c.states[name] = st
	c.loaded[name] = true
	delete(c.unreadable, name)
	c.mu.Unlock()
	return st, true
}

// put persists next for name idempotently: it skips WriteKey whenever the
// stored state is already readable and identical to next. A successful write
// clears any prior unreadable diagnostic for name. Storage I/O runs outside
// c.mu.
func (c *managedChainStore) put(name string, next fleet.ChainState) error {
	if c == nil {
		return nil
	}
	if name == "" {
		return errors.New("managed chain store: client name is required")
	}
	if !fleet.ValidDeviceKeyName(name) {
		return fmt.Errorf("managed chain store: invalid client name %q", name)
	}
	if next.Version == "" {
		return errors.New("managed chain store: version is required")
	}
	if !fleet.ValidChainHash(next.Chain) {
		return errors.New("managed chain store: invalid chain hash")
	}

	c.mu.Lock()
	wasLoaded := c.loaded[name]
	cur, haveCur := c.states[name]
	_, wasUnreadable := c.unreadable[name]
	c.mu.Unlock()

	if !wasLoaded && c.store != nil {
		c.get(name)
		c.mu.Lock()
		cur, haveCur = c.states[name]
		_, wasUnreadable = c.unreadable[name]
		c.mu.Unlock()
	}

	if haveCur && cur == next && !wasUnreadable {
		return nil
	}

	encoded, err := json.Marshal(next)
	if err != nil {
		return fmt.Errorf("encode managed chain state for %s: %w", name, err)
	}
	if len(encoded) > zefs.ManagedClientChainValueMax {
		return fmt.Errorf("managed chain state for %s exceeds %d bytes", name, zefs.ManagedClientChainValueMax)
	}

	if c.store != nil {
		key := ManagedClientChainKey(name)
		if writeErr := c.store.WriteKey(key, encoded); writeErr != nil {
			c.recordUnreadable(name, key, writeErr)
			c.mu.Lock()
			c.states[name] = next
			c.loaded[name] = true
			c.mu.Unlock()
			return writeErr
		}
	}

	c.mu.Lock()
	c.states[name] = next
	c.loaded[name] = true
	delete(c.unreadable, name)
	c.mu.Unlock()
	return nil
}

func (c *managedChainStore) recordUnreadable(name, key string, err error) {
	detail := boundedManagedDetail(err.Error())
	managedLogger().Warn("managed client hash chain key is unreadable or unwritable; run ze show data check",
		"code", CodeDoctorManagedChainUnreadable, "name", name, "key", key,
		"error", detail, "fix", managedChainCheckFix)
	c.mu.Lock()
	c.loaded[name] = true
	c.unreadable[name] = detail
	c.mu.Unlock()
}

// recordResyncFailure records name's most recent failed resync attempt.
func (c *managedChainStore) recordResyncFailure(name string, failure ManagedResyncFailure) {
	if c == nil {
		return
	}
	if name == "" {
		return
	}
	failure.FailedAt = failure.FailedAt.UTC()
	failure.DiffReason = fleet.BoundAckError(failure.DiffReason)
	failure.FullReason = fleet.BoundAckError(failure.FullReason)

	c.mu.Lock()
	c.resyncFailures[name] = &failure
	c.mu.Unlock()
}

// clearResyncFailure removes name's open resync failure and reports whether
// one was present.
func (c *managedChainStore) clearResyncFailure(name string) bool {
	if c == nil {
		return false
	}
	if name == "" {
		return false
	}
	c.mu.Lock()
	_, existed := c.resyncFailures[name]
	delete(c.resyncFailures, name)
	c.mu.Unlock()
	return existed
}

// doctorSnapshots returns deterministic client-sorted copies of open chain
// drift failures and unreadable chain key diagnostics for ManagedRuntimeHealth.
func (c *managedChainStore) doctorSnapshots() ([]ManagedChainDoctorEntry, []ManagedChainUnreadableEntry) {
	if c == nil {
		return nil, nil
	}
	c.mu.Lock()
	var drifted []ManagedChainDoctorEntry
	if len(c.resyncFailures) > 0 {
		drifted = make([]ManagedChainDoctorEntry, 0, len(c.resyncFailures))
		for name, failure := range c.resyncFailures {
			drifted = append(drifted, ManagedChainDoctorEntry{
				Client:        name,
				Reason:        formatResyncFailureReason(failure),
				FailedAt:      failure.FailedAt,
				ClientChain:   failure.ClientChain,
				ExpectedChain: failure.ExpectedChain,
			})
		}
	}
	var unreadable []ManagedChainUnreadableEntry
	if len(c.unreadable) > 0 {
		unreadable = make([]ManagedChainUnreadableEntry, 0, len(c.unreadable))
		for name, errText := range c.unreadable {
			unreadable = append(unreadable, ManagedChainUnreadableEntry{
				Client: name,
				Err:    errText,
			})
		}
	}
	c.mu.Unlock()

	slices.SortFunc(drifted, func(a, b ManagedChainDoctorEntry) int {
		return cmp.Compare(a.Client, b.Client)
	})
	slices.SortFunc(unreadable, func(a, b ManagedChainUnreadableEntry) int {
		return cmp.Compare(a.Client, b.Client)
	})
	return drifted, unreadable
}

func formatResyncFailureReason(f *ManagedResyncFailure) string {
	if f == nil {
		return managedChainDefaultCause
	}
	var tb textbuf.Buffer
	if f.DiffReason != "" && f.FullReason != "" {
		tb.Str("diff: ").Str(f.DiffReason).Str("; full: ").Str(f.FullReason)
		return tb.String()
	}
	if f.FullReason != "" {
		return f.FullReason
	}
	if f.DiffReason != "" {
		return f.DiffReason
	}
	return managedChainDefaultCause
}

func decodeManagedChainState(key string, raw []byte) (fleet.ChainState, error) {
	if len(raw) == 0 {
		return fleet.ChainState{}, fmt.Errorf("managed chain key %s is empty", key)
	}
	if len(raw) > zefs.ManagedClientChainValueMax {
		return fleet.ChainState{}, fmt.Errorf("managed chain key %s size %d exceeds max %d", key, len(raw), zefs.ManagedClientChainValueMax)
	}
	if !utf8.Valid(raw) {
		return fleet.ChainState{}, fmt.Errorf("managed chain key %s is not valid UTF-8", key)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var st fleet.ChainState
	if err := dec.Decode(&st); err != nil {
		return fleet.ChainState{}, fmt.Errorf("decode managed chain key %s: %w", key, err)
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return fleet.ChainState{}, fmt.Errorf("managed chain key %s has trailing data", key)
	}
	if st.Version == "" {
		return fleet.ChainState{}, fmt.Errorf("managed chain key %s is missing version", key)
	}
	if !fleet.ValidChainHash(st.Chain) {
		return fleet.ChainState{}, fmt.Errorf("managed chain key %s has invalid chain hash", key)
	}
	return st, nil
}

func clientNameFromChainKey(key string) (string, bool) {
	if !zefs.KeyManagedClientChain.Matches(key) {
		return "", false
	}
	trimmed := strings.TrimPrefix(key, zefs.ManagedClientChainPrefix)
	name, ok := strings.CutSuffix(trimmed, managedChainKeySuffix)
	if !ok {
		return "", false
	}
	if name == "" {
		return "", false
	}
	if strings.Contains(name, "/") {
		return "", false
	}
	return name, true
}
