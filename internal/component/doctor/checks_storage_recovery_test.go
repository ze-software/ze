// Design: docs/architecture/doctor-and-health-checks.md -- retained full-store recovery sources.
package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/configtext"
)

func recoveryDoctorCheck(t *testing.T) diagnostic.DoctorCheck {
	t.Helper()
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePreConfig) {
		if check.Name == "store-recovery-pending" {
			return check
		}
	}
	t.Fatal("doctor has no check for retained full-store recovery sources")
	return diagnostic.DoctorCheck{}
}

// TestDoctorRecoveryNamesExactTarget proves a retained owned stage has a
// complete manual resolution command for a nondefault target, with both path
// words quoted. The diagnosis itself preserves every retained source byte.
func TestDoctorRecoveryNamesExactTarget(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nondefault target")
	live, err := storage.Create(dir)
	require.NoError(t, err)
	artifactPath := filepath.Join(t.TempDir(), "caller.zefs")
	artifact, err := storage.CreateBlob(artifactPath)
	require.NoError(t, err)
	require.NoError(t, artifact.WriteKey("meta/test/restored", []byte{0, 255, 1}))
	require.NoError(t, artifact.Close())
	owner, err := storage.NewDaemonRecovery(storage.BindConfigSource(live, "ze.conf", storage.ConfigSourceStored))
	require.NoError(t, err)
	_, err = storage.RecoveryFor(owner.Borrow()).StagePath(context.Background(), artifactPath)
	require.NoError(t, err)
	closed, err := owner.Close()
	require.NoError(t, err)
	reader, err := storage.Open(dir)
	require.NoError(t, err)
	defer func() { require.NoError(t, reader.Close()) }()
	require.ErrorIs(t, closed.Apply(), storage.ErrBusy)
	stage, err := storage.PendingRecoveryStage(reader)
	require.NoError(t, err)
	snapshot := filepath.Join(stage, "snapshot.zefs")
	before, err := os.ReadFile(snapshot)
	require.NoError(t, err)
	findings := recoveryDoctorCheck(t).Check(diagnostic.DoctorCheckContext{Store: reader, ConfigDir: dir})
	require.Len(t, findings, 1)
	want := "ze data --path " + configtext.QuoteCommandWord(filepath.Join(dir, storage.TreeName)) + " restore " + configtext.QuoteCommandWord(snapshot) + " full"
	require.Equal(t, want, findings[0].Fix)
	after, err := os.ReadFile(snapshot)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.NoError(t, reader.Close())
	moved := filepath.Join(t.TempDir(), "actual moved target")
	require.NoError(t, os.Rename(dir, moved))
	foreign, err := storage.Create(dir)
	require.NoError(t, err)
	require.NoError(t, foreign.Close())
	reader, err = storage.Open(moved)
	require.NoError(t, err)
	findings = recoveryDoctorCheck(t).Check(diagnostic.DoctorCheckContext{Store: reader, ConfigDir: moved})
	require.Len(t, findings, 1)
	movedSnapshot := filepath.Join(moved, filepath.Base(stage), "snapshot.zefs")
	want = "ze data --path " + configtext.QuoteCommandWord(filepath.Join(moved, storage.TreeName)) + " restore " + configtext.QuoteCommandWord(movedSnapshot) + " full"
	require.Equal(t, want, findings[0].Fix)
}

// TestDoctorReportsRetainedRecoverySource proves the registered readiness check
// names the source directory and an actionable command without consuming bytes.
func TestDoctorReportsRetainedRecoverySource(t *testing.T) {
	check := recoveryDoctorCheck(t)
	dir := t.TempDir()
	store, err := storage.Create(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	ctx := diagnostic.DoctorCheckContext{Store: store, ConfigDir: dir}
	require.Empty(t, check.Check(ctx), "healthy owner must not report retained recovery")
	stage := filepath.Join(dir, "database.recovery-tmp-retained")
	require.NoError(t, os.Mkdir(stage, 0o700))
	marker := filepath.Join(stage, "unresolved-marker")
	require.NoError(t, os.WriteFile(marker, []byte("retain me"), 0o600))
	findings := check.Check(ctx)
	require.Len(t, findings, 1)
	require.Equal(t, "doctor-storage-recovery-pending", findings[0].Code)
	require.Equal(t, diagnostic.SeverityWarning, findings[0].Severity)
	require.Equal(t, stage, findings[0].Path)
	require.Contains(t, findings[0].Message, stage)
	require.NotEmpty(t, findings[0].Fix)
	content, err := os.ReadFile(marker)
	require.NoError(t, err)
	require.Equal(t, "retain me", string(content))
}

// TestDoctorCannotReadRecoveryState refuses a healthy answer when the bound
// store is closed and its retained directory authority cannot be inspected.
func TestDoctorCannotReadRecoveryState(t *testing.T) {
	check := recoveryDoctorCheck(t)
	dir := t.TempDir()
	store, err := storage.Create(dir)
	require.NoError(t, err)
	require.NoError(t, store.Close())
	findings := check.Check(diagnostic.DoctorCheckContext{Store: store, ConfigDir: dir})
	require.Len(t, findings, 1)
	require.Equal(t, "doctor-storage-recovery-pending", findings[0].Code)
	require.Equal(t, diagnostic.SeverityError, findings[0].Severity)
	require.NotEmpty(t, findings[0].Fix)
}

// TestDoctorRecoveryCodeRegistered proves the retained-source finding has an
// operator-readable description through the shared diagnostic registry.
func TestDoctorRecoveryCodeRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()
	require.NotNil(t, diagnostic.Lookup(diagnostic.CodeDoctorStorageRecoveryPending))
}
