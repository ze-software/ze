// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
//go:build linux

package dataplane

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/kernelcap"
)

// The MOBIKE migration message is enrolled from this package's init(), so a
// Docker host or an appliance without it is named by ze doctor. Method: read
// the process-wide enrolment the init() wrote.
func TestMOBIKEMigrationIsEnrolled(t *testing.T) {
	enrolled := kernelcap.Enrolled()
	for _, subsystem := range []string{"ipsec-mobike", "ipsec-esp-ipv4", "ipsec-esp-ipv6"} {
		if !slices.Contains(enrolled, subsystem) {
			t.Errorf("%s is not enrolled; enrolment is %v", subsystem, enrolled)
		}
	}
}

// Every kernel transform the algorithm tables can install is enrolled, once,
// and nothing else is: the enrolment is derived from the tables, never listed.
// Method: walk the three tables and compare against the enrolment.
func TestXFRMAlgorithmEnrolmentFollowsTheTable(t *testing.T) {
	want := map[string]bool{}
	for _, transform := range xfrmEncNames {
		want["ipsec-transform-"+kernelSubsystemWord(transform)] = true
	}
	for _, transform := range xfrmAEADNames {
		want["ipsec-transform-"+kernelSubsystemWord(transform)] = true
	}
	for _, auth := range xfrmAuthNames {
		want["ipsec-transform-"+kernelSubsystemWord(auth.name)] = true
	}

	got := map[string]bool{}
	for _, subsystem := range kernelcap.Enrolled() {
		if strings.HasPrefix(subsystem, "ipsec-transform-") {
			got[subsystem] = true
		}
	}
	for subsystem := range want {
		if !got[subsystem] {
			t.Errorf("table transform %s is not enrolled", subsystem)
		}
	}
	for subsystem := range got {
		if !want[subsystem] {
			t.Errorf("enrolled %s names no transform the tables hold", subsystem)
		}
	}
	if len(xfrmTransformKernel) != len(want) {
		t.Errorf("xfrmTransformKernel describes %d transforms, the tables name %d", len(xfrmTransformKernel), len(want))
	}
}

// The probe state must name a reserved SPI and documentation addresses so an
// update can never find a real SA, and it must put the transform under test in
// the role the backend installs it in.
func TestXFRMProbeStateShape(t *testing.T) {
	aead := xfrmProbeState(xfrmProbeSource4, xfrmProbeTarget4, xfrmRoleAEAD, xfrmAEADAESGCM)
	if aead.Spi < 1 || aead.Spi > 255 {
		t.Fatalf("probe SPI %d is outside the RFC 4303 reserved range", aead.Spi)
	}
	if aead.Aead == nil || aead.Crypt != nil || aead.Auth != nil {
		t.Fatalf("AEAD probe must carry only an AEAD transform: %+v", aead)
	}
	if len(aead.Aead.Key) != 20 {
		t.Errorf("rfc4106 key is %d octets, want 16 of key and 4 of salt", len(aead.Aead.Key))
	}

	cipher := xfrmProbeState(xfrmProbeSource4, xfrmProbeTarget4, xfrmRoleCipher, "cbc(des3_ede)")
	if cipher.Crypt == nil || cipher.Crypt.Name != "cbc(des3_ede)" || cipher.Auth == nil || cipher.Auth.Name != xfrmAuthSHA256 {
		t.Fatalf("cipher probe must pair the cipher with HMAC-SHA-256: %+v", cipher)
	}

	integrity := xfrmProbeState(xfrmProbeSource6, xfrmProbeTarget6, xfrmRoleIntegrity, "hmac(sha1)")
	if integrity.Auth == nil || integrity.Auth.Name != "hmac(sha1)" || integrity.Auth.TruncateLen != 96 {
		t.Fatalf("integrity probe must carry the backend's truncation: %+v", integrity.Auth)
	}
	if integrity.Crypt == nil || integrity.Crypt.Name != xfrmEncAESCBC {
		t.Fatalf("integrity probe must pair with AES-CBC: %+v", integrity.Crypt)
	}
}

// Each errno maps to the state the kernel source gives it, and no answer that
// did not reach the question reads as present.
func TestXFRMKernelProbeClassification(t *testing.T) {
	type row struct {
		name string
		err  error
		want kernelcap.State
	}
	run := func(t *testing.T, classify func(error) kernelcap.Result, rows []row) {
		t.Helper()
		for _, tc := range rows {
			got := classify(tc.err)
			if got.State != tc.want {
				t.Errorf("%s: state %s, want %s", tc.name, got.State, tc.want)
			}
			if got.State != kernelcap.StatePresent && got.Reason == nil {
				t.Errorf("%s: a %s answer carries no reason", tc.name, got.State)
			}
		}
	}
	t.Run("migration", func(t *testing.T) {
		run(t, classifyXFRMMigration, []row{
			{"handler-ran", unix.ESRCH, kernelcap.StatePresent},
			{"before-7.2", unix.EINVAL, kernelcap.StateAbsent},
			{"no-xfrm-migrate", unix.ENOPROTOOPT, kernelcap.StateAbsent},
			{"no-xfrm-netlink", unix.EPROTONOSUPPORT, kernelcap.StateAbsent},
			{"unprivileged", unix.EPERM, kernelcap.StateUnknown},
			{"accepted", nil, kernelcap.StateUnknown},
		})
	})
	t.Run("esp", func(t *testing.T) {
		run(t, classifyESPProbe, []row{
			{"built", unix.ESRCH, kernelcap.StatePresent},
			{"no-esp-type", unix.EPROTONOSUPPORT, kernelcap.StateAbsent},
			{"probe-aead-missing", unix.ENOSYS, kernelcap.StateUnknown},
			{"unprivileged", unix.EPERM, kernelcap.StateUnknown},
			{"accepted", nil, kernelcap.StateUnknown},
		})
	})
	t.Run("transform", func(t *testing.T) {
		run(t, classifyTransformProbe, []row{
			{"built", unix.ESRCH, kernelcap.StatePresent},
			{"xfrm-lookup-failed", unix.ENOSYS, kernelcap.StateAbsent},
			{"crypto-alloc-failed", unix.ENOENT, kernelcap.StateAbsent},
			{"no-esp-type", unix.EPROTONOSUPPORT, kernelcap.StateUnknown},
			{"bad-key", unix.EINVAL, kernelcap.StateUnknown},
			{"unprivileged", unix.EPERM, kernelcap.StateUnknown},
			{"accepted", nil, kernelcap.StateUnknown},
		})
	})
}

// The enrolled probes send exactly the update the classification reads.
// Method: stand in the netlink call and run every enrolled IKE probe.
func TestXFRMKernelProbesSendAnUpdate(t *testing.T) {
	old := xfrmProbeUpdate
	t.Cleanup(func() { xfrmProbeUpdate = old })
	var sent []*netlink.XfrmState
	xfrmProbeUpdate = func(state *netlink.XfrmState) error {
		sent = append(sent, state)
		return unix.ESRCH
	}
	capabilities := xfrmKernelCapabilities()
	for i := range capabilities {
		if capabilities[i].Subsystem == "ipsec-mobike" {
			continue
		}
		if got := capabilities[i].Probe(); got.State != kernelcap.StatePresent {
			t.Errorf("%s: state %s on ESRCH", capabilities[i].Subsystem, got.State)
		}
	}
	if len(sent) != len(capabilities)-1 {
		t.Fatalf("sent %d updates for %d update probes", len(sent), len(capabilities)-1)
	}
	for _, state := range sent {
		if state.Spi != xfrmProbeSPI {
			t.Errorf("probe sent SPI %d", state.Spi)
		}
	}
	if !errors.Is(classifyTransformProbe(unix.ENOSYS).Reason, unix.ENOSYS) {
		t.Error("an absent transform must carry the kernel's errno as its reason")
	}
}

// The real probes answer on this host's kernel with a verdict and leave no SA
// behind. Method: run every enrolled IKE probe against the kernel, then list the
// SAD. Without CAP_NET_ADMIN every update answers EPERM, which the probes report
// as unknown; the SAD read is then skipped because it needs the same capability.
func TestXFRMKernelProbesOnThisHost(t *testing.T) {
	capabilities := xfrmKernelCapabilities()
	for i := range capabilities {
		got := capabilities[i].Probe()
		if got.State == kernelcap.StateUnspecified {
			t.Errorf("%s: no verdict", capabilities[i].Subsystem)
		}
		t.Logf("%s (%s): %s %v", capabilities[i].Subsystem, capabilities[i].Kernel, got.State, got.Reason)
	}
	// The negative space: a transform no kernel holds must read absent through the
	// same update, or a present answer above proves nothing.
	bogus := xfrmProbeState(xfrmProbeSource4, xfrmProbeTarget4, xfrmRoleIntegrity, xfrmAuthSHA256)
	bogus.Crypt = &netlink.XfrmStateAlgo{Name: "cbc(ze-no-such-cipher)", Key: make([]byte, 16)}
	if bogusErr := xfrmProbeUpdate(bogus); !errors.Is(bogusErr, unix.EPERM) {
		if got := classifyTransformProbe(bogusErr); got.State != kernelcap.StateAbsent {
			t.Errorf("a transform no kernel holds read %s (%v)", got.State, got.Reason)
		}
	}

	states, err := netlink.XfrmStateList(netlink.FAMILY_ALL)
	if errors.Is(err, unix.EPERM) {
		t.Skip("the SAD cannot be listed without CAP_NET_ADMIN")
	}
	if err != nil {
		t.Fatalf("list SAD: %v", err)
	}
	for i := range states {
		if states[i].Spi == xfrmProbeSPI {
			t.Errorf("a probe left an SA behind: %+v", states[i])
		}
	}
}
