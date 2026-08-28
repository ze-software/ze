package devsetup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestClaudeServerIsOneRegisteredNativeSetupAction(t *testing.T) {
	found := 0
	for _, action := range Actions().Actions {
		if action.Verb != claudeServerVerb {
			continue
		}
		found++
		if !action.Writes {
			t.Errorf("claude-server action is not marked as writing: %#v", action)
		}
	}
	if found != 1 {
		t.Fatalf("claude-server action occurs %d times", found)
	}
}

func TestClaudeServerRefusesUnsupportedMachines(t *testing.T) {
	tests := []struct {
		name   string
		goos   string
		goarch string
		euid   int
		lookup func(string) (int, int, error)
		found  map[string]bool
		want   string
	}{
		{
			name: "operating system", goos: "darwin", goarch: "amd64",
			lookup: serverUserFound, found: serverPrerequisites(),
			want: "supports linux/amd64, got darwin/amd64",
		},
		{
			name: "architecture", goos: "linux", goarch: "arm64",
			lookup: serverUserFound, found: serverPrerequisites(),
			want: "supports linux/amd64, got linux/arm64",
		},
		{
			name: "root", goos: "linux", goarch: "amd64", euid: 1000,
			lookup: serverUserFound, found: serverPrerequisites(),
			want: "run as root (sudo le setup claude-server user alice)",
		},
		{
			name: "user", goos: "linux", goarch: "amd64",
			lookup: func(string) (int, int, error) { return 0, 0, errors.New("unknown user") },
			found:  serverPrerequisites(), want: "user \"alice\" does not exist",
		},
		{
			name: "apt", goos: "linux", goarch: "amd64",
			lookup: serverUserFound, found: map[string]bool{"sudo": true},
			want: "required command \"apt-get\" was not found",
		},
		{
			name: "sudo", goos: "linux", goarch: "amd64",
			lookup: serverUserFound, found: map[string]bool{"apt-get": true},
			want: "required command \"sudo\" was not found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			setup := &claudeServerSetup{
				User: "alice", GOOS: test.goos, GOARCH: test.goarch, Lookup: test.lookup,
				Shell: &Shell{
					Euid: func() int { return test.euid },
					Look: func(name string) (string, bool) { return name, test.found[name] },
					Exec: func(context.Context, Cmd) Result {
						calls++
						return Result{Code: 99, Err: "must not run"}
					},
				},
			}
			_, code, err := setup.setup()
			if code != 1 || err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Run() = code %d, error %v; want refusal containing %q", code, err, test.want)
			}
			if calls != 0 {
				t.Fatalf("refusal started %d processes", calls)
			}
		})
	}
}

func TestClaudeServerInstalledGoIsAnExactIdempotentMatch(t *testing.T) {
	goRoot := t.TempDir()
	calls := 0
	setup := &claudeServerSetup{
		Paths: ClaudeServerPaths{GoRoot: goRoot},
		Shell: &Shell{Exec: func(_ context.Context, cmd Cmd) Result {
			calls++
			return Result{Argv: cmd.Argv, Out: "go version go" + claudeServerGoVersion + " linux/amd64\n"}
		}},
		Fetch: func(context.Context, string, int64) ([]byte, error) {
			t.Fatal("an exact installed Go version was downloaded again")
			return nil, errors.New("unreachable")
		},
	}
	report := &claudeServerReport{}
	if err := setup.installGo(report); err != nil {
		t.Fatalf("installGo: %v", err)
	}
	if calls != 1 {
		t.Fatalf("the installed version was probed %d times", calls)
	}
	if !strings.Contains(report.Text(), "already installed, skipping") {
		t.Fatalf("installed result missing from report:\n%s", report.Text())
	}
}

func TestClaudeServerGoDownloadChecksumAndInstallFailures(t *testing.T) {
	archive := []byte("go archive fixture")
	digest := sha256.Sum256(archive)
	tests := []struct {
		name      string
		fetch     func(context.Context, string, int64) ([]byte, error)
		extract   func(string, string) error
		installed Result
		want      string
	}{
		{
			name:  "download",
			fetch: func(context.Context, string, int64) ([]byte, error) { return nil, errors.New("network down") },
			want:  "download Go " + claudeServerGoVersion + ": network down",
		},
		{
			name: "checksum download",
			fetch: func(_ context.Context, url string, _ int64) ([]byte, error) {
				if strings.HasSuffix(url, ".sha256") {
					return nil, errors.New("checksum unavailable")
				}
				return archive, nil
			},
			want: "download Go " + claudeServerGoVersion + " checksum: checksum unavailable",
		},
		{
			name: "checksum mismatch",
			fetch: func(_ context.Context, url string, _ int64) ([]byte, error) {
				if strings.HasSuffix(url, ".sha256") {
					return []byte(strings.Repeat("0", sha256.Size*2)), nil
				}
				return archive, nil
			},
			want: "verify Go " + claudeServerGoVersion + " checksum: SHA-256 mismatch",
		},
		{
			name: "install",
			fetch: func(_ context.Context, url string, _ int64) ([]byte, error) {
				if strings.HasSuffix(url, ".sha256") {
					return []byte(fmt.Sprintf("%x  go.tar.gz\n", digest)), nil
				}
				return archive, nil
			},
			extract: func(string, string) error { return errors.New("invalid tar header") },
			want:    "install Go " + claudeServerGoVersion + ": invalid tar header",
		},
		{
			name: "installed version verification",
			fetch: func(_ context.Context, url string, _ int64) ([]byte, error) {
				if strings.HasSuffix(url, ".sha256") {
					return []byte(fmt.Sprintf("%x  go.tar.gz\n", digest)), nil
				}
				return archive, nil
			},
			extract:   func(string, string) error { return nil },
			installed: Result{Out: "go version go1.25.0 linux/amd64\n"},
			want:      "verify installed Go: got version \"1.25.0\", want \"" + claudeServerGoVersion + "\"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			temporary := t.TempDir()
			extract := test.extract
			if extract == nil {
				extract = func(string, string) error {
					t.Fatal("an unverified download reached extraction")
					return nil
				}
			}
			probes := 0
			setup := &claudeServerSetup{
				Paths: ClaudeServerPaths{
					GoRoot:    filepath.Join(temporary, "usr", "local", "go"),
					GoArchive: filepath.Join(temporary, "tmp", "go.tar.gz"),
				},
				Shell: &Shell{Exec: func(_ context.Context, cmd Cmd) Result {
					probes++
					if probes > 1 && test.installed.Out != "" {
						test.installed.Argv = cmd.Argv
						return test.installed
					}
					return Result{Argv: cmd.Argv, Code: 1, Err: "not installed"}
				}},
				Fetch: test.fetch, Extract: extract,
			}
			err := setup.installGo(&claudeServerReport{})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("installGo error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestClaudeServerSetupSucceedsNativelyAndIsSafeToRepeat(t *testing.T) {
	temporary := t.TempDir()
	home := filepath.Join(temporary, "home", "alice")
	keySource := filepath.Join(temporary, "keys")
	repository := filepath.Join(home, "Code", "github.com", "ze-software", "ze", "main")
	for _, directory := range []string{home, keySource, filepath.Join(repository, ".git")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", directory, err)
		}
	}
	if err := os.WriteFile(filepath.Join(keySource, "id_ed25519"), []byte("PRIVATE\n"), 0o600); err != nil {
		t.Fatalf("private fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(keySource, "id_ed25519.pub"), []byte("ssh-ed25519 PUBLIC alice\n"), 0o644); err != nil {
		t.Fatalf("public fixture: %v", err)
	}

	goRoot := filepath.Join(temporary, "usr", "local", "go")
	goArchive := filepath.Join(temporary, "tmp", "go.tar.gz")
	nodeKey := filepath.Join(temporary, "etc", "apt", "keyrings", "nodesource.asc")
	nodeSource := filepath.Join(temporary, "etc", "apt", "sources.list.d", "nodesource.list")
	goArchiveContent := []byte("fixture Go distribution")
	goDigest := sha256.Sum256(goArchiveContent)
	states := map[string]bool{}
	var calls []Cmd
	fetches := 0

	shell := &Shell{
		Euid: func() int { return 0 },
		Look: func(name string) (string, bool) {
			return "/fixture/bin/" + name, name == "apt-get" || name == "sudo"
		},
		Exec: func(_ context.Context, cmd Cmd) Result {
			calls = append(calls, cmd)
			line := strings.Join(cmd.Argv, " ")
			switch {
			case line == filepath.Join(goRoot, "bin", "go")+" version":
				if states["go"] {
					return Result{Argv: cmd.Argv, Out: "go version go" + claudeServerGoVersion + " linux/amd64\n"}
				}
				return Result{Argv: cmd.Argv, Code: 1, Err: "missing"}
			case line == "golangci-lint --version":
				if states["lint"] {
					return Result{Argv: cmd.Argv, Out: "golangci-lint has version 2.13.1\n"}
				}
				return Result{Argv: cmd.Argv, Code: 1, Err: "missing"}
			case strings.Contains(line, "golangci-lint/v2/cmd/golangci-lint@"):
				states["lint"] = true
			case line == "staticcheck -version":
				if states["staticcheck"] {
					return Result{Argv: cmd.Argv, Out: "staticcheck " + StaticcheckVersion + " (v0.7.0)\n"}
				}
				return Result{Argv: cmd.Argv, Code: 1, Err: "missing"}
			case strings.Contains(line, "honnef.co/go/tools/cmd/staticcheck@"):
				states["staticcheck"] = true
			case line == "goimports --help":
				if states["goimports"] {
					return Result{Argv: cmd.Argv, Out: "usage: goimports\n"}
				}
				return Result{Argv: cmd.Argv, Code: 1, Err: "missing"}
			case strings.Contains(line, "golang.org/x/tools/cmd/goimports@"):
				states["goimports"] = true
			case line == "node --version":
				if states["node"] {
					return Result{Argv: cmd.Argv, Out: "v22.19.0\n"}
				}
				return Result{Argv: cmd.Argv, Code: 1, Err: "missing"}
			case line == "apt-get install -y -qq nodejs":
				states["node"] = true
			case line == "claude --version":
				if states["claude"] {
					return Result{Argv: cmd.Argv, Out: "2.1.0 (Claude Code)\n"}
				}
				return Result{Argv: cmd.Argv, Code: 1, Err: "missing"}
			case line == "npm install -g @anthropic-ai/claude-code":
				states["claude"] = true
			}
			return Result{Argv: cmd.Argv, Out: "ok\n"}
		},
	}
	setup := &claudeServerSetup{
		User: "alice", SSHKeyDir: keySource,
		Paths: ClaudeServerPaths{Home: home, GoRoot: goRoot, GoArchive: goArchive, NodeKey: nodeKey, NodeSource: nodeSource},
		Shell: shell, GOOS: "linux", GOARCH: "amd64",
		Lookup: func(name string) (int, int, error) {
			if name != "alice" {
				return 0, 0, errors.New("wrong user")
			}
			return os.Getuid(), os.Getgid(), nil
		},
		Fetch: func(_ context.Context, url string, _ int64) ([]byte, error) {
			fetches++
			switch {
			case strings.HasSuffix(url, ".sha256"):
				return []byte(fmt.Sprintf("%x  go.tar.gz\n", goDigest)), nil
			case strings.Contains(url, "go.dev/dl/"):
				return goArchiveContent, nil
			case strings.Contains(url, "nodesource-repo.gpg.key"):
				return []byte("NODE SOURCE KEY\n"), nil
			default:
				return nil, fmt.Errorf("unexpected download %s", url)
			}
		},
		Extract: func(archive, destination string) error {
			if archive != goArchive || destination != goRoot {
				return fmt.Errorf("extract %s to %s", archive, destination)
			}
			states["go"] = true
			return nil
		},
	}

	report, code, err := setup.setup()
	if err != nil || code != 0 {
		t.Fatalf("Run() = code %d, error %v\n%s", code, err, report.Text())
	}
	if fetches != 3 {
		t.Fatalf("download count = %d, want Go, checksum, and Node key", fetches)
	}
	if _, err := os.Stat(goArchive); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary Go archive was not removed: %v", err)
	}
	assertServerMode(t, filepath.Join(home, ".ssh"), 0o700)
	assertServerMode(t, filepath.Join(home, ".ssh", "id_ed25519"), 0o600)
	assertServerMode(t, filepath.Join(home, ".ssh", "id_ed25519.pub"), 0o644)
	assertServerMode(t, filepath.Join(home, ".bashrc"), 0o644)
	assertServerMode(t, nodeKey, 0o644)
	assertServerMode(t, nodeSource, 0o644)

	lines := serverCommandLines(calls)
	for _, want := range []string{
		"apt-get update -qq",
		"apt-get install -y -qq build-essential git curl wget jq unzip mosh",
		"go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@" + GolangCIVersion,
		"sudo -u alice env CGO_ENABLED=0 " + filepath.Join(goRoot, "bin", "go") + " install " + staticcheckTarget,
		"sudo -u alice env CGO_ENABLED=0 " + filepath.Join(goRoot, "bin", "go") + " install " + goimportsTarget,
		"apt-get install -y -qq nodejs",
		"npm install -g @anthropic-ai/claude-code",
	} {
		if !slices.Contains(lines, want) {
			t.Errorf("missing argv %q in\n%s", want, strings.Join(lines, "\n"))
		}
	}
	for _, call := range calls {
		if len(call.Argv) > 0 && (call.Argv[0] == "bash" || call.Argv[0] == "sh") {
			t.Fatalf("native setup invoked a shell: %v", call.Argv)
		}
	}

	firstCalls := len(calls)
	firstFetches := fetches
	repeated, repeatedCode, repeatedErr := setup.setup()
	if repeatedErr != nil || repeatedCode != 0 {
		t.Fatalf("second Run() = code %d, error %v\n%s", repeatedCode, repeatedErr, repeated.Text())
	}
	if fetches != firstFetches {
		t.Fatalf("second run downloaded %d additional files", fetches-firstFetches)
	}
	bashrc, err := os.ReadFile(filepath.Join(home, ".bashrc"))
	if err != nil {
		t.Fatalf("read .bashrc: %v", err)
	}
	for _, marker := range []string{"# Go environment", "# Claude alias", "# SSH agent", "# Land in ze dev repo"} {
		if count := strings.Count(string(bashrc), marker); count != 1 {
			t.Errorf("%q occurs %d times after two runs", marker, count)
		}
	}
	for _, line := range serverCommandLines(calls[firstCalls:]) {
		if strings.Contains(line, "golangci-lint@") || strings.Contains(line, "goimports@") || strings.HasPrefix(line, "npm install ") || line == "apt-get install -y -qq nodejs" {
			t.Errorf("idempotent branch reinstalled an already discovered tool: %s", line)
		}
	}
}

func TestClaudeServerFirewallDistinguishesInactiveFromActive(t *testing.T) {
	for _, active := range []bool{false, true} {
		t.Run(fmt.Sprintf("active=%t", active), func(t *testing.T) {
			var calls []Cmd
			status := "Status: inactive\n"
			if active {
				status = "Status: active\n"
			}
			shell := &Shell{
				Look: func(name string) (string, bool) { return name, name == "ufw" },
				Exec: func(_ context.Context, cmd Cmd) Result {
					calls = append(calls, cmd)
					if slices.Equal(cmd.Argv, []string{"ufw", "status"}) {
						return Result{Argv: cmd.Argv, Out: status}
					}
					return Result{Argv: cmd.Argv}
				},
			}
			setup := &claudeServerSetup{Shell: shell}
			if err := setup.configureFirewall(&claudeServerReport{}); err != nil {
				t.Fatalf("configureFirewall: %v", err)
			}
			hasEnable := slices.Contains(serverCommandLines(calls), "ufw enable")
			if hasEnable == active {
				t.Fatalf("ufw enable present = %t for active = %t", hasEnable, active)
			}
			if !active {
				for _, call := range calls {
					if slices.Equal(call.Argv, []string{"ufw", "enable"}) && string(call.Stdin) != "y\n" {
						t.Fatalf("ufw enable stdin = %q", call.Stdin)
					}
				}
			}
		})
	}
}

func TestClaudeServerGoExtractorPreservesExecutableModeAndRejectsEscape(t *testing.T) {
	t.Run("executable", func(t *testing.T) {
		temporary := t.TempDir()
		archive := filepath.Join(temporary, "go.tar.gz")
		writeServerTarFixture(t, archive, []serverTarEntry{{name: "go/bin/go", mode: 0o755, body: "go binary"}})
		destination := filepath.Join(temporary, "usr", "local", "go")
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			t.Fatalf("mkdir install parent: %v", err)
		}
		if err := extractGoArchive(archive, destination); err != nil {
			t.Fatalf("extractGoArchive: %v", err)
		}
		binary := filepath.Join(destination, "bin", "go")
		contents, err := os.ReadFile(binary)
		if err != nil {
			t.Fatalf("read installed Go: %v", err)
		}
		if string(contents) != "go binary" {
			t.Errorf("installed Go contents = %q", contents)
		}
		assertServerMode(t, binary, 0o755)
		assertServerMode(t, destination, 0o755)
	})

	t.Run("escape", func(t *testing.T) {
		temporary := t.TempDir()
		archive := filepath.Join(temporary, "go.tar.gz")
		writeServerTarFixture(t, archive, []serverTarEntry{{name: "go/../../escaped", mode: 0o644, body: "bad"}})
		destination := filepath.Join(temporary, "go")
		err := extractGoArchive(archive, destination)
		if err == nil || !strings.Contains(err.Error(), "outside go/") {
			t.Fatalf("escape error = %v", err)
		}
		if _, err := os.Stat(filepath.Join(temporary, "escaped")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("archive wrote outside its stage: %v", err)
		}
	})
}

type serverTarEntry struct {
	name string
	mode int64
	body string
}

func writeServerTarFixture(t *testing.T, path string, entries []serverTarEntry) {
	t.Helper()
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		header := &tar.Header{
			Name: entry.name, Mode: entry.mode, Size: int64(len(entry.body)), Typeflag: tar.TypeReg,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tarWriter.Write([]byte(entry.body)); err != nil {
			t.Fatalf("write tar body: %v", err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	if err := os.WriteFile(path, compressed.Bytes(), 0o644); err != nil {
		t.Fatalf("write tar fixture: %v", err)
	}
}

func serverUserFound(string) (int, int, error) { return 1000, 1000, nil }

func serverPrerequisites() map[string]bool { return map[string]bool{"apt-get": true, "sudo": true} }

func serverCommandLines(calls []Cmd) []string {
	lines := make([]string, 0, len(calls))
	for _, call := range calls {
		lines = append(lines, strings.Join(call.Argv, " "))
	}
	return lines
}

func assertServerMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s mode = %#o, want %#o", path, got, want)
	}
}
