// Design: ai/rules/repo-maintenance.md -- doctor check platform probe

//go:build linux

package ifacera

import (
	"errors"
	"io/fs"
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

// readIPv6KernelForwarding reports whether Linux forwards IPv6 arriving on
// device: all.forwarding is not 0, or the device's force_forwarding is not 0
// (ip6_forward, net/ipv6/ip6_output.c). A kernel older than 6.17 has no
// force_forwarding, so an absent file there means only all.forwarding counts.
// It answers known false when all.forwarding cannot be read.
func readIPv6KernelForwarding(device string) (on, known bool) {
	data, err := os.ReadFile("/proc/sys/net/ipv6/conf/all/forwarding")
	if err != nil {
		return false, false
	}
	if strings.TrimSpace(string(data)) != "0" {
		return true, true
	}
	if device == "" {
		return false, false
	}
	if strings.Contains(device, "..") {
		return false, false
	}
	if strings.ContainsAny(device, "/\x00") {
		return false, false
	}
	var tb textbuf.Buffer
	path := tb.Reset().Str("/proc/sys/net/ipv6/conf/").Str(device).Str("/force_forwarding").String()
	force, err := os.ReadFile(path) //nolint:gosec // the device name is checked above
	// refuse-by-default: guards checkRAForwarding -- a kernel with no force_forwarding leaf lets all.forwarding alone decide, and it is 0
	if errors.Is(err, fs.ErrNotExist) {
		return false, true
	}
	if err != nil {
		return false, false
	}
	return strings.TrimSpace(string(force)) != "0", true
}
