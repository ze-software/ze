// VALIDATES: AC-10 -- ValidateSubsystem and SubsystemsMatching work correctly.
// PREVENTS: Invalid subsystem names accepted, hierarchical matching broken.

package slogutil

import (
	"strings"
	"testing"
)

func TestValidateSubsystem(t *testing.T) {
	ResetLevelRegistry()
	_ = Logger("test.valid")
	_ = Logger("test.valid.child")

	tests := []struct {
		name  string
		valid bool
	}{
		{"all", true},
		{"test.valid", true},
		{"test.valid.child", true},
		{"test", true},
		{"nonexistent", false},
		{"", false},
		{"foo/bar", false},
		{"test\x00valid", false},
	}

	for _, tt := range tests {
		got := validateSubsystem(tt.name)
		if got != tt.valid {
			t.Errorf("ValidateSubsystem(%q) = %v, want %v", tt.name, got, tt.valid)
		}
	}
}

func TestSubsystemsMatching(t *testing.T) {
	ResetLevelRegistry()
	_ = Logger("test.match.a")
	_ = Logger("test.match.b")
	_ = Logger("test.other")

	matches := SubsystemsMatching("test.match")
	if len(matches) != 2 {
		t.Errorf("SubsystemsMatching(test.match): got %d matches, want 2: %v", len(matches), matches)
	}

	matches = SubsystemsMatching("test")
	if len(matches) < 3 {
		t.Errorf("SubsystemsMatching(test): got %d matches, want >=3: %v", len(matches), matches)
	}
}

func TestRestoreLevel(t *testing.T) {
	ResetLevelRegistry()
	_ = Logger("test.restore")

	_ = SetLevel("test.restore", "debug")
	levels := ListLevels()
	if levels["test.restore"] != "debug" {
		t.Fatalf("level = %q, want debug", levels["test.restore"])
	}

	RestoreLevel("test.restore")
	levels = ListLevels()
	if levels["test.restore"] == "debug" {
		t.Error("level should not be debug after RestoreLevel")
	}
}

func TestSubsystemsMatchingExact(t *testing.T) {
	ResetLevelRegistry()
	_ = Logger("test.exact")

	matches := SubsystemsMatching("test.exact")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d: %v", len(matches), matches)
	}
	if matches[0] != "test.exact" {
		t.Errorf("match = %q, want test.exact", matches[0])
	}
}

func TestSubsystemsMatchingNone(t *testing.T) {
	ResetLevelRegistry()
	_ = Logger("test.nomatch")

	matches := SubsystemsMatching("nonexistent")
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d: %v", len(matches), matches)
	}
}

func TestValidateSubsystemAll(t *testing.T) {
	ResetLevelRegistry()
	if !validateSubsystem("all") {
		t.Error("expected 'all' to be valid")
	}
}

func TestSubsystemsMatchingAll(t *testing.T) {
	ResetLevelRegistry()
	_ = Logger("test.all.a")
	_ = Logger("test.all.b")

	matches := SubsystemsMatching("all")
	if len(matches) < 2 {
		t.Errorf("SubsystemsMatching(all): got %d matches, want >=2", len(matches))
	}
}

func TestSubsystemsMatchingPrefix(t *testing.T) {
	ResetLevelRegistry()
	_ = Logger("test.prefix.a")
	_ = Logger("test.prefix.b")
	_ = Logger("test.other")

	matches := SubsystemsMatching("test.prefix")
	for _, m := range matches {
		if !strings.HasPrefix(m, "test.prefix") {
			t.Errorf("unexpected match: %q", m)
		}
	}
}
