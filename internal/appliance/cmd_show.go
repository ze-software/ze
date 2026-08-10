// Design: docs/architecture/appliance/builder.md -- show appliance config and cert expiry

package appliance

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var errNoPemBlock = errors.New("no PEM block")

func init() {
	cmdShow = runShow
}

func runShow(args []string) int {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: ze appliance show <name>\n")
		return exitError
	}
	name := args[0]
	dir := getBaseDir()

	cfg, err := LoadConfig(ConfigPath(dir, name))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	fmt.Printf("name:       %s\n", cfg.Identity.Name)
	fmt.Printf("hostname:   %s\n", cfg.Identity.Hostname)
	fmt.Printf("username:   %s\n", cfg.Credentials.Username)
	fmt.Printf("ssh:        %s:%s\n", cfg.SSH.Host, cfg.SSH.Port)
	if cfg.Web.Enabled {
		fmt.Printf("web:        %s:%s\n", cfg.Web.Host, cfg.Web.Port)
	} else {
		fmt.Printf("web:        disabled\n")
	}
	fmt.Printf("arch:       %s\n", cfg.Image.Arch)
	fmt.Printf("image-size: %d bytes\n", cfg.Image.SizeBytes)
	fmt.Printf("encrypted:  %v\n", isEncrypted(dir, name))

	if cfg.Managed {
		fmt.Printf("managed:    yes (fleet mode: accepts remote config push, reports to hub)\n")
	} else {
		fmt.Printf("managed:    no\n")
	}

	if !cfg.Credentials.AdminEnabled {
		fmt.Printf("admin:      disabled (RADIUS-only, serial console recovery only)\n")
	}

	if cfg.ConfigBase != "" {
		fmt.Printf("config-base: %s\n", cfg.ConfigBase)
	}

	certPath := filepath.Join(tLSDir(dir, name), "cert.pem")
	if expiry, certErr := certExpiry(certPath); certErr == nil {
		remaining := time.Until(expiry).Truncate(24 * time.Hour)
		fmt.Printf("expires:    %s (%s remaining)\n", expiry.Format("2006-01-02"), remaining)
	}

	return exitOK
}

func certExpiry(certPath string) (time.Time, error) {
	data, err := os.ReadFile(certPath) //nolint:gosec // appliance file
	if err != nil {
		return time.Time{}, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return time.Time{}, errNoPemBlock
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, err
	}
	return cert.NotAfter, nil
}
