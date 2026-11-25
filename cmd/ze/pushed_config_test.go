// Design: docs/architecture/appliance/device-config.md -- pushed config loading tests

//go:build ze_core

package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/zefs"
)

func setupPushedConfigTest(t *testing.T) storage.Storage {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.Create(dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test

	seedConfig := []byte("set environment log level info\n")
	if err := store.WriteFile(zefs.KeyFileActive.Key("ze.conf"), seedConfig, 0); err != nil {
		t.Fatalf("write seed config: %v", err)
	}
	return store
}

func TestBootWithSeedConfigOnly(t *testing.T) {
	store := setupPushedConfigTest(t)
	seedData, _ := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))

	readPushedConfig = func() ([]byte, error) {
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() { readPushedConfig = defaultReadPushedConfig })

	checkPushedConfig(store, "ze.conf", nil)

	got, err := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))
	if err != nil {
		t.Fatalf("read active config: %v", err)
	}
	if !bytes.Equal(got, seedData) {
		t.Errorf("active config changed: got %q, want %q", got, seedData)
	}
}

func TestBootWithValidPushedConfig(t *testing.T) {
	store := setupPushedConfigTest(t)

	pushedData := []byte("set environment log level debug\n")
	readPushedConfig = func() ([]byte, error) {
		return pushedData, nil
	}
	t.Cleanup(func() { readPushedConfig = defaultReadPushedConfig })

	checkPushedConfig(store, "ze.conf", nil)

	got, err := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))
	if err != nil {
		t.Fatalf("read active config: %v", err)
	}
	if !bytes.Equal(got, pushedData) {
		t.Errorf("active config = %q, want pushed %q", got, pushedData)
	}
}

func TestBootWithInvalidPushedConfigFallsBack(t *testing.T) {
	store := setupPushedConfigTest(t)
	seedData, _ := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))

	readPushedConfig = func() ([]byte, error) {
		return []byte("this is { not { valid config syntax !!!"), nil
	}
	removed := false
	removePushedConfig = func() error {
		removed = true
		return nil
	}
	t.Cleanup(func() {
		readPushedConfig = defaultReadPushedConfig
		removePushedConfig = defaultRemovePushedConfig
	})

	checkPushedConfig(store, "ze.conf", nil)

	got, err := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))
	if err != nil {
		t.Fatalf("read active config: %v", err)
	}
	if !bytes.Equal(got, seedData) {
		t.Errorf("active config should be seed: got %q, want %q", got, seedData)
	}
	if !removed {
		t.Error("invalid pushed config should have been removed")
	}
}

func TestConfigActiveHashWritten(t *testing.T) {
	store := setupPushedConfigTest(t)

	writeConfigActiveHash(store, "ze.conf")

	// The active-config hash is ze's own output state, persisted in the shared
	// zefs store under KeyConfigActiveHash (not a loose file).
	got, err := store.ReadFile(zefs.KeyConfigActiveHash.Pattern)
	if err != nil {
		t.Fatalf("config-active-hash not written to store: %v", err)
	}

	activeData, _ := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))
	h := sha256.Sum256(activeData)
	want := fmt.Sprintf("sha256:%x", h)
	if string(got) != want {
		t.Errorf("hash = %q, want %q", got, want)
	}
}

func TestLastKnownGoodHashVerification(t *testing.T) {
	store := setupPushedConfigTest(t)

	seedConfig := "set environment log level info\n"
	h := sha256.Sum256([]byte(seedConfig))
	lkgHash := fmt.Sprintf("sha256:%x", h)
	if err := store.WriteFile(zefs.KeyConfigLastKnownGood.Pattern, []byte(lkgHash), 0); err != nil {
		t.Fatalf("write LKG: %v", err)
	}

	got, err := store.ReadFile(zefs.KeyConfigLastKnownGood.Pattern)
	if err != nil {
		t.Fatalf("read LKG: %v", err)
	}

	activeData, _ := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))
	activeH := sha256.Sum256(activeData)
	activeHash := fmt.Sprintf("sha256:%x", activeH)

	if string(got) != activeHash {
		t.Errorf("LKG hash %q does not match active config hash %q", got, activeHash)
	}
}

func TestCheckPushedConfigRemoveError(t *testing.T) {
	store := setupPushedConfigTest(t)
	seedData, _ := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))

	readPushedConfig = func() ([]byte, error) {
		return []byte("this is { not { valid config syntax !!!"), nil
	}
	removePushedConfig = func() error {
		return errors.New("permission denied")
	}
	t.Cleanup(func() {
		readPushedConfig = defaultReadPushedConfig
		removePushedConfig = defaultRemovePushedConfig
	})

	checkPushedConfig(store, "ze.conf", nil)

	got, _ := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))
	if !bytes.Equal(got, seedData) {
		t.Errorf("active config should be seed even when remove fails")
	}
}

const pushedOptionalYANG = `module ze-pushed-optional-conf {
    namespace "urn:ze:pushed-optional:conf";
    prefix pushed-optional;

    container pushed-optional {
        presence "Enable the test-only optional plugin for pushed config.";
        description "Configuration root of a test-only optional plugin.";
    }
}`

// TestJudgeConfigBeforeBootHandsLoadConfigThePlugins proves judgeConfigBeforeBoot
// forwards its command-line plugins parameter to config.LoadConfig instead of
// passing nil.
//
// VALIDATES: AC-18: (1) a candidate using an optional plugin's YANG root parses
// before boot when that plugin is in plugins and fails without it; (2) a
// candidate whose command-line plugin has an unresolvable hard dependency is
// refused before boot by LoadConfig's dependency expansion (which FirstPass
// does not run), and accepted when plugins is nil.
// PREVENTS: judgeConfigBeforeBoot passing nil to LoadConfig, which dropped
// command-line plugins from pre-boot schema loading and plugin resolution.
func TestJudgeConfigBeforeBootHandsLoadConfigThePlugins(t *testing.T) {
	savedPlugins := config.CommandLinePlugins()
	config.RecordCommandLinePlugins(nil)
	t.Cleanup(func() { config.RecordCommandLinePlugins(savedPlugins) })

	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })

	if err := registry.Register(registry.Registration{
		Name:        "pushed-optional",
		Description: "test-only optional plugin with YANG schema",
		YANG:        pushedOptionalYANG,
		ConfigRoots: []string{"pushed-optional"},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}); err != nil {
		t.Fatalf("register pushed-optional: %v", err)
	}
	if err := registry.Register(registry.Registration{
		Name:         "pushed-needs-missing",
		Description:  "test-only plugin with an absent hard dependency",
		Dependencies: []string{"pushed-absent-dependency"},
		RunEngine:    func(net.Conn) int { return 0 },
		CLIHandler:   func([]string) int { return 0 },
	}); err != nil {
		t.Fatalf("register pushed-needs-missing: %v", err)
	}

	optionalCandidate := []byte("pushed-optional { }\n")
	if _, err := judgeConfigBeforeBoot(optionalCandidate, nil, nil); err == nil {
		t.Fatal("candidate with optional plugin root must fail when plugins is nil")
	}
	judgedAtBoot, err := judgeConfigBeforeBoot(optionalCandidate, nil, []string{"ze.pushed-optional"})
	if err != nil {
		t.Fatalf("judgeConfigBeforeBoot with optional plugin failed: %v", err)
	}
	if judgedAtBoot {
		t.Error("judgedAtBoot = true, want false for internal command-line plugin")
	}

	minimalCandidate := []byte("set environment log level info\n")
	if judgedAtBoot, err := judgeConfigBeforeBoot(minimalCandidate, nil, nil); err != nil || judgedAtBoot {
		t.Fatalf("minimal candidate without plugins = (judgedAtBoot=%v, err=%v), want (false, nil)", judgedAtBoot, err)
	}
	judgedAtBoot, err = judgeConfigBeforeBoot(minimalCandidate, nil, []string{"ze.pushed-needs-missing"})
	if err == nil {
		t.Fatal("judgeConfigBeforeBoot must refuse a command-line plugin with a missing dependency")
	}
	if judgedAtBoot {
		t.Error("judgedAtBoot = true, want false for internal plugin refusal")
	}
	if !strings.Contains(err.Error(), "pushed-absent-dependency") {
		t.Errorf("refusal %q does not name missing dependency pushed-absent-dependency", err.Error())
	}
}
