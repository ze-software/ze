// Design: docs/architecture/site-facts.md -- repository facts share the activity inventory.
package reporewrite

// MeasureGoInventory reads the activity dashboard's default Go inventory
// without scanning commit history. Source and tests exclude vendor dependencies
// and include comments and blank lines. It writes nothing.
func MeasureGoInventory(root string) (ActivityGo, error) {
	return collectGoStats(defaultActivityOptions(root))
}
