// Design: docs/architecture/testing/interop.md -- native accel-ppp and pppd Docker interop gate.
// Related: scenarios.go -- typed container plans for both PPPoE roles.
// Related: check_client.go -- Ze client assertions against accel-ppp.
// Related: check_ac.go -- Ze access-concentrator assertions against pppd.
// Related: checkers_l2tp.go -- the ze_l2tp half of the scenario table.
package pppoe

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

const (
	suitePath = "test/interop-pppoe"

	zeImageName     = "ze"
	accelImageName  = "accel"
	clientImageName = "client"
	pppdExecutable  = "pppd"

	zeImageTag     = "ze-pppoe-interop"
	accelImageTag  = "ze-pppoe-accel"
	clientImageTag = "ze-pppoe-client"

	roleZeClient = "ze-client"
	roleZeAC     = "ze-ac"

	zeHost     = 2
	accelHost  = 3
	clientHost = 4

	commandShow  = "show"
	commandPkill = "pkill"
	zeConfigPath = "/etc/ze/ze.conf"

	// netAdminCapability and pppDevice are what a PPPoE daemon needs beyond
	// Docker's default grants: NET_ADMIN for PPPIOCNEWUNIT and its addresses
	// and routes, and the /dev/ppp character device for its PPP units. The
	// discovery sockets need NET_RAW, which Docker grants by default. The
	// modules are the host's: a lab container never loads one
	// (spec-lab-containers-least-privilege, D-7).
	netAdminCapability = "NET_ADMIN"
	pppDevice          = "/dev/ppp"
)

// pppDeviceArguments hands a container the host's PPP character device.
func pppDeviceArguments() []string {
	return []string{"--device", pppDevice}
}

// Options carries the native scenario selector and image-build controls.
type Options struct {
	Scenario string
	NoBuild  bool
	Suffix   string
}

// Run resolves the checkout and runs the native PPPoE interop suite.
func Run(ctx context.Context, options Options) interoplab.SuiteReport {
	root, err := lepath.Root()
	if err != nil {
		return setupFailure(err)
	}
	return RunAt(ctx, root, options)
}

// RunAt runs the native PPPoE interop suite against root.
func RunAt(ctx context.Context, root string, options Options) interoplab.SuiteReport {
	environment := interoplab.ReadEnvironment(interoplab.EnvironmentOptions{
		SelectorVariable: "ZE_PPPOE_INTEROP_SCENARIO",
		SuffixVariable:   "ZE_PPPOE_INTEROP_SUFFIX",
	})
	if options.Scenario == "" {
		options.Scenario = environment.Selector
	}
	if options.Suffix == "" {
		options.Suffix = environment.Suffix
	}
	options.NoBuild = options.NoBuild || environment.NoBuild

	sources, err := interoplab.Discover(
		filepath.Join(root, suitePath, "scenarios"),
		options.Scenario,
		checkers(),
	)
	if err != nil {
		return setupFailure(err)
	}
	plans := make([]interoplab.ScenarioPlan, 0, len(sources))
	for _, source := range sources {
		plan, planErr := scenarioPlan(source, options.Suffix)
		if planErr != nil {
			return setupFailure(planErr)
		}
		plans = append(plans, plan)
	}

	docker := interoplab.NewDocker()
	suite := interoplab.Suite{
		Docker:   docker,
		StagedZe: interoplab.StagedZePath(root, LabBinaries()),
		// The kernel probe runs FIRST: a machine with no pppox module cannot
		// run this lab, and refusing it before the cross-compile costs that
		// machine nothing.
		Preflight: interoplab.Preflights(
			preflight(options.Suffix),
			interoplab.StageBinaries(root, options.NoBuild, LabBinaries()...),
		),
		Images:    imageBuilds(root),
		Scenarios: plans,
		NoBuild:   options.NoBuild,
	}
	return suite.Run(ctx)
}

func setupFailure(err error) interoplab.SuiteReport {
	return interoplab.SuiteReport{SetupError: err.Error(), Code: 1}
}

// wireCheckers holds the scenarios whose checkers decode PPPoE frames with
// internal/component/l2tp/pppoe, the feature's own codec. That package is
// compile-out-able under ze_l2tp, so an always-on file MUST NOT import it
// (ai/rules/architecture.md, `./le arch tier check`). checkers_l2tp.go carries the
// import behind the tag and fills this map from its init().
//
// The map is empty rather than absent in a build without the tag, because such
// a build holds no PPPoE access concentrator to drive: offering a scenario that
// cannot run is worse than offering none. Every build that RUNS this lab
// carries the tag, so nothing is lost -- the `le` script and the CI workflows
// both build the le personality with every tag feature-gates.txt declares.
var wireCheckers = map[string]interoplab.Checker{}

func checkers() map[string]interoplab.Checker {
	all := map[string]interoplab.Checker{
		"01-pppoe-chap-ipv4":   checkZeClient,
		"02-ze-ac-pppd-client": checkZeAccessConcentrator,
		"pppoe-pap-ze-client":  checkZeClientPAP,
	}
	maps.Copy(all, wireCheckers)
	return all
}

// ScenarioNames returns every typed PPPoE scenario in lexical selection order.
func ScenarioNames() []string {
	names := make([]string, 0, len(checkers()))
	for name := range checkers() {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// LabBinaries declares the one binary this lab stages into its Docker build
// context, which is what test/interop-pppoe/Dockerfile.ze copies in.
//
// The base is ze_core alone. This lab runs the daemon and asserts nothing from
// inside the container, so it needs neither the ze_distro plugin mode nor the
// harness that le carries. The producer adds every gate feature-gates.txt declares.
func LabBinaries() []interoplab.LabBinary {
	return []interoplab.LabBinary{
		{Name: "ze", Base: "ze_core", Output: "test/interop-pppoe/ze-linux"},
	}
}

// imageBuilds declares the three images this suite builds. None of them sets a
// Timeout, so each takes the machine build budget that BUILD_TIMEOUT names
// (`interoplab.Docker`). That field lengthens a bound for an image slower than
// the machine budget, and none of these three needs it: all three are one
// `apk add` on alpine plus a COPY, because the ze binary is cross-compiled on
// the host before the build rather than compiled inside the image.
func imageBuilds(root string) []interoplab.ImageBuild {
	directory := filepath.Join(root, suitePath)
	return []interoplab.ImageBuild{
		{
			Name:       zeImageName,
			Tag:        zeImageTag,
			Dockerfile: filepath.Join(directory, "Dockerfile.ze"),
			Context:    root,
			Required:   true,
		},
		{
			Name:       accelImageName,
			Tag:        accelImageTag,
			Dockerfile: filepath.Join(directory, "Dockerfile.accel"),
			Context:    directory,
			Required:   true,
		},
		{
			Name:       clientImageName,
			Tag:        clientImageTag,
			Dockerfile: filepath.Join(directory, "Dockerfile.client"),
			Context:    directory,
			Required:   true,
		},
	}
}

func preflight(suffix string) interoplab.PreflightCheck {
	return func(ctx context.Context, docker *interoplab.Docker) error {
		for _, key := range [...]string{
			"ZE_PPPOE_SKIP_KERNEL_PROBE",
			"ze.pppoe.skip-kernel-probe",
		} {
			if _, exists := os.LookupEnv(key); exists {
				return fmt.Errorf(
					"refusing to run with %s set; full proof must not skip the kernel probe",
					key,
				)
			}
		}

		result, err := docker.RunOneShot(ctx, preflightContainer(suffix))
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return errors.New("preflight probe container timed out")
			}
			return fmt.Errorf(
				"preflight probe failed (rc=%d): %s",
				result.ExitCode,
				strings.TrimSpace(result.Stderr),
			)
		}

		return validatePreflightOutput(result.Stdout)
	}
}

// preflightContainer probes the host kernel with the grants the lab's peers
// hold and nothing more, so it passes only where they can run: it loads no
// module, because a module it loaded would hide the host's missing setup.
func preflightContainer(suffix string) interoplab.OneShotContainer {
	var tb textbuf.Buffer
	containerName := tb.Str("ze-pppoe-preflight-").Str(suffix).String()
	arguments := append([]string{"--cap-add", netAdminCapability, "--name", containerName}, pppDeviceArguments()...)
	return interoplab.OneShotContainer{
		Image:     "alpine:3.21",
		Arguments: arguments,
		Command: []string{
			"sh",
			"-c",
			"echo DEV_PPP=$(test -c /dev/ppp && echo ok || echo missing); " +
				"echo PPPOE=$(test -d /sys/module/pppoe -o -f /proc/net/pppoe && echo ok || echo missing)",
		},
		Timeout: 120 * time.Second,
	}
}

func validatePreflightOutput(output string) error {
	checks := make(map[string]string, 2)
	for line := range strings.SplitSeq(output, "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if found {
			checks[key] = value
		}
	}
	missing := make([]string, 0, 2)
	if checks["DEV_PPP"] != "ok" {
		missing = append(missing, "/dev/ppp (the PPP device, module ppp_generic)")
	}
	if checks["PPPOE"] != "ok" {
		missing = append(missing, "PPPoE sockets (module pppoe)")
	}
	if len(missing) != 0 {
		return fmt.Errorf("the host kernel cannot run the PPPoE lab: it lacks %s.\n"+
			"Lab containers never load kernel modules, so the host must provide them.\n"+
			"Load them on the Docker host, then run the lab again:\n"+
			"  sudo modprobe -a ppp_generic pppoe\n", strings.Join(missing, ", "))
	}
	return nil
}

func scenarioPlan(
	source interoplab.ScenarioSource,
	suffix string,
) (interoplab.ScenarioPlan, error) {
	containers := containerNames(suffix)
	role, err := readRole(source.Directory)
	if err != nil {
		return interoplab.ScenarioPlan{}, err
	}
	var peers []interoplab.PeerConfig
	if role == roleZeAC {
		peers, err = prepareZeAccessConcentrator(source, containers)
	} else {
		peers, err = prepareZeClient(source, containers)
	}
	if err != nil {
		return interoplab.ScenarioPlan{}, err
	}
	var tb textbuf.Buffer
	networkName := tb.Str("ze-pppoe-").Str(suffix).String()
	return interoplab.ScenarioPlan{
		Source: source,
		Network: interoplab.NetworkSpec{
			Name: networkName,
			Candidates: []interoplab.Subnet{
				{IPv4: netip.MustParsePrefix("172.30.0.0/24")},
			},
		},
		Peers:      peers,
		Containers: []string{containers.ze, containers.accel, containers.client},
	}, nil
}
