// Design: docs/architecture/testing/qemu-integration.md -- real appliance OTA.
// Related: install_build.go -- the named image and host-driver builders.
// Related: install_boot.go -- QEMU arguments and process cleanup.

package testqemu

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
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

	"golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const otaReportPrefix = "APPLIANCE-OTA-QEMU: "
const otaPersistentHost = "ota-persistent"

// Mutation budgets cover synchronous consumers, independently of the short
// status probe: config reload owns thirty seconds, and gokrazy shutdown may
// wait one hundred seconds each for SIGTERM and SIGKILL before unmounting.
const otaUploadTimeout = 2 * time.Minute
const otaRebootTimeout = 5 * time.Minute

type otaStatus struct {
	BuildTimestamp string
	PermUUID       string
}

// otaGuest owns one VM, its bounded management client, and its serial log.
// A successful launchOTAGuest MUST be paired with stop on every return path.
type otaGuest struct {
	installer   *Installer
	process     *exec.Cmd
	done        <-chan struct{}
	serial      *os.File
	client      *http.Client
	endpoint    string
	webEndpoint string
	token       string
	sshPort     int
	updatePort  int
}

func (installer *Installer) executeApplianceOTA(ctx context.Context) (report InstallReport, resultErr error) {
	report.Action, report.Arch = "appliance-ota", installer.Options.Arch
	report.Accelerator = installer.accelerator()
	report.Verdict = InstallVerdictFail
	if installer.Options.Arch != runtime.GOARCH {
		return report, errors.New("appliance OTA proof requires a host of the image's architecture")
	}
	if installer.Options.Image != "" {
		return report, errors.New("appliance OTA proof builds its own credentialed images; unset ZE_INSTALL_IMAGE")
	}
	if _, err := installer.ops.Look(installer.qemuBinary()); err != nil {
		return report, err
	}
	work, err := installer.ops.FS.MkdirTemp("", "ze-appliance-ota-")
	if err != nil {
		return report, err
	}
	if installer.Options.Keep {
		report.Retained = work
	} else {
		defer func() {
			resultErr = joinInstallCleanup(resultErr, func() error { return installer.ops.FS.RemoveAll(work) }, "remove appliance OTA proof")
		}()
	}
	image, err := installer.prepareISOImage(ctx, work)
	if err != nil {
		return report, err
	}
	disk := filepath.Join(work, "target.img")
	if err := copyInstallFile(image.Image, disk); err != nil {
		return report, err
	}
	report.passed("first-image", filepath.Base(image.Image))
	result, err := installer.run(ctx, commandSpec{Name: image.Ze, Args: []string{zeApplianceArea, zeApplianceVerbBuild, InstallApplianceName}, Dir: installer.Tree, Env: image.Env})
	if err != nil {
		return report, err
	}
	if result.Code != 0 {
		return report, errors.New("second appliance image build failed: " + result.Stderr)
	}
	images, err := filepath.Glob(filepath.Join(image.AppDir, "ze-*.img"))
	if err != nil {
		return report, err
	}
	if len(images) == 0 {
		return report, errors.New("second appliance build produced no image")
	}
	updateImage := images[len(images)-1]
	cert, err := os.ReadFile(filepath.Join(image.AppDir, "secrets", "tls", "cert.pem")) //nolint:gosec // Proof-owned named appliance.
	if err != nil {
		return report, err
	}
	token, err := os.ReadFile(filepath.Join(image.AppDir, "secrets", "update.token")) //nolint:gosec // Proof-owned updater identity.
	if err != nil {
		return report, err
	}
	defer clear(token)
	guest, err := installer.launchOTAGuest(ctx, work, disk, cert, string(token))
	if err != nil {
		return report, err
	}
	defer func() { resultErr = joinInstallCleanup(resultErr, guest.stop, "stop appliance OTA guest") }()
	report.artifact("target-serial", guest.serial.Name(), 0)
	before, err := guest.await(ctx, "")
	if err != nil {
		return report, err
	}
	if before.PermUUID == "" {
		return report, errors.New("appliance status has no persistent partition identity")
	}
	seed, err := os.ReadFile(filepath.Join(installer.Tree, "gokrazy", "ze", "ze.conf")) //nolint:gosec // Shipped appliance seed.
	if err != nil {
		return report, err
	}
	if err := guest.uploadConfig(ctx, seedWithHost(string(seed), otaPersistentHost)); err != nil {
		return report, err
	}
	if err := guest.assertHost(ctx); err != nil {
		return report, err
	}
	if err := configureOTATarget(filepath.Join(image.AppDir, "appliance.json"), guest.updatePort); err != nil {
		return report, err
	}
	result, err = installer.run(ctx, commandSpec{Name: image.Ze, Args: []string{zeApplianceArea, "push", "--testboot", "--image", filepath.Base(updateImage), InstallApplianceName}, Dir: installer.Tree, Env: image.Env})
	if err != nil {
		return report, fmt.Errorf("appliance update command failed after %s: %w", otaPushProgress(result.Stderr), err)
	}
	if result.Code != 0 {
		return report, fmt.Errorf("appliance update command exited %d after %s", result.Code, otaPushProgress(result.Stderr))
	}
	after, err := guest.await(ctx, before.BuildTimestamp)
	if err != nil {
		return report, err
	}
	if after.PermUUID != before.PermUUID {
		return report, errors.New("OTA replaced the persistent partition identity")
	}
	if err := guest.assertHost(ctx); err != nil {
		return report, err
	}
	report.passed("updated-runtime", before.BuildTimestamp+" -> "+after.BuildTimestamp)
	report.passed("persistent-config", "host and login survived update; persistent UUID unchanged")
	// Testboot retains the original root in the boot configuration. Switch
	// would promote the tested root instead of recovering the original one.
	if err := guest.post(ctx, "/reboot"); err != nil {
		return report, err
	}
	recovered, err := guest.await(ctx, after.BuildTimestamp)
	if err != nil {
		return report, err
	}
	if recovered.BuildTimestamp != before.BuildTimestamp {
		return report, errors.New("previous-slot recovery did not return to the original runtime")
	}
	if recovered.PermUUID != before.PermUUID {
		return report, errors.New("previous-slot recovery changed persistent partition identity")
	}
	if err := guest.assertHost(ctx); err != nil {
		return report, err
	}
	report.passed("previous-slot", "original runtime, persisted host and original login are usable")
	report.line(otaReportPrefix, "PASS real image upload, reboot, persistent configuration and previous-slot recovery")
	report.Verdict = InstallVerdictPass
	return report, nil
}

// otaPushProgress retains only fixed progress messages from the host updater.
// Its other stderr can contain credentials and MUST NOT enter diagnostics.
func otaPushProgress(stderr string) string {
	last := "starting the updater"
	for line := range strings.SplitSeq(stderr, "\n") {
		for _, phase := range []string{"streaming root image...", "streaming boot image...", "streaming boot code...", "marking testboot...", "switching active partition...", "rebooting..."} {
			if strings.TrimSpace(line) == phase {
				last = phase
			}
		}
	}
	return last
}

func configureOTATarget(path string, port int) error {
	data, err := os.ReadFile(path) //nolint:gosec // Proof-owned appliance configuration.
	if err != nil {
		return err
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		return err
	}
	device, err := json.Marshal(struct {
		Address string `json:"address"`
		Port    int    `json:"update-port"`
	}{Address: forwardHost, Port: port})
	if err != nil {
		return err
	}
	document["device"] = device
	patched, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, patched, 0o600)
}

func (installer *Installer) launchOTAGuest(ctx context.Context, work, disk string, certificate []byte, token string) (*otaGuest, error) {
	sshPort, err := installer.ops.Port()
	if err != nil {
		return nil, err
	}
	updatePort, err := installer.ops.Port()
	if err != nil {
		return nil, err
	}
	webPort, err := installer.ops.Port()
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(certificate) {
		return nil, errors.New("appliance OTA certificate cannot be parsed")
	}
	client := &http.Client{
		Transport:     &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS13}},
		Timeout:       10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	var b textbuf.Buffer
	forward := b.Str("user,id=net0,hostfwd=tcp:").Str(forwardHost).Byte(':').Int(int64(sshPort)).Str("-:22,hostfwd=tcp:").Str(forwardHost).Byte(':').Int(int64(updatePort)).Str("-:443,hostfwd=tcp:").Str(forwardHost).Byte(':').Int(int64(webPort)).Str("-:8080").String()
	argv := installer.qemuBase(true)
	b.Reset()
	argv = append(argv, "-drive", b.Str("file=").Str(disk).Str(",format=raw,if=virtio").String(), "-netdev", forward, "-device", "virtio-net-pci,netdev=net0")
	serial, err := os.Create(filepath.Join(work, "ota-serial.log")) //nolint:gosec // Proof-owned serial log.
	if err != nil {
		return nil, err
	}
	process := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // Closed QEMU argument builders.
	process.Cancel = func() error { return nil }
	process.Env = installer.ops.Environ()
	process.Stdout, process.Stderr = serial, serial
	if err := installer.ops.Start(process); err != nil {
		return nil, errors.Join(err, serial.Close())
	}
	b.Reset()
	endpoint := b.Str("https://").Str(net.JoinHostPort(forwardHost, strconv.Itoa(updatePort))).String()
	b.Reset()
	webEndpoint := b.Str("https://").Str(net.JoinHostPort(forwardHost, strconv.Itoa(webPort))).String()
	return &otaGuest{installer: installer, process: process, done: waitVM(process), serial: serial, client: client, endpoint: endpoint, webEndpoint: webEndpoint, token: token, sshPort: sshPort, updatePort: updatePort}, nil
}

func (guest *otaGuest) uploadConfig(ctx context.Context, content string) error {
	ctx, cancel := context.WithTimeout(ctx, otaUploadTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, guest.webEndpoint+webUploadPath, strings.NewReader(content))
	if err != nil {
		return err
	}
	request.SetBasicAuth(guest.installer.Options.SSHUser, guest.installer.Options.SSHPassword)
	request.Header.Set("Content-Type", "text/plain")
	request.Header.Set("Origin", guest.webEndpoint)
	client := *guest.client
	client.Timeout = 0 // The mutation context owns its complete bounded lifetime.
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close() //nolint:errcheck // The upload status is authoritative.
	// The public upload route redirects to the editor after publication. The
	// shared client refuses to follow redirects, keeping credentials local.
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusBadRequest {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return fmt.Errorf("appliance configuration upload HTTP %d: %w", response.StatusCode, err)
	}
	return fmt.Errorf("appliance configuration upload HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
}

func (guest *otaGuest) stop() error {
	// stop MUST join the process before closing its serial log.
	stopVMWaiting(guest.process, guest.done)
	guest.client.CloseIdleConnections()
	return guest.serial.Close()
}

func (guest *otaGuest) status(ctx context.Context) (otaStatus, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, guest.endpoint+"/", http.NoBody)
	if err != nil {
		return otaStatus{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.SetBasicAuth("gokrazy", guest.token)
	response, err := guest.client.Do(request)
	if err != nil {
		return otaStatus{}, err
	}
	defer response.Body.Close() //nolint:errcheck // Read-only status response.
	if response.StatusCode != http.StatusOK {
		return otaStatus{}, fmt.Errorf("appliance status HTTP %d", response.StatusCode)
	}
	var status otaStatus
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&status); err != nil {
		return otaStatus{}, err
	}
	if status.BuildTimestamp == "" {
		return otaStatus{}, errors.New("appliance status has no runtime build timestamp")
	}
	return status, nil
}

func (guest *otaGuest) await(ctx context.Context, previous string) (otaStatus, error) {
	timeout := guest.installer.Options.BootTimeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	deadline, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var last error
	for deadline.Err() == nil {
		status, err := guest.status(deadline)
		if err == nil && status.BuildTimestamp != previous {
			if _, err := guest.ssh(deadline, HostnameQuery, ""); err == nil {
				return status, nil
			} else {
				last = err
			}
		} else if err != nil {
			last = err
		} else {
			last = errors.New("appliance still reports the previous runtime")
		}
		// sleep(poll-interval): return only after authenticated status names the new runtime and Ze answers SSH, bounded by deadline.
		if err := guest.installer.ops.Sleep(deadline, time.Second); err != nil {
			return otaStatus{}, errors.Join(err, last)
		}
	}
	return otaStatus{}, errors.Join(deadline.Err(), last)
}

func (guest *otaGuest) post(ctx context.Context, path string) error {
	ctx, cancel := context.WithTimeout(ctx, otaRebootTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, guest.endpoint+path, http.NoBody)
	if err != nil {
		return err
	}
	request.SetBasicAuth("gokrazy", guest.token)
	client := *guest.client
	client.Timeout = 0 // Keep the shared status probe's short timeout unchanged.
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close() //nolint:errcheck // The status is the operation's result.
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("appliance %s HTTP %d", path, response.StatusCode)
	}
	return nil
}

func (guest *otaGuest) ssh(ctx context.Context, command, input string) (result string, resultErr error) {
	address := net.JoinHostPort(forwardHost, strconv.Itoa(guest.sshPort))
	connection, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return "", err
	}
	finished, watcherDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			connection.Close() //nolint:errcheck // Cancellation owns transport shutdown.
		case <-finished:
		}
	}()
	defer func() {
		close(finished)
		connection.Close() //nolint:errcheck // The command/cancellation result is authoritative.
		<-watcherDone
		if ctx.Err() != nil {
			resultErr = errors.Join(resultErr, ctx.Err())
		}
	}()
	deadline := time.Now().Add(30 * time.Second)
	if phaseDeadline, ok := ctx.Deadline(); ok && phaseDeadline.Before(deadline) {
		deadline = phaseDeadline
	}
	if err := connection.SetDeadline(deadline); err != nil {
		return "", err
	}
	config := &ssh.ClientConfig{
		User: guest.installer.Options.SSHUser,
		Auth: []ssh.AuthMethod{ssh.Password(guest.installer.Options.SSHPassword)},
		// #nosec G106 -- this loopback forward reaches only the proof-owned VM and freshly built key.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	transport, channels, requests, err := ssh.NewClientConn(connection, address, config)
	if err != nil {
		return "", err
	}
	client := ssh.NewClient(transport, channels, requests)
	defer client.Close() //nolint:errcheck // Command result is authoritative.
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close() //nolint:errcheck // Command result is authoritative.
	session.Stdin = strings.NewReader(input)
	// SSH stdout is the rendered answer. Every Ze exec session also writes
	// its answer frame to stderr, which must remain outside JSON payloads.
	var stderr textbuf.Buffer
	session.Stderr = &stderr
	output, err := session.Output(command)
	if err != nil {
		return "", fmt.Errorf("appliance SSH %s: %w: stdout: %s; stderr: %s", command, err, strings.TrimSpace(string(output)), strings.TrimSpace(stderr.String()))
	}
	return string(output), nil
}

func (guest *otaGuest) assertHost(ctx context.Context) error {
	output, err := guest.ssh(ctx, HostnameQuery, "")
	if err != nil {
		return err
	}
	name, err := hostnameOf(output)
	if err != nil {
		return fmt.Errorf("decode appliance SSH hostname response: %w", err)
	}
	if name != otaPersistentHost {
		return fmt.Errorf("appliance host=%q, want preserved host %q", name, otaPersistentHost)
	}
	return nil
}
