package deployment

// The VPP interface proof, driven as functions.
//
// Goal: pin the parts of the run that the argv comparison beside the script
// (internal/le/deployment/vppiface_test.go) cannot reach. These parts are the
// scenario table's own invariants, the container and daemon arguments and the
// rule that a probe uses to produce an answer. Method: call each builder and
// read what it answers, with no Docker and no container anywhere.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureVPPIface answers a run over a checkout carrying the one file the build
// tags derive from.
func fixtureVPPIface(t *testing.T) *vppIface {
	t.Helper()

	tree := t.TempDir()
	manifest := "ze_bgp internal/component/bgp\nze_vpp internal/component/vpp\nze_web internal/component/web\n"
	if err := os.WriteFile(filepath.Join(tree, "feature-gates.txt"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("write the fixture manifest: %v", err)
	}

	return &vppIface{
		Tree: tree, Image: VPPImage, Platform: VPPPlatform, Goarch: VPPGoarch,
		SocketWait: VPPSocketWait, ScenarioWait: VPPScenarioWait, Progress: io.Discard,
	}
}

// VALIDATES: the table declares four features, each with its own configuration
// file, its own probe and something for that probe to look for.
// PREVENTS: a fifth scenario added with a field left empty, which would probe
// nothing and pass on the first poll.
func TestEveryScenarioNamesAFileAProbeAndANeedle(t *testing.T) {
	if len(vppScenarios) != 4 {
		t.Fatalf("the table holds %d scenarios, want 4", len(vppScenarios))
	}

	files := map[string]bool{}
	for i := range vppScenarios {
		one := &vppScenarios[i]
		switch {
		case one.feature == "":
			t.Errorf("scenario %d has no feature name", i)
		case one.file == "":
			t.Errorf("scenario %q writes no configuration file", one.feature)
		case one.config == "":
			t.Errorf("scenario %q has an empty configuration", one.feature)
		case one.probe == "":
			t.Errorf("scenario %q puts no query to VPP", one.feature)
		case len(one.needles) == 0:
			t.Errorf("scenario %q looks for nothing in VPP's answer", one.feature)
		case one.provenDetail == "" || one.missingDetail == "":
			t.Errorf("scenario %q has no sentence for one of its two outcomes", one.feature)
		}
		if files[one.file] {
			t.Errorf("two scenarios write %q, so one overwrites the other", one.file)
		}
		files[one.file] = true

		if len(one.needsPlugins) > 0 && one.skipDetail == "" {
			t.Errorf("scenario %q is gated on a plugin and has no skip reason", one.feature)
		}
		if len(one.needsPlugins) == 0 && one.skipDetail != "" {
			t.Errorf("scenario %q carries a skip reason it can never print", one.feature)
		}
	}
}

// VALIDATES: LCP is enabled in exactly one scenario's configuration.
// PREVENTS: the failure the comment in vppifacescenarios.go names. When LCP is
// enabled, ze creates an lcp_itf_pair for every loopback. This fails the whole
// apply on a VPP build without linux_cp_plugin.so. Thus, a scenario that enables
// it without a plugin gate fails because of its own configuration.
func TestOnlyTheGatedScenarioEnablesLCP(t *testing.T) {
	for i := range vppScenarios {
		one := &vppScenarios[i]
		enabled := strings.Contains(one.config, "lcp { enabled true;")
		gated := len(one.needsPlugins) > 0
		if enabled && !gated {
			t.Errorf("scenario %q enables LCP without gating on the plugin", one.feature)
		}
		if enabled && one.feature != "lcp-pair" {
			t.Errorf("scenario %q enables LCP and is not the LCP proof", one.feature)
		}
	}
}

// VALIDATES: the container mounts the checkout at /src and the scratch directory
// at /run/vpp, runs privileged on the named platform, and does not start the
// image's own entry point.
// PREVENTS: a rewrite that drops --privileged, which VPP needs to claim memory
// and create devices, and whose absence looks like a ze defect.
func TestTheVPPContainerMountsTheTreeAndTheScratch(t *testing.T) {
	run := fixtureVPPIface(t)
	work := filepath.Join(run.Tree, "tmp", "evidence", "vpp-iface-1")
	line := strings.Join(run.containerArgs("ze-vpp-iface-1", work), " ")

	for _, want := range []string{
		"--privileged", "--platform linux/amd64", "--entrypoint sleep",
		run.Tree + ":/src", work + ":/run/vpp", VPPImage,
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the container argv does not carry %q:\n%s", want, line)
		}
	}
}

// VALIDATES: ze is started with the `start` keyword in front of its
// configuration path.
// PREVENTS: the regression learned 1248 records. The CLI no longer supports the
// bare `ze <config>` launch form. Thus, a positional path now fails with
// "unknown command". This looks like a broken image instead of a wrong
// invocation.
func TestTheDaemonIsStartedWithTheStartKeyword(t *testing.T) {
	run := fixtureVPPIface(t)
	argv := run.daemonArgs("ze-vpp-iface-1", daemonRel(run.Goarch), "tunnel.conf")

	line := strings.Join(argv, " ")
	if !strings.HasSuffix(line, "start /run/vpp/tunnel.conf") {
		t.Errorf("the daemon argv does not end at `start /run/vpp/tunnel.conf`:\n%s", line)
	}
	if !strings.Contains(line, "/src/tmp/evidence/bin/ze-linux-amd64") {
		t.Errorf("the daemon argv does not name the cross-compiled binary inside the mount:\n%s", line)
	}
	for _, want := range []string{"ZE_STORAGE_BLOB=false", "ZE_CONFIG_DIR=/run/vpp/ze"} {
		if !strings.Contains(line, want) {
			t.Errorf("the daemon argv does not carry %q, so the run writes into the checkout:\n%s", want, line)
		}
	}
}

// VALIDATES: a query's words reach vppctl as separate arguments, behind the CLI
// socket the startup configuration creates.
// PREVENTS: a query passed as one string, which vppctl reads as an unknown
// command and answers with an error that the old matching rule then accepted as
// evidence.
func TestAQueryReachesVppctlAsWords(t *testing.T) {
	argv := vppctlArgs("show gre tunnel")
	want := []string{"vppctl", "-s", vppCLISock, "show", "gre", "tunnel"}
	if len(argv) != len(want) {
		t.Fatalf("the query argv is %v, want %v", argv, want)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Errorf("word %d is %q, want %q", i, argv[i], want[i])
		}
	}
}

// VALIDATES: the scratch directory holds VPP's startup file, every scenario's
// configuration, and the empty directory ze writes its own store into.
// PREVENTS: a scenario whose configuration was never written, which starts ze on
// a path that does not exist and reports the feature as absent.
func TestTheScratchDirectoryHoldsEveryConfiguration(t *testing.T) {
	run := fixtureVPPIface(t)
	work := t.TempDir()
	if err := run.writeScratch(work); err != nil {
		t.Fatalf("write the scratch directory: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(work, "startup.conf")) //nolint:gosec // a path this test made
	if err != nil {
		t.Fatalf("read the VPP startup file: %v", err)
	}
	if !strings.Contains(string(body), "cli-listen /run/vpp/cli.sock") {
		t.Errorf("the startup file does not create the CLI socket the queries use:\n%s", body)
	}

	for i := range vppScenarios {
		one := &vppScenarios[i]
		written, err := os.ReadFile(filepath.Join(work, one.file)) //nolint:gosec // a path this test made
		if err != nil {
			t.Errorf("read %s: %v", one.file, err)
			continue
		}
		if string(written) != one.config {
			t.Errorf("%s does not carry the scenario's configuration", one.file)
		}
	}

	if info, err := os.Stat(filepath.Join(work, "ze")); err != nil || !info.IsDir() {
		t.Errorf("the daemon's configuration directory was not made: %v", err)
	}
}

// VALIDATES: a needle is matched without regard to case.
// PREVENTS: a probe that misses because VPP capitalised a heading. Every needle
// is a VPP object name or a port number, and the case of the surrounding text
// varies between releases.
func TestANeedleIsMatchedWithoutRegardToCase(t *testing.T) {
	cases := []struct {
		text    string
		needles []string
		want    bool
	}{
		{"Rx     mdst0", []string{"rx", "both"}, true},
		{"BOTH   mdst0", []string{"rx", "both"}, true},
		{"span table is empty", []string{"rx", "both"}, false},
		{"", []string{"rx"}, false},
		{"anything", nil, false},
	}
	for _, one := range cases {
		if got := containsAny(one.text, one.needles); got != one.want {
			t.Errorf("containsAny(%q, %v) = %v, want %v", one.text, one.needles, got, one.want)
		}
	}
}

// VALIDATES: evidence is bounded at the same tail every other answer in this
// package is bounded by.
// PREVENTS: a VPP interface listing of unbounded length reaching a JSON
// document, where the lines that explain a failure are the last ones anyway.
func TestEvidenceIsBoundedAtTheSharedTail(t *testing.T) {
	var builder strings.Builder
	for i := range LogTailLines * 3 {
		builder.WriteString("line ")
		builder.WriteByte(byte('0' + i%10))
		builder.WriteByte('\n')
	}

	lines := tailLines(builder.String())
	if len(lines) != LogTailLines {
		t.Fatalf("a long answer produced %d lines, want %d", len(lines), LogTailLines)
	}
	if tailLines("") != nil {
		t.Error("an empty answer produced lines")
	}
	if got := tailLines("one\ntwo\n"); len(got) != 2 || got[1] != "two" {
		t.Errorf("a short answer produced %v, want the two lines it holds", got)
	}
}
