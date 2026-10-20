//go:build ze_bgp

package bgp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	lepath "github.com/ze-software/ze/internal/le/le/path"
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
	slices.Sort(names)
	digest := sha256.New()
	for _, name := range names {
		profile := exaProfiles[name]
		fmt.Fprintf(digest, "%s|%d|%d|%d|%t\n", name, profile.startup, profile.between, profile.wait, profile.repeat) //nolint:errcheck // hash.Hash.Write never returns an error
		for _, command := range profile.commands {
			fmt.Fprintln(digest, command) //nolint:errcheck // hash.Hash.Write never returns an error
		}
	}
	const want = "48c4069355c40a1126a92c7967add302c49b3ef5aba908254f4f211e363482fa"
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

// exaWriteLimit accepts limit lines, then fails every write, as a pipe does
// once the bridge that read it has gone.
type exaWriteLimit struct {
	lines []string
	limit int
}

func (w *exaWriteLimit) Write(data []byte) (int, error) {
	if len(w.lines) >= w.limit {
		return 0, errors.New("broken pipe")
	}
	w.lines = append(w.lines, strings.TrimSuffix(string(data), "\n"))
	return len(data), nil
}

// TestExaBGPRepeatProfileOutlivesClosedInput proves a repeating profile keeps
// cycling when its input is already closed, as upstream watchdog.run loops
// until its parent dies and never reads stdin. It ends only when a write fails.
func TestExaBGPRepeatProfileOutlivesClosedInput(t *testing.T) {
	profile := exaProfiles["watchdog"]
	profile.between = time.Millisecond
	output := &exaWriteLimit{limit: 6}
	err := runExaProfile(strings.NewReader(""), output, profile)
	if err == nil {
		t.Fatal("repeat profile returned no error, want the failed write that ended it")
	}
	want := []string{
		"announce watchdog dnsr", "withdraw watchdog dnsr",
		"announce watchdog dnsr", "withdraw watchdog dnsr",
		"announce watchdog dnsr", "withdraw watchdog dnsr",
	}
	if !slices.Equal(output.lines, want) {
		t.Fatalf("watchdog output = %q, want %q", output.lines, want)
	}
}

// TestExaBGPRepeatProfileStopsAtShutdown proves `shutdown` on the input ends
// a repeating profile cleanly, after the first command it had already written.
func TestExaBGPRepeatProfileStopsAtShutdown(t *testing.T) {
	profile := exaProfiles["watchdog"]
	profile.between = time.Second
	output := &exaWriteLimit{limit: 100}
	if err := runExaProfile(strings.NewReader("shutdown\n"), output, profile); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(output.lines, []string{"announce watchdog dnsr"}) {
		t.Fatalf("watchdog output = %q, want the first announce only", output.lines)
	}
}

func TestExaBGPServerCaseParserPreservesWireBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "case.ci")
	fixture := "option=asn:65000\n1:cmd:announce route 10.0.0.0/24 next-hop 1.2.3.4\n" +
		"1:raw:FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:0017:02:00000000\n"
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	spec, err := readExaBGPCase(path)
	if err != nil || spec.asn != 65000 || len(spec.steps[1]) != 1 || spec.steps[1][0].kind != exabgpStepFrame ||
		hex.EncodeToString(spec.steps[1][0].frame) != "ffffffffffffffffffffffffffffffff00170200000000" {
		t.Fatalf("parsed case = ASN %d steps %+v, error %v", spec.asn, spec.steps[1], err)
	}
}

// TestExaBGPServerCaseParserOrdersSignalWithFrames checks that a `signal`
// directive lands in the connection's script at the point the fixture wrote it,
// because that position is what tells the mock how many frames must match
// before the runner is asked to reload ze. api-reload is the case that depends
// on it: its withdrawal is produced BY the reload, so a signal read as nothing,
// or read out of order, waits for a frame the speaker never sends.
func TestExaBGPServerCaseParserOrdersSignalWithFrames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "case.ci")
	fixture := "A1:raw:FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:0017:02:00000000\n" +
		"A2:signal:SIGUSR1\n" +
		"A3:raw:FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:001B:02:0004180200000000\n"
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	spec, err := readExaBGPCase(path)
	if err != nil {
		t.Fatal(err)
	}
	steps := spec.steps[1]
	if len(steps) != 3 {
		t.Fatalf("connection 1 script = %d steps, want 3: %+v", len(steps), steps)
	}
	if steps[0].kind != exabgpStepFrame || steps[2].kind != exabgpStepFrame {
		t.Fatalf("frame steps = %v and %v, want both %v", steps[0].kind, steps[2].kind, exabgpStepFrame)
	}
	if steps[1].kind != exabgpStepSignal || steps[1].signal != "SIGUSR1" {
		t.Fatalf("signal step = kind %v name %q, want kind %v name SIGUSR1", steps[1].kind, steps[1].signal, exabgpStepSignal)
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
	slices.Sort(keys)
	if strings.Join(keys, ",") != "10.10.1.0/24,10.10.2.0/24,10.10.3.0/24" {
		t.Fatalf("LG prefixes = %v", keys)
	}
}

// TestExaBGPServerASNumbersReadASDot checks that the case's `option=asn:` and
// the config's `peer-as` are read by asn.Parse: the asdot spelling 3.10 is AS
// 196618 in both, where a hand-written decimal parse refuses it.
func TestExaBGPServerASNumbersReadASDot(t *testing.T) {
	dir := t.TempDir()
	casePath := filepath.Join(dir, "case.ci")
	if err := os.WriteFile(casePath, []byte("option=asn:3.10\n1:raw:FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:0017:02:00000000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	spec, err := readExaBGPCase(casePath)
	if err != nil || spec.asn != 196618 || !spec.asnStated {
		t.Fatalf("option=asn:3.10 = ASN %d stated %v, error %v; want 196618 stated", spec.asn, spec.asnStated, err)
	}

	configPath := filepath.Join(dir, "case.conf")
	if err := os.WriteFile(configPath, []byte("neighbor 127.0.0.1 {\n\tpeer-as 3.10;\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	peerAS, found, err := peerASFromExaBGPConfig(configPath)
	if err != nil || !found || peerAS != 196618 {
		t.Fatalf("peer-as 3.10 = %d found %v, error %v; want 196618 found", peerAS, found, err)
	}
}
