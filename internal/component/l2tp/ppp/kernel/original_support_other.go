//go:build !linux

// Design: docs/architecture/doctor-and-health-checks.md -- platform refusal.

package kernel

import "errors"

// CheckOriginalPPPSupport cannot inspect /dev/ppp on this platform.
func CheckOriginalPPPSupport() error {
	return errors.New("ppp: original-resource support requires Linux /dev/ppp; run Ze on the matching runtime kernel and inspect ze show doctor")
}
