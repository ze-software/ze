// Design: docs/architecture/cli/plugin-modes.md — ze provision: PXE/DHCP remote provisioning

package provision

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"

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

	fs.Usage = func() { usage() }

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if errs := validateFlags(*iface, *network, *image, *sshUser, *sshPass); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "error: %s\n", e)
		}
		return 1
	}

	serverIP, ipErr := resolveServerIP(*iface, *address)
	if ipErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", ipErr)
		return 1
	}

	ipxeDir := locateIPXEDir()
	sc := stagingConfig{
		KernelPath: *kernel,
		InitrdPath: *initrd,
		IPXEDir:    ipxeDir,
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

	bootScriptURL := "http://" + serverIP + "/install/boot/boot.ipxe"

	cfg := generateConfig(configParams{
		iface:         *iface,
		network:       *network,
		image:         *image,
		serverIP:      serverIP,
		sshUsername:   *sshUser,
		sshPassHash:   hash,
		bootScriptURL: bootScriptURL,
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
	bootScriptURL string
}

func generateConfig(p configParams) string {
	prefix, _ := netip.ParsePrefix(p.network)
	start, stop := dhcpRange(prefix, netip.MustParseAddr(p.serverIP))

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
	b.WriteString("        root-directory /var/lib/ze/install/tftp;\n")
	b.WriteString("    }\n")

	b.WriteString("    image-server {\n")
	b.WriteString("        enabled true;\n")
	b.WriteString("        listen-interface ")
	b.WriteString(p.iface)
	b.WriteString(";\n")
	b.WriteString("        image-directory ")
	b.WriteString(dirOf(p.image))
	b.WriteString(";\n")
	b.WriteString("        boot-directory /var/lib/ze/install/boot;\n")
	b.WriteString("        ssh-username ")
	b.WriteString(p.sshUsername)
	b.WriteString(";\n")
	b.WriteString("        ssh-password-hash \"")
	b.WriteString(p.sshPassHash)
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

func validateFlags(iface, network, image, sshUser, sshPass string) []error {
	var errs []error

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
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
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
			}},
		},
		Examples: []string{
			"ze provision --interface eth0 --network 10.0.0.0/24 --image /path/to/gokrazy.img --ssh-username admin --ssh-password secret",
		},
	}
	p.Write()
}
