// Design: docs/architecture/testing/qemu-integration.md -- a proof that boots an image
// Overview: actions.go -- the area table that reaches this run
// Related: mplsboot.go -- the booted proof this one follows
// Related: hugepages.go -- the proof whose settings and machinery this run reuses
// Related: boot.go -- the virtual machine and the SSH it is asked over
//
// hostname.go proves that the appliance's kernel host name follows
// `system host` (cmd/ze/hub/main_system_hostname.go reconcileKernelHostname).
//
// Two images are built and booted. The first carries `set system host
// cpe-lab` in its seed: after boot the kernel host name must be cpe-lab. The
// proof then commits the same configuration with cpe-lab2 through the web
// UI's configuration upload, which reloads the daemon, and the kernel host
// name must change to cpe-lab2 without a reboot. The second image leaves
// `system host` unset, and the kernel host name must stay the image's own.
//
// The kernel host name is read through `show host all | json`, whose
// host.hostname is os.Hostname in the daemon: the kernel's node name, which
// `uname -n` prints. The appliance's SSH server is the Ze daemon, and gokrazy
// ships no shell, so the Ze CLI is the reader.
//
// The self-skip contract is the hugepage proof's: a machine without QEMU,
// sshpass, e2fsprogs or Go answers SKIP, and an appliance that never answers is
// a SKIP only under software emulation.

package testqemu

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// HostnameQuery is the question the hostname proof asks. Its host.hostname is
// the daemon's os.Hostname, the kernel node name.
const HostnameQuery = "show host all | json"

// HostnameReportPrefix opens every line the hostname proof prints.
const HostnameReportPrefix = "APPLIANCE-HOSTNAME-QEMU: "

// The names the proof configures: the one the first image boots with, and the
// one it commits on the running appliance.
const (
	hostnameBooted    = "cpe-lab"
	hostnameCommitted = "cpe-lab2"
)

// webGuestPort is the web UI port the default appliance seed configures
// (`set environment web server default port 8080` in gokrazy/ze/ze.conf). The
// configuration upload is served there.
const webGuestPort = 8080

// webUploadPath is the web UI route that replaces the whole configuration and
// reloads the daemon (internal/component/web HandleConfigUpload, mounted by
// cmd/ze/hub/service_web.go).
const webUploadPath = "/config/upload"

// webUploadMax bounds one upload request, connection included.
const webUploadMax = 30 * time.Second

// imageConfigPath is the gokrazy instance config whose Hostname is the name the
// image boots with before Ze applies `system host` (docs/guide/appliance.md).
var imageConfigPath = filepath.Join("gokrazy", "ze", "config.json")

// HostnameStep is one kernel host name the proof required.
type HostnameStep struct {
	Name string `json:"name"`
	Want string `json:"want"`
	Got  string `json:"got,omitempty"`
}

// HostnameReport is one run of the appliance hostname proof.
//
// Steps carries each name the proof required and what the kernel carried, in
// order. ConsoleTail carries the serial console only when a step failed.
type HostnameReport struct {
	Verdict     Verdict        `json:"verdict"`
	Reason      string         `json:"reason,omitempty"`
	Arch        string         `json:"arch"`
	Accelerator string         `json:"accelerator"`
	ImageName   string         `json:"image-name,omitempty"`
	Steps       []HostnameStep `json:"steps,omitempty"`
	ConsoleTail []string       `json:"console-tail,omitempty"`
}

// Text renders the run for a person: one prefixed verdict line, the steps,
// then the serial console of an appliance that failed a step.
func (r HostnameReport) Text() string {
	var tb textbuf.Buffer
	tb.Str(HostnameReportPrefix)

	switch r.Verdict {
	case VerdictPass:
		tb.Str("PASS the kernel host name followed system host at boot and on commit, and stayed the image's when unset\n")
	case VerdictSkip:
		tb.Str("SKIP ").Str(r.Reason).Byte('\n')
		return tb.String()
	case VerdictFail, VerdictUnspecified:
		tb.Str("FAIL ").Str(r.Reason).Byte('\n')
	default:
		panic("BUG: invalid hostname proof verdict")
	}

	for _, step := range r.Steps {
		tb.Str("  ").Str(step.Name).Str(": want ").Str(step.Want).Str(", got ").Str(step.Got).Byte('\n')
	}
	if len(r.ConsoleTail) > 0 {
		tb.Str("  serial console tail:\n")
		for _, line := range r.ConsoleTail {
			tb.Str("    ").Str(line).Byte('\n')
		}
	}
	return tb.String()
}

// Hostname is one run of the appliance hostname proof.
//
// It holds a hugepage proof for that proof's settings (architecture, memory,
// password, port, firmware, deadline) and for its machinery (prerequisites,
// host build, appliance commands, QEMU arguments, SSH). The images it builds
// and the questions it asks are its own.
type Hostname struct {
	run *Hugepages
}

// newHostname answers the run the command performs over tree, with every
// setting taken from the environment or from its default.
func newHostname(tree string) *Hostname {
	return &Hostname{run: newHugepages(tree)}
}

// Run performs the proof and answers what happened. A missing prerequisite is
// a SKIP, and a step the run cannot perform is an error.
func (h *Hostname) Run() (HostnameReport, error) {
	report := HostnameReport{Arch: h.run.Arch, Accelerator: accelerator()}

	if reason := h.run.missingPrerequisite(); reason != "" {
		report.Verdict = VerdictSkip
		report.Reason = reason
		return report, nil
	}
	if h.run.Arch == ArchARM64 && !isFile(h.run.Bios) {
		report.Verdict = VerdictSkip
		report.Reason = firmwareSkipReason(h.run.Bios)
		return report, nil
	}

	imageName, err := imageHostName(h.run.Tree)
	if err != nil {
		return report, err
	}
	report.ImageName = imageName

	base, err := os.ReadFile(filepath.Join(h.run.Tree, defaultSeedPath)) //nolint:gosec // a fixed path in the checkout
	if err != nil {
		return report, err
	}

	work, err := h.workDir()
	if err != nil {
		return report, err
	}
	if !h.run.Keep {
		defer os.RemoveAll(work) //nolint:errcheck // the run's verdict is what the caller reads
	}

	h.run.note("building the host ze...")
	host, err := h.run.buildHostZe(context.Background(), work)
	if err != nil {
		return report, err
	}

	h.run.note("building the appliance image with system host " + hostnameBooted + "...")
	named, err := h.buildImage(host, filepath.Join(work, "named"), seedWithHost(string(base), hostnameBooted))
	if err != nil {
		return report, err
	}
	h.run.note("building the appliance image with system host unset...")
	unset, err := h.buildImage(host, filepath.Join(work, "unset"), string(base))
	if err != nil {
		return report, err
	}

	h.run.note("booting the appliance named " + hostnameBooted + "...")
	report, done, err := h.bootNamed(report, named, string(base))
	if err != nil {
		return report, err
	}
	if done {
		return report, nil
	}

	h.run.note("booting the appliance with system host unset...")
	return h.bootUnset(report, unset, imageName)
}

// seedWithHost answers the default seed followed by the line that sets
// `system host`. An appliance overlay replaces the default seed rather than
// adding to it, so the default is carried here.
func seedWithHost(base, name string) string {
	var tb textbuf.Buffer
	return tb.Str(base).Byte('\n').Str("set system host ").Str(name).Byte('\n').String()
}

// imageHostName reads the name the image boots with out of the gokrazy instance
// config. A config that does not parse, or names none, is an error: the unset
// step would otherwise compare the kernel with an empty name.
func imageHostName(tree string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(tree, imageConfigPath)) //nolint:gosec // a fixed path in the checkout
	if err != nil {
		return "", err
	}
	var parsed struct {
		Hostname string `json:"Hostname"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", errors.New("read " + imageConfigPath + ": " + err.Error())
	}
	if parsed.Hostname == "" {
		return "", errors.New(imageConfigPath + " names no Hostname, so the image's own name is unknown")
	}
	return parsed.Hostname, nil
}

// hostnameOf reads the kernel host name out of `show host all | json`. A
// document that does not parse is an error, because the appliance answered
// something other than the query's JSON. A document with no host.hostname is
// an error too: an empty name is never the answer.
func hostnameOf(document string) (string, error) {
	var parsed struct {
		Host struct {
			Hostname string `json:"hostname"`
		} `json:"host"`
	}
	if err := json.Unmarshal([]byte(document), &parsed); err != nil {
		return "", err
	}
	if parsed.Host.Hostname == "" {
		return "", errors.New("the answer carries no host.hostname")
	}
	return parsed.Host.Hostname, nil
}

// withGuestForward answers a copy of argv with one more host-to-guest TCP
// forward on its -nic, so the web UI is reachable beside SSH. An argv with no
// -nic value is an error: the forward would silently not exist.
func withGuestForward(argv []string, hostPort, guestPort int) ([]string, error) {
	out := slices.Clone(argv)
	for i := range len(out) - 1 {
		if out[i] != "-nic" {
			continue
		}
		var tb textbuf.Buffer
		out[i+1] = tb.Str(out[i+1]).Str(",hostfwd=tcp:").Str(forwardHost).Byte(':').
			Int(int64(hostPort)).Str("-:").Int(int64(guestPort)).String()
		return out, nil
	}
	return nil, errors.New("the QEMU arguments carry no -nic to add the web forward to")
}

// workDir answers a fresh directory for one run's files, under the checkout's
// scratch tree for the reason the hugepage proof's workDir gives.
func (h *Hostname) workDir() (string, error) {
	parent := filepath.Join(h.run.Tree, "tmp", "appliance-hostname-qemu")
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return "", err
	}
	return os.MkdirTemp(parent, "run-")
}

// buildImage builds an appliance image under work whose seed is seed.
func (h *Hostname) buildImage(host, work, seed string) (string, error) {
	return buildSeededImage(h.run, host, filepath.Join(work, "appliances"), seed)
}

// bootNamed boots the image that sets `system host cpe-lab`, requires that
// name, commits cpe-lab2 through the web upload, and requires the new name.
// done is true when the report already carries the verdict. The VM is stopped
// on every path out of this function.
func (h *Hostname) bootNamed(report HostnameReport, image, base string) (HostnameReport, bool, error) {
	vm, err := h.launch(image)
	if err != nil {
		return report, true, err
	}
	defer vm.stop()

	step := HostnameStep{Name: "at boot", Want: hostnameBooted}
	got, lastError := h.askHostname(vm.sshPort, hostnameBooted)
	step.Got = got
	report.Steps = append(report.Steps, step)
	if got != hostnameBooted {
		return hostnameMismatch(report, vm.console.Name(), lastError), true, nil
	}

	step = HostnameStep{Name: "after commit, no reboot", Want: hostnameCommitted}
	if err := h.upload(vm.webPort, seedWithHost(base, hostnameCommitted)); err != nil {
		step.Got = "commit refused"
		report.Steps = append(report.Steps, step)
		return hostnameMismatch(report, vm.console.Name(), err.Error()), true, nil
	}
	got, lastError = h.askHostname(vm.sshPort, hostnameCommitted)
	step.Got = got
	report.Steps = append(report.Steps, step)
	if got != hostnameCommitted {
		return hostnameMismatch(report, vm.console.Name(), lastError), true, nil
	}
	return report, false, nil
}

// bootUnset boots the image that leaves `system host` unset and requires the
// image's own name. The VM is stopped on every path out of this function.
func (h *Hostname) bootUnset(report HostnameReport, image, imageName string) (HostnameReport, error) {
	vm, err := h.launch(image)
	if err != nil {
		return report, err
	}
	defer vm.stop()

	step := HostnameStep{Name: "system host unset", Want: imageName}
	got, lastError := h.askHostname(vm.sshPort, imageName)
	step.Got = got
	report.Steps = append(report.Steps, step)
	if got != imageName {
		return hostnameMismatch(report, vm.console.Name(), lastError), nil
	}
	report.Verdict = VerdictPass
	return report, nil
}

// ports answers the SSH port (the configured one, or a reserved one) and a
// reserved web port for one boot.
func (h *Hostname) ports() (int, int, error) {
	sshPort := h.run.SSHPort
	if sshPort == 0 {
		picked, err := reserveForwardPort()
		if err != nil {
			return 0, 0, err
		}
		sshPort = picked
	}
	webPort, err := reserveForwardPort()
	if err != nil {
		return 0, 0, err
	}
	return sshPort, webPort, nil
}

// argv answers the QEMU arguments for one boot: the hugepage proof's, with the
// web UI forwarded beside SSH.
func (h *Hostname) argv(image string, sshPort, webPort int) ([]string, error) {
	plan, err := h.run.plan()
	if err != nil {
		return nil, err
	}
	return withGuestForward(h.run.qemuArgs(image, sshPort, plan), webPort, webGuestPort)
}

// hostnameVM is one booted appliance: its forwards, its serial console and
// its QEMU process. The caller MUST call stop on every path.
type hostnameVM struct {
	sshPort int
	webPort int
	console *os.File
	process *exec.Cmd
}

// stop ends the VM, then closes the console it wrote to.
func (v *hostnameVM) stop() {
	stopVM(v.process)
	v.console.Close() //nolint:errcheck // the VM has stopped writing to it by now
}

// launch reserves the forwards, opens the console, and starts the VM over
// image. On an error nothing is left running.
func (h *Hostname) launch(image string) (*hostnameVM, error) {
	sshPort, webPort, err := h.ports()
	if err != nil {
		return nil, err
	}
	argv, err := h.argv(image, sshPort, webPort)
	if err != nil {
		return nil, err
	}

	console, err := h.run.startConsole(image, sshPort)
	if err != nil {
		return nil, err
	}
	process := exec.CommandContext(context.Background(), qemuBinary(h.run.Arch), argv...) //nolint:gosec // the argv is built by this package, never by an operator
	process.Stdout = console
	process.Stderr = console
	if err := process.Start(); err != nil {
		console.Close() //nolint:errcheck // the start error is the answer
		return nil, err
	}
	return &hostnameVM{sshPort: sshPort, webPort: webPort, console: console, process: process}, nil
}

// askHostname asks HostnameQuery until the kernel host name is want or the
// deadline passes, and answers the last name seen with the last reason it
// was not want.
//
// A refused connection means the appliance is still booting, and an answer
// that is not the query's JSON means the daemon is not ready. A name other
// than want is retried too: the applier runs at start and after a reload, so
// the name can change after the daemon first answers.
func (h *Hostname) askHostname(port int, want string) (string, string) {
	end := time.Now().Add(h.run.Deadline)
	var got, lastError string
	for {
		out, stderr, err := h.run.ssh(context.Background(), port, HostnameQuery)
		switch name, parseErr := hostnameOf(out); {
		case err != nil:
			lastError = lastLine(stderr, out, err)
		case parseErr != nil:
			lastError = "answered, not the host inventory: " + parseErr.Error()
		case name == want:
			return name, ""
		default:
			got = name
			lastError = "the kernel host name is " + name
		}
		if !time.Now().Before(end) {
			return got, lastError
		}
		time.Sleep(sshRetryPause)
	}
}

// upload commits content as the whole configuration through the web UI's
// configuration upload, which validates it and reloads the daemon. The web UI
// serves TLS with the appliance's own new certificate, so the certificate is
// not checked, as the SSH host key is not; the password is the appliance's.
// The Origin header names the forward, which the same-origin check requires of
// a mutation.
func (h *Hostname) upload(webPort int, content string) error {
	var tb textbuf.Buffer
	origin := tb.Str("https://").Str(forwardHost).Byte(':').Int(int64(webPort)).String()

	ctx, cancel := context.WithTimeout(context.Background(), webUploadMax)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, origin+webUploadPath, strings.NewReader(content))
	if err != nil {
		return err
	}
	request.SetBasicAuth(SSHUser, h.run.SSHPass)
	request.Header.Set("Content-Type", "text/plain")
	request.Header.Set("Origin", origin)

	client := &http.Client{
		Timeout: webUploadMax,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // the appliance is built for this run and its certificate is new every time
		},
		// The upload answers a redirect to the editor; its status is the answer.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close() //nolint:errcheck // the status is the answer

	if response.StatusCode < http.StatusBadRequest {
		return nil
	}
	var body bytes.Buffer
	if _, err := body.ReadFrom(response.Body); err != nil {
		body.WriteString("(body unreadable: " + err.Error() + ")")
	}
	tb.Reset()
	return errors.New(tb.Str("the configuration upload answered ").Str(response.Status).Str(": ").
		Str(strings.TrimSpace(body.String())).String())
}

// hostnameMismatch answers the verdict for a step whose kernel host name never
// became the one it wanted. Under a hardware accelerator that is a failure
// carrying the serial console. Under software emulation an appliance that
// never answered a name is a skip, while one that answered a wrong name, or
// refused the commit, is still a failure: the name is what the proof judges.
func hostnameMismatch(report HostnameReport, consolePath, lastError string) HostnameReport {
	if len(report.Steps) == 0 {
		panic("BUG: hostnameMismatch called with no step")
	}
	detail := "no error captured"
	if lastError != "" {
		detail = lastError
	}
	last := report.Steps[len(report.Steps)-1]

	var tb textbuf.Buffer
	if Hardware(report.Accelerator) || last.Got != "" {
		report.Verdict = VerdictFail
		report.Reason = tb.Str(last.Name).Str(": the kernel host name never became ").Str(last.Want).
			Str(" (accel=").Str(report.Accelerator).Str("); ").Str(detail).String()
		report.ConsoleTail = consoleTail(consolePath)
		return report
	}

	report.Verdict = VerdictSkip
	report.Reason = tb.Str("appliance did not answer within the timeout (accel=").
		Str(report.Accelerator).Str(", software emulation); ").Str(detail).String()
	return report
}
