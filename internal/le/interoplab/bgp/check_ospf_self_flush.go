// Design: docs/architecture/testing/interop.md -- typed scenario registry.
// Related: checkers.go -- registers ospfSelfLSAFlushOperations for ospf-self-lsa-flush-frr.
// Related: check_engine.go -- opStop, the container stop this sequence restarts Ze with.
package bgp

import "time"

// ospfSelfLSAFlushOperations is the interop proof of D-8 (commit 19de202493):
// a self-originated LSA that Ze receives newer than its own database copy, while
// that copy is already at MaxAge because Ze withdrew it, is flushed and not
// brought back to life.
//
// RFC 2328 Section 13.4: "It may be the case the router no longer wishes to
// originate the received LSA. Possible examples include: 1) the LSA is a
// summary-LSA or AS-external-LSA and the router no longer has an (advertisable)
// route to the destination, [...] In all these cases, instead of updating the
// LSA, the LSA should be flushed from the routing domain by incrementing the
// received LSA's LS age to MaxAge and reflooding (see Section 14.1)."
//
// The condition needs a neighbor that holds a NEWER instance than the MaxAge
// copy Ze withdrew, and only a restart makes Ze's sequence number go backwards:
//
//  1. Ze originates its default AS-external-LSA (metric 4711). Two reloads
//     (4242, then 4711 again) leave FRR holding it at 0x80000003 or later:
//     every body change takes a new sequence number.
//  2. FRR's eth0 goes down, so FRR keeps Ze's LSA but hears no flush from Ze.
//  3. Ze stops and starts. It holds no record of its old instance and
//     originates the default again at InitialSequenceNumber 0x80000001.
//  4. ze-reload-withdraw.conf drops default-information. flushSelfLSA ages
//     Ze's own copy to MaxAge at 0x80000002. With no neighbor, no
//     acknowledgement ever deletes it, so the MaxAge copy stays in Ze's LSDB.
//  5. FRR's eth0 comes up. In the database exchange FRR's 0x80000003 is newer
//     than Ze's 0x80000002, so Ze requests it and handleSelfReceived
//     (internal/plugins/ospf/lsdb/origination.go) receives it.
//
// At HEAD the database copy is at MaxAge, so handleSelfReceived calls
// flushReceivedSelfLSA: Ze installs the received instance at MaxAge with the
// received sequence and refloods it, FRR takes it as more recent, and the LSA
// and FRR's OSPF default route disappear. With the `if local.Age.IsMaxAge()`
// branch reverted, handleSelfReceived re-originates the old body (metric 4711)
// at age 0 with the sequence one past the received one: FRR installs that, the
// withdrawn default comes back to life, and nothing in Ze ever withdraws it
// again (defaultInfoOriginated is already false, and RefreshSelf keeps the
// entry alive). The final absences then time out. The no-local-record path
// (Ze's copy deleted before step 5) would flush under both builds, which is
// why step 4 asserts the MaxAge copy in Ze's own LSDB before FRR comes back.
//
// FRR decides with ospf_lsa_more_recent (ospfd/ospf_lsa.c, FRR 10.3.1), called
// from ospf_ls_upd step (5), "if (current == NULL || (ret =
// ospf_lsa_more_recent(current, lsa)) < 0)" then "ospf_flood(...)":
//
//	/* compare LS sequence number. */
//	x = (int)ntohl(l1->data->ls_seqnum);
//	y = (int)ntohl(l2->data->ls_seqnum);
//	if (x > y)
//		return 1;
//	...
//	/* compare LS checksum. */
//	r = ntohs(l1->data->checksum) - ntohs(l2->data->checksum);
//	...
//	/* compare LS age. */
//	if (IS_LSA_MAXAGE(l1) && !IS_LSA_MAXAGE(l2))
//		return 1;
//
// Ze's flush carries FRR's own sequence and body, so the checksum is equal and
// the MaxAge comparison makes it the more recent instance; FRR then removes it
// through ospf_lsa_maxage and ospf_maxage_lsa_remover. The reverted build's
// age-0 instance at a higher sequence wins the first comparison instead.
//
// FRR prints the "OSPF Router with ID" heading whether or not it holds an
// AS-external-LSA, which is the proof each database absence ran.
var ospfSelfLSAFlushOperations = []operation{
	// Step 1. FRR holds Ze's default at 0x80000003 or later.
	{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	{kind: opWaitContains, peer: peerFRR, command: ospfSelfFlushFRRDefaultLSA, contains: []string{ospfSelfFlushZeAdvertises, ospfSelfFlushMetricBoot}, timeout: 60 * time.Second},
	{kind: opExec, peer: "ze", command: []string{"sh", "-c", "cp " + zeMountedReloadConfig + " " + zeRunningConfig}},
	{kind: opSignal, peer: "ze", argument: signalHUP},
	{kind: opWaitContains, peer: peerFRR, command: ospfSelfFlushFRRDefaultLSA, contains: []string{ospfSelfFlushZeAdvertises, ospfSelfFlushMetricReload}, timeout: 60 * time.Second},
	{kind: opExec, peer: "ze", command: []string{"sh", "-c", "cp " + zeMountedConfig + " " + zeRunningConfig}},
	{kind: opSignal, peer: "ze", argument: signalHUP},
	{kind: opWaitContains, peer: peerFRR, command: ospfSelfFlushFRRDefaultLSA, contains: []string{ospfSelfFlushZeAdvertises, ospfSelfFlushMetricBoot}, timeout: 60 * time.Second},
	{kind: opWaitContains, peer: peerFRR, command: ospfSelfFlushFRRRoutes, contains: []string{ospfSelfFlushDefaultPrefix}, timeout: 60 * time.Second},
	// Step 2. FRR is cut off and keeps Ze's LSA.
	{kind: opExec, peer: peerFRR, command: []string{"ip", ipObjectLink, ipActionSet, containerInterface, linkDown}},
	{kind: opWaitAbsent, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, absent: []string{ospfStateFull}, proof: []string{ospfNeighborHeading}, timeout: 60 * time.Second},
	// Step 3. Ze restarts from ze.conf and originates the default at 0x80000001.
	{kind: opStop, peer: "ze", timeout: 30 * time.Second},
	{kind: opStart, peer: "ze"},
	{kind: opWaitContains, peer: "ze", command: zeCommand("show ospf database external"), contains: []string{ospfSelfFlushZeRouter, ospfSelfFlushZeInitialSequence}, timeout: 90 * time.Second},
	{kind: opRequireContains, peer: peerFRR, command: ospfSelfFlushFRRDefaultLSA, contains: []string{ospfSelfFlushZeAdvertises, ospfSelfFlushMetricBoot}},
	// Step 4. Ze withdraws the default; its own copy stays at MaxAge, 0x80000002.
	{kind: opExec, peer: "ze", command: []string{"sh", "-c", "cp " + zeMountedStepReloadConfig(ospfSelfFlushWithdrawConfig) + " " + zeRunningConfig}},
	{kind: opSignal, peer: "ze", argument: signalHUP},
	{kind: opWaitContains, peer: "ze", command: zeCommand("show ospf database external"), contains: []string{ospfSelfFlushZeRouter, ospfSelfFlushZeWithdrawnSequence, ospfSelfFlushMaxAge}, timeout: 30 * time.Second},
	// Step 5. FRR returns and hands Ze its newer instance in the exchange.
	{kind: opExec, peer: peerFRR, command: []string{"ip", ipObjectLink, ipActionSet, containerInterface, "up"}},
	{kind: opWaitContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}, timeout: 90 * time.Second},
	// The verdict. FRR's maxage remover runs on a timer, so the LSDB absence gets
	// two minutes; the reverted build's live instance never leaves.
	{kind: opWaitAbsent, peer: peerFRR, command: ospfSelfFlushFRRDefaultLSA, absent: []string{ospfSelfFlushZeAdvertises}, proof: []string{ospfSelfFlushFRRHeading}, timeout: 120 * time.Second},
	{kind: opWaitAbsent, peer: peerFRR, command: ospfSelfFlushFRRRoutes, absent: []string{ospfSelfFlushDefaultPrefix}, proof: []string{ospfSelfFlushFRRRouteCodes}, timeout: 30 * time.Second},
	// A refresh or a late re-origination would bring it back; it stays gone.
	{kind: opExec, peer: peerFRR, command: []string{"sleep", "15"}},
	{kind: opRequireAbsent, peer: peerFRR, command: ospfSelfFlushFRRDefaultLSA, absent: []string{ospfSelfFlushZeAdvertises}, proof: []string{ospfSelfFlushFRRHeading}},
	{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, contains: []string{ospfStateFull}},
}

const (
	// ospfSelfFlushWithdrawConfig is the scenario's reload that drops default-information.
	ospfSelfFlushWithdrawConfig = "ze-reload-withdraw.conf"
	// ospfSelfFlushZeAdvertises ties an FRR LSDB entry to Ze, the only router
	// in the lab that originates a default.
	ospfSelfFlushZeAdvertises = "Advertising Router: " + zeLabAddress
	ospfSelfFlushFRRHeading   = "OSPF Router with ID"
	// ospfSelfFlushMetricBoot is the metric of ze.conf, the config Ze restarts
	// from and the body the reverted build brings back; ospfSelfFlushMetricReload
	// is ze-reload.conf's, which forces the extra sequence number.
	ospfSelfFlushMetricBoot   = "Metric: 4711"
	ospfSelfFlushMetricReload = "Metric: 4242"
	// ospfSelfFlushFRRRouteCodes is the legend FRR prints above its routes; FRR's
	// own OSPF route to the lab subnet keeps the table non-empty.
	ospfSelfFlushFRRRouteCodes = "Codes:"
	ospfSelfFlushDefaultPrefix = "0.0.0.0/0"
	ospfSelfFlushZeRouter      = `"advertising-router": "` + zeLabAddress + `"`
	// Ze prints an LS sequence number as a signed 32-bit integer:
	// 0x80000001 is -2147483647 and 0x80000002 is -2147483646.
	ospfSelfFlushZeInitialSequence   = `"sequence": "-2147483647"`
	ospfSelfFlushZeWithdrawnSequence = `"sequence": "-2147483646"`
	ospfSelfFlushMaxAge              = `"age": 3600`
)

var (
	ospfSelfFlushFRRDefaultLSA = []string{cmdVtysh, "-c", "show ip ospf database external 0.0.0.0"}
	ospfSelfFlushFRRRoutes     = []string{cmdVtysh, "-c", "show ip route ospf"}
)
