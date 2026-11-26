// Design: docs/architecture/pki/tls-listeners.md -- the web certificate reference finding says what ze does with the name

package web

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWebTLSDoctorSaysTheWebServerIsNotStarted(t *testing.T) {
	// VALIDATES: the doctor finding of a web certificate reference that cannot
	// serve TLS says what ze does with it, which is to start without the web
	// server and to refuse a reload, and carries the fix with the name the
	// operator configured. An expired certificate, which ze still serves, says
	// neither.
	// PREVENTS: a finding that reads as a refused start, which ze no longer
	// does, and a finding with no fix.
	now := time.Now()
	far := now.Add(365 * 24 * time.Hour)

	for _, name := range []string{"typo", "no-key"} {
		t.Run(name, func(t *testing.T) {
			diags := webTLSDiagnostic(webDoctorTree(t, "true", name, far), now)
			require.Equal(t, []string{"doctor-tls-reference"}, codes(diags))
			require.Contains(t, diags[0].Message, "ze starts without the web server")
			require.Contains(t, diags[0].Message, "keeps its other services running")
			require.Contains(t, diags[0].Message, "a reload that names this certificate is refused")
			require.Equal(t, "Define it with `edit pki certificate "+name+"`, or delete `environment web certificate`, then restart ze.", diags[0].Fix)
		})
	}

	t.Run("an expired certificate is still served", func(t *testing.T) {
		diags := webTLSDiagnostic(webDoctorTree(t, "true", "lan", now.Add(-time.Hour)), now)
		require.Equal(t, []string{"doctor-tls-expired"}, codes(diags))
		require.NotContains(t, diags[0].Message, "ze starts without the web server")
		require.Equal(t, "replace the material with `edit pki certificate lan`, then commit: the web server presents it at the next handshake", diags[0].Fix)
	})
}
