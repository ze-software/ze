package doctor

import (
	"errors"
	"net"
	"net/netip"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestDoctorRefusesUnderivableListenerSet makes the schema fail to load by
// swapping listenerSchema, then reads both the collector and the finding
// checkListeners reports.
//
// VALIDATES: doctor reports doctor-listener-set-unknown, naming the cause and
// the fix, and probes no schema-declared listener when the schema cannot load.
// PREVENTS: doctor answering a guessed listener list (the deleted hardcoded
// fallback) or an empty one, either of which reads as "every listener checked".
func TestDoctorRefusesUnderivableListenerSet(t *testing.T) {
	const cause = "cannot load the YANG schema: module ze-ssh-conf does not resolve"
	useListenerSchema(t, func() (*config.Schema, error) { return nil, errors.New("module ze-ssh-conf does not resolve") })
	probed := recordProbes(t)
	tree := sshEnabledTree()

	listeners, err := collectSchemaListeners(tree)
	require.Error(t, err)
	assert.Equal(t, cause, err.Error())
	assert.Empty(t, listeners, "an underivable listener set must yield no listener, not a guessed list")

	diags := checkListeners(tree)
	var refusal *diagnostic.Diagnostic
	for i := range diags {
		if diags[i].Code == diagnostic.CodeDoctorListenerSetUnknown {
			refusal = &diags[i]
		}
	}
	require.NotNil(t, refusal, "checkListeners must report %s, got %v", diagnostic.CodeDoctorListenerSetUnknown, diags)
	assert.Equal(t, diagnostic.SeverityError, refusal.Severity)
	assert.Contains(t, refusal.Message, cause, "the finding must name the cause")
	assert.True(t, strings.HasSuffix(refusal.Message, "then run: ze doctor"), "the finding must end with the command that confirms the fix: %q", refusal.Message)
	assert.NotContains(t, *probed, "ssh", "no schema-declared listener may be probed when the set is unknown")
	assert.NotNil(t, diagnostic.Lookup(diagnostic.CodeDoctorListenerSetUnknown), "the refusal code must be registered")
}

// TestDoctorSchemaWithNoListenerServiceIsAnAnswer gives doctor a schema that
// loads and declares no ze:listener service, as a build whose linked modules
// carry none does.
//
// VALIDATES: a loaded schema with zero listener services yields no schema
// listener and no error, and checkListeners reports no refusal.
// PREVENTS: a stripped build (or an untagged test run) being told its binary
// is broken when the schema has simply nothing to probe.
func TestDoctorSchemaWithNoListenerServiceIsAnAnswer(t *testing.T) {
	useListenerSchema(t, func() (*config.Schema, error) { return config.NewSchema(), nil })
	probed := recordProbes(t)
	tree := sshEnabledTree()

	listeners, err := collectSchemaListeners(tree)
	require.NoError(t, err, "a schema that loaded is an answer, even with no listener service")
	assert.Empty(t, listeners)

	for _, d := range checkListeners(tree) {
		assert.NotEqual(t, diagnostic.CodeDoctorListenerSetUnknown, d.Code, "a loaded schema must not be refused: %v", d)
	}
	assert.NotContains(t, *probed, "ssh", "a schema without the ssh service yields no ssh probe")
}

// bgpProbeRefusal is what the bind probe returns when the kernel refuses the
// bind of the BGP listener with errno.
func bgpProbeRefusal(errno syscall.Errno) error {
	return &net.OpError{Op: "listen", Net: "tcp", Err: os.NewSyscallError("bind", errno)}
}

// TestListenerProbeFindingNamesTheCapability checks the doctor finding for a
// BGP listener the probe could not bind for lack of permission.
//
// Method: listenerProbeFinding judges a BGP endpoint on port 179 refused with
// EACCES. The finding MUST keep the BGP code and severity, name the address,
// port and service, name the process it measured, and carry the same fix the
// daemon's own refusal carries (network.ListenError): the
// CAP_NET_BIND_SERVICE remedy wherever the host's unprivileged port start
// makes 179 privileged.
//
// VALIDATES: doctor names the capability and its remedy for port 179, and says
// which process its probe measured.
// PREVENTS: "bgp: cannot bind tcp 192.0.2.1:179: ... permission denied" with no
// capability and no fix, and a finding that reads the operator's own process
// as the daemon's.
func TestListenerProbeFindingNamesTheCapability(t *testing.T) {
	l := tcpListener("bgp", "192.0.2.1", "179", diagnostic.CodeDoctorBGPListen)
	cause := bgpProbeRefusal(syscall.EACCES)
	finding := listenerProbeFinding(&textbuf.Buffer{}, l, cause)

	assert.Equal(t, diagnostic.CodeDoctorBGPListen, finding.Code)
	assert.Equal(t, diagnostic.SeverityWarning, finding.Severity)
	assert.Contains(t, finding.Message, "cannot listen on 192.0.2.1:179 for bgp")
	assert.Contains(t, finding.Message, "The probe measured process")
	assert.Contains(t, finding.Message, "permission denied", "the kernel's answer MUST stay in the finding as evidence")
	want := network.ListenError(network.ListenSite{Network: "tcp", Endpoint: netip.MustParseAddrPort("192.0.2.1:179"), Owner: "bgp"}, cause)
	assert.Equal(t, want.Fix(), finding.Fix, "doctor MUST carry the fix the daemon's refusal carries")

	start, readErr := network.UnprivilegedPortStart()
	if readErr == nil && start <= 179 {
		return
	}
	assert.Contains(t, finding.Message, "CAP_NET_BIND_SERVICE")
	assert.Equal(t, privilege.CapabilityRemedy("CAP_NET_BIND_SERVICE"), finding.Fix)
}

// TestListenerProbeFindingAddressInUseKeepsItsFix checks a probe refusal that
// does not depend on the probing process.
//
// VALIDATES: an address in use names the command that finds its holder and
// no process note, and no capability.
// PREVENTS: a capability remedy for a port another program holds.
func TestListenerProbeFindingAddressInUseKeepsItsFix(t *testing.T) {
	l := tcpListener("bgp", "192.0.2.1", "179", diagnostic.CodeDoctorBGPListen)
	finding := listenerProbeFinding(&textbuf.Buffer{}, l, bgpProbeRefusal(syscall.EADDRINUSE))

	assert.Equal(t, "sudo ss -ltnp 'sport = :179'", finding.Fix)
	assert.NotContains(t, finding.Message, "The probe measured process")
	assert.NotContains(t, finding.Message, "CAP_NET_BIND_SERVICE")
}

// useListenerSchema replaces the schema loader for one test.
func useListenerSchema(t *testing.T, schema func() (*config.Schema, error)) {
	t.Helper()
	old := listenerSchema
	listenerSchema = schema
	t.Cleanup(func() { listenerSchema = old })
}

// recordProbes replaces the bind probe with one that records each service and
// binds nothing.
func recordProbes(t *testing.T) *[]string {
	t.Helper()
	var probed []string
	old := listenerProbe
	listenerProbe = func(l serviceListener) error {
		probed = append(probed, l.service)
		return nil
	}
	t.Cleanup(func() { listenerProbe = old })
	return &probed
}

// sshEnabledTree enables SSH with its default endpoint: a config the schema
// path probes when the ssh service is declared, so an empty probe set is
// evidence and not a config that binds nothing.
func sshEnabledTree() *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer("environment").GetOrCreateContainer("ssh").Set("enabled", "true")
	return tree
}
