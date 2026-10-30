// Design: docs/architecture/testing/interop.md -- the IS-IS narrow-metric scenario.
package bgp

func init() {
	specialCheckers["isis-narrow-metric-frr"] = checkISISNarrowMetric
}
