// VALIDATES: native child execution preserves ordinary exits and shell-compatible
// signal exits without routing through a shell process.
// PREVENTS: verify-lock or session seeding flattening a child's status accidentally.
package job

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestMain lets this executable stand in for Go's test launcher under a
// disposable name, without a shell discarding dotted names before the probe.
func TestMain(m *testing.M) {
	if os.Getenv("LEJOB_CHECKOUT_PROBE") != "" {
		isEnvQuery := len(os.Args) >= 3 && os.Args[len(os.Args)-2] == "env" && os.Args[len(os.Args)-1] == "GOFLAGS"
		if isEnvQuery {
			argv := append([]string{os.Getenv("LEJOB_REAL_GO")}, os.Args[1:]...)
			code, err := RunProcess(argv, ProcessIO{Stdout: os.Stdout, Stderr: os.Stderr})
			if err != nil {
				if _, writeErr := os.Stderr.WriteString(err.Error()); writeErr != nil {
					os.Exit(98)
				}
			}
			os.Exit(code)
		}
		os.Args = []string{os.Args[0], "-test.run=^TestGoTestChildRootProbe$"}
	}
	os.Exit(m.Run())
}

// TestGoTestEnvironmentDropsOnlyLauncherIdentity covers direct job commands,
// including both real execution boundaries, without admitting work or touching git.
func TestGoTestEnvironmentDropsOnlyLauncherIdentity(t *testing.T) {
	inherited := []string{
		"ZE_REPO_ROOT=outer", "ze.repo.root=other", "Ze.RePo_Root=mixed",
		"ZE_LE_BUILD_NAME=outer", "ze.le.build.name=other", "Ze.Le_Build.Name=mixed",
		"PATH=/keep", "ZE_RUN_JOB=/keep-job",
		"GOFLAGS=-race=false",
	}
	for _, command := range [][]string{
		{"go", "test", "./..."},
		{"/tools/go", "test", "./..."},
		{"go", "-C", "/fixture", "test", "./..."},
		{"go", "-C=/fixture", "test", "./..."},
	} {
		got, err := commandEnvironment(command, "", inherited)
		if err != nil {
			t.Fatalf("%v environment: %v", command, err)
		}
		if len(got) != 4 {
			t.Fatalf("%v inherited launcher identity: %v", command, got)
		}
		if got[0] != inherited[6] || got[1] != inherited[7] {
			t.Errorf("%v lost unrelated environment: %v", command, got)
		}
		if got[2] != inherited[8] {
			t.Errorf("%v lost explicit GOFLAGS: %q", command, got[2])
		}
		if got[3] != "CGO_ENABLED=0" {
			t.Errorf("%v ordinary test CGO = %q, want CGO_ENABLED=0", command, got[3])
		}
	}
	for _, command := range [][]string{
		{"le", "test", "unit", "all"}, {"go", "build"}, {"go"}, nil,
		{"go", "-C", "/fixture"}, {"go", "-C=/fixture"},
	} {
		got, err := commandEnvironment(command, "", inherited)
		if err != nil {
			t.Fatalf("%v environment: %v", command, err)
		}
		if len(got) != len(inherited) {
			t.Errorf("%v lost production launch context: %v", command, got)
		}
	}

	root := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(root, "go")
	if err := os.Symlink(binary, launcher); err != nil {
		t.Fatalf("create disposable Go test launcher: %v", err)
	}
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("find Go for effective configuration probes: %v", err)
	}
	for _, one := range []struct {
		name      string
		args      []string
		inherited string
		flags     string
		want      string
	}{
		{name: "ordinary test", args: []string{"./fixture"}, inherited: "1", want: "0"},
		{name: "race", args: []string{"-race", "./fixture"}, inherited: "0", want: "1"},
		{name: "explicit race", args: []string{"-race=true", "./fixture"}, inherited: "0", want: "1"},
		{name: "explicit false", args: []string{"-race=false", "./fixture"}, inherited: "1", want: "0"},
		{name: "last false wins", args: []string{"-race", "-race=false", "./fixture"}, inherited: "1", want: "0"},
		{name: "last true wins", args: []string{"-race=false", "-race", "./fixture"}, inherited: "0", want: "1"},
		{name: "test arguments", args: []string{"./fixture", "-args", "-race"}, inherited: "1", want: "0"},
		{name: "race before test arguments", args: []string{"-race", "./fixture", "-args", "-race=false"}, inherited: "0", want: "1"},
		{name: "run expression", args: []string{"-run", "-race", "./fixture"}, inherited: "1", want: "0"},
		{name: "flag value then race", args: []string{"-run", "-race=false", "-race", "./fixture"}, inherited: "0", want: "1"},
		{name: "build flag value", args: []string{"-ldflags", "-race", "./fixture"}, inherited: "1", want: "0"},
		{name: "boolean flag then race", args: []string{"-v", "-race", "./fixture"}, inherited: "0", want: "1"},
		{name: "double dash race", args: []string{"--race=1", "./fixture"}, inherited: "0", want: "1"},
		{name: "double dash arguments", args: []string{"./fixture", "--args", "-race"}, inherited: "1", want: "0"},
		{name: "flag terminator", args: []string{"./fixture", "--", "-race"}, inherited: "1", want: "0"},
		{name: "positional tail keeps race", args: []string{"-race", "./fixture", "-v", "positional", "-race=false"}, inherited: "0", want: "1"},
		{name: "positional tail cannot enable race", args: []string{"./fixture", "-v", "positional", "-race"}, inherited: "1", want: "0"},
		{name: "unknown flag value before race", args: []string{"./fixture", "-custom", "value", "-race"}, inherited: "0", want: "1"},
		{name: "unknown assigned flag closes packages", args: []string{"./fixture", "-custom=value", "positional", "-race"}, inherited: "1", want: "0"},
		{name: "environment enables race", args: []string{"./fixture"}, inherited: "0", flags: "-race", want: "1"},
		{name: "argv disables environment race", args: []string{"-race=false", "./fixture"}, inherited: "1", flags: "-race", want: "0"},
		{name: "argv enables environment false", args: []string{"-race", "./fixture"}, inherited: "0", flags: "-race=false", want: "1"},
		{name: "environment last flag wins", args: []string{"./fixture"}, inherited: "1", flags: "-race -race=false", want: "0"},
		{name: "quoted environment race", args: []string{"./fixture"}, inherited: "0", flags: "'-race'", want: "1"},
		{name: "quoted environment with spaces", args: []string{"./fixture"}, inherited: "0", flags: "\"-ldflags=-X main.probe=-race=false\" '--race=true'", want: "1"},
		{name: "environment value is not race", args: []string{"./fixture"}, inherited: "1", flags: "'-ldflags=-race'", want: "0"},
		{name: "environment race before positional tail", args: []string{"./fixture", "-v", "positional", "-race=false"}, inherited: "0", flags: "-race", want: "1"},
	} {
		t.Run(one.name, func(t *testing.T) {
			childEnv := inherited
			childEnv = append(childEnv, "LEJOB_CHECKOUT_PROBE="+root,
				"CGO_ENABLED="+one.inherited, "LEJOB_EXPECT_CGO="+one.want,
				"GOFLAGS=-race", "GOFLAGS="+one.flags, "LEJOB_EXPECT_GOFLAGS="+one.flags,
				"GOENV=off", "GOTOOLCHAIN=local", "LEJOB_REAL_GO="+realGo,
				"HOME="+root, "GOCACHE="+filepath.Join(root, "cache"))
			argv := append([]string{launcher, "test"}, one.args...)
			t.Run("RunProcess", func(t *testing.T) {
				var output bytes.Buffer
				code, err := RunProcess(argv, ProcessIO{
					Dir: root, Environ: childEnv, Stdout: &output, Stderr: &output,
				})
				if err != nil || code != 0 {
					t.Fatalf("RunProcess probe = %d, %v:\n%s", code, err, output.String())
				}
				if !strings.Contains(output.String(), "checkout-probe-ok") {
					t.Fatalf("RunProcess did not execute the root probe:\n%s", output.String())
				}
			})
			t.Run("Admission.stream", func(t *testing.T) {
				var output bytes.Buffer
				admission := Admission{Root: root, Out: &output}
				if code := admission.stream(argv, root, childEnv, nil); code != 0 {
					t.Fatalf("Admission.stream probe = %d:\n%s", code, output.String())
				}
				if !strings.Contains(output.String(), "checkout-probe-ok") {
					t.Fatalf("Admission.stream did not execute the root probe:\n%s", output.String())
				}
			})
		})
	}
}

// TestGoTestChildRootProbe proves the executed test child owns its root and
// build identity, gets the requested CGO mode, and retains the parent-job setting.
func TestGoTestChildRootProbe(t *testing.T) {
	root := os.Getenv("LEJOB_CHECKOUT_PROBE")
	if root == "" {
		return
	}
	t.Cleanup(env.ResetCache)
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	if got, err := lepath.Root(); err != nil || got != root {
		t.Fatalf("child Root = %q, %v, want fixture %q", got, err, root)
	}
	for _, entry := range os.Environ() {
		name, value, _ := strings.Cut(entry, "=")
		if strings.EqualFold(strings.ReplaceAll(name, "_", "."), "ze.le.build.name") {
			if value != "" {
				t.Fatalf("child inherited outer build identity %s", entry)
			}
		}
	}
	if got := env.Get(ParentKey); got != "/keep-job" {
		t.Fatalf("parent job setting = %q, want /keep-job", got)
	}
	if got, want := os.Getenv("CGO_ENABLED"), os.Getenv("LEJOB_EXPECT_CGO"); got != want {
		t.Fatalf("child CGO_ENABLED = %q, want %q", got, want)
	}
	if got, want := os.Getenv("GOFLAGS"), os.Getenv("LEJOB_EXPECT_GOFLAGS"); got != want {
		t.Fatalf("child GOFLAGS = %q, want preserved %q", got, want)
	}
	if _, err := os.Stdout.WriteString("checkout-probe-ok\n"); err != nil {
		t.Fatalf("publish executed probe: %v", err)
	}
}

func TestRunProcessPreservesExitAndSignalStatus(t *testing.T) {
	if os.Getenv("LEJOB_PROCESS_HELPER") == "1" {
		processHelper()
		return
	}
	environ := append(os.Environ(), "LEJOB_PROCESS_HELPER=1")
	for _, test := range []struct {
		name string
		mode string
		want int
	}{
		{name: "exit", mode: "37", want: 37},
		{name: "signal", mode: "term", want: 128 + int(syscall.SIGTERM)},
	} {
		t.Run(test.name, func(t *testing.T) {
			code, err := RunProcess(
				[]string{os.Args[0], "-test.run=^TestRunProcessPreservesExitAndSignalStatus$", "--", test.mode},
				ProcessIO{Dir: t.TempDir(), Environ: environ, Stdin: nil, Stdout: io.Discard, Stderr: io.Discard},
			)
			if err != nil || code != test.want {
				t.Fatalf("RunProcess = code %d err %v, want code %d", code, err, test.want)
			}
		})
	}
}

func processHelper() {
	mode := os.Args[len(os.Args)-1]
	if mode == "term" {
		if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
			os.Exit(96)
		}
		os.Exit(95)
	}
	code, err := strconv.Atoi(mode)
	if err != nil {
		os.Exit(97)
	}
	os.Exit(code)
}

// TestGoTestUsesPersistentRaceConfiguration compiles a disposable module with
// real Go. The race build tag, not a probe's interpretation of GOFLAGS, supplies
// the asserted outcome through both production execution paths.
func TestGoTestUsesPersistentRaceConfiguration(t *testing.T) {
	goBinary, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("find Go: %v", err)
	}
	root := t.TempDir()
	config := filepath.Join(root, "go.env")
	for name, body := range map[string]string{
		"go.mod":           "module example.test/goenvprobe\n\ngo 1.20\n",
		"go.env":           "GOFLAGS=-race\n",
		"probe_test.go":    goEnvironmentCompilerProbe,
		"race_on_test.go":  "//go:build race\n\npackage probe\n\nconst raceEnabled = true\n",
		"race_off_test.go": "//go:build !race\n\npackage probe\n\nconst raceEnabled = false\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write disposable Go fixture %s: %v", name, err)
		}
	}
	for _, one := range []struct {
		name string
		env  []string
		args []string
		want string
	}{
		{name: "absent GOFLAGS reads GOENV", want: "1"},
		{name: "empty GOFLAGS reads GOENV", env: []string{"GOFLAGS="}, want: "1"},
		{name: "explicit false overrides GOENV", env: []string{"GOFLAGS="}, args: []string{"-race=false"}, want: "0"},
		{name: "nonempty OS override wins", env: []string{"GOFLAGS=-race=false"}, want: "0"},
	} {
		t.Run(one.name, func(t *testing.T) {
			environ := env.Without(os.Environ(), "GOFLAGS", "GOENV", "ze.repo.root", "ze.le.build.name")
			environ = append(environ,
				"GOENV="+config, "GOTOOLCHAIN=local", "GOWORK=off", "CGO_ENABLED=0",
				"ZE_REPO_ROOT=outer", "ze.repo.root=other", "Ze.RePo_Root=mixed",
				"ZE_LE_BUILD_NAME=outer", "ze.le.build.name=other", "Ze.Le_Build.Name=mixed",
				"ZE_RUN_JOB=/keep-job", "LEJOB_EXPECT_RACE="+one.want)
			environ = append(environ, one.env...)
			argv := append([]string{goBinary, "test", "-count=1", "-v"}, one.args...)
			argv = append(argv, ".")
			t.Run("RunProcess", func(t *testing.T) {
				var output bytes.Buffer
				code, err := RunProcess(argv, ProcessIO{
					Dir: root, Environ: environ, Stdout: &output, Stderr: &output,
				})
				if err != nil || code != 0 {
					t.Fatalf("real Go probe = %d, %v:\n%s", code, err, output.String())
				}
				if !strings.Contains(output.String(), "goenv-compiler-probe-ok") {
					t.Fatalf("real Go did not execute the compiled probe:\n%s", output.String())
				}
			})
			t.Run("Admission.stream", func(t *testing.T) {
				var output bytes.Buffer
				admission := Admission{Root: root, Out: &output}
				if code := admission.stream(argv, root, environ, nil); code != 0 {
					t.Fatalf("real Go stream probe = %d:\n%s", code, output.String())
				}
				if !strings.Contains(output.String(), "goenv-compiler-probe-ok") {
					t.Fatalf("real Go stream did not execute the compiled probe:\n%s", output.String())
				}
			})
		})
	}
}

// TestExplicitRaceDoesNotQueryGoConfiguration uses a nonexistent Go executable
// to prove an explicit race decision requires no configuration subprocess.
func TestExplicitRaceDoesNotQueryGoConfiguration(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "go")
	for _, one := range []struct{ flag, want string }{
		{"-race", "CGO_ENABLED=1"},
		{"-race=false", "CGO_ENABLED=0"},
	} {
		environ, err := commandEnvironment([]string{missing, "test", one.flag}, "", []string{"GOENV=off"})
		if err != nil {
			t.Fatalf("%s queried Go: %v", one.flag, err)
		}
		if got := environ[len(environ)-1]; got != one.want {
			t.Errorf("%s selected %q, want %q", one.flag, got, one.want)
		}
	}
}

// TestGoTestConfigurationFailureStopsExecution proves both execution paths
// expose a configuration-resolution failure instead of inventing a CGO mode.
func TestGoTestConfigurationFailureStopsExecution(t *testing.T) {
	root := t.TempDir()
	argv := []string{filepath.Join(root, "go"), "test", "."}
	environ := []string{"GOENV=off"}
	code, err := RunProcess(argv, ProcessIO{Dir: root, Environ: environ})
	if err == nil {
		t.Fatal("RunProcess hid the configuration failure")
	}
	if code != 127 || !strings.Contains(err.Error(), "resolve Go test GOFLAGS") {
		t.Fatalf("RunProcess = %d, %v, want configuration failure", code, err)
	}
	var output bytes.Buffer
	admission := Admission{Root: root, Out: &output, Err: &output}
	if code := admission.stream(argv, root, environ, nil); code != 127 {
		t.Fatalf("Admission.stream = %d, want 127", code)
	}
	if !strings.Contains(output.String(), "resolve Go test GOFLAGS") {
		t.Fatalf("Admission.stream hid the configuration failure: %s", output.String())
	}
}

const goEnvironmentCompilerProbe = `package probe

import (
	"os"
	"testing"
)

func TestConfiguredRace(t *testing.T) {
	want := os.Getenv("LEJOB_EXPECT_RACE")
	if got := os.Getenv("CGO_ENABLED"); got != want {
		t.Fatalf("CGO_ENABLED = %q, want %q", got, want)
	}
	if raceEnabled != (want == "1") {
		t.Fatalf("compiled race tag = %v, want %q", raceEnabled, want)
	}
	for _, name := range []string{
		"ZE_REPO_ROOT", "ze.repo.root", "Ze.RePo_Root",
		"ZE_LE_BUILD_NAME", "ze.le.build.name", "Ze.Le_Build.Name",
	} {
		if _, exists := os.LookupEnv(name); exists {
			t.Fatalf("compiled fixture inherited launcher setting %s", name)
		}
	}
	if got := os.Getenv("ZE_RUN_JOB"); got != "/keep-job" {
		t.Fatalf("parent job = %q, want /keep-job", got)
	}
	t.Log("goenv-compiler-probe-ok")
}
`
