// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- VPP dataplane backend
// RFC: rfc/short/rfc4303.md -- ESP SA parameters mapped to VPP ipsec_sa_v5_add_del

//go:build ze_vpp

package dataplane

import (
	"fmt"
	"net"
	"sync"

	"go.fd.io/govpp/api"

	vppcomp "github.com/ze-software/ze/internal/component/vpp"
)

type vppBackend struct {
	mu   sync.Mutex
	conn *vppcomp.Connector
	ch   api.Channel
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

// vppUnsupportedSelector refuses a policy this backend would install WRONGLY.
//
// It fails closed on two counts, and both were silent before it existed.
//
// TRANSPORT MODE. The SPD entry below is built with tunnel semantics and this backend
// never reads p.Mode, so a transport-mode request installed a tunnel-shaped entry and
// reported success. That is the same silent-wrong-mode failure the netlink guard was
// written to stop (kernelXFRMMode, dataplane.go), reproduced in the other backend. It is
// reachable independently of IKE: internal/plugins/ospf/ipsec_install.go asks for
// ModeTransport today.
//
// PORT SELECTORS. The SPD entry carries no port range, so a port-restricted policy would
// install as any-port and protect more traffic than was negotiated.
//
// Refusing is the minimum ai/rules/protocol.md allows. Implementing transport mode
// here means the GoVPP SAD tunnel flag plus the matching SPD entry, and it is the better
// answer whenever someone can test it against a real VPP.
func vppUnsupportedSelector(p SPParams) error {
	if p.Mode != ModeTunnel {
		return fmt.Errorf(
			"%w: vpp: policy mode %d is not implemented by this backend, which builds tunnel-mode SPD entries only; installing it would program tunnel mode and report success",
			ErrNotSupported, p.Mode)
	}
	if !p.SrcPort.IsAny() || !p.DstPort.IsAny() {
		return fmt.Errorf(
			"%w: vpp: port-restricted policy selectors are not implemented by this backend, and installing one would match every port; use a traffic selector with port any",
			ErrNotSupported)
	}
	return nil
}

func (b *vppBackend) InstallSA(p SAParams) error {
	if p.Mode != ModeTunnel {
		return fmt.Errorf(
			"%w: vpp: SA mode %d is not implemented by this backend, which builds tunnel-mode SA entries only; installing it would program tunnel mode and report success",
			ErrNotSupported, p.Mode)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	cryptoAlg := vppCryptoAlg(p.EncAlgo, p.IsAEAD)
	integAlg := vppIntegAlg(p.AuthAlgo, p.IsAEAD)

	req := &ipsecSAAddDel{
		IsAdd: true,
		Entry: ipsecSAEntry{
			SAID:       p.SPI,
			SPI:        p.SPI,
			Protocol:   1, // ESP
			CryptoAlg:  cryptoAlg,
			CryptoKey:  vppKey(p.EncKey),
			IntegAlg:   integAlg,
			IntegKey:   vppKey(p.AuthKey), //nolint:gosec // ESP integrity key
			TunnelSrc:  vppAddress(p.Src),
			TunnelDst:  vppAddress(p.Dst),
			TxTableID:  0,
			Salt:       0,
			UDPSrcPort: 0,
			UDPDstPort: 0,
		},
	}

	// SAParams.AcceptBothESPForms needs no work here, and that is MEASURED rather than
	// assumed. VPP's inbound lookup is encapsulation-BLIND: ipsec_tun_in.c
	// ipsec_tun_protect_input_inline picks the ESP offset per packet from the wire and
	// then keys ipsec4_tunnel_mk_key on the peer address and SPI alone, and
	// ipsec_tun.c ipsec_tun_register_nodes points IP protocol 50 AND UDP port 4500 at the
	// same node unconditionally. ONE VPP SA therefore receives both ESP forms already,
	// where Linux XFRM binds one state to one form.
	//
	// So this backend does NOT refuse the capability under ai/rules/protocol.md:
	// it can express what the caller asked for. It refuses everything else instead. Every
	// message this backend declares carries CRC "00000000", so GoVPP fails identifier
	// resolution and no SA is installed at all
	// (plan/spec-fixit-vpp-ipsec-inoperable.md owns that defect).

	reply := &ipsecSAAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("vpp: ipsec sa add spi=%d: %w", p.SPI, err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("vpp: ipsec sa add spi=%d: retval %d", p.SPI, reply.Retval)
	}
	return nil
}

func (b *vppBackend) RemoveSA(spi uint32, _ net.IP, _ uint8) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	req := &ipsecSAAddDel{
		IsAdd: false,
		Entry: ipsecSAEntry{SAID: spi},
	}
	reply := &ipsecSAAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("vpp: ipsec sa del spi=%d: %w", spi, err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("vpp: ipsec sa del spi=%d: retval %d", spi, reply.Retval)
	}
	return nil
}

func (b *vppBackend) InstallPolicy(p SPParams) error {
	if err := vppUnsupportedSelector(p); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	req := &ipsecSPDEntryAddDel{
		IsAdd: true,
		Entry: ipsecSPDEntry{
			SPDID:      0,
			Priority:   100,
			IsOutbound: p.Dir == SADirOut,
			Protocol:   p.Proto,
			LocalAddr:  vppPrefix(p.Src),
			RemoteAddr: vppPrefix(p.Dst),
			Policy:     3, // PROTECT
			SAID:       0,
		},
	}
	reply := &ipsecSPDEntryAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("vpp: ipsec spd entry add: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("vpp: ipsec spd entry add: retval %d", reply.Retval)
	}
	return nil
}

func (b *vppBackend) RemovePolicy(src, dst *net.IPNet, dir SADir) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	req := &ipsecSPDEntryAddDel{
		IsAdd: false,
		Entry: ipsecSPDEntry{
			IsOutbound: dir == SADirOut,
			LocalAddr:  vppPrefix(src),
			RemoteAddr: vppPrefix(dst),
		},
	}
	reply := &ipsecSPDEntryAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("vpp: ipsec spd entry del: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("vpp: ipsec spd entry del: retval %d", reply.Retval)
	}
	return nil
}

// RemovePolicyParams removes a policy by its full selector. The VPP SPD is keyed
// by local/remote address, protocol, and direction, so it reuses RemovePolicy;
// the upper-layer selector is not distinct in the VPP model (RFC 4552 IPsec for
// OSPF targets the XFRM backend, not VPP -- see spec-ospf-ext-16 out-of-scope).
func (b *vppBackend) RemovePolicyParams(p SPParams) error {
	return b.RemovePolicy(p.Src, p.Dst, p.Dir)
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

func (b *vppBackend) ListPolicies() ([]PolicyInfo, error) {
	return nil, fmt.Errorf("%w: the vpp backend cannot enumerate the SPD; use the VPP CLI to read it back", ErrNotSupported)
}

func (b *vppBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ch != nil {
		b.ch.Close()
		b.ch = nil
	}
	return nil
}

// VPP IPsec binary API message types.
// Matches ipsec_sa_v5_add_del / ipsec_spd_entry_add_del_v2 from VPP 24.x.
// When govpp/binapi/ipsec is vendored, replace these with the generated types.

type ipsecSAEntry struct {
	SAID       uint32
	SPI        uint32
	Protocol   uint8
	CryptoAlg  uint8
	CryptoKey  vppKeyData
	IntegAlg   uint8
	IntegKey   vppKeyData //nolint:gosec // ESP integrity key
	TunnelSrc  vppAddr
	TunnelDst  vppAddr
	TxTableID  uint32
	Salt       uint32
	UDPSrcPort uint16
	UDPDstPort uint16
}

type ipsecSAAddDel struct {
	IsAdd bool
	Entry ipsecSAEntry
}

func (*ipsecSAAddDel) GetMessageName() string          { return "ipsec_sad_entry_add_del_v3" }
func (*ipsecSAAddDel) GetCrcString() string            { return "00000000" }
func (*ipsecSAAddDel) GetMessageType() api.MessageType { return api.RequestMessage }

type ipsecSAAddDelReply struct {
	Retval    int32
	StatIndex uint32
}

func (*ipsecSAAddDelReply) GetMessageName() string          { return "ipsec_sad_entry_add_del_v3_reply" }
func (*ipsecSAAddDelReply) GetCrcString() string            { return "00000000" }
func (*ipsecSAAddDelReply) GetMessageType() api.MessageType { return api.ReplyMessage }

type ipsecSPDEntry struct {
	SPDID      uint32
	Priority   int32
	IsOutbound bool
	Protocol   uint8
	LocalAddr  vppPfx
	RemoteAddr vppPfx
	Policy     uint8
	SAID       uint32
}

type ipsecSPDEntryAddDel struct {
	IsAdd bool
	Entry ipsecSPDEntry
}

func (*ipsecSPDEntryAddDel) GetMessageName() string          { return "ipsec_spd_entry_add_del_v2" }
func (*ipsecSPDEntryAddDel) GetCrcString() string            { return "00000000" }
func (*ipsecSPDEntryAddDel) GetMessageType() api.MessageType { return api.RequestMessage }

type ipsecSPDEntryAddDelReply struct {
	Retval    int32
	StatIndex uint32
}

func (*ipsecSPDEntryAddDelReply) GetMessageName() string          { return "ipsec_spd_entry_add_del_v2_reply" }
func (*ipsecSPDEntryAddDelReply) GetCrcString() string            { return "00000000" }
func (*ipsecSPDEntryAddDelReply) GetMessageType() api.MessageType { return api.ReplyMessage }

type vppKeyData struct {
	Length uint8
	Data   [128]byte
}

type vppAddr struct {
	Af uint8
	Un [16]byte
}

type vppPfx struct {
	Address vppAddr
	Len     uint8
}

func vppKey(key []byte) vppKeyData {
	var k vppKeyData
	k.Length = uint8(min(len(key), 128))
	copy(k.Data[:], key)
	return k
}

func vppAddress(ip net.IP) vppAddr {
	var a vppAddr
	if ip4 := ip.To4(); ip4 != nil {
		a.Af = 0
		copy(a.Un[:4], ip4)
	} else {
		a.Af = 1
		copy(a.Un[:], ip.To16())
	}
	return a
}

func vppPrefix(n *net.IPNet) vppPfx {
	if n == nil {
		return vppPfx{}
	}
	ones, _ := n.Mask.Size()
	return vppPfx{
		Address: vppAddress(n.IP),
		Len:     uint8(ones),
	}
}

// VPP crypto algorithm IDs matching VPP's ipsec_crypto_alg_t enum.
func vppCryptoAlg(algo string, isAEAD bool) uint8 {
	if isAEAD {
		switch algo {
		case "aes128gcm":
			return 7 // IPSEC_API_CRYPTO_ALG_AES_GCM_128
		case "aes256gcm":
			return 9 // IPSEC_API_CRYPTO_ALG_AES_GCM_256
		case "chacha20poly1305":
			return 12 // IPSEC_API_CRYPTO_ALG_CHACHA20_POLY1305
		}
		return 7
	}
	switch algo {
	case "aes128":
		return 1 // IPSEC_API_CRYPTO_ALG_AES_CBC_128
	case "aes256":
		return 3 // IPSEC_API_CRYPTO_ALG_AES_CBC_256
	case "3des":
		return 4 // IPSEC_API_CRYPTO_ALG_3DES_CBC
	}
	return 1
}

// VPP integrity algorithm IDs matching VPP's ipsec_integ_alg_t enum.
func vppIntegAlg(algo string, isAEAD bool) uint8 {
	const (
		integSHA256 = "sha256"
		integSHA384 = "sha384"
		integSHA512 = "sha512"
		integSHA1   = "sha1"
	)
	if isAEAD {
		return 0 // NONE for AEAD
	}
	switch algo {
	case integSHA256:
		return 4 // IPSEC_API_INTEG_ALG_SHA_256_128
	case integSHA384:
		return 5 // IPSEC_API_INTEG_ALG_SHA_384_192
	case integSHA512:
		return 6 // IPSEC_API_INTEG_ALG_SHA_512_256
	case integSHA1:
		return 2 // IPSEC_API_INTEG_ALG_SHA1_96
	}
	return 4
}
