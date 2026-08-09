// Design: docs/architecture/ospf/ospf-4-component-config.md -- OSPFv2 config resolution
// Related: yang/ze-ospf-conf.yang -- schema this resolver consumes
// RFC: rfc/short/rfc3101.md -- NSSA translate-role / stability-interval config
//
// Config flows file -> YANG schema -> validated tree -> SDK ConfigSection as
// root-wrapped JSON ({"ospf": {...}}). Tree.ToMap renders scalar leaves as
// strings, keyed lists as key -> entry maps, and nested containers as maps. This
// file mirrors the IS-IS config resolver shape while keeping OSPFv2 types local.
package ospf

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

const (
	DefaultReferenceBandwidth = uint32(100000)
	DefaultMaximumPaths       = uint8(8)
	DefaultSPFDelayMS         = uint32(50)
	DefaultSPFHoldMS          = uint32(200)
	DefaultSPFMaxHoldMS       = uint32(5000)
	DefaultMinLSIntervalMS    = uint32(5000)
	DefaultMinLSArrivalMS     = uint32(1000)
	DefaultExternalMetric     = uint32(20)
	DefaultDefaultMetric      = uint32(1)
	DefaultHelloInterval      = uint16(10)
	DefaultDeadInterval       = uint16(40)
	DefaultRetransmitInterval = uint16(5)
	DefaultTransmitDelay      = uint16(1)
	DefaultPriority           = uint8(1)
	// DefaultPollInterval is the RFC 2328 App C.5 PollInterval: the slower Hello rate
	// (seconds) at which an NBMA interface polls a configured but currently silent
	// neighbor.
	DefaultPollInterval = uint16(120)
	DefaultAreaCost     = uint32(1)
	// DefaultNSSAStabilityInterval is the RFC 3101 section 3.5 translator-stability
	// hysteresis (seconds) a newly demoted translator keeps translating.
	DefaultNSSAStabilityInterval = uint16(40)

	// BFD (RFC 5880 / RFC 5881) per-interface defaults. The config leaves are expressed
	// in milliseconds and stored as microseconds (api.SessionRequest is microsecond-valued),
	// so the 50 ms defaults become 50000 us. Detect multiplier 3 matches the BGP-BFD default.
	DefaultBFDMinTxUs    = uint32(50000)
	DefaultBFDMinRxUs    = uint32(50000)
	DefaultBFDMultiplier = uint8(3)
	// bfdMaxIntervalMs bounds the min-tx/min-rx leaves (milliseconds). 10000 ms (10 s) is a
	// generous ceiling for a sub-second liveness protocol; stored as microseconds (10000000)
	// it stays well within the uint32 api field.
	bfdMaxIntervalMs = uint64(10000)
)

const (
	areaTypeNormal = "normal"
	areaTypeStub   = "stub"
	areaTypeNSSA   = "nssa"

	translateRoleCandidate = "candidate"
	translateRoleAlways    = "always"
	translateRoleNever     = "never"

	networkBroadcast         = "broadcast"
	networkPointToPoint      = "point-to-point"
	networkLoopback          = "loopback"
	networkNBMA              = "nbma"
	networkPointToMultipoint = "point-to-multipoint"

	metricType1 = "type-1"
	metricType2 = "type-2"

	authModeInherit    = "inherit"
	authAlgorithmMD5   = "md5"
	rangeAdvertise     = "advertise"
	rangeNotAdvertise  = "not-advertise"
	redistributeStatic = "static"

	// RFC 4552 IPsec (IPv6 family) protocol and algorithm vocabulary. It mirrors
	// the internal/component/ike/ipsec crypto naming so operators see one vocabulary.
	ipsecProtoAH  = "ah"  // RFC 4302 Authentication Header
	ipsecProtoESP = "esp" // RFC 4303 Encapsulating Security Payload

	ipsecAuthSHA1   = "sha1"
	ipsecAuthSHA256 = "sha256"
	ipsecAuthSHA384 = "sha384"
	ipsecAuthSHA512 = "sha512"

	ipsecEncNull   = "null"
	ipsecEncAES128 = "aes128"
	ipsecEncAES256 = "aes256"

	// ipsecSPIMin is the lowest assignable SPI: RFC 4303 §2.1 reserves 0..255.
	ipsecSPIMin = 256
)

// ipsecAuthKeyLen is the required integrity key length in bytes per HMAC-SHA
// algorithm (the full HMAC key, RFC 2104 / RFC 4868).
var ipsecAuthKeyLen = map[string]int{
	ipsecAuthSHA1:   20,
	ipsecAuthSHA256: 32,
	ipsecAuthSHA384: 48,
	ipsecAuthSHA512: 64,
}

// ipsecEncKeyLen is the required ESP encryption key length in bytes per algorithm.
var ipsecEncKeyLen = map[string]int{
	ipsecEncAES128: 16,
	ipsecEncAES256: 32,
}

var (
	ErrRouterIDRequired   = errors.New("ospf: router-id is required or must be derivable from an IPv4 interface address")
	ErrUndeclaredArea     = errors.New("ospf: interface references undeclared area")
	ErrDuplicateArea      = errors.New("ospf: duplicate canonical area")
	ErrNonIPv4Range       = errors.New("ospf: area range prefix must be IPv4")
	ErrInvalidNSSARole    = errors.New("ospf: nssa translate-role must be candidate, always, or never")
	ErrESNRequiresHMAC    = errors.New("ospf: key-chain extended-sequence (AuType 3) requires an hmac-sha algorithm")
	ErrKeyIDTooWide       = errors.New("ospf: AuType 2 key-id must be 0..255 (the on-wire Key ID is one octet); use extended-sequence for 32-bit key-ids")
	ErrKeyLifetimeFormat  = errors.New("ospf: key send/accept lifetime start/end must be an RFC3339 timestamp")
	ErrKeyRolloverGap     = errors.New("ospf: key-chain send-lifetime rollover gap (a key's send start must be at or before the previous key's send end so signing coverage never lapses; RFC 5709 §X / RFC 7210)")
	ErrInterfaceCostZero  = errors.New("ospf: interface cost must be greater than 0 (RFC 2328 App C.3)")
	ErrTransmitDelayZero  = errors.New("ospf: interface transmit-delay must be greater than 0 (RFC 2328 App C.3 InfTransDelay)")
	ErrSimplePasswordLen  = errors.New("ospf: simple-password (AuType 1) secret must be at most 8 octets (RFC 2328 App D); use md5/hmac-sha for longer keys")
	ErrInstanceIDRange    = errors.New("ospf: address-family instance-id is outside its RFC 5838 §2.1 range (ipv6-unicast 0-31, ipv6-multicast 32-63, ipv4-unicast 64-95, ipv4-multicast 96-127)")
	ErrNBMANoNeighbors    = errors.New("ospf: nbma interface requires at least one nbma-neighbor (RFC 2328 App C.6: NBMA has no multicast fallback, so with no configured neighbor it unicasts to nobody and forms no adjacency); use point-to-multipoint for multicast neighbor discovery")
	ErrGraceIntervalRange = errors.New("ospf: graceful-restart restart-interval must be 1..1800 seconds (RFC 3623 sec 2.1 / App B.1: SHOULD NOT exceed LSRefreshTime)")
	// Virtual-link config-time rejections (RFC 2328 section 15 / RFC 5340 section 4.2).
	ErrVirtualLinkTransitMissing  = errors.New("ospf: virtual-link transit area is not declared under areas")
	ErrVirtualLinkTransitBackbone = errors.New("ospf: virtual-link transit area must not be the backbone 0.0.0.0 (RFC 2328 App C.4)")
	ErrVirtualLinkTransitStub     = errors.New("ospf: virtual-link transit area must not be a stub or NSSA area (RFC 2328 section 15 / RFC 5340 section 4.2)")
	ErrVirtualLinkNotABR          = errors.New("ospf: virtual-link requires this router to be an area border router (an interface in two or more areas; RFC 2328 section 15)")
	ErrVirtualLinkSelfRouterID    = errors.New("ospf: virtual-link remote-router-id must not equal this router's own router-id (RFC 2328 section 15)")
)

type areaType string

type networkType string

type authConfig struct {
	Mode     string
	KeyChain string
}

type timerConfig struct {
	SPFDelayMS      uint32
	SPFHoldMS       uint32
	SPFMaxHoldMS    uint32
	MinLSIntervalMS uint32
	MinLSArrivalMS  uint32
}

type defaultInformationConfig struct {
	Originate  bool
	Always     bool
	Metric     uint32
	MetricType string
}

type maxMetricConfig struct {
	RouterLSAAlways bool
	OnStartupSec    uint32 // RFC 6987 stub-router seconds after startup (0 = disabled)
	OnShutdownSec   uint32 // RFC 6987 stub-router seconds during graceful shutdown (0 = disabled)
}

// grSupport is the RFC 3623 Appendix B.1 RestartSupport level. It is a uint8 (not a string)
// so gracefulRestartConfig stays 8 bytes and does not grow the by-value ospfConfig snapshot
// past the gocritic hugeParam threshold.
type grSupport uint8

const (
	grSupportDisabled            grSupport = iota // never originate a Grace-LSA (default)
	grSupportPlanned                              // planned restart only
	grSupportPlannedAndUnplanned                  // planned + unplanned (RFC 3623 sec 5)
)

// parseGRSupport maps the YANG `support` enumeration token to a grSupport level; an
// unrecognized token (the YANG enumeration already rejects those) defaults to disabled.
func parseGRSupport(s string) grSupport {
	switch s {
	case "planned":
		return grSupportPlanned
	case "planned-and-unplanned":
		return grSupportPlannedAndUnplanned
	default:
		return grSupportDisabled
	}
}

// gracefulRestartConfig is the RFC 3623 / RFC 5187 Graceful Restart policy. It is family-
// neutral (a top-level `graceful-restart` container drives both address families, mirroring
// the RFC 7770 `router-information` precedent) because the restarter/helper control plane is
// shared across OSPFv2 and OSPFv3. present records whether the operator configured it so a
// top-level container inherits into the OSPFv3 sub-config only when the sub-config did not
// set its own. Kept at 8 bytes (uint32 + uint8 + 3 bool) to bound ospfConfig's by-value size.
type gracefulRestartConfig struct {
	// RestartInterval is the RFC 3623 Appendix B.1 grace period in seconds (1..1800,
	// default 120). It is the Grace Period TLV value neighbors honor. uint16 (max 65535)
	// covers the 1..1800 range and keeps the struct at 6 bytes so it packs into ospfConfig's
	// trailing padding (TestOspfConfigCopyBudget guards this).
	RestartInterval uint16
	// RestarterSupport is RFC 3623 Appendix B.1 RestartSupport (disabled / planned /
	// planned-and-unplanned). Default disabled: the router originates no Grace-LSA and
	// restarts exactly as a router without this feature.
	RestarterSupport grSupport
	// HelperEnabled is RFC 3623 Appendix B.2 RestartHelperSupport: whether this router acts
	// as a helper for a restarting neighbor. Default true (helping is low-risk, receive-side).
	HelperEnabled bool
	// StrictLSAChecking is RFC 3623 Appendix B.2 RestartHelperStrictLSAChecking (sec 3.2):
	// terminate helper mode on a changed LSA that would flood to the restarting router.
	// Default true.
	StrictLSAChecking bool
	present           bool
}

// RestarterEnabled reports whether the restarter originates Grace-LSAs at all (planned or
// planned-and-unplanned). Disabled is the default (RFC 3623 sec 4).
func (g gracefulRestartConfig) restarterEnabled() bool {
	return g.RestarterSupport == grSupportPlanned || g.RestarterSupport == grSupportPlannedAndUnplanned
}

// UnplannedEnabled reports whether the restarter may originate Grace-LSAs on an unplanned
// (cold) start (RFC 3623 sec 5). Off unless explicitly configured planned-and-unplanned.
func (g gracefulRestartConfig) unplannedEnabled() bool {
	return g.RestarterSupport == grSupportPlannedAndUnplanned
}

// routerInformationConfig is the RFC 7770 Router Information LSA advertisement policy: an
// enable flag and the flooding scope(s) at which the RI LSA is originated. present records
// whether the operator configured a `router-information` container (so a top-level
// container inherits into the OSPFv3 sub-config only when the sub-config did not set its
// own). When Enabled and no scope is listed, Scopes defaults to area + AS (RFC 7770 sec 2.7
// operator-selectable scope; the common Segment-Routing deployment needs both).
// fastRerouteConfig is the resolved RFC 5286 / TI-LFA fast-reroute policy. Bools
// only (3 bytes) so it packs into ospfConfig padding without growing the by-value
// copy budget (TestOspfConfigCopyBudget).
type fastRerouteConfig struct {
	present bool
	Enabled bool
	// TILFA selects "ti-lfa" mode (the SR repair-list fallback); false is base "lfa".
	TILFA bool
	// NodeProtection prefers node-protecting alternates (RFC 5286 Section 3.6).
	NodeProtection bool
}

type routerInformationConfig struct {
	present bool
	Enabled bool
	Scopes  []OpaqueScope
}

// HasScope reports whether the RI advertisement includes flooding scope s.
func (r routerInformationConfig) HasScope(s OpaqueScope) bool {
	return slices.Contains(r.Scopes, s)
}

type redistributeConfig struct {
	Source     string
	Metric     uint32
	MetricType string
	Tag        uint32
}

type rangeConfig struct {
	Prefix    netip.Prefix
	Advertise bool
	Cost      uint32
	HasCost   bool
}

type areaConfig struct {
	AreaID       types.AreaID
	AreaType     areaType
	NoSummary    bool
	DefaultCost  uint32
	AuthKeyChain string
	Ranges       []rangeConfig
	// NSSA-only (applied when AreaType is nssa).
	NSSATranslateRole     string // candidate | always | never
	NSSAStabilityInterval uint16 // seconds (RFC 3101 section 3.5 hysteresis)
	NSSADefaultOriginate  bool
}

// virtualLinkConfig is a configured OSPF virtual link (RFC 2328 section 15 / RFC 5340
// section 4.2): a logical unnumbered point-to-point link belonging to the backbone that
// runs THROUGH the (non-backbone, non-stub) TransitArea to the area-border router named by
// RemoteRouterID. Its output cost and the neighbor's reachable address are computed from
// the transit area's intra-area SPF -- never configured (RFC 2328 section 15 / RFC 5340
// section C.2), so only the point-to-point timers are tunable here.
type virtualLinkConfig struct {
	TransitArea        types.AreaID
	RemoteRouterID     types.RouterID
	HelloInterval      uint16
	DeadInterval       uint16
	RetransmitInterval uint16
	TransmitDelay      uint16
}

// interfaceConfig is copied by value in many `for _, ic := range ...Interfaces` loops, so
// its size is held under the gocritic rangeValCopy budget (see zz_size_test.go). Fields are
// grouped by alignment (8-byte, then 4-byte, then 2-byte, then 1-byte) so the scalar tail
// packs without interleaved padding and a new field does not silently inflate the copy cost;
// large/optional fields stay behind pointers (IPsec/TE/NBMA).
type interfaceConfig struct {
	Name           string
	NetworkType    networkType
	Authentication authConfig
	// InstanceIDs are the RFC 6549 OSPFv2 Instance IDs this interface is enrolled in
	// (the `instance-id` leaf-list). Empty means the base instance 0 only, which keeps a
	// config with no `instance-id` bit-for-bit identical to base OSPFv2. Listing several
	// values enrolls the one physical interface in several coexisting instances (§2/§3.1).
	InstanceIDs []uint8
	// IPsec is the RFC 4552 manual IPsec block. It is valid ONLY under the IPv6
	// address family (OSPFv3); the validator rejects it on an IPv4-family
	// interface and rejects it alongside a RFC 7166 (Authentication) key chain.
	// nil means no kernel IPsec is configured for the interface.
	IPsec *ipsecInterfaceConfig
	// TE is the per-interface traffic-engineering block (RFC 3630 / RFC 5392); nil means TE
	// is not configured on this interface (the default), so nothing is originated. A pointer
	// keeps interfaceConfig small (it is copied by many callers).
	TE *teConfig
	// NBMA holds the RFC 2328 App C.5/C.6 poll interval and static neighbor list for a
	// non-broadcast interface (network-type nbma, or the non-broadcast point-to-multipoint
	// variant); nil for the other network types. A pointer keeps interfaceConfig small (it
	// is copied by many callers), the same reason IPsec and TE are pointers.
	NBMA   *nbmaConfig
	AreaID types.AreaID
	// BFD carries the RFC 5880 / RFC 5881 single-hop failure-detection settings for this
	// interface. Disabled by default (opt-in). Shared by the IPv4 (OSPFv2) and the
	// address-family ipv6 (OSPFv3) interface lists via the one parseInterface path.
	BFD                bfdInterfaceConfig
	Cost               uint16
	HelloInterval      uint16
	DeadInterval       uint16
	RetransmitInterval uint16
	TransmitDelay      uint16
	// LDPSyncHoldDown is the RFC 5443 section 2 hold-down interval (seconds) after LDP
	// session establishment before the link is declared synchronized (the estimation for
	// "all label bindings exchanged"). The RFC defines no universal default.
	LDPSyncHoldDown  uint16
	Enabled          bool
	HasCost          bool
	Priority         uint8
	Passive          bool
	MTUIgnore        bool
	HasTransmitDelay bool
	// LDPSyncEnabled turns on RFC 5443 / RFC 6138 LDP-IGP synchronization for this
	// interface: hold the link at LSInfinity (P2P) or withhold the transit link
	// (broadcast) until LDP is synchronized. Cost is retained as the restore value.
	LDPSyncEnabled bool
}

// bfdInterfaceConfig is the per-interface BFD opt-in. Timers are stored in microseconds
// (the api.SessionRequest unit); the config leaves are milliseconds (parseInterfaceBFD
// multiplies by 1000). Multiplier is the RFC 5880 Detect Mult. Field order packs the two
// uint32 timers before the byte-sized fields so interfaceConfig, which embeds this by value
// and is ranged over in several hot config paths, stays under gocritic's rangeValCopy size.
type bfdInterfaceConfig struct {
	MinTxUs    uint32
	MinRxUs    uint32
	Multiplier uint8
	Enabled    bool
}

// nbmaConfig is the per-interface NBMA / non-broadcast point-to-multipoint policy.
type nbmaConfig struct {
	// PollInterval is the RFC 2328 App C.5 NBMA poll rate (seconds) for silent
	// configured neighbors.
	PollInterval uint16
	// Neighbors is the statically configured neighbor list (RFC 2328 App C.6). IPv4
	// keys by address; IPv6 keys by router-id with an optional link-local.
	Neighbors []nbmaNeighborConfig
}

// nbmaNeighborConfig is one statically configured NBMA neighbor. Address holds the
// IPv4 neighbor address (OSPFv2). RouterID and LinkLocal hold the OSPFv3 neighbor
// Router ID and its optional configured link-local. Priority 0 marks the neighbor
// ineligible for the DR/BDR election (RFC 2328 sec 9.4 step 6 still starts an
// adjacency once this router becomes DR/BDR).
type nbmaNeighborConfig struct {
	Address   netip.Addr
	RouterID  types.RouterID
	LinkLocal netip.Addr
	Priority  uint8
}

// pollInterval returns the interface's effective NBMA poll interval, or the default
// when no NBMA config or an unset value is present.
func (ic interfaceConfig) pollInterval() uint16 {
	if ic.NBMA != nil && ic.NBMA.PollInterval > 0 {
		return ic.NBMA.PollInterval
	}
	return DefaultPollInterval
}

// nbmaNeighborList returns the interface's configured NBMA neighbor list (nil when the
// interface has no NBMA config).
func (ic interfaceConfig) nbmaNeighborList() []nbmaNeighborConfig {
	if ic.NBMA == nil {
		return nil
	}
	return ic.NBMA.Neighbors
}

// inInstance reports whether this interface participates in OSPFv2 Instance ID id. An
// interface with no configured Instance IDs belongs to the base instance 0 only.
func (ic interfaceConfig) inInstance(id uint8) bool {
	if len(ic.InstanceIDs) == 0 {
		return id == 0
	}
	return slices.Contains(ic.InstanceIDs, id)
}

type lifetimeConfig struct {
	Start string
	End   string
}

type keyConfig struct {
	KeyID          uint32
	Algorithm      string
	Secret         string //nolint:gosec // G117: config field name, not a literal; masked via ze:sensitive in YANG and never logged
	SendLifetime   lifetimeConfig
	AcceptLifetime lifetimeConfig
}

type keyChainConfig struct {
	Name             string
	ExtendedSequence bool // RFC 7474 AuType 3 (extended 64-bit sequence) instead of AuType 2
	Keys             []keyConfig
}

type ospfConfig struct {
	present            bool
	RouterID           types.RouterID
	routerIDFromConfig bool
	ReferenceBandwidth uint32
	MaximumPaths       uint8
	// Opaque enables the RFC 5250 opaque-LSA capability: the router advertises the O-bit
	// in its Database Description packets and originates opaque LSAs for registered
	// consumers. Default false (opaque LSAs are still stored and reflooded by scope
	// regardless, but the router advertises no opaque capability and originates none).
	Opaque bool
	// TERouterAddress is the RFC 3630 sec 2.4.1 Router Address TLV value: a stable,
	// always-reachable IPv4 address (typically a loopback) advertised once per router in
	// its Router-Address TE LSA. When HasTERouterAddress is false, TE origination falls
	// back to the Router ID.
	TERouterAddress    [4]byte
	HasTERouterAddress bool
	DefaultInformation defaultInformationConfig
	Timers             timerConfig
	MaxMetric          maxMetricConfig
	// RouterInformation is the RFC 7770 Router Information LSA advertisement policy
	// (enable + flooding scope). Applies to both address families; a top-level container
	// inherits into the OSPFv3 sub-config unless that sub-config sets its own.
	RouterInformation routerInformationConfig
	// ExtendedPrefix / ExtendedLink gate origination of the RFC 7684 Extended Prefix
	// (Opaque Type 7) / Extended Link (Opaque Type 8) Opaque LSAs (spec-ospf-ext-4). Both
	// default false: the LSAs are containers a sub-TLV producer (Segment Routing) fills, so
	// they are off until such a producer needs them. Reception/decode of peers' Extended
	// Prefix/Link LSAs is always on once the plugin is built. Both require opaque=true.
	ExtendedPrefix bool
	ExtendedLink   bool
	// FastReroute is the RFC 5286 LFA / TI-LFA fast-reroute policy (spec-ospf-ext-6):
	// a per-instance enable, an lfa|ti-lfa mode, and a node-protection preference.
	// Family-neutral: it drives both the OSPFv2 and OSPFv3 SPF Computers via the AF
	// seam (SR repair labels are v4-only, OSPFv3 SR carriage being out of scope).
	FastReroute  fastRerouteConfig
	Redistribute []redistributeConfig
	Areas        []areaConfig
	Interfaces   []interfaceConfig
	// VirtualLinks are OSPF virtual links (RFC 2328 section 15 / RFC 5340 section 4.2)
	// configured on this address family. Each belongs to the backbone and transits a
	// non-backbone, non-stub area. The V6 family carries its own list.
	VirtualLinks []virtualLinkConfig
	KeyChains    []keyChainConfig
	// InstanceID is the OSPFv3 Instance ID (RFC 5340 sec 2.5); 0 for the IPv4 family.
	InstanceID uint8
	// GracefulRestart is the RFC 3623 / RFC 5187 Graceful Restart policy (restarter support
	// + interval, helper support + strict-LSA-checking). Family-neutral: a top-level
	// `graceful-restart` container drives both address families (inherited into V6 unless the
	// sub-config sets its own), because the restarter/helper control plane is shared. Placed
	// right after InstanceID so its 6 bytes pack into the padding before the 8-aligned V6
	// pointer, keeping ospfConfig at its by-value size budget (TestOspfConfigCopyBudget).
	GracefulRestart gracefulRestartConfig
	// V6 is the default IPv6-unicast (OSPFv3) address-family sub-config parsed from
	// `ospf { address-family { ipv6 | ipv6-unicast } { ... } }`; nil when the default v6
	// family is not configured. It carries its own areas and interfaces and inherits the
	// parent Router ID. A v6-codec engine instance consumes it.
	V6 *ospfConfig
	// V6Extra are the additional RFC 5838 address families (ipv6-multicast, ipv4-unicast,
	// ipv4-multicast), each a v6-codec engine instance keyed by its Instance-ID range.
	V6Extra []v6AFConfig
}

// v6AFConfig is one non-default OSPFv3 address family (RFC 5838): its declared address
// family plus the sub-config (Instance ID, areas, interfaces) a v6-codec engine consumes.
type v6AFConfig struct {
	af  addressFamily
	cfg ospfConfig
}

// v6Families returns every configured OSPFv3 address family uniformly: the default
// IPv6-unicast AF (from V6, if present) followed by the additional AFs. Register spawns one
// v6-codec engine per returned entry.
func (c ospfConfig) v6Families() []v6AFConfig {
	out := make([]v6AFConfig, 0, 1+len(c.V6Extra))
	if c.V6 != nil {
		af, ok := afFromInstanceID(c.V6.InstanceID)
		if !ok {
			af = afIPv6Unicast
		}
		out = append(out, v6AFConfig{af: af, cfg: *c.V6})
	}
	out = append(out, c.V6Extra...)
	return out
}

// multiAF reports whether the router runs more than one OSPFv3 address family, so the
// default IPv6-unicast instance emits the RFC 5838 AF-bit (§2.5).
func (c ospfConfig) multiAF() bool { return len(c.v6Families()) > 1 }

type configSection struct {
	Root string
	Data string
}

type routerIDSource interface {
	Interfaces() ([]iface.InterfaceInfo, error)
}

type systemRouterIDSource struct{}

func (systemRouterIDSource) Interfaces() ([]iface.InterfaceInfo, error) {
	return iface.ListInterfaces()
}

func defaultOSPFConfig() ospfConfig {
	return ospfConfig{
		ReferenceBandwidth: DefaultReferenceBandwidth,
		MaximumPaths:       DefaultMaximumPaths,
		DefaultInformation: defaultInformationConfig{Metric: DefaultDefaultMetric, MetricType: metricType2},
		Timers: timerConfig{
			SPFDelayMS:      DefaultSPFDelayMS,
			SPFHoldMS:       DefaultSPFHoldMS,
			SPFMaxHoldMS:    DefaultSPFMaxHoldMS,
			MinLSIntervalMS: DefaultMinLSIntervalMS,
			MinLSArrivalMS:  DefaultMinLSArrivalMS,
		},
	}
}

func (c ospfConfig) Present() bool { return c.present }

func (c ospfConfig) areaSet() map[types.AreaID]struct{} {
	areas := make(map[types.AreaID]struct{}, len(c.Areas))
	for _, a := range c.Areas {
		areas[a.AreaID] = struct{}{}
	}
	return areas
}

// enrolledInterfaces returns enabled interfaces that bind a declared area.
// Passive interfaces are included in area state, but activeInterfaces excludes
// them from raw-socket Hello processing.
func (c ospfConfig) enrolledInterfaces() []interfaceConfig {
	areas := c.areaSet()
	out := make([]interfaceConfig, 0, len(c.Interfaces))
	for _, ic := range c.Interfaces {
		if !ic.Enabled {
			continue
		}
		if _, ok := areas[ic.AreaID]; ok {
			out = append(out, ic)
		}
	}
	return out
}

// isAreaBorderRouter reports whether this router is an area border router: it has an
// enabled interface in two or more distinct areas (RFC 2328 section 3.3). A virtual link
// (which itself makes the endpoint backbone-attached at runtime, RFC 5340 section 3.5) may
// only be configured on an ABR, so this drives the config-time AC-3 rejection.
func (c ospfConfig) isAreaBorderRouter() bool {
	seen := make(map[types.AreaID]struct{}, len(c.Interfaces))
	for _, ic := range c.Interfaces {
		if ic.Enabled {
			seen[ic.AreaID] = struct{}{}
		}
	}
	return len(seen) >= 2
}

func (c ospfConfig) activeInterfaces() []interfaceConfig {
	enrolled := c.enrolledInterfaces()
	out := enrolled[:0]
	for _, ic := range enrolled {
		if !ic.Passive && ic.NetworkType != networkLoopback {
			out = append(out, ic)
		}
	}
	return out
}

// instanceIDSet returns the sorted, distinct set of RFC 6549 OSPFv2 Instance IDs the
// config demands, one full engine per element. The base instance 0 is always present so a
// single-instance config (no `instance-id` anywhere) yields exactly {0} (today's engine),
// and it also anchors redistribution / default-origination on the base IPv4 unicast table.
func (c ospfConfig) instanceIDSet() []uint8 {
	seen := map[uint8]struct{}{0: {}}
	for _, ic := range c.Interfaces {
		for _, id := range ic.InstanceIDs {
			seen[id] = struct{}{}
		}
	}
	out := make([]uint8, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

// forInstance returns a per-instance copy of the config for OSPFv2 Instance ID id: the
// shared globals (Router ID, areas, timers, redistribution, key-chains) are kept and the
// interface list is filtered to those enrolled in id, with InstanceID set so the engine's
// dispatcher demux and its transmit encoders adopt it. The V6 sub-config is dropped: the
// OSPFv3 family is a separate engine driven by cfg.V6, not an OSPFv2 instance.
func (c ospfConfig) forInstance(id uint8) ospfConfig {
	sub := c
	sub.InstanceID = id
	sub.V6 = nil
	sub.Interfaces = make([]interfaceConfig, 0, len(c.Interfaces))
	for _, ic := range c.Interfaces {
		if ic.inInstance(id) {
			sub.Interfaces = append(sub.Interfaces, ic)
		}
	}
	return sub
}

func parseOSPFConfig(sections []configSection, source routerIDSource) (ospfConfig, error) {
	cfg := defaultOSPFConfig()
	for _, s := range sections {
		if s.Root != "ospf" || s.Data == "" {
			continue
		}
		var wrapper map[string]any
		if err := json.Unmarshal([]byte(s.Data), &wrapper); err != nil {
			return cfg, fmt.Errorf("ospf: invalid config JSON: %w", err)
		}
		tree, _ := wrapper["ospf"].(map[string]any)
		if tree == nil {
			continue
		}
		cfg.present = true
		if err := applyTree(&cfg, tree); err != nil {
			return cfg, err
		}
	}
	if cfg.present && !cfg.routerIDFromConfig && source != nil {
		if rid, ok := deriveRouterID(source); ok {
			cfg.RouterID = rid
		}
	}
	// Each OSPFv3 address family inherits the (possibly derived) Router ID unless it set
	// its own (OSPFv3 still uses a 32-bit Router ID, RFC 5340 sec 2.11).
	if cfg.V6 != nil && !cfg.V6.routerIDFromConfig {
		cfg.V6.RouterID = cfg.RouterID
	}
	for i := range cfg.V6Extra {
		if !cfg.V6Extra[i].cfg.routerIDFromConfig {
			cfg.V6Extra[i].cfg.RouterID = cfg.RouterID
		}
	}
	// RFC 7770: the Router Information advertisement is a router-wide capability, so a
	// top-level `router-information` container drives both address families. The OSPFv3
	// sub-config inherits it unless it configured its own `router-information`.
	if cfg.V6 != nil && !cfg.V6.RouterInformation.present {
		cfg.V6.RouterInformation = cfg.RouterInformation
	}
	// RFC 3623 / RFC 5187: Graceful Restart is a router-wide policy shared by both address
	// families, so a top-level `graceful-restart` container drives the OSPFv3 family unless
	// that sub-config configured its own. Every RFC 5838 address family inherits it too.
	if cfg.GracefulRestart.present {
		if cfg.V6 != nil && !cfg.V6.GracefulRestart.present {
			cfg.V6.GracefulRestart = cfg.GracefulRestart
		}
		for i := range cfg.V6Extra {
			if !cfg.V6Extra[i].cfg.GracefulRestart.present {
				cfg.V6Extra[i].cfg.GracefulRestart = cfg.GracefulRestart
			}
		}
	}
	// RFC 5286 fast reroute is a router-wide policy: a top-level `fast-reroute`
	// container drives the OSPFv3 family too, unless the sub-config set its own
	// (SR repair labels stay v4-only via the AF seam).
	if cfg.FastReroute.present {
		if cfg.V6 != nil && !cfg.V6.FastReroute.present {
			cfg.V6.FastReroute = cfg.FastReroute
		}
		for i := range cfg.V6Extra {
			if !cfg.V6Extra[i].cfg.FastReroute.present {
				cfg.V6Extra[i].cfg.FastReroute = cfg.FastReroute
			}
		}
	}
	return cfg, nil
}

func applyTree(cfg *ospfConfig, tree map[string]any) error {
	if s := configString(tree["router-id"]); s != "" {
		rid, err := types.ParseRouterID(s)
		if err != nil {
			return fmt.Errorf("ospf: invalid router-id %q: %w", s, err)
		}
		cfg.RouterID = rid
		cfg.routerIDFromConfig = true
	}
	if v, ok := configUint32(tree["reference-bandwidth"]); ok && v > 0 {
		cfg.ReferenceBandwidth = v
	}
	if v, ok := configUint8(tree["maximum-paths"]); ok && v > 0 {
		cfg.MaximumPaths = v
	}
	cfg.Opaque = configBool(tree["opaque"], false)
	cfg.ExtendedPrefix = configBool(tree["extended-prefix"], false)
	cfg.ExtendedLink = configBool(tree["extended-link"], false)
	if s := configString(tree["router-address"]); s != "" {
		// RFC 3630 sec 2.4.1: the TE Router Address is a stable IPv4 address.
		addr, err := netip.ParseAddr(s)
		if err != nil || !addr.Is4() {
			return fmt.Errorf("%w: %q", ErrTERouterAddress, s)
		}
		cfg.TERouterAddress = addr.As4()
		cfg.HasTERouterAddress = true
	}
	if m, ok := tree["default-information"].(map[string]any); ok {
		cfg.DefaultInformation = parseDefaultInformation(m)
	}
	if m, ok := tree["max-metric"].(map[string]any); ok {
		cfg.MaxMetric = parseMaxMetric(m)
	}
	if m, ok := tree["router-information"].(map[string]any); ok {
		cfg.RouterInformation = parseRouterInformation(m)
	}
	if m, ok := tree["graceful-restart"].(map[string]any); ok {
		cfg.GracefulRestart = parseGracefulRestart(m)
	}
	if m, ok := tree["fast-reroute"].(map[string]any); ok {
		cfg.FastReroute = parseFastReroute(m)
	}
	if m, ok := tree["timers"].(map[string]any); ok {
		cfg.Timers = parseTimers(m)
	}
	for _, entry := range keyedList(tree["redistribute"], false) {
		cfg.Redistribute = append(cfg.Redistribute, parseRedistribute(entry))
	}
	if areas, ok := tree["areas"].(map[string]any); ok {
		for _, entry := range keyedList(areas["area"], false) {
			area, err := parseArea(entry)
			if err != nil {
				return err
			}
			cfg.Areas = append(cfg.Areas, area)
			// RFC 2328 section 15: a virtual link is configured on its TRANSIT area (the
			// parent area here). Flatten each nested virtual-link entry onto cfg.VirtualLinks
			// with TransitArea set to the enclosing area so the engine has one flat list.
			vls, err := parseVirtualLinks(area.AreaID, entry.data)
			if err != nil {
				return err
			}
			cfg.VirtualLinks = append(cfg.VirtualLinks, vls...)
		}
	}
	if interfaces, ok := tree["interfaces"].(map[string]any); ok {
		for _, entry := range keyedList(interfaces["interface"], false) {
			ic, err := parseInterface(entry)
			if err != nil {
				return err
			}
			cfg.Interfaces = append(cfg.Interfaces, ic)
		}
	}
	for _, entry := range keyedList(tree["key-chains"], false) {
		cfg.KeyChains = append(cfg.KeyChains, parseKeyChain(entry))
	}
	if v, ok := configUint8(tree["instance-id"]); ok {
		cfg.InstanceID = v
	}
	// RFC 5838: each OSPFv3 address family carries its own areas/interfaces/instance-id
	// under `address-family { <af> { ... } }`. Each parses into a sub-config a v6-codec
	// engine consumes; the sub reuses the area/interface shape and inherits the parent
	// Router ID (set in parseOSPFConfig after derivation). The default IPv6-unicast AF
	// (spelled `ipv6` or `ipv6-unicast`) lands in V6; the others in V6Extra.
	if afTree, ok := tree["address-family"].(map[string]any); ok {
		if err := applyAddressFamilies(cfg, afTree); err != nil {
			return err
		}
	}
	return nil
}

// applyAddressFamilies parses each configured OSPFv3 address-family container into a
// sub-config (RFC 5838). Unknown container names are ignored (the YANG enumeration rejects
// them earlier); the extra AFs are sorted by AF so the engine spawn order is deterministic.
func applyAddressFamilies(cfg *ospfConfig, afTree map[string]any) error {
	names := make([]string, 0, len(afTree))
	for name := range afTree {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		sub, ok := afTree[name].(map[string]any)
		if !ok {
			continue
		}
		af, known := afFromName(name)
		if !known {
			continue
		}
		child := defaultOSPFConfig()
		child.present = true
		if err := applyTree(&child, sub); err != nil {
			return err
		}
		if af.isDefault() {
			cfg.V6 = &child
		} else {
			cfg.V6Extra = append(cfg.V6Extra, v6AFConfig{af: af, cfg: child})
		}
	}
	return nil
}

// validNSSATranslateRole reports whether s is one of the RFC 3101 translator roles.
func validNSSATranslateRole(s string) bool {
	switch s {
	case translateRoleCandidate, translateRoleAlways, translateRoleNever:
		return true
	default:
		return false
	}
}

func validateConfig(cfg ospfConfig) error { return validateConfigAF(cfg, false) }

// validateConfigAF validates one address family. isV6 is true for the IPv6
// (OSPFv3) family, which alone may carry a RFC 4552 IPsec block.
func validateConfigAF(cfg ospfConfig, isV6 bool) error {
	if !cfg.present {
		return nil
	}
	if cfg.RouterID == (types.RouterID{}) {
		return ErrRouterIDRequired
	}
	// RFC 3623 sec 2.1 / Appendix B.1: the grace period (RestartInterval) must be 1..1800 s
	// (SHOULD NOT exceed LSRefreshTime, R-12). The YANG `range "1..1800"` is the primary
	// gate; this defends the non-YANG parse paths (doctor / offline verifier).
	if cfg.GracefulRestart.present {
		if iv := cfg.GracefulRestart.RestartInterval; iv < 1 || iv > 1800 {
			return fmt.Errorf("%w: %d", ErrGraceIntervalRange, iv)
		}
	}
	areas := make(map[types.AreaID]struct{}, len(cfg.Areas))
	for _, a := range cfg.Areas {
		if _, dup := areas[a.AreaID]; dup {
			return fmt.Errorf("%w: %s", ErrDuplicateArea, a.AreaID.String())
		}
		if !validNSSATranslateRole(a.NSSATranslateRole) {
			return fmt.Errorf("%w: %q (area %s)", ErrInvalidNSSARole, a.NSSATranslateRole, a.AreaID.String())
		}
		areas[a.AreaID] = struct{}{}
	}
	for _, ic := range cfg.Interfaces {
		if _, ok := areas[ic.AreaID]; !ok {
			return fmt.Errorf("%w: interface %q area %s", ErrUndeclaredArea, ic.Name, ic.AreaID.String())
		}
		// RFC 2328 Appendix C.3: the interface output cost MUST be greater than 0.
		if ic.HasCost && ic.Cost == 0 {
			return fmt.Errorf("%w: interface %q", ErrInterfaceCostZero, ic.Name)
		}
		// RFC 2328 Appendix C.3: InfTransDelay MUST be greater than 0.
		if ic.HasTransmitDelay && ic.TransmitDelay == 0 {
			return fmt.Errorf("%w: interface %q", ErrTransmitDelayZero, ic.Name)
		}
		// RFC 2328 Appendix C.6: an NBMA interface has no all-routers multicast, so it can
		// only reach the statically configured neighbor list. With an empty list it unicasts
		// to nobody and silently forms no adjacency. Reject it at config time. (An empty list
		// is valid for point-to-multipoint, which discovers neighbors via multicast Hellos.)
		if ic.NetworkType == networkNBMA && len(ic.nbmaNeighborList()) == 0 {
			return fmt.Errorf("%w: interface %q", ErrNBMANoNeighbors, ic.Name)
		}
		// RFC 3630 / RFC 5392: the traffic-engineering inter-as cross-field requirements.
		if err := validateTEInterface(ic); err != nil {
			return err
		}
		// RFC 4552 is an IPv6-family (OSPFv3) feature only (§5); an IPsec block on an
		// IPv4-family (OSPFv2) interface is a config error.
		if ic.IPsec != nil {
			if !isV6 {
				return fmt.Errorf("%w: interface %q", ErrIPsecIPv4Family, ic.Name)
			}
			if err := validateIPsecInterface(ic); err != nil {
				return err
			}
		}
	}
	if err := validateVirtualLinks(cfg); err != nil {
		return err
	}
	for _, kc := range cfg.KeyChains {
		for _, k := range kc.Keys {
			if kc.ExtendedSequence {
				// RFC 7474 AuType 3 only defines the HMAC-SHA algorithms.
				if !isHMACSHA(k.Algorithm) {
					return fmt.Errorf("%w: key-chain %q key %d uses %q", ErrESNRequiresHMAC, kc.Name, k.KeyID, k.Algorithm)
				}
				continue
			}
			// RFC 2328 App D / RFC 5709 AuType 2: the on-wire Key ID is a single octet, so a
			// crypto key-id above 255 cannot be represented and would truncate silently.
			// AuType 1 (simple) carries no Key ID on the wire, so it is unconstrained.
			if k.Algorithm != "simple" && k.KeyID > 255 {
				return fmt.Errorf("%w: key-chain %q key %d", ErrKeyIDTooWide, kc.Name, k.KeyID)
			}
			// RFC 2328 App D: the AuType 1 (Simple Password) authentication field is exactly 8
			// octets, so a longer simple-password secret cannot be carried on the wire and would
			// be silently truncated. Reject it at config time instead of truncating.
			if k.Algorithm == "simple" && len(decodeSecret(k.Secret)) > 8 {
				return fmt.Errorf("%w: key-chain %q key %d", ErrSimplePasswordLen, kc.Name, k.KeyID)
			}
		}
		if err := validateKeyRollover(kc); err != nil {
			return err
		}
	}
	if cfg.V6 != nil {
		// RFC 5838 §2.1: the default AF's Instance ID must fall in the IPv6-unicast range.
		if !afInstanceIDInRange(afIPv6Unicast, cfg.V6.InstanceID) {
			return fmt.Errorf("%w: ipv6-unicast instance-id %d", ErrInstanceIDRange, cfg.V6.InstanceID)
		}
		if err := validateConfigAF(*cfg.V6, true); err != nil {
			return fmt.Errorf("address-family ipv6-unicast: %w", err)
		}
	}
	for i := range cfg.V6Extra {
		ex := &cfg.V6Extra[i]
		// RFC 5838 §2.1: each AF's Instance ID must fall in that AF's range (and <= 127).
		if !afInstanceIDInRange(ex.af, ex.cfg.InstanceID) {
			return fmt.Errorf("%w: %s instance-id %d", ErrInstanceIDRange, ex.af, ex.cfg.InstanceID)
		}
		if err := validateConfig(ex.cfg); err != nil {
			return fmt.Errorf("address-family %s: %w", ex.af, err)
		}
	}
	return nil
}

// validateVirtualLinks enforces the config-time virtual-link rules of RFC 2328 section 15
// and RFC 5340 section 4.2: the transit area must be declared, must not be the backbone,
// and must not be a stub or NSSA area; this router must be an area border router; and the
// remote endpoint's Router ID must not be this router's own. These need the whole config
// (sibling areas + interfaces), so they live here rather than in a per-leaf YANG validator.
func validateVirtualLinks(cfg ospfConfig) error {
	if len(cfg.VirtualLinks) == 0 {
		return nil
	}
	areaTypes := make(map[types.AreaID]areaType, len(cfg.Areas))
	for _, a := range cfg.Areas {
		areaTypes[a.AreaID] = a.AreaType
	}
	abr := cfg.isAreaBorderRouter()
	for _, vl := range cfg.VirtualLinks {
		if vl.TransitArea == types.BackboneArea {
			return fmt.Errorf("%w: neighbor %s", ErrVirtualLinkTransitBackbone, vl.RemoteRouterID.String())
		}
		at, ok := areaTypes[vl.TransitArea]
		if !ok {
			return fmt.Errorf("%w: %s (neighbor %s)", ErrVirtualLinkTransitMissing, vl.TransitArea.String(), vl.RemoteRouterID.String())
		}
		if at == areaTypeStub || at == areaTypeNSSA {
			return fmt.Errorf("%w: area %s", ErrVirtualLinkTransitStub, vl.TransitArea.String())
		}
		if !abr {
			return fmt.Errorf("%w: neighbor %s", ErrVirtualLinkNotABR, vl.RemoteRouterID.String())
		}
		if vl.RemoteRouterID == cfg.RouterID {
			return fmt.Errorf("%w: %s", ErrVirtualLinkSelfRouterID, vl.RemoteRouterID.String())
		}
	}
	return nil
}

// validateKeyRollover checks the send-lifetime windows of a key chain in start order:
// a malformed timestamp is rejected, and a gap where one key's send window ends
// strictly before the next key's send window begins is rejected (RFC 5709 §X / RFC 7210
// require overlapping send lifetimes so signing coverage never lapses). Keys without a
// send-lifetime (unbounded) never create a gap.
func validateKeyRollover(kc keyChainConfig) error {
	type window struct {
		keyID      uint32
		start, end time.Time
	}
	windows := make([]window, 0, len(kc.Keys))
	for _, k := range kc.Keys {
		start, end, ok := lifetimeBounds(k.SendLifetime)
		if !ok {
			return fmt.Errorf("%w: key-chain %q key %d send-lifetime", ErrKeyLifetimeFormat, kc.Name, k.KeyID)
		}
		if _, _, ok := lifetimeBounds(k.AcceptLifetime); !ok {
			return fmt.Errorf("%w: key-chain %q key %d accept-lifetime", ErrKeyLifetimeFormat, kc.Name, k.KeyID)
		}
		windows = append(windows, window{keyID: k.KeyID, start: start, end: end})
	}
	// Order by send-start (zero start sorts first, i.e. earliest); equal starts keep config order.
	sort.SliceStable(windows, func(i, j int) bool { return windows[i].start.Before(windows[j].start) })
	for i := 1; i < len(windows); i++ {
		prev, cur := windows[i-1], windows[i]
		// An unbounded previous end (zero) never lapses; an unbounded current start (zero)
		// begins before any finite end, so it cannot open a gap.
		if prev.end.IsZero() || cur.start.IsZero() {
			continue
		}
		if cur.start.After(prev.end) {
			return fmt.Errorf("%w: key-chain %q key %d send starts %s after key %d send ends %s",
				ErrKeyRolloverGap, kc.Name, cur.keyID, cur.start.Format(time.RFC3339), prev.keyID, prev.end.Format(time.RFC3339))
		}
	}
	return nil
}

// isHMACSHA reports whether algo is one of the RFC 5709 HMAC-SHA algorithms (the only
// ones valid under RFC 7474 extended sequence numbers).
func isHMACSHA(algo string) bool {
	switch algo {
	case "hmac-sha-1", "hmac-sha-256", "hmac-sha-384", "hmac-sha-512":
		return true
	default:
		return false
	}
}

func parseDefaultInformation(m map[string]any) defaultInformationConfig {
	cfg := defaultInformationConfig{Metric: DefaultDefaultMetric, MetricType: metricType2}
	cfg.Originate = configBool(m["originate"], false)
	cfg.Always = configBool(m["always"], false)
	if v, ok := configUint32(m["metric"]); ok {
		cfg.Metric = v
	}
	if s := configString(m["metric-type"]); s != "" {
		cfg.MetricType = s
	}
	return cfg
}

func parseMaxMetric(m map[string]any) maxMetricConfig {
	cfg := maxMetricConfig{}
	if routerLSA, ok := m["router-lsa"].(map[string]any); ok {
		cfg.RouterLSAAlways = configBool(routerLSA["always"], false)
		if v, ok := configUint32(routerLSA["on-startup"]); ok {
			cfg.OnStartupSec = v
		}
		if v, ok := configUint32(routerLSA["on-shutdown"]); ok {
			cfg.OnShutdownSec = v
		}
	}
	return cfg
}

// DefaultRestartInterval is the RFC 3623 Appendix B.1 suggested default grace period (seconds).
const DefaultRestartInterval = 120

// parseGracefulRestart resolves the RFC 3623 / RFC 5187 `graceful-restart` container: a
// `restarter` sub-container (support enum + restart-interval seconds) and a `helper`
// sub-container (support boolean + strict-lsa-checking boolean). Defaults follow RFC 3623
// Appendix B: restarter support disabled, interval 120 s, helper enabled, strict checking on.
func parseGracefulRestart(m map[string]any) gracefulRestartConfig {
	cfg := gracefulRestartConfig{
		present:           true,
		RestartInterval:   DefaultRestartInterval,
		RestarterSupport:  grSupportDisabled,
		HelperEnabled:     true,
		StrictLSAChecking: true,
	}
	if r, ok := m["restarter"].(map[string]any); ok {
		if s := configString(r["support"]); s != "" {
			cfg.RestarterSupport = parseGRSupport(s)
		}
		if v, ok := configUint16(r["restart-interval"]); ok && v > 0 {
			cfg.RestartInterval = v
		}
	}
	if h, ok := m["helper"].(map[string]any); ok {
		cfg.HelperEnabled = configBool(h["support"], true)
		cfg.StrictLSAChecking = configBool(h["strict-lsa-checking"], true)
	}
	return cfg
}

// parseFastReroute resolves the RFC 5286 / TI-LFA `fast-reroute` container: an
// `enable` flag, a `mode` enum (lfa|ti-lfa, default lfa) and a `node-protection`
// preference. The mode enumeration is constrained by YANG; an unrecognized token
// degrades to base lfa.
func parseFastReroute(m map[string]any) fastRerouteConfig {
	cfg := fastRerouteConfig{
		present:        true,
		Enabled:        configBool(m["enable"], false),
		NodeProtection: configBool(m["node-protection"], true),
	}
	if configString(m["mode"]) == "ti-lfa" {
		cfg.TILFA = true
	}
	return cfg
}

// parseRouterInformation resolves the RFC 7770 `router-information` container: an `enabled`
// flag (default false) and a `scope` leaf-list of link|area|as. When enabled with no scope
// listed, the scope defaults to area + AS (RFC 7770 sec 2.7; the common Segment-Routing
// deployment needs both). Unrecognized scope tokens are ignored (the YANG enumeration
// constrains them; this defends the non-YANG doctor/verifier parse paths).
func parseRouterInformation(m map[string]any) routerInformationConfig {
	cfg := routerInformationConfig{present: true}
	cfg.Enabled = configBool(m["enabled"], false)
	if list, ok := m["scope"].([]any); ok {
		for _, item := range list {
			if s, ok := item.(string); ok {
				if sc, ok := routerInfoScope(s); ok && !cfg.HasScope(sc) {
					cfg.Scopes = append(cfg.Scopes, sc)
				}
			}
		}
	}
	if cfg.Enabled && len(cfg.Scopes) == 0 {
		cfg.Scopes = []OpaqueScope{OpaqueScopeArea, OpaqueScopeAS}
	}
	return cfg
}

// routerInfoScope maps a YANG `scope` enumeration token to its RFC 5250 flooding scope,
// reusing OpaqueScope.String() so the token spellings live in exactly one place.
func routerInfoScope(s string) (OpaqueScope, bool) {
	for _, sc := range []OpaqueScope{OpaqueScopeLink, OpaqueScopeArea, OpaqueScopeAS} {
		if sc.String() == s {
			return sc, true
		}
	}
	return 0, false
}

func parseTimers(m map[string]any) timerConfig {
	t := defaultOSPFConfig().Timers
	if v, ok := configUint32(m["spf-delay-ms"]); ok {
		t.SPFDelayMS = v
	}
	if v, ok := configUint32(m["spf-hold-ms"]); ok {
		t.SPFHoldMS = v
	}
	if v, ok := configUint32(m["spf-max-hold-ms"]); ok {
		t.SPFMaxHoldMS = v
	}
	if v, ok := configUint32(m["min-ls-interval-ms"]); ok {
		t.MinLSIntervalMS = v
	}
	if v, ok := configUint32(m["min-ls-arrival-ms"]); ok {
		t.MinLSArrivalMS = v
	}
	return t
}

func parseRedistribute(entry listEntry) redistributeConfig {
	r := redistributeConfig{Source: entry.key, Metric: DefaultExternalMetric, MetricType: metricType2}
	if s := configString(entry.data["source"]); s != "" {
		r.Source = s
	}
	if v, ok := configUint32(entry.data["metric"]); ok {
		r.Metric = v
	}
	if s := configString(entry.data["metric-type"]); s != "" {
		r.MetricType = s
	}
	if v, ok := configUint32(entry.data["tag"]); ok {
		r.Tag = v
	}
	return r
}

func parseArea(entry listEntry) (areaConfig, error) {
	idText := entry.key
	if s := configString(entry.data["area-id"]); s != "" {
		idText = s
	}
	id, err := types.ParseAreaID(idText)
	if err != nil {
		return areaConfig{}, fmt.Errorf("ospf: invalid area-id %q: %w", idText, err)
	}
	a := areaConfig{
		AreaID:                id,
		AreaType:              areaTypeNormal,
		DefaultCost:           DefaultAreaCost,
		NSSATranslateRole:     translateRoleCandidate,
		NSSAStabilityInterval: DefaultNSSAStabilityInterval,
	}
	if s := configString(entry.data["area-type"]); s != "" {
		// Validate against the YANG enum instead of silently coercing an unrecognized value
		// (which fell through to normal). Defends the non-YANG doctor/verifier parse paths.
		switch s {
		case areaTypeNormal, areaTypeStub, areaTypeNSSA:
			a.AreaType = areaType(s)
		default:
			return areaConfig{}, fmt.Errorf("ospf: area %s invalid area-type %q (want normal|stub|nssa)", id, s)
		}
	}
	a.NoSummary = configBool(entry.data["no-summary"], false)
	if v, ok := configUint32(entry.data["default-cost"]); ok {
		a.DefaultCost = v
	}
	if nssa, ok := entry.data["nssa"].(map[string]any); ok {
		if s := configString(nssa["translate-role"]); s != "" {
			a.NSSATranslateRole = s
		}
		if v, ok := configUint16(nssa["stability-interval"]); ok {
			a.NSSAStabilityInterval = v
		}
		a.NSSADefaultOriginate = configBool(nssa["default-originate"], false)
	}
	if auth, ok := entry.data["authentication"].(map[string]any); ok {
		a.AuthKeyChain = configString(auth["key-chain"])
	}
	if ranges, ok := entry.data["ranges"].(map[string]any); ok {
		for _, rangeEntry := range keyedList(ranges["range"], false) {
			r, err := parseRange(rangeEntry)
			if err != nil {
				return areaConfig{}, err
			}
			a.Ranges = append(a.Ranges, r)
		}
	}
	return a, nil
}

func parseRange(entry listEntry) (rangeConfig, error) {
	prefixText := entry.key
	if s := configString(entry.data["prefix"]); s != "" {
		prefixText = s
	}
	pfx, err := netip.ParsePrefix(prefixText)
	if err != nil {
		return rangeConfig{}, fmt.Errorf("ospf: invalid area range prefix %q: %w", prefixText, err)
	}
	if !pfx.Addr().Is4() {
		return rangeConfig{}, fmt.Errorf("%w: %s", ErrNonIPv4Range, prefixText)
	}
	r := rangeConfig{Prefix: pfx, Advertise: true}
	if s := configString(entry.data["advertise"]); s == rangeNotAdvertise {
		r.Advertise = false
	}
	if v, ok := configUint32(entry.data["cost"]); ok {
		r.Cost = v
		r.HasCost = true
	}
	return r, nil
}

// parseVirtualLinks resolves the `virtual-link` list nested under one area (the transit
// area) into virtualLinkConfig entries. It is called once per area during applyTree.
func parseVirtualLinks(transit types.AreaID, areaData map[string]any) ([]virtualLinkConfig, error) {
	entries := keyedList(areaData["virtual-link"], false)
	if len(entries) == 0 {
		return nil, nil
	}
	out := make([]virtualLinkConfig, 0, len(entries))
	for _, entry := range entries {
		vl, err := parseVirtualLink(transit, entry)
		if err != nil {
			return nil, err
		}
		out = append(out, vl)
	}
	return out, nil
}

func parseVirtualLink(transit types.AreaID, entry listEntry) (virtualLinkConfig, error) {
	ridText := entry.key
	if s := configString(entry.data["remote-router-id"]); s != "" {
		ridText = s
	}
	rid, err := types.ParseRouterID(ridText)
	if err != nil {
		return virtualLinkConfig{}, fmt.Errorf("ospf: area %s virtual-link invalid remote-router-id %q: %w", transit, ridText, err)
	}
	// RFC 2328 App C.4 / RFC 5340 App C.2: the configurable parameters are the transit area,
	// the neighbor Router ID, and the point-to-point timers (the cost is NOT configured).
	vl := virtualLinkConfig{
		TransitArea:        transit,
		RemoteRouterID:     rid,
		HelloInterval:      DefaultHelloInterval,
		DeadInterval:       DefaultDeadInterval,
		RetransmitInterval: DefaultRetransmitInterval,
		TransmitDelay:      DefaultTransmitDelay,
	}
	if v, ok := configUint16(entry.data["hello-interval"]); ok && v > 0 {
		vl.HelloInterval = v
	}
	if v, ok := configUint16(entry.data["dead-interval"]); ok && v > 0 {
		vl.DeadInterval = v
	}
	if v, ok := configUint16(entry.data["retransmit-interval"]); ok && v > 0 {
		vl.RetransmitInterval = v
	}
	if v, ok := configUint16(entry.data["transmit-delay"]); ok && v > 0 {
		vl.TransmitDelay = v
	}
	return vl, nil
}

func parseInterface(entry listEntry) (interfaceConfig, error) {
	m := entry.data
	ic := interfaceConfig{
		Name:               entry.key,
		Enabled:            configBool(m["enabled"], true),
		NetworkType:        networkBroadcast,
		HelloInterval:      DefaultHelloInterval,
		DeadInterval:       DefaultDeadInterval,
		Priority:           DefaultPriority,
		RetransmitInterval: DefaultRetransmitInterval,
		TransmitDelay:      DefaultTransmitDelay,
		Authentication:     authConfig{Mode: authModeInherit},
	}
	if s := configString(m["name"]); s != "" {
		ic.Name = s
	}
	areaText := configString(m["area"])
	if areaText == "" {
		return ic, fmt.Errorf("ospf: interface %q missing area", ic.Name)
	}
	areaID, err := types.ParseAreaID(areaText)
	if err != nil {
		return ic, fmt.Errorf("ospf: interface %q invalid area %q: %w", ic.Name, areaText, err)
	}
	ic.AreaID = areaID
	if s := configString(m["network-type"]); s != "" {
		// Validate against the YANG enum instead of silently coercing an unrecognized value
		// (which fell through to broadcast). Defends the non-YANG doctor/verifier parse paths.
		// The IPv6 leaf has no loopback enum, but the shared resolver accepts it here (the
		// YANG enum is the authoritative per-family gate); an IPv6 loopback never reaches an
		// interface because the v6 schema does not offer it.
		switch s {
		case networkBroadcast, networkPointToPoint, networkLoopback, networkNBMA, networkPointToMultipoint:
			ic.NetworkType = networkType(s)
		default:
			return ic, fmt.Errorf("ospf: interface %q invalid network-type %q (want broadcast|point-to-point|nbma|point-to-multipoint|loopback)", ic.Name, s)
		}
	}
	// RFC 2328 App C.5/C.6: the NBMA poll interval and static neighbor list live behind a
	// pointer (nil for the other network types) so interfaceConfig stays small. An nbma or
	// point-to-multipoint interface always carries the block (poll interval defaulted);
	// other types carry it only when the operator configured a poll interval or neighbor.
	poll := DefaultPollInterval
	pollSet := false
	if v, ok := configUint16(m["poll-interval"]); ok && v > 0 {
		poll = v
		pollSet = true
	}
	neighbors, err := parseNBMANeighbors(ic.Name, m["nbma-neighbor"])
	if err != nil {
		return ic, err
	}
	if ic.NetworkType == networkNBMA || ic.NetworkType == networkPointToMultipoint || pollSet || len(neighbors) > 0 {
		ic.NBMA = &nbmaConfig{PollInterval: poll, Neighbors: neighbors}
	}
	if v, ok := configNumber(m["cost"]); ok {
		// The interface cost is a 16-bit field (YANG range 1..65535); reject an out-of-range
		// value rather than silently truncating it via uint16 (e.g. 65536 -> 0).
		if v > 65535 {
			return ic, fmt.Errorf("ospf: interface %q cost %d out of range (1-65535)", ic.Name, v)
		}
		ic.Cost = uint16(v)
		ic.HasCost = true
	}
	if v, ok := configUint16(m["hello-interval"]); ok && v > 0 {
		ic.HelloInterval = v
	}
	if v, ok := configUint16(m["dead-interval"]); ok && v > 0 {
		ic.DeadInterval = v
	}
	if v, ok := configUint8(m["priority"]); ok {
		ic.Priority = v
	}
	ic.Passive = configBool(m["passive"], false)
	ic.MTUIgnore = configBool(m["mtu-ignore"], false)
	if v, ok := configUint16(m["retransmit-interval"]); ok && v > 0 {
		ic.RetransmitInterval = v
	}
	if v, ok := configUint16(m["transmit-delay"]); ok {
		ic.TransmitDelay = v
		ic.HasTransmitDelay = true
	}
	if auth, ok := m["authentication"].(map[string]any); ok {
		ic.Authentication = parseAuth(auth, authModeInherit)
	}
	// RFC 5443 / RFC 6138 LDP-IGP synchronization (per-interface, local mechanism).
	if ls, ok := m["ldp-sync"].(map[string]any); ok {
		ic.LDPSyncEnabled = configBool(ls["enable"], false)
		if v, ok := configNumber(ls["holddown"]); ok {
			// holddown is a 16-bit seconds field (YANG range 0..65535); reject an
			// out-of-range value rather than silently truncating via uint16.
			if v > 65535 {
				return ic, fmt.Errorf("ospf: interface %q ldp-sync holddown %d out of range (0-65535)", ic.Name, v)
			}
			ic.LDPSyncHoldDown = uint16(v)
		}
	}
	// RFC 4552 IPsec block (validated by validateIPsecInterface, which also enforces
	// the IPv6-family-only rule -- see config_ipsec.go).
	if ipsecTree, ok := m["ipsec"].(map[string]any); ok {
		ic.IPsec = parseIPsec(ipsecTree)
	}
	if te, ok := m["traffic-engineering"].(map[string]any); ok {
		teCfg, err := parseTE(te, ic.Name)
		if err != nil {
			return ic, err
		}
		ic.TE = &teCfg
	}
	// RFC 6549 §3: the per-interface Interface Instance ID(s). A leaf-list so one physical
	// interface can host several coexisting OSPFv2 instances; absent means the base
	// instance 0 only. The YANG range (0..255) plus the uint8 truncation keep every value
	// in range; a value above 255 is rejected rather than silently wrapped.
	ids, err := configInstanceIDs(m["instance-id"])
	if err != nil {
		return ic, fmt.Errorf("ospf: interface %q %w", ic.Name, err)
	}
	ic.InstanceIDs = ids
	bfd, err := parseInterfaceBFD(m["bfd"], ic.Name)
	if err != nil {
		return ic, err
	}
	ic.BFD = bfd
	return ic, nil
}

// parseInterfaceBFD reads the nested `bfd` container on an OSPF interface (RFC 5880 /
// RFC 5881). Called from parseInterface, so it serves BOTH the IPv4 interface list and the
// address-family ipv6 interface list from one code path. Timers are milliseconds in config,
// stored as microseconds. Defaults apply only when the leaf is omitted; an explicit 0 (or an
// out-of-range value) is rejected (AC-14) so an unusable session is never requested.
func parseInterfaceBFD(v any, ifName string) (bfdInterfaceConfig, error) {
	cfg := bfdInterfaceConfig{
		MinTxUs:    DefaultBFDMinTxUs,
		MinRxUs:    DefaultBFDMinRxUs,
		Multiplier: DefaultBFDMultiplier,
	}
	m, ok := v.(map[string]any)
	if !ok {
		return cfg, nil
	}
	cfg.Enabled = configBool(m["enabled"], false)
	if raw, ok := configNumber(m["min-tx"]); ok {
		if raw == 0 || raw > bfdMaxIntervalMs {
			return cfg, fmt.Errorf("ospf: interface %q bfd min-tx %d out of range (1..%d ms)", ifName, raw, bfdMaxIntervalMs)
		}
		cfg.MinTxUs = uint32(raw) * 1000
	}
	if raw, ok := configNumber(m["min-rx"]); ok {
		if raw == 0 || raw > bfdMaxIntervalMs {
			return cfg, fmt.Errorf("ospf: interface %q bfd min-rx %d out of range (1..%d ms)", ifName, raw, bfdMaxIntervalMs)
		}
		cfg.MinRxUs = uint32(raw) * 1000
	}
	if raw, ok := configNumber(m["multiplier"]); ok {
		if raw == 0 || raw > 255 {
			return cfg, fmt.Errorf("ospf: interface %q bfd multiplier %d out of range (1..255)", ifName, raw)
		}
		cfg.Multiplier = uint8(raw)
	}
	return cfg, nil
}

var errInstanceIDRange = errors.New("instance-id must be 0..255 (RFC 6549 8-bit Instance ID)")

// configInstanceIDs coerces the `instance-id` leaf-list into a sorted, de-duplicated
// []uint8. Tree.ToMap renders a single-element leaf-list as a bare scalar and a
// multi-element one as a []any, mirroring configLeafList in the IS-IS resolver. An out-of-
// range value is rejected (never truncated). An absent leaf yields nil (base instance 0).
func configInstanceIDs(v any) ([]uint8, error) {
	var raw []any
	switch list := v.(type) {
	case nil:
		return nil, nil
	case []any:
		raw = list
	default:
		raw = []any{list}
	}
	seen := make(map[uint8]struct{}, len(raw))
	out := make([]uint8, 0, len(raw))
	for _, item := range raw {
		n, ok := configNumber(item)
		if !ok {
			continue
		}
		if n > 255 {
			return nil, errInstanceIDRange
		}
		id := uint8(n)
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	slices.Sort(out)
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// parseNBMANeighbors resolves the `nbma-neighbor` list of one interface (RFC 2328
// App C.6). The list key is the IPv4 neighbor address (OSPFv2) or the neighbor
// Router ID (OSPFv3); both parse as a dotted quad, so this shared resolver records
// both interpretations and the engine selects by address family. A `priority` leaf
// (default 0 = ineligible) and an optional `link-local` leaf (IPv6 only) refine each
// entry.
func parseNBMANeighbors(ifaceName string, v any) ([]nbmaNeighborConfig, error) {
	entries := keyedList(v, false)
	if len(entries) == 0 {
		return nil, nil
	}
	out := make([]nbmaNeighborConfig, 0, len(entries))
	for _, ne := range entries {
		nc := nbmaNeighborConfig{}
		if addr, err := netip.ParseAddr(ne.key); err == nil && addr.Is4() {
			nc.Address = addr
		}
		if r, err := types.ParseRouterID(ne.key); err == nil {
			nc.RouterID = r
		}
		if !nc.Address.IsValid() && nc.RouterID == (types.RouterID{}) {
			return nil, fmt.Errorf("ospf: interface %q nbma-neighbor %q is not a valid IPv4 address or router-id", ifaceName, ne.key)
		}
		if s := configString(ne.data["link-local"]); s != "" {
			ll, err := netip.ParseAddr(s)
			if err != nil || !ll.Is6() || ll.Is4In6() || !ll.IsLinkLocalUnicast() {
				return nil, fmt.Errorf("ospf: interface %q nbma-neighbor %q link-local %q must be an IPv6 link-local address", ifaceName, ne.key, s)
			}
			nc.LinkLocal = ll
		}
		if p, ok := configUint8(ne.data["priority"]); ok {
			nc.Priority = p
		}
		out = append(out, nc)
	}
	return out, nil
}

func parseAuth(m map[string]any, defMode string) authConfig {
	a := authConfig{Mode: defMode}
	if s := configString(m["mode"]); s != "" {
		a.Mode = s
	}
	a.KeyChain = configString(m["key-chain"])
	return a
}

func parseKeyChain(entry listEntry) keyChainConfig {
	kc := keyChainConfig{Name: entry.key}
	if s := configString(entry.data["name"]); s != "" {
		kc.Name = s
	}
	kc.ExtendedSequence = configBool(entry.data["extended-sequence"], false)
	for _, keyEntry := range keyedList(entry.data["key"], true) {
		k := keyConfig{Algorithm: authAlgorithmMD5}
		if v, ok := configUint32(keyEntry.data["key-id"]); ok {
			k.KeyID = v
		} else if id, err := strconv.ParseUint(keyEntry.key, 10, 32); err == nil {
			k.KeyID = uint32(id)
		}
		if s := configString(keyEntry.data["algorithm"]); s != "" {
			k.Algorithm = s
		}
		k.Secret = configString(keyEntry.data["secret"])
		if sl, ok := keyEntry.data["send-lifetime"].(map[string]any); ok {
			k.SendLifetime = parseLifetime(sl)
		}
		if al, ok := keyEntry.data["accept-lifetime"].(map[string]any); ok {
			k.AcceptLifetime = parseLifetime(al)
		}
		kc.Keys = append(kc.Keys, k)
	}
	return kc
}

func parseLifetime(m map[string]any) lifetimeConfig {
	return lifetimeConfig{Start: configString(m["start"]), End: configString(m["end"])}
}

// lifetimeBounds parses a lifetimeConfig's RFC3339 timestamps into a half-open
// [start, end) window. An empty Start or End yields the zero time.Time for that
// bound, which the keystore treats as unbounded (always valid), so a key with no
// configured lifetime is active at all times. ok is false only when a present
// string fails to parse.
func lifetimeBounds(l lifetimeConfig) (start, end time.Time, ok bool) {
	if l.Start != "" {
		t, err := time.Parse(time.RFC3339, l.Start)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		start = t
	}
	if l.End != "" {
		t, err := time.Parse(time.RFC3339, l.End)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		end = t
	}
	return start, end, true
}

func deriveRouterID(source routerIDSource) (types.RouterID, bool) {
	infos, err := source.Interfaces()
	if err != nil {
		return types.RouterID{}, false
	}
	return deriveRouterIDFromInterfaces(infos)
}

// RFC 2328 Section C.1: when no Router ID is configured, routers commonly pick
// the highest loopback address, else the highest interface address. This helper
// keeps that policy pure and testable; the source owns OS discovery.
func deriveRouterIDFromInterfaces(infos []iface.InterfaceInfo) (types.RouterID, bool) {
	var loop, any netip.Addr
	var haveLoop, haveAny bool
	for i := range infos {
		isLoop := isLoopback(infos[i])
		for _, a := range infos[i].Addresses {
			addr, err := netip.ParseAddr(a.Address)
			if err != nil || !addr.Is4() || addr.IsUnspecified() {
				continue
			}
			if !haveAny || addr4Less(any, addr) {
				any = addr
				haveAny = true
			}
			if isLoop && (!haveLoop || addr4Less(loop, addr)) {
				loop = addr
				haveLoop = true
			}
		}
	}
	if haveLoop {
		return routerIDFromAddr(loop), true
	}
	if haveAny {
		return routerIDFromAddr(any), true
	}
	return types.RouterID{}, false
}

func routerIDFromAddr(addr netip.Addr) types.RouterID { return types.RouterID(addr.As4()) }

func isLoopback(info iface.InterfaceInfo) bool {
	return info.Type == "loopback" || info.Name == "lo" || info.Name == "lo0" || strings.HasPrefix(info.Name, "lo:")
}

func addr4Less(a, b netip.Addr) bool { return addr4Value(a) < addr4Value(b) }

func addr4Value(addr netip.Addr) uint32 {
	b := addr.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// configUint8, configUint16 and configUint32 read a config-tree scalar and
// narrow it, returning false when the value does not fit the target type.
//
// The bound belongs here, next to the narrowing, even though the config file
// parser already rejects anything outside the leaf's declared YANG type range
// (ValidateLeafValue, internal/component/config/schema.go:787-805, reached from
// parser.go:266). Relying on a guard three layers up means this code fails OPEN
// for any future entry point that delivers a tree without that validation, and a
// bare uintN(v) would then store a silently truncated value rather than reject
// it (ai/rules/evidence.md, ai/rules/protocol.md).
//
// The bound is the target type's own maximum, not a per-leaf maximum: every YANG
// leaf here declares the same width as the Go field it feeds, so no config the
// parser accepts can be refused by these helpers.
func configUint8(v any) (uint8, bool) {
	n, ok := configNumber(v)
	if !ok || n > math.MaxUint8 {
		return 0, false
	}
	return uint8(n), true
}

func configUint16(v any) (uint16, bool) {
	n, ok := configNumber(v)
	if !ok || n > math.MaxUint16 {
		return 0, false
	}
	return uint16(n), true
}

func configUint32(v any) (uint32, bool) {
	n, ok := configNumber(v)
	if !ok || n > math.MaxUint32 {
		return 0, false
	}
	return uint32(n), true
}

func configNumber(v any) (uint64, bool) {
	switch n := v.(type) {
	case float64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case string:
		u, err := strconv.ParseUint(n, 10, 64)
		if err != nil {
			return 0, false
		}
		return u, true
	default:
		return 0, false
	}
}

func configBool(v any, def bool) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		switch b {
		case "true":
			return true
		case "false":
			return false
		}
	}
	return def
}

func configString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

type listEntry struct {
	key  string
	data map[string]any
}

func keyedList(v any, numericKey bool) []listEntry {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	entries := make([]listEntry, 0, len(m))
	for key, raw := range m {
		em, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		entries = append(entries, listEntry{key: key, data: em})
	}
	if numericKey {
		sort.Slice(entries, func(i, j int) bool {
			ai, _ := strconv.ParseUint(entries[i].key, 10, 64)
			bj, _ := strconv.ParseUint(entries[j].key, 10, 64)
			return ai < bj
		})
	} else {
		sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })
	}
	return entries
}
