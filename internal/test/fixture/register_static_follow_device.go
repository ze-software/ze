// Design: docs/architecture/static-routes.md -- device presence as an activity input
// Related: static_follow_device_fixture.go -- the scenarios registered below

package fixture

func init() {
	// The follow-device driver reuses the kernel scenarios' setup link
	// (static/static-kernel-setup) for the gateway member.
	Register("static/static-interface-nexthop-pppoe-follows-device",
		observer02("static-follow-device-test", staticFollowDevice))

	// The two commit-time drivers read `show static`. They reuse the same
	// setup link as the pppoe-client's source interface, where no access
	// concentrator answers.
	Register("static/static-interface-nexthop-pppoe-absent",
		observer02("static-pppoe-test", staticPPPoEAbsent))
	Register("static/static-interface-nexthop-pppoe-same-commit",
		observer02("static-pppoe-test", staticPPPoESameCommit))
}
