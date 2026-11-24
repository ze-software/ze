//go:build ze_bgp

package infra_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/core/opererr"

	// The bgp block parses only with the gated BGP schema, so the file carries
	// the ze_bgp tag. plugin/all does not link bgp/config, whose init() fills
	// the ResolveBGPTree seam: the ze binary links it from its own gated root
	// (cmd/ze/infra_bgp.go), and config/cli's all_import_test.go does the same.
	_ "github.com/ze-software/ze/internal/component/bgp/config"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
)

// These tests are external (package infra_test): they need plugin/all for the
// compiled-in YANG schema and bgp/config for the BGP engine's resolver.

const (
	diffHubSecretOld = "hub-secret-old-0123456789abcdefghijkl"
	diffHubSecretNew = "hub-secret-new-0123456789abcdefghijkl"
	diffMD5Old       = "md5-group-old"
	diffMD5New       = "md5-group-new"
)

// diffBGP is a bgp block with one peer whose remote AS is remoteAS.
func diffBGP(routerID, remoteAS string) string {
	return "bgp {\n\trouter-id " + routerID + "\n\tsession { asn { local 65000; } }\n" +
		"\tpeer peer1 {\n\t\tconnection { remote { ip 10.0.0.1; } local { ip 10.0.0.254; } }\n" +
		"\t\tsession { asn { remote " + remoteAS + "; } }\n\t}\n}\n"
}

// diffHub is a hub block with one client edge-01 at host, holding secret.
func diffHub(host, secret string) string {
	return "bus {\n\thub {\n\t\tclient edge-01 {\n\t\t\thost " + host + "\n\t\t\tport 1790\n" +
		"\t\t\tsecret " + secret + "\n\t\t}\n\t}\n}\n"
}

// wholeTreeDiff parses both texts with the compiled-in schema and compares them
// through the producer every diff command uses.
func wholeTreeDiff(t *testing.T, before, after string) *config.ConfigDiff {
	t.Helper()
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	side := func(text string) *infra.DiffSide {
		tree, parseErr := config.NewParser(schema).Parse(text)
		require.NoError(t, parseErr)
		resolved, sideErr := infra.NewDiffSide(tree)
		require.NoError(t, sideErr)
		return resolved
	}
	diff, err := infra.DiffConfigs(schema, side(before), side(after))
	require.NoError(t, err)
	return diff
}

// TestWholeTreeDiffNonBGPRoot proves a change outside bgp is a diff entry.
//
// VALIDATES: AC-22: two configurations that differ only in a hub client's host
// answer one changed entry at the path the operator wrote, and a change count
// of one.
// PREVENTS: the BGP-only comparison, which answered "No differences" for this
// pair (spec-fleet-7 E-3).
func TestWholeTreeDiffNonBGPRoot(t *testing.T) {
	diff := wholeTreeDiff(t,
		diffBGP("1.1.1.1", "65001")+diffHub("10.0.0.1", diffHubSecretOld),
		diffBGP("1.1.1.1", "65001")+diffHub("10.0.0.2", diffHubSecretOld))

	assert.Empty(t, diff.Added)
	assert.Empty(t, diff.Removed)
	assert.Equal(t, map[string]config.DiffPair{
		"bus/hub/client/edge-01/host": {Old: "10.0.0.1", New: "10.0.0.2"},
	}, diff.Changed)
	assert.Equal(t, 1, infra.DiffChangeCount(diff))
}

// TestWholeTreeDiffMasksSecrets proves a secret outside bgp is reported and
// never shown.
//
// VALIDATES: a rotated hub client secret is a changed entry with the
// placeholder on both sides, and an added client answers one entry per leaf,
// its secret masked.
// PREVENTS: the whole-tree comparison printing a hub secret that the BGP-only
// comparison never reached; an added client printed as one Go map.
func TestWholeTreeDiffMasksSecrets(t *testing.T) {
	t.Run("rotated", func(t *testing.T) {
		diff := wholeTreeDiff(t, diffHub("10.0.0.1", diffHubSecretOld), diffHub("10.0.0.1", diffHubSecretNew))

		assert.Equal(t, map[string]config.DiffPair{
			"bus/hub/client/edge-01/secret": {Old: config.SecretDataPlaceholder, New: config.SecretDataPlaceholder},
		}, diff.Changed)
		assert.Equal(t, 1, infra.DiffChangeCount(diff))
	})
	t.Run("added", func(t *testing.T) {
		diff := wholeTreeDiff(t, diffBGP("1.1.1.1", "65001"),
			diffBGP("1.1.1.1", "65001")+diffHub("10.0.0.1", diffHubSecretNew))

		assert.Empty(t, diff.Removed)
		assert.Empty(t, diff.Changed)
		assert.Equal(t, map[string]any{
			"bus/hub/client/edge-01/host":   "10.0.0.1",
			"bus/hub/client/edge-01/port":   "1790",
			"bus/hub/client/edge-01/secret": config.SecretDataPlaceholder,
		}, diff.Added)
		assert.NotContains(t, fmt.Sprint(diff.Added), diffHubSecretNew)
	})
}

// TestWholeTreeDiffKeepsBGPPaths proves bgp is still compared resolved, at the
// paths every existing `ze config diff` expectation reads.
//
// VALIDATES: a router-id change is `router-id` and a peer's remote AS is
// `peer/peer1/session/asn/remote`, with no `bgp/` prefix and no second entry
// for the same change from the bgp block as written.
// PREVENTS: the whole-tree comparison renaming or doubling every BGP entry.
func TestWholeTreeDiffKeepsBGPPaths(t *testing.T) {
	diff := wholeTreeDiff(t, diffBGP("1.1.1.1", "65001"), diffBGP("2.2.2.2", "65002"))

	assert.Empty(t, diff.Added)
	assert.Empty(t, diff.Removed)
	assert.Equal(t, config.DiffPair{Old: "1.1.1.1", New: "2.2.2.2"}, diff.Changed["router-id"])
	assert.Equal(t, config.DiffPair{Old: "65001", New: "65002"}, diff.Changed["peer/peer1/session/asn/remote"])
	for path := range diff.Changed {
		assert.NotContains(t, path, "bgp/", "a bgp entry carries its root: %s", path)
	}
}

// diffDynamicGroup is a bgp block whose dynamic group ix signs its sessions
// with password.
func diffDynamicGroup(password string) string {
	return "bgp {\n\trouter-id 1.1.1.1\n\tsession { asn { local 65000; } }\n\tgroup ix {\n" +
		"\t\tconnection {\n\t\t\tremote { ip dynamic; connect false; range 10.0.0.0/8; }\n" +
		"\t\t\tlocal { ip 10.0.0.1; accept true; }\n\t\t\tmd5 { password " + password + "; }\n\t\t}\n" +
		"\t\tsession { asn { remote 65001; } }\n\t}\n}\n"
}

// TestWholeTreeDiffMasksDynamicGroupSecret proves the resolved dynamic groups
// carry no secret into the diff.
//
// VALIDATES: a rotated md5 password of a dynamic group reports a change and
// shows neither password.
// PREVENTS: the entry ResolveBGPTree answers for dynamic groups, a list of
// structs whose Template map holds the group's values, reaching the output
// unmasked: the name rule masks maps and named leaves, and reached neither.
func TestWholeTreeDiffMasksDynamicGroupSecret(t *testing.T) {
	diff := wholeTreeDiff(t, diffDynamicGroup(diffMD5Old), diffDynamicGroup(diffMD5New))

	require.NotZero(t, infra.DiffChangeCount(diff), "the rotation reported no change, so this proves nothing")
	rendered := fmt.Sprint(diff.Added, diff.Removed, diff.Changed)
	assert.NotContains(t, rendered, diffMD5Old)
	assert.NotContains(t, rendered, diffMD5New)
}

// TestWholeTreeDiffRefusesSharedPath proves a root named like a resolved bgp
// setting is refused rather than answered with one value hiding the other.
//
// VALIDATES: a root `router-id` beside a bgp block with a router-id is an
// operator error naming the root.
// PREVENTS: an external plugin root that shares a name with a bgp setting
// overwriting that setting's diff entry in silence.
func TestWholeTreeDiffRefusesSharedPath(t *testing.T) {
	build := func(value string) *infra.DiffSide {
		root, bgp, clash := config.NewTree(), config.NewTree(), config.NewTree()
		bgp.Set("router-id", "1.1.1.1")
		clash.Set("value", value)
		root.SetContainer("bgp", bgp)
		root.SetContainer("router-id", clash)
		side, err := infra.NewDiffSide(root)
		require.NoError(t, err)
		return side
	}

	_, err := infra.DiffConfigs(config.NewSchema(), build("a"), build("b"))
	require.Error(t, err)
	_, isOperator := errors.AsType[*opererr.Error](err)
	assert.True(t, isOperator, "the refusal is not an operator error: %v", err)
	assert.Contains(t, err.Error(), `"router-id"`)
}
