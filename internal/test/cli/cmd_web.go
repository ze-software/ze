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
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/runner"
	"github.com/ze-software/ze/internal/test/sessionpath"
	"github.com/ze-software/ze/internal/test/trace"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	webConcurrency = 4
	webPortBase    = 10200 // above .ci runner range (1790-based) to avoid collisions
)

const webUsageHeader = `Usage: ze-test web [options] [test-ids...]

Run web browser functional tests (.wb files) in parallel.
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
			BaseTest: runner.BaseTest{
				Name: rel,
				Nick: runner.GenerateNick(rel),
			},
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
	zeBin, err := buildZe(ctx, baseDir)
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
			return zeTestRunWebTest(runCtx, test, zeBin)
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

func zeTestRunWebTest(ctx context.Context, test *zeTestWebTest, zeBin string) (bool, error) {
	reservation, _, err := runner.ReservePorts(webPortBase, 1)
	if err != nil {
		test.SetError(fmt.Errorf("reserve port: %w", err))
		return false, test.GetError()
	}
	defer reservation.Release()

	port := reservation.Start
	var tb textbuf.Buffer
	listenAddr := tb.Str("127.0.0.1:").Int(int64(port)).String()

	// A test that declares option=auth needs real authentication (not the fast
	// single-implicit-admin --insecure-web path); pre-parse the .wb file to
	// decide the server-start mode and which users to seed.
	insecure, authUsers := zeTestWebAuth(test.Path)

	srv, err := zeTestStartWebServer(ctx, zeBin, listenAddr, insecure, authUsers)
	if err != nil {
		test.SetError(fmt.Errorf("start web server: %w", err))
		return false, test.GetError()
	}
	defer srv.stop()

	baseURL := tb.Reset().Str("https://").Str(listenAddr).String()
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
	tempDir string
}

// zeTestWebAuth reads a .wb file and reports whether the web server should run
// with authentication disabled (--insecure-web) plus the users to seed. A test
// that declares option=auth needs real authentication; otherwise the harness
// keeps the fast single-implicit-admin insecure mode. A read/parse error is a
// safe default of insecure=true (the file's own parse error surfaces later when
// the runner parses it again).
func zeTestWebAuth(path string) (insecure bool, users []webtesting.WBAuthUser) {
	content, err := os.ReadFile(path) //nolint:gosec // controlled test discovery path
	if err != nil {
		return true, nil
	}
	tc, err := webtesting.ParseWBFile(string(content))
	if err != nil {
		return true, nil
	}
	return !tc.RequiresAuth(), tc.Auth
}

func zeTestStartWebServer(ctx context.Context, zeBin, listenAddr string, insecure bool, authUsers []webtesting.WBAuthUser) (*zeTestWebServer, error) {
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
	cmd.Env = append(os.Environ(),
		tb.Str("ze.config.dir=").Str(tempDir).String(),
	)

	if err := cmd.Start(); err != nil {
		os.RemoveAll(tempDir) //nolint:errcheck // best-effort cleanup on start failure
		return nil, err
	}

	// Under `make ze-verify` the functional web suite overlaps the -race unit
	// stage: every .wb test forks a full --web-only daemon, and under that CPU
	// starvation a daemon can take well over 30s just to bind its listener
	// (symptom: connection-refused for the entire fixed window). Scale the
	// readiness budget by the same contention headroom the runner applies to
	// per-test budgets (runner.ParallelTimeoutHeadroom) when in verify mode;
	// standalone runs keep the tight 30s so a genuine "web never starts" still
	// surfaces fast.
	readyTimeout := 30 * time.Second
	if runner.VerifyModeEnabled() {
		readyTimeout *= runner.ParallelTimeoutHeadroom
	}
	if err := zeTestProbeReady(ctx, listenAddr, readyTimeout); err != nil {
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

func zeTestProbeReady(ctx context.Context, addr string, timeout time.Duration) error {
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
	url := tb.Str("https://").Str(addr).Byte('/').String()
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
	if s.tempDir != "" {
		os.RemoveAll(s.tempDir) //nolint:errcheck // best-effort temp dir cleanup
	}
}

func zeTestCloseAllBrowserSessions() {
	cmd := exec.CommandContext(context.Background(), "agent-browser", "--ignore-https-errors", "close", "--all") //nolint:gosec // fixed binary name
	cmd.Run()                                                                                                    //nolint:errcheck // best-effort cleanup
}
