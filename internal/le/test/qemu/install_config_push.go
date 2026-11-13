// Design: docs/architecture/testing/qemu-integration.md -- native SSH configuration publication.
// Related: install_config_push_ssh.go -- authenticated observations and refusal control.
// Related: install_config_push_agent.go -- private host-driver identity ownership.
package testqemu

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/ze-software/ze/internal/appliance"
	"github.com/ze-software/ze/internal/component/config"
)

const configPushBootstrapHost = "config-push-bootstrap"
const configPushAcceptedHost = "config-push-accepted"
const configPushObserver = "config-push-observer"
const configPushHostKey = "configpush_host_ed25519"

func (installer *Installer) executeApplianceConfigPush(ctx context.Context) (report InstallReport, resultErr error) {
	report.Action, report.Arch = "appliance-config-push", installer.Options.Arch
	report.Accelerator, report.Verdict = installer.accelerator(), InstallVerdictFail
	defer func() {
		if resultErr != nil {
			report.Verdict = InstallVerdictFail
		}
	}()
	if installer.Options.Arch != runtime.GOARCH {
		return report, errors.New("native configuration push requires the host architecture")
	}
	if installer.Options.Image != "" {
		return report, errors.New("native configuration push builds its own credentialed image; unset ZE_INSTALL_IMAGE")
	}
	if !Hardware(report.Accelerator) {
		return report, errors.New("native configuration push requires KVM or HVF hardware acceleration")
	}
	if _, err := installer.ops.Look(installer.qemuBinary()); err != nil {
		return report, err
	}
	work, err := installer.ops.FS.MkdirTemp("", "ze-appliance-config-push-")
	if err != nil {
		return report, err
	}
	if installer.Options.Keep {
		report.Retained = work
	} else {
		defer func() {
			resultErr = joinInstallCleanup(resultErr, func() error { return installer.ops.FS.RemoveAll(work) }, "remove native configuration proof")
		}()
	}
	// Unix socket paths are short even when the session scratch path is long.
	// The agent identity MUST be stopped before its private directory is removed.
	identityDir, err := os.MkdirTemp("", "zcp-")
	if err != nil {
		return report, err
	}
	defer func() {
		resultErr = joinInstallCleanup(resultErr, func() error { return os.RemoveAll(identityDir) }, "remove native push identity")
	}()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return report, err
	}
	defer clear(private)
	home, socket, stopIdentity, err := installer.configPushIdentity(identityDir, private)
	if err != nil {
		return report, err
	}
	defer func() { resultErr = joinInstallCleanup(resultErr, stopIdentity, "stop native push identity") }()
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		return report, err
	}
	key, err := ssh.NewPublicKey(public)
	if err != nil {
		return report, err
	}
	port, err := installer.ops.Port()
	if err != nil {
		return report, err
	}
	seed, err := installer.configPushSeed(key, port)
	if err != nil {
		return report, err
	}
	named := appliance.DefaultConfig(InstallApplianceName)
	named.Credentials.Username = installer.Options.SSHUser
	named.Credentials.SSHAuthorizedKeys = []string{strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))}
	named.SSH.Port = strconv.Itoa(port)
	input, err := json.Marshal(named)
	if err != nil {
		return report, err
	}
	initJSON := filepath.Join(work, "named-input.json")
	if err := os.WriteFile(initJSON, input, 0o600); err != nil {
		return report, err
	}
	image, err := installer.prepareISOImageInputs(ctx, work, initJSON, seed)
	if err != nil {
		return report, err
	}
	report.artifact("current-host-driver", image.Ze, 0)
	report.artifact("current-image", image.Image, 0)
	disk := filepath.Join(work, "target.img")
	report.artifact("target-disk", disk, 0)
	for _, phase := range []string{"bootstrap", "publication", "refusals"} {
		report.artifact(phase+"-serial", filepath.Join(work, phase+"-boot", "ota-serial.log"), 0)
	}
	if err := copyInstallFile(image.Image, disk); err != nil {
		return report, err
	}
	certificate, err := os.ReadFile(filepath.Join(image.AppDir, "secrets", "tls", "cert.pem")) //nolint:gosec // Owned appliance certificate.
	if err != nil {
		return report, err
	}
	token, err := os.ReadFile(filepath.Join(image.AppDir, "secrets", "update.token")) //nolint:gosec // Owned updater credential.
	if err != nil {
		return report, err
	}
	defer clear(token)
	bootstrap, err := installer.configPushBootstrap(ctx, work, disk, certificate, string(token), port)
	if err != nil {
		return report, err
	}
	stored, hostKey, err := installer.configPushDisk(ctx, work, disk, "bootstrap")
	if err != nil {
		return report, err
	}
	if err := configPushHost(stored, configPushBootstrapHost); err != nil {
		return report, err
	}
	// Bootstrap adds discovered interfaces to the seed. The replacement MUST
	// retain that complete selected source as well as the management settings.
	bootstrapConfig := stored
	seedPath := filepath.Join(image.AppDir, "ze.conf")
	defer func() {
		resultErr = joinInstallCleanup(resultErr, func() error { return os.WriteFile(seedPath, []byte(seed), 0o600) }, "restore original named seed")
	}()
	if err := configPushCandidate(seedPath, bootstrapConfig, configPushBootstrapHost); err != nil {
		return report, err
	}
	report.passed("bootstrap", "named password and SSH key installed; host pin read from stopped owned disk")
	identity := []string{"HOME=" + home, "SSH_AUTH_SOCK=" + socket}
	if err := installer.configPushPublish(ctx, work, disk, image, certificate, string(token), home, identity, signer, hostKey, bootstrap, port); err != nil {
		return report, err
	}
	stored, retainedKey, err := installer.configPushDisk(ctx, work, disk, "publication")
	if err != nil {
		return report, err
	}
	if !sshKeysEqual(hostKey, retainedKey) {
		return report, errors.New("configuration publication changed the guest host key")
	}
	if err := configPushHost(stored, configPushAcceptedHost); err != nil {
		return report, err
	}
	expected, err := configPushContent(bootstrapConfig, configPushAcceptedHost)
	if err != nil {
		return report, err
	}
	if err := configPushUnchanged(expected, stored); err != nil {
		return report, err
	}
	report.passed("live-publication", "current host driver accepted the previewed complete replacement; fresh pinned SSH reads its hostname and configuration")
	report.passed("selected-source", "stopped disk active pointer resolves to the accepted hostname")
	accepted := stored
	if err := installer.configPushRefusals(ctx, work, disk, image, certificate, string(token), home, identity, signer, hostKey, bootstrap, port); err != nil {
		return report, err
	}
	stored, retainedKey, err = installer.configPushDisk(ctx, work, disk, "refusals")
	if err != nil {
		return report, err
	}
	if !sshKeysEqual(hostKey, retainedKey) {
		return report, errors.New("refusal controls changed the guest host key")
	}
	if err := configPushHost(stored, configPushAcceptedHost); err != nil {
		return report, err
	}
	if err := configPushUnchanged(accepted, stored); err != nil {
		return report, err
	}
	report.passed("same-disk-reboot", "accepted hostname, password and named SSH key survive a fresh boot of the same disk")
	report.passed("refusals", "malformed input, edit denial and wrong host pin preserve live and selected persistent configuration")
	report.line("APPLIANCE-CONFIG-PUSH-QEMU: ", "PASS native SSH publication, same-disk retention and three refusal controls")
	report.Verdict = InstallVerdictPass
	return report, nil
}

func (installer *Installer) configPushSeed(key ssh.PublicKey, port int) (string, error) {
	data, err := os.ReadFile(filepath.Join(installer.Tree, "gokrazy", "ze", "ze.conf")) //nolint:gosec // Checked-out appliance seed.
	if err != nil {
		return "", err
	}
	tree, err := config.ParseTreeForValidation(string(data))
	if err != nil {
		return "", err
	}
	system := configPushContainer(tree, "system")
	system.Set("host", configPushBootstrapHost)
	configPushContainer(configPushContainer(tree, "environment"), "ssh").Set("host-key", configPushHostKey)
	servers := configPushContainer(configPushContainer(tree, "environment"), "ssh")
	server := servers.GetList("server")["default"]
	if server == nil {
		server = config.NewTree()
		servers.AddListEntry("server", "default", server)
	}
	server.Set("ip", "0.0.0.0")
	server.Set("port", strconv.Itoa(port))
	observer := config.NewTree()
	observer.SetLeafListMembers("profile", []string{configPushObserver})
	publicKey := config.NewTree()
	publicKey.Set("type", key.Type())
	publicKey.Set("key", base64.StdEncoding.EncodeToString(key.Marshal()))
	observer.AddListEntry("public-keys", "owned-operator", publicKey)
	configPushContainer(system, "authentication").AddListEntry("user", configPushObserver, observer)
	profile := config.NewTree()
	configPushContainer(profile, "run").Set("default-action", "allow")
	configPushContainer(profile, "edit").Set("default-action", "deny")
	configPushContainer(system, "authorization").AddListEntry("profile", configPushObserver, profile)
	return configPushSerialize(tree)
}

func configPushContainer(parent *config.Tree, name string) *config.Tree {
	child := parent.GetContainer(name)
	if child == nil {
		child = config.NewTree()
		parent.SetContainer(name, child)
	}
	return child
}

func configPushSerialize(tree *config.Tree) (string, error) {
	schema, err := config.YANGSchema()
	if err != nil {
		return "", err
	}
	return config.SerializeSet(tree, schema), nil
}

func configPushHost(content, expected string) error {
	tree, err := config.ParseTreeForValidation(content)
	if err != nil {
		return errors.New("native configuration observation is not valid complete configuration")
	}
	system := tree.GetContainer("system")
	if system == nil {
		return errors.New("native configuration observation has no system settings")
	}
	host, ok := system.Get("host")
	if !ok {
		return errors.New("native configuration observation has no hostname")
	}
	if host != expected {
		return errors.New("native configuration observation has an unexpected hostname")
	}
	return nil
}

func (installer *Installer) configPushBootstrap(ctx context.Context, work, disk string, certificate []byte, token string, port int) (status otaStatus, resultErr error) {
	guest, err := installer.configPushLaunch(ctx, work, disk, certificate, token, "bootstrap", port)
	if err != nil {
		return status, err
	}
	defer func() { resultErr = joinInstallCleanup(resultErr, guest.stop, "stop bootstrap guest") }()
	status, err = guest.await(ctx, "")
	if err != nil {
		return status, err
	}
	if status.PermUUID == "" {
		return status, errors.New("bootstrap status has no persistent partition identity")
	}
	return status, guest.configPushReboot(ctx)
}

func (installer *Installer) configPushPublish(ctx context.Context, work, disk string, image installISOImage, certificate []byte, token, home string, identity []string, signer ssh.Signer, hostKey ssh.PublicKey, bootstrap otaStatus, port int) (resultErr error) {
	guest, err := installer.configPushLaunch(ctx, work, disk, certificate, token, "publication", port)
	if err != nil {
		return err
	}
	defer func() { resultErr = joinInstallCleanup(resultErr, guest.stop, "stop publication guest") }()
	verify, err := guest.configPushPin(home, hostKey)
	if err != nil {
		return err
	}
	if err := guest.configPushReady(ctx, verify, bootstrap); err != nil {
		return err
	}
	namedPath := filepath.Join(image.AppDir, "appliance.json")
	namedBefore, err := os.ReadFile(namedPath) //nolint:gosec // Owned named appliance.
	if err != nil {
		return err
	}
	defer func() {
		resultErr = joinInstallCleanup(resultErr, func() error { return os.WriteFile(namedPath, namedBefore, 0o600) }, "restore owned named target")
	}()
	if err := configPushTarget(namedPath, guest.sshPort, installer.Options.SSHUser); err != nil {
		return err
	}
	seedPath := filepath.Join(image.AppDir, "ze.conf")
	seed, err := os.ReadFile(seedPath) //nolint:gosec // Owned complete replacement.
	if err != nil {
		return err
	}
	defer func() {
		resultErr = joinInstallCleanup(resultErr, func() error { return os.WriteFile(seedPath, seed, 0o600) }, "restore owned seed")
	}()
	if err := configPushCandidate(seedPath, string(seed), configPushAcceptedHost); err != nil {
		return err
	}
	preview, err := installer.configPushCommand(ctx, image, identity, true)
	if err != nil {
		return err
	}
	if err := configPushHost(preview.Stdout, configPushAcceptedHost); err != nil {
		return err
	}
	candidate, err := os.ReadFile(seedPath) //nolint:gosec // Preview baseline in the owned named directory.
	if err != nil {
		return err
	}
	if err := configPushUnchanged(string(candidate), preview.Stdout); err != nil {
		return err
	}
	if _, err := installer.configPushCommand(ctx, image, identity, false); err != nil {
		return err
	}
	live, err := guest.configPushLive(ctx, signer, verify, configPushAcceptedHost)
	if err != nil {
		return err
	}
	if err := configPushUnchanged(preview.Stdout, live); err != nil {
		return err
	}
	return guest.configPushReboot(ctx)
}

func (installer *Installer) configPushRefusals(ctx context.Context, work, disk string, image installISOImage, certificate []byte, token, home string, identity []string, signer ssh.Signer, hostKey ssh.PublicKey, bootstrap otaStatus, port int) (resultErr error) {
	guest, err := installer.configPushLaunch(ctx, work, disk, certificate, token, "refusals", port)
	if err != nil {
		return err
	}
	defer func() { resultErr = joinInstallCleanup(resultErr, guest.stop, "stop refusal guest") }()
	verify, err := guest.configPushPin(home, hostKey)
	if err != nil {
		return err
	}
	if err := guest.configPushReady(ctx, verify, bootstrap); err != nil {
		return err
	}
	baseline, err := guest.configPushLive(ctx, signer, verify, configPushAcceptedHost)
	if err != nil {
		return err
	}
	if err := guest.configPushMalformed(ctx, signer, verify); err != nil {
		return err
	}
	if err := guest.configPushUnchanged(ctx, signer, verify, baseline); err != nil {
		return err
	}
	seedPath, namedPath := filepath.Join(image.AppDir, "ze.conf"), filepath.Join(image.AppDir, "appliance.json")
	seed, err := os.ReadFile(seedPath) //nolint:gosec // Owned replacement source.
	if err != nil {
		return err
	}
	defer func() {
		resultErr = joinInstallCleanup(resultErr, func() error { return os.WriteFile(seedPath, seed, 0o600) }, "restore refusal seed")
	}()
	named, err := os.ReadFile(namedPath) //nolint:gosec // Owned target source.
	if err != nil {
		return err
	}
	defer func() {
		resultErr = joinInstallCleanup(resultErr, func() error { return os.WriteFile(namedPath, named, 0o600) }, "restore refusal target")
	}()
	if err := configPushCandidate(seedPath, string(seed), "config-push-denied"); err != nil {
		return err
	}
	// The observer MUST first be seen to log in and run an allowed command.
	// Without that positive, a broken observer login makes the host driver
	// fail at authentication, which reads the same as an edit refusal.
	if err := guest.configPushHostname(ctx, configPushObserver, ssh.PublicKeys(signer), verify, configPushAcceptedHost); err != nil {
		return errors.Join(errors.New("read-only observer could not log in and read the hostname before its edit-denial control"), err)
	}
	if err := configPushTarget(namedPath, guest.sshPort, configPushObserver); err != nil {
		return err
	}
	result, err := installer.configPushCommand(ctx, image, identity, false)
	if err := configPushRefused(result, err); err != nil {
		return err
	}
	if err := guest.configPushUnchanged(ctx, signer, verify, baseline); err != nil {
		return err
	}
	if err := configPushTarget(namedPath, guest.sshPort, installer.Options.SSHUser); err != nil {
		return err
	}
	if err := configPushCandidate(seedPath, string(seed), "config-push-wrong-pin"); err != nil {
		return err
	}
	_, wrong, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	defer clear(wrong)
	wrongSigner, err := ssh.NewSignerFromKey(wrong)
	if err != nil {
		return err
	}
	if _, err := guest.configPushPin(home, wrongSigner.PublicKey()); err != nil {
		return err
	}
	defer func() { _, err := guest.configPushPin(home, hostKey); resultErr = errors.Join(resultErr, err) }()
	result, err = installer.configPushCommand(ctx, image, identity, false)
	if err := configPushRefused(result, err); err != nil {
		return err
	}
	if err := guest.configPushUnchanged(ctx, signer, verify, baseline); err != nil {
		return err
	}
	return guest.configPushReboot(ctx)
}

func configPushRefused(result commandResult, err error) error {
	if err == nil {
		return errors.New("native push refusal control unexpectedly succeeded")
	}
	if result.Code == 0 {
		return errors.New("native push refusal control has no nonzero command status")
	}
	for _, output := range []string{result.Stdout, result.Stderr} {
		if strings.Contains(output, "config applied to "+InstallApplianceName) {
			return errors.New("native push refusal control returned a success receipt")
		}
	}
	return nil
}

func configPushCandidate(path, original, hostname string) error {
	content, err := configPushContent(original, hostname)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func configPushContent(original, hostname string) (string, error) {
	tree, err := config.ParseTreeForValidation(original)
	if err != nil {
		return "", err
	}
	configPushContainer(tree, "system").Set("host", hostname)
	return configPushSerialize(tree)
}

func configPushTarget(path string, port int, username string) error {
	data, err := os.ReadFile(path) //nolint:gosec // Owned named configuration.
	if err != nil {
		return err
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		return err
	}
	for name, edits := range map[string]map[string]any{
		"device":      {"address": forwardHost},
		"ssh":         {"port": strconv.Itoa(port)},
		"credentials": {"username": username},
	} {
		var section map[string]any
		if err := json.Unmarshal(document[name], &section); err != nil {
			return err
		}
		if section == nil {
			return errors.New("owned named appliance has an absent target section")
		}
		for key, value := range edits {
			section[key] = value
		}
		encoded, err := json.Marshal(section)
		if err != nil {
			return err
		}
		document[name] = encoded
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, encoded, 0o600)
}

func (guest *otaGuest) configPushPin(home string, key ssh.PublicKey) (ssh.HostKeyCallback, error) {
	path := filepath.Join(home, ".ssh", "known_hosts")
	line := knownhosts.Line([]string{knownhosts.Normalize(guest.configPushAddress())}, key) + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		return nil, err
	}
	return knownhosts.New(path)
}

// configPushReboot stops the guest through its authenticated update server and
// joins the QEMU process. gokrazy reboots by kexec unless told otherwise, and a
// kexec loads the next kernel with no machine reset, so -no-reboot would never
// exit. kexec=off asks for the reset that -no-reboot turns into an exit.
func (guest *otaGuest) configPushReboot(ctx context.Context) error {
	if err := guest.post(ctx, "/reboot?kexec=off"); err != nil {
		return err
	}
	deadline, cancel := context.WithTimeout(ctx, otaRebootTimeout)
	defer cancel()
	select {
	case <-guest.done:
		if !guest.process.ProcessState.Success() {
			return errors.New("native guest exited unsuccessfully after its authenticated reboot")
		}
		return nil
	case <-deadline.Done():
		return errors.New("native guest did not stop after its authenticated reboot")
	}
}

func (installer *Installer) configPushLaunch(ctx context.Context, work, disk string, certificate []byte, token, phase string, port int) (*otaGuest, error) {
	boot := filepath.Join(work, phase+"-boot")
	if err := os.Mkdir(boot, 0o700); err != nil {
		return nil, err
	}
	return installer.launchOTAGuestMode(ctx, boot, disk, certificate, token, true, port)
}

func (guest *otaGuest) configPushReady(ctx context.Context, verify ssh.HostKeyCallback, bootstrap otaStatus) error {
	timeout := guest.installer.Options.BootTimeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	deadline, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for deadline.Err() == nil {
		status, err := guest.status(deadline)
		if err == nil {
			if status.PermUUID != bootstrap.PermUUID {
				return errors.New("native reboot changed the persistent partition identity")
			}
			if status.BuildTimestamp != bootstrap.BuildTimestamp {
				return errors.New("native reboot changed the runtime image")
			}
			_, err = guest.configPushSSH(deadline, guest.installer.Options.SSHUser, ssh.Password(guest.installer.Options.SSHPassword), verify, HostnameQuery)
			if err == nil {
				return nil
			}
		}
		// sleep(poll-interval): fresh process status and pinned password login must both answer before the boot deadline.
		if err := guest.installer.ops.Sleep(deadline, time.Second); err != nil {
			return err
		}
	}
	return errors.New("native guest did not become ready within its boot deadline")
}

func sshKeysEqual(left, right ssh.PublicKey) bool {
	return string(left.Marshal()) == string(right.Marshal())
}
