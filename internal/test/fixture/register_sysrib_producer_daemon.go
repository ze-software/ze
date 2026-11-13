// Design: docs/architecture/forked-route-install.md -- real producer cleanup.
// Related: sysrib_producer_daemon_fixture.go -- independent live SDK producer.

package fixture

func init() {
	Register("plugin/sysrib-forked-producer-kernel", sysribProducerDriver, StopsDaemon)
	Register(sysribProducerFixture, sysribProducerSDKDriver)
	Register("plugin/sysrib-producer-kernel-empty", sysribProducerKernelEmpty)
}
