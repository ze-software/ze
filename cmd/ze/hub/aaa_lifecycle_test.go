// Design: docs/architecture/aaa-tacacs.md -- typed command authorization contract
package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
)

// chainDenyingStoreForTest answers a store whose "operator" profile runs every
// command except the piped line `show rows | count`, a prefix entry. Only an
// authorizer handed the chain tokens can tell that line from the bare command.
func chainDenyingStoreForTest() *authz.Store {
	run := authz.Section{Default: authz.Allow}
	run.Append(authz.Deny, "show rows | count", false)
	store := authz.NewStore()
	store.AddProfile(authz.Profile{Name: "operator", Run: run, Edit: authz.Section{Default: authz.Allow}})
	store.AssignProfiles("alice", []string{"operator"})
	return store
}

// chainTokens is how the dispatcher hands a typed authorizer the line
// `show rows | count`: the command, then the chain as tokens (T-6 A of
// spec-pipe-chain-honored-on-every-command-surface).
var chainTokens = []string{"rows", "|", "count"}

// TestTypedWrappersPassChainTokens proves each of the six typed wrappers in
// front of the AAA backends hands a typed authorizer the chain tokens
// unchanged, and a store judging the canonical string sees the chain with the
// argument boundaries kept, so a per-command policy on a piped line holds.
//
// Method: the three store-backed wrappers judge chainDenyingStoreForTest,
// which denies the piped line and allows the bare command; the three that
// front the bundle hand the tokens to a typed bundle authorizer that records
// them.
//
// VALIDATES: AC-27 under T-6 A.
// PREVENTS: a wrapper dropping or flattening the chain, so a policy denying a
// piped line allows it.
func TestTypedWrappersPassChainTokens(t *testing.T) {
	t.Run("store-backed", func(t *testing.T) {
		resetAAABundleForTest(t)
		store := chainDenyingStoreForTest()
		publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, store, nil, ""))

		wrappers := map[string]aaa.CommandArgsAuthorizer{
			"liveLocalAuthorizer":                    liveLocalAuthorizer{},
			"liveLocalProfileAuthorizer":             liveLocalProfileAuthorizer{profiles: []string{"operator"}},
			"acceptedLocalGenerationProfileFallback": acceptedLocalGenerationProfileFallback{store: store, profiles: []string{"operator"}},
		}
		for name, wrapper := range wrappers {
			assert.False(t, wrapper.AuthorizeCommandArgs("alice", "", "show", chainTokens, "*", true),
				"%s: the policy denying the piped line sees its chain", name)
			assert.True(t, wrapper.AuthorizeCommandArgs("alice", "", "show", []string{"rows"}, "*", true),
				"%s: the bare command stays allowed", name)
		}
	})

	t.Run("bundle-fronting", func(t *testing.T) {
		resetAAABundleForTest(t)
		remote := &typedBundleAuthorizer{allow: true}
		swapAAABundle(&aaa.Bundle{Authorizer: remote}, nil)

		wrappers := map[string]aaa.CommandArgsAuthorizer{
			"liveAAABundleAuthorizer":           liveAAABundleAuthorizer{},
			"liveAAABundleProfileAuthorizer":    liveAAABundleProfileAuthorizer{profiles: []string{"operator"}},
			"acceptedLocalGenerationAuthorizer": acceptedLocalGenerationAuthorizer{store: nil},
		}
		for name, wrapper := range wrappers {
			remote.args = nil
			remote.typedCalled = false
			assert.True(t, wrapper.AuthorizeCommandArgs("alice", "", "show", chainTokens, "*", true), name)
			assert.True(t, remote.typedCalled, "%s: the typed backend is asked", name)
			assert.Equal(t, chainTokens, remote.args, "%s: the chain tokens pass unchanged", name)
		}
	})
}
