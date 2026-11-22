package network

import (
	"errors"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/privilege"
)

// listenRefusal is the error net.Listen returns when bind fails with errno:
// the same chain the BGP listener and the doctor probe receive.
func listenRefusal(network string, endpoint netip.AddrPort, errno syscall.Errno) error {
	return &net.OpError{
		Op:   "listen",
		Net:  network,
		Addr: net.TCPAddrFromAddrPort(endpoint),
		Err:  os.NewSyscallError("bind", errno),
	}
}

// useUnprivilegedPortStart makes the kernel answer start, or fail to answer
// with readErr, for one test.
func useUnprivilegedPortStart(t *testing.T, start uint16, readErr error) {
	t.Helper()
	old := unprivilegedPortStart
	unprivilegedPortStart = func() (uint16, error) { return start, readErr }
	t.Cleanup(func() { unprivilegedPortStart = old })
}

var bgpSite = ListenSite{
	Network:  "tcp",
	Endpoint: netip.MustParseAddrPort("192.0.2.1:179"),
	Owner:    "bgp peer upstream1",
}

// TestListenErrorPermissionBelowUnprivilegedStartNamesTheCapability checks the
// refusal the owner met: an unprivileged process binding port 179 under the
// kernel's default unprivileged port start.
//
// Method: EACCES and EPERM on port 179 with the start at 1024. Each MUST name
// the configured address, port and owner, CAP_NET_BIND_SERVICE and the start,
// carry privilege.CapabilityRemedy as its fix on the last line, and keep the
// errno reachable through errors.Is.
//
// VALIDATES: a BGP bind refused for the capability says so, with the remedy.
// PREVENTS: "listen tcp 192.0.2.1:179: bind: permission denied" reaching the
// operator with no capability, no owner and no fix.
func TestListenErrorPermissionBelowUnprivilegedStartNamesTheCapability(t *testing.T) {
	useUnprivilegedPortStart(t, 1024, nil)
	remedy := privilege.CapabilityRemedy("CAP_NET_BIND_SERVICE")
	for _, errno := range []syscall.Errno{syscall.EACCES, syscall.EPERM} {
		t.Run(errno.Error(), func(t *testing.T) {
			cause := listenRefusal("tcp", bgpSite.Endpoint, errno)
			refusal := ListenError(bgpSite, cause)

			assert.Equal(t, "cannot listen on 192.0.2.1:179 for bgp peer upstream1", refusal.Problem())
			assert.Contains(t, refusal.Why(), "port 179 is below the kernel's unprivileged port start 1024")
			assert.Contains(t, refusal.Why(), "CAP_NET_BIND_SERVICE")
			assert.Equal(t, remedy, refusal.Fix(), "the fix MUST be the capability remedy privilege owns")
			assert.ErrorIs(t, refusal, errno, "the errno MUST stay reachable through the operator error")
			lines := strings.Split(refusal.Error(), "\n")
			assert.Equal(t, "fix: "+remedy, lines[len(lines)-1], "the fix MUST be the last line")
		})
	}
}

// TestListenErrorPermissionAtOrAboveUnprivilegedStartIsAPolicy checks that a
// permission refusal the capability cannot explain does not blame it.
//
// Method: EACCES on port 1790 under a start of 1024, and on port 179 under a
// start of 0 (the value a container runtime often sets). Neither MUST name
// the capability remedy; each MUST say a security policy refused the bind.
//
// VALIDATES: the capability is named only when the port is privileged.
// PREVENTS: telling the operator to grant a capability that changes nothing.
func TestListenErrorPermissionAtOrAboveUnprivilegedStartIsAPolicy(t *testing.T) {
	remedy := privilege.CapabilityRemedy("CAP_NET_BIND_SERVICE")
	cases := []struct {
		name     string
		start    uint16
		endpoint string
	}{
		{"port above the start", 1024, "192.0.2.1:1790"},
		{"start lowered to zero", 0, "192.0.2.1:179"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useUnprivilegedPortStart(t, tc.start, nil)
			site := bgpSite
			site.Endpoint = netip.MustParseAddrPort(tc.endpoint)
			refusal := ListenError(site, listenRefusal("tcp", site.Endpoint, syscall.EACCES))

			assert.Contains(t, refusal.Why(), "is not below the kernel's unprivileged port start")
			assert.Contains(t, refusal.Why(), "a security policy")
			assert.NotEqual(t, remedy, refusal.Fix(), "a port the capability does not gate MUST NOT get the capability remedy")
			assert.Equal(t, policyDenialCommand, refusal.Fix())
		})
	}
}

// TestListenErrorPermissionUnknownStartSaysMay checks the answer when the
// kernel's unprivileged port start cannot be read.
//
// Method: the reader fails. The refusal MUST say the port may need
// CAP_NET_BIND_SERVICE, name the sysctl it could not read, and still carry the
// capability remedy as the likely fix.
//
// VALIDATES: a check that cannot determine the cause says so.
// PREVENTS: asserting the capability as the cause, or falling back to a
// guessed 1024.
func TestListenErrorPermissionUnknownStartSaysMay(t *testing.T) {
	useUnprivilegedPortStart(t, 0, errors.New("open /proc/sys/net/ipv4/ip_unprivileged_port_start: no such file or directory"))
	refusal := ListenError(bgpSite, listenRefusal("tcp", bgpSite.Endpoint, syscall.EACCES))

	assert.Contains(t, refusal.Why(), "port 179 may need it")
	assert.Contains(t, refusal.Why(), unprivilegedPortStartPath)
	assert.Contains(t, refusal.Why(), "CAP_NET_BIND_SERVICE")
	assert.NotContains(t, refusal.Why(), "is below", "an unread start MUST NOT be reported as compared")
	assert.Equal(t, privilege.CapabilityRemedy("CAP_NET_BIND_SERVICE"), refusal.Fix())
}

// TestListenErrorOtherCausesKeepTheirOwnFix checks the refusals that are not
// about permission: each keeps its own cause and fix, and none names the
// capability.
//
// Method: EADDRINUSE over TCP and UDP, EADDRNOTAVAIL, and an error the
// producer does not classify, each with the start at 1024.
//
// VALIDATES: an address in use names the command that finds its holder, a
// missing address names the address list, anything else names ze doctor.
// PREVENTS: a capability remedy for a port another program holds.
func TestListenErrorOtherCausesKeepTheirOwnFix(t *testing.T) {
	useUnprivilegedPortStart(t, 1024, nil)
	cases := []struct {
		name    string
		network string
		cause   error
		why     string
		fix     string
	}{
		{"address in use tcp", "tcp", listenRefusal("tcp", bgpSite.Endpoint, syscall.EADDRINUSE), "already listens", "sudo ss -ltnp 'sport = :179'"},
		{"address in use udp", "udp", listenRefusal("udp", bgpSite.Endpoint, syscall.EADDRINUSE), "already listens", "sudo ss -lunp 'sport = :179'"},
		{"address not available", "tcp", listenRefusal("tcp", bgpSite.Endpoint, syscall.EADDRNOTAVAIL), "no interface on this host holds the address 192.0.2.1", addressListCommand},
		{"unclassified", "tcp", errors.New("forced listener failure"), "the kernel refused", unknownCauseCommand},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			site := bgpSite
			site.Network = tc.network
			refusal := ListenError(site, tc.cause)

			assert.Equal(t, "cannot listen on 192.0.2.1:179 for bgp peer upstream1", refusal.Problem())
			assert.Contains(t, refusal.Why(), tc.why)
			assert.Equal(t, tc.fix, refusal.Fix())
			assert.NotContains(t, refusal.Error(), "CAP_NET_BIND_SERVICE")
			assert.ErrorIs(t, refusal, tc.cause)
		})
	}
}

// TestListenErrorWithoutOwner checks the problem line when the caller cannot
// name what configured the listener.
//
// VALIDATES: the line names the endpoint and stops, with no dangling "for".
// PREVENTS: "cannot listen on 192.0.2.1:179 for " reaching the operator.
func TestListenErrorWithoutOwner(t *testing.T) {
	useUnprivilegedPortStart(t, 1024, nil)
	site := bgpSite
	site.Owner = ""
	refusal := ListenError(site, listenRefusal("tcp", site.Endpoint, syscall.EADDRINUSE))
	assert.Equal(t, "cannot listen on 192.0.2.1:179", refusal.Problem())
}

// TestIsPortPermissionRefusal checks the predicate a prober reads to say which
// process it measured.
//
// VALIDATES: EACCES and EPERM through a net.OpError answer true, an address
// in use answers false.
// PREVENTS: a probe note about the measured process on a refusal that does
// not depend on the process.
func TestIsPortPermissionRefusal(t *testing.T) {
	assert.True(t, IsPortPermissionRefusal(listenRefusal("tcp", bgpSite.Endpoint, syscall.EACCES)))
	assert.True(t, IsPortPermissionRefusal(listenRefusal("tcp", bgpSite.Endpoint, syscall.EPERM)))
	assert.False(t, IsPortPermissionRefusal(listenRefusal("tcp", bgpSite.Endpoint, syscall.EADDRINUSE)))
}

// TestUnprivilegedPortStartReadsTheKernel checks the reader against the file
// it reads, where the kernel provides one.
//
// VALIDATES: the value parsed is the value /proc holds.
// PREVENTS: a reader that answers a constant.
func TestUnprivilegedPortStartReadsTheKernel(t *testing.T) {
	data, err := os.ReadFile(unprivilegedPortStartPath)
	if err != nil {
		_, readErr := UnprivilegedPortStart()
		require.Error(t, readErr, "a start the file cannot give MUST be an error, never a guess")
		return
	}
	start, err := UnprivilegedPortStart()
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(string(data)), strconv.FormatUint(uint64(start), 10))
}
