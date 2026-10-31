package analysis

import (
	"os"
	"path/filepath"
	"strings"
)

// FindRelatedTestFiles finds test files related to the given file.
// This is a shared utility function used by multiple components.
func FindRelatedTestFiles(filePath string) []string {
	var testFiles []string

	// Get directory and base name
	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)

	// Remove .go extension
	nameWithoutExt := strings.TrimSuffix(base, ".go")

	// Common test file patterns
	patterns := []string{
		nameWithoutExt + "_test.go",
		"test_" + nameWithoutExt + ".go",
	}

	for _, pattern := range patterns {
		testFile := filepath.Join(dir, pattern)
		if _, err := os.Stat(testFile); err == nil {
			testFiles = append(testFiles, testFile)
		}
	}

	return testFiles
}
