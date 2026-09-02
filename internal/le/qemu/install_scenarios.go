// Design: docs/architecture/testing/qemu-integration.md -- installer scenario evidence
// Overview: install.go -- the base primitives scenarios import
package qemu

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	InstallFaultRecoverMark = "recovered goroutine panic"
	InstallKernelPanicMark  = "Attempted to kill init"
	InstallFatalPolicyMark  = "fatal policy"
	InstallPinMark          = "pinning to boot NIC"
	InstallPinReachableMark = "server reachable on pinned NIC"
	InstallPinFlushMark     = "pinned NIC cannot reach server, flushing"
	InstallFallbackMark     = "bringing up all NICs"
	InstallTokenPrompt      = "rescue token:"
	InstallAuthOK           = "authenticated"
	InstallAuthBad          = "incorrect"
	InstallMenuMark         = "Recovery Console"
	InstallReboot30s        = "rebooting in 30s"

	// InstallAmbiguousTargetMark is the refusal findTargetDisk prints when more
	// than one fixed disk survives filtering and no ze.target names one
	// (internal/install/disk/detect.go).
	InstallAmbiguousTargetMark = "multiple target disks found"
	// InstallStreamMark is the line the installer prints as it starts writing
	// the image. Its ABSENCE is what proves the refusal above stopped the run
	// before any disk was touched (internal/install/disk/run.go, runHTTP).
	InstallStreamMark = "streaming image to disk"
)

// The installer scenario names. A name is the report's Name field and the
// selector the dispatch below matches, so the two cannot drift apart.
const (
	installScenarioFault      = "fault"
	installScenarioPinAC4     = "pin-ac4"
	installScenarioPinAC5     = "pin-ac5"
	installScenarioRescueAC7  = "rescue-ac7"
	installScenarioRescueAC7B = "rescue-ac7b"
	installScenarioRescueAC7C = "rescue-ac7c"
	installScenarioAmbiguous  = "target-ambiguous"
)

type installFixtures struct {
	Initrd string
	Image  string
	Port   int
	Server *installHTTPServer
}

func (fixtures *installFixtures) Close() error {
	if fixtures == nil || fixtures.Server == nil {
		return nil
	}
	return fixtures.Server.Stop()
}

func (installer *Installer) buildScenarioInitrd(ctx context.Context, work string, fault bool) (string, error) {
	cache := filepath.Join(work, "cache-normal")
	value := "ZE_INITRD_FAULT="
	if fault {
		cache, value = filepath.Join(work, "cache-fault"), "ZE_INITRD_FAULT=1"
	}
	var tb textbuf.Buffer
	return installer.buildInitrd(ctx, work, tb.Str("XDG_CACHE_HOME=").Str(cache).String(), value)
}

func (installer *Installer) setupInstallFixtures(ctx context.Context, work string) (*installFixtures, string, error) {
	if installer.Options.Image == "" {
		if _, err := installer.ops.Look("debugfs"); err != nil {
			if installer.brewDebugfs() == "" {
				return nil, "debugfs missing (install e2fsprogs) — needed to inject zefs into /perm", nil
			}
		}
	}
	initrd, err := installer.buildScenarioInitrd(ctx, work, false)
	if err != nil {
		return nil, "", err
	}
	image, err := installer.buildImage(ctx, work)
	if err != nil {
		return nil, "", err
	}
	served := filepath.Join(work, "served")
	if err := installer.ops.FS.MkdirAll(served, 0o750); err != nil {
		return nil, "", err
	}
	if _, err := installer.writeChecksum(image.Path, served); err != nil {
		return nil, "", err
	}
	if image.ZeFS == "" {
		return nil, "", errors.New("no database.zefs produced by appliance assemble")
	}
	if err := copyInstallFile(image.ZeFS, filepath.Join(served, "database.zefs")); err != nil {
		return nil, "", err
	}
	server, port, err := startInstallHTTP(ctx, served)
	if err != nil {
		return nil, "", err
	}
	return &installFixtures{Initrd: initrd, Image: image.Path, Port: port, Server: server}, "", nil
}

// FaultArgv returns the forced-panic invocation used by scenario fault.
func (installer *Installer) FaultArgv(initrd, disk string) ([]string, error) {
	base := installer.qemuBase(false)
	console := installConsoleAMD64
	if installer.Options.Arch == ArchARM64 {
		console = installConsoleARM64
	}
	var tb textbuf.Buffer
	line := tb.Str("console=").Str(console).Str(" ze.server=").Str(InstallGuestServerIP).
		Str(" ze.image=").Str(InstallImageName).
		Str(" ip=dhcp panic=-1 ze.fault=panic-goroutine").String()
	tb.Reset()
	drive := tb.Str("file=").Str(disk).Str(",format=raw,if=virtio").String()
	tb.Reset()
	device := tb.Str(installer.Options.NIC).Str(",netdev=net0").String()
	return append(base, "-no-reboot", "-kernel", installer.Options.Kernel,
		"-initrd", initrd, "-append", line,
		"-drive", drive, "-netdev", "user,id=net0", "-device", device), nil
}

func (installer *Installer) bootFault(ctx context.Context, initrd, disk string) (string, error) {
	argv, err := installer.FaultArgv(initrd, disk)
	if err != nil {
		return "", err
	}
	return installer.runCapture(ctx, argv, installer.Options.FaultTimeout)
}

func (installer *Installer) scenarioFault(ctx context.Context, work string) (InstallScenarioReport, string, error) {
	initrd, err := installer.buildScenarioInitrd(ctx, work, true)
	if err != nil {
		return InstallScenarioReport{}, "", err
	}
	disk := filepath.Join(work, "fault-target.img")
	if err := truncateInstallFile(disk, 64<<20); err != nil {
		return InstallScenarioReport{}, "", err
	}
	serial, err := installer.bootFault(ctx, initrd, disk)
	if err != nil {
		return InstallScenarioReport{}, serial, err
	}
	return installFaultScenario(serial), serial, nil
}

func installFaultScenario(serial string) InstallScenarioReport {
	if strings.Contains(serial, InstallKernelPanicMark) {
		return InstallScenarioReport{Name: installScenarioFault, Verdict: InstallVerdictFail, Detail: "kernel reported PID-1 death"}
	}
	if !strings.Contains(serial, InstallFaultRecoverMark) {
		return InstallScenarioReport{Name: installScenarioFault, Verdict: InstallVerdictFail, Detail: "recover marker absent"}
	}
	if !strings.Contains(serial, InstallFatalPolicyMark) {
		return InstallScenarioReport{Name: installScenarioFault, Verdict: InstallVerdictFail, Detail: "fatal policy marker absent"}
	}
	return InstallScenarioReport{Name: installScenarioFault, Verdict: InstallVerdictPass, Detail: "goroutine panic recovered -> FATAL -> reboot; init never killed"}
}

func (installer *Installer) bootPin(
	ctx context.Context,
	fixtures *installFixtures,
	disk, pinned, foreign string,
) (string, error) {
	base := installer.qemuBase(false)
	console := installConsoleAMD64
	if installer.Options.Arch == ArchARM64 {
		console = installConsoleARM64
	}
	var tb textbuf.Buffer
	line := tb.Str("console=").Str(console).Str(" ze.server=").Str(InstallGuestServerIP).
		Str(" ze.port=").Int(int64(fixtures.Port)).Str(" ze.image=").Str(InstallImageName).
		Str(" ze.mac=").Str(InstallPinnedMAC).Str(" panic=-1").String()
	tb.Reset()
	drive := tb.Str("file=").Str(disk).Str(",format=raw,if=virtio").String()
	tb.Reset()
	pinnedDevice := tb.Str(installer.Options.NIC).Str(",netdev=net0,mac=").Str(InstallPinnedMAC).String()
	tb.Reset()
	foreignDevice := tb.Str(installer.Options.NIC).Str(",netdev=net1,mac=").Str(InstallForeignMAC).String()
	argv := slices.Clone(base)
	argv = append(argv, "-kernel", installer.Options.Kernel, "-initrd", fixtures.Initrd, "-append", line,
		"-drive", drive, "-netdev", pinned, "-device", pinnedDevice,
		"-netdev", foreign, "-device", foreignDevice)
	return installer.runCapture(ctx, argv, installer.Options.BootTimeout)
}

// bootAmbiguous boots the installer against TWO blank fixed disks with no
// ze.target on the cmdline. One NIC, and the fixtures' image server reachable,
// so the run reaches disk detection rather than stopping at the network.
func (installer *Installer) bootAmbiguous(ctx context.Context, fixtures *installFixtures, first, second string) (string, error) {
	base := installer.qemuBase(false)
	console := installConsoleAMD64
	if installer.Options.Arch == ArchARM64 {
		console = installConsoleARM64
	}
	var tb textbuf.Buffer
	line := tb.Str("console=").Str(console).Str(" ze.server=").Str(InstallGuestServerIP).
		Str(" ze.port=").Int(int64(fixtures.Port)).Str(" ze.image=").Str(InstallImageName).
		Str(" ip=dhcp panic=-1").String()
	tb.Reset()
	firstDrive := tb.Str("file=").Str(first).Str(",format=raw,if=virtio").String()
	tb.Reset()
	secondDrive := tb.Str("file=").Str(second).Str(",format=raw,if=virtio").String()
	tb.Reset()
	device := tb.Str(installer.Options.NIC).Str(",netdev=net0").String()
	argv := slices.Clone(base)
	argv = append(argv, "-no-reboot", "-kernel", installer.Options.Kernel,
		"-initrd", fixtures.Initrd, "-append", line,
		"-drive", firstDrive, "-drive", secondDrive,
		"-netdev", "user,id=net0", "-device", device)
	return installer.runCapture(ctx, argv, installer.Options.BootTimeout)
}

// scenarioAmbiguous proves the installer stops rather than choosing between two
// fixed disks. Until 2026-09-02 the HTTP path took the first entry of the
// /sys/block listing and wrote a whole disk image over it, which is a guess
// about which disk the operator wanted erased.
func (installer *Installer) scenarioAmbiguous(ctx context.Context, work string, fixtures *installFixtures) (InstallScenarioReport, string, error) {
	if fixtures == nil {
		return InstallScenarioReport{Name: installScenarioAmbiguous, Verdict: InstallVerdictSkip, Detail: "no install fixtures"}, "", nil
	}
	info, err := installer.ops.FS.Stat(fixtures.Image)
	if err != nil {
		return InstallScenarioReport{}, "", err
	}
	first := filepath.Join(work, installScenarioAmbiguous+"-first.img")
	if err := truncateInstallFile(first, info.Size()); err != nil {
		return InstallScenarioReport{}, "", err
	}
	second := filepath.Join(work, installScenarioAmbiguous+"-second.img")
	if err := truncateInstallFile(second, info.Size()); err != nil {
		return InstallScenarioReport{}, "", err
	}
	serial, err := installer.bootAmbiguous(ctx, fixtures, first, second)
	if err != nil {
		return InstallScenarioReport{}, serial, err
	}
	return installAmbiguousScenario(serial), serial, nil
}

// installAmbiguousScenario reads the refusal out of the serial log. Both halves
// are load-bearing: the refusal says the installer noticed, and the absent
// stream mark says it stopped before it wrote anything.
func installAmbiguousScenario(serial string) InstallScenarioReport {
	if !strings.Contains(serial, InstallAmbiguousTargetMark) {
		return InstallScenarioReport{Name: installScenarioAmbiguous, Verdict: InstallVerdictFail, Detail: "installer did not refuse two fixed disks"}
	}
	if strings.Contains(serial, InstallStreamMark) {
		return InstallScenarioReport{Name: installScenarioAmbiguous, Verdict: InstallVerdictFail, Detail: "installer wrote a disk it had refused to choose"}
	}
	return InstallScenarioReport{Name: installScenarioAmbiguous, Verdict: InstallVerdictPass, Detail: "two fixed disks, no ze.target -> refused, nothing written"}
}

func (installer *Installer) scenarioPin(ctx context.Context, work, name string, fixtures *installFixtures) (InstallScenarioReport, string, error) {
	if fixtures == nil {
		return InstallScenarioReport{Name: name, Verdict: InstallVerdictSkip, Detail: "no install fixtures"}, "", nil
	}
	info, err := installer.ops.FS.Stat(fixtures.Image)
	if err != nil {
		return InstallScenarioReport{}, "", err
	}
	disk := filepath.Join(work, name+"-target.img")
	if err := truncateInstallFile(disk, info.Size()); err != nil {
		return InstallScenarioReport{}, "", err
	}
	pinned, foreign := "user,id=net0", "user,id=net1,net=10.0.99.0/24,restrict=on"
	if name == installScenarioPinAC5 {
		pinned, foreign = "user,id=net0,net=10.0.88.0/24,restrict=on", "user,id=net1"
	}
	serial, err := installer.bootPin(ctx, fixtures, disk, pinned, foreign)
	if err != nil {
		return InstallScenarioReport{}, serial, err
	}
	return installPinScenario(name, serial), serial, nil
}

func installPinScenario(name, serial string) InstallScenarioReport {
	if !strings.Contains(serial, InstallPinMark) {
		return InstallScenarioReport{Name: name, Verdict: InstallVerdictFail, Detail: "installer did not pin to the ze.mac NIC"}
	}
	if !strings.Contains(serial, InstallPinnedMAC) {
		return InstallScenarioReport{Name: name, Verdict: InstallVerdictFail, Detail: "installer did not pin to the ze.mac NIC"}
	}
	if name == installScenarioPinAC4 {
		if !strings.Contains(serial, InstallPinReachableMark) {
			return InstallScenarioReport{Name: name, Verdict: InstallVerdictFail, Detail: "server not reachable over pinned NIC"}
		}
		if strings.Contains(serial, InstallFallbackMark) {
			return InstallScenarioReport{Name: name, Verdict: InstallVerdictFail, Detail: "foreign NIC was touched"}
		}
	} else {
		if !strings.Contains(serial, InstallPinFlushMark) {
			return InstallScenarioReport{Name: name, Verdict: InstallVerdictFail, Detail: "pinned NIC was not flushed"}
		}
		if !strings.Contains(serial, InstallFallbackMark) {
			return InstallScenarioReport{Name: name, Verdict: InstallVerdictFail, Detail: "remaining NICs were not scanned"}
		}
	}
	if !strings.Contains(serial, InstallMarkWritten) {
		return InstallScenarioReport{Name: name, Verdict: InstallVerdictFail, Detail: "install did not complete"}
	}
	if !strings.Contains(serial, InstallMarkDone) {
		return InstallScenarioReport{Name: name, Verdict: InstallVerdictFail, Detail: "install did not complete"}
	}
	detail := "pinned to ze.mac NIC, foreign NIC never up, install completed"
	if name == installScenarioPinAC5 {
		detail = "pinned NIC flushed, install recovered on remaining NIC"
	}
	return InstallScenarioReport{Name: name, Verdict: InstallVerdictPass, Detail: detail}
}

// RescueArgv returns one source and credential branch of the rescue proof.
func (installer *Installer) RescueArgv(initrd, source string, auth bool) ([]string, error) {
	base := installer.qemuBase(false)
	console := installConsoleAMD64
	if installer.Options.Arch == ArchARM64 {
		console = installConsoleARM64
	}
	var tb textbuf.Buffer
	consoleSetting := tb.Str("console=").Str(console).String()
	tb.Reset()
	sourceSetting := tb.Str("ze.source=").Str(source).String()
	tb.Reset()
	imageSetting := tb.Str("ze.image=").Str(InstallImageName).String()
	parts := []string{consoleSetting, "panic=-1", sourceSetting, imageSetting}
	if source == "http" {
		parts = append(parts, "ze.server=10.0.2.99", "ze.wait=3", "ip=dhcp")
	} else {
		tb.Reset()
		parts = append(parts, tb.Str("ze.media-id=").Str(InstallDummyMediaID).String())
	}
	if auth {
		tb.Reset()
		parts = append(parts, tb.Str("ze.rescue-auth=").Str(InstallRescueAuth).String())
	}
	tb.Reset()
	appendLine := tb.Join(parts, " ").String()
	tb.Reset()
	device := tb.Str(installer.Options.NIC).Str(",netdev=net0").String()
	return append(base, "-no-reboot", "-kernel", installer.Options.Kernel, "-initrd", initrd, "-append", appendLine,
		"-netdev", "user,id=net0", "-device", device), nil
}

type installInteractive struct {
	command *exec.Cmd
	serial  *installSerial
	input   io.WriteCloser
	done    <-chan struct{}
}

func (installer *Installer) startInteractive(ctx context.Context, argv []string) (*installInteractive, error) {
	// #nosec G204 -- argv is assembled exclusively by the installer's closed QEMU argument builders.
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	// installInteractive.Close owns the interrupt-then-kill bound; CommandContext must not kill QEMU first.
	command.Cancel = func() error { return nil }
	command.Env = installer.ops.Environ()
	input, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}
	output, err := command.StdoutPipe()
	if err != nil {
		return nil, errors.Join(err, input.Close())
	}
	command.Stderr = command.Stdout
	if err := installer.ops.Start(command); err != nil {
		return nil, errors.Join(err, input.Close())
	}
	return &installInteractive{
		command: command,
		serial:  newInstallSerial(output),
		input:   input,
		done:    waitVM(command),
	}, nil
}

func (run *installInteractive) Close() error {
	closeErr := run.input.Close()
	stopVMWaiting(run.command, run.done)
	return closeErr
}
func (run *installInteractive) send(line string) error {
	var tb textbuf.Buffer
	_, err := io.WriteString(run.input, tb.Str(line).Byte('\n').String())
	return err
}

func (installer *Installer) scenarioRescue(ctx context.Context, work, name string) (scenario InstallScenarioReport, serial string, resultErr error) {
	initrd, err := installer.buildScenarioInitrd(ctx, work, false)
	if err != nil {
		return InstallScenarioReport{}, "", err
	}
	source, auth := "http", false
	if name == installScenarioRescueAC7 {
		auth = true
	}
	if name == installScenarioRescueAC7B {
		source = "iso"
	}
	argv, err := installer.RescueArgv(initrd, source, auth)
	if err != nil {
		return InstallScenarioReport{}, "", err
	}
	if name == installScenarioRescueAC7C {
		serial, runErr := installer.runCapture(ctx, argv, installer.Options.RescueTimeout)
		if runErr != nil {
			return InstallScenarioReport{}, serial, runErr
		}
		if strings.Contains(serial, InstallMenuMark) || strings.Contains(serial, InstallTokenPrompt) {
			return InstallScenarioReport{Name: name, Verdict: InstallVerdictFail, Detail: "rescue console offered on unattended network install"}, serial, nil
		}
		if !strings.Contains(serial, InstallReboot30s) {
			return InstallScenarioReport{Name: name, Verdict: InstallVerdictFail, Detail: "30s reboot marker absent"}, serial, nil
		}
		return InstallScenarioReport{Name: name, Verdict: InstallVerdictPass, Detail: "network + no credential printed message and rebooted, never hung"}, serial, nil
	}
	run, err := installer.startInteractive(ctx, argv)
	if err != nil {
		return InstallScenarioReport{}, "", err
	}
	defer func() {
		resultErr = joinInstallCleanup(resultErr, run.Close, "close interactive installer QEMU")
	}()
	if name == installScenarioRescueAC7 {
		if ok, expectErr := run.serial.expect(ctx, InstallTokenPrompt, installer.Options.RescueStepTimeout); expectErr != nil || !ok {
			return InstallScenarioReport{Name: name, Verdict: InstallVerdictFail, Detail: "gated password prompt never appeared"}, run.serial.snapshot(), expectErr
		}
		if err := run.send("definitely-wrong"); err != nil {
			return InstallScenarioReport{}, run.serial.snapshot(), err
		}
		if ok, expectErr := run.serial.expect(ctx, InstallAuthBad, 30*time.Second); expectErr != nil || !ok {
			return InstallScenarioReport{Name: name, Verdict: InstallVerdictFail, Detail: "wrong password was not rejected"}, run.serial.snapshot(), expectErr
		}
		if err := run.send(InstallRescueToken); err != nil {
			return InstallScenarioReport{}, run.serial.snapshot(), err
		}
		okAuth, authErr := run.serial.expect(ctx, InstallAuthOK, 30*time.Second)
		okMenu, menuErr := run.serial.expect(ctx, InstallMenuMark, 30*time.Second)
		if authErr != nil {
			return InstallScenarioReport{}, run.serial.snapshot(), authErr
		}
		if menuErr != nil {
			return InstallScenarioReport{}, run.serial.snapshot(), menuErr
		}
		if !okAuth || !okMenu {
			return InstallScenarioReport{Name: name, Verdict: InstallVerdictFail, Detail: "correct password did not open recovery menu"}, run.serial.snapshot(), nil
		}
		if err := run.send("4"); err != nil {
			return InstallScenarioReport{}, run.serial.snapshot(), err
		}
		return InstallScenarioReport{Name: name, Verdict: InstallVerdictPass, Detail: "wrong password rejected, correct opens gated menu"}, run.serial.snapshot(), nil
	}
	if ok, expectErr := run.serial.expect(ctx, InstallMenuMark, installer.Options.RescueStepTimeout); expectErr != nil || !ok {
		return InstallScenarioReport{Name: name, Verdict: InstallVerdictFail, Detail: "ungated recovery menu never appeared"}, run.serial.snapshot(), expectErr
	}
	if strings.Contains(run.serial.snapshot(), InstallTokenPrompt) {
		return InstallScenarioReport{Name: name, Verdict: InstallVerdictFail, Detail: "password demanded on ungated ISO console"}, run.serial.snapshot(), nil
	}
	if err := run.send("4"); err != nil {
		return InstallScenarioReport{}, run.serial.snapshot(), err
	}
	return InstallScenarioReport{Name: name, Verdict: InstallVerdictPass, Detail: "ISO + no credential opens the menu without a password"}, run.serial.snapshot(), nil
}

func (installer *Installer) executeScenarios(ctx context.Context, work string, report InstallReport) (result InstallReport, resultErr error) {
	var tb textbuf.Buffer
	report.line(installer.prefix(), tb.Str("arch=").Str(installer.Options.Arch).
		Str(" accel=").Str(installer.accelerator()).Str(" kernel=").Str(installer.Options.Kernel).String())
	fixtures, skipReason, err := installer.setupInstallFixtures(ctx, work)
	if err != nil {
		return report, err
	}
	if fixtures != nil {
		defer func() {
			resultErr = joinInstallCleanup(resultErr, fixtures.Close, "close installer scenario fixtures")
		}()
	}
	if skipReason != "" {
		tb.Reset()
		report.line(installer.prefix(), tb.Str("install fixtures unavailable, fixture scenarios will skip: ").
			Str(skipReason).String())
	}
	names := []string{
		installScenarioFault, installScenarioPinAC4, installScenarioPinAC5,
		installScenarioRescueAC7, installScenarioRescueAC7B, installScenarioRescueAC7C,
		installScenarioAmbiguous,
	}
	for _, name := range names {
		var scenario InstallScenarioReport
		var serial string
		switch name {
		case installScenarioFault:
			scenario, serial, err = installer.scenarioFault(ctx, work)
		case installScenarioPinAC4, installScenarioPinAC5:
			scenario, serial, err = installer.scenarioPin(ctx, work, name, fixtures)
		case installScenarioAmbiguous:
			scenario, serial, err = installer.scenarioAmbiguous(ctx, work, fixtures)
		default:
			scenario, serial, err = installer.scenarioRescue(ctx, work, name)
		}
		if err != nil {
			return report, err
		}
		report.Scenarios = append(report.Scenarios, scenario)
		if scenario.Verdict == InstallVerdictSkip {
			tb.Reset()
			report.line(installer.prefix(), tb.Str("SKIP scenario ").Str(name).Str(" (no install fixtures)").String())
			continue
		}
		if scenario.Verdict != InstallVerdictPass {
			report.lines = append(report.lines, serial)
			tb.Reset()
			report.line(installer.prefix(), tb.Str("FAIL ").Str(name).Str(": ").Str(scenario.Detail).String())
			continue
		}
		tb.Reset()
		report.line(installer.prefix(), tb.Str("PASS ").Str(name).Str(": ").Str(scenario.Detail).String())
	}
	passed, skipped, failed := make([]string, 0, 6), make([]string, 0, 6), make([]string, 0, 6)
	for _, scenario := range report.Scenarios {
		switch scenario.Verdict {
		case InstallVerdictPass:
			passed = append(passed, scenario.Name)
		case InstallVerdictSkip:
			skipped = append(skipped, scenario.Name)
		case InstallVerdictUnspecified, InstallVerdictFail:
			failed = append(failed, scenario.Name)
		default:
			failed = append(failed, scenario.Name)
		}
	}
	tb.Reset()
	report.line(installer.prefix(), tb.Str("passed=").Str(installScenarioNames(passed)).
		Str(" skipped=").Str(installScenarioNames(skipped)).
		Str(" failed=").Str(installScenarioNames(failed)).String())
	if len(failed) != 0 {
		report.line(installer.prefix(), "FAIL one or more scenarios failed")
		report.Verdict = InstallVerdictFail
		return report, nil
	}
	if len(passed) == 0 {
		return installer.skip(report, "no scenarios ran (all skipped)")
	}
	report.line(installer.prefix(), "PASS")
	report.Verdict = InstallVerdictPass
	return report, nil
}

func installScenarioNames(names []string) string {
	var b textbuf.Buffer
	b.Byte('[')
	for index, name := range names {
		if index != 0 {
			b.Str(", ")
		}
		b.Byte('\'').Str(name).Byte('\'')
	}
	return b.Byte(']').String()
}
