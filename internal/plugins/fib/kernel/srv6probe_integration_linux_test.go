//go:build integration && linux

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: mplsipmtuprobe_integration_linux_test.go -- sibling namespace probe test for MPLS IP MTU
// Related: internal/component/kernelcap/probe_srv6_linux.go -- SRv6Encap and SRv6ForwardTTL

package fibkernel

import (
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/kernelcap"
)

const ipv4ForwardSysctl = "/proc/sys/net/ipv4/ip_forward"

// TestSRv6Integration_ProbeAnswersFromAFreshNamespace runs the enrolled SRv6
// encapsulation and forwarded-TTL probes from a fresh network namespace whose
// IPv4 forwarding is disabled, matching what a Docker probe container or an
// unconfigured router sees.
//
// VALIDATES: AC-7. On Ze's runtime kernel (release suffix "-ze"), both
// srv6-encap and srv6-forward-ttl answer present; on a stock kernel with seg6,
// srv6-encap answers present and srv6-forward-ttl answers absent; on a kernel
// without seg6, both answer absent; neither answers unknown or denied when the
// test holds CAP_SYS_ADMIN and /dev/net/tun; and the caller's
// /proc/sys/net/ipv4/ip_forward stays unchanged after the probe runs.
// PREVENTS: the SRv6 probe mutating the caller's namespace sysctls, timing out
// on a healthy kernel, or failing to distinguish Ze's patched kernel from a
// stock kernel.
func TestSRv6Integration_ProbeAnswersFromAFreshNamespace(t *testing.T) {
	skipWithoutInitNamespacePrivilege(t, "the kernelcap SRv6 probe binds /proc/sys, which a user namespace refuses (EINVAL)")
	withNetNS(t, func() {
		before := readIPv4Forward(t)

		encap := kernelcap.SRv6Encap()
		forwardTTL := kernelcap.SRv6ForwardTTL()
		release := kernelRelease(t)
		t.Logf("kernel %s: srv6-encap %v (%v); srv6-forward-ttl %v (%v)",
			release, encap.State, encap.Reason, forwardTTL.State, forwardTTL.Reason)

		if after := readIPv4Forward(t); after != before {
			t.Errorf("the SRv6 probe changed the caller's %s from %q to %q", ipv4ForwardSysctl, before, after)
		}

		if encap.State == kernelcap.StateUnknown {
			t.Fatalf("srv6-encap answered unknown (%v)", encap.Reason)
		}
		if encap.State == kernelcap.StateDenied {
			t.Fatalf("srv6-encap answered denied (%v)", encap.Reason)
		}
		if forwardTTL.State == kernelcap.StateUnknown {
			t.Fatalf("srv6-forward-ttl answered unknown (%v)", forwardTTL.Reason)
		}
		if forwardTTL.State == kernelcap.StateDenied {
			t.Fatalf("srv6-forward-ttl answered denied (%v)", forwardTTL.Reason)
		}

		if strings.HasSuffix(release, "-ze") {
			if encap.State != kernelcap.StatePresent {
				t.Fatalf("on Ze kernel %s: srv6-encap = %v (%v), want present", release, encap.State, encap.Reason)
			}
			if forwardTTL.State != kernelcap.StatePresent {
				t.Fatalf("on Ze kernel %s: srv6-forward-ttl = %v (%v), want present", release, forwardTTL.State, forwardTTL.Reason)
			}
			return
		}

		switch encap.State {
		case kernelcap.StatePresent:
			if forwardTTL.State != kernelcap.StateAbsent {
				t.Fatalf("on stock kernel %s with seg6: srv6-forward-ttl = %v (%v), want absent", release, forwardTTL.State, forwardTTL.Reason)
			}
		case kernelcap.StateAbsent:
			if forwardTTL.State != kernelcap.StateAbsent {
				t.Fatalf("on stock kernel %s without seg6: srv6-forward-ttl = %v (%v), want absent", release, forwardTTL.State, forwardTTL.Reason)
			}
		case kernelcap.StateUnknown, kernelcap.StateDenied, kernelcap.StateUnspecified:
			t.Fatalf("on stock kernel %s: unexpected srv6-encap state %v (%v)", release, encap.State, encap.Reason)
		}
	})
}

func readIPv4Forward(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(ipv4ForwardSysctl)
	if err != nil {
		t.Fatalf("read %s: %v", ipv4ForwardSysctl, err)
	}
	return strings.TrimSpace(string(data))
}
