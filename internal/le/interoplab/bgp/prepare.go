// Design: docs/architecture/testing/interop.md -- rendered, isolated Docker labs.
// Related: run.go -- suite construction and immutable image references.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	baseIPv4Prefix = "172.30.0."
	baseIPv6Prefix = "fd00:1e:0::"
	stayRTRPort    = 9847
)

const zeCLIConfig = `
system {
	authentication {
		user interop {
			password "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO"
		}
	}
}

environment {
	ssh {
		enabled true
		server main {
			ip 127.0.0.1;
			port 2222;
		}
	}
}
`

var containerRoles = []string{
	"ze", peerFRR, peerBIRD, peerGoBGP, peerBMP, peerRPKI, peerInject, peerSpeaker,
	peerSpeaker2, peerKeepalived, peerStayRTR,
}

func scenarioPlans(root, producer, suffix string, sources []interoplab.ScenarioSource) ([]interoplab.ScenarioPlan, error) {
	candidates, err := subnetCandidates()
	if err != nil {
		return nil, err
	}
	var name textbuf.Buffer
	networkName := name.Str("ze-iop-").Str(suffix).String()
	plans := make([]interoplab.ScenarioPlan, 0, len(sources))
	for _, source := range sources {
		dualStack, detectErr := needsIPv6(source.Directory)
		if detectErr != nil {
			return nil, detectErr
		}
		subnets := make([]interoplab.Subnet, len(candidates))
		for index, ipv4 := range candidates {
			subnets[index].IPv4 = ipv4
			if dualStack {
				subnets[index].IPv6 = ipv6For(ipv4)
			}
		}
		containers := make([]string, 0, len(containerRoles))
		for _, role := range containerRoles {
			containers = append(containers, containerName(role, suffix))
		}
		sourceCopy := source
		plans = append(plans, interoplab.ScenarioPlan{
			Source:     sourceCopy,
			Network:    interoplab.NetworkSpec{Name: networkName, Candidates: subnets},
			Containers: containers,
			Prepare: func(_ context.Context, prepare interoplab.PrepareContext) (interoplab.PreparedScenario, error) {
				return prepareScenario(root, producer, suffix, prepare)
			},
		})
	}
	return plans, nil
}

func subnetCandidates() ([]netip.Prefix, error) {
	if value := os.Getenv("ZE_INTEROP_SUBNET_PREFIX"); value != "" {
		prefix, err := parsePrefixToken(value)
		if err != nil {
			return nil, fmt.Errorf("invalid ZE_INTEROP_SUBNET_PREFIX %q: %w", value, err)
		}
		return []netip.Prefix{prefix}, nil
	}
	if value := os.Getenv("ZE_INTEROP_SUBNET_INDEX"); value != "" {
		index, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("invalid ZE_INTEROP_SUBNET_INDEX %q", value)
		}
		if index < 0 || index > 767 {
			return nil, errors.New("ZE_INTEROP_SUBNET_INDEX must be between 0 and 767")
		}
		pools := [][2]byte{{172, 30}, {172, 31}, {10, 254}}
		pool := pools[index/256]
		return []netip.Prefix{netip.PrefixFrom(netip.AddrFrom4([4]byte{pool[0], pool[1], byte(index % 256), 0}), 24)}, nil
	}
	candidates := make([]netip.Prefix, 0, 768)
	for _, pool := range [][2]byte{{172, 30}, {172, 31}, {10, 254}} {
		for third := range 256 {
			addr := netip.AddrFrom4([4]byte{pool[0], pool[1], byte(third), 0})
			candidates = append(candidates, netip.PrefixFrom(addr, 24))
		}
	}
	return candidates, nil
}

func parsePrefixToken(value string) (netip.Prefix, error) {
	value = strings.TrimSuffix(value, ".")
	var prefix textbuf.Buffer
	return netip.ParsePrefix(prefix.Str(value).Str(".0/24").Slice())
}

func ipv6For(ipv4 netip.Prefix) netip.Prefix {
	octets := ipv4.Addr().As4()
	addr := netip.AddrFrom16([16]byte{
		0xfd, 0x00,
		0x00, octets[1],
		0x00, octets[2],
	})
	return netip.PrefixFrom(addr, 64)
}

func needsIPv6(root string) (bool, error) {
	scenarioRoot, err := os.OpenRoot(root)
	if err != nil {
		return false, err
	}
	returnValue := false
	walkErr := fs.WalkDir(scenarioRoot.FS(), ".", func(relative string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, readErr := scenarioRoot.ReadFile(relative)
		if readErr != nil {
			return readErr
		}
		if !utf8.Valid(data) {
			return nil
		}
		text := string(data)
		if strings.Contains(text, baseIPv6Prefix) {
			returnValue = true
			return nil
		}
		if relative == "ze.conf" &&
			strings.Contains(text, "ospf") && strings.Contains(text, "address-family ipv6") {
			returnValue = true
		}
		return nil
	})
	return returnValue, errors.Join(walkErr, scenarioRoot.Close())
}

func prepareScenario(root, producer, suffix string, prepare interoplab.PrepareContext) (interoplab.PreparedScenario, error) {
	var name textbuf.Buffer
	renderedName := name.Str(prepare.Source.Name).Byte('-').Str(suffix).String()
	rendered := filepath.Join(root, "tmp", "interop-rendered", renderedName)
	if err := os.RemoveAll(rendered); err != nil {
		return interoplab.PreparedScenario{}, err
	}
	if err := renderScenario(prepare.Source.Directory, rendered, prepare.Network); err != nil {
		return interoplab.PreparedScenario{Cleanup: func() error { return os.RemoveAll(rendered) }}, err
	}
	peers, err := scenarioPeers(producer, rendered, suffix, prepare.Network)
	return interoplab.PreparedScenario{
		Peers: peers,
		Cleanup: func() error {
			return os.RemoveAll(rendered)
		},
	}, err
}

func renderScenario(source, target string, network interoplab.Network) error {
	ipv4 := network.IPv4.Addr().As4()
	ipv4Addr := netip.AddrFrom4([4]byte{ipv4[0], ipv4[1], ipv4[2], 0})
	var rendered textbuf.Buffer
	ipv4Token := strings.TrimSuffix(rendered.Addr(ipv4Addr).String(), "0")
	ipv6Token := ""
	if network.IPv6.IsValid() {
		ipv6Token = rendered.Reset().Str(
			strings.TrimSuffix(network.IPv6.Addr().String(), "::"),
		).Str("::").String()
	}
	sourceRoot, err := os.OpenRoot(source)
	if err != nil {
		return err
	}
	walkErr := fs.WalkDir(sourceRoot.FS(), ".", func(relative string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			targetRelative := filepath.FromSlash(relative)
			return os.MkdirAll(filepath.Join(target, targetRelative), 0o750)
		}
		data, readErr := sourceRoot.ReadFile(relative)
		if readErr != nil {
			return readErr
		}
		content := data
		if utf8.Valid(data) {
			text := strings.ReplaceAll(string(data), baseIPv4Prefix, ipv4Token)
			if ipv6Token != "" {
				text = strings.ReplaceAll(text, baseIPv6Prefix, ipv6Token)
			}
			if relative == "ze.conf" {
				text = rendered.Reset().Str(strings.TrimRight(text, "\n")).
					Byte('\n').Str(zeCLIConfig).String()
			}
			content = []byte(text)
		}
		targetPath := filepath.Join(target, filepath.FromSlash(relative))
		if writeErr := os.WriteFile(targetPath, content, 0o600); writeErr != nil {
			return writeErr
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		return os.Chmod(targetPath, info.Mode())
	})
	return errors.Join(walkErr, sourceRoot.Close())
}

func scenarioPeers(producer, scenario, suffix string, network interoplab.Network) ([]interoplab.PeerConfig, error) {
	if !regularFile(filepath.Join(scenario, "ze.conf")) {
		return nil, fmt.Errorf("missing ze.conf in %s", filepath.Base(scenario))
	}
	timeout := interoplab.ReadEnvironment(interoplab.EnvironmentOptions{}).SessionTimeout
	peers := make([]interoplab.PeerConfig, 0, len(containerRoles))
	var rendered textbuf.Buffer
	ready := func(command ...string) *interoplab.ReadyProbe {
		return &interoplab.ReadyProbe{Command: command, Timeout: 30 * time.Second, Interval: time.Second}
	}
	mount := func(source, target string) interoplab.Mount {
		return interoplab.Mount{Source: source, Target: target, ReadOnly: true}
	}

	if configContains(filepath.Join(scenario, "ze.conf"), "bmp {") {
		peers = append(peers, interoplab.PeerConfig{Name: peerBMP, Container: containerName(peerBMP, suffix), Image: "ze", Host: 6,
			Arguments: []string{dockerEntrypointFlag, zeTestBinary}, Command: []string{"interop-bgp", "bmp-collector"}})
	}
	if path := filepath.Join(scenario, "inject.msg"); regularFile(path) {
		arguments, err := readArguments(scenario, "inject-args")
		if err != nil {
			return nil, err
		}
		command := make([]string, 0, 4+len(arguments)+1)
		command = append(command, "peer", "--port", "179", "--decode")
		command = append(command, arguments...)
		command = append(command, "/inject.msg")
		peers = append(peers, interoplab.PeerConfig{Name: peerInject, Container: containerName(peerInject, suffix), Image: "ze", Host: 9,
			Mounts: []interoplab.Mount{mount(path, "/inject.msg")}, Arguments: []string{dockerEntrypointFlag, zeTestBinary}, Command: command})
	}
	if path := filepath.Join(scenario, "vrps.json"); regularFile(path) {
		peers = append(peers, interoplab.PeerConfig{Name: peerStayRTR, Container: containerName(peerStayRTR, suffix), Image: peerStayRTR, Host: 12,
			Mounts: []interoplab.Mount{mount(path, "/vrps.json")}, Ready: ready("wget", "-q", "-O", "-", rendered.Str("http://127.0.0.1:").Int(stayRTRPort).Str("/rpki.json").String())})
	}
	if path := filepath.Join(scenario, "rpki-server"); regularFile(path) {
		arguments, err := readArguments(scenario, "rpki-server")
		if err != nil {
			return nil, err
		}
		command := append([]string{"rpki", "--bind", "0.0.0.0"}, arguments...)
		peers = append(peers, interoplab.PeerConfig{Name: peerRPKI, Container: containerName(peerRPKI, suffix), Image: "ze", Host: 7,
			Arguments: []string{dockerEntrypointFlag, zeTestBinary}, Command: command})
	}

	zeMounts := []interoplab.Mount{mount(filepath.Join(scenario, "ze.conf"), "/etc/ze/bgp.conf")}
	peers = append(peers, interoplab.PeerConfig{Name: "ze", Container: containerName("ze", suffix), Image: "ze", Host: 2,
		Mounts: zeMounts, Capabilities: []string{capabilityNetAdmin}, Arguments: ipv6Sysctls(),
		Environment: []interoplab.EnvironmentVariable{{Name: "SESSION_TIMEOUT", Value: strconv.Itoa(int(timeout / time.Second))}},
		Command:     []string{"start", "/etc/ze/bgp.conf"}, Ready: ready("true")})

	for _, speaker := range []struct {
		file string
		name string
		host uint8
	}{{"speaker-args", peerSpeaker, 10}, {"speaker2-args", peerSpeaker2, 11}} {
		path := filepath.Join(scenario, speaker.file)
		if !regularFile(path) {
			continue
		}
		arguments, readErr := readArguments(scenario, speaker.file)
		if readErr != nil {
			return nil, readErr
		}
		command := []string{
			"interop-bgp",
			zeTestCommandSpeaker,
			"--connect",
			rendered.Reset().Str(networkHostAddress(network, 2)).Str(":179").String(),
		}
		command = append(command, arguments...)
		peers = append(peers, interoplab.PeerConfig{Name: speaker.name, Container: containerName(speaker.name, suffix), Image: "ze", Host: speaker.host,
			Arguments: []string{dockerEntrypointFlag, zeTestBinary}, Command: command})
	}
	if path := filepath.Join(scenario, "frr.conf"); regularFile(path) {
		peers = append(peers, interoplab.PeerConfig{Name: peerFRR, Container: containerName(peerFRR, suffix), Image: peerFRR, Host: 3,
			Mounts:       []interoplab.Mount{mount(path, "/etc/frr/frr.conf"), mount(filepath.Join(producer, "daemons"), "/etc/frr/daemons"), mount(filepath.Join(producer, "vtysh.conf"), "/etc/frr/vtysh.conf")},
			Capabilities: []string{capabilityNetAdmin, "SYS_ADMIN"}, Arguments: ipv6Sysctls(), Ready: ready(cmdVtysh, "-c", "show version")})
	}
	if path := filepath.Join(scenario, "bird.conf"); regularFile(path) {
		peers = append(peers, interoplab.PeerConfig{Name: peerBIRD, Container: containerName(peerBIRD, suffix), Image: peerBIRD, Host: 4,
			Mounts: []interoplab.Mount{mount(path, "/etc/bird/bird.conf")}, Capabilities: []string{capabilityNetAdmin}, Ready: ready(cmdBirdc, "show status")})
	}
	if path := filepath.Join(scenario, "keepalived.conf"); regularFile(path) {
		peers = append(peers, interoplab.PeerConfig{Name: peerKeepalived, Container: containerName(peerKeepalived, suffix), Image: peerKeepalived, Host: 8,
			Mounts: []interoplab.Mount{mount(path, "/etc/keepalived/keepalived.conf")}, Capabilities: []string{capabilityNetAdmin, "NET_RAW", "NET_BROADCAST"}, Arguments: ipv6Sysctls(), Ready: ready("ip", ipObjectLink)})
	}
	if path := filepath.Join(scenario, "gobgp.toml"); regularFile(path) {
		peers = append(peers, interoplab.PeerConfig{Name: peerGoBGP, Container: containerName(peerGoBGP, suffix), Image: peerGoBGP, Host: 5,
			Mounts: []interoplab.Mount{mount(path, "/etc/gobgp/gobgp.toml")}, Capabilities: []string{capabilityNetAdmin}})
	}
	return peers, nil
}

func ipv6Sysctls() []string {
	return []string{"--sysctl", "net.ipv6.conf.all.disable_ipv6=0", "--sysctl", "net.ipv6.conf.default.disable_ipv6=0"}
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func configContains(path, token string) bool {
	data, err := os.ReadFile(path) //nolint:gosec // the path is a scenario config inside the tracked checkout
	return err == nil && strings.Contains(string(data), token)
}

func readArguments(directory, name string) ([]string, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	data, readErr := root.ReadFile(name)
	closeErr := root.Close()
	if errors.Is(readErr, os.ErrNotExist) {
		return nil, closeErr
	}
	if readErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	return strings.Fields(string(data)), closeErr
}

func containerName(role, suffix string) string {
	var name textbuf.Buffer
	return name.Str("ze-iop-").Str(role).Byte('-').Str(suffix).String()
}

func networkHostAddress(network interoplab.Network, host uint8) string {
	octets := network.IPv4.Addr().As4()
	octets[3] = host
	return netip.AddrFrom4(octets).String()
}
