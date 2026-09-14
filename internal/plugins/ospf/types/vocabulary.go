// Design: docs/architecture/ospf/ospf-1-types.md -- OSPFv2 leaf domain value types
// Related: lstype.go -- the LSA type numbers these words travel beside
//
// vocabulary.go holds the two operator words every OSPF package reads: the network type
// of an interface and the type of an area. They are declared here because five packages
// act on them -- the config resolver, iface, neighbor, lsdb and spf -- and each one held
// its own copy until 2026-09-14. Five declarations of one word set drift, and the reader
// of the second copy cannot tell which one the daemon acts on (ai/rules/principles.md).
//
// The words are the ones ze-ospf-conf.yang declares, and yang_vocabulary_test.go in the
// ospf package gates the two sides against each other.

package types

// Interface network types (RFC 2328 Section 1.2 "Interfaces", RFC 5340 Section 3).
// The word decides whether a DR is elected, how Hellos are addressed, and which link
// records the Router-LSA carries for the interface.
// enumeration: gated by TestNetworkTypeVocabularyMatchesModel
const (
	NetworkBroadcast    = "broadcast"
	NetworkPointToPoint = "point-to-point"
	NetworkLoopback     = "loopback"
	// NetworkNBMA is a non-broadcast multi-access link (RFC 2328 App C.5): a DR/BDR is
	// elected over a manually configured neighbor list and Hellos are unicast/polled.
	NetworkNBMA = "nbma"
	// NetworkPointToMultipoint treats a multi-access link as a collection of
	// point-to-point links (RFC 2328 Section 9.5): no DR/BDR, an adjacency with every
	// reachable neighbor, and a host route for the interface address.
	NetworkPointToMultipoint = "point-to-multipoint"
	// NetworkVirtual marks a synthetic virtual-link interface (RFC 2328 Section 15 / RFC 5340
	// Section 4.2). It is a backbone point-to-point interface whose Router-LSA link is a
	// Type-4 virtual record (IPv4) / RouterLinkTypeVirtual record (IPv6); it carries no
	// Network-LSA/stub link and its packets are routed, not link-local.
	//
	// No configuration leaf carries it: a virtual link is configured as one, and the
	// engine gives the interface this network type.
	NetworkVirtual = "virtual"
)

// Area types (RFC 2328 Section 3.6 stub areas, RFC 3101 NSSA). The word decides which
// LSA types the ABR floods into the area and which default it originates there.
// enumeration: gated by TestAreaTypeVocabularyMatchesModel
const (
	AreaTypeNormal = "normal"
	AreaTypeStub   = "stub"
	AreaTypeNSSA   = "nssa"
)
