// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- fixed-key ESP negotiation and rekey
package engine

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestESPFixedWireNegotiation drives both the initiator's accepted-offer check
// and responder selection with the actual encoder, including malformed attributes.
func TestESPFixedWireNegotiation(t *testing.T) {
	for _, p := range []ipsec.ESPProposal{
		{Number: 1, Encryption: ipsec.EncryptionChaCha20Poly},
		{Number: 1, Encryption: ipsec.Encryption3DES, Hash: ipsec.HashSHA1},
	} {
		t.Run(p.Encryption.String(), func(t *testing.T) {
			wp, err := espProposalWire(p, 0x12345678, 1, dhGroupNone)
			if err != nil {
				t.Fatal(err)
			}
			if len(wp.Transforms[0].Attrs) != 0 {
				t.Fatal("fixed cipher emitted Key Length attribute")
			}
			wantID := uint16(3)
			if p.Encryption.IsAEAD() {
				wantID = 28
			}
			if wp.Transforms[0].ID != wantID {
				t.Fatalf("ENCR id %d, want %d", wp.Transforms[0].ID, wantID)
			}
			enc, integ, err := resolveESPTransforms(p)
			if err != nil {
				t.Fatal(err)
			}
			local := []crypto.ESPProposal{{Number: 1, Encryption: enc, Integrity: integ}}
			if _, err := crypto.NegotiateESP(wireProposalsToESP([]wire.Proposal{wp}), local); err != nil {
				t.Fatal(err)
			}
			if _, ok, err := matchOfferedESP(&wire.PayloadSA{Proposals: []wire.Proposal{wp}}, p, espDHMatch{}); err != nil || !ok {
				t.Fatalf("responder: %v, %v", ok, err)
			}
			for _, length := range []uint16{0, 192, 256, 65535} {
				bad := wp
				bad.Transforms = append([]wire.Transform(nil), wp.Transforms...)
				bad.Transforms[0].Attrs = []wire.TransformAttr{{Type: wire.AttrTypeKeyLength, Value: length}}
				if _, ok, err := matchOfferedESP(&wire.PayloadSA{Proposals: []wire.Proposal{bad}}, p, espDHMatch{}); err != nil || ok {
					t.Fatalf("accepted attribute %d: %v, %v", length, ok, err)
				}
			}
			if p.Encryption.IsAEAD() {
				wp.Transforms = append(wp.Transforms, wire.Transform{Type: wire.TransformTypeINTG, ID: 2})
				if _, ok, err := matchOfferedESP(&wire.PayloadSA{Proposals: []wire.Proposal{wp}}, p, espDHMatch{}); err != nil || ok {
					t.Fatalf("accepted separate AEAD integrity: %v, %v", ok, err)
				}
				if _, err := crypto.NegotiateESP(wireProposalsToESP([]wire.Proposal{wp}), local); err == nil {
					t.Fatal("initiator accepted separate AEAD integrity")
				}
			}
		})
	}
}

// TestESPFixedChildRekeyBothRoles drives the real encrypted CREATE_CHILD_SA
// exchange in both IKE roles, with and without PFS, and checks installed keys.
// The IKE SA deliberately retains its original AES ESP group. The per-child group
// is authoritative for rekey, even when it differs from that original offer.
func TestESPFixedChildRekeyBothRoles(t *testing.T) {
	for _, p := range []ipsec.ESPProposal{
		{Number: 1, Encryption: ipsec.EncryptionChaCha20Poly},
		{Number: 1, Encryption: ipsec.Encryption3DES, Hash: ipsec.HashSHA1},
	} {
		for _, reverse := range []bool{false, true} {
			for _, pfs := range []ipsec.PFSMode{ipsec.PFSDisable, ipsec.PFSEnable} {
				r := ctrSetup(t, false, false)
				group := testESPGroup()
				group.Proposals = []ipsec.ESPProposal{p}
				group.PFS = pfs
				log := slogutil.DiscardLogger()
				iniDP, respDP := &mockDP{}, &mockDP{}
				first, err := createFirstChildSA(r.ini, group, r.ini.PeerCfg.LocalAddress, r.ini.PeerCfg.RemoteAddress, 1, iniDP, log)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(first.Clear)
				other, err := createFirstChildSA(r.resp, group, r.resp.PeerCfg.LocalAddress, r.resp.PeerCfg.RemoteAddress, 1, respDP, log)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(other.Clear)
				local, peer := r.ini, r.resp
				if reverse {
					local, peer, first, other = peer, local, other, first
				}
				raw, pending, err := initiateChildRekey(local, first)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(pending.clear)
				request := mbDecrypt(t, peer, raw)
				offered := false
				for _, entry := range request {
					payload, ok := entry.Payload.(*wire.PayloadSA)
					if !ok {
						continue
					}
					if len(payload.Proposals) != 1 {
						t.Fatalf("%s reverse=%v pfs=%v: rekey offered %d proposals, want the child's single selection",
							p.Encryption, reverse, pfs, len(payload.Proposals))
					}
					wantID := uint16(3)
					if p.Encryption.IsAEAD() {
						wantID = 28
					}
					for _, transform := range payload.Proposals[0].Transforms {
						if transform.Type != wire.TransformTypeENCR {
							continue
						}
						if transform.ID != wantID || len(transform.Attrs) != 0 {
							t.Fatalf("%s reverse=%v pfs=%v: rekey offered ENCR %+v, want fixed-key ID %d without attributes",
								p.Encryption, reverse, pfs, transform, wantID)
						}
						offered = true
					}
				}
				if !offered {
					t.Fatal("rekey request has no encryption offer")
				}
				answer, replacementPeer, err := respondChildRekey(peer, request, other, parseMsg(t, raw).Header.MessageID, respDP, log)
				if err != nil {
					t.Fatalf("%s reverse=%v pfs=%v: %v", p.Encryption, reverse, pfs, err)
				}
				t.Cleanup(replacementPeer.Clear)
				replacement, err := applyChildRekeyResponse(local, pending, mbDecrypt(t, local, answer), iniDP, log)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(replacement.Clear)
				if replacement.ESPGroup.Proposals[0].Encryption != p.Encryption || replacementPeer.ESPGroup.Proposals[0].Encryption != p.Encryption {
					t.Fatal("rekey changed selected cipher")
				}
				if !bytes.Equal(replacement.Keys.EncryptKeyI, replacementPeer.Keys.EncryptKeyI) || !bytes.Equal(replacement.Keys.EncryptKeyR, replacementPeer.Keys.EncryptKeyR) {
					t.Fatal("rekey peers derived different encryption keys")
				}
				if !bytes.Equal(replacement.Keys.IntegKeyI, replacementPeer.Keys.IntegKeyI) || !bytes.Equal(replacement.Keys.IntegKeyR, replacementPeer.Keys.IntegKeyR) {
					t.Fatal("rekey peers derived different integrity keys")
				}
				if bytes.Equal(first.Keys.EncryptKeyI, replacement.Keys.EncryptKeyI) {
					t.Fatal("rekey reused old encryption key")
				}
				encLen, authLen := 24, 20
				if p.Encryption.IsAEAD() {
					encLen, authLen = 36, 0
				}
				for _, dp := range []*mockDP{iniDP, respDP} {
					if len(dp.sas) != 4 {
						t.Fatalf("installed %d states, want initial and replacement pairs", len(dp.sas))
					}
					for _, sa := range dp.sas {
						if sa.EncAlgo != p.Encryption.String() || sa.IsAEAD != p.Encryption.IsAEAD() || len(sa.EncKey) != encLen || len(sa.AuthKey) != authLen {
							t.Fatalf("incorrect installed transform: %+v", sa)
						}
					}
				}
			}
		}
	}
}
