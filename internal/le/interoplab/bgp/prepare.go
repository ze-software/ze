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
	"slices"
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
	peerSpeaker2, peerKeepalived, peerStayRTR, peerPMACCT, peerFRRTransit, peerFRRSink,
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
		if relative == zeConfigFile &&
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
	rendered := filepath.Join(root, "tmp", interoplab.RenderedConfigDirectory, renderedName)
	if err := os.RemoveAll(rendered); err != nil {
		return interoplab.PreparedScenario{}, err
	}
	if err := renderScenario(prepare.Source.Directory, rendered, prepare.Network); err != nil {
		return interoplab.PreparedScenario{Cleanup: func() error { return os.RemoveAll(rendered) }}, err
	}
	peers, err := scenarioPeers(producer, rendered, suffix, prepare.Network)
	if prepare.Source.Name == rfc2545Scenario {
		// Own the advertised Link-Local before Ze snapshots its interfaces.
		setup := name.Reset().Str("ip -6 address add ").Addr(rfc2545LinkLocal(prepare.Network)).
			Str("/64 dev eth0 nodad\nexec ze \"$@\"").String()
		for i := range peers {
			if peers[i].Name == "ze" {
				peers[i].Arguments = append(peers[i].Arguments, dockerEntrypointFlag, "/bin/sh")
				peers[i].Command = append([]string{shellErrexitCommand, setup, "--"}, peers[i].Command...)
			}
		}
	}
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
			if strings.Contains(text, rfc2545LinkLocalToken) {
				if !network.IPv6.IsValid() {
					return errors.New("RFC 2545 link-local rendering requires an IPv6 network")
				}
				text = strings.ReplaceAll(text, rfc2545LinkLocalToken, rfc2545LinkLocal(network).String())
			}
			if relative == "inject.msg" {
				text = renderInjectedNextHops(text, ipv4)
			}
			// Both ze configs get the CLI block: the reload one replaces the
			// running config, and a scenario that lost the `interop` user at
			// SIGHUP would fail every assertion after it for a reason that has
			// nothing to do with what it tests.
			if relative == zeConfigFile || relative == zeReloadConfigFile {
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
	if !regularFile(filepath.Join(scenario, zeConfigFile)) {
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

	// A scenario carrying pmbmpd.conf reads ze's BMP stream with pmacct, so ze's
	// own collector is not started for it: two collectors would be two readings
	// of one stream, and only the third-party one is interop evidence.
	pmacctConfig := filepath.Join(scenario, "pmbmpd.conf")
	if regularFile(pmacctConfig) {
		peers = append(peers, interoplab.PeerConfig{Name: peerPMACCT, Container: containerName(peerPMACCT, suffix), Image: peerPMACCT, Host: 13,
			Mounts: []interoplab.Mount{mount(pmacctConfig, "/etc/pmacct/pmbmpd.conf")}, Command: []string{"-f", "/etc/pmacct/pmbmpd.conf"}})
	} else if configContains(filepath.Join(scenario, zeConfigFile), "bmp {") {
		peers = append(peers, interoplab.PeerConfig{Name: peerBMP, Container: containerName(peerBMP, suffix), Image: "ze", Host: 6,
			Arguments: []string{dockerEntrypointFlag, leBinary}, Command: []string{leTestWord, "interop-bgp", "bmp-collector"}})
	}
	if path := filepath.Join(scenario, "inject.msg"); regularFile(path) {
		arguments, err := readArguments(scenario, "inject-args")
		if err != nil {
			return nil, err
		}
		command := make([]string, 0, 5+len(arguments)+1)
		command = append(command, leTestWord, "peer", "--port", "179", "--decode")
		command = append(command, arguments...)
		command = append(command, "/inject.msg")
		peers = append(peers, interoplab.PeerConfig{Name: peerInject, Container: containerName(peerInject, suffix), Image: "ze", Host: 9,
			Mounts: []interoplab.Mount{mount(path, "/inject.msg")}, Arguments: []string{dockerEntrypointFlag, leBinary}, Command: command})
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
		command := append([]string{leTestWord, "rpki", "--bind", "0.0.0.0"}, arguments...)
		peers = append(peers, interoplab.PeerConfig{Name: peerRPKI, Container: containerName(peerRPKI, suffix), Image: "ze", Host: 7,
			Arguments: []string{dockerEntrypointFlag, leBinary}, Command: command})
	}

	zeMounts := []interoplab.Mount{mount(filepath.Join(scenario, zeConfigFile), zeMountedConfig)}
	zeArguments := ipv6Sysctls()
	zeCommand := []string{"start", zeMountedConfig}
	// A scenario carrying keepalived.conf runs ze as a VRRP router, and VRRP
	// writes per-device sysctls on the virtual-MAC macvlan it creates at run
	// time (applyDataplaneSysctls, internal/plugins/vrrp/dataplane_linux.go),
	// so no `--sysctl` at container start can name them. Docker blocks the
	// write twice in an unprivileged container: it mounts /proc/sys read-only
	// (lifted by systempaths=unconfined), and its docker-default AppArmor
	// profile denies writes under /proc/sys except kernel/ (measured
	// 2026-09-27, the first alone answers "Permission denied"). AppArmor is
	// never lifted (owner decision D-7): ze runs under the VRRP lab profile,
	// which denies every /proc/sys write but the per-device conf trees. The
	// net.* knobs stay confined to the container's own network namespace.
	appArmorProfile := ""
	if regularFile(filepath.Join(scenario, "keepalived.conf")) {
		zeArguments = append(zeArguments, "--security-opt", "systempaths=unconfined")
		appArmorProfile = vrrpLabAppArmorProfileName
	}
	// A scenario carrying ze-reload.conf reloads ze mid-run, so ze must read a
	// config file the checker can REPLACE. The mounted one is not it: every
	// lab mount is read-only, and the file behind it is the checkout's own
	// scenario file. So ze starts from a copy under /run, which lives in the
	// container alone. `exec` leaves ze as PID 1, so `docker kill --signal HUP`
	// still reaches it without tini.
	if path := filepath.Join(scenario, zeReloadConfigFile); regularFile(path) {
		zeMounts = append(zeMounts, mount(path, zeMountedReloadConfig))
		zeArguments = append(zeArguments, dockerEntrypointFlag, "/bin/sh")
		zeCommand = []string{"-c", rendered.Reset().Str("cp ").Str(zeMountedConfig).Byte(' ').Str(zeRunningConfig).
			Str(" && exec ze start ").Str(zeRunningConfig).String()}
	}
	peers = append(peers, interoplab.PeerConfig{Name: "ze", Container: containerName("ze", suffix), Image: "ze", Host: 2,
		Mounts: zeMounts, Capabilities: []string{capabilityNetAdmin}, Arguments: zeArguments, AppArmorProfile: appArmorProfile,
		Environment: []interoplab.EnvironmentVariable{{Name: "SESSION_TIMEOUT", Value: strconv.Itoa(int(timeout / time.Second))}},
		Command:     zeCommand, Ready: ready("true")})

	// FRR-facing OPEN-only relays wait for both native configurations.
	// Relays to other daemons keep their existing startup positions.
	var relays [2]extendedRelayPeer
	relayCount := 0
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
			leTestWord,
			"interop-bgp",
			zeTestCommandSpeaker,
			"--connect",
			rendered.Reset().Str(networkHostAddress(network, 2)).Str(":179").String(),
		}
		command = append(command, arguments...)
		peer := interoplab.PeerConfig{Name: speaker.name, Container: containerName(speaker.name, suffix), Image: "ze", Host: speaker.host,
			Arguments: []string{dockerEntrypointFlag, leBinary}, Command: command}
		isRelay := slices.ContainsFunc(arguments, func(argument string) bool {
			name, _, _ := strings.Cut(argument, "=")
			return name == "--relay-peer"
		})
		frrRelay := ""
		if isRelay {
			options, parseErr := parseSpeakerOptions(command[3:])
			if parseErr != nil {
				return nil, parseErr
			}
			switch options.relayPeer {
			case networkHostAddress(network, 3) + ":179":
				frrRelay = peerFRR
			case networkHostAddress(network, extendedSinkHost) + ":179":
				frrRelay = peerFRRSink
			}
		}
		if frrRelay == "" {
			peers = append(peers, peer)
		} else {
			relays[relayCount] = extendedRelayPeer{peer: peer, destination: frrRelay}
			relayCount++
		}
	}
	for _, frr := range []struct {
		name   string
		config string
		host   uint8
	}{{peerFRR, "frr.conf", 3}, {peerFRRTransit, virtualLinkTransitConfig, 14}, {peerFRRSink, "frr-sink.conf", extendedSinkHost}} {
		path := filepath.Join(scenario, frr.config)
		if !regularFile(path) {
			continue
		}
		// The shared daemons file names which FRR daemons run and what each one
		// is started with. A scenario needing a bgpd MODULE, `-M bmp` for one
		// that drives ze's BMP receiver, carries its own copy rather than adding
		// the module to every scenario in the suite.
		daemons := filepath.Join(producer, "daemons")
		if scenarioDaemons := filepath.Join(scenario, "daemons"); regularFile(scenarioDaemons) {
			daemons = scenarioDaemons
		}
		image, err := scenarioFRRImage(scenario)
		if err != nil {
			return nil, err
		}
		peers = append(peers, interoplab.PeerConfig{Name: frr.name, Container: containerName(frr.name, suffix), Image: image, Host: frr.host,
			Mounts:       []interoplab.Mount{mount(path, "/etc/frr/frr.conf"), mount(daemons, "/etc/frr/daemons"), mount(filepath.Join(producer, "vtysh.conf"), "/etc/frr/vtysh.conf")},
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
	if relayCount > 0 {
		// MUST install native configuration barriers before appending the relays.
		var err error
		peers, err = prepareExtendedRelayPeers(peers, relays[:relayCount], network)
		if err != nil {
			return nil, err
		}
	}
	return prepareVirtualLinkPeers(peers, scenario)
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

// frrImageFile names the FRR release a scenario needs when the suite's image
// cannot answer it, such as a capability the suite's FRR does not speak. It
// holds one image reference.
const frrImageFile = "frr-image"

// scenarioFRRImage returns the logical image name the FRR containers of
// scenario start from: the suite's FRR, or the release the scenario's
// frrImageFile pins, which suiteFor pulls under frrImageName.
func scenarioFRRImage(scenario string) (string, error) {
	reference, err := scenarioFRRReference(scenario)
	if err != nil {
		return "", err
	}
	if reference == "" {
		return peerFRR, nil
	}
	return frrImageName(reference), nil
}

// scenarioFRRReference reads the image reference a scenario's frrImageFile
// pins. A scenario without the file answers "", which is the suite's image; a
// file holding anything but one reference is an error, never the suite image.
func scenarioFRRReference(scenario string) (string, error) {
	fields, err := readArguments(scenario, frrImageFile)
	if err != nil {
		return "", err
	}
	if len(fields) == 0 {
		if regularFile(filepath.Join(scenario, frrImageFile)) {
			return "", fmt.Errorf("%s in %s names no image", frrImageFile, filepath.Base(scenario))
		}
		return "", nil
	}
	if len(fields) != 1 {
		return "", fmt.Errorf("%s in %s names %d images, expected one", frrImageFile, filepath.Base(scenario), len(fields))
	}
	return fields[0], nil
}

// frrImageName is the logical image name of a pinned FRR reference, distinct
// from peerFRR so the suite's FRR_IMAGE override never replaces a pin.
func frrImageName(reference string) string {
	var name textbuf.Buffer
	return name.Str(peerFRR).Byte('@').Str(reference).String()
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

// networkHostAddress6 is the IPv6 sibling: host N on the selected /64, which
// is the address Docker gives the container at Host N (addressAtHost).
func networkHostAddress6(network interoplab.Network, host uint8) string {
	octets := network.IPv6.Masked().Addr().As16()
	octets[15] = host
	return netip.AddrFrom16(octets).String()
}

const (
	rfc2545Scenario       = "bgp-rfc2545-linklocal-nexthop-frr"
	rfc2545LinkLocalToken = "@ZE_LINK_LOCAL@"
)

// rfc2545LinkLocal puts the selected lab prefix and Ze's host number in the
// interface identifier. prepareScenario assigns it to Ze's eth0 before startup;
// rendering and the checker use this same address rather than an unowned literal.
func rfc2545LinkLocal(network interoplab.Network) netip.Addr {
	prefix := network.IPv6.Masked().Addr().As16()
	return netip.AddrFrom16([16]byte{
		0xfe, 0x80,
		8: prefix[0], 9: prefix[1], 10: prefix[2], 11: prefix[3],
		12: prefix[4], 13: prefix[5], 15: 2,
	})
}
