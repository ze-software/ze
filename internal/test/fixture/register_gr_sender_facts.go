// Design: docs/architecture/testing/ci-format.md — the compiled observer API
// Overview: plugin_fixture_gr_sender_facts.go — the three scenarios registered here
//
// The registration half of the graceful-restart sender-fact observers. It is a
// file of its own because the registration is the whole of its content, which is
// what `register` files in this repository are for.

package fixture

func init() {
	Register("plugin/gr-peer-restart-time-drives-timer", grSenderFactsDriver(grPeerRestartTimeDrivesTimer))
	Register("plugin/gr-peer-families-drive-mark-stale", grSenderFactsDriver(grPeerFamiliesDriveMarkStale))
	Register("plugin/llgr-peer-stale-time-drives-timer", grSenderFactsDriver(llgrPeerStaleTimeDrivesTimer))
}
