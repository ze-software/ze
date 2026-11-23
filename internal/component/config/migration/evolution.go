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
	// StampedOnly limits the evolution to a file whose stamp names a dated
	// release older than Release. An evolution that changes a shape is safe on
	// any file, because Detect sees the old shape. An evolution that writes an
	// old default into a file is not: an unset leaf looks the same under both
	// defaults, so only the stamp says which one the file was written under. An
	// unstamped file (written by hand, piped on stdin) or one a dev build
	// stamped proves neither, and gets the current default.
	StampedOnly bool
	// Notice is the operator's account of an evolution that changes how the
	// router behaves: what it wrote, why, and the command that undoes it. The
	// hub logs it at Warn, which the default log level shows, because the
	// operator did not ask for the change and must hear of it. An evolution
	// that changes only a shape (a renamed leaf, a moved block) keeps the
	// behavior, leaves Notice empty, and is logged at Info: a Warn at every
	// upgrade for a rename would teach the operator to ignore the one that
	// matters.
	Notice string
}

// EvolveResult holds the outcome of schema evolution.
type EvolveResult struct {
	Tree    *config.Tree
	Applied []string
	// Notices holds, in application order, the Notice of each applied
	// evolution that carries one.
	Notices []EvolveNotice
}

// EvolveNotice is the Notice of one applied evolution, named.
type EvolveNotice struct {
	Evolution string
	Text      string
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
		if !e.Detect(working) {
			continue
		}
		evolved, err := e.Apply(working)
		if err != nil {
			return nil, fmt.Errorf("evolution %s (%s): %w", e.Name, e.Release, err)
		}
		working = evolved
		result.Applied = append(result.Applied, e.Name)
		if e.Notice != "" {
			result.Notices = append(result.Notices, EvolveNotice{Evolution: e.Name, Text: e.Notice})
		}
	}

	if len(result.Applied) == 0 {
		return nil, nil //nolint:nilnil // all detected as already done
	}

	result.Tree = working
	return result, nil
}

// applicableEvolutions returns evolutions newer than stampRelease, sorted.
// When stampRelease is empty or unparseable (e.g. "dev"), every evolution applies
// except a StampedOnly one, which needs a dated stamp to know the file is old.
func applicableEvolutions(stampRelease string) []Evolution {
	evolutionMu.RLock()
	defer evolutionMu.RUnlock()

	stampValid := version.IsValidRelease(stampRelease)

	var applicable []Evolution
	for _, e := range evolutions {
		if stampValid {
			if version.IsNewerRelease(e.Release, stampRelease) {
				applicable = append(applicable, e)
			}
			continue
		}
		if e.StampedOnly {
			continue
		}
		applicable = append(applicable, e)
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
