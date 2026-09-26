// VALIDATES: rfcgap.Demonstrate inverts a gap body strictly: a recorded assertion
// failure passes the parent, no failure fails it, and a panic, a skip or a
// malformed id fails it too (spec-rfc-demonstrated-gap AC-1..AC-3, A-1).
// PREVENTS: a closed gap staying published as a gap, and a crash or a skip
// reading as the gap standing.
package rfcgap_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/rfcgap"
)

const testRID = "RFC9999-5.1-1"

// fakeParent stands in for the test that calls Demonstrate, so a test can
// observe the parent's verdict without that verdict failing the real run. It
// embeds the real TB for Helper, Cleanup and the rest, and records every
// failing and logging call. Its Fatal and FailNow do not end the goroutine,
// which also proves Demonstrate returns after each failing verdict.
type fakeParent struct {
	testing.TB

	mu     sync.Mutex
	fatals []string
	errors []string
	logs   []string
	failed bool
}

func (p *fakeParent) add(list *[]string, text string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	*list = append(*list, text)
}

func (p *fakeParent) Fatalf(format string, args ...any) {
	p.add(&p.fatals, fmt.Sprintf(format, args...))
	p.Fail()
}

func (p *fakeParent) Fatal(args ...any) {
	p.add(&p.fatals, fmt.Sprint(args...))
	p.Fail()
}

func (p *fakeParent) Errorf(format string, args ...any) {
	p.add(&p.errors, fmt.Sprintf(format, args...))
	p.Fail()
}

func (p *fakeParent) Error(args ...any) {
	p.add(&p.errors, fmt.Sprint(args...))
	p.Fail()
}

func (p *fakeParent) Logf(format string, args ...any) { p.add(&p.logs, fmt.Sprintf(format, args...)) }

func (p *fakeParent) Log(args ...any) { p.add(&p.logs, fmt.Sprint(args...)) }

func (p *fakeParent) Fail() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failed = true
}

func (p *fakeParent) FailNow() { p.Fail() }

func (p *fakeParent) Failed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.failed
}

// assertGapStands checks the parent passed and logged at least one
// "gap <rid> stands:" line.
func assertGapStands(t *testing.T, parent *fakeParent) {
	t.Helper()
	if parent.Failed() {
		t.Fatalf("parent failed, want pass: fatals %q errors %q", parent.fatals, parent.errors)
	}
	for _, line := range parent.logs {
		if strings.HasPrefix(line, "gap "+testRID+" stands: ") {
			return
		}
	}
	t.Fatalf("no %q line logged: %q", "gap "+testRID+" stands:", parent.logs)
}

// assertParentFatal checks the parent failed through one Fatalf carrying every
// fragment in want.
func assertParentFatal(t *testing.T, parent *fakeParent, want ...string) {
	t.Helper()
	if !parent.Failed() {
		t.Fatalf("parent passed, want fail; logs %q", parent.logs)
	}
	if len(parent.fatals) != 1 {
		t.Fatalf("want exactly one Fatalf verdict, got %q", parent.fatals)
	}
	for _, fragment := range want {
		if !strings.Contains(parent.fatals[0], fragment) {
			t.Errorf("verdict %q does not carry %q", parent.fatals[0], fragment)
		}
	}
}

// TestDemonstrateGapPassesWhileBodyFails proves AC-1 at the real entry point:
// a body whose assertion fails leaves the calling test green, and the failure
// is logged under the requirement id.
func TestDemonstrateGapPassesWhileBodyFails(t *testing.T) {
	t.Run("real parent passes", func(t *testing.T) {
		rfcgap.Demonstrate(t, testRID, func(tb testing.TB) {
			tb.Errorf("MP_REACH_NLRI at position %d, want %d", 3, 0)
		})
	})
	t.Run("failure logged with the id", func(t *testing.T) {
		parent := &fakeParent{TB: t}
		rfcgap.Demonstrate(parent, testRID, func(tb testing.TB) {
			tb.Errorf("MP_REACH_NLRI at position %d, want %d", 3, 0)
		})
		assertGapStands(t, parent)
		if parent.logs[0] != "gap "+testRID+" stands: MP_REACH_NLRI at position 3, want 0" {
			t.Errorf("log %q", parent.logs[0])
		}
	})
}

// TestDemonstrateGapFailsWhenBodyPasses proves AC-2: a body that records no
// failure fails the parent, naming the id, the summary and the retag owed. It
// also proves a malformed id fails the parent before the body runs.
func TestDemonstrateGapFailsWhenBodyPasses(t *testing.T) {
	t.Run("rfc id names its summary file", func(t *testing.T) {
		parent := &fakeParent{TB: t}
		rfcgap.Demonstrate(parent, testRID, func(tb testing.TB) { tb.Log("conforms") })
		assertParentFatal(t, parent,
			"gap "+testRID+" closed",
			"Remove {gap} from rfc/short/rfc9999.md",
			"`RFC requirement: "+testRID+" positive|negative`")
	})
	t.Run("draft id names no guessed path", func(t *testing.T) {
		const draftRID = "DRAFT-IETF-IDR-FOO-3-1"
		parent := &fakeParent{TB: t}
		rfcgap.Demonstrate(parent, draftRID, func(testing.TB) {})
		assertParentFatal(t, parent,
			"gap "+draftRID+" closed",
			"the rfc/short/ summary that declares "+draftRID)
	})
	for _, rid := range []string{"", "RFC9999", " RFC9999-5.1-1", "RFC9999-5.1-x", "RFC9999-5.1-1 gap", "-1"} {
		t.Run("malformed id "+rid, func(t *testing.T) {
			parent := &fakeParent{TB: t}
			ran := false
			rfcgap.Demonstrate(parent, rid, func(tb testing.TB) {
				ran = true
				tb.Error("would read as the gap standing")
			})
			assertParentFatal(t, parent, "is not an id")
			if ran {
				t.Error("body ran for a malformed id")
			}
		})
	}
}

// TestDemonstrateGapCountsEveryFailureStyle proves AC-1 and A-1: every
// assertion style the repository uses is captured by the recorder, leaves the
// parent green, and the ending styles stop the body without ending the parent.
func TestDemonstrateGapCountsEveryFailureStyle(t *testing.T) {
	cases := []struct {
		name   string
		ends   bool
		assert func(tb testing.TB)
	}{
		{"Error", false, func(tb testing.TB) { tb.Error("got", 2, "want", 1) }},
		{"Errorf", false, func(tb testing.TB) { tb.Errorf("got %d want %d", 2, 1) }},
		{"Fail", false, func(tb testing.TB) { tb.Fail() }},
		{"Fatal", true, func(tb testing.TB) { tb.Fatal("got", 2) }},
		{"Fatalf", true, func(tb testing.TB) { tb.Fatalf("got %d", 2) }},
		{"FailNow", true, func(tb testing.TB) { tb.FailNow() }},
		{"testify assert.Equal", false, func(tb testing.TB) { assert.Equal(tb, 1, 2) }},
		{"testify assert.True", false, func(tb testing.TB) { assert.True(tb, false, "order") }},
		{"testify require.Equal", true, func(tb testing.TB) { require.Equal(tb, 1, 2) }},
		{"testify require.NoError", true, func(tb testing.TB) { require.NoError(tb, fmt.Errorf("refused")) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parent := &fakeParent{TB: t}
			reached := false
			sawFailed := false
			rfcgap.Demonstrate(parent, testRID, func(tb testing.TB) {
				tc.assert(tb)
				reached = true
				sawFailed = tb.Failed()
			})
			assertGapStands(t, parent)
			if reached == tc.ends {
				t.Errorf("code after the assertion reached=%v, want %v", reached, !tc.ends)
			}
			if reached && !sawFailed {
				t.Error("tb.Failed() answered false after a recorded failure")
			}
		})
	}
	t.Run("real parent passes on require", func(t *testing.T) {
		rfcgap.Demonstrate(t, testRID, func(tb testing.TB) { require.Equal(tb, "first", "last") })
	})
}

// TestDemonstrateGapPanicIsNotAGap proves AC-3: a panic in the body fails the
// parent with the panic value, even after a recorded assertion failure, and is
// never logged as the gap standing.
func TestDemonstrateGapPanicIsNotAGap(t *testing.T) {
	cases := []struct {
		name  string
		value string
		body  func(tb testing.TB)
	}{
		{"explicit panic after a failure", "BUG: body panics on purpose", func(tb testing.TB) {
			tb.Error("recorded before the crash")
			panic("BUG: body panics on purpose")
		}},
		{"runtime panic", "integer divide by zero", func(tb testing.TB) {
			zero := len(tb.Name()) * 0
			_ = len(tb.Name()) / zero
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parent := &fakeParent{TB: t}
			rfcgap.Demonstrate(parent, testRID, tc.body)
			assertParentFatal(t, parent, "gap "+testRID+": the body panicked", tc.value, "goroutine")
			for _, line := range parent.logs {
				if strings.Contains(line, "stands") {
					t.Errorf("panic logged as the gap standing: %q", line)
				}
			}
		})
	}
}

// TestDemonstrateGapSkipIsNotAGap proves a skip in the body fails the parent:
// it shows neither that the gap stands nor that it closed.
func TestDemonstrateGapSkipIsNotAGap(t *testing.T) {
	cases := map[string]func(tb testing.TB){
		"Skip":               func(tb testing.TB) { tb.Skip("no fixture") },
		"Skipf":              func(tb testing.TB) { tb.Skipf("no %s", "fixture") },
		"SkipNow":            func(tb testing.TB) { tb.SkipNow() },
		"skip after failure": func(tb testing.TB) { tb.Error("recorded"); tb.Skip("then skipped") },
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			parent := &fakeParent{TB: t}
			rfcgap.Demonstrate(parent, testRID, body)
			assertParentFatal(t, parent, "gap "+testRID+": the body skipped")
		})
	}
}
