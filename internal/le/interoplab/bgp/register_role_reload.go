// Design: docs/architecture/testing/interop.md -- capability replacement against a foreign peer.
// Related: check_role_reload.go -- the same reload assertions over both plugin transports.
package bgp

const (
	roleReloadScenario       = "bgp-role-reload-frr"
	roleReloadForkedScenario = "bgp-role-reload-frr-forked"
)

func init() {
	specialCheckers[roleReloadScenario] = checkRoleReload
	specialCheckers[roleReloadForkedScenario] = checkRoleReload
}
