// Design: plan/spec-ipsec-6-ikev2-crypto.md -- IKEv2 transform type registry

package crypto

import "errors"

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

type EncryptionTransform struct {
	ID        EncryptionID
	KeyLength uint16 // in bits
	IsAEAD    bool
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

var encryptionRegistry = map[string]EncryptionTransform{
	"aes128":    {ID: ENCR_AES_CBC, KeyLength: 128, IsAEAD: false},
	"aes256":    {ID: ENCR_AES_CBC, KeyLength: 256, IsAEAD: false},
	"aes128gcm": {ID: ENCR_AES_GCM_16, KeyLength: 128, IsAEAD: true},
	"aes256gcm": {ID: ENCR_AES_GCM_16, KeyLength: 256, IsAEAD: true},
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
