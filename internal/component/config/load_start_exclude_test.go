// VALIDATES: LoadConfigForStart isolates validation failures scoped to
// registered service blocks (fixed paths, longest-match sub-blocks, list-entry
// wildcards, and scoped tree validators) by removing only the failing block
// from the in-memory tree and recording it in ExcludedBlocks, while LoadConfig
// and ReadConfig stay strictly refusing and unscoped failures still refuse start.
// PREVENTS: a single misconfigured service block (such as an undefined
// certificate name) from preventing ze start, sensitive leaf values leaking into
// ExcludedBlock.Cause, or strict validate/commit/reload paths losing enforcement.

package config

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	_ "github.com/ze-software/ze/internal/component/pki/pkiref"
	_ "github.com/ze-software/ze/internal/component/pki/yang"
	_ "github.com/ze-software/ze/internal/plugins/isis/yang"
)

const (
	testBlockWeb           = "environment/web"
	testBlockLG            = "environment/looking-glass"
	testBlockLGRange       = "environment/looking-glass/next-hop-range"
	testBlockHubServerWild = "bus/hub/server[*]"
	testServiceWeb         = "web server"
	testServiceLG          = "looking glass"
	testServiceLGRange     = "looking glass next-hop range names"
	testServiceHubServer   = "managed fleet server"
)

// TestLoadConfigForStartExcludesInvalidServiceBlock proves that an undefined
// ze:reference inside a registered service block drops only that block at start
// while LoadConfig and ReadConfig still refuse the exact same config.
func TestLoadConfigForStartExcludesInvalidServiceBlock(t *testing.T) {
	t.Cleanup(snapshotServiceBlocks())
	require.NoError(t, RegisterServiceBlock(testBlockWeb, testServiceWeb))
	require.NoError(t, RegisterServiceBlock(testBlockLG, testServiceLG))

	const cfg = `environment {
	web {
		enabled true;
		certificate missing-web-cert;
	}
	looking-glass {
		enabled true;
	}
}
`
	cfgPath := filepath.Join(t.TempDir(), "web-bad-cert.conf")

	result, err := LoadConfigForStart(cfg, cfgPath, nil)
	require.NoError(t, err, "LoadConfigForStart must succeed when only a registered service block fails")
	require.NotNil(t, result)
	require.NotNil(t, result.Tree)

	assert.Nil(t, result.Tree.GetContainerPath(testBlockWeb), "failing service block must be dropped from the tree")
	lg := result.Tree.GetContainerPath(testBlockLG)
	require.NotNil(t, lg, "healthy sibling block must survive")
	enabled, ok := lg.Get("enabled")
	assert.True(t, ok)
	assert.Equal(t, "true", enabled)

	require.Len(t, result.ExcludedBlocks, 1)
	assert.Equal(t, testBlockWeb, result.ExcludedBlocks[0].Path)
	assert.Equal(t, testServiceWeb, result.ExcludedBlocks[0].Service)
	require.Error(t, result.ExcludedBlocks[0].Cause)
	assert.Contains(t, result.ExcludedBlocks[0].Cause.Error(), "missing-web-cert")

	// LoadConfig and ReadConfig on the exact same config must stay strictly refusing.
	_, loadErr := LoadConfig(cfg, cfgPath, nil)
	require.Error(t, loadErr)
	assert.True(t, errors.Is(loadErr, ErrCustomValidation), "LoadConfig must fail with ErrCustomValidation: %v", loadErr)

	_, readErr := ReadConfig(cfg, cfgPath, nil)
	require.Error(t, readErr)
	assert.True(t, errors.Is(readErr, ErrCustomValidation), "ReadConfig must fail with ErrCustomValidation: %v", readErr)
}

// TestLoadConfigForStartLongestMatchScoping proves a more specific registered
// sub-block and scoped tree validator drops only the sub-block, keeping the
// parent service block intact.
func TestLoadConfigForStartLongestMatchScoping(t *testing.T) {
	t.Cleanup(snapshotServiceBlocks())
	t.Cleanup(snapshotTreeValidators())

	require.NoError(t, RegisterServiceBlock(testBlockLG, testServiceLG))
	require.NoError(t, RegisterServiceBlock(testBlockLGRange, testServiceLGRange))
	require.NoError(t, RegisterScopedTreeValidator("test-lg-next-hop-range", testBlockLGRange, func(tree *Tree) error {
		lg := tree.GetContainerPath(testBlockLG)
		if lg == nil {
			return nil
		}
		if len(lg.GetList("next-hop-range")) > 0 {
			return errors.New("duplicate next-hop-range entry")
		}
		return nil
	}))

	// Verify longest-match resolution directly as well.
	matchedPath, matchedSvc, ok := matchServiceBlock("environment/looking-glass/next-hop-range[192.0.2.0/24]/name")
	require.True(t, ok)
	assert.Equal(t, testBlockLGRange, matchedPath)
	assert.Equal(t, testServiceLGRange, matchedSvc)

	const cfg = `environment {
	looking-glass {
		enabled true;
		next-hop-range 192.0.2.0/24 {
			name ams-ix;
		}
	}
}
`
	cfgPath := filepath.Join(t.TempDir(), "lg-range-bad.conf")

	result, err := LoadConfigForStart(cfg, cfgPath, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	lg := result.Tree.GetContainerPath(testBlockLG)
	require.NotNil(t, lg, "parent looking-glass container must survive when only next-hop-range fails")
	assert.Empty(t, lg.GetList("next-hop-range"), "failing next-hop-range list must be removed")
	enabled, hasEnabled := lg.Get("enabled")
	assert.True(t, hasEnabled)
	assert.Equal(t, "true", enabled)

	require.Len(t, result.ExcludedBlocks, 1)
	assert.Equal(t, testBlockLGRange, result.ExcludedBlocks[0].Path)
	assert.Equal(t, testServiceLGRange, result.ExcludedBlocks[0].Service)
	require.Error(t, result.ExcludedBlocks[0].Cause)
	assert.Contains(t, result.ExcludedBlocks[0].Cause.Error(), "duplicate next-hop-range entry")

	_, loadErr := LoadConfig(cfg, cfgPath, nil)
	require.Error(t, loadErr)
	assert.True(t, errors.Is(loadErr, ErrCustomValidation))

	_, readErr := ReadConfig(cfg, cfgPath, nil)
	require.Error(t, readErr)
	assert.True(t, errors.Is(readErr, ErrCustomValidation))
}

// TestLoadConfigForStartWildcardListEntryScoping proves a "[*]" service block
// drops only the failing list entry while healthy sibling entries survive.
func TestLoadConfigForStartWildcardListEntryScoping(t *testing.T) {
	t.Cleanup(snapshotServiceBlocks())
	require.NoError(t, RegisterServiceBlock(testBlockHubServerWild, testServiceHubServer))

	const cfg = `bus {
	hub {
		server good {
			ip 127.0.0.1;
			port 1790;
			certificate good-cert;
		}
		server bad {
			ip 127.0.0.1;
			port 1791;
			certificate missing-hub-cert;
		}
	}
}
pki {
	certificate good-cert {
		certificate good-cert-pem;
	}
}
`
	cfgPath := filepath.Join(t.TempDir(), "hub-wildcard.conf")

	result, err := LoadConfigForStart(cfg, cfgPath, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	hub := result.Tree.GetContainerPath("bus/hub")
	require.NotNil(t, hub)
	assert.NotNil(t, hub.GetList("server")["good"], "valid sibling list entry must survive")
	assert.Nil(t, hub.GetList("server")["bad"], "invalid list entry must be dropped")

	require.Len(t, result.ExcludedBlocks, 1)
	assert.Equal(t, "bus/hub/server[bad]", result.ExcludedBlocks[0].Path)
	assert.Equal(t, "managed fleet server bad", result.ExcludedBlocks[0].Service)
	require.Error(t, result.ExcludedBlocks[0].Cause)
	assert.Contains(t, result.ExcludedBlocks[0].Cause.Error(), "missing-hub-cert")

	_, loadErr := LoadConfig(cfg, cfgPath, nil)
	require.Error(t, loadErr)
	assert.True(t, errors.Is(loadErr, ErrCustomValidation))
}

// TestLoadConfigForStartRefusesUnscopedFailure proves failures outside any
// registered service block (both section errors and unscoped tree validators)
// still fail LoadConfigForStart with the original ErrCustomValidation error.
func TestLoadConfigForStartRefusesUnscopedFailure(t *testing.T) {
	t.Cleanup(snapshotServiceBlocks())
	t.Cleanup(snapshotTreeValidators())
	require.NoError(t, RegisterServiceBlock(testBlockWeb, testServiceWeb))

	// Case 1: unscoped section validation error.
	const unscopedSectionCfg = `bus {
	endpoint p1 {
		internal no-such-plugin-here;
	}
}
`
	cfgPath := filepath.Join(t.TempDir(), "unscoped-section.conf")
	_, err := LoadConfigForStart(unscopedSectionCfg, cfgPath, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCustomValidation), "unscoped section error must fail with ErrCustomValidation: %v", err)
	assert.Contains(t, err.Error(), "no-such-plugin-here")

	// Case 2: unscoped original-tree validator failure.
	require.NoError(t, RegisterTreeValidator("test-unscoped-tree-validator", func(*Tree) error {
		return errors.New("global invariant broken")
	}))
	const validCfg = `environment {
	web {
		enabled true;
	}
}
`
	_, treeErr := LoadConfigForStart(validCfg, cfgPath, nil)
	require.Error(t, treeErr)
	assert.True(t, errors.Is(treeErr, ErrCustomValidation))
	assert.Contains(t, treeErr.Error(), "global invariant broken")
}

// TestLoadConfigForStartRedactsSensitiveLeafInExcludedBlock proves that when a
// sensitive leaf fails inside an excluded service block, ExcludedBlock.Cause
// redacts the secret value rather than leaking it into startup logs or doctor.
func TestLoadConfigForStartRedactsSensitiveLeafInExcludedBlock(t *testing.T) {
	t.Cleanup(snapshotServiceBlocks())
	require.NoError(t, RegisterServiceBlock("isis/key-chains[*]", "IS-IS key chain"))
	require.NoError(t, RegisterServiceBlock(testBlockHubServerWild, testServiceHubServer))

	// End-to-end through excludeInvalidServiceBlocks: isis key-chains secret is
	// ze:sensitive with length 1..255. Construct the Tree directly so the check
	// reaches validateCustomSectionsSchema (the parser enforces length earlier).
	const secretMarker = "TOP-SECRET-VALUE-DO-NOT-LOG"
	oversizedSecret := secretMarker + strings.Repeat("x", 260)
	tree := NewTree()
	kc := NewTree()
	keyEntry := NewTree()
	keyEntry.Set("secret", oversizedSecret)
	kc.AddListEntry("key", "1", keyEntry)
	tree.GetOrCreateContainer("isis").AddListEntry("key-chains", "kc1", kc)

	excluded, err := excludeInvalidServiceBlocks(tree)
	require.NoError(t, err)
	require.Len(t, excluded, 1)
	assert.Equal(t, "isis/key-chains[kc1]", excluded[0].Path)
	require.Error(t, excluded[0].Cause)
	assert.NotContains(t, excluded[0].Cause.Error(), secretMarker,
		"ExcludedBlock.Cause must not leak raw sensitive leaf value")

	// Direct check on groupScopedFailures for a non-operator-error sensitive
	// SectionValidationError: proves it formats via SectionValidationError.Message().
	pending, scoped := groupScopedFailures(nil, []SectionValidationError{{
		Section:   "bus",
		Sensitive: true,
		Err: yang.ValidationError{
			Path:    "bus/hub/server[bad]/secret",
			Type:    yang.ErrTypeType,
			Message: `"hunter2-secret-value" is invalid`,
		},
	}})
	require.True(t, scoped)
	require.Len(t, pending, 1)
	causeMsg := joinExclusionCauses(pending[0].causes).Error()
	assert.NotContains(t, causeMsg, "hunter2-secret-value")
	assert.Contains(t, causeMsg, "sensitive value redacted")
}

// TestLoadConfigForStartAncestorSubsumesDescendantInSamePass proves that when
// both a parent block and a child sub-block fail in the same pass, removing the
// parent cleanly subsumes the child instead of treating the child's already-gone
// path as a non-progress failure.
func TestLoadConfigForStartAncestorSubsumesDescendantInSamePass(t *testing.T) {
	t.Cleanup(snapshotServiceBlocks())
	t.Cleanup(snapshotTreeValidators())

	require.NoError(t, RegisterServiceBlock(testBlockLG, testServiceLG))
	require.NoError(t, RegisterServiceBlock(testBlockLGRange, testServiceLGRange))
	require.NoError(t, RegisterScopedTreeValidator("test-lg-range-ancestor", testBlockLGRange, func(tree *Tree) error {
		lg := tree.GetContainerPath(testBlockLG)
		if lg == nil {
			return nil
		}
		if len(lg.GetList("next-hop-range")) > 0 {
			return errors.New("overlapping next-hop-range")
		}
		return nil
	}))

	const cfg = `environment {
	looking-glass {
		enabled true;
		certificate missing-lg-cert;
		next-hop-range 192.0.2.0/24 {
			name ams-ix;
		}
	}
}
`
	result, err := LoadConfigForStart(cfg, filepath.Join(t.TempDir(), "lg-both-bad.conf"), nil)
	require.NoError(t, err)
	assert.Nil(t, result.Tree.GetContainerPath(testBlockLG))
	require.Len(t, result.ExcludedBlocks, 1)
	assert.Equal(t, testBlockLG, result.ExcludedBlocks[0].Path)
	assert.Contains(t, result.ExcludedBlocks[0].Cause.Error(), "missing-lg-cert")
	assert.Contains(t, result.ExcludedBlocks[0].Cause.Error(), "overlapping next-hop-range")
}

// TestServiceBlockRegistrationValidation proves invalid and duplicate
// registrations are refused and boundary matching rejects partial prefix names.
func TestServiceBlockRegistrationValidation(t *testing.T) {
	t.Cleanup(snapshotServiceBlocks())
	t.Cleanup(snapshotTreeValidators())

	assert.Error(t, RegisterServiceBlock("", "svc"))
	assert.Error(t, RegisterServiceBlock("/", "svc"))
	assert.Error(t, RegisterServiceBlock(testBlockWeb, ""))
	assert.Error(t, RegisterServiceBlock("[*]", "svc"))
	assert.Error(t, RegisterServiceBlock("bus/hub/[*]", "svc"))
	assert.Error(t, RegisterScopedTreeValidator("empty-block", "", func(*Tree) error { return nil }))

	require.NoError(t, RegisterServiceBlock("/"+testBlockWeb, testServiceWeb))
	assert.Error(t, RegisterServiceBlock(testBlockWeb, "duplicate"))

	// Boundary check: "environment/web-extra" must NOT match "environment/web".
	_, _, matched := matchServiceBlock("environment/web-extra/certificate")
	assert.False(t, matched)
}
