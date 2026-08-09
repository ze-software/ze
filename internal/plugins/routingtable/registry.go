// Design: docs/architecture/static-routes.md -- routing-table registry

package routingtable

import rt "github.com/ze-software/ze/internal/core/routingtable"

// Registry, New, ValidateTableID, GetRegistry, SetRegistry are re-exported
// from internal/core/routingtable so consumers that already import this
// plugin package continue to compile.
type Registry = rt.Registry

var (
	New             = rt.New
	ValidateTableID = rt.ValidateTableID
	GetRegistry     = rt.GetRegistry
	SetRegistry     = rt.SetRegistry
)
