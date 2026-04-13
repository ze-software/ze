// Design: docs/architecture/zefs-format.md -- ZeFS key definitions
// Related: registry.go -- MustRegister and KeyEntry types

package zefs

// Registered ZeFS keys. All zefs blob key strings in the codebase
// should reference these vars instead of hardcoding string literals.
var (
	KeySSHUsername     = MustRegister(KeyEntry{Pattern: "meta/ssh/username", Description: "SSH authentication username"})
	KeySSHPassword     = MustRegister(KeyEntry{Pattern: "meta/ssh/password", Description: "SSH password (bcrypt hash)", Private: true})
	KeySSHHost         = MustRegister(KeyEntry{Pattern: "meta/ssh/host", Description: "SSH server host address"})
	KeySSHPort         = MustRegister(KeyEntry{Pattern: "meta/ssh/port", Description: "SSH server port"})
	KeyInstanceName    = MustRegister(KeyEntry{Pattern: "meta/instance/name", Description: "Router instance name"})
	KeyInstanceManaged = MustRegister(KeyEntry{Pattern: "meta/instance/managed", Description: "Managed mode flag (true/false)"})
	KeyWebCert         = MustRegister(KeyEntry{Pattern: "meta/web/cert", Description: "HTTPS certificate (PEM)", Private: true})
	KeyWebKey          = MustRegister(KeyEntry{Pattern: "meta/web/key", Description: "HTTPS private key (PEM)", Private: true})
	KeyGRMarker        = MustRegister(KeyEntry{Pattern: "meta/bgp/gr-marker", Description: "Graceful restart marker (8-byte expiry)"})
	KeyHistoryMax      = MustRegister(KeyEntry{Pattern: "meta/history/max", Description: "Maximum history entries per mode"})
	KeyHistory         = MustRegister(KeyEntry{Pattern: "meta/history/{username}/{mode}", Description: "Per-user command history"})
	KeyFileActive      = MustRegister(KeyEntry{Pattern: "file/active/{basename}", Description: "Current active config file"})
	KeyFileDraft       = MustRegister(KeyEntry{Pattern: "file/draft/{basename}", Description: "Draft config file (in progress)"})
	KeyFileTemplate    = MustRegister(KeyEntry{Pattern: "file/template/{basename}", Description: "Config template (merged with discovery on first boot)"})
)
