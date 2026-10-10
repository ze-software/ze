// Design: docs/architecture/testing/interop.md -- RSVP-TE interop against freeRouter.
// Related: checkers.go -- what each scenario observes at the peers.
//
// The lab puts up to four nodes on one Docker segment, and each scenario
// decides which implementation fills each role by the files it carries:
// <role>.conf makes the role a Ze node, <role>-sw.txt a freeRouter node. Each
// freeRouter owns its own IPv4 stack and MAC, reached through rawInt.bin on
// its container's eth0 (test/interop-rsvpte/run-freertr.sh), so no host root,
// TAP device or network namespace is needed: Docker and privileged containers
// suffice. Ze runs in a privileged container because it programs MPLS labels.
package rsvpte

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

const (
	// catalogSuite is the name a feature declaration cites this lab by.
	catalogSuite = "rsvpte"
	labDirectory = "test/interop-rsvpte"

	peerIngress = "ingress"
	peerTransit = "transit"
	peerRelay   = "relay"
	peerEgress  = "egress"

	imageZe      = "ze"
	imageFreeRtr = "freertr"

	modulesPath        = "/lib/modules"
	privilegedArgument = "--privileged"
)

// labNetwork is fixed because every scenario file names its addresses: the
// freeRouter configurations and Ze's routes cannot follow a moved subnet.
var labNetwork = netip.MustParsePrefix("172.29.81.0/24")

// labRole is one position on the path and the host number its container
// takes. A Ze node answers on the container address 172.29.81.<host>; a
// freeRouter node owns 172.29.81.<10+host> beside it.
type labRole struct {
	name string
	host uint8
}

// labRoles lists every position downstream first, so the first PATH the
// ingress sends meets nodes that already listen.
var labRoles = []labRole{{peerEgress, 4}, {peerTransit, 3}, {peerRelay, 5}, {peerIngress, 2}}

func scenarioCheckerMap(timeout time.Duration) map[string]interoplab.Checker {
	return map[string]interoplab.Checker{
		scenarioLooseExpansion:  checker(checkLooseExpansion, timeout),
		scenarioResvErrRelayed:  checker(checkResvErrRelayed, timeout),
		scenarioStrictForwarded: checker(checkStrictForwarded, timeout),
		scenarioStrictRefused:   checker(checkStrictRefused, timeout),
		scenarioResvTearRelayed: checker(checkResvTearRelayed, timeout),
		scenarioIncreaseInPlace: checker(checkIncreaseInPlace, timeout),
		scenarioFFUnknownSender: checker(checkFFUnknownSender, timeout),
		scenarioBackupPathToMP:  checker(checkBackupPathToMP, timeout),
	}
}

// ScenarioNames returns every typed RSVP-TE scenario in lexical order.
func ScenarioNames() []string {
	names := make([]string, 0, len(scenarioCheckerMap(0)))
	for name := range scenarioCheckerMap(0) {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// RunAt runs the selected scenarios from an explicit repository tree. An empty
// selector reads RSVPTE_INTEROP_SCENARIO, and an empty value runs them all.
func RunAt(ctx context.Context, root, selector string) interoplab.SuiteReport {
	environment := interoplab.ReadEnvironment(interoplab.EnvironmentOptions{
		SelectorVariable: "RSVPTE_INTEROP_SCENARIO",
		SuffixVariable:   "ZE_RSVPTE_INTEROP_SUFFIX",
	})
	if strings.TrimSpace(selector) != "" {
		environment.Selector = strings.TrimSpace(selector)
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		var tb textbuf.Buffer
		return interoplab.SuiteReport{SetupError: tb.Str("resolve RSVP-TE tree: ").Err(err).String(), Code: 1}
	}
	suite, err := suiteFor(absoluteRoot, environment, interoplab.NewDocker())
	if err != nil {
		return interoplab.SuiteReport{SetupError: err.Error(), Code: 1}
	}
	return suite.Run(ctx)
}

// suiteFor builds the complete suite without starting anything, so a test can
// assert the topology this lab declares.
func suiteFor(root string, environment interoplab.Environment, docker *interoplab.Docker) (interoplab.Suite, error) {
	sources, err := interoplab.Discover(filepath.Join(root, labDirectory, "scenarios"), environment.Selector, scenarioCheckerMap(environment.SessionTimeout))
	if err != nil {
		return interoplab.Suite{}, err
	}
	plans := make([]interoplab.ScenarioPlan, 0, len(sources))
	for _, source := range sources {
		plan, err := scenarioPlan(environment.Suffix, source)
		if err != nil {
			return interoplab.Suite{}, err
		}
		plans = append(plans, plan)
	}
	labRoot := filepath.Join(root, labDirectory)
	return interoplab.Suite{
		Docker:   docker,
		StagedZe: interoplab.StagedZePath(root, LabBinaries()),
		// The MPLS probe runs first: a host kernel without mpls_router cannot
		// run this lab, and refusing it before the cross-compile costs nothing.
		Preflight: interoplab.Preflights(mplsPreflight(environment.Suffix), interoplab.StageBinaries(root, environment.NoBuild, LabBinaries()...)),
		Images: []interoplab.ImageBuild{
			{Name: imageZe, Tag: "ze-rsvpte-interop", Dockerfile: filepath.Join(labRoot, "Dockerfile.ze"), Context: root, Required: true},
			{Name: imageFreeRtr, Tag: "ze-rsvpte-freertr", Dockerfile: filepath.Join(labRoot, "Dockerfile.freertr"), Context: labRoot, Required: true},
		},
		Scenarios: plans,
		NoBuild:   environment.NoBuild,
	}, nil
}

// LabBinaries declares the one binary this lab stages into its Docker build
// context, which is what test/interop-rsvpte/Dockerfile.ze copies in.
func LabBinaries() []interoplab.LabBinary {
	return []interoplab.LabBinary{
		{Name: "ze", Base: repofeaturetags.DaemonBase, Output: filepath.Join(labDirectory, "ze-linux")},
	}
}

func containerName(role, suffix string) string {
	var tb textbuf.Buffer
	return tb.Str("ze-rsvpte-").Str(role).Byte('-').Str(suffix).String()
}

// scenarioPlan starts every role the scenario fills, downstream first. A role
// with <role>.conf is a Ze node and one with <role>-sw.txt a freeRouter node;
// a role with neither is absent from the scenario.
func scenarioPlan(suffix string, source interoplab.ScenarioSource) (interoplab.ScenarioPlan, error) {
	plan := interoplab.ScenarioPlan{
		Source: source,
		Network: interoplab.NetworkSpec{
			Name:       containerName("net", suffix),
			Candidates: []interoplab.Subnet{{IPv4: labNetwork}},
		},
	}
	for _, role := range labRoles {
		var peer interoplab.PeerConfig
		switch {
		case fileExists(filepath.Join(source.Directory, role.name+".conf")):
			peer = zePeer(role.name, role.host, suffix, source.Directory)
		case fileExists(filepath.Join(source.Directory, role.name+"-sw.txt")):
			environment, err := freeRtrEnvironment(filepath.Join(source.Directory, role.name+"-env.txt"))
			if err != nil {
				return interoplab.ScenarioPlan{}, err
			}
			peer = freeRtrPeer(role.name, role.host, suffix, source.Directory)
			peer.Environment = environment
		default:
			continue
		}
		plan.Containers = append(plan.Containers, peer.Container)
		plan.Peers = append(plan.Peers, peer)
	}
	return plan, nil
}

// freeRtrEnvironment reads the optional <role>-env.txt of a freeRouter role:
// one NAME=value per line, which the container receives as its environment.
// This is how a scenario turns on a test knob of
// test/interop-rsvpte/freertr/ze-interop-resv.patch. A missing file means no
// knob; a line that is not NAME=value is refused rather than skipped, so a
// typo cannot silently run the scenario against an unaltered peer.
func freeRtrEnvironment(path string) ([]interoplab.EnvironmentVariable, error) {
	content, err := os.ReadFile(path) //nolint:gosec // a scenario file in the checkout under test
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read freeRouter environment %s: %w", path, err)
	}
	var environment []interoplab.EnvironmentVariable
	for line := range strings.SplitSeq(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, found := strings.Cut(line, "=")
		if !found || name == "" {
			return nil, fmt.Errorf("freeRouter environment %s: line %q is not NAME=value", path, line)
		}
		environment = append(environment, interoplab.EnvironmentVariable{Name: name, Value: value})
	}
	return environment, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func freeRtrPeer(role string, host uint8, suffix, directory string) interoplab.PeerConfig {
	return interoplab.PeerConfig{
		Name:      role,
		Container: containerName(role, suffix),
		Image:     imageFreeRtr,
		Host:      host,
		Mounts: []interoplab.Mount{
			{Source: filepath.Join(directory, role+"-hw.txt"), Target: "/etc/freertr/rtr-hw.txt", ReadOnly: true},
			{Source: filepath.Join(directory, role+"-sw.txt"), Target: "/etc/freertr/rtr-sw.txt", ReadOnly: true},
		},
		Capabilities: []string{"NET_ADMIN", "NET_RAW"},
		Ready: &interoplab.ReadyProbe{
			Command:  []string{"sh", "-c", "pgrep -f rawInt.bin > /dev/null"},
			Timeout:  90 * time.Second,
			Interval: time.Second,
		},
	}
}

// zePeer runs <role>-setup.sh, which enables MPLS, adds the role's routes and
// execs the daemon on <role>.conf. tcpdump records the RSVP the node sends and
// receives in the same capture file a freeRouter node writes.
func zePeer(role string, host uint8, suffix, directory string) interoplab.PeerConfig {
	return interoplab.PeerConfig{
		Name:      role,
		Container: containerName(role, suffix),
		Image:     imageZe,
		Host:      host,
		Mounts: []interoplab.Mount{
			{Source: filepath.Join(directory, role+".conf"), Target: "/etc/ze/ze.conf", ReadOnly: true},
			{Source: filepath.Join(directory, role+"-setup.sh"), Target: "/etc/ze/setup.sh", ReadOnly: true},
		},
		Arguments:   []string{privilegedArgument},
		Environment: []interoplab.EnvironmentVariable{{Name: "ze.log.rsvp-te", Value: "debug"}},
		Command: []string{"sh", "-c", "mkdir -p /run/fr; " +
			"tcpdump -i eth0 -nn -l -vvv 'ip proto 46 or mpls' > " + captureFile + " 2> /run/fr/tcpdump.err & " +
			"exec sh /etc/ze/setup.sh"},
		Ready: &interoplab.ReadyProbe{
			// Ze listens on a raw IPv4 socket for protocol 46 once RSVP-TE runs.
			Command:  []string{"sh", "-c", "grep -q ':002E ' /proc/net/raw"},
			Timeout:  30 * time.Second,
			Interval: time.Second,
		},
	}
}

// mplsPreflight loads the MPLS modules the Ze container needs and refuses the
// run when the host kernel cannot provide them. Docker's privileged container
// is the only privilege it uses.
func mplsPreflight(suffix string) interoplab.PreflightCheck {
	return func(ctx context.Context, docker *interoplab.Docker) error {
		arguments := []string{privilegedArgument, "--name", containerName("preflight", suffix)}
		if info, err := os.Stat(modulesPath); err == nil && info.IsDir() {
			var tb textbuf.Buffer
			arguments = append(arguments, "-v", tb.Str(modulesPath).Byte(':').Str(modulesPath).Str(":ro").String())
		}
		result, err := docker.RunOneShot(ctx, interoplab.OneShotContainer{
			Image:     "alpine:3.21",
			Arguments: arguments,
			Command: []string{"sh", "-c",
				"apk add --no-cache -q kmod > /dev/null 2>&1; " +
					"modprobe mpls_router 2>/dev/null; modprobe mpls_iptunnel 2>/dev/null; " +
					"echo MPLS=$(test -f /proc/sys/net/mpls/platform_labels && echo ok || echo missing)"},
			Timeout: 120 * time.Second,
		})
		if err != nil {
			return fmt.Errorf("MPLS preflight probe failed: %w", err)
		}
		if !strings.Contains(result.Stdout, "MPLS=ok") {
			return fmt.Errorf("host kernel has no MPLS routing (mpls_router): %s", strings.TrimSpace(result.Stdout))
		}
		return nil
	}
}
