// Design: plan/learned/734-ipsec-3-data-model.md -- IPsec data model types
// Related: algorithm_support.go -- the predicates deciding which of these enums a build implements
// Related: config.go -- the parser that fills these types from the config tree

package ipsec

const unknownEnum = "unknown"

// EncryptionAlgo represents an IKE/ESP encryption algorithm.
// RFC 7296 Section 3.3.2: Transform Type 1 (Encryption).
type EncryptionAlgo int

const (
	EncryptionUnknown      EncryptionAlgo = iota
	EncryptionAES128                      // AES-CBC-128
	EncryptionAES256                      // AES-CBC-256
	EncryptionAES128GCM                   // AES-GCM-128 with 16-byte ICV
	EncryptionAES256GCM                   // AES-GCM-256 with 16-byte ICV
	EncryptionChaCha20Poly                // ChaCha20-Poly1305 AEAD
	Encryption3DES                        // Triple-DES-CBC (legacy)
)

var encryptionNames = map[EncryptionAlgo]string{
	EncryptionAES128:       "aes128",
	EncryptionAES256:       "aes256",
	EncryptionAES128GCM:    "aes128gcm",
	EncryptionAES256GCM:    "aes256gcm",
	EncryptionChaCha20Poly: "chacha20poly1305",
	Encryption3DES:         "3des",
}

var encryptionByName map[string]EncryptionAlgo

func init() {
	encryptionByName = make(map[string]EncryptionAlgo, len(encryptionNames))
	for k, name := range encryptionNames {
		encryptionByName[name] = k
	}
}

func (e EncryptionAlgo) String() string {
	if name, ok := encryptionNames[e]; ok {
		return name
	}
	return unknownEnum
}

// ParseEncryptionAlgo returns the EncryptionAlgo for a YANG enum value.
func ParseEncryptionAlgo(s string) (EncryptionAlgo, bool) {
	a, ok := encryptionByName[s]
	return a, ok
}

// IsAEAD reports whether the algorithm provides integrated authentication
// (no separate integrity algorithm needed for ESP).
// RFC 7296 Section 3.3: AEAD ciphers; when proposing AEAD for ESP,
// INTEG must be NONE.
func (e EncryptionAlgo) IsAEAD() bool {
	switch e {
	case EncryptionAES128GCM, EncryptionAES256GCM, EncryptionChaCha20Poly:
		return true
	case EncryptionUnknown, EncryptionAES128, EncryptionAES256, Encryption3DES:
		return false
	}
	return false
}

// HashAlgo represents an integrity/PRF algorithm.
// RFC 7296 Section 3.3.2: Transform Type 3 (Integrity).
type HashAlgo int

const (
	HashUnknown HashAlgo = iota
	HashSHA1             // HMAC-SHA-1-96
	HashSHA256           // HMAC-SHA-256-128
	HashSHA384           // HMAC-SHA-384-192
	HashSHA512           // HMAC-SHA-512-256
)

var hashNames = map[HashAlgo]string{
	HashSHA1:   "sha1",
	HashSHA256: "sha256",
	HashSHA384: "sha384",
	HashSHA512: "sha512",
}

var hashByName map[string]HashAlgo

func init() {
	hashByName = make(map[string]HashAlgo, len(hashNames))
	for k, name := range hashNames {
		hashByName[name] = k
	}
}

func (h HashAlgo) String() string {
	if name, ok := hashNames[h]; ok {
		return name
	}
	return unknownEnum
}

// ParseHashAlgo returns the HashAlgo for a YANG enum value.
func ParseHashAlgo(s string) (HashAlgo, bool) {
	a, ok := hashByName[s]
	return a, ok
}

// DHGroup is a Diffie-Hellman group number (RFC 7296 Section 3.3.2:
// Transform Type 4). Valid range: 1-31.
type DHGroup uint8

// ValidDHGroup reports whether g is a recognized DH group.
func ValidDHGroup(g DHGroup) bool {
	return g >= 1 && g <= 31
}

// PFSMode controls Perfect Forward Secrecy for Child SA rekeying.
type PFSMode int

const (
	PFSEnable PFSMode = iota
	PFSDisable
)

var pfsNames = map[PFSMode]string{
	PFSEnable:  "enable",
	PFSDisable: "disable",
}

var pfsByName map[string]PFSMode

func init() {
	pfsByName = make(map[string]PFSMode, len(pfsNames))
	for k, name := range pfsNames {
		pfsByName[name] = k
	}
}

func (p PFSMode) String() string {
	if name, ok := pfsNames[p]; ok {
		return name
	}
	return unknownEnum
}

// ParsePFSMode returns the PFSMode for a YANG enum value.
func ParsePFSMode(s string) (PFSMode, bool) {
	a, ok := pfsByName[s]
	return a, ok
}

// AuthMode discriminates between PSK and X.509 authentication.
type AuthMode int

const (
	AuthUnknown AuthMode = iota
	AuthPreSharedSecret
	AuthX509
	AuthEAPTLS
	AuthEAPMSCHAPv2
)

var authModeNames = map[AuthMode]string{ //nolint:gosec // enum name, not a credential
	AuthPreSharedSecret: "pre-shared-secret",
	AuthX509:            "x509",
	AuthEAPTLS:          "eap-tls",
	AuthEAPMSCHAPv2:     "eap-mschapv2",
}

var authModeByName map[string]AuthMode

func init() {
	authModeByName = make(map[string]AuthMode, len(authModeNames))
	for k, name := range authModeNames {
		authModeByName[name] = k
	}
}

func (a AuthMode) String() string {
	if name, ok := authModeNames[a]; ok {
		return name
	}
	return unknownEnum
}

// ParseAuthMode returns the AuthMode for a YANG enum value.
func ParseAuthMode(s string) (AuthMode, bool) {
	m, ok := authModeByName[s]
	return m, ok
}

// ConnectionType controls whether the local side initiates or waits.
type ConnectionType int

const (
	ConnectionInitiate ConnectionType = iota
	ConnectionRespond
)

var connectionTypeNames = map[ConnectionType]string{
	ConnectionInitiate: "initiate",
	ConnectionRespond:  "respond",
}

var connectionTypeByName map[string]ConnectionType

func init() {
	connectionTypeByName = make(map[string]ConnectionType, len(connectionTypeNames))
	for k, name := range connectionTypeNames {
		connectionTypeByName[name] = k
	}
}

func (c ConnectionType) String() string {
	if name, ok := connectionTypeNames[c]; ok {
		return name
	}
	return unknownEnum
}

// ParseConnectionType returns the ConnectionType for a YANG enum value.
func ParseConnectionType(s string) (ConnectionType, bool) {
	t, ok := connectionTypeByName[s]
	return t, ok
}

// CloseAction determines behavior when the peer closes the IKE SA.
type CloseAction int

const (
	CloseActionNone CloseAction = iota
	CloseActionStart
	CloseActionRestart
)

var closeActionNames = map[CloseAction]string{
	CloseActionNone:    "none",
	CloseActionStart:   "start",
	CloseActionRestart: "restart",
}

var closeActionByName map[string]CloseAction

func init() {
	closeActionByName = make(map[string]CloseAction, len(closeActionNames))
	for k, name := range closeActionNames {
		closeActionByName[name] = k
	}
}

func (c CloseAction) String() string {
	if name, ok := closeActionNames[c]; ok {
		return name
	}
	return unknownEnum
}

// ParseCloseAction returns the CloseAction for a YANG enum value.
func ParseCloseAction(s string) (CloseAction, bool) {
	a, ok := closeActionByName[s]
	return a, ok
}

// DPDAction determines behavior when Dead Peer Detection fires.
type DPDAction int

const (
	DPDActionRestart DPDAction = iota
	DPDActionHold
	DPDActionClear
)

var dpdActionNames = map[DPDAction]string{
	DPDActionRestart: "restart",
	DPDActionHold:    "hold",
	DPDActionClear:   "clear",
}

var dpdActionByName map[string]DPDAction

func init() {
	dpdActionByName = make(map[string]DPDAction, len(dpdActionNames))
	for k, name := range dpdActionNames {
		dpdActionByName[name] = k
	}
}

func (d DPDAction) String() string {
	if name, ok := dpdActionNames[d]; ok {
		return name
	}
	return unknownEnum
}

// ParseDPDAction returns the DPDAction for a YANG enum value.
func ParseDPDAction(s string) (DPDAction, bool) {
	a, ok := dpdActionByName[s]
	return a, ok
}

// KeyExchange is the IKE protocol version.
type KeyExchange int

const (
	KeyExchangeIKEv2 KeyExchange = iota
	KeyExchangeIKEv1
)

var keyExchangeNames = map[KeyExchange]string{
	KeyExchangeIKEv1: "ikev1",
	KeyExchangeIKEv2: "ikev2",
}

var keyExchangeByName map[string]KeyExchange

func init() {
	keyExchangeByName = make(map[string]KeyExchange, len(keyExchangeNames))
	for k, name := range keyExchangeNames {
		keyExchangeByName[name] = k
	}
}

func (k KeyExchange) String() string {
	if name, ok := keyExchangeNames[k]; ok {
		return name
	}
	return unknownEnum
}

// ParseKeyExchange returns the KeyExchange for a YANG enum value.
func ParseKeyExchange(s string) (KeyExchange, bool) {
	v, ok := keyExchangeByName[s]
	return v, ok
}

// DPDConfig holds Dead Peer Detection parameters.
type DPDConfig struct {
	Action   DPDAction
	Interval uint16
	Timeout  uint16
}

// ESPProposal is a single ESP crypto proposal, ordered by Number.
type ESPProposal struct {
	Number     uint16
	Encryption EncryptionAlgo
	Hash       HashAlgo // zero when encryption is AEAD
}

// ESPGroup is a named set of ESP proposals with shared lifetime and PFS settings.
type ESPGroup struct {
	Name      string
	Lifetime  uint32
	PFS       PFSMode
	Proposals []ESPProposal
}

// IKEProposal is a single IKE crypto proposal, ordered by Number.
type IKEProposal struct {
	Number     uint16
	Encryption EncryptionAlgo
	Hash       HashAlgo
	DHGroup    DHGroup
}

// IKEGroup is a named set of IKE proposals with shared DPD and lifetime settings.
type IKEGroup struct {
	Name        string
	KeyExchange KeyExchange
	Lifetime    uint32
	CloseAction CloseAction
	DPD         DPDConfig
	Proposals   []IKEProposal
}

// AuthConfig holds peer authentication settings.
type AuthConfig struct {
	Mode          AuthMode
	PSK           string // decoded plaintext (only for pre-shared-secret mode)
	LocalID       string
	RemoteID      string
	CACertificate string // PKI store name (x509 and EAP modes)
	// Certificate is a PKI store name. X.509 mode uses it, and every EAP mode
	// requires it, because RFC 7296 Section 2.16 makes the responder sign its
	// AUTH with a public key. ValidatePKIRefs enforces that.
	Certificate string
}

// SiteToSitePeer is a remote IPsec VPN peer.
type SiteToSitePeer struct {
	Name           string
	IKEGroup       string // reference to IKEGroup.Name
	ESPGroup       string // reference to ESPGroup.Name
	ConnectionType ConnectionType
	LocalAddress   string
	RemoteAddress  string
	Auth           AuthConfig
	VTIBind        string // VTI interface name
	IfID           uint32 // XFRM if_id for SA binding (must match the XFRM interface)
}

// EAPUser is a remote-access EAP user entry.
type EAPUser struct {
	Name        string
	Password    string `json:"-"` //nolint:gosec // decoded plaintext, never serialized
	Certificate string // PKI store name (EAP-TLS only)
}

// VirtualIPPool defines the address pool for road warrior clients.
type VirtualIPPool struct {
	Name   string
	Range  string   // IPv4 CIDR (e.g. "10.10.0.0/24")
	Range6 string   // IPv6 CIDR (e.g. "fd00::/64"), optional
	DNS    []string // DNS servers pushed to clients
	Domain string   // search domain pushed to clients
}

// RemoteAccessConfig holds EAP-based remote access VPN settings.
type RemoteAccessConfig struct {
	IKEGroup string // reference to IKEGroup.Name
	ESPGroup string // reference to ESPGroup.Name
	Auth     AuthConfig
	Pool     VirtualIPPool
	Users    map[string]EAPUser
}

// IPsecConfig holds the complete parsed IPsec configuration.
type IPsecConfig struct {
	Interface string

	// CookieThreshold is the number of half-open IKE SAs the responder tolerates
	// before an inbound IKE_SA_INIT must answer a COOKIE challenge (RFC 7296
	// Section 2.6). Zero, the YANG default, challenges every inbound initiation.
	CookieThreshold uint32

	ESPGroups    map[string]ESPGroup
	IKEGroups    map[string]IKEGroup
	Peers        map[string]SiteToSitePeer
	RemoteAccess *RemoteAccessConfig
}

// Changed returns the peer names whose configuration differs between
// the old config and c. Includes added, removed, and modified peers.
// The special name "remote-access" is included if the remote-access
// config changed.
func (c *IPsecConfig) Changed(old *IPsecConfig) []string {
	if old == nil && c == nil {
		return nil
	}

	var changed []string
	oldPeers := make(map[string]SiteToSitePeer)
	if old != nil {
		oldPeers = old.Peers
	}
	newPeers := make(map[string]SiteToSitePeer)
	if c != nil {
		newPeers = c.Peers
	}

	for name := range oldPeers {
		if _, ok := newPeers[name]; !ok {
			changed = append(changed, name)
		}
	}

	for name := range newPeers {
		oldPeer, ok := oldPeers[name]
		if !ok {
			changed = append(changed, name)
			continue
		}
		newPeer := newPeers[name]
		if !peersEqual(&oldPeer, &newPeer) {
			changed = append(changed, name)
		}
	}

	if !remoteAccessEqual(oldRA(old), oldRA(c)) {
		changed = append(changed, "remote-access")
	}

	return changed
}

func oldRA(cfg *IPsecConfig) *RemoteAccessConfig {
	if cfg == nil {
		return nil
	}
	return cfg.RemoteAccess
}

func remoteAccessEqual(a, b *RemoteAccessConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.IKEGroup != b.IKEGroup || a.ESPGroup != b.ESPGroup {
		return false
	}
	if a.Auth != b.Auth {
		return false
	}
	if a.Pool.Name != b.Pool.Name || a.Pool.Range != b.Pool.Range ||
		a.Pool.Range6 != b.Pool.Range6 || a.Pool.Domain != b.Pool.Domain {
		return false
	}
	if len(a.Pool.DNS) != len(b.Pool.DNS) {
		return false
	}
	for i, d := range a.Pool.DNS {
		if d != b.Pool.DNS[i] {
			return false
		}
	}
	if len(a.Users) != len(b.Users) {
		return false
	}
	for name, au := range a.Users {
		bu, ok := b.Users[name]
		if !ok || au.Password != bu.Password || au.Certificate != bu.Certificate {
			return false
		}
	}
	return true
}

func peersEqual(a, b *SiteToSitePeer) bool {
	return a.IKEGroup == b.IKEGroup &&
		a.ESPGroup == b.ESPGroup &&
		a.ConnectionType == b.ConnectionType &&
		a.LocalAddress == b.LocalAddress &&
		a.RemoteAddress == b.RemoteAddress &&
		a.VTIBind == b.VTIBind &&
		a.IfID == b.IfID &&
		a.Auth.Mode == b.Auth.Mode &&
		a.Auth.PSK == b.Auth.PSK &&
		a.Auth.LocalID == b.Auth.LocalID &&
		a.Auth.RemoteID == b.Auth.RemoteID &&
		a.Auth.CACertificate == b.Auth.CACertificate &&
		a.Auth.Certificate == b.Auth.Certificate
}
