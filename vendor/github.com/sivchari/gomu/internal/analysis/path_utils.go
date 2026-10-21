package analysis

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// GetRelativePath returns the relative path from base to target.
// If an error occurs, it returns the target path as-is.
func GetRelativePath(base, target string) string {
	relPath, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}

	return relPath
}

// IsGoSourceFile checks if a file is a Go source file (not a test file).
func IsGoSourceFile(path string) bool {
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}

// IsGoTestFile checks if a file is a Go test file.
func IsGoTestFile(path string) bool {
	return strings.HasSuffix(path, "_test.go")
}

// excludedDirs are directories excluded from mutation testing regardless of
// which file-discovery path (incremental git diff or full walk) is used.
var excludedDirs = []string{"vendor", "testdata"}

// IsExcludedPath reports whether the given path lies within an excluded
// directory (vendor or testdata). The path may use either OS-specific or
// forward slashes; it is matched per path segment so substrings such as
// "myvendor" are not falsely excluded.
func IsExcludedPath(path string) bool {
	segments := strings.Split(filepath.ToSlash(path), "/")
	for _, seg := range segments {
		for _, excluded := range excludedDirs {
			if seg == excluded {
				return true
			}
		}
	}

	return false
}

// IsGeneratedFile reports whether the file at path carries the standard Go
// generated-code marker (https://go.dev/s/generatedcode). Files that cannot
// be read or parsed are treated as not generated.
func IsGeneratedFile(path string) bool {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.PackageClauseOnly|parser.ParseComments)
	if err != nil {
		return false
	}

	return ast.IsGenerated(file)
}
