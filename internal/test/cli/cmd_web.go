// Design: docs/architecture/testing/ci-format.md -- web browser test CLI

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	webtesting "codeberg.org/thomas-mangin/ze/internal/component/web/testing"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/internal/test/runner"
	"codeberg.org/thomas-mangin/ze/internal/test/trace"
)

const (
	webConcurrency = 4
	webPortBase    = 10200 // above .ci runner range (1790-based) to avoid collisions
)

const webUsageHeader = `Usage: ze-test web [options] [test-ids...]

Run web browser functional tests (.wb files) in parallel.
Requires: agent-browser CLI, ze binary in bin/.

Options:
`

const webUsageExamples = `
Examples:
  ze-test web --all          Run all tests in test/web/
  ze-test web -p nav         Run tests matching "nav"
  ze-test web --start 4      Resume at id 4 and run through the end
  ze-test web 0 1            Run specific tests by id
  ze-test web -v             Verbose output
  ze-test web -l             List tests with N/TOTAL and id
`

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

	testDir := filepath.Join(baseDir, "test", "web")
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

	zeBin := filepath.Join(baseDir, "bin", "ze")
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

	srv, err := zeTestStartWebServer(ctx, zeBin, listenAddr)
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

func zeTestStartWebServer(ctx context.Context, zeBin, listenAddr string) (*zeTestWebServer, error) {
	_, portStr, _ := net.SplitHostPort(listenAddr)
	tempDir, tempErr := os.MkdirTemp("", "ze-web-test-*")
	if tempErr != nil {
		return nil, fmt.Errorf("create temp config dir: %w", tempErr)
	}
	cmd := exec.CommandContext(ctx, zeBin, "start", "--web", portStr, "--insecure-web") //nolint:gosec // test binary path
	var tb textbuf.Buffer
	cmd.Env = append(os.Environ(),
		tb.Str("ze.config.dir=").Str(tempDir).String(),
	)

	if err := cmd.Start(); err != nil {
		os.RemoveAll(tempDir) //nolint:errcheck // best-effort cleanup on start failure
		return nil, err
	}

	if err := zeTestProbePort(ctx, listenAddr, 30*time.Second); err != nil {
		zeTestKillCmd(cmd)
		os.RemoveAll(tempDir) //nolint:errcheck // best-effort cleanup on probe failure
		return nil, fmt.Errorf("daemon not ready: %w", err)
	}

	return &zeTestWebServer{cmd: cmd, tempDir: tempDir}, nil
}

func zeTestProbePort(ctx context.Context, addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	dialer := net.Dialer{Timeout: 200 * time.Millisecond}
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err == nil {
			conn.Close() //nolint:errcheck // probe connection, no data exchanged
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("port %s not reachable after %s", addr, timeout)
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
