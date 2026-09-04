// Design: docs/architecture/testing/interop.md -- native accel-ppp and pppd Docker interop gate.
// Related: scenarios.go -- typed container plans for both PPPoE roles.
// Related: check_client.go -- Ze client assertions against accel-ppp.
// Related: check_ac.go -- Ze access-concentrator assertions against pppd.
package pppoe

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

	commandShow        = "show"
	modulesPath        = "/lib/modules"
	privilegedArgument = "--privileged"
	zeConfigPath       = "/etc/ze/ze.conf"
)

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
		Docker:    docker,
		Preflight: preflight(options.Suffix),
		Images:    imageBuilds(root),
		Scenarios: plans,
		NoBuild:   options.NoBuild,
	}
	return suite.Run(ctx)
}

func setupFailure(err error) interoplab.SuiteReport {
	return interoplab.SuiteReport{SetupError: err.Error(), Code: 1}
}

func checkers() map[string]interoplab.Checker {
	return map[string]interoplab.Checker{
		"01-pppoe-chap-ipv4":   checkZeClient,
		"02-ze-ac-pppd-client": checkZeAccessConcentrator,
	}
}

// ScenarioNames returns every typed PPPoE scenario in lexical selection order.
func ScenarioNames() []string {
	names := make([]string, 0, len(checkers()))
	for name := range checkers() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// imageBuilds declares the three images this suite builds. None of them sets a
// Timeout, so each takes the machine build budget that BUILD_TIMEOUT names
// (`interoplab.Docker`). That field lengthens a bound for an image slower than
// the machine budget, and none of these three is: the accel and client images
// are one `apk add` on alpine, and Dockerfile.ze copies the whole tree and
// compiles ze, which is the build the machine budget was measured on.
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

		var tb textbuf.Buffer
		containerName := tb.Str("ze-pppoe-preflight-").Str(suffix).String()
		arguments := []string{privilegedArgument, "--name", containerName}
		if directoryExists(modulesPath) {
			mount := tb.Reset().Str(modulesPath).Byte(':').Str(modulesPath).Str(":ro").String()
			arguments = append(arguments, "-v", mount)
		}
		result, err := docker.RunOneShot(ctx, interoplab.OneShotContainer{
			Image:     "alpine:3.21",
			Arguments: arguments,
			Command: []string{
				"sh",
				"-c",
				"apk add --no-cache -q kmod > /dev/null 2>&1 && " +
					"modprobe ppp_generic 2>/dev/null; " +
					"modprobe pppoe 2>/dev/null; " +
					"echo DEV_PPP=$(test -c /dev/ppp && echo ok || echo missing); " +
					"echo PPPOE=$(test -d /sys/module/pppoe -o -f /proc/net/pppoe && echo ok || echo missing)",
			},
			Timeout: 120 * time.Second,
		})
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
		missing = append(missing, "/dev/ppp (PPP character device)")
	}
	if checks["PPPOE"] != "ok" {
		missing = append(missing, "pppoe (PPPoE pppox kernel module)")
	}
	if len(missing) != 0 {
		return fmt.Errorf("host kernel missing PPPoE requirements: %s", strings.Join(missing, ", "))
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

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
