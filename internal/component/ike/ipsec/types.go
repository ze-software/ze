// Design: docs/architecture/ike/ipsec-3-data-model.md -- IPsec data model types
// RFC: rfc/short/rfc7296.md -- identity types (Section 3.5), certificate payloads (Section 3.6)
// Related: algorithm_support.go -- the predicates deciding which of these enums a build implements
// Related: config.go -- the parser that fills these types from the config tree

package ipsec

import (
	"net/netip"
	"reflect"
)

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

// parseHashAlgo returns the HashAlgo for a YANG enum value.
func parseHashAlgo(s string) (HashAlgo, bool) {
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

// parsePFSMode returns the PFSMode for a YANG enum value.
func parsePFSMode(s string) (PFSMode, bool) {
	a, ok := pfsByName[s]
	return a, ok
}

// AuthMode discriminates between PSK and X.509 authentication.
//
// AuthEAPMD5 is the one mode that runs an EAP method establishing no shared
// key. RFC 7296 Section 2.16: "EAP methods that do not establish a shared key
// SHOULD NOT be used, as they are subject to a number of man-in-the-middle
// attacks". It is never a default and no other mode reaches it, so an operator
// holds it only by writing it, and the engine warns once when it is adopted
// (warnKeylessEAPModes, internal/component/ike/engine/eap_auth.go).
type AuthMode int

const (
	AuthUnknown AuthMode = iota
	AuthPreSharedSecret
	AuthX509
	AuthEAPTLS
	AuthEAPMSCHAPv2
	AuthEAPMD5
)

var authModeNames = map[AuthMode]string{ //nolint:gosec // enum name, not a credential
	AuthPreSharedSecret: "pre-shared-secret",
	AuthX509:            "x509",
	AuthEAPTLS:          "eap-tls",
	AuthEAPMSCHAPv2:     "eap-mschapv2",
	AuthEAPMD5:          "eap-md5",
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

// parseConnectionType returns the ConnectionType for a YANG enum value.
func parseConnectionType(s string) (ConnectionType, bool) {
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

// parseCloseAction returns the CloseAction for a YANG enum value.
func parseCloseAction(s string) (CloseAction, bool) {
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

// parseDPDAction returns the DPDAction for a YANG enum value.
func parseDPDAction(s string) (DPDAction, bool) {
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

// parseKeyExchange returns the KeyExchange for a YANG enum value.
func parseKeyExchange(s string) (KeyExchange, bool) {
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

// Equal reports whether two ESP groups are the same, including their proposals. A peer's
// SiteToSitePeer holds the group's NAME, so a peer comparison alone cannot see an operator
// rotating a cipher inside the group the peer points at. peerConfigChanged
// (engine/reconcile.go) asks this as well, because the running session holds the RESOLVED
// group and nothing refreshes it.
//
// Total for the same reason SiteToSitePeer.Equal is: a member added to the group, or to
// ESPProposal, is compared on the day it is added.
func (g ESPGroup) Equal(h ESPGroup) bool {
	return reflect.DeepEqual(g, h)
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

// Equal reports whether two IKE groups are the same, including their proposals and their
// DPD settings. It answers the same question ESPGroup.Equal answers, for the other half of
// a peer's crypto, and for the same reason: the peer holds a NAME and the running session
// holds the resolved group.
func (g IKEGroup) Equal(h IKEGroup) bool {
	return reflect.DeepEqual(g, h)
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

	// RemoteIDType pins the IKE ID type the peer must assert, as the type number
	// from RFC 7296 Section 3.5 (wire.IDType*). Zero means unset, which is the
	// default and keeps the historical behavior of accepting any comparable type.
	//
	// It is ACCEPT-SIDE ONLY. encodeIKEID still derives the type ze SENDS from
	// local-id alone. The reason is that strongSwan refuses an IP value sent as
	// ID_FQDN, and overloading one leaf across both directions would change that.
	RemoteIDType uint8

	// CertificateCount bounds the X.509 certificate chain in BOTH directions:
	// the most ze sends, and the most it accepts before it refuses the message.
	// RFC 7296 Section 3.6 sets the figure at four, and the YANG default is four
	// so an operator who never touches the leaf gets the conformant behavior.
	CertificateCount uint8

	// HashAndURL turns on the Hash and URL certificate encodings of RFC 7296
	// Section 3.6. Ze sends them, advertises HTTP_CERT_LOOKUP_SUPPORTED, and
	// resolves one a peer sends. HashAndURL defaults to FALSE, and that default
	// is a security property rather than a preference.
	//
	// Resolving a received payload means fetching a URL chosen by a peer that is
	// not yet authenticated. With the leaf off, ze advertises nothing. A
	// conforming peer then sends no such payload, and the collection loops drop a
	// non-conforming one. As a result, certurl.go is unreachable.
	HashAndURL bool

	// CertificateURL is the http URL at which ze's own certificate is published,
	// sent beside the SHA-1 in a Hash and URL CERT payload.
	CertificateURL string

	// CertificateURLAllow widens the fetcher's destination deny list. It is empty
	// by default, leaving loopback, private, link-local and metadata addresses
	// refused.
	CertificateURLAllow []netip.Prefix
}

// Equal reports whether two authentication configurations are the same. It is the ONE
// producer of that answer for the remote-access profile, and SiteToSitePeer.Equal reaches
// the same comparison for a peer through its Auth member. A reload decision therefore
// cannot disagree between a site-to-site peer and the remote-access profile.
//
// The comparison is deliberately structural rather than a list of field names. Every
// scalar field is compared by the `==` below, so a leaf added later is covered the day it
// is added. A leaf that was merely NAMED in a hand-written field list would be inert on
// reload. The operator edits it, the config commits, and `show configuration` agrees. But
// the session never renegotiates, because nothing noticed the change.
//
// reflect.DeepEqual is what makes that total. A hand-written field list is the shape this
// package already got wrong. The peer comparison named six auth fields, and
// remoteAccessEqual used struct equality. The two therefore disagreed about what "changed"
// meant, and a seventh field would have had to be remembered twice. Reload is a cold path,
// because it runs when an operator commits. The reflection therefore costs nothing that
// matters, and healthcheck/config.go and reactor_api.go already take the same trade for
// the same reason.
func (a AuthConfig) Equal(b AuthConfig) bool {
	return reflect.DeepEqual(a, b)
}

// DefaultCertificateCount is the X.509 certificate chain bound ze applies when the
// certificate-count leaf is absent. RFC 7296 Section 3.6 names four.
const DefaultCertificateCount uint8 = 4

// CertificateChainLimit reports the configured chain bound, substituting the RFC's
// figure when the leaf is unset. Every consumer reads the bound through this, so a
// zero-valued AuthConfig (a test fixture, a peer parsed before the leaf existed)
// cannot silently mean "no limit" (ai/rules/evidence.md).
func (a AuthConfig) CertificateChainLimit() uint8 {
	if a.CertificateCount == 0 {
		return DefaultCertificateCount
	}
	return a.CertificateCount
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

	// TrafficSelectors is the operator policy RFC 7296 Section 2.9 narrows a peer's
	// proposed TSi/TSr against. An EMPTY slice means "allow everything", which
	// preserves the behavior of every config written before this field existed: such a
	// peer accepts whatever the initiator proposes. It is the load-bearing default,
	// because narrowing an unconfigured peer to the empty set would answer every
	// existing deployment with TS_UNACCEPTABLE.
	TrafficSelectors []TrafficSelectorPolicy

	// Mode is the Child SA encapsulation mode this peer asks for: dataplane.ModeTunnel
	// or dataplane.ModeTransport. RFC 7296 Section 1.3.1: "Except when using this option
	// to negotiate transport mode, all Child SAs will use tunnel mode", so tunnel is the
	// default and it is the RFC's own default rather than a Ze preference.
	Mode uint8

	// TransportRequired records that tunnel mode is unacceptable to this peer.
	//
	// RFC 7296 Section 1.3.1: "If the responder declines the request, the Child SA will
	// be established in tunnel mode. If this is unacceptable to the initiator, the
	// initiator MUST delete the SA." Only the operator knows whether it is unacceptable,
	// so the MUST is conditional on this leaf. It fails SAFE rather than closed on
	// purpose: the default false keeps a declined request as a working tunnel-mode SA,
	// and setting it true is the operator stating that a silent downgrade to tunnel mode
	// is worse than no tunnel at all.
	TransportRequired bool

	// PolicyPriority is where this peer's SPD entries sit in the operator's ordering of
	// the Security Policy Database. LOWER VALUE MEANS HIGHER PRECEDENCE, in ze and in
	// the kernel (dataplane.PriorityChildSA).
	//
	// RFC 4301 Section 4.4.1: "The ordering requirement arises because entries often
	// will overlap due to the presence of (non-trivial) ranges as values for selectors.
	// Thus, a user or administrator MUST be able to order the entries to express a
	// desired access control policy." The same section binds the management interface:
	// it "MUST support (total) ordering of these entries, as seen via this interface".
	//
	// Every peer took the one constant dataplane.PriorityChildSA before this member
	// existed, so two peers whose selectors overlap reached the kernel at equal rank and
	// the winner was whichever established last. This member is the operator's answer.
	// parseSiteToSitePeer resolves an absent leaf to that same constant, so a peer that
	// states no order installs what it installed before.
	//
	// ValidatePolicyOrder refuses a rank that outranks the IKE control-plane bypass.
	//
	// It sits LAST on purpose. Mode and TransportRequired above are one octet each, so
	// the struct already carried six octets of tail padding and this member fits inside
	// it. Moved anywhere else it adds eight octets, the struct reaches 288, and every
	// function taking a peer by value crosses the gocritic hugeParam threshold
	// (TestSiteToSitePeerStaysUnderTheHugeParamThreshold holds it).
	PolicyPriority uint32
}

// Equal reports whether two site-to-site peer configurations are the same. It is the ONE
// producer of that answer: peerConfigChanged (engine/reconcile.go) asks it whether a
// reload gave a running session a configuration it is no longer serving, and Changed asks
// it which peers a new config file moved. The two therefore cannot disagree about what
// "changed" means.
//
// The comparison is TOTAL, and that is the point. It subtracts no member, so a member
// added to SiteToSitePeer is compared on the day it is added and the conservative answer,
// which is "this peer changed", is what an unclassified member draws.
//
// The alternative shape is a list of field names, and this package got that shape wrong
// twice over. The reload guard named eight members and this function named a DIFFERENT
// eight, so the two disagreed, and TrafficSelectors, Mode and TransportRequired were in
// neither. An operator narrowed a live peer's traffic selectors, the commit succeeded,
// `show configuration` agreed, and the tunnel kept carrying the prefix the edit removed.
//
// Subtracting a member later is allowed, and it MUST be done by NAME with the reason
// stated here. A member absorbed by a running session without a restart, or one that
// changes on every parse without an operator edit, is a decision somebody made. A member
// nobody thought about is not, and omission MUST NOT be how the two are told apart.
// TestPeerConfigChangedIsFailClosed (engine/reconcile_test.go) walks every member, so a
// member added tomorrow that this comment does not classify reddens that test.
//
// reflect.DeepEqual compares every member, and it follows the *net.IPNet in each
// TrafficSelectorPolicy to the prefix it points at rather than comparing pointers.
// Reload is a cold path, because it runs when an operator commits, so the reflection costs
// nothing that matters. AuthConfig.Equal above takes the same trade for the same reason.
func (p SiteToSitePeer) Equal(q SiteToSitePeer) bool {
	return reflect.DeepEqual(p, q)
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
		if !oldPeer.Equal(newPeers[name]) {
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
	if !a.Auth.Equal(b.Auth) {
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
