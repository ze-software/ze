// Design: docs/architecture/provisioning/pxe-staging.md -- PXE artifact staging and validation

package provision

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// defaultPXEDir is the consolidated build-output root for PXE artifacts.
	// `make ze-pxe-build` (mk/appliance.mk PXE_DIR) stages the kernel, initrd, and
	// iPXE binaries here, and provision serves boot files from <pxe-dir>/boot
	// and TFTP from <pxe-dir>/tftp. Keep the three literals in sync with
	// PXE_DIR in mk/appliance.mk; --pxe-dir overrides the root (e.g.
	// /var/lib/ze/install for a system install outside the repo checkout).
	defaultPXEDir  = "build/pxe"
	defaultTFTPDir = "build/pxe/tftp"
	defaultBootDir = "build/pxe/boot"
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

// pxeDirs turns a --pxe-dir root into absolute boot and TFTP serve directories.
// Absolute so the generated ze config does not depend on the forked server's
// working directory. filepath.Abs only fails when the working directory cannot
// be read; in that case the literal root is used so callers still get a usable
// (relative) path rather than an empty one. validateFlags gates the result for
// config safety before it reaches the generated config.
func pxeDirs(pxeDir string) (bootDir, tftpDir string) {
	if pxeDir == "" {
		pxeDir = defaultPXEDir
	}
	root := pxeDir
	if abs, err := filepath.Abs(pxeDir); err == nil {
		root = abs
	}
	return filepath.Join(root, "boot"), filepath.Join(root, "tftp")
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
