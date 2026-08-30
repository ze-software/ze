// Design: docs/architecture/testing/interop.md -- the names, addresses, and
// output tokens the interop scenarios share.
// Related: checkers.go, check_extras.go -- the scenario tables that read them.
package bgp

// Peers. A name is the container's role in a scenario lab. It is the
// PeerConfig name, the stem of the container name, and the peer argument that
// Exec, Query, Logs, Pause, and Unpause take.
const (
	peerFRR        = "frr"
	peerBIRD       = "bird"
	peerGoBGP      = "gobgp"
	peerBMP        = "bmp"
	peerRPKI       = "rpki"
	peerInject     = "inject"
	peerSpeaker    = "speaker"
	peerSpeaker2   = "speaker2"
	peerKeepalived = "keepalived"
	peerStayRTR    = "stayrtr"
)

// Programs a scenario runs inside a peer container. cmdGoBGP has the spelling
// of peerGoBGP and names a different thing: the client binary, not the
// container it runs in.
const (
	cmdVtysh     = "vtysh"
	cmdBirdc     = "birdc"
	cmdGoBGP     = "gobgp"
	cmdCat       = "cat"
	cmdIptables  = "iptables"
	zeTestBinary = "ze-test"
)

// vtysh commands. FRR takes one after each -c flag.
const (
	frrConfigureTerminal          = "configure terminal"
	frrShowBFDPeers               = "show bfd peers"
	frrShowISISDatabase           = "show isis database"
	frrShowISISNeighbor           = "show isis neighbor"
	frrShowOSPFDatabaseNetwork    = "show ip ospf database network"
	frrShowOSPFDatabaseOpaqueArea = "show ip ospf database opaque-area"
	frrShowOSPFDatabaseRouter     = "show ip ospf database router"
	frrShowOSPFNeighbor           = "show ip ospf neighbor"
	frrShowOSPF6DatabaseRouter    = "show ipv6 ospf6 database router"
	frrShowOSPF6Neighbor          = "show ipv6 ospf6 neighbor"
	frrShowOSPF6Route             = "show ipv6 route ospf6"
	frrShowZeNeighborJSON         = "show bgp neighbor " + zeLabAddress + " json"
	frrShowAS112PrefixJSON        = "show bgp ipv4 unicast " + as112DirectDelegationPrefix + " json"
)

// The FRR log file inside the FRR container.
const frrLogPath = "/tmp/frr.log"

// Address families, as FRR spells one in an operation and as the gobgp client
// spells one after -a.
const (
	frrFamilyIPv4VPN        = "ipv4 vpn"
	frrFamilyIPv6Unicast    = "ipv6 unicast"
	gobgpFamilyIPv4         = "ipv4"
	gobgpFamilyIPv4Flowspec = "ipv4-flowspec"
)

// gobgp client words.
const (
	gobgpGlobal   = "global"
	gobgpRIB      = "rib"
	gobgpAdd      = "add"
	gobgpNeighbor = "neighbor"
	gobgpNextHop  = "nexthop"
)

// ip(8) words. The scenarios break and restore a link, read an address, and
// read the IPsec state with them.
const (
	containerInterface = "eth0"
	ipActionSet        = "set"
	ipFamilyInet       = "inet"
	ipObjectAddr       = "addr"
	ipObjectLink       = "link"
	ipObjectXfrm       = "xfrm"
	linkDown           = "down"
)

// iptables words. The BFD failover scenario drops the BFD control port with
// them.
const (
	iptablesChainOutput         = "OUTPUT"
	iptablesDestinationPortFlag = "--dport"
	iptablesProtocolUDP         = "udp"
	iptablesTargetDrop          = "DROP"
)

// Docker arguments.
const (
	capabilityNetAdmin   = "NET_ADMIN"
	dockerEntrypointFlag = "--entrypoint"
)

// The ze cli invocation that zeCommand builds, and the command that reads it
// back out of an operation.
const (
	zeCLICommand       = "cli"
	zeShowBGPRIBStatus = "show bgp rib status"
)

// ze-test subcommands. A peer built from the Ze image runs one as its
// container command, and Helper dispatches on the same word.
const zeTestCommandSpeaker = "speaker"

// Lab addresses. networkHostAddress gives each peer a host number on the
// scenario's own /24, and these are the spellings on the default network.
const (
	zeLabAddress       = "172.30.0.2"
	frrLabAddress      = "172.30.0.3"
	gobgpLabAddress    = "172.30.0.5"
	vrrpVirtualAddress = "172.30.0.100"
)

// Prefixes the inject peer announces to the daemon under test. The first
// carries the standard and extended communities, the second the large
// community and the withdrawal cases, and the third the extra path.
const (
	injectPrefixFirst = "10.10.0.0/24"

	// The RFC 6793 Section 4.2.2 AGGREGATOR downgrade scenario: the prefix the
	// injector announces, and the non-mappable aggregating AS its AGGREGATOR
	// carries. BIRD must report that AS after Ze has forwarded the route over a
	// two-octet session, which is only possible through AS4_AGGREGATOR.
	aggregatorDowngradePrefix = "10.0.0.0/24"
	aggregatorDowngradeAS     = "4200000000"
	injectPrefixSecond        = "10.10.1.0/24"
	injectPrefixThird         = "10.10.2.0/24"

	injectPrefixV6First  = "2001:db8:1::/48"
	injectPrefixV6Second = "2001:db8:2::/48"
	injectPrefixV6Third  = "2001:db8:3::/48"
)

// Prefixes a peer daemon announces to Ze, and the prefixes the scenarios
// derive from them.
const (
	peerPrefixFirst     = "10.99.0.0/24"
	peerPrefixSecond    = "10.99.1.0/24"
	flowspecMatchPrefix = "10.99.5.0/24"
	flowspecOrOfAndRule = "[destination: 10.99.2.0/24][port: >80&<100 >443&<500]"
	ecmpPrefix          = "10.100.0.0/24"
	medPrefix           = "10.62.0.0/24"
	ospf6NSSAPrefix     = "2001:db8:7e5::/48"
)

// The AS112 blocks the as112 scenarios carry, and the host route inside each
// one that must not reach a peer.
const (
	as112DirectDelegationPrefix    = "192.175.48.0/24"
	as112DirectDelegationHostRoute = "192.175.48.1/32"
	as112DNAMERedirectionPrefix    = "192.31.196.0/24"
	as112DNAMERedirectionHostRoute = "192.31.196.1/32"
)

// Communities the bgp-send-community-suppress-frr scenario announces and
// requires the peer never to receive. FRR writes a standard community as
// AS:value and a large community as AS:value:value. BIRD writes the same
// standard communities in parentheses.
const (
	suppressedCommunityFirst      = "65004:100"
	suppressedCommunitySecond     = "65004:200"
	suppressedCommunityThird      = "65004:300"
	suppressedLargeCommunity      = "65004:1:2"
	birdSuppressedCommunityFirst  = "(65004,100)"
	birdSuppressedCommunitySecond = "(65004,200)"
)

// Well-known communities the AS112 scenarios assert.
const (
	communityNoExport = "no-export"
	communityNoPeer   = "no-peer"
)

// Tokens a peer daemon prints, which an assertion matches. Two carry the
// spelling of something else. peerStateEstablished is the session state that
// Ze, BIRD, and gobgp report, and gobgp's output is matched case-folded
// against it, while fieldEstablished is a speaker log field name.
// nextHopScopeGlobal is a scope in FRR's JSON, while gobgpGlobal is a client
// subcommand.
const (
	bfdStatusUp                    = "Status: up"
	birdExtendedCommunityAttribute = "ext_community"
	birdLargeCommunityAttribute    = "large_community"
	birdZeProtocol                 = "ze_peer"
	frrCapabilityNegotiated        = "advertisedAndReceived"
	peerStateEstablished           = "established"
	nextHopScopeGlobal             = "global"
	ospfNeighborHeading            = "Neighbor"
	ospfStateFull                  = "Full"
)

// IPsec state, as ip xfrm state prints it.
const (
	xfrmAuthTruncSHA256 = "auth-trunc hmac(sha256)"
	xfrmModeTransport   = "mode transport"
	xfrmObjectState     = "state"
	xfrmProtoAH         = "proto ah"
	xfrmProtoESP        = "proto esp"
)

// JSON and log field names the assertions read, and the values they expect.
const (
	fieldError       = "error"
	fieldEstablished = "established"
	fieldRoutesIn    = "routes-in"
	fieldRPKI        = "rpki"
	fieldState       = "state"
	fieldStatus      = "status"
	fieldTypes       = "types"

	logValueYes       = "yes"
	rpkiStateInvalid  = "invalid"
	rpkiStateNotFound = "not-found"
	rpkiStateValid    = "valid"
)

// The native speaker oracles. Each one names the wire defect it must not find.
const (
	speakerOracleNoDuplicateAttribute   = "no-duplicate-attribute"
	speakerOracleNoUnrecognizedEVPNType = "no-unrecognized-evpn-type"
)

// ExaBGP helper modes and API commands.
const (
	exabgpClearAdjRIBOut = "clear adj-rib out"
	exabgpCommandStatus  = "status"
	exabgpModeEcho       = "echo"
)

// Scenario identities. A scenario directory under test/interop/scenarios has
// the same name, and so does its row in every registry that carries it. A
// scenario named in one registry alone stays a literal there, so a name in
// this block says that two or more rows must move together.
const (
	scenarioAS112OriginASFRR                 = "as112-origin-as-frr"
	scenarioAddPathFRR                       = "bgp-addpath-frr"
	scenarioECMPFRR                          = "bgp-ecmp-frr"
	scenarioEVPNFRR                          = "bgp-evpn-frr"
	scenarioEVPNGoBGP                        = "bgp-evpn-gobgp"
	scenarioFlowspecFRR                      = "bgp-flowspec-frr"
	scenarioFlowspecGoBGP                    = "bgp-flowspec-gobgp"
	scenarioGracefulRestartFRR               = "bgp-graceful-restart-frr"
	scenarioMEDIBGPPostSelectionRemovalGoBGP = "bgp-med-ibgp-post-selection-removal-gobgp"
	scenarioRPKIFRR                          = "rpki-frr"
	scenarioShutdownCeaseFRR                 = "shutdown-cease-frr"
	scenarioVPNFRR                           = "bgp-vpn-frr"
	scenarioVPNGoBGP                         = "bgp-vpn-gobgp"
	scenarioWireEditAPIOriginBIRD            = "bgp-wire-edit-api-origin-bird"
)

// The looking-glass demo topology. Six /24 links carry two routers each, at
// 10.0.<link>.<router>. Four of the twelve also originate a prefix, so the
// same address is a session peer in one row and a next hop in another.
const (
	lgRouter1A = "10.0.1.1"
	lgRouter1B = "10.0.1.2"
	lgRouter2A = "10.0.2.1"
	lgRouter2B = "10.0.2.2"
	lgRouter3A = "10.0.3.1"
	lgRouter3B = "10.0.3.2"
	lgRouter4A = "10.0.4.1"
	lgRouter4B = "10.0.4.2"
	lgRouter5A = "10.0.5.1"
	lgRouter5B = "10.0.5.2"
	lgRouter6A = "10.0.6.1"
	lgRouter6B = "10.0.6.2"
)

// The prefixes the looking-glass demo injects, and the AS path each branch
// carries. The origin AS identifies the prefix, and the leading AS identifies
// the branch that reaches it. The prefix spellings repeat injectPrefixSecond
// and injectPrefixThird, which belong to a different lab.
const (
	lgPrefixFirst  = "10.10.1.0/24"
	lgPrefixSecond = "10.10.2.0/24"
	lgPrefixThird  = "10.10.3.0/24"

	lgASPathFirstVia1A  = "2914,65100"
	lgASPathFirstVia3A  = "174,65100"
	lgASPathSecondVia1B = "13335,65200"
	lgASPathThirdVia1B  = "13335,65300"
	lgASPathThirdVia3B  = "20940,65300"
)
