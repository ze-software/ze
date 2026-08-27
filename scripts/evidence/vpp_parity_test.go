package main

// AC-11 for the real VPP deployment gate: effective-vpp.py and
// `le deployment vpp-test` perform the same eight scenarios.
//
// Recording stand-ins play Docker, VPP, ze-test, and the daemon. The proof
// compares every effect: three Go builds, 62 Docker calls, ten scratch payloads,
// the exit code, and the complete text view of the structured Go report. The 62
// calls are 57 distinct steps plus five deliberate cleanup polls.
//
// This file stays beside the script so step 14 deletes both in one commit.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/deployment"
)

const vppEvidenceTimeout = 2 * time.Minute

var vppEvidenceScratchFiles = []string{
	"startup.conf",
	"peer-script",
	"fib.conf",
	"mpls-peer-script",
	"mpls-fib.conf",
	"traffic.conf",
	"traffic-proto.conf",
	"traffic-dscp.conf",
	"traffic-mc.conf",
	"firewall.conf",
}

const vppEvidenceGoStub = `#!/bin/sh
sep=$(printf '\037')
line=""
for a in "$@"; do
  if [ -z "$line" ]; then line="$a"; else line="$line$sep$a"; fi
done
printf '%s\n' "$line" >> "$ZE_RECORD_GO"
prev=""
for a in "$@"; do
  if [ "$prev" = "-o" ]; then mkdir -p "$(dirname "$a")"; : > "$a"; chmod +x "$a"; fi
  prev="$a"
done
exit 0
`

const vppEvidenceDockerStub = `#!/bin/sh
record() {
  sep=$(printf '\037')
  line=""
  for a in "$@"; do
    if [ -z "$line" ]; then line="$a"; else line="$line$sep$a"; fi
  done
  printf '%s\n' "$line" >> "$ZE_RECORD_DOCKER"
}
state() { printf '%s/%s' "$ZE_VPP_STATE" "$1"; }
work() { cat "$ZE_VPP_WORK"; }
has() { [ -f "$(state "$1")" ]; }
set_state() { : > "$(state "$1")"; }
await_state() {
  for _ in $(seq 60); do
    if has "$1"; then return 0; fi
    sleep 0.05
  done
  return 1
}
clear_state() { rm -f "$(state "$1")"; }
daemon() {
  conf=""
  for a in "$@"; do
    case "$a" in /run/vpp/*.conf) conf=${a#/run/vpp/} ;; esac
  done
  w=$(work)
  case "$conf" in
    traffic.conf)
      if grep -q 'interface loop0' "$w/$conf"; then set_state pol-default; else set_state clear-pol-default; fi
      echo 'traffic-control config applied' >&2 ;;
    traffic-proto.conf)
      if grep -q 'interface loop0' "$w/$conf"; then set_state pol-tcp; else set_state clear-pol-tcp; fi
      echo 'traffic-control config applied' >&2 ;;
    traffic-dscp.conf)
      if grep -q 'interface loop0' "$w/$conf"; then set_state pol-cs6; else set_state clear-pol-cs6; fi
      echo 'traffic-control config applied' >&2 ;;
    traffic-mc.conf)
      if grep -q 'interface loop0' "$w/$conf"; then set_state pol-web; set_state pol-dns; else set_state clear-pol-multi; fi
      echo 'traffic-control config applied' >&2 ;;
    firewall.conf)
      if grep -q 'table wan' "$w/$conf"; then set_state acl; else set_state clear-acl; fi
      echo 'firewall config applied' >&2 ;;
  esac
  exec sleep 300
}
peer() {
  case "$*" in *mpls-peer-script*) marker=mpls ;; *) marker=fib ;; esac
  set_state "$marker"
  echo 'listening on 127.0.0.1'
  trap 'clear_state "$marker"; exit 0' TERM INT
  while has "$marker"; do sleep 0.05; done
}
mkdir -p "$ZE_VPP_STATE"
case "$1" in
  image) record "$@"; exit 0 ;;
  pull) record "$@"; exit 0 ;;
  rm) record "$@"; exit 0 ;;
  logs) record "$@"; echo 'vpp container log'; exit 0 ;;
  run)
    record "$@"
    for a in "$@"; do
      case "$a" in *:/run/vpp) printf '%s' "${a%:/run/vpp}" > "$ZE_VPP_WORK" ;; esac
    done
    echo deadbeef
    exit 0 ;;
  exec)
    case "$*" in
      *'vpp -c /run/vpp/startup.conf'*)
        record "$@"; w=$(work); : > "$w/api.sock"; : > "$w/cli.sock"; exit 0 ;;
      *'TestVPPRealDataplaneInstalls'*)
        record "$@"
        printf '%s\n' 'ze-vpp-ipsec:spd-id=41' 'ze-vpp-ipsec:sad-id=42' \
          'ze-vpp-ipsec:close-removed-spi=43' 'ze-vpp-ipsec:close-removed-spd-id=44'
        exit 0 ;;
      *'show ipsec sa 0'*)
        record "$@"
        printf '%s' 'salt 0xdeadbeef aes-gcm-256 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f integrity alg none encap-copy-ecn decap-copy-ecn'
        exit 0 ;;
      *'show ipsec sa'*)
        record "$@"
        printf '%s' '[0] spi 287454020 protocol:esp tunnel
[1] spi 1432778632 protocol:esp tunnel inbound'
        exit 0 ;;
      *'show ipsec all'*)
        record "$@"
        printf '%s' 'spd 41
41 -> loop0
priority -100 action bypass type ip4-outbound protocol any
priority -2000 action protect type ip4-outbound protocol any'
        exit 0 ;;
      *'create loopback interface'*) record "$@"; echo loop0; exit 0 ;;
      *'set interface state loop0 up'*) record "$@"; exit 0 ;;
      *'show interface features loop0'*)
        record "$@"
        if has pol-default; then echo 'policer-output'; fi
        if has pol-tcp || has pol-cs6 || has pol-web || has pol-dns; then echo 'policer-classify'; fi
        exit 0 ;;
      *'show interface'*) record "$@"; echo 'loop0 7 up'; exit 0 ;;
      *'show version'*) record "$@"; echo 'vpp v24.02-release'; exit 0 ;;
      *'show ip fib 10.20.0.0/24'*)
        record "$@"
        if [ "${ZE_FIB_QUERY_EXIT:-0}" != 0 ] && ! has fib-failed; then
          set_state fib-failed
          echo 'show ip fib 10.20.0.0/24: unknown input' >&2
          exit "$ZE_FIB_QUERY_EXIT"
        fi
        if has fib; then echo '10.20.0.0/24 via 10.0.0.1'; fi
        exit 0 ;;
      *'show ip fib 10.30.0.0/24'*)
        record "$@"; if has mpls; then echo '10.30.0.0/24 via 10.0.0.1 label 100'; fi; exit 0 ;;
      *'show policer'*)
        w=$(work)
        if has pol-default && ! grep -q 'interface loop0' "$w/traffic.conf"; then await_state clear-pol-default; fi
        if has pol-tcp && ! grep -q 'interface loop0' "$w/traffic-proto.conf"; then await_state clear-pol-tcp; fi
        if has pol-cs6 && ! grep -q 'interface loop0' "$w/traffic-dscp.conf"; then await_state clear-pol-cs6; fi
        if has pol-web && ! grep -q 'interface loop0' "$w/traffic-mc.conf"; then await_state clear-pol-multi; fi
        record "$@"
        has pol-default && echo 'ze/loop0/default'
        has pol-tcp && echo 'ze/loop0/tcp'
        has pol-cs6 && echo 'ze/loop0/cs6'
        has pol-web && echo 'ze/loop0/web'
        has pol-dns && echo 'ze/loop0/dns'
        if has clear-pol-default; then clear_state pol-default; clear_state clear-pol-default; fi
        if has clear-pol-tcp; then clear_state pol-tcp; clear_state clear-pol-tcp; fi
        if has clear-pol-cs6; then clear_state pol-cs6; clear_state clear-pol-cs6; fi
        if has clear-pol-multi; then clear_state pol-web; clear_state pol-dns; clear_state clear-pol-multi; fi
        exit 0 ;;
      *'show classify tables'*)
        record "$@"
        if has pol-tcp || has pol-cs6 || has pol-web || has pol-dns; then echo 'table 0 sessions'; else echo 'No classifier tables configured'; fi
        exit 0 ;;
      *'show acl-plugin interface loop0'*)
        record "$@"; has acl && echo 'input acl 0'; exit 0 ;;
      *'show acl-plugin acl'*)
        if has acl && ! grep -q 'table wan' "$(work)/firewall.conf"; then await_state clear-acl; fi
        record "$@"
        has acl && echo 'ze/wan/input'
        if has clear-acl; then clear_state acl; clear_state clear-acl; fi
        exit 0 ;;
      *'pkill -TERM -f ze-test-linux-amd64'*)
        record "$@"; clear_state fib; clear_state mpls; exit 0 ;;
      *'/ze-test-linux-amd64 peer '*) record "$@"; peer "$@"; exit 0 ;;
      *'start /run/vpp/'*) record "$@"; daemon "$@" ;;
      *) record "$@"; exit 0 ;;
    esac ;;
esac
record "$@"
exit 0
`

type vppEvidenceRun struct {
	code    int
	stdout  string
	report  deployment.VPPReport
	docker  []string
	build   []string
	scratch map[string]string
}

func vppEvidenceFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range map[string]string{
		"go.mod":            "module example.test/m\n\ngo 1.26\n",
		"feature-gates.txt": "ze_bgp internal/component/bgp\nze_vpp internal/component/vpp\n",
	} {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	dir := filepath.Join(root, "scripts", "evidence")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("create script directory: %v", err)
	}
	for _, name := range []string{"effective-vpp.py", "feature_tags.py"} {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o600); err != nil {
			t.Fatalf("copy %s: %v", name, err)
		}
	}
	return root
}

func vppEvidenceStubs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{"docker": vppEvidenceDockerStub, "go": vppEvidenceGoStub} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil { //nolint:gosec // executable test stand-in
			t.Fatalf("write %s stub: %v", name, err)
		}
	}
	return dir
}

func vppEvidenceEnv(stubs, record string, extra ...string) []string {
	base := []string{
		"PATH=" + stubs + string(os.PathListSeparator) + os.Getenv("PATH"),
		"ZE_RECORD_DOCKER=" + filepath.Join(record, "docker"),
		"ZE_RECORD_GO=" + filepath.Join(record, "go"),
		"ZE_VPP_WORK=" + filepath.Join(record, "work"),
		"ZE_VPP_STATE=" + filepath.Join(record, "state"),
		"ZE_VPP_DOCKER_IMAGE=" + deployment.VPPImage,
		"ZE_VPP_DOCKER_PLATFORM=" + deployment.VPPPlatform,
		"ZE_VPP_DOCKER_GOARCH=" + deployment.VPPGoarch,
	}
	return append(base, extra...)
}

func vppEvidenceScratch(t *testing.T, root string) map[string]string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, "tmp", "evidence", "vpp-real-*"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("found %d VPP scratch directories, want 1: %v", len(matches), err)
	}
	out := map[string]string{"<work>": matches[0]}
	for _, name := range vppEvidenceScratchFiles {
		body, readErr := os.ReadFile(filepath.Join(matches[0], name)) //nolint:gosec // the test's run wrote this path
		if readErr != nil {
			out[name] = "<absent>"
			continue
		}
		out[name] = string(body)
	}
	return out
}

var vppEvidencePort = regexp.MustCompile(`(ZE_TEST_BGP_PORT=|--port )\d+`)
var vppEvidenceContainer = regexp.MustCompile(`ze-vpp-evidence-\d+`)

func normalizeVPPEvidence(lines []string, root, work string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.ReplaceAll(line, work, "<work>")
		line = strings.ReplaceAll(line, root, "<root>")
		line = vppEvidenceContainer.ReplaceAllString(line, "<container>")
		line = vppEvidencePort.ReplaceAllStringFunc(line, func(value string) string {
			prefix := value[:strings.LastIndexByte(value, '=')+1]
			if strings.HasPrefix(value, "--port ") {
				prefix = "--port "
			}
			return prefix + "<port>"
		})
		out = append(out, line)
	}
	return out
}

func callsContaining(calls []string, needle string) int {
	count := 0
	for _, call := range calls {
		if strings.Contains(call, needle) {
			count++
		}
	}
	return count
}

// sameCallMultiset compares every normalized argv and its absolute frequency.
// It does not compare the write order of independently started child recorders.
func sameCallMultiset(t *testing.T, what string, script, command []string) {
	t.Helper()
	scriptSorted := slices.Clone(script)
	commandSorted := slices.Clone(command)
	slices.Sort(scriptSorted)
	slices.Sort(commandSorted)
	sameCalls(t, what, scriptSorted, commandSorted)
}

func runVPPEvidenceScript(t *testing.T, extra ...string) vppEvidenceRun {
	t.Helper()
	root := vppEvidenceFixture(t)
	stubs := vppEvidenceStubs(t)
	record := t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), vppEvidenceTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", filepath.Join(root, "scripts", "evidence", "effective-vpp.py")) //nolint:gosec // copied test fixture
	cmd.Dir = root
	cmd.Env = append(os.Environ(), vppEvidenceEnv(stubs, record, extra...)...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard
	err := cmd.Run()
	code := 0
	if err != nil {
		exit, ok := errors.AsType[*exec.ExitError](err)
		if !ok {
			t.Fatalf("run script: %v", err)
		}
		code = exit.ExitCode()
	}
	scratch := vppEvidenceScratch(t, root)
	return vppEvidenceRun{
		code: code, stdout: stdout.String(),
		docker: normalizeVPPEvidence(calls(t, filepath.Join(record, "docker")), root, scratch["<work>"]),
		build: normalizeVPPEvidence(calls(t, filepath.Join(record, "go")), root, scratch["<work>"]),
		scratch: scratch,
	}
}

func runVPPEvidenceCommand(t *testing.T, extra ...string) vppEvidenceRun {
	t.Helper()
	root := vppEvidenceFixture(t)
	stubs := vppEvidenceStubs(t)
	record := t.TempDir()
	for _, entry := range vppEvidenceEnv(stubs, record, extra...) {
		name, value, _ := strings.Cut(entry, "=")
		t.Setenv(name, value)
	}
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	run := deployment.NewVPP(root)
	run.Progress = io.Discard
	report, err := run.Run()
	code := 0
	if err != nil || !report.Passed {
		code = 1
	}
	scratch := vppEvidenceScratch(t, root)
	return vppEvidenceRun{
		code: code, stdout: report.Text(), report: report,
		docker: normalizeVPPEvidence(calls(t, filepath.Join(record, "docker")), root, scratch["<work>"]),
		build: normalizeVPPEvidence(calls(t, filepath.Join(record, "go")), root, scratch["<work>"]),
		scratch: scratch,
	}
}

// TestScriptAndCommandProveAllVPPScenariosTheSameWay validates the complete
// payload, exit code, command sequence, constants, and scratch tree.
//
// VALIDATES: the Go action preserves every observable effect of effective-vpp.py.
// PREVENTS: two recordings that agree after a scenario, command, or payload field vanished.
func TestScriptAndCommandProveAllVPPScenariosTheSameWay(t *testing.T) {
	script := runVPPEvidenceScript(t)
	command := runVPPEvidenceCommand(t)
	if script.code != 0 || command.code != 0 {
		t.Fatalf("exit codes are script=%d command=%d, want 0 and 0", script.code, command.code)
	}
	if len(script.build) != 3 || len(command.build) != 3 {
		t.Fatalf("build counts are script=%d command=%d, want 3 and 3", len(script.build), len(command.build))
	}
	if len(script.docker) != 62 || len(command.docker) != 62 {
		t.Fatalf("Docker counts are script=%d command=%d, want 62 and 62 (57 distinct steps plus 5 cleanup polls)\nscript:\n  %s\ncommand:\n  %s",
			len(script.docker), len(command.docker),
			strings.Join(script.docker, "\n  "), strings.Join(command.docker, "\n  "))
	}
	for _, half := range []struct {
		name  string
		calls []string
	}{{name: "script", calls: script.docker}, {name: "command", calls: command.docker}} {
		for needle, want := range map[string]int{
			"show policer":                 15,
			"show acl-plugin acl":          4,
			"show ip fib":                  4,
			"/ze-test-linux-amd64 peer":    2,
			"pkill -TERM":                  2,
		} {
			if got := callsContaining(half.calls, needle); got != want {
				t.Errorf("%s made %d calls carrying %q, want %d", half.name, got, needle, want)
			}
		}
	}
	if len(command.report.Scenarios) != 8 {
		t.Fatalf("the report has %d scenarios, want 8", len(command.report.Scenarios))
	}
	if command.report.Image != "ligato/vpp-base:latest" ||
		command.report.Version != "vpp v24.02-release" ||
		command.report.Interface != "loop0" ||
		!strings.HasPrefix(command.report.Container, "ze-vpp-evidence-") {
		t.Fatalf("the report identity fields are %#v", command.report)
	}
	checks := 0
	for _, scenario := range command.report.Scenarios {
		checks += len(scenario.Checks)
		if scenario.Verdict.String() != "pass" {
			t.Errorf("scenario %s has verdict %s", scenario.Scenario, scenario.Verdict.String())
		}
	}
	if checks != 21 {
		t.Fatalf("the report has %d checks, want 21", checks)
	}
	if script.stdout != command.stdout {
		t.Errorf("full payload text differs:\nscript:\n%s\ncommand:\n%s", script.stdout, command.stdout)
	}
	body, err := json.Marshal(command.report)
	if err != nil {
		t.Fatalf("marshal the full report: %v", err)
	}
	if !bytes.Contains(body, []byte(`"scenarios"`)) || !bytes.Contains(body, []byte(`"checks"`)) {
		t.Fatalf("the full structured payload is %s", body)
	}
	sameCalls(t, "go", script.build, command.build)
	sameCallMultiset(t, "docker", script.docker, command.docker)
	for _, name := range vppEvidenceScratchFiles {
		if script.scratch[name] != command.scratch[name] {
			t.Errorf("scratch file %s differs:\nscript:\n%s\ncommand:\n%s", name, script.scratch[name], command.scratch[name])
		}
	}
}

// TestBothHalvesCleanupInProducerOrder validates every paired daemon phase and
// the final container cleanup.
//
// VALIDATES: 14 daemon phases run in producer order and docker rm is last.
// PREVENTS: a success return bypassing reconcile or leaving the container alive.
func TestBothHalvesCleanupInProducerOrder(t *testing.T) {
	script := runVPPEvidenceScript(t)
	command := runVPPEvidenceCommand(t)
	want := []string{
		"start /run/vpp/fib.conf",
		"start /run/vpp/mpls-fib.conf",
		"start /run/vpp/traffic.conf",
		"start /run/vpp/traffic.conf",
		"start /run/vpp/traffic.conf",
		"start /run/vpp/traffic-proto.conf",
		"start /run/vpp/traffic-proto.conf",
		"start /run/vpp/traffic-dscp.conf",
		"start /run/vpp/traffic-dscp.conf",
		"start /run/vpp/traffic-mc.conf",
		"start /run/vpp/traffic-mc.conf",
		"start /run/vpp/firewall.conf",
		"start /run/vpp/firewall.conf",
		"start /run/vpp/firewall.conf",
	}
	for _, half := range []struct {
		name  string
		calls []string
	}{{name: "script", calls: script.docker}, {name: "command", calls: command.docker}} {
		starts := make([]string, 0, len(want))
		for _, call := range half.calls {
			for _, expected := range want {
				if strings.Contains(call, expected) {
					starts = append(starts, expected)
					break
				}
			}
		}
		if !slices.Equal(starts, want) {
			t.Errorf("%s daemon cleanup order = %v, want %v", half.name, starts, want)
		}
		if len(half.calls) == 0 || !strings.HasPrefix(half.calls[len(half.calls)-1], "rm -f ") {
			t.Errorf("%s final Docker call is not container cleanup: %v", half.name, half.calls)
		}
	}
}

type vppSharedConstants struct {
	Image                  string `json:"VPP_IMAGE"`
	Platform               string `json:"VPP_PLATFORM"`
	Goarch                 string `json:"GOARCH"`
	Prefix                 string `json:"PREFIX"`
	NextHop                string `json:"NEXT_HOP"`
	MPLSPrefix             string `json:"MPLS_PREFIX"`
	MPLSLabel              int    `json:"MPLS_LABEL"`
	TrafficPolicerClass    string `json:"TRAFFIC_POLICER_CLASS"`
	TrafficProtoClass      string `json:"TRAFFIC_PROTO_CLASS"`
	TrafficProtoNumber     int    `json:"TRAFFIC_PROTO_NUMBER"`
	TrafficDSCPClass       string `json:"TRAFFIC_DSCP_CLASS"`
	TrafficDSCPValue       int    `json:"TRAFFIC_DSCP_VALUE"`
	TrafficMultiClassA     string `json:"TRAFFIC_MC_CLASS_A"`
	TrafficMultiProtocolA  int    `json:"TRAFFIC_MC_PROTO_A"`
	TrafficMultiClassB     string `json:"TRAFFIC_MC_CLASS_B"`
	TrafficMultiProtocolB  int    `json:"TRAFFIC_MC_PROTO_B"`
	IPsecReportPrefix      string `json:"IPSEC_REPORT_PREFIX"`
	IPsecSPI               uint64 `json:"IPSEC_SPI"`
	IPsecInboundSPI        uint64 `json:"IPSEC_INBOUND_SPI"`
	IPsecSalt              string `json:"IPSEC_SALT"`
	IPsecCipherKey         string `json:"IPSEC_CIPHER_KEY"`
	FirewallACLTag         string `json:"FIREWALL_ACL_TAG"`
}

// TestScriptAndCommandShareVPPConstantsByValue reads the Python assignments
// through Python's AST and compares all 22 values to the Go constants.
//
// VALIDATES: the producer and port use one exact set of scenario constants.
// PREVENTS: a coincidental substring elsewhere hiding a changed constant.
func TestScriptAndCommandShareVPPConstantsByValue(t *testing.T) {
	const program = `import ast, json, sys
names = {
    "VPP_IMAGE", "VPP_PLATFORM", "GOARCH",
    "PREFIX", "NEXT_HOP", "MPLS_PREFIX", "MPLS_LABEL",
    "TRAFFIC_POLICER_CLASS", "TRAFFIC_PROTO_CLASS", "TRAFFIC_PROTO_NUMBER",
    "TRAFFIC_DSCP_CLASS", "TRAFFIC_DSCP_VALUE", "TRAFFIC_MC_CLASS_A",
    "TRAFFIC_MC_PROTO_A", "TRAFFIC_MC_CLASS_B", "TRAFFIC_MC_PROTO_B",
    "IPSEC_REPORT_PREFIX", "IPSEC_SPI", "IPSEC_INBOUND_SPI", "IPSEC_SALT",
    "IPSEC_CIPHER_KEY", "FIREWALL_ACL_TAG",
}
environment = {"VPP_IMAGE", "VPP_PLATFORM", "GOARCH"}
tree = ast.parse(open(sys.argv[1], encoding="utf-8").read())
values = {}
for node in tree.body:
    if not isinstance(node, ast.Assign) or len(node.targets) != 1:
        continue
    target = node.targets[0]
    if isinstance(target, ast.Name) and target.id in names:
        if target.id in environment:
            values[target.id] = ast.literal_eval(node.value.args[1])
        else:
            values[target.id] = ast.literal_eval(node.value)
if values.keys() != names:
    raise SystemExit(f"constants differ: got={sorted(values)} want={sorted(names)}")
json.dump(values, sys.stdout, sort_keys=True)
`
	body, err := exec.CommandContext(t.Context(), "python3", "-c", program, "effective-vpp.py").Output() //nolint:gosec // fixed test program and repository script
	if err != nil {
		t.Fatalf("read the producer constants: %v", err)
	}
	var got vppSharedConstants
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode the producer constants: %v", err)
	}
	want := vppSharedConstants{
		Image: deployment.VPPImage, Platform: deployment.VPPPlatform, Goarch: deployment.VPPGoarch,
		Prefix: deployment.VPPFIBPrefix, NextHop: deployment.VPPNextHop,
		MPLSPrefix: deployment.VPPMPLSPrefix, MPLSLabel: deployment.VPPMPLSLabel,
		TrafficPolicerClass: deployment.VPPTrafficPolicerClass,
		TrafficProtoClass: deployment.VPPTrafficProtocolClass,
		TrafficProtoNumber: deployment.VPPTrafficProtocolNumber,
		TrafficDSCPClass: deployment.VPPTrafficDSCPClass,
		TrafficDSCPValue: deployment.VPPTrafficDSCPValue,
		TrafficMultiClassA: deployment.VPPTrafficMultiClassA,
		TrafficMultiProtocolA: deployment.VPPTrafficMultiProtocolA,
		TrafficMultiClassB: deployment.VPPTrafficMultiClassB,
		TrafficMultiProtocolB: deployment.VPPTrafficMultiProtocolB,
		IPsecReportPrefix: deployment.VPPIPsecReportPrefix,
		IPsecSPI: deployment.VPPIPsecSPI, IPsecInboundSPI: deployment.VPPIPsecInboundSPI,
		IPsecSalt: deployment.VPPIPsecSalt, IPsecCipherKey: deployment.VPPIPsecCipherKey,
		FirewallACLTag: deployment.VPPFirewallACLTag,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("producer constants = %#v, port constants = %#v", got, want)
	}
}


// TestThePortRefusesTheFIBQueryTheScriptAccepts validates the intentional
// fail-closed difference while the script remains authoritative until step 14.
//
// VALIDATES: a failed vppctl query cannot prove route installation in the port.
// PREVENTS: error text that repeats the prefix becoming route evidence.
func TestThePortRefusesTheFIBQueryTheScriptAccepts(t *testing.T) {
	script := runVPPEvidenceScript(t, "ZE_FIB_QUERY_EXIT=1")
	if script.code != 0 {
		t.Fatalf("the script now refuses the failed query (exit %d); delete this regression with the script", script.code)
	}
	command := runVPPEvidenceCommand(t, "ZE_FIB_QUERY_EXIT=1")
	if command.code == 0 {
		t.Fatal("the port accepted a failed FIB query as route evidence")
	}
	if len(command.report.Scenarios) != 1 {
		t.Fatalf("the port completed %d scenarios after the operating error, want only IPsec", len(command.report.Scenarios))
	}
}

// TestTheVPPActionIsConverted validates the action table instead of a source
// string search.
//
// VALIDATES: ze-deployment-vpp-test has a Go answer and no forked argv.
// PREVENTS: the parity runner existing while `le deployment vpp-test` still starts Python.
func TestTheVPPActionIsConverted(t *testing.T) {
	for _, action := range deployment.Actions().Actions {
		if action.Gate != "ze-deployment-vpp-test" {
			continue
		}
		if len(action.Forks) != 0 {
			t.Fatalf("the VPP action still forks %v", action.Forks)
		}
		return
	}
	t.Fatal("the deployment action table has no ze-deployment-vpp-test")
}
