// Design: docs/architecture/testing/interop.md -- the announce rail's build key, judged by two daemons.
// Related: test/interop/scenarios/local-as-replace-as-partition/ze.conf -- the two peers sharing one Local AS.
package bgp

import "time"

// scenarioLocalASPartition is RFC 7705 Section 3.3 on the announce rail, where
// peers are grouped on one key and each group is built once. FRR and BIRD both
// peer with ze's Local AS localASPartitionLocalAS, and only BIRD's session
// carries replace-as. Each daemon must install the AS_PATH its own session
// calls for: FRR the Local AS in front of the global AS, BIRD the Local AS
// alone. A key that merged the two sessions sends one daemon the other's
// AS_PATH, so exactly one of the two path checks fails, whichever built first.
//
// The announce runs only after both sessions are Established, because a route
// that exists before a session comes up reaches that peer through its own
// initial sync and never meets the group key: originating it from config at
// startup let a key with the prepend zeroed pass this scenario.
const (
	scenarioLocalASPartition = "local-as-replace-as-partition"

	localASPartitionPrefix  = "10.77.6.0/24"
	localASPartitionLocalAS = "65020"
	// localASPartitionFRRPath is the AS path string FRR's JSON prints for a
	// session with no option: the Local AS outermost, then the global AS.
	localASPartitionFRRPath = localASPartitionLocalAS + " 65001"
	// localASPartitionBIRDPath ends at the newline, so the two-AS path FRR is
	// owed does not satisfy it.
	localASPartitionBIRDPath    = "BGP.as_path: " + localASPartitionLocalAS + "\n"
	localASPartitionBIRDShowAll = "show route for " + localASPartitionPrefix + " all"
	// localASPartitionAnnounce reaches AnnounceNLRIBatch once for both peers.
	// The next hop is self, which is ze's one lab address toward either peer.
	localASPartitionAnnounce = "send bgp * unicast " + localASPartitionPrefix
)

func init() {
	scenarioOperations[scenarioLocalASPartition] = []operation{
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: birdZeProtocol},
		{kind: opExec, peer: "ze", command: zeCommand(localASPartitionAnnounce)},
		{kind: opFRRRoute, argument: localASPartitionPrefix, timeout: 60 * time.Second},
		{kind: opBIRDRoute, argument: localASPartitionPrefix, timeout: 60 * time.Second},
		{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", "show bgp ipv4 unicast " + localASPartitionPrefix + " json"}, contains: []string{localASPartitionFRRPath}},
		{kind: opRequireContains, peer: peerBIRD, command: []string{cmdBirdc, localASPartitionBIRDShowAll}, contains: []string{localASPartitionBIRDPath}},
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: birdZeProtocol},
	}
}
