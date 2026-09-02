// Design: docs/architecture/testing/qemu-integration.md -- installer VM lifecycle
// Overview: install.go -- the shared installer runner
// Related: install_build.go -- artifacts this lifecycle boots
package qemu

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/core/textbuf"
)

type installHTTPServer struct {
	server *http.Server
	done   chan error
}

func startInstallHTTP(ctx context.Context, served string) (*installHTTPServer, int, error) {
	listenConfig := net.ListenConfig{}
	// #nosec G102 -- QEMU's slirp guest reaches this ephemeral host listener through 10.0.2.2.
	listener, err := listenConfig.Listen(ctx, "tcp", "0.0.0.0:0")
	if err != nil {
		return nil, 0, fmt.Errorf("listen for installer HTTP service: %w", err)
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		closeErr := listener.Close()
		return nil, 0, errors.Join(errors.New("installer HTTP listener has non-TCP address"), closeErr)
	}
	mapping := map[string]string{
		"/install/image/" + InstallImageName:             filepath.Join(served, InstallImageName),
		"/install/image/" + InstallImageName + ".sha256": filepath.Join(served, InstallImageName+".sha256"),
		"/install/database.zefs":                         filepath.Join(served, "database.zefs"),
	}
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path, ok := mapping[request.URL.Path]
		if !ok {
			http.NotFound(writer, request)
			return
		}
		// #nosec G304 -- path comes only from the closed installer route-to-artifact mapping above.
		file, openErr := os.Open(path)
		if openErr != nil {
			http.NotFound(writer, request)
			return
		}
		info, statErr := file.Stat()
		if statErr != nil {
			// The stat error is primary. Record a read-only cleanup error separately.
			if closeErr := file.Close(); closeErr != nil {
				fmt.Fprintf(os.Stderr, "close installer HTTP artifact after stat failure: %v\n", closeErr) //nolint:errcheck // cleanup diagnostics
			}
			http.Error(writer, statErr.Error(), http.StatusInternalServerError)
			return
		}
		writer.Header().Set("Content-Type", "application/octet-stream")
		writer.Header().Set("Content-Length", textbuf.StringInt(info.Size()))
		writer.WriteHeader(http.StatusOK)
		if _, copyErr := io.Copy(writer, file); copyErr != nil {
			// A guest disconnect is the primary response outcome. Record cleanup separately.
			if closeErr := file.Close(); closeErr != nil {
				fmt.Fprintf(os.Stderr, "close installer HTTP artifact after guest disconnect: %v\n", closeErr) //nolint:errcheck // cleanup diagnostics
			}
			return
		}
		if closeErr := file.Close(); closeErr != nil {
			// The response is already committed, so only diagnostics can report this cleanup error.
			fmt.Fprintf(os.Stderr, "close installer HTTP artifact: %v\n", closeErr) //nolint:errcheck // cleanup diagnostics
		}
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	result := &installHTTPServer{server: server, done: make(chan error, 1)}
	go func() {
		result.done <- server.Serve(listener)
	}()
	return result, address.Port, nil
}

// Stop MUST be called for every server returned by startInstallHTTP.
func (server *installHTTPServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.server.Shutdown(ctx); err != nil {
		return err
	}
	select {
	case serveErr := <-server.done:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop installer HTTP server: %w", ctx.Err())
	}
}

type installSerial struct {
	mu sync.Mutex
	// text stays a strings.Builder. snapshot() reads it while the reader
	// goroutine keeps appending, and textbuf.Buffer.String() detaches the heap
	// slice above 128 bytes: every poll after the first would then answer with
	// the bytes read since the previous poll rather than the whole serial log,
	// so expect() would miss a needle that arrived earlier.
	text    strings.Builder
	changed chan struct{}
	failure chan error
}

func newInstallSerial(reader io.Reader) *installSerial {
	serial := &installSerial{changed: make(chan struct{}, 1), failure: make(chan error, 1)}
	go func() {
		buffer := make([]byte, serialReadSize)
		for {
			count, err := reader.Read(buffer)
			if count != 0 {
				serial.mu.Lock()
				serial.text.Write(buffer[:count])
				if serial.text.Len() > installSerialMax {
					text := serial.text.String()
					serial.text.Reset()
					serial.text.WriteString(text[len(text)-installSerialMax:])
				}
				serial.mu.Unlock()
				select {
				case serial.changed <- struct{}{}:
				default:
				}
			}
			if err != nil {
				serial.failure <- err
				return
			}
		}
	}()
	return serial
}

func (serial *installSerial) snapshot() string {
	serial.mu.Lock()
	defer serial.mu.Unlock()
	return serial.text.String()
}

func (serial *installSerial) expect(ctx context.Context, needle string, timeout time.Duration) (bool, error) {
	deadline, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		if strings.Contains(serial.snapshot(), needle) {
			return true, nil
		}
		select {
		case <-deadline.Done():
			if errors.Is(deadline.Err(), context.DeadlineExceeded) {
				return false, nil
			}
			return false, deadline.Err()
		case err := <-serial.failure:
			if errors.Is(err, io.EOF) {
				return strings.Contains(serial.snapshot(), needle), nil
			}
			return false, fmt.Errorf("read installer QEMU serial output: %w", err)
		case <-serial.changed:
		}
	}
}

func (installer *Installer) qemuBase(needsBIOS bool) []string {
	argv := []string{installer.qemuBinary(), "-smp", "2", "-m", "1024", "-nographic", "-serial", "mon:stdio"}
	var tb textbuf.Buffer
	if installer.Options.Arch == ArchARM64 {
		machine := tb.Str("virt,highmem=off,accel=").Str(installer.accelerator()).String()
		argv = append(argv, "-machine", machine, "-cpu", "max")
		if needsBIOS {
			bios := installer.Options.AArch64BIOS
			if bios == "" {
				found := (&Run{ops: installer.ops.runOps}).brewFiles("share/qemu/edk2-aarch64-code.fd")
				if len(found) != 0 {
					bios = found[0]
				} else {
					bios = InstallAArch64BIOSFallback
				}
			}
			argv = append(argv, "-bios", bios)
		}
		return argv
	}
	return append(argv, "-machine", tb.Str("accel=").Str(installer.accelerator()).String())
}

func (installer *Installer) runCapture(ctx context.Context, argv []string, timeout time.Duration) (output string, resultErr error) {
	// #nosec G204 -- argv is assembled exclusively by the installer's closed QEMU argument builders.
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	// stopVMWaiting owns the interrupt-then-kill bound; CommandContext must not kill QEMU first.
	command.Cancel = func() error { return nil }
	command.Env = installer.ops.Environ()
	reader, writer := io.Pipe()
	command.Stdout, command.Stderr = writer, writer
	if err := installer.ops.Start(command); err != nil {
		return "", errors.Join(
			fmt.Errorf("start %s: %w", argv[0], err),
			writer.Close(),
			reader.Close(),
		)
	}
	defer func() {
		resultErr = joinInstallCleanup(resultErr, reader.Close, "close installer QEMU serial reader")
	}()
	done := waitVM(command)
	defer stopVMWaiting(command, done)
	serial := newInstallSerial(reader)
	ended := make(chan error, 1)
	go func() {
		<-done
		ended <- writer.Close()
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		text := serial.snapshot()
		if strings.Contains(text, InstallMarkDone) || strings.Contains(text, InstallMarkISODone) {
			return text, nil
		}
		select {
		case <-ctx.Done():
			return serial.snapshot(), ctx.Err()
		case <-timer.C:
			return serial.snapshot(), nil
		case closeErr := <-ended:
			if closeErr != nil {
				return serial.snapshot(), fmt.Errorf("close installer QEMU serial writer: %w", closeErr)
			}
			return serial.snapshot(), nil
		case <-serial.changed:
		}
	}
}

// HTTPArgv returns the complete direct-kernel HTTP installer invocation.
func (installer *Installer) HTTPArgv(kernel, initrd, disk string, port int) ([]string, error) {
	base := installer.qemuBase(false)
	console := installConsoleAMD64
	if installer.Options.Arch == ArchARM64 {
		console = installConsoleARM64
	}
	var b textbuf.Buffer
	appendLine := b.Str("console=").Str(console).Str(" ze.server=").Str(InstallGuestServerIP).
		Str(" ze.port=").Int(int64(port)).Str(" ze.image=").Str(InstallImageName).
		Str(" ip=dhcp panic=-1").String()
	b.Reset()
	drive := b.Str("file=").Str(disk).Str(",format=raw,if=virtio").String()
	b.Reset()
	device := b.Str(installer.Options.NIC).Str(",netdev=net0").String()
	return append(base, "-kernel", kernel, "-initrd", initrd, "-append", appendLine,
		"-drive", drive, "-netdev", "user,id=net0", "-device", device), nil
}

func (installer *Installer) bootInstaller(ctx context.Context, kernel, initrd, disk string, port int) (string, error) {
	argv, err := installer.HTTPArgv(kernel, initrd, disk, port)
	if err != nil {
		return "", err
	}
	return installer.runCapture(ctx, argv, installer.Options.BootTimeout)
}

// haveSSHProbe preserves the producer's optional-tool skip boundary. The
// authenticated transport itself is native Go, so the action starts no Python.
func (installer *Installer) haveSSHProbe() bool {
	if _, err := installer.ops.Look("uv"); err == nil {
		return true
	}
	_, err := installer.ops.Look("sshpass")
	return err == nil
}

func (installer *Installer) sshLoginOK(ctx context.Context, port int) (bool, error) {
	if !installer.haveSSHProbe() {
		return false, errors.New("SSH probe prerequisite is unavailable")
	}
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	config := &ssh.ClientConfig{
		User: installer.Options.SSHUser,
		Auth: []ssh.AuthMethod{ssh.Password(installer.Options.SSHPassword)},
		// #nosec G106 -- the client reaches only the installer-owned loopback forward for this private work tree's fresh appliance key.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return false, fmt.Errorf("authenticate installer SSH at %s: %w", address, err)
	}
	if err := client.Close(); err != nil {
		return false, fmt.Errorf("close installer SSH client: %w", err)
	}
	return true, nil
}

func (installer *Installer) bootTargetSSH(
	ctx context.Context,
	work, disk string,
	timeout time.Duration,
) (ok bool, serialPath string, resultErr error) {
	port := installer.Options.SSHPort
	if port == 0 {
		var err error
		port, err = installer.ops.Port()
		if err != nil {
			return false, "", err
		}
	}
	base := installer.qemuBase(true)
	argv := slices.Clone(base)
	var b textbuf.Buffer
	drive := b.Str("file=").Str(disk).Str(",format=raw,if=virtio").String()
	b.Reset()
	forward := b.Str("user,id=net0,hostfwd=tcp::").Int(int64(port)).Str("-:22").String()
	argv = append(argv,
		"-drive", drive,
		"-netdev", forward,
		"-device", "virtio-net-pci,netdev=net0",
	)
	serialPath = filepath.Join(work, "target-serial.log")
	// #nosec G304 -- serialPath is constructed beneath the installer-owned work directory.
	serial, err := os.Create(serialPath)
	if err != nil {
		return false, serialPath, err
	}
	// #nosec G204 -- argv is assembled exclusively by the installer's closed QEMU argument builders.
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	// stopVMWaiting owns the interrupt-then-kill bound; CommandContext must not kill QEMU first.
	command.Cancel = func() error { return nil }
	command.Env = installer.ops.Environ()
	command.Stdout, command.Stderr = serial, serial
	if err := installer.ops.Start(command); err != nil {
		return false, serialPath, errors.Join(
			fmt.Errorf("start installed appliance QEMU: %w", err),
			serial.Close(),
		)
	}
	defer func() {
		resultErr = joinInstallCleanup(resultErr, serial.Close, "close installed appliance serial log")
	}()
	done := waitVM(command)
	defer stopVMWaiting(command, done)
	deadline := time.Now().Add(timeout)
	var loginErr error
	for time.Now().Before(deadline) {
		ok, attemptErr := installer.sshLoginOK(ctx, port)
		if ok {
			return true, serialPath, nil
		}
		if attemptErr != nil {
			loginErr = attemptErr
		}
		if err := installer.ops.Sleep(ctx, 3*time.Second); err != nil {
			return false, serialPath, err
		}
	}
	if loginErr != nil {
		return false, serialPath, fmt.Errorf("installer SSH login deadline expired: %w", loginErr)
	}
	return false, serialPath, errors.New("installer SSH login deadline expired without an attempt result")
}

func (installer *Installer) executeHTTP(ctx context.Context, work string, report InstallReport) (result InstallReport, resultErr error) {
	var b textbuf.Buffer
	report.line(installer.prefix(), b.Str("arch=").Str(installer.Options.Arch).
		Str(" accel=").Str(installer.accelerator()).Str(" kernel=").Str(installer.Options.Kernel).String())
	b.Reset()
	initrd, err := installer.buildInitrd(ctx, work)
	if err != nil {
		return report, err
	}
	initrdInfo, err := installer.ops.FS.Stat(initrd)
	if err != nil {
		return report, err
	}
	report.artifact("initrd", initrd, initrdInfo.Size())
	report.line(installer.prefix(), b.Str("initrd built (").Int(initrdInfo.Size()).Str(" bytes)").String())
	b.Reset()
	image, err := installer.buildImage(ctx, work)
	if err != nil {
		return report, err
	}
	imageInfo, err := installer.ops.FS.Stat(image.Path)
	if err != nil {
		return report, err
	}
	report.artifact("image", image.Path, imageInfo.Size())
	b.Reset()
	report.line(installer.prefix(), b.Str("image built ").Str(image.Path).String())
	served := filepath.Join(work, "served")
	if err := installer.ops.FS.MkdirAll(served, 0o750); err != nil {
		return report, err
	}
	if _, err := installer.writeChecksum(image.Path, served); err != nil {
		return report, err
	}
	if image.ZeFS == "" {
		return installer.fail(report, "no database.zefs produced by appliance assemble")
	}
	zefsInfo, err := installer.ops.FS.Stat(image.ZeFS)
	if err != nil {
		return installer.fail(report, "no database.zefs produced by appliance assemble")
	}
	if err := copyInstallFile(image.ZeFS, filepath.Join(served, "database.zefs")); err != nil {
		return report, err
	}
	report.artifact("database.zefs", image.ZeFS, zefsInfo.Size())
	b.Reset()
	report.line(installer.prefix(), b.Str("serving zefs ").Str(filepath.Base(image.ZeFS)).
		Str(" (").Int(zefsInfo.Size()).Str(" bytes)").String())
	server, port, err := startInstallHTTP(ctx, served)
	if err != nil {
		return report, err
	}
	defer func() {
		resultErr = joinInstallCleanup(resultErr, server.Stop, "stop installer HTTP server")
	}()
	b.Reset()
	report.line(installer.prefix(), b.Str("serving install artifacts on :").Int(int64(port)).String())
	disk := filepath.Join(work, "target.img")
	if err := truncateInstallFile(disk, imageInfo.Size()); err != nil {
		return report, err
	}
	serial, err := installer.bootInstaller(ctx, installer.Options.Kernel, initrd, disk, port)
	if err != nil {
		return report, err
	}
	if !strings.Contains(serial, InstallMarkWritten) || !strings.Contains(serial, InstallMarkDone) {
		report.lines = append(report.lines, serial)
		return installer.fail(report, "installer did not report success on serial")
	}
	report.check("installer-serial", InstallVerdictPass, "installer wrote disk and completed")
	report.line(installer.prefix(), "installer wrote disk + completed")
	if !installer.haveSSHProbe() {
		report.line(installer.prefix(), "SKIP AC-10 SSH login (install uv or sshpass to test)")
		report.line(installer.prefix(), "PASS (installer only, SSH probe skipped)")
		report.check("ssh-login", InstallVerdictSkip, "install uv or sshpass to test")
		report.Verdict = InstallVerdictPass
		return report, nil
	}
	ok, serialPath, err := installer.bootTargetSSH(ctx, work, disk, 120*time.Second)
	if err != nil {
		return report, err
	}
	if !ok {
		report.line(installer.prefix(), "FAIL second boot SSH login as power user failed (AC-10)")
		installer.appendSerialTail(&report, serialPath)
		report.Verdict, report.Reason = InstallVerdictFail, "second boot SSH login as power user failed (AC-10)"
		return report, nil
	}
	report.check("ssh-login", InstallVerdictPass, "power user authenticated")
	report.line(installer.prefix(), "AC-10 SSH login as power user succeeded")
	report.line(installer.prefix(), "PASS")
	report.Verdict = InstallVerdictPass
	return report, nil
}

func truncateInstallFile(name string, size int64) error {
	// #nosec G304 -- name is constructed beneath the installer-owned work directory.
	file, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if err := file.Truncate(size); err != nil {
		return errors.Join(err, file.Close())
	}
	return file.Close()
}

func (installer *Installer) appendSerialTail(report *InstallReport, path string) {
	data, err := installer.ops.FS.ReadFile(path)
	if err != nil {
		return
	}
	report.line(installer.prefix(), "--- target boot serial (tail) ---")
	if len(data) > installSerialTailBytes {
		data = data[len(data)-installSerialTailBytes:]
	}
	report.lines = append(report.lines, string(data))
}
