// Design: docs/contributing/rfc-conformance-gates.md -- demonstrated gaps
// Related: internal/le/rfc/gaps.go -- gapTagRefusal, the gate that ties a gap tag to this call

// Package rfcgap lets a Go test demonstrate a requirement Ze knowingly does not
// meet, rather than describe it in prose.
//
// A requirement Ze does not meet carries `{gap: reason}` in its
// `rfc/short/<stem>.md` row. A test tagged `RFC requirement: <ID> gap` calls
// Demonstrate with that id and asserts the RFC-correct behavior inside the
// body. While Ze still lacks the behavior the body records an assertion
// failure, and the test passes. The day the behavior lands the body records
// none, and the test fails naming the two edits the closed gap owes. This is a
// strict expected-failure: an unexpected pass is a red, never a warning,
// because a warning is how a fixed requirement stays published as a gap.
package rfcgap

import (
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
)

// ridRE is the requirement id grammar the tag scanner accepts
// (changedTagRE in internal/le/rfc/tags.go): a prefix, then a dash and an
// ordinal. It is restated rather than imported because internal/test MUST NOT
// depend on repository tooling under internal/le.
var ridRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*-\d+$`)

// rfcStemRE matches the id prefix of an RFC summary. Only this form maps back
// to its summary file without a lookup: a draft stem holds dashes, so where a
// draft id's prefix ends is not recoverable from the id alone.
var rfcStemRE = regexp.MustCompile(`^RFC(\d+)-`)

// Demonstrate runs body against a recorder and inverts its result.
//
// The body asserts the RFC-correct behavior for requirement rid through the
// testing.TB it receives. Every assertion style counts: Error, Errorf, Fatal,
// Fatalf, Fail, FailNow, and testify assert or require, which reach the TB
// through those same methods.
//
// The verdict on t:
//   - the body recorded an assertion failure: the gap stands, t passes, and each
//     recorded failure is logged as "gap <rid> stands: <failure>";
//   - the body recorded none: the gap closed, and t fails naming the summary row
//     and the retag owed;
//   - the body panicked: t fails with the panic value and stack. A crash is a
//     defect of its own and never counts as the gap standing;
//   - the body skipped: t fails. A skip shows neither that the gap stands nor
//     that it closed, so it is refused rather than read as either;
//   - rid is empty or malformed: t fails before the body runs, because a tag
//     cannot name an id the gate cannot read.
//
// Every verdict that fails t returns right after, so a t whose Fatalf does not
// end the goroutine still gets exactly one verdict.
//
// The body runs in a goroutine Demonstrate starts and joins before it returns,
// so Fatal and FailNow end the body with runtime.Goexit and never the caller.
func Demonstrate(t testing.TB, rid string, body func(tb testing.TB)) {
	t.Helper()
	if !ridRE.MatchString(rid) {
		t.Fatalf("rfcgap: requirement id %q is not an id of the form <PREFIX>-<section>-<ordinal>", rid)
		return
	}
	rec := &recorder{TB: t}
	var result bodyResult
	done := make(chan struct{})
	go rec.runBody(body, &result, done)
	<-done
	if result.panicked {
		t.Fatalf("gap %s: the body panicked, which is a defect of its own and never the gap standing: %v\n%s",
			rid, result.panicValue, result.stack)
		return
	}
	if rec.skip != nil {
		t.Logf("gap %s skipped: "+rec.skip.format, prepend(rid, rec.skip.args)...)
		t.Fatalf("gap %s: the body skipped; a skip neither demonstrates the gap nor proves it closed", rid)
		return
	}
	if len(rec.failures) == 0 {
		t.Fatalf("gap %s closed: the behavior now conforms. Remove {gap} from %s and retag this test "+
			"`RFC requirement: %s positive|negative`", rid, summaryOf(rid), rid)
		return
	}
	for _, failure := range rec.failures {
		t.Logf("gap %s stands: "+failure.format, prepend(rid, failure.args)...)
	}
}

// summaryOf names the summary that declares rid, as precisely as the id alone
// allows.
func summaryOf(rid string) string {
	match := rfcStemRE.FindStringSubmatch(rid)
	if match == nil {
		return "the rfc/short/ summary that declares " + rid
	}
	return "rfc/short/rfc" + match[1] + ".md"
}

func prepend(rid string, args []any) []any {
	all := make([]any, 0, len(args)+1)
	all = append(all, rid)
	return append(all, args...)
}

// bodyResult is what the body goroutine reports when it ends.
type bodyResult struct {
	panicked   bool
	panicValue any
	stack      []byte
}

// entry is one recorded failure or skip, kept as a format and its operands so
// the real TB formats it when Demonstrate logs it.
type entry struct {
	format string
	args   []any
}

// recorder is the testing.TB the body receives. It records every failure and
// skip in place of reporting it, and forwards every other method (Log, Helper,
// Cleanup, TempDir, Setenv, Context, Name, ...) to the real TB.
//
// Embedding is the only way to satisfy testing.TB, which carries an unexported
// method. Every failing or skipping method of testing.TB is overridden below,
// so none of them reaches the real TB.
//
// Safe for concurrent use: a body can assert from a goroutine it starts, as
// testing.T permits.
type recorder struct {
	testing.TB

	mu       sync.Mutex
	failures []entry
	skip     *entry
}

// runBody runs body and closes done when it ends, by return, by
// runtime.Goexit, or by panic. It is the body goroutine: a test helper with
// one lifecycle, which Demonstrate joins on done before it reads result.
func (r *recorder) runBody(body func(tb testing.TB), result *bodyResult, done chan<- struct{}) {
	defer close(done)
	defer func() {
		// runtime.Goexit also runs this deferred call, and recover then answers nil.
		value := recover()
		if value == nil {
			return
		}
		*result = bodyResult{panicked: true, panicValue: value, stack: debug.Stack()}
	}()
	body(r)
}

func (r *recorder) record(format string, args []any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failures = append(r.failures, entry{format: format, args: args})
}

// recordIfSilent records a bare Fail or FailNow, which carries no message, only
// when nothing was recorded yet: testify's require calls Errorf and then
// FailNow, and the Errorf already carries the failure.
func (r *recorder) recordIfSilent(what string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.failures) > 0 {
		return
	}
	r.failures = append(r.failures, entry{format: what})
}

// skipNow records the skip and ends the body.
func (r *recorder) skipNow(format string, args []any) {
	r.mu.Lock()
	r.skip = &entry{format: format, args: args}
	r.mu.Unlock()
	runtime.Goexit()
}

// Error records the failure in place of reporting it.
func (r *recorder) Error(args ...any) { r.record(lineFormat(len(args)), args) }

// Errorf records the failure in place of reporting it.
func (r *recorder) Errorf(format string, args ...any) { r.record(format, args) }

// Fail records a failure with no message.
func (r *recorder) Fail() { r.recordIfSilent("Fail called") }

// FailNow records a failure with no message and ends the body.
func (r *recorder) FailNow() {
	r.recordIfSilent("FailNow called")
	runtime.Goexit()
}

// Failed answers whether the body recorded a failure. It never reads the real
// TB, so a body that branches on Failed sees its own assertions only.
func (r *recorder) Failed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.failures) > 0
}

// Fatal records the failure and ends the body.
func (r *recorder) Fatal(args ...any) {
	r.record(lineFormat(len(args)), args)
	runtime.Goexit()
}

// Fatalf records the failure and ends the body.
func (r *recorder) Fatalf(format string, args ...any) {
	r.record(format, args)
	runtime.Goexit()
}

// Skip records the skip and ends the body. Demonstrate fails the parent for it.
func (r *recorder) Skip(args ...any) { r.skipNow(lineFormat(len(args)), args) }

// SkipNow records the skip and ends the body. Demonstrate fails the parent for it.
func (r *recorder) SkipNow() { r.skipNow("SkipNow called", nil) }

// Skipf records the skip and ends the body. Demonstrate fails the parent for it.
func (r *recorder) Skipf(format string, args ...any) { r.skipNow(format, args) }

// Skipped answers whether the body skipped.
func (r *recorder) Skipped() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.skip != nil
}

// lineFormat answers the format that renders count operands the way
// testing.T.Error does: each with %v, separated by one space.
func lineFormat(count int) string {
	if count == 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("%v ", count), " ")
}
