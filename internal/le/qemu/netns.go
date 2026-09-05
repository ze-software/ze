// Design: docs/architecture/testing/qemu-integration.md -- the per-test network namespace
// Overview: netns_linux.go -- the netns-test launcher, one selected subset at a time
// Related: alltests.go -- the whole-suite run that routes four suites the same way
//
// netns.go holds what the two callers of the per-test network namespace launch
// mode share: where the capability-carrying binaries live, where the dropped
// user's state lives, the capability set, and the environment keys the .ci
// runner reads (netnsModeActive and netnsChildIDs,
// internal/test/runner/runner_exec_util.go).
//
// The constants are HERE rather than in netns_linux.go because `all-tests`
// compiles on darwin as well and routes four suites through the same mode. One
// declaration, two callers: a capability set that differed between them would
// mean the whole-suite run proves something the focused run does not.

package qemu

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Where the launcher puts what the dropped user needs.
//
// The binaries are COPIED off the 9p mount. File capabilities are an extended
// attribute, and the 9p share does not carry one, so a setcap on /workspace
// silently buys nothing. /tmp is a tmpfs in the guest and does.
const (
	netnsCapDir   = "/tmp/zebin"
	netnsStateDir = "/tmp/zestate"
)

// netnsCapabilities is the set ze needs to program netlink, nftables and a
// privileged port while running as an ordinary user inside the namespace.
const netnsCapabilities = "cap_net_admin,cap_net_raw,cap_net_bind_service+ep"

// The environment the .ci runner reads to enter the mode.
//
// netnsUID is a number rather than a name on purpose: the guest is a live
// Alpine image whose passwd file this run does not edit, and the runner drops
// the daemon with setuid, which takes the number.
const (
	netnsModeKey   = "ZE_TEST_NETNS"
	netnsModeValue = "1"
	netnsUIDKey    = "ZE_TEST_UID"
	netnsGIDKey    = "ZE_TEST_GID"
	netnsUID       = "1000"
	netnsConfigKey = "ze.config.dir"
)

// The two commands the mode needs on the guest's PATH, from Alpine's `libcap`
// package. verifyNamespaces names the package in its refusal, because
// "setcap: not found" names the binary and a reader then has to guess which
// package carries it.
const (
	setcapCommand = "setcap"
	getcapCommand = "getcap"
	libcapPackage = "libcap"
)

// networkNamespace is where a suite's tests program the kernel.
//
// A named set rather than a bool, because the report carries the answer and a
// reader of `| json` needs the word rather than `true`. The zero value is
// unspecified and no suite may keep it: verifyNamespaces refuses a run whose
// table leaves one silent, so a suite added tomorrow states where it runs
// instead of inheriting the answer that cut the transport today.
type networkNamespace int

const (
	// namespaceUnspecified is the Go zero value and never a valid answer.
	namespaceUnspecified networkNamespace = iota
	// guestRoot is the guest's own network namespace, which is also where the
	// SSH transport that carries this run lives.
	guestRoot
	// perTest is a fresh network namespace for each test, entered by the .ci
	// runner before it spawns anything (enterTestNetns,
	// internal/test/runner/netns_linux.go). Nothing the test programs reaches
	// the transport, and ze runs as an ordinary user holding file capabilities.
	perTest
)

// String names the namespace for the report and for a refusal message.
func (n networkNamespace) String() string {
	switch n {
	case guestRoot:
		return "guest-root"
	case perTest:
		return "per-test"
	default:
		return "unspecified"
	}
}

// verifyNamespaces refuses a suite that does not say which network namespace
// its tests run in.
//
// The default is the one that cut the transport, so it is not a default. A row
// added without a Namespace is a decision nobody made, and this is where it is
// caught rather than an hour into a run.
func (a *allTestsRun) verifyNamespaces() error {
	var silent []string
	for _, suite := range vmSuites {
		if suite.Namespace == namespaceUnspecified {
			silent = append(silent, suite.Name)
		}
	}
	if len(silent) > 0 {
		// The sentinel is WRAPPED rather than rendered into a new message: a
		// caller, and this package's own tests, ask errors.Is which refusal this
		// is, and a match on the text would break on any wording change.
		return fmt.Errorf("%w: %s", ErrNamespaceUnspecified, strings.Join(silent, ", "))
	}

	if !a.usesPerTestNamespace() {
		return nil
	}
	for _, command := range []string{setcapCommand, getcapCommand} {
		if _, err := a.look(command); err != nil {
			var tb textbuf.Buffer
			return errors.New(tb.Str("qemu: ").Str(command).
				Str(" is not on the guest PATH, so the per-test network namespace suites cannot").
				Str(" hold their capabilities: add `packages \"").Str(libcapPackage).
				Str("\"` to the ./le qemu run command line").String())
		}
	}
	return nil
}

// usesPerTestNamespace reports whether this run will start a suite that needs
// the per-test network namespace. A skipped suite needs nothing.
func (a *allTestsRun) usesPerTestNamespace() bool {
	for _, suite := range vmSuites {
		if suite.Namespace == perTest && !slices.Contains(a.Skip, suite.Name) {
			return true
		}
	}
	return false
}

// prepareNamespace gives the per-test namespace launcher what it needs: a
// capability-carrying copy of ze and ze-stripped on a filesystem that keeps an
// extended attribute, and a state directory the dropped user owns.
//
// It is the same preparation ./le qemu netns-test performs
// (prepareNetnsBinaries and prepareNetnsState, netns_linux.go). It runs as one
// child so the run has one exit code to judge and one command line to report.
func (a *allTestsRun) prepareNamespace(environ []string) error {
	if !a.usesPerTestNamespace() {
		return nil
	}

	var tb textbuf.Buffer
	tb.Str("set -e").
		Str("; mkdir -p ").Str(netnsCapDir).
		Str("; chown 0:").Str(netnsUID).Byte(' ').Str(netnsCapDir).
		Str("; chmod 0750 ").Str(netnsCapDir).
		Str("; rm -rf ").Str(netnsStateDir).
		Str("; mkdir -p ").Str(netnsStateDir).
		Str("; chown ").Str(netnsUID).Byte(':').Str(netnsUID).Byte(' ').Str(netnsStateDir).
		Str("; chmod 0750 ").Str(netnsStateDir)
	for name, source := range map[string]string{
		zeName:         a.workspacePath(a.ZeBin),
		zeStrippedName: a.workspacePath(a.StrippedBin),
	} {
		target := filepath.Join(netnsCapDir, name)
		tb.Str("; cp ").Str(source).Byte(' ').Str(target).
			Str("; chmod 0755 ").Str(target).
			Str("; ").Str(setcapCommand).Byte(' ').Str(netnsCapabilities).Byte(' ').Str(target).
			Str("; ").Str(getcapCommand).Byte(' ').Str(target)
	}

	argv := []string{"sh", "-c", tb.String()}
	a.note(banner("per-test network namespace preparation"))
	if code := a.child(argv, environ, nil); code != 0 {
		return fmt.Errorf("%w: exit %d", ErrNamespacePreparation, code)
	}
	return nil
}

// suiteEnvironment is what one suite's child is given: the run's environment,
// plus what the per-test network namespace launcher reads.
//
// The .ci runner enters the namespace itself, once for each test, and it does
// that only when ZE_TEST_NETNS is set and a non-root uid is named
// (netnsModeActive and netnsChildIDs,
// internal/test/runner/runner_exec_util.go). ze is then an ordinary user, so
// it takes its privileges from the capability copies prepareNamespace made and
// writes its state where that user can.
func (a *allTestsRun) suiteEnvironment(suite vmSuite, environ []string) []string {
	if suite.Namespace != perTest {
		return environ
	}
	suiteEnv := append([]string(nil), environ...)
	suiteEnv = setEnv(suiteEnv, netnsModeKey, netnsModeValue)
	suiteEnv = setEnv(suiteEnv, netnsUIDKey, netnsUID)
	suiteEnv = setEnv(suiteEnv, netnsGIDKey, netnsUID)
	suiteEnv = setEnv(suiteEnv, netnsConfigKey, netnsStateDir)
	suiteEnv = setEnv(suiteEnv, zeBinKey, filepath.Join(netnsCapDir, zeName))
	return setEnv(suiteEnv, strippedBinKey, filepath.Join(netnsCapDir, zeStrippedName))
}
