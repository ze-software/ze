package as112

import (
	"errors"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/dnsserver"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// registrations records the loopback address registrations an installer
// makes, and fails the registration whose index is failAt (counting from 1).
type registrations struct {
	calls  [][]string
	failAt int
}

func (r *registrations) register(_, _ string, addrs []string) error {
	r.calls = append(r.calls, slices.Clone(addrs))
	if len(r.calls) == r.failAt {
		return errors.New("address 192.175.48.6 is owned by another plugin")
	}
	return nil
}

func (r *registrations) unregister(string) { r.calls = append(r.calls, nil) }

func (r *registrations) last() []string { return r.calls[len(r.calls)-1] }

// refusedReloadFixture boots an installer on an enabled IPv4-only config with
// a distinct origin ASN and community, then builds a candidate that adds IPv6
// and turns DoT on with a certificate the PKI store does not hold, so the
// reload is refused at the TLS load, before any listener stops.
func refusedReloadFixture(t *testing.T, regs *registrations) (inst *as112Installer, running, candidate as112Config) {
	t.Helper()
	prod := newAS112Producer()
	inst = newAS112Installer(prod, newServerManager(slog.Default(), prod.onServingChanged), slog.Default())
	inst.register, inst.unregister = regs.register, regs.unregister
	t.Cleanup(func() {
		inst.mgr.stopAll()
		stateP.Store(nil)
	})
	running = as112Config{Enabled: true, Hostname: "node-a.example", AddressFamily: addressFamilyIPv4Only,
		ASN: 64512, Community: []uint32{64512<<16 | 112}, Secure: dnsserver.DefaultSecureConfig()}
	inst.boot = true
	if err := inst.install(running); err != nil {
		t.Fatalf("boot install: %v", err)
	}
	inst.boot = false

	candidate = running
	candidate.AddressFamily = addressFamilyBoth
	candidate.ASN = 64513
	candidate.Community = []uint32{64513<<16 | 113}
	candidate.Secure.DoTEnabled = true
	candidate.Secure.DoTPort = freePort(t)
	candidate.Secure.Certificate = "absent-from-the-store"
	return inst, running, candidate
}

// VALIDATES: R-13 for as112. A reload that turns DoT on with a certificate
// the PKI store does not hold fails the install with an error naming the
// certificate and the command that lists the store; the previous snapshot
// stays published, the redistribute producer keeps the running config's
// origin ASN and community, and the loopback registration is the running
// config's again.
// PREVENTS: a DoT reload reported as committed while DoT is not served, and a
// refused reload that leaves the candidate's announcement or address
// registration in place.
// Not parallel: stateP is package state. The listeners bind port 53, which an
// unprivileged host refuses, so whether they answer is proven in
// as112-reload.ci (QEMU guest), not here.
func TestInstallReloadWithMissingCertificateIsRefused(t *testing.T) {
	regs := &registrations{}
	inst, running, candidate := refusedReloadFixture(t, regs)
	installed := loadState()

	err := inst.install(candidate)
	if err == nil {
		t.Fatal("reload naming a missing certificate succeeded")
	}
	for _, want := range []string{"as112: dnsserver: reload refused", `"absent-from-the-store"`, "ze show pki certificates"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	if errors.Is(err, sdk.ErrRunningConfigLost) {
		t.Errorf("error %q says the running config is lost, but nothing was stopped", err)
	}
	if loadState() != installed {
		t.Error("a refused reload left its snapshot published")
	}
	if !reflect.DeepEqual(inst.running, running) {
		t.Errorf("running config after the refusal = %+v, want the boot config", inst.running)
	}
	inst.prod.mu.Lock()
	asn, community := inst.prod.lastCfg.ASN, inst.prod.lastCfg.Community
	inst.prod.mu.Unlock()
	if asn != running.ASN || !slices.Equal(community, running.Community) {
		t.Errorf("producer after the refusal: ASN %d community %v, want the running %d %v",
			asn, community, running.ASN, running.Community)
	}
	if got, want := regs.last(), hostAddresses(running.AddressFamily); !slices.Equal(got, want) {
		t.Errorf("loopback registration after the refusal = %v, want the running %v", got, want)
	}
}

// VALIDATES: ISSUE-5. A refused reload whose running loopback registration
// cannot be put back returns that failure, wrapping sdk.ErrRunningConfigLost
// so the transaction's rollback installs the running config again or
// restarts the plugin.
// PREVENTS: a registration restore failure only logged, the commit refused as
// if the running config were intact.
// Not parallel: stateP is package state.
func TestInstallRefusedReloadReturnsAFailedRegistrationRestore(t *testing.T) {
	regs := &registrations{failAt: 3} // boot, candidate, then the restore
	inst, _, candidate := refusedReloadFixture(t, regs)

	err := inst.install(candidate)
	if !errors.Is(err, sdk.ErrRunningConfigLost) {
		t.Fatalf("refused reload whose registration restore failed: %v, want sdk.ErrRunningConfigLost", err)
	}
	for _, want := range []string{"reload refused", "loopback address registration could not be restored", "owned by another plugin"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}
