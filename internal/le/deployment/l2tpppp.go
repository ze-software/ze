// Design: docs/architecture/testing/interop.md -- ze against a real L2TP access concentrator
// Overview: actions.go -- the area table that reaches this run
// Detail: l2tppppreport.go -- the payload this run answers
// Related: netns.go -- the two namespaces the two daemons run in
// Related: pppstate.go -- the kernel state this run asserts about
// Related: hostkernel.go -- what the machine must provide before any of it runs
// Related: gokrazyl2tp.go -- the same proof against the appliance image
// Detail: hostkernel.go -- what the machine must provide before any of it runs
// Detail: l2tppppinputs.go -- the files both daemons read
// Detail: pppstate.go -- the kernel state this run asserts about
//
// l2tpppp.go proves the WHOLE L2TP path on one Linux host. A real xl2tpd dials
// ze. A real pppd negotiates LCP and IPCP over the tunnel. The kernel creates a
// pppN interface at each end.
//
// Traffic crosses those interfaces. ze injects the subscriber route. Every
// object disappears when the peer leaves.
//
// This is the proof that the container one (l2tp.go) cannot provide. That proof
// stops at the control session. A container shares the host's kernel. It cannot
// own a PPP interface unless it takes one from the machine. Here, each side gets
// a network namespace. Both daemons can bind the addresses they want, and the
// kernel objects belong to the run.
//
// Nothing about the kernel is stubbed or skipped. ze's own kernel-probe escape
// is REFUSED rather than honored. A run that took it would report a pass for a
// user-space path. This proof must never give that answer.

package deployment

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The dot-notation spellings of the ZE_L2TP_PPP_* variables. env.Get matches
// case-insensitively and treats a dot and an underscore as the same character,
// so these keys read the variables the Python original read.
const (
	L2TPPPPPrefixKey     = "ze.l2tp.ppp.underlay.prefix"
	L2TPPPPZeIPKey       = "ze.l2tp.ppp.ze.underlay.ip"
	L2TPPPPLACIPKey      = "ze.l2tp.ppp.lac.underlay.ip"
	L2TPPPPListenIPKey   = "ze.l2tp.ppp.listen.ip"
	L2TPPPPListenPortKey = "ze.l2tp.ppp.listen.port"
	L2TPPPPPeerPortKey   = "ze.l2tp.ppp.xl2tpd.port"
)

// What the run uses when the operator names nothing.
//
// The underlay is a private /24 nobody routes, carried by a veth pair inside
// two namespaces, so it cannot collide with anything the machine is doing. The
// two L2TP ports differ because each daemon binds its own: ze answers on the
// registered port and the peer dials from another.
const (
	L2TPPPPPrefix     = "24"
	L2TPPPPZeIP       = "172.30.0.1"
	L2TPPPPLACIP      = "172.30.0.2"
	L2TPPPPListenPort = "1701"
	L2TPPPPPeerPort   = "1702"
)

// The pool hands out these addresses. The gateway is ze's end of the PPP link.
// The start of the range is the peer's end. Thus, a reader of the report can
// identify the side that owns each interface.
const (
	L2TPPPPLocalAddr = "10.100.0.1"
	L2TPPPPPeerAddr  = "10.100.0.2"
	L2TPPPPPoolEnd   = "10.100.0.10"
)

var (
	l2tpPPPPrefixEntry = stringSetting(L2TPPPPPrefixKey, L2TPPPPPrefix,
		"the prefix length of the underlay the on-host L2TP PPP proof builds")
	l2tpPPPZeIPEntry = stringSetting(L2TPPPPZeIPKey, L2TPPPPZeIP,
		"the underlay address ze holds in the on-host L2TP PPP proof")
	l2tpPPPLACIPEntry = stringSetting(L2TPPPPLACIPKey, L2TPPPPLACIP,
		"the underlay address the xl2tpd peer holds in the on-host L2TP PPP proof")
	l2tpPPPListenIPEntry = stringSetting(L2TPPPPListenIPKey, "",
		"the address ze binds its L2TP listener to; the ze underlay address by default")
	l2tpPPPListenPortEntry = stringSetting(L2TPPPPListenPortKey, L2TPPPPListenPort,
		"the port ze binds its L2TP listener to in the on-host L2TP PPP proof")
	l2tpPPPPeerPortEntry = stringSetting(L2TPPPPPeerPortKey, L2TPPPPPeerPort,
		"the port the xl2tpd peer binds in the on-host L2TP PPP proof")
)

// proofName is what this proof is called in the sentences it refuses with. It
// is the Python original's wording, so an operator meeting the refusal on
// either half reads the same line.
const proofName = "full L2TP PPP/NCP evidence"

// The run waits for these daemon lines. Each line identifies one step of the
// path. The listener binds, the address pool loads, and the peer's tunnel
// arrives. The address is handed out, the route is injected, and the PPP link
// comes up. Finally, the route is withdrawn.
const (
	pppListenerLine = "L2TP listener bound"
	pppPoolLine     = "l2tp-pool: configured"
	pppSessionLine  = "l2tp: session established (incoming LNS)"
	pppIPLine       = "l2tp: session IP assigned"
	pppRouteLine    = "l2tp: subscriber route inject"
	pppUpLine       = "l2tp: PPP session up"
	// gosec reads this line's entropy as a credential. It is a log needle: the
	// sentence ze prints when it takes a subscriber's routes back out.
	pppWithdrawLine = "l2tp: subscriber routes withdrawn" //nolint:gosec // G101: a log needle, not a secret
)

// pppFatalLines are the daemon's own reports that its current path cannot carry
// the proof. Each line identifies a step that refused. The run watches for each
// line throughout. Without this watch, the run waits out the 45-second bound
// after a daemon has already said "genl family resolve failed". It then reports
// a timeout instead of the reason.
var pppFatalLines = []string{
	"skipping kernel module probe",
	"genl family resolve failed",
	"kernel integration disabled",
	"kernel session ready but no PPP driver wired",
	"ipcp: handler rejected",
	"ncp: timeout",
	"ip-response timeout",
}

// pppTeardownLine is fatal BEFORE the session is proven and expected after it.
// If the peer asks for teardown while the run waits for LCP, the negotiation
// has failed. During the teardown phase, the same line confirms that the run
// got the teardown it requested.
const pppTeardownLine = "PPP requested session teardown"

// These waits bound the run. Each wait is a real bound, not a guess. A listener
// that has not bound in twenty seconds has failed to bind. A plugin that has
// not loaded in thirty has failed to load. A tunnel has failed if it has not
// formed thirty seconds after the peer started.
//
// On a working path, LCP, IPCP, and route injection together take under a
// second. Thus, 45 is a wide margin. Route withdrawal follows the peer's own
// exit. The kernel removes its objects when the last descriptor closes.
const (
	L2TPPPPListenerWait = 20 * time.Second
	L2TPPPPPoolWait     = 30 * time.Second
	L2TPPPPSessionWait  = 30 * time.Second
	L2TPPPPNCPWait      = 45 * time.Second
	L2TPPPPWithdrawWait = 15 * time.Second
	L2TPPPPCleanupWait  = 30 * time.Second
)

// daemonBinaryName is what the daemon this proof builds is called under the
// tree's scratch directory.
const daemonBinaryName = "ze-l2tp-ppp"

// L2TPPPP is one run of the on-host L2TP PPP proof.
type L2TPPPP struct {
	// Tree is the checkout the daemon is built from.
	Tree string
	// The underlay the two namespaces are joined by, and the two L2TP ports.
	Prefix     string
	ZeIP       string
	LACIP      string
	ListenIP   string
	ListenPort string
	PeerPort   string
	// The namespaces and the veth pair this run owns. Each carries the process
	// id, so two runs on one machine do not collide.
	ZeNamespace  string
	LACNamespace string
	ZeVeth       string
	LACVeth      string
	// The bounds on each step of the run.
	ListenerWait time.Duration
	PoolWait     time.Duration
	SessionWait  time.Duration
	NCPWait      time.Duration
	WithdrawWait time.Duration
	CleanupWait  time.Duration
	// Progress receives each daemon's output line as it arrives, and every
	// diagnostic a failure produces. It MUST be safe for concurrent use: two
	// daemons write to it at once, each from its own goroutine.
	Progress io.Writer
}

// NewL2TPPPP answers the run the command performs over tree, with every setting
// taken from the environment or from its default.
func NewL2TPPPP(tree string) *L2TPPPP {
	suffix := namespaceSuffix()
	short := linkSuffix(suffix)
	zeIP := setting(l2tpPPPZeIPEntry.Key, L2TPPPPZeIP)

	var tb textbuf.Buffer
	run := &L2TPPPP{
		Tree:       tree,
		Prefix:     setting(l2tpPPPPrefixEntry.Key, L2TPPPPPrefix),
		ZeIP:       zeIP,
		LACIP:      setting(l2tpPPPLACIPEntry.Key, L2TPPPPLACIP),
		ListenIP:   setting(l2tpPPPListenIPEntry.Key, zeIP),
		ListenPort: setting(l2tpPPPListenPortEntry.Key, L2TPPPPListenPort),
		PeerPort:   setting(l2tpPPPPeerPortEntry.Key, L2TPPPPPeerPort),

		ZeNamespace:  tb.Str("ze-l2tp-ppp-ze-").Str(suffix).String(),
		LACNamespace: tb.Reset().Str("ze-l2tp-ppp-lac-").Str(suffix).String(),
		ZeVeth:       tb.Reset().Str("zpppz").Str(short).String(),
		LACVeth:      tb.Reset().Str("zpppl").Str(short).String(),

		ListenerWait: L2TPPPPListenerWait,
		PoolWait:     L2TPPPPPoolWait,
		SessionWait:  L2TPPPPSessionWait,
		NCPWait:      L2TPPPPNCPWait,
		WithdrawWait: L2TPPPPWithdrawWait,
		CleanupWait:  L2TPPPPCleanupWait,
		Progress:     os.Stderr,
	}
	return run
}

// Run performs the proof and answers what happened.
//
// Failure to perform a step is an error. The operator has something to fix, and
// the run reached no verdict. Failure of an L2TP path step to occur is NOT an
// error. It is the verdict. The report contains the verdict, the reason, and
// the daemon's last lines.
func (l *L2TPPPP) Run() (L2TPPPPReport, error) {
	report := L2TPPPPReport{
		Peer:         PeerName,
		ZeNamespace:  l.ZeNamespace,
		LACNamespace: l.LACNamespace,
		LocalAddress: L2TPPPPLocalAddr,
		PeerAddress:  L2TPPPPPeerAddr,
	}

	if err := refuseSkipKernelProbe(); err != nil {
		return report, err
	}
	if err := requireLinux(proofName); err != nil {
		return report, err
	}
	if err := look("ip", "ping", PeerName, "pppd"); err != nil {
		return report, err
	}
	if err := ensureKernelSupport(proofName); err != nil {
		return report, err
	}

	binary, err := hostDaemon(l.Tree, daemonBinaryName, l.Progress)
	if err != nil {
		return report, err
	}

	work, err := scratchDir(l.Tree, "effective-l2tp-ppp-")
	if err != nil {
		return report, err
	}
	if err := l.writeInputs(work); err != nil {
		return report, err
	}

	defer l.removeNamespaces()
	if err := l.setupNamespaces(); err != nil {
		return report, err
	}

	report, err = l.observe(report, binary, work)
	if err != nil {
		return report, err
	}
	if report.Proven {
		os.RemoveAll(work) //nolint:errcheck // the run passed; a scratch directory left behind is not a verdict
	} else {
		l.diagnose(work)
	}
	return report, nil
}

// setupNamespaces builds the two namespaces. It joins them with a veth pair and
// assigns addresses to both ends. Before L2TP starts, it proves that the
// underlay carries traffic.
//
// The underlay ping is not decoration. A proof that skipped it would report an
// L2TP failure when the veth pair was never up. That report would direct the
// reader to the wrong daemon.
func (l *L2TPPPP) setupNamespaces() error {
	l.removeNamespaces()
	if err := ensureNetnsDir(); err != nil {
		return err
	}

	var tb textbuf.Buffer
	steps := []struct {
		what string
		argv []string
	}{
		{tb.Str("create netns ").Str(l.ZeNamespace).String(), []string{"ip", ipNetns, ipAdd, l.ZeNamespace}},
		{tb.Reset().Str("create netns ").Str(l.LACNamespace).String(), []string{"ip", ipNetns, ipAdd, l.LACNamespace}},
		{"create L2TP underlay veth pair", []string{"ip", ipLink, ipAdd, l.ZeVeth, ipType, "veth", "peer", ipName, l.LACVeth}},
		{"move Ze veth", []string{"ip", ipLink, ipSet, l.ZeVeth, ipNetns, l.ZeNamespace}},
		{"move LAC veth", []string{"ip", ipLink, ipSet, l.LACVeth, ipNetns, l.LACNamespace}},
	}
	for _, step := range steps {
		if err := hostRequired(l.Progress, step.what, step.argv...); err != nil {
			return err
		}
	}

	zeCIDR := tb.Reset().Str(l.ZeIP).Byte('/').Str(l.Prefix).String()
	lacCIDR := tb.Reset().Str(l.LACIP).Byte('/').Str(l.Prefix).String()
	inside := []struct {
		ns   string
		what string
		argv []string
	}{
		{l.ZeNamespace, "bring up Ze loopback", []string{"ip", ipLink, ipSet, "lo", "up"}},
		{l.LACNamespace, "bring up LAC loopback", []string{"ip", ipLink, ipSet, "lo", "up"}},
		{l.ZeNamespace, "assign Ze underlay address", []string{"ip", ipAddr, ipAdd, zeCIDR, ipDev, l.ZeVeth}},
		{l.LACNamespace, "assign LAC underlay address", []string{"ip", ipAddr, ipAdd, lacCIDR, ipDev, l.LACVeth}},
		{l.ZeNamespace, "bring up Ze veth", []string{"ip", ipLink, ipSet, l.ZeVeth, "up"}},
		{l.LACNamespace, "bring up LAC veth", []string{"ip", ipLink, ipSet, l.LACVeth, "up"}},
	}
	for _, step := range inside {
		if err := nsRequired(l.Progress, step.ns, step.what, step.argv...); err != nil {
			return err
		}
	}

	if out, ok := nsText(l.LACNamespace, "ping", "-c", "1", "-W", "2", l.ZeIP); !ok {
		writeProgress(l.Progress, out)
		return errors.New("LAC namespace cannot reach Ze namespace underlay")
	}

	for _, ns := range []string{l.ZeNamespace, l.LACNamespace} {
		out, ok := nsText(ns, "ip", "l2tp", ipShow, tunnelObjectName)
		if ok {
			continue
		}
		writeProgress(l.Progress, out)
		return errors.New(tb.Reset().Str("ip l2tp unavailable in namespace ").Str(ns).String())
	}
	return nil
}

// removeNamespaces takes down both namespaces and the veth pair. It runs before
// setup and after the run. A previous run that was killed leaves its namespaces
// behind with names that this run derives in the same way.
func (l *L2TPPPP) removeNamespaces() {
	removeNamespaces([]string{l.ZeNamespace, l.LACNamespace}, []string{l.ZeVeth, l.LACVeth})
}

// observe starts both daemons, walks the L2TP path, and answers the verdict.
//
// It is separate from Run so that one function owns both process lifecycles and
// their stop paths. The peer stops before ze. The proof asserts that the PEER'S
// departure causes the teardown. If ze stops first, the session ends from the
// wrong endpoint. That sequence proves nothing about the withdraw path.
func (l *L2TPPPP) observe(report L2TPPPPReport, binary, work string) (L2TPPPPReport, error) {
	baselines, err := readPPPBaselines([]string{l.ZeNamespace, l.LACNamespace})
	if err != nil {
		return report, err
	}
	zeBase, lacBase := baselines[0], baselines[1]

	seen := newCollector(append([]string{
		pppListenerLine, pppPoolLine, pppSessionLine, pppIPLine,
		pppRouteLine, pppUpLine, pppWithdrawLine, pppTeardownLine,
	}, pppFatalLines...)...)

	daemon := nsCommand(l.ZeNamespace, binary, "start", filepath.Join(work, "ze.conf"))
	daemon.Env = l.daemonEnv(work)

	ze, err := startWatched(daemon, "ze> ", seen, l.Progress)
	if err != nil {
		return report, err
	}
	defer seen.wait()
	defer ze.stop()

	preSession := append(slices.Clone(pppFatalLines), pppTeardownLine)
	if verdict, ok := l.step(seen, report, ze, []string{pppListenerLine}, preSession,
		l.ListenerWait, "ze L2TP listener did not start"); !ok {
		return verdict, nil
	}
	if verdict, ok := l.step(seen, report, ze, []string{pppPoolLine}, nil,
		l.PoolWait, "ze l2tp-pool plugin did not load"); !ok {
		return verdict, nil
	}

	peer := nsCommand(l.LACNamespace, PeerName,
		"-D",
		"-c", filepath.Join(work, PeerConfigFile),
		"-s", filepath.Join(work, PeerSecretsFile),
		"-p", filepath.Join(work, "xl2tpd.pid"),
		"-C", filepath.Join(work, "l2tp-control"))

	said := newCollector()
	dialer, err := startWatched(peer, "xl2tpd> ", said, l.Progress)
	if err != nil {
		return report, err
	}
	defer said.wait()
	defer dialer.stop()

	if verdict, ok := l.step(seen, report, ze, []string{pppSessionLine}, preSession,
		l.SessionWait, "xl2tpd did not establish an incoming L2TP session"); !ok {
		return verdict, nil
	}
	if verdict, ok := l.step(seen, report, ze, []string{pppIPLine, pppRouteLine, pppUpLine}, pppFatalLines,
		l.NCPWait, "PPP LCP/IPCP completion and subscriber route injection were not observed"); !ok {
		return verdict, nil
	}

	report, proven := l.assertKernelState(report, seen, zeBase, lacBase)
	if !proven {
		return report, nil
	}

	// The peer leaves FIRST. The teardown assertion checks the effects of its
	// departure. If the run stopped ze instead, the session would end from the
	// wrong endpoint. A daemon that never withdraws a route would still pass.
	dialer.stop()
	said.wait()
	if verdict, ok := l.step(seen, report, ze, []string{pppWithdrawLine}, nil,
		l.WithdrawWait, "subscriber route withdraw was not observed during teardown"); !ok {
		return verdict, nil
	}

	zeBase.iface = report.ZeInterface
	lacBase.iface = report.LACInterface
	if err := awaitTeardown([]pppBaseline{zeBase, lacBase}, l.CleanupWait); err != nil {
		return l.fail(report, seen, err.Error()), nil
	}

	report.Proven = true
	return report, nil
}

// step waits for one set of daemon lines and turns a miss into the verdict.
//
// The caller supplies the failure sentence because it names the missing L2TP
// step. An operator reads that fact first. A fatal line instead uses the
// daemon's own sentence. Repeating the step name would hide the reason.
func (l *L2TPPPP) step(seen *collector, report L2TPPPPReport, ze *running,
	wanted, fatal []string, wait time.Duration, missed string,
) (L2TPPPPReport, bool) {
	arrived, err := awaitAll(seen, wanted, fatal, ze, wait)
	if err != nil {
		return l.fail(report, seen, err.Error()), false
	}
	if !arrived {
		return l.fail(report, seen, missed), false
	}
	return report, true
}

// fail answers the report for a proof that did not complete, with the reason
// and the daemon's last lines in it.
func (l *L2TPPPP) fail(report L2TPPPPReport, seen *collector, reason string) L2TPPPPReport {
	report.Proven = false
	report.Reason = reason
	report.LogTail = seen.tailLines()
	return report
}

// assertKernelState asks the kernel what the daemons' log lines claim.
//
// The assertion asks four separate questions because four failures are
// possible. It checks the pool address, each interface, both address pairs, and
// packet delivery. A daemon that logs every line and programs nothing passes
// none of these checks.
func (l *L2TPPPP) assertKernelState(report L2TPPPPReport, seen *collector, zeBase, lacBase pppBaseline) (L2TPPPPReport, bool) {
	assigned := seen.carrying(pppIPLine)
	var tb textbuf.Buffer
	wantAddress := tb.Str("address=").Str(L2TPPPPPeerAddr).String()
	if !anyLineCarrying(assigned, wantAddress) {
		return l.fail(report, seen, tb.Reset().Str("session IP assigned log missing expected address=").
			Str(L2TPPPPPeerAddr).String()), false
	}

	// The daemon's session-up lines name ze's interface. The assertion below is
	// NOT a restatement of those lines. When no log line names the interface,
	// discoverPPPIface uses the interface that appeared in the namespace. An
	// interface found that way has no session-up line behind it.
	upLines := seen.carrying(pppUpLine)
	zeIface, err := discoverPPPIface(l.ZeNamespace, zeBase.links, upLines, "Ze")
	if err != nil {
		return l.fail(report, seen, err.Error()), false
	}
	report.ZeInterface = zeIface

	lacIface, err := discoverPPPIface(l.LACNamespace, lacBase.links, nil, "LAC")
	if err != nil {
		return l.fail(report, seen, err.Error()), false
	}
	report.LACInterface = lacIface

	if !anyLineCarrying(upLines, tb.Reset().Str("interface=").Str(zeIface).String()) {
		return l.fail(report, seen, tb.Reset().Str("PPP session up log missing expected interface=").
			Str(zeIface).String()), false
	}

	if err := verifyPPPAddress(l.ZeNamespace, zeIface, L2TPPPPLocalAddr, L2TPPPPPeerAddr); err != nil {
		return l.fail(report, seen, err.Error()), false
	}
	if err := verifyPPPAddress(l.LACNamespace, lacIface, L2TPPPPPeerAddr, L2TPPPPLocalAddr); err != nil {
		return l.fail(report, seen, err.Error()), false
	}

	if _, ok := nsText(l.LACNamespace, "ping", "-c", "2", "-W", "3", L2TPPPPLocalAddr); !ok {
		return l.fail(report, seen, tb.Reset().Str("dataplane ping to ").Str(L2TPPPPLocalAddr).
			Str(" (LNS) through PPP tunnel failed").String()), false
	}

	return report, true
}

// anyLineCarrying reports whether any line carries needle.
func anyLineCarrying(lines []string, needle string) bool {
	for _, line := range lines {
		if strings.Contains(line, needle) {
			return true
		}
	}
	return false
}

// daemonEnv answers the environment ze is started with.
//
// Both spellings of the kernel-probe escape are REMOVED instead of left unset.
// This process can inherit either value from the operator's shell, and the
// daemon would then inherit it. hostkernel.go refuses the run, and this function
// removes the values. Together, these checks guarantee that the probe runs.
func (l *L2TPPPP) daemonEnv(work string) []string {
	kept := make([]string, 0, len(os.Environ())+6)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key == SkipKernelProbeEnv || key == SkipKernelProbeKey {
			continue
		}
		kept = append(kept, entry)
	}

	var tb textbuf.Buffer
	return append(kept,
		"ZE_LOG_L2TP=debug",
		storageBlobDisabledEnv,
		tb.Str("ZE_CONFIG_DIR=").Str(filepath.Join(work, "ze")).String(),
		"ze.l2tp.ncp.enable-ipv6cp=false",
		"ze.l2tp.ncp.ip-timeout=15s",
		"ze.l2tp.auth.timeout=15s",
	)
}

// diagnose writes what each namespace held when the proof failed.
//
// This evidence distinguishes a daemon that never bound from a kernel that
// never created a session. The evidence goes to the progress stream instead of
// the report. The diagnostics contain several listings. A pipe operator must be
// able to render the report.
func (l *L2TPPPP) diagnose(work string) {
	writeProgress(l.Progress, "\n--- diagnostics ---")

	queries := [][]string{
		{"ip", "l2tp", ipShow, tunnelObjectName},
		{"ip", "l2tp", ipShow, "session"},
		{"ip", ipLink, ipShow, ipType, pppPrefix},
	}
	var tb textbuf.Buffer
	for _, ns := range []string{l.ZeNamespace, l.LACNamespace} {
		for _, query := range queries {
			out, ok := nsText(ns, query...)
			if !ok || out == "" {
				continue
			}
			writeProgress(l.Progress, tb.Reset().Str(ns).Byte(' ').Str(strings.Join(query, " ")).
				Str(":\n").Str(out).String())
		}
	}

	if body, err := os.ReadFile(filepath.Join(work, "pppd.log")); err == nil { //nolint:gosec // a path this run wrote
		writeProgress(l.Progress, tb.Reset().Str("\npppd.log:\n").Str(string(body)).String())
	} else {
		writeProgress(l.Progress, "\npppd.log: not found")
	}

	for _, ns := range []string{l.ZeNamespace, l.LACNamespace} {
		out, ok := nsText(ns, "cat", "/proc/net/pppol2tp")
		if !ok || out == "" {
			continue
		}
		writeProgress(l.Progress, tb.Reset().Str("\n").Str(ns).Str(" /proc/net/pppol2tp:\n").Str(out).String())
	}
}
