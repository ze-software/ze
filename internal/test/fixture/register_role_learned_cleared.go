// Design: docs/architecture/meta/role.md -- Learned role lifecycle

package fixture

// The driver of test/plugin/role-learned-cleared-on-peer-delete.ci. Its body
// is role_learned_cleared_fixture.go.
func init() {
	Register("plugin/role-learned-cleared-on-peer-delete", roleLearnedClearedOnPeerDelete)
}
