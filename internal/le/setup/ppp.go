// Design: docs/guide/developer-setup.md -- /dev/ppp access for the developer
// Related: system.go -- the other machine-state steps, and privilegedStep
//
// The L2TP and PPPoE tests open /dev/ppp to drive the kernel PPP channel. The
// device is root:root 0600 on a stock Debian host, so a test run as the
// developer cannot open it, and a test that needs root runs nowhere an agent
// works. This step grants the device to the dip group and puts the developer
// in it.
//
// Why dip: Debian ships the group (gid 30) for exactly this, "PPP dial-up";
// pppd's own documentation names it, and the developer is usually a member
// already. A second, Ze-only group would grant the same device to the same
// person under another name. A host with no dip group gets one, made a system
// group, because the group is the whole of the grant.

package setup

import (
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// pppDevDefault is the device, pppRuleDefault the udev rule that grants it.
const pppDevDefault = "/dev/ppp"

const pppRuleDefault = "/etc/udev/rules.d/60-ze-ppp.rules"

// pppGroup is the group the rule grants /dev/ppp to.
const pppGroup = "dip"

// pppModule is the kernel module that registers /dev/ppp.
const pppModule = "ppp_generic"

// pppRuleComment and pppRuleLine are the two lines of the rule file.
// KERNEL== applies the rule when the module registers the device. static_node=
// applies it at boot to the node systemd creates before the module loads, so
// the first open (which autoloads ppp_generic) is already permitted. systemd's
// own 50-udev-default.rules grants /dev/kvm with the same line shape.
const pppRuleComment = "# ./le setup install: the dip group may open /dev/ppp, for the L2TP and PPPoE tests."

const pppRuleLine = `KERNEL=="ppp", GROUP="dip", MODE="0660", OPTIONS+="static_node=ppp"`

// pppRuleBody is the whole rule file.
const pppRuleBody = pppRuleComment + "\n" + pppRuleLine + "\n"

// ppp is whether this user can open /dev/ppp.
type ppp string

const (
	// pppOK means this process can open /dev/ppp for reading and writing now.
	pppOK ppp = "ok"
	// pppPendingLogin means the rule, the node and the membership are all in
	// place, but this session started before the membership. Group membership
	// is fixed at login.
	pppPendingLogin ppp = "pending-login"
	// pppNotGranted means a piece the install run supplies is absent.
	pppNotGranted ppp = "not-granted"
)

// pppFacts is each piece the grant needs, read from the machine.
type pppFacts struct {
	// group means the dip group exists.
	group bool
	// rule means the udev rule file holds pppRuleBody.
	rule bool
	// node means /dev/ppp exists.
	node bool
	// nodeGranted means /dev/ppp belongs to dip and grants it read and write.
	nodeGranted bool
	// member means the group database lists the user in dip.
	member bool
	// usable means this process may open /dev/ppp read-write now.
	usable bool
}

// pppDev answers the device the state is read from.
func (s *Setup) pppDev() string {
	if s.PppDev != "" {
		return s.PppDev
	}
	return pppDevDefault
}

// pppRule answers the rule file the state is read from and written to.
func (s *Setup) pppRule() string {
	if s.PppRule != "" {
		return s.PppRule
	}
	return pppRuleDefault
}

// dipGroupID answers the dip group's id, and whether the group exists.
func (s *Setup) dipGroupID() (int, bool) {
	if s.DipGroupID != nil {
		return s.DipGroupID()
	}
	group, err := user.LookupGroup(pppGroup)
	if err != nil {
		return 0, false
	}
	gid, err := strconv.Atoi(group.Gid)
	if err != nil {
		return 0, false
	}
	return gid, true
}

// inDipGroup reports whether the group database lists this user in dip. Like
// inKvmGroup it does not test the device: it tells "log back in" apart from
// "run this command".
func (s *Setup) inDipGroup() bool {
	if s.DipGroupMember != nil {
		return s.DipGroupMember()
	}
	return groupDatabaseLists(pppGroup)
}

// pppProbe reads every piece of the grant.
//
// usable is judged by access(2), not by opening the device as deviceOpenable
// does for /dev/kvm: opening /dev/ppp while ppp_generic is absent loads the
// module, and a check run changes nothing. access(2) reads the same permission
// bits against this process's ids and groups, which are the ones a test it
// starts holds. A rule file that exists and cannot be read is an error, because
// its content is then unknown.
func (s *Setup) pppProbe() (pppFacts, error) {
	var facts pppFacts
	device := s.pppDev()

	facts.usable = unix.Access(device, unix.R_OK|unix.W_OK) == nil

	gid, groupExists := s.dipGroupID()
	facts.group = groupExists
	if groupExists {
		facts.member = s.inDipGroup()
	}

	info, err := os.Stat(device)
	if err == nil {
		facts.node = true
		stat, ok := info.Sys().(*syscall.Stat_t)
		if ok && groupExists && int(stat.Gid) == gid {
			facts.nodeGranted = info.Mode().Perm()&0o060 == 0o060
		}
	}

	body, err := os.ReadFile(s.pppRule())
	if err != nil && !os.IsNotExist(err) {
		return facts, err
	}
	facts.rule = err == nil && string(body) == pppRuleBody
	return facts, nil
}

// pppState answers whether this user can open /dev/ppp, with the facts it was
// judged from.
func (s *Setup) pppState() (ppp, pppFacts, error) {
	facts, err := s.pppProbe()
	if err != nil {
		return pppNotGranted, facts, err
	}
	if facts.usable {
		return pppOK, facts, nil
	}
	if facts.rule && facts.nodeGranted && facts.member {
		return pppPendingLogin, facts, nil
	}
	return pppNotGranted, facts, nil
}

// pppLacking names each absent piece, for the outcome detail.
func pppLacking(facts pppFacts) []string {
	var lacking []string
	if !facts.group {
		lacking = append(lacking, "no dip group")
	}
	if !facts.rule {
		lacking = append(lacking, "no udev rule")
	}
	if !facts.node {
		lacking = append(lacking, "no /dev/ppp")
	}
	if !facts.nodeGranted {
		lacking = append(lacking, "/dev/ppp not dip 0660")
	}
	if !facts.member {
		lacking = append(lacking, "not in the dip group")
	}
	return lacking
}

// pppSteps answers the root commands that supply each absent piece, in the
// order they depend on each other: the group before anything names it, the
// rule before the module registers the device, the module before its node is
// changed.
//
// The node is changed directly rather than through `udevadm trigger`, because a
// static node with no module loaded has no sysfs device for udev to act on. The
// rule then keeps the grant across module loads and reboots.
func pppSteps(facts pppFacts, device, rule, userName string) []privilegedStep {
	var steps []privilegedStep
	if !facts.group {
		steps = append(steps, privilegedStep{argv: []string{"groupadd", "--system", pppGroup}})
	}
	if !facts.rule {
		var shown textbuf.Buffer
		shown.Str(`printf '%s\n' '`).Str(pppRuleComment).Str(`' '`).Str(pppRuleLine).Str(`' | `).
			Str(sudoPlaceholder).Str("tee ").Str(rule)
		steps = append(steps,
			privilegedStep{argv: []string{"tee", rule}, stdin: []byte(pppRuleBody), shown: shown.String()},
			privilegedStep{argv: []string{"udevadm", "control", "--reload"}},
		)
	}
	if !facts.node {
		steps = append(steps, privilegedStep{argv: []string{"modprobe", pppModule}})
	}
	if !facts.nodeGranted {
		steps = append(steps,
			privilegedStep{argv: []string{"chgrp", pppGroup, device}},
			privilegedStep{argv: []string{"chmod", "0660", device}},
		)
	}
	if !facts.member {
		steps = append(steps, privilegedStep{argv: []string{usermodBin, "-aG", pppGroup, userName}})
	}
	return steps
}

// visitPpp reports whether the developer can open /dev/ppp, and grants it on
// an install run.
func (s *Setup) visitPpp(report *Report) Outcome {
	const name = "ppp-access"

	if s.goos() != osLinux {
		return Outcome{Name: name, State: StateSkipped, Detail: "Linux only: macOS has no /dev/ppp; the PPP tests run in the Linux VM"}
	}

	state, facts, err := s.pppState()
	if err != nil {
		var tb textbuf.Buffer
		return Outcome{Name: name, State: StateMissing, Detail: tb.Str("unreadable: ").Err(err).String()}
	}
	switch state {
	case pppOK:
		return Outcome{Name: name, State: StatePresent}
	case pppPendingLogin:
		return Outcome{Name: name, State: StatePending, Detail: pppLoginDetail()}
	case pppNotGranted:
		// The steps below correct only this state.
	default:
		panic("BUG: unknown /dev/ppp access state")
	}

	var listed textbuf.Buffer
	lacking := listed.Join(pppLacking(facts), ", ").String()
	if s.Check {
		var tb textbuf.Buffer
		return Outcome{Name: name, State: StateMissing, Detail: tb.Str(detailRequired).Str(": ").Str(lacking).String()}
	}

	steps := pppSteps(facts, s.pppDev(), s.pppRule(), s.userName())
	if s.applyPpp(report, steps) {
		after, _, err := s.pppState()
		if err == nil && after == pppOK {
			return Outcome{Name: name, State: StateInstalled}
		}
		if err == nil && after == pppPendingLogin {
			return Outcome{Name: name, State: StatePending, Detail: pppLoginDetail()}
		}
	}
	report.Note("  Could not apply automatically; run manually:")
	notePppFix(report, steps)
	return Outcome{Name: name, State: StatePending, Detail: lacking}
}

// pppLoginDetail is the pending answer: what to do once the group is granted.
func pppLoginDetail() string {
	var tb textbuf.Buffer
	return tb.Str("in the dip group; log out and back in, or use: sg ").Str(pppGroup).Str(" -c '<command>'").String()
}

// applyPpp runs each step, and answers false at the first that fails.
func (s *Setup) applyPpp(report *Report, steps []privilegedStep) bool {
	for _, step := range steps {
		ok, detail := s.Shell.runPrivileged(report, step.argv, step.stdin, step.shown)
		if !ok {
			var tb textbuf.Buffer
			report.Note(tb.Str("  FAIL: ").Str(detail).String())
			return false
		}
	}
	return true
}

// notePppFix records the commands, for when root is out of reach.
func notePppFix(report *Report, steps []privilegedStep) {
	for _, step := range steps {
		var tb textbuf.Buffer
		if step.shown != "" {
			report.Note(tb.Str("  Run: ").Str(strings.ReplaceAll(step.shown, sudoPlaceholder, "sudo ")).String())
			continue
		}
		report.Note(tb.Str("  Run: sudo ").Join(step.argv, " ").String())
	}
	if len(steps) == 0 {
		return
	}
	if steps[len(steps)-1].argv[0] != usermodBin {
		return
	}
	// The membership is the last step, and only it waits for a new login.
	var then textbuf.Buffer
	report.Note(then.Str("  Then log out and back in, or prefix a command with: sg ").
		Str(pppGroup).Str(" -c '<command>'").String())
}
