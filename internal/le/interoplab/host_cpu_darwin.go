//go:build darwin

// Design: docs/architecture/testing/interop.md -- the persistent image cache.
// Related: host_cpu.go -- the Linux identity this one mirrors.
package interoplab

import (
	"errors"
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// darwinCPUBrand is the sysctl naming the processor model ("Apple M2 Max",
// "Intel(R) Core(TM) i9-9980HK ..."). It is required.
const darwinCPUBrand = "machdep.cpu.brand_string"

// darwinCPUFeatureNames are the sysctls naming an Intel Mac's instruction
// sets. Apple silicon has none of them, so an absent one is recorded as
// absent rather than refused.
var darwinCPUFeatureNames = []string{"machdep.cpu.features", "machdep.cpu.leaf7_features", "machdep.cpu.extfeatures"}

// darwinCPUFamilyNames are the sysctls naming the processor family and
// subfamily as numbers: on Apple silicon they tell one generation's cores,
// and so its instruction set, from another's.
var darwinCPUFamilyNames = []string{"hw.cpufamily", "hw.cpusubfamily"}

// hostCPUIdentity answers this machine's CPU identity from sysctl: the model,
// the family numbers, and on an Intel Mac the feature lists.
func hostCPUIdentity() (string, error) {
	var tb textbuf.Buffer
	brand, err := unix.Sysctl(darwinCPUBrand)
	if err != nil {
		return "", fmt.Errorf("sysctl %s: %w", darwinCPUBrand, err)
	}
	if brand == "" {
		return "", errors.New("sysctl " + darwinCPUBrand + " is empty")
	}
	tb.Str(darwinCPUBrand).Byte('=').Str(brand).Byte('\n')
	for _, name := range darwinCPUFamilyNames {
		value, err := unix.SysctlUint32(name)
		if errors.Is(err, syscall.ENOENT) {
			tb.Str(name).Str(" absent\n")
			continue
		}
		if err != nil {
			return "", fmt.Errorf("sysctl %s: %w", name, err)
		}
		tb.Str(name).Byte('=').Uint32(value).Byte('\n')
	}
	for _, name := range darwinCPUFeatureNames {
		value, err := unix.Sysctl(name)
		if errors.Is(err, syscall.ENOENT) {
			tb.Str(name).Str(" absent\n")
			continue
		}
		if err != nil {
			return "", fmt.Errorf("sysctl %s: %w", name, err)
		}
		tb.Str(name).Byte('=').Str(value).Byte('\n')
	}
	return tb.String(), nil
}
