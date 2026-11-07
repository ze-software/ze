// Design: docs/architecture/testing/interop.md -- the IPsec lab name server.
// Related: register_dns.go -- the image and the two scenarios that use it.
// Related: fqdn.go -- the checkers that start it late and read what it served.
// Related: ipsec.go -- prepareScenario, which adds the container and points Ze at it.
package ipsec

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// dnsConfigName keys the name server exactly as nat.conf keys the NAT box: a
	// scenario that carries it gets the container, and no other scenario does.
	dnsConfigName = "dns.conf"

	dnsImage = "ze-ipsec-dns"
	dnsPeer  = "dns"

	// dnsHost is the name server's host octet on the lab network. 2 to 5 are the
	// daemons and the NAT box, 6 and 7 the NAT box's public addresses.
	dnsHost = 8
	// dnsIP is dnsHost on the lab network: the one address Ze's resolv.conf names.
	dnsIP = "172.28.0.8"

	// dnsRecordsMax bounds a scenario's dns.conf. A scenario names the one or two
	// peers it dials by name, so a longer file is a mistake rather than a need.
	dnsRecordsMax = 8
	// dnsNameOctetsMax is the longest presentation-form name DNS carries (RFC 1035
	// Section 2.3.4 limits a name to 255 octets on the wire, 253 in text).
	dnsNameOctetsMax = 253

	// zeResolvConfTarget is where Ze's resolver reads its servers. Docker mounts its
	// own file there unless the container names a mount for that path, which the
	// lab does, so Ze asks the lab name server and nothing else.
	zeResolvConfTarget = "/etc/resolv.conf"

	// dnsAnswerTimeout bounds the wait for dnsmasq to answer once started. It binds
	// one socket on a local address, so it answers in well under a second.
	dnsAnswerTimeout = 20 * time.Second
)

// dnsRecord is one line of dns.conf: a name and the IPv4 address it answers with.
type dnsRecord struct {
	name    string
	address netip.Addr
}

// readDNSConfig reads a scenario's dns.conf, one `<name> <ipv4>` record per line.
//
// It refuses every shape that would leave the scenario dialing a name nothing answers,
// or answering with an address no lab container holds: an empty file, a malformed
// line, a name twice, an address outside the lab network, and the server's own address.
func readDNSConfig(directory string) ([]dnsRecord, error) {
	content, err := readFileUnder(directory, dnsConfigName)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errNoDNSConfig
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dnsConfigName, err)
	}
	server := netip.MustParseAddr(dnsIP)
	records := make([]dnsRecord, 0, dnsRecordsMax)
	for raw := range strings.SplitSeq(string(content), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		record, parseErr := parseDNSRecord(line, server)
		if parseErr != nil {
			return nil, fmt.Errorf("%s in %s: %w", dnsConfigName, filepath.Base(directory), parseErr)
		}
		for _, held := range records {
			if held.name == record.name {
				return nil, fmt.Errorf("%s in %s names %s twice; keep one line per name",
					dnsConfigName, filepath.Base(directory), record.name)
			}
		}
		if len(records) == dnsRecordsMax {
			return nil, fmt.Errorf("%s in %s holds more than %d records; a scenario dials one or two names",
				dnsConfigName, filepath.Base(directory), dnsRecordsMax)
		}
		records = append(records, record)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("%s in %s holds no record, so the name server would answer nothing; "+
			"add a line <name> <ipv4>, or delete the file", dnsConfigName, filepath.Base(directory))
	}
	return records, nil
}

// parseDNSRecord reads one non-comment line of dns.conf.
func parseDNSRecord(line string, server netip.Addr) (dnsRecord, error) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return dnsRecord{}, fmt.Errorf("line %q is not <name> <ipv4>", line)
	}
	name := fields[0]
	if err := validDNSName(name); err != nil {
		return dnsRecord{}, err
	}
	address, err := netip.ParseAddr(fields[1])
	if err != nil {
		return dnsRecord{}, fmt.Errorf("line %q: %q is not an address: %w", line, fields[1], err)
	}
	if !address.Is4() {
		return dnsRecord{}, fmt.Errorf("line %q: %s is not IPv4, and IKE over IPv6 is not supported", line, address)
	}
	if !networkPrefix.Contains(address) {
		return dnsRecord{}, fmt.Errorf("line %q: %s is outside the lab network %s, so no container holds it",
			line, address, networkPrefix)
	}
	if address == server {
		return dnsRecord{}, fmt.Errorf("line %q: %s is the name server's own address", line, address)
	}
	return dnsRecord{name: name, address: address}, nil
}

// validDNSName refuses a name dnsmasq would not serve as written: it must hold a dot,
// and every label must be letters, digits and inner hyphens.
func validDNSName(name string) error {
	if len(name) > dnsNameOctetsMax {
		return fmt.Errorf("name %q is longer than %d octets", name, dnsNameOctetsMax)
	}
	if !strings.Contains(name, ".") {
		return fmt.Errorf("name %q has no dot; give a fully qualified name such as vpn.example.net", name)
	}
	for label := range strings.SplitSeq(name, ".") {
		if label == "" {
			return fmt.Errorf("name %q has an empty label", name)
		}
		if strings.HasPrefix(label, "-") {
			return fmt.Errorf("name %q has a label that starts with a hyphen", name)
		}
		if strings.HasSuffix(label, "-") {
			return fmt.Errorf("name %q has a label that ends with a hyphen", name)
		}
		for _, character := range label {
			if !dnsLabelCharacter(character) {
				return fmt.Errorf("name %q holds %q, which a host name cannot carry", name, character)
			}
		}
	}
	return nil
}

func dnsLabelCharacter(character rune) bool {
	if character >= 'a' && character <= 'z' {
		return true
	}
	if character >= '0' && character <= '9' {
		return true
	}
	return character == '-'
}

// dnsPeerConfig is the name server's container. It starts with no server running
// (Dockerfile.dns): startDNS starts dnsmasq when the checker wants the name to answer,
// which is what lets a scenario boot Ze while DNS is down.
func dnsPeerConfig(container string) interoplab.PeerConfig {
	return interoplab.PeerConfig{
		Name:      dnsPeer,
		Container: container,
		Image:     dnsImage,
		Host:      dnsHost,
		// Every peer of this lab but FRR holds exactly NET_ADMIN (capabilityNetAdmin).
		Capabilities: []string{capabilityNetAdmin},
		// exec, so tini signals the reader itself and a stop is immediate.
		Command: []string{"-c", "exec tail -f /dev/null"},
		Ready: &interoplab.ReadyProbe{
			Command:  []string{"sh", "-c", "command -v dnsmasq >/dev/null"},
			Timeout:  30 * time.Second,
			Interval: time.Second,
		},
	}
}

// zeResolvConf writes the resolv.conf Ze's container mounts, naming the lab name
// server alone, and answers the mount.
//
// One attempt with a one-second timeout: while the server is down a query is
// refused at once (no listener on port 53), so a connect cycle fails fast and the
// scenario's DNS-down phase measures Ze's backoff rather than the resolver's.
func zeResolvConf(workDir string) (interoplab.Mount, error) {
	var tb textbuf.Buffer
	content := tb.Str("nameserver ").Str(dnsIP).Str("\noptions timeout:1 attempts:1\n").String()
	path := filepath.Join(workDir, "resolv.conf")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // resolv.conf is world-readable by convention, and Ze may not run as the file's owner
		return interoplab.Mount{}, fmt.Errorf("write Ze's resolv.conf: %w", err)
	}
	return interoplab.Mount{Source: path, Target: zeResolvConfTarget, ReadOnly: true}, nil
}

// dnsmasqOptions are dnsmasqCommand's fixed arguments, ahead of the records.
var dnsmasqOptions = []string{"dnsmasq", "--no-resolv", "--no-hosts", "--no-poll",
	"--bind-interfaces", "--listen-address=" + dnsIP, "--pid-file=/run/dnsmasq.pid"}

// dnsmasqCommand is the server the checker starts: one --host-record per dns.conf line,
// bound to the lab address alone, with no upstream and no hosts file, so a name the
// scenario did not declare never answers. dnsmasq forks into the background, so the
// exec returns once it is listening or has failed.
func dnsmasqCommand(records []dnsRecord) []string {
	command := make([]string, 0, len(dnsmasqOptions)+len(records))
	command = append(command, dnsmasqOptions...)
	var tb textbuf.Buffer
	for _, record := range records {
		command = append(command, tb.Reset().Str("--host-record=").Str(record.name).Byte(',').
			Str(record.address.String()).String())
	}
	return command
}

// startDNS starts the lab name server and waits until it answers every record of the
// scenario's dns.conf. A checker calls it at the point its scenario wants the names to
// resolve: at once, or after it has proved what Ze does while they cannot.
func (l *scenarioLab) startDNS(ctx context.Context) error {
	records, err := readDNSConfig(l.check.Source.Directory)
	if err != nil {
		return err
	}
	if _, err := l.exec(ctx, dnsPeer, dnsmasqCommand(records)...); err != nil {
		return fmt.Errorf("start dnsmasq on the lab name server: %w", err)
	}
	for _, record := range records {
		if err := l.waitDNSAnswer(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

// waitDNSAnswer waits until the name server answers record's name with its address,
// asked over the lab network as Ze asks it.
func (l *scenarioLab) waitDNSAnswer(ctx context.Context, record dnsRecord) error {
	var tb textbuf.Buffer
	description := tb.Str("lab name server answers ").Str(record.name).Str(" with ").
		Str(record.address.String()).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: dnsAnswerTimeout, Interval: time.Second, Description: description,
	}, func(probe context.Context) (string, error) {
		return l.exec(probe, dnsPeer, "nslookup", record.name, dnsIP)
	}, func(output string) bool {
		return nslookupAnswers(output, record.address)
	})
	return err
}

// nslookupAnswers reports whether busybox nslookup's output carries address as an
// answer. Its lines read `Address: 172.28.0.3` (or `Address 1: ...` on older busybox),
// and the server line carries the server's own address with its port, which
// parseDNSRecord already keeps distinct from every record, so a whole-field match on a
// line's last field never reads the server as the answer.
func nslookupAnswers(output string, address netip.Addr) bool {
	for line := range strings.SplitSeq(output, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "Address") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		answer, err := netip.ParseAddr(fields[len(fields)-1])
		if err != nil {
			continue
		}
		if answer == address {
			return true
		}
	}
	return false
}

// errNoDNSConfig answers a scenario that asks for the name server and carries no
// dns.conf, so a checker that calls startDNS by mistake fails naming the fix.
var errNoDNSConfig = errors.New("this scenario carries no dns.conf, so the lab started no name server; " +
	"add a dns.conf with one <name> <ipv4> line per name the scenario dials")
