// Package analysis provides mutation testing analysis functionality.
package analysis

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sivchari/gomu/internal/ignore"
)

// Analyzer handles code analysis and file discovery.
type Analyzer struct {
	fileSet      *token.FileSet
	typeInfo     *types.Info
	ignoreParser *ignore.Parser
}

// Option is a functional option for configuring an Analyzer.
type Option func(*Analyzer)

// WithIgnoreParser sets the ignore parser for the analyzer.
func WithIgnoreParser(parser *ignore.Parser) Option {
	return func(a *Analyzer) {
		a.ignoreParser = parser
	}
}

// New creates a new analyzer with optional configuration.
func New(opts ...Option) (*Analyzer, error) {
	a := &Analyzer{
		fileSet: token.NewFileSet(),
		typeInfo: &types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
			Uses:  make(map[*ast.Ident]types.Object),
			Defs:  make(map[*ast.Ident]types.Object),
		},
	}

	// Apply options
	for _, opt := range opts {
		opt(a)
	}

	return a, nil
}

// FileInfo represents information about a Go source file.
type FileInfo struct {
	Path     string
	FileAST  *ast.File
	Hash     string
	TypeInfo *types.Info
}

// shouldSkipDirectory checks if a directory should be skipped.
func (a *Analyzer) shouldSkipDirectory(rootPath, path string) bool {
	// Check if directory should be ignored by .gomuignore
	if a.ignoreParser != nil {
		relPath := GetRelativePath(rootPath, path)
		if a.ignoreParser.ShouldIgnore(relPath) {
			return true
		}
	}

	// Skip standard excluded directories (vendor, testdata)
	return IsExcludedPath(path)
}

// shouldIgnoreFile checks if a file should be ignored.
func (a *Analyzer) shouldIgnoreFile(rootPath, path string) bool {
	if a.ignoreParser == nil {
		return false
	}

	relPath := GetRelativePath(rootPath, path)

	return a.ignoreParser.ShouldIgnore(relPath)
}

// FindTargetFiles discovers Go source files to be tested.
func (a *Analyzer) FindTargetFiles(rootPath string) ([]string, error) {
	var files []string

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			if skip := a.shouldSkipDirectory(rootPath, path); skip {
				return filepath.SkipDir
			}

			return nil
		}

		// Only process Go source files (not test files)
		if !IsGoSourceFile(path) {
			return nil
		}

		// Check if file should be ignored by .gomuignore (for file-specific patterns)
		if a.shouldIgnoreFile(rootPath, path) {
			return nil
		}

		files = append(files, path)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return files, nil
}

// ParseFile parses a Go source file and returns its AST.
func (a *Analyzer) ParseFile(filePath string) (*FileInfo, error) {
	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Calculate file hash for incremental analysis
	hash := calculateFileHash(src)

	// Try to get type information by parsing all package files together
	fileAST, typeInfo, err := a.parseAndTypeCheck(filePath)
	if err != nil {
		// Fall back to syntax-only parsing
		fileAST, err = parser.ParseFile(a.fileSet, filePath, src, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("failed to parse file %s: %w", filePath, err)
		}

		typeInfo = nil
	}

	return &FileInfo{
		Path:     filePath,
		FileAST:  fileAST,
		Hash:     hash,
		TypeInfo: typeInfo,
	}, nil
}

// GetPosition returns the position information for a token.
func (a *Analyzer) GetPosition(pos token.Pos) token.Position {
	return a.fileSet.Position(pos)
}

// GetFileSet returns the file set used by the analyzer.
func (a *Analyzer) GetFileSet() *token.FileSet {
	return a.fileSet
}

// parseAndTypeCheck parses all package files and type checks them, returning the AST and type info.
func (a *Analyzer) parseAndTypeCheck(filePath string) (*ast.File, *types.Info, error) {
	pkgDir := filepath.Dir(filePath)

	astFiles, fileMap, err := a.parsePackageFiles(pkgDir)
	if err != nil {
		return nil, nil, err
	}

	targetAST, ok := fileMap[filePath]
	if !ok {
		return nil, nil, fmt.Errorf("target file not found in parsed files")
	}

	if len(astFiles) == 0 {
		return nil, nil, fmt.Errorf("no files to type check")
	}

	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Uses:  make(map[*ast.Ident]types.Object),
		Defs:  make(map[*ast.Ident]types.Object),
	}

	config := &types.Config{
		Importer: importer.Default(),
		Error: func(_ error) {
			// Ignore type errors - we want to return partial type info
		},
	}

	_, _ = config.Check(targetAST.Name.Name, a.fileSet, astFiles, info)

	return targetAST, info, nil
}

// parsePackageFiles parses all non-test Go files in a package directory.
// Returns the AST slice and a map from file path to its AST.
func (a *Analyzer) parsePackageFiles(pkgDir string) ([]*ast.File, map[string]*ast.File, error) {
	files, err := filepath.Glob(filepath.Join(pkgDir, "*.go"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to glob files: %w", err)
	}

	astFiles := make([]*ast.File, 0, len(files))
	fileMap := make(map[string]*ast.File, len(files))

	for _, file := range files {
		if IsGoTestFile(file) {
			continue
		}

		astFile, err := parser.ParseFile(a.fileSet, file, nil, parser.ParseComments)
		if err != nil {
			continue
		}

		astFiles = append(astFiles, astFile)
		fileMap[file] = astFile
	}

	return astFiles, fileMap, nil
}

// calculateFileHash calculates a SHA256 hash for the given file content.
func calculateFileHash(content []byte) string {
	h := sha256.Sum256(content)

	return fmt.Sprintf("%x", h)
}

// isValidBranchName validates branch names to prevent command injection.
func isValidBranchName(name string) bool {
	// Git branch names can contain letters, numbers, hyphens, underscores, dots, and forward slashes
	// but cannot start with a hyphen or contain special characters that could be interpreted as options
	validBranchPattern := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*$`)

	return validBranchPattern.MatchString(name) && !strings.HasPrefix(name, "-")
}
