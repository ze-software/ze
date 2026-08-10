// VALIDATES: the four VPP IPsec messages this backend sends are the GENERATED binapi
// messages. Each one resolves through the same name+"_"+crc key GoVPP uses against
// VPP's message table. Each also carries the values SAParams and SPParams asked for.
// Those values are ESP or AH from p.Proto, and 3DES at 11. They are also the SPD
// selector as an address RANGE, and the SA id a protect policy names. The last two
// are the tunnel flag and the AEAD key split from its salt.
// PREVENTS: the message identity regressing to a hand-rolled struct GoVPP cannot
// resolve, where every request fails with UnknownMsgError before the encoder runs.
// Also prevents a resolvable message that programs a different cipher, a different
// protocol, a one-address selector, or an SA the policy never names.

//go:build ze_vpp

package dataplane

import (
	"encoding/binary"
	"errors"
	"net"
	"path"
	"reflect"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/ipsec"
	"go.fd.io/govpp/binapi/ipsec_types"
	"go.fd.io/govpp/binapi/tunnel_types"
)

// capturingChannel is an api.Channel that records what the backend sent and leaves
// every reply at its zero value, which is Retval 0.
//
// A dump answers with the ids in spds and sads, so a test can put VPP's existing
// databases in front of the backend. Empty is a VPP holding neither.
//
// refuse makes VPP answer the named message with a non-zero retval, which is how a
// real VPP declines a request it received and understood. A test uses it to drive the
// paths where the backend has already recorded state for a request VPP then refuses.
type capturingChannel struct {
	sent  []api.Message
	dumps []api.Message

	spds []uint32
	sads []uint32

	refuse map[string]int32
}

func (c *capturingChannel) SendRequest(msg api.Message) api.RequestCtx {
	c.sent = append(c.sent, msg)
	if add, ok := msg.(*ipsec.IpsecSpdAddDel); ok && add.IsAdd {
		c.spds = append(c.spds, add.SpdID)
	}
	return capturedRequest{retval: c.refuse[msg.GetMessageName()]}
}

func (c *capturingChannel) SendMultiRequest(msg api.Message) api.MultiRequestCtx {
	c.dumps = append(c.dumps, msg)
	var fills []func(api.Message)
	switch msg.(type) {
	case *ipsec.IpsecSpdsDump:
		for _, id := range c.spds {
			fills = append(fills, func(reply api.Message) {
				details, ok := reply.(*ipsec.IpsecSpdsDetails)
				if !ok {
					panic("an ipsec_spds_dump reply must be ipsec_spds_details")
				}
				details.SpdID = id
			})
		}
	case *ipsec.IpsecSaV3Dump:
		for _, id := range c.sads {
			fills = append(fills, func(reply api.Message) {
				details, ok := reply.(*ipsec.IpsecSaV3Details)
				if !ok {
					panic("an ipsec_sa_v3_dump reply must be ipsec_sa_v3_details")
				}
				details.Entry.SadID = id
			})
		}
	default:
		panic("unexpected multi-request " + msg.GetMessageName())
	}
	return &capturedMulti{fills: fills}
}

// capturedMulti replays canned details messages, then reports the end of the dump the
// way GoVPP does: a final ReceiveReply with lastReplyReceived true.
type capturedMulti struct {
	fills []func(api.Message)
	next  int
}

func (m *capturedMulti) ReceiveReply(msg api.Message) (bool, error) {
	if m.next >= len(m.fills) {
		return true, nil
	}
	m.fills[m.next](msg)
	m.next++
	return false, nil
}

func (c *capturingChannel) SubscribeNotification(chan api.Message, api.Message) (api.SubscriptionCtx, error) {
	panic("the vpp ipsec backend subscribes to no notifications")
}

func (c *capturingChannel) SetReplyTimeout(time.Duration)          {}
func (c *capturingChannel) CheckCompatiblity(...api.Message) error { return nil }
func (c *capturingChannel) Close()                                 {}

type capturedRequest struct{ retval int32 }

func (r capturedRequest) ReceiveReply(reply api.Message) error {
	if r.retval == 0 {
		return nil
	}
	switch msg := reply.(type) {
	case *ipsec.IpsecSadEntryAddDelV3Reply:
		msg.Retval = r.retval
	case *ipsec.IpsecSpdEntryAddDelV2Reply:
		msg.Retval = r.retval
	case *ipsec.IpsecSpdAddDelReply:
		msg.Retval = r.retval
	case *ipsec.IpsecInterfaceAddDelSpdReply:
		msg.Retval = r.retval
	default:
		panic("no retval to set on " + reply.GetMessageName())
	}
	return nil
}

func newCapturingBackend() (*vppBackend, *capturingChannel) {
	ch := &capturingChannel{}
	return &vppBackend{ch: ch}, ch
}

// requireResolvable asserts that GoVPP can turn this message into a numeric VPP
// message id.
//
// It applies the SAME key GoVPP applies. api.RegisterMessage stores every generated
// message under name+"_"+crc within its binapi path
// (vendor/go.fd.io/govpp/api/binapi.go). The socket client then looks a request up
// with that literal string against the table VPP sends at connect
// (vendor/go.fd.io/govpp/adapter/socketclient/socketclient.go, GetMsgID). A miss is
// UnknownMsgError, returned from SendRequest before EncodeMsg runs
// (vendor/go.fd.io/govpp/core/request_handler.go).
func requireResolvable(t *testing.T, msg api.Message) {
	t.Helper()
	key := msg.GetMessageName() + "_" + msg.GetCrcString()
	binapiPath := path.Dir(reflect.TypeOf(msg).Elem().PkgPath())
	got, ok := api.GetRegisteredMessages()[binapiPath][key]
	if !ok {
		t.Fatalf("message %q does not resolve: GoVPP keys its table on name+\"_\"+crc, and nothing in %q is registered under that key", key, binapiPath)
	}
	if reflect.TypeOf(got) != reflect.TypeOf(msg) {
		t.Fatalf("key %q resolves to %T, want %T", key, got, msg)
	}
}

// The CRCs GoVPP registers for the four messages this backend sends. Each one stood
// at "00000000" in a hand-rolled struct until 2026-08-10, so every request failed
// identifier resolution and no SA was ever installed.
func TestVPPMessageCRCs(t *testing.T) {
	tests := []struct {
		msg  api.Message
		want string
	}{
		{&ipsec.IpsecSadEntryAddDelV3{}, "c77ebd92"},
		{&ipsec.IpsecSadEntryAddDelV3Reply{}, "9ffac24b"},
		{&ipsec.IpsecSpdEntryAddDelV2{}, "7bfe69fc"},
		{&ipsec.IpsecSpdEntryAddDelV2Reply{}, "9ffac24b"},
	}
	for _, tt := range tests {
		if got := tt.msg.GetCrcString(); got != tt.want {
			t.Errorf("%s crc = %q, want %q", tt.msg.GetMessageName(), got, tt.want)
		}
		requireResolvable(t, tt.msg)
	}
}

func testSAParams() SAParams {
	return SAParams{
		SPI:       0x11223344,
		Src:       net.ParseIP("192.0.2.1"),
		Dst:       net.ParseIP("198.51.100.1"),
		Dir:       SADirOut,
		Proto:     ProtoESP,
		Mode:      ModeTunnel,
		ReplayWin: 32,
		EncAlgo:   "aes256",
		EncKey:    make([]byte, 32),
		AuthAlgo:  "sha256",
		AuthKey:   make([]byte, 32),
	}
}

// testSPParams is the OUTBOUND policy of the SA testSAParams installs: the tunnel
// endpoints and SAID match that SA, so the backend can resolve the SAD id it
// allocated (spdEntry, vpp_policy.go). IfIndex is the VPP interface the policy's SPD is
// bound to (vppPolicyInterface).
func testSPParams() SPParams {
	_, src, _ := net.ParseCIDR("192.0.2.0/24")
	_, dst, _ := net.ParseCIDR("198.51.100.0/24")
	return SPParams{
		Src:       src,
		Dst:       dst,
		Dir:       SADirOut,
		Proto:     ProtoESP,
		Mode:      ModeTunnel,
		IfIndex:   3,
		Action:    SPActionProtect,
		Priority:  PriorityChildSA,
		TunnelSrc: net.ParseIP("192.0.2.1"),
		TunnelDst: net.ParseIP("198.51.100.1"),
		SAID:      0x11223344,
	}
}

// installedPolicyBackend returns a backend that has already installed testSAParams,
// so a PROTECT policy over it resolves. It leaves the channel empty, so a test reads
// only the messages ITS policy call sent.
func installedPolicyBackend(t *testing.T) (*vppBackend, *capturingChannel) {
	t.Helper()
	b, ch := newCapturingBackend()
	if err := b.InstallSA(testSAParams()); err != nil {
		t.Fatalf("InstallSA: %v", err)
	}
	ch.sent = nil
	return b, ch
}

func sentSAD(t *testing.T, ch *capturingChannel) *ipsec.IpsecSadEntryAddDelV3 {
	t.Helper()
	if len(ch.sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(ch.sent))
	}
	requireResolvable(t, ch.sent[0])
	req, ok := ch.sent[0].(*ipsec.IpsecSadEntryAddDelV3)
	if !ok {
		t.Fatalf("sent %T, want *ipsec.IpsecSadEntryAddDelV3", ch.sent[0])
	}
	return req
}

// sentSPD returns the one SPD ENTRY message in what the backend sent. The SPD
// creation and the interface binding travel with the first policy, and
// TestVPPInstallPolicyCreatesAndBindsSPD is what asserts that chain.
func sentSPD(t *testing.T, ch *capturingChannel) *ipsec.IpsecSpdEntryAddDelV2 {
	t.Helper()
	var found *ipsec.IpsecSpdEntryAddDelV2
	for _, msg := range ch.sent {
		requireResolvable(t, msg)
		req, ok := msg.(*ipsec.IpsecSpdEntryAddDelV2)
		if !ok {
			continue
		}
		if found != nil {
			t.Fatalf("sent %d SPD entry messages, want 1", len(ch.sent))
		}
		found = req
	}
	if found == nil {
		t.Fatalf("sent %d messages and none is an *ipsec.IpsecSpdEntryAddDelV2", len(ch.sent))
	}
	return found
}

func TestVPPInstallSASendsResolvableMessage(t *testing.T) {
	b, ch := newCapturingBackend()
	if err := b.InstallSA(testSAParams()); err != nil {
		t.Fatalf("InstallSA: %v", err)
	}
	req := sentSAD(t, ch)

	if !req.IsAdd {
		t.Error("IsAdd = false, want true")
	}
	// The SPI is what goes on the wire. The SAD id is this backend's own handle for
	// the SA, allocated above the ids VPP already holds (allocSadID, vpp.go), and it
	// is 1 here because this VPP holds none.
	if req.Entry.Spi != 0x11223344 {
		t.Errorf("Spi = %d, want 0x11223344", req.Entry.Spi)
	}
	if req.Entry.SadID != 1 {
		t.Errorf("SadID = %d, want 1", req.Entry.SadID)
	}
	if req.Entry.Tunnel.Src.Un.GetIP4() != (ip_types.IP4Address{192, 0, 2, 1}) {
		t.Errorf("Tunnel.Src = %v, want 192.0.2.1", req.Entry.Tunnel.Src.Un.GetIP4())
	}
	if req.Entry.Tunnel.Dst.Un.GetIP4() != (ip_types.IP4Address{198, 51, 100, 1}) {
		t.Errorf("Tunnel.Dst = %v, want 198.51.100.1", req.Entry.Tunnel.Dst.Un.GetIP4())
	}
	if req.Entry.CryptoAlgorithm != ipsec_types.IPSEC_API_CRYPTO_ALG_AES_CBC_256 {
		t.Errorf("CryptoAlgorithm = %v, want AES_CBC_256", req.Entry.CryptoAlgorithm)
	}
	if req.Entry.IntegrityAlgorithm != ipsec_types.IPSEC_API_INTEG_ALG_SHA_256_128 {
		t.Errorf("IntegrityAlgorithm = %v, want SHA_256_128", req.Entry.IntegrityAlgorithm)
	}
	// Tunnel mode and anti-replay are FLAGS. The struct this replaced had no flags
	// field, so both were off and the tunnel endpoints above were decoration.
	if req.Entry.Flags&ipsec_types.IPSEC_API_SAD_FLAG_IS_TUNNEL == 0 {
		t.Errorf("Flags = %d, want IS_TUNNEL set", req.Entry.Flags)
	}
	if req.Entry.Flags&ipsec_types.IPSEC_API_SAD_FLAG_USE_ANTI_REPLAY == 0 {
		t.Errorf("Flags = %d, want USE_ANTI_REPLAY set for ReplayWin 32", req.Entry.Flags)
	}
	if req.Entry.Flags&ipsec_types.IPSEC_API_SAD_FLAG_IS_TUNNEL_V6 != 0 {
		t.Errorf("Flags = %d, want IS_TUNNEL_V6 clear for an IPv4 tunnel", req.Entry.Flags)
	}
}

// AC-3: the protocol comes from p.Proto, so ESP is 50 rather than the literal 1 that
// stood here, and AH is reachable at all.
func TestVPPInstallSAProtocol(t *testing.T) {
	for _, tt := range []struct {
		proto uint8
		want  ipsec_types.IpsecProto
	}{
		{ProtoESP, ipsec_types.IPSEC_API_PROTO_ESP},
		{ProtoAH, ipsec_types.IPSEC_API_PROTO_AH},
	} {
		b, ch := newCapturingBackend()
		p := testSAParams()
		p.Proto = tt.proto
		if err := b.InstallSA(p); err != nil {
			t.Fatalf("InstallSA proto %d: %v", tt.proto, err)
		}
		if got := sentSAD(t, ch).Entry.Protocol; got != tt.want {
			t.Errorf("Protocol for p.Proto %d = %d, want %d", tt.proto, got, tt.want)
		}
	}

	b, _ := newCapturingBackend()
	p := testSAParams()
	p.Proto = 47
	if err := b.InstallSA(p); !errors.Is(err, ErrNotSupported) {
		t.Errorf("InstallSA with protocol 47 error = %v, want ErrNotSupported", err)
	}
}

// AC-11: an AEAD cipher key and its salt reach VPP in their own fields. EncKey holds
// the key FOLLOWED BY the salt, and all 36 octets used to go into a key field VPP
// sizes at 32 while Salt stayed 0.
func TestVPPInstallSAAEADSaltSplit(t *testing.T) {
	b, ch := newCapturingBackend()
	p := testSAParams()
	p.IsAEAD = true
	p.EncAlgo = "aes256gcm"
	p.EncKey = make([]byte, 36)
	for i := range p.EncKey {
		p.EncKey[i] = byte(i)
	}
	if err := b.InstallSA(p); err != nil {
		t.Fatalf("InstallSA: %v", err)
	}
	req := sentSAD(t, ch)

	if req.Entry.CryptoKey.Length != 32 {
		t.Errorf("CryptoKey.Length = %d, want 32 (the cipher key alone)", req.Entry.CryptoKey.Length)
	}
	if len(req.Entry.CryptoKey.Data) != 32 {
		t.Errorf("CryptoKey.Data = %d octets, want 32", len(req.Entry.CryptoKey.Data))
	}
	// The four KEYMAT octets after the key, in KEYMAT order on the wire.
	if want := binary.BigEndian.Uint32(p.EncKey[32:]); req.Entry.Salt != want {
		t.Errorf("Salt = %#08x, want %#08x", req.Entry.Salt, want)
	}
	if req.Entry.Salt == 0 {
		t.Error("Salt = 0, which is the hardcoded value this test exists to refuse")
	}
	if req.Entry.IntegrityAlgorithm != ipsec_types.IPSEC_API_INTEG_ALG_NONE {
		t.Errorf("IntegrityAlgorithm = %v, want NONE for an AEAD cipher", req.Entry.IntegrityAlgorithm)
	}
}

// An AEAD cipher whose salt length this backend does not know is REFUSED rather than
// keyed at a guessed offset.
func TestVPPInstallSAUnknownAEADRefused(t *testing.T) {
	b, _ := newCapturingBackend()
	p := testSAParams()
	p.IsAEAD = true
	p.EncAlgo = "no-such-aead"
	p.EncKey = make([]byte, 36)
	if err := b.InstallSA(p); !errors.Is(err, ErrNotSupported) {
		t.Errorf("InstallSA error = %v, want ErrNotSupported", err)
	}
}

// An SA whose parameters this backend cannot express is REFUSED, never installed in
// a shape that reports success.
func TestVPPInstallSARefusals(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*SAParams)
	}{
		{"transport mode", func(p *SAParams) { p.Mode = ModeTransport }},
		{"state selector", func(p *SAParams) { p.Sel = &SASelector{} }},
		{"xfrm interface id", func(p *SAParams) { p.IfID = 7 }},
		{"unset direction", func(p *SAParams) { p.Dir = 0 }},
		{"forward direction", func(p *SAParams) { p.Dir = SADirFwd }},
	}
	for _, tt := range tests {
		b, ch := newCapturingBackend()
		p := testSAParams()
		tt.mutate(&p)
		if err := b.InstallSA(p); !errors.Is(err, ErrNotSupported) {
			t.Errorf("%s: error = %v, want ErrNotSupported", tt.name, err)
		}
		if len(ch.sent) != 0 {
			t.Errorf("%s: sent %d messages, want none", tt.name, len(ch.sent))
		}
	}
}

// A tunnel-mode SA with a missing or mixed-family endpoint is REFUSED. A nil address
// reaches VPP as the unspecified address, which matches no traffic.
func TestVPPInstallSAEndpointsRefused(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*SAParams)
	}{
		{"no source", func(p *SAParams) { p.Src = nil }},
		{"no destination", func(p *SAParams) { p.Dst = nil }},
		{"unspecified source", func(p *SAParams) { p.Src = net.IPv4zero }},
		{"mixed families", func(p *SAParams) { p.Dst = net.ParseIP("2001:db8::1") }},
	}
	for _, tt := range tests {
		b, ch := newCapturingBackend()
		p := testSAParams()
		tt.mutate(&p)
		if err := b.InstallSA(p); err == nil {
			t.Errorf("%s: InstallSA returned no error", tt.name)
		}
		if len(ch.sent) != 0 {
			t.Errorf("%s: sent %d messages, want none", tt.name, len(ch.sent))
		}
	}
}

// An inbound SA carries IS_INBOUND and an outbound one does not.
//
// VPP selects an SA for inbound processing by that flag alone. An inbound SA sent
// without it decrypts nothing, and the tunnel carries no traffic. The flag comes from
// SAParams.Dir (vppSAFlags, vpp.go). The engine sets it on each of the two states it
// installs (installChildSA, ike/engine/child.go).
//
// It DISCRIMINATES in both directions: dropping the IS_INBOUND assignment fails the
// first case, and setting the flag unconditionally fails the second.
func TestVPPInstallSAInboundFlag(t *testing.T) {
	for _, tt := range []struct {
		name string
		dir  SADir
		want bool
	}{
		{"inbound", SADirIn, true},
		{"outbound", SADirOut, false},
	} {
		b, ch := newCapturingBackend()
		p := testSAParams()
		p.Dir = tt.dir
		if err := b.InstallSA(p); err != nil {
			t.Fatalf("%s: InstallSA: %v", tt.name, err)
		}
		flags := sentSAD(t, ch).Entry.Flags
		if got := flags&ipsec_types.IPSEC_API_SAD_FLAG_IS_INBOUND != 0; got != tt.want {
			t.Errorf("%s: Flags = %d, IS_INBOUND set = %v, want %v", tt.name, flags, got, tt.want)
		}
	}
}

// An IPv6 tunnel carries IS_TUNNEL_V6 beside IS_TUNNEL.
func TestVPPInstallSAIPv6TunnelFlag(t *testing.T) {
	b, ch := newCapturingBackend()
	p := testSAParams()
	p.Src = net.ParseIP("2001:db8::1")
	p.Dst = net.ParseIP("2001:db8::2")
	if err := b.InstallSA(p); err != nil {
		t.Fatalf("InstallSA: %v", err)
	}
	flags := sentSAD(t, ch).Entry.Flags
	if flags&ipsec_types.IPSEC_API_SAD_FLAG_IS_TUNNEL_V6 == 0 {
		t.Errorf("Flags = %d, want IS_TUNNEL_V6 set", flags)
	}
}

func TestVPPRemoveSA(t *testing.T) {
	b, ch := newCapturingBackend()
	p := testSAParams()
	if err := b.InstallSA(p); err != nil {
		t.Fatalf("InstallSA: %v", err)
	}
	installed := sentSAD(t, ch).Entry.SadID

	ch.sent = nil
	if err := b.RemoveSA(p.SPI, p.Dst, p.Proto); err != nil {
		t.Fatalf("RemoveSA: %v", err)
	}
	req := sentSAD(t, ch)
	if req.IsAdd {
		t.Error("IsAdd = true, want false")
	}
	if req.Entry.SadID != installed {
		t.Errorf("SadID = %d, want %d (the id the install allocated)", req.Entry.SadID, installed)
	}
}

// An SA this backend never installed cannot be deleted, because VPP deletes by the id
// this backend allocated and there is none. The SPI used to be sent as the id, which
// deleted whatever SA held that number.
func TestVPPRemoveSAUnknownRefused(t *testing.T) {
	b, ch := newCapturingBackend()
	p := testSAParams()
	if err := b.RemoveSA(p.SPI, p.Dst, p.Proto); !errors.Is(err, ErrNotSupported) {
		t.Errorf("RemoveSA of an SA never installed: error = %v, want ErrNotSupported", err)
	}
	if len(ch.sent) != 0 {
		t.Errorf("sent %d messages, want none", len(ch.sent))
	}
}

// AC-8, and the collision ISSUE-5 named: two peers that chose the SAME outbound SPI
// get DIFFERENT VPP SAD ids, and each policy names its own SA.
//
// RFC 7296 Section 2.8 has the receiver choose the SPI, so an outbound SPI is the
// peer's number and two peers can pick one value. The SPI was the SAD id, so the
// second InstallSA overwrote the first and both policies then resolved to the
// survivor. It DISCRIMINATES: restoring SadID = p.SPI makes the two ids equal.
func TestVPPInstallSASameSPIDifferentPeers(t *testing.T) {
	b, ch := newCapturingBackend()

	first := testSAParams()
	first.Dst = net.ParseIP("198.51.100.1")
	if err := b.InstallSA(first); err != nil {
		t.Fatalf("InstallSA first peer: %v", err)
	}
	firstID := sentSAD(t, ch).Entry.SadID

	ch.sent = nil
	second := testSAParams()
	second.Dst = net.ParseIP("203.0.113.9") // the same SPI, a different peer
	if err := b.InstallSA(second); err != nil {
		t.Fatalf("InstallSA second peer: %v", err)
	}
	secondID := sentSAD(t, ch).Entry.SadID

	if firstID == secondID {
		t.Fatalf("both peers got SAD id %d, so the second SA overwrote the first", firstID)
	}

	// Each peer's policy names ITS OWN SA.
	for _, tt := range []struct {
		name string
		dst  net.IP
		want uint32
	}{
		{"first peer", first.Dst, firstID},
		{"second peer", second.Dst, secondID},
	} {
		ch.sent = nil
		p := testSPParams()
		p.TunnelDst = tt.dst
		if err := b.InstallPolicy(p); err != nil {
			t.Fatalf("%s: InstallPolicy: %v", tt.name, err)
		}
		if got := sentSPD(t, ch).Entry.SaID; got != tt.want {
			t.Errorf("%s: SaID = %d, want %d", tt.name, got, tt.want)
		}
	}
}

// A rekey that re-installs the SAME SA keeps its id, so VPP updates one entry rather
// than growing a second beside it.
func TestVPPInstallSAReinstallKeepsID(t *testing.T) {
	b, ch := newCapturingBackend()
	if err := b.InstallSA(testSAParams()); err != nil {
		t.Fatalf("InstallSA: %v", err)
	}
	first := sentSAD(t, ch).Entry.SadID

	ch.sent = nil
	if err := b.InstallSA(testSAParams()); err != nil {
		t.Fatalf("InstallSA again: %v", err)
	}
	if got := sentSAD(t, ch).Entry.SadID; got != first {
		t.Errorf("re-install SadID = %d, want %d", got, first)
	}
}

// The SAD id starts ABOVE what VPP already holds, read from VPP. Starting at 1 would
// overwrite an SA left by an earlier run of this process, or one another API client
// owns.
func TestVPPInstallSASkipsSADIDsVPPHolds(t *testing.T) {
	b, ch := newCapturingBackend()
	ch.sads = []uint32{1, 7, 4}
	if err := b.InstallSA(testSAParams()); err != nil {
		t.Fatalf("InstallSA: %v", err)
	}
	if got := sentSAD(t, ch).Entry.SadID; got != 8 {
		t.Errorf("SadID = %d, want 8 (one past the highest id VPP holds)", got)
	}
}

// AC-5 and AC-8: the SPD entry carries the selector as an address RANGE and names
// the SA it protects with.
func TestVPPInstallPolicy(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	p := testSPParams()
	p.UpperProto = 89 // OSPF, RFC 4552
	if err := b.InstallPolicy(p); err != nil {
		t.Fatalf("InstallPolicy: %v", err)
	}
	req := sentSPD(t, ch)

	if !req.IsAdd || !req.Entry.IsOutbound {
		t.Errorf("IsAdd/IsOutbound = %v/%v, want true/true", req.IsAdd, req.Entry.IsOutbound)
	}
	if req.Entry.Policy != ipsec_types.IPSEC_API_SPD_ACTION_PROTECT {
		t.Errorf("Policy = %v, want PROTECT", req.Entry.Policy)
	}
	// The SAD id the InstallSA above allocated, NOT the SPI SPParams.SAID carries.
	if req.Entry.SaID != 1 {
		t.Errorf("SaID = %d, want 1 (the id allocated for the SA this policy names)", req.Entry.SaID)
	}
	if req.Entry.SpdID == 0 {
		t.Error("SpdID = 0, and VPP creates no SPD of its own, so entry 0 lands in no database")
	}
	if req.Entry.LocalAddressStart.Un.GetIP4() != (ip_types.IP4Address{192, 0, 2, 0}) ||
		req.Entry.LocalAddressStop.Un.GetIP4() != (ip_types.IP4Address{192, 0, 2, 255}) {
		t.Errorf("local range = %v..%v, want 192.0.2.0..192.0.2.255",
			req.Entry.LocalAddressStart.Un.GetIP4(), req.Entry.LocalAddressStop.Un.GetIP4())
	}
	if req.Entry.RemoteAddressStart.Un.GetIP4() != (ip_types.IP4Address{198, 51, 100, 0}) ||
		req.Entry.RemoteAddressStop.Un.GetIP4() != (ip_types.IP4Address{198, 51, 100, 255}) {
		t.Errorf("remote range = %v..%v, want 198.51.100.0..198.51.100.255",
			req.Entry.RemoteAddressStart.Un.GetIP4(), req.Entry.RemoteAddressStop.Un.GetIP4())
	}
	// The SPD protocol names the UPPER-LAYER protocol of the matched traffic. ESP (50)
	// stood here, which matched ESP-in-ESP and nothing an operator asked for.
	if req.Entry.Protocol != 89 {
		t.Errorf("Protocol = %d, want 89 (the upper-layer selector)", req.Entry.Protocol)
	}
	// Any port is the whole range. A zero pair matches port 0 alone.
	if req.Entry.LocalPortStart != 0 || req.Entry.LocalPortStop != 65535 ||
		req.Entry.RemotePortStart != 0 || req.Entry.RemotePortStop != 65535 {
		t.Errorf("port ranges = %d..%d / %d..%d, want 0..65535 on both sides",
			req.Entry.LocalPortStart, req.Entry.LocalPortStop,
			req.Entry.RemotePortStart, req.Entry.RemotePortStop)
	}
	// Ze ranks lower first and VPP ranks higher first, so PriorityChildSA (2000) must
	// not outrank PriorityIKEBypass (100) once translated.
	if req.Entry.Priority != -PriorityChildSA {
		t.Errorf("Priority = %d, want %d", req.Entry.Priority, -PriorityChildSA)
	}
	if vppPriority(PriorityIKEBypass) <= vppPriority(PriorityChildSA) {
		t.Errorf("vppPriority inverts the ranking: IKE bypass %d, child SA %d",
			vppPriority(PriorityIKEBypass), vppPriority(PriorityChildSA))
	}
}

// Ze's "every protocol" is 0 and VPP's is 255. MEASURED on VPP v26.06: a policy sent
// with protocol 0 is printed by `show ipsec spd` as
// "protocol IP6_HOP_BY_HOP_OPTIONS", and one sent with 255 as "protocol any", which
// is also what VPP's own CLI produces when no protocol is given.
//
// The zero used to be passed through, so every IKE Child SA policy would have matched
// IPv6 hop-by-hop options and protected no traffic while VPP reported it installed.
// It DISCRIMINATES: passing p.UpperProto straight through fails the first case.
func TestVPPInstallPolicyAnyProtocol(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	p := testSPParams()
	p.UpperProto = 0
	if err := b.InstallPolicy(p); err != nil {
		t.Fatalf("InstallPolicy: %v", err)
	}
	if got := sentSPD(t, ch).Entry.Protocol; got != 255 {
		t.Errorf("Protocol for Ze's any-protocol 0 = %d, want 255 (VPP's any)", got)
	}

	// 255 as an INPUT is refused: IANA reserves it, and passing it through would be
	// indistinguishable from the every-protocol selector above.
	b, ch = installedPolicyBackend(t)
	p = testSPParams()
	p.UpperProto = 255
	if err := b.InstallPolicy(p); !errors.Is(err, ErrNotSupported) {
		t.Errorf("InstallPolicy with upper protocol 255: error = %v, want ErrNotSupported", err)
	}
	if len(ch.sent) != 0 {
		t.Errorf("sent %d messages, want none", len(ch.sent))
	}
}

// AC-8: a protect policy that names no SA is REFUSED. Zero is a valid VPP SAD id, so
// sending it protects with whatever holds that id, or with nothing. So is a policy
// naming an SA this backend never installed: it would resolve to an id it did not
// allocate.
func TestVPPInstallPolicyProtectWithoutSARefused(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*SPParams)
	}{
		{"no SA named", func(p *SPParams) { p.SAID = 0 }},
		{"an SA that was never installed", func(p *SPParams) { p.SAID = 0x99999999 }},
		{"an SA installed toward a different peer", func(p *SPParams) { p.TunnelDst = net.ParseIP("203.0.113.9") }},
	} {
		b, ch := installedPolicyBackend(t)
		p := testSPParams()
		tt.mutate(&p)
		if err := b.InstallPolicy(p); !errors.Is(err, ErrNotSupported) {
			t.Errorf("%s: InstallPolicy error = %v, want ErrNotSupported", tt.name, err)
		}
		if len(ch.sent) != 0 {
			t.Errorf("%s: sent %d messages, want none", tt.name, len(ch.sent))
		}
	}
}

// ISSUE-4: a policy whose direction names neither inbound nor outbound is REFUSED, as
// vppUnsupportedSA refuses the same thing for an SA.
//
// ipsec_spd_entry_v2 carries one boolean, is_outbound, so SADirFwd and an unset Dir
// both reached VPP as INBOUND. plugins/ospf/ipsec_install.go produces SADirFwd
// (buildIPsecPolicies), and it was refused only because that policy is also transport
// mode and the mode check ran first. It DISCRIMINATES: this fixture is tunnel mode
// and BYPASS, so no other check in spdEntry reaches it.
func TestVPPInstallPolicyDirectionRefused(t *testing.T) {
	for _, tt := range []struct {
		name string
		dir  SADir
	}{
		{"forward", SADirFwd},
		{"unset", 0},
	} {
		b, ch := installedPolicyBackend(t)
		p := testSPParams()
		p.Action = SPActionBypass
		p.SAID = 0
		p.Dir = tt.dir
		if err := b.InstallPolicy(p); !errors.Is(err, ErrNotSupported) {
			t.Errorf("%s direction: InstallPolicy error = %v, want ErrNotSupported", tt.name, err)
		}
		if len(ch.sent) != 0 {
			t.Errorf("%s direction: sent %d messages, want none", tt.name, len(ch.sent))
		}
	}
}

// BLOCKER-2: the first policy CREATES an SPD and BINDS it to its interface, and the
// entry names that SPD.
//
// VPP creates no SPD of its own and has no node-wide one. The entry used to carry a
// hardcoded spd_id 0, and nothing in the tree ever sent ipsec_spd_add_del or
// ipsec_interface_add_del_spd, so every entry landed in a database nobody made.
//
// The id is the lowest VPP does not already hold: an id VPP holds belongs to whoever
// created it, and adding entries to it would put these policies in another client's
// database.
func TestVPPInstallPolicyCreatesAndBindsSPD(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	ch.spds = []uint32{1, 2} // two SPDs another API client already owns
	p := testSPParams()
	if err := b.InstallPolicy(p); err != nil {
		t.Fatalf("InstallPolicy: %v", err)
	}
	if len(ch.sent) != 3 {
		t.Fatalf("sent %d messages, want 3 (spd add, interface bind, entry add)", len(ch.sent))
	}

	add, ok := ch.sent[0].(*ipsec.IpsecSpdAddDel)
	if !ok {
		t.Fatalf("first message is %T, want *ipsec.IpsecSpdAddDel", ch.sent[0])
	}
	requireResolvable(t, add)
	if !add.IsAdd || add.SpdID != 3 {
		t.Errorf("spd add = {IsAdd:%v SpdID:%d}, want {true 3}", add.IsAdd, add.SpdID)
	}

	bind, ok := ch.sent[1].(*ipsec.IpsecInterfaceAddDelSpd)
	if !ok {
		t.Fatalf("second message is %T, want *ipsec.IpsecInterfaceAddDelSpd", ch.sent[1])
	}
	requireResolvable(t, bind)
	if !bind.IsAdd || bind.SpdID != add.SpdID || bind.SwIfIndex != 3 {
		t.Errorf("spd bind = {IsAdd:%v SpdID:%d SwIfIndex:%d}, want {true %d 3}",
			bind.IsAdd, bind.SpdID, bind.SwIfIndex, add.SpdID)
	}
	if got := sentSPD(t, ch).Entry.SpdID; got != add.SpdID {
		t.Errorf("entry SpdID = %d, want the SPD that was just created (%d)", got, add.SpdID)
	}

	// The SECOND policy on the same interface reuses both. An SPD created twice is a
	// second database, and the entries would split between them.
	ch.sent = nil
	if err := b.InstallPolicy(p); err != nil {
		t.Fatalf("second InstallPolicy: %v", err)
	}
	if len(ch.sent) != 1 {
		t.Fatalf("second policy sent %d messages, want 1 (the entry alone)", len(ch.sent))
	}
}

// A policy that names no interface is REFUSED. VPP has no node-wide SPD, and
// sw_if_index 0 is a real VPP interface, so reading an unset IfIndex as "any" would
// program the wrong one. IKE leaves IfIndex 0 (SPParams.IfIndex, dataplane.go).
func TestVPPInstallPolicyWithoutInterfaceRefused(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	p := testSPParams()
	p.IfIndex = 0
	if err := b.InstallPolicy(p); !errors.Is(err, ErrNotSupported) {
		t.Errorf("InstallPolicy error = %v, want ErrNotSupported", err)
	}
	if len(ch.sent) != 0 {
		t.Errorf("sent %d messages, want none", len(ch.sent))
	}
}

// A bypass policy reaches VPP as BYPASS. PROTECT was hardcoded, so a bypass was
// silently downgraded to a protect policy and black-holed the traffic it was meant
// to let through.
//
// This is the IKE bypass in the shape engine/bypass.go installs it: no mode, no SA,
// and one exact destination port. A bypass carries no template, so it needs no mode
// (SPActionBypass, dataplane.go).
func TestVPPInstallPolicyBypass(t *testing.T) {
	b, ch := newCapturingBackend()
	p := testSPParams()
	p.TunnelSrc, p.TunnelDst = nil, nil
	p.Action = SPActionBypass
	p.SAID = 0
	p.Mode = 0
	p.Priority = PriorityIKEBypass
	p.UpperProto = 17 // UDP
	p.SrcPort = AnyPortMatch()
	p.DstPort = ExactPortMatch(500)
	if err := b.InstallPolicy(p); err != nil {
		t.Fatalf("InstallPolicy: %v", err)
	}
	req := sentSPD(t, ch)
	if req.Entry.Policy != ipsec_types.IPSEC_API_SPD_ACTION_BYPASS {
		t.Errorf("Policy = %v, want BYPASS", req.Entry.Policy)
	}
	if req.Entry.SaID != 0 {
		t.Errorf("SaID = %d, want 0 for a bypass", req.Entry.SaID)
	}
	if req.Entry.RemotePortStart != 500 || req.Entry.RemotePortStop != 500 {
		t.Errorf("remote port range = %d..%d, want 500..500",
			req.Entry.RemotePortStart, req.Entry.RemotePortStop)
	}
	if req.Entry.LocalPortStart != 0 || req.Entry.LocalPortStop != 65535 {
		t.Errorf("local port range = %d..%d, want 0..65535",
			req.Entry.LocalPortStart, req.Entry.LocalPortStop)
	}
	if req.Entry.Priority != -PriorityIKEBypass {
		t.Errorf("Priority = %d, want %d", req.Entry.Priority, -PriorityIKEBypass)
	}
}

// A port selector that is neither every port nor exactly one is REFUSED, because an
// SPD entry carries a contiguous range.
func TestVPPInstallPolicyPartialPortMaskRefused(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	p := testSPParams()
	p.DstPort = PortMatch{Port: 500, Mask: 0xff00}
	if err := b.InstallPolicy(p); !errors.Is(err, ErrNotSupported) {
		t.Errorf("InstallPolicy error = %v, want ErrNotSupported", err)
	}
	if len(ch.sent) != 0 {
		t.Errorf("sent %d messages, want none", len(ch.sent))
	}
}

// AC-9: p.Mode is read for a PROTECT policy, so a transport-mode policy and a
// tunnel-mode policy do not reach VPP as the same request. ipsec_spd_entry_v2 has no
// mode field at all, so the only honest answer for transport mode is a refusal.
func TestVPPInstallPolicyModeIsRead(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	p := testSPParams()
	p.Mode = ModeTransport
	if err := b.InstallPolicy(p); !errors.Is(err, ErrNotSupported) {
		t.Errorf("transport-mode InstallPolicy error = %v, want ErrNotSupported", err)
	}
	if len(ch.sent) != 0 {
		t.Errorf("sent %d messages for a transport-mode policy, want none", len(ch.sent))
	}
}

// The delete carries the same entry the add carried, because VPP matches an SPD entry
// by its whole content rather than by a key.
func TestVPPRemovePolicyParams(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	p := testSPParams()
	if err := b.InstallPolicy(p); err != nil {
		t.Fatalf("InstallPolicy: %v", err)
	}
	added := sentSPD(t, ch).Entry

	ch.sent = nil
	if err := b.RemovePolicyParams(p); err != nil {
		t.Fatalf("RemovePolicyParams: %v", err)
	}
	req := sentSPD(t, ch)
	if req.IsAdd {
		t.Error("IsAdd = true, want false")
	}
	if !reflect.DeepEqual(req.Entry, added) {
		t.Errorf("delete entry = %+v, want the entry the add sent %+v", req.Entry, added)
	}
	if len(ch.sent) != 1 {
		t.Errorf("the delete sent %d messages, want 1: it must not create an SPD", len(ch.sent))
	}
}

// A delete before any install is REFUSED. This backend has created no SPD, so there
// is no database to delete from, and creating one would report success over an empty
// one.
func TestVPPRemovePolicyParamsWithoutSPDRefused(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	if err := b.RemovePolicyParams(testSPParams()); !errors.Is(err, ErrNotSupported) {
		t.Errorf("RemovePolicyParams error = %v, want ErrNotSupported", err)
	}
	if len(ch.sent) != 0 {
		t.Errorf("sent %d messages, want none", len(ch.sent))
	}
}

// VALIDATES: the SAD entry ze builds for a tunnel-mode Child SA asks VPP to copy the
// ECN congestion indication in BOTH directions. VPP copies neither unless it is
// asked: TUNNEL_API_ENCAP_DECAP_FLAG_NONE is zero, so an unset EncapDecapFlags is a
// tunnel that discards the indication on decapsulation.
//
// PREVENTS: the flags field going back to its zero value. This is the question the
// spec was opened to answer (owner ruling OR-F of the RFC 7296 pilot): whether this
// backend can copy the ECN bits at all. It could not, because it could not program an
// SA at all.
//
// RFC requirement: RFC7296-2.24-1 positive -- RFC 7296 Section 2.24: "tunnel
// encapsulators and decapsulators for all tunnel mode SAs created by IKEv2 MUST
// support the ECN full-functionality option for tunnels". This drives InstallSA, the
// whole of the SAParams-to-VPP mapping, and reads the message it sends.
// RFC requirement: RFC7296-2.24-2 positive -- the same section: an implementation
// "MUST implement the tunnel encapsulation and decapsulation processing specified in
// [IPSECARCH] to prevent discarding of ECN congestion indications". DECAP_COPY_ECN is
// the decapsulation half, and it is what stops the indication being discarded.
func TestVPPInstallSACopiesECN(t *testing.T) {
	b, ch := newCapturingBackend()
	if err := b.InstallSA(testSAParams()); err != nil {
		t.Fatalf("InstallSA: %v", err)
	}
	got := sentSAD(t, ch).Entry.Tunnel.EncapDecapFlags
	if got&tunnel_types.TUNNEL_API_ENCAP_DECAP_FLAG_ENCAP_COPY_ECN == 0 {
		t.Errorf("EncapDecapFlags = %d, which does not carry ENCAP_COPY_ECN: the outer header would not "+
			"carry the inner congestion indication", got)
	}
	if got&tunnel_types.TUNNEL_API_ENCAP_DECAP_FLAG_DECAP_COPY_ECN == 0 {
		t.Errorf("EncapDecapFlags = %d, which does not carry DECAP_COPY_ECN: a congestion indication set "+
			"in the tunnel would be discarded at the far end", got)
	}
}

// RFC requirement: RFC7296-2.24-1 negative -- the assertion above is not vacuous. The
// flags are read off the message the production path SENT, that message is the SA ze
// was asked to install, and Section 2.24 binds TUNNEL mode, so the entry must be a
// tunnel-mode entry for either row to hold over it.
// RFC requirement: RFC7296-2.24-2 negative -- an SA this backend REFUSES sends
// nothing, so the flags cannot be read off a message that was never built. Without
// this the positive rows would hold over a request VPP never saw.
func TestVPPInstallSAECNIsOnTheSAZeInstalls(t *testing.T) {
	b, ch := newCapturingBackend()
	p := testSAParams()
	if err := b.InstallSA(p); err != nil {
		t.Fatalf("InstallSA: %v", err)
	}
	entry := sentSAD(t, ch).Entry
	if entry.Spi != p.SPI {
		t.Errorf("the message carries SPI %#x, want %#x: the flags were read off an SA ze did not build",
			entry.Spi, p.SPI)
	}
	if entry.Flags&ipsec_types.IPSEC_API_SAD_FLAG_IS_TUNNEL == 0 {
		t.Error("the entry is not flagged IS_TUNNEL, and Section 2.24 binds tunnel mode, " +
			"so an ECN assertion over it would prove nothing")
	}

	// A mode this backend cannot express is refused rather than sent as a tunnel, so
	// no unflagged default reaches VPP wearing the ECN flags.
	b, ch = newCapturingBackend()
	p.Mode = ModeTransport
	if err := b.InstallSA(p); !errors.Is(err, ErrNotSupported) {
		t.Errorf("transport-mode InstallSA error = %v, want ErrNotSupported", err)
	}
	if len(ch.sent) != 0 {
		t.Errorf("sent %d messages for a mode this backend cannot express, want none", len(ch.sent))
	}
}

// An SA whose add VPP REFUSES is not recorded as installed. The SAD id map is what
// spdEntry's protect guard reads, so an identity left behind by a refused add would
// let a policy protect with an id VPP never gave out.
func TestVPPInstallSARefusedForgetsItsSADID(t *testing.T) {
	b, ch := newCapturingBackend()
	ch.refuse = map[string]int32{(&ipsec.IpsecSadEntryAddDelV3{}).GetMessageName(): -1}

	p := testSAParams()
	if err := b.InstallSA(p); err == nil {
		t.Fatal("InstallSA reported success for an add VPP refused")
	}
	if len(b.sadIDs) != 0 {
		t.Errorf("the backend records %d installed SAs after a refused add, want 0", len(b.sadIDs))
	}

	// The guard downstream of the map must now be closed for that SA.
	ch.sent = nil
	if err := b.InstallPolicy(testSPParams()); !errors.Is(err, ErrNotSupported) {
		t.Errorf("a protect policy over the refused SA gave error %v, want ErrNotSupported", err)
	}
	if len(ch.sent) != 0 {
		t.Errorf("the policy sent %d messages, want none: it names an SA VPP does not hold", len(ch.sent))
	}
}

// A re-install VPP refuses keeps the id of the EARLIER successful add. The refusal did
// not remove the SA VPP already holds, so forgetting it would strand that entry and
// close the protect guard over an SA that is really there.
func TestVPPInstallSARefusedReinstallKeepsTheEarlierID(t *testing.T) {
	b, ch := newCapturingBackend()
	p := testSAParams()
	if err := b.InstallSA(p); err != nil {
		t.Fatalf("first InstallSA: %v", err)
	}
	identity, err := saIdentityOf(p.SPI, p.Dst, p.Proto)
	if err != nil {
		t.Fatalf("saIdentityOf: %v", err)
	}
	want := b.sadIDs[identity]

	ch.refuse = map[string]int32{(&ipsec.IpsecSadEntryAddDelV3{}).GetMessageName(): -1}
	if err := b.InstallSA(p); err == nil {
		t.Fatal("the re-install reported success for an add VPP refused")
	}
	if got, ok := b.sadIDs[identity]; !ok || got != want {
		t.Errorf("sad id after a refused re-install = %d (present %v), want %d: the earlier add succeeded",
			got, ok, want)
	}
}

// A policy whose ENTRY ADD is refused leaves no SPD bound to the interface.
//
// The pre-send refusals hold that invariant by ordering alone: spdEntry runs before
// ensureSPD and vppPolicyInterface runs before the lock, so nothing reaches VPP. This
// is the path that does not: the SPD is created and BOUND before the entry is sent,
// and a refused entry used to leave it bound with nothing in it.
func TestVPPInstallPolicyEntryRefusedRemovesTheSPDItCreated(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	ch.refuse = map[string]int32{(&ipsec.IpsecSpdEntryAddDelV2{}).GetMessageName(): -1}

	if err := b.InstallPolicy(testSPParams()); err == nil {
		t.Fatal("InstallPolicy reported success for an entry VPP refused")
	}
	if b.spdID != 0 {
		t.Errorf("the backend still holds SPD %d after the entry was refused", b.spdID)
	}
	if len(b.spdBound) != 0 {
		t.Errorf("the backend still holds %d interface bindings, want 0", len(b.spdBound))
	}

	// spd add, interface bind, entry add, interface unbind, spd del.
	unbind, ok := ch.sent[len(ch.sent)-2].(*ipsec.IpsecInterfaceAddDelSpd)
	if !ok {
		t.Fatalf("the fourth message is %T, want *ipsec.IpsecInterfaceAddDelSpd", ch.sent[len(ch.sent)-2])
	}
	requireResolvable(t, unbind)
	if unbind.IsAdd || unbind.SwIfIndex != 3 {
		t.Errorf("unbind = {IsAdd:%v SwIfIndex:%d}, want {false 3}", unbind.IsAdd, unbind.SwIfIndex)
	}
	del, ok := ch.sent[len(ch.sent)-1].(*ipsec.IpsecSpdAddDel)
	if !ok {
		t.Fatalf("the last message is %T, want *ipsec.IpsecSpdAddDel", ch.sent[len(ch.sent)-1])
	}
	requireResolvable(t, del)
	if del.IsAdd || del.SpdID != unbind.SpdID {
		t.Errorf("spd del = {IsAdd:%v SpdID:%d}, want {false %d}", del.IsAdd, del.SpdID, unbind.SpdID)
	}
}

// A policy whose entry add is refused in an SPD this backend ALREADY had leaves that
// SPD alone. Only what the failed call created is undone: the earlier policies live in
// that database, and deleting it would remove them.
func TestVPPInstallPolicyEntryRefusedKeepsAnSPDItDidNotCreate(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	if err := b.InstallPolicy(testSPParams()); err != nil {
		t.Fatalf("first InstallPolicy: %v", err)
	}
	spdID := b.spdID

	ch.sent = nil
	ch.refuse = map[string]int32{(&ipsec.IpsecSpdEntryAddDelV2{}).GetMessageName(): -1}
	if err := b.InstallPolicy(testSPParams()); err == nil {
		t.Fatal("InstallPolicy reported success for an entry VPP refused")
	}
	if b.spdID != spdID {
		t.Errorf("spdID = %d after a refused entry, want the SPD the earlier policy lives in (%d)", b.spdID, spdID)
	}
	if !b.spdBound[3] {
		t.Error("the interface was unbound, and the earlier policy needs that binding to act")
	}
	if len(ch.sent) != 1 {
		t.Errorf("sent %d messages, want 1 (the refused entry alone)", len(ch.sent))
	}
}

// Close REMOVES what this backend installed, policy first, then closes the channel.
//
// Nothing in VPP expires an SA or an SPD, and neither carries a name or an owner, so
// the only moment this backend can tell its own state from another API client's is
// while it still holds the maps that record it.
func TestVPPCloseRemovesWhatItInstalled(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	if err := b.InstallPolicy(testSPParams()); err != nil {
		t.Fatalf("InstallPolicy: %v", err)
	}
	spdID := b.spdID
	installed := sentSPD(t, ch).Entry

	ch.sent = nil
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if len(ch.sent) != 4 {
		t.Fatalf("Close sent %d messages, want 4 (interface unbind, spd entry del, spd del, sa del)", len(ch.sent))
	}

	unbind, ok := ch.sent[0].(*ipsec.IpsecInterfaceAddDelSpd)
	if !ok {
		t.Fatalf("first message is %T, want *ipsec.IpsecInterfaceAddDelSpd", ch.sent[0])
	}
	requireResolvable(t, unbind)
	if unbind.IsAdd || unbind.SpdID != spdID || unbind.SwIfIndex != 3 {
		t.Errorf("unbind = {IsAdd:%v SpdID:%d SwIfIndex:%d}, want {false %d 3}",
			unbind.IsAdd, unbind.SpdID, unbind.SwIfIndex, spdID)
	}
	// The ENTRY goes back before the SPD does. MEASURED on VPP v26.06: deleting the
	// SPD alone left the SA of its PROTECT entry installed, and reported success.
	entryDel, ok := ch.sent[1].(*ipsec.IpsecSpdEntryAddDelV2)
	if !ok {
		t.Fatalf("second message is %T, want *ipsec.IpsecSpdEntryAddDelV2", ch.sent[1])
	}
	requireResolvable(t, entryDel)
	if entryDel.IsAdd || !reflect.DeepEqual(entryDel.Entry, installed) {
		t.Errorf("spd entry del = {IsAdd:%v Entry:%+v}, want {false, the entry the add sent}",
			entryDel.IsAdd, entryDel.Entry)
	}
	del, ok := ch.sent[2].(*ipsec.IpsecSpdAddDel)
	if !ok {
		t.Fatalf("third message is %T, want *ipsec.IpsecSpdAddDel", ch.sent[2])
	}
	if del.IsAdd || del.SpdID != spdID {
		t.Errorf("spd del = {IsAdd:%v SpdID:%d}, want {false %d}", del.IsAdd, del.SpdID, spdID)
	}
	saDel, ok := ch.sent[3].(*ipsec.IpsecSadEntryAddDelV3)
	if !ok {
		t.Fatalf("fourth message is %T, want *ipsec.IpsecSadEntryAddDelV3", ch.sent[3])
	}
	requireResolvable(t, saDel)
	if saDel.IsAdd || saDel.Entry.SadID == 0 {
		t.Errorf("sa del = {IsAdd:%v SadID:%d}, want {false, the id InstallSA allocated}",
			saDel.IsAdd, saDel.Entry.SadID)
	}
	if len(b.sadIDs) != 0 || b.spdID != 0 || len(b.spdBound) != 0 {
		t.Errorf("after Close the backend still records %d SAs, spd %d and %d bindings, want none",
			len(b.sadIDs), b.spdID, len(b.spdBound))
	}
}

// A removal VPP refuses does not strand the removals after it, and Close reports the
// refusal rather than swallowing it. One SA that will not delete must not leave the
// SPD bound to the interface.
func TestVPPCloseReportsARefusedRemovalAndContinues(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	if err := b.InstallPolicy(testSPParams()); err != nil {
		t.Fatalf("InstallPolicy: %v", err)
	}

	ch.sent = nil
	ch.refuse = map[string]int32{(&ipsec.IpsecSadEntryAddDelV3{}).GetMessageName(): -1}
	if err := b.Close(); err == nil {
		t.Fatal("Close reported success while VPP refused the SA delete")
	}
	if len(ch.sent) != 4 {
		t.Errorf("Close sent %d messages, want 4: the refusal must not stop the rest", len(ch.sent))
	}
	if b.spdID != 0 || len(b.spdBound) != 0 {
		t.Errorf("the SPD (%d) or its %d bindings survived a refusal on a different message",
			b.spdID, len(b.spdBound))
	}
	if len(b.sadIDs) != 1 {
		t.Errorf("the backend forgot the SA VPP refused to delete: %d recorded, want 1", len(b.sadIDs))
	}
}

// A policy already removed through RemovePolicyParams is not sent back at Close. The
// record of what this backend installed must shrink when a policy goes, or the close
// would delete an entry VPP no longer holds and report the refusal as an error.
func TestVPPCloseSkipsAPolicyAlreadyRemoved(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	p := testSPParams()
	if err := b.InstallPolicy(p); err != nil {
		t.Fatalf("InstallPolicy: %v", err)
	}
	if err := b.RemovePolicyParams(p); err != nil {
		t.Fatalf("RemovePolicyParams: %v", err)
	}

	ch.sent = nil
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	for _, msg := range ch.sent {
		if _, ok := msg.(*ipsec.IpsecSpdEntryAddDelV2); ok {
			t.Error("Close sent an SPD entry delete for a policy RemovePolicyParams had already removed")
		}
	}
}

// A backend that installed nothing sends nothing on Close, so closing a session that
// never programmed VPP cannot delete another client's SPD 0 or SAD 0.
func TestVPPCloseWithoutInstallsSendsNothing(t *testing.T) {
	b, ch := newCapturingBackend()
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if len(ch.sent) != 0 {
		t.Errorf("Close sent %d messages for a backend that installed nothing, want none", len(ch.sent))
	}
	if err := b.Close(); err != nil {
		t.Errorf("the second Close returned %v, want nil", err)
	}
}

// RemovePolicy by address and direction alone is REFUSED. Those three cannot name a
// VPP SPD entry, so the delete would miss or remove a different policy.
func TestVPPRemovePolicyRefused(t *testing.T) {
	b, ch := newCapturingBackend()
	_, src, _ := net.ParseCIDR("192.0.2.0/24")
	_, dst, _ := net.ParseCIDR("198.51.100.0/24")
	if err := b.RemovePolicy(src, dst, SADirOut); !errors.Is(err, ErrNotSupported) {
		t.Errorf("RemovePolicy error = %v, want ErrNotSupported", err)
	}
	if len(ch.sent) != 0 {
		t.Errorf("sent %d messages, want none", len(ch.sent))
	}
}
