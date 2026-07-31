// Design: plan/learned/739-ipsec-6-ikev2-crypto.md -- SKEYSEED and SK_* key derivation

package crypto

// encKeyMaterialLen returns the KEYMAT one direction takes for this cipher, in octets.
//
// An AEAD cipher takes a salt beyond its key. RFC 4106 Section 8.1: "The size of the
// KEYMAT for the AES-GCM-ESP MUST be four octets longer than is needed for the
// associated AES key." RFC 5282 Section 4 carries the same rule into the IKE SA.
// AES-GCM-256 therefore takes 36 octets and AES-GCM-128 takes 20.
//
// The salt is read per algorithm from aeadSaltBytes. It never comes from one number
// shared by every AEAD. A cipher whose salt is not four octets therefore cannot be
// keyed silently at the wrong length.
//
// The AEAD verdict comes from the Transform ID, never from the IsAEAD field. A caller
// that fills the ID and leaves the field at its zero value still gets the salt. The
// Linux kernel refuses a short key. A short key also makes a peer read the second
// direction's key at the wrong offset (ai/rules/fail-closed-guards.md).
func encKeyMaterialLen(enc EncryptionTransform) int {
	return int(enc.KeyLength)/8 + enc.ID.aeadSalt()
}

// SKKeys holds the IKE SA key hierarchy derived from SKEYSEED.
type SKKeys struct {
	SK_d  []byte // Key derivation key for Child SA KEYMAT.
	SK_ai []byte // Initiator integrity key.
	SK_ar []byte // Responder integrity key.
	SK_ei []byte // Initiator encryption key.
	SK_er []byte // Responder encryption key.
	SK_pi []byte // Initiator AUTH payload key.
	SK_pr []byte // Responder AUTH payload key.
}

func (k *SKKeys) Clear() {
	clear(k.SK_d)
	clear(k.SK_ai)
	clear(k.SK_ar)
	clear(k.SK_ei)
	clear(k.SK_er)
	clear(k.SK_pi)
	clear(k.SK_pr)
}

// DeriveSKEYSEED computes the initial IKE SA seed key.
// RFC 7296 Section 2.14: SKEYSEED = prf(Ni | Nr, g^ir).
func DeriveSKEYSEED(prfID PRFID, ni, nr, sharedSecret []byte) ([]byte, error) {
	nonceKey := append(append([]byte(nil), ni...), nr...)
	return PRF(prfID, nonceKey, sharedSecret)
}

// DeriveRekeyedSKEYSEED computes the seed key for a rekeyed IKE SA.
// RFC 7296 Section 2.18: SKEYSEED = prf(SK_d_old, g^ir_new | Ni | Nr).
func DeriveRekeyedSKEYSEED(prfID PRFID, skDOld, newSharedSecret, ni, nr []byte) ([]byte, error) {
	data := append(append(append([]byte(nil), newSharedSecret...), ni...), nr...)
	return PRF(prfID, skDOld, data)
}

// DeriveSKKeys expands SKEYSEED into the full SK_* key hierarchy.
// RFC 7296 Section 2.14:
// {SK_d | SK_ai | SK_ar | SK_ei | SK_er | SK_pi | SK_pr} =
//
//	prf+(SKEYSEED, Ni | Nr | SPIi | SPIr).
func DeriveSKKeys(prfID PRFID, skeyseed, ni, nr, spiI, spiR []byte, enc EncryptionTransform, integ IntegrityTransform) (*SKKeys, error) {
	prf, err := LookupPRF(prfIDToName(prfID))
	if err != nil {
		return nil, err
	}

	skDLen := int(prf.OutputLength)
	integKeyLen := int(integ.KeyLength)
	encKeyLen := encKeyMaterialLen(enc)
	prfKeyLen := int(prf.KeyLength)

	totalLen := skDLen + 2*integKeyLen + 2*encKeyLen + 2*prfKeyLen

	seed := make([]byte, 0, len(ni)+len(nr)+len(spiI)+len(spiR))
	seed = append(seed, ni...)
	seed = append(seed, nr...)
	seed = append(seed, spiI...)
	seed = append(seed, spiR...)

	keymat, err := PRFPlus(prfID, skeyseed, seed, totalLen)
	if err != nil {
		return nil, err
	}

	keys := &SKKeys{}
	off := 0
	keys.SK_d = dup(keymat[off : off+skDLen])
	off += skDLen
	keys.SK_ai = dup(keymat[off : off+integKeyLen])
	off += integKeyLen
	keys.SK_ar = dup(keymat[off : off+integKeyLen])
	off += integKeyLen
	keys.SK_ei = dup(keymat[off : off+encKeyLen])
	off += encKeyLen
	keys.SK_er = dup(keymat[off : off+encKeyLen])
	off += encKeyLen
	keys.SK_pi = dup(keymat[off : off+prfKeyLen])
	off += prfKeyLen
	keys.SK_pr = dup(keymat[off : off+prfKeyLen])

	clear(keymat)
	return keys, nil
}

// ChildSAKeys holds ESP keying material for one direction.
type ChildSAKeys struct {
	EncryptKeyI []byte
	IntegKeyI   []byte
	EncryptKeyR []byte
	IntegKeyR   []byte
}

func (k *ChildSAKeys) Clear() {
	clear(k.EncryptKeyI)
	clear(k.IntegKeyI)
	clear(k.EncryptKeyR)
	clear(k.IntegKeyR)
}

// DeriveChildSAKeys derives ESP keys from SK_d and nonces.
// RFC 7296 Section 2.17: KEYMAT = prf+(SK_d, Ni | Nr).
//
// An AEAD cipher takes four octets of salt beyond its key (RFC 4106 Section 8.1), so
// encKeyMaterialLen decides the length of each of the four keys below.
func DeriveChildSAKeys(prfID PRFID, skD, ni, nr []byte, enc EncryptionTransform, integ IntegrityTransform) (*ChildSAKeys, error) {
	encKeyLen := encKeyMaterialLen(enc)
	integKeyLen := int(integ.KeyLength)

	totalLen := 2*encKeyLen + 2*integKeyLen

	seed := append(append([]byte(nil), ni...), nr...)

	keymat, err := PRFPlus(prfID, skD, seed, totalLen)
	if err != nil {
		return nil, err
	}

	keys := &ChildSAKeys{}
	off := 0
	keys.EncryptKeyI = dup(keymat[off : off+encKeyLen])
	off += encKeyLen
	keys.IntegKeyI = dup(keymat[off : off+integKeyLen])
	off += integKeyLen
	keys.EncryptKeyR = dup(keymat[off : off+encKeyLen])
	off += encKeyLen
	keys.IntegKeyR = dup(keymat[off : off+integKeyLen])

	clear(keymat)
	return keys, nil
}

// DeriveChildSAKeysPFS derives ESP keys with Perfect Forward Secrecy.
// RFC 7296 Section 2.17: KEYMAT = prf+(SK_d, g^ir | Ni | Nr).
//
// The AEAD salt rule of DeriveChildSAKeys applies here without change.
func DeriveChildSAKeysPFS(prfID PRFID, skD, dhSharedSecret, ni, nr []byte, enc EncryptionTransform, integ IntegrityTransform) (*ChildSAKeys, error) {
	encKeyLen := encKeyMaterialLen(enc)
	integKeyLen := int(integ.KeyLength)

	totalLen := 2*encKeyLen + 2*integKeyLen

	seed := make([]byte, 0, len(dhSharedSecret)+len(ni)+len(nr))
	seed = append(seed, dhSharedSecret...)
	seed = append(seed, ni...)
	seed = append(seed, nr...)

	keymat, err := PRFPlus(prfID, skD, seed, totalLen)
	if err != nil {
		return nil, err
	}

	keys := &ChildSAKeys{}
	off := 0
	keys.EncryptKeyI = dup(keymat[off : off+encKeyLen])
	off += encKeyLen
	keys.IntegKeyI = dup(keymat[off : off+integKeyLen])
	off += integKeyLen
	keys.EncryptKeyR = dup(keymat[off : off+encKeyLen])
	off += encKeyLen
	keys.IntegKeyR = dup(keymat[off : off+integKeyLen])

	clear(keymat)
	return keys, nil
}

func dup(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

func prfIDToName(id PRFID) string {
	switch id {
	case PRF_HMAC_SHA2_256:
		return "sha256"
	case PRF_HMAC_SHA2_384:
		return "sha384"
	case PRF_HMAC_SHA2_512:
		return "sha512"
	default:
		return ""
	}
}
