// Design: docs/guide/bmp.md -- router identity in BMP Initiation.
// Related: sender.go -- sendInitiation; bmp.go -- system config lifecycle.
// RFC 7854 Section 4.4 and RFC 1213 Section 6.3 define these identity values.

package bmp

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/core/version"
)

// systemNameConfig keeps an explicit empty name distinct from an absent host.
// RFC 1213 permits a zero-length sysName for an unconfigured object.
type systemNameConfig struct {
	Host   *string `json:"host"`
	Domain string  `json:"domain"`
}

type systemIdentity struct {
	name        string
	description string
}

func parseSystemNameConfig(data string) (*systemNameConfig, error) {
	var section struct {
		System *systemNameConfig `json:"system"`
	}
	if err := json.Unmarshal([]byte(data), &section); err != nil {
		return nil, fmt.Errorf("bmp system identity: %w", err)
	}
	if section.System == nil {
		return &systemNameConfig{}, nil
	}
	if section.System.Host != nil {
		host := system.ExpandEnvValue(*section.System.Host)
		section.System.Host = &host
	}
	section.System.Domain = system.ExpandEnvValue(section.System.Domain)
	return section.System, nil
}

// setSystemName publishes immutable configuration. The caller MUST NOT change
// config after this call; reconnecting senders read it under mu.
func (bp *BMPPlugin) setSystemName(config *systemNameConfig) {
	bp.mu.Lock()
	bp.systemName = config
	bp.mu.Unlock()
}

func (bp *BMPPlugin) readSystemIdentity() (systemIdentity, error) {
	bp.mu.RLock()
	config := bp.systemName
	bp.mu.RUnlock()
	return readSystemIdentity(config, os.Hostname, unix.Uname)
}

func defaultSystemIdentity() (systemIdentity, error) {
	return readSystemIdentity(nil, os.Hostname, unix.Uname)
}

// readSystemIdentity uses the router's administrative name, the running kernel,
// and the same software build identity as the version command. There is no SNMP
// object provider in Ze; these sources define its RFC 1213 system identity.
func readSystemIdentity(config *systemNameConfig, hostname func() (string, error), uname func(*unix.Utsname) error) (systemIdentity, error) {
	var id systemIdentity
	var host *string
	var domain string
	if config != nil {
		host = config.Host
		domain = config.Domain
	}
	if host != nil {
		id.name = *host
	} else {
		name, err := hostname()
		if err != nil {
			return id, fmt.Errorf("bmp system hostname: %w", err)
		}
		id.name = name
	}
	if domain != "" {
		if id.name != "" {
			// An already-qualified name must not acquire the domain twice.
			if !strings.Contains(id.name, ".") {
				id.name += "." + domain
			}
		}
	}
	var kernel unix.Utsname
	if err := uname(&kernel); err != nil {
		return systemIdentity{}, fmt.Errorf("bmp system description: %w", err)
	}
	var description textbuf.Buffer
	id.description = description.Str(version.Short()).Str("; ").
		Str(unix.ByteSliceToString(kernel.Sysname[:])).Byte(' ').
		Str(unix.ByteSliceToString(kernel.Release[:])).Byte(' ').
		Str(unix.ByteSliceToString(kernel.Machine[:])).String()
	if err := checkSystemIdentityText(id.name); err != nil {
		return systemIdentity{}, fmt.Errorf("bmp sysName: %w", err)
	}
	if err := checkSystemIdentityText(id.description); err != nil {
		return systemIdentity{}, fmt.Errorf("bmp sysDescr: %w", err)
	}
	return id, nil
}

// RFC 1213 Section 6.3: "It is mandatory that this only contain printable
// ASCII characters." Both objects have DisplayString SIZE (0..255).
func checkSystemIdentityText(value string) error {
	if len(value) > 255 {
		return fmt.Errorf("system identity exceeds 255 bytes")
	}
	for i := range len(value) {
		if value[i] < 0x20 {
			return fmt.Errorf("system identity contains a non-printable ASCII byte")
		}
		if value[i] > 0x7e {
			return fmt.Errorf("system identity contains a non-printable ASCII byte")
		}
	}
	return nil
}
