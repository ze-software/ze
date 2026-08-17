// Smoke test for the //go:build ignore checker in this directory.
//
// command_ownership.go uses //go:build ignore so it is excluded from normal
// compilation and from golangci-lint's type-checking pipeline. This test file
// does NOT have the ignore tag, so it is the only buildable file in the package
// and gives the linter and verify-changed a real target. It runs the checker as
// a subprocess and asserts the current tree passes the command-surface-ownership
// gate.

package main

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

const checkTimeout = 60 * time.Second

// TestNoOwnerAllowlistIsEnforced runs scripts/checks/command_ownership.go and
// asserts the repository passes the ownership gate: no owner command package
// imports cmd/ze (TestOwnerCommandRegistrationHasNoCmdZeImport), every
// RegisterRootHandler lives in an internal owner, and every central root is in
// the no-owner allowlist (AC-1, AC-2, AC-4, AC-8).
func TestNoOwnerAllowlistIsEnforced(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/checks/command_ownership.go")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command-ownership gate failed (the migration left a violation):\n%s", out)
	}
	if !strings.Contains(string(out), "command-ownership: OK") {
		t.Fatalf("command_ownership.go did not report OK:\n%s", out)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

func walkFirstPartyFiles(t *testing.T, visit func(string, []byte)) {
	t.Helper()
	root := repoRoot(t)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if entry.Name() == "testdata" {
				return fs.SkipDir
			}
			switch rel {
			case ".git", ".cache", "vendor", "third_party", "tmp", "cache", "bin", "build", "dist", "node_modules", ".venv", "gokrazy/modcache", "gokrazy/ze/builddir", ".claude/worktrees":
				return fs.SkipDir
			}
			return nil
		}
		base := entry.Name()
		ext := filepath.Ext(base)
		// Scan authored usage documentation. ai/*.md command indexes are rendered
		// artifacts; their source points are the owned surface and regeneration is
		// deliberately outside this structural check.
		relevant := ext == ".go" || ext == ".py" || ext == ".mk" || ext == ".sh" ||
			ext == ".ci" || ext == ".yml" || ext == ".yaml" || base == "Makefile" ||
			strings.HasPrefix(base, "Dockerfile") || rel == "README.md" ||
			strings.HasPrefix(rel, "docs/") ||
			rel == "test/exabgp-compat/bin/test-migrate"
		if !relevant {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		visit(rel, data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk first-party files: %v", err)
	}
}

// isExternalGoProducerPath derives exclusions from ownership boundaries, not
// individual files. Interop Dockerfiles other than Dockerfile.ze build
// third-party daemons, and external-plugin trees document/build consumer code.
func isExternalGoProducerPath(rel string) bool {
	if strings.HasPrefix(rel, "examples/external-plugin/") ||
		strings.HasPrefix(rel, "docs/plugin-development/") {
		return true
	}
	return strings.HasPrefix(rel, "test/interop/Dockerfile.") &&
		rel != "test/interop/Dockerfile.ze"
}

var goProducerCommandRE = regexp.MustCompile(
	`(^|[^[:alnum:]_])(go|\$\(GO\))[[:space:]]+(build|install|test|run)([[:space:]]|$)`,
)

var cgoEnabledOneRE = regexp.MustCompile(`CGO_ENABLED[[:space:]]*([:?+]?=|:)[[:space:]]*["']?1([^0-9]|$)`)

var raceFlagRE = regexp.MustCompile(`(^|[[:space:]"'])-race([[:space:]"',\\]|$)`)

func shellCGOOneAllowed(producerKind, command string) bool {
	return producerKind == "test" && raceFlagRE.MatchString(command)
}

func pythonCGOOneAllowed(rel string, build bool, call string) bool {
	if !raceFlagRE.MatchString(call) {
		return false
	}
	return rel == "scripts/dev/stress-repro.py" && build
}

func pythonEnablesCGO(scope string) bool {
	return strings.Contains(scope, `"CGO_ENABLED": "1"`) ||
		strings.Contains(scope, `'CGO_ENABLED': '1'`) ||
		strings.Contains(scope, `env["CGO_ENABLED"] = "1"`) ||
		strings.Contains(scope, `env['CGO_ENABLED'] = '1'`) ||
		strings.Contains(scope, `CGO_ENABLED="1"`)
}

// firstGoProducer matches command tokens. Substring matching is incorrect:
// "cargo build" contains "go build" but does not invoke the Go tool.
func firstGoProducer(command string) (int, string, bool) {
	match := goProducerCommandRE.FindStringSubmatchIndex(command)
	if match == nil {
		return -1, "", false
	}
	return match[3], command[match[6]:match[7]], true
}

func pythonFunctionScope(lines []string, at int) string {
	start := 0
	for i := at; i >= 0; i-- {
		if strings.HasPrefix(lines[i], "def ") || strings.HasPrefix(lines[i], "async def ") {
			start = i
			break
		}
	}
	end := len(lines)
	for i := at + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "def ") || strings.HasPrefix(lines[i], "async def ") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

// VALIDATES: every first-party shell, workflow, Make, Docker, documentation,
// Python, and go:generate producer explicitly disables cgo, except a derived
// test-only process whose same command actually uses -race.
// PREVENTS: inherited cgo and cgo-enabled release/build-evidence producers.
func TestFirstPartyGoProducerCommandsDisableCGO(t *testing.T) {
	var shellProducers, shellRunProducers, functionalCIProducers, pythonProducers, pythonRunProducers, usageRunCommands int
	makefile := string(mustReadFile(t, filepath.Join(repoRoot(t), "Makefile")))
	if !strings.Contains(makefile, "export CGO_ENABLED := 0") {
		t.Fatal("Makefile must export CGO_ENABLED := 0 for included recipes")
	}
	if _, _, found := firstGoProducer("cargo build --release"); found {
		t.Fatal("Go producer matcher must not treat cargo build as go build")
	}
	if _, kind, found := firstGoProducer("CGO_ENABLED=0 go test ./..."); !found || kind != "test" {
		t.Fatalf("Go producer matcher missed tokenized go test: found=%v kind=%q", found, kind)
	}
	if !shellCGOOneAllowed("test", "CGO_ENABLED=1 go test -race ./...") {
		t.Fatal("race-instrumented test command must permit explicit cgo enablement")
	}
	if shellCGOOneAllowed("test", "CGO_ENABLED=1 go test ./...") {
		t.Fatal("non-race test command must not permit cgo enablement")
	}
	if shellCGOOneAllowed("build", "CGO_ENABLED=1 go build -race ./cmd/ze") {
		t.Fatal("race-enabled build command must not be treated as a test-only shell producer")
	}
	if !cgoEnabledOneRE.MatchString("CGO_ENABLED: 1") {
		t.Fatal("cgo enablement matcher must recognize workflow environment syntax")
	}
	if !pythonCGOOneAllowed("scripts/dev/stress-repro.py", true, `["go", "build", "-race"]`) {
		t.Fatal("test-only Python stress race build must permit explicit cgo enablement")
	}
	if pythonCGOOneAllowed("scripts/evidence/release.py", true, `["go", "build", "-race"]`) {
		t.Fatal("Python race builds must not receive a general shipping/evidence exemption")
	}
	if pythonCGOOneAllowed("scripts/dev/stress-repro.py", true, `["go", "build"]`) {
		t.Fatal("Python stress build without -race must not permit cgo enablement")
	}
	walkFirstPartyFiles(t, func(rel string, data []byte) {
		if isExternalGoProducerPath(rel) {
			return
		}
		base := filepath.Base(rel)
		ext := filepath.Ext(base)
		makeLike := base == "Makefile" || ext == ".mk"
		if ext == ".ci" && strings.Contains(string(data), "var=CGO_ENABLED") {
			t.Errorf("%s sets CGO_ENABLED through a functional env directive; childEnv owns that boundary", rel)
		}
		shellLike := makeLike || ext == ".sh" || ext == ".ci" ||
			ext == ".yml" || ext == ".yaml" || strings.HasPrefix(base, "Dockerfile") ||
			rel == "README.md" || strings.HasPrefix(rel, "docs/")
		if shellLike {
			var logical string
			inCodeFence := ext != ".md"
			for lineNumber, line := range strings.Split(string(data), "\n") {
				trimmed := strings.TrimSpace(line)
				if ext == ".md" && strings.HasPrefix(trimmed, "```") {
					inCodeFence = !inCodeFence
					continue
				}
				if !inCodeFence {
					continue
				}
				if logical == "" && strings.HasPrefix(trimmed, "#") {
					continue
				}
				logical += " " + strings.TrimSuffix(trimmed, "\\")
				if strings.HasSuffix(trimmed, "\\") {
					continue
				}
				segments := strings.Split(strings.ReplaceAll(logical, "&&", ";"), ";")
				logical = ""
				for _, segment := range segments {
					producerAt, producerKind, found := firstGoProducer(segment)
					if gokAt := strings.Index(segment, "bin/gok --parent_dir"); gokAt >= 0 &&
						(!found || gokAt < producerAt) {
						producerAt, producerKind, found = gokAt, "build", true
					}
					if !found {
						if cgoEnabledOneRE.MatchString(segment) {
							t.Errorf("%s:%d cgo enablement is not attached to a derived race-test command: %s", rel, lineNumber+1, strings.TrimSpace(segment))
						}
						continue
					}
					if producerAt < 0 {
						continue
					}
					shellProducers++
					if producerKind == "run" {
						shellRunProducers++
					}
					if ext == ".ci" {
						functionalCIProducers++
						if cgoEnabledOneRE.MatchString(segment) {
							t.Errorf("%s:%d functional command enables cgo: %s", rel, lineNumber+1, strings.TrimSpace(segment))
						}
						continue
					}
					prefix := segment[:producerAt]
					if cgoEnabledOneRE.MatchString(prefix) {
						if !shellCGOOneAllowed(producerKind, segment[producerAt:]) {
							t.Errorf("%s:%d non-race or non-test Go producer enables cgo: %s", rel, lineNumber+1, strings.TrimSpace(segment))
						}
						continue
					}
					zeroAt := strings.Index(prefix, "CGO_ENABLED=0")
					overridesCGO := strings.Contains(prefix, "CGO_ENABLED=")
					containerized := strings.Contains(prefix, "docker run") || strings.Contains(prefix, "docker exec") ||
						strings.Contains(prefix, "podman run") || strings.Contains(prefix, "podman exec")
					inheritsMakeZero := makeLike && !overridesCGO && !containerized
					if zeroAt < 0 && !inheritsMakeZero {
						t.Errorf("%s:%d Go producer does not set CGO_ENABLED=0: %s", rel, lineNumber+1, strings.TrimSpace(segment))
					}
				}
			}
		}

		allLines := strings.Split(string(data), "\n")
		for lineNumber, line := range allLines {
			trimmed := strings.TrimSpace(line)
			goRunAt, producerKind, found := firstGoProducer(line)
			if !found || producerKind != "run" {
				continue
			}
			commentBody := strings.TrimSpace(strings.TrimPrefix(trimmed, "//"))
			documentedUsage := strings.Contains(line, "Usage:") ||
				strings.Contains(line, "Run:") ||
				strings.Contains(line, "or:") ||
				strings.HasPrefix(commentBody, "go run ") ||
				strings.HasPrefix(commentBody, "CGO_ENABLED=") ||
				strings.Contains(line, "//go:generate") ||
				(ext == ".md" && strings.Contains(line, "| `"))
			if !documentedUsage {
				continue
			}
			usageRunCommands++
			if !strings.Contains(line[:goRunAt], "CGO_ENABLED=0") {
				t.Errorf("%s:%d independently runnable go run usage does not set CGO_ENABLED=0", rel, lineNumber+1)
			}
		}

		if (ext != ".py" && rel != "test/exabgp-compat/bin/test-migrate") ||
			strings.HasSuffix(rel, "_test.py") {
			return
		}
		lines := allLines
		for i, line := range lines {
			commandHead := strings.Join(lines[i:min(len(lines), i+6)], "\n")
			multilineGo := strings.TrimSpace(line) == `"go",` || strings.TrimSpace(line) == `'go',`
			build := strings.Contains(line, `["go", "build"`) || strings.Contains(line, `['go', 'build'`) ||
				(multilineGo && (strings.Contains(commandHead, `"build",`) || strings.Contains(commandHead, `'build',`)))
			install := strings.Contains(line, `["go", "install"`) || strings.Contains(line, `['go', 'install'`) ||
				(multilineGo && (strings.Contains(commandHead, `"install",`) || strings.Contains(commandHead, `'install',`)))
			run := strings.Contains(line, `["go", "run"`) || strings.Contains(line, `['go', 'run'`) ||
				strings.Contains(line, `("go", "run"`) || strings.Contains(line, `('go', 'run'`) ||
				(multilineGo && (strings.Contains(commandHead, `"run",`) || strings.Contains(commandHead, `'run',`)))
			compiledTest := multilineGo &&
				((strings.Contains(commandHead, `"test",`) && strings.Contains(commandHead, `"-c",`)) ||
					(strings.Contains(commandHead, `'test',`) && strings.Contains(commandHead, `'-c',`)))
			if !build && !install && !run && !compiledTest {
				continue
			}
			pythonProducers++
			if run {
				pythonRunProducers++
			}
			end := min(len(lines), i+24)
			call := strings.Join(lines[i:end], "\n")
			if !strings.Contains(call, "env=") {
				t.Errorf("%s:%d Go producer does not pass an explicit environment", rel, i+1)
				continue
			}
			scope := pythonFunctionScope(lines, i)
			copiesParent := strings.Contains(scope, "os.environ.copy()") ||
				strings.Contains(scope, "{**os.environ") ||
				strings.Contains(scope, "dict(os.environ")
			if !copiesParent {
				t.Errorf("%s:%d Go producer environment does not copy the parent environment", rel, i+1)
			}
			enablesCGO := pythonEnablesCGO(scope)
			if enablesCGO {
				sameProcessEnablesCGO := pythonEnablesCGO(call)
				if !sameProcessEnablesCGO || !pythonCGOOneAllowed(rel, build, call) {
					t.Errorf("%s:%d non-race or non-test Go producer environment enables cgo", rel, i+1)
				}
				continue
			}
			zero := strings.Contains(scope, `"CGO_ENABLED": "0"`) ||
				strings.Contains(scope, `'CGO_ENABLED': '0'`) ||
				strings.Contains(scope, `env["CGO_ENABLED"] = "0"`) ||
				strings.Contains(scope, `env['CGO_ENABLED'] = '0'`) ||
				strings.Contains(scope, `CGO_ENABLED="0"`)
			if !zero {
				t.Errorf("%s:%d Go producer environment does not set CGO_ENABLED=0", rel, i+1)
			}
		}
	})
	if shellProducers == 0 || shellRunProducers == 0 || functionalCIProducers == 0 ||
		pythonProducers == 0 || pythonRunProducers == 0 || usageRunCommands == 0 {
		t.Fatalf(
			"producer scan was vacuous: shell=%d shell-run=%d functional-ci=%d python=%d python-run=%d usage-run=%d",
			shellProducers, shellRunProducers, functionalCIProducers, pythonProducers, pythonRunProducers, usageRunCommands,
		)
	}

	runnerEnv := string(mustReadFile(t, filepath.Join(repoRoot(t), "internal", "test", "runner", "runner_exec_util.go")))
	if !strings.Contains(runnerEnv, `return append(env, "CGO_ENABLED=0")`) {
		t.Error("functional .ci command runner must force CGO_ENABLED=0 in childEnv")
	}

	migration := string(mustReadFile(t, filepath.Join(repoRoot(t), "scripts", "dev", "migrate_module.py")))
	if !strings.Contains(migration, "NEXT (manual): CGO_ENABLED=0 go build ./...") {
		t.Error("migrate_module.py manual build command must set CGO_ENABLED=0")
	}

	gokMain := string(mustReadFile(t, filepath.Join(repoRoot(t), "cmd", "ze-gok", "main.go")))
	if !strings.Contains(gokMain, `os.Setenv("CGO_ENABLED", "0")`) {
		t.Error("cmd/ze-gok must force its nested Go builds to CGO_ENABLED=0")
	}

	stressRepro := string(mustReadFile(t, filepath.Join(repoRoot(t), "scripts", "dev", "stress-repro.py")))
	if strings.Contains(stressRepro, `os.path.join(REPO, "bin", "ze-stress")`) ||
		!strings.Contains(stressRepro, `os.path.join(outdir, f"ze-race-{os.getpid()}")`) {
		t.Error("stress-repro.py race binary must stay in tmp/stress-repro, outside the shipping bin tree")
	}
}

// VALIDATES: each exec.Cmd that starts a nested first-party Go build or run
// sets a CGO-free child environment before it starts the command.
// PREVENTS: package tests and structural gates producing host-dependent binaries.
func TestNestedGoCompilationCommandsDisableCGO(t *testing.T) {
	var buildProducers, runProducers int
	walkFirstPartyFiles(t, func(rel string, data []byte) {
		if filepath.Ext(rel) != ".go" || rel == "scripts/checks/checks_test.go" {
			return
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			build := strings.Contains(line, `"go", "build"`) ||
				strings.Contains(line, `"go", buildArgs`)
			run := strings.Contains(line, `"go", "run"`)
			command := build || run
			if !command || (!strings.Contains(line, "exec.Command") && !strings.Contains(line, "osexec.Command")) {
				continue
			}
			if build {
				buildProducers++
			}
			if run {
				runProducers++
			}
			assignAt := strings.Index(line, ":=")
			if assignAt < 0 {
				assignAt = strings.Index(line, "=")
			}
			if assignAt < 0 {
				t.Errorf("%s:%d cannot derive exec.Cmd variable", rel, i+1)
				continue
			}
			fields := strings.Fields(strings.TrimSpace(line[:assignAt]))
			if len(fields) == 0 {
				t.Errorf("%s:%d cannot derive exec.Cmd variable", rel, i+1)
				continue
			}
			name := fields[len(fields)-1]
			end := min(len(lines), i+10)
			block := strings.Join(lines[i:end], "\n")
			if !strings.Contains(block, name+".Env =") {
				t.Errorf("%s:%d nested Go compilation does not set %s.Env", rel, i+1, name)
				continue
			}
			if !strings.Contains(block, "CGO_ENABLED=0") && !strings.Contains(block, ".Env = goEnv(") {
				t.Errorf("%s:%d nested Go compilation does not set CGO_ENABLED=0", rel, i+1)
			}
		}
	})
	if buildProducers == 0 || runProducers == 0 {
		t.Fatalf("nested Go compilation scan was vacuous: build=%d run=%d", buildProducers, runProducers)
	}

	trackedBuild := string(mustReadFile(t, filepath.Join(repoRoot(t), "scripts", "checks", "tracked_build.go")))
	if !strings.Contains(trackedBuild, `append(os.Environ(), "CGO_ENABLED=0")`) {
		t.Error("tracked_build.go goEnv must set CGO_ENABLED=0 for every flavor")
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
// TestStaticcheckFeatureMatrixMakeTargetRunsExecutable exercises the public
// Make entry point and requires the checker to reach its real verdict path.
//
// VALIDATES: make ze-staticcheck-feature-matrix-check reaches Staticcheck and
// reports either a checked-row verdict or an explicit unable-to-judge result.
// PREVENTS: a missing target, a green no-op recipe, or the phase-one stub.
func TestStaticcheckFeatureMatrixMakeTargetRunsExecutable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "make", "--no-print-directory", "ze-staticcheck-feature-matrix-check")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("matrix checker did not return before the wiring timeout: %v", ctx.Err())
	}
	text := string(out)
	if strings.Contains(text, "matrix judgment is unavailable") {
		t.Fatalf("Make target still reached the phase-one stub:\n%s", out)
	}
	checked := strings.Contains(text, "staticcheck feature matrix: checked ")
	unable := strings.Contains(text, "matrix could not be judged")
	if !checked && !unable {
		t.Fatalf("Make target did not reach the checker verdict path:\nerror: %v\n%s", err, out)
	}
	if checked && err != nil {
		t.Fatalf("checker reported checked rows but Make failed: %v\n%s", err, out)
	}
	if !checked && err == nil {
		t.Fatalf("checker reported no checked rows but Make passed:\n%s", out)
	}
}


// TestMigratedDaemonCommandsLiveInOwners asserts the command-surface-ownership
// daemon-RPC migrations stay migrated: each owner-specific command package
// lives under its owner, not under the central internal/component/cmd/ verb
// tree. Prevents a regression that re-adds an owner command centrally.
func TestMigratedDaemonCommandsLiveInOwners(t *testing.T) {
	root := repoRoot(t)
	moved := map[string]string{
		"internal/component/cmd/l2tp":       "internal/component/l2tp/cmd",
		"internal/component/cmd/pppoe":      "internal/component/l2tp/pppoe/cmd",
		"internal/component/cmd/subscriber": "internal/component/l2tp/subscriber/cmd",
		"internal/component/cmd/bfd":        "internal/component/bfd/cmd",
		"internal/component/cmd/archive":    "internal/component/config/archive/cmd",
		// cache/commit handlers live in bgp/plugins/cmd/{cache,commit}; their
		// YANG schema was the last central remnant and now lives beside them.
		"internal/component/cmd/cache":  "internal/component/bgp/plugins/cmd/cache/yang",
		"internal/component/cmd/commit": "internal/component/bgp/plugins/cmd/commit/yang",
	}
	for central, owner := range moved {
		if _, err := os.Stat(filepath.Join(root, central)); err == nil {
			t.Errorf("central daemon-command package %s still exists; it must live in %s", central, owner)
		}
		if _, err := os.Stat(filepath.Join(root, owner)); err != nil {
			t.Errorf("owner daemon-command package %s is missing: %v", owner, err)
		}
	}
	// Every clear command is fully owned: its handler AND its YANG schema live
	// in the owning component. The central clear verb package is a bare
	// verb-root anchor that declares no owner command (each owner merges its own
	// `clear <noun> ...` subtree). See ai/rules/plugins.md.
	for _, ownerHandler := range []string{
		"internal/component/resolve/cmd/dns.go", // ze-clear:dns-cache (schema: resolve/yang)
		"internal/component/ike/cmd/ipsec.go",   // ze-clear:vpn-ipsec-sa (schema: ike/yang)
		"internal/component/iface/cmd/clear.go", // ze-clear:interface-counters (schema: iface/yang)
	} {
		if _, err := os.Stat(filepath.Join(root, ownerHandler)); err != nil {
			t.Errorf("extracted clear handler %s is missing: %v", ownerHandler, err)
		}
	}

	// The metrics verb is generic (Prometheus registry); only ze-bgp:pool-stats
	// is owner-specific (reads the BGP RIB attribute pools). Its handler moved to
	// the RIB command cluster and must not return to the central metrics package.
	poolStatsHandler := "internal/component/bgp/plugins/cmd/rib/pool_stats.go"
	centralMetrics := "internal/component/cmd/metrics/metrics.go"
	centralMetricsSchema := "internal/component/cmd/metrics/yang/ze-cli-metrics-cmd.yang"
	if _, err := os.Stat(filepath.Join(root, poolStatsHandler)); err != nil {
		t.Errorf("pool-stats handler must live in the RIB command owner: %v", err)
	}
	metricsBody, err := os.ReadFile(filepath.Join(root, centralMetrics))
	if err != nil {
		t.Fatalf("read central metrics handler: %v", err)
	}
	if strings.Contains(string(metricsBody), "handlePoolStats") {
		t.Error("central metrics package still defines handlePoolStats; pool-stats is owned by the BGP RIB command cluster")
	}
	metricsSchema, err := os.ReadFile(filepath.Join(root, centralMetricsSchema))
	if err != nil {
		t.Fatalf("read central metrics schema: %v", err)
	}
	if strings.Contains(string(metricsSchema), "ze-bgp:pool-stats") {
		t.Error("central metrics schema still declares ze-bgp:pool-stats; it is owned by bgp/plugins/cmd/rib/yang")
	}

	// The ping feature (show ping, monitor ping, resolve ping) is owned by
	// the dedicated ping module. show/monitor ping run as local handlers.
	// None of its handlers may remain in the central show, BGP monitor,
	// resolve, or diag packages.
	pingOwner := "internal/component/ping/cmd"
	if _, err := os.Stat(filepath.Join(root, pingOwner)); err != nil {
		t.Errorf("ping feature module %s is missing: %v", pingOwner, err)
	}
	for _, gone := range []string{
		"internal/component/cmd/show/ping.go",        // show ping handler -> ping module
		"internal/component/cmd/show/ping_stream.go", // monitor ping stream -> ping module
	} {
		if _, err := os.Stat(filepath.Join(root, gone)); err == nil {
			t.Errorf("central ping file %s still exists; the ping feature is owned by %s", gone, pingOwner)
		}
	}
	for _, c := range []struct{ file, symbol string }{
		{"internal/component/bgp/plugins/cmd/monitor/monitor.go", "handleMonitorPing"},
		{"internal/component/resolve/cmd/resolve.go", "func handlePing"},
	} {
		body, err := os.ReadFile(filepath.Join(root, c.file))
		if err != nil {
			t.Errorf("read %s: %v", c.file, err)
			continue
		}
		if strings.Contains(string(body), c.symbol) {
			t.Errorf("%s still defines %q; the ping feature is owned by %s", c.file, c.symbol, pingOwner)
		}
	}
	diagPath := filepath.Join(root, "cmd", "ze", "diag", "diag.go")
	if _, err := os.Stat(diagPath); err == nil {
		body, readErr := os.ReadFile(diagPath)
		if readErr != nil {
			t.Errorf("read %s: %v", diagPath, readErr)
		} else if strings.Contains(string(body), "func RunPing") {
			t.Errorf("cmd/ze/diag/diag.go still defines \"func RunPing\"; the ping feature is owned by %s", pingOwner)
		}
	}

	// The traceroute feature (show traceroute, show probe-round, monitor
	// traceroute, resolve traceroute) is owned by the dedicated traceroute
	// module. None of its handlers may remain in the central show, BGP monitor,
	// or resolve packages. show/monitor traceroute run as local handlers.
	tracerouteOwner := "internal/component/traceroute/cmd"
	if _, err := os.Stat(filepath.Join(root, tracerouteOwner)); err != nil {
		t.Errorf("traceroute feature module %s is missing: %v", tracerouteOwner, err)
	}
	for _, gone := range []string{
		"internal/component/cmd/show/traceroute.go",          // show traceroute -> traceroute module
		"internal/component/cmd/show/traceroute_parallel.go", // show probe-round -> traceroute module
		"internal/component/cmd/show/traceroute_stream.go",   // monitor traceroute stream -> traceroute module
	} {
		if _, err := os.Stat(filepath.Join(root, gone)); err == nil {
			t.Errorf("central traceroute file %s still exists; the traceroute feature is owned by %s", gone, tracerouteOwner)
		}
	}
	for _, c := range []struct{ file, symbol string }{
		{"internal/component/bgp/plugins/cmd/monitor/monitor.go", "handleMonitorTraceroute"},
		{"internal/component/resolve/cmd/resolve.go", "func handleTraceroute"},
	} {
		body, err := os.ReadFile(filepath.Join(root, c.file))
		if err != nil {
			t.Errorf("read %s: %v", c.file, err)
			continue
		}
		if strings.Contains(string(body), c.symbol) {
			t.Errorf("%s still defines %q; the traceroute feature is owned by %s", c.file, c.symbol, tracerouteOwner)
		}
	}

	// The `show interface` family (interface, detail, counters, scan, rate) and
	// `monitor interface rate` are owned by the iface component: every handler
	// reads interface state through the iface backend. None may remain in the
	// central show package.
	ifaceOwner := "internal/component/iface/cmd"
	for _, want := range []string{
		"internal/component/iface/cmd/show_interface.go", // show interface family
		"internal/component/iface/cmd/interface_rate.go", // show/monitor interface rate
	} {
		if _, err := os.Stat(filepath.Join(root, want)); err != nil {
			t.Errorf("iface interface command file %s is missing: %v", want, err)
		}
	}
	centralRate := "internal/component/cmd/show/interface_rate.go"
	if _, err := os.Stat(filepath.Join(root, centralRate)); err == nil {
		t.Errorf("central %s still exists; the interface rate surface is owned by %s", centralRate, ifaceOwner)
	}
	centralShow := "internal/component/cmd/show/show.go"
	showBody, err := os.ReadFile(filepath.Join(root, centralShow))
	if err != nil {
		t.Fatalf("read central show.go: %v", err)
	}
	for _, symbol := range []string{"func handleShowInterface", "func handleMonitorInterfaceRate"} {
		if strings.Contains(string(showBody), symbol) {
			t.Errorf("central show.go still defines %q; the interface surface is owned by %s", symbol, ifaceOwner)
		}
	}

	// The `show traffic` (QoS) command is owned by the traffic component: its
	// handler reads traffic.GetBackend(). It must not remain in central show.
	trafficOwner := "internal/component/traffic/cmd/traffic.go"
	if _, err := os.Stat(filepath.Join(root, trafficOwner)); err != nil {
		t.Errorf("traffic command handler %s is missing: %v", trafficOwner, err)
	}
	if strings.Contains(string(showBody), "func handleShowTraffic") {
		t.Errorf("central show.go still defines handleShowTraffic; the traffic surface is owned by %s", trafficOwner)
	}

	// The `monitor vpn ipsec` command is owned by the ike component: its
	// handler reads ike/engine events. The YANG node must live in ike/schema,
	// not in the central monitor schema.
	ikeMonitorSchema := "internal/component/ike/yang/ze-ipsec-cmd.yang"
	centralMonitorSchema := "internal/component/cmd/monitor/yang/ze-cli-monitor-cmd.yang"
	ikeSchemaBody, err := os.ReadFile(filepath.Join(root, ikeMonitorSchema))
	if err != nil {
		t.Fatalf("read ike schema: %v", err)
	}
	if !strings.Contains(string(ikeSchemaBody), `ze:command "ze-monitor:vpn-ipsec"`) {
		t.Errorf("ike schema %s must declare ze-monitor:vpn-ipsec", ikeMonitorSchema)
	}
	centralMonBody, err := os.ReadFile(filepath.Join(root, centralMonitorSchema))
	if err != nil {
		t.Fatalf("read central monitor schema: %v", err)
	}
	if strings.Contains(string(centralMonBody), `ze-monitor:vpn-ipsec`) {
		t.Errorf("central monitor schema still declares ze-monitor:vpn-ipsec; it is owned by %s", ikeMonitorSchema)
	}

	// The `show bgp-health` overview and the `delete bgp peer` command surface
	// are owned by the BGP peer command package: their handlers live in
	// bgp/plugins/cmd/peer and the YANG nodes live in that owner's schema, not
	// the central show/delete schemas. Removing the delete command leaves the
	// central delete schema a bare verb-root anchor.
	peerOwnerSchema := "internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang"
	peerSchemaBody, err := os.ReadFile(filepath.Join(root, peerOwnerSchema))
	if err != nil {
		t.Fatalf("read peer owner schema: %v", err)
	}
	for _, tok := range []string{
		"ze-show:bgp-health",
		"ze-delete:bgp-peer",
	} {
		if !strings.Contains(string(peerSchemaBody), tok) {
			t.Errorf("peer owner schema %s must declare %q", peerOwnerSchema, tok)
		}
	}
	if strings.Contains(string(showBody), "func handleShowBGPHealth") {
		t.Error("central show.go still defines handleShowBGPHealth; show bgp-health is owned by bgp/plugins/cmd/peer")
	}
	deleteSchema := "internal/component/cmd/delete/yang/ze-cli-delete-cmd.yang"
	deleteBody, err := os.ReadFile(filepath.Join(root, deleteSchema))
	if err != nil {
		t.Fatalf("read delete schema: %v", err)
	}
	if strings.Contains(string(deleteBody), "ze:command") {
		t.Errorf("central delete schema %s must be a bare verb-root anchor (no ze:command); delete bgp peer is owned by the BGP peer command package", deleteSchema)
	}
}

// TestGenericCentralCommandsStayCentral is the inverse of
// TestMigratedDaemonCommandsLiveInOwners: it asserts that generic, cross-cutting
// commands that have NO removable owner intentionally remain in the central verb
// packages. Prevents a future session from migrating a command that was already
// decided to stay central.
//
// Criterion for inclusion: the handler reads from multiple subsystems, the
// process, or a cross-plugin registry. Removing any single component must not
// remove these commands.
func TestGenericCentralCommandsStayCentral(t *testing.T) {
	root := repoRoot(t)

	// Central verb packages that must continue to exist.
	centralDirs := []string{
		"internal/component/cmd/show",
		"internal/component/cmd/meta",
		"internal/component/cmd/log",
		"internal/component/cmd/subscribe",
		"internal/component/cmd/set",
		"internal/component/cmd/update",
		"internal/component/cmd/metrics",
		"internal/component/cmd/monitor",
		"internal/component/cmd/clear",
	}
	for _, d := range centralDirs {
		if _, err := os.Stat(filepath.Join(root, d)); err != nil {
			t.Errorf("generic central verb package %s must exist: %v", d, err)
		}
	}

	// Generic show handlers that read process/system state, not a single
	// removable component. Each must remain in cmd/show/show.go.
	showFile := filepath.Join(root, "internal", "component", "cmd", "show", "show.go")
	showBody, err := os.ReadFile(showFile)
	if err != nil {
		t.Fatalf("read show.go: %v", err)
	}
	genericShowHandlers := []string{
		"ze-show:version",
		"ze-show:uptime",
		"ze-show:warnings",
		"ze-show:errors",
		"ze-show:health",
	}
	for _, h := range genericShowHandlers {
		if !strings.Contains(string(showBody), h) {
			t.Errorf("generic central handler %q missing from show.go; it has no removable owner and must stay central", h)
		}
	}

	// Generic show subcommands that read process/kernel/cross-cutting state.
	// Listed by schema file so the test survives handler refactoring within
	// the central show package.
	showSchema := filepath.Join(root, "internal", "component", "cmd", "show", "yang", "ze-cli-show-cmd.yang")
	schemaBody, err := os.ReadFile(showSchema)
	if err != nil {
		t.Fatalf("read show schema: %v", err)
	}
	genericShowCommands := []string{
		`"ze-show:version"`,
		`"ze-show:uptime"`,
		`"ze-show:warnings"`,
		`"ze-show:errors"`,
		`"ze-show:health"`,
		`"ze-show:system-memory"`,
		`"ze-show:system-cpu"`,
		`"ze-show:system-date"`,
	}
	for _, cmd := range genericShowCommands {
		if !strings.Contains(string(schemaBody), cmd) {
			t.Errorf("generic central YANG command %s missing from show schema; it has no removable owner and must stay central", cmd)
		}
	}

	// The central monitor schema is a bare verb-root anchor; all monitor
	// subcommands are owned by their respective components (BGP, iface, command).
	monSchema := filepath.Join(root, "internal", "component", "cmd", "monitor", "yang", "ze-cli-monitor-cmd.yang")
	if _, err := os.Stat(monSchema); err != nil {
		t.Fatalf("central monitor verb-root schema %s must exist: %v", monSchema, err)
	}
}
