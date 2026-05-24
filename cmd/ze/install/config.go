// Design: plan/spec-install-0-umbrella.md — config generation for ze install remote

package install

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type configParams struct {
	iface       string
	network     string
	image       string
	serverIP    string
	sshUsername string
	sshPassHash string
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
