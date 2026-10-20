// Design: docs/architecture/testing/interop.md -- Crash-safe teardown: owner labels and the orphan cleanup
// Related: docker.go -- every create argv appends ownerIdentity.labelArguments.
// Related: lab.go -- Suite.Run sweeps before Preflight; Lab.preClean claims names through claimName.
// Related: ../procscan/procscan.go -- the pid start token the verdict compares.
package interoplab

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/procscan"
)

// The owner label keys. Every resource the wrapper creates carries all of
// them, and the cleanup lists only resources carrying labelMarker, so a
// resource without it is never a candidate. docs/architecture/testing/interop.md
// lists these keys from this declaration.
const (
	labelMarker       = "ze.lab"
	labelMarkerValue  = "1"
	labelRun          = "ze.lab.run"
	labelPID          = "ze.lab.pid"
	labelPIDStart     = "ze.lab.pid-start"
	labelBootID       = "ze.lab.boot-id"
	labelMachineID    = "ze.lab.machine-id"
	labelStarted      = "ze.lab.started"
	labelPurpose      = "ze.lab.purpose"
	labelKind         = "ze.lab.kind"
	labelUID          = "ze.lab.uid"
	labelOS           = "ze.lab.os"
	labelPIDNamespace = "ze.lab.pid-ns"
)

// timeLayoutRFC3339 is the layout of the labelStarted value.
const timeLayoutRFC3339 = time.RFC3339

// retainedTagRepository is the repository of the run-owned image tag Build
// adds. It is the only image tag the cleanup may remove: the shared cache tag
// beside it is what NO_BUILD=1 reuses.
const retainedTagRepository = "ze-interop-retained"

// The client-side identity sources. Each is read on the machine running le,
// never asked of the daemon, which on a Mac runs in a VM with its own pids.
const (
	machineIDPath    = "/etc/machine-id"
	bootIDPath       = "/proc/sys/kernel/random/boot_id"
	pidNamespacePath = "/proc/self/ns/pid"
	procStatPath     = "/proc/stat"
	hostQueryTimeout = 5 * time.Second
	purposeBytesMax  = 128
	purposeWordsMax  = 5
	cleanupCommand   = "./le test integration cleanup"
)

// The identity newDocker gives a test double, so its argv is deterministic.
const (
	fixtureMachineID    = "0123456789abcdef0123456789abcdef"
	fixtureBootID       = "01234567-89ab-cdef-0123-456789abcdef"
	fixturePIDNamespace = "4026531836"
	fixtureUID          = 1000
)

// ownerIdentity is this process as the owner labels record it. pidNamespace
// is the inode of the Linux pid namespace the process runs in, and is empty
// on macOS, which has no pid namespace: its pid label is the host's own.
type ownerIdentity struct {
	run          string
	pid          int
	pidStart     string
	bootID       string
	machineID    string
	started      time.Time
	purpose      string
	uid          int
	goos         string
	pidNamespace string
}

// labelArguments is the one place a create argv gets its labels from. kind is
// the object the argv creates, recorded so a container that inherited a lab
// image's labels (kind image) is never judged as one a lab run created.
func (o ownerIdentity) labelArguments(kind resourceKind) []string {
	pairs := o.labelPairs(kind)
	argv := make([]string, 0, 2*len(pairs))
	var pair textbuf.Buffer
	for _, label := range pairs {
		pair.Reset()
		argv = append(argv, "--label", pair.Str(label[0]).Byte('=').Str(label[1]).String())
	}
	return argv
}

// labelPairs is every owner label a create of kind carries, as key and value.
// The pid namespace label is present only where the client has one (Linux).
func (o ownerIdentity) labelPairs(kind resourceKind) [][2]string {
	// Twelve: the eleven labels every create carries, and the pid namespace.
	pairs := make([][2]string, 0, 12)
	pairs = append(pairs,
		[2]string{labelMarker, labelMarkerValue},
		[2]string{labelKind, kind.labelValue()},
		[2]string{labelRun, o.run},
		[2]string{labelPID, strconv.Itoa(o.pid)},
		[2]string{labelPIDStart, o.pidStart},
		[2]string{labelBootID, o.bootID},
		[2]string{labelMachineID, o.machineID},
		[2]string{labelStarted, o.started.UTC().Format(timeLayoutRFC3339)},
		[2]string{labelPurpose, o.purpose},
		[2]string{labelUID, strconv.Itoa(o.uid)},
		[2]string{labelOS, o.goos},
	)
	if o.pidNamespace == "" {
		return pairs
	}
	return append(pairs, [2]string{labelPIDNamespace, o.pidNamespace})
}

// created reports whether labels are exactly the ones this client's own
// creates of kind carry: the same run id, pid, pid start token, boot, machine,
// uid, OS, pid namespace and start time. Only this process writes that set, so
// a resource carrying it is this run's own leftover, such as a shared lab
// network a previous scenario's removal left behind while it outlasted its
// bound. An identity with no run id answers false: it names no run.
func (o ownerIdentity) created(kind resourceKind, labels map[string]string) bool {
	if o.run == "" {
		return false
	}
	pairs := o.labelPairs(kind)
	for _, label := range pairs {
		if labels[label[0]] != label[1] {
			return false
		}
	}
	if o.pidNamespace != "" {
		return true
	}
	return labels[labelPIDNamespace] == ""
}

func fixtureOwner() ownerIdentity {
	return ownerIdentity{
		run: "run-fixture", pid: 4242, pidStart: "100", bootID: fixtureBootID, machineID: fixtureMachineID,
		started: time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC), purpose: "le test integration interop",
		uid: fixtureUID, goos: hostGOOSLinux, pidNamespace: fixturePIDNamespace,
	}
}

func fixtureJudge() ownerJudge {
	return ownerJudge{
		host: hostFacts{machineID: fixtureMachineID, bootID: fixtureBootID, goos: hostGOOSLinux, uid: fixtureUID,
			pidNamespace: fixturePIDNamespace, bootStarted: time.Date(2026, 10, 18, 11, 0, 0, 0, time.UTC)},
		lookup: func(int) (procscan.Process, bool, error) { return procscan.Process{}, false, nil },
	}
}

// processIdentity is read once per process: the owner every resource this
// process creates names, and the host facts its verdicts compare against.
var processIdentity = sync.OnceValues(func() (ownerIdentity, ownerJudge) {
	host := readHostFacts(systemHostReaders())
	owner := ownerIdentity{
		run: rand.Text(), pid: os.Getpid(), bootID: host.bootID, machineID: host.machineID,
		started: time.Now(), purpose: purposeOf(os.Args),
		uid: host.uid, goos: host.goos, pidNamespace: host.pidNamespace,
	}
	// A start token that cannot be read leaves the label empty, which every
	// later verdict reads as malformed and skips: the safe direction.
	if self, found, err := procscan.Lookup(owner.pid); err == nil && found {
		owner.pidStart = self.Start
	}
	return owner, ownerJudge{host: host, lookup: procscan.Lookup}
})

// purposeOf names the caller from its command line, for the operator reading
// a skip line: the program's base name and the words that follow it.
func purposeOf(argv []string) string {
	var purpose textbuf.Buffer
	for index, word := range argv {
		if index == purposeWordsMax {
			break
		}
		if index == 0 {
			word = filepath.Base(word)
		}
		if index > 0 {
			purpose.Byte(' ')
		}
		for _, character := range word {
			if character < ' ' || character > '~' || character == ',' || character == '"' {
				character = '_'
			}
			purpose.WriteRune(character)
		}
	}
	text := purpose.String()
	if len(text) > purposeBytesMax {
		return text[:purposeBytesMax]
	}
	return text
}

// hostFacts is the client machine's identity. problem is set, and the ids are
// empty, when the machine id, the boot id or (on Linux) the pid namespace
// cannot be read; every verdict is then "unknown". bootStartedProblem is set
// when the boot time cannot be read; only an owner under another boot id is
// then "unknown", because nothing tells a reboot from a clone.
type hostFacts struct {
	machineID          string
	bootID             string
	problem            string
	goos               string
	uid                int
	pidNamespace       string
	bootStarted        time.Time
	bootStartedProblem string
}

// The client operating systems readHostFacts can read an identity from, as
// runtime.GOOS spells them.
const (
	hostGOOSLinux  = "linux"
	hostGOOSDarwin = "darwin"
)

// hostReaders are the client-side sources readHostFacts asks, injected for tests.
type hostReaders struct {
	goos     string
	uid      int
	readFile func(string) ([]byte, error)
	readLink func(string) (string, error)
	command  func(name string, arguments ...string) ([]byte, error)
}

func systemHostReaders() hostReaders {
	return hostReaders{
		goos:     runtime.GOOS,
		uid:      os.Getuid(),
		readFile: os.ReadFile,
		readLink: os.Readlink,
		command: func(name string, arguments ...string) ([]byte, error) {
			ctx, cancel := context.WithTimeout(context.Background(), hostQueryTimeout)
			defer cancel()
			return exec.CommandContext(ctx, name, arguments...).Output() //nolint:gosec // fixed identity queries
		},
	}
}

func readHostFacts(readers hostReaders) hostFacts {
	// The system is a fact the client always knows, so it is carried even when
	// the identity is a problem: an AppArmor refusal and the kernel route pick
	// their text by it, and an empty one would name no system at all.
	facts := readHostIdentity(readers)
	facts.goos = readers.goos
	return facts
}

// readHostIdentity answers the machine and boot identity readHostFacts
// carries, or the problem that kept it from being read.
func readHostIdentity(readers hostReaders) hostFacts {
	switch readers.goos {
	case hostGOOSLinux:
		machine, problem := readIDFile(readers, machineIDPath)
		if problem != "" {
			return hostFacts{problem: problem}
		}
		boot, problem := readIDFile(readers, bootIDPath)
		if problem != "" {
			return hostFacts{problem: problem}
		}
		namespace, problem := readPIDNamespace(readers)
		if problem != "" {
			return hostFacts{problem: problem}
		}
		facts := hostFacts{machineID: machine, bootID: boot, goos: readers.goos, uid: readers.uid, pidNamespace: namespace}
		facts.bootStarted, facts.bootStartedProblem = readLinuxBootStarted(readers)
		return facts
	case hostGOOSDarwin:
		return readDarwinFacts(readers)
	}
	var problem textbuf.Buffer
	return hostFacts{problem: problem.Str("this host (").Str(readers.goos).
		Str(") has no machine or boot identity source the lab cleanup reads").String()}
}

func readIDFile(readers hostReaders, path string) (string, string) {
	var problem textbuf.Buffer
	data, err := readers.readFile(path)
	if err != nil {
		return "", problem.Str(path).Str(" is unreadable: ").Err(err).String()
	}
	id, ok := normalizedID(string(data))
	if !ok {
		return "", problem.Str(path).Str(" holds no 32-digit hex id").String()
	}
	return id, ""
}

func readDarwinFacts(readers hostReaders) hostFacts {
	var problem textbuf.Buffer
	platform, err := readers.command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice")
	if err != nil {
		return hostFacts{problem: problem.Str("`ioreg -rd1 -c IOPlatformExpertDevice` failed: ").Err(err).String()}
	}
	machine := ""
	for line := range strings.SplitSeq(string(platform), "\n") {
		if !strings.Contains(line, `"IOPlatformUUID"`) {
			continue
		}
		_, value, _ := strings.Cut(line, "=")
		machine = strings.Trim(strings.TrimSpace(value), `"`)
	}
	machineID, ok := normalizedID(machine)
	if !ok {
		return hostFacts{problem: "`ioreg -rd1 -c IOPlatformExpertDevice` printed no IOPlatformUUID"}
	}
	boot, err := readers.command("sysctl", "-n", "kern.bootsessionuuid")
	if err != nil {
		return hostFacts{problem: problem.Str("`sysctl -n kern.bootsessionuuid` failed: ").Err(err).String()}
	}
	bootID, ok := normalizedID(string(boot))
	if !ok {
		return hostFacts{problem: "`sysctl -n kern.bootsessionuuid` printed no UUID"}
	}
	facts := hostFacts{machineID: machineID, bootID: bootID, goos: readers.goos, uid: readers.uid}
	facts.bootStarted, facts.bootStartedProblem = readDarwinBootStarted(readers)
	return facts
}

// readPIDNamespace answers the inode of this process's pid namespace, from
// the `pid:[<inode>]` link the kernel gives.
func readPIDNamespace(readers hostReaders) (string, string) {
	var problem textbuf.Buffer
	link, err := readers.readLink(pidNamespacePath)
	if err != nil {
		return "", problem.Str(pidNamespacePath).Str(" is unreadable: ").Err(err).String()
	}
	malformed := problem.Str(pidNamespacePath).Str(" links to ").Str(link).Str(", not pid:[<inode>]").String()
	inode, prefixed := strings.CutPrefix(link, "pid:[")
	if !prefixed {
		return "", malformed
	}
	inode, closed := strings.CutSuffix(inode, "]")
	if !closed {
		return "", malformed
	}
	if !namespaceID(inode) {
		return "", malformed
	}
	return inode, ""
}

// namespaceID reports whether a pid namespace inode, read from this host or
// from a label, is the decimal number the kernel prints.
func namespaceID(value string) bool {
	if value == "" {
		return false
	}
	if len(value) > 20 {
		return false
	}
	for _, character := range value {
		if character < '0' {
			return false
		}
		if character > '9' {
			return false
		}
	}
	return true
}

// readLinuxBootStarted answers when this machine booted, from the btime line
// of /proc/stat.
func readLinuxBootStarted(readers hostReaders) (time.Time, string) {
	var problem textbuf.Buffer
	data, err := readers.readFile(procStatPath)
	if err != nil {
		return time.Time{}, problem.Str(procStatPath).Str(" is unreadable: ").Err(err).String()
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		seconds, found := strings.CutPrefix(line, "btime ")
		if !found {
			continue
		}
		return bootStartedAt(strings.TrimSpace(seconds), procStatPath)
	}
	return time.Time{}, problem.Str(procStatPath).Str(" holds no btime line").String()
}

// readDarwinBootStarted answers when this Mac booted, from the
// `{ sec = <seconds>, usec = ... }` that `sysctl -n kern.boottime` prints.
func readDarwinBootStarted(readers hostReaders) (time.Time, string) {
	const source = "`sysctl -n kern.boottime`"
	var problem textbuf.Buffer
	output, err := readers.command("sysctl", "-n", "kern.boottime")
	if err != nil {
		return time.Time{}, problem.Str(source).Str(" failed: ").Err(err).String()
	}
	_, rest, found := strings.Cut(string(output), "sec = ")
	if !found {
		return time.Time{}, problem.Str(source).Str(" printed no sec field").String()
	}
	seconds, _, found := strings.Cut(rest, ",")
	if !found {
		return time.Time{}, problem.Str(source).Str(" printed no sec field").String()
	}
	return bootStartedAt(strings.TrimSpace(seconds), source)
}

// bootStartedAt reads a boot time given in seconds since the epoch.
func bootStartedAt(seconds, source string) (time.Time, string) {
	var problem textbuf.Buffer
	value, err := strconv.ParseInt(seconds, 10, 64)
	if err != nil {
		return time.Time{}, problem.Str(source).Str(" gave a boot time that is not a number: ").Str(seconds).String()
	}
	if value < 1 {
		return time.Time{}, problem.Str(source).Str(" gave a boot time before the epoch: ").Str(seconds).String()
	}
	return time.Unix(value, 0).UTC(), ""
}

// normalizedID answers a machine or boot id in lower case: 32 hex digits,
// optionally grouped by dashes as a UUID. Anything else is refused, so a label
// value read back from the daemon is never trusted as an id it does not parse as.
func normalizedID(value string) (string, bool) {
	id := strings.ToLower(strings.TrimSpace(value))
	if len(id) > 36 {
		return "", false
	}
	digits := 0
	for _, character := range id {
		if character == '-' {
			continue
		}
		hex := character >= '0' && character <= '9' || character >= 'a' && character <= 'f'
		if !hex {
			return "", false
		}
		digits++
	}
	if digits != 32 {
		return "", false
	}
	return id, true
}

// ownerVerdict is what the cleanup may do with a labeled resource. Only
// ownerGone permits a removal; every case it cannot prove is ownerUnknown.
type ownerVerdict uint8

const (
	ownerUnspecified ownerVerdict = iota
	ownerGone
	ownerAlive
	ownerUnknown
)

func (v ownerVerdict) String() string {
	switch v {
	case ownerUnspecified:
		return "unspecified"
	case ownerGone:
		return "gone"
	case ownerAlive:
		return "alive"
	case ownerUnknown:
		return "unknown"
	}
	return "invalid"
}

// ownerJudge decides a resource's owner on this client.
type ownerJudge struct {
	host   hostFacts
	lookup func(pid int) (procscan.Process, bool, error)
}

type ownerJudgement struct {
	verdict ownerVerdict
	reason  string
	fix     string
}

// pidLabelMax is the largest pid a label may carry; anything larger, or below
// 1, is malformed and its resource is skipped.
const pidLabelMax = 1<<31 - 1

// judge answers ownerGone only on three proven paths: the same machine and a
// different boot; the same machine and boot with the pid absent; the same
// machine and boot with the pid held by a process of another start token.
// kind is the object the labels were read from: labels whose kind label names
// another object are never a lab run's own, whatever their owner says.
func (j ownerJudge) judge(kind resourceKind, labels map[string]string) ownerJudgement {
	var reason textbuf.Buffer
	if labels[labelMarker] != labelMarkerValue {
		return ownerJudgement{verdict: ownerUnknown,
			reason: "it carries no Ze lab owner label, so nothing proves its owner gone",
			fix:    "inspect it with `docker inspect` and remove it by hand only if no run uses it"}
	}
	malformed := ownerJudgement{verdict: ownerUnknown,
		reason: "its owner labels are missing or malformed, so its owner cannot be judged",
		fix:    "inspect it with `docker inspect` and remove it by hand only if no run uses it"}
	labeledKind := labels[labelKind]
	if labeledKind != kind.labelValue() {
		// A container inherits its image's labels, so one a person starts by
		// hand from a lab-built image says kind image: its owner labels name
		// the image's builder, never the container's creator.
		if kind == resourceContainer && labeledKind == resourceImageTag.labelValue() {
			return ownerJudgement{verdict: ownerUnknown,
				reason: "it carries image labels only, not created by a lab run: a container started from a" +
					" lab-built image inherits them",
				fix: "remove it by hand with `docker rm -f` once you no longer use it"}
		}
		return malformed
	}
	owner, parsed := parseOwnerLabels(labels)
	if !parsed {
		return malformed
	}
	if j.host.problem != "" {
		return ownerJudgement{verdict: ownerUnknown,
			reason: reason.Str("this machine's identity cannot be read (").Str(j.host.problem).Str(")").String(),
			fix: reason.Reset().Str("make ").Str(machineIDPath).Str(", ").Str(bootIDPath).Str(" and ").
				Str(pidNamespacePath).Str(" readable to this user, then run `").Str(cleanupCommand).Str("`").String()}
	}
	if owner.machineID != j.host.machineID {
		return ownerJudgement{verdict: ownerUnknown,
			reason: reason.Str("its owner ran on machine ").Str(owner.machineID).Str(", and this is machine ").
				Str(j.host.machineID).String(),
			fix: reason.Reset().Str("run `").Str(cleanupCommand).Str("` on machine ").Str(owner.machineID).String()}
	}
	if owner.goos != j.host.goos {
		return ownerJudgement{verdict: ownerUnknown,
			reason: reason.Str("its owner ran on ").Str(owner.goos).Str(", and this client runs on ").Str(j.host.goos).String(),
			fix:    reason.Reset().Str("run `").Str(cleanupCommand).Str("` on the machine that created it").String()}
	}
	// A /proc mounted with hidepid=2 hides another user's live pid, so an
	// absent pid proves nothing across uids.
	if owner.uid != j.host.uid {
		return ownerJudgement{verdict: ownerUnknown,
			reason: reason.Str("its owner ran as uid ").Int(int64(owner.uid)).Str(", and this client runs as uid ").
				Int(int64(j.host.uid)).Str(", so a process table that hides other users' processes cannot prove it gone").String(),
			fix: reason.Reset().Str("run `").Str(cleanupCommand).Str("` as uid ").Int(int64(owner.uid)).
				Str(", or remove it by hand once that run ends").String()}
	}
	// Clients in two pid namespaces (a devcontainer mounting the host's
	// docker.sock) share the machine and boot id but not the pids.
	if owner.pidNamespace != j.host.pidNamespace {
		return ownerJudgement{verdict: ownerUnknown,
			reason: reason.Str("its owner ran in pid namespace ").Str(owner.pidNamespace).
				Str(", and this client runs in pid namespace ").Str(j.host.pidNamespace).
				Str(" (another container sharing this daemon), so its pid cannot be looked up here").String(),
			fix: reason.Reset().Str("run `").Str(cleanupCommand).Str("` in the pid namespace that created it,").
				Str(" or remove it by hand once that run ends").String()}
	}
	if owner.bootID != j.host.bootID {
		return j.judgeOtherBoot(owner)
	}
	pid := owner.pid
	start := owner.pidStart
	process, found, err := j.lookup(pid)
	if err != nil {
		return ownerJudgement{verdict: ownerUnknown,
			reason: reason.Str("whether its owner, pid ").Int(int64(pid)).Str(", still runs could not be read: ").
				Err(err).String(),
			fix: reason.Reset().Str("run `").Str(cleanupCommand).Str("` again once the process table is readable").String()}
	}
	if !found {
		return ownerJudgement{verdict: ownerGone,
			reason: reason.Str("its owner, pid ").Int(int64(pid)).Str(", has exited").String()}
	}
	if process.Start != start {
		return ownerJudgement{verdict: ownerGone,
			reason: reason.Str("its owner, pid ").Int(int64(pid)).
				Str(", has exited and the pid now belongs to another process").String()}
	}
	return ownerJudgement{verdict: ownerAlive,
		reason: reason.Str("its owner, pid ").Int(int64(pid)).Str(" (").Join(process.Argv, " ").
			Str("), is still running").String(),
		fix: reason.Reset().Str("that run removes it when it ends; if it is stuck, stop pid ").Int(int64(pid)).
			Str(", then run `").Str(cleanupCommand).Str("`").String()}
}

// parseOwnerLabels reads the owner a resource's labels name. ok is false when
// a label the verdict compares is missing or malformed. A Linux owner always
// carries its pid namespace; a macOS owner has none to carry.
func parseOwnerLabels(labels map[string]string) (ownerIdentity, bool) {
	var owner ownerIdentity
	var ok bool
	if owner.machineID, ok = normalizedID(labels[labelMachineID]); !ok {
		return owner, false
	}
	if owner.bootID, ok = normalizedID(labels[labelBootID]); !ok {
		return owner, false
	}
	pid, err := strconv.Atoi(labels[labelPID])
	if err != nil {
		return owner, false
	}
	if pid < 1 {
		return owner, false
	}
	if pid > pidLabelMax {
		return owner, false
	}
	owner.pid = pid
	owner.pidStart = labels[labelPIDStart]
	if owner.pidStart == "" {
		return owner, false
	}
	uid, err := strconv.Atoi(labels[labelUID])
	if err != nil {
		return owner, false
	}
	if uid < 0 {
		return owner, false
	}
	owner.uid = uid
	owner.goos = labels[labelOS]
	if owner.goos == "" {
		return owner, false
	}
	owner.pidNamespace = labels[labelPIDNamespace]
	if owner.goos == hostGOOSLinux && !namespaceID(owner.pidNamespace) {
		return owner, false
	}
	started, err := time.Parse(timeLayoutRFC3339, labels[labelStarted])
	if err != nil {
		return owner, false
	}
	owner.started = started
	return owner, true
}

// judgeOtherBoot decides an owner recorded under another boot id. A reboot of
// this machine and a cloned machine sharing its machine id both give one, so
// the owner is gone only when it started before this machine booted: no
// process outlives the boot that followed its start. An owner that started
// after this boot runs on another machine, which may be a live clone.
func (j ownerJudge) judgeOtherBoot(owner ownerIdentity) ownerJudgement {
	var reason textbuf.Buffer
	if j.host.bootStartedProblem != "" {
		return ownerJudgement{verdict: ownerUnknown,
			reason: reason.Str("its owner ran under another boot id, and this machine's boot time cannot be read (").
				Str(j.host.bootStartedProblem).Str("), so a reboot cannot be told from a clone sharing this machine id").String(),
			fix: reason.Reset().Str("make ").Str(procStatPath).Str(" (or `sysctl -n kern.boottime` on macOS) readable,").
				Str(" then run `").Str(cleanupCommand).Str("`").String()}
	}
	started := owner.started.UTC().Format(timeLayoutRFC3339)
	booted := j.host.bootStarted.UTC().Format(timeLayoutRFC3339)
	if owner.started.Before(j.host.bootStarted) {
		return ownerJudgement{verdict: ownerGone,
			reason: reason.Str("its owner, pid ").Int(int64(owner.pid)).Str(", started at ").Str(started).
				Str(", before this machine's last boot at ").Str(booted).String()}
	}
	return ownerJudgement{verdict: ownerUnknown,
		reason: reason.Str("its owner started at ").Str(started).Str(", after this machine booted at ").Str(booted).
			Str(", under another boot id: a cloned machine sharing this machine id may still run it").String(),
		fix: reason.Reset().Str("run `").Str(cleanupCommand).Str("` on the machine that created it,").
			Str(" or remove it by hand once no run uses it").String()}
}

// resourceKind is the Docker object a cleanup line names.
type resourceKind uint8

const (
	resourceUnspecified resourceKind = iota
	resourceContainer
	resourceNetwork
	resourceImageTag
)

func (k resourceKind) String() string {
	switch k {
	case resourceUnspecified:
		return "unspecified"
	case resourceContainer:
		return dockerContainerObject
	case resourceNetwork:
		return dockerNetworkObject
	case resourceImageTag:
		return "image tag"
	}
	return "invalid"
}

// labelValue is the value of the labelKind label on a resource of this kind.
func (k resourceKind) labelValue() string {
	switch k {
	case resourceContainer:
		return dockerContainerObject
	case resourceNetwork:
		return dockerNetworkObject
	case resourceImageTag:
		return dockerImageObject
	case resourceUnspecified:
	}
	return ""
}

// MarshalText renders the kind for `| json`.
func (k resourceKind) MarshalText() ([]byte, error) { return []byte(k.String()), nil }

// OrphanResource is one resource a cleanup removed, kept, failed to remove,
// or could not tell whether it removed.
type OrphanResource struct {
	Kind   resourceKind `json:"kind"`
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Reason string       `json:"reason"`
	Fix    string       `json:"fix,omitempty"`
	Error  string       `json:"error,omitempty"`
}

// OrphanReport is what Docker.CleanupOrphans did. In a dry run Removed lists
// what would be removed and nothing is. Unknown holds a resource whose removal
// call failed and whose re-observation failed too, so the daemon said neither
// that it is gone nor that it stays.
type OrphanReport struct {
	Dry       bool             `json:"dry"`
	Removed   []OrphanResource `json:"removed"`
	Skipped   []OrphanResource `json:"skipped"`
	Failed    []OrphanResource `json:"failed"`
	Unknown   []OrphanResource `json:"unknown"`
	Unlabeled []OrphanResource `json:"unlabeled"`
}

// Code is 1 when a removal failed or its outcome is unknown, else 0. A skip
// is never a failure.
func (r OrphanReport) Code() int {
	if len(r.Failed) > 0 {
		return 1
	}
	if len(r.Unknown) > 0 {
		return 1
	}
	return 0
}

// Empty reports that the cleanup met no Ze lab resource at all.
func (r OrphanReport) Empty() bool {
	return len(r.Removed)+len(r.Skipped)+len(r.Failed)+len(r.Unknown)+len(r.Unlabeled) == 0
}

// Lines answers one sentence per resource, for the text report and for a lab
// run's progress output.
func (r OrphanReport) Lines() []string {
	lines := make([]string, 0, len(r.Removed)+len(r.Skipped)+len(r.Failed)+len(r.Unknown)+len(r.Unlabeled))
	var line textbuf.Buffer
	removed := "cleanup: removed "
	if r.Dry {
		removed = "cleanup: would remove "
	}
	for _, resource := range r.Removed {
		lines = append(lines, resourceLine(line.Reset().Str(removed), resource).Str(": ").Str(resource.Reason).String())
	}
	for _, resource := range r.Skipped {
		resourceLine(line.Reset().Str("cleanup: kept "), resource).Str(": ").Str(resource.Reason)
		lines = append(lines, line.Str(". Fix: ").Str(resource.Fix).String())
	}
	for _, resource := range r.Failed {
		resourceLine(line.Reset().Str("cleanup: FAILED to remove "), resource).Str(" (").Str(resource.Reason)
		lines = append(lines, line.Str("): ").Str(resource.Error).Str(". Fix: ").Str(resource.Fix).String())
	}
	for _, resource := range r.Unknown {
		resourceLine(line.Reset().Str("cleanup: UNKNOWN whether this removed "), resource).Str(" (").Str(resource.Reason)
		lines = append(lines, line.Str("): ").Str(resource.Error).Str(". Fix: ").Str(resource.Fix).String())
	}
	for _, resource := range r.Unlabeled {
		resourceLine(line.Reset().Str("cleanup: unlabeled "), resource).Str(": ").Str(resource.Reason)
		lines = append(lines, line.Str(". Fix: ").Str(resource.Fix).String())
	}
	return lines
}

func resourceLine(line *textbuf.Buffer, resource OrphanResource) *textbuf.Buffer {
	line.Str(resource.Kind.String()).Byte(' ').Str(resource.Name)
	if resource.ID != "" {
		line.Str(" (").Str(shortID(resource.ID)).Byte(')')
	}
	return line
}

func shortID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// Text renders the report for an operator: the per-resource lines, then the
// summary.
func (r OrphanReport) Text() string {
	var text textbuf.Buffer
	for _, line := range r.Lines() {
		text.Str(line).Byte('\n')
	}
	return text.Str(r.Summary()).Byte('\n').String()
}

// Summary answers the one line that closes the report, with no newline: the
// totals, or that the cleanup met no Ze lab resource. A sweep that met nothing
// still answers a line, so an operator can tell it from a sweep that never ran.
func (r OrphanReport) Summary() string {
	if r.Empty() {
		return "cleanup: no Ze lab resource on this Docker daemon"
	}
	var text textbuf.Buffer
	text.Str("cleanup: ").Int(int64(len(r.Removed)))
	if r.Dry {
		text.Str(" to remove, ")
	} else {
		text.Str(" removed, ")
	}
	text.Int(int64(len(r.Skipped))).Str(" kept, ").Int(int64(len(r.Failed))).Str(" failed, ").
		Int(int64(len(r.Unknown))).Str(" unknown")
	return text.String()
}

// listedResource is one row of a filtered listing.
type listedResource struct {
	id   string
	name string
}

// CleanupOrphans removes the containers, networks and run-owned image tags
// whose owner labels prove the owner gone, in that order and by the id the
// labeled listing gave. It lists only resources carrying labelMarker, so an
// unlabeled resource, a volume and a cache tag are never candidates. A listing
// the daemon refuses is the returned error; every per-resource outcome is in
// the report. dry lists and removes nothing.
func (d *Docker) CleanupOrphans(ctx context.Context, dry bool) (OrphanReport, error) {
	report := OrphanReport{Dry: dry, Removed: []OrphanResource{}, Skipped: []OrphanResource{},
		Failed: []OrphanResource{}, Unknown: []OrphanResource{}, Unlabeled: []OrphanResource{}}
	containers, err := d.listResources(ctx, resourceContainer, "label="+labelMarker)
	if err != nil {
		return report, err
	}
	networks, err := d.listResources(ctx, resourceNetwork, "label="+labelMarker)
	if err != nil {
		return report, err
	}
	images, err := d.listResources(ctx, resourceImageTag, "label="+labelMarker)
	if err != nil {
		return report, err
	}

	for _, container := range containers {
		d.sweepOne(ctx, &report, resourceContainer, container, container.id)
	}
	for _, network := range networks {
		d.sweepOne(ctx, &report, resourceNetwork, network, network.id)
	}
	labeledImages := map[string]bool{}
	for _, image := range images {
		labeledImages[image.id] = true
		if !strings.HasPrefix(image.name, retainedTagRepository+":") {
			continue
		}
		d.sweepOne(ctx, &report, resourceImageTag, image, image.id)
	}
	if !dry {
		return report, nil
	}

	// Q-3: run-owned tags from before the labels existed are listed once, as
	// an aid, and never removed: nothing proves their owner gone.
	retained, err := d.listResources(ctx, resourceImageTag, "reference="+retainedTagRepository)
	if err != nil {
		return report, err
	}
	var fix textbuf.Buffer
	for _, image := range retained {
		if labeledImages[image.id] {
			continue
		}
		report.Unlabeled = append(report.Unlabeled, OrphanResource{
			Kind: resourceImageTag, ID: image.id, Name: image.name,
			Reason: "it predates the owner labels, so this command never removes it",
			Fix:    fix.Reset().Str("`docker image rm --no-prune ").Str(image.name).Str("` once no run uses it").String(),
		})
	}
	return report, nil
}

// sweepOne judges one listed resource and, outside a dry run, removes it when
// its owner is gone. inspectID is the id its labels are read from: an image
// tag's labels are its image's.
func (d *Docker) sweepOne(ctx context.Context, report *OrphanReport, kind resourceKind, listed listedResource, inspectID string) {
	resource := OrphanResource{Kind: kind, ID: listed.id, Name: listed.name}
	labels, vanished, err := d.resourceLabels(ctx, kind, inspectID)
	if vanished {
		return
	}
	if err != nil {
		resource.Reason = "its owner labels could not be read"
		resource.Error = err.Error()
		resource.Fix = "run `" + cleanupCommand + "` again once the daemon answers"
		report.Failed = append(report.Failed, resource)
		return
	}
	judgement := d.judge.judge(kind, labels)
	resource.Reason = judgement.reason
	if judgement.verdict != ownerGone {
		resource.Fix = judgement.fix
		report.Skipped = append(report.Skipped, resource)
		return
	}
	if report.Dry {
		report.Removed = append(report.Removed, resource)
		return
	}
	err = d.removeResource(ctx, kind, listed)
	if err == nil {
		report.Removed = append(report.Removed, resource)
		return
	}
	if _, unknown := errors.AsType[*removalUnknownError](err); unknown {
		resource.Error = err.Error()
		resource.Fix = "run `" + cleanupCommand + "` again once the daemon answers, which removes it if it stays"
		report.Unknown = append(report.Unknown, resource)
		return
	}
	if kind == resourceNetwork && strings.Contains(err.Error(), "active endpoints") {
		resource.Reason = "a container whose owner is not proven gone is still attached to it (active endpoints)"
		resource.Fix = "run `" + cleanupCommand + "` again once that container's run ends"
		report.Skipped = append(report.Skipped, resource)
		return
	}
	resource.Error = err.Error()
	resource.Fix = removalCommand(kind, listed) + " by hand, or run `" + cleanupCommand + "` again"
	report.Failed = append(report.Failed, resource)
}

// removeResource removes one listed resource by id, or an image tag by tag.
// Docker's name-specific absent answer is success (R-6): another sweep, or
// the owner's own teardown, removed it first. A failed call is re-observed
// (confirmRemoval): gone is success, a failed observation a
// removalUnknownError.
func (d *Docker) removeResource(ctx context.Context, kind resourceKind, listed listedResource) error {
	switch kind {
	case resourceContainer:
		if !dockerID(listed.id) {
			return errors.New("the container listing gave an id that is not hex: " + listed.id)
		}
		return d.RemoveContainer(ctx, listed.id)
	case resourceNetwork:
		if !dockerID(listed.id) {
			return errors.New("the network listing gave an id that is not hex: " + listed.id)
		}
		return d.RemoveNetwork(ctx, listed.id)
	case resourceImageTag:
		return d.ReleaseImage(ctx, ImageResult{retainedTag: listed.name})
	case resourceUnspecified:
	}
	return errors.New("BUG: removeResource called without a resource kind")
}

// removalCommand is the command an operator types to remove one resource.
func removalCommand(kind resourceKind, listed listedResource) string {
	var command textbuf.Buffer
	switch kind {
	case resourceContainer:
		command.Str("run `docker rm -f ").Str(listed.id).Byte('`')
	case resourceNetwork:
		command.Str("run `docker network rm ").Str(listed.id).Byte('`')
	case resourceImageTag:
		command.Str("run `docker image rm --no-prune ").Str(listed.name).Byte('`')
	case resourceUnspecified:
		command.Str("remove it")
	}
	return command.String()
}

// dockerID reports whether an id read back from the daemon is the hex a
// removal argv may carry.
func dockerID(id string) bool {
	if len(id) < 12 {
		return false
	}
	if len(id) > 64 {
		return false
	}
	for _, character := range id {
		hex := character >= '0' && character <= '9' || character >= 'a' && character <= 'f'
		if !hex {
			return false
		}
	}
	return true
}

// listResources runs the listing of one kind under one filter. Image rows are
// one per tag; untagged images carry no tag the cleanup could remove.
func (d *Docker) listResources(ctx context.Context, kind resourceKind, filter string) ([]listedResource, error) {
	var argv []string
	switch kind {
	case resourceContainer:
		argv = []string{DockerExecutable, "ps", "-a", dockerNoTruncFlag, dockerFilterFlag, filter, dockerFormatFlag, "{{.ID}}\t{{.Names}}"}
	case resourceNetwork:
		argv = []string{DockerExecutable, dockerNetworkObject, "ls", dockerNoTruncFlag, dockerFilterFlag, filter, dockerFormatFlag, "{{.ID}}\t{{.Name}}"}
	case resourceImageTag:
		argv = []string{DockerExecutable, dockerImageObject, "ls", dockerNoTruncFlag, dockerFilterFlag, filter, dockerFormatFlag, "{{.ID}}\t{{.Repository}}:{{.Tag}}"}
	case resourceUnspecified:
		return nil, errors.New("BUG: listResources called without a resource kind")
	}
	result, err := d.command(ctx, dockerCommandTimeout, argv...)
	if err != nil {
		return nil, err
	}
	var rows []listedResource
	for line := range strings.SplitSeq(result.Stdout, "\n") {
		id, name, found := strings.Cut(strings.TrimSpace(line), "\t")
		if !found {
			continue
		}
		if strings.HasSuffix(name, ":<none>") {
			continue
		}
		rows = append(rows, listedResource{id: id, name: name})
	}
	return rows, nil
}

// resourceLabels reads one resource's labels. vanished is true when the daemon
// answers that the resource no longer exists, which a concurrent sweep or the
// owner's own teardown produces.
func (d *Docker) resourceLabels(ctx context.Context, kind resourceKind, id string) (map[string]string, bool, error) {
	format := "{{json .Config.Labels}}"
	if kind == resourceNetwork {
		format = "{{json .Labels}}"
	}
	argv, err := inspectCommand(kind, format, id)
	if err != nil {
		return nil, false, err
	}
	result, err := d.command(ctx, dockerCommandTimeout, argv...)
	if err != nil {
		if absentAnswer(err, id) {
			return nil, true, nil
		}
		return nil, false, err
	}
	labels := map[string]string{}
	if text := strings.TrimSpace(result.Stdout); text != "null" {
		if jsonErr := json.Unmarshal([]byte(text), &labels); jsonErr != nil {
			var problem textbuf.Buffer
			return nil, false, errors.New(problem.Str("`").Join(argv, " ").Str("` printed unreadable labels: ").
				Err(jsonErr).String())
		}
	}
	return labels, false, nil
}

// resourcePresent observes one resource by id, name or tag, under the same
// bound as every other daemon call. A failed observation is the error, never
// an answer either way.
func (d *Docker) resourcePresent(ctx context.Context, kind resourceKind, ref string) (bool, error) {
	argv, err := inspectCommand(kind, "{{.Id}}", ref)
	if err != nil {
		return false, err
	}
	_, err = d.command(ctx, dockerCommandTimeout, argv...)
	if err == nil {
		return true, nil
	}
	if absentAnswer(err, ref) {
		return false, nil
	}
	return false, err
}

// inspectCommand is the inspect argv that prints format for one resource.
func inspectCommand(kind resourceKind, format, ref string) ([]string, error) {
	switch kind {
	case resourceContainer:
		return []string{DockerExecutable, dockerContainerObject, dockerInspectVerb, dockerFormatFlag, format, ref}, nil
	case resourceNetwork:
		return []string{DockerExecutable, dockerNetworkObject, dockerInspectVerb, dockerFormatFlag, format, ref}, nil
	case resourceImageTag:
		return []string{DockerExecutable, dockerImageObject, dockerInspectVerb, dockerFormatFlag, format, ref}, nil
	case resourceUnspecified:
	}
	return nil, errors.New("BUG: a Docker inspect was asked for without a resource kind")
}

// absentAnswer reports whether a failed inspect is the daemon saying that
// exactly ref does not exist, rather than a daemon that did not answer or an
// absence named for another resource.
func absentAnswer(err error, ref string) bool {
	commandErr, ok := errors.AsType[*commandError](err)
	if !ok {
		return false
	}
	if commandErr.Cause != nil {
		return false
	}
	for line := range strings.SplitSeq(commandErr.Stderr, "\n") {
		if !strings.Contains(line, ref) {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "no such") {
			return true
		}
		if strings.Contains(lower, "not found") {
			return true
		}
	}
	return false
}

// claimName clears an existing resource of exactly this name before a
// scenario creates its own: removed when every owner label equals this run's
// own (an earlier scenario's leftover, ownerIdentity.created) or its owner is
// proven gone, refused otherwise, unlabeled included. A run never force-removes
// a name another live run may hold. The refusal's fix names suffixVariable, the lab's own
// suffix variable, when it has one, and otherwise the verdict's own fix.
func (d *Docker) claimName(ctx context.Context, kind resourceKind, name, suffixVariable string) error {
	rows, err := d.listResources(ctx, kind, "name="+name)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.name != name {
			continue
		}
		labels, vanished, labelErr := d.resourceLabels(ctx, kind, row.id)
		if vanished {
			continue
		}
		if labelErr != nil {
			return labelErr
		}
		if d.owner.created(kind, labels) {
			if removeErr := d.removeResource(ctx, kind, row); removeErr != nil {
				return removeErr
			}
			continue
		}
		judgement := d.judge.judge(kind, labels)
		if judgement.verdict == ownerGone {
			if removeErr := d.removeResource(ctx, kind, row); removeErr != nil {
				return removeErr
			}
			continue
		}
		var refusal textbuf.Buffer
		refusal.Str(kind.String()).Byte(' ').Str(name).Str(" already exists and is not this run's to remove: ").
			Str(judgement.reason).Str(". Fix: ")
		if suffixVariable != "" {
			refusal.Str("set ").Str(suffixVariable).Str(" to a value no other run uses, or ")
		}
		refusal.Str(judgement.fix)
		return errors.New(refusal.String())
	}
	return nil
}
