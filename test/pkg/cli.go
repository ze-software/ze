package functional

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// CLI handles command-line argument parsing.
type CLI struct {
	// Subcommand: encoding, api
	Command string

	// Test selection
	All      bool
	TestArgs []string

	// Modes
	List      bool
	ShortList bool
	Edit      bool
	Dry       bool

	// Options
	Timeout  time.Duration
	Parallel int
	Verbose  bool
	Debug    []string
	Quiet    bool
	SaveDir  string
	Stress   int

	// Server/client mode (for debugging)
	Server string
	Client string
	Port   int
}

// DefaultCLI returns a CLI with default values.
func DefaultCLI() *CLI {
	return &CLI{
		Timeout:  30 * time.Second,
		Parallel: 4,
		Port:     1790,
	}
}

// Parse parses command-line arguments.
func (c *CLI) Parse(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: functional <encoding|api> [options] [tests...]")
	}

	c.Command = args[0]
	if c.Command != "encoding" && c.Command != "api" {
		return fmt.Errorf("unknown command: %s (use 'encoding' or 'api')", c.Command)
	}

	fs := flag.NewFlagSet(c.Command, flag.ContinueOnError)

	// Mode flags
	fs.BoolVar(&c.List, "list", false, "list available tests")
	fs.BoolVar(&c.List, "l", false, "list available tests (shorthand)")
	fs.BoolVar(&c.ShortList, "short-list", false, "list test codes only")
	fs.BoolVar(&c.Edit, "edit", false, "edit test files in $EDITOR")
	fs.BoolVar(&c.Dry, "dry", false, "show commands without running")
	fs.BoolVar(&c.All, "all", false, "run all tests")

	// Option flags
	fs.DurationVar(&c.Timeout, "timeout", 30*time.Second, "timeout per test")
	fs.IntVar(&c.Parallel, "parallel", 4, "max concurrent tests")
	fs.BoolVar(&c.Verbose, "verbose", false, "show output for each test")
	fs.BoolVar(&c.Verbose, "v", false, "verbose (shorthand)")
	fs.BoolVar(&c.Quiet, "quiet", false, "minimal output")
	fs.BoolVar(&c.Quiet, "q", false, "quiet (shorthand)")
	fs.StringVar(&c.SaveDir, "save", "", "save logs to directory")
	fs.IntVar(&c.Stress, "stress", 0, "run test N times (stress mode)")

	// Debug mode
	fs.IntVar(&c.Port, "port", 1790, "base port to use")
	fs.StringVar(&c.Server, "server", "", "run server for specific test")
	fs.StringVar(&c.Client, "client", "", "run client for specific test")

	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	c.TestArgs = fs.Args()

	return nil
}

// ToRunOptions converts CLI settings to RunOptions.
func (c *CLI) ToRunOptions() *RunOptions {
	return &RunOptions{
		Timeout:    c.Timeout,
		Parallel:   c.Parallel,
		Verbose:    c.Verbose,
		DebugNicks: c.Debug,
		Quiet:      c.Quiet,
		SaveDir:    c.SaveDir,
	}
}

// PrintUsage prints usage information.
func (c *CLI) PrintUsage() {
	fmt.Fprintf(os.Stderr, `Usage: functional <command> [options] [tests...]

Commands:
  encoding    Run encoding tests (static routes)
  api         Run API tests (dynamic routes via .run scripts)

Modes:
  --list, -l          List available tests
  --short-list        List test codes only (space separated)
  --all               Run all tests
  --edit              Open test files in $EDITOR
  --dry               Show commands without running

Options:
  --timeout N         Timeout per test (default: 30s)
  --parallel N        Max concurrent tests (default: 4)
  --verbose, -v       Show output for each test
  --quiet, -q         Minimal output
  --save DIR          Save logs to directory
  --stress N          Run test N times

Debugging:
  --server NICK       Run server only for test
  --client NICK       Run client only for test
  --port N            Base port to use (default: 1790)

Examples:
  functional encoding --list
  functional encoding 0 1 2
  functional encoding --all --verbose
  functional api --all --quiet
  functional encoding --stress 10 ebgp
`)
}

// PrintSummary prints test result summary.
func PrintSummary(passed, failed, timedOut, skipped int, failedNicks []string) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("TEST SUMMARY")
	fmt.Println(strings.Repeat("=", 60))

	if passed > 0 {
		fmt.Printf("%spassed%s    %d\n", colorGreen, colorReset, passed)
	}
	if failed > 0 {
		fmt.Printf("%sfailed%s    %d [%s]\n", colorRed, colorReset, failed, strings.Join(failedNicks, ", "))
	}
	if timedOut > 0 {
		fmt.Printf("%stimed out%s %d\n", colorYellow, colorReset, timedOut)
	}
	if skipped > 0 {
		fmt.Printf("%sskipped%s   %d\n", colorGray, colorReset, skipped)
	}

	fmt.Println(strings.Repeat("=", 60))

	total := passed + failed + timedOut
	if total > 0 {
		rate := float64(passed) / float64(total) * 100
		fmt.Printf("Total: %d test(s) run, %.1f%% passed\n", total, rate)
	}
	fmt.Println()
}
