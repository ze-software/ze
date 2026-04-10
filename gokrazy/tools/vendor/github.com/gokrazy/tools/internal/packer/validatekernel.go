package packer

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gokrazy/tools/packer"
)

// kernelGoarch returns the GOARCH value that corresponds to the provided
// vmlinuz header. It returns one of "arm", "arm64", "386", "amd64" or the empty
// string if not detected.
func kernelGoarch(hdr []byte) string {
	// Some constants from the file(1) command's magic.
	const (
		// 32-bit arm: https://github.com/file/file/blob/65be1904/magic/Magdir/linux#L238-L241
		arm32Magic       = 0x016f2818
		arm32MagicOffset = 0x24
		// 64-bit arm: https://github.com/file/file/blob/65be1904/magic/Magdir/linux#L253-L254
		arm64Magic       = 0x644d5241
		arm64MagicOffset = 0x38
		// x86: https://github.com/file/file/blob/65be1904/magic/Magdir/linux#L137-L152
		x86Magic            = 0xaa55
		x86MagicOffset      = 0x1fe
		x86XloadflagsOffset = 0x236
	)
	if len(hdr) >= arm64MagicOffset+4 && binary.LittleEndian.Uint32(hdr[arm64MagicOffset:]) == arm64Magic {
		return "arm64"
	}
	if len(hdr) >= arm32MagicOffset+4 && binary.LittleEndian.Uint32(hdr[arm32MagicOffset:]) == arm32Magic {
		return "arm"
	}
	if len(hdr) >= x86XloadflagsOffset+2 && binary.LittleEndian.Uint16(hdr[x86MagicOffset:]) == x86Magic {
		// XLF0 in arch/x86/boot/header.S
		if hdr[x86XloadflagsOffset]&1 != 0 {
			return "amd64"
		} else {
			return "386"
		}
	}
	return ""
}

// validateTargetArchMatchesKernel validates that the packer.TargetArch
// corresponds to the kernel's architecture.
//
// See https://github.com/gokrazy/gokrazy/issues/191 for background. Maybe the
// TargetArch will become automatic in the future but for now this is a safety
// net to prevent people from bricking their appliances with the wrong userspace
// architecture.
func (pack *Pack) validateTargetArchMatchesKernel() error {
	cfg := pack.Cfg
	kernelDir, err := packer.PackageDir(cfg.KernelPackageOrDefault())
	if err != nil {
		return err
	}
	kernelPath := filepath.Join(kernelDir, "vmlinuz")
	k, err := os.Open(kernelPath)
	if err != nil {
		return err
	}
	defer k.Close()
	hdr := make([]byte, 1<<10) // plenty
	if _, err := io.ReadFull(k, hdr); err != nil {
		return err
	}
	kernelArch := kernelGoarch(hdr)
	if kernelArch == "" {
		return fmt.Errorf("kernel %v architecture in %s not detected", cfg.KernelPackageOrDefault(), kernelPath)
	}
	targetArch := packer.TargetArch()
	if kernelArch != targetArch {
		return fmt.Errorf("target architecture %q (GOARCH) doesn't match the %s kernel type %q",
			targetArch,
			cfg.KernelPackageOrDefault(),
			kernelArch)
	}
	return nil
}
