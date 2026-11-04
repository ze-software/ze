// Design: docs/contributing/running-commands.md -- actual inherited-root child proofs.
package session

import (
	"bytes"
	"crypto/rand"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/seedinit"
	"github.com/ze-software/ze/internal/le/job"
)

func nativeSeedFixture(t *testing.T) (string, string, string) {
	t.Helper()
	args := flag.Args()
	if len(args) != 1 {
		t.Skip("supply one current native Ze artifact with -args -- <absolute-ze-path>")
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "seed-bound-owner")
	root := t.TempDir()
	rel := filepath.Join("tmp", "session", "2026-11-03-170000")
	config := filepath.Join(root, rel, "etc", "ze")
	binary := filepath.Join(rel, "bin", "ze")
	for _, dir := range []string{config, filepath.Dir(filepath.Join(root, binary))} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, rel, ".session-id"), []byte("seed-bound-owner\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(args[0], filepath.Join(root, binary)); err != nil {
		t.Fatal(err)
	}
	return root, binary, config
}

// TestSeedNativeBoundPositive drives the ordinary native seeder entry and
// requires both genuine children to publish a live tree and retire their seed.
func TestSeedNativeBoundPositive(t *testing.T) {
	root, binary, config := nativeSeedFixture(t)
	var output bytes.Buffer
	report, code, err := seedStore(root, binary, streams{Out: &output, Err: &output})
	if err != nil || code != 0 || !report.Seeded {
		t.Fatalf("native seed: code=%d report=%+v err=%v output=%s", code, report, err, output.String())
	}
	assertNativeSeedTree(t, config)
}

// TestSeedNativeSecondChildReplacement swaps the root after the real seed child
// returns and before the real importer starts. Both the import and its startup
// crash capture must use the originally accepted directory.
func TestSeedNativeSecondChildReplacement(t *testing.T) {
	root, binary, config := nativeSeedFixture(t)
	if err := validateSeedEntry(root, binary, os.Environ()); err != nil {
		t.Fatal(err)
	}
	resources, err := openSeedResources(root, binary)
	if err != nil {
		t.Fatal(err)
	}
	defer resources.config.Close()     //nolint:errcheck // Test cleanup after both children.
	defer resources.executable.Close() //nolint:errcheck // Test cleanup after both children.
	calls := 0
	run := func(argv []string, streams job.ProcessIO) (int, error) {
		calls++
		if calls == 2 {
			if _, err := os.Stat(filepath.Join(config, "database.zefs")); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(config, config+"-held"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(config, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(config, "sentinel"), []byte("foreign"), 0o640); err != nil {
				t.Fatal(err)
			}
		}
		return job.RunProcess(argv, streams)
	}
	var output bytes.Buffer
	report, code, err := seedStoreWithOps(root, binary, streams{Out: &output, Err: &output}, seedOps{
		configRoot: resources.config, executable: resources.executable, environ: os.Environ(),
		random: rand.Reader, sleep: time.Sleep, waits: seedWaitLimit, run: run,
	})
	if err != nil || code != 0 || !report.Seeded || calls != 2 {
		t.Fatalf("native second child: code=%d calls=%d report=%+v err=%v output=%s", code, calls, report, err, output.String())
	}
	assertNativeSeedTree(t, config+"-held")
	entries, err := os.ReadDir(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("foreign directory changed: %v", entries)
	}
	got, err := os.ReadFile(filepath.Join(config, "sentinel"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "foreign" {
		t.Errorf("foreign bytes changed: %q", got)
	}
	info, err := os.Stat(filepath.Join(config, "sentinel"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Errorf("foreign mode changed: %v", info.Mode())
	}
}

func assertNativeSeedTree(t *testing.T, config string) {
	t.Helper()
	for _, name := range []string{"database", "crash"} {
		info, err := os.Stat(filepath.Join(config, name))
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", name)
		}
	}
	if _, err := os.Stat(filepath.Join(config, "database.zefs")); !os.IsNotExist(err) {
		t.Fatalf("seed was not retired: %v", err)
	}
	if _, err := os.Stat(filepath.Join(config, "database.import-intent")); !os.IsNotExist(err) {
		t.Fatalf("import intent was not completed: %v", err)
	}
	archives, err := filepath.Glob(filepath.Join(config, "database.zefs.replaced-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(archives) != 2 {
		t.Errorf("want one retired seed and its lock, got %v", archives)
	}
}

// TestSeedNativeMarkerWithoutDescriptor proves the environment marker alone
// cannot enable path-based init or even its ordinary startup crash probing.
func TestSeedNativeMarkerWithoutDescriptor(t *testing.T) {
	root, binary, config := nativeSeedFixture(t)
	var output bytes.Buffer
	code, err := job.RunProcess([]string{filepath.Join(root, binary), "init", "--seed"}, job.ProcessIO{
		Dir: root, Environ: append(os.Environ(), seedinit.DirectoryKey+"="+config),
		Stdout: &output, Stderr: &output,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code == 0 {
		t.Fatalf("marker without descriptor accepted: %s", output.String())
	}
	entries, err := os.ReadDir(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("unadmitted child wrote entries: %v; output=%s", entries, output.String())
	}
}

// TestSeedNativeOrdinaryInit proves that a native invocation without the internal
// marker still resolves its normal binary-relative store and imports it.
func TestSeedNativeOrdinaryInit(t *testing.T) {
	root, binary, config := nativeSeedFixture(t)
	var output bytes.Buffer
	program := filepath.Join(root, binary)
	for _, argv := range [][]string{{program, "init", "--seed"}, {program, "init", "--from", filepath.Join(config, "database.zefs")}} {
		code, err := job.RunProcess(argv, job.ProcessIO{
			Dir: root, Environ: os.Environ(), Stdin: strings.NewReader("admin\nfixture-password\n127.0.0.1\n2222\nfixture\n"),
			Stdout: &output, Stderr: &output,
		})
		if err != nil || code != 0 {
			t.Fatalf("ordinary native init: code=%d err=%v output=%s", code, err, output.String())
		}
	}
	assertNativeSeedTree(t, config)
}

// TestSeedNativeEmptyMarker refuses a malformed inherited contract before
// ordinary crash probing; an empty marker is not the marker's absence.
func TestSeedNativeEmptyMarker(t *testing.T) {
	root, binary, config := nativeSeedFixture(t)
	var output bytes.Buffer
	code, err := job.RunProcess([]string{filepath.Join(root, binary), "init", "--seed"}, job.ProcessIO{
		Dir: root, Environ: append(os.Environ(), seedinit.DirectoryKey+"="),
		Stdout: &output, Stderr: &output,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code == 0 {
		t.Fatalf("empty marker accepted: %s", output.String())
	}
	entries, err := os.ReadDir(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("empty marker fell through to ordinary startup: %v; output=%s", entries, output.String())
	}
}

// TestSeedNativeAdmissionRefusals enters the native child directly so its own
// admission, rather than only the parent's checks, must reject unsafe contracts.
func TestSeedNativeAdmissionRefusals(t *testing.T) {
	for _, scenario := range []string{"file descriptor", "public directory", "wrong source", "wrong command", "crash override", "crash symlink"} {
		t.Run(scenario, func(t *testing.T) {
			root, binary, config := nativeSeedFixture(t)
			outside := t.TempDir()
			sentinel := filepath.Join(outside, "sentinel")
			if err := os.WriteFile(sentinel, []byte("foreign"), 0o640); err != nil {
				t.Fatal(err)
			}
			program := filepath.Join(root, binary)
			executable, err := os.Open(program)
			if err != nil {
				t.Fatal(err)
			}
			defer executable.Close() //nolint:errcheck // Test child has returned.
			entry := config
			if scenario == "file descriptor" {
				entry = sentinel
			}
			directory, err := os.Open(entry)
			if err != nil {
				t.Fatal(err)
			}
			defer directory.Close() //nolint:errcheck // Test child has returned.
			args := []string{"init", "--seed"}
			environ := append(os.Environ(), seedinit.DirectoryKey+"="+config)
			switch scenario {
			case "public directory":
				if err := os.Chmod(config, 0o755); err != nil {
					t.Fatal(err)
				}
			case "wrong source":
				args = []string{"init", "--from", sentinel}
			case "wrong command":
				args = []string{"start", "--seed"}
			case "crash override":
				environ = append(environ, "ZE_CRASH_DIR="+outside)
			case "crash symlink":
				if err := os.Symlink(outside, filepath.Join(config, "crash")); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.CommandContext(t.Context(), program, args...) //nolint:gosec // Test-owned native artifact.
			cmd.Dir, cmd.Env = root, environ
			cmd.ExtraFiles = []*os.File{executable, directory}
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("unsafe child accepted: %s", output)
			}
			if !strings.Contains(string(output), "session init:") {
				t.Fatalf("child did not reach admission refusal: %v: %s", err, output)
			}
			entries, err := os.ReadDir(config)
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if scenario == "crash symlink" {
				want = 1
			}
			if len(entries) != want {
				t.Errorf("refused child wrote config entries: %v", entries)
			}
			entries, err = os.ReadDir(outside)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Errorf("refused child wrote foreign entries: %v", entries)
			}
			got, err := os.ReadFile(sentinel)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != "foreign" {
				t.Errorf("foreign sentinel changed: %q", got)
			}
		})
	}
}
