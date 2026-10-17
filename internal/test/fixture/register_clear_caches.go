// Design: docs/architecture/resolve.md -- Clearing a resolver cache
// Related: clear_caches_fixture.go -- the scenarios registered here

package fixture

func init() {
	Register("plugin/clear-peeringdb-cache", observe04(clearPeeringDBCache04))
	Register("plugin/clear-irr-cache", observe04(clearIRRCache04))
	Register("plugin/clear-cymru-cache", observe04(clearCymruCache04))
}
