// Design: docs/guide/route-policy.md -- built-in chain reference completion and validation
// Related: reference_source.go -- MustRegisterChainBuiltins and filterReference

package policy

import (
	"errors"
	"maps"
	"testing"

	"github.com/stretchr/testify/require"
)

// snapshotChainReferenceState isolates tests that register synthetic
// attachments and chain leaves so global registries stay clean.
func snapshotChainReferenceState(t *testing.T) {
	t.Helper()
	vocab.mu.Lock()
	savedAttachments := maps.Clone(vocab.attachments)
	vocab.mu.Unlock()

	chainLeaves.mu.Lock()
	savedLeaves := maps.Clone(chainLeaves.bySite)
	chainLeaves.mu.Unlock()

	t.Cleanup(func() {
		vocab.mu.Lock()
		vocab.attachments = savedAttachments
		vocab.mu.Unlock()

		chainLeaves.mu.Lock()
		chainLeaves.bySite = savedLeaves
		chainLeaves.mu.Unlock()
	})
}

// TestMustRegisterChainBuiltins covers AC-20 / policy reference completion for
// built-in chain references.
//
// VALIDATES: built-in names registered for a chain site are offered in
// filterReference on that direction, remain absent on other directions of the
// same root, and are filtered by the leaf's ChainMemberCheck.
//
// PREVENTS: built-in export references (such as next-hop:self,
// next-hop:unchanged, next-hop:auto) leaking onto import chains, or bypassing
// the attachment owner's ChainMemberCheck during completion.
func TestMustRegisterChainBuiltins(t *testing.T) {
	snapshotChainReferenceState(t)

	const (
		probeRoot      = "probe-builtins"
		probeExportAtt = "probe-builtins export"
		probeImportAtt = "probe-builtins import"
		refSelf        = "next-hop:self"
		refUnchanged   = "next-hop:unchanged"
		refAuto        = "next-hop:auto"
		refRejected    = "next-hop:forbidden"
	)

	MustRegisterAttachment(probeRoot, probeExportAtt, nil, nil)
	MustRegisterAttachment(probeRoot, probeImportAtt, nil, nil)

	exportCheck := func(_ *Registry, member, _ string) error {
		switch member {
		case refSelf, refUnchanged, refAuto:
			return nil
		default:
			return errors.New("not accepted on export")
		}
	}
	importCheck := func(_ *Registry, _ string, _ string) error {
		return errors.New("not accepted on import")
	}

	MustRegisterChainLeaf(probeRoot, "export", probeExportAtt, exportCheck)
	MustRegisterChainLeaf(probeRoot, "import", probeImportAtt, importCheck)
	MustRegisterChainBuiltins(probeRoot, "export", refSelf, refUnchanged, refAuto, refRejected)

	exportAnswer := filterReference(map[string]any{}, []string{probeRoot, "peer", "p1", "filter", "export", "filters"})
	require.Equal(t, []string{refSelf, refUnchanged, refAuto}, exportAnswer.Names)
	require.NoError(t, exportAnswer.Check(refSelf))
	require.NoError(t, exportAnswer.Check(refUnchanged))
	require.NoError(t, exportAnswer.Check(refAuto))
	require.Error(t, exportAnswer.Check(refRejected))

	importAnswer := filterReference(map[string]any{}, []string{probeRoot, "peer", "p1", "filter", "import", "filters"})
	require.Empty(t, importAnswer.Names)
	require.Error(t, importAnswer.Check(refSelf))
}

// TestMustRegisterChainBuiltinsPanicsOnMisuse verifies the init-time panic
// invariants for MustRegisterChainBuiltins.
//
// VALIDATES: empty root, empty direction, unregistered chain leaf, empty name
// list, empty name element, and duplicate built-in names each panic with the
// "BUG: policy:" prefix.
//
// PREVENTS: silent registration drops or duplicate completion entries caused
// by init-order or programmer mistakes.
func TestMustRegisterChainBuiltinsPanicsOnMisuse(t *testing.T) {
	snapshotChainReferenceState(t)

	const (
		probeRoot = "probe-builtins-panic"
		probeAtt  = "probe-builtins-panic export"
		dirExport = "export"
		dirImport = "import"
		refSelf   = "next-hop:self"
	)
	MustRegisterAttachment(probeRoot, probeAtt, nil, nil)
	MustRegisterChainLeaf(probeRoot, dirExport, probeAtt, func(*Registry, string, string) error { return nil })

	assertBugPanic := func(t *testing.T, fn func()) {
		t.Helper()
		defer func() {
			r := recover()
			require.NotNil(t, r, "expected panic")
			msg, ok := r.(string)
			require.True(t, ok, "expected string panic, got %T: %v", r, r)
			require.Contains(t, msg, "BUG: policy:")
		}()
		fn()
	}

	t.Run("empty_root", func(t *testing.T) {
		assertBugPanic(t, func() { MustRegisterChainBuiltins("", dirExport, refSelf) })
	})
	t.Run("empty_direction", func(t *testing.T) {
		assertBugPanic(t, func() { MustRegisterChainBuiltins(probeRoot, "", refSelf) })
	})
	t.Run("no_names", func(t *testing.T) {
		assertBugPanic(t, func() { MustRegisterChainBuiltins(probeRoot, dirExport) })
	})
	t.Run("unregistered_chain_leaf", func(t *testing.T) {
		assertBugPanic(t, func() { MustRegisterChainBuiltins(probeRoot, dirImport, refSelf) })
	})
	t.Run("empty_name_entry", func(t *testing.T) {
		assertBugPanic(t, func() { MustRegisterChainBuiltins(probeRoot, dirExport, refSelf, "") })
	})
	t.Run("duplicate_in_one_call", func(t *testing.T) {
		assertBugPanic(t, func() { MustRegisterChainBuiltins(probeRoot, dirExport, refSelf, refSelf) })
	})
	t.Run("duplicate_across_calls", func(t *testing.T) {
		MustRegisterChainBuiltins(probeRoot, dirExport, refSelf)
		assertBugPanic(t, func() { MustRegisterChainBuiltins(probeRoot, dirExport, refSelf) })
	})
}
