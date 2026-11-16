// Design: docs/architecture/zefs-format.md — command history persistence
// Overview: model.go — editor model delegates history to this type

package cli

import (
	"errors"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/configtext"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/redact"
	"github.com/ze-software/ze/internal/core/syncutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
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
// Entries and browsing belong to the serial UI owner; they are not concurrent.
// Save starts one worker lazily and transfers detached whole snapshots to it.
// The owner MUST call Close after its last Save, before releasing the borrowed
// store. Close drains and joins independently of the UI and never closes rw.
// Close is safe concurrently with Save and other Close calls; copies of Model
// share this *History. A nil rw remains memory-only and starts no worker.
// A History MUST NOT be copied; share its pointer instead.
type History struct {
	rw         historyRW
	max        int
	prefix     string   // Key prefix for storage (includes username)
	entries    []string // Previous commands (oldest first)
	idx        int      // Current browsing position (-1 = not browsing)
	tmp        string   // Saved input when browsing history
	mode       EditorMode
	scopes     map[EditorMode][]string
	loaded     map[EditorMode]bool
	loadErrors map[EditorMode]error
	mask       func(string, []string) string

	mu          sync.Mutex // Protects the publication lifetime, not UI entries.
	writer      *syncutil.Coalescer[string, []string]
	closed      bool
	writeErrors map[string]error // Latest completed publication outcome, bounded to two modes.
}

// NewHistory creates a History backed by the given reader/writer.
// Pass nil for no persistence (in-memory only).
// The username scopes history storage per user (meta/history/<user>/<mode>).
// The max entry count is read from meta/history/max (default 100).
// Construction starts no worker. The caller MUST call Close after its last Save.
func NewHistory(rw historyRW, username string) *History {
	return newHistory(rw, username, config.DisplayCommandAtPath)
}

// NewHistoryWithMask binds the owner's display producer before any history can
// be loaded, recorded or published. The mask MUST be nonnil and MUST resolve the
// owning editor's actual schema. The caller MUST close the returned History
// before releasing its borrowed store. Construction starts no worker.
func NewHistoryWithMask(rw historyRW, username string, mask func(string, []string) string) (*History, error) {
	if mask == nil {
		return nil, errors.New("command history requires a display mask")
	}
	return newHistory(rw, username, mask), nil
}

func newHistory(rw historyRW, username string, mask func(string, []string) string) *History {
	prefix := historyKeyPrefix
	if username != "" {
		var tb textbuf.Buffer
		prefix = tb.Str(historyKeyPrefix).Str(username).Byte('/').String()
	}
	h := &History{
		rw:         rw,
		max:        historyMaxDefault,
		prefix:     prefix,
		idx:        -1,
		scopes:     make(map[EditorMode][]string),
		loaded:     make(map[EditorMode]bool),
		loadErrors: make(map[EditorMode]error),
		mask:       mask,
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

// Entries returns a detached copy of the current history entries.
func (h *History) Entries() []string {
	return slices.Clone(h.entries)
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

// resetBrowsing clears the browsing state without changing entries.
// Called when the user types a character, so the next Up starts fresh.
func (h *History) resetBrowsing() {
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

// Load reads stored records for one mode. Missing history and a nil store are
// empty successful reads; other storage failures MUST reach the owner. Recall
// consumers MUST use Snapshot, which also masks records written by older code.
func (h *History) Load(mode string) ([]string, error) {
	if mode != modeNameConfig {
		if mode != modeNameOperational {
			return nil, opererr.New("The command history mode is invalid.", "History belongs to configuration or operational mode.", "Use configure or exit to select a CLI mode")
		}
	}
	if h.rw == nil {
		return nil, nil
	}

	data, err := h.rw.ReadFile(h.prefix + mode)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		reportHistoryReadFailure(mode)
		return nil, opererr.Wrap(err, "Your "+mode+" command history is unavailable.", "Previously saved commands could not be read; this mode's history will not be overwritten.", "Reconnect after the configuration store is available to restore command recall")
	}

	// Split and filter empty lines.
	result := make([]string, 0, h.max)
	for line := range strings.SplitSeq(string(data), "\n") {
		if line != "" {
			result = append(result, line)
		}
	}
	if len(result) == 0 {
		return nil, nil
	}

	// Trim to max on load (#5: prevent unbounded memory from crafted blob).
	if len(result) > h.max {
		result = result[len(result)-h.max:]
	}

	return result, nil
}

// Snapshot returns bounded, masked recall for mode without exposing its backing
// slice. The serial owner MUST hold its session lock. Failed reads remain errors
// for this owner; a new History retries storage when the session is reopened.
func (h *History) Snapshot(mode EditorMode) ([]string, error) {
	switch mode {
	case ModeConfig, ModeOperational:
	default:
		return nil, opererr.New("The command history mode is invalid.", "History belongs to configuration or operational mode.", "Use configure or exit to select a CLI mode")
	}
	if err := h.loadErrors[mode]; err != nil {
		return nil, err
	}
	if h.loaded[mode] {
		entries := h.scopes[mode]
		if h.mode == mode {
			entries = h.entries
		}
		if len(entries) > h.max {
			entries = entries[len(entries)-h.max:]
		}
		return slices.Clone(entries), nil
	}
	stored, err := h.Load(mode.String())
	if err != nil {
		h.loadErrors[mode] = err
		return nil, err
	}
	if h.rw == nil {
		if h.mode == mode {
			stored = h.entries
		}
	}
	entries := make([]string, 0, len(stored))
	for _, line := range stored {
		line = h.displayCommand(mode, nil, line)
		if len(entries) > 0 {
			if entries[len(entries)-1] == line {
				continue
			}
		}
		entries = append(entries, line)
	}
	if len(entries) > h.max {
		entries = entries[len(entries)-h.max:]
	}
	h.scopes[mode] = entries
	h.loaded[mode] = true
	if h.mode == mode {
		h.entries = entries
	}
	return slices.Clone(entries), nil
}

// Record appends one command in its submitted mode, using the shared display
// mask before either recall or publication. Content supplied separately to load
// MUST NOT be passed as command. The serial owner MUST hold its session lock,
// stop recording before Close, and close before releasing the borrowed store.
func (h *History) Record(mode EditorMode, path []string, command string) ([]string, error) {
	h.mu.Lock()
	closed := h.closed
	h.mu.Unlock()
	if closed {
		return nil, opererr.New("Command history is closed.", "This session has ended, so no command can be recorded.", "Reconnect to open a new CLI session")
	}
	entries, err := h.Snapshot(mode)
	if err != nil {
		return nil, err
	}
	if h.mode != mode {
		h.scopes[h.mode] = slices.Clone(h.entries)
		h.mode = mode
	}
	h.entries = entries
	command, err = absoluteHistoryCommand(mode, path, command)
	if err != nil {
		return nil, err
	}
	if h.Append(h.displayCommand(mode, path, command)) {
		if len(h.entries) > h.max {
			h.entries = h.entries[len(h.entries)-h.max:]
		}
		h.scopes[mode] = h.entries
		h.Save(mode)
	}
	return slices.Clone(h.entries), nil
}

func absoluteHistoryCommand(mode EditorMode, path []string, command string) (string, error) {
	if mode != ModeConfig {
		return command, nil
	}
	if len(path) == 0 {
		return command, nil
	}
	words, err := ParseCommandWords(command)
	if err != nil {
		return "", err
	}
	tokens := words.Tokens()
	if len(tokens) == 0 {
		return command, nil
	}
	if tokens[0] != cmdSet {
		return command, nil
	}
	var absolute textbuf.Buffer
	absolute.Str(cmdTop).Byte(' ').Str(cmdSet)
	for _, token := range path {
		absolute.Byte(' ').Str(configtext.QuoteCommandWord(token))
	}
	absolute.Byte(' ').Str(words.Tail().String())
	return absolute.String(), nil
}

func (h *History) displayCommand(mode EditorMode, path []string, command string) string {
	if mode == ModeOperational {
		if IsSessionFormatCommand(command) {
			return redact.Command(command)
		}
	}
	return h.mask(command, path)
}

// Save queues the newest whole history for mode without waiting for storage.
// It trims persistent entries to max before detaching their slice; strings are
// immutable. EditorMode.String resolves to exactly the two existing storage mode
// names, bounding work to two waiting snapshots plus one active publication.
// Nil-store Save returns before trimming, preserving memory-only behavior.
// The owner MUST stop calling Save before Close and release the store afterward.
func (h *History) Save(mode EditorMode) {
	if h.loadErrors[mode] != nil {
		return // Snapshot exposes this failure; unread history must not be replaced.
	}
	if h.rw == nil {
		return
	}

	entries := h.entries
	// Trim to max, keeping newest.
	if len(entries) > h.max {
		entries = entries[len(entries)-h.max:]
		h.entries = entries
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	if h.writer == nil {
		h.writer = syncutil.NewCoalescer(1,
			func(_, next []string) []string { return next },
			h.publish,
		)
	}
	h.writer.Enqueue(mode.String(), slices.Clone(entries))
}

// publish encodes only snapshots the worker takes, not superseded admissions.
// Its detached entries MUST NOT be mutated after Save transfers them.
func (h *History) publish(mode string, entries []string) {
	buf := textbuf.Get()
	defer buf.Release()
	err := h.rw.WriteFile(h.prefix+mode, buf.Join(entries, "\n").Bytes(), 0)
	if err != nil {
		reportHistoryWriteFailure(mode)
	}
	h.mu.Lock()
	if h.writeErrors == nil {
		h.writeErrors = make(map[string]error, 2)
	}
	h.writeErrors[mode] = err
	h.mu.Unlock()
}

// LastWriteError reports known publication failures. Before Close has joined
// the worker, nil means only that no completed write is known to have failed.
// The owner MUST call Close before treating this as its final drain status.
// Safe for concurrent use; a later successful write clears that mode's failure.
func (h *History) LastWriteError() error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	var failures []error
	for _, mode := range []string{modeNameConfig, modeNameOperational} {
		if err := h.writeErrors[mode]; err != nil {
			failures = append(failures, opererr.Wrap(err,
				"Your "+mode+" command history could not be saved.",
				"Command recall may be lost when this session ends.",
				"Check the disk holding the configuration store, then reconnect"))
		}
	}
	return errors.Join(failures...)
}

// Close drains every accepted publication and joins the worker, if one started.
// The owner MUST call Close before closing the borrowed store. It is idempotent
// and needs neither command-result application nor a live dispatch context.
// Nil and an unused History have nothing to drain.
// The owner MUST inspect LastWriteError after Close to judge publication.
func (h *History) Close() {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.closed = true
	writer := h.writer
	h.mu.Unlock()
	if writer != nil {
		writer.Stop()
	}
}
