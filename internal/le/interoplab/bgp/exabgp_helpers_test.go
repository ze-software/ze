package bgp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

func TestExaBGPProfilePopulationMatchesNativeConfigs(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(root, "test", "exabgp-compat", "native", "*.conf"))
	if err != nil {
		t.Fatal(err)
	}
	binding := regexp.MustCompile(`exabgp-api ([A-Za-z0-9_.-]+)`)
	bound := make(map[string]struct{})
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if strings.Contains(text, ".run") || strings.Contains(text, ".py") {
			t.Errorf("script binding remains in %s", path)
		}
		for _, match := range binding.FindAllStringSubmatch(text, -1) {
			bound[match[1]] = struct{}{}
		}
	}
	for name := range exaProfiles {
		if _, ok := bound[name]; !ok {
			t.Errorf("compiled profile %s has no native config binding", name)
		}
	}
	for name := range bound {
		if name == "syslog" || name == "stderr" || name == "example-api-program" {
			continue
		}
		if _, ok := exaProfiles[name]; !ok {
			t.Errorf("native config binds missing compiled profile %s", name)
		}
	}
	if len(exaProfiles) != 46 {
		t.Fatalf("compiled ExaBGP profiles = %d, want 46", len(exaProfiles))
	}
}

func TestExaBGPProfileOrderAndTimingFixture(t *testing.T) {
	names := make([]string, 0, len(exaProfiles))
	for name := range exaProfiles {
		names = append(names, name)
	}
	sort.Strings(names)
	digest := sha256.New()
	for _, name := range names {
		profile := exaProfiles[name]
		fmt.Fprintf(digest, "%s|%d|%d|%d\n", name, profile.startup, profile.between, profile.wait) //nolint:errcheck // hash.Hash.Write never returns an error
		for _, command := range profile.commands {
			fmt.Fprintln(digest, command) //nolint:errcheck // hash.Hash.Write never returns an error
		}
	}
	const want = "48c4f2f8acd1c70d419252442ac9f8979df692d46887289bb68c7f5dc8f2606c"
	if got := hex.EncodeToString(digest.Sum(nil)); got != want {
		t.Fatalf("ExaBGP profile digest = %s, want %s", got, want)
	}
}

func TestExaBGPProfileEmitsExactCommands(t *testing.T) {
	profile := exaProfiles["api-announce"]
	profile.wait = 0
	var output bytes.Buffer
	if err := runExaProfile(strings.NewReader("shutdown\n"), &output, profile); err != nil {
		t.Fatal(err)
	}
	want := "announce route 1.1.0.0/24 next-hop 101.1.101.1\n" +
		"announce route 1.2.0.0/25 next-hop 101.1.101.1\n"
	if output.String() != want {
		t.Fatalf("api-announce output = %q, want %q", output.String(), want)
	}
}
func TestExaBGPServerCaseParserPreservesWireBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "case.ci")
	fixture := "option=asn:65000\n1:cmd:announce route 10.0.0.0/24 next-hop 1.2.3.4\n" +
		"1:raw:FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:0017:02:00000000\n"
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	frames, asn, err := readExaBGPCase(path)
	if err != nil || asn != 65000 || len(frames[1]) != 1 ||
		hex.EncodeToString(frames[1][0]) != "ffffffffffffffffffffffffffffffff00170200000000" {
		t.Fatalf("parsed case = ASN %d frames %x, error %v", asn, frames[1], err)
	}
}

func TestExaBGPEchoPreservesAnnounceAndWithdraw(t *testing.T) {
	event := `{"type":"update","neighbor":{"address":{"peer":"10.0.0.1"},"message":{"update":{"announce":{"ipv4 unicast":{"10.0.0.2":[{"nlri":"192.0.2.0/24"}]}},"withdraw":{"ipv4 unicast":[{"nlri":"198.51.100.0/24"}]}}}}}` + "\n"
	var output, diagnostic bytes.Buffer
	t.Setenv("TEST_MODE", "echo")
	if err := runExaBGPEcho(strings.NewReader(event), &output, &diagnostic); err != nil {
		t.Fatal(err)
	}
	want := "neighbor 10.0.0.1 announce route 192.0.2.0/24 next-hop 10.0.0.2\n" +
		"neighbor 10.0.0.1 withdraw route 198.51.100.0/24\n"
	if output.String() != want {
		t.Fatalf("echo output = %q, want %q", output.String(), want)
	}
	if !strings.Contains(diagnostic.String(), "starting, mode=echo") || !strings.Contains(diagnostic.String(), "exiting") {
		t.Fatalf("echo diagnostics = %q", diagnostic.String())
	}
}

func TestLGLabInjectionPopulationAndOrder(t *testing.T) {
	if len(lgInjections) != 36 {
		t.Fatalf("LG injections = %d, want 36", len(lgInjections))
	}
	digest := sha256.New()
	prefixes := make(map[string]int)
	for _, injection := range lgInjections {
		fmtLine := strings.Join([]string{injection.peer, injection.prefix, injection.nextHop, injection.asPath}, "|")
		_, _ = digest.Write([]byte(fmtLine + "\n"))
		prefixes[injection.prefix]++
	}
	const want = "b3d41488607e00041ced072c952e86d621a15eccb2e873dc7e691fd82ab773af"
	if got := hex.EncodeToString(digest.Sum(nil)); got != want {
		t.Fatalf("LG injection digest = %s, want %s", got, want)
	}
	keys := make([]string, 0, len(prefixes))
	for prefix, count := range prefixes {
		keys = append(keys, prefix)
		if count != 12 {
			t.Errorf("prefix %s injections = %d, want 12", prefix, count)
		}
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != "10.10.1.0/24,10.10.2.0/24,10.10.3.0/24" {
		t.Fatalf("LG prefixes = %v", keys)
	}
}
