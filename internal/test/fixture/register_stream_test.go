// VALIDATES: the stream drivers `dynamic` and `watchdog` are registered in the
// harness fixture registry, which `le test fixture` runs, and write the same
// lines at the same pauses as the le actions they replaced (AC-35 of
// spec-le-subject-first-command-tree).
package fixture

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestFixtureCarriesTheStreamDrivers proves the merge: the bare `le test
// fixture` lists every driver from Names, so the two stream drivers must be
// there, and a stream driver given an argument refuses rather than ignoring it.
func TestFixtureCarriesTheStreamDrivers(t *testing.T) {
	names := Names()
	for _, want := range []string{"dynamic", "watchdog"} {
		if !slices.Contains(names, want) {
			t.Errorf("fixture registry holds no %q driver: %v", want, names)
		}
	}

	driversMu.RLock()
	driver := drivers["dynamic"]
	driversMu.RUnlock()
	if err := driver(t.Context(), []string{"extra"}); err == nil {
		t.Error("dynamic accepted an argument; a stream driver takes none")
	}
}

func TestStreamDynamicPreservesProtocolSequence(t *testing.T) {
	var out strings.Builder
	var pauses []time.Duration
	pause := func(_ context.Context, duration time.Duration) bool {
		pauses = append(pauses, duration)
		return len(pauses) < 4
	}

	if err := streamDynamic(t.Context(), &out, pause); err != nil {
		t.Fatalf("streamDynamic: %v", err)
	}

	want := "announce flow route {\\n match {\\n source 10.0.0.1/32;\\n destination 1.2.3.4/32;\\n }\\n then {\\n discard;\\n }\\n }\\n\n" +
		"update text nhop set 10.0.0.1 nlri ipv4/unicast add 192.0.2.1/32\n" +
		"update text nlri ipv4/unicast del 192.0.2.1/32\n" +
		"withdraw flow route {\\n match {\\n source 10.0.0.1/32;\\n destination 1.2.3.4/32;\\n }\\n then {\\n discard;\\n }\\n }\\n\n"
	if got := out.String(); got != want {
		t.Fatalf("dynamic output:\n got %q\nwant %q", got, want)
	}
	if got, wantPauses := pauses, []time.Duration{10 * time.Second, 10 * time.Second, 10 * time.Second, 10 * time.Second}; !equalDurations(got, wantPauses) {
		t.Fatalf("dynamic pauses = %v, want %v", got, wantPauses)
	}
}

func TestStreamWatchdogPreservesProtocolSequence(t *testing.T) {
	var out strings.Builder
	var pauses []time.Duration
	pause := func(_ context.Context, duration time.Duration) bool {
		pauses = append(pauses, duration)
		return len(pauses) < 5
	}

	if err := streamWatchdog(t.Context(), &out, pause); err != nil {
		t.Fatalf("streamWatchdog: %v", err)
	}

	want := "bgp watchdog withdraw\n" +
		"bgp watchdog withdraw watchdog-one\n" +
		"bgp watchdog announce\n" +
		"bgp watchdog announce watchdog-one\n"
	if got := out.String(); got != want {
		t.Fatalf("watchdog output:\n got %q\nwant %q", got, want)
	}
	if got, wantPauses := pauses, []time.Duration{10 * time.Second, 5 * time.Second, 5 * time.Second, 5 * time.Second, 5 * time.Second}; !equalDurations(got, wantPauses) {
		t.Fatalf("watchdog pauses = %v, want %v", got, wantPauses)
	}
}

func TestStreamWatchdogWritesAdjacentUnusedNameCommands(t *testing.T) {
	var out strings.Builder
	calls := 0
	pause := func(_ context.Context, _ time.Duration) bool {
		calls++
		return calls < 6
	}

	if err := streamWatchdog(t.Context(), &out, pause); err != nil {
		t.Fatalf("streamWatchdog: %v", err)
	}
	if got, want := out.String(), "bgp watchdog withdraw\n"+
		"bgp watchdog withdraw watchdog-one\n"+
		"bgp watchdog announce\n"+
		"bgp watchdog announce watchdog-one\n"+
		"bgp watchdog announce watchdog-two\n"+
		"bgp watchdog withdraw watchdog-two\n"; got != want {
		t.Fatalf("watchdog output:\n got %q\nwant %q", got, want)
	}
}

func TestStreamReturnsWriterError(t *testing.T) {
	want := errors.New("closed")
	writer := errorWriter{err: want}
	pause := func(context.Context, time.Duration) bool { return true }

	err := streamDynamic(t.Context(), writer, pause)
	if !errors.Is(err, want) {
		t.Fatalf("streamDynamic error = %v, want wrapped %v", err, want)
	}
}

type errorWriter struct{ err error }

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }

func equalDurations(a, b []time.Duration) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
