// Design: docs/architecture/testing/ci-format.md -- capability probe uncertainty.

//go:build linux

package editorcase

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
)

var probeSource = flag.String("editor-probe-source", "", "owned absent status path compiled into the fault-injected -editor-child artifact")

// TestEditorSuiteNativeProbeUncertainty drives the registered editor entry using
// an artifact whose probe source path was fault-injected at build time. Only the
// input path changes; the real reader, parser and admission decision all execute.
// No production environment variable can replace the capability source.
func TestEditorSuiteNativeProbeUncertainty(t *testing.T) {
	if *probeSource == "" {
		t.Skip("requires a source-path fault-injected native artifact and -editor-probe-source")
	}
	if *nativeExecutable == "" {
		t.Fatal("supply the fault-injected -editor-child artifact")
	}
	if !filepath.IsAbs(*probeSource) {
		t.Fatal("probe source must be an absolute owned path")
	}
	if _, err := os.Stat(*probeSource); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("probe source must not exist: %v", err)
	}
	for _, tc := range []struct{ name, content, cause string }{
		{"unreadable", "", "cannot read"},
		{"malformed", "CapEff:\tnot-hex\n", "cannot parse CapEff"},
		{"missing", "Name:\tfixture\n", "no CapEff line"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.content != "" {
				if err := os.WriteFile(*probeSource, []byte(tc.content), 0o600); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Remove(*probeSource); err != nil {
						t.Error(err)
					}
				})
			}
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module probe-proof\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			content := "option=needs-linux:caps=net-admin\noption=file:path=must-not-open.conf\n"
			if err := os.WriteFile(filepath.Join(root, "probe.et"), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, *nativeExecutable, "test", "editor", "--dir", root, "--all")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "TMPDIR="+root, "ze_le_store_trim=off")
			log, err := os.Create(filepath.Join(root, "native.log"))
			if err != nil {
				t.Fatal(err)
			}
			defer log.Close() //nolint:errcheck // Read after the owned child exits.
			cmd.Stdout, cmd.Stderr = log, log
			plugin.KillGroupOnCancel(cmd)
			if err := plugin.StartChild(cmd); err != nil {
				t.Fatal(err)
			}
			waitErr := cmd.Wait()
			if ctx.Err() != nil {
				t.Fatal("probe refusal needed watchdog")
			}
			if waitErr == nil {
				t.Fatal("uncertain capability source did not fail the suite")
			}
			body, err := os.ReadFile(log.Name())
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"admission failed", "capability probe", tc.cause, *probeSource} {
				if !strings.Contains(string(body), want) {
					t.Fatalf("missing %q:\n%s", want, body)
				}
			}
			for _, forbidden := range []string{"must-not-open", "capability absent", "SKIP"} {
				if strings.Contains(string(body), forbidden) {
					t.Fatalf("uncertainty reached execution or skip:\n%s", body)
				}
			}
			if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.Name() == "request.json" {
					t.Errorf("uncertainty allocated child resources: %s", path)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
