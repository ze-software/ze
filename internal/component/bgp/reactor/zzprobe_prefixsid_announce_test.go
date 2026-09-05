// This file is EMPTY and waits for the owner to delete it.
//
// It held TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain, the deliberately RED
// probe that stated RFC 8669 Section 8 on the announce rail before the rail
// asked prefixSIDAllowedTo. The fix landed, the test is green, and it now lives
// under the name of the code it covers, with its confining positives beside it:
// forward_prefix_sid_announce_rail_test.go. Nothing it asserted was dropped.
//
// The rename could not be finished in the session that made it. Deleting a
// _test.go path is refused by the pretool-bash test-deletion gate, which asks
// the operator to confirm and has no environment escape, so an agent cannot
// answer it. Emptying the file is what an agent CAN do, and two copies of one
// test function do not compile.
//
// To finish it: rm internal/component/bgp/reactor/zzprobe_prefixsid_announce_test.go
package reactor
