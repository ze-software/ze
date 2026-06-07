// Package main provides the CLI interface for gomu mutation testing tool.
package main

import (
	"fmt"
	"os"

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
}

var runCmd = &cobra.Command{
	Use:   "run [path]",
	Short: "Run mutation testing on the specified path",
	Long:  "Run mutation testing on the specified path or current directory",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runMutationTesting,
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

	// Run command flags
	runCmd.Flags().Bool("ci-mode", false, "enable CI mode with quality gates and reporting")
	runCmd.Flags().Float64("threshold", 80.0, "minimum mutation score threshold")
	runCmd.Flags().String("output", "console", "output format (console, json, html, text)")
	runCmd.Flags().Bool("fail-on-gate", true, "fail build when quality gate is not met")
	runCmd.Flags().Int("workers", 4, "number of parallel workers")
	runCmd.Flags().Int("timeout", 30, "test timeout in seconds")
	runCmd.Flags().Bool("incremental", true, "enable incremental analysis")
	runCmd.Flags().String("base-branch", "main", "base branch for incremental analysis")
}

func runMutationTesting(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	// Read CLI flags
	ciMode, _ := cmd.Flags().GetBool("ci-mode")
	workers, _ := cmd.Flags().GetInt("workers")
	timeout, _ := cmd.Flags().GetInt("timeout")
	output, _ := cmd.Flags().GetString("output")
	incremental, _ := cmd.Flags().GetBool("incremental")
	baseBranch, _ := cmd.Flags().GetString("base-branch")
	threshold, _ := cmd.Flags().GetFloat64("threshold")
	failOnGate, _ := cmd.Flags().GetBool("fail-on-gate")

	if verbose {
		fmt.Printf("Running mutation testing with the following settings:\n")
		fmt.Printf("  Path: %s\n", path)
		fmt.Printf("  CI Mode: %t\n", ciMode)
		fmt.Printf("  Workers: %d\n", workers)
		fmt.Printf("  Timeout: %d seconds\n", timeout)
		fmt.Printf("  Output: %s\n", output)
		fmt.Printf("  Incremental: %t\n", incremental)
		fmt.Printf("  Base Branch: %s\n", baseBranch)

		if ciMode {
			fmt.Printf("  Threshold: %.1f%%\n", threshold)
			fmt.Printf("  Fail on Gate: %t\n", failOnGate)
		}

		fmt.Println()
	}

	// Create run options from CLI flags
	opts := &gomu.RunOptions{
		Workers:     workers,
		Timeout:     timeout,
		Output:      output,
		Incremental: incremental,
		BaseBranch:  baseBranch,
		Threshold:   threshold,
		FailOnGate:  failOnGate,
		Verbose:     verbose,
		CIMode:      ciMode,
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

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
