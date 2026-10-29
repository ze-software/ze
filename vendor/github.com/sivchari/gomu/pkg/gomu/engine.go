// Package gomu provides the main API for mutation testing.
package gomu

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/sivchari/gomu/internal/analysis"
	"github.com/sivchari/gomu/internal/ci"
	"github.com/sivchari/gomu/internal/execution"
	"github.com/sivchari/gomu/internal/history"
	"github.com/sivchari/gomu/internal/ignore"
	"github.com/sivchari/gomu/internal/mutation"
	"github.com/sivchari/gomu/internal/report"
)

// Engine is the main mutation testing engine.
type Engine struct {
	analyzer            *analysis.Analyzer
	mutator             *mutation.Engine
	executor            *execution.Engine
	history             *history.Store
	reporter            *report.Generator
	incrementalAnalyzer *analysis.IncrementalAnalyzer
	qualityGate         *ci.QualityGateEvaluator
	ciReporter          *ci.Reporter
	github              *ci.GitHubIntegration
}

// RunOptions contains options for running mutation testing.
type RunOptions struct {
	Workers     int
	Timeout     int
	Output      string
	Incremental bool
	BaseBranch  string
	BaseSHA     string // Commit to diff against for incremental analysis; takes precedence over BaseBranch
	Threshold   float64
	FailOnGate  bool
	Verbose     bool
	CIMode      bool
	DryRun      bool     // Discover mutants per file without executing any mutation
	TestArgs    []string // Flags forwarded to go test (and, where applicable, go build)

	// IncludeGenerated makes files carrying the standard Go generated-code
	// marker eligible as mutation targets. They are excluded by default.
	IncludeGenerated bool
}

// rejectedTestFlags maps a go test flag name (without leading dashes) that
// TestArgs must not contain to the reason it conflicts with gomu's mutation
// overlay execution.
var rejectedTestFlags = map[string]string{
	"overlay": "gomu manages the overlay for mutation testing",
	"c":       "it would compile a test binary instead of running tests",
	"o":       "it would compile a test binary instead of running tests",
	"args":    "it would break gomu's positional package argument",
}

// ValidateTestArgs rejects TestArgs entries that conflict with gomu's
// mutation overlay execution (-overlay, -c, -o, -args, in -flag/--flag/-flag=value
// form). It is called by Engine.Run before any mutation runs, so both CLI and
// library callers get the same actionable error.
func ValidateTestArgs(testArgs []string) error {
	for _, arg := range testArgs {
		name := strings.TrimLeft(arg, "-")
		if name == arg {
			continue // not a flag
		}

		if idx := strings.Index(name, "="); idx >= 0 {
			name = name[:idx]
		}

		if reason, rejected := rejectedTestFlags[name]; rejected {
			return fmt.Errorf("test arg %q is not allowed: %s", arg, reason)
		}
	}

	return nil
}

// MutatorInfo describes a single supported mutator for catalog/listing purposes.
type MutatorInfo struct {
	Name        string
	Description string
}

// SupportedMutators returns the catalog of mutators gomu can apply. It does not
// run any mutation and constructs no analyzer, so it is cheap to call for
// listing purposes.
func SupportedMutators() []MutatorInfo {
	mutators := mutation.SupportedMutators()
	infos := make([]MutatorInfo, len(mutators))

	for i, m := range mutators {
		infos[i] = MutatorInfo{Name: m.Name(), Description: m.Description()}
	}

	return infos
}

// NewEngine creates a new mutation testing engine.
func NewEngine(opts *RunOptions) (*Engine, error) {
	// Create analyzer without ignore parser - it will be set later in Run
	analyzer, err := analysis.New(analysis.WithIncludeGenerated(opts != nil && opts.IncludeGenerated))
	if err != nil {
		return nil, fmt.Errorf("failed to create analyzer: %w", err)
	}

	mutator, err := mutation.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create mutator: %w", err)
	}

	executor, err := execution.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	if opts != nil {
		executor.SetTestArgs(opts.TestArgs)
	}

	historyFile := ".gomu_history.json"

	historyStore, err := history.New(historyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create history store: %w", err)
	}

	outputFormat := "console"
	if opts != nil && opts.Output != "" {
		outputFormat = opts.Output
	}

	reporter, err := report.New(outputFormat)
	if err != nil {
		return nil, fmt.Errorf("failed to create reporter: %w", err)
	}

	engine := &Engine{
		analyzer:            analyzer,
		mutator:             mutator,
		executor:            executor,
		history:             historyStore,
		reporter:            reporter,
		incrementalAnalyzer: nil,
	}

	// Initialize CI components if CI mode is enabled
	if opts != nil && opts.CIMode {
		engine.initializeCIComponents(opts)
	}

	return engine, nil
}

// initializeCIComponents initializes CI-specific components.
func (e *Engine) initializeCIComponents(opts *RunOptions) {
	// Set intelligent defaults if opts is nil
	threshold := 80.0
	outputFormat := "console"

	if opts != nil {
		threshold = opts.Threshold
		if opts.Output != "" {
			outputFormat = opts.Output
		}
	}

	// Initialize quality gate
	e.qualityGate = ci.NewQualityGateEvaluator(
		true, // enabled by default
		threshold,
	)

	// Initialize CI reporter
	outputDir := "."
	e.ciReporter = ci.NewReporter(outputDir, outputFormat)

	// Initialize GitHub integration with environment detection
	e.initializeGitHubIntegration(opts)
}

// initializeGitHubIntegration initializes GitHub integration if conditions are met.
func (e *Engine) initializeGitHubIntegration(opts *RunOptions) {
	ciConfig := ci.LoadConfigFromEnv()
	if !ciConfig.IsCIMode() || ciConfig.PRNumber < 0 {
		return
	}

	token := os.Getenv("GITHUB_TOKEN")
	repo := os.Getenv("GITHUB_REPOSITORY")

	if token != "" && repo != "" {
		e.github = ci.NewGitHubIntegration(token, repo, ciConfig.PRNumber)
		if opts != nil && opts.Verbose {
			log.Printf("Initialized GitHub integration for PR #%d", ciConfig.PRNumber)
		}
	} else if opts != nil && opts.Verbose {
		log.Printf("GitHub integration disabled: missing token or repository")
	}
}

// setDefaultOptions sets default values for options if not provided.
func (e *Engine) setDefaultOptions(opts *RunOptions) *RunOptions {
	if opts == nil {
		return &RunOptions{
			Workers:     4,
			Timeout:     30,
			Output:      "console",
			Incremental: true,
			BaseBranch:  "main",
			Threshold:   80.0,
			FailOnGate:  true,
			Verbose:     false,
		}
	}

	return opts
}

// logStartupInfo logs startup information if verbose mode is enabled.
func (e *Engine) logStartupInfo(path string, opts *RunOptions) {
	if opts.Verbose {
		log.Printf("Starting mutation testing on path: %s", path)
		log.Printf("Running with options: workers=%d, timeout=%d, output=%s, incremental=%t",
			opts.Workers, opts.Timeout, opts.Output, opts.Incremental)
	}
}

// getAbsolutePath gets the absolute path for the working directory.
func (e *Engine) getAbsolutePath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	return absPath, nil
}

// performIncrementalAnalysis performs incremental analysis and returns results and files to process.
func (e *Engine) performIncrementalAnalysis(absPath string, opts *RunOptions, ignoreParser analysis.IgnoreParser) ([]analysis.FileAnalysisResult, []string, error) {
	// Initialize incremental analyzer
	historyWrapper := &historyStoreWrapper{store: e.history}

	var err error

	e.incrementalAnalyzer, err = analysis.NewIncrementalAnalyzer(absPath, historyWrapper, opts.Incremental, opts.BaseBranch)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create incremental analyzer: %w", err)
	}

	// Set ignore parser if provided
	if ignoreParser != nil {
		e.incrementalAnalyzer.SetIgnoreParser(ignoreParser)
	}

	e.incrementalAnalyzer.SetIncludeGenerated(opts.IncludeGenerated)
	e.incrementalAnalyzer.SetBaseSHA(opts.BaseSHA)

	// Perform incremental analysis
	analysisResults, err := e.incrementalAnalyzer.AnalyzeFiles()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to analyze files: %w", err)
	}

	if opts.Verbose {
		e.incrementalAnalyzer.PrintAnalysisReport(analysisResults)
	}

	// Get files that need processing
	files, err := e.incrementalAnalyzer.GetFilesNeedingUpdate()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get files needing update: %w", err)
	}

	if opts.Verbose {
		log.Printf("Processing %d files", len(files))
	}

	return analysisResults, files, nil
}

// Run executes mutation testing on the specified path.
func (e *Engine) Run(ctx context.Context, path string, opts *RunOptions) error {
	opts = e.setDefaultOptions(opts)

	if err := ValidateTestArgs(opts.TestArgs); err != nil {
		return err
	}

	start := time.Now()

	e.logStartupInfo(path, opts)

	absPath, err := e.getAbsolutePath(path)
	if err != nil {
		return err
	}

	// Load .gomuignore file from the target path
	ignoreFile, err := ignore.FindIgnoreFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to find .gomuignore file: %w", err)
	}

	var ignoreParser analysis.IgnoreParser

	if ignoreFile != "" {
		parser := ignore.New()
		if err := parser.LoadFromFile(ignoreFile); err != nil {
			return fmt.Errorf("failed to load .gomuignore file: %w", err)
		}

		if opts.Verbose {
			log.Printf("Loaded .gomuignore file from: %s", ignoreFile)
		}

		// Create new analyzer with ignore parser
		analyzer, err := analysis.New(analysis.WithIgnoreParser(parser), analysis.WithIncludeGenerated(opts.IncludeGenerated))
		if err != nil {
			return fmt.Errorf("failed to create analyzer with ignore parser: %w", err)
		}

		e.analyzer = analyzer
		ignoreParser = parser
	}

	analysisResults, files, err := e.performIncrementalAnalysis(absPath, opts, ignoreParser)
	if err != nil {
		return err
	}

	// Dry run stops after discovery: report the mutants that would be generated
	// per file without applying any mutation, running tests, or reporting.
	if opts.DryRun {
		return e.dryRun(os.Stdout, files, opts)
	}

	if len(files) == 0 {
		if opts.Verbose {
			log.Println("No files need processing - all files are up to date")
		}

		return nil
	}

	allResults, totalMutants, processedFiles := e.processFiles(files, opts)

	if err := e.cleanupAndSave(opts); err != nil {
		return err
	}

	summary := e.buildSummary(analysisResults, totalMutants, allResults, processedFiles, start)

	if err := e.reporter.Generate(summary); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	if err := e.handleCIWorkflow(ctx, summary, opts); err != nil {
		return err
	}

	if opts.Verbose {
		log.Printf("Mutation testing completed in %v", time.Since(start))
	}

	return nil
}

// processFiles processes all files for mutation testing.
func (e *Engine) processFiles(files []string, opts *RunOptions) ([]mutation.Result, int, int) {
	var (
		allResults     []mutation.Result
		totalMutants   int
		processedFiles int
	)

	hasher := analysis.NewFileHasher()
	totalFiles := len(files)

	fmt.Printf("Processing %d file(s)...\n", totalFiles)

	for i, file := range files {
		fmt.Printf("[%d/%d] %s ", i+1, totalFiles, filepath.Base(file))

		if opts.Verbose {
			log.Printf("Processing file: %s", file)
		}

		mutants, err := e.mutator.GenerateMutants(file)
		if err != nil {
			fmt.Printf("(error: %v)\n", err)

			if opts.Verbose {
				log.Printf("Warning: failed to generate mutants for %s: %v", file, err)
			}

			continue
		}

		if len(mutants) == 0 {
			fmt.Println("(no mutants)")

			if opts.Verbose {
				log.Printf("No mutants generated for file: %s", file)
			}

			continue
		}

		totalMutants += len(mutants)

		fmt.Printf("(%d mutants) ", len(mutants))

		if opts.Verbose {
			log.Printf("Generated %d mutants for %s", len(mutants), file)
		}

		results, err := e.executor.RunMutationsWithOptions(mutants, opts.Workers, opts.Timeout)
		if err != nil {
			fmt.Printf("(execution error: %v)\n", err)

			if opts.Verbose {
				log.Printf("Warning: failed to execute mutations for %s: %v", file, err)
			}

			continue
		}

		// Count killed mutants for this file
		killed := 0

		for _, r := range results {
			if r.Status == mutation.StatusKilled {
				killed++
			}
		}

		fmt.Printf("-> %d/%d killed\n", killed, len(mutants))

		allResults = append(allResults, results...)

		fileHash, err := hasher.HashFile(file)
		if err != nil {
			if opts.Verbose {
				log.Printf("Warning: failed to hash file %s: %v", file, err)
			}

			fileHash = ""
		}

		testHash := hasher.HashRelatedTests(file)
		e.history.UpdateFileWithHashes(file, mutants, results, fileHash, testHash)

		processedFiles++
	}

	return allResults, totalMutants, processedFiles
}

// dryRun reports the mutants that would be generated for each file without
// applying any mutation, running any tests, or producing reports.
func (e *Engine) dryRun(w io.Writer, files []string, opts *RunOptions) error {
	if len(files) == 0 {
		if opts.Incremental {
			base := opts.BaseBranch
			if opts.BaseSHA != "" {
				base = opts.BaseSHA
			}

			fmt.Fprintf(w, "Dry run: no files to analyze (no changes since %s; use --incremental=false to preview all files)\n", base)
		} else {
			fmt.Fprintln(w, "Dry run: no files to analyze")
		}

		return nil
	}

	fmt.Fprintf(w, "Dry run: analyzing %d file(s) (no mutations will be executed)\n", len(files))

	// Discovered files are absolute (derived from the resolved target path);
	// display them relative to the working directory.
	wd, wdErr := os.Getwd()
	if wdErr != nil {
		wd = ""
	}

	totalMutants := 0
	filesWithMutants := 0

	for _, file := range files {
		mutants, err := e.mutator.GenerateMutants(file)
		displayPath := analysis.GetRelativePath(wd, file)

		if err != nil {
			fmt.Fprintf(w, "\n%s (error: %v)\n", displayPath, err)

			continue
		}

		if len(mutants) == 0 {
			continue
		}

		filesWithMutants++
		totalMutants += len(mutants)

		if err := writeDryRunFile(w, displayPath, mutants); err != nil {
			return err
		}
	}

	fmt.Fprintf(w, "\nTotal: %d mutant(s) across %d file(s)\n", totalMutants, filesWithMutants)

	return nil
}

// writeDryRunFile renders a file's discovered mutants as an aligned table.
func writeDryRunFile(w io.Writer, file string, mutants []mutation.Mutant) error {
	fmt.Fprintf(w, "\n%s (%d mutants)\n", file, len(mutants))

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, m := range mutants {
		fmt.Fprintf(tw, "  L%d:%d\t%s\t%s\n", m.Line, m.Column, m.Type, m.Description)
	}

	if err := tw.Flush(); err != nil {
		return fmt.Errorf("failed to write mutant table: %w", err)
	}

	return nil
}

// cleanupAndSave handles cleanup and saving operations.
func (e *Engine) cleanupAndSave(opts *RunOptions) error {
	if err := e.executor.Close(); err != nil {
		if opts.Verbose {
			log.Printf("Warning: failed to cleanup execution engine: %v", err)
		}
	}

	if err := e.history.Save(); err != nil {
		if opts.Verbose {
			log.Printf("Warning: failed to save history: %v", err)
		}
	}

	return nil
}

// buildSummary builds the mutation testing summary.
func (e *Engine) buildSummary(analysisResults []analysis.FileAnalysisResult, totalMutants int, allResults []mutation.Result, processedFiles int, start time.Time) *report.Summary {
	return &report.Summary{
		TotalFiles:     len(analysisResults),
		TotalMutants:   totalMutants,
		Results:        allResults,
		Duration:       time.Since(start),
		ProcessedFiles: processedFiles,
	}
}

// handleCIWorkflow handles CI-specific processing if CI mode is enabled.
func (e *Engine) handleCIWorkflow(ctx context.Context, summary *report.Summary, opts *RunOptions) error {
	if opts == nil || !opts.CIMode {
		return nil
	}

	e.initializeCIComponents(opts)

	if err := e.processCIWorkflow(ctx, summary, opts); err != nil {
		return fmt.Errorf("CI workflow failed: %w", err)
	}

	return nil
}

// processCIWorkflow handles CI-specific processing after mutation testing.
func (e *Engine) processCIWorkflow(ctx context.Context, summary *report.Summary, opts *RunOptions) error {
	if opts.Verbose {
		log.Println("Processing CI workflow...")
	}

	ciSummary := e.convertToCISummary(summary)

	var qualityResult *ci.QualityGateResult
	if e.qualityGate != nil {
		qualityResult = e.qualityGate.Evaluate(ciSummary)
		fmt.Printf("Quality Gate: %s (Score: %.1f%%)\n",
			map[bool]string{true: "PASSED", false: "FAILED"}[qualityResult.Pass],
			qualityResult.MutationScore)

		if !qualityResult.Pass {
			fmt.Printf("Reason: %s\n", qualityResult.Reason)
		}
	}

	if e.ciReporter != nil {
		if err := e.ciReporter.Generate(ciSummary, e.qualityGate); err != nil {
			return fmt.Errorf("failed to generate CI reports: %w", err)
		}
	}

	if e.github != nil && qualityResult != nil {
		if err := e.github.CreatePRComment(ctx, ciSummary, qualityResult); err != nil {
			fmt.Printf("Warning: Failed to create PR comment: %v\n", err)
		} else {
			fmt.Println("Created PR comment with mutation testing results")
		}
	}

	if qualityResult != nil && !qualityResult.Pass && opts.FailOnGate {
		return fmt.Errorf("quality gate failed: %s", qualityResult.Reason)
	}

	return nil
}

// convertToCISummary converts report.Summary to CI format.
func (e *Engine) convertToCISummary(summary *report.Summary) *report.Summary {
	// Create file reports map
	files := make(map[string]*report.FileReport)

	totalMutants := 0
	killedMutants := 0

	// Group results by file
	fileResults := make(map[string][]mutation.Result)
	for _, result := range summary.Results {
		fileResults[result.Mutant.FilePath] = append(fileResults[result.Mutant.FilePath], result)
	}

	// Create file reports - only include files with actual mutations
	for filePath, results := range fileResults {
		// Skip files with no results (shouldn't happen but be defensive)
		if len(results) == 0 {
			continue
		}

		fileReport := &report.FileReport{
			FilePath:      filePath,
			TotalMutants:  len(results),
			KilledMutants: 0,
		}

		for _, result := range results {
			switch result.Status {
			case mutation.StatusKilled:
				fileReport.KilledMutants++
				killedMutants++
			case mutation.StatusNotViable:
				fileReport.NotViable++
			}

			totalMutants++
		}

		if fileReport.TotalMutants > 0 {
			fileReport.MutationScore = report.MutationScore(fileReport.KilledMutants, fileReport.TotalMutants, fileReport.NotViable)
			// Only add files with mutations to the report
			files[filePath] = fileReport
		}
	}

	return &report.Summary{
		Files:         files,
		TotalMutants:  totalMutants,
		KilledMutants: killedMutants,
		Duration:      summary.Duration,
		Statistics:    summary.Statistics,
	}
}

// historyStoreWrapper wraps history.Store to implement analysis.HistoryStore interface.
type historyStoreWrapper struct {
	store *history.Store
}

func (w *historyStoreWrapper) GetEntry(filePath string) (analysis.HistoryEntry, bool) {
	entry, exists := w.store.GetEntry(filePath)
	if !exists {
		return analysis.HistoryEntry{}, false
	}

	return analysis.HistoryEntry{
		FileHash:      entry.FileHash,
		TestHash:      entry.TestHash,
		MutationScore: entry.MutationScore,
	}, true
}

func (w *historyStoreWrapper) HasChanged(filePath, currentHash string) bool {
	return w.store.HasChanged(filePath, currentHash)
}
