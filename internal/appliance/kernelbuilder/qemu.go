package kernelbuilder

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	alpineVersion     = "3.21"
	alpineMinor       = "3"
	vmMemoryMin       = 9216
	vmMemoryMax       = 12288
	bootTimeout       = 120 * time.Second
	buildTimeout      = 4 * time.Hour
	bootstrapTimeout  = 180 * time.Second
	sshTimeout        = 60 * time.Second
	bootstrapAttempts = 5
	buildPackages     = "build-base bc bison flex elfutils-dev openssl-dev linux-headers perl wget xz diffutils findutils cpio patch kmod zstd"
)

var alpineReleaseBaseURL = "https://dl-cdn.alpinelinux.org/alpine"

func runQEMU(ctx context.Context, req Request) error {
	arch := alpineArch(req.Arch)
	iso, err := ensureAlpineISO(ctx, req, arch)
	if err != nil {
		return err
	}
	workerRel, err := buildGuestWorker(ctx, req)
	if err != nil {
		return err
	}
	return runQEMUBuild(ctx, req, iso, workerRel)
}

func alpineArch(arch string) string {
	if arch == archARM64 {
		return archAArch64
	}
	return archX8664
}
func qemuBinary(arch string) string {
	if arch == archARM64 || arch == archAArch64 {
		return "qemu-system-aarch64"
	}
	return "qemu-system-x86_64"
}

func ensureAlpineISO(ctx context.Context, req Request, arch string) (string, error) {
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		cacheHome = filepath.Join(home, ".cache")
	}
	cache := filepath.Join(cacheHome, "ze", "alpine-iso")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return "", fmt.Errorf("create Alpine ISO cache: %w", err)
	}
	name := fmt.Sprintf("alpine-virt-%s.%s-%s.iso", alpineVersion, alpineMinor, arch)
	iso := filepath.Join(cache, name)
	sidecar := iso + ".sha256"
	if expected, err := readChecksum(sidecar); err == nil {
		actual, hashErr := fileSHA256(iso)
		if hashErr == nil && actual == expected {
			return iso, nil
		}
		_ = os.Remove(iso)
		_ = os.Remove(sidecar)
	}
	url := fmt.Sprintf("%s/v%s/releases/%s/%s", alpineReleaseBaseURL, alpineVersion, arch, name)
	expected, err := fetchPublishedChecksum(ctx, url+".sha256")
	if err != nil {
		return "", err
	}
	part := iso + ".part"
	_ = os.Remove(part)
	if err := downloadFile(ctx, url, part); err != nil {
		_ = os.Remove(part)
		return "", err
	}
	actual, err := fileSHA256(part)
	if err != nil {
		_ = os.Remove(part)
		return "", err
	}
	if actual != expected {
		_ = os.Remove(part)
		return "", fmt.Errorf("Alpine ISO checksum mismatch for %s: got %s, want %s", name, actual, expected)
	}
	if err := os.Rename(part, iso); err != nil {
		_ = os.Remove(part)
		return "", fmt.Errorf("publish Alpine ISO: %w", err)
	}
	if err := os.WriteFile(sidecar, []byte(fmt.Sprintf("%s  %s\n", expected, name)), 0o644); err != nil {
		return "", fmt.Errorf("write Alpine ISO checksum: %w", err)
	}
	return iso, nil
}

func fetchPublishedChecksum(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download Alpine checksum: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return "", fmt.Errorf("download Alpine checksum: HTTP %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 || len(fields[0]) != 64 {
		return "", fmt.Errorf("malformed Alpine checksum from %s", url)
	}
	if _, err := hex.DecodeString(fields[0]); err != nil {
		return "", fmt.Errorf("malformed Alpine checksum from %s", url)
	}
	return strings.ToLower(fields[0]), nil
}

func downloadFile(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("download %s: HTTP %s", url, response.Status)
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, response.Body)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		return errors.Join(copyErr, closeErr)
	}
	return nil
}
func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
func readChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 || len(fields[0]) != 64 {
		return "", errors.New("malformed checksum")
	}
	if _, err := hex.DecodeString(fields[0]); err != nil {
		return "", err
	}
	return strings.ToLower(fields[0]), nil
}

func buildGuestWorker(ctx context.Context, req Request) (string, error) {
	rel := filepath.Join("tmp", backendQEMU, "bin", workerGOARCH(req.Arch), "ze-kernel-builder")
	path := filepath.Join(req.Root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "go", "build", "-o", path, "./tools/kernel-builder") //nolint:gosec // fixed Go package and validated output
	cmd.Dir = req.Root
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+workerGOARCH(req.Arch), "CGO_ENABLED=0")
	cmd.Stdout, cmd.Stderr = req.Stdout, req.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("build QEMU kernel worker: %w", err)
	}
	return filepath.ToSlash(rel), nil
}

func runQEMUBuild(ctx context.Context, req Request, iso, workerRel string) error {
	port, err := freeTCPPort()
	if err != nil {
		return err
	}
	ccache := filepath.Join(req.Root, "tmp", backendQEMU, "ccache")
	build := filepath.Join(req.Root, "tmp", backendQEMU, "build", alpineArch(req.Arch))
	for _, dir := range []string{ccache, build, hostOutputPath(req)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	memory := vmMemoryMiB(ctx)
	args, err := qemuArgs(ctx, req, iso, port, memory, ccache, build)
	if err != nil {
		return err
	}
	fmt.Fprintf(req.Stderr, ">>> booting Alpine VM (%s, %dMB RAM, ssh port %d)...\n", alpineArch(req.Arch), memory, port) //nolint:errcheck // progress output
	cmd := exec.CommandContext(ctx, qemuBinary(req.Arch), args...)                                                        //nolint:gosec // executable selected from a fixed architecture map
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start QEMU: %w", err)
	}
	console := newConsole(stdout)
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	if !console.expect(ctx, "login:", bootTimeout) {
		return errors.New("timeout waiting for VM login prompt")
	}
	if !sleepContext(ctx, time.Second) {
		return ctx.Err()
	}
	if err := sendConsole(stdin, "root"); err != nil {
		return err
	}
	if !sleepContext(ctx, 3*time.Second) {
		return ctx.Err()
	}
	bootstrap := "setup-interfaces -a 2>/dev/null; ifup eth0 2>/dev/null; ifup lo 2>/dev/null; echo nameserver 8.8.8.8 > /etc/resolv.conf; apk add --no-cache openssh; echo PermitRootLogin yes >> /etc/ssh/sshd_config; echo PermitEmptyPasswords yes >> /etc/ssh/sshd_config; passwd -d root; ssh-keygen -t ed25519 -f /etc/ssh/ssh_host_ed25519_key -N '' 2>/dev/null; ssh-keygen -t rsa -f /etc/ssh/ssh_host_rsa_key -N '' 2>/dev/null; /usr/sbin/sshd; echo SSHD_READY"
	ready := false
	for attempt := 1; attempt <= bootstrapAttempts; attempt++ {
		if err := sendConsole(stdin, bootstrap); err != nil {
			return err
		}
		fmt.Fprintf(req.Stderr, "  bootstrapping VM, attempt %d...\n", attempt) //nolint:errcheck // progress output
		if !console.expect(ctx, "SSHD_READY", bootstrapTimeout) {
			continue
		}
		fmt.Fprintln(req.Stderr, "  waiting for SSH...") //nolint:errcheck // progress output
		if waitForSSH(ctx, port, sshTimeout) == nil {
			ready = true
			break
		}
		fmt.Fprintln(req.Stderr, "  SSH not up; retrying...") //nolint:errcheck // progress output
		if !sleepContext(ctx, 2*time.Second) {
			return ctx.Err()
		}
	}
	if !ready {
		return errors.New("VM bootstrap failed: SSH not reachable")
	}
	fmt.Fprintln(req.Stderr, "  VM ready, installing build dependencies...") //nolint:errcheck // progress output
	setup := fmt.Sprintf("set -e && printf 'https://dl-cdn.alpinelinux.org/alpine/v%s/main\\nhttps://dl-cdn.alpinelinux.org/alpine/v%s/community\\n' > /etc/apk/repositories && apk update && apk add --no-cache %s ccache && mkdir -p /workspace && mount -t 9p -o trans=virtio,version=9p2000.L,msize=1048576 workspace /workspace && mkdir -p /ccache && mount -t 9p -o trans=virtio,version=9p2000.L,msize=1048576 ccache /ccache && mkdir -p /build && mount -t 9p -o trans=virtio,version=9p2000.L,msize=1048576 builddir /build", alpineVersion, alpineVersion, buildPackages)
	guestOutput := "/workspace/" + filepath.ToSlash(req.OutputDir)
	if filepath.IsAbs(req.OutputDir) {
		setup += " && mkdir -p /output && mount -t 9p -o trans=virtio,version=9p2000.L,msize=1048576 output /output"
		guestOutput = "/output"
	}
	if req.FirmwareDir != "" {
		setup += " && mkdir -p /firmware && mount -t 9p -o trans=virtio,version=9p2000.L,msize=1048576,ro firmware /firmware"
	}
	workerArgs := []string{"/workspace/" + workerRel, "--version", req.Version, "--arch", req.Arch, "--profile", req.Profile, "--src-dir", "/workspace/" + filepath.ToSlash(req.SourceDir), "--out-dir", guestOutput, "--modules", req.Modules}
	for _, fragment := range req.Fragments {
		workerArgs = append(workerArgs, "--fragment", "/workspace/"+filepath.ToSlash(fragment))
	}
	if req.PatchesDir != "" {
		workerArgs = append(workerArgs, "--patches-dir", "/workspace/"+filepath.ToSlash(req.PatchesDir))
	}
	if req.FirmwareDir != "" {
		workerArgs = append(workerArgs, "--firmware-dir", "/firmware")
	}
	if req.Jobs != "" {
		workerArgs = append(workerArgs, "--jobs", req.Jobs)
	}
	fullCommand := "sh -c " + shellQuote(setup+" && CCACHE_DIR=/ccache CCACHE_MAXSIZE=5G PATH=/usr/lib/ccache/bin:$PATH "+shellJoin(workerArgs))
	tarball := filepath.Join(build, kernelTarballName(req.Version))
	if !regularFile(tarball) {
		fmt.Fprintf(req.Stderr, "  downloading %s on host...\n", filepath.Base(tarball)) //nolint:errcheck // progress output
		if err := downloadFile(ctx, kernelTarballURL(req.Version), tarball); err != nil {
			_ = os.Remove(tarball)
			return fmt.Errorf("kernel tarball download failed: %w", err)
		}
	} else {
		fmt.Fprintf(req.Stderr, "  %s cached on host\n", filepath.Base(tarball))
	} //nolint:errcheck // progress output
	fmt.Fprintf(req.Stderr, "  building kernel (version=%s, arch=%s, profile=%s)...\n", req.Version, req.Arch, req.Profile) //nolint:errcheck // progress output
	buildCtx, cancel := context.WithTimeout(ctx, buildTimeout)
	defer cancel()
	if err := sshRun(buildCtx, port, fullCommand, req); err != nil {
		return fmt.Errorf("kernel build failed: %w", err)
	}
	fmt.Fprintln(req.Stderr, ">>> kernel build complete") //nolint:errcheck // progress output
	return nil
}

func qemuArgs(ctx context.Context, req Request, iso string, port, memory int, ccache, build string) ([]string, error) {
	args := make([]string, 0, 40)
	if req.Arch == archARM64 {
		firmware, err := findAArch64Firmware()
		if err != nil {
			return nil, err
		}
		args = append(args, "-machine", "virt,highmem=on", "-cpu", "max", "-bios", firmware)
	}
	accels := availableAccelerators(ctx, qemuBinary(req.Arch))
	for _, accel := range []string{"hvf", "kvm"} {
		if accels[accel] {
			args = append(args, "-accel", accel)
		}
	}
	if accels["tcg"] {
		args = append(args, "-accel", "tcg,thread=multi,tb-size=512")
	}
	args = append(args, "-smp", strconv.Itoa(max(2, runtime.NumCPU())), "-m", strconv.Itoa(memory), "-cdrom", iso, "-boot", "d", "-nographic", "-serial", "mon:stdio", "-netdev", fmt.Sprintf("user,id=net0,hostfwd=tcp::%d-:22", port), "-device", "virtio-net-pci,netdev=net0", "-virtfs", fmt.Sprintf("local,path=%s,mount_tag=workspace,security_model=none,id=ws0,readonly=off", req.Root), "-virtfs", fmt.Sprintf("local,path=%s,mount_tag=ccache,security_model=none,id=cc0,readonly=off", ccache), "-virtfs", fmt.Sprintf("local,path=%s,mount_tag=builddir,security_model=none,id=bd0,readonly=off", build))
	if filepath.IsAbs(req.OutputDir) {
		args = append(args, "-virtfs", fmt.Sprintf("local,path=%s,mount_tag=output,security_model=none,id=out0,readonly=off", req.OutputDir))
	}
	if req.FirmwareDir != "" {
		args = append(args, "-virtfs", fmt.Sprintf("local,path=%s,mount_tag=firmware,security_model=none,id=fw0,readonly=on", req.FirmwareDir))
	}
	return args, nil
}

func availableAccelerators(ctx context.Context, binary string) map[string]bool {
	result := make(map[string]bool)
	output, err := exec.CommandContext(ctx, binary, "-accel", "help").Output()
	if err != nil {
		return result
	}
	known := map[string]bool{"hvf": true, "kvm": true, "tcg": true, "whpx": true, "xen": true}
	for _, line := range strings.Split(string(output), "\n") {
		token := strings.TrimSpace(line)
		if known[token] {
			result[token] = true
		}
	}
	return result
}
func findAArch64Firmware() (string, error) {
	prefixes := []string{}
	if prefix := os.Getenv("HOMEBREW_PREFIX"); prefix != "" {
		prefixes = append(prefixes, prefix)
	}
	if brew, err := exec.LookPath("brew"); err == nil {
		prefixes = append(prefixes, filepath.Dir(filepath.Dir(brew)))
	}
	prefixes = append(prefixes, "/opt/homebrew", "/usr/local")
	candidates := []string{}
	for _, prefix := range prefixes {
		candidates = append(candidates, filepath.Join(prefix, "share/qemu/edk2-aarch64-code.fd"))
	}
	candidates = append(candidates, "/usr/share/qemu/edk2-aarch64-code.fd", "/usr/share/AAVMF/AAVMF_CODE.fd", "/usr/share/edk2/aarch64/QEMU_EFI-pflash.raw")
	for _, path := range candidates {
		if regularFile(path) {
			return path, nil
		}
	}
	return "", errors.New("aarch64 UEFI firmware not found; install QEMU with firmware (brew install qemu) or qemu-efi-aarch64 on Debian/Ubuntu")
}
func freeTCPPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	address := listener.Addr()
	tcpAddress, ok := address.(*net.TCPAddr)
	if !ok {
		_ = listener.Close()
		return 0, fmt.Errorf("unexpected TCP listener address type %T", address)
	}
	return tcpAddress.Port, listener.Close()
}
func vmMemoryMiB(ctx context.Context) int {
	total := int64(0)
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					kb, _ := strconv.ParseInt(fields[1], 10, 64)
					total = kb * 1024
				}
				break
			}
		}
	}
	if total == 0 && runtime.GOOS == "darwin" {
		if output, err := exec.CommandContext(ctx, "sysctl", "-n", "hw.memsize").Output(); err == nil {
			total, _ = strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
		}
	}
	quarter := int(total / 4 / (1024 * 1024))
	if quarter < vmMemoryMin {
		return vmMemoryMin
	}
	if quarter > vmMemoryMax {
		return vmMemoryMax
	}
	return quarter
}

func sshOptions(port int) []string {
	return []string{"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-o", "PreferredAuthentications=none", "-o", "LogLevel=ERROR", "-p", strconv.Itoa(port)}
}
func waitForSSH(ctx context.Context, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		attempt, cancel := context.WithTimeout(ctx, 2*time.Second)
		args := append(sshOptions(port), "-o", "ConnectTimeout=2", "root@localhost", "true")
		cmd := exec.CommandContext(attempt, "ssh", args...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, io.Discard, io.Discard
		err := cmd.Run()
		cancel()
		if err == nil {
			return nil
		}
		if !sleepContext(ctx, 2*time.Second) {
			return ctx.Err()
		}
	}
	return fmt.Errorf("SSH not reachable on port %d after %s", port, timeout)
}
func sshRun(ctx context.Context, port int, command string, req Request) error {
	args := append(sshOptions(port), "-o", "ServerAliveInterval=30", "root@localhost", command)
	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdout, cmd.Stderr = req.Stdout, req.Stderr
	return cmd.Run()
}
func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for index, arg := range args {
		quoted[index] = shellQuote(arg)
	}
	return strings.Join(quoted, " ")
}
func sendConsole(writer io.Writer, command string) error {
	_, err := io.WriteString(writer, command+"\n")
	return err
}
func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

type consoleReader struct {
	chunks chan []byte
	errors chan error
	tail   string
}

func newConsole(reader io.Reader) *consoleReader {
	console := &consoleReader{chunks: make(chan []byte, 32), errors: make(chan error, 1)}
	go func() {
		buffered := bufio.NewReader(reader)
		for {
			chunk := make([]byte, 4096)
			count, err := buffered.Read(chunk)
			if count > 0 {
				console.chunks <- chunk[:count]
			}
			if err != nil {
				console.errors <- err
				close(console.chunks)
				return
			}
		}
	}()
	return console
}
func (reader *consoleReader) expect(ctx context.Context, pattern string, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	buffer := reader.tail
	for {
		if strings.Contains(buffer, pattern) {
			if len(buffer) > 10000 {
				buffer = buffer[len(buffer)-10000:]
			}
			reader.tail = buffer
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			return false
		case chunk, ok := <-reader.chunks:
			if !ok {
				return false
			}
			buffer += string(chunk)
			if len(buffer) > 20000 {
				buffer = buffer[len(buffer)-10000:]
			}
		}
	}
}
