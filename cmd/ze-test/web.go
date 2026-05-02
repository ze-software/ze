// Design: docs/architecture/testing/ci-format.md -- web browser test CLI

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
	"strings"
	"syscall"
	"time"

	webtesting "codeberg.org/thomas-mangin/ze/internal/component/web/testing"
	"codeberg.org/thomas-mangin/ze/internal/test/runner"
)

var _ = register("web", "Run web browser functional tests (.wb files)", webCmd)

func webCmd() int {
	if err := webMain(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func webMain() error {
	fs := flag.NewFlagSet("web", flag.ExitOnError)
	pattern := fs.String("p", "", "run only tests matching pattern")
	fs.StringVar(pattern, "pattern", "", "run only tests matching pattern")
	verbose := fs.Bool("v", false, "verbose output")
	fs.BoolVar(verbose, "verbose", false, "verbose output")
	listOnly := fs.Bool("l", false, "list tests without running")
	fs.BoolVar(listOnly, "list", false, "list tests without running")
	port := fs.String("port", "", "port for test web server (default: random free port)")
	fs.Bool("all", false, "run all tests (default behavior)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze-test web [options]

Run web browser functional tests (.wb files).
Requires: agent-browser CLI, ze binary in bin/.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  ze-test web                  Run all tests in test/web/
  ze-test web -p nav           Run tests matching "nav"
  ze-test web -v               Verbose output
  ze-test web -l               List tests without running
`)
	}

	if len(os.Args) > 1 && isHelpArg(os.Args[1]) {
		fs.Usage()
		return nil
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	baseDir, err := findBaseDir()
	if err != nil {
		return fmt.Errorf("find base dir: %w", err)
	}

	testDir := filepath.Join(baseDir, "test", "web")

	// Discover .wb files.
	var tests []webTest
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
		tests = append(tests, webTest{Name: rel, Path: path})
		return nil
	}); walkErr != nil {
		return walkErr
	}

	if len(tests) == 0 {
		return fmt.Errorf("no .wb files found in %s", testDir)
	}

	// Filter by pattern.
	if *pattern != "" {
		var filtered []webTest
		for _, t := range tests {
			if strings.Contains(t.Name, *pattern) {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("no tests matching pattern %q", *pattern)
		}
		tests = filtered
	}

	// List mode.
	if *listOnly {
		fmt.Fprintf(os.Stdout, "Found %d web tests:\n", len(tests)) //nolint:errcheck // terminal output
		for _, t := range tests {
			fmt.Fprintf(os.Stdout, "  %s\n", t.Name) //nolint:errcheck // terminal output
		}
		return nil
	}

	// Pick a free port if none specified.
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
		*port = fmt.Sprintf("%d", tcpAddr.Port)
		if closeErr := ln.Close(); closeErr != nil {
			return fmt.Errorf("close temp listener: %w", closeErr)
		}
	}

	// Signal handling: close agent-browser daemon on interrupt.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		closeBrowser()
		cancel()
	}()

	// Start ze web server.
	listenAddr := "127.0.0.1:" + *port
	baseURL := "https://" + listenAddr
	zeBin := filepath.Join(baseDir, "bin", "ze")

	srv, err := startTestWebServer(zeBin, listenAddr)
	if err != nil {
		return fmt.Errorf("start web server: %w", err)
	}
	defer srv.stop()

	closeBrowser()
	defer closeBrowser()

	// Run tests sequentially (one browser session, shared server).
	colors := runner.NewColors()
	passed, failed, skipped := 0, 0, 0

	for _, t := range tests {
		if ctx.Err() != nil {
			break
		}
		result := webtesting.RunWBFile(t.Path, baseURL)
		switch {
		case result.Skipped:
			skipped++
			if *verbose {
				fmt.Fprintf(os.Stdout, "○ %s (%s)\n", t.Name, result.SkipReason) //nolint:errcheck // terminal output
			}
		case result.Passed:
			passed++
			if *verbose {
				fmt.Fprintln(os.Stdout, colors.Green("✓ "+t.Name)) //nolint:errcheck // terminal output
			}
		default:
			failed++
			fmt.Fprintln(os.Stdout, colors.Red("✗ "+t.Name)) //nolint:errcheck // terminal output
			fmt.Fprintf(os.Stdout, "  %s\n", result.Error)   //nolint:errcheck // terminal output
		}
	}

	if skipped > 0 {
		fmt.Fprintf(os.Stdout, "\n%d passed, %d failed, %d skipped\n", passed, failed, skipped) //nolint:errcheck // terminal output
	} else {
		fmt.Fprintf(os.Stdout, "\n%d passed, %d failed\n", passed, failed) //nolint:errcheck // terminal output
	}

	if failed > 0 {
		return fmt.Errorf("%d test(s) failed", failed)
	}
	return nil
}

func closeBrowser() {
	cmd := exec.CommandContext(context.Background(), "agent-browser", "--ignore-https-errors", "close", "--all") //nolint:gosec // fixed binary name
	_ = cmd.Run()
}

type webTest struct {
	Name string
	Path string
}

// testWebServer holds the ze web process.
type testWebServer struct {
	cmd     *exec.Cmd
	tempDir string
}

func startTestWebServer(zeBin, listenAddr string) (*testWebServer, error) {
	ctx := context.Background()
	// Extract port from listenAddr (e.g. "127.0.0.1:8443" -> "8443").
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

	// Wait for server to be ready.
	time.Sleep(3 * time.Second)

	return &testWebServer{cmd: cmd, tempDir: tempDir}, nil
}

func (s *testWebServer) stop() {
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_ = s.cmd.Wait()
	}
	if s.tempDir != "" {
		_ = os.RemoveAll(s.tempDir)
	}
}
