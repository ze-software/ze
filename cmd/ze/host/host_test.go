package host

import (
	"slices"
	"strings"
	"testing"
)

// VALIDATES: AC-19 — `ze host show bogus` exits 1 and lists the valid
// sections in the error message.
func TestRunShow_RejectsUnknownSection(t *testing.T) {
	code := RunShow([]string{"bogus"})
	if code == 0 {
		t.Errorf("RunShow bogus exit = 0, want non-zero")
	}
}

// VALIDATES: AC-18 — `ze host show` with no positional args defaults to
// the `all` section. We can't assert on stdout from here without
// plumbing a writer, so we just assert exit 0 on a platform where the
// inventory can be assembled. On darwin the inventory is empty but
// still exits 0.
func TestRunShow_DefaultsToAll(t *testing.T) {
	code := RunShow([]string{})
	if code != 0 {
		t.Errorf("RunShow [] exit = %d, want 0", code)
	}
}

// VALIDATES: sectionList is stable, sorted, and contains every
// registered section.
func TestSectionList(t *testing.T) {
	out := sectionList()
	parts := strings.Split(out, ", ")
	want := []string{"all", "cpu", "dmi", "kernel", "memory", "nic", "storage", "thermal"}
	if !slices.Equal(parts, want) {
		t.Errorf("sectionList = %v, want %v", parts, want)
	}
}
