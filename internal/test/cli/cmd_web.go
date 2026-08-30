// Design: docs/architecture/testing/ci-format.md -- web browser test CLI

package cli

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"

	webtesting "github.com/ze-software/ze/internal/component/web/testing"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/runner"
	"github.com/ze-software/ze/internal/test/sessionpath"
	"github.com/ze-software/ze/internal/test/trace"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	webConcurrency = 4
	webPortBase    = 10200 // above .ci runner range (1790-based) to avoid collisions

	// Only the web UI serves TLS. The looking glass and the chaos dashboard
	// listen in plain HTTP under test, because these tests read CONTENT.
	schemeHTTP  = "http://"
	schemeHTTPS = "https://"
)

const webUsageHeader = `Usage: ze-test web [options] [test-ids...]

Run web browser functional tests in parallel.
Requires: agent-browser CLI. The ze binary is resolved like every other suite
(ZE_BIN, else this session's bin/, else built here).

Options:
`

const webUsageExamples = `
Examples:
  ze-test web --all          Run all tests in test/web/
  ze-test web -p nav         Run tests matching "nav"
  ze-test web --start 4      Resume at id 4 and run through the end
  ze-test web 1 2            Run specific tests by id
  ze-test web -v             Verbose output
  ze-test web -l             List available tests with N/TOTAL and one-based id
`

// webBrowserMissing decides what happens when agent-browser is not in PATH:
// hard failure under the verify gate (a silent skip would record a full PASS
// that never ran the .wb suite), silent skip for casual local runs.
func webBrowserMissing(verifyMode bool) error {
	if verifyMode {
		return errors.New("agent-browser not found in PATH: the web suite is part of the verify gate and cannot be skipped silently; install agent-browser or skip explicitly with ZE_SKIP_SUITES=web")
	}
	return nil
}

func cmdWeb(args []string) int {
	if err := cmdWebMain(args); err != nil {
		if !errors.Is(err, ErrTestsFailed) {
			os.Stderr.WriteString(err.Error()) //nolint:errcheck // terminal output
			os.Stderr.WriteString("\n")        //nolint:errcheck // terminal output
		}
		return 1
	}
	return 0
}

func cmdWebMain(args []string) error {
	fs := flag.NewFlagSet("web", flag.ExitOnError)
	all := fs.Bool("a", false, "run all tests")
	fs.BoolVar(all, "all", false, "run all tests")
	pattern := fs.String("p", "", "run only tests matching pattern")
	fs.StringVar(pattern, "pattern", "", "run only tests matching pattern")
	verbose := fs.Bool("v", false, "verbose output")
	fs.BoolVar(verbose, "verbose", false, "verbose output")
	quiet := fs.Bool("q", false, "minimal output")
	fs.BoolVar(quiet, "quiet", false, "minimal output")
	listOnly := fs.Bool("l", false, "list tests without running")
	fs.BoolVar(listOnly, "list", false, "list tests without running")
	start := fs.String("start", "", "start at test id/name and run through the end")
	draft := fs.Bool("draft", false, "discover from test/draft/web instead of test/web (tests under development)")

	fs.Usage = func() {
		os.Stderr.WriteString(webUsageHeader) //nolint:errcheck // terminal output
		fs.PrintDefaults()
		os.Stderr.WriteString(webUsageExamples) //nolint:errcheck // terminal output
	}

	if len(args) > 0 && isHelpArg(args[0]) {
		fs.Usage()
		return nil
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	baseDir, err := FindBaseDir()
	if err != nil {
		return fmt.Errorf("find base dir: %w", err)
	}

	testDir := runner.SuiteDir(baseDir, "web", *draft)
	runner.ResetNickCounter()
	tests := runner.NewTestSet[*zeTestWebTest]()
	if walkErr := filepath.WalkDir(testDir, func(path string, d os.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() || !strings.HasSuffix(path, ".wb") {
			return nil
		}
		rel, _ := filepath.Rel(baseDir, path)
		if rel == "" {
			rel = path
		}
		tests.Add(&zeTestWebTest{
			Name: rel,
			Nick: runner.GenerateNick(rel),
			Path: path,
		})
		return nil
	}); walkErr != nil {
		return walkErr
	}

	if tests.Count() == 0 {
		return fmt.Errorf("no .wb files found in %s", testDir)
	}

	if !*all && *pattern == "" && *start == "" && fs.NArg() == 0 {
		*all = true
	}
	selected, err := tests.Select(runner.Selection{
		All:     *all,
		Start:   *start,
		Pattern: *pattern,
		Args:    fs.Args(),
	})
	if err != nil {
		return err
	}
	if selected == 0 && !*listOnly {
		fs.Usage()
		return nil
	}

	if *listOnly {
		tests.List()
		return nil
	}

	if _, lookErr := exec.LookPath("agent-browser"); lookErr != nil {
		if err := webBrowserMissing(runner.VerifyModeEnabled()); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "agent-browser not found in PATH, skipping web tests\n") //nolint:errcheck // terminal output
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		cancel()
	}()

	// Resolve (and, off ZE_TEST_NO_BUILD, build) the DUT through the same helper
	// every other suite uses, so ZE_BIN is honored. Hardcoding <baseDir>/bin/ze
	// was wrong in BOTH directions: the functional flow builds its isolated set
	// into tmp/testbin-*/bin and exports ZE_BIN there, never populating
	// <baseDir>/bin, so on a fresh checkout all 87 tests died instantly on
	// "fork/exec bin/ze: no such file or directory" (every CI run), while on a
	// developer host a leftover bin/ze made the suite silently test a stale
	// binary that was not the one under test.
	bins, err := zeTestResolveWebBinaries(ctx, baseDir, tests.Selected())
	if err != nil {
		return err
	}
	defer zeTestCloseAllBrowserSessions()

	colors := runner.NewColors()
	pr := runner.NewParallelRunner[*zeTestWebTest](colors)
	pr.SetLabel("web")
	pr.SetConcurrency(webConcurrency)
	pr.SetQuiet(*quiet)
	pr.SetVerbose(*verbose)
	pr.SetBaseDir(baseDir)

	for _, t := range tests.Selected() {
		pr.AddTestWithNick(t.Name, t.Nick, t, func(runCtx context.Context, test *zeTestWebTest) (bool, error) {
			return zeTestRunWebTest(runCtx, test, bins)
		})
	}

	pr.SetOnFail(func(t *zeTestWebTest, _ error) {
		fmt.Fprintf(os.Stdout, "  %s\n", t.GetError()) //nolint:errcheck // terminal output
		if len(t.Steps) > 0 {
			trace.PrintTrace(os.Stdout, t.Name, t.Steps, colors.Enabled())
		}
	})

	success := pr.Run(ctx)

	if *verbose {
		for _, t := range tests.Selected() {
			if t.GetError() == nil && len(t.Steps) > 0 {
				trace.PrintTrace(os.Stdout, t.Name, t.Steps, colors.Enabled())
			}
		}
	}

	if !success {
		return ErrTestsFailed
	}
	return nil
}

func zeTestRunWebTest(ctx context.Context, test *zeTestWebTest, bins zeTestWebBinaries) (bool, error) {
	// A test that declares option=auth needs real authentication (not the fast
	// single-implicit-admin --insecure-web path), and one that declares
	// option=server drives a different program altogether. Pre-parse the .wb
	// file for both decisions before anything is started.
	tc := zeTestWebCase(test.Path)
	kind := tc.ServerKind()

	// The looking glass needs a second port: its pages read `show bgp`,
	// so the harness gives the daemon a peer to report on.
	ports := 1
	if kind == webtesting.WBServerLG {
		ports = 2
	}

	reservation, _, err := runner.ReservePorts(webPortBase, ports)
	if err != nil {
		test.SetError(fmt.Errorf("reserve port: %w", err))
		return false, test.GetError()
	}
	defer reservation.Release()

	port := reservation.Start
	var tb textbuf.Buffer
	listenAddr := tb.Str("127.0.0.1:").Int(int64(port)).String()

	var (
		srv    *zeTestWebServer
		scheme string
	)

	switch kind {
	case webtesting.WBServerLG:
		scheme = schemeHTTP
		srv, err = zeTestStartLGServer(ctx, bins, listenAddr, reservation.Start+1, tc.Env)
	case webtesting.WBServerLGNoEngine:
		scheme = schemeHTTP
		srv, err = zeTestStartLGNoEngineServer(ctx, bins, listenAddr, tc.Env)
	case webtesting.WBServerChaos:
		scheme = schemeHTTP
		srv, err = zeTestStartChaosServer(ctx, bins, listenAddr, tc.Env)
	case webtesting.WBServerWeb:
		scheme = schemeHTTPS
		srv, err = zeTestStartWebServer(ctx, bins.ze, listenAddr, !tc.RequiresAuth(), tc.Auth, tc.Env)
	}

	if err != nil {
		test.SetError(fmt.Errorf("start %s server: %w", kind, err))
		return false, test.GetError()
	}
	defer srv.stop()

	baseURL := tb.Reset().Str(scheme).Str(listenAddr).String()
	result := webtesting.RunWBFileWithSession(test.Path, baseURL, test.Nick)
	test.Steps = result.Steps

	if result.Skipped {
		return true, nil
	}
	if !result.Passed {
		test.SetError(fmt.Errorf("%s", result.Error))
		return false, test.GetError()
	}
	return true, nil
}

type zeTestWebTest struct {
	runner.BaseTest
	Path  string
	Steps []trace.StepResult
}

type zeTestWebServer struct {
	cmd     *exec.Cmd
	aux     *exec.Cmd // a second process the server needs, e.g. the BGP peer the looking glass reports on
	tempDir string
}

// zeTestWebBinaries are the programs this suite can start. Ze serves its three
// htmx interfaces from three binaries, so a suite that proves all three in a
// browser needs all three.
type zeTestWebBinaries struct {
	// ze serves the web UI, and the looking glass as one of its listeners.
	ze string
	// zeTest is THIS program, which supplies the BGP sink peer the looking
	// glass reports on. os.Executable is what names it: the suite is already
	// running from the binary the run was built against, so resolving it any
	// other way could pick up a different build.
	zeTest string
	// chaos serves the chaos dashboard. Empty when no selected test asks for
	// it, because building it costs a minute that a web-only run should not
	// pay. zeTestStartChaosServer refuses rather than starting nothing.
	chaos string
}

// zeTestResolveWebBinaries resolves the programs the selected tests need.
//
// The chaos build is CONDITIONAL and the looking glass one is not: `ze` is
// needed by every test, while `ze-chaos` is a second full compile of cmd/ze
// under different tags. It is built only when a selected test names the chaos
// server, so the common run is unchanged.
func zeTestResolveWebBinaries(ctx context.Context, baseDir string, tests []*zeTestWebTest) (zeTestWebBinaries, error) {
	var bins zeTestWebBinaries

	ze, err := buildZe(ctx, baseDir)
	if err != nil {
		return bins, err
	}

	bins.ze = ze

	self, err := os.Executable()
	if err != nil {
		return bins, fmt.Errorf("resolve this test binary: %w", err)
	}

	bins.zeTest = self

	if !zeTestWantsChaos(tests) {
		return bins, nil
	}

	chaos, err := zeTestBuildChaos(ctx, baseDir, ze)
	if err != nil {
		return bins, err
	}

	bins.chaos = chaos

	return bins, nil
}

// zeTestWantsChaos reports whether any selected test drives the chaos dashboard.
func zeTestWantsChaos(tests []*zeTestWebTest) bool {
	for _, t := range tests {
		if zeTestWebCase(t.Path).ServerKind() == webtesting.WBServerChaos {
			return true
		}
	}

	return false
}

// zeTestBuildChaos resolves ze-chaos BESIDE the ze binary this run uses.
//
// Beside, rather than in a directory of its own: the functional flow builds an
// isolated binary set into a throwaway directory and points ZE_BIN at it, while
// a plain run uses this session's bin/. One rule reaches both, and it cannot
// pick up a stale chaos binary from the other tree.
//
// The build is skipped under ZE_TEST_NO_BUILD, the native functional flow's
// promise that the caller built the isolated set already. A miss there is an
// error rather than a build: building would defeat the isolation the flag gives.
func zeTestBuildChaos(ctx context.Context, baseDir, zeBin string) (string, error) {
	chaosPath := filepath.Join(filepath.Dir(zeBin), "ze-chaos")

	if _, err := os.Stat(chaosPath); err == nil {
		return chaosPath, nil
	}

	if env.IsEnabled("ze.test.no.build") {
		if dir := sessionpath.FindPrebuiltDir(baseDir, "ze-chaos"); dir != "" {
			return filepath.Join(dir, "ze-chaos"), nil
		}

		return "", fmt.Errorf("ZE_TEST_NO_BUILD set but %s is missing (unset ZE_TEST_NO_BUILD or run the native functional action that prepared this suite)", chaosPath)
	}

	cmd := exec.CommandContext(ctx, "go", "build", "-tags", "ze_chaos ze_bgp", "-o", chaosPath, packageZe) //nolint:gosec // paths from internal runner
	cmd.Dir = baseDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build ze-chaos: %w: %s", err, output)
	}

	return chaosPath, nil
}

// zeTestWebCase reads a .wb file for the decisions the harness makes BEFORE the
// browser runs: which server to start (option=server), which users to seed
// (option=auth), and what environment the server process gets (option=env).
//
// A read or parse error yields the zero case, which is the default web server
// with no auth and no environment: the file's own error surfaces a moment later
// when the runner parses it again to execute it, and reporting it here as well
// would hide that message behind a start failure.
func zeTestWebCase(path string) *webtesting.WBTestCase {
	content, err := os.ReadFile(path) //nolint:gosec // controlled test discovery path
	if err != nil {
		return &webtesting.WBTestCase{}
	}

	tc, err := webtesting.ParseWBFile(string(content))
	if err != nil {
		return &webtesting.WBTestCase{}
	}

	return tc
}

// zeTestEnv returns the server process environment: this process's, plus the
// pairs the test declared with option=env.
func zeTestEnv(vars []webtesting.WBEnvVar, extra ...string) []string {
	out := append(os.Environ(), extra...)

	var tb textbuf.Buffer
	for _, v := range vars {
		out = append(out, tb.Reset().Str(v.Var).Byte('=').Str(v.Value).String())
	}

	return out
}

// lgConfigTemplate is the daemon configuration the looking-glass suite runs.
//
// It states three things the pages need and nothing else. The looking glass
// listens on its own port with TLS off, because these tests read CONTENT over
// a plain URL. One peer dials the sink on the second reserved port, so the
// session reaches Established and `show bgp` reports a real row rather
// than an empty table. `accept false` keeps the daemon off port 179, which a
// test host does not grant an unprivileged process.
//
// The two ends carry DIFFERENT addresses. 127.0.0.0/8 is one loopback, so both
// ends can sit on it, and a session whose local and remote address are the same
// string is a state no operator has: a next-hop rule that compares a route's
// next hop against the peer's address cannot be tested by a config where the
// two are one. The sink listens on 127.0.0.1, so the daemon takes 127.0.0.2.
const lgConfigTemplate = `environment {
	looking-glass {
		enabled true
		tls false
		server main {
			ip 127.0.0.1
			port $LG_PORT
		}
	}
}

bgp {
	peer sink {
		connection {
			remote {
				ip 127.0.0.1
				port $BGP_PORT
			}
			local {
				ip 127.0.0.2
				accept false
			}
		}
		session {
			asn {
				local 65000
				remote 65000
			}
			router-id 10.0.0.1
			family {
				ipv4/unicast { prefix { maximum 1000; } }
			}
			capability {
				graceful-restart disable
			}
		}
	}
}
`

// zeTestStartLGServer starts the looking glass: a BGP sink peer, then a daemon
// that dials it and serves /lg/ on listenAddr.
//
// The sink comes FIRST on purpose. The daemon's connect-retry timer is measured
// in tens of seconds, so a peer that is not listening when the daemon starts
// costs the test that whole timer before the table shows anything.
func zeTestStartLGServer(ctx context.Context, bins zeTestWebBinaries, listenAddr string, bgpPort int, envVars []webtesting.WBEnvVar) (*zeTestWebServer, error) {
	_, lgPort, _ := net.SplitHostPort(listenAddr)
	bgpPortText := textbuf.StringInt(int64(bgpPort))

	tempDir, tempErr := os.MkdirTemp(sessionpath.DefaultScratchRoot(), "ze-lg-test-*")
	if tempErr != nil {
		return nil, fmt.Errorf("create temp config dir: %w", tempErr)
	}

	peer := exec.CommandContext(ctx, bins.zeTest, "peer", "--mode", "sink", "--port", bgpPortText) //nolint:gosec // test binary path
	peer.Env = zeTestEnv(envVars)

	if err := peer.Start(); err != nil {
		os.RemoveAll(tempDir) //nolint:errcheck // best-effort cleanup on start failure

		return nil, fmt.Errorf("start bgp sink peer: %w", err)
	}

	config := strings.NewReplacer("$LG_PORT", lgPort, "$BGP_PORT", bgpPortText).Replace(lgConfigTemplate)

	var tb textbuf.Buffer

	cmd := exec.CommandContext(ctx, bins.ze, "-") //nolint:gosec // test binary path
	cmd.Stdin = strings.NewReader(config)
	cmd.Env = zeTestEnv(envVars, tb.Str("ze.config.dir=").Str(tempDir).String())

	if err := cmd.Start(); err != nil {
		zeTestKillCmd(peer)
		os.RemoveAll(tempDir) //nolint:errcheck // best-effort cleanup on start failure

		return nil, err
	}

	srv := &zeTestWebServer{cmd: cmd, aux: peer, tempDir: tempDir}

	// Probe the peers PAGE, not the root: /lg/ redirects, and a redirect proves
	// the mux is mounted without proving the page renders.
	if err := zeTestProbeReady(ctx, schemeHTTP, listenAddr, "/lg/peers", zeTestReadyTimeout()); err != nil {
		srv.stop()

		return nil, fmt.Errorf("looking glass not ready: %w", err)
	}

	return srv, nil
}

// zeTestStartLGNoEngineServer starts the looking glass with an engine that
// always fails, so its pages and its stream take the engine-unavailable path.
//
// It runs `ze-test lg`, not the daemon: the looking glass dispatches in
// process, so a daemon with no BGP configured still answers an empty peer list,
// and the engine-error path cannot be reached through configuration.
func zeTestStartLGNoEngineServer(ctx context.Context, bins zeTestWebBinaries, listenAddr string, envVars []webtesting.WBEnvVar) (*zeTestWebServer, error) {
	cmd := exec.CommandContext(ctx, bins.zeTest, "lg", "--listen", listenAddr) //nolint:gosec // test binary path
	cmd.Env = zeTestEnv(envVars)

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	srv := &zeTestWebServer{cmd: cmd}

	if err := zeTestProbeReady(ctx, schemeHTTP, listenAddr, "/lg/peers", zeTestReadyTimeout()); err != nil {
		srv.stop()

		return nil, fmt.Errorf("looking glass not ready: %w", err)
	}

	return srv, nil
}

// zeTestStartChaosServer starts the chaos dashboard.
//
// The run duration outlives any .wb budget on purpose: ze-chaos stops itself
// when its run ends, and a dashboard that exited mid-test would fail the
// assertions with an empty page instead of a verdict. srv.stop() ends this one.
func zeTestStartChaosServer(ctx context.Context, bins zeTestWebBinaries, listenAddr string, envVars []webtesting.WBEnvVar) (*zeTestWebServer, error) {
	if bins.chaos == "" {
		return nil, errors.New("no ze-chaos binary was built for this run")
	}

	_, portStr, _ := net.SplitHostPort(listenAddr)

	var tb textbuf.Buffer

	cmd := exec.CommandContext(ctx, bins.chaos, //nolint:gosec // test binary path
		"--in-process", "--web", tb.Byte(':').Str(portStr).String(),
		"--duration", "10m", "--peers", "6", "--seed", "42", "--routes", "20", "--quiet")
	cmd.Env = zeTestEnv(envVars)

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	srv := &zeTestWebServer{cmd: cmd}

	if err := zeTestProbeReady(ctx, schemeHTTP, listenAddr, "/", zeTestReadyTimeout()); err != nil {
		srv.stop()

		return nil, fmt.Errorf("chaos dashboard not ready: %w", err)
	}

	return srv, nil
}

// zeTestReadyTimeout bounds a server start. Under
// `./le verify current mode full`, the web suite overlaps the -race unit stage,
// and a forked daemon can take well over 30s just to bind under CPU starvation,
// so the budget scales by the contention headroom applied to per-test budgets. A standalone run
// keeps the tight 30s so a genuine "server never starts" still surfaces fast.
func zeTestReadyTimeout() time.Duration {
	ready := 30 * time.Second
	if runner.VerifyModeEnabled() {
		ready *= runner.ParallelTimeoutHeadroom
	}

	return ready
}

func zeTestStartWebServer(ctx context.Context, zeBin, listenAddr string, insecure bool, authUsers []webtesting.WBAuthUser, envVars []webtesting.WBEnvVar) (*zeTestWebServer, error) {
	_, portStr, _ := net.SplitHostPort(listenAddr)
	tempDir, tempErr := os.MkdirTemp(sessionpath.DefaultScratchRoot(), "ze-web-test-*")
	if tempErr != nil {
		return nil, fmt.Errorf("create temp config dir: %w", tempErr)
	}
	if err := zeTestSeedWebUsers(tempDir, insecure, authUsers); err != nil {
		os.RemoveAll(tempDir) //nolint:errcheck // best-effort cleanup on seed failure
		return nil, fmt.Errorf("seed web users: %w", err)
	}
	// --web-only starts the standalone web UI (no BGP engine, no config
	// required). The .wb suite exercises the UI surface -- config editor,
	// navigation, fragments, SSE log stream -- which RunWebOnly serves in full;
	// it never needs live peer data. Plain "start --web" would demand a loaded
	// config and exit before binding the port (ze_core_start.go:219), so every
	// test timed out at the readiness probe.
	//
	// --insecure-web is dropped for tests that declare option=auth so the login
	// flow and role gating are exercised against real authentication.
	args := []string{"start", "--web", portStr, "--web-only"}
	if insecure {
		args = append(args, "--insecure-web")
	}
	cmd := exec.CommandContext(ctx, zeBin, args...) //nolint:gosec // test binary path
	var tb textbuf.Buffer
	cmd.Env = zeTestEnv(envVars, tb.Str("ze.config.dir=").Str(tempDir).String())

	if err := cmd.Start(); err != nil {
		os.RemoveAll(tempDir) //nolint:errcheck // best-effort cleanup on start failure
		return nil, err
	}

	if err := zeTestProbeReady(ctx, schemeHTTPS, listenAddr, "/", zeTestReadyTimeout()); err != nil {
		zeTestKillCmd(cmd)
		os.RemoveAll(tempDir) //nolint:errcheck // best-effort cleanup on probe failure
		return nil, fmt.Errorf("daemon not ready: %w", err)
	}

	return &zeTestWebServer{cmd: cmd, tempDir: tempDir}, nil
}

// zeTestSeedWebUsers writes a zefs local-admin credential into the temp config
// store (pre-creating database.zefs, which ze then opens rather than recreates)
// so the Users page lists the always-on "(system)" power user, matching a real
// appliance provisioned by `ze init`.
//
// Under --insecure-web the hash is never used to log in; it only needs to be
// non-empty for the power-user loader to surface the account
// (cmd/ze/hub/main_servers.go usersFromZefsDB). For an auth test (insecure ==
// false) the declared admin user is seeded with a real bcrypt hash so the login
// flow authenticates. NOTE: zefs stores a single local admin, so a multi-user
// test that also needs a distinct read-only login requires config-file authz
// users; the admin credential seeded here covers admin-login and single-user
// tests (see the AC-6/7 runbook in the spec).
func zeTestSeedWebUsers(configDir string, insecure bool, authUsers []webtesting.WBAuthUser) error {
	store, err := zefs.Create(filepath.Join(configDir, "database.zefs"))
	if err != nil {
		return err
	}
	username := "admin"
	passwordHash := "$2y$10$ze.web.test.placeholder.admin.hash.value.unused" //nolint:gosec // G101: placeholder test hash, replaced with a real bcrypt hash for auth tests
	if !insecure && len(authUsers) > 0 {
		admin := zeTestPickAdmin(authUsers)
		username = admin.Name
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(admin.Password), bcrypt.DefaultCost)
		if hashErr != nil {
			store.Close() //nolint:errcheck // returning the primary hashing error
			return hashErr
		}
		passwordHash = string(hash)
	}
	if err := store.WriteFile(zefs.KeyLocalAdminUsername.Pattern, []byte(username), 0); err != nil {
		store.Close() //nolint:errcheck // returning the primary write error
		return err
	}
	if err := store.WriteFile(zefs.KeyLocalAdminPassword.Pattern, []byte(passwordHash), 0); err != nil {
		store.Close() //nolint:errcheck // returning the primary write error
		return err
	}
	return store.Close()
}

// zeTestPickAdmin returns the declared user that should back the seeded local
// admin: the first with an admin (or unset) role, else the first declared user.
func zeTestPickAdmin(users []webtesting.WBAuthUser) webtesting.WBAuthUser {
	for _, u := range users {
		if u.Role == "admin" || u.Role == "" {
			return u
		}
	}
	return users[0]
}

func zeTestProbeReady(ctx context.Context, scheme, addr, path string, timeout time.Duration) error {
	// A bare TCP connect succeeds the instant the listener binds, which can be
	// before the HTTP routes are mounted: a browser hitting the server in that
	// window gets an empty page. Require a real HTTP response (any status proves
	// the mux is serving) so the readiness signal matches what the browser needs.
	// This closes the flaky "(empty page)" failures seen under parallel load.
	deadline := time.Now().Add(timeout)
	client := &http.Client{
		Timeout: 500 * time.Millisecond,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // local self-signed cert under --insecure-web
		},
	}
	var tb textbuf.Buffer
	url := tb.Str(scheme).Str(addr).Str(path).String()
	var lastErr error
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
		if reqErr != nil {
			return reqErr
		}
		resp, doErr := client.Do(req)
		if doErr == nil {
			resp.Body.Close() //nolint:errcheck // readiness probe, body unused
			return nil
		}
		lastErr = doErr
		time.Sleep(100 * time.Millisecond)
	}
	if lastErr != nil {
		return fmt.Errorf("web server %s not ready after %s: %w", addr, timeout, lastErr)
	}
	return fmt.Errorf("web server %s not ready after %s", addr, timeout)
}

func zeTestKillCmd(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill() //nolint:errcheck // best-effort process cleanup
		cmd.Wait()         //nolint:errcheck // reap zombie
	}
}

func (s *zeTestWebServer) stop() {
	zeTestKillCmd(s.cmd)
	zeTestKillCmd(s.aux)
	if s.tempDir != "" {
		os.RemoveAll(s.tempDir) //nolint:errcheck // best-effort temp dir cleanup
	}
}

func zeTestCloseAllBrowserSessions() {
	cmd := exec.CommandContext(context.Background(), "agent-browser", "--ignore-https-errors", "close", "--all") //nolint:gosec // fixed binary name
	cmd.Run()                                                                                                    //nolint:errcheck // best-effort cleanup
}
