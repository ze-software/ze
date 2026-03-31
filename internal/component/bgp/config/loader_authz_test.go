package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"
)

// TestExtractAuthzConfig verifies that authorization profiles are correctly
// parsed from the config tree into an authz.Store.
//
// VALIDATES: extractAuthzConfig creates Store with profiles, sections, entries.
// PREVENTS: Config authz block silently ignored — profiles never loaded.
func TestExtractAuthzConfig(t *testing.T) {
	input := `
bgp {
    peer loopback {
        connection {
            remote {
                ip 127.0.0.1
            }
            local {
                ip 127.0.0.1
            }
        }
        session {
            asn {
                local 65533
                remote 65533
            }
        }
    }
}

system {
    authorization {
        profile noc {
            run {
                default-action deny
                entry 10 {
                    action allow
                    match "peer show"
                }
                entry 20 {
                    action allow
                    match "peer summary"
                }
            }
            edit {
                default-action deny
            }
        }
        profile admin {
            run {
                default-action allow
            }
            edit {
                default-action allow
            }
        }
    }
}
`
	tree, err := parseTreeWithYANG(input, nil)
	require.NoError(t, err)

	store := extractAuthzConfig(tree)
	require.NotNil(t, store, "store should not be nil when profiles exist")
	assert.True(t, store.HasProfiles(), "store should have profiles")
}

// TestExtractAuthzConfig_NoSystem verifies nil return when no system block.
//
// VALIDATES: extractAuthzConfig returns nil when no system container.
// PREVENTS: Panic on missing system container.
func TestExtractAuthzConfig_NoSystem(t *testing.T) {
	input := `
bgp {
    peer loopback {
        connection {
            remote {
                ip 127.0.0.1
            }
            local {
                ip 127.0.0.1
            }
        }
        session {
            asn {
                local 65533
                remote 65533
            }
        }
    }
}
`
	tree, err := parseTreeWithYANG(input, nil)
	require.NoError(t, err)

	store := extractAuthzConfig(tree)
	assert.Nil(t, store, "no system block means no authz store")
}

// TestExtractAuthzConfig_NoAuthorization verifies nil when system exists but no authorization.
//
// VALIDATES: extractAuthzConfig returns nil when system has no authorization container.
// PREVENTS: Empty store created from SSH-only system config.
func TestExtractAuthzConfig_NoAuthorization(t *testing.T) {
	input := `
bgp {
    peer loopback {
        connection {
            remote {
                ip 127.0.0.1
            }
            local {
                ip 127.0.0.1
            }
        }
        session {
            asn {
                local 65533
                remote 65533
            }
        }
    }
}

system {
    authentication {
        user admin {
            password "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012"
        }
    }
}
`
	tree, err := parseTreeWithYANG(input, nil)
	require.NoError(t, err)

	store := extractAuthzConfig(tree)
	assert.Nil(t, store, "no authorization block means no authz store")
}

// TestExtractAuthzConfig_UserAssignments verifies user→profile assignments are extracted.
//
// VALIDATES: extractAuthzConfig reads profile leaf-list from authentication.user entries.
// PREVENTS: Users authenticated but never assigned profiles — all get admin by default.
func TestExtractAuthzConfig_UserAssignments(t *testing.T) {
	input := `
bgp {
    peer loopback {
        connection {
            remote {
                ip 127.0.0.1
            }
            local {
                ip 127.0.0.1
            }
        }
        session {
            asn {
                local 65533
                remote 65533
            }
        }
    }
}

system {
    authentication {
        user operator {
            password "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012"
            profile [ noc ]
        }
        user superadmin {
            password "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012"
            profile [ admin ]
        }
    }
    authorization {
        profile noc {
            run {
                default-action allow
                entry 10 {
                    action deny
                    match restart
                }
            }
            edit {
                default-action deny
            }
        }
        profile admin {
            run {
                default-action allow
            }
            edit {
                default-action allow
            }
        }
    }
}
`
	tree, err := parseTreeWithYANG(input, nil)
	require.NoError(t, err)

	store := extractAuthzConfig(tree)
	require.NotNil(t, store)
	assert.True(t, store.HasProfiles())
	assert.True(t, store.HasUserAssignments(), "user assignments should be extracted")
}

// TestExtractAuthzConfig_DeniesRestrictedCommand verifies the extracted store
// correctly denies commands based on profile entries.
//
// VALIDATES: Config→Store pipeline produces working authorization decisions.
// PREVENTS: Profiles parsed but entries ignored — everything allowed.
func TestExtractAuthzConfig_DeniesRestrictedCommand(t *testing.T) {
	input := `
bgp {
    peer loopback {
        connection {
            remote {
                ip 127.0.0.1
            }
            local {
                ip 127.0.0.1
            }
        }
        session {
            asn {
                local 65533
                remote 65533
            }
        }
    }
}

system {
    authentication {
        user operator {
            password "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012"
            profile [ restricted ]
        }
    }
    authorization {
        profile restricted {
            run {
                default-action deny
                entry 10 {
                    action allow
                    match "peer show"
                }
            }
            edit {
                default-action deny
            }
        }
    }
}
`
	tree, err := parseTreeWithYANG(input, nil)
	require.NoError(t, err)

	store := extractAuthzConfig(tree)
	require.NotNil(t, store)

	// Operator can run "peer show" (allowed by entry 10)
	assert.Equal(t, authz.Allow, store.Authorize("operator", "peer show", true),
		"operator should be allowed to run 'peer show'")

	// Operator cannot run "restart" (no matching entry, default deny)
	assert.Equal(t, authz.Deny, store.Authorize("operator", "restart", true),
		"operator should be denied 'restart' (default deny)")

	// Operator cannot edit anything (edit section default deny)
	assert.Equal(t, authz.Deny, store.Authorize("operator", "peer set", false),
		"operator should be denied edit commands")

	// Unknown user (no assignment) gets admin default (allow all)
	assert.Equal(t, authz.Allow, store.Authorize("unknown", "restart", true),
		"unassigned user should get admin default (allow)")
}

// TestExtractAuthzConfig_AdminAllowsAll verifies the admin profile allows everything.
//
// VALIDATES: Admin profile with default-action allow works end-to-end from config.
// PREVENTS: Admin locked out by misconfigured extraction.
func TestExtractAuthzConfig_AdminAllowsAll(t *testing.T) {
	input := `
bgp {
    peer loopback {
        connection {
            remote {
                ip 127.0.0.1
            }
            local {
                ip 127.0.0.1
            }
        }
        session {
            asn {
                local 65533
                remote 65533
            }
        }
    }
}

system {
    authentication {
        user boss {
            password "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012"
            profile [ admin ]
        }
    }
    authorization {
        profile admin {
            run {
                default-action allow
            }
            edit {
                default-action allow
            }
        }
    }
}
`
	tree, err := parseTreeWithYANG(input, nil)
	require.NoError(t, err)

	store := extractAuthzConfig(tree)
	require.NotNil(t, store)

	assert.Equal(t, authz.Allow, store.Authorize("boss", "restart", true))
	assert.Equal(t, authz.Allow, store.Authorize("boss", "peer set something", false))
}

// TestExtractAuthzConfig_EntryOrder verifies entries are sorted by number
// regardless of map iteration order.
//
// VALIDATES: extractAuthzSection sorts entries by number ascending.
// PREVENTS: First-match evaluation depends on random map order.
func TestExtractAuthzConfig_EntryOrder(t *testing.T) {
	input := `
bgp {
    peer loopback {
        connection {
            remote {
                ip 127.0.0.1
            }
            local {
                ip 127.0.0.1
            }
        }
        session {
            asn {
                local 65533
                remote 65533
            }
        }
    }
}

system {
    authentication {
        user tester {
            password "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012"
            profile [ ordered ]
        }
    }
    authorization {
        profile ordered {
            run {
                default-action deny
                entry 30 {
                    action deny
                    match "peer"
                }
                entry 10 {
                    action allow
                    match "peer show"
                }
            }
            edit {
                default-action deny
            }
        }
    }
}
`
	tree, err := parseTreeWithYANG(input, nil)
	require.NoError(t, err)

	store := extractAuthzConfig(tree)
	require.NotNil(t, store)

	// Entry 10 (allow "peer show") comes before entry 30 (deny "peer").
	// First match wins, so "peer show" should be allowed.
	assert.Equal(t, authz.Allow, store.Authorize("tester", "peer show", true),
		"entry 10 (allow) should match before entry 30 (deny)")

	// "peer restart" matches entry 30 (deny "peer") — denied.
	assert.Equal(t, authz.Deny, store.Authorize("tester", "peer restart", true),
		"entry 30 (deny 'peer') should deny 'peer restart'")
}

// TestValidateAuthzConfig_UndefinedProfileReference verifies that referencing
// a non-existent profile in user config produces an error.
//
// VALIDATES: AC-8 — config referencing non-existent profile returns error.
// PREVENTS: User silently assigned non-existent profile → falls to admin default.
func TestValidateAuthzConfig_UndefinedProfileReference(t *testing.T) {
	input := `
bgp {
    peer loopback {
        connection {
            remote {
                ip 127.0.0.1
            }
            local {
                ip 127.0.0.1
            }
        }
        session {
            asn {
                local 65533
                remote 65533
            }
        }
    }
}

system {
    authentication {
        user operator {
            password "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012"
            profile [ nonexistent ]
        }
    }
    authorization {
        profile restricted {
            run {
                default-action deny
            }
            edit {
                default-action deny
            }
        }
    }
}
`
	tree, err := parseTreeWithYANG(input, nil)
	require.NoError(t, err)

	err = ValidateAuthzConfig(tree)
	require.Error(t, err, "referencing undefined profile should produce error")
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Contains(t, err.Error(), "operator")
}

// TestValidateAuthzConfig_ValidReferences verifies no error for valid profile references.
//
// VALIDATES: AC-8 — valid profile references pass validation.
// PREVENTS: False positives rejecting valid configs.
func TestValidateAuthzConfig_ValidReferences(t *testing.T) {
	input := `
bgp {
    peer loopback {
        connection {
            remote {
                ip 127.0.0.1
            }
            local {
                ip 127.0.0.1
            }
        }
        session {
            asn {
                local 65533
                remote 65533
            }
        }
    }
}

system {
    authentication {
        user operator {
            password "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012"
            profile [ restricted ]
        }
    }
    authorization {
        profile restricted {
            run {
                default-action deny
            }
            edit {
                default-action deny
            }
        }
    }
}
`
	tree, err := parseTreeWithYANG(input, nil)
	require.NoError(t, err)

	err = ValidateAuthzConfig(tree)
	assert.NoError(t, err, "valid profile reference should not produce error")
}

// TestValidateAuthzConfig_InvalidRegex verifies that an invalid regex in a profile
// entry produces a hard error at config validation time.
//
// VALIDATES: Spec line 284 — reject if regex flag set and match is invalid regex.
// PREVENTS: Invalid regex silently skipped — profile dropped without error.
func TestValidateAuthzConfig_InvalidRegex(t *testing.T) {
	input := `
bgp {
    peer loopback {
        connection {
            remote {
                ip 127.0.0.1
            }
            local {
                ip 127.0.0.1
            }
        }
        session {
            asn {
                local 65533
                remote 65533
            }
        }
    }
}

system {
    authorization {
        profile broken {
            run {
                default-action deny
                entry 10 {
                    action allow
                    match "[invalid"
                    regex true
                }
            }
            edit {
                default-action deny
            }
        }
    }
}
`
	tree, err := parseTreeWithYANG(input, nil)
	require.NoError(t, err)

	err = ValidateAuthzConfig(tree)
	require.Error(t, err, "invalid regex should produce config error")
	assert.Contains(t, err.Error(), "invalid regex")
}
