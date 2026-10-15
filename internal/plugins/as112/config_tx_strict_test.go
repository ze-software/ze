package as112

import (
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/dnsserver"
)

// VALIDATES: R-13 for as112. A reload that turns DoT on with a certificate
// the PKI store does not hold fails the install with an error naming the
// certificate and the command that lists the store; the previous snapshot
// stays published and the running config is unchanged.
// PREVENTS: a DoT reload reported as committed while DoT is not served.
// Not parallel: stateP is package state.
func TestInstallReloadWithMissingCertificateIsRefused(t *testing.T) {
	prod := newAS112Producer()
	inst := &as112Installer{prod: prod, mgr: newServerManager(slog.Default(), prod.onServingChanged), log: slog.Default()}
	t.Cleanup(func() {
		inst.mgr.stopAll()
		stateP.Store(nil)
	})
	running := as112Config{Hostname: "node-a.example", AddressFamily: addressFamilyIPv4Only, Secure: dnsserver.DefaultSecureConfig()}
	inst.boot = true
	if err := inst.install(running); err != nil {
		t.Fatalf("boot install: %v", err)
	}
	inst.boot = false
	installed := loadState()

	candidate := running
	candidate.Enabled = true
	candidate.Secure.DoTEnabled = true
	candidate.Secure.DoTPort = freePort(t)
	candidate.Secure.Certificate = "absent-from-the-store"
	err := inst.install(candidate)
	if err == nil {
		t.Fatal("reload naming a missing certificate succeeded")
	}
	for _, want := range []string{"as112: dnsserver: reload refused", `"absent-from-the-store"`, "ze show pki certificates"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	if loadState() != installed {
		t.Error("a refused reload left its snapshot published")
	}
	if !reflect.DeepEqual(inst.running, running) {
		t.Errorf("running config after the refusal = %+v, want the boot config", inst.running)
	}
}
