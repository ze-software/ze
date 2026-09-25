// VALIDATES: session store seeding is one-time under concurrent builds, dotted
// config overrides fail closed, and recovery snapshots preserve every handoff.
// PREVENTS: two seeders racing on the session store, a database.zefs blob
// counted as a store ze refuses to open, or a Stop hook deleting phase state.
package session

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/job"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

func TestConfigOverrideRecognizesDottedEnvironmentSpellings(t *testing.T) {
	for _, name := range []string{"ZE_CONFIG_DIR", "ze.config.dir", "Ze.Config_Dir", "ze_config.dir"} {
		t.Run(name, func(t *testing.T) {
			got, found := configOverride([]string{"HOME=/home/test", name + "=/elsewhere"})
			if !found || got != name {
				t.Fatalf("configOverride = %q, %t; want %q, true", got, found, name)
			}
		})
	}
	if got, found := configOverride([]string{"ZE_CONFIG_DIRECTORY=/elsewhere"}); found || got != "" {
		t.Fatalf("unrelated variable matched as %q", got)
	}
}

func TestScratchDefaultReturnsPathWithoutCreatingIt(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "fixture")
	root := t.TempDir()
	sessionDir := filepath.Join(root, "tmp", "session", "2001-02-03-fixture")
	if err := os.MkdirAll(sessionDir, 0o750); err != nil {
		t.Fatal(err)
	}
	report, err := Scratch(root, false)
	if err != nil {
		t.Fatal(err)
	}
	want := "tmp/session/2001-02-03-fixture/scratch"
	if report.Path != want || report.Ensured || report.Text() != want+"\n" {
		t.Fatalf("scratch report = %#v, text %q", report, report.Text())
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(report.Path))); !os.IsNotExist(err) {
		t.Fatalf("default scratch changed the filesystem: %v", err)
	}
}

func TestScratchEnsureCreatesTheResolvedDirectory(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "fixture")
	root := t.TempDir()
	sessionDir := filepath.Join(root, "tmp", "session", "2001-02-03-fixture")
	if err := os.MkdirAll(sessionDir, 0o750); err != nil {
		t.Fatal(err)
	}
	report, err := Scratch(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Ensured {
		t.Fatalf("scratch report = %#v", report)
	}
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(report.Path)))
	if err != nil || !info.IsDir() {
		t.Fatalf("ensured scratch directory: info %#v err %v", info, err)
	}
}

func TestScratchFailsClosedOnUnsafeSessionIdentity(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "unsafe/id")
	root := t.TempDir()
	report, err := Scratch(root, true)
	if err == nil || !strings.Contains(err.Error(), `unsafe session id "unsafe/id"`) {
		t.Fatalf("Scratch = %#v, err %v", report, err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "tmp", "session")); !os.IsNotExist(statErr) {
		t.Fatalf("unsafe identity changed the filesystem: %v", statErr)
	}
}

func TestScratchGrammarRejectsOpenAndRepeatedKeywords(t *testing.T) {
	for _, args := range [][]string{
		{"scratch", "path"},
		{"scratch", "ensure", "ensure"},
		{"scratch", "ensure", "value"},
	} {
		payload, code := Answer(args)
		if payload != nil || code != 2 {
			t.Errorf("Answer(%q) = %#v, %d; want nil, 2", args, payload, code)
		}
	}
}

func TestSeedStorePersistsCredentialsAndExactInitInput(t *testing.T) {
	root := t.TempDir()
	binary := filepath.ToSlash(filepath.Join("tmp", "session", "2026-08-27-fixture", "bin", "ze"))
	binaryPath := filepath.Join(root, filepath.FromSlash(binary))
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	var gotArgv []string
	var gotInput string
	ops := seedOps{
		environ: []string{"HOME=/home/test", "PATH=/bin"},
		random:  bytes.NewReader(bytes.Repeat([]byte{0xab}, 24)),
		sleep:   func(time.Duration) {},
		waits:   1,
		run: func(argv []string, processIO job.ProcessIO) (int, error) {
			gotArgv = append(gotArgv, strings.Join(argv, " "))
			if processIO.Stdin != nil {
				content, err := io.ReadAll(processIO.Stdin)
				if err != nil {
					return 127, err
				}
				gotInput += string(content)
			}
			return fakeInit(root, argv)
		},
	}
	report, code, err := seedStoreWithOps(root, binary, streams{Out: io.Discard, Err: io.Discard}, ops)
	if err != nil || code != 0 || !report.Seeded {
		t.Fatalf("seed = %#v, code %d, err %v", report, code, err)
	}
	password := strings.Repeat("ab", 24)
	wantInput := "admin\n" + password + "\n127.0.0.1\n2222\n2026-08-27-fixture\n"
	wantArgv := binary + " init --seed|" + binary + " init --from tmp/session/2026-08-27-fixture/etc/ze/database.zefs"
	if strings.Join(gotArgv, "|") != wantArgv || gotInput != wantInput {
		t.Fatalf("init argv %q input %q", gotArgv, gotInput)
	}
	passwordPath := filepath.Join(root, filepath.FromSlash(report.Password))
	content, err := os.ReadFile(passwordPath)
	if err != nil || string(content) != password+"\n" {
		t.Fatalf("password = %q, err %v", content, err)
	}
	second, code, err := seedStoreWithOps(root, binary, streams{}, seedOps{
		environ: []string{"ze.config.dir=/must-not-matter-after-seeding"},
		random:  bytes.NewReader(nil),
		sleep:   func(time.Duration) {},
		run: func([]string, job.ProcessIO) (int, error) {
			t.Fatal("existing store ran the seeder")
			return 1, nil
		},
		waits: 1,
	})
	if err != nil || code != 0 || !second.Existing || second.Seeded {
		t.Fatalf("second seed = %#v, code %d, err %v", second, code, err)
	}
	after, err := os.ReadFile(passwordPath)
	if err != nil || string(after) != password+"\n" {
		t.Fatalf("password rotated to %q, err %v", after, err)
	}
}

func TestConcurrentSeedStoreRunsOneSeederAndBothObserveTheDatabase(t *testing.T) {
	root := t.TempDir()
	binary := filepath.ToSlash(filepath.Join("tmp", "session", "2026-08-27-fixture", "bin", "ze"))
	binaryPath := filepath.Join(root, filepath.FromSlash(binary))
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	waiting := make(chan struct{})
	release := make(chan struct{})
	var waitOnce sync.Once
	var calls atomic.Int32
	run := func(argv []string, _ job.ProcessIO) (int, error) {
		if argv[len(argv)-1] == "--seed" {
			if calls.Add(1) == 1 {
				close(started)
			}
			<-release
		}
		return fakeInit(root, argv)
	}
	ops := seedOps{
		environ: []string{"HOME=/home/test"},
		random:  bytes.NewReader(bytes.Repeat([]byte{0xab}, 48)),
		sleep: func(time.Duration) {
			waitOnce.Do(func() { close(waiting) })
			runtime.Gosched()
		},
		run:   run,
		waits: 10000,
	}
	type outcome struct {
		report seedReport
		code   int
		err    error
	}
	first := make(chan outcome, 1)
	second := make(chan outcome, 1)
	go func() {
		report, code, err := seedStoreWithOps(root, binary, streams{Out: io.Discard, Err: io.Discard}, ops)
		first <- outcome{report: report, code: code, err: err}
	}()
	<-started
	go func() {
		report, code, err := seedStoreWithOps(root, binary, streams{Out: io.Discard, Err: io.Discard}, ops)
		second <- outcome{report: report, code: code, err: err}
	}()
	<-waiting
	close(release)
	one := <-first
	two := <-second
	if one.err != nil || one.code != 0 || !one.report.Seeded {
		t.Fatalf("first seed = %#v, code %d, err %v", one.report, one.code, one.err)
	}
	if two.err != nil || two.code != 0 || !two.report.Existing || !two.report.Waited {
		t.Fatalf("second seed = %#v, code %d, err %v", two.report, two.code, two.err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("seeder process calls = %d, want 1", got)
	}
	password := filepath.Join(root, "tmp", "session", "2026-08-27-fixture", "etc", "ze", ".dev-password")
	info, err := os.Stat(password)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("password mode = %o, want 600", info.Mode().Perm())
	}
}

// fakeInit stands in for the two ze runs of the session seeder: `init --seed`
// writes the blob artifact, and `init --from` publishes the tree and retires
// the blob, as storage.ImportBlob does.
func fakeInit(root string, argv []string) (int, error) {
	etc := filepath.Join(root, "tmp", "session", "2026-08-27-fixture", "etc", "ze")
	blob := filepath.Join(etc, "database.zefs")
	switch strings.Join(argv[1:], " ") {
	case "init --seed":
		return 0, os.WriteFile(blob, []byte("seeded"), 0o600)
	case "init --from tmp/session/2026-08-27-fixture/etc/ze/database.zefs":
		if err := os.Mkdir(filepath.Join(etc, "database"), 0o700); err != nil {
			return 127, err
		}
		return 0, os.Rename(blob, blob+".replaced-fixture")
	}
	return 127, fmt.Errorf("unexpected ze argv %q", argv)
}

// sessionFixture creates the session binary and returns its root-relative path.
func sessionFixture(t *testing.T, root string) string {
	t.Helper()
	binary := filepath.ToSlash(filepath.Join("tmp", "session", "2026-08-27-fixture", "bin", "ze"))
	binaryPath := filepath.Join(root, filepath.FromSlash(binary))
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "tmp", "session", "2026-08-27-fixture", "etc", "ze"), 0o700); err != nil {
		t.Fatal(err)
	}
	return binary
}

// VALIDATES: a database.zefs blob in the session config directory is refused
// with the `ze init --from` repair, with or without a tree beside it.
// PREVENTS: the seeder reporting "existing" for a store `ze start` refuses to
// open, which is what a development box still holding a pre-cutover blob had.
func TestSeedStoreRefusesABlobInTheConfigDirectory(t *testing.T) {
	for _, withTree := range []bool{false, true} {
		root := t.TempDir()
		binary := sessionFixture(t, root)
		etc := filepath.Join(root, "tmp", "session", "2026-08-27-fixture", "etc", "ze")
		if err := os.WriteFile(filepath.Join(etc, "database.zefs"), []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
		if withTree {
			if err := os.Mkdir(filepath.Join(etc, "database"), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		report, code, err := seedStoreWithOps(root, binary, streams{}, seedOps{
			environ: []string{"HOME=/home/test"},
			random:  bytes.NewReader(bytes.Repeat([]byte{0xab}, 24)),
			sleep:   func(time.Duration) {},
			run: func([]string, job.ProcessIO) (int, error) {
				t.Fatal("a blob in the config directory ran the seeder")
				return 1, nil
			},
			waits: 1,
		})
		want := binary + " init --from tmp/session/2026-08-27-fixture/etc/ze/database.zefs"
		if err == nil || code != 1 || report.Existing || !strings.Contains(err.Error(), want) {
			t.Fatalf("tree %v: seed = %#v, code %d, err %v; want refusal naming %q", withTree, report, code, err, want)
		}
	}
}

// VALIDATES: a seed whose import leaves the blob in place fails.
// PREVENTS: reporting a store as seeded while the blob beside it makes ze
// refuse to open it.
func TestSeedStoreFailsWhenTheImportLeavesTheBlob(t *testing.T) {
	root := t.TempDir()
	binary := sessionFixture(t, root)
	etc := filepath.Join(root, "tmp", "session", "2026-08-27-fixture", "etc", "ze")
	report, code, err := seedStoreWithOps(root, binary, streams{}, seedOps{
		environ: []string{"HOME=/home/test"},
		random:  bytes.NewReader(bytes.Repeat([]byte{0xab}, 24)),
		sleep:   func(time.Duration) {},
		run: func(argv []string, _ job.ProcessIO) (int, error) {
			if argv[len(argv)-1] == "--seed" {
				return 0, os.WriteFile(filepath.Join(etc, "database.zefs"), []byte("seeded"), 0o600)
			}
			return 0, os.Mkdir(filepath.Join(etc, "database"), 0o700)
		},
		waits: 1,
	})
	if err == nil || code != 1 || report.Seeded || !strings.Contains(err.Error(), "init --from") {
		t.Fatalf("seed = %#v, code %d, err %v; want failure naming init --from", report, code, err)
	}
}

func TestEndSummaryKeepsThreeSnapshotsAndEveryNonSnapshotBlock(t *testing.T) {
	root := t.TempDir()
	paths := lepath.SessionPaths{ID: "fixture", Dir: filepath.ToSlash(filepath.Join("tmp", "session", "2026-08-27-fixture"))}
	marker := filepath.Join(root, "tmp", "session", ".session-fixture")
	if err := os.MkdirAll(filepath.Dir(marker), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("spec-native-session.md\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stateRel := stateFile(paths, "spec-native-session.md")
	statePath := filepath.Join(root, stateRel)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o750); err != nil {
		t.Fatal(err)
	}
	prior := "# Session State\n\n" +
		"## Session: newest\n\nBranch: `main`\n\n---\n" +
		"## Phase 4 handoff\nkept between\n\n---\n" +
		"## Session: middle\n\nBranch: `main`\n\n---\n" +
		"## Session: oldest\n\nBranch: `main`\n\n---\n" +
		"## Last Compaction\nkept after\n"
	if err := os.WriteFile(statePath, []byte(prior), 0o600); err != nil {
		t.Fatal(err)
	}
	compaction := filepath.Join(root, ".claude", ".compaction-detected-fixture")
	if err := os.MkdirAll(filepath.Dir(compaction), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(compaction, []byte("1"), 0o600); err != nil {
		t.Fatal(err)
	}
	query := summaryQuery(true)
	report, err := endSummary(root, paths, time.Date(2026, 8, 27, 14, 15, 16, 0, time.FixedZone("fixture", 3600)), query)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Written || report.Spec != "spec-native-session.md" {
		t.Fatalf("summary report = %#v", report)
	}
	content, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if strings.Count(text, "## Session:") != 3 {
		t.Fatalf("session snapshots = %d, want 3\n%s", strings.Count(text, "## Session:"), text)
	}
	for _, kept := range []string{"## Phase 4 handoff\nkept between", "## Last Compaction\nkept after", "## Session: newest", "## Session: middle"} {
		if !strings.Contains(text, kept) {
			t.Errorf("state did not preserve %q\n%s", kept, text)
		}
	}
	if strings.Contains(text, "## Session: oldest") {
		t.Fatalf("state kept the fourth snapshot\n%s", text)
	}
	if !strings.Contains(text, "## Session: 2026-08-27T14:15:16+01:00") {
		t.Fatalf("state timestamp changed format\n%s", text)
	}
	if _, err := os.Stat(compaction); !os.IsNotExist(err) {
		t.Fatalf("compaction marker still exists: %v", err)
	}
}

func TestEndSummaryMatchesTheStateFileFixtureBytes(t *testing.T) {
	root := t.TempDir()
	paths := lepath.SessionPaths{
		ID:  "fixture",
		Dir: filepath.ToSlash(filepath.Join("tmp", "session", "2026-08-27-fixture")),
	}
	marker := filepath.Join(root, "tmp", "session", ".session-fixture")
	if err := os.MkdirAll(filepath.Dir(marker), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("spec-native-session.md\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(root, stateFile(paths, "spec-native-session.md"))
	if err := os.MkdirAll(filepath.Dir(statePath), 0o750); err != nil {
		t.Fatal(err)
	}
	prior := "# Session State\n\n" +
		"## Session: prior\n\nBranch: `old`\n\n---\n" +
		"## Phase handoff\n  Staged:\nkeep this\n"
	if err := os.WriteFile(statePath, []byte(prior), 0o600); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 8, 27, 11, 12, 13, 0, time.UTC)
	if _, err := endSummary(root, paths, at, summaryQuery(true)); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Session State\n\n" +
		"## Session: 2026-08-27T11:12:13Z\n\n" +
		"Branch: `main`\n" +
		"Last commit: abcdef fixture\n" +
		"Spec: `spec-native-session.md`\n\n" +
		"Uncommitted:\n- `a.go`\n- `b.go`\n\n" +
		"Staged:\n- `staged.go`\n\n---\n" +
		"## Session: prior\n\nBranch: `old`\n\n---\n" +
		"## Phase handoff\n  Staged:\nkeep this\n"
	if string(content) != want {
		t.Fatalf("state bytes:\n%q\nwant:\n%q", content, want)
	}
}

func TestCleanEndSummaryChangesNeitherStateNorCompactionMarker(t *testing.T) {
	root := t.TempDir()
	paths := lepath.SessionPaths{ID: "clean", Dir: filepath.ToSlash(filepath.Join("tmp", "session", "2026-08-27-clean"))}
	statePath := filepath.Join(root, stateFile(paths, ""))
	if err := os.MkdirAll(filepath.Dir(statePath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte("prior\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	compaction := filepath.Join(root, ".claude", ".compaction-detected-clean")
	if err := os.MkdirAll(filepath.Dir(compaction), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(compaction, []byte("1"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := endSummary(root, paths, time.Now(), summaryQuery(false))
	if err != nil {
		t.Fatal(err)
	}
	if report.Written {
		t.Fatalf("clean summary wrote state: %#v", report)
	}
	content, err := os.ReadFile(statePath)
	if err != nil || string(content) != "prior\n" {
		t.Fatalf("clean state = %q, err %v", content, err)
	}
	if _, err := os.Stat(compaction); err != nil {
		t.Fatalf("clean summary removed compaction marker: %v", err)
	}
}

func summaryQuery(dirty bool) gitQuery {
	return func(_ string, args ...string) string {
		switch strings.Join(args, " ") {
		case "branch --show-current":
			return "main"
		case "log -1 --oneline":
			return "abcdef fixture"
		case "diff --name-only":
			if dirty {
				return "a.go\nb.go"
			}
		case "diff --cached --name-only":
			return "staged.go"
		case "status --porcelain":
			if dirty {
				return " M a.go"
			}
		}
		return ""
	}
}
