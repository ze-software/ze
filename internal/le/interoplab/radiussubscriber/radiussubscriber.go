// Design: docs/architecture/testing/interop.md -- the FreeRADIUS subscriber suite
// Detail: checkers.go -- the observations each scenario makes, on ze's side and on the server's
// Related: ../pppdevice.go -- the shared /dev/ppp grant, probe and refusal
//
// This suite puts a real FreeRADIUS server in front of ze's subscriber RADIUS
// surface: the accountant (authradius/acct.go) and the CoA and Disconnect
// listener (authradius/coa.go). Both act only on a kernel L2TP session, so the
// lab carries its own LAC (xl2tpd and pppd), and ze and the LAC hold NET_ADMIN
// and the host's /dev/ppp. That is why it is not a scenario of the admin-login
// suite (internal/le/interoplab/radius), which promises no capability at all,
// and why it is not the L2TP lab, whose RADIUS peer is a mock ze wrote.
package radiussubscriber

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

const (
	// Action is the native action identity served by this callable runner.
	Action = "test integration/interop-radius-subscriber"

	// serverImage is the pinned FreeRADIUS release the admin-login suite also
	// runs; its radclient is the Dynamic Authorization Client. A moving tag
	// would change what these scenarios mean with no ze change.
	serverImage = "docker.io/freeradius/freeradius-server:3.2.7"

	zeImage  = "ze-radius-subscriber-ze"
	lacImage = "ze-radius-subscriber-lac"

	peerZe     = "ze"
	peerServer = "freeradius"
	peerLAC    = "lac"

	scenarioCoADuplicate = "radius-coa-duplicate-freeradius"
	scenarioAcctRestart  = "radius-acct-restart-freeradius"

	labDirectory = "test/interop-radius-subscriber"

	// netAdminCapability is what ze's kernel L2TP and the LAC's pppd need
	// beyond Docker's default grants besides the PPP device: the L2TP Generic
	// Netlink tunnels and sessions, and PPPIOCNEWUNIT.
	netAdminCapability = "NET_ADMIN"

	zeConfigTarget = "/etc/ze/ze.conf"

	// zeHost, serverHost and lacHost are the host octets on networkPrefix.
	// The scenario ze.conf files and the lab's xl2tpd.conf and clients.conf
	// name these addresses directly, so they read as operator config.
	zeHost     = 2
	serverHost = 3
	lacHost    = 4
)

// networkPrefix is this lab's own subnet. The BGP suite holds 172.30.0.0/24,
// L2TP 172.29.0.0/24, IPsec 172.28.0.0/24 and the admin-login RADIUS suite
// 172.27.0.0/24, so a concurrent run of any of them cannot collide with this one.
var networkPrefix = netip.MustParsePrefix("172.26.0.0/24")

// scenarioCheckerMap binds every scenario directory to its typed checker.
// Discover refuses a directory this map does not name, so a fixture added
// without a checker breaks the run instead of passing silently.
func scenarioCheckerMap(timeout time.Duration) map[string]interoplab.Checker {
	return map[string]interoplab.Checker{
		scenarioCoADuplicate: checker(checkCoADuplicate, timeout),
		scenarioAcctRestart:  checker(checkAcctRestart, timeout),
	}
}

// ScenarioNames returns every scenario of this suite in lexical selection order.
func ScenarioNames() []string {
	names := make([]string, 0, len(scenarioCheckerMap(0)))
	for name := range scenarioCheckerMap(0) {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Run resolves the checkout and runs every scenario, or the one
// RADIUS_SUBSCRIBER_INTEROP_SCENARIO names.
func Run(ctx context.Context) interoplab.SuiteReport {
	root, err := lepath.Root()
	if err != nil {
		var tb textbuf.Buffer
		return interoplab.SuiteReport{SetupError: tb.Str("resolve repository root: ").Err(err).String(), Code: 1}
	}
	return RunAt(ctx, root, "")
}

// RunAt runs the selected scenarios against root. A non-empty selector names
// the one scenario to run and wins over RADIUS_SUBSCRIBER_INTEROP_SCENARIO.
func RunAt(ctx context.Context, root, selector string) interoplab.SuiteReport {
	environment := interoplab.ReadEnvironment(interoplab.EnvironmentOptions{
		SelectorVariable: selectorVariable,
		SuffixVariable:   "ZE_RADIUS_SUBSCRIBER_INTEROP_SUFFIX",
		DefaultSuffix:    strconv.Itoa(os.Getpid()),
	})
	if selector != "" {
		environment.Selector = selector
	}
	suite, err := suiteFor(root, environment, interoplab.NewDocker())
	if err != nil {
		return interoplab.SuiteReport{SetupError: err.Error(), Code: 1}
	}
	return suite.Run(ctx)
}

// suiteFor builds the complete suite without starting anything, so a test can
// assert the topology and the grants this lab declares.
func suiteFor(root string, environment interoplab.Environment, docker *interoplab.Docker) (interoplab.Suite, error) {
	sources, err := interoplab.Discover(filepath.Join(root, filepath.FromSlash(scenarioDirectory)), environment.Selector,
		scenarioCheckerMap(environment.SessionTimeout))
	if err != nil {
		return interoplab.Suite{}, err
	}
	plans := make([]interoplab.ScenarioPlan, 0, len(sources))
	for _, source := range sources {
		plans = append(plans, scenarioPlan(root, environment.Suffix, source))
	}
	lab := filepath.Join(root, labDirectory)
	return interoplab.Suite{
		SuffixVariable: environment.SuffixVariable,
		Docker:         docker,
		StagedZe:       interoplab.StagedZePath(root, LabBinaries()),
		// The /dev/ppp probe runs FIRST: a host with no PPP device cannot run
		// this lab, and refusing it before the cross-compile costs it nothing.
		// Suite.Run checks the kernel capabilities (l2tp, l2tp-ppp) itself.
		Preflight: interoplab.Preflights(preflight(),
			interoplab.StageBinaries(root, environment.NoBuild, LabBinaries()...)),
		Images: []interoplab.ImageBuild{
			{Name: peerZe, Tag: zeImage, Dockerfile: filepath.Join(lab, "Dockerfile.ze"), Context: root, Required: true},
			{Name: peerServer, Tag: serverImage, Pull: true, Required: true},
			{Name: peerLAC, Tag: lacImage, Dockerfile: filepath.Join(lab, "Dockerfile.lac"), Context: lab, Required: true},
		},
		Scenarios: plans,
		NoBuild:   environment.NoBuild,
	}, nil
}

func scenarioPlan(root, suffix string, source interoplab.ScenarioSource) interoplab.ScenarioPlan {
	var tb textbuf.Buffer
	zeContainer := tb.Str("ze-radsub-ze-").Str(suffix).String()
	serverContainer := tb.Reset().Str("ze-radsub-server-").Str(suffix).String()
	lacContainer := tb.Reset().Str("ze-radsub-lac-").Str(suffix).String()
	networkName := tb.Reset().Str("ze-radsub-").Str(suffix).String()
	return interoplab.ScenarioPlan{
		Source: source,
		Network: interoplab.NetworkSpec{
			Name:       networkName,
			Candidates: []interoplab.Subnet{{IPv4: networkPrefix}},
		},
		Containers: []string{zeContainer, serverContainer, lacContainer},
		// The server starts first so ze's first Access-Request meets a
		// listening socket, and the LAC last so it dials a listening LNS.
		Peers: []interoplab.PeerConfig{
			serverPeerConfig(root, serverContainer),
			zePeerConfig(source, zeContainer),
			lacPeerConfig(root, lacContainer),
		},
	}
}

func serverPeerConfig(root, container string) interoplab.PeerConfig {
	lab := filepath.Join(root, labDirectory)
	return interoplab.PeerConfig{
		Name:      peerServer,
		Container: container,
		Image:     peerServer,
		Host:      serverHost,
		Mounts: []interoplab.Mount{
			{Source: filepath.Join(lab, "clients.conf"), Target: "/etc/raddb/clients.conf", ReadOnly: true},
			{Source: filepath.Join(lab, "site-default"), Target: "/etc/raddb/sites-enabled/default", ReadOnly: true},
			{Source: filepath.Join(lab, "mods-ze-detail"), Target: "/etc/raddb/mods-enabled/ze_detail", ReadOnly: true},
			{Source: filepath.Join(lab, "users"), Target: "/etc/raddb/mods-config/files/authorize", ReadOnly: true},
		},
		// -X keeps every decoded request on the container log, so a failed
		// scenario names what the server saw.
		Command: []string{"radiusd", "-f", "-X"},
		Ready: &interoplab.ReadyProbe{
			// 0714 is 1812, the one port ze sends both kinds of request to.
			Command:  []string{"sh", "-c", "grep -q ':0714 ' /proc/net/udp"},
			Timeout:  90 * time.Second,
			Interval: time.Second,
		},
	}
}

func zePeerConfig(source interoplab.ScenarioSource, container string) interoplab.PeerConfig {
	return interoplab.PeerConfig{
		Name:      peerZe,
		Container: container,
		Image:     peerZe,
		Host:      zeHost,
		// Only the input file is mounted; its writable, container-owned parent
		// holds the database tree.
		Mounts:       []interoplab.Mount{{Source: filepath.Join(source.Directory, "ze.conf"), Target: zeConfigTarget, ReadOnly: true}},
		Capabilities: []string{netAdminCapability},
		Arguments:    interoplab.PPPDeviceArguments(),
		Environment: []interoplab.EnvironmentVariable{
			{Name: "ZE_LOG_LEVEL", Value: "debug"},
			{Name: "ze.l2tp.ncp.enable-ipv6cp", Value: "false"},
			{Name: "ze.l2tp.ncp.ip-timeout", Value: "15s"},
			{Name: "ze.l2tp.auth.timeout", Value: "15s"},
		},
		Command: []string{"start", zeConfigTarget},
		Ready: &interoplab.ReadyProbe{
			Command:  []string{"sh", "-c", "ss -lun | grep -q ':1701 ' && ss -lun | grep -q ':3799 '"},
			Timeout:  60 * time.Second,
			Interval: time.Second,
		},
	}
}

func lacPeerConfig(root, container string) interoplab.PeerConfig {
	lab := filepath.Join(root, labDirectory)
	return interoplab.PeerConfig{
		Name:      peerLAC,
		Container: container,
		Image:     peerLAC,
		Host:      lacHost,
		Mounts: []interoplab.Mount{
			{Source: filepath.Join(lab, "xl2tpd.conf"), Target: "/etc/xl2tpd/xl2tpd.conf", ReadOnly: true},
			{Source: filepath.Join(lab, "l2tp-secrets"), Target: "/etc/xl2tpd/l2tp-secrets", ReadOnly: true},
			{Source: filepath.Join(lab, "ppp-options"), Target: "/etc/ppp/options.l2tpd.client", ReadOnly: true},
		},
		Capabilities: []string{netAdminCapability},
		Arguments:    interoplab.PPPDeviceArguments(),
		Ready:        &interoplab.ReadyProbe{Command: []string{"sh", "-c", "kill -0 1"}, Timeout: 15 * time.Second, Interval: time.Second},
	}
}

// preflight refuses a host whose Docker containers cannot hold /dev/ppp, with
// the grants ze and the LAC hold and nothing more. The kernel modules are
// Suite.Run's shared kernel check; the character device is the one fact that
// check cannot see.
func preflight() interoplab.PreflightCheck {
	return func(ctx context.Context, docker *interoplab.Docker) error {
		result, err := docker.RunOneShot(ctx, preflightContainer())
		if err != nil {
			return interoplab.PPPPreflightRunRefusal("RADIUS subscriber", err)
		}
		return preflightRefusal(interoplab.ParsePreflight(result.Stdout))
	}
}

// preflightContainer runs the shared PPP device probe, and loads no module,
// because a module it loaded would hide the host's missing setup. RunOneShot
// names the container, so the probe passes no --name of its own: a second one
// would win, and the remove-by-name after a cancel would then miss.
func preflightContainer() interoplab.OneShotContainer {
	arguments := append([]string{"--cap-add", netAdminCapability}, interoplab.PPPDeviceArguments()...)
	return interoplab.OneShotContainer{
		Image:     "alpine:3.21",
		Arguments: arguments,
		Command:   []string{"sh", "-c", interoplab.PPPDeviceProbe},
		Timeout:   120 * time.Second,
	}
}

// preflightRefusal answers nil when the probe saw the PPP device, and the host
// command that provides it when it did not.
func preflightRefusal(checks map[string]string) error {
	device, absent := interoplab.PPPDeviceMissing(checks)
	if !absent {
		return nil
	}
	return fmt.Errorf("the host cannot run the RADIUS subscriber lab: it lacks %s.\n"+
		"Lab containers never load kernel modules, so the host must provide them.\n"+
		"Load it on the Docker host, then run the lab again:\n"+
		"  sudo modprobe ppp_generic", device)
}

// LabBinaries declares the one binary this lab stages into its Docker build
// context, which is what test/interop-radius-subscriber/Dockerfile.ze copies in.
func LabBinaries() []interoplab.LabBinary {
	return []interoplab.LabBinary{
		{Name: "ze", Base: repofeaturetags.DaemonBase, Output: filepath.Join(labDirectory, "ze-linux")},
	}
}

// checker adapts a scenario body to the protocol-neutral Checker signature,
// and appends the three peers' recent logs to a failure.
func checker(body func(context.Context, *scenarioLab) error, timeout time.Duration) interoplab.Checker {
	return func(ctx context.Context, check *interoplab.CheckContext) error {
		if check == nil || check.Lab == nil {
			return errors.New("RADIUS subscriber scenario received no lab")
		}
		lab := &scenarioLab{lab: check.Lab, timeout: timeout}
		err := body(ctx, lab)
		if err == nil {
			return nil
		}
		return lab.withDiagnostics(ctx, err)
	}
}
