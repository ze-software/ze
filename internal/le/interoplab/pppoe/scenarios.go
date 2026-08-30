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
	zeMounts := []interoplab.Mount{
		{Source: zeConfig, Target: zeConfigPath, ReadOnly: true},
	}
	if directoryExists(modulesPath) {
		modules := interoplab.Mount{
			Source:   modulesPath,
			Target:   modulesPath,
			ReadOnly: true,
		}
		accelMounts = append(accelMounts, modules)
		zeMounts = append(zeMounts, modules)
	}

	return []interoplab.PeerConfig{
		{
			Name:      accelImageName,
			Container: containers.accel,
			Image:     accelImageName,
			Host:      accelHost,
			Mounts:    accelMounts,
			Arguments: []string{privilegedArgument},
			Ready: &interoplab.ReadyProbe{
				Command:  []string{"accel-cmd", commandShow, "stat"},
				Timeout:  60 * time.Second,
				Interval: time.Second,
			},
		},
		{
			Name:      zeImageName,
			Container: containers.ze,
			Image:     zeImageName,
			Host:      zeHost,
			Mounts:    zeMounts,
			Arguments: []string{
				privilegedArgument,
				"-e",
				"ze.log.interface=debug",
				"-e",
				"ZE_STORAGE_BLOB=false",
			},
			Command: []string{"start", zeConfigPath},
		},
	}, nil
}

func prepareZeAccessConcentrator(
	source interoplab.ScenarioSource,
	containers names,
) ([]interoplab.PeerConfig, error) {
	zeConfig, err := requiredScenarioFile(source, "ze.conf")
	if err != nil {
		return nil, err
	}
	zeMounts := []interoplab.Mount{
		{Source: zeConfig, Target: zeConfigPath, ReadOnly: true},
	}
	clientMounts := make([]interoplab.Mount, 0, 1)
	if directoryExists(modulesPath) {
		modules := interoplab.Mount{
			Source:   modulesPath,
			Target:   modulesPath,
			ReadOnly: true,
		}
		zeMounts = append(zeMounts, modules)
		clientMounts = append(clientMounts, modules)
	}

	return []interoplab.PeerConfig{
		{
			Name:      zeImageName,
			Container: containers.ze,
			Image:     zeImageName,
			Host:      zeHost,
			Mounts:    zeMounts,
			Arguments: []string{
				privilegedArgument,
				"-e",
				"ze.log.pppoe=debug",
				"-e",
				"ze.log.l2tp=debug",
				"-e",
				"ZE_STORAGE_BLOB=false",
			},
			Command: []string{"start", zeConfigPath},
		},
		{
			Name:      clientImageName,
			Container: containers.client,
			Image:     clientImageName,
			Host:      clientHost,
			Mounts:    clientMounts,
			Arguments: []string{privilegedArgument},
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
