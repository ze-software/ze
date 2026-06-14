// Design: plan/spec-unified-update-backend.md -- shared gokrazy management helpers

package gokrazyutil

import (
	"encoding/base64"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// DefaultSocketPath is the Unix socket for gokrazy's HTTP management interface.
const DefaultSocketPath = "/run/gokrazy-http.sock"

// AuthHeader returns the gokrazy HTTP Basic Auth header, or an empty string.
func AuthHeader() string {
	password := ReadPassword()
	if password == "" {
		return ""
	}
	var tb textbuf.Buffer
	creds := base64.StdEncoding.EncodeToString(tb.Str("gokrazy:").Bytes())
	return tb.Reset().Str("Basic ").Str(creds).String()
}

// ReadPassword reads the HTTP password from the same locations gokrazy uses.
func ReadPassword() string {
	for _, path := range []string{"/perm/gokr-pw.txt", "/etc/gokr-pw.txt", "/gokr-pw.txt"} {
		data, err := os.ReadFile(path) //nolint:gosec // paths are hardcoded constants
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}
