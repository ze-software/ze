// Design: docs/architecture/testing/interop.md -- native FreeRADIUS scenario selection, images, topology, and verdicts.
// Detail: checkers.go -- every typed observation this suite makes, on ze's side and on the server's.
// Related: docs/guide/radius.md -- the admin config surface these scenarios drive.
//
// This suite exists because every other RADIUS proof ze holds runs against a
// mock ze wrote. A mock built beside ze's encoder agrees with ze by
// construction, and ze now computes a CHAP digest a server must reproduce from
// its own stored password. Only a server ze did not write can disagree.
//
// It is its own suite rather than four more L2TP scenarios. The L2TP lab probes
// for the l2tp_ppp or pppol2tp kernel module and refuses to run without it
// (internal/le/interoplab/l2tp/l2tp.go, preflight), which is correct for a suite
// that carries PPP sessions. Admin login is ze's SSH listener, a UDP socket and
// a RADIUS server, so it declares no preflight and no kernel dependency at all.
package radius

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/featuretags"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/lepath"
)

const (
	// Action is the native action identity served by this callable runner.
	Action = "integration/interop-radius"

	// serverImage is pinned to an exact tag. A moving tag changes what these
	// scenarios mean with no ze change, and the failure then looks like a ze
	// regression (R-3). quay.io/frrouting/frr:10.3.1 is pinned for the same
	// reason in the BGP and L2TP suites.
	serverImage = "docker.io/freeradius/freeradius-server:3.2.7"
	zeImage     = "ze-radius-interop"

	zePeer     = "ze"
	serverPeer = "freeradius"

	scenarioPAP        = "radius-admin-pap-freeradius"
	scenarioCHAP       = "radius-admin-chap-freeradius"
	scenarioCHAPHashed = "radius-admin-chap-hashed-freeradius"

	// nasIdentifier is the container hostname, and ze sends it as
	// NAS-Identifier (RFC 2865 Section 4.1) because radiusBackend.Build reads
	// os.Hostname(). The server records it, so a checker can tell a request ze
	// sent from a request anything else on the lab network sent.
	nasIdentifier = "ze-interop-nas"

	// labRoot is the directory holding the lab-wide FreeRADIUS configuration
	// every scenario mounts, plus the ze image definition.
	labDirectory = "test/interop-radius"

	// serverLogPath is where the lab's linelog module writes one line per
	// answered request. checkers.go reads it as the server's own record.
	serverLogPath = "/var/log/freeradius/ze-request.log"

	zeConfigTarget = "/etc/ze/ze.conf"
	sshPort        = "2222"
)

// networkPrefix is this lab's own subnet. The BGP suite holds 172.30.0.0/24,
// L2TP 172.29.0.0/24 and IPsec 172.28.0.0/24, so a concurrent run of any of
// them cannot collide with this one. The scenario ze.conf files name
// 172.27.0.3 directly, which is what makes them readable as operator config.
var networkPrefix = netip.MustParsePrefix("172.27.0.0/24")

// Report is the structured result returned by the callable RADIUS interop gate.
type Report struct {
	interoplab.SuiteReport
}

// Text renders the native scenario and summary presentation.
func (r Report) Text() string {
	var out textbuf.Buffer
	out.SetColor(slogutil.UseColor(os.Stdout))
	color := textbuf.C
	out.Str("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	out.Str(" Ze RADIUS Admin Interop Lab (FreeRADIUS)\n")
	out.Str("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
	if r.SetupError != "" {
		out.Str("  ").Colored(color.BoldRed).Str("✗ FAIL: ").Str(r.SetupError).
			Colored(color.Reset).Byte('\n')
	}
	for _, scenario := range r.Scenarios {
		out.Str("── ").Str(scenario.Name).Str(" ──\n")
		if scenario.Passed {
			out.Str("  ").Colored(color.BrightGreen).Str("✓ PASS").
				Colored(color.Reset).Str("\n\n")
			continue
		}
		out.Str("  ").Colored(color.BoldRed).Str("✗ FAIL: ").Str(scenario.Error).
			Colored(color.Reset).Str("\n\n")
	}
	out.Str("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	if r.Code == 0 {
		out.Colored(color.BrightGreen).Str("PASS  ").Int(int64(r.Passed)).
			Str(" scenario(s)").Colored(color.Reset).Byte('\n')
	} else {
		out.Colored(color.BoldRed).Str("FAIL  ").Int(int64(r.Passed)).Str(" passed, ").
			Int(int64(r.Failed)).Str(" failed: ").Join(r.FailedNames, " ").
			Colored(color.Reset).Byte('\n')
	}
	out.Str("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	return out.String()
}

// scenarioCheckerMap binds every scenario directory to its typed checker.
// Discover refuses a directory this map does not name, so a fixture added
// without a checker breaks the run instead of passing silently.
func scenarioCheckerMap(timeout time.Duration) map[string]interoplab.Checker {
	return map[string]interoplab.Checker{
		scenarioPAP:        checker(checkPAP, timeout),
		scenarioCHAP:       checker(checkCHAP, timeout),
		scenarioCHAPHashed: checker(checkCHAPHashed, timeout),
	}
}

// ScenarioNames returns every RADIUS interop scenario in lexical selection order.
func ScenarioNames() []string {
	names := make([]string, 0, len(scenarioCheckerMap(0)))
	for name := range scenarioCheckerMap(0) {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// setupFailure answers the report of a lab that never reached a scenario. The
// suite ran nothing, so it reports no scenario result at all rather than a
// scenario that failed: an empty Scenarios list and a SetupError is what tells
// the reader the peer was never contacted.
func setupFailure(err error) (Report, int) {
	var report Report
	report.SetupError = err.Error()
	report.Code = 1
	return report, 1
}

// Run resolves the checkout and invokes the native gate.
func Run() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		return setupFailure(err)
	}
	report, code := RunAt(context.Background(), root)
	return report, code
}

// RunAt runs the selected FreeRADIUS scenarios against root.
func RunAt(ctx context.Context, root string) (Report, int) {
	environment := interoplab.ReadEnvironment(interoplab.EnvironmentOptions{
		SelectorVariable: "RADIUS_INTEROP_SCENARIO",
		SuffixVariable:   "ZE_RADIUS_INTEROP_SUFFIX",
		DefaultSuffix:    strconv.Itoa(os.Getpid()),
	})
	return runAt(ctx, root, environment, interoplab.NewDocker())
}

func runAt(ctx context.Context, root string, environment interoplab.Environment, docker *interoplab.Docker) (Report, int) {
	suite, err := suiteFor(root, environment, docker)
	if err != nil {
		return setupFailure(err)
	}
	suiteReport := suite.Run(ctx)
	report := Report{SuiteReport: suiteReport}
	return report, suiteReport.Code
}

// suiteFor builds the complete suite without starting anything, so a test can
// assert the topology this lab declares.
func suiteFor(root string, environment interoplab.Environment, docker *interoplab.Docker) (interoplab.Suite, error) {
	scenarioRoot := filepath.Join(root, labDirectory, "scenarios")
	sources, err := interoplab.Discover(scenarioRoot, environment.Selector, scenarioCheckerMap(environment.SessionTimeout))
	if err != nil {
		return interoplab.Suite{}, err
	}

	plans := make([]interoplab.ScenarioPlan, 0, len(sources))
	for _, source := range sources {
		plans = append(plans, scenarioPlan(root, environment.Suffix, source))
	}

	return interoplab.Suite{
		Docker: docker,
		// The only preflight this lab owns is the ze cross-compile. It probes
		// no kernel module, because nothing on the admin login path needs one.
		Preflight: func(buildContext context.Context, _ *interoplab.Docker) error {
			if environment.NoBuild {
				return nil
			}
			return buildZe(buildContext, root)
		},
		Images: []interoplab.ImageBuild{
			{Name: zePeer, Tag: zeImage, Dockerfile: filepath.Join(root, labDirectory, "Dockerfile.ze"), Context: root, Required: true},
			{Name: serverPeer, Tag: serverImage, Pull: true, Required: true},
		},
		Scenarios: plans,
		NoBuild:   environment.NoBuild,
	}, nil
}

func scenarioPlan(root, suffix string, source interoplab.ScenarioSource) interoplab.ScenarioPlan {
	var tb textbuf.Buffer
	zeContainer := tb.Str("ze-radius-ze-").Str(suffix).String()
	serverContainer := tb.Reset().Str("ze-radius-server-").Str(suffix).String()
	networkName := tb.Reset().Str("ze-radius-").Str(suffix).String()
	return interoplab.ScenarioPlan{
		Source: source,
		Network: interoplab.NetworkSpec{
			Name:       networkName,
			Candidates: []interoplab.Subnet{{IPv4: networkPrefix}},
		},
		Containers: []string{zeContainer, serverContainer},
		// The server starts first so ze's first Access-Request meets a
		// listening socket rather than the retransmit path.
		Peers: []interoplab.PeerConfig{
			serverPeerConfig(root, source, serverContainer),
			zePeerConfig(source, zeContainer),
		},
	}
}

func serverPeerConfig(root string, source interoplab.ScenarioSource, container string) interoplab.PeerConfig {
	lab := filepath.Join(root, labDirectory)
	return interoplab.PeerConfig{
		Name:      serverPeer,
		Container: container,
		Image:     serverPeer,
		Host:      3,
		Mounts: []interoplab.Mount{
			{Source: filepath.Join(lab, "clients.conf"), Target: "/etc/raddb/clients.conf", ReadOnly: true},
			{Source: filepath.Join(lab, "site-default"), Target: "/etc/raddb/sites-enabled/default", ReadOnly: true},
			{Source: filepath.Join(lab, "mods-ze-request-log"), Target: "/etc/raddb/mods-enabled/ze_request_log", ReadOnly: true},
			{Source: filepath.Join(source.Directory, "users"), Target: "/etc/raddb/mods-config/files/authorize", ReadOnly: true},
		},
		// -X keeps the full request and reply on the container log, so a failed
		// scenario names what the server decoded rather than only that it said no.
		Command: []string{"radiusd", "-f", "-X"},
		Ready: &interoplab.ReadyProbe{
			// 0714 is 1812. The probe reads the listening socket rather than
			// sleeping, so a slow start delays the scenario instead of failing it.
			Command:  []string{"sh", "-c", "grep -q ':0714 ' /proc/net/udp"},
			Timeout:  90 * time.Second,
			Interval: time.Second,
		},
	}
}

func zePeerConfig(source interoplab.ScenarioSource, container string) interoplab.PeerConfig {
	return interoplab.PeerConfig{
		Name:      zePeer,
		Container: container,
		Image:     zePeer,
		Host:      2,
		Mounts: []interoplab.Mount{
			{Source: filepath.Join(source.Directory, "ze.conf"), Target: zeConfigTarget, ReadOnly: true},
		},
		Environment: []interoplab.EnvironmentVariable{
			{Name: "ZE_STORAGE_BLOB", Value: "false"},
			{Name: "ZE_LOG_LEVEL", Value: "debug"},
		},
		// The hostname is what ze sends as NAS-Identifier, so fixing it here
		// makes the server's record name ze rather than a container id.
		Arguments: []string{"--hostname", nasIdentifier},
		Command:   []string{"start", zeConfigTarget},
		Ready: &interoplab.ReadyProbe{
			Command:  []string{"sh", "-c", "ss -ltn | grep -q ':" + sshPort + " '"},
			Timeout:  90 * time.Second,
			Interval: time.Second,
		},
	}
}

// buildZe cross-compiles the daemon this lab runs. The tags come from
// feature-gates.txt through featuretags.DaemonBuildTags, so the lab daemon
// carries the same features as every other build of this tree and a scenario
// cannot fail on a feature that was compiled out.
//
// The environment comes from gotoolchain rather than from os.Environ, because
// that is what puts GOCACHE inside the checkout and pins GOTOOLCHAIN to the
// one go.mod names. An ambient environment resolves GOCACHE to the machine's
// default, which nothing in this repository manages: `./le scratch cache-clean`
// does not clear it, the full-disk discipline does not measure it, and another
// tool trimming it mid-build fails this cross-compile with a wave of
// "no such file or directory" against Go's own standard library
// (plan/journal/full-disk-false-red.md). Measured here on 2026-09-04, twice.
func buildZe(ctx context.Context, root string) error {
	tags, err := featuretags.DaemonBuildTags(root, "ze_core ze_distro")
	if err != nil {
		return err
	}
	toolchain, err := gotoolchain.New(root)
	if err != nil {
		return err
	}
	output := filepath.Join(root, labDirectory, "ze-linux")
	buildContext, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	command := exec.CommandContext(buildContext, "go", "build", "-tags", tags, "-o", output, "./cmd/ze") // #nosec G204 -- the fixed Go build consumes only checked-in feature tags and writes the fixed lab binary.
	command.Dir = root
	command.Env = toolchain.Environment(gotoolchain.EnvOptions{GOOS: "linux"})
	combined, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cross-compile ze: %w: %s", err, strings.TrimSpace(string(combined)))
	}
	return nil
}

// checker adapts a scenario body to the protocol-neutral Checker signature.
func checker(body func(context.Context, *scenarioLab) error, timeout time.Duration) interoplab.Checker {
	return func(ctx context.Context, check *interoplab.CheckContext) error {
		if check == nil || check.Lab == nil {
			return errors.New("RADIUS scenario received no lab")
		}
		return body(ctx, &scenarioLab{lab: check.Lab, timeout: timeout})
	}
}
