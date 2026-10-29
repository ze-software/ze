// Package main provides the CLI interface for gomu mutation testing tool.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/sivchari/gomu/pkg/gomu"
	"github.com/spf13/cobra"
)

var (
	// Version information set by ldflags.
	version = "dev"
	commit  = "none"
	date    = "unknown"

	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "gomu",
	Short: "A high-performance mutation testing tool for Go",
	Long: `gomu is a mutation testing tool that helps validate the quality of your Go test suite.
It introduces controlled changes (mutations) to your code and checks if your tests catch them.

Features:
- Incremental analysis for fast reruns
- Git integration for change detection  
- Parallel execution with goroutines
- Go-specific mutations (generics, error handling, etc.)`,
	RunE: runMutationTesting,
	// Usage is noise once flags have parsed and RunE fails (for example on a
	// quality-gate miss); main prints the error itself, so cobra must not too.
	SilenceUsage:  true,
	SilenceErrors: true,
}

var runCmd = newRunCommand()

// newRunCommand builds the "run" subcommand. It is a constructor, rather than
// a package-level literal, so tests can create isolated instances instead of
// reusing the shared runCmd - cobra/pflag retain parse state such as
// ArgsLenAtDash across repeated Execute calls on the same command.
func newRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [path] [-- test flags]",
		Short: "Run mutation testing on the specified path",
		Long: `Run mutation testing on the specified path or current directory.

Flags placed after -- are forwarded to "go test" verbatim (and, for the
flags go build also understands such as -tags or -race, to the compile
check that precedes it):

  gomu run ./pkg -- -short
  gomu run ./pkg -- -run '^TestUnit'
  gomu run ./pkg -- -tags=integration`,
		Args: validateRunArgs,
		RunE: runMutationTesting,
		// See rootCmd: cobra's usage dump after a RunE error is noise.
		SilenceUsage: true,
	}

	cmd.Flags().BoolP("list", "l", false, "list supported mutators and exit")
	cmd.Flags().Bool("dry-run", false, "show which mutations would run per file, without executing them")
	cmd.Flags().Bool("ci-mode", false, "enable CI mode with quality gates and reporting")
	cmd.Flags().Float64("threshold", 80.0, "minimum mutation score threshold")
	cmd.Flags().String("output", "console", "output format (console, json, html, text)")
	cmd.Flags().Bool("fail-on-gate", true, "fail build when quality gate is not met")
	cmd.Flags().Int("workers", 4, "number of parallel workers")
	cmd.Flags().Int("timeout", 30, "test timeout in seconds")
	cmd.Flags().Bool("incremental", true, "skip files unchanged since the last run; false runs all files, ignoring history")
	cmd.Flags().String("base-branch", "main", "base branch for incremental analysis")
	cmd.Flags().String("base-sha", "", "base commit SHA for incremental analysis; when set, changed files are computed with a tree diff against it, so no git history is needed")
	cmd.Flags().Bool("include-generated", false, "include files marked with the standard Go generated-code comment as mutation targets")

	return cmd
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("gomu version %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", date)
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(versionCmd)
}

// validateRunArgs allows at most one positional argument before "--"; any
// number of arguments after "--" are forwarded to go test, not counted here.
func validateRunArgs(cmd *cobra.Command, args []string) error {
	n := len(args)
	if dashAt := cmd.ArgsLenAtDash(); dashAt >= 0 {
		n = dashAt
	}

	if n > 1 {
		return fmt.Errorf("accepts at most 1 arg(s) before --, received %d", n)
	}

	return nil
}

// splitPathAndTestArgs separates the optional path argument from the go test
// flags forwarded after "--", using cmd.ArgsLenAtDash() to find the split
// point Cobra recorded while parsing.
func splitPathAndTestArgs(cmd *cobra.Command, args []string) (path string, testArgs []string) {
	path = "."

	dashAt := cmd.ArgsLenAtDash()
	if dashAt < 0 {
		if len(args) > 0 {
			path = args[0]
		}

		return path, nil
	}

	if dashAt > 0 {
		path = args[0]
	}

	return path, args[dashAt:]
}

func runMutationTesting(cmd *cobra.Command, args []string) error {
	// --list is informational only: print the supported mutators and exit
	// without discovering files or running any mutation.
	if list, _ := cmd.Flags().GetBool("list"); list {
		warnIgnoredFlags(cmd)

		return listMutators(cmd.OutOrStdout())
	}

	path, testArgs := splitPathAndTestArgs(cmd, args)

	// Read CLI flags
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	ciMode, _ := cmd.Flags().GetBool("ci-mode")
	workers, _ := cmd.Flags().GetInt("workers")
	timeout, _ := cmd.Flags().GetInt("timeout")
	output, _ := cmd.Flags().GetString("output")
	incremental, _ := cmd.Flags().GetBool("incremental")
	baseBranch, _ := cmd.Flags().GetString("base-branch")
	baseSHA, _ := cmd.Flags().GetString("base-sha")
	threshold, _ := cmd.Flags().GetFloat64("threshold")
	failOnGate, _ := cmd.Flags().GetBool("fail-on-gate")
	includeGenerated, _ := cmd.Flags().GetBool("include-generated")

	if verbose {
		fmt.Printf("Running mutation testing with the following settings:\n")
		fmt.Printf("  Path: %s\n", path)
		fmt.Printf("  CI Mode: %t\n", ciMode)
		fmt.Printf("  Workers: %d\n", workers)
		fmt.Printf("  Timeout: %d seconds\n", timeout)
		fmt.Printf("  Output: %s\n", output)
		fmt.Printf("  Incremental: %t\n", incremental)
		fmt.Printf("  Base Branch: %s\n", baseBranch)

		if baseSHA != "" {
			fmt.Printf("  Base SHA: %s\n", baseSHA)
		}

		if ciMode {
			fmt.Printf("  Threshold: %.1f%%\n", threshold)
			fmt.Printf("  Fail on Gate: %t\n", failOnGate)
		}

		fmt.Println()
	}

	// Create run options from CLI flags
	opts := &gomu.RunOptions{
		Workers:          workers,
		Timeout:          timeout,
		Output:           output,
		Incremental:      incremental,
		BaseBranch:       baseBranch,
		BaseSHA:          baseSHA,
		Threshold:        threshold,
		FailOnGate:       failOnGate,
		Verbose:          verbose,
		CIMode:           ciMode,
		DryRun:           dryRun,
		IncludeGenerated: includeGenerated,
		TestArgs:         testArgs,
	}

	engine, err := gomu.NewEngine(opts)
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}

	if err := engine.Run(cmd.Context(), path, opts); err != nil {
		return fmt.Errorf("mutation testing failed: %w", err)
	}

	return nil
}

// warnIgnoredFlags warns on stderr about flags the user explicitly set that
// --list ignores, since --list exits before those flags take effect.
func warnIgnoredFlags(cmd *cobra.Command) {
	ignorable := []string{"dry-run", "ci-mode", "threshold", "output", "fail-on-gate", "workers", "timeout", "incremental", "base-branch", "base-sha", "include-generated"}

	var ignored []string

	for _, name := range ignorable {
		if cmd.Flags().Changed(name) {
			ignored = append(ignored, "--"+name)
		}
	}

	if len(ignored) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: --list ignores %s\n", strings.Join(ignored, ", "))
	}
}

// listMutators writes the catalog of supported mutators, one per line, with
// names and descriptions aligned into columns.
func listMutators(w io.Writer) error {
	mutators := gomu.SupportedMutators()

	fmt.Fprintf(w, "Supported mutators (%d):\n", len(mutators))

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, m := range mutators {
		fmt.Fprintf(tw, "  %s\t%s\n", m.Name, m.Description)
	}

	if err := tw.Flush(); err != nil {
		return fmt.Errorf("failed to write mutator list: %w", err)
	}

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
