// Design: plan/spec-le-is-a-ze-binary.md -- native L2TP interoperability gate.
// Related: checkers.go -- protocol-specific observations and assertions.
// Related: radiusmock/main.go -- independent RADIUS wire peer for scenario 04.
package l2tp

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/lepath"
)

const (
	defaultFRRImage = "quay.io/frrouting/frr:10.3.1"
	peerZe          = "ze"
	peerLAC         = "xl2tpd"
	peerFRR         = "frr"
	peerRadius      = "radius"

	scenarioPPPIPv4         = "01-ppp-ipv4"
	scenarioBGPRedistribute = "02-ppp-bgp-redistribute-frr"
	scenarioInitiator       = "03-ze-lac-xl2tpd-lns"
	scenarioRadiusAttrs     = "04-radius-acct-attrs"

	commandShow        = "show"
	commandLink        = "link"
	commandType        = "type"
	commandVTYSH       = "vtysh"
	modulesPath        = "/lib/modules"
	privilegedArgument = "--privileged"
)

func scenarioCheckerMap(timeout time.Duration) map[string]interoplab.Checker {
	return map[string]interoplab.Checker{
		scenarioPPPIPv4:         checker(checkPPPIPv4, timeout, peerZe, peerLAC),
		scenarioBGPRedistribute: checker(checkBGPRedistribute, timeout, peerZe, peerLAC, peerFRR),
		scenarioInitiator:       checker(checkInitiator, timeout, peerZe, peerLAC),
		scenarioRadiusAttrs:     checker(checkRadiusAttributes, timeout, peerZe, peerLAC, peerRadius),
	}
}

// ScenarioNames returns every typed L2TP scenario in lexical selection order.
func ScenarioNames() []string {
	names := make([]string, 0, len(scenarioCheckerMap(0)))
	for name := range scenarioCheckerMap(0) {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Run resolves the repository root and reads the optional native selector.
func Run(ctx context.Context) interoplab.SuiteReport {
	root, err := lepath.Root()
	if err != nil {
		var tb textbuf.Buffer
		return interoplab.SuiteReport{
			SetupError: tb.Str("resolve repository root: ").Err(err).String(),
			Code:       1,
		}
	}
	selector := strings.TrimSpace(os.Getenv("ZE_L2TP_INTEROP_SCENARIO"))
	return RunAt(ctx, root, selector)
}

// RunAt runs the selected L2TP scenarios from an explicit repository tree.
func RunAt(ctx context.Context, root, selector string) interoplab.SuiteReport {
	if strings.TrimSpace(selector) == "" {
		selector = strings.TrimSpace(os.Getenv("ZE_L2TP_INTEROP_SCENARIO"))
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		var tb textbuf.Buffer
		return interoplab.SuiteReport{
			SetupError: tb.Str("resolve L2TP tree: ").Err(err).String(),
			Code:       1,
		}
	}
	environment := interoplab.ReadEnvironment(interoplab.EnvironmentOptions{
		SuffixVariable: "ZE_L2TP_INTEROP_SUFFIX",
		DefaultImage:   defaultFRRImage,
	})
	return runAt(ctx, absoluteRoot, selector, interoplab.NewDocker(), environment)
}

func runAt(ctx context.Context, root, selector string, docker *interoplab.Docker, environment interoplab.Environment) interoplab.SuiteReport {
	plans, standardFlow, err := plansAt(root, selector, environment)
	if err != nil {
		return interoplab.SuiteReport{SetupError: err.Error(), Code: 1}
	}

	suite := interoplab.Suite{
		Docker:    docker,
		Images:    imageBuilds(root, plans, environment.Image),
		Scenarios: plans,
		NoBuild:   environment.NoBuild,
	}
	if standardFlow {
		suite.Preflight = preflight(environment.Suffix)
	}
	return suite.Run(ctx)
}

func plansAt(root, selector string, environment interoplab.Environment) ([]interoplab.ScenarioPlan, bool, error) {
	scenariosRoot := filepath.Join(root, "test", "interop-l2tp", "scenarios")
	sources, err := interoplab.Discover(scenariosRoot, selector, scenarioCheckerMap(environment.SessionTimeout))
	if err != nil {
		return nil, false, err
	}

	plans := make([]interoplab.ScenarioPlan, 0, len(sources))
	standardFlow := false
	for _, source := range sources {
		plans = append(plans, scenarioPlan(root, source, environment))
		if source.Name != scenarioInitiator {
			standardFlow = true
		}
	}
	return plans, standardFlow, nil
}

func scenarioPlan(root string, source interoplab.ScenarioSource, environment interoplab.Environment) interoplab.ScenarioPlan {
	var tb textbuf.Buffer
	networkName := tb.Str("ze-l2tp-").Str(environment.Suffix).String()
	containers := []string{
		tb.Reset().Str("ze-l2tp-ze-").Str(environment.Suffix).String(),
		tb.Reset().Str("ze-l2tp-lac-").Str(environment.Suffix).String(),
		tb.Reset().Str("ze-l2tp-frr-").Str(environment.Suffix).String(),
		tb.Reset().Str("ze-l2tp-radius-").Str(environment.Suffix).String(),
	}
	plan := interoplab.ScenarioPlan{
		Source: source,
		Network: interoplab.NetworkSpec{
			Name:       networkName,
			Candidates: []interoplab.Subnet{{IPv4: netip.MustParsePrefix("172.29.0.0/24")}},
		},
		Containers: containers,
	}
	plan.Prepare = func(ctx context.Context, prepared interoplab.PrepareContext) (interoplab.PreparedScenario, error) {
		return prepareScenario(ctx, root, source, environment, prepared.Network)
	}
	return plan
}

func prepareScenario(_ context.Context, root string, source interoplab.ScenarioSource, environment interoplab.Environment, network interoplab.Network) (interoplab.PreparedScenario, error) {
	directory := source.Directory
	zeConfig := filepath.Join(directory, "ze.conf")
	var cleanup func() error
	if source.Name == scenarioInitiator {
		rendered, remove, err := renderInitiatorConfig(directory, network)
		if err != nil {
			return interoplab.PreparedScenario{}, err
		}
		zeConfig = rendered
		cleanup = remove
	}

	ze := zePeer(environment.Suffix, zeConfig)
	if source.Name == scenarioInitiator {
		ze.Environment = append(ze.Environment,
			interoplab.EnvironmentVariable{Name: "ze.l2tp.skip-kernel-probe", Value: "true"},
			interoplab.EnvironmentVariable{Name: "ze.log.l2tp", Value: "debug"})
		ze.Ready.Command = []string{"sh", "-c",
			"ss -lun | grep -q ':17011 ' && ss -ltn | grep -q ':17012 '"}
	}
	lac := lacPeer(environment.Suffix, directory)
	peers := []interoplab.PeerConfig{ze}
	switch source.Name {
	case scenarioPPPIPv4:
		peers = append(peers, lac)
	case scenarioBGPRedistribute:
		peers = append(peers, frrPeer(root, environment.Suffix, directory), lac)
	case scenarioInitiator:
		peers = []interoplab.PeerConfig{lac, ze}
	case scenarioRadiusAttrs:
		peers = []interoplab.PeerConfig{radiusPeer(environment.Suffix), ze, lac}
	default:
		if cleanup != nil {
			_ = cleanup()
		}
		return interoplab.PreparedScenario{}, fmt.Errorf("unsupported L2TP scenario %s", source.Name)
	}
	return interoplab.PreparedScenario{Peers: peers, Cleanup: cleanup}, nil
}

func zePeer(suffix, config string) interoplab.PeerConfig {
	mounts := []interoplab.Mount{{Source: config, Target: "/etc/ze/ze.conf", ReadOnly: true}}
	if modulesAvailable() {
		mounts = append(mounts, interoplab.Mount{Source: modulesPath, Target: modulesPath, ReadOnly: true})
	}
	var tb textbuf.Buffer
	return interoplab.PeerConfig{
		Name:      peerZe,
		Container: tb.Str("ze-l2tp-ze-").Str(suffix).String(),
		Image:     "ze",
		Host:      2,
		Mounts:    mounts,
		Arguments: []string{privilegedArgument},
		Environment: []interoplab.EnvironmentVariable{
			{Name: "ZE_LOG_L2TP", Value: "debug"},
			{Name: "ZE_STORAGE_BLOB", Value: "false"},
			{Name: "ze.l2tp.ncp.enable-ipv6cp", Value: "false"},
			{Name: "ze.l2tp.ncp.ip-timeout", Value: "15s"},
			{Name: "ze.l2tp.auth.timeout", Value: "15s"},
		},
		Command: []string{"start", "/etc/ze/ze.conf"},
		Ready: &interoplab.ReadyProbe{
			Command:  []string{"sh", "-c", "ss -lun | grep -q ':1701 '"},
			Timeout:  30 * time.Second,
			Interval: time.Second,
		},
	}
}

func lacPeer(suffix, directory string) interoplab.PeerConfig {
	mounts := make([]interoplab.Mount, 0, 4)
	for _, file := range []struct{ source, target string }{
		{"xl2tpd.conf", "/etc/xl2tpd/xl2tpd.conf"},
		{"ppp-options", "/etc/ppp/options.l2tpd.client"},
		{"l2tp-secrets", "/etc/xl2tpd/l2tp-secrets"},
		{"options.xl2tpd", "/etc/ppp/options.xl2tpd"},
	} {
		source := filepath.Join(directory, file.source)
		if _, err := os.Stat(source); err == nil {
			mounts = append(mounts, interoplab.Mount{Source: source, Target: file.target, ReadOnly: true})
		}
	}
	if modulesAvailable() {
		mounts = append(mounts, interoplab.Mount{Source: modulesPath, Target: modulesPath, ReadOnly: true})
	}
	var tb textbuf.Buffer
	return interoplab.PeerConfig{
		Name:      peerLAC,
		Container: tb.Str("ze-l2tp-lac-").Str(suffix).String(),
		Image:     "lac",
		Host:      3,
		Mounts:    mounts,
		Arguments: []string{privilegedArgument},
		Ready:     &interoplab.ReadyProbe{Command: []string{"sh", "-c", "kill -0 1"}, Timeout: 15 * time.Second, Interval: time.Second},
	}
}

func frrPeer(root, suffix, directory string) interoplab.PeerConfig {
	var tb textbuf.Buffer
	container := tb.Str("ze-l2tp-frr-").Str(suffix).String()
	readyCommand := tb.Reset().Str(commandShow).Str(" bgp summary").String()
	return interoplab.PeerConfig{
		Name:      peerFRR,
		Container: container,
		Image:     "frr",
		Host:      4,
		Mounts: []interoplab.Mount{
			{Source: filepath.Join(directory, "frr.conf"), Target: "/etc/frr/frr.conf", ReadOnly: true},
			{Source: filepath.Join(root, "test", "interop-l2tp", "daemons"), Target: "/etc/frr/daemons", ReadOnly: true},
			{Source: filepath.Join(root, "test", "interop-l2tp", "vtysh.conf"), Target: "/etc/frr/vtysh.conf", ReadOnly: true},
		},
		Capabilities: []string{"NET_ADMIN", "SYS_ADMIN"},
		Ready: &interoplab.ReadyProbe{
			Command:  []string{commandVTYSH, "-c", readyCommand},
			Timeout:  30 * time.Second,
			Interval: 2 * time.Second,
		},
	}
}

func radiusPeer(suffix string) interoplab.PeerConfig {
	var tb textbuf.Buffer
	return interoplab.PeerConfig{
		Name:      peerRadius,
		Container: tb.Str("ze-l2tp-radius-").Str(suffix).String(),
		Image:     "radius",
		Host:      5,
		Ready: &interoplab.ReadyProbe{
			Command:  []string{"sh", "-c", "grep -q ':0714 ' /proc/net/udp"},
			Timeout:  20 * time.Second,
			Interval: 500 * time.Millisecond,
		},
	}
}

func imageBuilds(root string, plans []interoplab.ScenarioPlan, frrImage string) []interoplab.ImageBuild {
	needFRR := false
	needRadius := false
	for _, plan := range plans {
		needFRR = needFRR || plan.Source.Name == scenarioBGPRedistribute
		needRadius = needRadius || plan.Source.Name == scenarioRadiusAttrs
	}
	labRoot := filepath.Join(root, "test", "interop-l2tp")
	builds := []interoplab.ImageBuild{
		{Name: "ze", Tag: "ze-l2tp-interop", Dockerfile: filepath.Join(labRoot, "Dockerfile.ze"), Context: root, Required: true},
		{Name: "lac", Tag: "ze-l2tp-lac", Dockerfile: filepath.Join(labRoot, "Dockerfile.lac"), Context: labRoot, Required: true},
	}
	if needFRR {
		builds = append(builds, interoplab.ImageBuild{Name: "frr", Tag: frrImage, Pull: true, Required: true})
	}
	if needRadius {
		builds = append(builds, interoplab.ImageBuild{Name: "radius", Tag: "ze-l2tp-radius", Dockerfile: filepath.Join(root, "internal", "le", "interoplab", "l2tp", "radiusmock", "Dockerfile"), Context: root, Required: true})
	}
	return builds
}

func preflight(suffix string) interoplab.PreflightCheck {
	return func(ctx context.Context, docker *interoplab.Docker) error {
		for _, key := range []string{"ZE_L2TP_SKIP_KERNEL_PROBE", "ze.l2tp.skip-kernel-probe"} {
			if _, exists := os.LookupEnv(key); exists {
				return fmt.Errorf("refusing to run with %s set; full proof must not skip the kernel probe", key)
			}
		}
		var tb textbuf.Buffer
		containerName := tb.Str("ze-l2tp-preflight-").Str(suffix).String()
		arguments := []string{privilegedArgument, "--name", containerName}
		if modulesAvailable() {
			mount := tb.Reset().Str(modulesPath).Byte(':').Str(modulesPath).Str(":ro").String()
			arguments = append(arguments, "-v", mount)
		}
		result, err := docker.RunOneShot(ctx, interoplab.OneShotContainer{
			Image:     "alpine:3.21",
			Arguments: arguments,
			Command: []string{"sh", "-c",
				"apk add --no-cache -q iproute2 kmod > /dev/null 2>&1 && " +
					"modprobe ppp_generic 2>/dev/null; modprobe l2tp_ppp 2>/dev/null; " +
					"modprobe pppol2tp 2>/dev/null; " +
					"echo DEV_PPP=$(test -c /dev/ppp && echo ok || echo missing); " +
					"echo L2TP_PPP=$(test -d /sys/module/l2tp_ppp -o -d /sys/module/pppol2tp -o -f /proc/net/pppol2tp && echo ok || echo missing); " +
					"echo IP_L2TP=$(ip l2tp show tunnel > /dev/null 2>&1 && echo ok || echo missing)"},
			Timeout: 120 * time.Second,
		})
		if err != nil {
			return fmt.Errorf("preflight probe failed: %w", err)
		}
		checks := parsePreflight(result.Stdout)
		missing := make([]string, 0, 3)
		if checks["DEV_PPP"] != "ok" {
			missing = append(missing, "/dev/ppp (PPP character device)")
		}
		if checks["L2TP_PPP"] != "ok" {
			missing = append(missing, "l2tp_ppp/pppol2tp kernel module")
		}
		if checks["IP_L2TP"] != "ok" {
			missing = append(missing, "ip l2tp (L2TP Generic Netlink)")
		}
		if len(missing) > 0 {
			message := tb.Reset().Str("host kernel missing PPPoL2TP requirements: ").
				Join(missing, ", ").String()
			return errors.New(message)
		}
		return nil
	}
}

func parsePreflight(output string) map[string]string {
	checks := make(map[string]string, 3)
	for line := range strings.SplitSeq(output, "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if found {
			checks[key] = value
		}
	}
	return checks
}

func renderInitiatorConfig(directory string, network interoplab.Network) (string, func() error, error) {
	configPath := filepath.Join(directory, "ze.conf")
	config, err := os.ReadFile(configPath) // #nosec G304 -- configPath is inside the discovered, repository-owned L2TP scenario fixture.
	if err != nil {
		return "", nil, fmt.Errorf("read initiator config: %w", err)
	}
	peerAddress, err := addressAtHost(network.IPv4, 3)
	if err != nil {
		return "", nil, err
	}
	oldLine := "\t\taddress 127.0.0.1"
	var tb textbuf.Buffer
	newLine := tb.Str("\t\taddress ").Addr(peerAddress).String()
	if strings.Count(string(config), oldLine) != 1 {
		return "", nil, errors.New("initiator config must contain exactly one xl2tpd address")
	}
	rendered := []byte(strings.Replace(string(config), oldLine, newLine, 1))
	directoryRendered, err := os.MkdirTemp("", "ze-l2tp-03-")
	if err != nil {
		return "", nil, fmt.Errorf("create initiator config directory: %w", err)
	}
	remove := func() error { return os.RemoveAll(directoryRendered) }
	path := filepath.Join(directoryRendered, "ze.conf")
	if err := os.WriteFile(path, rendered, 0o600); err != nil {
		_ = remove()
		return "", nil, fmt.Errorf("write initiator config: %w", err)
	}
	return path, remove, nil
}

func addressAtHost(prefix netip.Prefix, host byte) (netip.Addr, error) {
	if !prefix.IsValid() || !prefix.Addr().Is4() || prefix.Bits() != 24 {
		return netip.Addr{}, errors.New("L2TP scenario requires an IPv4 /24")
	}
	octets := prefix.Masked().Addr().As4()
	octets[3] = host
	return netip.AddrFrom4(octets), nil
}

func modulesAvailable() bool {
	info, err := os.Stat(modulesPath)
	return err == nil && info.IsDir()
}
