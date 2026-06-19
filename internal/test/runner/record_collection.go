// Design: docs/architecture/testing/ci-format.md — test record collection and querying
// Overview: record.go — Record type definitions and methods
// Related: record_parse.go — CI file parsing and EncodingTests discovery

package runner

import (
	"sort"
	"sync"
)

// Tests is a container for test records.
type Tests struct {
	byNick  map[string]*Record
	ordered []string
	mu      sync.RWMutex
}

// NewTests creates a new test container.
func NewTests() *Tests {
	return &Tests{
		byNick:  make(map[string]*Record),
		ordered: nil,
	}
}

// Add creates and registers a new test record.
func (ts *Tests) Add(name string) *Record {
	return ts.add(name, "")
}

// addWithNick creates and registers a new test record with a stable caller-supplied nick.
func (ts *Tests) addWithNick(name, nick string) *Record {
	return ts.add(name, nick)
}

func (ts *Tests) add(name, nick string) *Record {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	r := newRecord(name)
	if nick != "" {
		r.Nick = nick
	}
	ts.byNick[r.Nick] = r
	ts.ordered = append(ts.ordered, r.Nick)
	return r
}

// GetByNick returns the test with the given nick.
func (ts *Tests) GetByNick(nick string) *Record {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.byNick[nick]
}

// Registered returns all tests in order.
func (ts *Tests) Registered() []*Record {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	result := make([]*Record, 0, len(ts.ordered))
	for _, nick := range ts.ordered {
		result = append(result, ts.byNick[nick])
	}
	return result
}

// Selected returns active tests.
func (ts *Tests) Selected() []*Record {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var result []*Record
	for _, nick := range ts.ordered {
		r := ts.byNick[nick]
		if r.IsActive() {
			result = append(result, r)
		}
	}
	return result
}

// Count returns the number of tests.
func (ts *Tests) Count() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return len(ts.ordered)
}

// Summary returns counts by state.
func (ts *Tests) Summary() (passed, failed, timedOut, skipped int) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	for _, r := range ts.byNick {
		switch r.State { //nolint:exhaustive // only count terminal states
		case StateSuccess:
			passed++
		case StateFail:
			failed++
		case StateTimeout:
			timedOut++
		case StateSkip:
			skipped++
		}
	}
	return
}

// failedRecords returns failed test records.
func (ts *Tests) failedRecords() []*Record {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var result []*Record
	for _, nick := range ts.ordered {
		r := ts.byNick[nick]
		if r.State == StateFail || r.State == StateTimeout {
			result = append(result, r)
		}
	}
	return result
}

// failedNicks returns nicks of failed tests (not including timed out).
func (ts *Tests) failedNicks() []string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var result []string
	for _, nick := range ts.ordered {
		r := ts.byNick[nick]
		if r.State == StateFail {
			result = append(result, nick)
		}
	}
	return result
}

// skippedNicks returns nicks of skipped tests (option=skip-os match).
func (ts *Tests) skippedNicks() []string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var result []string
	for _, nick := range ts.ordered {
		r := ts.byNick[nick]
		if r.State == StateSkip {
			result = append(result, nick)
		}
	}
	return result
}

// timedOutNicks returns nicks of timed out tests.
func (ts *Tests) timedOutNicks() []string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var result []string
	for _, nick := range ts.ordered {
		r := ts.byNick[nick]
		if r.State == StateTimeout {
			result = append(result, nick)
		}
	}
	return result
}

// Sort orders tests by name.
func (ts *Tests) Sort() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	sort.Slice(ts.ordered, func(i, j int) bool {
		return ts.byNick[ts.ordered[i]].Name < ts.byNick[ts.ordered[j]].Name
	})
}

// List prints available tests.
func (ts *Tests) List() {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	writeTestListHeader("Available tests")
	total := len(ts.ordered)
	for i, nick := range ts.ordered {
		r := ts.byNick[nick]
		writeTestListLine(i+1, total, r.Nick, r.Name, "")
	}
	writeTestListFooter()
}
