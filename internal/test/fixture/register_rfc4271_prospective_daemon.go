// Design: docs/architecture/route-selection.md -- prospective next-hop admission
// Related: rfc4271_prospective_daemon_fixture.go -- new internal/forked drivers
package fixture

func init() {
	Register("plugin/rfc4271-prospective-next-hop-kernel", rfc4271ProspectiveKernelDriver, StopsDaemon)
	Register("plugin/rfc4271-prospective-next-hop-kernel-empty", rfc4271ProspectiveKernelEmpty)
	Register("plugin/rfc4271-prospective-next-hop-link-cleanup", rfc4271ProspectiveLinkCleanup)
}
