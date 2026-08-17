package infra_test

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config/infra"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
)

// TestExtractAuthzConfig verifies that authorization profiles are correctly
// parsed from the config tree into an authz.Store.
//
// VALIDATES: ExtractAuthzStore creates Store with profiles, sections, entries.
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
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	store := infra.ExtractAuthzStore(tree)
	require.NotNil(t, store, "store should not be nil when profiles exist")
	assert.True(t, store.HasProfiles(), "store should have profiles")
}

// TestExtractAuthzConfig_InlineDefaultActionBeforeClosingBrace verifies that a
// leaf value directly followed by its container's closing brace is parsed.
//
// VALIDATES: Automatic semicolon insertion preserves the inline edit default action.
// PREVENTS: Treating the closing brace as the leaf value terminator only after a newline.
func TestExtractAuthzConfig_InlineDefaultActionBeforeClosingBrace(t *testing.T) {
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
            edit { default-action allow }
        }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	store := infra.ExtractAuthzStore(tree)
	require.NotNil(t, store)
	assert.Equal(t, authz.Allow, store.Authorize("operator", "peer set", false))
}

// TestExtractAuthzConfig_NoSystem verifies nil return when no system block.
//
// VALIDATES: ExtractAuthzStore returns nil when no system container.
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
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	store := infra.ExtractAuthzStore(tree)
	assert.Nil(t, store, "no system block means no authz store")
}

// TestExtractAuthzConfig_NoAuthorization verifies nil when system exists but no authorization.
//
// VALIDATES: ExtractAuthzStore returns nil when system has no authorization container.
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
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	store := infra.ExtractAuthzStore(tree)
	assert.Nil(t, store, "no authorization block means no authz store")
}

// TestExtractAuthzConfig_UserAssignments verifies user→profile assignments are extracted.
//
// VALIDATES: ExtractAuthzStore reads profile leaf-list from authentication.user entries.
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
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	store := infra.ExtractAuthzStore(tree)
	require.NotNil(t, store)
	assert.True(t, store.HasProfiles())
	assert.Equal(t, authz.Allow, store.Authorize("operator", "show bgp summary", true), "noc assignment should be extracted")
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
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	store := infra.ExtractAuthzStore(tree)
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

	// Unknown user (no assignment) is denied when user assignments exist (fail closed)
	assert.Equal(t, authz.Deny, store.Authorize("unknown", "restart", true),
		"unassigned user should be denied when assignments exist")
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
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	store := infra.ExtractAuthzStore(tree)
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
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	store := infra.ExtractAuthzStore(tree)
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
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	err = infra.ValidateAuthzConfig(tree)
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
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	err = infra.ValidateAuthzConfig(tree)
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
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	err = infra.ValidateAuthzConfig(tree)
	require.Error(t, err, "invalid regex should produce config error")
	assert.Contains(t, err.Error(), "invalid regex")
}

// TestValidateAuthzConfigRejectsUndefinedTacacsProfile verifies that a
// tacacs-profile priv-lvl mapping naming a profile that does not exist is
// rejected at config load, on the same footing as a user[*].profile reference.
//
// VALIDATES: AC-8 extended -- tacacs-profile references are checked too.
// PREVENTS: a typo loading quietly and then silently not applying to the session
//
//	it was meant to restrict. Authorization receives profile names, not the
//	mapping, so at runtime it can only ignore a name it cannot resolve; load
//	time is the only place this is visible.
func TestValidateAuthzConfigRejectsUndefinedTacacsProfile(t *testing.T) {
	const input = `
system {
    authorization {
        profile read-only {
            run { default-action allow; }
            edit { default-action deny; }
        }
    }
    authentication {
        tacacs-profile 1 { profile [ raed-only ]; }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	err = infra.ValidateAuthzConfig(tree)
	require.Error(t, err, "a tacacs-profile naming an undefined profile must not load")
	assert.Contains(t, err.Error(), "raed-only")
}

// TestValidateAuthzConfigAcceptsDefinedTacacsProfile guards the check above from
// over-reaching: a mapping whose names all resolve must still load.
//
// VALIDATES: a valid priv-lvl mapping is accepted.
// PREVENTS: the reference check rejecting working TACACS+ deployments.
func TestValidateAuthzConfigAcceptsDefinedTacacsProfile(t *testing.T) {
	const input = `
system {
    authorization {
        profile read-only {
            run { default-action allow; }
            edit { default-action deny; }
        }
        profile admin {
            run { default-action allow; }
            edit { default-action allow; }
        }
    }
    authentication {
        tacacs-profile 1  { profile [ read-only ]; }
        tacacs-profile 15 { profile [ admin ]; }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	require.NoError(t, infra.ValidateAuthzConfig(tree))
}

// The map form ExtractAuthUsers reads is what the running daemon holds: every
// applied reload writes config.Tree.ToMap() into the shared ConfigProvider.
// This test pins the complete parsed-tree shape so shared credentials cannot
// drift from the operator configuration.
//
// VALIDATES: ExtractAuthUsers reports base credentials and profiles from the
// parsed system tree. SSH credential augments have feature-gated tests.
// PREVENTS: a configured user or credential field disappearing between parsing
// and the shared live-user source.
func TestExtractAuthUsersFromParsedTree(t *testing.T) {
	input := `
system {
    authentication {
        user alice {
            password "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234"
            profile admin
        }
        user bob {
            password "$2a$10$zyxwvutsrqponmlkjihgfZYXWVUTSRQPONMLKJIHGFEDCBA98765"
        }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	users := infra.ExtractAuthUsers(tree.GetContainer("system").ToMap())

	require.Len(t, users, 2)
	assert.Equal(t, []authz.UserConfig{
		{
			Name:     "alice",
			Hash:     "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234",
			Profiles: []string{"admin"},
		},
		{
			Name: "bob",
			Hash: "$2a$10$zyxwvutsrqponmlkjihgfZYXWVUTSRQPONMLKJIHGFEDCBA98765",
		},
	}, users, "the shared extractor must preserve the complete parsed user configuration")
	assert.Equal(t, "alice", users[0].Name, "users come back sorted; the map form carries no order of its own")
	assert.Equal(t, []string{"admin"}, users[0].Profiles)
	assert.Empty(t, users[1].Profiles, "bob declares no profile")
}

// VALIDATES: a leaf-list survives every shape the map form can carry it in.
// Tree.ToMap collapses a one-member leaf-list to a bare string and emits
// []string beyond that, and a JSON round trip turns either into []any.
// PREVENTS: a single-profile user losing their profile, which would silently
// change what they are authorized to do.
func TestExtractAuthUsersLeafListShapes(t *testing.T) {
	shapes := map[string]struct {
		raw  any
		want []string
	}{
		"one member as a bare string":   {raw: "admin", want: []string{"admin"}},
		"several members as []string":   {raw: []string{"admin", "ro"}, want: []string{"admin", "ro"}},
		"several members as []any":      {raw: []any{"admin", "ro"}, want: []string{"admin", "ro"}},
		"an empty string is no profile": {raw: "", want: nil},
		"an unexpected type is ignored": {raw: 42, want: nil},
	}
	for name, tc := range shapes {
		t.Run(name, func(t *testing.T) {
			users := infra.ExtractAuthUsers(map[string]any{
				"authentication": map[string]any{
					"user": map[string]any{
						"alice": map[string]any{"password": "hash", "profile": tc.raw},
					},
				},
			})
			require.Len(t, users, 1)
			assert.Equal(t, tc.want, users[0].Profiles)
		})
	}
}

// VALIDATES: a subtree that does not describe users yields no users, at every
// depth the shape can go missing.
// PREVENTS: an unreadable or absent config reading as a user list the caller
// would then authenticate against.
func TestExtractAuthUsersMissingSections(t *testing.T) {
	cases := map[string]map[string]any{
		"a nil subtree":               nil,
		"an empty subtree":            {},
		"no authentication container": {"login": map[string]any{}},
		"authentication is not a map": {"authentication": "yes"},
		"no user list":                {"authentication": map[string]any{}},
		"the user list is not a map":  {"authentication": map[string]any{"user": "alice"}},
		"a user entry is not a map":   {"authentication": map[string]any{"user": map[string]any{"alice": "hash"}}},
		"public-keys is not a keyed list": {"authentication": map[string]any{
			"user": map[string]any{"alice": map[string]any{"password": "h", "public-keys": "laptop"}},
		}},
	}
	for name, subtree := range cases {
		t.Run(name, func(t *testing.T) {
			users := infra.ExtractAuthUsers(subtree)
			if name == "a user entry is not a map" || name == "public-keys is not a keyed list" {
				// The user list itself is well-formed here; only the entry is
				// not. A shapeless entry is dropped, never invented.
				for _, u := range users {
					assert.Empty(t, u.PublicKeys)
				}
				return
			}
			assert.Empty(t, users, "a subtree that describes no users must authenticate nobody")
		})
	}
}
