// Design: docs/architecture/testing/ci-format.md -- web browser test CLI

//go:build ze_test

package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	webtesting "codeberg.org/thomas-mangin/ze/internal/component/web/testing"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/internal/test/runner"
)

func zeTestWebCmd(args []string) int {
	if err := zeTestWebMain(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func zeTestWebMain(args []string) error {
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
	port := fs.String("port", "", "port for test web server (default: random free port)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze-test web [options] [test-ids...]

Run web browser functional tests (.wb files).
Requires: agent-browser CLI, ze binary in bin/.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  ze-test web --all          Run all tests in test/web/
  ze-test web -p nav         Run tests matching "nav"
  ze-test web --start 4      Resume at id 4 and run through the end
  ze-test web 0 1            Run specific tests by id
  ze-test web -v             Verbose output
  ze-test web -l             List tests with N/TOTAL and id
`)
	}

	if len(args) > 0 && isHelpArg(args[0]) {
		fs.Usage()
		return nil
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	baseDir, err := findBaseDir()
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

	if *port == "" {
		lc := net.ListenConfig{}
		ln, listenErr := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
		if listenErr != nil {
			return fmt.Errorf("find free port: %w", listenErr)
		}
		tcpAddr, ok := ln.Addr().(*net.TCPAddr)
		if !ok {
			ln.Close() //nolint:errcheck // best-effort cleanup
			return fmt.Errorf("unexpected listener address type: %T", ln.Addr())
		}
		*port = strconv.Itoa(tcpAddr.Port)
		if closeErr := ln.Close(); closeErr != nil {
			return fmt.Errorf("close temp listener: %w", closeErr)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		zeTestCloseBrowser()
		cancel()
	}()

	listenAddr := "127.0.0.1:" + *port
	baseURL := "https://" + listenAddr
	zeBin := filepath.Join(baseDir, "bin", "ze")

	srv, err := zeTestStartWebServer(zeBin, listenAddr)
	if err != nil {
		return fmt.Errorf("start web server: %w", err)
	}
	defer srv.stop()

	zeTestCloseBrowser()
	defer zeTestCloseBrowser()

	colors := runner.NewColors()
	passed, failed, skipped := 0, 0, 0
	selectedTests := tests.Selected()
	total := len(selectedTests)

	if !*quiet {
		runner.PrintHeader("web")
		fmt.Fprintf(os.Stdout, "progress 0/%d\n", total) //nolint:errcheck // terminal output
	}

	for i, t := range selectedTests {
		if ctx.Err() != nil {
			break
		}
		startTime := time.Now()
		done := make(chan struct{})
		if !*quiet {
			go func(index int, test *zeTestWebTest) {
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-done:
						return
					case <-ticker.C:
						fmt.Fprintf(os.Stdout, "%-7s  %d/%d  RUN   %s  %s\n", zeTestFormatElapsed(time.Since(startTime)), index+1, total, test.Nick, test.Name) //nolint:errcheck // terminal output
					}
				}
			}(i, t)
		}
		result := webtesting.RunWBFile(t.Path, baseURL)
		close(done)
		elapsed := time.Since(startTime)
		tag := colors.Green("PASS")
		switch {
		case result.Skipped:
			skipped++
			tag = colors.Gray("SKIP")
		case result.Passed:
			passed++
		default:
			failed++
			tag = colors.Red("FAIL")
		}
		if !*quiet {
			fmt.Fprintf(os.Stdout, "%-7s  %d/%d  %s  %s  %s\n", zeTestFormatElapsed(elapsed), i+1, total, tag, t.Nick, t.Name) //nolint:errcheck // terminal output
			if result.Skipped && result.SkipReason != "" {
				fmt.Fprintf(os.Stdout, "  skip reason: %s\n", result.SkipReason) //nolint:errcheck // terminal output
			}
			if !result.Passed && !result.Skipped {
				fmt.Fprintf(os.Stdout, "  %s\n", result.Error) //nolint:errcheck // terminal output
			}
		}
	}

	if !*quiet {
		if skipped > 0 {
			fmt.Fprintf(os.Stdout, "\n%d passed, %d failed, %d skipped\n", passed, failed, skipped) //nolint:errcheck // terminal output
		} else {
			fmt.Fprintf(os.Stdout, "\n%d passed, %d failed\n", passed, failed) //nolint:errcheck // terminal output
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d test(s) failed", failed)
	}
	return nil
}

func zeTestFormatElapsed(d time.Duration) string {
	if d < time.Second {
		return textbuf.IntStr(d.Milliseconds(), "ms")
	}
	var b textbuf.Buffer
	b.Float2(d.Seconds()).Str("s")
	return b.String()
}

func zeTestCloseBrowser() {
	cmd := exec.CommandContext(context.Background(), "agent-browser", "--ignore-https-errors", "close", "--all") //nolint:gosec // fixed binary name
	_ = cmd.Run()
}

type zeTestWebTest struct {
	runner.BaseTest
	Path string
}

type zeTestWebServer struct {
	cmd     *exec.Cmd
	tempDir string
}

func zeTestStartWebServer(zeBin, listenAddr string) (*zeTestWebServer, error) {
	ctx := context.Background()
	_, port, _ := net.SplitHostPort(listenAddr)
	tempDir, tempErr := os.MkdirTemp("", "ze-web-test-*")
	if tempErr != nil {
		return nil, fmt.Errorf("create temp config dir: %w", tempErr)
	}
	cmd := exec.CommandContext(ctx, zeBin, "start", "--web", port, "--insecure-web") //nolint:gosec // test binary path
	cmd.Env = append(os.Environ(), "ze.web.ui=finder", "ZE_WEB_UI=finder", "ze.config.dir="+tempDir)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}

	time.Sleep(3 * time.Second)

	return &zeTestWebServer{cmd: cmd, tempDir: tempDir}, nil
}

func (s *zeTestWebServer) stop() {
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_ = s.cmd.Wait()
	}
	if s.tempDir != "" {
		_ = os.RemoveAll(s.tempDir)
	}
}
