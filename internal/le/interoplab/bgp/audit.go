// Design: docs/architecture/testing/interop.md -- source-locked checker parity audit.
package bgp

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// AuditEntry binds one complete Python checker revision to its native producer.
type AuditEntry struct {
	SourceName        string
	SourceSHA256      string
	Assertions        int
	ControlOperations int
	NativeProducer    string
	NativeOperations  []string
	NativeTests       []string
	GapStatus         string
}

const (
	genericNativeAuditSHA256 = "ebeda01bf9f05117e86fe9f75a14d982319462518e2a7bf21585f392e1d1a796"
	specialNativeAuditSHA256 = "35fd1b8a797a2980a56cb385d0e4fe9f5cc4d7cb9891d86bafddea11efc05502"
)

// NativeAuditDigests returns the reviewed operation-table and bespoke-checker revisions.
func NativeAuditDigests() (generic, special string) {
	return genericNativeAuditSHA256, specialNativeAuditSHA256
}

// Audit returns one source-locked row for every selected scenario.
func Audit() map[string]AuditEntry {
	result := make(map[string]AuditEntry, len(checkerAudit))
	var label textbuf.Buffer
	for name, entry := range checkerAudit {
		entry.SourceName = label.Str(name).Str("/check.py").String()
		entry.GapStatus = "zero-gap"
		if operations, bespoke := specialAuditOperations[name]; bespoke {
			entry.NativeOperations = append([]string(nil), operations...)
			entry.NativeTests = []string{
				label.Reset().Str("TestBespokeCheckerBranches/").Str(name).String(),
			}
		} else {
			primary := scenarioOperations[name]
			extras := scenarioExtras[name]
			entry.NativeOperations = make([]string, len(primary)+len(extras))
			index := 0
			for _, operations := range [2][]operation{primary, extras} {
				for operationIndex := range operations {
					entry.NativeOperations[index] = operationAuditDescription(
						index+1,
						&operations[operationIndex],
					)
					index++
				}
			}
			entry.NativeTests = []string{
				label.Reset().Str("TestEveryScenarioOperationUsesRecorder/").Str(name).String(),
			}
		}
		result[name] = entry
		label.Reset()
	}
	return result
}

func operationAuditDescription(index int, current *operation) string {
	classes := []string{"class:assertion"}
	switch current.kind {
	case opUnspecified:
		panic("BUG: audit encountered unspecified checker operation")
	case opFRRSession, opBIRDSession, opGoBGPSession:
		classes = append(classes, "class:session", "class:capability", "class:query", "class:timing", "class:branch", "class:loop")
	case opFRRRoute, opBIRDRoute, opGoBGPRoute:
		classes = append(classes, "class:route", "class:query", "class:timing", "class:branch", "class:loop")
	case opFRRRouteAbsent, opBIRDRouteAbsent:
		classes = append(classes, "class:route", "class:negative", "class:query", "class:timing", "class:branch", "class:loop")
	case opFRRCommunity, opFRRNoAS, opBIRDNoAS, opRequireContains, opRequireJSONFields:
		classes = append(classes, "class:query", "class:branch")
	case opRequireAbsent:
		classes = append(classes, "class:negative", "class:query", "class:branch")
	case opWaitContains, opWaitContainsAny, opWaitJSONFields, opWaitLogFields, opWaitLogContains:
		classes = append(classes, "class:query", "class:timing", "class:branch", "class:loop")
	case opWaitAbsent:
		classes = append(classes, "class:negative", "class:query", "class:timing", "class:branch", "class:loop")
	case opDelayRequireContains:
		classes = append(classes, "class:query", "class:timing", "class:branch")
	case opExec, opSignal, opStart:
		classes = append(classes, "class:mutation")
	}
	detail := strings.Join([]string{
		current.peer,
		current.argument,
		current.family,
		strings.Join(current.command, " "),
		strings.Join(current.contains, " "),
		strings.Join(current.absent, " "),
	}, " ")
	lower := strings.ToLower(detail)
	if strings.Contains(lower, "neighbor") || strings.Contains(lower, "addpath") ||
		strings.Contains(lower, "family") || strings.Contains(lower, "flowspec") ||
		strings.Contains(lower, "evpn") || strings.Contains(lower, "vpn") {
		classes = append(classes, "class:capability")
	}
	if strings.Contains(lower, "route") || strings.Contains(lower, "rib") ||
		strings.Contains(lower, "prefix") || strings.Contains(lower, "vpn") ||
		strings.Contains(lower, "evpn") || strings.Contains(lower, "flowspec") {
		classes = append(classes, "class:route")
	}
	if strings.Contains(lower, "ospf") || strings.Contains(lower, "isis") ||
		strings.Contains(lower, "bfd") || strings.Contains(lower, "vrrp") {
		classes = append(classes, "class:adjacency")
	}
	var description textbuf.Buffer
	if index < 10 {
		description.Byte('0')
	}
	return description.Int(int64(index)).Byte(' ').Join(classes, " ").
		Str(" kind:").Int(int64(current.kind)).Str(" target:").
		Str(strings.TrimSpace(detail)).String()
}

var specialAuditOperations = map[string][]string{
	"bfd-frr": {
		"class:session class:timing class:query wait for BGP Established",
		"class:session class:timing class:query wait for the RFC 5880 BFD peer Up",
		"class:mutation install the outbound UDP/3784 drop rule",
		"class:negative class:branch class:timing class:loop class:query require BGP down inside the five-second safety cap",
		"class:assertion class:timing require measured teardown below two seconds",
		"class:cleanup remove the drop rule on every return path",
	},
	"bgp-addpath-rail-agreement-speaker": {
		"class:session class:timing class:query wait until speaker1 and GoBGP are Established",
		"class:mutation announce 10.99.0.0/24 while speaker2 is not Established",
		"class:route class:timing class:query wait until Ze stores the route",
		"class:negative class:branch reject an already-Established speaker2 because replay was not exercised",
		"class:byte-agreement class:capability class:branch class:loop require one complete route-bearing UPDATE with eight-octet ADD-PATH NLRI from each speaker",
		"class:byte-agreement class:assertion compare decoded live and replay UPDATE bodies byte for byte",
	},
	"bgp-relay-withdraw-reflector-frr": {
		"class:session class:timing class:query wait for FRR Established",
		"class:route class:timing class:loop class:query require one reflected advertisement carrying ORIGINATOR_ID and CLUSTER_LIST",
		"class:negative class:assertion reject a withdrawal body present before the FRR mutation",
		"class:mutation remove FRR network 10.20.0.0/24",
		"class:route class:timing class:loop class:query require exact attribute-free withdrawal bytes",
		"class:session class:negative class:branch class:query require FRR still Established",
	},
	"bgp-holdtime-deadpeer-frr": {
		"class:session class:timing class:query wait for FRR Established",
		"class:mutation freeze FRR while its kernel continues acknowledging TCP",
		"class:observer class:timing class:loop class:branch require notification-sent and hold-timer-expired on one Ze log line",
		"class:timing class:assertion require 6s <= notification elapsed < 13.5s",
		"class:cleanup unpause FRR on every return path",
		"class:negative class:query class:branch reject FRR-originated notification direction",
		"class:assertion require FRR lastNotificationReason Hold Timer Expired",
		"class:session class:timing class:query require automatic re-establishment",
	},
	"bgp-max-prefix-per-family-frr": {
		"class:session class:timing class:query wait for FRR Established",
		"class:observer class:assertion class:branch class:timing class:loop require overflow, family=ipv4/unicast, and teardown=false on one Ze log line",
		"class:negative class:session class:query class:loop prove Established continuously for 20 seconds",
	},
	"bgp-wire-edit-api-origin-bird": {
		"class:timing class:branch parse connect-delay barrier and reject SESSION_TIMEOUT below delay plus 10 seconds",
		"class:observer class:query fail immediately on ZE-OBSERVER-FAIL before or after peer checks",
		"class:session class:timing class:query wait for ze_peer Established",
		"class:route class:timing class:loop class:query require queue and batch rail prefixes",
		"class:assertion class:query require both standard communities on each BGP.community line",
		"class:assertion class:query require the large community on each BGP.large_community line",
		"class:negative class:session class:query require exactly one ze_peer State changed to up event",
	},
	"isis-purge-reorig-frr": {
		"class:adjacency class:assertion class:timing class:loop class:query wait for Level-1 adjacency Up",
		"class:route class:query require a live body-bearing Ze LSP before injection",
		"class:negative class:branch reject a pre-injection sequence at or above 4096",
		"class:mutation class:cleanup obtain Ze PID, enter its network namespace, send an exact 802.3 LLC Level-1 purge through AF_PACKET, and restore the original namespace",
		"class:route class:timing class:query require sequence above 4096, live holdtime, and body after injection",
		"class:route class:timing class:query require Ze system ID or hostname in the Level-1 topology",
		"class:adjacency class:negative class:timing prove the adjacency remains Up five seconds later",
	},
	"ospf-lfa-frr": {
		"class:adjacency class:assertion class:branch class:timing class:query wait for both OSPF adjacencies Full",
		"class:route class:timing class:query require a protected backup for 172.30.255.3/32",
		"class:mutation class:cleanup bring FRR eth0 down and restore it on every path",
		"class:timing wait two seconds for the failure to propagate",
		"class:negative class:route class:timing class:loop require ping reachability through the backup during the cut",
	},
	"ospf-ti-lfa-frr": {
		"class:adjacency class:assertion class:branch class:timing class:query wait for both OSPF adjacencies Full",
		"class:capability class:timing class:query require enabled SRGB 16000..23999 and Prefix-SID index 200",
		"class:route class:capability class:timing class:query require a protected backup and validate every repair label as 20-bit MPLS",
		"class:mutation class:cleanup bring FRR eth0 down and restore it on every path",
		"class:timing wait two seconds for the failure to propagate",
		"class:negative class:route class:timing class:loop require ping reachability through the backup during the cut",
	},
	"show-rib-under-frr-load": {
		"class:session class:timing class:query wait for FRR Established and at least 256 routes",
		"class:route class:query require a document with at least 256 routes and exactly 50 bounded best-path rows",
		"class:concurrency class:mutation class:loop toggle FRR static redistribution concurrently with eight RIB walkers for 45 seconds",
		"class:query class:branch fail closed on every malformed or unanswered count, document, or best-path row",
		"class:negative class:assertion require at least one walk and at least two observed route totals",
		"class:cleanup restore FRR redistribution enabled after the load",
		"class:route class:timing class:query require the RIB to return to at least 256 routes",
		"class:session class:query require FRR still Established after the concurrent load",
	},
}

var checkerAudit = map[string]AuditEntry{
	"as112-community-frr":                       {SourceSHA256: "bad31c2f450183cade3a2ab470255b419d5a413bd35bb5eda1696a03d06346b9", Assertions: 1, ControlOperations: 7, NativeProducer: "scenarioOperations + scenarioExtras"},
	"as112-origin-as-frr":                       {SourceSHA256: "194fed13ad747ba3b4f129d5ed3d294c07bba75371c6b1d33414385e73f4ffef", Assertions: 5, ControlOperations: 18, NativeProducer: "scenarioOperations + scenarioExtras"},
	"as112-redistribute-community-frr":          {SourceSHA256: "61dd908ed5e0e5e3badca4a355ff9e5b0e0af4a7c75d361717ca2b8a2fdfd75c", Assertions: 1, ControlOperations: 7, NativeProducer: "scenarioOperations + scenarioExtras"},
	"as112-redistribute-lab":                    {SourceSHA256: "fb7585d7d824916a667f43b72c5ea487fe517dabd454436380b3de43c9dc4c70", Assertions: 3, ControlOperations: 15, NativeProducer: "scenarioOperations + scenarioExtras"},
	"as112-redistribute-origin-custom-frr":      {SourceSHA256: "6091563902ef3016ec8da9e4bd3a46125925ef99c1d7ebfaa4e5c3537ec33d73", Assertions: 2, ControlOperations: 11, NativeProducer: "scenarioOperations + scenarioExtras"},
	"as112-redistribute-origin-frr":             {SourceSHA256: "965596fcf9daf1954caf3ea6c518cac0a50d9724c0ca6e0338cd3ff474e2a550", Assertions: 5, ControlOperations: 20, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bfd-frr":                                   {SourceSHA256: "07fb91b14f4bc7d6e7d0431d03b806c429eed242789e259bab155f8a2fe1e582", Assertions: 2, ControlOperations: 10, NativeProducer: "checkBFDFailover"},
	"bgp-4byte-asn-frr":                         {SourceSHA256: "26e25abcefbeedb5002d12a9f30581814b7666a44697eb9e78ccb98964f94b8b", Assertions: 1, ControlOperations: 6, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-addpath-frr":                           {SourceSHA256: "01d73ce3ddb809ef4c3b3be739bbb496eb499016ed5f1cd603ca7cfdea4567c4", Assertions: 3, ControlOperations: 9, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-addpath-rail-agreement-speaker":        {SourceSHA256: "7cd1f38d4d85d056b198cef4efa2efe784606af8190daa5316ddd3895c0dc163", Assertions: 6, ControlOperations: 25, NativeProducer: "checkAddPathRailAgreement"},
	"bgp-addpath-readvertise-collision-frr":     {SourceSHA256: "162b35d4610e868d57cb584613135afd4b92f8e63475583509e201573f7435e0", Assertions: 5, ControlOperations: 26, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-community-frr":                         {SourceSHA256: "0c09f91f22744b09749e6a137abbbab7b72a918afdf6024d0d1d08cbb34be093", Assertions: 1, ControlOperations: 8, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-ebgp-gobgp":                            {SourceSHA256: "25f0b425428ea8f7bdddf7ea997955917254b9b9b34486f342b871fa8c584ce1", Assertions: 0, ControlOperations: 1, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-ebgp-ipv4-bird":                        {SourceSHA256: "a7f60977381fc818cd1593b231421c35601037a511681ac2289b4648e1cef174", Assertions: 0, ControlOperations: 1, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-ebgp-ipv4-frr":                         {SourceSHA256: "71253a5e74f1e3857cdcf95729e2232221ee59a5fdbb9c603f7a8ec22109d427", Assertions: 0, ControlOperations: 1, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-ecmp-frr":                              {SourceSHA256: "63510558c525abd70395319f8dd94d1522c9eaa17df2ee7e0fa5cfa20a1cbcd5", Assertions: 2, ControlOperations: 15, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-evpn-frr":                              {SourceSHA256: "5e2ccbb0aebf0795924a217662d929cc3b2dcefc3f57238ccf8ee67c6a641689", Assertions: 5, ControlOperations: 13, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-evpn-gobgp":                            {SourceSHA256: "fbd4543367673d7e5b9c4c969c6d375777ba0a39ced0241a1a484ba901658c05", Assertions: 3, ControlOperations: 7, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-extended-community-frr":                {SourceSHA256: "49cc7bc15e45850a9d1a722b4b6cb16ea8458cbb3dfa52b6fee167f15321448b", Assertions: 2, ControlOperations: 6, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-flowspec-frr":                          {SourceSHA256: "de1d210856a242812a6e020aafe84b76fb57ab4a1354994d7e188a04c162eccb", Assertions: 5, ControlOperations: 12, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-flowspec-gobgp":                        {SourceSHA256: "8580ee61a1e6c210f6cc26334c2f64b0517cc61bc1f89adf530cea02b1cca3c7", Assertions: 3, ControlOperations: 7, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-flowspec-sctp-gobgp":                   {SourceSHA256: "d3e10f97a1b3ea36713bac514634dfccbec1375254bd93d4d93d2f66bd1cd891", Assertions: 5, ControlOperations: 12, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-graceful-restart-frr":                  {SourceSHA256: "2b55b1391cd3b581d0d10d87b5c32b02796bfbdd25958e3106fa6aac5d0a6229", Assertions: 2, ControlOperations: 14, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-gtsm-frr":                              {SourceSHA256: "ce8cd8ee958074deb6e253c90900805e0447afc6fa0ac7c586247cea26188621", Assertions: 0, ControlOperations: 1, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-holdtime-deadpeer-frr":                 {SourceSHA256: "957e12049f126211b6ec14b20b05a49a80370efb4da1c43cd7e87fdd161b8091", Assertions: 5, ControlOperations: 14, NativeProducer: "checkHoldtimeDeadPeer"},
	"bgp-ibgp-frr":                              {SourceSHA256: "3704b40e519685f2c4f1064f783ea95068f44769455e6dfcc7affc68ee75b303", Assertions: 0, ControlOperations: 1, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-ipv6-ebgp-bird":                        {SourceSHA256: "582fd1b331163dc7ceb5c251d2e955739e64ecc2f43bf41e2443b53f732b7c78", Assertions: 1, ControlOperations: 6, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-ipv6-ebgp-frr":                         {SourceSHA256: "dfcfbf6ffcc42f6867d37419f270aac9b1880f3cce98de4846b63d0c17d97c88", Assertions: 1, ControlOperations: 6, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-ipv6-ebgp-gobgp":                       {SourceSHA256: "58f8e462cb90c8fca093f86aab89cf25872d986e9844fbb0f4deade84e290af7", Assertions: 1, ControlOperations: 6, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-local-pref-strip-gobgp":                {SourceSHA256: "6021ee0e9003e911e582359cd14b9fd196dadb426d64b1ed4d3844fa31ef7c70", Assertions: 5, ControlOperations: 22, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-max-prefix-cease-frr":                  {SourceSHA256: "ec33d1483a952f56cc4022eff29e72f93819ff2d3cda211577f8fd2fb35537f1", Assertions: 3, ControlOperations: 9, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-max-prefix-per-family-frr":             {SourceSHA256: "90823c79449b738123cb98d757cc0c64ef74b7e1fb3d8c28f41874a57eb11f39", Assertions: 5, ControlOperations: 10, NativeProducer: "checkMaxPrefixPerFamily"},
	"bgp-md5-auth-frr":                          {SourceSHA256: "8dd8a07f6950f7a3e15e8e584b7bc4b70b9a15e6db84d9bbe1c6922faa0053dc", Assertions: 0, ControlOperations: 1, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-med-across-as-gobgp":                   {SourceSHA256: "34e0a560d41f94d7b3e91b02c9e86dc268d38e2be9a5bee6141095471079436d", Assertions: 6, ControlOperations: 23, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-med-ibgp-post-selection-removal-gobgp": {SourceSHA256: "6dec61b6f5cd7d4ad1b107b4b5062c3fa057a98e9201085063766f79cedb8861", Assertions: 6, ControlOperations: 15, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-med-remove-configured-gobgp":           {SourceSHA256: "6df58d5dd656c315e3b299685ec6553ed8fd4ca1478144202d348f985d089e62", Assertions: 5, ControlOperations: 15, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-multihop-ebgp-bird":                    {SourceSHA256: "53a4be58d05bef8f66d314114ddeddf4a0c009a1a1299c3fff4ddcb09b1e97b7", Assertions: 1, ControlOperations: 5, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-multihop-ebgp-frr":                     {SourceSHA256: "0438502146fa1ddcd21075364e35f5c823d4183cb742a478c668fe1dff410a01", Assertions: 2, ControlOperations: 8, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-multihop-ebgp-gobgp":                   {SourceSHA256: "0c5ec0a7c992f8919e8a3ba8c5371eb3ede563739312f9c832c51c2a43fe73db", Assertions: 1, ControlOperations: 5, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-paths-limit-frr":                       {SourceSHA256: "c80fe3ac377c17052ba58839cfaef4b2b88436c32584d04ec543a90f2f4e8f22", Assertions: 2, ControlOperations: 7, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-policy-import-export-frr":              {SourceSHA256: "4a7d7211e8eb56343b8524d1fbfa2eb9e73761cdf3fb7b7ab107adcff66793e9", Assertions: 7, ControlOperations: 14, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-redist-late-join-dynamic-frr":          {SourceSHA256: "554f4dd41ce71601f5cf2833109db0ef5d4926efc1f8ab7a2732342ef2286777", Assertions: 1, ControlOperations: 8, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-relay-withdraw-nexthop-self-frr":       {SourceSHA256: "ab462385491ebb0e4450828c09a40932732d4b2f4d5b061b54994f02121cb559", Assertions: 4, ControlOperations: 12, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-relay-withdraw-reflector-frr":          {SourceSHA256: "f3dd6c383000bd7efe22ba8699b4e66b9b4098ebb8ddd614024d2b38ac1f9622", Assertions: 4, ControlOperations: 6, NativeProducer: "checkReflectorWithdrawal"},
	"bgp-relay-withdraw-shape-frr":              {SourceSHA256: "78d32801f842166ea1169c9b5bc519f38e1a434a229ff9792022ee5eb6d6e04e", Assertions: 4, ControlOperations: 12, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-remove-private-as-as4path-frr":         {SourceSHA256: "04fafff5cc4d707041fcb8862c34249f890b369676223bbbbf4f040ee78b84a1", Assertions: 3, ControlOperations: 13, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-remove-private-as-frr":                 {SourceSHA256: "bc31429adeeb68868c64b1e26ae02a081a0c0e7330b88ae2ef8a340d35bbf8a6", Assertions: 3, ControlOperations: 13, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-rfc2545-linklocal-nexthop-frr":         {SourceSHA256: "109de49a3fec94e57b277e4b732cb0ef2cd3de4be7ed8e445018861e406ff9a4", Assertions: 13, ControlOperations: 10, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-rfc7606-relay-shape-frr":               {SourceSHA256: "2bcbac34a08c79bea8fbcdda200d4301c5a7b327dc0d24715ac6ebb191ef1796", Assertions: 2, ControlOperations: 15, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-rfc7606-speaker-dup-attr":              {SourceSHA256: "e3d311aad17996976c520ddb16822bbc7a2063fa651ae99cbdf006403873fad1", Assertions: 3, ControlOperations: 11, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-rfc7606-typed-nlri-discard":            {SourceSHA256: "58fb39d40fc9f28e1c5638b62052ec3de8b1d4b27a1b483af4aebf9cb86a0cd8", Assertions: 2, ControlOperations: 11, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-rfc7999-blackhole-frr":                 {SourceSHA256: "4d7421dc924b9f8db6db781b421e4ed055f8e475b3603ec1c8f60b638657a906", Assertions: 5, ControlOperations: 10, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-role-frr":                              {SourceSHA256: "90c1129acff96bb20d0b4b60d9bc3a12c6762be8fcd483803424e550f8de7ade", Assertions: 2, ControlOperations: 5, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-role-gobgp":                            {SourceSHA256: "911b7c9b34359a9b775b8292d9ab043a00a505f325bfbffb15a3c864c9d9765f", Assertions: 1, ControlOperations: 2, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-role-otc-withdraw-frr":                 {SourceSHA256: "854180305c3e6db88b84782185d8872bef0379dfc72fe7c6de65b45fe7cb47cc", Assertions: 6, ControlOperations: 15, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-route-reflection-frr":                  {SourceSHA256: "94139856aa1abf99d160c75e4289fcfc196bad0e0c4e370f016b1e8fa7d9107f", Assertions: 3, ControlOperations: 8, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-route-refresh-frr":                     {SourceSHA256: "432a341004ae2b417327e5d7c924f4b451a33b5640c3f5fb41aad020a2aee26d", Assertions: 1, ControlOperations: 10, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-route-server-frr":                      {SourceSHA256: "acf8220aae12718c94b319b907e84da4a64d08d4f09da8ee0a3cea7a2567d468", Assertions: 2, ControlOperations: 9, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-route-withdrawal-frr":                  {SourceSHA256: "36c8149109210b50e7af5cef3bf5167d745c11a165d4134854325cb42680f05d", Assertions: 1, ControlOperations: 12, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-routes-from-bird":                      {SourceSHA256: "8af275cce4b58e28cf1f278f3848271bcd40c710d0893f7e06cea92b245d77a7", Assertions: 2, ControlOperations: 10, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-routes-from-frr":                       {SourceSHA256: "81ec7b4766d2862209a046f5418e9e0c4b879fba29d7ac7da1c03e13b7d132eb", Assertions: 2, ControlOperations: 10, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-routes-gobgp":                          {SourceSHA256: "3891923a9340db7691bf32539b0ea5ac3272bea42caca0264f81258417c68950", Assertions: 5, ControlOperations: 14, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-routes-to-frr":                         {SourceSHA256: "56ad4f84e47801806cf9b74ceb89981413c829c1346638a25a5b0538ee225bcd", Assertions: 1, ControlOperations: 9, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-self-nexthop-withheld-frr":             {SourceSHA256: "cc0621bb73c25c706d0635a4ce2ecf44919a115180590781697fcd208c5a2fd6", Assertions: 4, ControlOperations: 11, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-send-community-suppress-frr":           {SourceSHA256: "6e6461178c5537158b54e4eaba360b70c1310e89dc4df64f882543fef95ae3ba", Assertions: 7, ControlOperations: 30, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-speaker-two-instance":                  {SourceSHA256: "be043359a44c02968cac71d6a61a92b3d438c77754393e5c21a7651559c08869", Assertions: 2, ControlOperations: 10, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-srv6-frr":                              {SourceSHA256: "7f8f85680cb5287418d20e2efb393a44798e4f89c284ad367ca7cd69cb99b008", Assertions: 5, ControlOperations: 20, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-triangle":                              {SourceSHA256: "23d07cbc96b0bf4e704ac38d530ad118e9451aa6f265d25ad5801b43ccf3a610", Assertions: 3, ControlOperations: 8, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-vpn-frr":                               {SourceSHA256: "fe571e120279851d136de48d28cfbce7fb03c0aa3347f002946b240d9b493b0c", Assertions: 4, ControlOperations: 13, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-vpn-gobgp":                             {SourceSHA256: "ae3541c18f98c303e4c92f665ff1e4b6ea97c0df9a777353b965ebd64205b046", Assertions: 3, ControlOperations: 7, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-wellknown-noexport-frr":                {SourceSHA256: "4735388783c45db1bb5b11022309e044fb23e55756a2f91c4f73d6e95a30cd27", Assertions: 2, ControlOperations: 12, NativeProducer: "scenarioOperations + scenarioExtras"},
	"bgp-wire-edit-api-origin-bird":             {SourceSHA256: "7d3da425d04db0073334845b1a33624340a3b33b12b75247cd9ebee5bb7d6749", Assertions: 7, ControlOperations: 25, NativeProducer: "checkWireEditAPIOriginBIRD"},
	"bmp-frr":                                   {SourceSHA256: "ef6a224079afecbc7fd10b1ecdd38e902e41f3fa17a0d7d439d6b42cb5f959bf", Assertions: 1, ControlOperations: 7, NativeProducer: "scenarioOperations + scenarioExtras"},
	"isis-auth-frr":                             {SourceSHA256: "bf9bed1082acc36b4995f4d9d5b27489345f697e2fef69199263a01598fbca9f", Assertions: 1, ControlOperations: 4, NativeProducer: "scenarioOperations + scenarioExtras"},
	"isis-convergence-frr":                      {SourceSHA256: "e21424cf3be5c85d953cf1a6dc7c9162da10da31c5dfa85f04c155fe097b3617", Assertions: 1, ControlOperations: 10, NativeProducer: "scenarioOperations + scenarioExtras"},
	"isis-dualstack-frr":                        {SourceSHA256: "f000ccbb63b226c8cdc3004e2d95786f6bd915d2e485a37df5fd020c68717d35", Assertions: 2, ControlOperations: 14, NativeProducer: "scenarioOperations + scenarioExtras"},
	"isis-lan-dis-frr":                          {SourceSHA256: "0ea6a5da7dd7836c16ace77c9749d7d44ab2a269042ca8c2aa80e85bfd81061b", Assertions: 2, ControlOperations: 9, NativeProducer: "scenarioOperations + scenarioExtras"},
	"isis-p2p-frr":                              {SourceSHA256: "2c1096ca08dd600a990e4eab4fee34fdecc6163f0e45cb3c85feacb83eaed143", Assertions: 2, ControlOperations: 11, NativeProducer: "scenarioOperations + scenarioExtras"},
	"isis-purge-reorig-frr":                     {SourceSHA256: "fb1091528fec9a55804d8e81300ba6ca3cf311da6dcaf4f3bb9f2ba9af0be581", Assertions: 4, ControlOperations: 23, NativeProducer: "checkISISOwnLSPPurge"},
	"isis-redist-frr":                           {SourceSHA256: "72ba479e8f32c7aa8cc250bdd7dbe13ffc69133e758d99810aa5540438b5b3a0", Assertions: 2, ControlOperations: 13, NativeProducer: "scenarioOperations + scenarioExtras"},
	"no-family-peer-eor-frr":                    {SourceSHA256: "c1aa2ba4886e3a30cd8ed13cd71b245143c33e6d2a59069e9002e1962b4af68f", Assertions: 4, ControlOperations: 13, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-auth-frr":                             {SourceSHA256: "b62a60df474129eac2aca5b55e4b7ddade29d0e978794b919d8581a980f78700", Assertions: 0, ControlOperations: 2, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-bfd-frr":                              {SourceSHA256: "a8466813bede0b431d72c5302c769b88b1062ceaea3c970b8a007641b4edea34", Assertions: 1, ControlOperations: 7, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-broadcast-frr":                        {SourceSHA256: "f40875f731f7e174bc96547b145a3e6cdfee6e5c8498a7108feb2fb2de07a23d", Assertions: 2, ControlOperations: 7, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-convergence-frr":                      {SourceSHA256: "435a0054c4f3c394b8f4a24fdb3ff9c87c4ccc80eace755951cc155ffcecadc8", Assertions: 1, ControlOperations: 10, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-debug-inject-frr":                     {SourceSHA256: "68ef0f9cacf4114760e2d32dae1e67b5eaa8f3eb57e1b00fc5a80046fe5d518a", Assertions: 3, ControlOperations: 10, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-debug-te-frr":                         {SourceSHA256: "afa20d9d0db3b5df32e6b27e7c32345ead7cdb9a9d3787b9ae00a62719534d2b", Assertions: 1, ControlOperations: 4, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-ext-prefix-link-frr":                  {SourceSHA256: "72e0e424b3cd7089efb70e6608bd69249eec33f0f66fa1606eb2567c7d78468a", Assertions: 2, ControlOperations: 9, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-gr-fib-retention":                     {SourceSHA256: "63a6e4fd8ca104d165eeb5f92ca4eb53475e29997026564d3bf9647bb0bbf8b6", Assertions: 0, ControlOperations: 2, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-gr-frr":                               {SourceSHA256: "4cf508bcf589cc7701498dae0d5538bf6bd89698f2d5647bc922a3b7743f02c9", Assertions: 2, ControlOperations: 7, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-ipsec-ah-frr":                         {SourceSHA256: "46d1875cc169631e6b5aafd03b2a268b4cdd0ad24c9067eecba3be4e18c4580d", Assertions: 0, ControlOperations: 2, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-ipsec-frr":                            {SourceSHA256: "3deae7076676b0adbda7bc72106f70fd6aadec989007eb72a43aeafdd8d207d4", Assertions: 0, ControlOperations: 2, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-ldp-sync-frr":                         {SourceSHA256: "d16efbfb43f2aa0fad322ca9c726bba7260330aa60045d2a686dcf6a709d1e7a", Assertions: 1, ControlOperations: 10, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-lfa-frr":                              {SourceSHA256: "750e84d0ebb3011518cb3469e61c6e742479d1505869c37ef153c7e008cb1f35", Assertions: 2, ControlOperations: 14, NativeProducer: "checkOSPFLFA"},
	"ospf-multiaf-frr":                          {SourceSHA256: "0fdb1b958f4a37670ad7040038a82e028d94142c81d6e108816d02cb70ab2959", Assertions: 2, ControlOperations: 8, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-multiaf-v4-frr":                       {SourceSHA256: "d7e0dd5156fc655b58eb1c4fe4ad7bbafd46e9564fc6e0a70b8d9e2eb56a7775", Assertions: 1, ControlOperations: 2, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-multiarea-frr":                        {SourceSHA256: "4b865a9477ac5ef313e0dcf8a93be40e2bbfb12d03a6e763b131ad899a02705b", Assertions: 1, ControlOperations: 5, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-multiinstance-frr":                    {SourceSHA256: "534937e4650a8efbc5b8ea75a97ff95aa77f585defad09c454da1381de3648d0", Assertions: 3, ControlOperations: 12, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-nbma-frr":                             {SourceSHA256: "c36cd3a5f703a00885611a47f3134dfaecbc8737c8b54edeef399e9e0549f48f", Assertions: 3, ControlOperations: 9, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-opaque-frr":                           {SourceSHA256: "17408ddbe7dfd37d7cd9b6f7d076f87f9d1c38f91dd2962785f64f0f60cd84d2", Assertions: 2, ControlOperations: 9, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-p2p-frr":                              {SourceSHA256: "d0f9c0cb31aa42b308430e2d9377b4b30a1e2749969a44f1f5deea03ad8821ba", Assertions: 2, ControlOperations: 10, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-ptmp-frr":                             {SourceSHA256: "a562d269b2a89190e9b0558b91a1d63ff1090ef9f52a9e492763edda1aface21", Assertions: 3, ControlOperations: 9, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-ri-frr":                               {SourceSHA256: "e71e199469263cc17eaa9b5dbea01d4b551ae4c58069e4a57c2d4eb0ac8bcb1f", Assertions: 2, ControlOperations: 9, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-sr-frr":                               {SourceSHA256: "831d3e1ddfd946806be0a3118804cdb824d680e9ed0e0c5f659b1e9013e7af5e", Assertions: 4, ControlOperations: 14, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-stub-nssa-frr":                        {SourceSHA256: "87d5b1aaab1f4262b1d8046864de01f86fc0862dc34284d767420b3c590b4b5a", Assertions: 2, ControlOperations: 7, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-te-frr":                               {SourceSHA256: "a4cc52e1bf5ab369b97f4f29b1420690a49f1de342aee35a25b74a8b0bd31477", Assertions: 2, ControlOperations: 9, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-te-interas-frr":                       {SourceSHA256: "4465489871e45cbe2eca11816ef6056a546988ac4dfa6bb3182aec9ca93de1aa", Assertions: 2, ControlOperations: 9, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospf-ti-lfa-frr":                           {SourceSHA256: "d6e957ac5e573569ce3a73e0b783a34bc46b3793975692dd875fdc7c1a863a84", Assertions: 4, ControlOperations: 18, NativeProducer: "checkOSPFTILFA"},
	"ospf-virtual-link-frr":                     {SourceSHA256: "7062e6543292ecce3bf4f2706352f3cbd1e1851e6a128f9ec5abfc7f2a660c29", Assertions: 1, ControlOperations: 6, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospfv3-bfd-frr":                            {SourceSHA256: "077876a902ca99e33ff7d8da4b1efc3ab197f90329ad1442096a091b58a8cf89", Assertions: 2, ControlOperations: 10, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospfv3-broadcast-frr":                      {SourceSHA256: "5cf272dffc1adf45f03a4161f6778777d067669a8bfc94e7f0f4b093226fefc6", Assertions: 3, ControlOperations: 12, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospfv3-debug-decode-frr":                   {SourceSHA256: "1d51f4c5fef919157381f4474a618e374164ebab38fc40357e9ed48de15ed68d", Assertions: 2, ControlOperations: 5, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospfv3-debug-inject-frr":                   {SourceSHA256: "07d9c71eafa0bf5b18ec5f5c87c0f1d4d4ffecfa67a87ece374962e0a98b1e01", Assertions: 2, ControlOperations: 8, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospfv3-frr":                                {SourceSHA256: "8c4a88fd7028b860f8e92bcd252e52672e658e576b9e0e6e6f32086e212ae13c", Assertions: 2, ControlOperations: 9, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospfv3-gr-fib-retention":                   {SourceSHA256: "fd4852af454421ead46fd499ea28402f8695e8d0361048f55153d1c1c3fb60a0", Assertions: 0, ControlOperations: 2, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospfv3-gr-frr":                             {SourceSHA256: "2a14be1a83ac19dab858400c4bfd6bd88343e790aecaa7e4a0e40ffee8ba0874", Assertions: 2, ControlOperations: 7, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospfv3-multiarea-frr":                      {SourceSHA256: "58dc02f5ad28c7e3175ff7e5d363b0e5ddd85f77498a4f0c8f1a6378f2254102", Assertions: 1, ControlOperations: 7, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospfv3-nbma-frr":                           {SourceSHA256: "ac89776078e05b07c375aa3b8c56c2c477f2fd996e51f03ab5a61aaf4b6ff518", Assertions: 0, ControlOperations: 3, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospfv3-nssa-redist-frr":                    {SourceSHA256: "7addc151d9427a7584dd5e6660ca13eeade92d99ad467aeb43df4cf22727f9e6", Assertions: 3, ControlOperations: 15, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospfv3-ptmp-frr":                           {SourceSHA256: "3feb48908c6bf41dfe07de8a1eb6e4ddbbbc73b75b4009e3cbec749d04e519c3", Assertions: 3, ControlOperations: 9, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospfv3-redist-frr":                         {SourceSHA256: "dfcbc3c4e8a696e8e0449782ce17787b3e93aff26fd20ab7c18983350c42704c", Assertions: 1, ControlOperations: 8, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospfv3-ri-frr":                             {SourceSHA256: "9d38949c7548879fe93ffb4fa538d306d343be8aebfc95f8ae531d2af88c2f19", Assertions: 2, ControlOperations: 8, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospfv3-sr-frr":                             {SourceSHA256: "47b2c087ecd0a7e50abfe68622fe24c38f5a62382391d20aa40636183300f7bc", Assertions: 3, ControlOperations: 12, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospfv3-stub-frr":                           {SourceSHA256: "3f58f3c94d6778c9dc9e80460f12dd7393765bae2f7580b677cc58cbddfeb671", Assertions: 1, ControlOperations: 6, NativeProducer: "scenarioOperations + scenarioExtras"},
	"ospfv3-vlink-frr":                          {SourceSHA256: "aab78e92d44cdbba10dd502375c02bdc6fabb79ec55cf6c0d62efacafa97d011", Assertions: 2, ControlOperations: 6, NativeProducer: "scenarioOperations + scenarioExtras"},
	"rpki-frr":                                  {SourceSHA256: "70da84fdd0283fa367b3f11fbf4fc46b88b539f4d74f3e90c4e4f2ec87a9af12", Assertions: 4, ControlOperations: 10, NativeProducer: "scenarioOperations + scenarioExtras"},
	"rtr-stayrtr":                               {SourceSHA256: "e59d9a3f077dccf90ca10a01bb05670470df9327db6c7672e3b36882599bcdee", Assertions: 5, ControlOperations: 14, NativeProducer: "scenarioOperations + scenarioExtras"},
	"show-rib-under-frr-load":                   {SourceSHA256: "63ca958adce0057f4c8f86b1be7894bd99a8ed0849885dcff82eb9bdc85e4657", Assertions: 10, ControlOperations: 21, NativeProducer: "checkShowRIBUnderFRRLoad"},
	"shutdown-cease-frr":                        {SourceSHA256: "c5c1b7b5ed88fa0dc3d894a1c2484447256eea82e3045a275d3088e6b52324cd", Assertions: 4, ControlOperations: 24, NativeProducer: "scenarioOperations + scenarioExtras"},
	"vrrp-mastership-keepalived":                {SourceSHA256: "cf7ea59144c4087cd227668a5f5ddf51dd8e5c42d7617a5a0bc420a9912836d2", Assertions: 4, ControlOperations: 16, NativeProducer: "scenarioOperations + scenarioExtras"},
}
