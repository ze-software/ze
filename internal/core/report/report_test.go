package report

import (
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// VALIDATES: AC-1, single RaiseWarning creates one entry with matching fields and Raised == Updated.
// PREVENTS: regression where the bus loses the first raise or sets timestamps inconsistently.
func TestRaiseWarningNew(t *testing.T) {
	reset()
	RaiseWarning("bgp", "prefix-threshold", "10.0.0.1", "ipv4/unicast over limit", map[string]any{"family": "ipv4/unicast"})
	got := Warnings()
	if len(got) != 1 {
		t.Fatalf("want 1 warning, got %d", len(got))
	}
	w := got[0]
	if w.Source != "bgp" || w.Code != "prefix-threshold" || w.Subject != "10.0.0.1" {
		t.Errorf("wrong key: source=%s code=%s subject=%s", w.Source, w.Code, w.Subject)
	}
	if w.Severity != SeverityWarning {
		t.Errorf("severity=%d want %d", w.Severity, SeverityWarning)
	}
	if w.Message != "ipv4/unicast over limit" {
		t.Errorf("message=%q", w.Message)
	}
	if w.Detail["family"] != "ipv4/unicast" {
		t.Errorf("detail.family=%v", w.Detail["family"])
	}
	if !w.Raised.Equal(w.Updated) {
		t.Errorf("Raised=%v Updated=%v want equal on first raise", w.Raised, w.Updated)
	}
}

// VALIDATES: AC-2, repeat RaiseWarning with same key updates Updated, latest message wins, Raised stays.
// PREVENTS: duplicate entries from re-raising the same condition.
func TestRaiseWarningDedup(t *testing.T) {
	reset()
	RaiseWarning("bgp", "prefix-threshold", "10.0.0.1", "first message", nil)
	first := Warnings()[0]
	time.Sleep(2 * time.Millisecond)
	RaiseWarning("bgp", "prefix-threshold", "10.0.0.1", "second message", nil)

	got := Warnings()
	if len(got) != 1 {
		t.Fatalf("want 1 entry after dedup, got %d", len(got))
	}
	w := got[0]
	if w.Message != "second message" {
		t.Errorf("latest message should win, got %q", w.Message)
	}
	if !w.Raised.Equal(first.Raised) {
		t.Errorf("Raised changed: want %v got %v", first.Raised, w.Raised)
	}
	if !w.Updated.After(first.Updated) {
		t.Errorf("Updated did not advance: first=%v second=%v", first.Updated, w.Updated)
	}
}

// VALIDATES: AC-2 dedup boundary, distinct subjects produce distinct entries even with same source+code.
// PREVENTS: per-family or per-peer collisions when callers use composite subjects.
func TestRaiseWarningDistinctSubjects(t *testing.T) {
	reset()
	RaiseWarning("bgp", "prefix-threshold", "10.0.0.1/ipv4/unicast", "v4 over", nil)
	RaiseWarning("bgp", "prefix-threshold", "10.0.0.1/ipv6/unicast", "v6 over", nil)
	got := Warnings()
	if len(got) != 2 {
		t.Fatalf("want 2 entries for distinct subjects, got %d", len(got))
	}
}

// VALIDATES: AC-3, ClearWarning removes the matching active entry.
// PREVENTS: cleared warnings continuing to appear in operator output.
func TestClearWarning(t *testing.T) {
	reset()
	RaiseWarning("bgp", "prefix-threshold", "10.0.0.1", "msg", nil)
	if len(Warnings()) != 1 {
		t.Fatalf("setup: want 1 entry")
	}
	ClearWarning("bgp", "prefix-threshold", "10.0.0.1")
	if got := Warnings(); len(got) != 0 {
		t.Errorf("want 0 entries after clear, got %d", len(got))
	}
}

// VALIDATES: AC-4, ClearSource removes all entries for one source while leaving others.
// PREVENTS: cross-source contamination in clear-on-shutdown paths.
func TestClearSource(t *testing.T) {
	reset()
	RaiseWarning("bgp", "prefix-threshold", "10.0.0.1", "a", nil)
	RaiseWarning("bgp", "prefix-stale", "10.0.0.2", "b", nil)
	RaiseWarning("config", "noisy-leaf", "/foo", "c", nil)
	ClearSource("bgp")
	got := Warnings()
	if len(got) != 1 {
		t.Fatalf("want 1 remaining entry after ClearSource(bgp), got %d", len(got))
	}
	if got[0].Source != "config" {
		t.Errorf("survivor has wrong source: %s", got[0].Source)
	}
}

// VALIDATES: AC-5, error ring buffer evicts oldest at cap.
// PREVENTS: unbounded growth of error history.
func TestErrorsRingBufferEviction(t *testing.T) {
	resetWithCaps(8, 4)
	for i := range 5 {
		RaiseError("bgp", "notification-sent", "10.0.0."+strconv.Itoa(i), "msg "+strconv.Itoa(i), nil)
	}
	got := Errors(0)
	if len(got) != 4 {
		t.Fatalf("want 4 entries (cap), got %d", len(got))
	}
	if got[0].Subject != "10.0.0.4" {
		t.Errorf("most recent should be index 0, got subject %q", got[0].Subject)
	}
	if got[3].Subject != "10.0.0.1" {
		t.Errorf("oldest survivor should be index 3 (10.0.0.1), got %q", got[3].Subject)
	}
	for _, e := range got {
		if e.Subject == "10.0.0.0" {
			t.Errorf("oldest entry should have been evicted, but found it")
		}
	}
}

// VALIDATES: errors ring with limit returns only N most recent.
func TestErrorsRingBufferLimit(t *testing.T) {
	resetWithCaps(8, 16)
	for i := range 10 {
		RaiseError("bgp", "notification-sent", "10.0.0."+strconv.Itoa(i), "m", nil)
	}
	got := Errors(3)
	if len(got) != 3 {
		t.Fatalf("want 3 entries with limit=3, got %d", len(got))
	}
	if got[0].Subject != "10.0.0.9" {
		t.Errorf("most recent first, got %q", got[0].Subject)
	}
}

// VALIDATES: AC-15, warning map evicts oldest by Updated when over cap.
// PREVENTS: unbounded warning state growth.
func TestWarningsCapEviction(t *testing.T) {
	resetWithCaps(4, 4)
	for i := range 5 {
		RaiseWarning("test", "code", "subj-"+strconv.Itoa(i), "msg", nil)
		// 5ms sleep ensures Updated timestamps differ even on
		// low-resolution clocks (some Windows configurations, virtualised CI).
		time.Sleep(5 * time.Millisecond)
	}
	got := Warnings()
	if len(got) != 4 {
		t.Fatalf("want 4 entries (cap), got %d", len(got))
	}
	for _, w := range got {
		if w.Subject == "subj-0" {
			t.Errorf("oldest entry subj-0 should have been evicted")
		}
	}
}

// VALIDATES: AC-14, concurrent raise/clear/snapshot is race-free.
// PREVENTS: data races in production. MUST run with -race.
func TestRaiseClearConcurrent(t *testing.T) {
	reset()
	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Go(func() {
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				RaiseWarning("bgp", "prefix-threshold", "peer-"+strconv.Itoa(i%10), "m", nil)
				i++
			}
		}
	})

	wg.Go(func() {
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				ClearWarning("bgp", "prefix-threshold", "peer-"+strconv.Itoa(i%10))
				i++
			}
		}
	})

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				_ = Warnings()
			}
		}
	})

	wg.Go(func() {
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				RaiseError("bgp", "notification-sent", "peer-"+strconv.Itoa(i%10), "m", nil)
				_ = Errors(0)
				i++
			}
		}
	})

	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// VALIDATES: snapshot mutation does not affect bus state.
// PREVENTS: callers accidentally corrupting the bus by mutating returned slice or detail maps.
func TestSnapshotIsCopy(t *testing.T) {
	reset()
	RaiseWarning("bgp", "prefix-threshold", "10.0.0.1", "msg", map[string]any{"family": "ipv4/unicast"})
	snap := Warnings()
	if len(snap) != 1 {
		t.Fatalf("setup: want 1 entry")
	}
	snap[0].Message = "tampered"
	snap[0].Detail["family"] = "tampered"

	again := Warnings()
	if again[0].Message != "msg" {
		t.Errorf("bus message tampered via snapshot: got %q", again[0].Message)
	}
	if again[0].Detail["family"] != "ipv4/unicast" {
		t.Errorf("bus detail tampered via snapshot: got %v", again[0].Detail["family"])
	}
}

// VALIDATES: AC-16, RaiseError appends to ring with no dedup, even for identical fields.
// PREVENTS: losing repeated error events (e.g., a peer flapping).
func TestRaiseErrorAppendsNoDedup(t *testing.T) {
	resetWithCaps(8, 8)
	RaiseError("bgp", "notification-received", "10.0.0.1", "first", nil)
	RaiseError("bgp", "notification-received", "10.0.0.1", "second", nil)
	got := Errors(0)
	if len(got) != 2 {
		t.Fatalf("want 2 entries (no dedup), got %d", len(got))
	}
	if got[0].Message != "second" || got[1].Message != "first" {
		t.Errorf("ordering: got[0]=%q got[1]=%q want second/first", got[0].Message, got[1].Message)
	}
}

// VALIDATES: AC-17, ClearWarning for a missing key is a no-op (no panic, no log noise).
func TestClearWarningMissingKey(t *testing.T) {
	reset()
	ClearWarning("bgp", "nonexistent", "10.0.0.1") // must not panic
	if got := Warnings(); len(got) != 0 {
		t.Errorf("clear of missing key created entry: %v", got)
	}
}

// VALIDATES: AC-18, Raise* with empty Source/Code/Subject is rejected silently.
// PREVENTS: malformed entries leaking into operator output.
func TestRaiseRejectsEmptyFields(t *testing.T) {
	reset()
	RaiseWarning("", "code", "subj", "m", nil)
	RaiseWarning("src", "", "subj", "m", nil)
	RaiseWarning("src", "code", "", "m", nil)
	RaiseError("", "code", "subj", "m", nil)
	RaiseError("src", "", "subj", "m", nil)
	RaiseError("src", "code", "", "m", nil)

	if got := Warnings(); len(got) != 0 {
		t.Errorf("warnings should be empty, got %d entries", len(got))
	}
	if got := Errors(0); len(got) != 0 {
		t.Errorf("errors should be empty, got %d entries", len(got))
	}
}

// VALIDATES: empty snapshot returns empty slice, not nil (for consistent JSON encoding).
func TestEmptySnapshotsAreNonNil(t *testing.T) {
	reset()
	w := Warnings()
	if w == nil {
		t.Error("Warnings() returned nil; expected non-nil empty slice")
	}
	e := Errors(0)
	if e == nil {
		t.Error("Errors(0) returned nil; expected non-nil empty slice")
	}
}

// VALIDATES: snapshot of an entry raised with nil detail returns Detail == nil
// without panicking when callers index it conditionally.
// PREVENTS: regression where copyDetail returns an empty map for nil input,
// causing nil checks in operator UI / JSON encoding to fail.
func TestSnapshotNilDetail(t *testing.T) {
	reset()
	RaiseWarning("bgp", "prefix-stale", "10.0.0.1", "msg", nil)
	got := Warnings()
	if len(got) != 1 {
		t.Fatalf("want 1 entry, got %d", len(got))
	}
	if got[0].Detail != nil {
		t.Errorf("Detail should be nil for nil-detail raise, got %v", got[0].Detail)
	}
	// Reading from nil map is OK in Go; assigning panics. Verify read does not panic.
	_ = got[0].Detail["nonexistent"]

	// Same for errors.
	RaiseError("bgp", "notification-sent", "10.0.0.1", "msg", nil)
	gotErrs := Errors(0)
	if len(gotErrs) != 1 {
		t.Fatalf("want 1 error, got %d", len(gotErrs))
	}
	if gotErrs[0].Detail != nil {
		t.Errorf("error Detail should be nil for nil-detail raise, got %v", gotErrs[0].Detail)
	}
}

// VALIDATES: ring buffer of cap 1 keeps only the most recent error.
// PREVENTS: off-by-one in the ring buffer index arithmetic at the smallest cap.
func TestErrorRingCap1(t *testing.T) {
	resetWithCaps(8, 1)
	RaiseError("bgp", "notification-sent", "10.0.0.1", "first", nil)
	RaiseError("bgp", "notification-sent", "10.0.0.2", "second", nil)
	got := Errors(0)
	if len(got) != 1 {
		t.Fatalf("want 1 entry (cap=1), got %d", len(got))
	}
	if got[0].Subject != "10.0.0.2" {
		t.Errorf("want most recent (10.0.0.2), got %q", got[0].Subject)
	}
}

// VALIDATES: warning map of cap 1 evicts on every distinct second key.
// PREVENTS: off-by-one in the cap-overflow check for the smallest meaningful cap.
func TestWarningCap1(t *testing.T) {
	resetWithCaps(1, 8)
	RaiseWarning("bgp", "prefix-threshold", "10.0.0.1", "first", nil)
	if got := Warnings(); len(got) != 1 || got[0].Subject != "10.0.0.1" {
		t.Fatalf("first raise: got %d entries (%v)", len(got), got)
	}
	time.Sleep(5 * time.Millisecond)
	RaiseWarning("bgp", "prefix-threshold", "10.0.0.2", "second", nil)
	got := Warnings()
	if len(got) != 1 {
		t.Fatalf("want 1 entry (cap=1), got %d", len(got))
	}
	if got[0].Subject != "10.0.0.2" {
		t.Errorf("want second entry (10.0.0.2) after eviction, got %q", got[0].Subject)
	}
}

// VALIDATES: Errors with negative limit treats it as "all" (matches godoc).
// PREVENTS: silent surprise where Errors(-1) returns 0 entries instead of all.
func TestErrorsNegativeLimit(t *testing.T) {
	resetWithCaps(8, 8)
	for i := range 5 {
		RaiseError("bgp", "notification-sent", "10.0.0."+strconv.Itoa(i), "m", nil)
	}
	got := Errors(-1)
	if len(got) != 5 {
		t.Errorf("Errors(-1) should return all 5 entries, got %d", len(got))
	}
	gotZero := Errors(0)
	if len(gotZero) != 5 {
		t.Errorf("Errors(0) should return all 5 entries, got %d", len(gotZero))
	}
}

// VALIDATES: Issue #1 fix, env-supplied caps are clamped at the upper bound.
// PREVENTS: a typo in ze.report.warnings.max from causing OOM at startup.
func TestNewStoreClampsUpperCap(t *testing.T) {
	s := newStore(maxWarningCap*2, maxErrorCap*2)
	if s.warningCap != maxWarningCap {
		t.Errorf("warningCap: got %d want %d", s.warningCap, maxWarningCap)
	}
	if s.errorCap != maxErrorCap {
		t.Errorf("errorCap: got %d want %d", s.errorCap, maxErrorCap)
	}

	// Storage should be allocated at the clamped size.
	if cap(s.errors) != maxErrorCap {
		t.Errorf("errors slice cap: got %d want %d", cap(s.errors), maxErrorCap)
	}
}

// VALIDATES: Issue #1 fix, zero or negative caps fall back to defaults.
func TestNewStoreNegativeCapDefaults(t *testing.T) {
	s := newStore(0, -100)
	if s.warningCap != defaultWarningCap {
		t.Errorf("warningCap: got %d want %d", s.warningCap, defaultWarningCap)
	}
	if s.errorCap != defaultErrorCap {
		t.Errorf("errorCap: got %d want %d", s.errorCap, defaultErrorCap)
	}
}

// VALIDATES: Issue #2 fix, oversized fields are rejected and not stored.
// PREVENTS: a buggy producer (or malicious caller) from filling the bus with
// multi-megabyte entries that consume GB of memory.
func TestRaiseRejectsOversizedFields(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		code    string
		subject string
		message string
	}{
		{"long source", strings.Repeat("a", maxSourceLen+1), "code", "subj", "msg"},
		{"long code", "src", strings.Repeat("a", maxCodeLen+1), "subj", "msg"},
		{"long subject", "src", "code", strings.Repeat("a", maxSubjectLen+1), "msg"},
		{"long message", "src", "code", "subj", strings.Repeat("a", maxMessageLen+1)},
	}
	for _, tc := range cases {
		t.Run("warning/"+tc.name, func(t *testing.T) {
			reset()
			RaiseWarning(tc.source, tc.code, tc.subject, tc.message, nil)
			if got := Warnings(); len(got) != 0 {
				t.Errorf("oversized %s should be rejected, got %d entries", tc.name, len(got))
			}
		})
		t.Run("error/"+tc.name, func(t *testing.T) {
			reset()
			RaiseError(tc.source, tc.code, tc.subject, tc.message, nil)
			if got := Errors(0); len(got) != 0 {
				t.Errorf("oversized %s should be rejected, got %d entries", tc.name, len(got))
			}
		})
	}
}

// VALIDATES: Issue #2 fix, detail map with too many keys is rejected.
func TestRaiseRejectsLargeDetail(t *testing.T) {
	reset()
	detail := make(map[string]any, maxDetailKeys+1)
	for i := range maxDetailKeys + 1 {
		detail["key-"+strconv.Itoa(i)] = i
	}
	RaiseWarning("bgp", "prefix-threshold", "10.0.0.1", "msg", detail)
	if got := Warnings(); len(got) != 0 {
		t.Errorf("detail with %d keys should be rejected, got %d entries", maxDetailKeys+1, len(got))
	}
}

// VALIDATES: Issue #2 fix, fields exactly at the limit are accepted (boundary).
func TestRaiseAcceptsFieldsAtLimit(t *testing.T) {
	reset()
	RaiseWarning(
		strings.Repeat("a", maxSourceLen),
		strings.Repeat("b", maxCodeLen),
		strings.Repeat("c", maxSubjectLen),
		strings.Repeat("d", maxMessageLen),
		nil,
	)
	if got := Warnings(); len(got) != 1 {
		t.Errorf("fields at the limit should be accepted, got %d entries", len(got))
	}
}

// VALIDATES: Note #12, deterministic sequence of raise/clear operations
// produces the expected final state. Complements the race-only concurrent test.
// PREVENTS: subtle dedup or eviction bugs that the concurrent test cannot
// catch because it has no post-state assertion.
func TestSequentialConsistency(t *testing.T) {
	reset()

	// Raise 10 distinct warnings.
	for i := range 10 {
		RaiseWarning("bgp", "prefix-threshold", "peer-"+strconv.Itoa(i), "m", nil)
	}
	if got := Warnings(); len(got) != 10 {
		t.Fatalf("after 10 raises: want 10, got %d", len(got))
	}

	// Re-raise 5 (dedup, count stays the same).
	for i := range 5 {
		RaiseWarning("bgp", "prefix-threshold", "peer-"+strconv.Itoa(i), "updated", nil)
	}
	got := Warnings()
	if len(got) != 10 {
		t.Fatalf("after dedup raises: want 10, got %d", len(got))
	}

	// Clear 3.
	for i := range 3 {
		ClearWarning("bgp", "prefix-threshold", "peer-"+strconv.Itoa(i))
	}
	if got := Warnings(); len(got) != 7 {
		t.Fatalf("after 3 clears: want 7, got %d", len(got))
	}

	// ClearSource removes all 7 remaining BGP warnings.
	ClearSource("bgp")
	if got := Warnings(); len(got) != 0 {
		t.Fatalf("after ClearSource: want 0, got %d", len(got))
	}

	// Errors are independent: raise 5, snapshot returns 5 newest-first.
	for i := range 5 {
		RaiseError("bgp", "notification-sent", "peer-"+strconv.Itoa(i), "m", nil)
	}
	gotErrs := Errors(0)
	if len(gotErrs) != 5 {
		t.Fatalf("after 5 errors: want 5, got %d", len(gotErrs))
	}
	if gotErrs[0].Subject != "peer-4" {
		t.Errorf("most recent error: want peer-4, got %s", gotErrs[0].Subject)
	}
}
