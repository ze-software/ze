// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: docker.go -- the Docker client the probe runs through
//
// A Docker lab runs Ze on the Docker daemon's kernel, so a kernel feature Ze
// needs and the daemon's kernel lacks fails scenarios for a host fact. The
// required set is not listed here: the staged ze answers it, by running
// `ze doctor --json kernel-capabilities` on the daemon's kernel, which probes
// every capability Ze enrolls (internal/component/kernelcap). A capability a
// package enrolls later is required with no change to this file.

package interoplab

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/textbuf"
	buildhostdriver "github.com/ze-software/ze/internal/le/build/hostdriver"
)

const (
	// kernelProbeImage only carries the staged static ze; any Linux userland
	// that runs a static binary answers the same, because the probes ask the
	// kernel, not the image.
	kernelProbeImage   = "alpine:3.21"
	kernelProbeTimeout = 2 * time.Minute
	// kernelProbeMount is where the staged ze appears inside the probe container.
	kernelProbeMount = "/ze"
)

// DockerKernelRoute answers the next step a refusal names for a reader on goos
// (owner D-6), its command on a line of its own (user-facing-errors.md). On
// Linux the labs run on the host's Docker, so the step is a kernel with the
// feature or Ze's own (D-5) through the install action, which never reboots;
// inside the Docker lab's QEMU guest, whose kernel is the appliance's, the step
// is that kernel's config and the host's cache rebuild instead. Elsewhere the labs run inside the Alpine QEMU guest booted on Ze's runtime
// kernel, and the Linux-only install would refuse: the command reruns lab, the
// le words of the run that was refused, inside that guest, under environment,
// the NAME=value selector assignments that chose its scenario, through the
// guest's `env` keyword. Without them the rerun would run the whole suite. With
// no lab, or a lab or an assignment the guest's `lab` and `env` keywords cannot
// carry intact, the command boots the guest and runs its kernel check only,
// which is still a command the reader can run as printed.
func DockerKernelRoute(goos string, lab, environment []string) string {
	if goos == "linux" {
		if inDockerLabGuest() {
			return "This check ran inside the throwaway Ze-kernel QEMU guest of `./le test qemu docker-lab`," +
				" which boots the appliance's runtime kernel, so installing a kernel there changes nothing:" +
				" the appliance kernel lacks the feature. Add its options to gokrazy/kernel/runtime.config" +
				" and gokrazy/kernel/runtime.require, then rebuild the kernel cache entry on the host with:\n" +
				buildhostdriver.RuntimeKernelBuild(runtime.GOARCH, "")
		}
		return "Run Docker on a kernel with every feature, or install Ze's kernel and reboot." +
			" The install prints its steps and runs none until you confirm the release:\n" +
			"  ./le setup docker-kernel install"
	}
	quotedLab, labOK := quotedGuestWords(lab)
	quotedEnvironment, environmentOK := quotedGuestWords(environment)
	if labOK && (environmentOK || len(environment) == 0) {
		command := "  ./le test qemu docker-lab lab " + quotedLab
		if environmentOK {
			command += " env " + quotedEnvironment
		}
		return "On " + goos + " the labs run inside the Ze-kernel QEMU guest. Run the same lab there with:\n" + command
	}
	return "On " + goos + " the labs run inside the Ze-kernel QEMU guest. Boot it and check its kernel with:\n" +
		"  ./le test qemu docker-lab"
}

// DockerLabGuestVariable is set to 1 by the shell line the Docker lab's QEMU
// guest runs (testqemu dockerLabCommand), so a refusal inside that guest names
// the appliance kernel's own fix rather than an install into a throwaway VM.
const DockerLabGuestVariable = "ZE_DOCKER_LAB_GUEST"

// inDockerLabGuest reports whether this process runs inside the Docker lab's
// QEMU guest. A var so a unit test names either answer.
var inDockerLabGuest = func() bool { return os.Getenv(DockerLabGuestVariable) == "1" }

// quotedGuestWords answers words as one single-quoted shell word for the
// guest's `lab` or `env` keyword, and false when there are none or a word is
// empty or holds white space or a quote: the guest splits the value with
// strings.Fields (testqemu dockerlab.go), which breaks on every unicode.IsSpace
// rune, so such a word would not arrive as typed.
func quotedGuestWords(words []string) (string, bool) {
	if len(words) == 0 {
		return "", false
	}
	for _, word := range words {
		if word == "" {
			return "", false
		}
		if strings.IndexFunc(word, unicode.IsSpace) >= 0 {
			return "", false
		}
		if strings.ContainsAny(word, "'\"") {
			return "", false
		}
	}
	return "'" + strings.Join(words, " ") + "'", true
}

// selectorVariables holds the name of every scenario selector variable a suite
// in this process declared to ReadEnvironment, so the route reruns a refused
// lab under the same selection. The suites declare the names; nothing here
// lists them.
var selectorVariables sync.Map

// selectorEnvironment answers the NAME=value selector assignments of this
// process's run. A var so a unit test names an environment.
var selectorEnvironment = invokedSelectorEnvironment

// invokedSelectorEnvironment answers NAME=value, sorted by name, for every
// declared selector variable set to a non-empty value in this process's
// environment.
func invokedSelectorEnvironment() []string {
	var assignments []string
	selectorVariables.Range(func(key, _ any) bool {
		name, _ := key.(string)
		if value, ok := os.LookupEnv(name); ok && strings.TrimSpace(value) != "" {
			assignments = append(assignments, name+"="+strings.TrimSpace(value))
		}
		return true
	})
	slices.Sort(assignments)
	return assignments
}

// leWords answers the le words of the command line this process runs, the
// words `./le <words>` repeats, and nil when this process is not le (a unit
// test binary). A var so a unit test names a command line.
var leWords = invokedLeWords

// invokedLeWords reads the words from os.Args: after the program for `le`
// (bin/le, which ./le execs), after `le` for `ze le`.
func invokedLeWords() []string {
	if len(os.Args) == 0 {
		return nil
	}
	name := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	if name == "le" {
		return os.Args[1:]
	}
	if name == "ze" && len(os.Args) > 1 && os.Args[1] == "le" {
		return os.Args[2:]
	}
	return nil
}

// dockerSecurityOptionsFormat asks the daemon which security modules it applies
// to a container; a daemon applying AppArmor lists "name=apparmor".
const dockerSecurityOptionsFormat = "{{json .SecurityOptions}}"

// appArmorProfilesPath lists the AppArmor profiles a Linux kernel has loaded,
// one "name (mode)" per line.
const appArmorProfilesPath = "/sys/kernel/security/apparmor/profiles"

// appArmorProfiles answers the local kernel's loaded profile list, and false
// when it cannot be read: off Linux, or without AppArmor. A var so a unit test
// fakes the host.
var appArmorProfiles = readAppArmorProfiles

func readAppArmorProfiles() (string, bool) {
	if runtime.GOOS != "linux" {
		return "", false
	}
	data, err := os.ReadFile(appArmorProfilesPath)
	if err != nil {
		return "", false
	}
	return string(data), true
}

// appArmorProfileLoaded reports whether profiles, in the kernel's
// "name (mode)" lines, lists name.
func appArmorProfileLoaded(profiles, name string) bool {
	for line := range strings.SplitSeq(profiles, "\n") {
		if strings.HasPrefix(line, name+" (") {
			return true
		}
	}
	return false
}

// kernelProbeCommand is what the probe container runs. It is named in every
// refusal, so an operator can run the same question by hand.
var kernelProbeCommand = []string{kernelProbeMount, "doctor", "--json", "kernel-capabilities"}

// kernelRow is one row of `ze doctor --json kernel-capabilities`, read at this
// boundary as JSON. The producer is kernelcap.Row (internal/component/kernelcap).
type kernelRow struct {
	Subsystem string `json:"subsystem"`
	Kernel    string `json:"kernel"`
	State     string `json:"state"`
	Reason    string `json:"reason"`
}

type kernelAnswer struct {
	Ready        bool        `json:"ready"`
	Capabilities []kernelRow `json:"capabilities"`
}

// DockerKernel returns the preflight that refuses a Docker daemon whose kernel
// lacks any kernel feature Ze enrolls. zePath is the staged linux ze for the
// daemon's architecture, so it runs after StageBinaries.
func DockerKernel(zePath string) PreflightCheck {
	return func(ctx context.Context, docker *Docker) error {
		return checkDockerKernel(ctx, docker, zePath)
	}
}

// StagedZePath answers the absolute path of the binary named ze among the
// binaries a lab stages, which is what Suite.StagedZe names. A lab that stages
// no ze answers empty, and Suite.Run refuses that answer rather than skipping.
func StagedZePath(root string, binaries []LabBinary) string {
	for _, binary := range binaries {
		if binary.Name == "ze" {
			return filepath.Join(root, binary.Output)
		}
	}
	return ""
}

// kernelProbeContainer is the probe container. It grants NET_ADMIN
// for the netlink probes and SYS_ADMIN for the probes that unshare a network
// namespace, and gives the container its own network so nothing it creates
// reaches the host's. On a daemon applying AppArmor it runs under Ze's profile,
// because docker-default denies the MPLS probe's mount and sysctl write (D-7).
// It runs through RunOneShot, so it carries the owner labels and a canceled
// probe is removed by name.
func kernelProbeContainer(zePath string, appArmor bool) OneShotContainer {
	arguments := make([]string, 0, 10)
	arguments = append(arguments, "--cap-add", "NET_ADMIN", "--cap-add", "SYS_ADMIN", "--network", "none")
	if appArmor {
		arguments = append(arguments, "--security-opt", "apparmor="+kernelcap.ProbeAppArmorProfileName)
	}
	arguments = append(arguments, "-v", zePath+":"+kernelProbeMount+":ro")
	return OneShotContainer{Image: kernelProbeImage, Arguments: arguments, Command: kernelProbeCommand, Timeout: kernelProbeTimeout}
}

// probeFailed reports a probe that did not run to an exit code: the docker
// client failed or was stopped. A probe that exited non-zero is judged on its
// answer, as the AppArmor refusal and the unreadable-answer error need.
func probeFailed(err error) bool {
	if err == nil {
		return false
	}
	commandErr, ok := errors.AsType[*commandError](err)
	if !ok {
		return true
	}
	return commandErr.Cause != nil
}

// dockerAppArmor asks whether the daemon applies AppArmor to its containers. A
// question it cannot answer is an error: the probe's argv depends on it.
func dockerAppArmor(ctx context.Context, docker *Docker) (bool, error) {
	argv := []string{DockerExecutable, "info", dockerFormatFlag, dockerSecurityOptionsFormat}
	result, err := docker.runner.Run(ctx, ProcessCommand{Arguments: argv, Timeout: dockerInfoTimeout})
	var problem textbuf.Buffer
	problem.Str("the Docker kernel check could not read the daemon's security options: `").
		Str(strings.Join(argv, " ")).Str("` ")
	if err != nil {
		return false, errors.New(problem.Str("failed: ").Err(err).String())
	}
	if result.ExitCode != 0 {
		return false, errors.New(problem.Str("exit ").Str(strconv.Itoa(result.ExitCode)).
			Str(": ").Str(strings.TrimSpace(result.Stderr)).String())
	}
	var options []string
	if jsonErr := json.Unmarshal([]byte(result.Stdout), &options); jsonErr != nil {
		return false, errors.New(problem.Str("answered unreadable JSON (").Err(jsonErr).Str("): ").
			Str(strings.TrimSpace(result.Stdout)).String())
	}
	for _, option := range options {
		if option == "name=apparmor" || strings.HasPrefix(option, "name=apparmor,") {
			return true, nil
		}
	}
	return false, nil
}

// appArmorProfileMissing answers the refusal for a Linux host whose kernel
// lists its loaded profiles and lacks Ze's, or nil. An unreadable list is no
// answer either way (apparmorfs refuses it to a reader without policy-view
// privilege, such as a non-root user), so it is never read as "loaded": the
// probe runs, and a daemon that cannot apply the profile refuses
// through appArmorProfileUnapplied instead.
func appArmorProfileMissing(release string) error {
	profiles, readable := appArmorProfiles()
	if !readable {
		return nil
	}
	if appArmorProfileLoaded(profiles, kernelcap.ProbeAppArmorProfileName) {
		return nil
	}
	return appArmorRefusal(release, appArmorProfilesPath+" does not list")
}

// runcAppArmorUnapplied is the phrase runc writes when the daemon applies
// AppArmor and cannot put the container under the profile it was asked for,
// which is what a profile the daemon's kernel has not loaded produces
// ("... unable to apply apparmor profile: ... attr/apparmor/exec: no such
// file or directory", Ubuntu 6.8, 2026-10-10).
const runcAppArmorUnapplied = "unable to apply apparmor profile"

// appArmorProfileUnapplied reports whether a probe that exited exit with
// stderr was refused by the daemon because it could not apply Ze's profile.
func appArmorProfileUnapplied(exit int, stderr string) bool {
	if exit == 0 {
		return false
	}
	return strings.Contains(strings.ToLower(stderr), runcAppArmorUnapplied)
}

// appArmorRefusal names the profile, what showed it is not in force, the
// command that loads it, and the kernel route. Loading the profile lets the
// probe answer and changes no kernel feature, so the operator reads the whole
// path at once rather than one step per run.
func appArmorRefusal(release, finding string) error {
	var refusal textbuf.Buffer
	return errors.New(refusal.Str("the Docker daemon applies AppArmor, and the kernel probe runs under Ze's profile ").
		Str(kernelcap.ProbeAppArmorProfileName).Str(", which ").Str(finding).
		Str(". Docker's docker-default profile denies the probe's mount and its /proc/sys write. Load Ze's profile with:\n  ").
		Str(AppArmorLoadCommandFor(kernelcap.ProbeAppArmorProfileName)).
		Str("\nThe profile lets the probe answer and adds no kernel feature: every feature the daemon's kernel (").
		Str(release).Str(") lacks, MOBIKE's xfrm migrate for example, is then named.\n").
		Str(DockerKernelRoute(runtime.GOOS, leWords(), selectorEnvironment())).String())
}

// checkDockerKernel runs the probe and returns nil only when the answer holds at
// least one row and every row is present. Unknown is a refusal (owner D-4: a
// wrong kernel "should not be possible - fail"), and so is a probe that did not
// answer: the check never proceeds on a question it could not ask.
func checkDockerKernel(ctx context.Context, docker *Docker, zePath string) error {
	if docker == nil {
		return errors.New("the Docker kernel check needs a Docker client")
	}
	if zePath == "" {
		return errors.New("the Docker kernel check needs the staged ze to probe with, and this suite names none (Suite.StagedZe)")
	}
	// The probe bind-mounts zePath, and Docker answers a missing source by
	// creating a root-owned directory there, which a later staging then cannot
	// replace. So the file is checked before any docker command runs.
	if missing := stagedZeMissing(zePath); missing != nil {
		return missing
	}

	appArmor, err := dockerAppArmor(ctx, docker)
	if err != nil {
		return err
	}
	release := dockerKernelRelease(ctx, docker)
	if appArmor {
		if missing := appArmorProfileMissing(release); missing != nil {
			return missing
		}
	}
	probe := kernelProbeContainer(zePath, appArmor)
	result, err := docker.RunOneShot(ctx, probe)

	var problem textbuf.Buffer
	problem.Str("the Docker daemon's kernel (").Str(release).Str(") could not be checked: `").
		Str(strings.Join(probe.Command, " ")).Str("` in ").Str(probe.Image).Str(" ")
	if probeFailed(err) {
		return errors.New(problem.Str("failed: ").Err(err).String())
	}
	if appArmor && appArmorProfileUnapplied(result.ExitCode, result.Stderr) {
		return appArmorRefusal(release, "the daemon could not apply ("+strings.TrimSpace(result.Stderr)+")")
	}
	var answer kernelAnswer
	if jsonErr := json.Unmarshal([]byte(result.Stdout), &answer); jsonErr != nil {
		return errors.New(problem.Str("exit ").Str(strconv.Itoa(result.ExitCode)).
			Str(", unreadable answer (").Err(jsonErr).Str("): stdout ").Str(strings.TrimSpace(result.Stdout)).
			Str("; stderr ").Str(strings.TrimSpace(result.Stderr)).String())
	}
	if len(answer.Capabilities) == 0 {
		return errors.New(problem.Str("answered no capability, so nothing about the kernel was asked").String())
	}

	var missing textbuf.Buffer
	count := 0
	denied := false
	deniedBySELinux := false
	for _, row := range answer.Capabilities {
		if row.State == "present" {
			continue
		}
		if row.State == "denied" {
			denied = true
			deniedBySELinux = deniedBySELinux || strings.Contains(row.Reason, "SELinux")
		}
		missing.Str("\n  ").Str(row.Subsystem).Str(" (").Str(row.Kernel).Str("): ").Str(row.State)
		if row.Reason != "" {
			missing.Str(": ").Str(row.Reason)
		}
		count++
	}
	if count == 0 {
		// Every row reads present, so the probe's own verdict must agree: the
		// producer (doctor writeKernelCapabilities) exits 0 with ready true only
		// then. A disagreement is a probe this check cannot read, never a pass.
		if answer.Ready && result.ExitCode == 0 {
			return nil
		}
		return errors.New(problem.Str("answered every row present but exit ").Str(strconv.Itoa(result.ExitCode)).
			Str(", ready ").Str(strconv.FormatBool(answer.Ready)).Str(", so its verdict disagrees with its rows").String())
	}
	var refusal textbuf.Buffer
	refusal.Str("the Docker daemon's kernel ").Str(release).Str(" lacks ").Str(strconv.Itoa(count)).
		Str(" kernel feature(s) Ze needs (`").Str(strings.Join(kernelProbeCommand, " ")).Str("` answered):").
		Str(missing.String()).
		Byte('\n').Str(DockerKernelRoute(runtime.GOOS, leWords(), selectorEnvironment()))
	if denied {
		refusal.Str("\nA denied row is the host's security policy refusing the probe, not the kernel.").
			Str(deniedFix(appArmor, deniedBySELinux))
	}
	return errors.New(refusal.String())
}

// seLinuxDenialsCommand lists the SELinux denials the audit log recorded
// recently, which name the access the host's policy refused.
const seLinuxDenialsCommand = "sudo ausearch -m avc -ts recent"

// deniedFix answers the fix a denied row's refusal names. Under AppArmor it is
// Ze's profile, loaded or reloaded by one command (D-7). Ze ships no policy for
// another module, so a daemon without AppArmor is told which grant the row's
// policy owes, and under SELinux given the command that shows the denial.
func deniedFix(appArmor, seLinux bool) string {
	if appArmor {
		return " Load or reload Ze's AppArmor profile with:\n  " +
			AppArmorLoadCommandFor(kernelcap.ProbeAppArmorProfileName)
	}
	fix := " This Docker daemon applies no AppArmor, so Ze's AppArmor profile cannot help:" +
		" the policy the row names must grant the probe what the row says it needs."
	if seLinux {
		fix += " Read the SELinux denials with:\n  " + seLinuxDenialsCommand
	}
	return fix
}

// dockerKernelRelease reads the daemon's kernel release for the refusal. It is
// context for the reader, never a verdict, so a failed read is spelled out in
// its place rather than stopping the check.
func dockerKernelRelease(ctx context.Context, docker *Docker) string {
	result, err := docker.runner.Run(ctx, ProcessCommand{
		Arguments: []string{DockerExecutable, "info", dockerFormatFlag, "{{.KernelVersion}}"},
		Timeout:   dockerInfoTimeout,
	})
	if err != nil {
		var unread textbuf.Buffer
		return unread.Str("release unread: ").Err(err).String()
	}
	release := strings.TrimSpace(result.Stdout)
	if release == "" {
		return "release unread"
	}
	return release
}

// stagedZeMissing answers the refusal for a staged ze path that holds no
// regular file, or nil. A run that skipped its build (NO_BUILD=1) on a checkout
// that never staged one is the usual cause, so the fix is that build.
func stagedZeMissing(zePath string) error {
	info, err := os.Stat(zePath)
	var problem textbuf.Buffer
	problem.Str("the Docker kernel check cannot run: the linux ze it probes with is not at ").Str(zePath)
	if err != nil {
		return errors.New(problem.Str(" (").Err(err).
			Str(").\nThe check mounts that file into the probe container, so without it nothing is checked.\n").
			Str(stageZeAgain(os.LookupEnv)).String())
	}
	if !info.Mode().IsRegular() {
		return errors.New(problem.Str(": that path is a directory, not the binary").
			Str(" (Docker creates one when it mounts a path that does not exist).").
			Str("\nThe check mounts that file into the probe container, so without it nothing is checked.").
			Str("\nRemove the directory, then run the same command again without NO_BUILD=1:\n").
			Str("  sudo rm -r ").Str(zePath).String())
	}
	return nil
}

// stageZeAgain answers the step that stages the missing ze: the same le
// command run with its build, on a line of its own. A run under NO_BUILD=1
// skipped the build that stages it, so the command unsets it; any other run
// (perf stages before it checks) is run again so its build stages the file.
// lookup reads the environment, a parameter so a unit test names it.
func stageZeAgain(lookup func(string) (string, bool)) string {
	noBuild, _ := lookup("NO_BUILD")
	words := leWords()
	if noBuild == "1" {
		if len(words) == 0 {
			return "This run skipped its build (NO_BUILD=1). Run the same ./le command again without NO_BUILD=1."
		}
		return "This run skipped its build (NO_BUILD=1), which stages it. Run the same command with its build:\n" +
			"  env -u NO_BUILD ./le " + shellJoin(words)
	}
	if len(words) == 0 {
		return "No build staged it there. Run the same ./le command again so its build stages it."
	}
	return "No build staged it there. Run the same command again so its build stages it:\n" +
		"  ./le " + shellJoin(words)
}

// shellJoin joins words into one shell command line, single-quoting a word
// that holds white space, a quote or another character the shell reads, so the
// printed command runs as the words were given.
func shellJoin(words []string) string {
	quoted := make([]string, 0, len(words))
	for _, word := range words {
		if word != "" && !strings.ContainsAny(word, " \t\n'\"\\$`;&|<>()*?[]{}~#!") {
			quoted = append(quoted, word)
			continue
		}
		quoted = append(quoted, "'"+strings.ReplaceAll(word, "'", `'\''`)+"'")
	}
	return strings.Join(quoted, " ")
}
