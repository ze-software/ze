// Design: plan/learned/909-unified-update-backend.md -- shared gokrazy management helpers

package gokrazyutil

import (
	"encoding/base64"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// DefaultSocketPath is the Unix socket for gokrazy's HTTP management interface.
const DefaultSocketPath = "/run/gokrazy-http.sock"

// AuthHeader returns the gokrazy HTTP Basic Auth header, or an empty string.
func AuthHeader() string {
	return authHeaderFor(ReadPassword())
}

// authHeaderFor builds the gokrazy HTTP Basic Auth header for the given
// password, or an empty string when no password is configured. gokrazy
// authenticates as user "gokrazy" with the configured password as the
// secret (gokrazy/authenticated.go), so the credentials must encode
// "gokrazy:<password>".
func authHeaderFor(password string) string {
	if password == "" {
		return ""
	}
	var tb textbuf.Buffer
	creds := base64.StdEncoding.EncodeToString(tb.Str("gokrazy:").Str(password).Bytes())
	return tb.Reset().Str("Basic ").Str(creds).String()
}

// passwordPaths are the locations gokrazy stores the HTTP password, tried
// in order. Kept as a variable so tests can point it at a fixture dir.
var passwordPaths = []string{"/perm/gokr-pw.txt", "/etc/gokr-pw.txt", "/gokr-pw.txt"}

// ReadPassword reads the HTTP password from the same locations gokrazy uses.
func ReadPassword() string {
	return readPasswordFrom(passwordPaths)
}

func readPasswordFrom(paths []string) string {
	for _, path := range paths {
		data, err := os.ReadFile(path) //nolint:gosec // trusted caller-controlled paths
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}
