// Design: docs/guide/radius.md -- the unsigned EAP answer discard and its counter
// Related: metrics.go -- registerRadiusMetrics, bindRadiusMetrics: the hook this init hands over

package radius

// init hands the RADIUS client counters to the plugin registry, which binds
// them when the metrics registry exists. It sits apart from register.go, whose
// init registers the AAA backend and the doctor check, because the metrics hook
// cannot fail and needs none of that file's exit path.
func init() {
	registerRadiusMetrics()
}
