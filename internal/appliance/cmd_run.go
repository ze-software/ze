// Design: plan/learned/675-appliance-1-builder.md — QEMU boot with port conflict detection

package appliance

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var errPortAlreadyInUse = errors.New("port already in use")

func init() {
	cmdRun = runRun
}

var qemuAccel = resolveQEMUAccel()

func resolveQEMUAccel() string {
	if v := os.Getenv("GOKRAZY_QEMU_ACCEL"); v != "" {
		return v
	}
	return "tcg"
}

func runRun(args []string) int {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: ze appliance run <name>\n")
		return exitError
	}
	name := args[0]
	dir := getBaseDir()

	cfg, err := LoadConfig(ConfigPath(dir, name))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	ports := map[string]int{
		"ssh":     cfg.QEMU.SSHPort,
		"web":     cfg.QEMU.WebPort,
		"gokrazy": cfg.QEMU.GokrazyPort,
	}
	for label, port := range ports {
		if port == 0 {
			continue
		}
		if err := checkPortAvailable(port); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s port %d: %v\n", label, port, err)
			return exitError
		}
	}

	imgPath := findLatestImage(dir, name)
	if imgPath == "" {
		fmt.Fprintf(os.Stderr, "error: no image found for %s (run: ze appliance build %s)\n", name, name)
		return exitError
	}

	return launchQEMU(cfg, imgPath)
}

func findLatestImage(dir, name string) string {
	appDir := AppliancePath(dir, name)
	entries, err := os.ReadDir(appDir)
	if err != nil {
		return ""
	}
	var latest string
	for _, e := range entries {
		n := e.Name()
		if len(n) > 4 && n[:3] == "ze-" && n[len(n)-4:] == ".img" {
			if n > latest {
				latest = n
			}
		}
	}
	if latest == "" {
		return ""
	}
	return filepath.Join(appDir, latest)
}

func launchQEMU(cfg *applianceConfig, imgPath string) int {
	qemuBin, qemuArgs := buildQEMUCommand(cfg, imgPath)

	if _, err := exec.LookPath(qemuBin); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s not found (brew install qemu)\n", qemuBin)
		return exitError
	}

	fmt.Fprintf(os.Stderr, "Booting Ze gokrazy appliance...\n")
	if cfg.QEMU.WebPort != 0 {
		fmt.Fprintf(os.Stderr, "  Ze web:      https://localhost:%d/\n", cfg.QEMU.WebPort)
	}
	if cfg.QEMU.GokrazyPort != 0 {
		fmt.Fprintf(os.Stderr, "  Gokrazy:     https://localhost:%d/gokrazy/\n", cfg.QEMU.GokrazyPort)
	}
	if cfg.QEMU.SSHPort != 0 {
		fmt.Fprintf(os.Stderr, "  Ze SSH:      ssh -p %d admin@localhost\n", cfg.QEMU.SSHPort)
	}
	fmt.Fprintf(os.Stderr, "  Quit:        Ctrl-A X\n\n")

	cmd := exec.CommandContext(context.Background(), qemuBin, qemuArgs...) //nolint:gosec // user-initiated VM launch
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: qemu: %v\n", err)
		return exitError
	}
	return exitOK
}

func buildQEMUCommand(cfg *applianceConfig, imgPath string) (string, []string) {
	var tb textbuf.Buffer
	var hostfwds []string
	if cfg.QEMU.WebPort != 0 {
		hostfwds = append(hostfwds, tb.Reset().Str("hostfwd=tcp::").Int(int64(cfg.QEMU.WebPort)).Str("-:").Str(cfg.Web.Port).String())
	}
	if cfg.QEMU.SSHPort != 0 {
		hostfwds = append(hostfwds, tb.Reset().Str("hostfwd=tcp::").Int(int64(cfg.QEMU.SSHPort)).Str("-:").Str(cfg.SSH.Port).String())
	}
	if cfg.QEMU.GokrazyPort != 0 {
		hostfwds = append(hostfwds, tb.Reset().Str("hostfwd=tcp::").Int(int64(cfg.QEMU.GokrazyPort)).Str("-:443").String())
	}

	fwdStr := ""
	if len(hostfwds) > 0 {
		fwdStr = tb.Reset().Byte(',').Join(hostfwds, ",").String()
	}

	mem := qemuMemoryMiB(cfg.Image)

	switch cfg.Image.Arch {
	case archARM64:
		bios := qemuAARCH64Firmware()
		cpuModel := os.Getenv("GOKRAZY_QEMU_AARCH64_CPU")
		if cpuModel == "" {
			cpuModel = "max"
		}
		return "qemu-system-aarch64", []string{
			"-machine", tb.Reset().Str("virt,highmem=off,accel=").Str(qemuAccel).String(),
			"-cpu", cpuModel,
			"-smp", "2", "-m", mem,
			"-bios", bios,
			"-drive", tb.Reset().Str("file=").Str(imgPath).Str(",format=raw").String(),
			"-nographic", "-serial", "mon:stdio",
			"-netdev", tb.Reset().Str("user,id=net0").Str(fwdStr).String(),
			"-device", "e1000,netdev=net0",
		}
	default:
		return "qemu-system-x86_64", []string{
			"-machine", tb.Reset().Str("accel=").Str(qemuAccel).String(),
			"-smp", "2", "-m", mem,
			"-drive", tb.Reset().Str("file=").Str(imgPath).Str(",format=raw").String(),
			"-nographic", "-serial", "mon:stdio",
			"-nic", tb.Reset().Str("user,model=e1000").Str(fwdStr).String(),
		}
	}
}

// qemuMemoryMiB returns the QEMU `-m` size in MiB (as a string) for the image.
// When image.memory-bytes is set it derives from that so evidence and operators
// can reproduce the reservation shape; otherwise it keeps today's 512 MiB
// default (AC-13).
func qemuMemoryMiB(img ImageConfig) string {
	memBytes, ok, err := img.memoryBytes()
	if !ok || err != nil {
		return "512"
	}
	mib := max(memBytes/(1024*1024), 1)
	return textbuf.StringInt(mib)
}

func checkPortAvailable(port int) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", textbuf.HostPort("127.0.0.1", uint16(port)))
	if err != nil {
		return errPortAlreadyInUse
	}
	ln.Close() //nolint:errcheck // probe only
	return nil
}
