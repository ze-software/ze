//go:build linux

package firewallnft

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
)

// VALIDATES: Category A -- lowerFamily rejects unknown values.
// PREVENTS: silent default to inet when an invalid family reaches the backend.
func TestLowerFamilyUnknownRejects(t *testing.T) {
	if _, err := lowerFamily(firewall.TableFamily(0)); err == nil {
		t.Fatal("lowerFamily(0) must reject")
	}
	if _, err := lowerFamily(firewall.TableFamily(99)); err == nil {
		t.Fatal("lowerFamily(99) must reject")
	}
	got, err := lowerFamily(firewall.FamilyInet)
	if err != nil {
		t.Fatalf("lowerFamily(Inet): %v", err)
	}
	if got != nftables.TableFamilyINet {
		t.Errorf("lowerFamily(Inet) = %v, want %v", got, nftables.TableFamilyINet)
	}
}

// VALIDATES: Category A -- lowerHook rejects unknown hooks.
// PREVENTS: silent fall-back to ingress for an arbitrary hook value.
func TestLowerHookUnknownRejects(t *testing.T) {
	if _, err := lowerHook(firewall.ChainHook(0)); err == nil {
		t.Fatal("lowerHook(0) must reject")
	}
	if _, err := lowerHook(firewall.ChainHook(99)); err == nil {
		t.Fatal("lowerHook(99) must reject")
	}
}

// VALIDATES: Category A -- lowerFlowtableHook rejects any non-ingress hook.
// PREVENTS: operator asking for an input/output/forward flowtable and getting an ingress
// flowtable programmed instead.
func TestLowerFlowtableHookRejectsNonIngress(t *testing.T) {
	for _, h := range []firewall.ChainHook{
		firewall.HookInput, firewall.HookOutput, firewall.HookForward,
		firewall.HookPrerouting, firewall.HookPostrouting, firewall.HookEgress,
	} {
		if _, err := lowerFlowtableHook(h); err == nil {
			t.Errorf("lowerFlowtableHook(%q) must reject", h)
		}
	}
	if _, err := lowerFlowtableHook(firewall.HookIngress); err != nil {
		t.Errorf("lowerFlowtableHook(Ingress): %v", err)
	}
}

// VALIDATES: Category A -- lowerChainType / lowerPolicy / lowerSetType reject
// unknown values instead of defaulting.
// PREVENTS: a corrupt or zero-valued chain type, policy, or set type becoming
// filter / accept / ipv4 at Apply.
func TestLowerEnumsRejectUnknown(t *testing.T) {
	if _, err := lowerChainType(firewall.ChainType(0)); err == nil {
		t.Error("lowerChainType(0) must reject")
	}
	if _, err := lowerPolicy(firewall.Policy(0)); err == nil {
		t.Error("lowerPolicy(0) must reject")
	}
	if _, err := lowerSetType(firewall.SetType(0)); err == nil {
		t.Error("lowerSetType(0) must reject")
	}
}

// VALIDATES: Category A -- Counter.Name with a non-empty value is rejected.
// PREVENTS: a named counter silently collapsing to an anonymous one.
func TestLowerCounterRejectsName(t *testing.T) {
	if _, err := lowerCounter(firewall.Counter{}); err != nil {
		t.Fatalf("anonymous counter: %v", err)
	}
	_, err := lowerCounter(firewall.Counter{Name: "allow-ssh"})
	if err == nil || !strings.Contains(err.Error(), "named counter") {
		t.Fatalf("lowerCounter(Name=allow-ssh) err = %v, want \"named counter\" rejection", err)
	}
}

// VALIDATES: Category A -- every Log field the operator set reaches the
// kernel: a prefix is emitted only when NFTA_LOG_PREFIX is present in Key;
// the previous code set Key=0 which dropped every attribute including the
// prefix.
// PREVENTS: silent drop of Level / Group / Snaplen / Prefix.
func TestLowerLogEmitsAllFields(t *testing.T) {
	level := uint32(expr.LogLevelWarning)
	group := uint16(7)
	snap := uint32(128)
	exprs, err := lowerLog(firewall.Log{
		Prefix:  "drop: ",
		Level:   &level,
		Group:   &group,
		Snaplen: &snap,
	})
	if err != nil {
		t.Fatalf("lowerLog: %v", err)
	}
	if len(exprs) != 1 {
		t.Fatalf("lowerLog produced %d exprs, want 1", len(exprs))
	}
	got, ok := exprs[0].(*expr.Log)
	if !ok {
		t.Fatalf("lowerLog produced %T, want *expr.Log", exprs[0])
	}
	if string(got.Data) != "drop: " {
		t.Errorf("Data = %q, want %q", got.Data, "drop: ")
	}
	wantKey := uint32(1<<unix.NFTA_LOG_PREFIX | 1<<unix.NFTA_LOG_LEVEL | 1<<unix.NFTA_LOG_GROUP | 1<<unix.NFTA_LOG_SNAPLEN)
	if got.Key != wantKey {
		t.Errorf("Key = %#x, want %#x", got.Key, wantKey)
	}
	if got.Level != expr.LogLevelWarning {
		t.Errorf("Level = %d, want %d", got.Level, expr.LogLevelWarning)
	}
	if got.Group != 7 {
		t.Errorf("Group = %d, want 7", got.Group)
	}
	if got.Snaplen != 128 {
		t.Errorf("Snaplen = %d, want 128", got.Snaplen)
	}
}

// VALIDATES: Category A -- an empty Log still programs nothing silently;
// prefix-only emits only the prefix bit.
func TestLowerLogPrefixOnly(t *testing.T) {
	exprs, err := lowerLog(firewall.Log{Prefix: "x"})
	if err != nil {
		t.Fatalf("lowerLog: %v", err)
	}
	got, _ := exprs[0].(*expr.Log)
	if got.Key != 1<<unix.NFTA_LOG_PREFIX {
		t.Errorf("Key = %#x, want prefix-only %#x", got.Key, uint32(1<<unix.NFTA_LOG_PREFIX))
	}
}

// VALIDATES: Category A -- an explicit `level 0` (syslog emerg) reaches
// the kernel with the NFTA_LOG_LEVEL bit set, rather than being silently
// remapped to the kernel default (warning) because 0 looked like
// "unset".
// PREVENTS: operator writes `log level 0` and the kernel logs at warning.
func TestLowerLogExplicitLevelZero(t *testing.T) {
	zero := uint32(0)
	exprs, err := lowerLog(firewall.Log{Level: &zero})
	if err != nil {
		t.Fatalf("lowerLog: %v", err)
	}
	got, _ := exprs[0].(*expr.Log)
	if got.Key&(1<<unix.NFTA_LOG_LEVEL) == 0 {
		t.Errorf("NFTA_LOG_LEVEL bit missing for explicit level 0: Key = %#x", got.Key)
	}
	if got.Level != expr.LogLevelEmerg {
		t.Errorf("Level = %d, want LogLevelEmerg (%d)", got.Level, expr.LogLevelEmerg)
	}
}

// VALIDATES: Category A -- an unset Level / Group / Snaplen does NOT set
// its Key bit, leaving the kernel defaults in force.
func TestLowerLogUnsetLeavesDefaults(t *testing.T) {
	exprs, err := lowerLog(firewall.Log{Prefix: "x"})
	if err != nil {
		t.Fatalf("lowerLog: %v", err)
	}
	got, _ := exprs[0].(*expr.Log)
	if got.Key&(1<<unix.NFTA_LOG_LEVEL) != 0 {
		t.Error("NFTA_LOG_LEVEL set when Level was nil")
	}
	if got.Key&(1<<unix.NFTA_LOG_GROUP) != 0 {
		t.Error("NFTA_LOG_GROUP set when Group was nil")
	}
	if got.Key&(1<<unix.NFTA_LOG_SNAPLEN) != 0 {
		t.Error("NFTA_LOG_SNAPLEN set when Snaplen was nil")
	}
}

// VALIDATES: Category A -- Masquerade fields the backend cannot program
// (Port, PortEnd, Flags) reject rather than silently disappear.
func TestLowerMasqueradeRejectsUnsupportedFields(t *testing.T) {
	if _, err := lowerMasquerade(firewall.Masquerade{}); err != nil {
		t.Fatalf("plain masquerade: %v", err)
	}
	if _, err := lowerMasquerade(firewall.Masquerade{Port: 1024}); err == nil {
		t.Error("masquerade with port must reject")
	}
	if _, err := lowerMasquerade(firewall.Masquerade{PortEnd: 2048}); err == nil {
		t.Error("masquerade with port end must reject")
	}
	if _, err := lowerMasquerade(firewall.Masquerade{Flags: 1}); err == nil {
		t.Error("masquerade with flags must reject")
	}
}

// VALIDATES: Category A -- lowerNAT honors PortEnd via RegProtoMax and
// rejects Flags until they are wired through.
// PREVENTS: an addr:lo-hi SNAT/DNAT silently collapsing to addr:lo.
func TestLowerNATPortRange(t *testing.T) {
	addr := netip.MustParseAddr("198.51.100.10")
	exprs, err := lowerNAT(addr, netip.Addr{}, 1024, 2048, 0, expr.NATTypeSourceNAT)
	if err != nil {
		t.Fatalf("lowerNAT(range): %v", err)
	}
	var nat *expr.NAT
	for _, e := range exprs {
		if n, ok := e.(*expr.NAT); ok {
			nat = n
		}
	}
	if nat == nil {
		t.Fatal("lowerNAT produced no *expr.NAT")
	}
	if nat.RegProtoMin != 2 {
		t.Errorf("RegProtoMin = %d, want 2", nat.RegProtoMin)
	}
	if nat.RegProtoMax != 3 {
		t.Errorf("RegProtoMax = %d, want 3", nat.RegProtoMax)
	}
}

// VALIDATES: Category A -- NAT flags reject until the backend can program them.
func TestLowerNATRejectsFlags(t *testing.T) {
	addr := netip.MustParseAddr("198.51.100.10")
	_, err := lowerNAT(addr, netip.Addr{}, 1024, 0, 1, expr.NATTypeSourceNAT)
	if err == nil {
		t.Error("lowerNAT with flags must reject")
	}
}

// VALIDATES: Category A -- inverted NAT port ranges reject instead of
// being programmed as a backwards range.
func TestLowerNATRejectsInvertedRange(t *testing.T) {
	addr := netip.MustParseAddr("198.51.100.10")
	if _, err := lowerNAT(addr, netip.Addr{}, 2048, 1024, 0, expr.NATTypeSourceNAT); err == nil {
		t.Error("lowerNAT(2048-1024) must reject inverted range")
	}
	if _, err := lowerNAT(addr, netip.Addr{}, 0, 2048, 0, expr.NATTypeSourceNAT); err == nil {
		t.Error("lowerNAT(0-2048) must reject range without lower bound")
	}
}

// VALIDATES: happy-path -- every TableFamily the model exposes round-trips
// through lowerFamily to the expected nftables family.
// PREVENTS: dropping a case from the switch and silently emitting a
// different family at Apply.
func TestLowerFamilyAllValid(t *testing.T) {
	tests := []struct {
		in   firewall.TableFamily
		want nftables.TableFamily
	}{
		{firewall.FamilyInet, nftables.TableFamilyINet},
		{firewall.FamilyIP, nftables.TableFamilyIPv4},
		{firewall.FamilyIP6, nftables.TableFamilyIPv6},
		{firewall.FamilyARP, nftables.TableFamilyARP},
		{firewall.FamilyBridge, nftables.TableFamilyBridge},
		{firewall.FamilyNetdev, nftables.TableFamilyNetdev},
	}
	for _, tt := range tests {
		t.Run(tt.in.String(), func(t *testing.T) {
			got, err := lowerFamily(tt.in)
			if err != nil {
				t.Fatalf("lowerFamily(%v): %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("lowerFamily(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// VALIDATES: happy-path -- every ChainHook round-trips to its nftables
// equivalent pointer.
// PREVENTS: dropping a case and silently remapping prerouting to ingress.
func TestLowerHookAllValid(t *testing.T) {
	tests := []struct {
		in   firewall.ChainHook
		want *nftables.ChainHook
	}{
		{firewall.HookInput, nftables.ChainHookInput},
		{firewall.HookOutput, nftables.ChainHookOutput},
		{firewall.HookForward, nftables.ChainHookForward},
		{firewall.HookPrerouting, nftables.ChainHookPrerouting},
		{firewall.HookPostrouting, nftables.ChainHookPostrouting},
		{firewall.HookIngress, nftables.ChainHookIngress},
		{firewall.HookEgress, nftables.ChainHookEgress},
	}
	for _, tt := range tests {
		t.Run(tt.in.String(), func(t *testing.T) {
			got, err := lowerHook(tt.in)
			if err != nil {
				t.Fatalf("lowerHook(%v): %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("lowerHook(%v) = %p, want %p", tt.in, got, tt.want)
			}
		})
	}
}

// VALIDATES: happy-path -- every ChainType round-trips; rejection already
// covered in TestLowerEnumsRejectUnknown.
func TestLowerChainTypeAllValid(t *testing.T) {
	tests := []struct {
		in   firewall.ChainType
		want nftables.ChainType
	}{
		{firewall.ChainFilter, nftables.ChainTypeFilter},
		{firewall.ChainNAT, nftables.ChainTypeNAT},
		{firewall.ChainRoute, nftables.ChainTypeRoute},
	}
	for _, tt := range tests {
		t.Run(tt.in.String(), func(t *testing.T) {
			got, err := lowerChainType(tt.in)
			if err != nil {
				t.Fatalf("lowerChainType(%v): %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("lowerChainType(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// VALIDATES: happy-path -- every Policy round-trips.
func TestLowerPolicyAllValid(t *testing.T) {
	tests := []struct {
		in   firewall.Policy
		want nftables.ChainPolicy
	}{
		{firewall.PolicyAccept, nftables.ChainPolicyAccept},
		{firewall.PolicyDrop, nftables.ChainPolicyDrop},
	}
	for _, tt := range tests {
		t.Run(tt.in.String(), func(t *testing.T) {
			got, err := lowerPolicy(tt.in)
			if err != nil {
				t.Fatalf("lowerPolicy(%v): %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("lowerPolicy(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// VALIDATES: happy-path -- every SetType round-trips to the right
// nftables SetDatatype struct. Comparing by Name is enough; the
// underlying values (NFT_DATA_*) are what google/nftables encodes on
// the wire.
func TestLowerSetTypeAllValid(t *testing.T) {
	tests := []struct {
		in       firewall.SetType
		wantName string
	}{
		{firewall.SetTypeIPv4, nftables.TypeIPAddr.Name},
		{firewall.SetTypeIPv6, nftables.TypeIP6Addr.Name},
		{firewall.SetTypeEther, nftables.TypeEtherAddr.Name},
		{firewall.SetTypeInetService, nftables.TypeInetService.Name},
		{firewall.SetTypeMark, nftables.TypeMark.Name},
		{firewall.SetTypeIfname, nftables.TypeIFName.Name},
	}
	for _, tt := range tests {
		t.Run(tt.wantName, func(t *testing.T) {
			got, err := lowerSetType(tt.in)
			if err != nil {
				t.Fatalf("lowerSetType(%v): %v", tt.in, err)
			}
			if got.Name != tt.wantName {
				t.Errorf("lowerSetType(%v).Name = %q, want %q", tt.in, got.Name, tt.wantName)
			}
		})
	}
}

// VALIDATES: regression -- plain SNAT (no port, no range, no flags)
// still produces address-only NAT after the signature refactor that
// added PortEnd + Flags parameters.
// PREVENTS: the simple `snat to 1.2.3.4` config silently breaking.
func TestLowerSNATAddressOnly(t *testing.T) {
	exprs, err := lowerSNAT(firewall.SNAT{Address: netip.MustParseAddr("203.0.113.5")})
	if err != nil {
		t.Fatalf("lowerSNAT: %v", err)
	}
	var nat *expr.NAT
	var imms int
	for _, e := range exprs {
		switch ex := e.(type) {
		case *expr.NAT:
			nat = ex
		case *expr.Immediate:
			imms++
		}
	}
	if nat == nil {
		t.Fatal("no *expr.NAT emitted")
	}
	if nat.Type != expr.NATTypeSourceNAT {
		t.Errorf("Type = %v, want SourceNAT", nat.Type)
	}
	if nat.RegProtoMin != 0 || nat.RegProtoMax != 0 {
		t.Errorf("address-only SNAT set port registers: RegProtoMin=%d RegProtoMax=%d", nat.RegProtoMin, nat.RegProtoMax)
	}
	if imms != 1 {
		t.Errorf("immediates = %d, want 1 (address only)", imms)
	}
}

// VALIDATES: regression -- plain DNAT with a single port sets only the
// lo-port register, not the hi-port register.
func TestLowerDNATSinglePort(t *testing.T) {
	exprs, err := lowerDNAT(firewall.DNAT{
		Address: netip.MustParseAddr("203.0.113.5"),
		Port:    8080,
	})
	if err != nil {
		t.Fatalf("lowerDNAT: %v", err)
	}
	var nat *expr.NAT
	for _, e := range exprs {
		if n, ok := e.(*expr.NAT); ok {
			nat = n
		}
	}
	if nat == nil {
		t.Fatal("no *expr.NAT emitted")
	}
	if nat.Type != expr.NATTypeDestNAT {
		t.Errorf("Type = %v, want DestNAT", nat.Type)
	}
	if nat.RegProtoMin != 2 {
		t.Errorf("RegProtoMin = %d, want 2", nat.RegProtoMin)
	}
	if nat.RegProtoMax != 0 {
		t.Errorf("RegProtoMax = %d, want 0 (single port, no range)", nat.RegProtoMax)
	}
}

// VALIDATES: lowerNAT with an IPv6 address uses AF_INET6 (=10) rather
// than AF_INET. Covers the `addr.Is4()` false branch that no other
// test exercises.
func TestLowerNATIPv6(t *testing.T) {
	addr := netip.MustParseAddr("2001:db8::1")
	exprs, err := lowerNAT(addr, netip.Addr{}, 0, 0, 0, expr.NATTypeDestNAT)
	if err != nil {
		t.Fatalf("lowerNAT(ipv6): %v", err)
	}
	var nat *expr.NAT
	var imm *expr.Immediate
	for _, e := range exprs {
		switch ex := e.(type) {
		case *expr.NAT:
			nat = ex
		case *expr.Immediate:
			imm = ex
		}
	}
	if nat == nil || imm == nil {
		t.Fatalf("expected 1 Immediate + 1 NAT, got %d exprs", len(exprs))
	}
	const AFInet6 = 10
	if nat.Family != AFInet6 {
		t.Errorf("Family = %d, want %d (AF_INET6)", nat.Family, AFInet6)
	}
	if len(imm.Data) != 16 {
		t.Errorf("address bytes = %d, want 16 for IPv6", len(imm.Data))
	}
}

// VALIDATES: anonymous Counter still works after the name-rejection fix.
// PREVENTS: Counter{} stopped producing a counter expression.
func TestLowerCounterAnonymous(t *testing.T) {
	exprs, err := lowerCounter(firewall.Counter{})
	if err != nil {
		t.Fatalf("lowerCounter({}): %v", err)
	}
	if len(exprs) != 1 {
		t.Fatalf("len = %d, want 1", len(exprs))
	}
	if _, ok := exprs[0].(*expr.Counter); !ok {
		t.Errorf("type = %T, want *expr.Counter", exprs[0])
	}
}

// VALIDATES: plain Masquerade{} still produces an expr.Masq; the
// new rejection logic only fires on non-zero fields.
func TestLowerMasqueradePlain(t *testing.T) {
	exprs, err := lowerMasquerade(firewall.Masquerade{})
	if err != nil {
		t.Fatalf("lowerMasquerade({}): %v", err)
	}
	if len(exprs) != 1 {
		t.Fatalf("len = %d, want 1", len(exprs))
	}
	if _, ok := exprs[0].(*expr.Masq); !ok {
		t.Errorf("type = %T, want *expr.Masq", exprs[0])
	}
}

// VALIDATES: Category B -- MatchConnMark lowers via Ct(MARK) rather
// than Meta(MARK), and carries the mask through to the Bitwise step.
// PREVENTS: the parser accepting `connection-mark 0x10/0xff` and
// Apply returning "unsupported match type" (the pre-P0 behavior).
func TestLowerMatchConnMark(t *testing.T) {
	exprs, err := lowerConnMarkMatch(0x10, 0xFF)
	if err != nil {
		t.Fatalf("lowerConnMarkMatch: %v", err)
	}
	if len(exprs) != 3 {
		t.Fatalf("len = %d, want 3 (Ct + Bitwise + Cmp)", len(exprs))
	}
	ct, ok := exprs[0].(*expr.Ct)
	if !ok {
		t.Fatalf("exprs[0] = %T, want *expr.Ct", exprs[0])
	}
	if ct.Key != expr.CtKeyMARK {
		t.Errorf("Ct.Key = %v, want CtKeyMARK", ct.Key)
	}
	if ct.SourceRegister {
		t.Error("Ct.SourceRegister = true; read should have dest register")
	}
}

// VALIDATES: Category B -- SetConnMark writes through Ct(MARK) with
// SourceRegister=true. Full-mask path is immediate+ct, masked path
// reads-clears-ors-writes.
func TestLowerSetConnMarkFullMask(t *testing.T) {
	exprs, err := lowerSetConnMark(0x10, 0xFFFFFFFF)
	if err != nil {
		t.Fatalf("lowerSetConnMark(fullmask): %v", err)
	}
	if len(exprs) != 2 {
		t.Fatalf("len = %d, want 2 (Immediate + Ct write)", len(exprs))
	}
	ct, ok := exprs[1].(*expr.Ct)
	if !ok {
		t.Fatalf("exprs[1] = %T, want *expr.Ct", exprs[1])
	}
	if !ct.SourceRegister {
		t.Error("Ct.SourceRegister = false; write needs source register")
	}
	if ct.Key != expr.CtKeyMARK {
		t.Errorf("Ct.Key = %v, want CtKeyMARK", ct.Key)
	}
}

func TestLowerSetConnMarkMasked(t *testing.T) {
	exprs, err := lowerSetConnMark(0x10, 0xFF)
	if err != nil {
		t.Fatalf("lowerSetConnMark(masked): %v", err)
	}
	if len(exprs) != 3 {
		t.Fatalf("len = %d, want 3 (Ct read + Bitwise + Ct write)", len(exprs))
	}
	first, ok := exprs[0].(*expr.Ct)
	if !ok || first.SourceRegister {
		t.Error("first expr must be Ct read (SourceRegister=false)")
	}
	last, ok := exprs[2].(*expr.Ct)
	if !ok || !last.SourceRegister {
		t.Error("last expr must be Ct write (SourceRegister=true)")
	}
}

// VALIDATES: Category B -- SetDSCP lowers to payload-read + bitwise +
// payload-write with IPv4 header checksum recomputation.
// PREVENTS: the parser accepting `dscp-set ef` and Apply rejecting it.
func TestLowerSetDSCP(t *testing.T) {
	exprs, err := lowerSetDSCP(46) // EF
	if err != nil {
		t.Fatalf("lowerSetDSCP: %v", err)
	}
	if len(exprs) != 3 {
		t.Fatalf("len = %d, want 3 (Payload read + Bitwise + Payload write)", len(exprs))
	}
	read, ok := exprs[0].(*expr.Payload)
	if !ok || read.OperationType != expr.PayloadLoad {
		t.Error("exprs[0] must be a Payload Load")
	}
	if read.Offset != 1 || read.Len != 1 {
		t.Errorf("Payload read offset=%d len=%d, want 1/1 (TOS byte)", read.Offset, read.Len)
	}
	bw, ok := exprs[1].(*expr.Bitwise)
	if !ok {
		t.Fatalf("exprs[1] = %T, want *expr.Bitwise", exprs[1])
	}
	if bw.Mask[0] != 0x03 {
		t.Errorf("Bitwise.Mask = %#x, want 0x03 (preserve ECN)", bw.Mask[0])
	}
	if bw.Xor[0] != 46<<2 {
		t.Errorf("Bitwise.Xor = %#x, want %#x (dscp<<2)", bw.Xor[0], uint8(46<<2))
	}
	write, ok := exprs[2].(*expr.Payload)
	if !ok || write.OperationType != expr.PayloadWrite {
		t.Error("exprs[2] must be a Payload Write")
	}
	if write.CsumType != expr.CsumTypeInet || write.CsumOffset != 10 {
		t.Errorf("checksum = %v/%d, want Inet/10", write.CsumType, write.CsumOffset)
	}
}

// VALIDATES: Category B -- SetDSCP rejects out-of-range values rather
// than truncating. 64 occupies bit 6 which would spill into the ECN
// field once shifted.
func TestLowerSetDSCPOutOfRange(t *testing.T) {
	if _, err := lowerSetDSCP(64); err == nil {
		t.Error("lowerSetDSCP(64) must reject")
	}
	if _, err := lowerSetDSCP(255); err == nil {
		t.Error("lowerSetDSCP(255) must reject")
	}
}

// VALIDATES: Category B -- Redirect with a port loads it into a
// register and hands it to the Redir expression.
// PREVENTS: the parser accepting `redirect to 8080` and Apply rejecting.
func TestLowerRedirectPort(t *testing.T) {
	exprs, err := lowerRedirect(firewall.Redirect{Port: 8080})
	if err != nil {
		t.Fatalf("lowerRedirect: %v", err)
	}
	if len(exprs) != 2 {
		t.Fatalf("len = %d, want 2 (Immediate + Redir)", len(exprs))
	}
	imm, ok := exprs[0].(*expr.Immediate)
	if !ok {
		t.Fatalf("exprs[0] = %T, want *expr.Immediate", exprs[0])
	}
	if len(imm.Data) != 2 {
		t.Errorf("Immediate.Data len = %d, want 2 (port bytes)", len(imm.Data))
	}
	// port 8080 big-endian = 0x1f 0x90
	if imm.Data[0] != 0x1f || imm.Data[1] != 0x90 {
		t.Errorf("Immediate.Data = %v, want [0x1f 0x90] for port 8080", imm.Data)
	}
	red, ok := exprs[1].(*expr.Redir)
	if !ok {
		t.Fatalf("exprs[1] = %T, want *expr.Redir", exprs[1])
	}
	if red.RegisterProtoMin == 0 {
		t.Error("Redir.RegisterProtoMin = 0; must reference the port register")
	}
}

// VALIDATES: Category B -- Redirect without a port produces a bare
// Redir expression (redirects to the same port on localhost). This is
// uncommon but valid at the nftables layer.
func TestLowerRedirectNoPort(t *testing.T) {
	exprs, err := lowerRedirect(firewall.Redirect{})
	if err != nil {
		t.Fatalf("lowerRedirect({}): %v", err)
	}
	if len(exprs) != 1 {
		t.Fatalf("len = %d, want 1 (Redir only)", len(exprs))
	}
	red, ok := exprs[0].(*expr.Redir)
	if !ok {
		t.Fatalf("exprs[0] = %T, want *expr.Redir", exprs[0])
	}
	if red.RegisterProtoMin != 0 {
		t.Errorf("RegisterProtoMin = %d, want 0 (no port load)", red.RegisterProtoMin)
	}
}

// VALIDATES: Category B -- Redirect rejects unsupported flags rather
// than silently dropping them.
func TestLowerRedirectRejectsFlags(t *testing.T) {
	_, err := lowerRedirect(firewall.Redirect{Port: 8080, Flags: 1})
	if err == nil {
		t.Error("lowerRedirect with flags must reject")
	}
}

// VALIDATES: spec-fw-8 AC-1 -- MatchICMPType lowers to a transport
// header payload read + Cmp against a single byte, which is what the
// nftables `icmp type <n>` rule compiles to.
// PREVENTS: LNS rule 40 (icmp type-name echo-request) lowering wrong.
func TestLowerICMPTypeMatch(t *testing.T) {
	exprs, err := lowerICMPTypeMatch(8)
	if err != nil {
		t.Fatalf("lowerICMPTypeMatch: %v", err)
	}
	if len(exprs) != 2 {
		t.Fatalf("len = %d, want 2 (Payload load + Cmp)", len(exprs))
	}
	p, ok := exprs[0].(*expr.Payload)
	if !ok {
		t.Fatalf("exprs[0] = %T, want *expr.Payload", exprs[0])
	}
	if p.OperationType != expr.PayloadLoad {
		t.Errorf("OperationType = %v, want PayloadLoad", p.OperationType)
	}
	if p.Base != expr.PayloadBaseTransportHeader {
		t.Errorf("Base = %v, want PayloadBaseTransportHeader", p.Base)
	}
	if p.Offset != 0 || p.Len != 1 {
		t.Errorf("offset/len = %d/%d, want 0/1 (ICMP type byte)", p.Offset, p.Len)
	}
	c, ok := exprs[1].(*expr.Cmp)
	if !ok {
		t.Fatalf("exprs[1] = %T, want *expr.Cmp", exprs[1])
	}
	if len(c.Data) != 1 || c.Data[0] != 8 {
		t.Errorf("Cmp.Data = %v, want [8]", c.Data)
	}
}

// VALIDATES: spec-fw-8 AC-5 / AC-6 -- exact interface match compares
// all 16 IFNAMSIZ bytes (the NUL-padding enforces no-prefix leak);
// wildcard match compares only len(name) bytes so the kernel does a
// prefix comparison.
// PREVENTS: `l2tp*` failing to match `l2tp1` because we compared 16
// bytes with a name that only has 4.
func TestLowerIfaceMatchExactVsWildcard(t *testing.T) {
	exact, err := lowerIfaceMatch(expr.MetaKeyIIFNAME, "eth0", false)
	if err != nil {
		t.Fatalf("lowerIfaceMatch(exact): %v", err)
	}
	cExact, _ := exact[1].(*expr.Cmp)
	if len(cExact.Data) != 16 {
		t.Errorf("exact Cmp.Data len = %d, want 16 (IFNAMSIZ)", len(cExact.Data))
	}
	if cExact.Data[0] != 'e' || cExact.Data[1] != 't' || cExact.Data[2] != 'h' || cExact.Data[3] != '0' || cExact.Data[4] != 0 {
		t.Errorf("exact Cmp.Data prefix = %v, want \"eth0\\x00\"", cExact.Data[:5])
	}

	wild, err := lowerIfaceMatch(expr.MetaKeyIIFNAME, "l2tp", true)
	if err != nil {
		t.Fatalf("lowerIfaceMatch(wildcard): %v", err)
	}
	cWild, _ := wild[1].(*expr.Cmp)
	if len(cWild.Data) != 4 {
		t.Errorf("wildcard Cmp.Data len = %d, want 4 (prefix only)", len(cWild.Data))
	}
	if string(cWild.Data) != "l2tp" {
		t.Errorf("wildcard Cmp.Data = %q, want %q", cWild.Data, "l2tp")
	}
}

// VALIDATES: empty interface name rejects rather than producing a
// zero-length Cmp that matches every packet.
func TestLowerIfaceMatchEmptyRejects(t *testing.T) {
	if _, err := lowerIfaceMatch(expr.MetaKeyIIFNAME, "", false); err == nil {
		t.Error("empty interface name must reject")
	}
	if _, err := lowerIfaceMatch(expr.MetaKeyIIFNAME, "", true); err == nil {
		t.Error("empty wildcard interface name must reject")
	}
}

// VALIDATES: the unix NFTA_LOG_* constants we pack into expr.Log.Key
// are non-zero and distinct. A silent rename or constant reshuffle in
// VALIDATES: gap-8 -- lowerNAT with AddressEnd emits a second
// Immediate on register 4 and sets RegAddrMax on the NAT expression
// so the kernel programs a pool range rather than collapsing to the
// lower address.
// PREVENTS: `snat to 10.0.0.1-10.0.0.10` silently mapping every
// packet to 10.0.0.1.
func TestLowerNATAddressRange(t *testing.T) {
	lo := netip.MustParseAddr("10.0.0.1")
	hi := netip.MustParseAddr("10.0.0.10")
	exprs, err := lowerNAT(lo, hi, 0, 0, 0, expr.NATTypeSourceNAT)
	if err != nil {
		t.Fatalf("lowerNAT: %v", err)
	}
	var nat *expr.NAT
	var immRegs []uint32
	for _, e := range exprs {
		switch ex := e.(type) {
		case *expr.NAT:
			nat = ex
		case *expr.Immediate:
			immRegs = append(immRegs, ex.Register)
		}
	}
	if nat == nil {
		t.Fatal("no NAT expression emitted")
	}
	if nat.RegAddrMin != 1 || nat.RegAddrMax != 4 {
		t.Errorf("RegAddr{Min,Max} = %d/%d, want 1/4", nat.RegAddrMin, nat.RegAddrMax)
	}
	// The two address immediates must land on r1 and r4, in that order.
	if len(immRegs) != 2 || immRegs[0] != 1 || immRegs[1] != 4 {
		t.Errorf("Immediate registers = %v, want [1 4]", immRegs)
	}
}

// VALIDATES: gap-8 -- inverted address range rejects with a clear
// message. Same posture as the port-range inversion check.
func TestLowerNATRejectsInvertedAddressRange(t *testing.T) {
	lo := netip.MustParseAddr("10.0.0.10")
	hi := netip.MustParseAddr("10.0.0.1")
	if _, err := lowerNAT(lo, hi, 0, 0, 0, expr.NATTypeSourceNAT); err == nil {
		t.Error("expected inverted-range rejection")
	}
}

// VALIDATES: gap-7 -- packet-rate Limit lowers to LimitTypePkts;
// byte-rate Limit lowers to LimitTypePktBytes with the caller-scaled
// Rate flowing into expr.Limit.Rate unchanged. Zero Dimension rejects
// so a Limit built outside parseRateSpec cannot silently produce a
// packet rule.
// PREVENTS: `limit-rate 1mbytes/second` being emitted as a packet
// rate and silently dropping traffic at 1 packet/sec.
func TestLowerLimitDimension(t *testing.T) {
	tests := []struct {
		name    string
		in      firewall.Limit
		wantLim expr.LimitType
	}{
		{"packets", firewall.Limit{Rate: 10, Unit: "second", Dimension: firewall.RateDimensionPackets}, expr.LimitTypePkts},
		{"bytes", firewall.Limit{Rate: 1024 * 1024, Unit: "second", Dimension: firewall.RateDimensionBytes}, expr.LimitTypePktBytes},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprs, err := lowerLimit(tt.in)
			if err != nil {
				t.Fatalf("lowerLimit: %v", err)
			}
			if len(exprs) != 1 {
				t.Fatalf("expected 1 expression, got %d", len(exprs))
			}
			lim, ok := exprs[0].(*expr.Limit)
			if !ok {
				t.Fatalf("expr[0] type = %T, want *expr.Limit", exprs[0])
			}
			if lim.Type != tt.wantLim {
				t.Errorf("Type = %v, want %v", lim.Type, tt.wantLim)
			}
			if lim.Rate != tt.in.Rate {
				t.Errorf("Rate = %d, want %d", lim.Rate, tt.in.Rate)
			}
		})
	}
}

// VALIDATES: gap-7 -- Limit with no Dimension rejects. parseRateSpec
// always sets the field; a zero value means a programmatic caller
// bypassed the parser.
func TestLowerLimitRejectsUnspecifiedDimension(t *testing.T) {
	_, err := lowerLimit(firewall.Limit{Rate: 10, Unit: "second"})
	if err == nil {
		t.Fatal("expected error for unspecified dimension")
	}
}

// VALIDATES: gap-1 -- MatchInSet on a source-address named set lowers to
// Payload(Network, 12, 4) + Lookup against the set. Before this, the
// lowerMatch switch had no case for MatchInSet and the rule silently
// fell through to "unsupported match type".
// PREVENTS: an operator writing `from { source-address "@blocked"; }`
// and discovering at Apply that ze rejects the configured rule.
func TestLowerMatchInSet_SourceAddr_IPv4(t *testing.T) {
	set := &nftables.Set{Name: "blocked", ID: 1, KeyType: nftables.TypeIPAddr}
	ctx := &lowerCtx{sets: map[string]*nftables.Set{"blocked": set}}
	exprs, err := lowerMatch(ctx, firewall.MatchInSet{
		SetName:    "blocked",
		MatchField: firewall.SetFieldSourceAddr,
	})
	if err != nil {
		t.Fatalf("lowerMatch: %v", err)
	}
	if len(exprs) != 2 {
		t.Fatalf("expected 2 expressions, got %d", len(exprs))
	}
	payload, ok := exprs[0].(*expr.Payload)
	if !ok {
		t.Fatalf("expr[0] type = %T, want *expr.Payload", exprs[0])
	}
	if payload.Base != expr.PayloadBaseNetworkHeader || payload.Offset != 12 || payload.Len != 4 {
		t.Errorf("payload = {Base:%v Offset:%d Len:%d}, want {Network 12 4}",
			payload.Base, payload.Offset, payload.Len)
	}
	lookup, ok := exprs[1].(*expr.Lookup)
	if !ok {
		t.Fatalf("expr[1] type = %T, want *expr.Lookup", exprs[1])
	}
	if lookup.SetID != 1 || lookup.SetName != "blocked" {
		t.Errorf("lookup = {SetID:%d SetName:%q}, want {1 blocked}", lookup.SetID, lookup.SetName)
	}
}

// VALIDATES: gap-1 -- MatchInSet on a destination-address named set uses
// the IPv4 destination offset (16) not the source offset (12).
func TestLowerMatchInSet_DestAddr_IPv4(t *testing.T) {
	set := &nftables.Set{Name: "targets", ID: 2, KeyType: nftables.TypeIPAddr}
	ctx := &lowerCtx{sets: map[string]*nftables.Set{"targets": set}}
	exprs, err := lowerMatch(ctx, firewall.MatchInSet{
		SetName:    "targets",
		MatchField: firewall.SetFieldDestAddr,
	})
	if err != nil {
		t.Fatalf("lowerMatch: %v", err)
	}
	payload, ok := exprs[0].(*expr.Payload)
	if !ok {
		t.Fatalf("expr[0] type = %T, want *expr.Payload", exprs[0])
	}
	if payload.Offset != 16 || payload.Len != 4 {
		t.Errorf("payload offset/len = %d/%d, want 16/4", payload.Offset, payload.Len)
	}
}

// VALIDATES: gap-1 -- IPv6 address sets use 16-byte reads at the IPv6
// header offsets (8 for source, 24 for destination), not the IPv4 ones.
func TestLowerMatchInSet_Addr_IPv6(t *testing.T) {
	tests := []struct {
		name       string
		field      firewall.SetFieldType
		wantOffset uint32
	}{
		{"source", firewall.SetFieldSourceAddr, 8},
		{"dest", firewall.SetFieldDestAddr, 24},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := &nftables.Set{Name: "peers", ID: 3, KeyType: nftables.TypeIP6Addr}
			ctx := &lowerCtx{sets: map[string]*nftables.Set{"peers": set}}
			exprs, err := lowerMatch(ctx, firewall.MatchInSet{
				SetName:    "peers",
				MatchField: tt.field,
			})
			if err != nil {
				t.Fatalf("lowerMatch: %v", err)
			}
			payload, ok := exprs[0].(*expr.Payload)
			if !ok {
				t.Fatalf("expr[0] type = %T, want *expr.Payload", exprs[0])
			}
			if payload.Offset != tt.wantOffset || payload.Len != 16 {
				t.Errorf("payload offset/len = %d/%d, want %d/16",
					payload.Offset, payload.Len, tt.wantOffset)
			}
		})
	}
}

// VALIDATES: gap-1 -- MatchInSet referencing a set name that was not
// registered on the table rejects with a clear error instead of
// emitting a Lookup against a zero-valued Set that the kernel would
// refuse at Flush.
func TestLowerMatchInSet_UnknownSet(t *testing.T) {
	ctx := &lowerCtx{sets: map[string]*nftables.Set{}}
	_, err := lowerMatch(ctx, firewall.MatchInSet{
		SetName:    "missing",
		MatchField: firewall.SetFieldSourceAddr,
	})
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("expected error naming the missing set, got %v", err)
	}
}

// VALIDATES: gap-1 -- SetFieldSourceAddr with an inet-service (port) set
// rejects: the field and the set's datatype disagree, so emitting a
// rule against it would compare 4 header bytes against 2-byte entries.
func TestLowerMatchInSet_FieldTypeMismatch(t *testing.T) {
	set := &nftables.Set{Name: "ports", ID: 4, KeyType: nftables.TypeInetService}
	ctx := &lowerCtx{sets: map[string]*nftables.Set{"ports": set}}
	_, err := lowerMatch(ctx, firewall.MatchInSet{
		SetName:    "ports",
		MatchField: firewall.SetFieldSourceAddr,
	})
	if err == nil {
		t.Fatal("expected mismatch rejection, got nil")
	}
}

// golang.org/x/sys would otherwise collapse multiple fields onto the
// same bit without any test catching it.
func TestLogKeyBitsDistinct(t *testing.T) {
	bits := []struct {
		name string
		bit  uint32
	}{
		{"prefix", 1 << unix.NFTA_LOG_PREFIX},
		{"level", 1 << unix.NFTA_LOG_LEVEL},
		{"group", 1 << unix.NFTA_LOG_GROUP},
		{"snaplen", 1 << unix.NFTA_LOG_SNAPLEN},
	}
	seen := make(map[uint32]string)
	for _, b := range bits {
		if b.bit == 0 {
			t.Errorf("%s bit is zero", b.name)
		}
		if other, dup := seen[b.bit]; dup {
			t.Errorf("%s shares bit %#x with %s", b.name, b.bit, other)
		}
		seen[b.bit] = b.name
	}
}
