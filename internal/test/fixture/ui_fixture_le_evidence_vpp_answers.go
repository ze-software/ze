package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func init() {
	Register("ui/le-evidence-vpp-answers", uiDriver(leEvidenceVPPAnswers))
}

type uiLeEvidenceVppAnswersCommandResult struct {
	stdout string
	stderr string
	code   int
}

type leOptions struct {
	record      string
	pluginsExit string
	cmdline     string
	total       string
}

func leEvidenceVPPAnswers(ctx context.Context) error {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return errors.New("ZE_REPO_ROOT is not set")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve ZE_REPO_ROOT: %w", err)
	}
	work, _, err := temporaryLEFixtureWorkspace("le-evidence-vpp-answers-")
	if err != nil {
		return fmt.Errorf("create fixture working directory: %w", err)
	}
	defer os.RemoveAll(work)

	goTool, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("find go tool: %w", err)
	}

	binary, err := uiLEBinary(root)
	if err != nil {
		return err
	}

	fixtureRoot := filepath.Join(work, "fixture")
	if err := os.MkdirAll(fixtureRoot, 0o755); err != nil {
		return fmt.Errorf("create fixture checkout: %w", err)
	}
	if err := os.WriteFile(filepath.Join(fixtureRoot, "go.mod"), []byte("module example.test/m\n\ngo 1.26\n"), 0o644); err != nil {
		return fmt.Errorf("write fixture go.mod: %w", err)
	}
	if err := os.WriteFile(filepath.Join(fixtureRoot, "feature-gates.txt"), []byte("ze_bgp internal/component/bgp\nze_vpp internal/component/vpp\n"), 0o644); err != nil {
		return fmt.Errorf("write fixture feature manifest: %w", err)
	}

	stubs := filepath.Join(work, "stubs")
	if err := os.MkdirAll(stubs, 0o755); err != nil {
		return fmt.Errorf("create stand-in directory: %w", err)
	}
	helperFile := filepath.Join(work, "vpp-evidence-helper.go")
	if err := os.WriteFile(helperFile, []byte(vppEvidenceHelperSource), 0o644); err != nil {
		return fmt.Errorf("write stand-in helper source: %w", err)
	}
	var buildOut, buildErr bytes.Buffer
	firstStub := filepath.Join(stubs, "docker")
	helperBuild := exec.CommandContext(ctx, goTool, "build", "-o", firstStub, helperFile)
	helperBuild.Dir = work
	helperBuild.Env = overlayEnv(os.Environ(), map[string]string{"CGO_ENABLED": "0"})
	helperBuild.Stdout = &buildOut
	helperBuild.Stderr = &buildErr
	buildOut.Reset()
	buildErr.Reset()
	if err := helperBuild.Run(); err != nil {
		return fmt.Errorf("compile VPP command stand-ins: %v\n%s", err, buildErr.String())
	}
	helperBytes, err := os.ReadFile(firstStub)
	if err != nil {
		return fmt.Errorf("read compiled stand-in: %w", err)
	}
	for _, name := range []string{"go", "qemu-system-x86_64", "qemu-system-aarch64", "sshpass", "mkfs.ext4", "debugfs"} {
		if err := os.WriteFile(filepath.Join(stubs, name), helperBytes, 0o755); err != nil {
			return fmt.Errorf("install %s stand-in: %w", name, err)
		}
	}
	if err := os.Chmod(firstStub, 0o755); err != nil {
		return fmt.Errorf("make docker stand-in executable: %w", err)
	}

	runLE := func(args []string, options leOptions) uiLeEvidenceVppAnswersCommandResult {
		record := options.record
		if record == "" {
			record = filepath.Join(work, "docker-argv")
		}
		pluginsExit := options.pluginsExit
		if pluginsExit == "" {
			pluginsExit = "0"
		}
		cmdline := options.cmdline
		if cmdline == "" {
			cmdline = "default_hugepagesz=2M hugepagesz=2M hugepages=64"
		}
		total := options.total
		if total == "" {
			total = "64"
		}
		env := overlayEnv(os.Environ(), map[string]string{
			"PATH":                   stubs + string(os.PathListSeparator) + os.Getenv("PATH"),
			"ZE_REPO_ROOT":           fixtureRoot,
			"ZE_RECORD_DOCKER":       record,
			"ZE_VPP_WORK":            filepath.Join(work, "vpp-work"),
			"ZE_VPP_STATE":           filepath.Join(work, "vpp-state-"+filepath.Base(record)),
			"ZE_VPP_DOCKER_IMAGE":    "ligato/vpp-base:latest",
			"ZE_VPP_DOCKER_PLATFORM": "linux/amd64",
			"ZE_VPP_DOCKER_GOARCH":   "amd64",
			"ZE_PLUGINS_EXIT":        pluginsExit,
			"ZE_VPP_HP_KEEP":         "1",
			"ZE_VPP_HP_SSH_PORT":     "34122",
			"ZE_HP_CMDLINE":          cmdline,
			"ZE_HP_TOTAL":            total,
		})
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = work
		cmd.Env = env
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		code := 0
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			} else {
				code = -1
			}
		}
		return uiLeEvidenceVppAnswersCommandResult{stdout: stdout.String(), stderr: stderr.String(), code: code}
	}

	usage := runLE([]string{"--help"}, leOptions{})
	page := usage.stdout + usage.stderr
	for _, name := range []string{"deployment", "qemu"} {
		if !strings.Contains(page, name) {
			return fmt.Errorf("le --help does not list the %s command", name)
		}
	}

	for _, test := range []struct{ area, action string }{
		{"deployment", "vpp-iface-test"},
		{"qemu", "vpp-hugepages-test"},
	} {
		listing := runLE([]string{test.area}, leOptions{})
		if listing.code != 0 {
			return fmt.Errorf("le %s exited %d", test.area, listing.code)
		}
		if !strings.Contains(listing.stdout, test.action) {
			return fmt.Errorf("le %s does not list %s: %q", test.area, test.action, listing.stdout)
		}
		if !strings.Contains(listing.stdout, "checks") {
			return fmt.Errorf("le %s does not mark %s read-only", test.area, test.action)
		}
	}

	ifaceRecord := filepath.Join(work, "iface-argv")
	proof := runLE([]string{"deployment", "vpp-iface-test", "|", "json"}, leOptions{record: ifaceRecord})
	if proof.code != 0 {
		return fmt.Errorf("the VPP interface proof exited %d: %s", proof.code, uiLeEvidenceVppAnswersTail(proof.stderr, 800))
	}
	var iface map[string]any
	if err := json.Unmarshal([]byte(proof.stdout), &iface); err != nil {
		return fmt.Errorf("decode VPP interface proof: %w; output %q", err, proof.stdout)
	}
	for _, key := range []string{"image", "container", "vpp-version", "plugins", "scenarios", "passed"} {
		if _, ok := iface[key]; !ok {
			return fmt.Errorf("the proof answered no %q key: %v", key, uiLeEvidenceVppAnswersSortedKeys(iface))
		}
	}
	if passed, ok := iface["passed"].(bool); !ok || !passed {
		return errors.New("the proof did not reach a pass")
	}
	ifaceScenarios, ok := iface["scenarios"].([]any)
	if !ok {
		return fmt.Errorf("the proof scenarios have unexpected type %T", iface["scenarios"])
	}
	if len(ifaceScenarios) != 4 {
		return fmt.Errorf("the proof ran %d scenarios, want 4", len(ifaceScenarios))
	}
	for _, raw := range ifaceScenarios {
		one, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("the proof returned a non-object scenario: %T", raw)
		}
		if one["outcome"] != "pass" {
			return fmt.Errorf("scenario %v answered %v: %v", one["feature"], one["outcome"], one["detail"])
		}
	}
	plugins, ok := iface["plugins"].([]any)
	if !ok || len(plugins) != 3 {
		return fmt.Errorf("the proof reported %d plugins, want 3", len(plugins))
	}
	ifaceCalls, err := uiLeEvidenceVppAnswersRecorded(ifaceRecord)
	if err != nil {
		return err
	}
	if !containsCall(ifaceCalls, "--privileged") {
		return fmt.Errorf("the container is not privileged:\n%s", strings.Join(ifaceCalls, "\n"))
	}
	for _, owed := range []string{"start /run/vpp/tunnel.conf", "show gre tunnel", "start /run/vpp/wg.conf", "show wireguard interface"} {
		if !containsCall(ifaceCalls, owed) {
			return fmt.Errorf("the proof never made a call carrying %q:\n%s", owed, strings.Join(ifaceCalls, "\n"))
		}
	}

	brokenRecord := filepath.Join(work, "iface-broken")
	broken := runLE([]string{"deployment", "vpp-iface-test"}, leOptions{record: brokenRecord, pluginsExit: "1"})
	if broken.code != 1 {
		return fmt.Errorf("a failed plugin query exited %d, want 1", broken.code)
	}
	brokenCalls, err := uiLeEvidenceVppAnswersRecorded(brokenRecord)
	if err != nil {
		return err
	}
	if containsCall(brokenCalls, "start /run/vpp/") {
		return errors.New("ze was started on a plugin answer nobody obtained")
	}

	vppRecord := filepath.Join(work, "vpp-evidence-argv")
	evidence := runLE([]string{"deployment", "vpp-test", "|", "json"}, leOptions{record: vppRecord})
	if evidence.code != 0 {
		return fmt.Errorf("the VPP deployment proof exited %d: %s", evidence.code, uiLeEvidenceVppAnswersTail(evidence.stderr, 800))
	}
	var vpp map[string]any
	if err := json.Unmarshal([]byte(evidence.stdout), &vpp); err != nil {
		return fmt.Errorf("decode VPP deployment proof: %w; output %q", err, evidence.stdout)
	}
	for _, key := range []string{"image", "container", "vpp-version", "interface", "scenarios", "passed"} {
		if _, ok := vpp[key]; !ok {
			return fmt.Errorf("the VPP deployment proof answered no %q: %v", key, uiLeEvidenceVppAnswersSortedKeys(vpp))
		}
	}
	if passed, ok := vpp["passed"].(bool); !ok || !passed {
		return errors.New("the VPP deployment proof did not pass")
	}
	vppScenarios, ok := vpp["scenarios"].([]any)
	if !ok {
		return fmt.Errorf("the VPP deployment scenarios have unexpected type %T", vpp["scenarios"])
	}
	if len(vppScenarios) != 8 {
		return fmt.Errorf("the VPP deployment proof ran %d scenarios, want 8", len(vppScenarios))
	}
	checkCount := 0
	for _, raw := range vppScenarios {
		one, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("the VPP deployment proof returned a non-object scenario: %T", raw)
		}
		checks, ok := one["checks"].([]any)
		if !ok {
			return fmt.Errorf("VPP scenario %v returned non-list checks", one["scenario"])
		}
		checkCount += len(checks)
		if one["verdict"] != "pass" {
			return fmt.Errorf("VPP scenario %v answered %v", one["scenario"], one["verdict"])
		}
	}
	if checkCount != 21 {
		return fmt.Errorf("the VPP deployment proof returned %d checks, want 21", checkCount)
	}
	vppCalls, err := uiLeEvidenceVppAnswersRecorded(vppRecord)
	if err != nil {
		return err
	}
	if len(vppCalls) != 62 {
		return fmt.Errorf("the VPP deployment proof made %d Docker calls, want 62 (57 distinct steps plus five cleanup polls)", len(vppCalls))
	}
	for _, owed := range []string{"TestVPPRealDataplaneInstalls", "show ip fib 10.20.0.0/24", "show classify tables", "show acl-plugin acl"} {
		if !containsCall(vppCalls, owed) {
			return fmt.Errorf("the VPP deployment proof made no call carrying %q", owed)
		}
	}

	evidenceYAML := runLE([]string{"deployment", "vpp-test", "|", "yaml"}, leOptions{record: filepath.Join(work, "vpp-evidence-yaml-argv")})
	if evidenceYAML.code != 0 {
		return fmt.Errorf("the YAML VPP deployment proof exited %d", evidenceYAML.code)
	}
	if !strings.Contains(evidenceYAML.stdout, "scenarios:") || !strings.Contains(evidenceYAML.stdout, "passed: true") {
		return fmt.Errorf("the YAML renderer omitted the structured report: %q", head(evidenceYAML.stdout, 300))
	}

	evidenceTable := runLE([]string{"deployment", "vpp-test", "|", "table"}, leOptions{record: filepath.Join(work, "vpp-evidence-table-argv")})
	if evidenceTable.code != 0 {
		return fmt.Errorf("the table VPP deployment proof exited %d", evidenceTable.code)
	}
	for _, heading := range []string{"scenario", "verdict", "checks"} {
		if !strings.Contains(evidenceTable.stdout, heading) {
			return fmt.Errorf("the VPP table has no %q scenario column: %q", heading, head(evidenceTable.stdout, 500))
		}
	}
	for _, scenario := range []string{"ipsec", "ipv4-fib", "mpls-fib", "traffic-interface-class", "traffic-protocol-class", "traffic-dscp-class", "traffic-multi-class", "firewall-acl"} {
		if !strings.Contains(evidenceTable.stdout, scenario) {
			return fmt.Errorf("the VPP table omitted scenario %q: %q", scenario, head(evidenceTable.stdout, 500))
		}
	}

	boot := runLE([]string{"qemu", "vpp-hugepages-test", "|", "json"}, leOptions{})
	if boot.code != 0 {
		return fmt.Errorf("the hugepage proof exited %d: %s", boot.code, uiLeEvidenceVppAnswersTail(boot.stderr, 800))
	}
	var verdict map[string]any
	if err := json.Unmarshal([]byte(boot.stdout), &verdict); err != nil {
		return fmt.Errorf("decode hugepage proof: %w; output %q", err, boot.stdout)
	}
	for _, key := range []string{"verdict", "arch", "accelerator", "page-token", "pages", "memory-mib", "cmdline", "hugepages-total"} {
		if _, ok := verdict[key]; !ok {
			return fmt.Errorf("the proof answered no %q key: %v", key, uiLeEvidenceVppAnswersSortedKeys(verdict))
		}
	}
	if verdict["verdict"] != "pass" {
		return fmt.Errorf("the proof answered %v", verdict["verdict"])
	}
	if verdict["pages"] != float64(64) {
		return fmt.Errorf("the proof asked for %v pages, want 64", verdict["pages"])
	}
	if verdict["hugepages-total"] != float64(64) {
		return fmt.Errorf("the kernel reserved %v, want 64", verdict["hugepages-total"])
	}

	partial := runLE([]string{"qemu", "vpp-hugepages-test"}, leOptions{cmdline: "default_hugepagesz=2M hugepages=64"})
	if partial.code != 1 {
		return fmt.Errorf("a cmdline with no standalone hugepagesz exited %d, want 1", partial.code)
	}

	none := runLE([]string{"qemu", "vpp-hugepages-test"}, leOptions{total: "0"})
	if none.code != 1 {
		return fmt.Errorf("a kernel that reserved no pages exited %d, want 1", none.code)
	}
	if !strings.Contains(none.stdout, "hugepages-total=0") {
		return fmt.Errorf("the failure does not name the kernel answer: %q", none.stdout)
	}

	for _, rendering := range []struct {
		args   []string
		needle string
	}{
		{[]string{"deployment", "vpp-iface-test", "|", "yaml"}, "passed:"},
		{[]string{"qemu", "vpp-hugepages-test", "|", "yaml"}, "verdict:"},
	} {
		out := runLE(rendering.args, leOptions{record: filepath.Join(work, "render-argv")})
		if !strings.Contains(out.stdout, rendering.needle) {
			return fmt.Errorf("le %s did not render the payload: %q", strings.Join(rendering.args, " "), head(out.stdout, 200))
		}
	}

	missing := runLE([]string{"qemu", "no-such-action"}, leOptions{})
	if missing.code != 2 {
		return fmt.Errorf("an unknown action exited %d, want 2", missing.code)
	}
	extra := runLE([]string{"deployment", "vpp-iface-test", "somewhere"}, leOptions{})
	if extra.code != 2 {
		return fmt.Errorf("a value after an action exited %d, want 2", extra.code)
	}

	fmt.Println("OK")
	return nil
}

func uiLeEvidenceVppAnswersFeatureTags(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read feature manifest: %w", err)
	}
	set := map[string]struct{}{"ze_le": {}}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 0 && strings.HasPrefix(fields[0], "ze_") {
			set[fields[0]] = struct{}{}
		}
	}
	tags := make([]string, 0, len(set))
	for tag := range set {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags, nil
}

func overlayEnv(base []string, changes map[string]string) []string {
	out := make([]string, 0, len(base)+len(changes))
	for _, entry := range base {
		key := entry
		if i := strings.IndexByte(entry, '='); i >= 0 {
			key = entry[:i]
		}
		if _, replaced := changes[key]; !replaced {
			out = append(out, entry)
		}
	}
	keys := make([]string, 0, len(changes))
	for key := range changes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out = append(out, key+"="+changes[key])
	}
	return out
}

func uiLeEvidenceVppAnswersRecorded(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read recorded Docker calls: %w", err)
	}
	parts := bytes.Split(data, []byte{0x1e})
	calls := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		calls = append(calls, string(bytes.ReplaceAll(part, []byte{0x1f}, []byte(" "))))
	}
	return calls, nil
}

func containsCall(calls []string, text string) bool {
	for _, call := range calls {
		if strings.Contains(call, text) {
			return true
		}
	}
	return false
}

func uiLeEvidenceVppAnswersSortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func head(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func uiLeEvidenceVppAnswersTail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

const vppEvidenceHelperSource = `package main

import (
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "time"
)

func main() {
    switch filepath.Base(os.Args[0]) {
    case "docker":
        dockerMain()
    case "go":
        goMain()
    case "qemu-system-x86_64", "qemu-system-aarch64":
        fmt.Println("appliance booting")
        time.Sleep(30 * time.Second)
    case "sshpass":
        sshpassMain()
    case "mkfs.ext4", "debugfs":
        return
    default:
        hostMain()
    }
}

func record(args []string) {
    path := os.Getenv("ZE_RECORD_DOCKER")
    file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        os.Exit(125)
    }
    defer file.Close()
    payload := strings.Join(args, string([]byte{0x1f})) + string([]byte{0x1e})
    if _, err := file.Write([]byte(payload)); err != nil {
        os.Exit(125)
    }
}

func workPath() string { return os.Getenv("ZE_VPP_WORK") }
func stateDir() string { return os.Getenv("ZE_VPP_STATE") }
func statePath(name string) string { return filepath.Join(stateDir(), name) }
func hasState(name string) bool { _, err := os.Stat(statePath(name)); return err == nil }
func setState(name string) { _ = os.WriteFile(statePath(name), nil, 0644) }
func clearState(name string) { _ = os.Remove(statePath(name)) }

func awaitState(name string) bool {
    for i := 0; i < 60; i++ {
        if hasState(name) { return true }
        time.Sleep(50 * time.Millisecond)
    }
    return false
}

func awaitDaemon(name string) {
    for i := 0; i < 60; i++ {
        if _, err := os.Stat(workPath() + "." + name); err == nil { return }
        time.Sleep(50 * time.Millisecond)
    }
}

func readWork() string {
    data, _ := os.ReadFile(workPath())
    return string(data)
}

func configHas(name, text string) bool {
    data, _ := os.ReadFile(filepath.Join(readWork(), name))
    return strings.Contains(string(data), text)
}

func dockerMain() {
    args := os.Args[1:]
    _ = os.MkdirAll(stateDir(), 0755)
    if len(args) == 0 { return }
    joined := strings.Join(args, " ")
    switch args[0] {
    case "image", "pull", "rm":
        record(args)
        return
    case "logs":
        record(args)
        fmt.Println("vpp: container log")
        return
    case "run":
        record(args)
        for _, arg := range args {
            if strings.HasSuffix(arg, ":/run/vpp") {
                _ = os.WriteFile(workPath(), []byte(strings.TrimSuffix(arg, ":/run/vpp")), 0644)
            }
        }
        fmt.Println("deadbeef")
        return
    case "exec":
        dockerExec(args, joined)
        return
    }
}

func dockerExec(args []string, joined string) {
    switch {
    case strings.Contains(joined, "vpp -c /run/vpp/startup.conf"):
        record(args)
        work := readWork()
        _ = os.WriteFile(filepath.Join(work, "api.sock"), nil, 0644)
        _ = os.WriteFile(filepath.Join(work, "cli.sock"), nil, 0644)
    case strings.Contains(joined, "TestVPPRealDataplaneInstalls"):
        record(args)
        fmt.Println("ze-vpp-ipsec:spd-id=41")
        fmt.Println("ze-vpp-ipsec:sad-id=42")
        fmt.Println("ze-vpp-ipsec:close-removed-spi=43")
        fmt.Println("ze-vpp-ipsec:close-removed-spd-id=44")
    case strings.Contains(joined, "show ipsec sa 0"):
        record(args)
        fmt.Print("salt 0xdeadbeef aes-gcm-256 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f integrity alg none encap-copy-ecn decap-copy-ecn")
    case strings.Contains(joined, "show ipsec sa"):
        record(args)
        fmt.Print("[0] spi 287454020 protocol:esp tunnel\n[1] spi 1432778632 protocol:esp tunnel inbound")
    case strings.Contains(joined, "show ipsec all"):
        record(args)
        fmt.Print("spd 41\n41 -> loop0\npriority -100 action bypass type ip4-outbound protocol any\npriority -2000 action protect type ip4-outbound protocol any")
    case strings.Contains(joined, "show plugins"):
        record(args)
        code, _ := strconv.Atoi(os.Getenv("ZE_PLUGINS_EXIT"))
        if code != 0 {
            fmt.Fprintln(os.Stderr, "show plugins: unknown input")
            os.Exit(code)
        }
        fmt.Println("wireguard_plugin.so")
        fmt.Println("linux_cp_plugin.so")
        fmt.Println("linux_nl_plugin.so")
    case strings.Contains(joined, "show version"):
        record(args)
        fmt.Println("vpp v24.02-release")
    case strings.Contains(joined, "create loopback interface"):
        record(args)
        fmt.Println("loop0")
    case strings.Contains(joined, "set interface state loop0 up"):
        record(args)
    case strings.Contains(joined, "show gre tunnel"):
        awaitDaemon("tunnel.conf")
        record(args)
        fmt.Println("[0] instance 0 src 10.10.10.1 dst 10.10.10.2")
    case strings.Contains(joined, "show interface span"):
        awaitDaemon("mirror.conf")
        record(args)
        fmt.Println("msrc0    rx    mdst0")
    case strings.Contains(joined, "show interface features loop0"):
        record(args)
        if hasState("pol-default") { fmt.Println("policer-output") }
        if hasState("pol-tcp") || hasState("pol-cs6") || hasState("pol-web") || hasState("pol-dns") { fmt.Println("policer-classify") }
    case strings.Contains(joined, "show interface"):
        record(args)
        fmt.Print("loop0 7 up\nlocal0 0 down\n")
    case strings.Contains(joined, "show wireguard interface"):
        awaitDaemon("wg.conf")
        record(args)
        fmt.Println("[0] wg0 port 51820")
    case strings.Contains(joined, "show lcp"):
        awaitDaemon("lcp.conf")
        record(args)
        fmt.Println("itf-pair: [0] loop0 tap0 host")
    case strings.Contains(joined, "show ip fib 10.20.0.0/24"):
        record(args)
        if hasState("fib") { fmt.Println("10.20.0.0/24 via 10.0.0.1") }
    case strings.Contains(joined, "show ip fib 10.30.0.0/24"):
        record(args)
        if hasState("mpls") { fmt.Println("10.30.0.0/24 via 10.0.0.1 label 100") }
    case strings.Contains(joined, "show policer"):
        if hasState("pol-default") && !configHas("traffic.conf", "interface loop0") { awaitState("clear-pol-default") }
        if hasState("pol-tcp") && !configHas("traffic-proto.conf", "interface loop0") { awaitState("clear-pol-tcp") }
        if hasState("pol-cs6") && !configHas("traffic-dscp.conf", "interface loop0") { awaitState("clear-pol-cs6") }
        if hasState("pol-web") && !configHas("traffic-mc.conf", "interface loop0") { awaitState("clear-pol-multi") }
        record(args)
        if hasState("pol-default") { fmt.Println("ze/loop0/default") }
        if hasState("pol-tcp") { fmt.Println("ze/loop0/tcp") }
        if hasState("pol-cs6") { fmt.Println("ze/loop0/cs6") }
        if hasState("pol-web") { fmt.Println("ze/loop0/web") }
        if hasState("pol-dns") { fmt.Println("ze/loop0/dns") }
        if hasState("clear-pol-default") { clearState("pol-default"); clearState("clear-pol-default") }
        if hasState("clear-pol-tcp") { clearState("pol-tcp"); clearState("clear-pol-tcp") }
        if hasState("clear-pol-cs6") { clearState("pol-cs6"); clearState("clear-pol-cs6") }
        if hasState("clear-pol-multi") { clearState("pol-web"); clearState("pol-dns"); clearState("clear-pol-multi") }
    case strings.Contains(joined, "show classify tables"):
        record(args)
        fmt.Println("table 0 sessions")
    case strings.Contains(joined, "show acl-plugin interface loop0"):
        record(args)
        if hasState("acl") { fmt.Println("input acl 0") }
    case strings.Contains(joined, "show acl-plugin acl"):
        if hasState("acl") && !configHas("firewall.conf", "table wan") { awaitState("clear-acl") }
        record(args)
        if hasState("acl") { fmt.Println("ze/wan/input") }
        if hasState("clear-acl") { clearState("acl"); clearState("clear-acl") }
    case strings.Contains(joined, "pkill -TERM -f ze-test-linux-amd64"):
        record(args)
        clearState("fib")
        clearState("mpls")
    case strings.Contains(joined, "/ze-test-linux-amd64 peer "):
        record(args)
        marker := "fib"
        if strings.Contains(joined, "mpls-peer-script") { marker = "mpls" }
        setState(marker)
        fmt.Println("listening on 127.0.0.1")
        for hasState(marker) { time.Sleep(50 * time.Millisecond) }
    case strings.Contains(joined, "ip link show"):
        record(args)
        fmt.Println("1: lo: <LOOPBACK,UP>")
    case strings.Contains(joined, "start /run/vpp/"):
        record(args)
        conf := ""
        for _, arg := range args {
            if strings.HasPrefix(arg, "/run/vpp/") && strings.HasSuffix(arg, ".conf") {
                conf = strings.TrimPrefix(arg, "/run/vpp/")
            }
        }
        _ = os.WriteFile(workPath()+"."+conf, nil, 0644)
        switch conf {
        case "traffic.conf":
            if configHas(conf, "interface loop0") { setState("pol-default") } else { setState("clear-pol-default") }
            fmt.Fprintln(os.Stderr, "traffic-control config applied")
        case "traffic-proto.conf":
            if configHas(conf, "interface loop0") { setState("pol-tcp") } else { setState("clear-pol-tcp") }
            fmt.Fprintln(os.Stderr, "traffic-control config applied")
        case "traffic-dscp.conf":
            if configHas(conf, "interface loop0") { setState("pol-cs6") } else { setState("clear-pol-cs6") }
            fmt.Fprintln(os.Stderr, "traffic-control config applied")
        case "traffic-mc.conf":
            if configHas(conf, "interface loop0") { setState("pol-web"); setState("pol-dns") } else { setState("clear-pol-multi") }
            fmt.Fprintln(os.Stderr, "traffic-control config applied")
        case "firewall.conf":
            if configHas(conf, "table wan") { setState("acl") } else { setState("clear-acl") }
            fmt.Fprintln(os.Stderr, "firewall config applied")
        default:
            fmt.Fprintln(os.Stderr, "ze: interface backend vpp ready")
        }
        time.Sleep(300 * time.Second)
    default:
        record(args)
    }
}

func goMain() {
    args := os.Args[1:]
    out := ""
    for i := 1; i < len(args); i++ {
        if args[i-1] == "-o" { out = args[i] }
    }
    if out == "" { return }
    _ = os.MkdirAll(filepath.Dir(out), 0755)
    executable, err := os.Executable()
    if err != nil { os.Exit(1) }
    src, err := os.Open(executable)
    if err != nil { os.Exit(1) }
    defer src.Close()
    dst, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
    if err != nil { os.Exit(1) }
    if _, err := io.Copy(dst, src); err != nil { dst.Close(); os.Exit(1) }
    if err := dst.Close(); err != nil { os.Exit(1) }
    _ = os.Chmod(out, 0755)
}

func hostMain() {
    args := os.Args[1:]
    if len(args) < 3 || args[0] != "appliance" { return }
    dir := filepath.Join(os.Getenv("ZE_APPLIANCE_DIR"), args[2])
    switch args[1] {
    case "init":
        _ = os.MkdirAll(dir, 0755)
        _ = os.WriteFile(filepath.Join(dir, "appliance.json"), []byte("{\"name\":\""+args[2]+"\",\"image\":{\"arch\":\"amd64\"}}"), 0644)
    case "build":
        _ = os.MkdirAll(dir, 0755)
        _ = os.WriteFile(filepath.Join(dir, "ze-hugepages.img"), nil, 0644)
    }
}

func sshpassMain() {
    for _, arg := range os.Args[1:] {
        switch arg {
        case "show host kernel | json":
            fmt.Printf("{\"cmdline\":\"console=ttyS0 %s\"}\n", os.Getenv("ZE_HP_CMDLINE"))
            return
        case "show host memory | json":
            total := os.Getenv("ZE_HP_TOTAL")
            if total == "" { total = "64" }
            fmt.Printf("{\"hugepages-total\":%s}\n", total)
            return
        }
    }
}
`
