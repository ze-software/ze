//go:generate go run generate.go

package mutation

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/sivchari/gomu/internal/analysis"
)

// Engine handles mutation generation.
type Engine struct {
	analyzer *analysis.Analyzer
	mutators []Mutator
}

// Mutant represents a single mutation.
type Mutant struct {
	ID          string `json:"id"`
	FilePath    string `json:"filePath"`
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	Type        string `json:"type"`
	Original    string `json:"original"`
	Mutated     string `json:"mutated"`
	Description string `json:"description"`
	Context     string `json:"context,omitempty"`  // Source code context around the mutation
	Function    string `json:"function,omitempty"` // Function name containing the mutation
}

// Result represents the result of testing a mutant.
type Result struct {
	Mutant        Mutant     `json:"mutant"`
	Status        Status     `json:"status"`
	Output        string     `json:"output,omitempty"`
	Error         string     `json:"error,omitempty"`
	ExecutionTime int64      `json:"executionTime,omitempty"` // Execution time in milliseconds
	TestsRun      int        `json:"testsRun,omitempty"`      // Number of tests executed
	TestsFailed   int        `json:"testsFailed,omitempty"`   // Number of tests that failed
	TestOutput    []TestInfo `json:"testOutput,omitempty"`    // Detailed test execution information
}

// TestInfo represents information about a specific test execution.
type TestInfo struct {
	Name     string `json:"name"`               // Test function name
	Package  string `json:"package,omitempty"`  // Package name
	Status   string `json:"status"`             // PASS, FAIL, SKIP
	Duration int64  `json:"duration,omitempty"` // Duration in milliseconds
	Output   string `json:"output,omitempty"`   // Test output/error message
}

// Status represents the outcome of testing a mutant.
type Status string

const (
	// StatusKilled indicates that a mutant was detected by tests.
	StatusKilled Status = "KILLED" // Mutant was detected by tests
	// StatusSurvived indicates that a mutant was not detected by tests.
	StatusSurvived Status = "SURVIVED" // Mutant was not detected by tests
	// StatusTimedOut indicates that tests timed out.
	StatusTimedOut Status = "TIMED_OUT" // Tests timed out
	// StatusError indicates a build or runtime error.
	StatusError Status = "ERROR" // Build or runtime error
	// StatusNotViable indicates that a mutant causes compilation failure.
	StatusNotViable Status = "NOT_VIABLE" // Mutant causes compilation failure
)

// Mutator interface for different types of mutations.
type Mutator interface {
	Name() string
	CanMutate(node ast.Node) bool
	Mutate(node ast.Node, fset *token.FileSet) []Mutant
	Apply(node ast.Node, mutant Mutant) bool
}

// CursorApplier is an optional interface that mutators can implement
// to support mutations requiring parent node context (e.g., node replacement).
type CursorApplier interface {
	ApplyWithCursor(node ast.Node, replaceFunc func(ast.Node), mutant Mutant) bool
}

// New creates a new mutation engine.
func New() (*Engine, error) {
	analyzer, err := analysis.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create analyzer: %w", err)
	}

	engine := &Engine{
		analyzer: analyzer,
		mutators: make([]Mutator, 0),
	}

	// Register all mutators from generated registry
	engine.mutators = getAllMutators()

	return engine, nil
}

// GenerateMutants generates all possible mutants for a given file.
func (e *Engine) GenerateMutants(filePath string) ([]Mutant, error) {
	fileInfo, err := e.analyzer.ParseFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	// Create type checker if type info is available
	var typeChecker *TypeChecker
	if fileInfo.TypeInfo != nil {
		typeChecker = NewTypeChecker(fileInfo.TypeInfo)
	}

	var allMutants []Mutant

	// Walk the AST and apply mutators
	ast.Inspect(fileInfo.FileAST, func(node ast.Node) bool {
		if node == nil {
			return false
		}

		for _, mutator := range e.mutators {
			if mutator.CanMutate(node) {
				mutants := mutator.Mutate(node, e.analyzer.GetFileSet())

				// Filter mutants based on type information
				for i := range mutants {
					mutants[i].FilePath = filePath
					mutants[i].ID = fmt.Sprintf("%s_%d", filePath, len(allMutants)+i)

					// Only add mutant if it passes type check
					if typeChecker == nil || typeChecker.IsValidMutation(node, mutants[i]) {
						allMutants = append(allMutants, mutants[i])
					}
				}
			}
		}

		return true
	})

	return allMutants, nil
}

// GetFileSet returns the file set used by the engine.
func (e *Engine) GetFileSet() *token.FileSet {
	return e.analyzer.GetFileSet()
}

// GetMutators returns all mutators in the engine.
func (e *Engine) GetMutators() []Mutator {
	return e.mutators
}
