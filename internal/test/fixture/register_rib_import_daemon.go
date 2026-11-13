// Design: docs/guide/route-policy.md -- import gates retain candidates across placement and reload.
// Related: rib_import_daemon_fixture.go -- real daemon, FIB and producer-placement proof.

package fixture

func init() {
	Register("plugin/connected-import-filter-hides", importConnectedDriver, StopsDaemon)
	Register("plugin/rib-import-tag-placement", importTagDriver, StopsDaemon)
	Register("plugin/import-policy-kernel-empty", importDaemonKernelEmpty)
	Register("plugin/import-policy-gateway-link-cleanup", importDaemonGatewayLinkCleanup)
	Register("plugin/import-policy-connected-link-cleanup", importConnectedLinkCleanup)
	Register("plugin/import-policy-rollback", importRollbackParticipant)
}
