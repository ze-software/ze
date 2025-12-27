package functional

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// MaxHistorySize is the number of past runs to keep for averaging.
	MaxHistorySize = 5
	// DefaultCacheDir is the cache directory relative to home.
	DefaultCacheDir = ".cache/zebgp"
	// DefaultCacheFile is the timing cache filename.
	DefaultCacheFile = "test_times.json"
)

// Timing holds timing data for a single test.
type Timing struct {
	Expected time.Duration   `json:"expected"` // Expected time based on history
	Actual   time.Duration   `json:"actual"`   // Most recent actual time
	History  []time.Duration `json:"history"`  // Last N run times
}

// UpdateExpected recalculates Expected from History.
func (t *Timing) UpdateExpected() {
	if len(t.History) == 0 {
		t.Expected = 0
		return
	}

	var sum time.Duration
	for _, d := range t.History {
		sum += d
	}
	t.Expected = sum / time.Duration(len(t.History))
}

// AddToHistory adds a duration to history, keeping MaxHistorySize entries.
func (t *Timing) AddToHistory(d time.Duration) {
	t.History = append(t.History, d)
	if len(t.History) > MaxHistorySize {
		t.History = t.History[len(t.History)-MaxHistorySize:]
	}
	t.Actual = d
	t.UpdateExpected()
}

// TimingCache stores timing data for all tests.
type TimingCache struct {
	path   string
	times  map[string]*Timing
	mu     sync.RWMutex
	loaded bool
}

// NewTimingCache creates a cache with default path.
func NewTimingCache() *TimingCache {
	return &TimingCache{
		path:  defaultCachePath(),
		times: make(map[string]*Timing),
	}
}

// NewTimingCacheWithPath creates a cache with specified path.
func NewTimingCacheWithPath(path string) *TimingCache {
	return &TimingCache{
		path:  path,
		times: make(map[string]*Timing),
	}
}

// defaultCachePath returns the default cache file path.
func defaultCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), DefaultCacheFile)
	}
	return filepath.Join(home, DefaultCacheDir, DefaultCacheFile)
}

// Load reads timing data from the cache file.
func (tc *TimingCache) Load() error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	data, err := os.ReadFile(tc.path)
	if err != nil {
		if os.IsNotExist(err) {
			tc.loaded = true
			return nil // No cache file is OK
		}
		return err
	}

	if err := json.Unmarshal(data, &tc.times); err != nil {
		return err
	}

	tc.loaded = true
	return nil
}

// Save writes timing data to the cache file.
func (tc *TimingCache) Save() error {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	// Ensure directory exists
	dir := filepath.Dir(tc.path)
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:gosec // Cache dir needs to be accessible
		return err
	}

	data, err := json.MarshalIndent(tc.times, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(tc.path, data, 0o600)
}

// Get returns timing data for a test, or nil if not found.
func (tc *TimingCache) Get(name string) *Timing {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.times[name]
}

// Update records a new timing for a test.
func (tc *TimingCache) Update(name string, duration time.Duration) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	t := tc.times[name]
	if t == nil {
		t = &Timing{}
		tc.times[name] = t
	}
	t.AddToHistory(duration)
}

// ETA returns the estimated total time for the given test names.
func (tc *TimingCache) ETA(names []string) time.Duration {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	var total time.Duration
	for _, name := range names {
		if t := tc.times[name]; t != nil {
			total += t.Expected
		}
	}
	return total
}

// Count returns the number of cached timing entries.
func (tc *TimingCache) Count() int {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return len(tc.times)
}
