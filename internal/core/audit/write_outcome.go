// Design: docs/guide/audit.md -- a failed audit write is reported by ze doctor
// Related: audit.go -- Log.Record, which records the outcome of each write
// Related: store.go -- persistLocked, the write that can fail

package audit

import (
	"errors"
	"sync/atomic"
	"time"
)

// ErrNotPublished is the answer when no audit log is published in this
// process: the local `ze doctor` process, a test, or a daemon before the hub
// opened its log. It is never read as "every audit write landed".
var ErrNotPublished = errors.New("the audit log is not published: no daemon audit log is open in this process")

// WriteFailure is the outcome of the audit log's most recent failed write, as
// a value type a doctor check reads. Every surface writes through Log.Record
// (the only Recorder), so one record answers for CLI, SSH, web, REST, gRPC
// and MCP alike.
type WriteFailure struct {
	// Path is the file the log could not write.
	Path string
	// Err is the write error's text.
	Err string
	// Time is when the most recent write failed, in UTC.
	Time time.Time
	// Failures counts the writes that failed since the last one that landed.
	Failures int
	// Surface and Action name the entry whose write failed most recently.
	Surface string
	Action  string
}

// published is the log the hub opened for the daemon, or nil.
var published atomic.Pointer[Log]

// Publish makes l the log PublishedWriteFailure reads. The hub calls it once
// it has opened the daemon's log; the most recent publication wins. Safe for
// concurrent use.
func (l *Log) Publish() {
	published.Store(l)
}

// PublishedWriteFailure answers the published log's write failure and whether
// one stands, or ErrNotPublished. Safe for concurrent use.
func PublishedWriteFailure() (WriteFailure, bool, error) {
	l := published.Load()
	if l == nil {
		return WriteFailure{}, false, ErrNotPublished
	}
	failure, failed := l.WriteFailure()
	return failure, failed, nil
}

// WriteFailure answers the most recent failed write and true, or false when
// the last write landed (or none failed). A write that lands persists every
// entry the log holds, so it also restores what the failed writes missed,
// which is why it clears the record. Safe for concurrent use.
func (l *Log) WriteFailure() (WriteFailure, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failure.Failures == 0 {
		return WriteFailure{}, false
	}
	return l.failure, true
}

// recordWriteLocked keeps the outcome of one write. Caller holds l.mu.
func (l *Log) recordWriteLocked(entry *Entry, err error) {
	if err == nil {
		l.failure = WriteFailure{}
		return
	}
	l.failure = WriteFailure{
		Path:     l.path,
		Err:      err.Error(),
		Time:     time.Now().UTC(),
		Failures: l.failure.Failures + 1,
		Surface:  entry.Surface,
		Action:   entry.Action,
	}
}
