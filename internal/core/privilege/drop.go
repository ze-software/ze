// Design: docs/architecture/system-architecture.md — daemon privilege dropping
// Detail: drop_unix.go — Unix setuid/setgid implementation
// Detail: drop_other.go — no-op for non-Unix platforms
//
// Package privilege provides daemon privilege dropping after port binding.
// When ze starts as root to bind port 179, it drops to the configured
// user/group before spawning plugins or accepting connections.
package privilege

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
)

// DropConfig holds the user/group to drop privileges to.
type DropConfig struct {
	User  string // Username to switch to (e.g., "zeuser")
	Group string // Group name to switch to (empty = primary group of User)
}

// DropConfigFromEnv reads ze.user and ze.group from environment variables.
// Returns empty User if neither ze.user nor ze_user is set (caller should skip drop).
func DropConfigFromEnv() DropConfig {
	return DropConfig{
		User:  envLookup("ze.user", "ze_user"),
		Group: envLookup("ze.group", "ze_group"),
	}
}

// envLookup returns the first non-empty env var value.
func envLookup(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// resolveIDs resolves username and group name to numeric uid/gid and
// the target user's supplementary group list for setgroups.
// Accepts both names ("zeuser") and numeric IDs ("1000").
// If group is empty, uses the primary group of the user.
func resolveIDs(cfg DropConfig) (uid, gid int, suppGroups []int, err error) {
	if cfg.User == "" {
		return 0, 0, nil, fmt.Errorf("empty user")
	}

	u, err := lookupUser(cfg.User)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("lookup user %q: %w", cfg.User, err)
	}

	uid, err = strconv.Atoi(u.Uid)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("parse uid %q: %w", u.Uid, err)
	}

	if cfg.Group != "" {
		g, err := lookupGroup(cfg.Group)
		if err != nil {
			return 0, 0, nil, fmt.Errorf("lookup group %q: %w", cfg.Group, err)
		}
		gid, err = strconv.Atoi(g.Gid)
		if err != nil {
			return 0, 0, nil, fmt.Errorf("parse gid %q: %w", g.Gid, err)
		}
	} else {
		gid, err = strconv.Atoi(u.Gid)
		if err != nil {
			return 0, 0, nil, fmt.Errorf("parse gid %q: %w", u.Gid, err)
		}
	}

	// Resolve supplementary groups for setgroups.
	// Always include the primary gid.
	suppGroups = []int{gid}
	if groupIDs, err := u.GroupIds(); err == nil {
		for _, gidStr := range groupIDs {
			if g, err := strconv.Atoi(gidStr); err == nil && g != gid {
				suppGroups = append(suppGroups, g)
			}
		}
	}

	return uid, gid, suppGroups, nil
}

// lookupUser looks up a user by name, falling back to numeric UID.
func lookupUser(name string) (*user.User, error) {
	u, err := user.Lookup(name)
	if err == nil {
		return u, nil
	}
	// Fall back to numeric UID lookup (non-negative only)
	if n, numErr := strconv.Atoi(name); numErr == nil && n >= 0 {
		return user.LookupId(name)
	}
	return nil, err
}

// lookupGroup looks up a group by name, falling back to numeric GID.
func lookupGroup(name string) (*user.Group, error) {
	g, err := user.LookupGroup(name)
	if err == nil {
		return g, nil
	}
	// Fall back to numeric GID lookup (non-negative only)
	if n, numErr := strconv.Atoi(name); numErr == nil && n >= 0 {
		return user.LookupGroupId(name)
	}
	return nil, err
}
