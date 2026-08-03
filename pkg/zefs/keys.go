// Design: docs/architecture/zefs-format.md -- ZeFS key definitions
// Related: registry.go -- MustRegister and KeyEntry types

package zefs

// Registered ZeFS keys. All zefs blob key strings in the codebase
// should reference these vars instead of hardcoding string literals.
var (
	KeySSHUsername           = MustRegister(KeyEntry{Pattern: "meta/ssh/{host}/{port}/username", Description: "SSH authentication username"})
	KeySSHPassword           = MustRegister(KeyEntry{Pattern: "meta/ssh/{host}/{port}/password", Description: "SSH password (bcrypt hash)", Private: true})
	KeySSHDefault            = MustRegister(KeyEntry{Pattern: "meta/ssh/default", Description: "Default remote target (host/port)"})
	KeyLocalAdminUsername    = MustRegister(KeyEntry{Pattern: "meta/auth/local/username", Description: "Local administrative username"})
	KeyLocalAdminPassword    = MustRegister(KeyEntry{Pattern: "meta/auth/local/password", Description: "Local administrative password (bcrypt hash)", Private: true})
	KeyInstanceName          = MustRegister(KeyEntry{Pattern: "meta/instance/name", Description: "Router instance name"})
	KeyInstanceManaged       = MustRegister(KeyEntry{Pattern: "meta/instance/managed", Description: "Managed mode flag (true/false)"})
	KeyWebCert               = MustRegister(KeyEntry{Pattern: "meta/web/cert", Description: "HTTPS certificate (PEM)", Private: true})
	KeyWebKey                = MustRegister(KeyEntry{Pattern: "meta/web/key", Description: "HTTPS private key (PEM)", Private: true})
	KeySSHAuthorizedKeys     = MustRegister(KeyEntry{Pattern: "meta/ssh/authorized-keys", Description: "SSH authorized public keys"})
	KeyInstanceAdminDisabled = MustRegister(KeyEntry{Pattern: "meta/instance/admin-disabled", Description: "Admin login disabled (RADIUS-only)"})
	KeyGRMarker              = MustRegister(KeyEntry{Pattern: "meta/bgp/gr-marker", Description: "Graceful restart marker (8-byte expiry)"})
	KeyOSPFAuthBootCount     = MustRegister(KeyEntry{Pattern: "meta/ospf/auth/boot-count", Description: "OSPFv2 cryptographic-auth boot count (4-byte, RFC 7474 monotonic high word)"})
	KeyHistoryMax            = MustRegister(KeyEntry{Pattern: "meta/history/max", Description: "Maximum history entries per mode"})
	KeyHistory               = MustRegister(KeyEntry{Pattern: "meta/history/{username}/{mode}", Description: "Per-user command history"})
	KeyConfigActive          = MustRegister(KeyEntry{Pattern: "meta/config/active", Description: "Active config version pointer"})
	KeyConfigCandidate       = MustRegister(KeyEntry{Pattern: "meta/config/candidate", Description: "Pending config version pointer"})
	KeyConfigRollback        = MustRegister(KeyEntry{Pattern: "meta/config/rollback", Description: "Previous active config version pointer"})
	KeyConfigRecovery        = MustRegister(KeyEntry{Pattern: "meta/config/recovery", Description: "Operator-selected recovery config version pointer"})
	KeyFileActive            = MustRegister(KeyEntry{Pattern: "file/active/{basename}", Description: "Current active config file"})
	KeyFileCandidate         = MustRegister(KeyEntry{Pattern: "file/candidate/{basename}", Description: "Candidate config file"})
	KeyFileDraft             = MustRegister(KeyEntry{Pattern: "file/draft/{basename}", Description: "Draft config file (in progress)"})
	KeyFileVersion           = MustRegister(KeyEntry{Pattern: "file/{date}/{basename}", Description: "Historical config version"})
	KeyFileTemplate          = MustRegister(KeyEntry{Pattern: "file/template/{basename}", Description: "Config template (merged with discovery on first boot)"})
	KeyConfigLastKnownGood   = MustRegister(KeyEntry{Pattern: "meta/config/last-known-good", Description: "SHA-256 hash of validated seed config"})
	KeyMachineID             = MustRegister(KeyEntry{Pattern: "meta/instance/machine-id", Description: "Stable machine identity (hex string)"})
	KeyDebugProfile          = MustRegister(KeyEntry{Pattern: "debug/profile/{name}", Description: "Named debug profile (JSON)"})
	KeyIRRCache              = MustRegister(KeyEntry{Pattern: "meta/bgp/irr-cache", Description: "IRR-resolved prefix cache (JSON, all ASNs; legacy, migrated to meta/irr/{name})"})
	KeyIRRPrefixCache        = MustRegister(KeyEntry{Pattern: "meta/irr/{name}", Description: "IRR-resolved prefix cache, per ASN/AS-SET (JSON)"})

	// Persisted runtime state (see ai/rules/architecture.md). These live in
	// the shared store instead of loose files so appliance state is managed.
	KeyDDoSDetectBaseline   = MustRegister(KeyEntry{Pattern: "meta/ddos/detect-baseline", Description: "ddos-detect rolling PPS/BPS baseline snapshot (JSON)"})
	KeyTrafficTCSnapshot    = MustRegister(KeyEntry{Pattern: "meta/traffic/tc-snapshot", Description: "traffic-usage original-qdisc snapshot for restore (JSON)"})
	KeyNTPLastTime          = MustRegister(KeyEntry{Pattern: "meta/ntp/last-time", Description: "NTP last-known monotonic time (RFC3339)"})
	KeyBFDAuthSeq           = MustRegister(KeyEntry{Pattern: "meta/bfd/auth/{session}", Description: "BFD meticulous-keyed TX sequence number, per session (RFC 5880)"})
	KeyConfigPreviousActive = MustRegister(KeyEntry{Pattern: "meta/config/health-revert-previous", Description: "Pre-change active config snapshot for health-check auto-revert"})
	KeyConfigLastGoodPushed = MustRegister(KeyEntry{Pattern: "meta/config/last-known-good-pushed", Description: "SHA-256 of the last health-confirmed pushed config"})
	KeyConfigActiveHash     = MustRegister(KeyEntry{Pattern: "meta/config/active-hash", Description: "SHA-256 of the running active config (fleet drift detection)"})
	KeyConfigUpdateHistory  = MustRegister(KeyEntry{Pattern: "meta/config/update-history", Description: "Self-update event history (JSON)"})
)
