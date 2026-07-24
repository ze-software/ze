package hub

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/migration"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/version"
)

func evolveTestSetup(t *testing.T) func() {
	t.Helper()
	version.Stamp("26.05.26", "2026-05-26")
	return func() { version.Stamp("dev", "unknown") }
}

func registerTestEvolution(t *testing.T, release, name string) func() {
	t.Helper()
	err := migration.RegisterEvolution(migration.Evolution{
		Release: release,
		Name:    name,
		Detect: func(tree *zeconfig.Tree) bool {
			_, ok := tree.Get("evolved-" + name)
			return !ok
		},
		Apply: func(tree *zeconfig.Tree) (*zeconfig.Tree, error) {
			result := tree.Clone()
			result.Set("evolved-"+name, "yes")
			return result, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return func() { resetEvolutions(t) }
}

func resetEvolutions(t *testing.T) {
	t.Helper()
	migration.ResetForTest()
}

func TestApplyEvolutionsBackupCreated(t *testing.T) {
	cleanup := evolveTestSetup(t)
	defer cleanup()
	defer registerTestEvolution(t, "26.06.01", "backup-test")()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")
	originalData := []byte("# ze-schema: 26.05.01\nset environment log level warn\n")
	if err := os.WriteFile(configPath, originalData, 0o600); err != nil {
		t.Fatal(err)
	}
	rollbackDir := filepath.Join(dir, "rollback")
	if err := os.MkdirAll(rollbackDir, 0o700); err != nil {
		t.Fatal(err)
	}

	store := storage.NewFilesystem()
	tree, err := parseTestConfig(t, string(originalData))
	if err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	outcome, evolveErr := applyEvolutions(logger, store, configPath, originalData, tree, "26.05.01")
	if evolveErr != nil {
		t.Fatal(evolveErr)
	}
	if len(outcome.applied) == 0 {
		t.Fatal("expected evolutions to apply")
	}

	entries, err := os.ReadDir(rollbackDir)
	if err != nil {
		t.Fatal(err)
	}
	foundBackup := false
	for _, e := range entries {
		backupData, rErr := os.ReadFile(filepath.Join(rollbackDir, e.Name()))
		if rErr != nil {
			continue
		}
		if zeconfig.ScanStampRelease(backupData) == "26.05.01" {
			foundBackup = true
			break
		}
	}
	if !foundBackup {
		t.Error("original config should have been backed up to rollback dir before write-back")
	}
}

func TestApplyEvolutionsDataUpdated(t *testing.T) {
	cleanup := evolveTestSetup(t)
	defer cleanup()
	defer registerTestEvolution(t, "26.06.01", "data-update")()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")
	originalData := []byte("# ze-schema: 26.05.01\nset environment log level warn\n")
	if err := os.WriteFile(configPath, originalData, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "rollback"), 0o700); err != nil {
		t.Fatal(err)
	}

	store := storage.NewFilesystem()
	tree, err := parseTestConfig(t, string(originalData))
	if err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	outcome, evolveErr := applyEvolutions(logger, store, configPath, originalData, tree, "26.05.01")
	if evolveErr != nil {
		t.Fatal(evolveErr)
	}

	if bytes.Equal(outcome.data, originalData) {
		t.Error("data should be updated to evolved bytes, got original")
	}
	stampedRelease := zeconfig.ScanStampRelease(outcome.data)
	if stampedRelease != "26.05.26" {
		t.Errorf("evolved data stamp = %q, want %q", stampedRelease, "26.05.26")
	}
}

func TestApplyEvolutionsNoEvolutionsNeeded(t *testing.T) {
	cleanup := evolveTestSetup(t)
	defer cleanup()
	defer registerTestEvolution(t, "26.04.01", "old-change")()

	originalData := []byte("# ze-schema: 26.05.01\nset environment log level warn\n")
	tree, err := parseTestConfig(t, string(originalData))
	if err != nil {
		t.Fatal(err)
	}

	store := storage.NewFilesystem()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	outcome, evolveErr := applyEvolutions(logger, store, "", originalData, tree, "26.05.01")
	if evolveErr != nil {
		t.Fatal(evolveErr)
	}
	if len(outcome.applied) != 0 {
		t.Errorf("expected no evolutions, got %v", outcome.applied)
	}
	if !bytes.Equal(outcome.data, originalData) {
		t.Error("data should be unchanged when no evolutions apply")
	}
}

func TestApplyEvolutionsStdinConfig(t *testing.T) {
	cleanup := evolveTestSetup(t)
	defer cleanup()
	defer registerTestEvolution(t, "26.06.01", "stdin-test")()

	originalData := []byte("# ze-schema: 26.05.01\nset environment log level warn\n")
	tree, err := parseTestConfig(t, string(originalData))
	if err != nil {
		t.Fatal(err)
	}

	store := storage.NewFilesystem()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	outcome, evolveErr := applyEvolutions(logger, store, "-", originalData, tree, "26.05.01")
	if evolveErr != nil {
		t.Fatal(evolveErr)
	}
	if len(outcome.applied) == 0 {
		t.Fatal("expected evolutions to apply even for stdin")
	}
	if !bytes.Equal(outcome.data, originalData) {
		t.Error("data should be unchanged for stdin config (no write-back)")
	}
}

func TestApplyEvolutionsWriteBackMatchesDisk(t *testing.T) {
	cleanup := evolveTestSetup(t)
	defer cleanup()
	defer registerTestEvolution(t, "26.06.01", "disk-match")()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")
	originalData := []byte("# ze-schema: 26.05.01\nset environment log level warn\n")
	if err := os.WriteFile(configPath, originalData, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "rollback"), 0o700); err != nil {
		t.Fatal(err)
	}

	store := storage.NewFilesystem()
	tree, err := parseTestConfig(t, string(originalData))
	if err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	outcome, evolveErr := applyEvolutions(logger, store, configPath, originalData, tree, "26.05.01")
	if evolveErr != nil {
		t.Fatal(evolveErr)
	}

	diskData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(outcome.data, diskData) {
		t.Error("outcome.data should match what was written to disk")
	}
}

func parseTestConfig(t *testing.T, input string) (*zeconfig.Tree, error) {
	t.Helper()
	return zeconfig.ParseTreeWithYANG(input, nil)
}
