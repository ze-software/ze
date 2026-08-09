// Design: docs/architecture/appliance/on-device-installer.md -- kernel cmdline parsing for on-device installer

package disk

import (
	"os"
	"strings"
)

// InstallConfig holds parsed kernel cmdline parameters for the installer.
type InstallConfig struct {
	Source     string // http or iso
	Server     string // ze-install server IP (http mode)
	Image      string // image filename
	Port       string // install HTTP port
	Target     string // explicit target disk path
	Wait       string // max server probe attempts
	MediaID    string // ISO media identifier (32 hex chars)
	Mac        string // boot NIC MAC for pinning (ze.mac)
	RescueAuth string // salted argon2id of the rescue token (ze.rescue-auth)
}

func defaultConfig() InstallConfig {
	return InstallConfig{
		Source: "http",
		Image:  "ze.img",
		Port:   "80",
		Wait:   "30",
	}
}

// parseCmdlineString parses ze.* parameters from a kernel cmdline string.
func parseCmdlineString(line string) InstallConfig {
	cfg := defaultConfig()
	for param := range strings.FieldsSeq(line) {
		k, v, ok := strings.Cut(param, "=")
		if !ok {
			continue
		}
		switch k {
		case "ze.source":
			cfg.Source = v
		case "ze.server":
			cfg.Server = v
		case "ze.image":
			cfg.Image = v
		case "ze.port":
			cfg.Port = v
		case "ze.target":
			cfg.Target = v
		case "ze.wait":
			cfg.Wait = v
		case "ze.media-id":
			cfg.MediaID = v
		case "ze.mac":
			cfg.Mac = v
		case "ze.rescue-auth":
			cfg.RescueAuth = v
		}
	}
	return cfg
}

// parseCmdline reads /proc/cmdline and parses ze.* parameters.
func parseCmdline() InstallConfig {
	data, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return defaultConfig()
	}
	return parseCmdlineString(strings.TrimSpace(string(data)))
}
