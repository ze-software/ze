// Design: docs/architecture/cli/plugin-modes.md — ze provision: PXE/DHCP remote provisioning

package provision

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/rescueauth"

	"golang.org/x/crypto/bcrypt"
)

func Run(args []string) int {
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		usage()
		return 0
	}

	fs := flag.NewFlagSet("provision", flag.ContinueOnError)

	iface := fs.String("interface", "", "Network interface for provisioning")
	network := fs.String("network", "", "Provisioning network CIDR (e.g. 10.0.0.0/24)")
	image := fs.String("image", "", "Path to gokrazy disk image")
	sshUser := fs.String("ssh-username", "", "Admin username for installed target")
	sshPass := fs.String("ssh-password", "", "Admin password for installed target (bcrypt-hashed before use)")
	address := fs.String("address", "", "Override server IP (default: first IPv4 on interface)")
	kernel := fs.String("kernel", "", "Path to installer kernel (staged to boot directory)")
	initrd := fs.String("initrd", "", "Path to installer initrd (staged to boot directory)")
	pxeDir := fs.String("pxe-dir", defaultPXEDir, "PXE serve root, resolved against the working directory unless absolute: boot files served from <dir>/boot, TFTP from <dir>/tftp")

	fs.Usage = func() { usage() }

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if errs := validateFlags(*iface, *network, *image, *sshUser, *sshPass, *pxeDir); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "error: %s\n", e)
		}
		return 1
	}

	serverIP, addedCIDR, ipErr := resolveOrConfigureIP(*iface, *address, *network)
	if addedCIDR != "" {
		defer cleanupAddress(*iface, addedCIDR)
	}
	if ipErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", ipErr)
		return 1
	}

	bootDir, tftpDir := pxeDirs(*pxeDir)

	ipxeDir := locateIPXEDir()
	sc := stagingConfig{
		KernelPath: *kernel,
		InitrdPath: *initrd,
		IPXEDir:    ipxeDir,
		BootDir:    bootDir,
		TFTPDir:    tftpDir,
	}
	if stageErr := stageArtifacts(sc); stageErr != nil {
		fmt.Fprintf(os.Stderr, "error: staging: %v\n", stageErr)
		return 1
	}

	if valErr := validateStaging(sc); valErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", valErr)
		return 1
	}

	hash, hashErr := hashPassword(*sshPass)
	if hashErr != nil {
		fmt.Fprintf(os.Stderr, "error: hashing password: %v\n", hashErr)
		return 1
	}

	// The rescue credential is a dedicated random token, never the admin
	// password: it is published on the installer kernel cmdline over an
	// unauthenticated PXE network, so whatever it commits to is effectively
	// public. Losing it costs a rescue shell on a machine that already failed to
	// install; it must never cost the admin password.
	rescueToken, rescueAuthValue, rescueErr := rescueauth.NewValue()
	if rescueErr != nil {
		fmt.Fprintf(os.Stderr, "error: generating rescue token: %v\n", rescueErr)
		return 1
	}
	printRescueToken(os.Stderr, rescueToken)

	bootScriptURL := "http://" + serverIP + "/install/boot/boot.ipxe"

	cfg := generateConfig(configParams{
		iface:         *iface,
		network:       *network,
		image:         *image,
		serverIP:      serverIP,
		sshUsername:   *sshUser,
		sshPassHash:   hash,
		rescueAuth:    rescueAuthValue,
		bootScriptURL: bootScriptURL,
		bootDir:       bootDir,
		tftpDir:       tftpDir,
	})

	return forkAndServe(cfg)
}

type configParams struct {
	iface         string
	network       string
	image         string
	serverIP      string
	sshUsername   string
	sshPassHash   string
	rescueAuth    string
	bootScriptURL string
	bootDir       string
	tftpDir       string
}

func generateConfig(p configParams) string {
	prefix, _ := netip.ParsePrefix(p.network)
	start, stop := dhcpRange(prefix, netip.MustParseAddr(p.serverIP))

	tftpRoot := p.tftpDir
	if tftpRoot == "" {
		tftpRoot = defaultTFTPDir
	}
	bootRoot := p.bootDir
	if bootRoot == "" {
		bootRoot = defaultBootDir
	}

	var b strings.Builder

	b.WriteString("service {\n")

	b.WriteString("    dhcp-server {\n")
	b.WriteString("        enabled true;\n")
	b.WriteString("        listen-interface ")
	b.WriteString(p.iface)
	b.WriteString(";\n")
	b.WriteString("        shared-network install {\n")
	b.WriteString("            subnet ")
	b.WriteString(p.network)
	b.WriteString(" {\n")
	b.WriteString("                range pool1 {\n")
	b.WriteString("                    start ")
	b.WriteString(start.String())
	b.WriteString(";\n")
	b.WriteString("                    stop ")
	b.WriteString(stop.String())
	b.WriteString(";\n")
	b.WriteString("                }\n")
	b.WriteString("                default-router ")
	b.WriteString(p.serverIP)
	b.WriteString(";\n")
	b.WriteString("            }\n")
	b.WriteString("        }\n")
	b.WriteString("        pxe {\n")
	b.WriteString("            enabled true;\n")
	b.WriteString("            tftp-server ")
	b.WriteString(p.serverIP)
	b.WriteString(";\n")
	b.WriteString("            bootfile-bios ipxe.pxe;\n")
	b.WriteString("            bootfile-uefi ipxe.efi;\n")
	if p.bootScriptURL != "" {
		b.WriteString("            boot-script-url ")
		b.WriteString(p.bootScriptURL)
		b.WriteString(";\n")
	}
	b.WriteString("        }\n")
	b.WriteString("    }\n")

	b.WriteString("    tftp-server {\n")
	b.WriteString("        enabled true;\n")
	b.WriteString("        listen-interface ")
	b.WriteString(p.iface)
	b.WriteString(";\n")
	b.WriteString("        root-directory ")
	b.WriteString(tftpRoot)
	b.WriteString(";\n")
	b.WriteString("    }\n")

	b.WriteString("    image-server {\n")
	b.WriteString("        enabled true;\n")
	b.WriteString("        listen-interface ")
	b.WriteString(p.iface)
	b.WriteString(";\n")
	b.WriteString("        image-directory ")
	b.WriteString(dirOf(p.image))
	b.WriteString(";\n")
	b.WriteString("        boot-directory ")
	b.WriteString(bootRoot)
	b.WriteString(";\n")
	b.WriteString("        ssh-username ")
	b.WriteString(p.sshUsername)
	b.WriteString(";\n")
	b.WriteString("        ssh-password-hash \"")
	b.WriteString(p.sshPassHash)
	b.WriteString("\";\n")
	b.WriteString("        rescue-auth \"")
	b.WriteString(p.rescueAuth)
	b.WriteString("\";\n")
	b.WriteString("    }\n")

	b.WriteString("}\n")

	return b.String()
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}

func validateFlags(iface, network, image, sshUser, sshPass, pxeDir string) []error {
	var errs []error

	if bootDir, tftpDir := pxeDirs(pxeDir); !safeConfigValue(bootDir) || !safeConfigValue(tftpDir) {
		// Name the resolved path, not just the flag: the default build/pxe is
		// resolved against the working directory, so an unsafe character can come
		// from the checkout path even when the operator passed no --pxe-dir.
		errs = append(errs, fmt.Errorf("--pxe-dir %q resolves to %q, which contains characters unsafe for the generated config (no spaces, ';', '{', '}'); run from a checkout path without them or pass --pxe-dir an absolute clean path", pxeDir, bootDir))
	}

	if iface == "" {
		errs = append(errs, errors.New("missing required flag: --interface"))
	} else if !safeConfigValue(iface) {
		errs = append(errs, errors.New("invalid interface name: contains forbidden characters"))
	}

	if network == "" {
		errs = append(errs, errors.New("missing required flag: --network"))
	} else {
		prefix, err := netip.ParsePrefix(network)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid --network: %w", err))
		} else {
			bits := prefix.Bits()
			if prefix.Addr().Is4() {
				if bits < 8 || bits > 30 {
					errs = append(errs, fmt.Errorf("invalid --network: prefix /%d out of range /8../30", bits))
				}
			}
		}
	}

	if image == "" {
		errs = append(errs, errors.New("missing required flag: --image"))
	} else {
		info, err := os.Stat(image)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("--image: %w", err))
		case !info.Mode().IsRegular():
			errs = append(errs, fmt.Errorf("--image: %s is not a regular file", image))
		case !safeConfigValue(dirOf(image)):
			errs = append(errs, errors.New("--image: directory path contains forbidden characters"))
		}
	}

	if sshUser == "" {
		errs = append(errs, errors.New("missing required flag: --ssh-username"))
	} else if !safeConfigValue(sshUser) {
		errs = append(errs, errors.New("invalid --ssh-username: contains forbidden characters"))
	}

	if sshPass == "" {
		errs = append(errs, errors.New("missing required flag: --ssh-password"))
	}

	if iface != "" && safeConfigValue(iface) {
		_, err := net.InterfaceByName(iface)
		if err != nil {
			errs = append(errs, fmt.Errorf("--interface %s: %w", iface, err))
		}
	}

	return errs
}

func safeConfigValue(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		switch s[i] {
		case '{', '}', ';', '\n', '\r', '\t', ' ', 0:
			return false
		}
	}
	return true
}

func resolveServerIP(iface, override string) (string, error) {
	if override != "" {
		addr, err := netip.ParseAddr(override)
		if err != nil {
			return "", fmt.Errorf("invalid --address: %w", err)
		}
		if !addr.Is4() {
			return "", errors.New("--address must be IPv4")
		}
		return addr.String(), nil
	}

	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		return "", fmt.Errorf("interface %s: %w", iface, err)
	}

	addrs, err := ifi.Addrs()
	if err != nil {
		return "", fmt.Errorf("interface %s addresses: %w", iface, err)
	}

	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip4 := ipNet.IP.To4()
		if ip4 != nil {
			return ip4.String(), nil
		}
	}

	return "", fmt.Errorf("interface %s has no IPv4 address", iface)
}

// printRescueToken shows the operator the one-time rescue token. It is written
// to w (stderr) and never stored: only the salted argon2id of it reaches config
// or the kernel cmdline, so this is the only chance to record it.
func printRescueToken(w io.Writer, token string) {
	fmt.Fprintf(w, "\nrescue token: %s\n", token)                                                    //nolint:errcheck // console notice; a failed write must not abort provisioning
	fmt.Fprintf(w, "  needed to open a rescue shell if an install fails; not recoverable later\n\n") //nolint:errcheck // console notice
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func dhcpRange(prefix netip.Prefix, serverIP netip.Addr) (start, stop netip.Addr) {
	masked := prefix.Masked()
	network := masked.Addr()
	bits := masked.Bits()

	hostBits := 32 - bits
	totalHosts := uint32(1) << hostBits

	networkU32 := addrToU32(network)
	firstHost := networkU32 + 1
	lastHost := networkU32 + totalHosts - 2
	serverU32 := addrToU32(serverIP)

	poolStart := firstHost
	if poolStart == serverU32 {
		poolStart++
	}

	poolStop := lastHost
	if poolStop == serverU32 {
		poolStop--
	}

	return u32ToAddr(poolStart), u32ToAddr(poolStop)
}

func addrToU32(a netip.Addr) uint32 {
	b := a.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

func u32ToAddr(v uint32) netip.Addr {
	return netip.AddrFrom4([4]byte{
		byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v),
	})
}

// withInstallLogDefaults raises the install servers to info-level logging for a
// provisioning session so the operator sees DHCP leases, TFTP/HTTP downloads,
// and the served image, unless they already chose a level. An explicit global
// ze.log is respected as-is; otherwise the three install subsystems default to
// info while everything else stays at the normal warn level.
func withInstallLogDefaults(environ []string) []string {
	if hasEnvKey(environ, "ze.log") {
		return environ
	}
	out := append([]string(nil), environ...)
	for _, kv := range []struct{ key, assign string }{
		{"ze.log.dhcpserver", "ze.log.dhcpserver=info"},
		{"ze.log.tftpserver", "ze.log.tftpserver=info"},
		{"ze.log.imageserver", "ze.log.imageserver=info"},
	} {
		if !hasEnvKey(out, kv.key) {
			out = append(out, kv.assign)
		}
	}
	return out
}

func hasEnvKey(environ []string, key string) bool {
	want := normalizeEnvKey(key)
	for _, e := range environ {
		name, _, ok := strings.Cut(e, "=")
		if ok && normalizeEnvKey(name) == want {
			return true
		}
	}
	return false
}

// normalizeEnvKey matches the env package: lookup is case-insensitive and treats
// dots and underscores as equivalent, so ze.log, ze_log and ZE_LOG are one key.
func normalizeEnvKey(k string) string {
	return strings.ToLower(strings.ReplaceAll(k, ".", "_"))
}

func forkAndServe(config string) int {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot find own binary: %v\n", err)
		return 1
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, self, "-") // #nosec G204 - self is our own binary
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = withInstallLogDefaults(os.Environ())

	stdin, pipeErr := cmd.StdinPipe()
	if pipeErr != nil {
		fmt.Fprintf(os.Stderr, "error: creating stdin pipe: %v\n", pipeErr)
		return 1
	}

	if startErr := cmd.Start(); startErr != nil {
		fmt.Fprintf(os.Stderr, "error: starting ze: %v\n", startErr)
		return 1
	}

	if _, writeErr := stdin.Write([]byte(config)); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: writing config to ze: %v\n", writeErr)
		killAndWait(cmd)
		return 1
	}
	if _, writeErr := stdin.Write([]byte{0}); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: writing config sentinel: %v\n", writeErr)
		killAndWait(cmd)
		return 1
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigCh
		if closeErr := stdin.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: closing stdin pipe: %v\n", closeErr)
		}
		if sigErr := cmd.Process.Signal(sig); sigErr != nil {
			fmt.Fprintf(os.Stderr, "warning: forwarding signal: %v\n", sigErr)
		}
	}()

	if waitErr := cmd.Wait(); waitErr != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](waitErr); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "error: ze exited: %v\n", waitErr)
		return 1
	}

	return 0
}

func killAndWait(cmd *exec.Cmd) {
	if killErr := cmd.Process.Kill(); killErr != nil {
		fmt.Fprintf(os.Stderr, "warning: killing ze: %v\n", killErr)
	}
	if waitErr := cmd.Wait(); waitErr != nil {
		fmt.Fprintf(os.Stderr, "warning: waiting for ze: %v\n", waitErr)
	}
}

func usage() {
	p := helpfmt.Page{
		Command: "ze provision",
		Summary: "Start DHCP+PXE, TFTP, and HTTP provisioning servers (requires root)",
		Usage:   []string{"ze provision --interface <name> --network <cidr> --image <path> --ssh-username <user> --ssh-password <pass>"},
		Sections: []helpfmt.HelpSection{
			{Title: "Required flags", Entries: []helpfmt.HelpEntry{
				{Name: "--interface", Desc: "Network interface for provisioning"},
				{Name: "--network", Desc: "Provisioning network CIDR (e.g. 10.0.0.0/24)"},
				{Name: "--image", Desc: "Path to gokrazy disk image"},
				{Name: "--ssh-username", Desc: "Admin username for installed target"},
				{Name: "--ssh-password", Desc: "Admin password (bcrypt-hashed before embedding in config)"},
			}},
			{Title: "Optional flags", Entries: []helpfmt.HelpEntry{
				{Name: "--address", Desc: "Override server IP (default: first IPv4 on --interface)"},
				{Name: "--kernel", Desc: "Path to installer kernel (copied to boot directory)"},
				{Name: "--initrd", Desc: "Path to installer initrd (copied to boot directory)"},
				{Name: "--pxe-dir", Desc: "PXE serve root, resolved against the working directory unless absolute; boot files under <dir>/boot, TFTP under <dir>/tftp (default: build/pxe, where make ze-pxe-build stages from the repo root)"},
			}},
		},
		Examples: []string{
			"ze provision --interface eth0 --network 10.0.0.0/24 --image /path/to/gokrazy.img --ssh-username admin --ssh-password secret",
		},
	}
	p.WriteErr()
}
