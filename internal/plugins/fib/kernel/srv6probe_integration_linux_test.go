//go:build integration && linux

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: mplsipmtuprobe_integration_linux_test.go -- sibling namespace probe test for MPLS IP MTU
// Related: internal/component/kernelcap/probe_srv6_linux.go -- SRv6Encap, SRv6ForwardTTL, and SRv6ForwardFrag

package fibkernel

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/kernelcap"
)

const ipv4ForwardSysctl = "/proc/sys/net/ipv4/ip_forward"

// TestSRv6Integration_ProbeAnswersFromAFreshNamespace runs the enrolled SRv6
// encapsulation, forwarded-TTL, and DF-clear fragmentation probes from a fresh
// network namespace whose IPv4 forwarding is disabled, matching what a Docker
// probe container or an unconfigured router sees.
//
// VALIDATES: AC-5 (srv6-df-clear-fragmentation) and AC-7
// (srv6-headend-decrements-inner-ttl). On Ze's runtime kernel (release suffix
// "-ze"), srv6-encap, srv6-forward-ttl, and srv6-forward-frag all answer
// present; on a stock kernel with seg6, srv6-encap answers present while
// srv6-forward-ttl and srv6-forward-frag answer absent without waiting for the
// 1-second timeout; on a kernel without seg6, all three answer absent; none
// answers unknown or denied when the test holds CAP_SYS_ADMIN and /dev/net/tun;
// and the caller's /proc/sys/net/ipv4/ip_forward stays unchanged after the
// probe runs.
// PREVENTS: the SRv6 probe mutating the caller's namespace sysctls, timing out
// on a healthy or stock kernel, or failing to distinguish Ze's patched kernel
// from a stock kernel.
func TestSRv6Integration_ProbeAnswersFromAFreshNamespace(t *testing.T) {
	skipWithoutInitNamespacePrivilege(t, "the kernelcap SRv6 probe binds /proc/sys, which a user namespace refuses (EINVAL)")
	withNetNS(t, func() {
		before := readIPv4Forward(t)

		start := time.Now()
		encap := kernelcap.SRv6Encap()
		forwardTTL := kernelcap.SRv6ForwardTTL()
		forwardFrag := kernelcap.SRv6ForwardFrag()
		elapsed := time.Since(start)
		release := kernelRelease(t)
		t.Logf("kernel %s (probe %v): srv6-encap %v (%v); srv6-forward-ttl %v (%v); srv6-forward-frag %v (%v)",
			release, elapsed,
			encap.State, encap.Reason,
			forwardTTL.State, forwardTTL.Reason,
			forwardFrag.State, forwardFrag.Reason)

		after := readIPv4Forward(t)
		if after != before {
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
		if forwardFrag.State == kernelcap.StateUnknown {
			t.Fatalf("srv6-forward-frag answered unknown (%v)", forwardFrag.Reason)
		}
		if forwardFrag.State == kernelcap.StateDenied {
			t.Fatalf("srv6-forward-frag answered denied (%v)", forwardFrag.Reason)
		}

		if strings.HasSuffix(release, "-ze") {
			if encap.State != kernelcap.StatePresent {
				t.Fatalf("on Ze kernel %s: srv6-encap = %v (%v), want present", release, encap.State, encap.Reason)
			}
			if forwardTTL.State != kernelcap.StatePresent {
				t.Fatalf("on Ze kernel %s: srv6-forward-ttl = %v (%v), want present", release, forwardTTL.State, forwardTTL.Reason)
			}
			if forwardFrag.State != kernelcap.StatePresent {
				t.Fatalf("on Ze kernel %s: srv6-forward-frag = %v (%v), want present", release, forwardFrag.State, forwardFrag.Reason)
			}
			return
		}

		switch encap.State {
		case kernelcap.StatePresent:
			if forwardTTL.State != kernelcap.StateAbsent {
				t.Fatalf("on stock kernel %s with seg6: srv6-forward-ttl = %v (%v), want absent", release, forwardTTL.State, forwardTTL.Reason)
			}
			if forwardFrag.State != kernelcap.StateAbsent {
				t.Fatalf("on stock kernel %s with seg6: srv6-forward-frag = %v (%v), want absent", release, forwardFrag.State, forwardFrag.Reason)
			}
		case kernelcap.StateAbsent:
			if forwardTTL.State != kernelcap.StateAbsent {
				t.Fatalf("on stock kernel %s without seg6: srv6-forward-ttl = %v (%v), want absent", release, forwardTTL.State, forwardTTL.Reason)
			}
			if forwardFrag.State != kernelcap.StateAbsent {
				t.Fatalf("on stock kernel %s without seg6: srv6-forward-frag = %v (%v), want absent", release, forwardFrag.State, forwardFrag.Reason)
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
