// Design: plan/learned/675-appliance-1-builder.md — QEMU boot with port conflict detection

package appliance

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

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

func launchQEMU(cfg *ApplianceConfig, imgPath string) int {
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

func buildQEMUCommand(cfg *ApplianceConfig, imgPath string) (string, []string) {
	var hostfwds []string
	if cfg.QEMU.WebPort != 0 {
		hostfwds = append(hostfwds, fmt.Sprintf("hostfwd=tcp::%d-:8080", cfg.QEMU.WebPort))
	}
	if cfg.QEMU.SSHPort != 0 {
		hostfwds = append(hostfwds, fmt.Sprintf("hostfwd=tcp::%d-:22", cfg.QEMU.SSHPort))
	}
	if cfg.QEMU.GokrazyPort != 0 {
		hostfwds = append(hostfwds, fmt.Sprintf("hostfwd=tcp::%d-:443", cfg.QEMU.GokrazyPort))
	}

	fwdStr := ""
	if len(hostfwds) > 0 {
		fwdStr = "," + strings.Join(hostfwds, ",")
	}

	switch cfg.Image.Arch {
	case archARM64:
		bios := os.Getenv("GOKRAZY_QEMU_AARCH64_BIOS")
		if bios == "" {
			bios = "/opt/homebrew/share/qemu/edk2-aarch64-code.fd"
		}
		cpuModel := os.Getenv("GOKRAZY_QEMU_AARCH64_CPU")
		if cpuModel == "" {
			cpuModel = "max"
		}
		return "qemu-system-aarch64", []string{
			"-machine", "virt,highmem=off,accel=" + qemuAccel,
			"-cpu", cpuModel,
			"-smp", "2", "-m", "512",
			"-bios", bios,
			"-drive", "file=" + imgPath + ",format=raw",
			"-nographic", "-serial", "mon:stdio",
			"-netdev", "user,id=net0" + fwdStr,
			"-device", "e1000,netdev=net0",
		}
	default:
		return "qemu-system-x86_64", []string{
			"-machine", "accel=" + qemuAccel,
			"-smp", "2", "-m", "512",
			"-drive", "file=" + imgPath + ",format=raw",
			"-nographic", "-serial", "mon:stdio",
			"-nic", "user,model=e1000" + fwdStr,
		}
	}
}

func checkPortAvailable(port int) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return fmt.Errorf("port already in use")
	}
	ln.Close() //nolint:errcheck // probe only
	return nil
}
