// Design: ai/rules/repo-maintenance.md -- doctor check platform probe

//go:build linux

package ifacera

import (
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// readIPv6Forwarding reports whether IPv6 forwarding is on for device, and
// whether the state could be read. An unreadable file yields (false, false):
// the caller must not read the first value as "forwarding is off", because the
// state was never seen.
func readIPv6Forwarding(device string) (on, known bool) {
	if device == "" || strings.Contains(device, "..") || strings.ContainsAny(device, "/\x00") {
		return false, false
	}
	var tb textbuf.Buffer
	path := tb.Reset().Str("/proc/sys/net/ipv6/conf/").Str(device).Str("/forwarding").String()
	data, err := os.ReadFile(path) //nolint:gosec // the device name is checked above
	if err != nil {
		return false, false
	}
	return strings.TrimSpace(string(data)) != "0", true
}
