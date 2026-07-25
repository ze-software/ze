// Design: plan/learned/675-appliance-1-builder.md — ZeFS assembly with config layering

package appliance

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

func init() {
	cmdAssemble = runAssemble
}

func runAssemble(args []string) int {
	fs := flag.NewFlagSet("appliance assemble", flag.ContinueOnError)
	keepFlag := fs.Bool("keep", false, "Retain database.zefs after assembly (contains plaintext secrets)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze appliance assemble [options] <name>\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}
	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: requires <name>\n")
		fs.Usage()
		return exitError
	}

	name := fs.Arg(0)
	dir := getBaseDir()

	cfg, err := LoadConfig(ConfigPath(dir, name))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	var passphrase []byte
	if IsEncrypted(dir, name) {
		var resolveErr error
		passphrase, _, resolveErr = ResolvePassphrase(nil)
		if resolveErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", resolveErr)
			return exitError
		}
		defer ZeroBytes(passphrase)
	}

	dbPath := DatabasePath(dir, name)
	if code := assembleZeFS(dir, name, cfg, passphrase, dbPath); code != exitOK {
		return code
	}

	if !*keepFlag {
		os.Remove(dbPath) //nolint:errcheck // best-effort
		fmt.Fprintf(os.Stderr, "database.zefs assembled (contains plaintext secrets, auto-deleted)\n")
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: database.zefs retained (contains plaintext secrets, delete when done)\n")
	}

	return exitOK
}

func assembleZeFS(baseDir, name string, cfg *applianceConfig, passphrase []byte, dbPath string) int {
	passwordHash, err := ReadSecret(secretFilePath(baseDir, name, "password.hash"), passphrase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read password hash: %v\n", err)
		return exitError
	}
	defer ZeroBytes(passwordHash)

	certPEM, err := os.ReadFile(filepath.Join(TLSDir(baseDir, name), "cert.pem")) //nolint:gosec // appliance secret
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read cert: %v\n", err)
		return exitError
	}

	keyPEM, err := ReadSecret(filepath.Join(TLSDir(baseDir, name), "key.pem"), passphrase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read key: %v\n", err)
		return exitError
	}
	defer ZeroBytes(keyPEM)

	seedConfig, err := resolveSeedConfig(baseDir, name, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	store, err := zefs.Create(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: create database: %v\n", err)
		return exitError
	}

	managedValue := "false"
	if cfg.Managed {
		managedValue = "true"
	}

	entries := []struct{ key, value string }{
		{zefs.KeySSHUsername.Key(cfg.SSH.Host, cfg.SSH.Port), cfg.Credentials.Username},
		{zefs.KeySSHPassword.Key(cfg.SSH.Host, cfg.SSH.Port), string(passwordHash)},
		{zefs.KeyLocalAdminUsername.Pattern, cfg.Credentials.Username},
		{zefs.KeyLocalAdminPassword.Pattern, string(passwordHash)},
		{zefs.KeySSHDefault.Pattern, func() string { var tb textbuf.Buffer; return tb.Str(cfg.SSH.Host).Byte('/').Str(cfg.SSH.Port).String() }()},
		{zefs.KeyInstanceName.Pattern, cfg.Identity.Name},
		{zefs.KeyInstanceManaged.Pattern, managedValue},
		{zefs.KeyWebCert.Pattern, string(certPEM)},
		{zefs.KeyWebKey.Pattern, string(keyPEM)},
	}

	if seedConfig != "" {
		entries = append(entries,
			struct{ key, value string }{zefs.KeyFileTemplate.Key("ze.conf"), seedConfig},
			struct{ key, value string }{zefs.KeyConfigLastKnownGood.Pattern, ConfigHash(seedConfig)},
		)
	}

	if !cfg.Credentials.AdminEnabled {
		entries = append(entries, struct{ key, value string }{
			zefs.KeyInstanceAdminDisabled.Pattern, "true",
		})
	}

	authKeysPath := secretFilePath(baseDir, name, "authorized_keys")
	if authData, readErr := os.ReadFile(authKeysPath); readErr == nil { //nolint:gosec // appliance file
		entries = append(entries, struct{ key, value string }{
			zefs.KeySSHAuthorizedKeys.Pattern, string(authData),
		})
	}

	for _, e := range entries {
		if writeErr := store.WriteFile(e.key, []byte(e.value), 0); writeErr != nil {
			fmt.Fprintf(os.Stderr, "error: write %s: %v\n", e.key, writeErr)
			store.Close()     //nolint:errcheck // cleanup
			os.Remove(dbPath) //nolint:errcheck // cleanup
			return exitError
		}
	}

	if err := store.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "error: close database: %v\n", err)
		os.Remove(dbPath) //nolint:errcheck // cleanup
		return exitError
	}

	return exitOK
}

func resolveSeedConfig(baseDir, name string, cfg *applianceConfig) (string, error) {
	appDir := AppliancePath(baseDir, name)
	var base, overlay string

	if cfg.ConfigBase != "" {
		var basePath string
		if filepath.IsAbs(cfg.ConfigBase) {
			basePath = cfg.ConfigBase
		} else {
			basePath = filepath.Join(appDir, cfg.ConfigBase)
		}
		data, err := os.ReadFile(basePath) //nolint:gosec // user-configured path
		if err != nil {
			return "", fmt.Errorf("read config-base %s: %w", cfg.ConfigBase, err)
		}
		base = string(data)
	}

	overlayPath := filepath.Join(appDir, "ze.conf")
	if data, err := os.ReadFile(overlayPath); err == nil { //nolint:gosec // appliance file
		overlay = string(data)
	}

	var seed string
	switch {
	case base == "" && overlay == "":
		defaultPath := filepath.Join("gokrazy", "ze", "ze.conf")
		if data, err := os.ReadFile(defaultPath); err == nil { //nolint:gosec // source tree default
			seed = string(data)
		}
	case overlay != "" && base != "":
		var tb textbuf.Buffer
		seed = tb.Str(base).Byte('\n').Str(overlay).String()
	case base != "":
		seed = base
	default:
		seed = overlay
	}

	if seed == "" {
		return "", nil
	}

	return appendListenerOverrides(seed, cfg), nil
}

func appendListenerOverrides(seed string, cfg *applianceConfig) string {
	var tb textbuf.Buffer
	tb.Str(seed)
	if cfg.SSH.Host != "" {
		tb.Byte('\n').Str("set environment ssh server default ip ").Str(cfg.SSH.Host)
	}
	if cfg.SSH.Port != "" {
		tb.Byte('\n').Str("set environment ssh server default port ").Str(cfg.SSH.Port)
	}
	if cfg.Web.Enabled {
		if cfg.Web.Host != "" {
			tb.Byte('\n').Str("set environment web server default ip ").Str(cfg.Web.Host)
		}
		if cfg.Web.Port != "" {
			tb.Byte('\n').Str("set environment web server default port ").Str(cfg.Web.Port)
		}
	}
	tb.Byte('\n')
	return tb.String()
}
