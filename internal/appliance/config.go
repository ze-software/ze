// Design: plan/learned/675-appliance-1-builder.md — appliance config structs and validation

package appliance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/textbuf"
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
	// Hugepages, when set, reserves hugepages on the target at boot via the
	// kernel cmdline (default_hugepagesz/hugepagesz/hugepages). nil = no
	// reservation, and the built image's /cmdline.txt is unchanged.
	Hugepages *Hugepages `json:"hugepages,omitempty"`
	// Memory is the target's total RAM as a byte-size string (e.g. "8gb").
	// Optional; when set it bounds the hugepage reservation (which may not
	// exceed 50% of it) and drives the QEMU `-m` size for `ze appliance run`.
	// Empty = unset (run defaults to 512 MiB and only the static 512 GiB
	// reservation ceiling applies).
	Memory string `json:"memory,omitempty"`
}

// Hugepages describes a boot-time hugepage reservation as a total size reserved
// in pages of a given size. Both are byte-size strings (e.g. Size "1gb",
// PageSize "2mb"); the reserved page count is Size / PageSize.
type Hugepages struct {
	Size     string `json:"size"`      // total reservation, e.g. "1gb"
	PageSize string `json:"page-size"` // "2mb" or "1gb"
}

type QEMUConfig struct {
	SSHPort     int `json:"ssh-port"`
	WebPort     int `json:"web-port"`
	GokrazyPort int `json:"gokrazy-port"`
}

type applianceConfig struct {
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
)

var validNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

func DefaultConfig(name string) applianceConfig {
	return applianceConfig{
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
			KernelProfile: defaultKernelProfile,
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
	// maxHugepageTotalBytes is the static ceiling on a boot-time hugepage
	// reservation when image.memory-bytes is not declared (512 GiB).
	maxHugepageTotalBytes int64 = 512 * 1024 * 1024 * 1024
	minMemoryBytes        int64 = 256 * 1024 * 1024         // 256 MiB
	maxMemoryBytes        int64 = 1024 * 1024 * 1024 * 1024 // 1 TiB
)

// parseByteSize parses a byte-size string ("10b", "512mb", "8gb", "1tb";
// case-insensitive, 1024-based) into a byte count. A unit suffix is required.
func parseByteSize(s string) (int64, error) {
	t := strings.ToLower(strings.TrimSpace(s))
	var mult int64
	var num string
	switch {
	case strings.HasSuffix(t, "tb"):
		mult, num = 1<<40, strings.TrimSuffix(t, "tb")
	case strings.HasSuffix(t, "gb"):
		mult, num = 1<<30, strings.TrimSuffix(t, "gb")
	case strings.HasSuffix(t, "mb"):
		mult, num = 1<<20, strings.TrimSuffix(t, "mb")
	case strings.HasSuffix(t, "kb"):
		mult, num = 1<<10, strings.TrimSuffix(t, "kb")
	case strings.HasSuffix(t, "b"):
		mult, num = 1, strings.TrimSuffix(t, "b")
	default:
		return 0, fmt.Errorf("size %q: must end in b, kb, mb, gb, or tb", s)
	}
	n, err := strconv.ParseInt(strings.TrimSpace(num), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("size %q: not a number", s)
	}
	if n < 0 {
		return 0, fmt.Errorf("size %q: must not be negative", s)
	}
	if n != 0 && n > (int64(1)<<62)/mult {
		return 0, fmt.Errorf("size %q: too large", s)
	}
	return n * mult, nil
}

// hugepageToken maps a supported hugepage byte size to the kernel cmdline token
// (2M or 1G) and whether it is a supported size.
func hugepageToken(pageSizeBytes int64) (string, bool) {
	switch pageSizeBytes {
	case 2 * 1024 * 1024:
		return "2M", true
	case 1024 * 1024 * 1024:
		return "1G", true
	default:
		return "", false
	}
}

// pageSizeBytes parses PageSize and requires a kernel-supported hugepage size.
func (h Hugepages) pageSizeBytes() (int64, error) {
	b, err := parseByteSize(h.PageSize)
	if err != nil {
		return 0, fmt.Errorf("image.hugepages.page-size %w", err)
	}
	if _, ok := hugepageToken(b); !ok {
		return 0, fmt.Errorf("image.hugepages.page-size %q: must be 2mb or 1gb", h.PageSize)
	}
	return b, nil
}

// pageCount returns the number of pages to reserve (Size / PageSize); Size must
// be a positive whole multiple of PageSize.
func (h Hugepages) pageCount() (int64, error) {
	ps, err := h.pageSizeBytes()
	if err != nil {
		return 0, err
	}
	total, err := parseByteSize(h.Size)
	if err != nil {
		return 0, fmt.Errorf("image.hugepages.size %w", err)
	}
	if total < ps {
		return 0, fmt.Errorf("image.hugepages.size %q: must be at least one %q page", h.Size, h.PageSize)
	}
	if total%ps != 0 {
		return 0, fmt.Errorf("image.hugepages.size %q: must be a whole multiple of page-size %q", h.Size, h.PageSize)
	}
	return total / ps, nil
}

// memoryBytes parses Memory; ok is false when Memory is unset.
func (i ImageConfig) memoryBytes() (memBytes int64, ok bool, err error) {
	if strings.TrimSpace(i.Memory) == "" {
		return 0, false, nil
	}
	b, perr := parseByteSize(i.Memory)
	if perr != nil {
		return 0, false, fmt.Errorf("image.memory %w", perr)
	}
	return b, true, nil
}

func (c *applianceConfig) Validate() error {
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
	if c.Image.KernelProfile != "" {
		if err := validateKernelProfileName(c.Image.KernelProfile); err != nil {
			return fmt.Errorf("image.kernel-profile %q: must match %s", c.Image.KernelProfile, validKernelProfileRe.String())
		}
	}
	if c.QEMU.SSHPort != 0 {
		if c.QEMU.SSHPort < 1024 || c.QEMU.SSHPort > 65535 {
			return fmt.Errorf("qemu.ssh-port %d: must be 1024-65535", c.QEMU.SSHPort)
		}
	}
	if err := c.validateImageMemory(); err != nil {
		return err
	}
	return nil
}

// validateImageMemory bounds image.memory and image.hugepages. Both are
// optional; when present they gate the boot-time hugepage reservation so it
// cannot starve the target into OOM (R-2).
func (c *applianceConfig) validateImageMemory() error {
	memBytes, memSet, err := c.Image.memoryBytes()
	if err != nil {
		return err
	}
	if memSet {
		if memBytes < minMemoryBytes {
			return fmt.Errorf("image.memory %q: minimum is 256mb", c.Image.Memory)
		}
		if memBytes > maxMemoryBytes {
			return fmt.Errorf("image.memory %q: maximum is 1tb", c.Image.Memory)
		}
	}
	if c.Image.Hugepages == nil {
		return nil
	}
	pageBytes, err := c.Image.Hugepages.pageSizeBytes()
	if err != nil {
		return err
	}
	count, err := c.Image.Hugepages.pageCount()
	if err != nil {
		return err
	}
	// Check the page count against the static ceiling before multiplying, so a
	// huge count cannot overflow int64 in the total computation.
	if count > maxHugepageTotalBytes/pageBytes {
		return fmt.Errorf("image.hugepages.size %q: exceeds the 512 GiB reservation ceiling", c.Image.Hugepages.Size)
	}
	total := count * pageBytes
	if memSet && total > memBytes/2 {
		return fmt.Errorf("image.hugepages.size %q: exceeds 50%% of image.memory %q", c.Image.Hugepages.Size, c.Image.Memory)
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

func LoadConfig(path string) (*applianceConfig, error) {
	data, err := cliio.ReadFile(path) // "-" reads stdin
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var cfg applianceConfig
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}

func saveConfig(path string, cfg *applianceConfig) error {
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
