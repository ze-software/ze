// Design: docs/architecture/zefs-format.md — command history persistence
// Overview: model.go — editor model delegates history to this type

package cli

import (
	"io/fs"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// History key paths sourced from the zefs key registry.
var (
	historyKeyPrefix = zefs.KeyHistory.Prefix()
	historyKeyMax    = zefs.KeyHistoryMax.Pattern
)

const (
	historyMaxDefault = 100
	historyMaxCeiling = 10000 // #7: upper bound to prevent unbounded growth
)

// historyRW is the minimal I/O interface for history persistence.
// Both storage.Storage and *zefs.BlobStore satisfy this.
type historyRW interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
}

// History manages command history entries with browsing and optional persistence.
// It owns the entries list, the Up/Down browsing state, and the save/load logic.
// A nil rw means no persistence (in-memory only, graceful degradation).
type History struct {
	rw      historyRW
	max     int
	prefix  string   // Key prefix for storage (includes username)
	entries []string // Previous commands (oldest first)
	idx     int      // Current browsing position (-1 = not browsing)
	tmp     string   // Saved input when browsing history
}

// NewHistory creates a History backed by the given reader/writer.
// Pass nil for no persistence (in-memory only).
// The username scopes history storage per user (meta/history/<user>/<mode>).
// The max entry count is read from meta/history/max (default 100).
func NewHistory(rw historyRW, username string) *History {
	prefix := historyKeyPrefix
	if username != "" {
		prefix = historyKeyPrefix + username + "/"
	}
	h := &History{
		rw:     rw,
		max:    historyMaxDefault,
		prefix: prefix,
		idx:    -1,
	}
	if rw == nil {
		return h
	}

	data, err := rw.ReadFile(historyKeyMax)
	if err == nil && len(data) > 0 {
		if v, parseErr := strconv.Atoi(strings.TrimSpace(string(data))); parseErr == nil {
			h.max = v
		}
	}

	// Clamp to [1, historyMaxCeiling].
	if h.max < 1 {
		h.max = 1
	}
	if h.max > historyMaxCeiling {
		h.max = historyMaxCeiling
	}

	return h
}

// Max returns the configured maximum history entries.
func (h *History) Max() int {
	return h.max
}

// Entries returns the current history entries.
func (h *History) Entries() []string {
	return h.entries
}

// Append adds a command to history with consecutive dedup.
// Empty commands and commands containing newlines are rejected.
// Returns true if the entry was added (not a duplicate or invalid).
func (h *History) Append(cmd string) bool {
	if cmd == "" {
		return false
	}
	// Newlines would break the line-delimited storage format.
	cmd = strings.ReplaceAll(cmd, "\n", " ")
	if len(h.entries) > 0 && h.entries[len(h.entries)-1] == cmd {
		return false
	}
	h.entries = append(h.entries, cmd)
	h.idx = -1
	h.tmp = ""
	return true
}

// Up recalls the previous command from history.
// On first call, saves currentInput and returns the most recent entry.
// Returns the recalled value and true, or empty string and false if history is empty.
func (h *History) Up(currentInput string) (string, bool) {
	if len(h.entries) == 0 {
		return "", false
	}

	if h.idx == -1 {
		// Start browsing: save current input, go to most recent.
		h.tmp = currentInput
		h.idx = len(h.entries) - 1
	} else if h.idx > 0 {
		h.idx--
	}

	return h.entries[h.idx], true
}

// Down recalls the next command from history, or restores the saved input.
// Returns the value and true if there is something to show, or empty and false
// if not currently browsing.
func (h *History) Down() (string, bool) {
	if h.idx == -1 {
		return "", false
	}

	if h.idx < len(h.entries)-1 {
		h.idx++
		return h.entries[h.idx], true
	}

	// Back to current input.
	saved := h.tmp
	h.idx = -1
	h.tmp = ""
	return saved, true
}

// ResetBrowsing clears the browsing state without changing entries.
// Called when the user types a character, so the next Up starts fresh.
func (h *History) ResetBrowsing() {
	h.idx = -1
	h.tmp = ""
}

// historySnapshot captures the state of a History for mode save/restore.
type historySnapshot struct {
	entries []string
	idx     int
	tmp     string
}

// snapshot returns a deep copy of the current state for mode switching.
func (h *History) snapshot() historySnapshot {
	// Deep copy entries to prevent shared backing array aliasing (#6).
	var copied []string
	if len(h.entries) > 0 {
		copied = make([]string, len(h.entries))
		copy(copied, h.entries)
	}
	return historySnapshot{
		entries: copied,
		idx:     h.idx,
		tmp:     h.tmp,
	}
}

// restore replaces the current state from a saved snapshot.
func (h *History) restore(snap historySnapshot) {
	h.entries = snap.entries
	h.idx = snap.idx
	h.tmp = snap.tmp
	// Guard: can't browse empty history (#1: zero-value idx=0 fix).
	if len(h.entries) == 0 && h.idx != -1 {
		h.idx = -1
	}
}

// Load reads the history for the given mode from the store.
// Returns nil if no history exists or the store is nil.
// Results are trimmed to the configured max.
func (h *History) Load(mode string) []string {
	if h.rw == nil {
		return nil
	}

	data, err := h.rw.ReadFile(h.prefix + mode)
	if err != nil || len(data) == 0 {
		return nil
	}

	// Split and filter empty lines.
	lines := strings.Split(string(data), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" {
			result = append(result, line)
		}
	}
	if len(result) == 0 {
		return nil
	}

	// Trim to max on load (#5: prevent unbounded memory from crafted blob).
	if len(result) > h.max {
		result = result[len(result)-h.max:]
	}

	return result
}

// Save writes the history for the given mode to the store.
// Trims to the configured max, keeping the newest entries.
// No-op if the store is nil.
func (h *History) Save(mode string) {
	if h.rw == nil {
		return
	}

	entries := h.entries
	// Trim to max, keeping newest.
	if len(entries) > h.max {
		entries = entries[len(entries)-h.max:]
		h.entries = entries
	}

	data := []byte(strings.Join(entries, "\n"))
	_ = h.rw.WriteFile(h.prefix+mode, data, 0) //nolint:errcheck // best-effort persist
}
