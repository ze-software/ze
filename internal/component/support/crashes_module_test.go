// Design: docs/architecture/diagnostics/crash-capture.md -- the crashes support module
//
// VALIDATES: AC-8: the `crashes` module payload carries kernel artifacts with
//            their kind, the full report content, and the readiness block.
// PREVENTS:  a support archive that is ambiguous about a box with no kernel
//            report -- an engineer cannot tell "never faulted" from "was never
//            armed" out of a file list alone.

package support

import (
	"os"
	"path/filepath"
	"testing"

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

func TestSupportCrashesModuleCarriesKernelReadiness(t *testing.T) {
	dir := t.TempDir()
	kernelName := "crash-20260905-110000-dmesg-ramoops-0-kernel.log"
	panicName := "crash-20260905-100000.log"
	for _, name := range []string{kernelName, panicName} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("report "+name+"\n"), 0o600); err != nil {
			t.Fatalf("write artifact: %v", err)
		}
	}
	t.Cleanup(crashlog.SetCrashDirForTest(dir))

	// A config path that does not exist: the module must still answer, and its
	// readiness must say what it could not read rather than reporting capture
	// as switched off.
	result, err := collectCrashes(&collectOptions{ConfigPath: filepath.Join(t.TempDir(), "absent.conf")})
	if err != nil {
		t.Fatalf("collectCrashes: %v", err)
	}

	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("payload is %T, want a map", result)
	}
	if payload[keyCount] != 2 {
		t.Fatalf("count = %v, want 2", payload[keyCount])
	}

	reports, ok := payload["crashes"].([]map[string]any)
	if !ok {
		t.Fatalf("crashes is %T, want rows", payload["crashes"])
	}
	kinds := map[string]any{}
	for _, row := range reports {
		name, _ := row[keyName].(string)
		kinds[name] = row["kind"]
		if row["content"] == "" {
			t.Fatalf("row %s carries no content", name)
		}
	}
	if kinds[kernelName] != crashlog.KindKernel {
		t.Fatalf("kernel artifact carried kind %v, want %q", kinds[kernelName], crashlog.KindKernel)
	}
	if kinds[panicName] != crashlog.KindPanic {
		t.Fatalf("panic report carried kind %v, want %q", kinds[panicName], crashlog.KindPanic)
	}

	readiness, ok := payload["readiness"].(map[string]any)
	if !ok {
		t.Fatalf("payload carries no readiness block: %v", payload)
	}
	for _, key := range []string{"configured", "armed", "directory-writable", "pstore-available", "memory-image"} {
		if _, ok := readiness[key]; !ok {
			t.Fatalf("readiness has no %q key: %v", key, readiness)
		}
	}
	if readiness[keyReason] == nil {
		t.Fatal("readiness carries no reason for a config it could not read")
	}
}

func TestSupportCrashesModuleWithNoReports(t *testing.T) {
	// An empty crash directory still answers with the readiness block, which is
	// the whole point: this is the archive an engineer reads to find out whether
	// the absence of a report means anything.
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))

	result, err := collectCrashes(&collectOptions{ConfigPath: filepath.Join(t.TempDir(), "absent.conf")})
	if err != nil {
		t.Fatalf("collectCrashes: %v", err)
	}
	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("payload is %T, want a map", result)
	}
	if payload[keyCount] != 0 {
		t.Fatalf("count = %v, want 0", payload[keyCount])
	}
	if _, ok := payload["readiness"].(map[string]any); !ok {
		t.Fatalf("payload carries no readiness block: %v", payload)
	}
}
