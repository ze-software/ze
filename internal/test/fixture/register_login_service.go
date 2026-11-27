// Design: docs/architecture/aaa-tacacs.md -- register login-service functional test fixtures.
// Related: login_service_doors.go -- per-surface login-service fixtures.
// Related: login_service_modes.go -- remote backend, reload, recovery, and mode fixtures.

package fixture

func init() {
	Register("plugin/login-service-ssh-password", lsFixtureSSHPassword)
	Register("plugin/login-service-ssh-publickey", lsFixtureSSHPublicKey)
	Register("plugin/login-service-ze-cli", lsFixtureZeCLI)
	Register("plugin/login-service-web-form", lsFixtureWebForm)
	Register("plugin/login-service-web-basic", lsFixtureWebBasic)
	Register("plugin/login-service-web-cookie-reload", lsFixtureWebCookieReload)
	Register("plugin/login-service-rest", lsFixtureREST)
	Register("plugin/login-service-grpc", lsFixtureGRPC)
	Register("plugin/login-service-mcp", lsFixtureMCP)

	Register("plugin/login-service-tacacs", lsFixtureTacacs)
	Register("plugin/login-service-radius", lsFixtureRadius)
	Register("plugin/login-service-reload-tightens", lsFixtureReloadTightens)
	Register("plugin/login-service-recovery-user", lsFixtureRecoveryUser)
	Register("plugin/login-service-shadowed-recovery-name", lsFixtureShadowedRecoveryName)
	Register("plugin/login-service-absent-section", lsFixtureAbsentSection)
	Register("plugin/login-service-empty-set", lsFixtureEmptySet)
	Register("plugin/login-service-chain-terminal", lsFixtureChainTerminal)
	Register("plugin/login-service-shared-token", lsFixtureSharedToken)
}
