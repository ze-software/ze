package ipsec

import (
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// VALIDATES: dns.conf refuses every shape that would leave a scenario dialing a name
// nothing answers, or answering with an address no lab container holds.
// PREVENTS: a scenario whose name server starts with no record, or with one pointing
// outside the lab, so the checker waits out its timeout on a name Ze could never reach
// and the red reads as a Ze defect.
func TestDNSConfigRefusesWhatCannotAnswer(t *testing.T) {
	for name, content := range map[string]string{
		"no record at all":          "# only a comment\n",
		"one field":                 "vpn.example.net\n",
		"three fields":              "vpn.example.net 172.28.0.3 extra\n",
		"no dot in the name":        "vpn 172.28.0.3\n",
		"empty label":               "vpn..example.net 172.28.0.3\n",
		"label starts with hyphen":  "-vpn.example.net 172.28.0.3\n",
		"upper case":                "VPN.example.net 172.28.0.3\n",
		"not an address":            "vpn.example.net example\n",
		"ipv6 address":              "vpn.example.net fd00::3\n",
		"outside the lab":           "vpn.example.net 10.0.0.3\n",
		"the server's own address":  "vpn.example.net " + dnsIP + "\n",
		"the same name twice":       "vpn.example.net 172.28.0.3\nvpn.example.net 172.28.0.7\n",
		"more records than allowed": "x.example.net 172.28.0.3\n" + manyDNSRecords(dnsRecordsMax),
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, dnsConfigName), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if records, err := readDNSConfig(directory); err == nil {
				t.Fatalf("readDNSConfig accepted %q as %v", content, records)
			}
		})
	}
}

// VALIDATES: a missing dns.conf answers the named error, which says how to fix it.
func TestDNSConfigMissingNamesTheFix(t *testing.T) {
	if _, err := readDNSConfig(t.TempDir()); !errors.Is(err, errNoDNSConfig) {
		t.Fatalf("missing dns.conf = %v, want errNoDNSConfig", err)
	}
}

// VALIDATES: every record reaches dnsmasq as one --host-record, and dnsmasq is given
// no upstream and no hosts file, so only the declared names answer.
func TestDNSMasqServesOnlyTheDeclaredRecords(t *testing.T) {
	directory := t.TempDir()
	content := "# comment\n\nvpn.example.net 172.28.0.3\nnat.example.net 172.28.0.7\n"
	if err := os.WriteFile(filepath.Join(directory, dnsConfigName), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	records, err := readDNSConfig(directory)
	if err != nil {
		t.Fatal(err)
	}
	command := dnsmasqCommand(records)
	for _, want := range []string{"--no-resolv", "--no-hosts", "--listen-address=" + dnsIP,
		"--host-record=vpn.example.net,172.28.0.3", "--host-record=nat.example.net,172.28.0.7"} {
		if !slices.Contains(command, want) {
			t.Errorf("dnsmasq command %v lacks %s", command, want)
		}
	}
	if command[0] != "dnsmasq" {
		t.Errorf("dnsmasq command runs %s", command[0])
	}
}

// VALIDATES: the readiness wait accepts busybox nslookup's answer line in both of its
// forms, and never reads the server line as the answer.
func TestNSLookupAnswersReadsTheAnswerLine(t *testing.T) {
	address := netip.MustParseAddr("172.28.0.3")
	answered := "Server:\t\t172.28.0.8\nAddress:\t172.28.0.8:53\n\nName:\tvpn.example.net\nAddress: 172.28.0.3\n"
	if !nslookupAnswers(answered, address) {
		t.Errorf("nslookupAnswers missed the answer in %q", answered)
	}
	if !nslookupAnswers("Name: vpn.example.net\nAddress 1: 172.28.0.3\n", address) {
		t.Error("nslookupAnswers missed the older busybox form")
	}
	refused := "Server:\t\t172.28.0.8\nAddress:\t172.28.0.8:53\n\n** server can't find vpn.example.net: REFUSED\n"
	if nslookupAnswers(refused, address) {
		t.Errorf("nslookupAnswers read %q as an answer", refused)
	}
	if nslookupAnswers("Name: vpn.example.net\nAddress: 172.28.0.30\n", address) {
		t.Error("nslookupAnswers matched 172.28.0.30 as 172.28.0.3")
	}
}

// VALIDATES: Ze's resolv.conf names the lab name server alone.
func TestZeResolvConfNamesTheLabServer(t *testing.T) {
	mount, err := zeResolvConf(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if mount.Target != zeResolvConfTarget || !mount.ReadOnly {
		t.Errorf("mount = %#v", mount)
	}
	content, err := os.ReadFile(mount.Source)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(content), "nameserver ") != 1 || !strings.Contains(string(content), "nameserver "+dnsIP+"\n") {
		t.Errorf("resolv.conf = %q", content)
	}
}

func manyDNSRecords(count int) string {
	var builder strings.Builder
	for index := range count {
		builder.WriteString("host")
		builder.WriteByte(byte('a' + index))
		builder.WriteString(".example.net 172.28.0.3\n")
	}
	return builder.String()
}
