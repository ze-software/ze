// Design: docs/architecture/testing/qemu-integration.md -- a proof that boots an image
// Overview: actions.go -- the area table that reaches this run
// Related: hugepages.go -- the proof whose settings and machinery this run reuses
// Related: boot.go -- the virtual machine and the SSH it is asked over
//
// mplsboot.go proves that an appliance image starts Ze with MPLS configured.
//
// Ze refuses to start when the configuration uses a kernel capability the
// running kernel lacks (internal/component/kernelcap). MPLS counts as in use
// for a configuration that carries `fib { kernel }` and `ldp`. The appliance
// kernel must therefore carry CONFIG_MPLS_ROUTING, built in, or every appliance
// that runs LDP would refuse its own boot. This run builds an image whose seed
// configuration is the default appliance configuration plus `set fib kernel`
// and `set ldp`, boots it, and asks the Ze CLI `show ldp neighbor | json`.
//
// The appliance's SSH server is the Ze daemon. A daemon that refused to start
// never answers, so an answer proves the gate passed. The question is one only
// the LDP engine answers, so the answer also proves that the configuration the
// gate judged is the one that put MPLS in use, rather than a default that did
// not.
//
// The self-skip contract is the hugepage proof's: a machine without QEMU,
// sshpass, e2fsprogs or Go answers SKIP, and an appliance that never answers is
// a SKIP only under software emulation.

package testqemu

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// MPLSQuery is the one question the MPLS boot proof asks. Only the LDP engine
// answers it, and the LDP engine runs only when the configuration enables LDP.
const MPLSQuery = "show ldp neighbor | json"

// MPLSReportPrefix opens every line the MPLS boot proof prints. It is the word
// the functional suite greps for.
const MPLSReportPrefix = "APPLIANCE-MPLS-QEMU: "

// mplsSeedLines are what the proof adds to the default appliance seed. Together
// they make MPLS in use: `fib kernel` programs the kernel's label table and
// `ldp` allocates labels into it.
const mplsSeedLines = "set fib kernel\nset ldp\n"

// defaultSeedPath is the seed an appliance boots with when it names none. The
// proof starts from it so the appliance keeps its SSH server and DHCP.
var defaultSeedPath = filepath.Join("gokrazy", "ze", "ze.conf")

// MPLSBootReport is one run of the appliance MPLS boot proof.
//
// Answer carries what the LDP engine said. ConsoleTail carries the serial
// console only when the appliance never answered under a hardware accelerator,
// where a refused start leaves its reason.
type MPLSBootReport struct {
	Verdict     Verdict  `json:"verdict"`
	Reason      string   `json:"reason,omitempty"`
	Arch        string   `json:"arch"`
	Accelerator string   `json:"accelerator"`
	Seed        string   `json:"seed"`
	Answer      string   `json:"answer,omitempty"`
	ConsoleTail []string `json:"console-tail,omitempty"`
}

// Text renders the run for a person: one prefixed verdict line, then the
// serial console of an appliance that never answered.
func (r MPLSBootReport) Text() string {
	var tb textbuf.Buffer
	tb.Str(MPLSReportPrefix)

	switch r.Verdict {
	case VerdictPass:
		tb.Str("PASS ze started with MPLS in use; `").Str(MPLSQuery).Str("` answered\n")
		return tb.String()
	case VerdictSkip:
		tb.Str("SKIP ").Str(r.Reason).Byte('\n')
		return tb.String()
	case VerdictFail, VerdictUnspecified:
		tb.Str("FAIL ").Str(r.Reason).Byte('\n')
	default:
		panic("BUG: invalid MPLS boot verdict")
	}

	if len(r.ConsoleTail) > 0 {
		tb.Str("  serial console tail:\n")
		for _, line := range r.ConsoleTail {
			tb.Str("    ").Str(line).Byte('\n')
		}
	}
	return tb.String()
}

// MPLSBoot is one run of the appliance MPLS boot proof.
//
// It holds a hugepage proof for that proof's settings (architecture, memory,
// password, port, firmware, deadline) and for its machinery (prerequisites,
// host build, appliance commands, QEMU arguments, SSH). The image it builds
// and the question it asks are its own.
type MPLSBoot struct {
	run *Hugepages
}

// newMPLSBoot answers the run the command performs over tree, with every
// setting taken from the environment or from its default.
func newMPLSBoot(tree string) *MPLSBoot {
	return &MPLSBoot{run: newHugepages(tree)}
}

// Run performs the proof and answers what happened. A missing prerequisite is
// a SKIP, and a step the run cannot perform is an error.
func (m *MPLSBoot) Run() (MPLSBootReport, error) {
	report := MPLSBootReport{Arch: m.run.Arch, Accelerator: accelerator()}

	if reason := m.run.missingPrerequisite(); reason != "" {
		report.Verdict = VerdictSkip
		report.Reason = reason
		return report, nil
	}

	seed, err := m.seed()
	if err != nil {
		return report, err
	}
	report.Seed = seed

	work, err := m.workDir()
	if err != nil {
		return report, err
	}
	if !m.run.Keep {
		defer os.RemoveAll(work) //nolint:errcheck // the run's verdict is what the caller reads
	}

	m.run.note("building the host ze...")
	host, err := m.run.buildHostZe(work)
	if err != nil {
		return report, err
	}

	m.run.note("building the appliance image with an MPLS seed...")
	image, err := m.buildImage(host, work, seed)
	if err != nil {
		return report, err
	}

	m.run.note("booting the appliance...")
	return m.bootAndAssert(report, image)
}

// seed answers the configuration the appliance boots with: the default seed
// followed by the lines that put MPLS in use. An appliance overlay replaces the
// default seed rather than adding to it, so the default is carried here.
func (m *MPLSBoot) seed() (string, error) {
	base, err := os.ReadFile(filepath.Join(m.run.Tree, defaultSeedPath)) //nolint:gosec // a fixed path in the checkout
	if err != nil {
		return "", err
	}
	var tb textbuf.Buffer
	return tb.Str(string(base)).Byte('\n').Str(mplsSeedLines).String(), nil
}

// workDir answers a fresh directory for one run's files, under the checkout's
// scratch tree for the reason the hugepage proof's workDir gives.
func (m *MPLSBoot) workDir() (string, error) {
	parent := filepath.Join(m.run.Tree, "tmp", "appliance-mpls-qemu")
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return "", err
	}
	return os.MkdirTemp(parent, "run-")
}

// buildImage initializes an appliance, writes its architecture, memory and
// seed configuration, and builds the image.
func (m *MPLSBoot) buildImage(host, work, seed string) (string, error) {
	dir := filepath.Join(work, "appliances")
	environment := m.run.applianceEnv(dir)

	if out, err := m.run.appliance(host, environment, "init"); err != nil {
		return "", applianceStepError("init", "", out, err)
	}
	applianceDir := filepath.Join(dir, ApplianceName)
	err := editImageConfig(filepath.Join(applianceDir, "appliance.json"), func(image map[string]any) {
		image["arch"] = m.run.Arch
		image["memory"] = m.run.Memory
	})
	if err != nil {
		return "", err
	}
	// The overlay resolveSeedConfig (internal/appliance/cmd_assemble.go) reads.
	if err := os.WriteFile(filepath.Join(applianceDir, "ze.conf"), []byte(seed), 0o600); err != nil {
		return "", err
	}
	out, err := m.run.appliance(host, environment, zeApplianceVerbBuild)
	if err != nil {
		return "", applianceStepError(zeApplianceVerbBuild, buildHint(out), out, err)
	}
	return findImage(dir)
}

// bootAndAssert boots the image, asks the LDP question, and answers the
// verdict. The VM is stopped on every path out of this function.
func (m *MPLSBoot) bootAndAssert(report MPLSBootReport, image string) (MPLSBootReport, error) {
	plan, err := m.run.plan()
	if err != nil {
		return report, err
	}

	port := m.run.SSHPort
	if port == 0 {
		picked, err := freePort()
		if err != nil {
			return report, err
		}
		port = picked
	}
	if m.run.Arch == ArchARM64 {
		if !isFile(m.run.Bios) {
			report.Verdict = VerdictSkip
			report.Reason = firmwareSkipReason(m.run.Bios)
			return report, nil
		}
	}

	console, err := m.run.startConsole(image, port)
	if err != nil {
		return report, err
	}
	answer, lastError, err := m.boot(m.run.qemuArgs(image, port, plan), port, console)
	console.Close() //nolint:errcheck // the VM has stopped writing to it by now
	if err != nil {
		return report, err
	}

	if answer != "" {
		report.Verdict = VerdictPass
		report.Answer = answer
		return report, nil
	}
	return mplsNoAnswer(report, console.Name(), lastError), nil
}

// boot runs the VM for as long as the question takes and answers the JSON the
// LDP engine gave, or the last reason it gave none.
func (m *MPLSBoot) boot(argv []string, port int, console *os.File) (string, string, error) {
	vm := exec.CommandContext(context.Background(), qemuBinary(m.run.Arch), argv...) //nolint:gosec // the argv is built by this package, never by an operator
	vm.Stdout = console
	vm.Stderr = console
	if err := vm.Start(); err != nil {
		return "", "", err
	}
	defer stopVM(vm)

	answer, lastError := m.askJSON(port)
	return answer, lastError, nil
}

// askJSON asks MPLSQuery until it answers a JSON document or the deadline
// passes.
//
// A refused connection means the appliance is still booting. A connected
// session that answers text that is not JSON means the daemon is up and the
// LDP engine is not yet, or never will be. Both are retried, and the last one
// seen is what a failure reports, so the reader can tell them apart.
func (m *MPLSBoot) askJSON(port int) (string, string) {
	end := time.Now().Add(m.run.Deadline)
	var lastError string
	for {
		out, stderr, err := m.run.ssh(port, MPLSQuery)
		switch {
		case err != nil:
			lastError = lastLine(stderr, out, err)
		case json.Valid([]byte(out)):
			return out, ""
		default:
			var tb textbuf.Buffer
			lastError = tb.Str("answered, not JSON: ").Str(lastLine("", out, errors.New("empty answer"))).String()
		}
		if !time.Now().Before(end) {
			return "", lastError
		}
		time.Sleep(sshRetryPause)
	}
}

// mplsNoAnswer answers the verdict for an appliance whose LDP engine never
// answered. Under a hardware accelerator that is a failure: either the daemon
// refused to start, which the console tail shows, or LDP did not run. Only
// software emulation can still skip on it.
func mplsNoAnswer(report MPLSBootReport, consolePath, lastError string) MPLSBootReport {
	detail := "no ssh error captured"
	if lastError != "" {
		var tb textbuf.Buffer
		detail = tb.Str("last ssh error: ").Str(lastError).String()
	}

	var tb textbuf.Buffer
	if Hardware(report.Accelerator) {
		report.Verdict = VerdictFail
		report.Reason = tb.Str("`").Str(MPLSQuery).Str("` never answered (accel=").
			Str(report.Accelerator).Str("); ").Str(detail).String()
		report.ConsoleTail = consoleTail(consolePath)
		return report
	}

	report.Verdict = VerdictSkip
	report.Reason = tb.Str("appliance did not answer within the timeout (accel=").
		Str(report.Accelerator).Str(", software emulation); ").Str(detail).String()
	return report
}
