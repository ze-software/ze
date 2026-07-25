// Design: docs/architecture/config/syntax.md -- config schema evolution
// Related: migrate.go -- ExaBGP-era structural migration (content-detection, runs first)
// Related: ../stamp.go -- schema stamp parsing and formatting

package migration

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/version"
)

var (
	errInvalidRelease  = errors.New("invalid evolution release format (want YY.MM.DD)")
	errDuplicateEvolve = errors.New("duplicate evolution registration")
)

// Evolution defines a single Ze-to-Ze schema change.
// Each evolution is tagged with the Ze release (YY.MM.DD) that introduced the change.
// On startup, evolutions with Release > the stamp's release are applied in order.
type Evolution struct {
	Release     string
	Name        string
	Description string
	Detect      func(*config.Tree) bool
	Apply       func(*config.Tree) (*config.Tree, error)
}

// EvolveResult holds the outcome of schema evolution.
type EvolveResult struct {
	Tree    *config.Tree
	Applied []string
}

var (
	evolutionMu sync.RWMutex
	evolutions  []Evolution
)

// ResetForTest clears all registered evolutions. Test-only.
func ResetForTest() {
	evolutionMu.Lock()
	evolutions = nil
	evolutionMu.Unlock()
}

// RegisterEvolution adds a schema evolution step.
func RegisterEvolution(e Evolution) error {
	if !version.IsValidRelease(e.Release) {
		return fmt.Errorf("%w: %s", errInvalidRelease, e.Release)
	}
	evolutionMu.Lock()
	defer evolutionMu.Unlock()
	for _, existing := range evolutions {
		if existing.Release == e.Release && existing.Name == e.Name {
			return fmt.Errorf("%w: %s %s", errDuplicateEvolve, e.Release, e.Name)
		}
	}
	evolutions = append(evolutions, e)
	return nil
}

// Evolve applies all registered evolutions newer than stampRelease.
// Returns nil result when no evolutions are needed.
func Evolve(tree *config.Tree, stampRelease string) (*EvolveResult, error) {
	if tree == nil {
		return nil, ErrNilTree
	}

	applicable := applicableEvolutions(stampRelease)
	if len(applicable) == 0 {
		return nil, nil //nolint:nilnil // nil means "nothing to do"
	}

	working := tree.Clone()
	result := &EvolveResult{}

	for _, e := range applicable {
		if e.Detect(working) {
			evolved, err := e.Apply(working)
			if err != nil {
				return nil, fmt.Errorf("evolution %s (%s): %w", e.Name, e.Release, err)
			}
			working = evolved
			result.Applied = append(result.Applied, e.Name)
		}
	}

	if len(result.Applied) == 0 {
		return nil, nil //nolint:nilnil // all detected as already done
	}

	result.Tree = working
	return result, nil
}

// applicableEvolutions returns evolutions newer than stampRelease, sorted.
// When stampRelease is empty or unparseable (e.g. "dev"), all evolutions apply.
func applicableEvolutions(stampRelease string) []Evolution {
	evolutionMu.RLock()
	defer evolutionMu.RUnlock()

	stampValid := version.IsValidRelease(stampRelease)

	var applicable []Evolution
	for _, e := range evolutions {
		if !stampValid || version.IsNewerRelease(e.Release, stampRelease) {
			applicable = append(applicable, e)
		}
	}

	sortEvolutions(applicable)
	return applicable
}

func sortEvolutions(evos []Evolution) {
	sort.Slice(evos, func(i, j int) bool {
		cmp := version.CompareReleases(evos[i].Release, evos[j].Release)
		if cmp != 0 {
			return cmp < 0
		}
		return evos[i].Name < evos[j].Name
	})
}
