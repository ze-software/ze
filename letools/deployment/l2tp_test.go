// Related: l2tp.go -- the L2TP peer proof these tests call as a function

package deployment

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// tree builds a checkout the build step can read: a manifest declaring three
// gates, and the go.mod that makes it a module.
func tree(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"feature-gates.txt": "# a comment\nze_bgp internal/component/bgp\nze_l2tp internal/component/l2tp\nze_web internal/component/web\n",
		"go.mod":            "module example.test/m\n\ngo 1.26\n",
	}
	for rel, body := range files {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			t.Fatalf("write the fixture %s: %v", rel, err)
		}
	}
	return root
}

func fixtureL2TP(t *testing.T) *L2TP {
	t.Helper()

	run := NewL2TP(tree(t))
	run.Image = "alpine:3.20"
	run.Platform = "linux/amd64"
	run.Goarch = "amd64"
	return run
}

// VALIDATES: the daemon is cross-compiled with EVERY gate the manifest
// declares, derived at run time.
// PREVENTS: the regression this derivation exists for. ze_l2tp became a gate on
// 2026-07-24 and no evidence script was updated, so for a month this proof
// built a daemon with no L2TP in it and died on "unknown top-level keyword:
// l2tp" before reaching a kernel feature.
func TestTheDaemonIsBuiltWithEveryFeatureGate(t *testing.T) {
	run := fixtureL2TP(t)

	argv, err := run.buildArgs()
	if err != nil {
		t.Fatalf("derive the build argv: %v", err)
	}

	line := strings.Join(argv, " ")
	for _, want := range []string{"ze_bgp", "ze_l2tp", "ze_web", "ze_core", "ze_distro"} {
		if !strings.Contains(line, want) {
			t.Errorf("the build argv does not carry %q:\n%s", want, line)
		}
	}
	if !strings.HasSuffix(line, "./cmd/ze") {
		t.Errorf("the build argv does not end at ./cmd/ze:\n%s", line)
	}
	if !strings.Contains(line, filepath.Join("tmp", "evidence", "bin", "ze-linux-amd64")) {
		t.Errorf("the build argv does not name the arch-suffixed output:\n%s", line)
	}
}

// VALIDATES: a manifest that declares no gate stops the run rather than
// building a daemon with every feature compiled out.
// PREVENTS: the fail-open scripts/evidence/feature_tags.py has -- measured
// 2026-08-26, it answers "ze_core ze_distro" for a manifest of comments, and the
// caller then reads the daemon's "unknown top-level keyword: l2tp" as a
// protocol defect rather than as a build that carried no L2TP.
func TestAManifestWithNoGateStopsTheBuild(t *testing.T) {
	run := fixtureL2TP(t)
	manifest := filepath.Join(run.Tree, "feature-gates.txt")
	if err := os.WriteFile(manifest, []byte("# nothing here\n"), 0o644); err != nil {
		t.Fatalf("empty the fixture manifest: %v", err)
	}

	if argv, err := run.buildArgs(); err == nil {
		t.Fatalf("a manifest with no gate answered the build argv %v, want an error", argv)
	}
}

// VALIDATES: the container mounts the checkout at /src and the scratch
// directory at /run/l2tp, and runs privileged on the named platform.
// PREVENTS: a rewrite that drops --privileged, which the peer needs to open a
// PPP device, and whose absence looks like a protocol failure.
func TestTheContainerMountsTheTreeAndTheScratch(t *testing.T) {
	run := fixtureL2TP(t)
	work := filepath.Join(run.Tree, "tmp", "evidence", "l2tp-peer-1")

	line := strings.Join(run.containerArgs("ze-l2tp-evidence-1", work), " ")
	for _, want := range []string{
		"run --rm --detach --privileged",
		"--platform linux/amd64",
		"--name ze-l2tp-evidence-1",
		"-v " + run.Tree + ":/src",
		"-v " + work + ":/run/l2tp",
		"-w /src",
		"alpine:3.20 sleep infinity",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the container argv does not carry %q:\n%s", want, line)
		}
	}
}

// VALIDATES: the daemon is started inside the container with the environment
// the proof needs, reading its configuration from standard input.
// PREVENTS: the kernel-probe skip or the blob-storage switch going missing,
// each of which makes the daemon fail for a reason that has nothing to do with
// L2TP.
func TestTheDaemonRunsInTheContainerWithItsEnvironment(t *testing.T) {
	run := fixtureL2TP(t)

	line := strings.Join(run.daemonArgs("ze-l2tp-evidence-1", "tmp/evidence/bin/ze-linux-amd64"), " ")
	for _, want := range []string{
		"exec --interactive",
		"--env ZE_LOG_L2TP=debug",
		"--env ze.l2tp.skip-kernel-probe=true",
		"--env ZE_STORAGE_BLOB=false",
		"--env ZE_CONFIG_DIR=/run/l2tp/ze",
		"/src/tmp/evidence/bin/ze-linux-amd64 -",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the daemon argv does not carry %q:\n%s", want, line)
		}
	}
}

// VALIDATES: the peer configuration and the daemon configuration agree about
// the port, which is the one number that decides whether a tunnel can form.
// PREVENTS: a silent edit to either side leaving a proof that can never pass,
// reported as an L2TP failure.
func TestThePeerAndTheDaemonAgreeAboutThePort(t *testing.T) {
	peerPort := "lns = 127.0.0.1"
	if !strings.Contains(PeerConfig, peerPort) {
		t.Errorf("the peer configuration does not dial the daemon:\n%s", PeerConfig)
	}
	if !strings.Contains(PeerConfig, "port = "+strconv.Itoa(PeerPort)) {
		t.Errorf("the peer configuration does not bind port %d:\n%s", PeerPort, PeerConfig)
	}
	if !strings.Contains(DaemonConfig, "port "+strconv.Itoa(DaemonPort)) {
		t.Errorf("the daemon configuration does not bind port %d:\n%s", DaemonPort, DaemonConfig)
	}
	if PeerPort == DaemonPort {
		t.Error("the peer and the daemon bind the same port, so neither can start")
	}
}

// VALIDATES: the scratch files a run writes are the three the peer reads, and
// each lands under the directory mounted into the container.
// PREVENTS: a file written under a name the peer does not open, which arrives
// as a tunnel that never forms.
func TestTheRunWritesThePeersThreeFiles(t *testing.T) {
	run := fixtureL2TP(t)
	work := t.TempDir()

	if err := run.writeScratch(work); err != nil {
		t.Fatalf("write the scratch files: %v", err)
	}
	for name, want := range map[string]string{
		"xl2tpd.conf":  PeerConfig,
		"l2tp-secrets": PeerSecrets,
		"ppp-options":  PeerPPPOptions,
	} {
		body, err := os.ReadFile(filepath.Join(work, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(body) != want {
			t.Errorf("%s holds %q, want %q", name, body, want)
		}
	}
	if _, err := os.Stat(filepath.Join(work, "ze")); err != nil {
		t.Errorf("the daemon's config directory was not created: %v", err)
	}
}

// VALIDATES: a collector keeps every line it saw, reports the one asked about,
// and answers a BOUNDED tail.
// PREVENTS: an unbounded log reaching the payload, which would put a whole
// session's debug output into a JSON document nobody can read.
func TestTheCollectorIsBoundedAndAnswersWhatItSaw(t *testing.T) {
	seen := newCollector(sessionLine)
	for i := range LogTailLines * 3 {
		seen.add("line " + strconv.Itoa(i))
	}
	seen.add("ze> l2tp: session established for tunnel 1")

	if !seen.saw("session established") {
		t.Error("the collector did not report the line it was given")
	}
	if seen.saw("never written") {
		t.Error("the collector reported a line nothing wrote")
	}

	tail := seen.tailLines()
	if len(tail) != LogTailLines {
		t.Fatalf("the tail is %d lines, want the bound of %d", len(tail), LogTailLines)
	}
	if !strings.Contains(tail[len(tail)-1], "session established") {
		t.Errorf("the tail ends at %q, want the last line written", tail[len(tail)-1])
	}
}

// VALIDATES: the answer is structured data with kebab-case keys, so | json,
// | yaml and | table each render it (AC-7).
// PREVENTS: a tool that answers finished text, which no operator can pipe.
func TestTheL2TPReportIsStructuredData(t *testing.T) {
	report := L2TPReport{
		Peer: "xl2tpd", Image: "alpine:3.20", Container: "ze-l2tp-evidence-1",
		Established: false, LogTail: []string{"one", "two"},
	}

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("the report does not encode: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("the encoded report does not decode: %v", err)
	}
	for _, key := range []string{"peer", "image", "container", "established", "log-tail"} {
		if _, ok := back[key]; !ok {
			t.Errorf("the report has no %q key: %s", key, raw)
		}
	}
	for key := range back {
		if strings.ContainsAny(key, "_ ") || strings.ToLower(key) != key {
			t.Errorf("the key %q is not kebab-case", key)
		}
	}

	if text := report.Text(); !strings.Contains(text, "FAIL") || !strings.Contains(text, "one") {
		t.Errorf("a failed run does not render its log tail:\n%s", text)
	}
	report.Established = true
	if text := report.Text(); !strings.Contains(text, "OK") || !strings.Contains(text, "xl2tpd") {
		t.Errorf("an established session does not render as a pass:\n%s", text)
	}
}
