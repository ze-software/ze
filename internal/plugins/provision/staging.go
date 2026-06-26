// Design: plan/learned/851-install-10-pxe-staging.md -- PXE artifact staging and validation

package provision

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultTFTPDir = "/var/lib/ze/install/tftp"
	defaultBootDir = "/var/lib/ze/install/boot"
	// stagedKernelName is the boot-directory filename the PXE kernel is staged
	// as. It must stay "vmlinuz" so iPXE/GRUB configs and the appliance build
	// pipeline find it; this is the provision package's own constant, distinct
	// from the appliance build-output name "Image" (internal/appliance/cache.go).
	stagedKernelName = "vmlinuz"
	stagedInitrdName = "initrd.img.gz"
)

var ipxeBinaries = []string{"ipxe.pxe", "ipxe.efi"}

var bootArtifactNames = []string{stagedKernelName, stagedInitrdName}

type stagingConfig struct {
	KernelPath string
	InitrdPath string
	IPXEDir    string
	TFTPDir    string
	BootDir    string
}

func (c *stagingConfig) tftpDir() string {
	if c.TFTPDir != "" {
		return c.TFTPDir
	}
	return defaultTFTPDir
}

func (c *stagingConfig) bootDir() string {
	if c.BootDir != "" {
		return c.BootDir
	}
	return defaultBootDir
}

func stageArtifacts(cfg stagingConfig) error {
	td, bd := cfg.tftpDir(), cfg.bootDir()
	for _, dir := range []string{td, bd} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	if cfg.KernelPath != "" {
		if err := copyFileIfRegular(cfg.KernelPath, filepath.Join(bd, stagedKernelName)); err != nil {
			return fmt.Errorf("stage kernel: %w", err)
		}
	}
	if cfg.InitrdPath != "" {
		if err := copyFileIfRegular(cfg.InitrdPath, filepath.Join(bd, stagedInitrdName)); err != nil {
			return fmt.Errorf("stage initrd: %w", err)
		}
	}

	if cfg.IPXEDir != "" {
		for _, name := range ipxeBinaries {
			src := filepath.Join(cfg.IPXEDir, name)
			dst := filepath.Join(td, name)
			if _, err := os.Stat(dst); err == nil {
				continue
			}
			if err := copyFileIfRegular(src, dst); err != nil {
				return fmt.Errorf("stage iPXE binary %s: %w", name, err)
			}
		}
	}

	return nil
}

func validateStaging(cfg stagingConfig) error {
	td, bd := cfg.tftpDir(), cfg.bootDir()
	var missing []string

	for _, name := range bootArtifactNames {
		path := filepath.Join(bd, name)
		if _, err := os.Stat(path); err != nil {
			missing = append(missing, path)
		}
	}

	for _, name := range ipxeBinaries {
		path := filepath.Join(td, name)
		if _, err := os.Stat(path); err != nil {
			missing = append(missing, path)
		}
	}

	if len(missing) > 0 {
		return &missingArtifactsError{paths: missing}
	}
	return nil
}

type missingArtifactsError struct {
	paths []string
}

func (e *missingArtifactsError) Error() string {
	var b strings.Builder
	b.WriteString("missing required boot artifacts:")
	for _, p := range e.paths {
		b.WriteString("\n  ")
		b.WriteString(p)
	}
	return b.String()
}

func locateIPXEDir() string {
	self, err := os.Executable()
	if err != nil {
		return ""
	}
	candidate := filepath.Join(filepath.Dir(filepath.Dir(self)), "tools", "ipxe-binaries")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate
	}
	return ""
}

func copyFileIfRegular(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	in, err := os.Open(src) //nolint:gosec // src is validated above
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // read-only

	out, err := os.Create(dst) //nolint:gosec // dst is a controlled staging path
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close() //nolint:errcheck // cleanup
		return err
	}

	return out.Close()
}
