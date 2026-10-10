//go:build linux

package ifacenetlink

import (
	"errors"
	"slices"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
)

// TestIfaceKernelCapabilitiesAreEnrolled proves the backend declares WireGuard
// and the xfrm interface with their build symbols, and that the package init()
// enrolled both. Method: read the declaration and the live enrolment.
func TestIfaceKernelCapabilitiesAreEnrolled(t *testing.T) {
	want := map[string]string{"wireguard": "CONFIG_WIREGUARD", "xfrm-interface": "CONFIG_XFRM_INTERFACE"}
	for _, capability := range ifaceKernelCapabilities() {
		kernel, ok := want[capability.Subsystem]
		if !ok {
			t.Errorf("unexpected capability %s", capability.Subsystem)
			continue
		}
		if capability.Kernel != kernel {
			t.Errorf("%s enrolls %s, want %s", capability.Subsystem, capability.Kernel, kernel)
		}
	}
	enrolled := kernelcap.Enrolled()
	for subsystem := range want {
		if !slices.Contains(enrolled, subsystem) {
			t.Errorf("%s is not enrolled by the package init()", subsystem)
		}
	}
}

// TestInterfaceKindInUse checks the predicate: an interface of the kind under
// the netlink backend uses the kernel, under vpp it does not. Method: one tree
// per shape.
func TestInterfaceKindInUse(t *testing.T) {
	inUse := interfaceKindInUse("wireguard")
	if inUse(nil) || inUse(config.NewTree()) {
		t.Fatal("no interface block uses no kernel kind")
	}
	defaulted := config.NewTree()
	defaulted.GetOrCreateContainer("interface").AddListEntry("wireguard", "wg0", config.NewTree())
	if !inUse(defaulted) {
		t.Fatal("a wireguard interface under the default backend uses the kernel")
	}
	if interfaceKindInUse("xfrm")(defaulted) {
		t.Fatal("a wireguard interface does not use the xfrm kind")
	}
	vpp := config.NewTree()
	block := vpp.GetOrCreateContainer("interface")
	block.Set("backend", "vpp")
	block.AddListEntry("wireguard", "wg0", config.NewTree())
	if inUse(vpp) {
		t.Fatal("the vpp backend creates its interfaces outside the kernel")
	}
}

// TestXFRMInterfaceProbeClassification checks every branch of the xfrm link
// verdict. Method: feed each rtnetlink answer through the classifier.
func TestXFRMInterfaceProbeClassification(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want kernelcap.State
	}{
		{name: "older kernel created it in the throwaway namespace", err: nil, want: kernelcap.StatePresent},
		{name: "if_id must be non zero", err: unix.EINVAL, want: kernelcap.StatePresent},
		{name: "unknown device type", err: unix.EOPNOTSUPP, want: kernelcap.StateAbsent},
		{name: "no CAP_NET_ADMIN", err: unix.EPERM, want: kernelcap.StateUnknown},
		{name: "wrapped unknown device type", err: errors.Join(errors.New("rtnl"), unix.EOPNOTSUPP), want: kernelcap.StateAbsent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyXFRMInterfaceProbe(tc.err); got.State != tc.want {
				t.Fatalf("classifyXFRMInterfaceProbe(%v) = %s, want %s", tc.err, got.State, tc.want)
			}
		})
	}
}

// TestXFRMInterfaceProbeOnThisHost runs the real probe and a negative control
// in throwaway namespaces. Method: a link kind no kernel registers must read
// absent, proving EOPNOTSUPP is what rtnetlink really answers; the xfrm answer
// is logged, because this host's kernel is not under test. Without
// CAP_SYS_ADMIN both read unknown and the control is skipped with the reason.
func TestXFRMInterfaceProbeOnThisHost(t *testing.T) {
	got := xfrmInterfaceProbe()
	t.Logf("xfrm interface: %s %v", got.State, got.Reason)

	saved := xfrmInterfaceProbeAdd
	t.Cleanup(func() { xfrmInterfaceProbeAdd = saved })
	xfrmInterfaceProbeAdd = func(handle *netlink.Handle) error {
		return handle.LinkAdd(&netlink.GenericLink{LinkAttrs: netlink.LinkAttrs{Name: xfrmProbeLinkName}, LinkType: "zenosuchkind"})
	}
	control := xfrmInterfaceProbe()
	if control.State == kernelcap.StateUnknown {
		t.Skipf("the probe namespace is not available here: %v", control.Reason)
	}
	if control.State != kernelcap.StateAbsent {
		t.Fatalf("an unregistered link kind read %s (%v), want absent", control.State, control.Reason)
	}
}
