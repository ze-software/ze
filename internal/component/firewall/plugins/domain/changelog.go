// Design: docs/architecture/firewall/firewall-domain-group.md -- DNS change log
//
// The log answers one operator question: what did this name point at last
// week, and when did it move. That is a question about a name's history, so
// only an actual CHANGE is recorded. A refresh that confirmed the same
// addresses writes nothing (AC-2), which is what keeps a name refreshed every
// 60 seconds from filling the disk with a record of nothing happening.
//
// It is JSON-lines on the filesystem rather than a zefs blob, following the
// exception docs/architecture/zefs-format.md already grants internal/core/audit
// for the same reason: zefs has NO append. BlobStore.WriteFile replaces a whole
// value, and a value that outgrows its capacity headroom forces a rewrite of
// the entire store through a temp file and a rename. A log that grows would pay
// that on every entry.
//
// It is NOT the operator audit log (internal/core/audit,
// <config>.audit.jsonl). That log records operator ACTIONS and is bounded by
// how often a person acts; a name whose addresses rotate is neither, and
// sharing the file would evict commit history to record DNS churn.

package domain

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/paths"
)

const (
	// changeLogMaxEntries is how many records the log keeps. Past it the oldest
	// are dropped, because the recent history is what an operator asks about
	// and an unbounded log on an appliance fills a partition the rest of Ze
	// needs (R-3).
	changeLogMaxEntries = 10000
	// changeLogFileName sits beside the operator audit log that openAuditLog
	// (cmd/ze/hub/audit.go) writes as <config-dir>/<name>.audit.jsonl, and is
	// deliberately a different file. See the package comment above.
	changeLogFileName = "firewall-domain-group.dns.jsonl"
)

// changeRecord is one line of the log: what a name resolved to before, what it
// resolves to now, and when it moved.
//
// Old and New are both written even when one is empty, so a reader never has
// to infer a direction from an absent field: an empty New with a non-empty Old
// is a name that went away, and the reverse is one that arrived.
type changeRecord struct {
	Time   time.Time `json:"time"`
	Group  string    `json:"group"`
	Name   string    `json:"name"`
	Family string    `json:"family"`
	Status string    `json:"status"`
	Old    []string  `json:"old"`
	New    []string  `json:"new"`
}

// changeLog appends records to a bounded JSON-lines file.
// Safe for concurrent use.
type changeLog struct {
	path string

	mu      sync.Mutex
	entries int
}

// newChangeLog builds a log over path. It performs no I/O; open counts what is
// already there so the bound is enforced across a restart.
func newChangeLog(path string) *changeLog {
	return &changeLog{path: path}
}

// open counts the records already on disk, so the entry bound is enforced over
// the file's whole life rather than over this process's share of it. A missing
// file is not an error: the first record creates it.
func (l *changeLog) open() error {
	if l.path == "" {
		return errors.New("firewall domain-group: no config directory, the change log cannot be written")
	}

	f, err := os.Open(l.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			l.mu.Lock()
			l.entries = 0
			l.mu.Unlock()
			return nil
		}
		return fmt.Errorf("firewall domain-group: open change log: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only handle, nothing to flush

	count := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), maxChangeLogLineBytes)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("firewall domain-group: read change log: %w", err)
	}

	l.mu.Lock()
	l.entries = count
	l.mu.Unlock()
	return nil
}

// maxChangeLogLineBytes bounds one line. A record holds at most 64 addresses
// per family plus its fields, so 64 KiB is far above any line Ze writes; the
// bound exists so a corrupted file cannot make the scanner allocate without
// limit.
const maxChangeLogLineBytes = 64 * 1024

// append writes one record and evicts the oldest when the file is full.
//
// The eviction rewrites the file, which is why only a real change is recorded:
// at one record per change the rewrite happens once every changeLogMaxEntries
// appends, and at one record per refresh it would happen constantly.
func (l *changeLog) append(record changeRecord) error {
	if l.path == "" {
		return errors.New("firewall domain-group: no config directory, the change log cannot be written")
	}
	if record.Old == nil {
		record.Old = []string{}
	}
	if record.New == nil {
		record.New = []string{}
	}
	line, err := json.Marshal(&record)
	if err != nil {
		return fmt.Errorf("firewall domain-group: encode change record: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.entries >= changeLogMaxEntries {
		if err := l.evictOldestLocked(); err != nil {
			return err
		}
	}

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("firewall domain-group: open change log: %w", err)
	}
	defer f.Close() //nolint:errcheck // the write below is checked and the data is flushed by Sync

	line = append(line, '\n')
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("firewall domain-group: write change record: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("firewall domain-group: flush change record: %w", err)
	}
	l.entries++
	return nil
}

// evictOldestLocked makes room for one more record. The caller MUST hold l.mu.
func (l *changeLog) evictOldestLocked() error {
	return l.keepNewest(changeLogMaxEntries - 1)
}

// keepNewest rewrites the file keeping at most the last `keep` records. The
// caller MUST hold l.mu.
//
// It goes through a temp file and a rename so a crash mid-rewrite leaves the
// previous log whole rather than a truncated one: the log exists to be read
// after something went wrong, so losing it to the same event defeats it.
//
// The entry count is recomputed from what was written, never carried forward.
// A count that drifted from the file would let the log grow past its bound,
// which is the one thing the bound exists to stop.
func (l *changeLog) keepNewest(keep int) error {
	f, err := os.Open(l.path) //nolint:gosec // l.path is built by changeLogPath from the config directory, never from a request
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			l.entries = 0
			return nil
		}
		return fmt.Errorf("firewall domain-group: read change log: %w", err)
	}

	lines := make([]string, 0, changeLogMaxEntries)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), maxChangeLogLineBytes)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		lines = append(lines, scanner.Text())
	}
	scanErr := scanner.Err()
	closeErr := f.Close()
	if scanErr != nil {
		return fmt.Errorf("firewall domain-group: read change log: %w", scanErr)
	}
	if closeErr != nil {
		return fmt.Errorf("firewall domain-group: close change log: %w", closeErr)
	}

	if len(lines) > keep {
		lines = lines[len(lines)-keep:]
	}

	tmpPath := l.path + ".tmp"
	tmp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //nolint:gosec // tmpPath derives from changeLogPath, built from the config directory, never from a request
	if err != nil {
		return fmt.Errorf("firewall domain-group: open change log temp: %w", err)
	}
	w := bufio.NewWriter(tmp)
	for _, line := range lines {
		if _, err := w.WriteString(line); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("firewall domain-group: rewrite change log: %w", err)
		}
		if err := w.WriteByte('\n'); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("firewall domain-group: rewrite change log: %w", err)
		}
	}
	if err := w.Flush(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("firewall domain-group: flush change log: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("firewall domain-group: sync change log: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("firewall domain-group: close change log temp: %w", err)
	}
	if err := os.Rename(tmpPath, l.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("firewall domain-group: replace change log: %w", err)
	}

	l.entries = len(lines)
	return nil
}

// records reads the log back, newest last. limit caps how many are returned,
// counting from the end, so a caller asking for the recent history does not
// load the whole file.
func (l *changeLog) records(limit int) ([]changeRecord, error) {
	if l.path == "" {
		return nil, nil
	}
	f, err := os.Open(l.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("firewall domain-group: read change log: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only handle, nothing to flush

	var out []changeRecord
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), maxChangeLogLineBytes)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var record changeRecord
		if json.Unmarshal(scanner.Bytes(), &record) != nil {
			continue // a truncated tail from an interrupted write is skipped, not fatal
		}
		out = append(out, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("firewall domain-group: read change log: %w", err)
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

// changeLogPath is where the log lives: beside the operator audit log, in the
// config directory, and not inside the zefs store.
func changeLogPath() string {
	configDir := paths.DefaultConfigDir()
	if configDir == "" {
		return ""
	}
	return filepath.Join(configDir, changeLogFileName)
}
