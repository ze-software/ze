// Package config allows tools such as the gokr-packer or breakglass reading
// gokrazy instance configuration (with fallback to the older host-specific
// configuration) for data such as the HTTP password, or package-specific
// command line flags.
package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gokrazy/internal/instanceflag"
)

// InternalCompatibilityFlags keep older gokr-packer behavior or user interface
// working, but should never be set by users in config.json manually.
type InternalCompatibilityFlags struct {
	// These have become 'gok overwrite' flags:
	Overwrite          string `json:",omitempty"` // -overwrite
	OverwriteBoot      string `json:",omitempty"` // -overwrite_boot
	OverwriteMBR       string `json:",omitempty"` // -overwrite_mbr
	OverwriteRoot      string `json:",omitempty"` // -overwrite_root
	Sudo               string `json:",omitempty"` // -sudo
	TargetStorageBytes int    `json:",omitempty"` // -target_storage_bytes

	// These have become 'gok update' flags:
	Update   string `json:",omitempty"` // -update
	Insecure bool   `json:",omitempty"` // -insecure
	Testboot bool   `json:",omitempty"` // -testboot

	// These will likely not be carried over from gokr-packer to gok because of
	// a lack of usage.
	InitPkg       string `json:",omitempty"` // -init_pkg
	OverwriteInit string `json:",omitempty"` // -overwrite_init
}

type UpdateStruct struct {
	// Hostname (in UpdateStruct) overrides Struct.Hostname, but only for
	// deploying the update via HTTP, not in the generated image.
	Hostname string `json:",omitempty"`

	// UseTLS can be one of:
	//
	// - empty (""), meaning use TLS if certificates exist
	// - "off", disabling TLS even if certificates exist
	// - "self-signed", creating TLS certificates if needed
	UseTLS string `json:",omitempty"` // -tls

	// NoPassword, if true, prevents any password from being
	// included in the image. Without a password, the only access
	// will be over the serial console and the web interface will
	// only accept connections from localhost.
	//
	// If the appliance image wishes for the gokrazy web interface
	// and API to be accessible, it is then the responsible of a
	// package in the image to make it available, supplying
	// authentication, authorization, and proxying to localhost as
	// desired. For example, Tailscale might be used.
	NoPassword bool `json:",omitempty"`

	HTTPPort     string `json:",omitempty"` // -http_port
	HTTPSPort    string `json:",omitempty"` // -https_port
	HTTPPassword string `json:",omitempty"` // http-password.txt
	CertPEM      string `json:",omitempty"` // cert.pem
	KeyPEM       string `json:",omitempty"` // key.pem
}

func (u *UpdateStruct) WithFallbackToHostSpecific(host string) (*UpdateStruct, error) {
	if u == nil {
		u = &UpdateStruct{}
	}
	result := UpdateStruct{
		Hostname:   u.Hostname,
		NoPassword: u.NoPassword,
	}

	if u.HTTPPort != "" {
		result.HTTPPort = u.HTTPPort
	} else {
		port, err := HostnameSpecific(host).ReadFile("http-port.txt")
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		result.HTTPPort = port
	}

	if u.HTTPSPort != "" {
		result.HTTPSPort = u.HTTPSPort
	} else {
		// There was no extra file for the HTTPS port (http-port.txt was used).
		result.HTTPSPort = result.HTTPPort
	}

	if !u.NoPassword {
		if u.HTTPPassword != "" {
			result.HTTPPassword = u.HTTPPassword
		} else {
			pw, err := HostnameSpecific(host).ReadFile("http-password.txt")
			if err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			result.HTTPPassword = pw
		}
	}

	// Intentionally no fallback for CertPEM and KeyPEM at this stage: their
	// fallback happens conditionally if UseTLS != off, in
	// tlsflag.CertificatePathsFor().

	return &result, nil
}

type PackageConfig struct {
	// --------------------------------------------------------------------------------
	// build time package configuration
	// --------------------------------------------------------------------------------

	// GoBuildEnvironment contains key=value pairs, like in Go’s os.Environ(),
	// that will be applied when running “go build”. In contrast to the
	// Struct.Environment field, this setting is specific to a package.
	GoBuildEnvironment []string `json:",omitempty"`

	// GoBuildFlags will be passed to “go build” as extra arguments.
	//
	// To pass build tags, do not use -tags=mycustomtag; instead set the
	// GoBuildTags field to not overwrite the gokrazy default build tags.
	GoBuildFlags []string `json:",omitempty"`

	// GoBuildTags will be added to the list of gokrazy default build tags.
	GoBuildTags []string `json:",omitempty"`

	// ExtraFilePaths maps from root file system destination path to a relative
	// or absolute path on the host on which the packer is running.
	//
	// Lookup order:
	// 1. <path>_<target_goarch>.tar
	// 2. <path>.tar
	// 3. <path> (directory)
	ExtraFilePaths map[string]string `json:",omitempty"`

	// ExtraFileContents maps from root file system destination path to the
	// plain text contents of the file.
	ExtraFileContents map[string]string `json:",omitempty"`

	// Basename overrides the basename of the package.
	Basename string `json:",omitempty"`

	// --------------------------------------------------------------------------------
	// run time package configuration
	// --------------------------------------------------------------------------------

	// Environment contains key=value pairs, like in Go’s os.Environ().
	Environment []string `json:",omitempty"`

	// CommandLineFlags will be set when starting the program.
	CommandLineFlags []string `json:",omitempty"`

	// DontStart makes the gokrazy init not start this program
	// automatically. Users can still start it manually via the web interface,
	// or interactively via breakglass.
	DontStart bool `json:",omitempty"`

	// WaitForClock makes the gokrazy init wait for clock synchronization before
	// starting the program. This is useful when modifying the program source to
	// call gokrazy.WaitForClock() is inconvenient.
	WaitForClock bool `json:",omitempty"`
}

// MountDevice instructs gokrazy to mount the specified source on the specified
// target using the specified file system type and the specified options.
//
// If Target contains a directory in /mnt (not a subdirectory), the packer will
// create it on the (read-only) root file system.
type MountDevice struct {
	Source  string // e.g. /dev/sdx or PARTUUID=abcdef
	Type    string // e.g. ext4
	Target  string // e.g. /mnt/usb, will be created by the packer
	Options string // a subset of mount(8)-like options
}

// Fields where we need to distinguish between not being set (= use the default)
// and being set to an empty value (= disable), such as FirmwarePackage, are
// pointers. Fields that are required (Hostname) or where the empty value is not
// valid (InternalCompatibilityFlags.Sudo) don’t need to be pointers. If needed,
// we can switch fields from non-pointer to pointer, as the JSON on-disk
// representation does not change, and the config package is in
// github.com/gokrazy/internal.
type Struct struct {
	Hostname   string        // -hostname
	DeviceType string        `json:",omitempty"` // -device_type
	Update     *UpdateStruct `json:",omitempty"`

	// Environment contains key=value pairs, like in Go’s os.Environ(),
	// and is meant to be used for setting GOOS and GOARCH of an instance.
	Environment []string `json:",omitempty"`

	Packages []string // flag.Args()

	// If PackageConfig is specified, all package config is taken from the
	// config struct, no longer from the file system, except for extrafiles/.
	PackageConfig map[string]PackageConfig `json:",omitempty"`

	SerialConsole string `json:",omitempty"`

	GokrazyPackages *[]string `json:",omitempty"` // -gokrazy_pkgs
	KernelPackage   *string   `json:",omitempty"` // -kernel_package
	FirmwarePackage *string   `json:",omitempty"` // -firmware_package
	EEPROMPackage   *string   `json:",omitempty"` // -eeprom_package

	// extra arguments to append to the kernel command line (space separated):
	// https://www.kernel.org/doc/html/latest/admin-guide/kernel-parameters.html
	KernelExtraArgs []string `json:",omitempty"`

	// extra lines to append to the config.txt file on the boot partition which
	// is read by the Raspberry Pi bootloader:
	// https://www.raspberrypi.com/documentation/computers/config_txt.html
	BootloaderExtraLines []string `json:",omitempty"`

	// extra lines to append to the bootconf.txt EEPROM section which is read by
	// the Raspberry Pi bootloader:
	// https://www.raspberrypi.com/documentation/computers/raspberry-pi.html#raspberry-pi-bootloader-configuration
	BootloaderExtraEEPROM []string `json:",omitempty"`

	MountDevices []MountDevice `json:",omitempty"`

	// Do not set these manually in config.json, these fields only exist so that
	// the entire old gokr-packer flag surface keeps working.
	InternalCompatibilityFlags *InternalCompatibilityFlags `json:",omitempty"`

	Meta struct {
		Instance     string
		Path         string
		LastModified time.Time
	} `json:"-"` // omit from JSON
}

// NewStruct returns a config.Struct that was not loaded from a file, but
// instead created empty, with only the hostname field set.
//
// This is handy for best-effort compatibility for older setups (before instance
// config was introduced). Aside from compatibility, ReadFromFile() should be
// used instead of NewStruct().
func NewStruct(hostname string) *Struct {
	return &Struct{
		Hostname:                   hostname,
		Update:                     &UpdateStruct{},
		InternalCompatibilityFlags: &InternalCompatibilityFlags{},
	}
}

func (s *Struct) SerialConsoleOrDefault() string {
	if s.SerialConsole == "" {
		return "serial0,115200"
	}
	return s.SerialConsole
}

func (s *Struct) GokrazyPackagesOrDefault() []string {
	if s.GokrazyPackages == nil {
		return []string{
			"github.com/gokrazy/gokrazy/cmd/dhcp",
			"github.com/gokrazy/gokrazy/cmd/ntp",
			"github.com/gokrazy/gokrazy/cmd/randomd",
			"github.com/gokrazy/gokrazy/cmd/heartbeat",
		}
	}
	return *s.GokrazyPackages
}

func (s *Struct) KernelPackageOrDefault() string {
	if s.KernelPackage == nil {
		// KernelPackage unspecified, fall back to the default.
		return "github.com/gokrazy/kernel.rpi"
	}
	return *s.KernelPackage
}

func (s *Struct) FirmwarePackageOrDefault() string {
	if s.FirmwarePackage == nil {
		// FirmwarePackage unspecified, fall back to the default.
		return "github.com/gokrazy/firmware"
	}
	return *s.FirmwarePackage
}

func (s *Struct) EEPROMPackageOrDefault() string {
	if s.EEPROMPackage == nil {
		// EEPROMPackage unspecified, fall back to the default.
		return "github.com/gokrazy/rpi-eeprom"
	}
	return *s.EEPROMPackage
}

func (s *Struct) ApplyEnvironment() {
	for _, kv := range s.Environment {
		// validated in ReadFromFile()
		key, value, _ := strings.Cut(kv, "=")
		os.Setenv(key, value)
	}
}

// FormatForFile pretty-prints the config struct as JSON, ready for storing it
// in the config.json file.
func (s *Struct) FormatForFile() ([]byte, error) {
	b, err := json.MarshalIndent(s, "", "    ")
	if err != nil {
		return nil, err
	}
	b = append(b, '\n')
	return b, nil
}

func (i *InternalCompatibilityFlags) SudoOrDefault() string {
	if i.Sudo == "" {
		return "auto"
	}

	return i.Sudo
}

func InstancePath() string {
	return filepath.Join(instanceflag.ParentDir(), instanceflag.Instance())
}

func InstanceConfigPath() string {
	return filepath.Join(InstancePath(), "config.json")
}

// ApplyInstanceFlag reads the config from InstanceConfigPath(),
// applies the configured environment and returns the config struct.
func ApplyInstanceFlag() (*Struct, error) {
	configJSON := InstanceConfigPath()
	cfg, err := ReadFromFile(configJSON)
	if err != nil {
		return nil, err
	}
	cfg.ApplyEnvironment()
	return cfg, nil
}

func ReadFromFile(fn string) (*Struct, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	var cfg Struct
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("decoding %s: %v", fn, err)
	}
	if err := validate(&cfg); err != nil {
		return nil, err
	}
	if cfg.Update == nil {
		cfg.Update = &UpdateStruct{}
	}
	if cfg.InternalCompatibilityFlags == nil {
		cfg.InternalCompatibilityFlags = &InternalCompatibilityFlags{}
	}
	cfg.Meta.Instance = instanceflag.Instance()
	cfg.Meta.Path = fn
	cfg.Meta.LastModified = st.ModTime()
	return &cfg, nil
}

func validate(cfg *Struct) error {
	for _, kv := range cfg.Environment {
		if _, _, ok := strings.Cut(kv, "="); !ok {
			return fmt.Errorf("malformed Environment entry %q, expected key=value", kv)
		}
	}
	return nil
}
