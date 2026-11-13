//go:build linux

// register_ldp_neighbor_linux.go names the fixtures of the ldp suite's live
// neighbor: the launcher that builds the two-namespace veth link and runs the
// daemon on one side of it, and the two scenarios whose observer plugin plays
// the LDP neighbor on the other side.
//
// Related: ldp_neighbor_linux.go -- the link, the neighbor and its RFC 5036 codec
// Related: ldp_session_end_linux.go -- the two scenarios
// Related: test/ldp/ldp-session-end-clears-backoff.ci -- an operational session ends, Ze redials at once
// Related: test/ldp/ldp-setup-failure-publishes-no-session-down.ci -- a rejected setup is delayed and silent

package fixture

func init() {
	Register("ldp/neighbor-link", ldpNeighborLinkDriver)
	Register("ldp/session-end-clears-backoff", ldpSessionEndClearsBackoffDriver, StopsDaemon)
	Register("ldp/setup-failure-publishes-no-session-down", ldpSetupFailurePublishesNoSessionDownDriver, StopsDaemon)
}
