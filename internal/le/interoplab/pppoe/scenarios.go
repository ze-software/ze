// Design: docs/architecture/testing/interop.md -- role-selected Docker topology for both PPPoE scenarios.
// Related: pppoe.go -- suite discovery, images, preflight, and network lifecycle.
package pppoe

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

type names struct {
	ze     string
	accel  string
	client string
}

func containerNames(suffix string) names {
	var tb textbuf.Buffer
	return names{
		ze:     tb.Str("ze-pppoe-ze-").Str(suffix).String(),
		accel:  tb.Reset().Str("ze-pppoe-accel-").Str(suffix).String(),
		client: tb.Reset().Str("ze-pppoe-client-").Str(suffix).String(),
	}
}

func prepareZeClient(source interoplab.ScenarioSource, containers names) ([]interoplab.PeerConfig, error) {
	accelConfig, err := requiredScenarioFile(source, "accel-ppp.conf")
	if err != nil {
		return nil, err
	}
	chapSecrets, err := requiredScenarioFile(source, "chap-secrets")
	if err != nil {
		return nil, err
	}
	zeConfig, err := requiredScenarioFile(source, "ze.conf")
	if err != nil {
		return nil, err
	}

	accelMounts := []interoplab.Mount{
		{Source: accelConfig, Target: "/etc/accel-ppp.conf", ReadOnly: true},
		{Source: chapSecrets, Target: "/etc/accel-ppp/chap-secrets", ReadOnly: true},
	}
	// Keep /etc/ze container-owned and writable; only the input file is mounted.
	zeMounts := []interoplab.Mount{
		{Source: zeConfig, Target: zeConfigPath, ReadOnly: true},
	}

	accelArguments := interoplab.PPPDeviceArguments()
	zeArguments := append(interoplab.PPPDeviceArguments(), "-e", "ze.log.interface=debug")
	zeAppArmorProfile := ""
	ipv6, err := ipv6SessionScenario(source)
	if err != nil {
		return nil, err
	}
	if ipv6 {
		accelArguments = append(accelArguments, ipv6SessionArguments()...)
		zeArguments = append(zeArguments, ipv6SessionArguments()...)
		// The client writes accept_ra, autoconf and forwarding on the session
		// device each time a session creates it, so the writes happen at run
		// time and cannot be --sysctl arguments (apparmor_netsysctl.go).
		zeArguments = append(zeArguments, interoplab.NetSysctlWriteArguments()...)
		zeAppArmorProfile = interoplab.NetSysctlAppArmorProfileName
	}

	return []interoplab.PeerConfig{
		{
			Name:         accelImageName,
			Container:    containers.accel,
			Image:        accelImageTag,
			Host:         accelHost,
			Mounts:       accelMounts,
			Capabilities: []string{netAdminCapability},
			Arguments:    accelArguments,
			Ready: &interoplab.ReadyProbe{
				Command:  []string{"accel-cmd", commandShow, "stat"},
				Timeout:  60 * time.Second,
				Interval: time.Second,
			},
		},
		{
			Name:            zeImageName,
			Container:       containers.ze,
			Image:           zeImageTag,
			Host:            zeHost,
			Mounts:          zeMounts,
			Capabilities:    []string{netAdminCapability},
			Arguments:       zeArguments,
			AppArmorProfile: zeAppArmorProfile,
			Command:         []string{"start", zeConfigPath},
		},
	}, nil
}

// ipv6SessionMarker names the scenario file whose presence says the session
// negotiates IPv6CP. Its content is a comment for the reader; only its
// presence is read.
const ipv6SessionMarker = "ipv6-session"

// ipv6SessionScenario reports whether the scenario carries ipv6SessionMarker,
// and refuses a marker it cannot read or that is a directory.
func ipv6SessionScenario(source interoplab.ScenarioSource) (bool, error) {
	info, err := os.Stat(filepath.Join(source.Directory, ipv6SessionMarker))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read %s in %s: %w", ipv6SessionMarker, source.Name, err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("%s in %s is a directory: write it as a file", ipv6SessionMarker, source.Name)
	}
	return true, nil
}

// ipv6SessionArguments keeps IPv6 enabled on the PPP device each peer creates
// at run time. A PPP device takes its disable_ipv6 from the namespace default
// when the kernel creates it, and Docker may disable IPv6 in a container on
// a network that carries none, which would leave the device without the
// link-local address IPv6CP negotiates and without Router Advertisements.
// The default is set once before the daemon starts, inside the container's
// own network namespace, so it is a --sysctl argument and needs no write
// grant.
func ipv6SessionArguments() []string {
	return []string{"--sysctl", "net.ipv6.conf.default.disable_ipv6=0"}
}

func prepareZeAccessConcentrator(
	source interoplab.ScenarioSource,
	containers names,
) ([]interoplab.PeerConfig, error) {
	zeConfig, err := requiredScenarioFile(source, "ze.conf")
	if err != nil {
		return nil, err
	}
	// Keep /etc/ze container-owned and writable; only the input file is mounted.
	zeMounts := []interoplab.Mount{
		{Source: zeConfig, Target: zeConfigPath, ReadOnly: true},
	}

	return []interoplab.PeerConfig{
		{
			Name:         zeImageName,
			Container:    containers.ze,
			Image:        zeImageTag,
			Host:         zeHost,
			Mounts:       zeMounts,
			Capabilities: []string{netAdminCapability},
			Arguments: append(interoplab.PPPDeviceArguments(),
				"-e",
				"ze.log.pppoe=debug",
				"-e",
				"ze.log.l2tp=debug",
			),
			Command: []string{"start", zeConfigPath},
		},
		{
			Name:         clientImageName,
			Container:    containers.client,
			Image:        clientImageTag,
			Host:         clientHost,
			Capabilities: []string{netAdminCapability},
			Arguments:    interoplab.PPPDeviceArguments(),
		},
	}, nil
}

func requiredScenarioFile(source interoplab.ScenarioSource, name string) (string, error) {
	path := filepath.Join(source.Directory, name)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("missing %s in %s", name, source.Name)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return absolute, nil
}

func readRole(directory string) (string, error) {
	path := filepath.Join(directory, "role")
	content, err := os.ReadFile(path) // #nosec G304 -- path is the fixed role file inside a discovered, repository-owned PPPoE scenario fixture.
	name := filepath.Base(filepath.Clean(directory))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf(
				"missing role file in %s: write %s or %s into it",
				name,
				roleZeClient,
				roleZeAC,
			)
		}
		return "", err
	}
	role := strings.TrimSpace(string(content))
	if role != roleZeClient && role != roleZeAC {
		return "", fmt.Errorf(
			"unknown role %q in %s: expected %s or %s",
			role,
			name,
			roleZeClient,
			roleZeAC,
		)
	}
	return role, nil
}
