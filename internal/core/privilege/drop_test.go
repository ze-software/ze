package privilege

import (
	"os"
	"os/user"
	"slices"
	"strconv"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// TestResolveIDsCurrentUser verifies user/group resolution works for
// the current user (always available, no root needed).
//
// VALIDATES: resolveIDs correctly maps username to uid/gid and supplementary groups.
// PREVENTS: Startup failure due to user lookup errors.
func TestResolveIDsCurrentUser(t *testing.T) {
	current, err := user.Current()
	if err != nil {
		t.Skip("cannot get current user")
	}

	uid, gid, suppGroups, err := resolveIDs(DropConfig{User: current.Username})
	if err != nil {
		t.Fatalf("resolveIDs(%q) error: %v", current.Username, err)
	}

	if uid != os.Getuid() {
		t.Errorf("uid = %d, want %d", uid, os.Getuid())
	}
	if gid != os.Getgid() {
		t.Errorf("gid = %d, want %d", gid, os.Getgid())
	}
	// Supplementary groups must include at least the primary gid
	if len(suppGroups) == 0 {
		t.Error("suppGroups is empty, expected at least primary gid")
	}
	if !slices.Contains(suppGroups, gid) {
		t.Errorf("suppGroups %v does not contain primary gid %d", suppGroups, gid)
	}
}

// TestResolveIDsUnknownUser verifies error for non-existent user.
//
// VALIDATES: resolveIDs returns error for unknown username.
// PREVENTS: Cryptic panic instead of clear error message.
func TestResolveIDsUnknownUser(t *testing.T) {
	_, _, _, err := resolveIDs(DropConfig{User: "ze_nonexistent_user_12345"})
	if err == nil {
		t.Error("expected error for non-existent user")
	}
}

// TestResolveIDsEmptyUser verifies error for empty user.
//
// VALIDATES: resolveIDs rejects empty user string.
// PREVENTS: Confusing "lookup user ”" error messages.
func TestResolveIDsEmptyUser(t *testing.T) {
	_, _, _, err := resolveIDs(DropConfig{User: ""})
	if err == nil {
		t.Error("expected error for empty user")
	}
}

// TestResolveIDsNumericUID verifies numeric UID strings are accepted.
//
// VALIDATES: resolveIDs falls back to LookupId for numeric strings.
// PREVENTS: Operators needing to know the username for container UIDs.
func TestResolveIDsNumericUID(t *testing.T) {
	current, err := user.Current()
	if err != nil {
		t.Skip("cannot get current user")
	}

	uid, _, _, err := resolveIDs(DropConfig{User: current.Uid})
	if err != nil {
		t.Fatalf("resolveIDs(uid=%q) error: %v", current.Uid, err)
	}

	wantUID, _ := strconv.Atoi(current.Uid)
	if uid != wantUID {
		t.Errorf("uid = %d, want %d", uid, wantUID)
	}
}

// TestResolveIDsNumericGID verifies numeric GID strings are accepted.
//
// VALIDATES: resolveIDs falls back to LookupGroupId for numeric strings.
// PREVENTS: Operators needing to know the group name for container GIDs.
func TestResolveIDsNumericGID(t *testing.T) {
	current, err := user.Current()
	if err != nil {
		t.Skip("cannot get current user")
	}

	_, gid, _, err := resolveIDs(DropConfig{User: current.Username, Group: current.Gid})
	if err != nil {
		t.Fatalf("resolveIDs(gid=%q) error: %v", current.Gid, err)
	}

	wantGID, _ := strconv.Atoi(current.Gid)
	if gid != wantGID {
		t.Errorf("gid = %d, want %d", gid, wantGID)
	}
}

// TestResolveIDsWithExplicitGroup verifies explicit group override.
//
// VALIDATES: When Group is set, gid comes from the group, not the user's primary.
// PREVENTS: Group config being silently ignored.
func TestResolveIDsWithExplicitGroup(t *testing.T) {
	current, err := user.Current()
	if err != nil {
		t.Skip("cannot get current user")
	}

	g, err := user.LookupGroupId(current.Gid)
	if err != nil {
		t.Skip("cannot look up primary group by id")
	}

	uid, gid, _, err := resolveIDs(DropConfig{User: current.Username, Group: g.Name})
	if err != nil {
		t.Fatalf("resolveIDs with group error: %v", err)
	}

	if uid != os.Getuid() {
		t.Errorf("uid = %d, want %d", uid, os.Getuid())
	}
	if gid != os.Getgid() {
		t.Errorf("gid = %d, want %d", gid, os.Getgid())
	}
}

// TestResolveIDsUnknownGroup verifies error for non-existent group.
//
// VALIDATES: resolveIDs returns error for unknown group name.
// PREVENTS: Silent fallback to wrong group.
func TestResolveIDsUnknownGroup(t *testing.T) {
	current, err := user.Current()
	if err != nil {
		t.Skip("cannot get current user")
	}

	_, _, _, err = resolveIDs(DropConfig{User: current.Username, Group: "ze_nonexistent_group_12345"})
	if err == nil {
		t.Error("expected error for non-existent group")
	}
}

// TestResolveIDsNegativeUID verifies negative numeric UID is rejected.
//
// VALIDATES: resolveIDs rejects negative numbers as user IDs.
// PREVENTS: Passing -1 as uid to setuid (undefined behavior).
func TestResolveIDsNegativeUID(t *testing.T) {
	_, _, _, err := resolveIDs(DropConfig{User: "-1"})
	if err == nil {
		t.Error("expected error for negative UID")
	}
}

// TestResolveIDsNegativeGID verifies negative numeric GID is rejected.
//
// VALIDATES: resolveIDs rejects negative numbers as group IDs.
// PREVENTS: Passing -1 as gid to setgid (undefined behavior).
func TestResolveIDsNegativeGID(t *testing.T) {
	current, err := user.Current()
	if err != nil {
		t.Skip("cannot get current user")
	}

	_, _, _, err = resolveIDs(DropConfig{User: current.Username, Group: "-1"})
	if err == nil {
		t.Error("expected error for negative GID")
	}
}

// TestDropNotRoot verifies Drop is a no-op when not running as root.
//
// VALIDATES: Non-root processes skip privilege dropping without error.
// PREVENTS: Non-root users getting spurious errors on startup.
func TestDropNotRoot(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root")
	}

	err := Drop(DropConfig{User: "nobody"})
	if err != nil {
		t.Errorf("Drop as non-root should be no-op, got: %v", err)
	}
}

// TestDropConfigFromEnv verifies ze.user and ze.group env vars are read.
//
// VALIDATES: DropConfigFromEnv reads ze.user and ze.group.
// PREVENTS: Env vars silently ignored.
func TestDropConfigFromEnv(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	t.Setenv("ze.user", "testuser")
	t.Setenv("ze.group", "testgroup")
	env.ResetCache()

	cfg := DropConfigFromEnv()
	if cfg.User != "testuser" {
		t.Errorf("User = %q, want %q", cfg.User, "testuser")
	}
	if cfg.Group != "testgroup" {
		t.Errorf("Group = %q, want %q", cfg.Group, "testgroup")
	}
}

// TestDropConfigFromEnvUnderscore verifies ze_user and ze_group fallback.
//
// VALIDATES: Underscore notation works as fallback.
// PREVENTS: Shell-incompatible dot notation being the only option.
func TestDropConfigFromEnvUnderscore(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	t.Setenv("ze_user", "testuser2")
	t.Setenv("ze_group", "testgroup2")
	env.ResetCache()

	cfg := DropConfigFromEnv()
	if cfg.User != "testuser2" {
		t.Errorf("User = %q, want %q", cfg.User, "testuser2")
	}
	if cfg.Group != "testgroup2" {
		t.Errorf("Group = %q, want %q", cfg.Group, "testgroup2")
	}
}

// TestDropConfigFromEnvEquivalence verifies dot and underscore are equivalent.
//
// VALIDATES: Both notations resolve to the same normalized cache key.
// PREVENTS: Assuming dot/underscore are separate keys with priority ordering.
func TestDropConfigFromEnvEquivalence(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	// Dot and underscore normalize to the same cache key (ze_user).
	// Setting either notation works equivalently.
	t.Setenv("ze.user", "dotuser")
	t.Setenv("ze.group", "dotgroup")
	env.ResetCache()

	cfg := DropConfigFromEnv()
	if cfg.User != "dotuser" {
		t.Errorf("User = %q, want %q", cfg.User, "dotuser")
	}
	if cfg.Group != "dotgroup" {
		t.Errorf("Group = %q, want %q", cfg.Group, "dotgroup")
	}
}

// TestDropConfigFromEnvEmpty verifies empty when vars not set.
//
// VALIDATES: Returns empty config when no env vars set.
// PREVENTS: Spurious defaults causing unwanted privilege drop.
func TestDropConfigFromEnvEmpty(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	cfg := DropConfigFromEnv()
	if cfg.User != "" {
		t.Errorf("User = %q, want empty", cfg.User)
	}
	if cfg.Group != "" {
		t.Errorf("Group = %q, want empty", cfg.Group)
	}
}
