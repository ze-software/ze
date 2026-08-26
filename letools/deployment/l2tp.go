// Design: docs/architecture/testing/interop.md -- ze against a real L2TP peer
// Overview: actions.go -- the area table that reaches this run
// Detail: report.go -- the payload this run answers
// Related: deployment.go -- the container and the collector
// Related: daemonbuild.go -- the daemon this proof builds
//
// l2tp.go proves that ze terminates an L2TP tunnel a peer somebody else wrote
// dials. xl2tpd is that peer: it runs in a container beside a cross-compiled
// ze, dials the daemon's listener, and the run passes when the daemon reports a
// session.
//
// The two daemons talk over the container's loopback rather than over a
// network, so the proof is about the protocol and not about the container's
// networking. The peer binds 1702 and ze binds 1701, which is why both numbers
// are stated here rather than left inside two configuration blobs.
package deployment

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The dot-notation spellings of ZE_L2TP_DOCKER_IMAGE, ZE_L2TP_DOCKER_PLATFORM
// and ZE_L2TP_DOCKER_GOARCH. env.Get matches case-insensitively and treats a
// dot and an underscore as the same character, so these keys read the variables
// the Python original read.
const (
	L2TPImageKey    = "ze.l2tp.docker.image"
	L2TPPlatformKey = "ze.l2tp.docker.platform"
	L2TPGoarchKey   = "ze.l2tp.docker.goarch"
)

// What the run uses when the operator names nothing.
//
// Alpine is pinned to a minor release because the proof installs xl2tpd and ppp
// from its package index: a floating tag changes the peer under the test, and a
// red run then says nothing about ze.
const (
	L2TPImage    = "alpine:3.20"
	L2TPPlatform = "linux/amd64"
	L2TPGoarch   = "amd64"
)

// stringSetting registers one of this package's variables and answers its entry.
//
// Private keeps every one of them out of `ze env list`. They are le's
// variables, and an operator reading an appliance has no container to point one
// at.
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
	l2tpImageEntry = stringSetting(L2TPImageKey, L2TPImage,
		"the container image the L2TP peer proof runs xl2tpd in")
	l2tpPlatformEntry = stringSetting(L2TPPlatformKey, L2TPPlatform,
		"the container platform the L2TP peer proof runs in")
	l2tpGoarchEntry = stringSetting(L2TPGoarchKey, L2TPGoarch,
		"the GOARCH the L2TP peer proof cross-compiles the daemon for")
)

// PeerName is the implementation this proof runs ze against.
const PeerName = "xl2tpd"

// The two ports the proof uses. They differ because both daemons run on one
// loopback: ze answers on the L2TP port, and the peer binds another so it can
// dial without binding the port it is dialing.
const (
	DaemonPort = 1701
	PeerPort   = 1702
)

// The docker words this file spells more than once. They are docker's own
// grammar rather than this proof's, so they are named where a reader can see
// that every use is the same word.
const (
	dockerExec   = "exec"
	dockerEnv    = "--env"
	dockerDetach = "--detach"
)

// The daemon lines that decide the verdict. They are the collector's needles,
// so a change to either one is a change to what this proof watches for.
const (
	listenerLine = "L2TP listener bound"
	sessionLine  = "session established"
)

// PeerConfig is the xl2tpd configuration. It dials ze on the loopback and
// leaves authentication off, because what is being proven is that a tunnel and
// a session form, and an authentication failure would hide that.
const PeerConfig = `[global]
port = 1702
auth file = /run/l2tp/l2tp-secrets
debug tunnel = yes
debug state = yes
debug packet = yes
debug avp = yes

[lac ze]
lns = 127.0.0.1
autodial = yes
redial = yes
redial timeout = 1
max redials = 5
require authentication = no
ppp debug = yes
pppoptfile = /run/l2tp/ppp-options
length bit = yes
`

// PeerSecrets is the xl2tpd secrets file. Its one entry matches any peer,
// because the tunnel does not authenticate.
const PeerSecrets = "* * s3cr3t\n"

// PeerPPPOptions is what xl2tpd hands pppd. It refuses EAP and takes whatever
// addresses the far end offers, so the PPP layer cannot be the thing that
// fails.
const PeerPPPOptions = `noauth
name alice
password alice
refuse-eap
nodefaultroute
ipcp-accept-local
ipcp-accept-remote
debug
nodetach
`

// DaemonConfig is what ze is started with: an L2TP server on the loopback,
// taking sessions from anybody.
const DaemonConfig = `l2tp {
    enabled true;
    auth-method none;
    allow-no-auth true;
    hello-interval 5;
    max-tunnels 4;
    max-sessions 4;
}
environment {
    l2tp {
        server main {
            ip 127.0.0.1;
            port 1701;
        }
    }
}
`

// The two bounds on the steps that lead up to the proof. Each is generous
// enough for a cold machine and short enough that a hung step is reported
// rather than waited on: a container start, and an Alpine package install over
// the network. The cross-compile is bounded by daemonbuild.go, which every
// proof shares.
const (
	dockerTimeout  = 2 * time.Minute
	installTimeout = 5 * time.Minute
)

// The two waits the run is bounded by. Each is a real bound: a listener that
// has not bound in twenty seconds is not going to, and a tunnel that has not
// formed twenty seconds after the peer started has failed rather than stalled.
const (
	L2TPListenerWait = 20 * time.Second
	L2TPSessionWait  = 20 * time.Second
)

// L2TP is one run of the L2TP peer proof.
//
// Progress is where the two daemons' output goes while the run happens. It is
// stderr for an operator, because the answer is the report and a pipe operator
// must be able to carry it.
type L2TP struct {
	// Tree is the checkout: the daemon is built from it, and it is mounted
	// into the container.
	Tree string
	// Image, Platform and Goarch say what the peer runs in and what the daemon
	// is built for.
	Image    string
	Platform string
	Goarch   string
	// ListenerWait bounds the wait for ze's listener; SessionWait bounds the
	// wait for the peer's session.
	ListenerWait time.Duration
	SessionWait  time.Duration
	// Progress receives each daemon's output line as it arrives. It MUST be
	// safe for concurrent use: two daemons write to it at once, each from its
	// own goroutine. os.Stderr and io.Discard both are.
	Progress io.Writer
}

// NewL2TP answers the run the command performs over tree, with every setting
// taken from the environment or from its default.
func NewL2TP(tree string) *L2TP {
	return &L2TP{
		Tree:         tree,
		Image:        setting(l2tpImageEntry.Key, L2TPImage),
		Platform:     setting(l2tpPlatformEntry.Key, L2TPPlatform),
		Goarch:       setting(l2tpGoarchEntry.Key, L2TPGoarch),
		ListenerWait: L2TPListenerWait,
		SessionWait:  L2TPSessionWait,
		Progress:     os.Stderr,
	}
}

// setting answers what the operator named for key, or fallback when they named
// nothing. env.Get answers the empty string for an unset variable rather than
// the registered default, so the default is applied at the one place that reads
// it.
func setting(key, fallback string) string {
	if named := env.Get(key); named != "" {
		return named
	}
	return fallback
}

// containerArgs answers the container the peer and the daemon both run in.
//
// It is privileged because xl2tpd opens a PPP device, and it holds the checkout
// at /src and the run's scratch directory at /run/l2tp, which is where both
// daemons read their configuration from.
func (l *L2TP) containerArgs(name, work string) []string {
	var tb textbuf.Buffer
	src := tb.Str(l.Tree).Str(":/src").String()
	tb.Reset()
	scratch := tb.Str(work).Str(":/run/l2tp").String()

	return []string{
		"run", "--rm", dockerDetach, "--privileged",
		"--platform", l.Platform,
		"--name", name,
		"-v", src,
		"-v", scratch,
		"-w", "/src",
		l.Image,
		"sleep", "infinity",
	}
}

// installArgs answers the peer's installation. Alpine's package index is read
// at run time rather than baked into an image, so the proof needs no registry
// of its own.
func (l *L2TP) installArgs(name string) []string {
	return []string{dockerExec, name, "apk", "add", "--no-cache", PeerName, "ppp"}
}

// daemonArgs answers ze, started inside the container reading its configuration
// from standard input.
//
// The kernel probe is skipped because the container's kernel is the host's and
// the proof is about the control plane; blob storage is off and the config
// directory is the scratch mount, so the run leaves nothing behind in the
// checkout.
func (l *L2TP) daemonArgs(name, binaryRel string) []string {
	var tb textbuf.Buffer
	binary := tb.Str("/src/").Str(filepath.ToSlash(binaryRel)).String()

	return []string{
		dockerExec, "--interactive",
		dockerEnv, "ZE_LOG_L2TP=debug",
		dockerEnv, "ze.l2tp.skip-kernel-probe=true",
		dockerEnv, "ZE_STORAGE_BLOB=false",
		dockerEnv, "ZE_CONFIG_DIR=/run/l2tp/ze",
		name,
		binary,
		"-",
	}
}

// peerArgs answers xl2tpd, run in the foreground so its output reaches this
// process rather than a log file inside the container.
func (l *L2TP) peerArgs(name string) []string {
	return []string{
		dockerExec, name, PeerName,
		"-D",
		"-c", "/run/l2tp/xl2tpd.conf",
		"-s", "/run/l2tp/l2tp-secrets",
		"-p", "/run/l2tp/xl2tpd.pid",
		"-C", "/run/l2tp/l2tp-control",
	}
}

// writeScratch lays out the directory both daemons read from: the peer's three
// files, and the empty directory ze writes its configuration store into.
func (l *L2TP) writeScratch(work string) error {
	if err := os.MkdirAll(filepath.Join(work, "ze"), 0o750); err != nil {
		return err
	}
	files := map[string]string{
		PeerConfigFile:  PeerConfig,
		PeerSecretsFile: PeerSecrets,
		PeerOptionsFile: PeerPPPOptions,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(work, name), []byte(body), 0o644); err != nil { //nolint:gosec // a scratch file two daemons in a container read
			return err
		}
	}
	return nil
}

// Run performs the proof and answers what happened.
//
// A step that could not be performed is an error: the operator has something to
// fix, and no verdict was reached. A session that did not establish is NOT an
// error. It is the verdict, and it travels in the report with the daemon's last
// lines behind it.
func (l *L2TP) Run() (L2TPReport, error) {
	report := L2TPReport{Peer: PeerName, Image: l.Image}

	if err := look("docker", "go"); err != nil {
		return report, err
	}
	if err := ensureImage(l.Image, l.Progress); err != nil {
		return report, err
	}
	if err := buildDaemon(l.Tree, l.Goarch, l.Progress); err != nil {
		return report, err
	}

	work, err := scratchDir(l.Tree, "l2tp-peer-")
	if err != nil {
		return report, err
	}
	if err := l.writeScratch(work); err != nil {
		return report, err
	}

	report.Container = containerName()
	if err := l.startContainer(report.Container, work); err != nil {
		return report, err
	}
	defer removeContainer(report.Container)

	if err := l.installPeer(report.Container); err != nil {
		return report, err
	}

	return l.observe(report)
}

// observe starts both daemons, waits for the session, and answers the verdict.
//
// It is separate from Run so one function owns the two process lifecycles and
// their stop paths, rather than adding two more deferred stops to a function
// that already owns the container's.
func (l *L2TP) observe(report L2TPReport) (L2TPReport, error) {
	seen := newCollector(listenerLine, sessionLine)

	// Background rather than a deadline: this daemon is stopped by the run's
	// own stop path once a verdict is reached, and the two waits below are
	// where the run is bounded in time.
	daemon := exec.CommandContext(context.Background(), "docker", l.daemonArgs(report.Container, daemonRel(l.Goarch))...) //nolint:gosec // the argv is built above, never by an operator
	daemon.Stdin = strings.NewReader(DaemonConfig)

	ze, err := startWatched(daemon, "ze> ", seen, l.Progress)
	if err != nil {
		return report, err
	}
	defer seen.wait()
	defer ze.stop()

	if !await(seen, listenerLine, ze, l.ListenerWait) {
		report.LogTail = seen.tailLines()
		return report, errors.New("the ze L2TP listener did not start")
	}

	peer := exec.CommandContext(context.Background(), "docker", l.peerArgs(report.Container)...) //nolint:gosec // the argv is built above, never by an operator

	// The peer's own collector watches for nothing: what it says never decides
	// the verdict, and the daemon is the side that reports a session. It exists
	// so the peer's output reaches the operator and so its pipe is drained.
	said := newCollector()
	dialer, err := startWatched(peer, "xl2tpd> ", said, l.Progress)
	if err != nil {
		return report, err
	}
	defer said.wait()
	defer dialer.stop()

	report.Established = await(seen, sessionLine, ze, l.SessionWait)
	if !report.Established {
		report.LogTail = seen.tailLines()
	}
	return report, nil
}

// startContainer starts the container the proof runs in.
func (l *L2TP) startContainer(name, work string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dockerTimeout)
	defer cancel()

	start := exec.CommandContext(ctx, "docker", l.containerArgs(name, work)...) //nolint:gosec // the argv is built above, never by an operator
	start.Stderr = l.Progress
	if err := start.Run(); err != nil {
		return errors.New("failed to start the L2TP evidence container")
	}
	return nil
}

// installPeer installs xl2tpd and pppd in the running container.
func (l *L2TP) installPeer(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), installTimeout)
	defer cancel()

	install := exec.CommandContext(ctx, "docker", l.installArgs(name)...) //nolint:gosec // the argv is built above, never by an operator
	install.Stdout = l.Progress
	install.Stderr = l.Progress
	if err := install.Run(); err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str("apk add ").Str(PeerName).Str(" ppp failed").String())
	}
	return nil
}

// containerName answers a name no other run of this tool on this machine will
// pick, so two developers, or one developer twice, do not collide on it.
func containerName() string {
	var tb textbuf.Buffer
	return tb.Str("ze-l2tp-evidence-").Int(int64(os.Getpid())).String()
}
