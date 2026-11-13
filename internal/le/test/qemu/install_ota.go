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

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const otaReportPrefix = "APPLIANCE-OTA-QEMU: "
const otaPersistentHost = "ota-persistent"

// Mutation budgets cover synchronous consumers, independently of the short
// status probe: config reload owns thirty seconds, and gokrazy shutdown may
// wait one hundred seconds each for SIGTERM and SIGKILL before unmounting.
const otaUploadTimeout = 2 * time.Minute
const otaRebootTimeout = 5 * time.Minute

var errOTAHealthNotReady = errors.New("daemon health is not ready")

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
	seedTree, err := config.ParseTreeForValidation(string(seed))
	if err != nil {
		return report, opererr.Wrap(err, "appliance health seed could not be read",
			"The update proof cannot determine which services the seed configures.",
			"./le test qemu appliance-ota")
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
	if err := guest.assertHealth(ctx, seedTree); err != nil {
		return report, err
	}
	report.passed("updated-runtime", before.BuildTimestamp+" -> "+after.BuildTimestamp)
	report.passed("updated-health", "live daemon health checks passed after update")
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
	if err := guest.assertHealth(ctx, seedTree); err != nil {
		return report, err
	}
	report.passed("previous-slot", "original runtime, persisted host and original login are usable")
	report.passed("recovered-health", "live daemon health checks passed after recovery")
	report.line(otaReportPrefix, "PASS real image upload, reboot, live health, persistent configuration and previous-slot recovery")
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
	return installer.launchOTAGuestMode(ctx, work, disk, certificate, token, false, 0)
}

// launchOTAGuestMode keeps OTA's guest port 22 and allocates its host port.
// A nonzero proof port binds that same owned port on both sides of QEMU.
// A successful launch MUST be paired with otaGuest.stop on every return path.
func (installer *Installer) launchOTAGuestMode(ctx context.Context, work, disk string, certificate []byte, token string, stopOnReboot bool, sshPort int) (*otaGuest, error) {
	guestPort := 22
	if sshPort == 0 {
		allocated, err := installer.ops.Port()
		if err != nil {
			return nil, err
		}
		sshPort = allocated
	} else {
		guestPort = sshPort
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
	forward := b.Str("user,id=net0,hostfwd=tcp:").Str(forwardHost).Byte(':').Int(int64(sshPort)).Str("-:").Int(int64(guestPort)).Str(",hostfwd=tcp:").Str(forwardHost).Byte(':').Int(int64(updatePort)).Str("-:443,hostfwd=tcp:").Str(forwardHost).Byte(':').Int(int64(webPort)).Str("-:8080").String()
	argv := installer.qemuBase(true)
	if stopOnReboot {
		argv = append(argv, "-no-reboot")
	}
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

func (guest *otaGuest) assertHealth(ctx context.Context, seed *config.Tree) error {
	timeout := guest.installer.Options.BootTimeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	deadline, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var last error
	for deadline.Err() == nil {
		output, err := guest.ssh(deadline, "show health | json", "")
		if err != nil {
			last = err
			break
		}
		last = validateOTAHealth(output, seed)
		if last == nil {
			return nil
		}
		if !errors.Is(last, errOTAHealthNotReady) {
			break
		}
		// sleep(poll-interval): SSH can answer before plugin startup completes; re-probe live health until ready, bounded by deadline.
		if err := guest.installer.ops.Sleep(deadline, time.Second); err != nil {
			last = errors.Join(err, last)
			break
		}
	}
	return opererr.Wrap(errors.Join(deadline.Err(), last), "appliance health could not be verified",
		"The update proof cannot confirm that the rebooted daemon and its services are usable.",
		"ZE_INSTALL_KEEP=1 ./le test qemu appliance-ota")
}

// validateOTAHealth reads the show-health payload produced by handleShowHealth.
// A missing report or required probe cannot stand in for measured health.
func validateOTAHealth(output string, seed *config.Tree) error {
	var report health.Report
	decoder := json.NewDecoder(strings.NewReader(output))
	if err := decoder.Decode(&report); err != nil {
		return fmt.Errorf("decode daemon health response: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("daemon health response has trailing data")
	}
	if _, err := time.Parse(time.RFC3339, report.CheckedAt); err != nil {
		return fmt.Errorf("daemon health check timestamp is invalid: %w", err)
	}
	// The daemon's answer is external input, so reject unknown status values.
	switch report.Status {
	case health.StatusHealthy, health.StatusDegraded, health.StatusDown:
	default:
		return fmt.Errorf("daemon health response has unknown status %q", report.Status)
	}
	seen := make(map[string]bool, len(report.Components))
	overall := health.StatusHealthy
	var unhealthy []error
	for _, component := range report.Components {
		if component.Name == "" {
			return errors.New("daemon health response has an unnamed component")
		}
		if seen[component.Name] {
			return fmt.Errorf("daemon health response repeats component %q", component.Name)
		}
		seen[component.Name] = true
		// Component status values are external input too.
		switch component.Status {
		case health.StatusHealthy:
		case health.StatusDegraded:
			if overall != health.StatusDown {
				overall = health.StatusDegraded
			}
		case health.StatusDown:
			overall = health.StatusDown
		default:
			return fmt.Errorf("daemon health component %q has unknown status %q", component.Name, component.Status)
		}
		if component.Status != health.StatusHealthy {
			if !otaInactiveHealth(component, seed) {
				unhealthy = append(unhealthy, fmt.Errorf("%w: component %q is %q: %s", errOTAHealthNotReady, component.Name, component.Status, component.Reason))
			}
		}
	}
	if report.Status != overall {
		return fmt.Errorf("daemon health overall status %q disagrees with its component status %q", report.Status, overall)
	}
	for _, required := range []string{"plugins", "iface"} {
		if !seen[required] {
			return fmt.Errorf("daemon health response has no %q probe", required)
		}
	}
	return errors.Join(unhealthy...)
}

// otaInactiveHealth recognizes only the inactive outcomes produced by
// l2tp.checkHealth and ike/engine.checkIPsecHealth for the management-only seed.
func otaInactiveHealth(component health.ComponentHealth, seed *config.Tree) bool {
	switch component.Name {
	case "l2tp":
		if seed.GetContainer("l2tp") != nil {
			return false
		}
		return component.Status == health.StatusDegraded && component.Reason == "subsystem not running"
	case "ipsec":
		if seed.GetContainerPath("vpn/ipsec") != nil {
			return false
		}
		return component.Status == health.StatusDown && component.Reason == "ike engine not running"
	default:
		return false
	}
}
