// Design: docs/architecture/appliance/on-device-installer.md -- on-device installer input validation

package disk

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/rescueauth"
)

func validateIPv4(s string) error {
	if s == "" {
		return fmt.Errorf("empty IPv4 address")
	}
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return fmt.Errorf("IPv4 %q: need 4 octets, got %d", s, len(parts))
	}
	for _, p := range parts {
		if p == "" {
			return fmt.Errorf("IPv4 %q: empty octet", s)
		}
		if len(p) > 1 && p[0] == '0' {
			return fmt.Errorf("IPv4 %q: leading zero in octet %q", s, p)
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return fmt.Errorf("IPv4 %q: non-numeric octet %q", s, p)
		}
		if n < 0 || n > 255 {
			return fmt.Errorf("IPv4 %q: octet %d out of range", s, n)
		}
	}
	return nil
}

func validatePort(s string) error {
	if s == "" {
		return fmt.Errorf("empty port")
	}
	if len(s) > 1 && s[0] == '0' {
		return fmt.Errorf("port %q: leading zero", s)
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("port %q: not a number", s)
	}
	if n < 1 || n > 65535 {
		return fmt.Errorf("port %d: out of range 1-65535", n)
	}
	return nil
}

func validateImageName(s string) error {
	if s == "" {
		return fmt.Errorf("empty image name")
	}
	if s == "." || s == ".." {
		return fmt.Errorf("image name %q is a reserved path component", s)
	}
	if len(s) > 255 {
		return fmt.Errorf("image name too long (%d chars, max 255)", len(s))
	}
	for _, c := range s {
		if !isImageNameChar(c) {
			return fmt.Errorf("image name %q: invalid character %q", s, string(c))
		}
	}
	return nil
}

func isImageNameChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-'
}

func validateSource(s string) error {
	if s == sourceHTTP || s == sourceISO {
		return nil
	}
	return fmt.Errorf("source %q: must be %s or %s", s, sourceHTTP, sourceISO)
}

func validateMediaID(s string) error {
	if len(s) != 32 {
		return fmt.Errorf("media-id: must be 32 hex chars, got %d", len(s))
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return fmt.Errorf("media-id: invalid character %q (lowercase hex only)", string(c))
		}
	}
	return nil
}

func validateSHA256(s string) error {
	if len(s) != 64 {
		return fmt.Errorf("sha256: must be 64 hex chars, got %d", len(s))
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return fmt.Errorf("sha256: invalid character %q", string(c))
		}
	}
	return nil
}

// validateRescueAuth checks the shape of the ze.rescue-auth cmdline value. The
// value arrives from a PXE network the installer does not authenticate, so a
// malformed one is rejected here rather than reaching the rescue gate, where a
// decode failure would have to be distinguished from a wrong token.
func validateRescueAuth(s string) error {
	if err := rescueauth.Validate(s); err != nil {
		return fmt.Errorf("%w (got %d chars)", err, len(s))
	}
	return nil
}

func validateTargetPath(s string) error {
	if !strings.HasPrefix(s, "/dev/") {
		return fmt.Errorf("target %q: must start with /dev/", s)
	}
	name := s[5:]
	if name == "" {
		return fmt.Errorf("target %q: empty device name", s)
	}
	if strings.Contains(name, "/") || strings.Contains(name, "..") {
		return fmt.Errorf("target %q: path traversal", s)
	}

	switch {
	case isSkippedDisk(name):
		return fmt.Errorf("target %q: virtual/skip device", s)
	case isWholeDiskSD(name):
		return nil
	case isWholeDiskNVMe(name):
		return nil
	case isWholeDiskMMC(name):
		return nil
	}
	return fmt.Errorf("target %q: not a recognized whole-disk device", s)
}

func isSkippedDisk(name string) bool {
	for _, prefix := range []string{"loop", "ram", "dm-", "sr", "fd", "md", "zram", "mtdblock"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func isWholeDiskSD(name string) bool {
	for _, prefix := range []string{"sd", "vd", "xvd", "hd"} {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := name[len(prefix):]
		if rest == "" {
			return false
		}
		for _, c := range rest {
			if c < 'a' || c > 'z' {
				return false
			}
		}
		return true
	}
	return false
}

func isWholeDiskNVMe(name string) bool {
	if !strings.HasPrefix(name, "nvme") {
		return false
	}
	rest := name[4:]
	nIdx := strings.IndexByte(rest, 'n')
	if nIdx < 1 {
		return false
	}
	controller := rest[:nIdx]
	namespace := rest[nIdx+1:]
	if namespace == "" {
		return false
	}
	if _, err := strconv.Atoi(controller); err != nil {
		return false
	}
	if _, err := strconv.Atoi(namespace); err != nil {
		return false
	}
	return true
}

func isWholeDiskMMC(name string) bool {
	if !strings.HasPrefix(name, "mmcblk") {
		return false
	}
	rest := name[6:]
	if rest == "" {
		return false
	}
	_, err := strconv.Atoi(rest)
	return err == nil
}

func validateDecimal(s string) error {
	if s == "" {
		return fmt.Errorf("empty decimal value")
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return fmt.Errorf("decimal %q: non-digit character", s)
		}
	}
	return nil
}
