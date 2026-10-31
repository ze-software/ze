package execution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sivchari/gomu/internal/mutation"
)

// OverlayMutator manages overlay-based mutation without modifying original files.
type OverlayMutator struct {
	baseDir string
}

// OverlayConfig represents the JSON structure for go build/test -overlay option.
// Note: "Replace" must be capitalized as per Go's overlay specification.
type OverlayConfig struct {
	Replace map[string]string `json:"Replace"` //nolint:tagliatelle // Go overlay spec requires "Replace"
}

// MutationContext holds the context for a single mutation execution.
type MutationContext struct {
	OriginalPath string // Absolute path to the original file
	MutatedPath  string // Path to the mutated file in temp directory
	OverlayPath  string // Path to the overlay.json file
	MutantDir    string // Directory containing this mutant's files
}

// NewOverlayMutator creates a new overlay-based mutator.
func NewOverlayMutator() (*OverlayMutator, error) {
	baseDir := filepath.Join(os.TempDir(), fmt.Sprintf("gomu_overlay_%d_%d", os.Getpid(), time.Now().UnixNano()))
	if err := os.MkdirAll(baseDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create overlay base directory: %w", err)
	}

	return &OverlayMutator{
		baseDir: baseDir,
	}, nil
}

// PrepareMutation prepares the mutation execution by creating mutated file and overlay.json.
func (om *OverlayMutator) PrepareMutation(mutant mutation.Mutant) (*MutationContext, error) {
	// Create unique directory for this mutant
	mutantDir := filepath.Join(om.baseDir, fmt.Sprintf("mutant_%s", mutant.ID))
	if err := os.MkdirAll(mutantDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create mutant directory: %w", err)
	}

	// Get absolute path of original file
	originalPath, err := filepath.Abs(mutant.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Create mutated file
	mutatedPath := filepath.Join(mutantDir, filepath.Base(mutant.FilePath))

	if err := om.createMutatedFile(mutant, originalPath, mutatedPath); err != nil {
		// Cleanup on failure
		os.RemoveAll(mutantDir)

		return nil, fmt.Errorf("failed to create mutated file: %w", err)
	}

	// Generate overlay.json
	overlayPath := filepath.Join(mutantDir, "overlay.json")

	if err := om.generateOverlayJSON(originalPath, mutatedPath, overlayPath); err != nil {
		os.RemoveAll(mutantDir)

		return nil, fmt.Errorf("failed to generate overlay.json: %w", err)
	}

	return &MutationContext{
		OriginalPath: originalPath,
		MutatedPath:  mutatedPath,
		OverlayPath:  overlayPath,
		MutantDir:    mutantDir,
	}, nil
}

// CleanupMutation removes temporary files for a single mutation.
func (om *OverlayMutator) CleanupMutation(ctx *MutationContext) error {
	if ctx == nil || ctx.MutantDir == "" {
		return nil
	}

	if err := os.RemoveAll(ctx.MutantDir); err != nil {
		return fmt.Errorf("failed to cleanup mutant directory: %w", err)
	}

	return nil
}

// Cleanup removes all temporary files.
func (om *OverlayMutator) Cleanup() error {
	if err := os.RemoveAll(om.baseDir); err != nil {
		return fmt.Errorf("failed to cleanup overlay directory: %w", err)
	}

	return nil
}

// createMutatedFile creates a mutated version of the source file.
func (om *OverlayMutator) createMutatedFile(mutant mutation.Mutant, originalPath, mutatedPath string) error {
	src, err := os.ReadFile(originalPath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	mutated, err := mutation.ApplyMutantToSource(src, originalPath, mutant)
	if err != nil {
		return fmt.Errorf("failed to apply mutant: %w", err)
	}

	if err := os.WriteFile(mutatedPath, mutated, 0600); err != nil {
		return fmt.Errorf("failed to write mutated file: %w", err)
	}

	return nil
}

// generateOverlayJSON creates the overlay.json file for go build/test.
func (om *OverlayMutator) generateOverlayJSON(originalPath, mutatedPath, overlayPath string) error {
	config := OverlayConfig{
		Replace: map[string]string{
			originalPath: mutatedPath,
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal overlay config: %w", err)
	}

	if err := os.WriteFile(overlayPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write overlay.json: %w", err)
	}

	return nil
}
