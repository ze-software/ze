// Design: docs/architecture/testing/qemu-integration.md -- a proof that boots an image
// Overview: actions.go -- the area table that reaches this run
// Detail: boot.go -- the virtual machine and the two questions
// Detail: report.go -- the payload this run answers
//
// hugepages.go proves boot-time hugepage reservation end to end. It builds a
// host ze and then initializes an appliance. The configuration includes
// image.hugepages and image.memory. Next, it builds the gokrazy image. This
// build exercises the derived-instance-config kernel-argument path in
// internal/appliance/kernelargs.go. Finally, it boots that image in QEMU and
// asks the running appliance two questions over SSH.
//
// The proof sends both questions through the Ze CLI. The appliance's SSH server
// IS that CLI, not a Unix shell. Thus, `cat /proc/cmdline` returns "error:
// unknown command". `show host kernel` proves that the baked command line has
// the derived kernel arguments. `show host memory` proves that the kernel
// honored the reservation.
//
// The memory check can fail on a machine with insufficient contiguous memory.
// That failure is why the command-line check is not sufficient.
//
// SELF-SKIP CONTRACT. If a machine has no QEMU, no sshpass, no e2fsprogs, or no
// Go toolchain, the run returns SKIP and exits 0. The run is therefore safe in a
// suite and in CI that lacks the artifacts. An appliance that never answers is
// a SKIP only under software emulation. Under a hardware accelerator, it is a
// FAILURE. It stayed broken once already because it was reported as a skip.

package qemu

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The dot-notation spellings of the ZE_VPP_HP_* variables the Python original
// read. env.Get matches case-insensitively and treats a dot and an underscore
// as the same character, so these keys read the same variables.
const (
	ArchKey        = "ze.vpp.hp.arch"
	PageSizeKey    = "ze.vpp.hp.pagesize"
	ReservationKey = "ze.vpp.hp.size"
	MemoryKey      = "ze.vpp.hp.memory"
	SSHPassKey     = "ze.vpp.hp.ssh.pass" //nolint:gosec // the NAME of a variable, not a password
	SSHPortKey     = "ze.vpp.hp.ssh.port"
	KeepKey        = "ze.vpp.hp.keep"
	BiosKey        = "ze.vpp.hp.aarch64.bios"
)

// What the run uses when the operator names nothing.
//
// The reservation is 128 MB of 2 MB pages, or 64 pages. This size is small
// enough that a laptop can reserve it at boot and large enough to make the
// kernel's answer unambiguous. The appliance gets 1 GB because the reservation
// comes out of it.
const (
	DefaultPageSize    = "2mb"
	DefaultReservation = "128mb"
	DefaultMemory      = "1gb"
	DefaultSSHPass     = "secret"
	DefaultBios        = "/usr/share/qemu/edk2-aarch64-code.fd"
)

// SSHUser is the account the appliance image creates, and the only one this
// proof can log in as.
const SSHUser = "admin"

// The two architectures an appliance image is built for. They are the words the
// appliance configuration spells, so they are named once and compared against
// rather than repeated.
const (
	ArchARM64 = "arm64"
	ArchAMD64 = "amd64"
)

// ApplianceName is what the appliance built for this proof is called. It is
// fixed rather than derived, because the run makes its own directory for each
// invocation and two runs therefore never share one.
const ApplianceName = "ze-vpp-hp-qemu"

// memoryMiBMin is the minimum memory allocation for QEMU. A Linux kernel does
// not finish booting with less memory. A configuration that asks for less
// therefore receives this minimum instead of a VM that cannot start.
const memoryMiBMin = 256

// The bounds on each step. The image build is the long one: it resolves the
// gokrazy system packages and writes a multi-gigabyte image.
const (
	hostBuildTimeout = 20 * time.Minute
	applianceTimeout = 45 * time.Minute
	sshAttemptMax    = time.Minute
	shutdownGrace    = 10 * time.Second
)

// AnswerDeadline is the maximum time for one appliance CLI response over SSH.
// sshRetryPause is the interval between attempts. Both values match those in
// the Python original. A cold boot under software emulation takes minutes. A
// connection refusal while the kernel starts is expected, not a failure.
const (
	AnswerDeadline = 180 * time.Second
	sshRetryPause  = 3 * time.Second
)

// consoleTailLines is how many of the appliance's last serial console lines a
// failure reports. The console carries a whole boot, and the lines that explain
// a failure to answer are the last ones.
const consoleTailLines = 25

// stringSetting registers one of this package's variables and answers its entry.
//
// Private keeps every one of them out of `ze env list`. They are le's
// variables, and an operator reading an appliance has no QEMU invocation to
// point one at.
func stringSetting(key, fallback, description string) env.EnvEntry {
	return env.MustRegister(env.EnvEntry{
		Key:         key,
		Type:        "string",
		Default:     fallback,
		Description: description,
		Private:     true,
	})
}

var (
	archEntry = stringSetting(ArchKey, "",
		"the target architecture the hugepage proof builds and boots, defaulting to the host's")
	pageSizeEntry = stringSetting(PageSizeKey, DefaultPageSize,
		"the hugepage size the proof reserves, 2mb or 1gb")
	reservationEntry = stringSetting(ReservationKey, DefaultReservation,
		"how much memory the hugepage proof reserves in total")
	memoryEntry = stringSetting(MemoryKey, DefaultMemory,
		"how much memory the appliance the hugepage proof boots is given")
	sshPassEntry = stringSetting(SSHPassKey, DefaultSSHPass,
		"the password the hugepage proof logs in to the booted appliance with")
	sshPortEntry = stringSetting(SSHPortKey, "",
		"the host port the hugepage proof forwards to the appliance's SSH server, or a free one")
	keepEntry = stringSetting(KeepKey, "",
		"set to 1 to keep the hugepage proof's work directory for inspection")
	biosEntry = stringSetting(BiosKey, DefaultBios,
		"where the aarch64 UEFI firmware the hugepage proof boots arm64 images with lives")
)

// pageTokens maps a configured page size onto the token a Linux kernel command
// line spells it with. A size this table does not hold is refused, because the
// kernel has no other spelling and a guess would reach a boot argument.
var pageTokens = map[string]string{"2mb": "2M", "1gb": "1G"}

// Hugepages is one run of the boot-time hugepage reservation proof.
type Hugepages struct {
	// Tree is the checkout the host ze is built from.
	Tree string
	// Arch is the architecture the image is built and booted for.
	Arch string
	// PageSize, Reservation and Memory are the operator's own spellings, and
	// they travel unchanged into the appliance configuration.
	PageSize    string
	Reservation string
	Memory      string
	// SSHPass is the appliance's password, and SSHPort the host port forwarded
	// to its SSH server. A zero port means one this run picks.
	SSHPass string
	SSHPort int
	// Bios is the aarch64 UEFI firmware, read only for an arm64 image.
	Bios string
	// Keep leaves the work directory behind for inspection.
	Keep bool
	// Deadline bounds how long the appliance gets to answer over SSH.
	Deadline time.Duration
	// Progress receives each step's narration and every build's output. It is
	// stderr for an operator, because the answer is the report and a pipe
	// operator must be able to carry it.
	Progress io.Writer
}

// NewHugepages answers the run the command performs over tree, with every
// setting taken from the environment or from its default.
func NewHugepages(tree string) *Hugepages {
	return &Hugepages{
		Tree:        tree,
		Arch:        settingOr(archEntry.Key, HostArch()),
		PageSize:    settingOr(pageSizeEntry.Key, DefaultPageSize),
		Reservation: settingOr(reservationEntry.Key, DefaultReservation),
		Memory:      settingOr(memoryEntry.Key, DefaultMemory),
		SSHPass:     settingOr(sshPassEntry.Key, DefaultSSHPass),
		SSHPort:     portSetting(sshPortEntry.Key),
		Bios:        settingOr(biosEntry.Key, DefaultBios),
		Keep:        env.Get(keepEntry.Key) == "1",
		Deadline:    AnswerDeadline,
		Progress:    os.Stderr,
	}
}

// settingOr answers the operator's value for key. If the operator named
// nothing, it answers fallback. env.Get answers the empty string for an unset
// variable instead of the registered default. This function therefore applies
// the default at the one place that reads the variable.
func settingOr(key, fallback string) string {
	if named := env.Get(key); named != "" {
		return named
	}
	return fallback
}

// portSetting answers the port the operator named, or zero for a port this run
// picks. A value that is not a port number is read as none, which is what the
// Python's int(... or 0) did.
func portSetting(key string) int {
	named := env.Get(key)
	if named == "" {
		return 0
	}
	value, err := ParseSize(joinDigits(named))
	if err != nil || value == 0 || value > 65535 {
		return 0
	}
	return int(value)
}

// joinDigits answers the setting with a byte unit appended, so the one
// unsigned-decimal reader in this package can read a port too.
func joinDigits(named string) string {
	var tb textbuf.Buffer
	return tb.Str(named).Byte('b').String()
}

// HostArch answers the architecture this machine is, in the two spellings the
// appliance configuration accepts.
func HostArch() string {
	if runtime.GOARCH == ArchARM64 {
		return ArchARM64
	}
	return ArchAMD64
}

// QemuBinary answers the QEMU system emulator for an architecture.
func QemuBinary(arch string) string {
	if arch == ArchARM64 {
		return "qemu-system-aarch64"
	}
	return "qemu-system-x86_64"
}

// Accelerator answers the hypervisor QEMU is asked for on this machine.
//
// macOS has no /dev/kvm and uses the Apple hypervisor. On Linux, the existence
// of /dev/kvm does not guarantee usability. The node is root:kvm 0660. A user
// outside the kvm group can see the node.
//
// QEMU then reports a KVM permission error and does not use the fallback.
// Accelerator opens the node for reading and writing, as QEMU itself does. It
// then closes the node.
func Accelerator() string {
	if runtime.GOOS == "darwin" {
		return "hvf"
	}
	node, err := os.OpenFile("/dev/kvm", os.O_RDWR, 0)
	if err != nil {
		return "tcg"
	}
	node.Close() //nolint:errcheck // the probe is the open; nothing was written
	return "kvm"
}

// freePort answers a TCP port that has no listener on this machine. It binds a
// port and then releases it.
//
// A race exists between that release and QEMU's bind. The Python original had
// the same race. This implementation accepts the race. The alternative would
// keep the listener open and give QEMU a descriptor. QEMU's user-mode
// networking has no way to take that descriptor.
func freePort() (int, error) {
	var config net.ListenConfig
	listener, err := config.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close() //nolint:errcheck // the port is released deliberately

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, errors.New("the port probe did not answer a TCP address")
	}
	return addr.Port, nil
}

// Hardware reports whether an accelerator is a hypervisor rather than software
// emulation. It is what decides whether an appliance that never answered is a
// failure or a slow machine.
func Hardware(accelerator string) bool {
	return accelerator == "kvm" || accelerator == "hvf"
}

// Run performs the proof and answers what happened.
//
// A missing machine prerequisite causes a SKIP, not an error. The functional
// suite and CI rely on this contract. A setting that is not a quantity IS an
// error. It is the operator's own mistake. A skip would hide the fact that
// nothing was proven.
func (h *Hugepages) Run() (HugepagesReport, error) {
	report, err := h.plan()
	if err != nil {
		return report, err
	}

	if reason := h.missingPrerequisite(); reason != "" {
		report.Verdict = VerdictSkip
		report.Reason = reason
		return report, nil
	}

	work, err := h.workDir()
	if err != nil {
		return report, err
	}
	if !h.Keep {
		defer os.RemoveAll(work) //nolint:errcheck // the run's verdict is what the caller reads
	}

	h.note("building the host ze...")
	host, err := h.buildHostZe(work)
	if err != nil {
		return report, err
	}

	h.note("building the appliance image...")
	image, err := h.buildImage(host, work)
	if err != nil {
		return report, err
	}

	h.note("booting the appliance...")
	return h.bootAndAssert(report, image)
}

// note writes one line of narration to the progress stream.
//
// The three steps this run performs take twenty minutes, forty-five minutes and
// three minutes, and the Python original printed nothing during any of them. The
// report is the answer and it reaches standard output, so the narration reaches
// the other stream where a pipe operator cannot see it.
func (h *Hugepages) note(line string) {
	if h.Progress == nil {
		return
	}
	var tb textbuf.Buffer
	io.WriteString(h.Progress, tb.Str(line).Byte('\n').String()) //nolint:errcheck // progress output
}

// plan answers the report with every value that the run ASKED for already
// present. A failure at any later step therefore still identifies the proof
// that was attempted.
func (h *Hugepages) plan() (HugepagesReport, error) {
	report := HugepagesReport{
		Arch:        h.Arch,
		Accelerator: Accelerator(),
		PageSize:    h.PageSize,
		Reservation: h.Reservation,
	}

	token, ok := pageTokens[lower(h.PageSize)]
	if !ok {
		var tb textbuf.Buffer
		return report, errors.New(tb.Str("the hugepage size must be 2mb or 1gb, got ").Quoted(h.PageSize).String())
	}
	report.PageToken = token

	pages, err := PageCount(h.Reservation, h.PageSize)
	if err != nil {
		return report, err
	}
	report.Pages = pages

	memory, err := ParseSize(h.Memory)
	if err != nil {
		return report, err
	}
	report.MemoryMiB = max(memory/(1024*1024), memoryMiBMin)

	return report, nil
}

// missingPrerequisite names the first tool this machine does not have, or
// nothing when it has them all.
//
// All four checks occur here instead of at each use site. If an operator lacks
// two tools, the operator learns about the first before a twenty-minute build,
// not after it.
func (h *Hugepages) missingPrerequisite() string {
	required := []struct {
		command string
		reason  string
	}{
		{"go", "go toolchain not found"},
		{QemuBinary(h.Arch), ""},
		{"sshpass", "sshpass not found (needed for non-interactive SSH assert)"},
		{"mkfs.ext4", "e2fsprogs (mkfs.ext4/debugfs) not found"},
		{"debugfs", "e2fsprogs (mkfs.ext4/debugfs) not found"},
	}
	for _, one := range required {
		if _, err := exec.LookPath(one.command); err == nil {
			continue
		}
		if one.reason != "" {
			return one.reason
		}
		var tb textbuf.Buffer
		return tb.Str(one.command).Str(" not found").String()
	}
	return ""
}

// workDir answers a fresh directory for one run's files.
//
// It lives under the checkout's scratch tree, not in the system temporary
// directory. This build writes a multi-gigabyte image. On some hosts, /tmp is a
// small tmpfs. On other hosts, it is on a different filesystem from the
// checkout. In either case, the operator cannot predict or clean the image
// location.
//
// A failed run leaves the image for inspection. Git ignores it, but the
// operator can see it.
func (h *Hugepages) workDir() (string, error) {
	parent := filepath.Join(h.Tree, "tmp", "vpp-hugepages-qemu")
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return "", err
	}
	return os.MkdirTemp(parent, "run-")
}

// buildHostZe compiles the ze that RUNS ON THIS MACHINE to drive the appliance
// commands.
//
// It is a host binary and is never cross-compiled. A target-architecture build
// cannot run here. The appliance configuration carries the image architecture
// instead.
//
// The setup surface provides `appliance init` and `appliance build`. Therefore,
// ze_setup is the tag beside ze_core. Feature gates select daemon features. They
// do not apply to the setup surface.
func (h *Hugepages) buildHostZe(work string) (string, error) {
	host := filepath.Join(work, "ze-host")

	ctx, cancel := context.WithTimeout(context.Background(), hostBuildTimeout)
	defer cancel()

	build := exec.CommandContext(ctx, "go", "build", "-tags", "ze_core,ze_setup", "-o", host, "./cmd/ze") //nolint:gosec // host is a path under the work directory this run made
	build.Dir = h.Tree
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := build.CombinedOutput()
	if err != nil {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("host ze build failed:\n").Str(string(out)).String())
	}
	return host, nil
}

// applianceEnv answers the environment both appliance commands run under: where
// the appliance lives, and the password its SSH server will accept.
func (h *Hugepages) applianceEnv(dir string) []string {
	var tb textbuf.Buffer
	applianceDir := tb.Str("ZE_APPLIANCE_DIR=").Str(dir).String()
	tb.Reset()
	password := tb.Str("ze.appliance.ssh.password=").Str(h.SSHPass).String()
	return append(os.Environ(), applianceDir, password)
}

// buildImage initializes an appliance, writes the hugepage reservation into its
// configuration, and builds the image.
func (h *Hugepages) buildImage(host, work string) (string, error) {
	dir := filepath.Join(work, "appliances")
	environment := h.applianceEnv(dir)

	if out, err := h.appliance(host, environment, "init"); err != nil {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("ze appliance init failed:\n").Str(out).String())
	}
	if err := h.writeApplianceConfig(filepath.Join(dir, ApplianceName, "appliance.json")); err != nil {
		return "", err
	}
	out, err := h.appliance(host, environment, "build")
	if err != nil {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("ze appliance build failed:").Str(buildHint(out)).Byte('\n').Str(out).String())
	}
	return findImage(dir)
}

// appliance runs one `ze appliance <verb> <name>` and answers what it wrote.
//
// Standard input is closed because `init` prompts when it has a terminal, and
// this run answers no prompt.
func (h *Hugepages) appliance(host string, environment []string, verb string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), applianceTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, host, "appliance", verb, ApplianceName) //nolint:gosec // host is a binary this run just built
	cmd.Env = environment
	cmd.Stdin = strings.NewReader("")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// writeApplianceConfig sets the image fields the proof is about, leaving every
// other field `appliance init` wrote exactly as it found it.
//
// The proof keeps the operator's spellings for both sizes. It does not convert
// them to byte counts. The appliance configuration parses those spellings, and
// this proof tests that path directly.
func (h *Hugepages) writeApplianceConfig(path string) error {
	raw, err := os.ReadFile(path) //nolint:gosec // a path under the work directory this run made
	if err != nil {
		return err
	}
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return err
	}

	image, _ := config["image"].(map[string]any)
	if image == nil {
		image = map[string]any{}
	}
	image["arch"] = h.Arch
	image["memory"] = h.Memory
	image["hugepages"] = map[string]any{"size": h.Reservation, "page-size": h.PageSize}
	config["image"] = image

	written, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return os.WriteFile(path, written, 0o600)
}

// findImage answers the image the appliance build wrote.
//
// A build that wrote no image is an error, not an empty answer. Otherwise, the
// run would boot nothing and report on a kernel command line that it never read.
func findImage(dir string) (string, error) {
	var found []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if strings.HasPrefix(name, "ze-") && strings.HasSuffix(name, ".img") {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(found) == 0 {
		return "", errors.New("the appliance build produced no image")
	}
	slices.Sort(found)
	return found[0], nil
}

// The three markers in a build's output that name an unpopulated module cache.
//
// `ze appliance build` uses the repository-local gokrazy/modcache and sets
// GOPROXY=off (internal/appliance/cmd_build.go, ensureModcache). If the download
// has never run, the cache lacks the kernel module and pinned Go toolchain.
// gok then reports "toolchain not available". That message incorrectly suggests
// a broken Go installation.
var modcacheMarkers = []string{"toolchain not available", "incomplete packages", "GOPROXY=off"}

// buildHint names the one-time setup step when a build's output shows it is
// missing. It does NOT skip: the prerequisite is one documented command away,
// and skipping would delete the only coverage of the boot-time reservation.
func buildHint(out string) string {
	for _, marker := range modcacheMarkers {
		if strings.Contains(out, marker) {
			return " (gokrazy/modcache looks unpopulated -- run `make ze-gokrazy-deps-download`" +
				" once, then retry)"
		}
	}
	return ""
}
