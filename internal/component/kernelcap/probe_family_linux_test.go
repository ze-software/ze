//go:build linux

package kernelcap

import (
	"errors"
	"testing"

	"golang.org/x/sys/unix"
)

// TestGenericNetlinkFamilyClassification checks every branch of the family
// verdict. Method: feed each controller answer through the classifier.
func TestGenericNetlinkFamilyClassification(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want State
	}{
		{name: "registered", err: nil, want: StatePresent},
		{name: "no such family", err: unix.ENOENT, want: StateAbsent},
		{name: "wrapped no such family", err: errors.Join(errors.New("genl"), unix.ENOENT), want: StateAbsent},
		{name: "refused", err: unix.EPERM, want: StateUnknown},
		{name: "socket failure", err: unix.EMFILE, want: StateUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyGenericNetlinkFamily(tc.err)
			if got.State != tc.want {
				t.Fatalf("classifyGenericNetlinkFamily(%v) = %s, want %s", tc.err, got.State, tc.want)
			}
			if got.State != StatePresent && got.Reason == nil {
				t.Fatalf("a %s verdict must carry its reason", got.State)
			}
		})
	}
}

// TestPPPoXProtocolClassification checks every branch of the socket verdict.
// Method: feed each socket answer through the classifier.
func TestPPPoXProtocolClassification(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want State
	}{
		{name: "opened", err: nil, want: StatePresent},
		{name: "no AF_PPPOX", err: unix.EAFNOSUPPORT, want: StateAbsent},
		{name: "no protocol handler", err: unix.EPROTONOSUPPORT, want: StateAbsent},
		{name: "seccomp", err: unix.EPERM, want: StateUnknown},
		{name: "out of range protocol", err: unix.EPROTOTYPE, want: StateUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyPPPoXProtocol(tc.err)
			if got.State != tc.want {
				t.Fatalf("classifyPPPoXProtocol(%v) = %s, want %s", tc.err, got.State, tc.want)
			}
		})
	}
}

// TestFamilyProbesUseTheirSeams proves the exported probes ask the kernel the
// question they name. Method: replace each seam, record the argument, restore.
func TestFamilyProbesUseTheirSeams(t *testing.T) {
	savedLookup, savedOpen := genlFamilyLookup, pppoxSocketOpen
	t.Cleanup(func() { genlFamilyLookup, pppoxSocketOpen = savedLookup, savedOpen })

	var family string
	genlFamilyLookup = func(name string) error { family = name; return unix.ENOENT }
	if got := GenericNetlinkFamily("wireguard"); got.State != StateAbsent || family != "wireguard" {
		t.Fatalf("GenericNetlinkFamily asked %q and answered %s", family, got.State)
	}

	protocol := -1
	pppoxSocketOpen = func(p int) error { protocol = p; return nil }
	if got := PPPoXProtocol(1); got.State != StatePresent || protocol != 1 {
		t.Fatalf("PPPoXProtocol opened protocol %d and answered %s", protocol, got.State)
	}
}

// TestFamilyProbesOnThisHost runs the real probes. Method: a family no kernel
// registers must read absent, which proves the ENOENT path is what the
// controller really answers. Its name stays under GENL_NAMSIZ, or the
// controller's attribute policy answers EINVAL before any lookup. The real
// families are logged, not asserted, because this host's kernel is not under
// test.
func TestFamilyProbesOnThisHost(t *testing.T) {
	if got := GenericNetlinkFamily("zenosuchfam"); got.State != StateAbsent {
		t.Fatalf("an unregistered generic netlink family read %s (%v), want absent", got.State, got.Reason)
	}
	for _, name := range []string{"l2tp", "wireguard"} {
		got := GenericNetlinkFamily(name)
		t.Logf("generic netlink family %s: %s %v", name, got.State, got.Reason)
	}
	for _, protocol := range []int{0, 1} {
		got := PPPoXProtocol(protocol)
		t.Logf("PPPoX protocol %d: %s %v", protocol, got.State, got.Reason)
	}
}
