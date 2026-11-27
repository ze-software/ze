// Design: docs/architecture/core-design.md -- explicit default-deny RBAC authorization
// Related: authz.go -- Store.AuthorizeExplicit, Store.authorizeExplicitWithProfiles
// Related: register.go -- StoreAuthorizer.AuthorizeExplicit, boundStoreAuthorizer.AuthorizeExplicit

package authz

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/aaa"
)

// VALIDATES: Store.AuthorizeExplicit, Store.authorizeExplicitWithProfiles,
// StoreAuthorizer.AuthorizeExplicit, and boundStoreAuthorizer.AuthorizeExplicit
// enforce default-deny explicit entry matching: rejecting nil stores, empty
// usernames, default-action allow, read-only, reserved/break-glass identities,
// broad prefixes, wildcard regexes, and plugins without explicit compiled bus
// grants while permitting exact canonical paths, per-certificate paths,
// targeted regexes, explicit plugin bus entries, and preserving deny-before-allow
// ordering.
// PREVENTS: private-key export or other RequireExplicitAuthz commands being
// granted implicitly via unconfigured RBAC, default-action allow, break-glass
// recovery, broad prefixes, wildcard regexes, or unprivileged plugins.
func TestAuthorizeExplicitDefaultDenyAndExplicitGrants(t *testing.T) {
	const (
		canonicalPath = "show pki certificate name bundle pem"
		dev1Typed     = "show pki certificate name dev-1 bundle pem confirm"
		dev2Typed     = "show pki certificate name dev-2 bundle pem confirm"
		dev10Typed    = "show pki certificate name dev-10 bundle pem confirm"
	)
	dev1Tokens := []string{"show", "pki", "certificate", "name", "dev-1", "bundle", "pem", "confirm"}
	dev2Tokens := []string{"show", "pki", "certificate", "name", "dev-2", "bundle", "pem", "confirm"}

	t.Run("nil store and unconfigured RBAC fail closed for non-plugin identities", func(t *testing.T) {
		var nilStore *Store
		assert.Equal(t, Deny, nilStore.AuthorizeExplicit("exporter", canonicalPath, dev1Typed, false))
		assert.Equal(t, Deny, nilStore.authorizeExplicitWithProfiles("exporter", []string{"admin"}, canonicalPath, dev1Typed, false))

		nilAuth := StoreAuthorizer{Store: nil}
		allowed, supported := aaa.AuthorizeExplicit(nilAuth, "exporter", "192.0.2.1:22", canonicalPath, dev1Tokens, false)
		assert.True(t, supported)
		assert.False(t, allowed)

		allowed, supported = aaa.AuthorizeExplicit(nilAuth.BindProfiles([]string{aaa.ReservedRecoveryProfile}), "root", "192.0.2.1:22", canonicalPath, dev1Tokens, false)
		assert.True(t, supported)
		assert.False(t, allowed)

		allowed, supported = aaa.AuthorizeExplicit(nilAuth, aaa.ReservedInternalRPCIdentity, "", canonicalPath, dev1Tokens, false)
		assert.True(t, supported)
		assert.False(t, allowed)

		allowed, supported = aaa.AuthorizeExplicit(nilAuth, aaa.ReservedSharedAPIUsername, "192.0.2.1:443", canonicalPath, dev1Tokens, false)
		assert.True(t, supported)
		assert.False(t, allowed)
	})

	t.Run("default-action allow, read-only, empty user, and reserved identities deny", func(t *testing.T) {
		store := NewStore()
		store.AddProfile(builtinAdminProfile())
		store.AddProfile(builtinReadOnlyProfile())
		store.AssignProfiles("admin-user", []string{"admin"})
		store.AssignProfiles("ro-user", []string{"read-only"})

		assert.Equal(t, Deny, store.AuthorizeExplicit("", canonicalPath, dev1Typed, false))
		assert.Equal(t, Deny, store.AuthorizeExplicit("admin-user", canonicalPath, dev1Typed, false))
		assert.Equal(t, Deny, store.AuthorizeExplicit("ro-user", canonicalPath, dev1Typed, false))
		assert.Equal(t, Deny, store.AuthorizeExplicit("unassigned", canonicalPath, dev1Typed, false))
		assert.Equal(t, Deny, store.AuthorizeExplicit(aaa.ReservedInternalRPCIdentity, canonicalPath, dev1Typed, false))
		assert.Equal(t, Deny, store.AuthorizeExplicit(aaa.ReservedSharedAPIUsername, canonicalPath, dev1Typed, false))
		assert.Equal(t, Deny, store.authorizeExplicitWithProfiles("bootstrap-admin", []string{aaa.ReservedRecoveryProfile}, canonicalPath, dev1Typed, false))
		assert.Equal(t, Deny, store.authorizeExplicitWithProfiles("admin-user", []string{"admin"}, canonicalPath, dev1Typed, false))

		// Empty canonical path or typed command fails closed.
		assert.Equal(t, Deny, store.AuthorizeExplicit("admin-user", "", dev1Typed, false))
		assert.Equal(t, Deny, store.AuthorizeExplicit("admin-user", canonicalPath, "", false))
	})

	t.Run("broad prefixes and wildcard regexes deny", func(t *testing.T) {
		broadMatches := []struct {
			name  string
			entry Entry
		}{
			{name: "prefix show", entry: Entry{Number: 10, Action: Allow, Match: "show"}},
			{name: "prefix show pki", entry: Entry{Number: 10, Action: Allow, Match: "show pki"}},
			{name: "prefix show pki certificate", entry: Entry{Number: 10, Action: Allow, Match: "show pki certificate"}},
			{name: "prefix show pki certificate name", entry: Entry{Number: 10, Action: Allow, Match: "show pki certificate name"}},
			{name: "prefix show pki certificate name dev-1", entry: Entry{Number: 10, Action: Allow, Match: "show pki certificate name dev-1"}},
			{name: "prefix show pki certificate name dev-1 bundle", entry: Entry{Number: 10, Action: Allow, Match: "show pki certificate name dev-1 bundle"}},
			{name: "wildcard regex dot-star", entry: Entry{Number: 10, Action: Allow, Match: ".*", Regex: true}},
			{name: "wildcard regex dot-plus", entry: Entry{Number: 10, Action: Allow, Match: ".+", Regex: true}},
			{name: "broad regex show", entry: Entry{Number: 10, Action: Allow, Match: "^show.*", Regex: true}},
			{name: "broad regex show pki", entry: Entry{Number: 10, Action: Allow, Match: "^show pki.*", Regex: true}},
			{name: "broad regex cert name", entry: Entry{Number: 10, Action: Allow, Match: "^show pki certificate name .*", Regex: true}},
			{name: "broad regex dev-1 wildcard", entry: Entry{Number: 10, Action: Allow, Match: "^show pki certificate name dev-1 .*", Regex: true}},
			{name: "broad regex skipping bundle", entry: Entry{Number: 10, Action: Allow, Match: "^show pki certificate name .* pem", Regex: true}},
			{name: "unanchored regex pem", entry: Entry{Number: 10, Action: Allow, Match: "pem", Regex: true}},
			{name: "unanchored regex confirm", entry: Entry{Number: 10, Action: Allow, Match: "confirm", Regex: true}},
			{name: "broad regex skipping middle canonical token", entry: Entry{Number: 10, Action: Allow, Match: "^show .* pem$", Regex: true}},
		}

		for _, tc := range broadMatches {
			t.Run(tc.name, func(t *testing.T) {
				store := NewStore()
				p := Profile{
					Name: "broad",
					Run:  Section{Default: Allow, Entries: []Entry{tc.entry}},
					Edit: Section{Default: Allow, Entries: []Entry{tc.entry}},
				}
				assert.NoError(t, p.Validate())
				store.AddProfile(p)
				store.AssignProfiles("alice", []string{"broad"})

				assert.Equal(t, Deny, store.AuthorizeExplicit("alice", canonicalPath, dev1Typed, false))
				assert.Equal(t, Deny, store.authorizeExplicitWithProfiles("alice", []string{"broad"}, canonicalPath, dev1Typed, false))
			})
		}
	})

	t.Run("exact canonical path grants all certificates in edit section only", func(t *testing.T) {
		store := NewStore()
		store.AddProfile(Profile{
			Name: "pki-exporter",
			Run:  Section{Default: Deny},
			Edit: Section{
				Default: Deny,
				Entries: []Entry{
					{Number: 10, Action: Allow, Match: canonicalPath},
				},
			},
		})
		store.AddProfile(Profile{
			Name: "run-only-grant",
			Run: Section{
				Default: Deny,
				Entries: []Entry{
					{Number: 10, Action: Allow, Match: canonicalPath},
				},
			},
			Edit: Section{Default: Deny},
		})
		store.AssignProfiles("exporter", []string{"pki-exporter"})
		store.AssignProfiles("run-only", []string{"run-only-grant"})

		assert.Equal(t, Allow, store.AuthorizeExplicit("exporter", canonicalPath, dev1Typed, false))
		assert.Equal(t, Allow, store.AuthorizeExplicit("exporter", canonicalPath, dev2Typed, false))
		// Edit-section grant does not apply when isReadOnly is true, and Run-section grant does not satisfy isReadOnly=false.
		assert.Equal(t, Deny, store.AuthorizeExplicit("exporter", canonicalPath, dev1Typed, true))
		assert.Equal(t, Deny, store.AuthorizeExplicit("run-only", canonicalPath, dev1Typed, false))

		// Through StoreAuthorizer and boundStoreAuthorizer.
		sa := StoreAuthorizer{Store: store}
		allowed, supported := aaa.AuthorizeExplicit(sa, "exporter", "192.0.2.2:22", canonicalPath, dev1Tokens, false)
		assert.True(t, supported)
		assert.True(t, allowed)

		bound := sa.BindProfiles([]string{"pki-exporter"})
		allowed, supported = aaa.AuthorizeExplicit(bound, "remote-user", "192.0.2.2:22", canonicalPath, dev2Tokens, false)
		assert.True(t, supported)
		assert.True(t, allowed)

		emptyBound := sa.BindProfiles(nil)
		allowed, supported = aaa.AuthorizeExplicit(emptyBound, "exporter", "192.0.2.2:22", canonicalPath, dev1Tokens, false)
		assert.True(t, supported)
		assert.False(t, allowed)

		// Recovery profile paired with admin (default-allow) still denies, while paired with explicit profile uses only the explicit profile.
		assert.Equal(t, Deny, store.authorizeExplicitWithProfiles("bootstrap-admin", []string{aaa.ReservedRecoveryProfile}, canonicalPath, dev1Typed, false))
		assert.Equal(t, Allow, store.authorizeExplicitWithProfiles("bootstrap-admin", []string{aaa.ReservedRecoveryProfile, "pki-exporter"}, canonicalPath, dev1Typed, false))
	})

	t.Run("per-certificate explicit grant restricts to named certificate", func(t *testing.T) {
		store := NewStore()
		store.AddProfile(Profile{
			Name: "dev1-exporter",
			Edit: Section{
				Default: Deny,
				Entries: []Entry{
					{Number: 10, Action: Allow, Match: "show pki certificate name dev-1 bundle pem"},
				},
			},
		})
		store.AssignProfiles("dev1-op", []string{"dev1-exporter"})

		assert.Equal(t, Allow, store.AuthorizeExplicit("dev1-op", canonicalPath, dev1Typed, false))
		assert.Equal(t, Deny, store.AuthorizeExplicit("dev1-op", canonicalPath, dev2Typed, false))
		assert.Equal(t, Deny, store.AuthorizeExplicit("dev1-op", canonicalPath, dev10Typed, false))
	})

	t.Run("targeted regex allows matching certs and denies others", func(t *testing.T) {
		store := NewStore()
		p := Profile{
			Name: "regex-exporter",
			Edit: Section{
				Default: Deny,
				Entries: []Entry{
					{Number: 10, Action: Allow, Match: `^show pki certificate name dev-[12] bundle pem( confirm)?$`, Regex: true},
				},
			},
		}
		assert.NoError(t, p.Validate())
		store.AddProfile(p)
		store.AssignProfiles("regex-op", []string{"regex-exporter"})

		assert.Equal(t, Allow, store.AuthorizeExplicit("regex-op", canonicalPath, dev1Typed, false))
		assert.Equal(t, Allow, store.AuthorizeExplicit("regex-op", canonicalPath, dev2Typed, false))
		assert.Equal(t, Deny, store.AuthorizeExplicit("regex-op", canonicalPath, dev10Typed, false))
	})

	t.Run("deny-before-allow ordering holds for both specific and canonical denies", func(t *testing.T) {
		store := NewStore()
		store.AddProfile(Profile{
			Name: "deny-dev1-allow-rest",
			Edit: Section{
				Default: Deny,
				Entries: []Entry{
					{Number: 10, Action: Deny, Match: "show pki certificate name dev-1 bundle pem"},
					{Number: 20, Action: Allow, Match: canonicalPath},
				},
			},
		})
		store.AddProfile(Profile{
			Name: "deny-canonical-before-specific",
			Edit: Section{
				Default: Deny,
				Entries: []Entry{
					{Number: 10, Action: Deny, Match: canonicalPath},
					{Number: 20, Action: Allow, Match: "show pki certificate name dev-1 bundle pem"},
				},
			},
		})
		store.AddProfile(Profile{
			Name: "deny-prefix-before-allow",
			Edit: Section{
				Default: Deny,
				Entries: []Entry{
					{Number: 10, Action: Deny, Match: "show pki"},
					{Number: 20, Action: Allow, Match: canonicalPath},
				},
			},
		})
		store.AssignProfiles("op-rest", []string{"deny-dev1-allow-rest"})
		store.AssignProfiles("op-canon-deny", []string{"deny-canonical-before-specific"})
		store.AssignProfiles("op-prefix-deny", []string{"deny-prefix-before-allow"})

		assert.Equal(t, Deny, store.AuthorizeExplicit("op-rest", canonicalPath, dev1Typed, false))
		assert.Equal(t, Allow, store.AuthorizeExplicit("op-rest", canonicalPath, dev2Typed, false))
		assert.Equal(t, Deny, store.AuthorizeExplicit("op-canon-deny", canonicalPath, dev1Typed, false))
		assert.Equal(t, Deny, store.AuthorizeExplicit("op-prefix-deny", canonicalPath, dev1Typed, false))
	})

	t.Run("plugin identities evaluate compiled bus profiles under explicit-entry rules", func(t *testing.T) {
		profiles := map[string]aaa.PluginCommandProfile{
			"none-plugin": {
				Endpoint:  "none-plugin",
				AllowNone: true,
			},
			"broad-plugin": {
				Endpoint:        "broad-plugin",
				AllowedPrefixes: []string{"show", "show pki", "show pki certificate name"},
			},
			"explicit-plugin": {
				Endpoint:        "explicit-plugin",
				AllowedPrefixes: []string{canonicalPath},
			},
			"dev1-plugin": {
				Endpoint:        "dev1-plugin",
				AllowedPrefixes: []string{"show pki certificate name dev-1 bundle pem"},
			},
		}
		aaa.SetPluginCommandProfileLookup(func(endpoint string) (aaa.PluginCommandProfile, bool) {
			prof, ok := profiles[endpoint]
			return prof, ok
		})
		t.Cleanup(func() { aaa.SetPluginCommandProfileLookup(nil) })

		store := NewStore()
		store.AddProfile(builtinAdminProfile())

		for _, auth := range []aaa.Authorizer{
			StoreAuthorizer{Store: store},
			StoreAuthorizer{Store: nil},
		} {
			// Absent profile, AllowNone, malformed empty endpoint, and broad prefixes all deny.
			allowed, _ := aaa.AuthorizeExplicit(auth, aaa.ReservedInternalPluginPrefix+"unknown", "", canonicalPath, dev1Tokens, false)
			assert.False(t, allowed, "plugin with no bus profile must deny")

			allowed, _ = aaa.AuthorizeExplicit(auth, aaa.ReservedInternalPluginPrefix+"none-plugin", "", canonicalPath, dev1Tokens, false)
			assert.False(t, allowed, "plugin with AllowNone must deny")

			allowed, _ = aaa.AuthorizeExplicit(auth, aaa.ReservedInternalPluginPrefix, "", canonicalPath, dev1Tokens, false)
			assert.False(t, allowed, "malformed empty plugin identity must deny")

			allowed, _ = aaa.AuthorizeExplicit(auth, aaa.ReservedInternalPluginPrefix+"broad-plugin", "", canonicalPath, dev1Tokens, false)
			assert.False(t, allowed, "plugin with only broad prefixes must deny")

			// Explicit canonical or per-certificate bus grant allows matching commands only.
			allowed, _ = aaa.AuthorizeExplicit(auth, aaa.ReservedInternalPluginPrefix+"explicit-plugin", "", canonicalPath, dev1Tokens, false)
			assert.True(t, allowed, "plugin with explicit canonical bus grant must allow")

			allowed, _ = aaa.AuthorizeExplicit(auth, aaa.ReservedInternalPluginPrefix+"dev1-plugin", "", canonicalPath, dev1Tokens, false)
			assert.True(t, allowed, "plugin with explicit dev-1 bus grant must allow dev-1")

			allowed, _ = aaa.AuthorizeExplicit(auth, aaa.ReservedInternalPluginPrefix+"dev1-plugin", "", canonicalPath, dev2Tokens, false)
			assert.False(t, allowed, "plugin with explicit dev-1 bus grant must deny dev-2")
		}
	})
}
