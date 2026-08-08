// VALIDATES: option=netns-link parsing -- name= is required, address= is an
// optional CIDR parsed into a netip.Prefix, a malformed address is rejected at
// parse time on every platform, and a name-only spec leaves a zero (invalid)
// address meaning "link, no address".
// PREVENTS: the test/policy 005-next-hop regression where a policy next-hop had
// no connected route to resolve its gateway. The fix provisions an interface in
// the per-test netns from this option; a parse that silently dropped the option
// or accepted a bad address would leave the interface unprovisioned and the
// route add failing "network is unreachable" again, on Linux only.

package runner

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"
)

func parseNetnsLinkLine(t *testing.T, line string) (*Record, error) {
	t.Helper()
	et := &EncodingTests{}
	r := newRecord("netns-link-test")
	return r, et.parseLine(r, "test/policy/fake.ci", line)
}

func TestNetnsLinkParsesNameAndAddress(t *testing.T) {
	r, err := parseNetnsLinkLine(t, "option=netns-link:name=eth1:address=10.0.0.2/24")
	if err != nil {
		t.Fatalf("parseLine: %v", err)
	}
	if len(r.NetnsLinks) != 1 {
		t.Fatalf("NetnsLinks = %d, want 1", len(r.NetnsLinks))
	}
	got := r.NetnsLinks[0]
	if got.Name != "eth1" {
		t.Errorf("Name = %q, want eth1", got.Name)
	}
	want := netip.MustParsePrefix("10.0.0.2/24")
	if got.Address != want {
		t.Errorf("Address = %v, want %v", got.Address, want)
	}
}

func TestNetnsLinkNameOnlyLeavesAddressInvalid(t *testing.T) {
	r, err := parseNetnsLinkLine(t, "option=netns-link:name=eth1")
	if err != nil {
		t.Fatalf("parseLine: %v", err)
	}
	if len(r.NetnsLinks) != 1 {
		t.Fatalf("NetnsLinks = %d, want 1", len(r.NetnsLinks))
	}
	if r.NetnsLinks[0].Address.IsValid() {
		t.Errorf("Address = %v, want invalid (name-only means no address)", r.NetnsLinks[0].Address)
	}
}

func TestNetnsLinkMissingNameRejected(t *testing.T) {
	if _, err := parseNetnsLinkLine(t, "option=netns-link:address=10.0.0.2/24"); err == nil {
		t.Fatal("a netns-link without name= was accepted; the runner would have nothing to create")
	}
}

func TestNetnsLinkBadAddressRejected(t *testing.T) {
	if _, err := parseNetnsLinkLine(t, "option=netns-link:name=eth1:address=not-a-cidr"); err == nil {
		t.Fatal("a malformed address was accepted; it would fail silently at run time on Linux only")
	}
}

// VALIDATES: peer= and vlan= reach the spec, and a name-only spec leaves both at
// their zero values (dummy, untagged).
// PREVENTS: a silently dropped peer=, which would provision a DUMMY where the
// test asked for a veth pair. A dummy discards what is written to it, so the
// PPPoE client's PADI would never reach the daemon and the test would fail on a
// discovery timeout that names neither the option nor the link type.
func TestNetnsLinkParsesPeerAndVLAN(t *testing.T) {
	r, err := parseNetnsLinkLine(t, "option=netns-link:name=veth-bng:peer=veth-sub:vlan=100")
	if err != nil {
		t.Fatalf("parseLine: %v", err)
	}
	if len(r.NetnsLinks) != 1 {
		t.Fatalf("NetnsLinks = %d, want 1", len(r.NetnsLinks))
	}
	got := r.NetnsLinks[0]
	if got.Name != "veth-bng" || got.Peer != "veth-sub" || got.VLAN != 100 {
		t.Errorf("spec = {Name:%q Peer:%q VLAN:%d}, want {veth-bng veth-sub 100}", got.Name, got.Peer, got.VLAN)
	}
}

func TestNetnsLinkWithoutPeerIsDummy(t *testing.T) {
	r, err := parseNetnsLinkLine(t, "option=netns-link:name=eth1")
	if err != nil {
		t.Fatalf("parseLine: %v", err)
	}
	if r.NetnsLinks[0].Peer != "" {
		t.Errorf("Peer = %q, want empty (no peer means a dummy link)", r.NetnsLinks[0].Peer)
	}
	if r.NetnsLinks[0].VLAN != 0 {
		t.Errorf("VLAN = %d, want 0 (no vlan= means an untagged link)", r.NetnsLinks[0].VLAN)
	}
}

// VALIDATES: a vlan tag outside 1..4094 and a peer equal to the name are both
// rejected at parse time, on every platform.
// PREVENTS: the fail-open shape this option already had once -- a value netlink
// rejects at run time only, on Linux, inside a QEMU VM, where the diagnosis is a
// suite-wide setup error rather than the one .ci line that caused it.
func TestNetnsLinkRejectsBadPeerAndVLAN(t *testing.T) {
	for _, line := range []string{
		"option=netns-link:name=veth-bng:peer=veth-bng",
		"option=netns-link:name=veth-bng:peer=veth-sub:vlan=0",
		"option=netns-link:name=veth-bng:peer=veth-sub:vlan=4095",
		"option=netns-link:name=veth-bng:peer=veth-sub:vlan=abc",
	} {
		if _, err := parseNetnsLinkLine(t, line); err == nil {
			t.Errorf("accepted %q; it cannot produce the topology the test asked for", line)
		}
	}
}

// parseNetnsLinkFile drives the REAL entry point (parseAndAdd, which applies the
// post-parse gates) over a .ci file declaring a netns-link, optionally preceded
// by the needs-linux marker the eleven ospf/ospfv3 tests and
// test/policy/005-next-hop carry.
//
// withNeedsLinux is a parameter, not a constant, because needs-linux skips on
// any non-Linux host: baking it in would make the "runs under netns mode"
// polarity untestable on the macOS dev box where these tests are authored.
func parseNetnsLinkFile(t *testing.T, netnsOn, withNeedsLinux bool) *Record {
	t.Helper()
	ResetNickCounter()
	prev := netnsActive
	netnsActive = func() bool { return netnsOn }
	t.Cleanup(func() { netnsActive = prev })

	dir := t.TempDir()
	ciFile := filepath.Join(dir, "netns-gate.ci")
	content := "option=netns-link:name=eth1:address=10.0.0.1/24\n" +
		"expect=exit:code=0\n"
	if withNeedsLinux {
		content = "option=needs-linux:caps=net-admin\n" + content
	}
	if err := os.WriteFile(ciFile, []byte(content), 0o600); err != nil {
		t.Fatalf("write .ci: %v", err)
	}
	rec, err := NewEncodingTests(dir).parseAndAdd(ciFile)
	if err != nil {
		t.Fatalf("parseAndAdd: %v", err)
	}
	return rec
}

// VALIDATES: a test declaring option=netns-link is SKIPPED, with a reason naming
// the netns targets, when the per-test netns launch mode is off.
// PREVENTS: the 2026-07-25 QEMU wipeout -- `make ze-qemu-needs-linux-test` does
// not set ZE_TEST_NETNS, so provisionNetnsLinks never ran, the daemon was asked
// to open interfaces (eth1/nbma0/ptmp0) that did not exist, the OSPF engine
// exited 1, plugin startup timed out, and all 8 ospf + 3 ospfv3 + policy
// 005-next-hop tests failed with an unrelated-looking observer TLS timeout.
func TestNetnsLinkSkipsWithoutNetnsMode(t *testing.T) {
	// withNeedsLinux=true mirrors the twelve real files, and proves the netns
	// reason REPLACES the weaker needs-linux one: the targets it names are the
	// only ones that can run these tests.
	rec := parseNetnsLinkFile(t, false, true)
	if rec.SkipReason != skipReasonNetnsLink {
		t.Fatalf("SkipReason = %q, want %q -- without netns mode the links are never provisioned, so the test cannot pass and must not run",
			rec.SkipReason, skipReasonNetnsLink)
	}
}

// VALIDATES: the same test RUNS (no skip reason) when netns mode is active.
// PREVENTS: the gate silencing the eleven ospf/ospfv3 tests in the one mode that
// does provision their links (make ze-netns-test / ze-netns-qemu-test), which
// would turn a precise gate into a permanent coverage hole.
func TestNetnsLinkRunsUnderNetnsMode(t *testing.T) {
	// withNeedsLinux=false: needs-linux skips on every non-Linux host, which would
	// mask this polarity on the macOS box these tests are authored on. Netns mode
	// implies Linux anyway, so the marker is redundant here.
	rec := parseNetnsLinkFile(t, true, false)
	if rec.SkipReason != "" {
		t.Fatalf("SkipReason = %q, want empty -- under netns mode the links ARE provisioned and the test must run", rec.SkipReason)
	}
	if len(rec.NetnsLinks) != 1 {
		t.Fatalf("NetnsLinks = %d, want 1", len(rec.NetnsLinks))
	}
}

// VALIDATES: the gate covers the REAL .ci files, not just a synthetic fixture --
// every test under test/ospf, test/ospfv3 and test/policy that declares
// option=netns-link is skipped off netns mode, and runs under it.
// PREVENTS: the gate being asserted only against a hand-written fixture while a
// real suite keeps the fail-open behavior (an author adding an OSPF interface
// test that declares its link and then fails everywhere but ze-netns-test). It
// is host-independent: netnsActive is driven through the seam, never probed.
func TestNetnsLinkGateCoversRealSuites(t *testing.T) {
	root := repoRootForTest(t)
	dirs := []string{"ospf", "ospfv3", "policy", "pppoe"}

	for _, netnsOn := range []bool{false, true} {
		prev := netnsActive
		netnsActive = func() bool { return netnsOn }

		found := 0
		for _, d := range dirs {
			ResetNickCounter()
			dir := filepath.Join(root, "test", d)
			et := NewEncodingTests(dir)
			if err := et.Discover(dir); err != nil {
				netnsActive = prev
				t.Fatalf("discover %s: %v", dir, err)
			}
			for _, rec := range et.Registered() {
				if len(rec.NetnsLinks) == 0 {
					continue
				}
				found++
				switch {
				case !netnsOn && rec.SkipReason != skipReasonNetnsLink:
					t.Errorf("%s/%s: SkipReason = %q, want %q -- off netns mode its link is never provisioned",
						d, rec.Name, rec.SkipReason, skipReasonNetnsLink)
				case netnsOn && rec.SkipReason == skipReasonNetnsLink:
					t.Errorf("%s/%s: skipped under netns mode, where the link IS provisioned", d, rec.Name)
				}
			}
		}
		netnsActive = prev

		// The eight test/ospf, three test/ospfv3, one test/policy and three
		// test/pppoe files that declare a link today. A drop to zero would make the
		// loop above vacuous.
		if found < 15 {
			t.Fatalf("found %d netns-link tests across %v, want at least 15; the assertions above ran on nothing", found, dirs)
		}
	}
}
