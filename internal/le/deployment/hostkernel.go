// Design: docs/architecture/testing/interop.md -- what the host must provide
// Overview: l2tpppp.go -- the proof this gate stands in front of
// Related: netns.go -- the namespaces the proof builds once this passes
// Related: l2tpppp.go -- the on-host proof this gate stands in front of
//
// hostkernel.go answers whether THIS machine can carry a full L2TP PPP proof.
// The proof uses the kernel's own L2TP and PPP implementations rather than a
// user-space stand-in. It needs the character device that pppd opens and the
// PPPoL2TP module that receives the session. It also needs the Generic Netlink
// family that ip uses and the capability that permits creation of a namespace.
//
// The gate reports every missing requirement LOUDLY rather than skipping it. A
// proof that reports a pass on a machine with no PPPoL2TP support has proven
// nothing. The operator cannot distinguish that pass from the real one
// (ai/rules/completion.md).

package deployment

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// These are the two spellings of the variable that turns off ze's own kernel
// probe. Either one makes the daemon skip the check that the proof exists to
// make. A run that finds either one set refuses to proceed rather than report a
// pass over a user-space path.
const (
	SkipKernelProbeEnv = "ZE_L2TP_SKIP_KERNEL_PROBE"
	SkipKernelProbeKey = "ze.l2tp.skip-kernel-probe"
)

// devPPP is the character device pppd opens to create a PPP unit.
const devPPP = "/dev/ppp"

// pppol2tpEvidence lists the locations where the kernel reports support for PPP
// over L2TP. Any one is enough. The first is the module's own procfs entry. The
// other two are module directories under the two names that the module has had.
var pppol2tpEvidence = []string{
	"/proc/net/pppol2tp",
	"/sys/module/l2tp_ppp",
	"/sys/module/pppol2tp",
}

// l2tpModules are the modules loaded before the kernel is asked about PPPoL2TP.
// A distribution that ships them unloaded answers "no support" until something
// asks for them, so the run asks first and then reads the answer.
var l2tpModules = []string{"ppp_generic", "l2tp_core", "l2tp_netlink", "pppox", "l2tp_ppp"}

// capNetAdmin is the capability bit that lets this process make a namespace and
// move a link into it. It is bit 12 of the effective capability set.
const capNetAdmin = 12

// requireLinux answers an error on any other system. The proof drives Linux
// kernel objects, so there is nothing to run elsewhere and nothing to report.
func requireLinux(what string) error {
	if runtime.GOOS == "linux" {
		return nil
	}
	var tb textbuf.Buffer
	return errors.New(tb.Str(what).Str(" requires Linux").String())
}

// refuseSkipKernelProbe answers an error when either spelling of ze's
// kernel-probe escape is set in this process's environment.
//
// The function reads the OS environment instead of env.Get. It must detect
// whether the daemon will inherit the variable. A registered default would
// return a value for a variable that nobody set.
func refuseSkipKernelProbe() error {
	for _, key := range []string{SkipKernelProbeEnv, SkipKernelProbeKey} {
		if _, set := os.LookupEnv(key); !set {
			continue
		}
		var tb textbuf.Buffer
		return errors.New(tb.Str("refusing to run with ").Str(key).
			Str(" set; full proof must not skip the kernel probe").String())
	}
	return nil
}

// hasNetAdmin reports whether this process CAN build a network namespace.
//
// The function answers root first because a root process holds the capability
// regardless of what the bounding set says. For anybody else, the function
// reads the effective set from procfs. That is the only place where a process
// can learn its own capabilities without a syscall wrapper.
func hasNetAdmin() bool {
	if os.Geteuid() == 0 {
		return true
	}
	body, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(string(body), "\n") {
		rest, ok := strings.CutPrefix(line, "CapEff:")
		if !ok {
			continue
		}
		effective, err := strconv.ParseUint(strings.TrimSpace(rest), 16, 64)
		if err != nil {
			return false
		}
		return effective&(1<<capNetAdmin) != 0
	}
	return false
}

// loadL2TPModules asks the kernel for each module that the proof needs.
//
// Each call is a request rather than a requirement. A kernel with the code built
// in answers "module not found" for every one of them and carries the proof
// perfectly well. The evidence read afterwards decides whether support exists.
// These calls only make sure the answer is not "not loaded yet".
//
// The function does nothing for a process that is not root because modprobe
// would be refused. It also does nothing on a machine with no modprobe at all.
func loadL2TPModules() {
	if os.Geteuid() != 0 {
		return
	}
	if _, err := exec.LookPath("modprobe"); err != nil {
		return
	}
	for _, module := range l2tpModules {
		hostText("modprobe", module) //nolint:errcheck // a module that will not load is judged by the evidence read after this
	}
}

// ensureKernelSupport answers an error that names the first thing this machine
// does not provide.
//
// The checks put the cheapest question first and the one that starts a process
// last. An operator on a laptop therefore learns they are not root before
// anything is loaded.
func ensureKernelSupport(what string) error {
	if err := requireLinux(what); err != nil {
		return err
	}
	var tb textbuf.Buffer
	if !hasNetAdmin() {
		return errors.New(tb.Str(what).Str(" requires root or CAP_NET_ADMIN").String())
	}

	info, err := os.Stat(devPPP)
	if err != nil {
		return errors.New("missing /dev/ppp")
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return errors.New("/dev/ppp exists but is not a character device")
	}

	loadL2TPModules()

	if !anyPathExists(pppol2tpEvidence) {
		return errors.New("missing PPPoL2TP kernel support: expected /proc/net/pppol2tp or l2tp_ppp module")
	}

	if out, ok := hostText("ip", "l2tp", ipShow, "tunnel"); !ok {
		return errors.New(tb.Str("ip l2tp cannot access the kernel L2TP Generic Netlink family: ").
			Str(strings.TrimSpace(out)).String())
	}
	return nil
}

// anyPathExists reports whether any of the paths is present.
func anyPathExists(paths []string) bool {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// These two variables point a proof at a ze binary that it did not build. The
// first is shared by every evidence run. The second is this proof's own.
const (
	EvidenceBinaryKey = "ze.evidence.ze.binary"
	L2TPPPPBinaryKey  = "ze.l2tp.ppp.ze.binary"
)

var (
	evidenceBinaryEntry = stringSetting(EvidenceBinaryKey, "",
		"a ze binary the deployment proofs run instead of building one")
	l2tpPPPBinaryEntry = stringSetting(L2TPPPPBinaryKey, "",
		"a ze binary the on-host L2TP PPP proof runs instead of building one")
)

// hostDaemon answers the ze binary that the proof drives. If the operator names
// none, the function builds it.
//
// The function checks an override rather than trusting it. A path that does not
// exist or cannot be executed is an operator error worth naming. Without this
// check, the command fails deep inside a namespace where the reason is much
// harder to see.
func hostDaemon(tree, name string, progress io.Writer) (string, error) {
	var tb textbuf.Buffer
	if override := firstSetting(evidenceBinaryEntry.Key, l2tpPPPBinaryEntry.Key); override != "" {
		info, err := os.Stat(override)
		if err != nil || info.IsDir() {
			return "", errors.New(tb.Str("ze binary override does not exist: ").Str(override).String())
		}
		if info.Mode()&0o111 == 0 {
			return "", errors.New(tb.Str("ze binary override is not executable: ").Str(override).String())
		}
		return override, nil
	}

	if err := look("go"); err != nil {
		return "", err
	}
	rel := hostDaemonRel(name)
	if err := buildHostDaemon(tree, rel, progress); err != nil {
		return "", err
	}
	return filepath.Join(tree, rel), nil
}

// hostDaemonRel answers where a host-built daemon lands, relative to the tree.
// The proof's own name is in it, so two proofs running at once do not write
// over each other's binary.
func hostDaemonRel(name string) string {
	return filepath.Join("tmp", "evidence", "bin", name)
}

// firstSetting answers the first key an operator gave a value to.
func firstSetting(keys ...string) string {
	for _, key := range keys {
		if value := env.Get(key); value != "" {
			return value
		}
	}
	return ""
}
