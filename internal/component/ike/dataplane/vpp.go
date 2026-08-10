// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- VPP dataplane backend
// RFC: rfc/short/rfc4303.md -- ESP SA parameters mapped to VPP ipsec_sad_entry_add_del_v3
// Related: vpp_policy.go -- the security POLICY half: the SPD, its interface binding, and its entries

//go:build ze_vpp

package dataplane

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"
	"sync"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/ipsec"
	"go.fd.io/govpp/binapi/ipsec_types"
	"go.fd.io/govpp/binapi/tunnel_types"

	vppcomp "github.com/ze-software/ze/internal/component/vpp"
)

// This backend sends the GENERATED binary-API messages from go.fd.io/govpp/binapi.
//
// It used to declare its own message structs beside them, and each one carried the
// CRC "00000000". GoVPP resolves a message id from the name and the CRC together. It
// uses the literal key name+"_"+crc against the table VPP sends at connect
// (vendor/go.fd.io/govpp/adapter/socketclient/socketclient.go, GetMsgID). The key
// never matched, so every InstallSA, RemoveSA, InstallPolicy and RemovePolicy
// returned UnknownMsgError before the encoder ran. The layouts were wrong too, and
// that was invisible underneath the refusal.
//
// A hand-maintained copy of a foreign wire format is the defect, so correcting the
// four CRCs by hand would have preserved it. The generated types carry the CRC, the
// layout, Marshal and Size, and they change with the module.

type vppBackend struct {
	mu   sync.Mutex
	conn *vppcomp.Connector
	ch   api.Channel

	// sadIDs maps an SA's identity to the VPP SAD id this backend allocated for it,
	// and nextSadID is the next free id. See saIdentity and allocSadID: the SPI is
	// NOT the id, because a peer chooses the outbound SPI and two peers can choose
	// the same one.
	sadIDs    map[saIdentity]uint32
	nextSadID uint32

	// spdID is the SPD this backend created, 0 until it has, and spdBound holds the
	// interfaces it is bound to. VPP has NO default SPD and no node-wide one: a
	// policy lives in an SPD, and an SPD applies to the interfaces it is bound to.
	// See ensureSPD.
	spdID    uint32
	spdBound map[uint32]bool

	// spdEntries are the SPD entries this backend added and has not removed. VPP
	// matches an SPD entry by its whole content rather than by a key, so the entry
	// that was sent IS the handle, and Close sends each one back (removeInstalled).
	//
	// MEASURED on VPP v26.06 (scripts/evidence/effective-vpp.py, run_ipsec_evidence):
	// a close that unbound the interface and deleted the SPD, without sending the
	// entries back, left the SA of its PROTECT entry INSTALLED while every request
	// returned retval 0. Sending the entries back first is what removes it.
	//
	// VPP says why in its own output: `show ipsec sa` reports "locks 2" for an SA a
	// PROTECT entry names. An SA is REFCOUNTED, the add takes one reference and the
	// entry takes another, and a delete releases one. So RemoveSA returning success
	// while VPP keeps the SA is CORRECT: it released the reference this backend's add
	// took, and the other one belongs to a policy this backend must remove itself.
	spdEntries []ipsec_types.IpsecSpdEntryV2
}

func newVPPBackend() (Dataplane, error) {
	conn := vppcomp.GetActiveConnector()
	if conn == nil {
		return nil, fmt.Errorf("%w: vpp connector not available", ErrNotSupported)
	}
	ch, err := conn.NewChannel()
	if err != nil {
		return nil, fmt.Errorf("vpp dataplane: channel: %w", err)
	}
	return &vppBackend{conn: conn, ch: ch}, nil
}

// saIdentity names one SA the way RFC 4301 Section 4.1 does: "uniquely identified by
// the triple consisting of a Security Parameter Index (SPI), an IP Destination
// Address, and a security protocol (AH or ESP) identifier".
//
// The SPI alone is NOT that identity. RFC 7296 Section 2.8 has each side choose the
// SPI it wants traffic to arrive with, so the OUTBOUND SPI is the peer's choice and
// two peers can choose the same number. The destination address separates them,
// because it is the peer's own address.
//
// The inbound SPI is this node's choice and is unique here by construction, so the
// triple never collides in either direction.
type saIdentity struct {
	spi   uint32
	dst   string
	proto uint8
}

func saIdentityOf(spi uint32, dst net.IP, proto uint8) (saIdentity, error) {
	if !validTunnelEndpoint(dst) {
		return saIdentity{}, fmt.Errorf(
			"%w: vpp: an SA is named by its SPI, its destination address and its protocol, and the destination is %v",
			ErrNotSupported, dst)
	}
	return saIdentity{spi: spi, dst: dst.String(), proto: proto}, nil
}

// allocSadID returns the VPP SAD id for this SA, allocating one on first sight. The
// second result says whether THIS call allocated it, so a caller whose add VPP then
// refuses can drop the entry again and leave the map saying what VPP holds.
//
// The SPI used to be the id. VPP names an SA by a single u32 sad_id, and an
// ipsec_spd_entry_v2 protects by naming that id. Two peers that chose the same outbound
// SPI therefore shared one SAD entry. The second InstallSA overwrote the first, and both
// policies then resolved to the surviving SA. The identity above separates them, and the
// id is allocated here instead.
//
// A re-install of the SAME identity -- the same SPI, destination and protocol -- keeps
// its id, so VPP updates one entry rather than growing a second. That is an idempotent
// re-install, the shape the OSPF installer produces when it re-applies an interface
// (plugins/ospf/ipsec_install.go). A REKEY is not that case: RFC 7296 Section 2.8 gives
// the replacement Child SA a new SPI, so it is a new identity and gets a new id.
//
// The counter starts ABOVE the highest id VPP already holds, read from VPP itself.
// Starting at 1 would overwrite an SA left by an earlier run of this process, or one
// another API client owns. b.mu is held.
func (b *vppBackend) allocSadID(id saIdentity) (sadID uint32, fresh bool, err error) {
	if got, ok := b.sadIDs[id]; ok {
		return got, false, nil
	}
	if b.nextSadID == 0 {
		next, err := b.firstFreeSadID()
		if err != nil {
			return 0, false, err
		}
		b.nextSadID = next
	}
	if b.nextSadID == math.MaxUint32 {
		return 0, false, fmt.Errorf("vpp: the SAD id space is exhausted at %d", b.nextSadID)
	}
	allocated := b.nextSadID
	b.nextSadID++
	if b.sadIDs == nil {
		b.sadIDs = make(map[saIdentity]uint32)
	}
	b.sadIDs[id] = allocated
	return allocated, true, nil
}

// firstFreeSadID reads the SAD ids VPP holds and returns one past the highest.
//
// It is one dump, on the first InstallSA. VPP is the authority on what it holds: this
// backend's own map is empty at start and says nothing about entries an earlier run
// installed.
func (b *vppBackend) firstFreeSadID() (uint32, error) {
	highest := uint32(0)
	req := b.ch.SendMultiRequest(&ipsec.IpsecSaV3Dump{})
	for {
		details := &ipsec.IpsecSaV3Details{}
		stop, err := req.ReceiveReply(details)
		if err != nil {
			return 0, fmt.Errorf("vpp: ipsec sa dump: %w", err)
		}
		if stop {
			break
		}
		if details.Entry.SadID > highest {
			highest = details.Entry.SadID
		}
	}
	if highest == math.MaxUint32 {
		return 0, fmt.Errorf("vpp: the SAD id space is exhausted at %d", highest)
	}
	// 0 is a valid VPP SAD id and this backend never allocates it, so a policy that
	// reaches VPP naming SA 0 names an SA this backend did not install.
	return max(highest+1, 1), nil
}

// vppUnsupportedSA refuses an SA this backend would install WRONGLY.
//
// TRANSPORT MODE. The SAD entry below is built with tunnel semantics, so a
// transport-mode request would install a tunnel-shaped entry and report success.
//
// STATE SELECTOR. ipsec_sad_entry_v3 has no selector field. An explicit Sel would be
// dropped, and the SA would then resolve for more flows than the caller asked for.
//
// XFRM INTERFACE ID. IfID binds a Linux XFRM state to an xfrm device. VPP has no such
// binding on the SAD entry, so a non-zero IfID would install a node-wide SA.
//
// DIRECTION. ipsec_sad_entry_v3 flags one direction per SA, and an unset SAParams.Dir
// names neither (SAParams.Dir, dataplane.go). Reading it as outbound would install a
// decrypting SA that VPP never selects for inbound processing, so the tunnel would
// establish and carry nothing. The one caller that leaves Dir unset means a
// bidirectional SA (plugins/ospf/ipsec_install.go, buildIPsecSA), which VPP cannot
// express at all.
func vppUnsupportedSA(p SAParams) error {
	if p.Dir != SADirIn && p.Dir != SADirOut {
		return fmt.Errorf(
			"%w: vpp: SA direction %d names neither inbound (%d) nor outbound (%d), and an ipsec_sad_entry_v3 carries one direction; installing it would program an outbound SA that never decrypts",
			ErrNotSupported, p.Dir, SADirIn, SADirOut)
	}
	if p.Mode != ModeTunnel {
		return fmt.Errorf(
			"%w: vpp: SA mode %d is not implemented by this backend, which builds tunnel-mode SA entries only; installing it would program tunnel mode and report success",
			ErrNotSupported, p.Mode)
	}
	if p.Sel != nil {
		return fmt.Errorf(
			"%w: vpp: an explicit SA state selector is not implemented by this backend, and ipsec_sad_entry_v3 carries no selector field; installing it would match more flows than the selector names",
			ErrNotSupported)
	}
	if p.IfID != 0 {
		return fmt.Errorf(
			"%w: vpp: xfrm interface id %d is not implemented by this backend, and ipsec_sad_entry_v3 has no equivalent binding; installing it would program a node-wide SA",
			ErrNotSupported, p.IfID)
	}
	return nil
}

// vppSAEndpoints checks the pair that becomes the SAD entry's tunnel.
//
// ip_types.NewAddress turns a nil into the unspecified address of a family it picks
// itself, so an absent endpoint would reach VPP as :: and no traffic would match the
// SA. That is the same absent-endpoint fault tunnelEndpoints guards on the policy
// side (dataplane.go), in the form this message takes.
func vppSAEndpoints(p SAParams) error {
	if !validTunnelEndpoint(p.Src) || !validTunnelEndpoint(p.Dst) {
		return fmt.Errorf(
			"vpp: a tunnel-mode SA needs both endpoints, got src=%v dst=%v: an absent endpoint reaches VPP as the unspecified address",
			p.Src, p.Dst)
	}
	if (p.Src.To4() != nil) != (p.Dst.To4() != nil) {
		return fmt.Errorf("vpp: SA tunnel endpoints must share an address family, got src=%v dst=%v", p.Src, p.Dst)
	}
	return nil
}

func (b *vppBackend) InstallSA(p SAParams) error {
	if err := vppUnsupportedSA(p); err != nil {
		return err
	}
	if err := vppSAEndpoints(p); err != nil {
		return err
	}
	proto, err := vppProto(p.Proto)
	if err != nil {
		return err
	}
	cryptoAlg, err := vppCryptoAlg(p.EncAlgo, p.IsAEAD)
	if err != nil {
		return err
	}
	integAlg, err := vppIntegAlg(p.AuthAlgo, p.IsAEAD)
	if err != nil {
		return err
	}
	cryptoKey, salt, err := vppCryptoKeyAndSalt(p)
	if err != nil {
		return err
	}
	identity, err := saIdentityOf(p.SPI, p.Dst, p.Proto)
	if err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	sadID, fresh, err := b.allocSadID(identity)
	if err != nil {
		return err
	}

	req := &ipsec.IpsecSadEntryAddDelV3{
		IsAdd: true,
		Entry: ipsec_types.IpsecSadEntryV3{
			// The SAD id is what an SPD entry names in SaID. InstallPolicy resolves the
			// same identity to the same id (spdEntry).
			SadID:              sadID,
			Spi:                p.SPI,
			Protocol:           proto,
			CryptoAlgorithm:    cryptoAlg,
			CryptoKey:          vppKey(cryptoKey),
			IntegrityAlgorithm: integAlg,
			IntegrityKey:       vppKey(p.AuthKey), //nolint:gosec // ESP integrity key
			Flags:              vppSAFlags(p),
			Tunnel: tunnel_types.Tunnel{
				Src: ip_types.NewAddress(p.Src),
				Dst: ip_types.NewAddress(p.Dst),
				// RFC 7296 Section 2.24: a tunnel encapsulator and decapsulator for a
				// tunnel-mode SA created by IKEv2 MUST support the ECN
				// full-functionality option, which copies the congestion indication
				// from the inner header on encapsulation and back onto it on
				// decapsulation. VPP does neither unless it is asked: the flags
				// default to TUNNEL_API_ENCAP_DECAP_FLAG_NONE, so an unset field is a
				// tunnel that DISCARDS the indication.
				EncapDecapFlags: ecnFullFunctionality,
			},
			Salt:       salt,
			UDPSrcPort: p.UDPEncapSPort,
			UDPDstPort: p.UDPEncapDPort,
		},
	}

	// SAParams.AcceptBothESPForms is passed through UNVERIFIED, and this is the one
	// place in this file where that is true of behavior rather than of a field value.
	//
	// The field asks that ONE inbound SA receive both the UDP-encapsulated and the bare
	// ESP form (RFC 7296 Section 2.23). Linux XFRM binds one state to one form, so the
	// XFRM backend serves the second form beside the kernel (espform.go). Whether VPP
	// needs the same help is a property of VPP's inbound node graph, and NOTHING this
	// repository can open states it. The vendored binapi is a generated binding, and a
	// binding states that a field exists, never what the foreign system does with it
	// (ai/rules/evidence.md). This comment cited ipsec_tun_in.c and ipsec_tun.c as
	// MEASURED. Those files are VPP source and are not in this tree, so no reader here
	// can re-derive the claim. It is withdrawn.
	//
	// The proof is a real VPP receiving BOTH forms on one inbound SA and decrypting
	// both. The AC-7 deployment run (scripts/evidence/effective-vpp.py,
	// run_ipsec_evidence) programs an SA and reads it back through vppctl. It sends no
	// ESP, so it does not settle this.
	//
	// The install is not refused. ai/rules/protocol.md requires a backend that CANNOT
	// receive both forms to reject rather than report success, and what is known here
	// is that nobody has measured it, which is not the same finding. Refusing on it
	// would reject every inbound SA the IKE engine installs (child.go sets the field on
	// all of them) on evidence that does not exist either.

	reply := &ipsec.IpsecSadEntryAddDelV3Reply{}
	err = b.ch.SendRequest(req).ReceiveReply(reply)
	if err == nil && reply.Retval != 0 {
		err = fmt.Errorf("retval %d", reply.Retval)
	}
	if err != nil {
		// VPP REFUSED the add, so it holds no such SA and this backend must not say
		// it does. b.sadIDs is what spdEntry's "this backend has installed no such
		// SA" guard reads, and a refused add that left its identity there opened the
		// guard for an SA VPP never made: the policy would then protect with a SAD id
		// nothing answers to.
		//
		// Only an id allocated by THIS call is dropped. An identity already in the map
		// records an EARLIER successful add, which the refusal did not undo. That is an
		// idempotent re-install of one SA, not a rekey: a rekey arrives with a new SPI
		// and therefore a new identity (allocSadID).
		if fresh {
			delete(b.sadIDs, identity)
		}
		return fmt.Errorf("vpp: ipsec sa add spi=%d: %w", p.SPI, err)
	}
	return nil
}

// RemoveSA deletes the SAD entry this backend installed for the SA identity its three
// arguments name (saIdentity). The SPI alone used to be the id, which deleted whatever
// SA held that number.
//
// A VPP SA is REFCOUNTED (vppBackend.spdEntries), so this releases the reference the
// add took and the SA survives while a PROTECT entry still names it. Removing it
// therefore needs the caller to send that entry back FIRST, and ONE CALLER DOES NOT:
// removeChildSAExcept (ike/engine/child.go) passes dropPolicy=false on a
// make-before-break rekey, because both Child SAs answer to one policy per direction.
// It still calls RemoveSA.
//
// VPP then keeps the retired SA and deleteSAD still returns retval 0, the behavior
// removeInstalled measured on v26.06. This function reads that as success and drops the
// identity from b.sadIDs. removeInstalled iterates that map, so the delete is never
// sent again and the retired SA OUTLIVES Close.
//
// Inert today: vppPolicyInterface (vpp_policy.go) refuses every policy the IKE engine
// produces, so no rekey reaches this backend. Recorded in the Known Limitations of
// plan/spec-fixit-vpp-ipsec-inoperable.md, in
// plan/journal/false-synchronization-claim.md, and as AC-5 of
// plan/future/spec-ipsec-vpp-policy-interface.md.
func (b *vppBackend) RemoveSA(spi uint32, dst net.IP, proto uint8) error {
	identity, err := saIdentityOf(spi, dst, proto)
	if err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	sadID, ok := b.sadIDs[identity]
	if !ok {
		return fmt.Errorf(
			"%w: vpp: no SA is installed for spi=%d dst=%v proto=%d, and VPP deletes an SA by the id this backend allocated for it",
			ErrNotSupported, spi, dst, proto)
	}

	if err := b.deleteSAD(sadID); err != nil {
		return fmt.Errorf("vpp: ipsec sa del spi=%d: %w", spi, err)
	}
	delete(b.sadIDs, identity)
	return nil
}

// ListSAs and ListPolicies REFUSE rather than report an empty dataplane.
//
// VPP holds the SAs and the policies this backend installed, and reading them
// back needs the VPP binary-API dump this backend does not implement. Returning
// an empty list would report VPP's populated dataplane as empty, which is worse
// than saying nothing: it is a wrong answer that looks like a right one
// (ai/rules/evidence.md). Implementing the dump is separable work, recorded in
// the spec's Known Limitations.
func (b *vppBackend) ListSAs(_ uint32) ([]SAInfo, error) {
	return nil, fmt.Errorf("%w: the vpp backend cannot enumerate the SAD; use the VPP CLI to read it back", ErrNotSupported)
}

// Close REMOVES the VPP state this backend installed, then closes the channel.
//
// Nothing in VPP expires an SA or an SPD. Both live until an API client deletes them,
// so a ze that exited without deleting left the SAs of a dead IKE session installed
// and its SPD still BOUND to the interface, enforcing PROTECT entries naming those
// SAs. The next run then stepped OVER all of it (firstFreeSadID, freeSPDID), so the
// dead set accumulated once per restart.
//
// The VPP traffic backend removes the same class of leftover, and it removes it at
// STARTUP (cleanupStartupOrphans, internal/plugins/traffic/vpp/backend_linux.go),
// because a VPP policer carries a name this backend puts a "ze/" prefix on. An
// ipsec_sad_entry_v3 and an ipsec_spd carry no name and no owner: they are numbers
// VPP hands out, and an id another API client created is indistinguishable from one
// an earlier ze run created. So the only point at which this backend can tell its own
// state from a foreign client's is while it still holds the maps recording what it
// installed, which is here. That is the same limit the traffic backend states for its
// own anonymous classify tables, which it cannot reclaim at startup either.
//
// A ze that is KILLED rather than closed still leaves the state behind, and nothing
// can identify it afterwards. Recorded in the spec's Known Limitations.
//
// Every removal is attempted even after one fails, so a single refusal does not
// strand the rest, and the errors are joined.
func (b *vppBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ch == nil {
		return nil
	}
	err := b.removeInstalled()
	b.ch.Close()
	b.ch = nil
	return err
}

// removeInstalled deletes everything this backend put in VPP, POLICY FIRST.
//
// The order is the reverse of the order it was created in: unbind every interface,
// send back every SPD entry, delete the SPD, then delete the SAs. An SPD entry names
// the SAD id of the SA that protects its traffic (spdEntry, vpp_policy.go), so
// deleting the SAs first would leave entries naming ids VPP no longer holds.
// Unbinding first stops the SPD acting on traffic while it is emptied.
//
// The ENTRIES are sent back rather than left to the SPD delete. Measured on VPP
// v26.06: without that step the SA of a PROTECT entry stayed installed and every
// request still returned retval 0 (vppBackend.spdEntries). b.mu is held.
func (b *vppBackend) removeInstalled() error {
	var errs []error
	for swIfIndex := range b.spdBound {
		if err := b.unbindSPD(swIfIndex); err != nil {
			errs = append(errs, err)
		}
	}
	var kept []ipsec_types.IpsecSpdEntryV2
	for _, entry := range b.spdEntries {
		if err := b.sendSPDEntry(false, entry); err != nil {
			errs = append(errs, fmt.Errorf("vpp: close: %w", err))
			kept = append(kept, entry)
		}
	}
	b.spdEntries = kept
	if b.spdID != 0 {
		if err := b.deleteSPD(); err != nil {
			errs = append(errs, err)
		}
	}
	for identity, sadID := range b.sadIDs {
		if err := b.deleteSAD(sadID); err != nil {
			errs = append(errs, fmt.Errorf("vpp: close: ipsec sa del spi=%d: %w", identity.spi, err))
			continue
		}
		delete(b.sadIDs, identity)
	}
	return errors.Join(errs...)
}

// deleteSAD sends the SAD delete for one id. VPP deletes an SA by the id alone, so the
// entry carries nothing else. b.mu is held.
func (b *vppBackend) deleteSAD(sadID uint32) error {
	req := &ipsec.IpsecSadEntryAddDelV3{
		IsAdd: false,
		Entry: ipsec_types.IpsecSadEntryV3{SadID: sadID},
	}
	reply := &ipsec.IpsecSadEntryAddDelV3Reply{}
	err := b.ch.SendRequest(req).ReceiveReply(reply)
	if err == nil && reply.Retval != 0 {
		err = fmt.Errorf("retval %d", reply.Retval)
	}
	if err != nil {
		return fmt.Errorf("sad-id=%d: %w", sadID, err)
	}
	return nil
}

// vppProto maps the Ze IPsec transform protocol to the VPP enum.
//
// It never defaults. The value sent before was the literal 1, commented "ESP".
// ProtoESP in this package is already 50, and so is IPSEC_API_PROTO_ESP. The backend
// disagreed with its own package, AH was unreachable, and p.Proto was ignored.
func vppProto(proto uint8) (ipsec_types.IpsecProto, error) {
	switch proto {
	case ProtoESP:
		return ipsec_types.IPSEC_API_PROTO_ESP, nil
	case ProtoAH:
		return ipsec_types.IPSEC_API_PROTO_AH, nil
	}
	return 0, fmt.Errorf(
		"%w: vpp: IPsec protocol %d is neither ESP (%d) nor AH (%d)",
		ErrNotSupported, proto, ProtoESP, ProtoAH)
}

// ecnFullFunctionality is the ECN option RFC 7296 Section 2.24 makes mandatory for a
// tunnel-mode SA created by IKEv2, in the form a VPP tunnel takes it.
//
// ENCAP_COPY_ECN copies the inner header's congestion indication onto the outer one
// when the packet is encapsulated. DECAP_COPY_ECN copies it back onto the inner
// header when the packet is decapsulated. Together they are the "full-functionality
// option for tunnels" of RFC 3168 Section 9.1, and the second one is what stops a
// congestion indication set inside the tunnel being discarded at its far end.
//
// The XFRM backend needs no equivalent assignment: Linux does this by default and the
// state ze builds asks it to disable nothing (TestEcnInstalledStateDisablesNothing,
// rfc7296_ecn_linux_test.go). VPP is the opposite way round. Zero is
// TUNNEL_API_ENCAP_DECAP_FLAG_NONE, so a tunnel that says nothing copies nothing.
const ecnFullFunctionality = tunnel_types.TUNNEL_API_ENCAP_DECAP_FLAG_ENCAP_COPY_ECN |
	tunnel_types.TUNNEL_API_ENCAP_DECAP_FLAG_DECAP_COPY_ECN

// vppSAFlags derives the SAD entry flags from the parameters.
//
// The hand-rolled struct had no flags field at all, so every one of these was zero.
// Zero means transport mode with anti-replay off: the tunnel endpoints below would
// have been ignored and a replayed packet accepted.
func vppSAFlags(p SAParams) ipsec_types.IpsecSadFlags {
	flags := ipsec_types.IPSEC_API_SAD_FLAG_NONE
	// An inbound SA carries IS_INBOUND and an outbound one does not.
	// vppUnsupportedSA has already refused a Dir that names neither direction.
	//
	// UNVERIFIED against a running VPP: that VPP selects an SA for inbound processing
	// by this flag ALONE is read from the VPP IPsec documentation, not from code this
	// repository can compile. The AC-7 deployment run
	// (scripts/evidence/effective-vpp.py, run_ipsec_evidence) reads the installed SA
	// back through `vppctl show ipsec sa`, so what it settles is that VPP RECORDS the
	// flag this backend sent. What it does not settle is that inbound selection reads
	// nothing else, which needs ESP arriving at a real VPP.
	if p.Dir == SADirIn {
		flags |= ipsec_types.IPSEC_API_SAD_FLAG_IS_INBOUND
	}
	if p.Mode == ModeTunnel {
		flags |= ipsec_types.IPSEC_API_SAD_FLAG_IS_TUNNEL
		if p.Dst.To4() == nil {
			flags |= ipsec_types.IPSEC_API_SAD_FLAG_IS_TUNNEL_V6
		}
	}
	// ipsec_sad_entry_v3 carries no window SIZE, so p.ReplayWin selects anti-replay on
	// or off and VPP applies its own window. ipsec_sad_entry_v4 adds
	// anti_replay_window_size and go.fd.io/govpp v0.13.0 does not generate it.
	if p.ReplayWin > 0 {
		flags |= ipsec_types.IPSEC_API_SAD_FLAG_USE_ANTI_REPLAY
	}
	if p.UDPEncap {
		flags |= ipsec_types.IPSEC_API_SAD_FLAG_UDP_ENCAP
	}
	return flags
}

// aeadSaltBytes returns the octets this AEAD cipher takes as salt beyond its key, and
// false for a cipher whose salt this backend does not know. It is read per algorithm
// rather than from one number shared by every AEAD, the same rule
// crypto.encKeyMaterialLen follows.
func aeadSaltBytes(algo string) (int, bool) {
	switch algo {
	case algoAES128GCM, algoAES256GCM:
		return 4, true // RFC 4106 Section 8.1
	case algoChaCha20Poly1305:
		return 4, true // RFC 7634 Section 2
	}
	return 0, false
}

// The AEAD cipher names SAParams.EncAlgo carries. Both the salt length and the VPP
// algorithm id are keyed on them, so they are named once.
const (
	algoAES128GCM        = "aes128gcm"
	algoAES256GCM        = "aes256gcm"
	algoChaCha20Poly1305 = "chacha20poly1305"
)

// vppCryptoKeyAndSalt splits SAParams.EncKey into the two fields VPP takes.
//
// EncKey carries the cipher key FOLLOWED BY that cipher's salt when IsAEAD is true
// (SAParams.EncKey, dataplane.go). Linux XFRM takes the two together, because
// rfc4106(gcm(aes)) expects that layout. VPP does not. ipsec_sad_entry_v3 has its own
// salt field, and its key field is sized at the cipher key. All 36 octets of an
// AES-GCM-256 KEYMAT used to go into the key field, with the salt hardcoded to 0. VPP
// read an over-long key, and it encrypted with a zero salt while the peer used the
// real one.
//
// The salt reaches the wire as the four KEYMAT octets in KEYMAT order, because GoVPP
// encodes a u32 big-endian (vendor/go.fd.io/govpp/codec/buffer.go, EncodeUint32).
//
// MEASURED on VPP v26.06 in the AC-7 deployment run
// (scripts/evidence/effective-vpp.py, run_ipsec_evidence). An AES-GCM-256 SA whose
// KEYMAT ends de ad be ef is reported by `show ipsec sa 0` as "salt 0xdeadbeef", with
// "crypto alg aes-gcm-256 key 000102...1e1f", the 32 cipher octets alone. So the four
// KEYMAT octets reach VPP's salt in KEYMAT order, and the key field holds the cipher
// key without them.
//
// What that does NOT settle is where VPP then places those octets in the GCM nonce,
// which needs ESP arriving at a real VPP and a peer to decrypt it.
func vppCryptoKeyAndSalt(p SAParams) ([]byte, uint32, error) {
	if !p.IsAEAD {
		return p.EncKey, 0, nil
	}
	saltLen, ok := aeadSaltBytes(p.EncAlgo)
	if !ok {
		return nil, 0, fmt.Errorf(
			"%w: vpp: the salt length of AEAD cipher %q is unknown to this backend, and guessing it would key the SA at the wrong offset",
			ErrNotSupported, p.EncAlgo)
	}
	if saltLen != 4 {
		return nil, 0, fmt.Errorf(
			"%w: vpp: ipsec_sad_entry_v3 carries a four octet salt and AEAD cipher %q takes %d",
			ErrNotSupported, p.EncAlgo, saltLen)
	}
	if len(p.EncKey) <= saltLen {
		return nil, 0, fmt.Errorf(
			"vpp: AEAD cipher %s key material is %d octets, which leaves no cipher key once its %d octet salt is taken",
			p.EncAlgo, len(p.EncKey), saltLen)
	}
	split := len(p.EncKey) - saltLen
	return p.EncKey[:split], binary.BigEndian.Uint32(p.EncKey[split:]), nil
}

func vppKey(key []byte) ipsec_types.Key {
	// Marshal writes exactly 128 octets whatever the slice holds
	// (vendor/go.fd.io/govpp/binapi/ipsec/ipsec.ba.go, EncodeBytes(..., 128)).
	return ipsec_types.Key{
		Length: uint8(min(len(key), 128)), //nolint:gosec // clamped to 128
		Data:   key,
	}
}

// vppCryptoAlg maps the negotiated cipher name to VPP's ipsec_crypto_alg_t.
//
// It never defaults. An unknown name used to become AES_CBC_128, so a cipher this
// backend does not know was programmed as a cipher it does know, and the SA came up
// encrypting with something the operator never configured. That is the same fault as
// "3des" reaching VPP as AES_CTR_128, which is what this function was rewritten to
// remove, and ai/rules/protocol.md forbids applying a config the backend cannot
// express exactly.
func vppCryptoAlg(algo string, isAEAD bool) (ipsec_types.IpsecCryptoAlg, error) {
	if isAEAD {
		switch algo {
		case algoAES128GCM:
			return ipsec_types.IPSEC_API_CRYPTO_ALG_AES_GCM_128, nil
		case algoAES256GCM:
			return ipsec_types.IPSEC_API_CRYPTO_ALG_AES_GCM_256, nil
		case algoChaCha20Poly1305:
			return ipsec_types.IPSEC_API_CRYPTO_ALG_CHACHA20_POLY1305, nil
		}
		return 0, fmt.Errorf(
			"%w: vpp: AEAD cipher %q is not one this backend can name to VPP; installing it would encrypt with a different cipher",
			ErrNotSupported, algo)
	}
	switch algo {
	case "aes128":
		return ipsec_types.IPSEC_API_CRYPTO_ALG_AES_CBC_128, nil
	case "aes256":
		return ipsec_types.IPSEC_API_CRYPTO_ALG_AES_CBC_256, nil
	case "3des":
		// 4 stood here, and 4 is AES_CTR_128. The operator configured 3DES and VPP
		// encrypted with a different cipher.
		return ipsec_types.IPSEC_API_CRYPTO_ALG_3DES_CBC, nil
	}
	return 0, fmt.Errorf(
		"%w: vpp: cipher %q is not one this backend can name to VPP; installing it would encrypt with a different cipher",
		ErrNotSupported, algo)
}

// vppIntegAlg maps the negotiated integrity algorithm to VPP's ipsec_integ_alg_t.
//
// It never defaults either, and for the same reason: an unknown name became
// SHA_256_128, so the SA authenticated with an algorithm the peer did not negotiate
// and every packet failed its integrity check.
func vppIntegAlg(algo string, isAEAD bool) (ipsec_types.IpsecIntegAlg, error) {
	const (
		integSHA256 = "sha256"
		integSHA384 = "sha384"
		integSHA512 = "sha512"
		integSHA1   = "sha1"
	)
	// An AEAD cipher authenticates in the cipher, so VPP takes NONE here whatever the
	// SA carries beside it (RFC 4106 Section 4).
	if isAEAD {
		return ipsec_types.IPSEC_API_INTEG_ALG_NONE, nil
	}
	switch algo {
	case integSHA256:
		return ipsec_types.IPSEC_API_INTEG_ALG_SHA_256_128, nil
	case integSHA384:
		return ipsec_types.IPSEC_API_INTEG_ALG_SHA_384_192, nil
	case integSHA512:
		return ipsec_types.IPSEC_API_INTEG_ALG_SHA_512_256, nil
	case integSHA1:
		return ipsec_types.IPSEC_API_INTEG_ALG_SHA1_96, nil
	}
	return 0, fmt.Errorf(
		"%w: vpp: integrity algorithm %q is not one this backend can name to VPP; installing it would authenticate with a different algorithm",
		ErrNotSupported, algo)
}
