// Design: docs/architecture/zefs-format.md -- ZeFS key definitions
// Related: registry.go -- MustRegister and KeyEntry types

package zefs

// Registered storage keys, shared by live trees and ZeFS artifacts. Callers
// reference these declarations instead of hardcoding registered key strings.
var (
	KeySSHUsername              = MustRegister(KeyEntry{Pattern: "meta/ssh/{host}/{port}/username", Description: "SSH authentication username"})
	KeySSHPassword              = MustRegister(KeyEntry{Pattern: "meta/ssh/{host}/{port}/password", Description: "SSH password (bcrypt hash)", Private: true})
	KeySSHDefault               = MustRegister(KeyEntry{Pattern: "meta/ssh/default", Description: "Default remote target (host/port)"})
	KeyLocalAdminUsername       = MustRegister(KeyEntry{Pattern: "meta/auth/local/username", Description: "Local administrative username"})
	KeyLocalAdminPassword       = MustRegister(KeyEntry{Pattern: "meta/auth/local/password", Description: "Local administrative password (bcrypt hash)", Private: true})
	KeyLocalAdminAuthorizedKeys = MustRegister(KeyEntry{Pattern: "meta/auth/local/authorized-keys", Description: "Local administrative SSH public keys"})
	KeyInstanceName             = MustRegister(KeyEntry{Pattern: "meta/instance/name", Description: "Router instance name"})
	KeyInstanceManaged          = MustRegister(KeyEntry{Pattern: "meta/instance/managed", Description: "Managed mode flag (true/false)"})
	KeyManagedHubAddress        = MustRegister(KeyEntry{Pattern: "meta/managed/hub/address", Description: "Hub address (host:port) a managed client reaches on its first start, written by ze init --managed"})
	KeyManagedHubToken          = MustRegister(KeyEntry{Pattern: "meta/managed/hub/token", Description: "Secret a managed client authenticates to its hub with on its first start", Private: true})
	KeyManagedHubCA             = MustRegister(KeyEntry{Pattern: "meta/managed/hub/ca", Description: "Hub certificate authority root (PEM) a managed client authenticates its hub against when its config names no ca"})
	KeyManagedHubTrust          = MustRegister(KeyEntry{Pattern: "meta/managed/hub/trust", Description: "How a managed client with no stored hub root gets one: first-use keeps the authority the hub presents on the first connection"})
	KeyWebCert                  = MustRegister(KeyEntry{Pattern: "meta/web/cert", Description: "HTTPS certificate (PEM)", Private: true})
	KeyWebKey                   = MustRegister(KeyEntry{Pattern: "meta/web/key", Description: "HTTPS private key (PEM)", Private: true})
	KeyInstanceAdminDisabled    = MustRegister(KeyEntry{Pattern: "meta/instance/admin-disabled", Description: "Admin login disabled (RADIUS-only)"})
	KeyGRMarker                 = MustRegister(KeyEntry{Pattern: "meta/bgp/gr-marker", Description: "Graceful restart marker (8-byte expiry)"})
	KeyOSPFAuthBootCount        = MustRegister(KeyEntry{Pattern: "meta/ospf/auth/boot-count", Description: "OSPFv2 cryptographic-auth boot count (4-byte, RFC 7474 monotonic high word)"})
	KeyHistoryMax               = MustRegister(KeyEntry{Pattern: "meta/history/max", Description: "Maximum history entries per mode"})
	KeyHistory                  = MustRegister(KeyEntry{Pattern: "meta/history/{username}/{mode}", Description: "Per-user command history"})
	KeyConfigActive             = MustRegister(KeyEntry{Pattern: "meta/config/{name}/active", Description: "Active config version pointer"})
	KeyConfigCandidate          = MustRegister(KeyEntry{Pattern: "meta/config/{name}/candidate", Description: "Pending config version pointer"})
	KeyConfigRollback           = MustRegister(KeyEntry{Pattern: "meta/config/{name}/rollback", Description: "Previous active config version pointer"})
	KeyConfigRecovery           = MustRegister(KeyEntry{Pattern: "meta/config/{name}/recovery", Description: "Operator-selected recovery config version pointer"})
	KeyConfigFileCommit         = MustRegister(KeyEntry{Pattern: "meta/config/{name}/file-commit", Description: "Durable explicit-file commit intent", Private: true})
	KeyConfigConfirmPending     = MustRegister(KeyEntry{Pattern: "meta/config/{name}/confirm-pending", Description: "Open confirmed-commit window: owner, deadline and rollback config (JSON)", Private: true})
	KeyConfigHeld               = MustRegister(KeyEntry{Pattern: "meta/config/{name}/held", Description: "Hub copy of the configuration a managed client holds: its bytes, hash, activation time and any revert decision (JSON)", Private: true})
	KeyOSPFGRFact               = MustRegister(KeyEntry{Pattern: "meta/ospf/gr-fact-{instance}", Description: "OSPF graceful restart state"})
	KeyFileActive               = MustRegister(KeyEntry{Pattern: "file/active/{basename}", Description: "Current active config file"})
	KeyFileCandidate            = MustRegister(KeyEntry{Pattern: "file/candidate/{basename}", Description: "Candidate config file"})
	KeyFileDraft                = MustRegister(KeyEntry{Pattern: "file/draft/{basename}", Description: "Draft config file (in progress)"})
	KeyFileVersion              = MustRegister(KeyEntry{Pattern: "file/{date}/{basename}", Description: "Historical config version entry (sha256:<hex> naming its object)"})
	KeyObject                   = MustRegister(KeyEntry{Pattern: "object/{hex}", Description: "Config version bytes, named by their SHA-256"})
	KeyFileTemplate             = MustRegister(KeyEntry{Pattern: "file/template/{basename}", Description: "Config template (merged with discovery on first boot)"})
	KeyConfigLastKnownGood      = MustRegister(KeyEntry{Pattern: "meta/config/last-known-good", Description: "SHA-256 hash of validated seed config"})
	KeyMachineID                = MustRegister(KeyEntry{Pattern: "meta/instance/machine-id", Description: "Stable machine identity (hex string)"})
	KeyDebugProfile             = MustRegister(KeyEntry{Pattern: "debug/profile/{name}", Description: "Named debug profile (JSON)"})
	KeyIRRPrefixCache           = MustRegister(KeyEntry{Pattern: "meta/irr/{name}", Description: "Legacy firewall IRR prefix list, per ASN/AS-SET (JSON); the irr plugin converts each to a dataset and removes it"})
	KeyFirewallDomainGroup      = MustRegister(KeyEntry{Pattern: "meta/firewall/domain-group/{group}/{name}/{family}", Description: "Firewall domain-group last-good addresses, per DNS name and family (JSON)"})

	// Persisted runtime state (see ai/rules/architecture.md). These live in
	// the shared store instead of loose files so appliance state is managed.
	KeyDDoSDetectBaseline   = MustRegister(KeyEntry{Pattern: "meta/ddos/detect-baseline", Description: "ddos-detect rolling PPS/BPS baseline snapshot (JSON)"})
	KeyTrafficTCSnapshot    = MustRegister(KeyEntry{Pattern: "meta/traffic/tc-snapshot", Description: "traffic-usage original-qdisc snapshot for restore (JSON)"})
	KeyNTPLastTime          = MustRegister(KeyEntry{Pattern: "meta/ntp/last-time", Description: "NTP last-known monotonic time (RFC3339)"})
	KeyBFDAuthSeq           = MustRegister(KeyEntry{Pattern: "meta/bfd/auth/{session}", Description: "BFD meticulous-keyed TX sequence number, per session (RFC 5880)"})
	KeyRIRDelegation        = MustRegister(KeyEntry{Pattern: "meta/rir/delegation", Description: "ASN-to-registry delegation table refreshed by 'update resolve rir'"})
	KeyConfigPreviousActive = MustRegister(KeyEntry{Pattern: "meta/config/health-revert-previous", Description: "Pre-change active config snapshot for health-check auto-revert"})
	KeyConfigLastGoodPushed = MustRegister(KeyEntry{Pattern: "meta/config/last-known-good-pushed", Description: "SHA-256 of the last health-confirmed pushed config"})
	KeyConfigActiveHash     = MustRegister(KeyEntry{Pattern: "meta/config/active-hash", Description: "SHA-256 of the running active config (fleet drift detection)"})
	KeyConfigUpdateHistory  = MustRegister(KeyEntry{Pattern: "meta/config/update-history", Description: "Self-update event history (JSON)"})
)

// The certificate authority Ze issues its own components' certificates from.
// The root CERTIFICATE is public material an operator copies into a peer's
// trust anchor, so it stays listable. The root KEY is Private, which keeps its
// pattern out of "ze data registered". The flag controls discovery only.
// Live trees enforce 0600 for every frame; blob artifacts protect all their
// values with the enclosing file's 0600 mode.
//
// They are their own declaration rather than two more lines of the block above,
// so that a later key added here does not read as an edit to every var in that
// block. The documentation drift check reads a Go declaration whole.
var (
	KeyCACert = MustRegister(KeyEntry{Pattern: "meta/ca/cert", Description: "Local CA root certificate (PEM)"})
	KeyCAKey  = MustRegister(KeyEntry{Pattern: "meta/ca/key", Description: "Local CA root private key (PEM)", Private: true})
)

// The IRR data the irr plugin (internal/plugins/irr) fetches, schedules and
// stores, so a restart enforces the lists it enforced before. The plugin is
// the only writer of the dataset, pointer, binding and generation keys;
// bgp-filter-irr and firewall-irr read them, and each consumer writes its own
// interest key, which tells the owner what to fetch
// (docs/architecture/irr-owner.md). A dataset key ends in the digest of its
// value, so a write never replaces the bytes a pointer already names. The
// pointer is written only after the dataset it names, so a consumer never
// reads a pointer to nothing, and a pointer write that fails leaves the old
// pointer naming the old copy.
//
// IRRDatasetValueMax bounds one dataset value: 500,000 IPv4 prefixes at five
// bytes and 500,000 IPv6 prefixes at seventeen bytes are 11,000,000 bytes, and
// the header and digest fit in what is left. Its base64 form is 16,000,000
// bytes, which leaves room for the JSON envelope inside the 16 MiB RPC message
// (rpc.MaxMessageSize).
const IRRDatasetValueMax = 12_000_000

var (
	KeyIRRDataset    = MustRegister(KeyEntry{Pattern: "meta/irr-data/{server}/{as-set}/{digest}", Description: "IRR prefix dataset of one AS-SET from one server, keyed by its own SHA-256 (binary, digest-checked)", ValueMax: IRRDatasetValueMax})
	KeyIRRSet        = MustRegister(KeyEntry{Pattern: "meta/irr-set/{as-set}", Description: "The IRR dataset currently enforced for one AS-SET (JSON pointer)"})
	KeyIRRASN        = MustRegister(KeyEntry{Pattern: "meta/irr-asn/{asn}", Description: "The AS-SET PeeringDB named for a bare ASN (JSON)"})
	KeyIRRGeneration = MustRegister(KeyEntry{Pattern: "meta/irr-generation", Description: "IRR dataset generation, raised on every pointer write or removal (4-byte counter)"})
	KeyIRRInterest   = MustRegister(KeyEntry{Pattern: "meta/irr-interest/{consumer}", Description: "The AS-SETs and ASNs one IRR consumer enforces (JSON)"})
)

// The container image store's records (internal/plugins/container/imagestore).
// The container plugin is their only writer. An image reference holds a slash
// and a colon, and a key parameter matches one path segment, so a binding is
// keyed by the SHA-256 of the reference and holds the reference in its value.
// What an image is, how large it is and when it was fetched is in its folder
// on the data directory, never in a key, so a lost store loses the bindings
// and the generations only.
var (
	KeyContainerBinding    = MustRegister(KeyEntry{Pattern: "meta/container/binding/{reference-digest}", Description: "The image digest one container image reference is bound to, keyed by the reference's SHA-256 (JSON)"})
	KeyContainerGeneration = MustRegister(KeyEntry{Pattern: "meta/container/generation/{container}", Description: "One container's current and previous image digests and its last 20 events (JSON)"})
	KeyContainerProjectID  = MustRegister(KeyEntry{Pattern: "meta/container/project-id", Description: "The counter the next container storage project id comes from (4-byte counter)"})
)

// The container networks' address record (internal/plugins/container/network):
// one value per network, the map from each attached container's name to its
// addresses and its link slot, written whole and read tolerantly. The
// container plugin is its only writer.
var KeyContainerNetworkAddress = MustRegister(KeyEntry{Pattern: "meta/container/address/{network}", Description: "The addresses and link slots of the containers attached to one container network (JSON)"})

// A support archive the daemon collected for a caller in another process,
// which `show data cat <key>` reads back (internal/component/support
// storeAnswer). The archive redacts secrets, as every archive does, so the
// pattern stays listable.
var KeySupportArchive = MustRegister(KeyEntry{Pattern: "support/archive/{name}", Description: "Support archive the daemon collected for a remote caller (tar.gz)"})

// Why ze last stopped itself when a plugin caused the stop, or when a
// stop-time cleanup ran late: the plugin server writes it before the stop and
// deletes it on the next clean start, and the next `ze doctor` reads it
// (internal/component/plugin/server, last_stop.go). It is new, so no earlier
// image writes or reads it.
var KeyPluginLastStop = MustRegister(KeyEntry{Pattern: "meta/plugin/last-stop", Description: "Why ze last stopped because of a plugin, or ran a stop cleanup late (JSON)"})

// The baseline of a managed client: the version hash (fleet.VersionHash) of
// the last configuration that became active from its hub. Comparing it with
// the hash of the configuration the device runs says whether the device
// changed since its hub last gave it one. An absent key is a valid state: no
// baseline was recorded yet. It sits beside the meta/managed/hub keys, because
// meta/instance/managed is a value and the store refuses a key below a value.
// It is its own declaration for the reason given above KeyCACert.
var KeyManagedBaseVersion = MustRegister(KeyEntry{Pattern: "meta/managed/base-version", Description: "Version hash of the last configuration a managed client took from its hub (absent until one is recorded)"})

// ManagedHubTrustFirstUse is the one value KeyManagedHubTrust holds: the
// managed client keeps the certificate authority its hub presents on the
// first connection, and authenticates the hub against it from then on.
const ManagedHubTrustFirstUse = "first-use"
