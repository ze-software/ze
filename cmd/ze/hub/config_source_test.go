// Design: docs/architecture/hub-architecture.md -- file authority and recovery
package hub

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// An offline edit wins on the next explicit start, even with an old active pointer.
func TestExplicitSourceRestartUsesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "router.conf")
	store := newTestFileStore(t, path)
	if err := os.WriteFile(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := initializeConfigSource(store, path, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("offline edit"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.Open(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close() //nolint:errcheck // test cleanup
	bound := storage.BindConfigSource(reopened, path, storage.ConfigSourceFile)
	content, err := storage.ReadConfigSource(bound, path)
	if err != nil || string(content) != "offline edit" {
		t.Fatalf("startup = %q, %v", content, err)
	}
	if err := initializeConfigSource(bound, path, content); err != nil {
		t.Fatal(err)
	}
	active, err := storage.ReadActiveConfig(bound, path)
	if err != nil || !bytes.Equal(active, content) {
		t.Fatalf("active = %q, %v", active, err)
	}
}

// Every durable interruption point either completes the same publication or
// refuses a third file value. It never removes a candidate still needed by intent.
func TestExplicitCommitRecovery(t *testing.T) {
	for _, phase := range []string{"intent", "file", "promoted", "external"} {
		t.Run(phase, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "router.conf")
			store := newTestFileStore(t, path)
			old, next := []byte("accepted"), []byte("candidate")
			if err := os.WriteFile(path, old, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := initializeConfigSource(store, path, old); err != nil {
				t.Fatal(err)
			}
			stamp, err := storage.WriteCandidateVersion(store, path, next, time.Now().Add(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			intent, err := json.Marshal(fileCommit{Path: path, Expected: old, Content: next, Stamp: stamp})
			if err != nil {
				t.Fatal(err)
			}
			key := zefs.KeyConfigFileCommit.Key(filepath.Base(path))
			if err := store.WriteKey(key, intent); err != nil {
				t.Fatal(err)
			}
			if phase != "intent" {
				value := next
				if phase == "external" {
					value = []byte("external edit")
				}
				if err := storage.WriteConfigFile(path, value); err != nil {
					t.Fatal(err)
				}
			}
			if phase == "promoted" {
				if err := storage.PromoteCandidate(store, path); err != nil {
					t.Fatal(err)
				}
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := storage.Open(filepath.Dir(path))
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close() //nolint:errcheck // test cleanup
			bound := storage.BindConfigSource(reopened, path, storage.ConfigSourceFile)
			err = recoverFileCommit(bound, path)
			if phase == "external" {
				if err == nil || !strings.Contains(err.Error(), "changed externally") {
					t.Fatalf("recovery = %v", err)
				}
				disk, readErr := os.ReadFile(path)
				if readErr != nil || string(disk) != "external edit" {
					t.Fatalf("external bytes = %q, %v", disk, readErr)
				}
				if _, readErr := bound.ReadKey(key); readErr != nil {
					t.Fatalf("intent lost: %v", readErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			disk, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(disk, next) {
				t.Fatalf("file = %q, %v", disk, err)
			}
			active, err := storage.ReadActiveConfig(bound, path)
			if err != nil || !bytes.Equal(active, next) {
				t.Fatalf("active = %q, %v", active, err)
			}
			if _, err := bound.ReadKey(key); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("intent retained: %v", err)
			}
		})
	}
}

// An editor candidate cannot overwrite an external edit made since acceptance.
func TestExplicitCandidateRejectsExternalEdit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "router.conf")
	store := newTestFileStore(t, path)
	if err := os.WriteFile(path, []byte("accepted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := initializeConfigSource(store, path, []byte("accepted")); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.WriteCandidateVersion(store, path, []byte("editor"), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("external"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := promoteConfigCandidate(store, path); err == nil || !strings.Contains(err.Error(), "changed externally") {
		t.Fatalf("commit = %v", err)
	}
	disk, err := os.ReadFile(path)
	if err != nil || string(disk) != "external" {
		t.Fatalf("file = %q, %v", disk, err)
	}
	active, err := storage.ReadActiveConfig(store, path)
	if err != nil || string(active) != "accepted" {
		t.Fatalf("active = %q, %v", active, err)
	}
}

func TestExplicitRuntimeCommitPublishesBothAuthorities(t *testing.T) {
	path := filepath.Join(t.TempDir(), "router.conf")
	store := newTestFileStore(t, path)
	before, after := []byte("accepted"), []byte("committed")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := initializeConfigSource(store, path, before); err != nil {
		t.Fatal(err)
	}
	err := commitRuntimeConfig(store, path, path, before, after, func() error {
		return promoteConfigCandidate(store, path)
	})
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(file, after) {
		t.Fatalf("explicit file = %q, %v", file, err)
	}
	active, err := storage.ReadActiveConfig(store, path)
	if err != nil || !bytes.Equal(active, after) {
		t.Fatalf("stored active = %q, %v", active, err)
	}
	versions, err := store.ListVersions(path)
	if err != nil {
		t.Fatal(err)
	}
	retained := false
	for _, version := range versions {
		content, err := store.ReadFile(version.Path)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(content, before) {
			retained = true
		}
	}
	if !retained {
		t.Fatal("prior accepted config disappeared from history")
	}
}
