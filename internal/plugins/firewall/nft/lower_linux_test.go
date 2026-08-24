//go:build linux

package firewallnft

import (
	"bytes"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/firewall"
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

func TestShouldDeleteTableScopesToDesiredOrApplied(t *testing.T) {
	b := &backend{applied: map[string]struct{}{"ze_old": {}}}
	desired := tableNameSet([]firewall.Table{{Name: "ze_new", Family: firewall.FamilyInet}})

	if !b.shouldDeleteTable(&nftables.Table{Name: "ze_new"}, desired) {
		t.Fatal("desired ze table should be replaced")
	}
	if !b.shouldDeleteTable(&nftables.Table{Name: "ze_old"}, desired) {
		t.Fatal("previously applied ze table should be removed as orphan")
	}
	if b.shouldDeleteTable(&nftables.Table{Name: "ze_other"}, desired) {
		t.Fatal("unknown ze table must not be swept by prefix")
	}
	if b.shouldDeleteTable(&nftables.Table{Name: "external"}, desired) {
		t.Fatal("non-ze table must not be touched")
	}
}

// The name/family/chain decision this file used to test in
// TestLegacyTableIsSweptOnlyInTheFamilyZeWroteIt now lives where it is decided,
// firewall.IsLegacyTable, and is tested there with the chain cases this file
// could not reach (internal/component/firewall/legacy_tables_test.go). The
// kernel-reading half of the wrapper is driven end to end against a real kernel
// by TestApplyRemovesTheLegacyUnprefixedTableOnce and
// TestApplyLeavesAnotherToolsTableAlone in integration_linux_test.go. What
// neither of those reaches is the wrapper's three REFUSALS, which return before
// any netlink call, so the test below keeps them covered here.

// VALIDATES: (*backend).isLegacyTable answers no, without touching the kernel,
// for each input that cannot be a table an older ze build wrote.
// PREVENTS: two regressions the integration tests cannot see. They drive the
// wrapper against a live connection, so they exercise only the path that gets
// as far as reading chains. These three returns happen BEFORE that, and the nil
// guard is the one that would turn a defensive check into a panic inside the
// sweep loop if it were dropped. The receiver carries a nil conn on purpose: a
// case that reached netlink would panic here rather than pass quietly.
func TestIsLegacyTableRefusesBeforeItReadsTheKernel(t *testing.T) {
	b := &backend{}

	if b.isLegacyTable(nil) {
		t.Fatal("a nil table must not be swept")
	}
	if b.isLegacyTable(&nftables.Table{Name: "flowspec", Family: nftables.TableFamilyUnspecified}) {
		t.Fatal("a family raiseFamily cannot name must not be swept")
	}
	if b.isLegacyTable(&nftables.Table{Name: "operator-table", Family: nftables.TableFamilyINet}) {
		t.Fatal("a name ze never wrote bare must not be swept")
	}
	if b.isLegacyTable(&nftables.Table{Name: "flowspec", Family: nftables.TableFamilyIPv4}) {
		t.Fatal("a legacy name in another family must not be swept")
	}
	if b.isLegacyTable(&nftables.Table{Name: "ze_flowspec", Family: nftables.TableFamilyINet}) {
		t.Fatal("the renamed table is owned by its prefix, not by the legacy list")
	}
}

// VALIDATES: fw-10 AC-13 -- an element `timeout` leaf reaches the kernel:
// lowerSet maps SetFlagTimeout to nftables.Set.HasTimeout and converts each
// element's uint32 seconds to time.Duration. The vendored library emits
// NFTA_SET_ELEM_TIMEOUT only when the parent set has HasTimeout
// (vendor/github.com/google/nftables/set.go SetAddElements), so a set
// without the flag silently drops every element timeout.
// PREVENTS: regression of the firewall firewall-set-element-timeout failure where
// applySet never set HasTimeout and elements were programmed timeout-less.
func TestLowerSetTimeoutFlagAndElementTimeouts(t *testing.T) {
	table := &nftables.Table{Name: "ze_t", Family: nftables.TableFamilyINet}
	s := &firewall.Set{
		Name:  "transient",
		Type:  firewall.SetTypeIPv4,
		Flags: firewall.SetFlagTimeout,
		Elements: []firewall.SetElement{
			{Value: "10.0.0.1", Timeout: 3600},
			{Value: "10.0.0.2"},
		},
	}
	nftSet, elements, err := lowerSet(table, s)
	if err != nil {
		t.Fatalf("lowerSet: %v", err)
	}
	if !nftSet.HasTimeout {
		t.Error("SetFlagTimeout must lower to nftables.Set.HasTimeout=true; without it the library drops every element timeout")
	}
	if len(elements) != 2 {
		t.Fatalf("lowerSet elements = %d, want 2", len(elements))
	}
	if got, want := elements[0].Timeout, time.Hour; got != want {
		t.Errorf("element[0].Timeout = %v, want %v", got, want)
	}
	if elements[1].Timeout != 0 {
		t.Errorf("element[1].Timeout = %v, want 0 (unset stays no-timeout)", elements[1].Timeout)
	}

	s.Flags = firewall.SetFlagInterval
	nftSet, _, err = lowerSet(table, s)
	if err != nil {
		t.Fatalf("lowerSet(interval): %v", err)
	}
	if nftSet.HasTimeout {
		t.Error("HasTimeout must be false without SetFlagTimeout")
	}
	if !nftSet.Interval {
		t.Error("SetFlagInterval must still lower to Interval=true")
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

// VALIDATES: a masquerade port-range end without a start port is rejected --
// the only unsupported masquerade shape after port mapping and flags support
// landed. Port, Port+PortEnd, and the random/fully-random/persistent flags are
// now programmed by lowerMasquerade and covered by
// TestLowerMasqueradeWith{Ports,PortsSingle,Flags} and test/firewall/015,016.
func TestLowerMasqueradeRejectsPortEndWithoutStartPort(t *testing.T) {
	if _, err := lowerMasquerade(firewall.Masquerade{}); err != nil {
		t.Fatalf("plain masquerade: %v", err)
	}
	if _, err := lowerMasquerade(firewall.Masquerade{PortEnd: 2048}); err == nil {
		t.Error("masquerade with port-range end but no start port must reject")
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

func TestLowerMasqueradeWithPorts(t *testing.T) {
	m := firewall.Masquerade{Port: 1024, PortEnd: 65535}
	exprs, err := lowerMasquerade(m)
	if err != nil {
		t.Fatalf("lowerMasquerade: %v", err)
	}
	// Expect: Immediate(reg1, port_min) + Immediate(reg2, port_max) + Masq{ToPorts: true, ...}
	if len(exprs) < 2 {
		t.Fatalf("len = %d, want >= 2", len(exprs))
	}
	masq, ok := exprs[len(exprs)-1].(*expr.Masq)
	if !ok {
		t.Fatalf("last expr = %T, want *expr.Masq", exprs[len(exprs)-1])
	}
	if !masq.ToPorts {
		t.Error("ToPorts = false, want true")
	}
	if masq.RegProtoMin == 0 {
		t.Error("RegProtoMin = 0, want non-zero")
	}
}

func TestLowerMasqueradeWithPortsSingle(t *testing.T) {
	m := firewall.Masquerade{Port: 8080}
	exprs, err := lowerMasquerade(m)
	if err != nil {
		t.Fatalf("lowerMasquerade: %v", err)
	}
	masq, ok := exprs[len(exprs)-1].(*expr.Masq)
	if !ok {
		t.Fatalf("last expr = %T, want *expr.Masq", exprs[len(exprs)-1])
	}
	if !masq.ToPorts {
		t.Error("ToPorts = false, want true")
	}
	if masq.RegProtoMax != 0 {
		t.Errorf("RegProtoMax = %d, want 0 (single port)", masq.RegProtoMax)
	}
}

func TestLowerMasqueradeWithFlags(t *testing.T) {
	tests := []struct {
		name string
		flag uint32
		want func(*expr.Masq) bool
	}{
		{"random", firewall.MasqFlagRandom, func(m *expr.Masq) bool { return m.Random }},
		{"random full", firewall.MasqFlagFullyRandom, func(m *expr.Masq) bool { return m.FullyRandom }},
		{"persistent", firewall.MasqFlagPersistent, func(m *expr.Masq) bool { return m.Persistent }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := firewall.Masquerade{Flags: tt.flag}
			exprs, err := lowerMasquerade(m)
			if err != nil {
				t.Fatalf("lowerMasquerade: %v", err)
			}
			if len(exprs) != 1 {
				t.Fatalf("len = %d, want 1", len(exprs))
			}
			masq, ok := exprs[0].(*expr.Masq)
			if !ok {
				t.Fatalf("type = %T, want *expr.Masq", exprs[0])
			}
			if !tt.want(masq) {
				t.Errorf("flag %s not set on expr.Masq", tt.name)
			}
			if masq.ToPorts {
				t.Error("ToPorts = true, want false (flags path)")
			}
		})
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
	exprs, err := lowerSetDSCP(unix.NFPROTO_IPV4, 46) // EF
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

// VALIDATES: SetTCPMSS lowers to Immediate(r1, mss) + Exthdr(write,
// tcp option 2, offset 2, len 2). This is the nftables equivalent of
// `tcp option maxseg size set <mss>`.
func TestLowerSetTCPMSS(t *testing.T) {
	exprs, err := lowerSetTCPMSS(1400)
	if err != nil {
		t.Fatalf("lowerSetTCPMSS: %v", err)
	}
	if len(exprs) != 2 {
		t.Fatalf("len = %d, want 2 (Immediate + Exthdr)", len(exprs))
	}
	imm, ok := exprs[0].(*expr.Immediate)
	if !ok {
		t.Fatalf("exprs[0] = %T, want *expr.Immediate", exprs[0])
	}
	if imm.Register != 1 {
		t.Errorf("Immediate.Register = %d, want 1", imm.Register)
	}
	if len(imm.Data) != 2 || imm.Data[0] != 0x05 || imm.Data[1] != 0x78 {
		t.Errorf("Immediate.Data = %v, want [0x05 0x78] for MSS 1400", imm.Data)
	}
	exthdr, ok := exprs[1].(*expr.Exthdr)
	if !ok {
		t.Fatalf("exprs[1] = %T, want *expr.Exthdr", exprs[1])
	}
	if exthdr.SourceRegister != 1 {
		t.Errorf("Exthdr.SourceRegister = %d, want 1", exthdr.SourceRegister)
	}
	if exthdr.Type != 2 {
		t.Errorf("Exthdr.Type = %d, want 2 (TCP MSS option kind)", exthdr.Type)
	}
	if exthdr.Offset != 2 {
		t.Errorf("Exthdr.Offset = %d, want 2 (past kind+length)", exthdr.Offset)
	}
	if exthdr.Len != 2 {
		t.Errorf("Exthdr.Len = %d, want 2 (MSS is uint16)", exthdr.Len)
	}
	if exthdr.Op != expr.ExthdrOpTcpopt {
		t.Errorf("Exthdr.Op = %v, want ExthdrOpTcpopt", exthdr.Op)
	}
}

// VALIDATES: SetTCPMSS rejects zero MSS.
func TestLowerSetTCPMSSZeroRejects(t *testing.T) {
	if _, err := lowerSetTCPMSS(0); err == nil {
		t.Error("lowerSetTCPMSS(0) must reject")
	}
}

// VALIDATES: Category B -- SetDSCP rejects out-of-range values rather
// than truncating. 64 occupies bit 6 which would spill into the ECN
// field once shifted.
func TestLowerSetDSCPOutOfRange(t *testing.T) {
	if _, err := lowerSetDSCP(unix.NFPROTO_IPV4, 64); err == nil {
		t.Error("lowerSetDSCP(64) must reject")
	}
	if _, err := lowerSetDSCP(unix.NFPROTO_IPV4, 255); err == nil {
		t.Error("lowerSetDSCP(255) must reject")
	}
	if _, err := lowerSetDSCP(unix.NFPROTO_IPV6, 64); err == nil {
		t.Error("lowerSetDSCP(ipv6, 64) must reject")
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
	exprs, err := lowerICMPTypeMatch(unix.IPPROTO_ICMP, 8)
	if err != nil {
		t.Fatalf("lowerICMPTypeMatch: %v", err)
	}
	if len(exprs) != 4 {
		t.Fatalf("len = %d, want 4 (l4proto dependency + Payload load + Cmp)", len(exprs))
	}
	rest := l4protoDepRest(t, exprs, unix.IPPROTO_ICMP)
	p, ok := rest[0].(*expr.Payload)
	if !ok {
		t.Fatalf("expression after the dependency = %T, want *expr.Payload", rest[0])
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
	c, ok := rest[1].(*expr.Cmp)
	if !ok {
		t.Fatalf("expression after the payload = %T, want *expr.Cmp", rest[1])
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

// inetCtx builds a lowering context whose parent table is an inet table
// holding the named sets. The family is stated rather than defaulted, so a
// test reads as an assertion about inet tables specifically.
func inetCtx(sets map[string]*nftables.Set) *lowerCtx {
	return &lowerCtx{
		table: &nftables.Table{Name: "t", Family: nftables.TableFamilyINet},
		sets:  sets,
	}
}

// lowerTermOneRule asserts that a term lowers to exactly ONE rule and returns
// its expressions. Most terms do; a term that lowers to two is a family split
// and its test says so by reading both rules.
func lowerTermOneRule(t *testing.T, ctx *lowerCtx, term *firewall.Term) []expr.Any {
	t.Helper()
	rules, err := lowerTerm(ctx, term)
	if err != nil {
		t.Fatalf("lowerTerm: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("lowerTerm produced %d rules, want 1", len(rules))
	}
	return rules[0]
}

// l4protoDepRest asserts that exprs opens with the `meta l4proto <want>`
// dependency pair, and returns the expressions that follow it. want is an IANA
// protocol number, e.g. unix.IPPROTO_ICMP or unix.IPPROTO_ICMPV6.
func l4protoDepRest(t *testing.T, exprs []expr.Any, want byte) []expr.Any {
	t.Helper()
	if len(exprs) < 2 {
		t.Fatalf("expected at least the 2-expression l4proto dependency, got %d", len(exprs))
	}
	meta, ok := exprs[0].(*expr.Meta)
	if !ok {
		t.Fatalf("expr[0] type = %T, want *expr.Meta carrying l4proto", exprs[0])
	}
	if meta.Key != expr.MetaKeyL4PROTO {
		t.Fatalf("expr[0] meta key = %v, want MetaKeyL4PROTO", meta.Key)
	}
	if meta.Register != 1 {
		t.Errorf("expr[0] meta register = %d, want 1", meta.Register)
	}
	cmp, ok := exprs[1].(*expr.Cmp)
	if !ok {
		t.Fatalf("expr[1] type = %T, want *expr.Cmp", exprs[1])
	}
	if cmp.Op != expr.CmpOpEq {
		t.Errorf("expr[1] cmp op = %v, want CmpOpEq", cmp.Op)
	}
	if cmp.Register != 1 {
		t.Errorf("expr[1] cmp register = %d, want 1", cmp.Register)
	}
	if len(cmp.Data) != 1 || cmp.Data[0] != want {
		t.Fatalf("expr[1] cmp data = %v, want [%d]", cmp.Data, want)
	}
	return exprs[2:]
}

// nfprotoGuardRest asserts that exprs opens with the `meta nfproto <want>`
// guard pair, and returns the expressions that follow it. want is
// unix.NFPROTO_IPV4 or unix.NFPROTO_IPV6.
func nfprotoGuardRest(t *testing.T, exprs []expr.Any, want byte) []expr.Any {
	t.Helper()
	if len(exprs) < 2 {
		t.Fatalf("expected at least the 2-expression nfproto guard, got %d", len(exprs))
	}
	meta, ok := exprs[0].(*expr.Meta)
	if !ok {
		t.Fatalf("expr[0] type = %T, want *expr.Meta carrying nfproto", exprs[0])
	}
	if meta.Key != expr.MetaKeyNFPROTO {
		t.Fatalf("expr[0] meta key = %v, want MetaKeyNFPROTO", meta.Key)
	}
	if meta.Register != 1 {
		t.Errorf("expr[0] meta register = %d, want 1", meta.Register)
	}
	cmp, ok := exprs[1].(*expr.Cmp)
	if !ok {
		t.Fatalf("expr[1] type = %T, want *expr.Cmp", exprs[1])
	}
	if cmp.Op != expr.CmpOpEq {
		t.Errorf("expr[1] cmp op = %v, want CmpOpEq", cmp.Op)
	}
	if cmp.Register != 1 {
		t.Errorf("expr[1] cmp register = %d, want 1", cmp.Register)
	}
	if len(cmp.Data) != 1 {
		t.Fatalf("expr[1] cmp data = %v, want one byte", cmp.Data)
	}
	if cmp.Data[0] != want {
		t.Errorf("expr[1] cmp data = %d, want %d", cmp.Data[0], want)
	}
	return exprs[2:]
}

// VALIDATES: gap-1 -- MatchInSet on a source-address named set lowers to
// Payload(Network, 12, 4) + Lookup against the set. Before this, the
// lowerMatch switch had no case for MatchInSet and the rule silently
// fell through to "unsupported match type".
// PREVENTS: an operator writing `from { source-address "@blocked"; }`
// and discovering at Apply that ze rejects the configured rule.
func TestLowerMatchInSet_SourceAddr_IPv4(t *testing.T) {
	set := &nftables.Set{Name: "blocked", ID: 1, KeyType: nftables.TypeIPAddr}
	ctx := inetCtx(map[string]*nftables.Set{"blocked": set})
	exprs, err := lowerMatch(ctx, firewall.MatchInSet{
		SetName:    "blocked",
		MatchField: firewall.SetFieldSourceAddr,
	})
	if err != nil {
		t.Fatalf("lowerMatch: %v", err)
	}
	exprs = nfprotoGuardRest(t, exprs, unix.NFPROTO_IPV4)
	if len(exprs) != 2 {
		t.Fatalf("expected 2 expressions after the guard, got %d", len(exprs))
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
	ctx := inetCtx(map[string]*nftables.Set{"targets": set})
	exprs, err := lowerMatch(ctx, firewall.MatchInSet{
		SetName:    "targets",
		MatchField: firewall.SetFieldDestAddr,
	})
	if err != nil {
		t.Fatalf("lowerMatch: %v", err)
	}
	exprs = nfprotoGuardRest(t, exprs, unix.NFPROTO_IPV4)
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
			ctx := inetCtx(map[string]*nftables.Set{"peers": set})
			exprs, err := lowerMatch(ctx, firewall.MatchInSet{
				SetName:    "peers",
				MatchField: tt.field,
			})
			if err != nil {
				t.Fatalf("lowerMatch: %v", err)
			}
			exprs = nfprotoGuardRest(t, exprs, unix.NFPROTO_IPV6)
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

// TestLowerProtoMatchAcceptsEveryCanonicalName is the wiring row for the term
// the FlowSpec bridge now produces: whatever name translation emits, this
// backend must lower.
//
// VALIDATES: every name firewall.ProtocolNames returns lowers to an L4PROTO
// comparison against its IANA number, and a spelling outside the table is
// refused rather than programmed as protocol 0.
// PREVENTS: a producer and this backend disagreeing about the accepted names.
// That disagreement is not rule-local: Apply returns a lowering error before
// its single Flush, so one unlowerable term leaves every owner's ruleset
// unapplied in the kernel.
func TestLowerProtoMatchAcceptsEveryCanonicalName(t *testing.T) {
	for _, name := range firewall.ProtocolNames() {
		exprs, err := lowerProtoMatch(name)
		if err != nil {
			t.Fatalf("canonical protocol %q must lower: %v", name, err)
		}
		if len(exprs) != 2 {
			t.Fatalf("protocol %q lowered to %d expressions, want 2", name, len(exprs))
		}
		meta, ok := exprs[0].(*expr.Meta)
		if !ok || meta.Key != expr.MetaKeyL4PROTO {
			t.Fatalf("protocol %q did not load the L4PROTO meta key, got %#v", name, exprs[0])
		}
		cmp, ok := exprs[1].(*expr.Cmp)
		if !ok {
			t.Fatalf("protocol %q did not compare, got %#v", name, exprs[1])
		}
		num, _ := firewall.ProtocolNumber(name)
		if len(cmp.Data) != 1 || cmp.Data[0] != num {
			t.Errorf("protocol %q compared against %v, want the IANA number %d", name, cmp.Data, num)
		}
	}

	for _, name := range []string{"", "132", "TCP", "bogus"} {
		if _, err := lowerProtoMatch(name); err == nil {
			t.Errorf("lowerProtoMatch(%q) must refuse a name outside the canonical table", name)
		}
	}
}

// --- nfproto guard on family-specific network-header reads ---
//
// An inet table's chains see IPv4 and IPv6 packets alike, and an address
// match reads the network header raw. Without a leading `meta nfproto`
// guard the IPv4 destination offset (16) lands inside the IPv6 SOURCE
// address (IPv6 src spans bytes 8..23), so an IPv4 rule matches IPv6
// traffic the operator never named. nft's own compiler emits the guard for
// an inet table and omits it for ip / ip6, where nfproto is constant.

// VALIDATES: a literal address match in an inet table emits
// `meta nfproto ipv4|ipv6` BEFORE the network-header payload load, driven
// through lowerTerm, the entry point applyChain calls.
// PREVENTS: an IPv4 FlowSpec or firewall rule for 10.1.0.0/24 also
// dropping IPv6 traffic whose source address carries those bytes, and the
// uncooked `@nh,128,32 & 0xffffff00 == 0xa010000` rendering that goes with
// it.
func TestLowerTermInetAddrMatchGuardsOnNFProto(t *testing.T) {
	tests := []struct {
		name       string
		match      firewall.Match
		wantProto  byte
		wantOffset uint32
		wantLen    uint32
	}{
		{
			name:       "ipv4 source",
			match:      firewall.MatchSourceAddress{Prefix: netip.MustParsePrefix("10.1.0.0/24")},
			wantProto:  unix.NFPROTO_IPV4,
			wantOffset: 12,
			wantLen:    4,
		},
		{
			name:       "ipv4 destination",
			match:      firewall.MatchDestinationAddress{Prefix: netip.MustParsePrefix("10.1.0.0/24")},
			wantProto:  unix.NFPROTO_IPV4,
			wantOffset: 16,
			wantLen:    4,
		},
		{
			name:       "ipv6 source",
			match:      firewall.MatchSourceAddress{Prefix: netip.MustParsePrefix("2001:db8::/32")},
			wantProto:  unix.NFPROTO_IPV6,
			wantOffset: 8,
			wantLen:    16,
		},
		{
			name:       "ipv6 destination",
			match:      firewall.MatchDestinationAddress{Prefix: netip.MustParsePrefix("2001:db8::/32")},
			wantProto:  unix.NFPROTO_IPV6,
			wantOffset: 24,
			wantLen:    16,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := &firewall.Term{
				Name:    "t",
				Matches: []firewall.Match{tt.match},
				Actions: []firewall.Action{firewall.Drop{}},
			}
			exprs := lowerTermOneRule(t, inetCtx(nil), term)
			rest := nfprotoGuardRest(t, exprs, tt.wantProto)
			payload, ok := rest[0].(*expr.Payload)
			if !ok {
				t.Fatalf("expression after the guard = %T, want *expr.Payload", rest[0])
			}
			if payload.Base != expr.PayloadBaseNetworkHeader {
				t.Errorf("payload base = %v, want PayloadBaseNetworkHeader", payload.Base)
			}
			if payload.Offset != tt.wantOffset {
				t.Errorf("payload offset = %d, want %d", payload.Offset, tt.wantOffset)
			}
			if payload.Len != tt.wantLen {
				t.Errorf("payload len = %d, want %d", payload.Len, tt.wantLen)
			}
		})
	}
}

// VALIDATES: an ip or ip6 table emits NO nfproto guard, so a rule in a
// single-family table keeps the shape it had. nfproto is constant for the
// whole ruleset there, and every ze producer of such a table splits its
// prefixes by address family at the source (buildTables in
// internal/plugins/anomaly/shape/match.go, familyFromPrefix in
// internal/plugins/ddos/local/responder.go).
// PREVENTS: charging the DDoS and anomaly mitigation tables, the highest
// rule counts ze programs, two extra per-packet expressions per rule.
func TestLowerTermSingleFamilyTableOmitsNFProtoGuard(t *testing.T) {
	tests := []struct {
		name   string
		family nftables.TableFamily
		prefix string
	}{
		{"ip table, ipv4 prefix", nftables.TableFamilyIPv4, "10.1.0.0/24"},
		{"ip6 table, ipv6 prefix", nftables.TableFamilyIPv6, "2001:db8::/32"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &lowerCtx{table: &nftables.Table{Name: "t", Family: tt.family}}
			term := &firewall.Term{
				Name:    "t",
				Matches: []firewall.Match{firewall.MatchSourceAddress{Prefix: netip.MustParsePrefix(tt.prefix)}},
			}
			exprs := lowerTermOneRule(t, ctx, term)
			if len(exprs) != 3 {
				t.Fatalf("expected Payload+Bitwise+Cmp only, got %d expressions", len(exprs))
			}
			if _, ok := exprs[0].(*expr.Payload); !ok {
				t.Fatalf("expr[0] = %T, want *expr.Payload with no guard in front", exprs[0])
			}
		})
	}
}

// VALIDATES: a named-set address match carries the same guard as a literal
// prefix match. matchInSetPayloadLayout reads the same raw network-header
// offsets, so it owes the same guard.
// PREVENTS: `from { source-address "@blocked"; }` matching IPv6 traffic in
// an inet table while the literal form is guarded.
func TestLowerMatchInSetInetAddrGuardsOnNFProto(t *testing.T) {
	tests := []struct {
		name      string
		keyType   nftables.SetDatatype
		field     firewall.SetFieldType
		wantProto byte
	}{
		{"ipv4 source set", nftables.TypeIPAddr, firewall.SetFieldSourceAddr, unix.NFPROTO_IPV4},
		{"ipv4 dest set", nftables.TypeIPAddr, firewall.SetFieldDestAddr, unix.NFPROTO_IPV4},
		{"ipv6 source set", nftables.TypeIP6Addr, firewall.SetFieldSourceAddr, unix.NFPROTO_IPV6},
		{"ipv6 dest set", nftables.TypeIP6Addr, firewall.SetFieldDestAddr, unix.NFPROTO_IPV6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := &nftables.Set{Name: "s", ID: 7, KeyType: tt.keyType}
			ctx := inetCtx(map[string]*nftables.Set{"s": set})
			term := &firewall.Term{
				Name:    "t",
				Matches: []firewall.Match{firewall.MatchInSet{SetName: "s", MatchField: tt.field}},
			}
			exprs := lowerTermOneRule(t, ctx, term)
			rest := nfprotoGuardRest(t, exprs, tt.wantProto)
			if _, ok := rest[0].(*expr.Payload); !ok {
				t.Fatalf("expression after the guard = %T, want *expr.Payload", rest[0])
			}
			if _, ok := rest[1].(*expr.Lookup); !ok {
				t.Fatalf("expression after the payload = %T, want *expr.Lookup", rest[1])
			}
		})
	}
}

// VALIDATES: a port set match emits no nfproto guard. A transport-header
// port sits at the same offset whichever network header precedes it, so
// the read is valid for both families and a guard would only narrow it.
// PREVENTS: an inet port rule silently losing its IPv6 half.
func TestLowerMatchInSetPortNeedsNoNFProtoGuard(t *testing.T) {
	set := &nftables.Set{Name: "ports", ID: 8, KeyType: nftables.TypeInetService}
	ctx := inetCtx(map[string]*nftables.Set{"ports": set})
	exprs, err := lowerMatch(ctx, firewall.MatchInSet{
		SetName:    "ports",
		MatchField: firewall.SetFieldDestPort,
	})
	if err != nil {
		t.Fatalf("lowerMatch: %v", err)
	}
	if len(exprs) != 2 {
		t.Fatalf("expected Payload+Lookup only, got %d expressions", len(exprs))
	}
	payload, ok := exprs[0].(*expr.Payload)
	if !ok {
		t.Fatalf("expr[0] = %T, want *expr.Payload with no guard in front", exprs[0])
	}
	if payload.Base != expr.PayloadBaseTransportHeader {
		t.Errorf("payload base = %v, want PayloadBaseTransportHeader", payload.Base)
	}
}

// VALIDATES: a DSCP match in an inet table is guarded to IPv4. The
// lowering reads the IPv4 TOS byte at network-header offset 1, and
// validateMatch already declares the match IPv4-only.
// PREVENTS: `dscp 46` matching an IPv6 packet on offset 1, which holds the
// traffic-class low nibble and the flow-label high nibble.
func TestLowerDSCPMatchInetGuardsOnNFProto(t *testing.T) {
	exprs, err := lowerMatch(inetCtx(nil), firewall.MatchDSCP{Value: 46})
	if err != nil {
		t.Fatalf("lowerMatch: %v", err)
	}
	rest := nfprotoGuardRest(t, exprs, unix.NFPROTO_IPV4)
	payload, ok := rest[0].(*expr.Payload)
	if !ok {
		t.Fatalf("expression after the guard = %T, want *expr.Payload", rest[0])
	}
	if payload.Base != expr.PayloadBaseNetworkHeader {
		t.Errorf("payload base = %v, want PayloadBaseNetworkHeader", payload.Base)
	}
	if payload.Offset != 1 {
		t.Errorf("payload offset = %d, want 1 (IPv4 TOS byte)", payload.Offset)
	}
}

// VALIDATES: a lowering context that carries no table answers inet, so an
// address match through it is guarded rather than left as a bare raw read.
// PREVENTS: a future call site that forgets the table silently reopening
// the unguarded path, which is the shape of the original defect.
func TestLowerCtxTableFamilyFailsClosedToInet(t *testing.T) {
	var absent *lowerCtx
	if got := absent.tableFamily(); got != nftables.TableFamilyINet {
		t.Errorf("nil ctx family = %v, want TableFamilyINet", got)
	}
	if got := (&lowerCtx{}).tableFamily(); got != nftables.TableFamilyINet {
		t.Errorf("ctx with no table family = %v, want TableFamilyINet", got)
	}
	if got := (&lowerCtx{table: &nftables.Table{Family: nftables.TableFamilyIPv6}}).tableFamily(); got != nftables.TableFamilyIPv6 {
		t.Errorf("ip6 table family = %v, want TableFamilyIPv6", got)
	}
}

// --- family split on a network-header write ---

// VALIDATES: `then { dscp-set 46; accept; }` in an inet table lowers to TWO
// rules, one guarded `meta nfproto ipv4` carrying the IPv4 TOS-byte write with
// its header-checksum fixup, one guarded `meta nfproto ipv6` carrying the IPv6
// traffic-class write with NO checksum, and each rule repeating the term's own
// matches and its accept verdict. Driven through lowerTerm, the entry point
// applyChain calls.
// PREVENTS: the IPv4 write reaching an IPv6 packet, where offset 1 holds the
// traffic-class low nibble and the flow-label high nibble and the checksum
// fixup at offset 10 lands inside the IPv6 source address. Measured in a
// network namespace on 2026-08-19: with the IPv4 write applied to IPv6, all 6
// packets of an ICMPv6 exchange arrived with DSCP 2 rather than the 46 asked
// for, ECN forced to CE, and a polluted flow label.
// PREVENTS ALSO: the single-rule repair, a `meta nfproto ipv4` guard inline in
// one rule, which gates the accept as well as the write and so drops IPv6
// traffic the term was written to accept.
func TestLowerTermInetDSCPSetSplitsPerFamily(t *testing.T) {
	term := &firewall.Term{
		Name:    "voip",
		Matches: []firewall.Match{firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 5060, Hi: 5060}}}},
		Actions: []firewall.Action{firewall.SetDSCP{Value: 46}, firewall.Accept{}},
	}

	rules, err := lowerTerm(inetCtx(nil), term)
	if err != nil {
		t.Fatalf("lowerTerm: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("lowerTerm produced %d rules, want 2 (one per address family)", len(rules))
	}

	tests := []struct {
		name       string
		rule       []expr.Any
		wantProto  byte
		wantOffset uint32
		wantLen    uint32
		wantMask   []byte
		wantXor    []byte
		wantCsum   expr.PayloadCsumType
		wantCsumAt uint32
	}{
		{
			name:       "ipv4 rule writes the TOS byte and fixes the header checksum",
			rule:       rules[0],
			wantProto:  unix.NFPROTO_IPV4,
			wantOffset: 1,
			wantLen:    1,
			wantMask:   []byte{0x03},
			wantXor:    []byte{46 << 2},
			wantCsum:   expr.CsumTypeInet,
			wantCsumAt: 10,
		},
		{
			// nft v1.0.9 compiles `ip6 dscp set 46` to exactly this:
			// payload load 2b @ nh+0, bitwise & 0x3ff0 ^ 0x800b (little
			// endian: mask f0 3f, xor 0b 80), payload write 2b @ nh+0
			// csum_type 0.
			name:       "ipv6 rule writes the traffic class and fixes no checksum",
			rule:       rules[1],
			wantProto:  unix.NFPROTO_IPV6,
			wantOffset: 0,
			wantLen:    2,
			wantMask:   []byte{0xF0, 0x3F},
			wantXor:    []byte{46 >> 2, (46 & 0x03) << 6},
			wantCsum:   expr.CsumTypeNone,
			wantCsumAt: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rest := nfprotoGuardRest(t, tt.rule, tt.wantProto)

			// The term's own match is repeated in both rules, so neither
			// family loses the destination port the operator wrote.
			port, ok := rest[0].(*expr.Payload)
			if !ok {
				t.Fatalf("expression after the guard = %T, want the term's port match", rest[0])
			}
			if port.Base != expr.PayloadBaseTransportHeader || port.Offset != 2 {
				t.Errorf("port match = {Base:%v Offset:%d}, want {Transport 2}", port.Base, port.Offset)
			}

			read, ok := rest[2].(*expr.Payload)
			if !ok {
				t.Fatalf("dscp read = %T, want *expr.Payload", rest[2])
			}
			if read.OperationType != expr.PayloadLoad {
				t.Errorf("dscp read op = %v, want PayloadLoad", read.OperationType)
			}
			if read.Base != expr.PayloadBaseNetworkHeader {
				t.Errorf("dscp read base = %v, want PayloadBaseNetworkHeader", read.Base)
			}
			if read.Offset != tt.wantOffset || read.Len != tt.wantLen {
				t.Errorf("dscp read offset/len = %d/%d, want %d/%d",
					read.Offset, read.Len, tt.wantOffset, tt.wantLen)
			}

			bw, ok := rest[3].(*expr.Bitwise)
			if !ok {
				t.Fatalf("dscp rewrite = %T, want *expr.Bitwise", rest[3])
			}
			if !bytes.Equal(bw.Mask, tt.wantMask) {
				t.Errorf("dscp mask = %#v, want %#v", bw.Mask, tt.wantMask)
			}
			if !bytes.Equal(bw.Xor, tt.wantXor) {
				t.Errorf("dscp xor = %#v, want %#v", bw.Xor, tt.wantXor)
			}

			write, ok := rest[4].(*expr.Payload)
			if !ok {
				t.Fatalf("dscp write = %T, want *expr.Payload", rest[4])
			}
			if write.OperationType != expr.PayloadWrite {
				t.Errorf("dscp write op = %v, want PayloadWrite", write.OperationType)
			}
			if write.Offset != tt.wantOffset || write.Len != tt.wantLen {
				t.Errorf("dscp write offset/len = %d/%d, want %d/%d",
					write.Offset, write.Len, tt.wantOffset, tt.wantLen)
			}
			if write.CsumType != tt.wantCsum || write.CsumOffset != tt.wantCsumAt {
				t.Errorf("dscp write checksum = %v/%d, want %v/%d",
					write.CsumType, write.CsumOffset, tt.wantCsum, tt.wantCsumAt)
			}

			// The verdict survives the split, so neither family is quietly
			// dropped from the term the operator wrote.
			verdict, ok := rest[5].(*expr.Verdict)
			if !ok {
				t.Fatalf("last expression = %T, want *expr.Verdict", rest[5])
			}
			if verdict.Kind != expr.VerdictAccept {
				t.Errorf("verdict = %v, want VerdictAccept", verdict.Kind)
			}
		})
	}
}

// VALIDATES: a term with no network-header write stays ONE rule, in an inet
// table as in an ip one, and a dscp-set in a single-family table stays one rule
// carrying that family's write.
// PREVENTS: charging every rule in every inet table a second copy of itself,
// and splitting a term whose actions have no address family to split on.
func TestLowerTermSplitsOnlyWhereAFamilyIsWritten(t *testing.T) {
	tests := []struct {
		name       string
		family     nftables.TableFamily
		actions    []firewall.Action
		wantOffset uint32
		wantLen    uint32
	}{
		{
			name:    "inet table, no network-header write",
			family:  nftables.TableFamilyINet,
			actions: []firewall.Action{firewall.Accept{}},
		},
		{
			name:       "ip table, dscp-set writes the IPv4 TOS byte",
			family:     nftables.TableFamilyIPv4,
			actions:    []firewall.Action{firewall.SetDSCP{Value: 46}},
			wantOffset: 1,
			wantLen:    1,
		},
		{
			name:       "ip6 table, dscp-set writes the IPv6 traffic class",
			family:     nftables.TableFamilyIPv6,
			actions:    []firewall.Action{firewall.SetDSCP{Value: 46}},
			wantOffset: 0,
			wantLen:    2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &lowerCtx{table: &nftables.Table{Name: "t", Family: tt.family}}
			term := &firewall.Term{Name: "t", Actions: tt.actions}

			exprs := lowerTermOneRule(t, ctx, term)
			if _, ok := exprs[0].(*expr.Meta); ok {
				t.Fatalf("rule opens with a meta guard; a single-rule term owes none")
			}
			if tt.wantLen == 0 {
				return
			}
			write, ok := exprs[2].(*expr.Payload)
			if !ok {
				t.Fatalf("dscp write = %T, want *expr.Payload", exprs[2])
			}
			if write.Offset != tt.wantOffset || write.Len != tt.wantLen {
				t.Errorf("dscp write offset/len = %d/%d, want %d/%d",
					write.Offset, write.Len, tt.wantOffset, tt.wantLen)
			}
		})
	}
}

// VALIDATES: a family-neutral rule refuses to write the DSCP rather than
// guessing a header layout. arp, bridge and netdev tables carry no IP header,
// so tableNFProto answers NFPROTO_UNSPEC for them.
// PREVENTS: the guard failing open, which is what a silent default to IPv4
// would be: a write at the wrong offset in a header nobody checked.
func TestLowerSetDSCPRejectsWithoutAFamily(t *testing.T) {
	if _, err := lowerSetDSCP(unix.NFPROTO_UNSPEC, 46); err == nil {
		t.Fatal("lowerSetDSCP with no address family must reject")
	}
	for _, family := range []nftables.TableFamily{
		nftables.TableFamilyARP,
		nftables.TableFamilyBridge,
		nftables.TableFamilyNetdev,
	} {
		if got := tableNFProto(family); got != unix.NFPROTO_UNSPEC {
			t.Errorf("tableNFProto(%v) = %d, want NFPROTO_UNSPEC", family, got)
		}
	}
}

// --- l4proto dependency on an ICMP type match ---

// VALIDATES: `icmp-type` and `icmpv6-type` each lower behind their own
// `meta l4proto` dependency, so the type byte at transport-header offset 0 is
// read only for the protocol whose numbering the operator wrote. Driven
// through lowerTerm, the entry point applyChain calls.
// PREVENTS: `icmp-type echo-request` (ICMPv4 type 8) also matching an ICMPv6
// packet whose type byte is 8, which is `unassigned` in ICMPv6, and
// `icmpv6-type echo-request` (128) matching an ICMPv4 packet.
func TestLowerTermICMPTypeCarriesL4ProtoDependency(t *testing.T) {
	tests := []struct {
		name      string
		match     firewall.Match
		wantProto byte
		wantType  byte
	}{
		{"icmpv4 echo-request", firewall.MatchICMPType{Type: 8}, unix.IPPROTO_ICMP, 8},
		{"icmpv6 echo-request", firewall.MatchICMPv6Type{Type: 128}, unix.IPPROTO_ICMPV6, 128},
		{"icmpv4 type 128 is not icmpv6", firewall.MatchICMPType{Type: 128}, unix.IPPROTO_ICMP, 128},
		{"icmpv6 type 8 is not icmpv4", firewall.MatchICMPv6Type{Type: 8}, unix.IPPROTO_ICMPV6, 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := &firewall.Term{
				Name:    "t",
				Matches: []firewall.Match{tt.match},
				Actions: []firewall.Action{firewall.Accept{}},
			}
			exprs := lowerTermOneRule(t, inetCtx(nil), term)
			rest := l4protoDepRest(t, exprs, tt.wantProto)

			payload, ok := rest[0].(*expr.Payload)
			if !ok {
				t.Fatalf("expression after the dependency = %T, want *expr.Payload", rest[0])
			}
			if payload.Base != expr.PayloadBaseTransportHeader {
				t.Errorf("payload base = %v, want PayloadBaseTransportHeader", payload.Base)
			}
			if payload.Offset != 0 || payload.Len != 1 {
				t.Errorf("payload offset/len = %d/%d, want 0/1", payload.Offset, payload.Len)
			}
			cmp, ok := rest[1].(*expr.Cmp)
			if !ok {
				t.Fatalf("expression after the payload = %T, want *expr.Cmp", rest[1])
			}
			if len(cmp.Data) != 1 || cmp.Data[0] != tt.wantType {
				t.Errorf("type compare = %v, want [%d]", cmp.Data, tt.wantType)
			}
		})
	}
}

// VALIDATES: the rules of one term are summed into a single TermCounter, in
// rule order, and a rule ze did not program keeps a row of its own.
// PREVENTS: a term split across two rules reporting one family's packets as
// its total. Every caller keys the result by term name
// (handleShowFirewallRuleset, the web firewall page), so a second row for the
// same term replaces the first instead of adding to it.
func TestMergeRuleCountersSumsTheRulesOfOneTerm(t *testing.T) {
	rule := func(name string, packets, octets uint64) *nftables.Rule {
		return &nftables.Rule{
			UserData: []byte(name),
			Exprs:    []expr.Any{&expr.Counter{Packets: packets, Bytes: octets}},
		}
	}

	got := mergeRuleCounters([]*nftables.Rule{
		rule("voip", 3, 300), // the term's IPv4 rule
		rule("voip", 4, 400), // the term's IPv6 rule
		rule("", 9, 900),     // programmed outside ze
		rule("web", 5, 500),  //
		rule("", 1, 100),     // programmed outside ze
	})

	want := []firewall.TermCounter{
		{Name: "voip", Packets: 7, Bytes: 700},
		{Name: "", Packets: 9, Bytes: 900},
		{Name: "web", Packets: 5, Bytes: 500},
		{Name: "", Packets: 1, Bytes: 100},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d counters, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("counter %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
