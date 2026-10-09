// Design: docs/architecture/testing/qemu-integration.md -- a proof that boots an image
// Overview: actions.go -- the area table that reaches these runs
// Related: hugepages.go -- the proof whose settings and machinery these runs reuse
// Related: boot.go -- the virtual machine and the SSH it is asked over
// Related: mplsboot.go -- the default seed these runs extend
//
// crashcapture.go proves kernel crash capture on a booted appliance image
// (docs/architecture/diagnostics/crash-capture.md).
//
// Both proofs build an image whose appliance configuration reserves a crash
// region (image.crash-dump, rendered as reserve_mem plus ramoops.mem_name on the
// kernel command line), boot it, and read `show crashes | json` until readiness
// reports the region armed and the crash directory on /perm. Each then reboots
// the machine its own way and reads the answer again on the next boot.
//
//   - The panic-harvest proof injects an NMI through the QEMU monitor. The seed
//     sets kernel.unknown_nmi_panic, so the kernel panics, writes its record into
//     the reserved region, and warm-resets. The proof passes when the next boot's
//     daemon has harvested a kernel-kind artifact carrying the panic text into
//     the crash directory.
//   - The OTA proof asks gokrazy's update server for the reboot `gok update` ends
//     with, which kexecs the next kernel on amd64. The proof passes when the
//     next boot is armed again and no kernel artifact appeared, because a clean
//     reboot is not a crash.
//
// Both proofs are amd64-only: an unknown NMI panics only on x86, and gokrazy
// kexecs only on amd64. Another architecture answers SKIP with that reason.

package testqemu

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// CrashReportPrefix opens every line a crash-capture proof prints. It is the
// word the functional suite greps for.
const CrashReportPrefix = "APPLIANCE-CRASH-CAPTURE-QEMU: "

// CrashesQuery is the question both proofs ask on each boot: the crash listing
// with its readiness block.
const CrashesQuery = "show crashes | json"

// crashReserve is the region the image reserves, in the appliance
// configuration's own spelling.
const crashReserve = "16mb"

// crashDirectory is the directory the crash directory probe picks on an
// appliance: /perm is the only writable store that survives a reboot. A
// harvest that lands anywhere else loses the record on the next reboot (A-5).
const crashDirectory = "/perm/ze/crash"

// crashSeedLines are what the proofs add to the default appliance seed.
// crash-dump enabled makes readiness report the feature configured. The two
// sysctls make an injected NMI a panic, and make the panic reboot after one
// second rather than wait for the boot argument's longer timeout.
const crashSeedLines = "set system crash-dump enabled true\n" +
	"set sysctl setting kernel.unknown_nmi_panic value 1\n" +
	"set sysctl setting kernel.panic value 1\n"

// kernelKind is the kind field a harvested kernel record carries in the
// listing.
const kernelKind = "kernel"

// panicText is the line every kernel panic record opens its panic with.
const panicText = "Kernel panic"

// gokrazyUser is the user name gokrazy's update server authenticates.
const gokrazyUser = "gokrazy"

// gokrazyHTTPPort is the guest port gokrazy's update server listens on.
const gokrazyHTTPPort = 80

// CrashLab names which crash-capture proof a run performs.
type CrashLab uint8

const (
	crashLabUnspecified CrashLab = iota
	// CrashLabPanicHarvest panics the kernel and expects the next boot to
	// harvest the record.
	CrashLabPanicHarvest
	// CrashLabOTAUnaffected kexec-reboots the way an OTA update does and
	// expects the next boot armed, with no kernel artifact.
	CrashLabOTAUnaffected
)

// String names the proof for a person.
func (l CrashLab) String() string {
	switch l {
	case CrashLabPanicHarvest:
		return "panic-harvest"
	case CrashLabOTAUnaffected:
		return "ota-unaffected"
	case crashLabUnspecified:
		return "unspecified"
	}
	return "unknown"
}

// CrashCaptureReport is one run of a crash-capture proof.
//
// Armed carries the first boot's readiness block, Rebooted the boot time each
// boot reported, and Artifact the kernel artifact the panic proof read back.
// ConsoleTail carries the serial console when the proof failed.
type CrashCaptureReport struct {
	Verdict     Verdict        `json:"verdict"`
	Reason      string         `json:"reason,omitempty"`
	Lab         string         `json:"lab"`
	Arch        string         `json:"arch"`
	Accelerator string         `json:"accelerator"`
	Readiness   map[string]any `json:"readiness,omitempty"`
	BootBefore  int64          `json:"boot-time-before,omitempty"`
	BootAfter   int64          `json:"boot-time-after,omitempty"`
	Artifact    string         `json:"artifact,omitempty"`
	PanicLine   string         `json:"panic-line,omitempty"`
	ConsoleTail []string       `json:"console-tail,omitempty"`
}

// Text renders the run for a person: one prefixed verdict line, then the
// serial console of a run that failed.
func (r CrashCaptureReport) Text() string {
	var tb textbuf.Buffer
	tb.Str(CrashReportPrefix)

	switch r.Verdict {
	case VerdictPass:
		tb.Str("PASS ").Str(r.Lab).Str(": ").Str(r.Reason).Byte('\n')
		return tb.String()
	case VerdictSkip:
		tb.Str("SKIP ").Str(r.Lab).Str(": ").Str(r.Reason).Byte('\n')
		return tb.String()
	case VerdictFail, VerdictUnspecified:
		tb.Str("FAIL ").Str(r.Lab).Str(": ").Str(r.Reason).Byte('\n')
	default:
		panic("BUG: invalid crash-capture verdict")
	}

	for _, line := range r.ConsoleTail {
		tb.Str("    ").Str(line).Byte('\n')
	}
	return tb.String()
}

// CrashCapture is one run of a crash-capture proof.
//
// It holds a hugepage proof for that proof's settings and machinery, as the
// MPLS boot proof does. The image, the reboot and the questions are its own.
type CrashCapture struct {
	run *Hugepages
	lab CrashLab
}

// newCrashCapture answers the run the command performs over tree.
func newCrashCapture(tree string, lab CrashLab) *CrashCapture {
	return &CrashCapture{run: newHugepages(tree), lab: lab}
}

// Run performs the proof and answers what happened. A missing prerequisite is
// a SKIP, and a step the run cannot perform is an error.
func (c *CrashCapture) Run() (CrashCaptureReport, error) {
	report := CrashCaptureReport{Lab: c.lab.String(), Arch: c.run.Arch, Accelerator: accelerator()}

	if c.run.Arch != ArchAMD64 {
		report.Verdict = VerdictSkip
		report.Reason = "amd64 only: an unknown NMI panics only on x86, and gokrazy kexecs only on amd64"
		return report, nil
	}
	if reason := c.run.missingPrerequisite(); reason != "" {
		report.Verdict = VerdictSkip
		report.Reason = reason
		return report, nil
	}

	work, err := c.workDir()
	if err != nil {
		return report, err
	}
	if !c.run.Keep {
		defer os.RemoveAll(work) //nolint:errcheck // the run's verdict is what the caller reads
	}

	c.run.note("building the host ze...")
	host, err := c.run.buildHostZe(work)
	if err != nil {
		return report, err
	}

	c.run.note("building the appliance image with a crash reservation...")
	image, err := c.buildImage(host, work)
	if err != nil {
		return report, err
	}

	c.run.note("booting the appliance...")
	return c.bootAndAssert(report, image)
}

// workDir answers a fresh directory for one run's files, under the checkout's
// scratch tree for the reason the hugepage proof's workDir gives.
func (c *CrashCapture) workDir() (string, error) {
	parent := filepath.Join(c.run.Tree, "tmp", "appliance-crash-capture-qemu")
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return "", err
	}
	return os.MkdirTemp(parent, "run-")
}

// buildImage initializes an appliance, writes its crash reservation and seed,
// and builds the image.
func (c *CrashCapture) buildImage(host, work string) (string, error) {
	dir := filepath.Join(work, "appliances")
	environment := c.run.applianceEnv(dir)

	if out, err := c.run.appliance(host, environment, "init"); err != nil {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("ze appliance init failed:\n").Str(out).String())
	}
	applianceDir := filepath.Join(dir, ApplianceName)
	err := editImageConfig(filepath.Join(applianceDir, "appliance.json"), func(image map[string]any) {
		image["arch"] = c.run.Arch
		image["memory"] = c.run.Memory
		image["crash-dump"] = map[string]any{"reserve": crashReserve}
	})
	if err != nil {
		return "", err
	}

	base, err := os.ReadFile(filepath.Join(c.run.Tree, defaultSeedPath)) //nolint:gosec // a fixed path in the checkout
	if err != nil {
		return "", err
	}
	var seed textbuf.Buffer
	seed.Str(string(base)).Byte('\n').Str(crashSeedLines)
	// The overlay resolveSeedConfig (internal/appliance/cmd_assemble.go) reads.
	if err := os.WriteFile(filepath.Join(applianceDir, "ze.conf"), []byte(seed.String()), 0o600); err != nil {
		return "", err
	}

	out, err := c.run.appliance(host, environment, zeApplianceVerbBuild)
	if err != nil {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("ze appliance build failed:").Str(buildHint(out)).Byte('\n').Str(out).String())
	}
	return findImage(dir)
}

// crashPorts are the host ports one run forwards into the VM.
type crashPorts struct {
	ssh     int
	monitor int
	http    int
}

// pickCrashPorts answers three free host ports.
func (c *CrashCapture) pickCrashPorts() (crashPorts, error) {
	var ports crashPorts
	var err error
	if ports.ssh, err = freePort(); err != nil {
		return ports, err
	}
	if ports.monitor, err = freePort(); err != nil {
		return ports, err
	}
	ports.http, err = freePort()
	return ports, err
}

// bootAndAssert boots the image, performs the lab's reboot, and answers the
// verdict. The VM is stopped on every path out of this function.
func (c *CrashCapture) bootAndAssert(report CrashCaptureReport, image string) (CrashCaptureReport, error) {
	plan, err := c.run.plan()
	if err != nil {
		return report, err
	}
	ports, err := c.pickCrashPorts()
	if err != nil {
		return report, err
	}

	console, err := c.run.startConsole(image, ports.ssh)
	if err != nil {
		return report, err
	}

	vm := exec.CommandContext(context.Background(), qemuBinary(c.run.Arch), c.qemuArgs(image, ports, plan)...) //nolint:gosec // the argv is built by this package, never by an operator
	vm.Stdout = console
	vm.Stderr = console
	if err := vm.Start(); err != nil {
		console.Close() //nolint:errcheck // the start failure is what the caller reads
		return report, err
	}

	report, failure := c.exercise(report, ports)
	stopVM(vm)
	console.Close() //nolint:errcheck // the VM has stopped writing to it by now

	if failure == "" {
		report.Verdict = VerdictPass
		return report, nil
	}
	return c.failed(report, console.Name(), failure), nil
}

// failed answers the verdict for a run that did not reach its proof. Under a
// hardware accelerator that is a failure. Only software emulation can skip, and
// only before the first boot answered, because an appliance that answered once
// has shown the machine is fast enough.
func (c *CrashCapture) failed(report CrashCaptureReport, consolePath, failure string) CrashCaptureReport {
	if report.Readiness == nil && !Hardware(report.Accelerator) {
		var tb textbuf.Buffer
		report.Verdict = VerdictSkip
		report.Reason = tb.Str("appliance did not answer within the timeout (accel=").
			Str(report.Accelerator).Str(", software emulation); ").Str(failure).String()
		return report
	}
	report.Verdict = VerdictFail
	report.Reason = failure
	report.ConsoleTail = consoleTail(consolePath)
	return report
}

// exercise runs the lab against a booted VM and answers the report and, when
// the proof did not hold, the reason.
func (c *CrashCapture) exercise(report CrashCaptureReport, ports crashPorts) (CrashCaptureReport, string) {
	listing, failure := c.askArmed(ports.ssh)
	if failure != "" {
		return report, failure
	}
	report.Readiness = listing.readiness
	if names := listing.kernelArtifacts(); len(names) > 0 {
		return report, "a kernel artifact was present before any panic: " + names[0]
	}

	before, failure := c.bootTime(ports.ssh, 0)
	if failure != "" {
		return report, failure
	}
	report.BootBefore = before

	if failure := c.reboot(ports); failure != "" {
		return report, failure
	}

	after, failure := c.bootTime(ports.ssh, before)
	if failure != "" {
		return report, failure
	}
	report.BootAfter = after

	listing, failure = c.askArmed(ports.ssh)
	if failure != "" {
		return report, "after the reboot: " + failure
	}
	return c.judge(report, listing, ports.ssh)
}

// reboot performs the lab's reboot and answers why it could not.
func (c *CrashCapture) reboot(ports crashPorts) string {
	switch c.lab {
	case CrashLabPanicHarvest:
		c.run.note("injecting an NMI through the QEMU monitor...")
		return injectNMI(ports.monitor)
	case CrashLabOTAUnaffected:
		c.run.note("asking gokrazy's update server for an OTA reboot...")
		return otaReboot(ports.http)
	case crashLabUnspecified:
		panic("BUG: crash-capture lab unspecified")
	}
	panic("BUG: invalid crash-capture lab")
}

// judge answers the verdict over the second boot's listing.
func (c *CrashCapture) judge(report CrashCaptureReport, listing crashListing, port int) (CrashCaptureReport, string) {
	artifacts := listing.kernelArtifacts()
	switch c.lab {
	case CrashLabOTAUnaffected:
		if len(artifacts) > 0 {
			return report, "a clean OTA reboot produced a kernel artifact: " + artifacts[0]
		}
		report.Reason = "the OTA reboot came back armed with no kernel artifact"
		return report, ""
	case CrashLabPanicHarvest:
		if len(artifacts) == 0 {
			return report, "the boot after the panic harvested no kernel artifact into " + crashDirectory
		}
		report.Artifact = artifacts[0]
		text, failure := c.askText(port, "show crashes name "+artifacts[0])
		if failure != "" {
			return report, failure
		}
		line := lineWith(text, panicText)
		if line == "" {
			return report, "the kernel artifact " + artifacts[0] + " carries no `" + panicText + "` line"
		}
		report.PanicLine = line
		report.Reason = "the panic record survived the warm reset and was harvested into " + crashDirectory
		return report, ""
	case crashLabUnspecified:
		panic("BUG: crash-capture lab unspecified")
	}
	panic("BUG: invalid crash-capture lab")
}

// crashListing is the part of `show crashes | json` the proofs read.
type crashListing struct {
	readiness map[string]any
	crashes   []any
}

// kernelArtifacts answers the names of the listed kernel-kind artifacts.
func (l crashListing) kernelArtifacts() []string {
	var names []string
	for _, entry := range l.crashes {
		crash, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if crash["kind"] != kernelKind {
			continue
		}
		name, ok := crash["name"].(string)
		if ok {
			names = append(names, name)
		}
	}
	return names
}

// askArmed asks CrashesQuery until readiness reports the region armed in the
// expected directory, or the deadline passes, and answers the last reason it
// was not.
//
// A refused connection means the appliance is still booting, and an answer
// that is not armed yet may be a daemon still starting. Both are retried. An
// armed answer in the wrong directory is final: the harvest lands there.
func (c *CrashCapture) askArmed(port int) (crashListing, string) {
	end := time.Now().Add(c.run.Deadline)
	var lastError string
	for {
		listing, reason, final := c.readListing(port)
		if reason == "" {
			return listing, ""
		}
		if final {
			return listing, reason
		}
		lastError = reason
		if !time.Now().Before(end) {
			return listing, "`" + CrashesQuery + "` never reported armed: " + lastError
		}
		time.Sleep(sshRetryPause)
	}
}

// readListing asks CrashesQuery once. It answers the listing, the reason it is
// not yet acceptable, and whether that reason is final.
func (c *CrashCapture) readListing(port int) (crashListing, string, bool) {
	var listing crashListing
	out, stderr, err := c.run.ssh(port, CrashesQuery)
	if err != nil {
		return listing, lastLine(stderr, out, err), false
	}
	var document any
	if err := json.Unmarshal([]byte(out), &document); err != nil {
		return listing, "answered, not JSON: " + lastLine("", out, err), false
	}
	readiness, ok := findKey(document, "readiness", findDepthMax).(map[string]any)
	if !ok {
		return listing, "the answer carries no readiness block", false
	}
	listing.readiness = readiness
	listing.crashes, _ = findKey(document, "crashes", findDepthMax).([]any) //nolint:errcheck // an absent list is an empty directory
	if readiness["pstore-available"] != true {
		return listing, "pstore is not available: " + reasonOf(readiness), true
	}
	if readiness["armed"] != true {
		return listing, "readiness is not armed: " + reasonOf(readiness), true
	}
	if readiness["directory"] != crashDirectory {
		return listing, "the crash directory is not " + crashDirectory, true
	}
	if readiness["directory-writable"] != true {
		return listing, "the crash directory " + crashDirectory + " is not writable", true
	}
	return listing, "", false
}

// bootTime asks the kernel's boot time until it answers one different from
// previous, and answers it. A zero previous accepts the first answer.
func (c *CrashCapture) bootTime(port int, previous int64) (int64, string) {
	end := time.Now().Add(c.run.Deadline)
	lastError := "no answer yet"
	for {
		out, stderr, err := c.run.ssh(port, KernelQuery)
		if err != nil {
			lastError = lastLine(stderr, out, err)
		} else if booted, ok := bootTimeOf(out); ok && booted != previous {
			return booted, ""
		}
		if !time.Now().Before(end) {
			if previous == 0 {
				return 0, "`" + KernelQuery + "` never answered a boot time: " + lastError
			}
			return 0, "the appliance never came back from the reboot: " + lastError
		}
		time.Sleep(sshRetryPause)
	}
}

// askText asks one question until it answers, and answers the text.
func (c *CrashCapture) askText(port int, query string) (string, string) {
	out, lastError := c.run.ask(port, query)
	if out == "" {
		return "", "`" + query + "` never answered: " + lastError
	}
	return out, ""
}

// qemuArgs answers the hugepage proof's VM arguments plus a monitor this run
// can inject an NMI through and a forward to gokrazy's update server.
func (c *CrashCapture) qemuArgs(image string, ports crashPorts, plan HugepagesReport) []string {
	argv := c.run.qemuArgs(image, ports.ssh, plan)
	for i := range argv {
		if argv[i] != "-nic" {
			continue
		}
		if i+1 < len(argv) {
			var tb textbuf.Buffer
			argv[i+1] = tb.Str(argv[i+1]).Str(",hostfwd=tcp::").Int(int64(ports.http)).
				Str("-:").Int(gokrazyHTTPPort).String()
		}
	}
	var tb textbuf.Buffer
	monitor := tb.Str("tcp:127.0.0.1:").Int(int64(ports.monitor)).Str(",server=on,wait=off").String()
	return append(argv, "-monitor", monitor)
}

// injectNMI sends the monitor's nmi command, which delivers an NMI to every
// virtual CPU, and answers why it could not.
func injectNMI(port int) string {
	var tb textbuf.Buffer
	address := tb.Str("127.0.0.1:").Int(int64(port)).String()

	var dialer net.Dialer
	ctx, cancel := context.WithTimeout(context.Background(), sshAttemptMax)
	defer cancel()

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return "the QEMU monitor did not answer: " + err.Error()
	}
	defer conn.Close() //nolint:errcheck // the command was written or the write error is reported

	if err := conn.SetDeadline(time.Now().Add(sshAttemptMax)); err != nil {
		return "the QEMU monitor connection: " + err.Error()
	}
	if _, err := conn.Write([]byte("nmi\n")); err != nil {
		return "writing nmi to the QEMU monitor: " + err.Error()
	}
	// The monitor executes a command line once it has read it. Reading the
	// prompt that follows proves it did before the connection closes.
	reply := make([]byte, 512)
	if _, err := conn.Read(reply); err != nil {
		return "the QEMU monitor did not acknowledge nmi: " + err.Error()
	}
	return ""
}

// otaReboot asks gokrazy's update server for the reboot `gok update` performs
// after writing the new partitions, and answers why it could not. The plain
// POST is the kexec reboot: the updater adds kexec=off only to avoid it.
//
// The connection may drop as the machine goes down, so a transport error is
// not a failure here. Whether the machine rebooted is what the boot time
// answers next.
func otaReboot(port int) string {
	password, err := gokrazyPassword()
	if err != nil {
		return "the gokrazy update password: " + err.Error()
	}

	var tb textbuf.Buffer
	url := tb.Str("http://127.0.0.1:").Int(int64(port)).Str("/reboot").String()
	ctx, cancel := context.WithTimeout(context.Background(), sshAttemptMax)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, http.NoBody)
	if err != nil {
		return err.Error()
	}
	request.SetBasicAuth(gokrazyUser, password)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close() //nolint:errcheck // the status is what this reads

	if response.StatusCode != http.StatusOK {
		tb.Reset()
		return tb.Str("gokrazy refused the reboot: HTTP ").Int(int64(response.StatusCode)).String()
	}
	return ""
}

// gokrazyPassword answers the update password the image was built with. gok
// reads it from the host-specific file under the user configuration directory
// and falls back to the global one (github.com/gokrazy/internal/config,
// HostnameDir.ReadFile); this reads the same files in the same order.
func gokrazyPassword() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(base, "gokrazy", "hosts", ApplianceName, "http-password.txt"),
		filepath.Join(base, "gokrazy", "hosts", "ze", "http-password.txt"),
		filepath.Join(base, "gokrazy", "http-password.txt"),
	}
	for _, path := range candidates {
		raw, err := os.ReadFile(path) //nolint:gosec // a fixed path under the user configuration directory
		if err == nil {
			return strings.TrimSpace(string(raw)), nil
		}
	}
	return "", errors.New("no http-password.txt under " + filepath.Join(base, "gokrazy"))
}

// findDepthMax bounds findKey. The documents it walks are the appliance's own
// answers, a few levels deep.
const findDepthMax = 8

// findKey answers the first value stored under key anywhere in document, at
// most depth levels down, or nil. The recursion is bounded by depth.
func findKey(document any, key string, depth int) any {
	if depth == 0 {
		return nil
	}
	switch node := document.(type) {
	case map[string]any:
		if value, ok := node[key]; ok {
			return value
		}
		for _, child := range node {
			if found := findKey(child, key, depth-1); found != nil {
				return found
			}
		}
	case []any:
		for _, child := range node {
			if found := findKey(child, key, depth-1); found != nil {
				return found
			}
		}
	}
	return nil
}

// bootTimeOf answers the boot-time-unix field of `show host kernel | json`.
func bootTimeOf(out string) (int64, bool) {
	var document any
	if err := json.Unmarshal([]byte(out), &document); err != nil {
		return 0, false
	}
	seconds, ok := findKey(document, "boot-time-unix", findDepthMax).(float64)
	if !ok {
		return 0, false
	}
	return int64(seconds), true
}

// reasonOf answers the readiness block's reason field, or says it gave none.
func reasonOf(readiness map[string]any) string {
	reason, ok := readiness["reason"].(string)
	if !ok {
		return "(no reason given)"
	}
	return reason
}

// lineWith answers the first line of text that contains needle, trimmed.
func lineWith(text, needle string) string {
	for line := range strings.SplitSeq(text, "\n") {
		if strings.Contains(line, needle) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}
