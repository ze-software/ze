// Design: docs/architecture/meta/role.md -- Learned role lifecycle
// Related: role.go -- recordLearnedRole, the one writer these helpers call

package role

// setFilterRemoteRole stores a learned role the way an OPEN that carried a
// Role capability does, stamped with no generation: the tests that use it set
// up filter state and judge no removal ordering.
func setFilterRemoteRole(peerID, group, remoteRole string) {
	recordLearnedRole(peerID, group, remoteRole, 0)
}

// recordNoRemoteRole stores what an OPEN without a usable Role capability
// records, and returns the role it replaced.
func recordNoRemoteRole(peerID, group string) string {
	return recordLearnedRole(peerID, group, "", 0)
}
