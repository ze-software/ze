// Design: plan/learned/675-appliance-1-builder.md — ApplianceConfig struct and validation

package appliance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

var errIdentityNameIsRequired = errors.New("identity.name is required")

type IdentityConfig struct {
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
}

type CredentialsConfig struct {
	Username          string   `json:"username"`
	AdminEnabled      bool     `json:"admin-enabled"`
	SSHAuthorizedKeys []string `json:"ssh-authorized-keys,omitempty"`
}

type SSHConfig struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

type WebConfig struct {
	Enabled bool   `json:"enabled"`
	Host    string `json:"host"`
	Port    string `json:"port"`
}

type TLSConfig struct {
	CertName      string `json:"cert-name"`
	CertFile      string `json:"cert-file,omitempty"`
	KeyFile       string `json:"key-file,omitempty"`
	ValidityYears int    `json:"validity-years"`
}

type DeviceConfig struct {
	Address    string `json:"address,omitempty"`
	UpdatePort int    `json:"update-port"`
}

type ImageConfig struct {
	Arch          string `json:"arch"`
	SizeBytes     int64  `json:"size-bytes"`
	KernelProfile string `json:"kernel-profile"`
}

type QEMUConfig struct {
	SSHPort     int `json:"ssh-port"`
	WebPort     int `json:"web-port"`
	GokrazyPort int `json:"gokrazy-port"`
}

type ApplianceConfig struct {
	Identity    IdentityConfig    `json:"identity"`
	Credentials CredentialsConfig `json:"credentials"`
	SSH         SSHConfig         `json:"ssh"`
	Web         WebConfig         `json:"web"`
	TLS         TLSConfig         `json:"tls"`
	Managed     bool              `json:"managed"`
	Device      DeviceConfig      `json:"device"`
	ConfigBase  string            `json:"config-base,omitempty"`
	Image       ImageConfig       `json:"image"`
	QEMU        QEMUConfig        `json:"qemu"`
}

const (
	maxNameLen      = 64
	archAMD64       = "amd64"
	archARM64       = "arm64"
	defaultUsername = "admin"

	ProfileQEMU     = "qemu"
	ProfileHardware = "hardware"
)

var validNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

func DefaultConfig(name string) ApplianceConfig {
	return ApplianceConfig{
		Identity: IdentityConfig{
			Name:     name,
			Hostname: name,
		},
		Credentials: CredentialsConfig{
			Username:     defaultUsername,
			AdminEnabled: true,
		},
		SSH: SSHConfig{
			Host: "0.0.0.0",
			Port: "22",
		},
		Web: WebConfig{
			Enabled: true,
			Host:    "0.0.0.0",
			Port:    "8080",
		},
		TLS: TLSConfig{
			ValidityYears: 10,
		},
		Device: DeviceConfig{
			UpdatePort: 443,
		},
		Image: ImageConfig{
			Arch:          archAMD64,
			SizeBytes:     2 * 1024 * 1024 * 1024, // 2 GiB
			KernelProfile: ProfileQEMU,
		},
		QEMU: QEMUConfig{
			SSHPort:     2222,
			WebPort:     28080,
			GokrazyPort: 18080,
		},
	}
}

const (
	minImageSize int64 = 512 * 1024 * 1024
	maxImageSize int64 = 64 * 1024 * 1024 * 1024
)

func (c *ApplianceConfig) Validate() error {
	if c.Identity.Name == "" {
		return errIdentityNameIsRequired
	}
	if len(c.Identity.Name) > maxNameLen {
		return fmt.Errorf("identity.name: maximum length is %d characters", maxNameLen)
	}
	if !validNameRe.MatchString(c.Identity.Name) {
		return fmt.Errorf("identity.name %q: must match [a-zA-Z0-9][a-zA-Z0-9._-]*", c.Identity.Name)
	}
	if err := validatePort("ssh.port", c.SSH.Port); err != nil {
		return err
	}
	if err := validatePort("web.port", c.Web.Port); err != nil {
		return err
	}
	if c.Image.SizeBytes < minImageSize {
		return fmt.Errorf("image.size-bytes %d: minimum is %d (512 MiB)", c.Image.SizeBytes, minImageSize)
	}
	if c.Image.SizeBytes > maxImageSize {
		return fmt.Errorf("image.size-bytes %d: maximum is %d (64 GiB)", c.Image.SizeBytes, maxImageSize)
	}
	if c.TLS.ValidityYears < 1 {
		return fmt.Errorf("tls.validity-years %d: minimum is 1", c.TLS.ValidityYears)
	}
	if c.TLS.ValidityYears > 25 {
		return fmt.Errorf("tls.validity-years %d: maximum is 25", c.TLS.ValidityYears)
	}
	if c.Image.Arch != archAMD64 && c.Image.Arch != archARM64 {
		return fmt.Errorf("image.arch %q: must be amd64 or arm64", c.Image.Arch)
	}
	if c.Image.KernelProfile != "" && c.Image.KernelProfile != ProfileQEMU && c.Image.KernelProfile != ProfileHardware {
		return fmt.Errorf("image.kernel-profile %q: must be qemu or hardware", c.Image.KernelProfile)
	}
	if c.QEMU.SSHPort != 0 {
		if c.QEMU.SSHPort < 1024 || c.QEMU.SSHPort > 65535 {
			return fmt.Errorf("qemu.ssh-port %d: must be 1024-65535", c.QEMU.SSHPort)
		}
	}
	return nil
}

func validatePort(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	port, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("%s %q: not a valid port number", field, value)
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s %d: must be 1-65535", field, port)
	}
	return nil
}

func LoadConfig(path string) (*ApplianceConfig, error) {
	data, err := os.ReadFile(path) //nolint:gosec // user-provided path
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var cfg ApplianceConfig
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}

func SaveConfig(path string, cfg *ApplianceConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')
	var tb textbuf.Buffer
	tmpPath := tb.Str(path).Str(".tmp").String()
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil { //nolint:gosec // config file, not secrets
		return fmt.Errorf("write %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}
