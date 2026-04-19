package host

import (
	"errors"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// VALIDATES: SectionNames returns every registered section, sorted,
// with no duplicates.
// PREVENTS: SectionNames silently diverging from the sectionDetectors
// map (e.g. a future refactor that builds the list from a different
// source and drifts).
func TestSectionNames_Sorted(t *testing.T) {
	names := SectionNames()
	if len(names) == 0 {
		t.Fatal("SectionNames returned empty")
	}
	if len(names) != len(sectionDetectors) {
		t.Errorf("SectionNames len = %d, sectionDetectors len = %d — must be equal",
			len(names), len(sectionDetectors))
	}
	if !slices.IsSorted(names) {
		t.Errorf("SectionNames not sorted: %v", names)
	}
	// No duplicates.
	seen := make(map[string]struct{}, len(names))
	for _, n := range names {
		if _, dup := seen[n]; dup {
			t.Errorf("duplicate section name: %q", n)
		}
		seen[n] = struct{}{}
	}
	// Every map key surfaces.
	for k := range sectionDetectors {
		if !slices.Contains(names, k) {
			t.Errorf("sectionDetectors has key %q missing from SectionNames", k)
		}
	}
}

// VALIDATES: SectionList is the comma-joined form of SectionNames in
// the same sorted order. Callers rely on this exact shape for error
// messages and Meta.Subs strings.
func TestSectionList_Format(t *testing.T) {
	got := SectionList()
	want := strings.Join(SectionNames(), ", ")
	if got != want {
		t.Errorf("SectionList = %q, want %q (must join SectionNames with \", \")", got, want)
	}
	// A representative spot-check: the sorted list should start with
	// "all" (first alphabetically) and contain "kernel" somewhere.
	parts := strings.Split(got, ", ")
	if parts[0] != "all" {
		t.Errorf("SectionList first entry = %q, want \"all\" (alphabetical order)", parts[0])
	}
	if !slices.Contains(parts, "kernel") {
		t.Errorf("SectionList missing \"kernel\": %s", got)
	}
}

// VALIDATES: DetectSection dispatches every registered name through
// its detector function and succeeds against the N100 fixture (or
// returns a non-Unsupported error the caller can surface). Fixture
// drives sysfs/procfs reads, so every section runs a real code path.
func TestDetectSection_DispatchesEachSection(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("fixture drives sysfs/procfs reads; detectors are Linux-only (cpu_other.go etc. return ErrUnsupported at build time)")
	}
	d := &Detector{Root: "testdata/n100-4x-igc"}
	for _, name := range SectionNames() {
		t.Run(name, func(t *testing.T) {
			data, err := d.DetectSection(name)
			if err != nil {
				t.Fatalf("DetectSection(%q): %v", name, err)
			}
			if data == nil {
				// `all` returns *Inventory; other sections return
				// their own pointer or slice — never nil on the
				// populated N100 fixture.
				t.Errorf("DetectSection(%q) returned nil data", name)
			}
		})
	}
}

// VALIDATES: AC-11 on the online side — unknown section name returns
// a wrapped ErrUnknownSection so callers can distinguish "typo" from
// "detection failure" via errors.Is.
// PREVENTS: a future refactor swallowing ErrUnknownSection (e.g. by
// returning a plain fmt.Errorf without %w) and breaking the online
// handler's reject-branch guard.
func TestDetectSection_UnknownReturnsSentinel(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	data, err := d.DetectSection("not-a-section")
	if err == nil {
		t.Fatal("expected error for unknown section; got nil")
	}
	if !errors.Is(err, ErrUnknownSection) {
		t.Errorf("err = %v, want errors.Is(err, ErrUnknownSection)", err)
	}
	if data != nil {
		t.Errorf("data = %v, want nil on error", data)
	}
	// Error message must contain the canonical valid-sections list so
	// the operator sees what they can use. This assertion ties the
	// error to SectionList() — if someone hardcodes a parallel list
	// in the error message, the test catches it.
	msg := err.Error()
	if !strings.Contains(msg, SectionList()) {
		t.Errorf("error message %q must contain SectionList() = %q", msg, SectionList())
	}
}

// VALIDATES: the canonical error does NOT include an unbounded `name`
// argument — a malicious 1 MiB name argument must be truncated in
// the error string, not leaked verbatim.
// PREVENTS: accidental log-flooding or error-message bloat when an
// attacker (or a mis-configured script) passes garbage.
func TestDetectSection_UnknownNameCapped(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	longName := strings.Repeat("A", 1<<20) // 1 MiB
	_, err := d.DetectSection(longName)
	if err == nil {
		t.Fatal("expected error for 1 MiB section name")
	}
	if len(err.Error()) > 4096 {
		t.Errorf("error message length = %d, want ≤ 4096 (name must be truncated)", len(err.Error()))
	}
	if !errors.Is(err, ErrUnknownSection) {
		t.Errorf("err = %v, want errors.Is(err, ErrUnknownSection)", err)
	}
}

// VALIDATES: AC-23 — 32 parallel Detect() calls against the same
// Detector produce no data races (under -race) and each returns an
// independent *Inventory pointer. Exercises every sectional detector
// together — Detect() is the public entry point host RPCs use.
// Moved here from cpu_linux_test.go to align with the TDD plan's
// intent that inventory-level tests live in inventory_test.go.
func TestDetect_Concurrent(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	const n = 32
	done := make(chan *Inventory, n)
	errCh := make(chan error, n)
	for range n {
		go func() {
			inv, err := d.Detect()
			done <- inv
			errCh <- err
		}()
	}
	seen := make(map[*Inventory]struct{})
	for range n {
		inv := <-done
		err := <-errCh
		if err != nil {
			t.Errorf("concurrent Detect: %v", err)
		}
		if inv == nil {
			t.Error("Detect returned nil Inventory under concurrency")
			continue
		}
		if _, dup := seen[inv]; dup {
			t.Error("Detect returned the same *Inventory pointer twice — callers must get independent values")
		}
		seen[inv] = struct{}{}
	}
}
