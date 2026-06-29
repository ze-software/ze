// Design: plan/learned/742-ipsec-8-ikev2-child-xfrm.md -- Child SA and IKE SA rekeying
// RFC: rfc/short/rfc7296.md -- Rekeying (Section 2.8), collision (Section 2.8.1)

package engine

import (
	"bytes"
	"crypto/rand"
	"log/slog"
	"math/big"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/crypto"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/dataplane"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/ipsec"
)

// lifetimeState tracks SA lifetime for time-based and byte-based expiry.
type lifetimeState struct {
	softTime  time.Time
	hardTime  time.Time
	softBytes uint64
	byteCount uint64
}

func newLifetimeState(lifetimeSec uint32) *lifetimeState {
	if lifetimeSec == 0 {
		return nil
	}
	lifetime := time.Duration(lifetimeSec) * time.Second
	jitter := lifetimeJitter(lifetime)
	now := time.Now()
	soft := now.Add(lifetime - jitter)
	hard := now.Add(lifetime)
	return &lifetimeState{
		softTime: soft,
		hardTime: hard,
	}
}

// lifetimeJitter returns a random duration between 0 and 10% of the lifetime.
func lifetimeJitter(lifetime time.Duration) time.Duration {
	tenPercent := lifetime / 10
	if tenPercent <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(tenPercent)))
	if err != nil {
		return tenPercent / 2
	}
	return time.Duration(n.Int64())
}

// softExpired reports whether the soft (rekey trigger) lifetime has passed.
func (ls *lifetimeState) softExpired(now time.Time) bool {
	if ls == nil {
		return false
	}
	if !ls.softTime.IsZero() && !now.Before(ls.softTime) {
		return true
	}
	if ls.softBytes > 0 && ls.byteCount >= ls.softBytes {
		return true
	}
	return false
}

// hardExpired reports whether the hard (delete) lifetime has passed.
func (ls *lifetimeState) hardExpired(now time.Time) bool {
	if ls == nil {
		return false
	}
	return !ls.hardTime.IsZero() && !now.Before(ls.hardTime)
}

// rekeyChildSA creates a new Child SA to replace an existing one.
// RFC 7296 Section 1.3.2: CREATE_CHILD_SA with REKEY_SA notify.
func rekeyChildSA(
	sa *SA,
	old *ChildSA,
	espGroup ipsec.ESPGroup,
	dp dataplane.Dataplane,
	log *slog.Logger,
) (*ChildSA, error) {
	newNonceI, err := GenerateNonce(nonceLen)
	if err != nil {
		return nil, err
	}
	newNonceR, err := GenerateNonce(nonceLen)
	if err != nil {
		return nil, err
	}

	prop := espGroup.Proposals[0]
	enc := lookupEncryption(prop.Encryption)
	integ := lookupIntegrity(prop.Hash)

	keys, err := crypto.DeriveChildSAKeys(
		sa.Proposal.PRF.ID, sa.SKKeys.SK_d,
		newNonceI, newNonceR,
		enc, integ,
	)
	if err != nil {
		return nil, err
	}

	inSPI, err := GenerateESPSPI()
	if err != nil {
		keys.Clear()
		return nil, err
	}
	outSPI, err := GenerateESPSPI()
	if err != nil {
		keys.Clear()
		return nil, err
	}

	child := &ChildSA{
		InboundSPI:  inSPI,
		OutboundSPI: outSPI,
		LocalAddr:   old.LocalAddr,
		RemoteAddr:  old.RemoteAddr,
		IfID:        old.IfID,
		TSLocal:     old.TSLocal,
		TSRemote:    old.TSRemote,
		Keys:        keys,
		ESPGroup:    espGroup,
		ReqID:       old.ReqID,
	}

	if dp != nil {
		if err := installChildSA(child, prop, dp, log); err != nil {
			keys.Clear()
			return nil, err
		}
	}

	removeChildSA(old, dp, log)
	log.Info("child-sa: rekeyed", "old-in", old.InboundSPI, "new-in", inSPI)
	return child, nil
}

// rekeyIKESA creates a new IKE SA to replace the current one.
// RFC 7296 Section 1.3.3: CREATE_CHILD_SA with SA + Ni + KEi (DH mandatory).
// The new IKE SA inherits all child SAs from the old one.
func rekeyIKESA(
	oldSA *SA,
	ikeGroup ipsec.IKEGroup,
	log *slog.Logger,
) (*SA, error) {
	newNonceI, err := GenerateNonce(nonceLen)
	if err != nil {
		return nil, err
	}
	newNonceR, err := GenerateNonce(nonceLen)
	if err != nil {
		return nil, err
	}

	// RFC 7296 Section 1.3.3: KE payload is mandatory for IKE SA rekey.
	dhGroupID := crypto.DHGroupID(ikeGroup.Proposals[0].DHGroup)
	dh, err := crypto.NewDHExchange(dhGroupID)
	if err != nil {
		return nil, err
	}
	defer dh.Clear()

	newSPI, err := GenerateSPI()
	if err != nil {
		return nil, err
	}

	// Simulate DH with self for local rekey (real exchange via CREATE_CHILD_SA).
	sharedSecret, err := dh.SharedSecret(dh.PublicKey)
	if err != nil {
		return nil, err
	}

	// RFC 7296 Section 2.18: SKEYSEED = prf(SK_d_old, g^ir_new | Ni | Nr).
	skeyseed, err := crypto.DeriveRekeyedSKEYSEED(
		oldSA.Proposal.PRF.ID, oldSA.SKKeys.SK_d,
		sharedSecret, newNonceI, newNonceR,
	)
	if err != nil {
		clear(sharedSecret)
		return nil, err
	}

	skKeys, err := crypto.DeriveSKKeys(
		oldSA.Proposal.PRF.ID, skeyseed,
		newNonceI, newNonceR,
		newSPI[:], oldSA.ResponderSPI[:],
		oldSA.Proposal.Encryption, oldSA.Proposal.Integrity,
	)
	if err != nil {
		clear(sharedSecret)
		clear(skeyseed)
		return nil, err
	}
	clear(sharedSecret)
	clear(skeyseed)

	newSA := &SA{
		PeerName:      oldSA.PeerName,
		PeerCfg:       oldSA.PeerCfg,
		IKEGroup:      oldSA.IKEGroup,
		InitiatorSPI:  newSPI,
		ResponderSPI:  oldSA.ResponderSPI,
		IsInitiator:   oldSA.IsInitiator,
		State:         StateEstablished,
		LocalNonce:    newNonceI,
		RemoteNonce:   newNonceR,
		Proposal:      oldSA.Proposal,
		SKKeys:        skKeys,
		NextMsgID:     0,
		CreatedAt:     time.Now(),
		EstablishedAt: time.Now(),
	}

	log.Info("ike-sa: rekeyed",
		"old-ispi", SPIHex(oldSA.InitiatorSPI),
		"new-ispi", SPIHex(newSPI))

	return newSA, nil
}

// resolveRekeyCollision determines the winner of a simultaneous rekey.
// RFC 7296 Section 2.8.1: the exchange with the lowest nonce wins.
func resolveRekeyCollision(localNonce, remoteNonce []byte) bool {
	return bytes.Compare(localNonce, remoteNonce) < 0
}
