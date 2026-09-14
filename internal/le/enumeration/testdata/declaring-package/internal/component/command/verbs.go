// The CLI verb vocabulary's own package, holding the map that BUILDS it and the
// constants that spell its keys. Both are the declaration.
package command

const (
	VerbShow    = "show"
	VerbMonitor = "monitor"
	VerbResolve = "resolve"
)

var Verbs = map[string]verbRole{
	VerbShow:    RoleRead,
	VerbMonitor: RoleRead,
	VerbResolve: RoleRead,
}
