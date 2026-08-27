//go:build linux

// Design: plan/spec-le-is-a-ze-binary.md -- step 10 guest-side evidence ports
// Producer: scripts/evidence/effective-vrrp-keepalived.py.
package qemu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
)

const (
	vrrpPrefixLength = "24"
	vrrpVIP          = "192.0.2.1"
	vrrpZeAddress    = "192.0.2.251"
	vrrpKAAddress    = "192.0.2.252"
	vrrpOBAddress    = "192.0.2.253"
	vrrpVRID         = 10
	vrrpZePriority   = 200
	vrrpKAPriority   = 100
	vrrpAdvertMS     = 1000
	vrrpAdvertCS     = 100
	vrrpVirtualMAC   = "00:00:5e:00:01:0a"
	vrrpAdvertTTL    = 255
	vrrpMulticastV4  = "224.0.0.18"
	vrrpAdvertKind   = "Advertisement"
	vrrpVersionV3    = 3
)

const (
	vrrpQS2PromoteMin = 3.0
	vrrpQS2PromoteMax = 6.0
	vrrpQS2PreemptMin = 2.8
	vrrpQS2PreemptMax = 6.0
	vrrpQS3PromoteMax = 3.0
	vrrpQS3NoSkewPath = 3.61
)

const (
	vrrpCaptureReadyTimeout = 20 * time.Second
	vrrpZeStartTimeout      = 45 * time.Second
	vrrpZeMasterTimeout     = 45 * time.Second
	vrrpKAStateTimeout      = 30 * time.Second
	vrrpWireEventTimeout    = 30 * time.Second
	vrrpKAGARPTimeout       = 25 * time.Second
	vrrpPingTimeout         = 20 * time.Second
)

var (
	vrrpRecordStart = regexp.MustCompile(`^(\d+\.\d+)\s`)
	vrrpEtherRE     = regexp.MustCompile(`^\d+\.\d+\s+([0-9a-f:]{17})\s+>\s+([0-9a-f:]{17}),`)
	vrrpTTLRE       = regexp.MustCompile(`\bttl (\d+)`)
	vrrpPacketRE    = regexp.MustCompile(`([\d.]+) > ([\d.]+): VRRPv(\d), (\w+), vrid (\d+), prio (\d+)`)
	vrrpIntervalRE  = regexp.MustCompile(`intvl (\d+)(cs|s)\b`)
	vrrpGARPRE      = regexp.MustCompile(`Request who-has ([\d.]+)(?: \(([0-9a-f:]+)\))? tell ([\d.]+)`)
)

var vrrpFatalNeedles = []string{
	"vrrp: instance create failed",
	"vrrp: this plugin must run in-process",
	"vrrp: install virtual addresses failed",
	"vrrp: unhandled FSM action",
	"auto-load config path plugin startup failed",
}

type vrrpNames struct {
	zeNS, kaNS, obNS, lanNS, probeNS string
	zeVeth, zeBridge                 string
	kaVeth, kaBridge                 string
	obVeth, obBridge, bridge         string
}

func newVRRPNames() vrrpNames {
	suffix := pidSuffix("")
	var tb textbuf.Buffer
	name := func(prefix string) string {
		value := tb.Str(prefix).Str(suffix).String()
		tb.Reset()
		return value
	}
	return vrrpNames{
		zeNS: name("ze-vrrp-ze-"), kaNS: name("ze-vrrp-ka-"),
		obNS: name("ze-vrrp-ob-"), lanNS: name("ze-vrrp-lan-"),
		probeNS: name("ze-vrrp-probe-"),
		zeVeth:  name("zvz"), zeBridge: name("zvzb"),
		kaVeth: name("zvk"), kaBridge: name("zvkb"),
		obVeth: name("zvo"), obBridge: name("zvob"),
		bridge: name("zvbr"),
	}
}

func (n vrrpNames) namespaces() []string { return []string{n.zeNS, n.kaNS, n.obNS, n.lanNS, n.probeNS} }
func (n vrrpNames) links() []string {
	return []string{n.zeVeth, n.zeBridge, n.kaVeth, n.kaBridge, n.obVeth, n.obBridge}
}

func vrrpSkew(priority int) float64       { return float64(256-priority) / 256 }
func vrrpMasterDown(priority int) float64 { return 3 + vrrpSkew(priority) }

func vrrpZeConfig(names vrrpNames) []byte {
	var tb textbuf.Buffer
	return tb.Str("interface {\n").
		Str("    backend netlink;\n").
		Str("    ethernet ").Str(names.zeVeth).Str(" {\n").
		Str("        unit 0 {\n").
		Str("            ipv4 {\n").
		Str("                address [ ").Str(vrrpZeAddress).Byte('/').Str(vrrpPrefixLength).Str(" ];\n").
		Str("                vrrp {\n").
		Str("                    group lab {\n").
		Str("                        vrid ").Int(vrrpVRID).Str(";\n").
		Str("                        virtual-address [ ").Str(vrrpVIP).Str(" ];\n").
		Str("                        priority ").Int(vrrpZePriority).Str(";\n").
		Str("                        preempt true;\n").
		Str("                        accept-mode true;\n").
		Str("                        advertise-interval-milliseconds ").Int(vrrpAdvertMS).Str(";\n").
		Str("                    }\n").
		Str("                }\n").
		Str("            }\n").
		Str("        }\n").
		Str("    }\n").
		Str("}\n").
		Bytes()
}

func vrrpKeepalivedConfig(names vrrpNames, notify, marker string, priority int) []byte {
	var tb textbuf.Buffer
	return tb.Str("global_defs {\n").
		Str("    vrrp_version 3\n").
		Str("    enable_script_security\n").
		Str("    script_user root\n").
		Str("}\n").
		Str("vrrp_instance lab {\n").
		Str("    state BACKUP\n").
		Str("    interface ").Str(names.kaVeth).Byte('\n').
		Str("    virtual_router_id ").Int(vrrpVRID).Byte('\n').
		Str("    priority ").Int(int64(priority)).Byte('\n').
		Str("    advert_int 1\n").
		Str("    virtual_ipaddress {\n").
		Str("        ").Str(vrrpVIP).Byte('/').Str(vrrpPrefixLength).Byte('\n').
		Str("    }\n").
		Str("    notify_master \"").Str(notify).Str(" MASTER ").Str(marker).Str("\"\n").
		Str("    notify_backup \"").Str(notify).Str(" BACKUP ").Str(marker).Str("\"\n").
		Str("    notify_fault  \"").Str(notify).Str(" FAULT ").Str(marker).Str("\"\n").
		Str("}\n").
		Bytes()
}

const vrrpNotifyScript = `#!/bin/sh
# keepalived notify hook: append the new state to the marker file the evidence
# script polls. $1 and $2 are our own arguments (state, marker path); keepalived
# appends its (type, name, state, priority) arguments after them, unused here.
printf '%s\n' "$1" >> "$2"
`

type vrrpAdvert struct {
	timestamp                     float64
	etherSource, etherDestination string
	ipSource, ipDestination, kind string
	version, vrid, priority       int
	ttl, interval                 int
	intervalUnit, raw             string
}

type vrrpGARP struct {
	timestamp                          float64
	etherSource, etherDestination      string
	senderIP, targetIP, targetMAC, raw string
}

func captureRecords(lines []string) [][]string {
	records := make([][]string, 0)
	var current []string
	for _, line := range lines {
		line = strings.TrimSuffix(line, "\n")
		if vrrpRecordStart.MatchString(line) {
			if current != nil {
				records = append(records, current)
			}
			current = []string{line}
		} else if current != nil {
			current = append(current, line)
		}
	}
	if current != nil {
		records = append(records, current)
	}
	return records
}

func regexInt(match []string, index int) int { value, _ := strconv.Atoi(match[index]); return value }

func parseVRRPAdverts(lines []string) []vrrpAdvert {
	out := make([]vrrpAdvert, 0)
	for _, record := range captureRecords(lines) {
		raw := strings.Join(trimStrings(record), " ")
		packet := vrrpPacketRE.FindStringSubmatch(raw)
		ether := vrrpEtherRE.FindStringSubmatch(record[0])
		start := vrrpRecordStart.FindStringSubmatch(record[0])
		if packet == nil || ether == nil || start == nil {
			continue
		}
		timestamp, _ := strconv.ParseFloat(start[1], 64)
		advert := vrrpAdvert{timestamp: timestamp, etherSource: ether[1], etherDestination: ether[2],
			ipSource: packet[1], ipDestination: packet[2], version: regexInt(packet, 3),
			kind: packet[4], vrid: regexInt(packet, 5), priority: regexInt(packet, 6), raw: raw}
		if ttl := vrrpTTLRE.FindStringSubmatch(raw); ttl != nil {
			advert.ttl = regexInt(ttl, 1)
		}
		if interval := vrrpIntervalRE.FindStringSubmatch(raw); interval != nil {
			advert.interval, advert.intervalUnit = regexInt(interval, 1), interval[2]
		}
		out = append(out, advert)
	}
	return out
}

func parseVRRPGARPs(lines []string) []vrrpGARP {
	out := make([]vrrpGARP, 0)
	for _, record := range captureRecords(lines) {
		raw := strings.Join(trimStrings(record), " ")
		if !strings.Contains(raw, "ethertype ARP") {
			continue
		}
		garp := vrrpGARPRE.FindStringSubmatch(raw)
		ether := vrrpEtherRE.FindStringSubmatch(record[0])
		start := vrrpRecordStart.FindStringSubmatch(record[0])
		if garp == nil || ether == nil || start == nil || garp[1] != garp[3] {
			continue
		}
		timestamp, _ := strconv.ParseFloat(start[1], 64)
		out = append(out, vrrpGARP{timestamp: timestamp, etherSource: ether[1], etherDestination: ether[2],
			targetIP: garp[1], targetMAC: garp[2], senderIP: garp[3], raw: raw})
	}
	return out
}

func trimStrings(lines []string) []string {
	out := make([]string, len(lines))
	for index, line := range lines {
		out[index] = strings.TrimSpace(line)
	}
	return out
}

func probeVRRPKernel(ctx context.Context, names vrrpNames) error {
	if !hasGuestNetAdmin() {
		return errors.New("VRRP keepalived interop evidence requires root or CAP_NET_ADMIN (netns/veth/bridge/macvlan creation) plus CAP_NET_RAW (ze's raw proto-112 and AF_PACKET sockets)")
	}
	if modprobe, err := execLookPath("modprobe"); err == nil && os.Geteuid() == 0 {
		for _, module := range []string{"veth", "bridge", "macvlan", "dummy"} {
			if _, loadErr := guestRun(ctx, "", []string{modprobe, module}, nil); loadErr != nil {
				fmt.Fprintf(os.Stderr, "load optional VRRP module %s: %v\n", module, loadErr) //nolint:errcheck // probe diagnostics
			}
		}
	}
	if _, deleteErr := guestRun(ctx, "", []string{"ip", "netns", "delete", names.probeNS}, nil); deleteErr != nil {
		fmt.Fprintf(os.Stderr, "delete stale VRRP probe namespace: %v\n", deleteErr) //nolint:errcheck // probe diagnostics
	}
	if err := os.MkdirAll("/run/netns", 0o750); err != nil {
		return fmt.Errorf("create /run/netns: %w", err)
	}
	var tb textbuf.Buffer
	if err := guestRequired(ctx, "", []string{"ip", "netns", "add", names.probeNS},
		tb.Str("create probe netns ").Str(names.probeNS).String()); err != nil {
		return err
	}
	defer func() {
		if _, cleanupErr := guestRun(context.Background(), "", []string{"ip", "netns", "delete", names.probeNS}, nil); cleanupErr != nil {
			fmt.Fprintf(os.Stderr, "delete VRRP probe namespace: %v\n", cleanupErr) //nolint:errcheck // cleanup diagnostics
		}
	}()
	probes := []struct {
		argv            []string
		symbol, feature string
	}{
		{[]string{"ip", "link", "add", "zprobe0", "type", "veth", "peer", "name", "zprobe1"}, "CONFIG_VETH", "veth pair support"},
		{[]string{"ip", "link", "add", "zprobebr", "type", "bridge"}, "CONFIG_BRIDGE", "bridge support"},
		{[]string{"ip", "link", "add", "zprobemv", "link", "zprobe0", "type", "macvlan", "mode", "bridge"}, "CONFIG_MACVLAN", "bridge-mode macvlan support"},
	}
	for _, probe := range probes {
		result, err := guestRun(ctx, names.probeNS, probe.argv, nil)
		if err != nil {
			return err
		}
		if result.Code != 0 {
			fmt.Fprint(os.Stderr, result.Stdout, result.Stderr) //nolint:errcheck // kernel probe diagnostics
			return fmt.Errorf(
				"kernel lacks %s: `%s` failed in a private namespace. This lab needs %s in the running kernel. "+
					"It already boots ze's runtime kernel, so a module package is not the answer and there is "+
					"nothing to load: add %s to gokrazy/kernel/runtime.config and to "+
					"gokrazy/kernel/runtime.require, then `make ze-kernel-vmlinuz-stage KERNEL_ARCH=<arch>`. "+
					"The require entry makes a later build fail rather than silently ship without the symbol",
				probe.feature, strings.Join(probe.argv, " "), probe.symbol, probe.symbol)
		}
	}
	release, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	tb.Reset()
	gaterun.Note(tb.Str("kernel probe: ").Str(strings.TrimSpace(string(release))).
		Str(": veth OK, bridge OK, macvlan (bridge mode) OK -- this kernel carries everything the lab builds").String())
	return nil
}

type vrrpLab struct {
	root, binary, scenario string
	names                  vrrpNames
	work, secure           string
	notify, marker, pcap   string
	ze, keepalived         *guestProcess
	capture, pcapProcess   *guestProcess
	captureLines, zeLines  *lineCollector
	kaMAC                  string
	cleanup, details       []string
}

func newVRRPLab(root, binary, scenario string, names vrrpNames) (*vrrpLab, error) {
	parent := filepath.Join(root, "tmp", "evidence")
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return nil, err
	}
	var tb textbuf.Buffer
	scenarioName := strings.ToLower(scenario)
	work, err := os.MkdirTemp(parent, tb.Str("effective-vrrp-").Str(scenarioName).Byte('-').String())
	if err != nil {
		return nil, err
	}
	tb.Reset()
	secure, err := os.MkdirTemp("/root", tb.Str("vrrp-ka-").Str(scenarioName).Byte('-').String())
	if err != nil {
		_ = os.RemoveAll(work)
		return nil, err
	}
	return &vrrpLab{root: root, binary: binary, scenario: scenario, names: names,
		work: work, secure: secure, notify: filepath.Join(secure, "notify.sh"),
		marker: filepath.Join(secure, "ka-state.log"), pcap: filepath.Join(work, "observer.pcap")}, nil
}

func (l *vrrpLab) teardown(success bool) {
	var tb textbuf.Buffer
	for _, item := range []struct {
		name    string
		process *guestProcess
	}{
		{"keepalived", l.keepalived}, {"ze", l.ze}, {"capture", l.capture}, {"pcap", l.pcapProcess},
	} {
		if item.process != nil {
			item.process.stop()
			l.cleanup = append(l.cleanup, tb.Str("process:").Str(item.name).String())
			tb.Reset()
		}
	}
	cleanupNamespaces(context.Background(), l.names.namespaces(), l.names.links(), &l.cleanup)
	_ = os.RemoveAll(l.secure)
	l.cleanup = append(l.cleanup, tb.Str("secure:").Str(l.secure).String())
	if success {
		_ = os.RemoveAll(l.work)
		tb.Reset()
		l.cleanup = append(l.cleanup, tb.Str("work:").Str(l.work).String())
	}
}

func (l *vrrpLab) addLeaf(ctx context.Context, namespace, leaf, bridgeEnd, address string) error {
	var tb textbuf.Buffer
	operation := func(prefix, value, separator, other string) string {
		text := tb.Str(prefix).Str(value).Str(separator).Str(other).String()
		tb.Reset()
		return text
	}
	steps := []struct {
		ns   string
		argv []string
		what string
	}{
		{"", []string{"ip", "link", "add", leaf, "type", "veth", "peer", "name", bridgeEnd},
			operation("create veth pair ", leaf, "/", bridgeEnd)},
		{"", []string{"ip", "link", "set", leaf, "netns", namespace},
			operation("move ", leaf, " into ", namespace)},
		{"", []string{"ip", "link", "set", bridgeEnd, "netns", l.names.lanNS},
			operation("move ", bridgeEnd, " into ", l.names.lanNS)},
		{l.names.lanNS, []string{"ip", "link", "set", bridgeEnd, "master", l.names.bridge},
			operation("enslave ", bridgeEnd, "", "")},
		{l.names.lanNS, []string{"ip", "link", "set", bridgeEnd, "up"},
			operation("up ", bridgeEnd, "", "")},
		{namespace, []string{"ip", "link", "set", "lo", "up"},
			operation("up loopback in ", namespace, "", "")},
		{namespace, []string{"ip", "link", "set", leaf, "up"},
			operation("up ", leaf, "", "")},
	}
	for _, step := range steps {
		if err := guestRequired(ctx, step.ns, step.argv, step.what); err != nil {
			return err
		}
	}
	if address != "" {
		addressWithPrefix := tb.Str(address).Byte('/').Str(vrrpPrefixLength).String()
		tb.Reset()
		assign := tb.Str("assign ").Str(address).String()
		return guestRequired(ctx, namespace,
			[]string{"ip", "addr", "add", addressWithPrefix, "dev", leaf}, assign)
	}
	return nil
}

func (l *vrrpLab) setup(ctx context.Context) error {
	var tb textbuf.Buffer
	cleanupNamespaces(context.Background(), l.names.namespaces(), l.names.links(), nil)
	if err := os.MkdirAll("/run/netns", 0o750); err != nil {
		return err
	}
	for _, namespace := range []string{l.names.lanNS, l.names.zeNS, l.names.kaNS, l.names.obNS} {
		if err := guestRequired(ctx, "", []string{"ip", "netns", "add", namespace},
			tb.Str("create netns ").Str(namespace).String()); err != nil {
			return err
		}
		tb.Reset()
	}
	for _, step := range [][]string{{"ip", "link", "set", "lo", "up"}, {"ip", "link", "add", l.names.bridge, "type", "bridge"}, {"ip", "link", "set", l.names.bridge, "type", "bridge", "mcast_snooping", "0"}, {"ip", "link", "set", l.names.bridge, "up"}} {
		if err := guestRequired(ctx, l.names.lanNS, step, "prepare LAN bridge"); err != nil {
			return err
		}
	}
	if err := l.addLeaf(ctx, l.names.zeNS, l.names.zeVeth, l.names.zeBridge, ""); err != nil {
		return err
	}
	if err := l.addLeaf(ctx, l.names.kaNS, l.names.kaVeth, l.names.kaBridge, vrrpKAAddress); err != nil {
		return err
	}
	if err := l.addLeaf(ctx, l.names.obNS, l.names.obVeth, l.names.obBridge, vrrpOBAddress); err != nil {
		return err
	}
	ping, err := guestRun(ctx, l.names.obNS, []string{"ping", "-c", "1", "-W", "3", vrrpKAAddress}, nil)
	if err != nil {
		return err
	}
	if ping.Code != 0 {
		return fmt.Errorf("observer namespace cannot reach the keepalived namespace over %s", l.names.bridge)
	}
	mac, err := linkMAC(ctx, l.names.kaNS, l.names.kaVeth)
	if err != nil {
		return err
	}
	l.kaMAC = mac
	if err := os.WriteFile(l.notify, []byte(vrrpNotifyScript), 0o600); err != nil {
		return err
	}
	return os.Chmod(l.notify, 0o700) // #nosec G302 -- keepalived executes this notify script, and only the owner may access it
}

func linkMAC(ctx context.Context, namespace, device string) (string, error) {
	result, err := guestRun(ctx, namespace, []string{"ip", "-j", "link", "show", device}, nil)
	if err != nil {
		return "", err
	}
	if result.Code != 0 {
		return "", fmt.Errorf("ip -j link show %s failed in %s", device, namespace)
	}
	var links []struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &links); err != nil || len(links) == 0 || links[0].Address == "" {
		return "", fmt.Errorf("cannot read %s MAC in %s", device, namespace)
	}
	return links[0].Address, nil
}

func (l *vrrpLab) startCapture(ctx context.Context) error {
	filter := "ip proto 112 or arp or icmp"
	pcapProcess, pcapLines, err := startGuestProcess(ctx, l.names.obNS, []string{"tcpdump", "-i", l.names.obVeth, "-n", "-s", "0", "-U", "-w", l.pcap, filter}, os.Environ(), "pcap> ")
	if err != nil {
		return err
	}
	l.pcapProcess = pcapProcess
	l.capture, l.captureLines, err = startGuestProcess(ctx, l.names.obNS, []string{"tcpdump", "-i", l.names.obVeth, "-n", "-e", "-vv", "-l", "-tt", "-s", "0", filter}, os.Environ(), "cap> ")
	if err != nil {
		return err
	}
	containsListening := func(lines []string) bool { return linesContain(lines, "listening on") }
	if err := l.captureLines.wait(ctx, vrrpCaptureReadyTimeout, l.capture, containsListening, nil); err != nil {
		return fmt.Errorf("tcpdump line capture did not start on %s: %w", l.names.obVeth, err)
	}
	if err := pcapLines.wait(ctx, vrrpCaptureReadyTimeout, l.pcapProcess, containsListening, nil); err != nil {
		return fmt.Errorf("tcpdump pcap capture did not start on %s: %w", l.names.obVeth, err)
	}
	return nil
}

func linesContain(lines []string, needle string) bool {
	for _, line := range lines {
		if strings.Contains(line, needle) {
			return true
		}
	}
	return false
}
func fatalVRRP(lines []string) error {
	var tb textbuf.Buffer
	for _, line := range lines {
		for _, needle := range vrrpFatalNeedles {
			if strings.Contains(line, needle) {
				return errors.New(tb.Str("ze reported fatal failure: ").Str(needle).String())
			}
		}
	}
	return nil
}

func (l *vrrpLab) adverts() []vrrpAdvert { return parseVRRPAdverts(l.captureLines.snapshot()) }
func (l *vrrpLab) garps() []vrrpGARP     { return parseVRRPGARPs(l.captureLines.snapshot()) }
func (l *vrrpLab) zeAdverts() []vrrpAdvert {
	adverts := l.adverts()
	out := make([]vrrpAdvert, 0, len(adverts))
	for index := range adverts {
		if adverts[index].ipSource == vrrpZeAddress {
			out = append(out, adverts[index])
		}
	}
	return out
}
func (l *vrrpLab) kaAdverts() []vrrpAdvert {
	adverts := l.adverts()
	out := make([]vrrpAdvert, 0, len(adverts))
	for index := range adverts {
		if adverts[index].ipSource == vrrpKAAddress {
			out = append(out, adverts[index])
		}
	}
	return out
}

func (l *vrrpLab) startZe(ctx context.Context, config []byte) (float64, error) {
	if err := os.MkdirAll(filepath.Join(l.work, "ze"), 0o750); err != nil {
		return 0, err
	}
	path := filepath.Join(l.work, "ze.conf")
	if err := os.WriteFile(path, config, 0o600); err != nil {
		return 0, err
	}
	environ := withGuestEnv(os.Environ(), map[string]string{
		"ZE_LOG_VRRP": "info", "ZE_LOG_PLUGIN_RELAY": "info",
		guestStorageBlobKey: "false", guestConfigDirKey: filepath.Join(l.work, "ze"),
	})
	started := float64(time.Now().UnixNano()) / float64(time.Second)
	process, lines, err := startGuestProcess(ctx, l.names.zeNS, []string{l.binary, "start", path}, environ, "ze> ")
	if err != nil {
		return 0, err
	}
	l.ze, l.zeLines = process, lines
	err = lines.wait(ctx, vrrpZeStartTimeout, process, func(lines []string) bool { return linesContain(lines, "vrrp: parent usable, starting virtual router") }, fatalVRRP)
	if err != nil {
		return 0, fmt.Errorf("ze did not start its virtual router: %w", err)
	}
	return started, nil
}

func (l *vrrpLab) waitZeState(ctx context.Context, state string) error {
	var tb textbuf.Buffer
	needle := tb.Str("to=").Str(state).String()
	err := l.zeLines.wait(ctx, vrrpZeMasterTimeout, l.ze, func(lines []string) bool {
		for _, line := range lines {
			if strings.Contains(line, "vrrp: state change") && strings.Contains(line, needle) {
				return true
			}
		}
		return false
	}, fatalVRRP)
	if err != nil {
		return fmt.Errorf("ze did not reach %s state: %w", state, err)
	}
	return nil
}

func (l *vrrpLab) keepalivedVersion(ctx context.Context) string {
	result, _ := guestRun(ctx, l.names.kaNS, []string{"keepalived", "-v"}, nil)
	var tb textbuf.Buffer
	text := strings.TrimSpace(tb.Str(result.Stdout).Str(result.Stderr).String())
	if line, _, ok := strings.Cut(text, "\n"); ok {
		return line
	}
	if text != "" {
		return text
	}
	return "unknown"
}

func (l *vrrpLab) startKeepalived(ctx context.Context, config []byte) error {
	path := filepath.Join(l.work, "keepalived.conf")
	if err := os.WriteFile(path, config, 0o600); err != nil {
		return err
	}
	check, err := guestRun(ctx, l.names.kaNS, []string{"keepalived", "-t", "-f", path}, nil)
	if err != nil {
		return err
	}
	if check.Code != 0 {
		fmt.Fprint(os.Stderr, check.Stdout, check.Stderr) //nolint:errcheck // peer diagnostics
		return fmt.Errorf("keepalived rejected the generated config (%s); see %s",
			l.keepalivedVersion(ctx), path)
	}
	l.keepalived, _, err = startGuestProcess(ctx, l.names.kaNS, []string{"keepalived", "-n", "-l", "-D", "-P", "-f", path, "-p", filepath.Join(l.work, "keepalived.pid")}, os.Environ(), "ka> ")
	return err
}

func (l *vrrpLab) kaStates() []string {
	data, err := os.ReadFile(l.marker)
	if err != nil {
		return nil
	}
	states := make([]string, 0)
	for line := range strings.SplitSeq(string(data), "\n") {
		if state := strings.TrimSpace(line); state != "" {
			states = append(states, state)
		}
	}
	return states
}
func (l *vrrpLab) waitKAState(ctx context.Context, state string) error {
	err := waitGuest(ctx, vrrpKAStateTimeout, guestPollInterval, func() (bool, error) {
		states := l.kaStates()
		return len(states) != 0 && states[len(states)-1] == state, nil
	})
	if err != nil {
		return fmt.Errorf("keepalived did not reach %s: markers so far %v", state, l.kaStates())
	}
	return nil
}
func (l *vrrpLab) establishMaster(ctx context.Context) error {
	if err := l.startCapture(ctx); err != nil {
		return err
	}
	if _, err := l.startZe(ctx, vrrpZeConfig(l.names)); err != nil {
		return err
	}
	if err := l.waitZeState(ctx, "master"); err != nil {
		return err
	}
	if err := l.startKeepalived(ctx, vrrpKeepalivedConfig(l.names, l.notify, l.marker, vrrpKAPriority)); err != nil {
		return err
	}
	return l.waitKAState(ctx, "BACKUP")
}

func (l *vrrpLab) assertAdvertFields(ctx context.Context) error {
	if err := waitGuest(ctx, vrrpWireEventTimeout, guestPollInterval, func() (bool, error) { return len(l.zeAdverts()) != 0, nil }); err != nil {
		return errors.New("no ze-sourced VRRP advert captured on the observer's segment")
	}
	adverts := l.zeAdverts()
	var tb textbuf.Buffer
	for index := range adverts {
		advert := &adverts[index]
		problems := make([]string, 0)
		if advert.version != vrrpVersionV3 {
			problems = append(problems, tb.Str("version ").Int(int64(advert.version)).
				Str(" != ").Int(vrrpVersionV3).String())
			tb.Reset()
		}
		if advert.kind != vrrpAdvertKind {
			problems = append(problems, tb.Str("type ").Str(advert.kind).
				Str(" != ").Str(vrrpAdvertKind).String())
			tb.Reset()
		}
		if advert.vrid != vrrpVRID {
			problems = append(problems, tb.Str("vrid ").Int(int64(advert.vrid)).
				Str(" != ").Int(vrrpVRID).String())
			tb.Reset()
		}
		if advert.priority != vrrpZePriority {
			problems = append(problems, tb.Str("prio ").Int(int64(advert.priority)).
				Str(" != ").Int(vrrpZePriority).String())
			tb.Reset()
		}
		if advert.ttl != vrrpAdvertTTL {
			problems = append(problems, tb.Str("ttl ").Int(int64(advert.ttl)).
				Str(" != ").Int(vrrpAdvertTTL).String())
			tb.Reset()
		}
		if advert.etherSource != vrrpVirtualMAC {
			problems = append(problems, tb.Str("ether src ").Str(advert.etherSource).
				Str(" != ").Str(vrrpVirtualMAC).String())
			tb.Reset()
		}
		if advert.ipDestination != vrrpMulticastV4 {
			problems = append(problems, tb.Str("ip dst ").Str(advert.ipDestination).
				Str(" != ").Str(vrrpMulticastV4).String())
			tb.Reset()
		}
		if advert.interval != vrrpAdvertCS || advert.intervalUnit != "cs" {
			problems = append(problems, tb.Str("intvl ").Int(int64(advert.interval)).
				Str(advert.intervalUnit).Str(" != ").Int(vrrpAdvertCS).Str("cs").String())
			tb.Reset()
		}
		if len(problems) != 0 {
			return errors.New(tb.Str("ze advert violates RFC 9568: ").Join(problems, "; ").
				Str("\nrecord: ").Str(advert.raw).String())
		}
	}
	l.details = append(l.details, tb.Str("  wire: ").Int(int64(len(adverts))).
		Str(" ze adverts, all VRRPv3 type 1 vrid ").Int(vrrpVRID).
		Str(" prio ").Int(vrrpZePriority).Str(" intvl ").Int(vrrpAdvertCS).
		Str("cs ttl ").Int(vrrpAdvertTTL).Str(" ether-src ").Str(vrrpVirtualMAC).
		Str(" dst ").Str(vrrpMulticastV4).String())
	return nil
}
func (l *vrrpLab) assertNoKAAdverts(since float64) error {
	adverts := l.kaAdverts()
	count := 0
	for index := range adverts {
		if adverts[index].timestamp >= since {
			count++
		}
	}
	if count != 0 {
		return fmt.Errorf("keepalived sent %d advert(s) while ze was Active", count)
	}
	return nil
}

func neighborMAC(ctx context.Context, namespace, device, destination string) string {
	result, err := guestRun(ctx, namespace, []string{"ip", "-j", "neigh", "show", "dev", device}, nil)
	if err != nil || result.Code != 0 {
		return ""
	}
	var entries []struct {
		Destination string `json:"dst"`
		Address     string `json:"lladdr"`
	}
	if json.Unmarshal([]byte(result.Stdout), &entries) != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.Destination == destination {
			return entry.Address
		}
	}
	return ""
}
func (l *vrrpLab) assertVIP(ctx context.Context) error {
	err := waitGuest(ctx, vrrpPingTimeout, guestPollInterval, func() (bool, error) {
		if _, flushErr := guestRun(ctx, l.names.obNS, []string{"ip", "neigh", "flush", "all"}, nil); flushErr != nil {
			return false, flushErr
		}
		ping, err := guestRun(ctx, l.names.obNS, []string{"ping", "-c", "2", "-W", "3", vrrpVIP}, nil)
		if err != nil {
			return false, err
		}
		return ping.Code == 0 && neighborMAC(ctx, l.names.obNS, l.names.obVeth, vrrpVIP) == vrrpVirtualMAC, nil
	})
	if err != nil {
		return fmt.Errorf("observer resolves %s to %s, expected the virtual MAC %s", vrrpVIP, neighborMAC(ctx, l.names.obNS, l.names.obVeth, vrrpVIP), vrrpVirtualMAC)
	}
	var tb textbuf.Buffer
	l.details = append(l.details, tb.Str("  dataplane: observer pings ").Str(vrrpVIP).
		Str(", ARP resolves to ").Str(vrrpVirtualMAC).String())
	return nil
}
func (l *vrrpLab) waitWireWindow(ctx context.Context, seconds float64) error {
	if err := waitGuest(ctx, vrrpWireEventTimeout, guestPollInterval, func() (bool, error) { return len(l.adverts()) != 0, nil }); err != nil {
		return errors.New("no adverts captured to anchor the observation window")
	}
	adverts := l.adverts()
	anchor := adverts[len(adverts)-1].timestamp
	return waitGuest(ctx, vrrpWireEventTimeout, guestPollInterval, func() (bool, error) {
		current := l.adverts()
		return len(current) != 0 && current[len(current)-1].timestamp-anchor >= seconds, nil
	})
}

func (l *vrrpLab) runQS1(ctx context.Context) error {
	if err := l.establishMaster(ctx); err != nil {
		return err
	}
	window := vrrpMasterDown(vrrpKAPriority) + 1
	if err := l.waitWireWindow(ctx, window); err != nil {
		return err
	}
	states := l.kaStates()
	if len(states) == 0 || states[len(states)-1] != "BACKUP" {
		return fmt.Errorf("keepalived did not settle BACKUP: markers %v", states)
	}
	if err := l.assertAdvertFields(ctx); err != nil {
		return err
	}
	if err := l.assertNoKAAdverts(0); err != nil {
		return err
	}
	if err := l.assertVIP(ctx); err != nil {
		return err
	}
	var tb textbuf.Buffer
	l.details = append(l.details, tb.Str("  state: keepalived settled BACKUP across ").
		Float(window, 2).Str("s of wire time (> its ").
		Float(vrrpMasterDown(vrrpKAPriority), 3).Str("s master-down)").String())
	return nil
}

func (l *vrrpLab) runQS2(ctx context.Context) error {
	var tb textbuf.Buffer
	if err := l.establishMaster(ctx); err != nil {
		return err
	}
	if err := l.assertAdvertFields(ctx); err != nil {
		return err
	}
	zeAdverts := l.zeAdverts()
	if len(zeAdverts) == 0 {
		return errors.New("no ze advert to anchor the failover measurement")
	}
	lastZe := zeAdverts[len(zeAdverts)-1].timestamp
	if err := l.ze.kill(); err != nil {
		return err
	}
	tb.Reset()
	if err := guestRequired(ctx, l.names.zeNS, []string{"ip", "link", "set", l.names.zeVeth, "down"},
		tb.Str("down ").Str(l.names.zeVeth).String()); err != nil {
		return err
	}
	if err := l.waitKAState(ctx, "MASTER"); err != nil {
		return err
	}
	if err := waitGuest(ctx, vrrpWireEventTimeout, guestPollInterval, func() (bool, error) {
		adverts := l.kaAdverts()
		for index := range adverts {
			if adverts[index].timestamp > lastZe {
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		return errors.New("keepalived reported MASTER but sent no advert of its own")
	}
	var first vrrpAdvert
	adverts := l.kaAdverts()
	for index := range adverts {
		if adverts[index].timestamp > lastZe {
			first = adverts[index]
			break
		}
	}
	delta := first.timestamp - lastZe
	if delta < vrrpQS2PromoteMin || delta > vrrpQS2PromoteMax {
		return fmt.Errorf("keepalived promoted %.3fs after ze's last advert, outside the [%.1f, %.1f]s band", delta, vrrpQS2PromoteMin, vrrpQS2PromoteMax)
	}
	if first.priority != vrrpKAPriority {
		return fmt.Errorf("keepalived advertised prio %d, expected %d", first.priority, vrrpKAPriority)
	}
	l.details = append(l.details, tb.Str("  failover: keepalived promoted ").Float(delta, 3).
		Str("s after ze's last advert (band [").Float(vrrpQS2PromoteMin, 1).Str(", ").
		Float(vrrpQS2PromoteMax, 1).Str("]s), advertising prio ").Int(int64(first.priority)).
		Str(" from ").Str(first.etherSource).String())
	tb.Reset()
	if err := waitGuest(ctx, vrrpKAGARPTimeout, guestPollInterval, func() (bool, error) {
		for _, garp := range l.garps() {
			if garp.timestamp > lastZe && garp.senderIP == vrrpVIP && garp.etherSource == l.kaMAC {
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		return fmt.Errorf("keepalived sent no gratuitous ARP for %s from its own MAC %s after promoting", vrrpVIP, l.kaMAC)
	}
	tb.Reset()
	l.details = append(l.details, tb.Str("  failover: keepalived sent gratuitous ARP for ").
		Str(vrrpVIP).Str(" from ").Str(l.kaMAC).String())
	tb.Reset()
	if err := guestRequired(ctx, l.names.zeNS, []string{"ip", "link", "set", l.names.zeVeth, "up"},
		tb.Str("up ").Str(l.names.zeVeth).String()); err != nil {
		return err
	}
	restart, err := l.startZe(ctx, vrrpZeConfig(l.names))
	if err != nil {
		return err
	}
	if err := l.waitZeState(ctx, "master"); err != nil {
		return err
	}
	if err := waitGuest(ctx, vrrpWireEventTimeout, guestPollInterval, func() (bool, error) {
		adverts := l.zeAdverts()
		for index := range adverts {
			if adverts[index].timestamp > restart {
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		return errors.New("restarted ze reached master but sent no advert")
	}
	var returned vrrpAdvert
	adverts = l.zeAdverts()
	for index := range adverts {
		if adverts[index].timestamp > restart {
			returned = adverts[index]
			break
		}
	}
	preemptDelta := returned.timestamp - restart
	if preemptDelta < vrrpQS2PreemptMin || preemptDelta > vrrpQS2PreemptMax {
		return fmt.Errorf("ze preempted %.3fs after restart, outside the [%.1f, %.1f]s band", preemptDelta, vrrpQS2PreemptMin, vrrpQS2PreemptMax)
	}
	if err := waitGuest(ctx, vrrpWireEventTimeout, guestPollInterval, func() (bool, error) {
		for _, garp := range l.garps() {
			if garp.timestamp > restart && garp.senderIP == vrrpVIP && garp.etherSource == vrrpVirtualMAC {
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		return fmt.Errorf("restarted ze sent no gratuitous ARP for %s from %s", vrrpVIP, vrrpVirtualMAC)
	}
	burst := make([]vrrpGARP, 0)
	for _, garp := range l.garps() {
		if garp.timestamp > restart && garp.senderIP == vrrpVIP && garp.etherSource == vrrpVirtualMAC {
			if garp.targetIP != vrrpVIP || garp.senderIP != vrrpVIP {
				tb.Reset()
				return errors.New(tb.Str("ze GARP is not gratuitous: ").Str(garp.raw).String())
			}
			if garp.targetMAC != "" && garp.targetMAC != vrrpVirtualMAC {
				tb.Reset()
				return errors.New(tb.Str("ze GARP target link-layer is not the virtual MAC: ").Str(garp.raw).String())
			}
			burst = append(burst, garp)
		}
	}
	l.details = append(l.details, tb.Str("  preempt: ze promoted ").Float(preemptDelta, 3).
		Str("s after restart (band [").Float(vrrpQS2PreemptMin, 1).Str(", ").
		Float(vrrpQS2PreemptMax, 1).Str("]s), GARP burst of ").Int(int64(len(burst))).
		Str(" frame(s), sender IP == target IP == ").Str(vrrpVIP).
		Str(", MAC ").Str(vrrpVirtualMAC).String())
	if err := l.waitKAState(ctx, "BACKUP"); err != nil {
		return err
	}
	if err := l.assertVIP(ctx); err != nil {
		return err
	}
	l.details = append(l.details, "  preempt: keepalived returned to BACKUP, observer repointed to the VIP")
	return nil
}

func (l *vrrpLab) runQS3(ctx context.Context) error {
	if err := l.establishMaster(ctx); err != nil {
		return err
	}
	if err := l.assertAdvertFields(ctx); err != nil {
		return err
	}
	if err := l.ze.signal(syscall.SIGTERM); err != nil {
		return err
	}
	if err := waitGuest(ctx, vrrpWireEventTimeout, guestPollInterval, func() (bool, error) {
		adverts := l.zeAdverts()
		for index := range adverts {
			if adverts[index].priority == 0 {
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		return errors.New("ze sent no Priority-0 advert on SIGTERM")
	}
	var priorityZero vrrpAdvert
	adverts := l.zeAdverts()
	for index := range adverts {
		if adverts[index].priority == 0 {
			priorityZero = adverts[index]
			break
		}
	}
	var tb textbuf.Buffer
	l.details = append(l.details, tb.Str("  wire: ze sent the Priority-0 resignation advert at ").
		Float(priorityZero.timestamp, 6).String())
	if err := l.waitKAState(ctx, "MASTER"); err != nil {
		return err
	}
	if err := waitGuest(ctx, vrrpWireEventTimeout, guestPollInterval, func() (bool, error) {
		adverts := l.kaAdverts()
		for index := range adverts {
			if adverts[index].timestamp >= priorityZero.timestamp {
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		return errors.New("keepalived reported MASTER but sent no advert after ze's Priority-0")
	}
	var first vrrpAdvert
	adverts = l.kaAdverts()
	for index := range adverts {
		if adverts[index].timestamp >= priorityZero.timestamp {
			first = adverts[index]
			break
		}
	}
	delta := first.timestamp - priorityZero.timestamp
	if delta > vrrpQS3PromoteMax {
		detail := ""
		if delta >= vrrpQS3NoSkewPath {
			tb.Reset()
			detail = tb.Str(" (>= ").Float(vrrpQS3NoSkewPath, 2).
				Str("s means the skew path was not taken)").String()
		}
		return fmt.Errorf("keepalived promoted %.3fs after ze's Priority-0 advert, above the %.1fs skew-path bound%s", delta, vrrpQS3PromoteMax, detail)
	}
	tb.Reset()
	l.details = append(l.details, tb.Str("  prio-0: keepalived promoted ").Float(delta, 3).
		Str("s after the resignation (bound ").Float(vrrpQS3PromoteMax, 1).
		Str("s; its skew is ").Float(vrrpSkew(vrrpKAPriority), 3).
		Str("s, its full master-down ").Float(vrrpMasterDown(vrrpKAPriority), 3).
		Str("s), proving the Skew_Time path").String())
	return nil
}

func vrrpDiagnosticOutput(namespace string, argv []string) string {
	result, _ := guestRun(context.Background(), namespace, argv, nil)
	var tb textbuf.Buffer
	return tb.Str(result.Stdout).Str(result.Stderr).String()
}

func (l *vrrpLab) diagnostics() {
	fmt.Fprintln(os.Stderr, "\n--- diagnostics ---") //nolint:errcheck // evidence diagnostics
	if l.zeLines != nil {
		fmt.Fprint(os.Stderr, "ze log tail:\n", strings.Join(lastLines(l.zeLines.snapshot(), 80), "")) //nolint:errcheck // evidence diagnostics
	}
	fmt.Fprintf(os.Stderr, "\nkeepalived version: %s\n", l.keepalivedVersion(context.Background())) //nolint:errcheck // evidence diagnostics
	fmt.Fprintf(os.Stderr, "keepalived state markers: %v\n", l.kaStates())                          //nolint:errcheck // evidence diagnostics
	for _, namespace := range []string{l.names.zeNS, l.names.kaNS, l.names.obNS} {
		fmt.Fprintf(os.Stderr, "\n%s links:\n%s", namespace,
			vrrpDiagnosticOutput(namespace, []string{"ip", "addr"})) //nolint:errcheck // evidence diagnostics
		fmt.Fprintf(os.Stderr, "%s neigh:\n%s", namespace,
			vrrpDiagnosticOutput(namespace, []string{"ip", "neigh", "show"})) //nolint:errcheck // evidence diagnostics
	}
	fmt.Fprintf(os.Stderr, "\n%s macvlan detail:\n%s", l.names.zeNS,
		vrrpDiagnosticOutput(l.names.zeNS, []string{"ip", "-d", "link", "show"})) //nolint:errcheck // evidence diagnostics
	sysctls := "grep -H . /proc/sys/net/ipv4/conf/*/arp_ignore " +
		"/proc/sys/net/ipv4/conf/*/arp_filter /proc/sys/net/ipv4/conf/*/rp_filter " +
		"/proc/sys/net/ipv6/conf/*/disable_ipv6 2>/dev/null | grep -vE '/(default|lo)/'"
	fmt.Fprintf(os.Stderr, "%s dataplane sysctls:\n%s", l.names.zeNS,
		vrrpDiagnosticOutput(l.names.zeNS, []string{"sh", "-c", sysctls})) //nolint:errcheck // evidence diagnostics
	fmt.Fprintf(os.Stderr, "\n%s bridge:\n%s", l.names.lanNS,
		vrrpDiagnosticOutput(l.names.lanNS, []string{"ip", "link", "show", "master", l.names.bridge})) //nolint:errcheck // evidence diagnostics
	if l.captureLines != nil {
		fmt.Fprint(os.Stderr, "capture tail:\n", strings.Join(lastLines(l.captureLines.snapshot(), 60), "")) //nolint:errcheck // evidence diagnostics
	}
	var tb textbuf.Buffer
	fmt.Fprintln(os.Stderr, tb.Str("artifacts kept in: ").Str(l.work).String()) //nolint:errcheck // evidence diagnostics
}
func lastLines(lines []string, count int) []string {
	if len(lines) <= count {
		return lines
	}
	return lines[len(lines)-count:]
}

func runVRRPGuest(ctx context.Context, root string, selected []string) (GuestLabReport, error) {
	report := GuestLabReport{Lab: "vrrp-keepalived", Selected: append([]string(nil), selected...), Verdict: VerdictUnspecified}
	if err := requireGuestCommands("ip", "ping", "tcpdump", "keepalived"); err != nil {
		return report, err
	}
	names := newVRRPNames()
	if err := probeVRRPKernel(ctx, names); err != nil {
		return report, err
	}
	binary, err := buildGuestZe(ctx, root, "ze-vrrp-keepalived",
		guestEvidenceZeKey, "ZE_VRRP_KEEPALIVED_ZE_BINARY")
	if err != nil {
		return report, err
	}
	report.Verdict = VerdictPass
	for _, name := range selected {
		lab, err := newVRRPLab(root, binary, name, names)
		if err != nil {
			return report, err
		}
		scenario := GuestScenario{Name: name, Verdict: VerdictUnspecified}
		success := false
		scenarioErr := lab.setup(ctx)
		if scenarioErr == nil {
			switch name {
			case vrrpQS1:
				scenarioErr = lab.runQS1(ctx)
			case vrrpQS2:
				scenarioErr = lab.runQS2(ctx)
			case vrrpQS3:
				scenarioErr = lab.runQS3(ctx)
			}
		}
		if scenarioErr != nil {
			scenario.Verdict = VerdictFail
			scenario.Failure = scenarioErr.Error()
			scenario.Artifacts = []string{
				lab.work, lab.pcap, filepath.Join(lab.work, "ze.conf"),
				filepath.Join(lab.work, "keepalived.conf"),
			}
			report.Verdict = VerdictFail
			fmt.Fprintf(os.Stderr, "FAIL: %s: %s\n", name, scenarioErr) //nolint:errcheck // evidence verdict
			lab.diagnostics()
		} else {
			scenario.Verdict = VerdictPass
			success = true
		}
		scenario.Details = append([]string(nil), lab.details...)
		lab.teardown(success)
		report.Cleanup = append(report.Cleanup, lab.cleanup...)
		report.Artifacts = append(report.Artifacts, scenario.Artifacts...)
		report.Scenarios = append(report.Scenarios, scenario)
	}
	return report, nil
}
