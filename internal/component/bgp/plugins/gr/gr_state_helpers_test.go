package gr

import (
	"github.com/ze-software/ze/internal/core/family"
	"slices"
)

// familyRetained asks the same exact original physical-source view as runtime.
// The caller MUST NOT hold the state manager lock.
func (m *grStateManager) familyRetained(source grSource, fam family.Family) bool {
	return slices.Contains(m.retainedFamilies(source), fam)
}
