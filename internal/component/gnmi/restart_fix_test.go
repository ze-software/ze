// Design: docs/guide/gnmi.md -- TLS: the transport serves TLS or nothing
// Related: tls.go -- tlsRestart, the restart every TLS fix line ends on

package gnmi

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
)

// VALIDATES: the fix of a gNMI TLS refusal names the restart for this process:
// `sudo systemctl restart ze` when ze runs under ze.service, and the stop-and-start
// steps (ze signal stop, then ze start) for a ze started another way, with no
// systemctl. Neither names `ze signal restart`, which exits without starting a new
// process.
// METHOD: fix the membership through privilege.SetServiceMembershipForTest, then read
// the fix of loadTLSCredentials over a key without a certificate and over a
// certificate file that cannot be read.
// PREVENTS: an operator whose ze runs outside a supervisor told to run a command that
// stops ze and leaves gNMI down with it, or told to run systemctl for a ze systemd did
// not start.
func TestGNMITLSFixNamesTheRestartForThisProcess(t *testing.T) {
	dir := t.TempDir()
	_, keyFile := writeTestKeyPair(t, dir, "gnmi")
	missing := filepath.Join(dir, "absent.pem")
	producers := []struct {
		name  string
		exact string
		cert  string
	}{
		{
			name:  "key without certificate",
			exact: "set environment gnmi tls cert (or ze.gnmi.tls.cert) to the certificate of that key, then restart ze: sudo systemctl restart ze",
			cert:  "",
		},
		{
			name: "unreadable certificate",
			exact: "restore " + missing + " or point environment gnmi tls cert (or ze.gnmi.tls.cert) at a readable PEM file, " +
				"then restart ze: sudo systemctl restart ze",
			cert: missing,
		},
	}
	fix := func(t *testing.T, cert string) string {
		t.Helper()
		_, err := loadTLSCredentials(cert, keyFile)
		refusal, ok := errors.AsType[*opererr.Error](err)
		if !ok {
			t.Fatalf("loadTLSCredentials answered %v, want an operator error", err)
		}
		return refusal.Fix()
	}
	for _, producer := range producers {
		t.Run(producer.name+"/ze.service", func(t *testing.T) {
			t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
			if got := fix(t, producer.cert); got != producer.exact {
				t.Fatalf("under ze.service the fix is %q, want %q", got, producer.exact)
			}
		})
		t.Run(producer.name+"/other", func(t *testing.T) {
			t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipOther, nil))
			got := fix(t, producer.cert)
			if strings.Contains(got, "systemctl") {
				t.Errorf("outside ze.service the fix names systemctl: %q", got)
			}
			if strings.Contains(got, "ze signal restart") {
				t.Errorf("the fix names ze signal restart, which leaves ze stopped: %q", got)
			}
			for _, want := range []string{"ze signal stop", "ze start"} {
				if !strings.Contains(got, want) {
					t.Errorf("outside ze.service the fix does not name %q: %q", want, got)
				}
			}
		})
	}
}
