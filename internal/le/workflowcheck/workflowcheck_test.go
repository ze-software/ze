package workflowcheck

// These tests pin the GitHub workflow contract at its permanent native command
// boundary: schedules and privileges stay attached to the evidence they enable,
// while every repository action resolves through the live LE registry.

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/leroot"

	_ "github.com/ze-software/ze/internal/le/buildartifacts"
	_ "github.com/ze-software/ze/internal/le/deployment"
	_ "github.com/ze-software/ze/internal/le/fuzz"
	_ "github.com/ze-software/ze/internal/le/integration"
	"github.com/ze-software/ze/internal/le/leaction"
	_ "github.com/ze-software/ze/internal/le/qemu"
	_ "github.com/ze-software/ze/internal/le/verify"
	_ "github.com/ze-software/ze/internal/le/verify/deps"
	"github.com/ze-software/ze/internal/le/verify/engine"
)

const workflowsDir = ".github/workflows"

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func workflowSource(t *testing.T, name string) string {
	t.Helper()
	return readFile(t, filepath.Join(repoRoot(t), workflowsDir, name))
}

func workflowNames(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repoRoot(t), workflowsDir))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || (filepath.Ext(entry.Name()) != ".yml" && filepath.Ext(entry.Name()) != ".yaml") {
			continue
		}
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	if len(names) == 0 {
		t.Fatal("workflow directory contains no workflow")
	}
	return names
}

func stripComments(source string) string {
	lines := strings.Split(source, "\n")
	for index, line := range lines {
		lines[index], _, _ = strings.Cut(line, "#")
	}
	return strings.Join(lines, "\n")
}

func leadingSpace(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

func topLevelBlock(t *testing.T, name, key string) string {
	t.Helper()
	lines := strings.Split(stripComments(workflowSource(t, name)), "\n")
	for index, line := range lines {
		if strings.TrimRight(line, " \t") != key+":" {
			continue
		}
		var body []string
		for _, next := range lines[index+1:] {
			if strings.TrimSpace(next) != "" && leadingSpace(next) == 0 {
				break
			}
			body = append(body, next)
		}
		joined := strings.Join(body, "\n")
		if strings.TrimSpace(joined) == "" {
			t.Fatalf("%s has an empty %s block", name, key)
		}
		return joined
	}
	t.Fatalf("%s has no top-level %s block", name, key)
	return ""
}

func onBlock(t *testing.T, name string) string {
	t.Helper()
	return topLevelBlock(t, name, "on")
}

type jobBlock struct {
	name     string
	body     string
	advisory bool
}

func jobBlocks(t *testing.T, name string) []jobBlock {
	t.Helper()
	body := topLevelBlock(t, name, "jobs")
	lines := strings.Split(body, "\n")
	jobIndent := -1
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		jobIndent = leadingSpace(line)
		break
	}
	if jobIndent < 0 {
		t.Fatalf("%s has no job", name)
	}

	var jobs []jobBlock
	for index := 0; index < len(lines); {
		line := lines[index]
		trimmed := strings.TrimSpace(line)
		if leadingSpace(line) != jobIndent || !strings.HasSuffix(trimmed, ":") {
			index++
			continue
		}
		jobName := strings.TrimSuffix(trimmed, ":")
		end := index + 1
		for end < len(lines) {
			next := strings.TrimSpace(lines[end])
			if next != "" && leadingSpace(lines[end]) == jobIndent && strings.HasSuffix(next, ":") {
				break
			}
			end++
		}
		jobBody := strings.Join(lines[index+1:end], "\n")
		directIndent := -1
		for _, candidate := range lines[index+1 : end] {
			if strings.TrimSpace(candidate) == "" {
				continue
			}
			directIndent = leadingSpace(candidate)
			break
		}
		advisory := false
		for _, candidate := range lines[index+1 : end] {
			if leadingSpace(candidate) == directIndent && strings.TrimSpace(candidate) == "continue-on-error: true" {
				advisory = true
			}
		}
		jobs = append(jobs, jobBlock{name: jobName, body: jobBody, advisory: advisory})
		index = end
	}
	if len(jobs) == 0 {
		t.Fatalf("%s has no parsed job", name)
	}
	return jobs
}

// nativeActionPattern reads up to three words after le, because a command may
// be a namespace and its member before its verb: `le verify deps vulnerability`
// is one command of two words and one verb.
var nativeActionPattern = regexp.MustCompile(
	`(?m)(^|[[:space:];&|])(\./)?le[[:space:]]+([a-z0-9-]+)[[:space:]]+([a-z0-9-]+)([[:space:]]+([a-z0-9-]+))?`,
)

// nativeActionsIn answers each `le` invocation in a workflow as command/verb.
//
// Which words are the command is a question only the registry can answer, so
// the two-word reading is tried first and the one-word reading is the
// fallback. Guessing from the text would make `le verify deps vulnerability`
// read as the verb `deps` of the command `verify`, which exists and does
// something else.
func nativeActionsIn(source string) []string {
	matches := nativeActionPattern.FindAllStringSubmatch(source, -1)
	actions := make([]string, 0, len(matches))
	for _, match := range matches {
		first, second, third := match[3], match[4], match[6]
		if third != "" && leroot.LookupCommand(first+" "+second) != nil {
			actions = append(actions, first+" "+second+"/"+third)
			continue
		}
		actions = append(actions, first+"/"+second)
	}
	return actions
}

func nativeActions(t *testing.T, name string) []string {
	t.Helper()
	return nativeActionsIn(stripComments(workflowSource(t, name)))
}

func actionExists(t *testing.T, identity string) {
	t.Helper()
	area, verb, ok := strings.Cut(identity, "/")
	if !ok || area == "" || verb == "" {
		t.Fatalf("invalid native action identity %q", identity)
	}
	handler := leroot.LookupCommand(area)
	if handler == nil {
		t.Fatalf("native root %q is not registered", area)
	}
	payload, code := handler(nil)
	list, ok := payload.(leaction.List)
	if !ok {
		t.Fatalf("native root %q answered %T with code %d, want leaction.List", area, payload, code)
	}
	if code != 0 && code != 2 {
		t.Fatalf("native root %q refused its action listing with %d", area, code)
	}
	if !slices.ContainsFunc(list.Actions, func(row leaction.Row) bool { return row.Verb == verb }) {
		t.Fatalf("native action %q does not exist; %s verbs: %v", identity, area, list.Actions)
	}
}

func TestEveryWorkflowNativeActionExists(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range workflowNames(t) {
		for _, identity := range nativeActions(t, name) {
			if seen[identity] {
				continue
			}
			seen[identity] = true
			actionExists(t, identity)
		}
	}
	if len(seen) == 0 {
		t.Fatal("workflow set invokes no native action")
	}
}

func TestNativeActionExtractorHandlesWorkflowCommands(t *testing.T) {
	source := "run: sudo -E env PATH=$PATH ./le integration iface && ./le verify deps vulnerability\n" +
		"# ./le integration absent-action\n"
	// integration is one word and verify deps is two, so the same line proves
	// both readings: the extractor asks the registry rather than counting words.
	want := []string{"integration/iface", "verify deps/vulnerability"}
	if got := nativeActionsIn(stripComments(source)); !slices.Equal(got, want) {
		t.Fatalf("actions = %v, want %v", got, want)
	}
}

func requireScheduledOnly(t *testing.T, name string) {
	t.Helper()
	on := onBlock(t, name)
	if !strings.Contains(on, "schedule:") {
		t.Errorf("%s must be scheduled", name)
	}
	for _, forbidden := range []string{"push:", "pull_request:"} {
		if strings.Contains(on, forbidden) {
			t.Errorf("%s must not contain %s", name, forbidden)
		}
	}
}

func requireEveryJobAdvisory(t *testing.T, name string) {
	t.Helper()
	for _, job := range jobBlocks(t, name) {
		if !job.advisory {
			t.Errorf("%s job %q must remain advisory", name, job.name)
		}
	}
}

func TestVerifyWorkflowIsTheFastMergeGate(t *testing.T) {
	on := onBlock(t, "verify.yml")
	for _, trigger := range []string{"push:", "pull_request:"} {
		if !strings.Contains(on, trigger) {
			t.Errorf("verify.yml lacks %s", trigger)
		}
	}
	if strings.Contains(on, "schedule:") {
		t.Error("verify.yml must not be scheduled")
	}
	actions := nativeActions(t, "verify.yml")
	if !slices.Equal(actions, []string{"verify/list"}) {
		t.Fatalf("verify.yml native actions = %v, want only verify/list", actions)
	}
	source := workflowSource(t, "verify.yml")
	if !strings.Contains(source, "./le verify list mode full") {
		t.Error("verify.yml must consume the full native verifier list")
	}
	if !strings.Contains(source, "fail-fast: false") {
		t.Error("a red shard must not cancel its siblings")
	}
}

var shardMatrix = regexp.MustCompile(`(?m)^\s*shard:\s*\[([0-9,\s]+)]\s*$`)

func shardIndices(t *testing.T, source string) []int {
	t.Helper()
	match := shardMatrix.FindStringSubmatch(source)
	if len(match) != 2 {
		t.Fatal("verify.yml must declare one numeric shard matrix")
	}
	var indices []int
	for raw := range strings.SplitSeq(match[1], ",") {
		value, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			t.Fatal(err)
		}
		indices = append(indices, value)
	}
	for index, value := range indices {
		if value != index+1 {
			t.Fatalf("shards = %v, want contiguous 1..N", indices)
		}
	}
	return indices
}

func TestVerifyShardsCoverEveryNativeStage(t *testing.T) {
	source := workflowSource(t, "verify.yml")
	indices := shardIndices(t, source)
	if !strings.Contains(source, "NR % n == i % n") {
		t.Fatal("verify.yml must retain the round-robin selection rule")
	}
	stages := verifyengine.StagesForMode(verifyengine.Mode)
	if len(stages) == 0 {
		t.Fatal("native full verifier lists no stage")
	}
	seen := map[string]int{}
	for number, stage := range stages {
		for _, shard := range indices {
			if (number+1)%len(indices) == shard%len(indices) {
				seen[stage.Identity.Name]++
			}
		}
	}
	for _, stage := range stages {
		if seen[stage.Identity.Name] != 1 {
			t.Errorf("native stage %q appears in %d shards", stage.Identity.Name, seen[stage.Identity.Name])
		}
	}
}

func TestVerifyShardRunsNativeActionsAndContinuesAfterFailures(t *testing.T) {
	source := workflowSource(t, "verify.yml")
	for _, required := range []string{
		`while IFS=/ read -r -a action`,
		`./le "${action[@]}" || status=$?`,
		`exit "$status"`,
		`ZE_VERIFY_MODE: "1"`,
	} {
		if !strings.Contains(source, required) {
			t.Errorf("verify.yml lacks %q", required)
		}
	}
	if strings.Contains(source, "set -e") {
		t.Error("the stage loop must continue after a red action")
	}
}

func TestVerifyProvisionsLoopbackBeforeReadingStages(t *testing.T) {
	source := workflowSource(t, "verify.yml")
	address := strings.Index(source, "sudo ip -6 addr add fd00::2/128 dev lo")
	stages := strings.Index(source, "./le verify list mode full")
	if address < 0 || stages < 0 || address > stages {
		t.Fatalf("loopback step index %d, stage-list index %d", address, stages)
	}
}

func TestVerifyInstallsPinnedNativeTools(t *testing.T) {
	source := stripComments(workflowSource(t, "verify.yml"))
	for _, command := range []string{
		"github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1",
		"honnef.co/go/tools/cmd/staticcheck@2026.1",
		"golang.org/x/vuln/cmd/govulncheck@v1.4.0",
	} {
		if strings.Count(source, command) != 1 {
			t.Errorf("verify.yml must install %q exactly once", command)
		}
	}
}

func TestEvidenceNightlyScheduleActionsAndPrivileges(t *testing.T) {
	const name = "evidence-nightly.yml"
	requireScheduledOnly(t, name)
	requireEveryJobAdvisory(t, name)
	want := []string{
		"fuzz/run",
		"integration/iface", "integration/fib", "integration/firewall",
		"integration/traffic", "integration/gtsm", "integration/as112",
		"integration/interop", "integration/interop-ipsec",
	}
	actions := nativeActions(t, name)
	for _, action := range want {
		if !slices.Contains(actions, action) {
			t.Errorf("%s lacks native action %q; found %v", name, action, actions)
		}
	}
	if slices.ContainsFunc(actions, func(action string) bool { return strings.HasPrefix(action, "qemu/") }) {
		t.Errorf("%s must leave VM evidence to qemu-nightly.yml: %v", name, actions)
	}
	for _, job := range jobBlocks(t, name) {
		if job.name == "integration" && !strings.Contains(job.body, `sudo -E env "PATH=$PATH"`) {
			t.Error("kernel integration must preserve the toolchain under sudo")
		}
	}
}

func TestQEMUNightlyScheduleActionsCachesAndBudgets(t *testing.T) {
	const name = "qemu-nightly.yml"
	requireScheduledOnly(t, name)
	requireEveryJobAdvisory(t, name)
	source := workflowSource(t, name)
	for _, budget := range []string{"timeout-minutes: 180", "timeout-minutes: 120", "timeout-minutes: 150"} {
		if !strings.Contains(source, budget) {
			t.Errorf("%s lacks %s", name, budget)
		}
	}
	for _, job := range jobBlocks(t, name) {
		if !strings.Contains(job.body, "actions/cache/restore@v6") || !strings.Contains(job.body, "actions/cache/save@v6") {
			t.Errorf("%s job %q must restore and save the runtime kernel cache", name, job.name)
		}
		if strings.Contains(job.body, "./le qemu run") {
			for _, required := range []string{
				"./le build-artifacts host",
				"./ze-host appliance kernel --target runtime --arch amd64",
				"kernel tmp/kernel/build/vmlinuz",
			} {
				if !strings.Contains(job.body, required) {
					t.Errorf("%s job %q runs a VM without %q", name, job.name, required)
				}
			}
		}
	}
	for _, proof := range []string{
		"qemu all-tests", "TestLDPInteropFRR", "TestISISInteropFRR",
		"qemu vrrp-keepalived-test", "deployment gokrazy-l2tp-ppp-test",
		"qemu pppoe-accel-test", "qemu pppoe-test", "TestAttachTCX_CountsTraffic",
	} {
		if !strings.Contains(source, proof) {
			t.Errorf("%s lacks proof %q", name, proof)
		}
	}
	for _, identity := range []string{
		"qemu/all-tests", "qemu/vrrp-keepalived-test",
		"qemu/pppoe-accel-test", "qemu/pppoe-test",
	} {
		actionExists(t, identity)
	}
}

func TestCapabilityGatedTestsHaveANativeVMHome(t *testing.T) {
	source := workflowSource(t, "qemu-nightly.yml")
	if !strings.Contains(source, "ZE_QEMU_LINUX_ONLY=1") || !strings.Contains(source, "qemu all-tests") {
		t.Fatal("capability-gated fixtures need the Linux-only native VM run")
	}
	root := filepath.Join(repoRoot(t), "test")
	found := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".ci" {
			return nil
		}
		fixture := readFile(t, path)
		if !strings.Contains(fixture, "option=needs-linux:caps=") {
			return nil
		}
		found++
		for line := range strings.SplitSeq(fixture, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "skip-env:") && strings.Contains(trimmed, "var=ZE_QEMU") {
				t.Errorf("%s opts out of its native VM home with %q", path, trimmed)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found == 0 {
		t.Fatal("no capability-gated fixture found")
	}
}

func TestPerfNightlyRemainsScheduledAndNative(t *testing.T) {
	const name = "perf-nightly.yml"
	requireScheduledOnly(t, name)
	source := workflowSource(t, name)
	for _, required := range []string{"go build -tags ze_perf", "bin/ze-perf track --check"} {
		if !strings.Contains(source, required) {
			t.Errorf("%s lacks %q", name, required)
		}
	}
}

func TestVulnerabilityWorkflowRemainsScheduledAndUsesSharedAction(t *testing.T) {
	const name = "govulncheck.yml"
	requireScheduledOnly(t, name)
	if !strings.Contains(onBlock(t, name), "workflow_dispatch:") {
		t.Error("govulncheck.yml must retain manual dispatch")
	}
	if actions := nativeActions(t, name); !slices.Equal(actions, []string{"verify deps/vulnerability"}) {
		t.Fatalf("govulncheck.yml actions = %v", actions)
	}
	if !strings.Contains(workflowSource(t, name), "golang.org/x/vuln/cmd/govulncheck@v1.4.0") {
		t.Error("govulncheck.yml must install the scanner version used by verification")
	}
}

func TestCodeQLKeepsOnlyTheShippedLanguagePopulation(t *testing.T) {
	source := workflowSource(t, "codeql.yml")
	languageRow := regexp.MustCompile(`(?m)^\s*-\s+language:\s+([a-z-]+)\s*$`)
	var languages []string
	for _, match := range languageRow.FindAllStringSubmatch(source, -1) {
		languages = append(languages, match[1])
	}
	want := []string{"go", "javascript-typescript"}
	if !slices.Equal(languages, want) {
		t.Errorf("codeql.yml languages = %v, want %v", languages, want)
	}
	if !strings.Contains(source, "./le feature-tags write") {
		t.Error("CodeQL tag provenance must name the current native producer")
	}
}
