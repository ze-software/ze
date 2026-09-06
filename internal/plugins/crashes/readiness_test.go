// Design: docs/architecture/diagnostics/crash-capture.md -- readiness surface tests
//
// VALIDATES: AC-1, AC-4, AC-8 and AC-14 on the surfaces an operator reaches:
//            the readiness answer names an unreadable config rather than
//            reporting capture as off, and the listing payload carries the kind
//            field with lower kebab-case keys.
// PREVENTS:  the daemon path and the offline fallback answering different
//            shapes, which would make the answer depend on the health of the box
//            an operator is trying to diagnose.

package crashes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/ze-software/ze/internal/component/config/system/yang" // registers ze-system-conf.yang so a config carrying crash-dump parses
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/env"
)

// TestMain registers the env key resolve.Storage reads. The `ze` binary
// registers it in cmd/ze; a test binary links no main, and env.Get is fatal on
// an unregistered key by design.
func TestMain(m *testing.M) {
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.storage.blob",
		Type:        "bool",
		Default:     "true",
		Description: "Use blob storage (false = filesystem)",
	})
	os.Exit(m.Run())
}

func TestReadinessNamesAnUnreadableConfig(t *testing.T) {
	// A config that cannot be read is a different fact from crash capture being
	// off. Reporting the second in place of the first would tell an operator
	// their setting is absent when the daemon simply could not look.
	readiness := Readiness(filepath.Join(t.TempDir(), "absent.conf"))

	if readiness.Configured {
		t.Fatal("Configured = true with no config read")
	}
	if !strings.Contains(readiness.Reason, "absent.conf") {
		t.Fatalf("Reason = %q, want the unreadable file named", readiness.Reason)
	}
}

func TestReadinessReadsTheConfiguredIntent(t *testing.T) {
	// AC-1: the leaf an operator commits reaches the readiness answer.
	path := filepath.Join(t.TempDir(), "ze.conf")
	text := "system {\n    crash-dump {\n        enabled true\n        reserve 32\n    }\n}\n"
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	readiness := Readiness(path)

	if !readiness.Configured {
		t.Fatalf("Configured = false, want true (reason %q)", readiness.Reason)
	}
}

func TestReadinessFieldsAreRenderable(t *testing.T) {
	// AC-14: the block `show crashes` carries is structured data, so `| json`,
	// `| yaml` and `| table` are three renderings of one payload.
	fields := Readiness(filepath.Join(t.TempDir(), "absent.conf")).Fields()

	for _, key := range []string{"configured", "armed", "directory-writable", "pstore-available", "memory-image"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("readiness fields have no %q key: %v", key, fields)
		}
	}
	for key := range fields {
		if strings.ContainsAny(key, "_ ") || strings.ToLower(key) != key {
			t.Fatalf("readiness key %q is not lower kebab-case", key)
		}
	}
}

func TestOfflineShowCrashesListsKernelKind(t *testing.T) {
	// AC-4 on the offline fallback: the path an operator takes when the daemon
	// has died lists a kernel artifact beside a Go panic report, says which kind
	// each one is, and carries the readiness block. The assertion runs over the
	// bytes the command prints, because that is what the operator reads.
	dir := t.TempDir()
	kernelName := "crash-20260905-110000-dmesg-ramoops-0-kernel.log"
	panicName := "crash-20260905-100000.log"
	for _, name := range []string{kernelName, panicName} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("report\n"), 0o600); err != nil {
			t.Fatalf("write artifact: %v", err)
		}
	}
	t.Cleanup(crashlog.SetCrashDirForTest(dir))

	payload := captureShowList(t)

	entries, ok := payload["crashes"].([]any)
	if !ok || len(entries) != 2 {
		t.Fatalf("crashes = %v, want two rows", payload["crashes"])
	}
	kinds := map[string]string{}
	for _, entry := range entries {
		row, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("row is %T, want an object", entry)
		}
		name, _ := row["name"].(string)
		kind, _ := row["kind"].(string)
		kinds[name] = kind
	}
	if kinds[kernelName] != crashlog.KindKernel {
		t.Fatalf("kernel artifact listed with kind %q, want %q", kinds[kernelName], crashlog.KindKernel)
	}
	if kinds[panicName] != crashlog.KindPanic {
		t.Fatalf("panic report listed with kind %q, want %q", kinds[panicName], crashlog.KindPanic)
	}
	if _, ok := payload["readiness"].(map[string]any); !ok {
		t.Fatalf("payload carries no readiness block: %v", payload)
	}
}

// captureShowList runs the offline listing and decodes what it printed.
func captureShowList(t *testing.T) map[string]any {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	previous := os.Stdout
	os.Stdout = writer
	code := showList()
	os.Stdout = previous
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if code != 0 {
		t.Fatalf("showList = %d, want 0", code)
	}

	var payload map[string]any
	if err := json.NewDecoder(reader).Decode(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	return payload
}
