// Design: plan/learned/739-ipsec-6-ikev2-crypto.md -- IKEv2 transform type registry

package crypto

import (
	"errors"
	"slices"
	"sort"
)

// RFC 7296 Section 3.3.2: Transform Type Values.
type TransformType uint8

const (
	TransformTypeENCR  TransformType = 1
	TransformTypePRF   TransformType = 2
	TransformTypeINTEG TransformType = 3
	TransformTypeDH    TransformType = 4
	TransformTypeESN   TransformType = 5
)

// RFC 7296 Section 3.3.3: Encryption Algorithm Transform IDs.
type EncryptionID uint16

const (
	ENCR_AES_CBC    EncryptionID = 12
	ENCR_AES_GCM_16 EncryptionID = 20
)

// RFC 7296 Section 3.3.4: PRF Transform IDs.
type PRFID uint16

const (
	PRF_HMAC_SHA2_256 PRFID = 5
	PRF_HMAC_SHA2_384 PRFID = 6
	PRF_HMAC_SHA2_512 PRFID = 7
)

// RFC 7296 Section 3.3.5: Integrity Algorithm Transform IDs.
type IntegrityID uint16

const (
	AUTH_NONE              IntegrityID = 0
	AUTH_HMAC_SHA2_256_128 IntegrityID = 12
	AUTH_HMAC_SHA2_384_192 IntegrityID = 13
	AUTH_HMAC_SHA2_512_256 IntegrityID = 14
)

// RFC 7296 Section 3.3.6: DH Group Transform IDs.
type DHGroupID uint16

const (
	DH_MODP_2048 DHGroupID = 14
	DH_ECP_256   DHGroupID = 19
	DH_ECP_384   DHGroupID = 20
)

var ErrUnsupportedAlgorithm = errors.New("unsupported algorithm")

const unknownAlgo = "unknown"

func (id EncryptionID) String() string {
	switch id {
	case ENCR_AES_CBC:
		return "aes-cbc"
	case ENCR_AES_GCM_16:
		return "aes-gcm"
	default:
		return unknownAlgo
	}
}

func (id IntegrityID) String() string { //nolint:goconst // display names intentionally match registry keys
	switch id {
	case AUTH_NONE:
		return "none"
	case AUTH_HMAC_SHA2_256_128:
		return "sha256"
	case AUTH_HMAC_SHA2_384_192:
		return "sha384"
	case AUTH_HMAC_SHA2_512_256:
		return "sha512"
	default:
		return unknownAlgo
	}
}

func (id DHGroupID) String() string {
	switch id {
	case DH_MODP_2048:
		return "modp2048"
	case DH_ECP_256:
		return "ecp256"
	case DH_ECP_384:
		return "ecp384"
	default:
		return unknownAlgo
	}
}

// aeadEncryption names every encryption transform that combines integrity with
// encryption. It is the one place this property is written down for a wire Transform
// ID. Every site that decides the property asks EncryptionID.IsAEAD, and the tests in
// aead_predicate_test.go enumerate this map to prove it.
//
// The comment above once said the same thing. ikeProposalComplete kept a private copy
// that compared against ENCR_AES_GCM_16 alone. An entry added here therefore produced a
// cipher that keyed correctly, and negotiation then refused it with
// ErrProposalIncomplete.
//
// Two neighboring maps are NOT this property, and neither is derived from it. A new
// AEAD cipher needs an entry in each. specifiedEncryption (proposal.go) lists the IDs
// this build accepts off the wire. encryptionRegistry (below) maps a config name to a
// transform. ipsec.EncryptionAlgo.IsAEAD answers the same question over the config
// enum rather than the wire ID. A test in that package binds the two.
//
// The value is the salt the cipher takes beyond its key, in octets. A future AEAD
// whose salt is not four octets records it here. The salt was once a package constant
// applied to every AEAD. That is correct while AES-GCM is the only entry, and it gives
// a wrong key length silently for the first cipher that differs. Ze holds no RFC text
// for the AES-CCM or the ChaCha20-Poly1305 salt, so neither is asserted here. Read RFC
// 4309 or RFC 7634 first.
var aeadSaltBytes = map[EncryptionID]int{
	ENCR_AES_GCM_16: 4, // RFC 4106 Section 8.1
}

// IsAEAD reports whether this encryption transform combines integrity with
// encryption. RFC 7296 Section 3.3 makes an integrity transform of NONE the correct
// value for such a cipher.
//
// The verdict comes from the Transform ID, which is the identity the peer put on the
// wire. A caller that holds an EncryptionTransform asks this method on the ID rather
// than read the IsAEAD field. The field is a cached view. A construction site can
// leave it at its zero value, and that false value reads as a valid "not AEAD"
// answer (ai/rules/fail-closed-guards.md). The ID cannot lie in that way.
//
// Membership in aeadSaltBytes IS the AEAD property. A miss means the cipher is not
// AEAD. A hit gives that cipher's own salt. Neither answer can be a zero value that
// reads as valid.
func (id EncryptionID) IsAEAD() bool {
	_, ok := aeadSaltBytes[id]
	return ok
}

// aeadSalt returns the salt this cipher takes beyond its key, in octets, and zero for
// a cipher that is not AEAD.
func (id EncryptionID) aeadSalt() int {
	return aeadSaltBytes[id]
}

// EncryptionTransform names one encryption algorithm and the key it takes.
//
// IsAEAD caches the verdict of EncryptionID.IsAEAD for the ID this transform holds.
// Build a transform with NewEncryptionTransform to keep the two in agreement. Any
// decision that must be correct reads ID.IsAEAD, never the field.
type EncryptionTransform struct {
	ID        EncryptionID
	KeyLength uint16 // in bits
	IsAEAD    bool
}

// NewEncryptionTransform builds a transform whose IsAEAD field agrees with its ID.
// Every site that learns an encryption ID at run time uses it, so no site has to
// remember the AEAD property on its own.
func NewEncryptionTransform(id EncryptionID, keyLengthBits uint16) EncryptionTransform {
	return EncryptionTransform{ID: id, KeyLength: keyLengthBits, IsAEAD: id.IsAEAD()}
}

type PRFTransform struct {
	ID           PRFID
	KeyLength    uint16 // in bytes
	OutputLength uint16 // in bytes
}

type IntegrityTransform struct {
	ID              IntegrityID
	KeyLength       uint16 // in bytes
	TruncatedLength uint16 // ICV length in bytes
}

type DHGroupTransform struct {
	ID DHGroupID
}

// encryptionRegistry maps each configured algorithm name to its transform. The
// entries name an ID and a key length only. NewEncryptionTransform fills IsAEAD from
// the ID, so an entry added later cannot disagree with the AEAD predicate.
var encryptionRegistry = buildEncryptionRegistry()

func buildEncryptionRegistry() map[string]EncryptionTransform {
	return map[string]EncryptionTransform{
		"aes128":    NewEncryptionTransform(ENCR_AES_CBC, 128),
		"aes256":    NewEncryptionTransform(ENCR_AES_CBC, 256),
		"aes128gcm": NewEncryptionTransform(ENCR_AES_GCM_16, 128),
		"aes256gcm": NewEncryptionTransform(ENCR_AES_GCM_16, 256),
	}
}

var prfRegistry = map[string]PRFTransform{
	"sha256": {ID: PRF_HMAC_SHA2_256, KeyLength: 32, OutputLength: 32},
	"sha384": {ID: PRF_HMAC_SHA2_384, KeyLength: 48, OutputLength: 48},
	"sha512": {ID: PRF_HMAC_SHA2_512, KeyLength: 64, OutputLength: 64},
}

var integrityRegistry = map[string]IntegrityTransform{
	"sha256": {ID: AUTH_HMAC_SHA2_256_128, KeyLength: 32, TruncatedLength: 16},
	"sha384": {ID: AUTH_HMAC_SHA2_384_192, KeyLength: 48, TruncatedLength: 24},
	"sha512": {ID: AUTH_HMAC_SHA2_512_256, KeyLength: 64, TruncatedLength: 32},
}

var dhGroupRegistry = map[uint8]DHGroupTransform{
	14: {ID: DH_MODP_2048},
	19: {ID: DH_ECP_256},
	20: {ID: DH_ECP_384},
}

func LookupEncryption(name string) (EncryptionTransform, error) {
	t, ok := encryptionRegistry[name]
	if !ok {
		return EncryptionTransform{}, ErrUnsupportedAlgorithm
	}
	return t, nil
}

// SupportedEncryptionNames lists every encryption algorithm this build implements, in
// sorted order. The config parser names the list in the error it returns for an
// algorithm it refuses, so the two can never disagree (ai/rules/derive-not-hardcode.md).
func SupportedEncryptionNames() []string {
	return sortedKeys(encryptionRegistry)
}

// SupportedIntegrityNames lists every integrity algorithm this build implements, in
// sorted order. SupportedEncryptionNames gives the reason it is derived.
func SupportedIntegrityNames() []string {
	return sortedKeys(integrityRegistry)
}

// SupportedPRFNames lists every PRF this build implements, in sorted order.
func SupportedPRFNames() []string {
	return sortedKeys(prfRegistry)
}

// SupportedDHGroupIDs lists every Diffie-Hellman group this build implements, in
// ascending order. RFC 7296 Section 3.3.2 assigns Transform Type 4 a far wider number
// space than any build carries. The config parser therefore names this list in the error
// it returns for a group it refuses, so the two can never disagree
// (ai/rules/derive-not-hardcode.md).
func SupportedDHGroupIDs() []uint8 {
	ids := make([]uint8, 0, len(dhGroupRegistry))
	for id := range dhGroupRegistry {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func sortedKeys[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func LookupPRF(name string) (PRFTransform, error) {
	t, ok := prfRegistry[name]
	if !ok {
		return PRFTransform{}, ErrUnsupportedAlgorithm
	}
	return t, nil
}

func LookupIntegrity(name string) (IntegrityTransform, error) {
	t, ok := integrityRegistry[name]
	if !ok {
		return IntegrityTransform{}, ErrUnsupportedAlgorithm
	}
	return t, nil
}

func LookupDHGroup(id uint8) (DHGroupTransform, error) {
	t, ok := dhGroupRegistry[id]
	if !ok {
		return DHGroupTransform{}, ErrUnsupportedAlgorithm
	}
	return t, nil
}
