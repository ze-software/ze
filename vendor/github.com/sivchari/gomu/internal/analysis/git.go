package analysis

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitIntegration provides Git integration functionality.
type GitIntegration struct {
	workDir      string
	ignoreParser IgnoreParser
}

// NewGitIntegration creates a new Git integration.
func NewGitIntegration(workDir string) *GitIntegration {
	return &GitIntegration{
		workDir: workDir,
	}
}

// SetIgnoreParser sets the ignore parser for the Git integration.
func (g *GitIntegration) SetIgnoreParser(parser IgnoreParser) {
	g.ignoreParser = parser
}

// IsGitRepository checks if the current directory is a Git repository.
func (g *GitIntegration) IsGitRepository() bool {
	gitDir := filepath.Join(g.workDir, ".git")
	_, err := os.Stat(gitDir)

	return err == nil
}

// GetChangedFiles returns the list of changed files compared to the base branch.
func (g *GitIntegration) GetChangedFiles(baseBranch string) ([]string, error) {
	if !g.IsGitRepository() {
		return nil, fmt.Errorf("not a git repository")
	}

	// Get the merge base with the base branch
	ctx := context.Background()
	mergeBaseCmd := exec.CommandContext(ctx, "git", "merge-base", "HEAD", baseBranch)
	mergeBaseCmd.Dir = g.workDir

	mergeBaseOutput, err := mergeBaseCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get merge base: %w", err)
	}

	mergeBase := strings.TrimSpace(string(mergeBaseOutput))

	// Get changed files since merge base
	diffCmd := exec.CommandContext(ctx, "git", "diff", "--name-only", mergeBase, "HEAD")
	diffCmd.Dir = g.workDir

	output, err := diffCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(files) == 1 && files[0] == "" {
		return []string{}, nil
	}

	// Filter for Go files and apply ignore patterns
	var goFiles []string

	for _, file := range files {
		if IsGoSourceFile(file) {
			// Skip standard excluded directories (vendor, testdata)
			if IsExcludedPath(file) {
				continue
			}

			// Check if file should be ignored by .gomuignore
			if g.ignoreParser != nil && g.ignoreParser.ShouldIgnore(file) {
				continue
			}

			// Convert to absolute path
			absPath := filepath.Join(g.workDir, file)
			goFiles = append(goFiles, absPath)
		}
	}

	return goFiles, nil
}

// GetCurrentBranch returns the current Git branch name.
func (g *GitIntegration) GetCurrentBranch() (string, error) {
	if !g.IsGitRepository() {
		return "", fmt.Errorf("not a git repository")
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = g.workDir

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetAllGoFiles returns all Go files in the repository.
func (g *GitIntegration) GetAllGoFiles() ([]string, error) {
	var goFiles []string

	err := filepath.Walk(g.workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and standard excluded directories (vendor, testdata)
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") || IsExcludedPath(GetRelativePath(g.workDir, path)) {
				return filepath.SkipDir
			}

			// Check if directory should be ignored by .gomuignore
			if g.ignoreParser != nil {
				relPath := GetRelativePath(g.workDir, path)
				if g.ignoreParser.ShouldIgnore(relPath) {
					return filepath.SkipDir
				}
			}

			return nil
		}

		// Only include Go source files (not test files)
		if IsGoSourceFile(path) {
			// Check if file should be ignored by .gomuignore
			if g.ignoreParser != nil {
				relPath := GetRelativePath(g.workDir, path)
				if g.ignoreParser.ShouldIgnore(relPath) {
					return nil
				}
			}

			goFiles = append(goFiles, path)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return goFiles, nil
}

// HasUncommittedChanges checks if there are uncommitted changes.
func (g *GitIntegration) HasUncommittedChanges() (bool, error) {
	if !g.IsGitRepository() {
		return false, fmt.Errorf("not a git repository")
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = g.workDir

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}

	return strings.TrimSpace(string(output)) != "", nil
}
