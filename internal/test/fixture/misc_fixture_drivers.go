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
	"strconv"
	"strings"
	"syscall"
	"time"
)

func init() {
	Register("ui/pipe-local-command-runtime", pipeLocalCommandDriver)
	Register("reload/background-ze-readiness", backgroundReadinessDriver)
	Register("reload/mgmt-guard-reload-auth-rebuild-trigger", reloadTriggerDriver("observer.initial-ok", true))
	Register("reload/mgmt-guard-reload-refuses-unauth-trigger", reloadTriggerDriver("observer.initial-ok", true))
	Register("reload/mgmt-guard-reload-unbuilt-transport-trigger", reloadTriggerDriver("observer.initial-ok", true))
	Register("reload/reload-prefix-updated-clears-stale-trigger", reloadTriggerDriver("saw-stale", false))
}

func commandOutput(ctx context.Context, dir string, env []string, stdin string, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	if env != nil {
		command.Env = env
	}
	if stdin != "" {
		command.Stdin = strings.NewReader(stdin)
	}
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, output)
	}
	return output, nil
}

func miscEnvironment(overrides map[string]string) []string {
	environment := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, overridden := overrides[key]; !overridden {
			environment = append(environment, entry)
		}
	}
	for key, value := range overrides {
		environment = append(environment, key+"="+value)
	}
	return environment
}

func waitForFile(ctx context.Context, path string, attempts int, delay time.Duration) bool {
	return Poll(ctx, attempts, delay, func() bool {
		_, err := os.Stat(path)
		return err == nil
	})
}

func readPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid < 1 {
		return 0, fmt.Errorf("invalid pid %q", strings.TrimSpace(string(data)))
	}
	return pid, nil
}

func backgroundReadinessDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("background readiness fixture takes no arguments")
	}
	if !Poll(ctx, 200, 100*time.Millisecond, func() bool {
		_, pidErr := readPID("daemon.pid")
		_, readyErr := os.Stat("daemon.ready")
		return pidErr == nil && readyErr == nil
	}) {
		return errors.New("background ze readiness files missing")
	}
	pid, err := readPID("daemon.pid")
	if err != nil {
		return err
	}
	if pid == os.Getpid() {
		return errors.New("daemon.pid holds the driver's own pid, not the ze daemon")
	}
	fmt.Fprintln(os.Stdout, "background ze readiness files present")
	return syscall.Kill(pid, syscall.SIGTERM)
}

func reloadTriggerDriver(barrier string, writeDone bool) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 0 {
			return errors.New("reload trigger takes no arguments")
		}
		for _, path := range []string{"daemon.pid", "daemon.ready", barrier} {
			if !waitForFile(ctx, path, 300, 100*time.Millisecond) {
				return fmt.Errorf("reload trigger timed out waiting for %s", path)
			}
		}
		config, err := os.ReadFile("config2.conf")
		if err != nil {
			return err
		}
		if err := os.WriteFile("ze-bgp.conf", config, 0o600); err != nil {
			return err
		}
		pid, err := readPID("daemon.pid")
		if err != nil {
			return err
		}
		if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
			return err
		}
		if writeDone {
			return os.WriteFile("reload.done", nil, 0o600)
		}
		return nil
	}
}

func pipeLocalCommandDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("pipe local command fixture takes no arguments")
	}
	work, err := os.Getwd()
	if err != nil {
		return err
	}
	env := miscEnvironment(map[string]string{"ZE_CONFIG_DIR": work})
	run := func(name string, args ...string) ([]byte, error) {
		return commandOutput(ctx, work, env, "", name, args...)
	}
	configPath := filepath.Join(work, "pipe-local.conf")
	config := "environment {\n    cli {\n        format {\n            default table;\n        }\n    }\n}\nbgp {\n}\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(configPath+".draft", []byte("set environment cli format default json\n"), 0o600); err != nil {
		return err
	}
	for _, item := range [][2]string{{"file/active/pipe-local.conf", configPath}, {"file/active/pipe-local.conf.draft", configPath + ".draft"}} {
		if _, err := run("ze", "data", "write", item[0], item[1]); err != nil {
			return fmt.Errorf("runtime-store fixture %s: %w", item[0], err)
		}
	}
	blobInput := filepath.Join(work, "router-name.txt")
	blobPath := filepath.Join(work, "pipe-local.zefs")
	if err := os.WriteFile(blobInput, []byte("router-a"), 0o600); err != nil {
		return err
	}
	if _, err := run("ze", "data", "--path", blobPath, "write", "meta/instance/name", blobInput); err != nil {
		return err
	}
	localJSON := func(command, evidence string) (any, error) {
		if evidence == "" || !strings.HasPrefix(command, evidence) || !strings.Contains(command, "| ") {
			return nil, fmt.Errorf("invalid evidence %q for %q", evidence, command)
		}
		output, err := run("ze", "cli", "-c", command)
		if err != nil {
			return nil, err
		}
		var value any
		if err := json.Unmarshal(output, &value); err != nil {
			return nil, fmt.Errorf("decode %s: %w", command, err)
		}
		fmt.Fprintln(os.Stdout, "COVERED: "+evidence)
		return value, nil
	}
	asMap := func(value any) map[string]any { result, _ := value.(map[string]any); return result }
	rows := func(value any, key string) []any {
		if object, ok := value.(map[string]any); ok {
			value = object[key]
		}
		result, _ := value.([]any)
		return result
	}
	hasRow := func(list []any, predicate func(map[string]any) bool) bool {
		for _, raw := range list {
			if row, ok := raw.(map[string]any); ok && predicate(row) {
				return true
			}
		}
		return false
	}
	countPayload := func(value any, want int) bool {
		object := asMap(value)
		count, ok := object["count"].(float64)
		pipe := rows(object["pipe"], "")
		if !ok || int(count) != want || len(pipe) != 1 {
			return false
		}
		return asMap(pipe[0])["op"] == "count"
	}
	must := func(value any, err error) (any, error) { return value, err }
	value, err := must(localJSON("show config dump "+configPath+" | json compact", "show config dump"))
	if err != nil || asMap(asMap(asMap(asMap(value)["environment"])["cli"])["format"])["default"] != "table" {
		return fmt.Errorf("config dump lost default table: %v: %w", value, err)
	}
	value, err = localJSON("show config history pipe-local.conf | json compact", "show config history")
	if err != nil || !hasRow(rows(value, "revisions"), func(row map[string]any) bool {
		return row["revision"] == "draft" && row["state"] == "editing in progress"
	}) {
		return fmt.Errorf("config history lost draft row: %v: %w", value, err)
	}
	value, err = localJSON("show config ls | json compact", "show config ls")
	if err != nil || !hasRow(rows(value, "configs"), func(row map[string]any) bool { return row["source"] == "fs" && row["path"] == configPath }) {
		return fmt.Errorf("config ls lost filesystem row: %v: %w", value, err)
	}
	value, err = localJSON("show schema list | json compact", "show schema list")
	if err != nil || !hasRow(rows(value, "schemas"), func(row map[string]any) bool {
		return row["module"] == "ze-fib-conf" && row["namespace"] == "ze.fib.conf"
	}) {
		return fmt.Errorf("schema list lost ze-fib-conf: %v: %w", value, err)
	}
	value, err = localJSON("show schema methods | json compact", "show schema methods")
	methods := rows(value, "methods")
	if err != nil || len(methods) == 0 {
		return fmt.Errorf("schema methods returned no method/module rows: %v: %w", value, err)
	}
	for _, raw := range methods {
		row := asMap(raw)
		if row["method"] == nil || row["module"] == nil {
			return fmt.Errorf("schema methods returned an incomplete row: %v", row)
		}
	}
	eventsValue, err := localJSON("show schema events | json compact", "show schema events")
	if err != nil {
		return err
	}
	events := rows(eventsValue, "events")
	value, err = localJSON("show schema events | count | json compact", "show schema events")
	if err != nil || !countPayload(value, len(events)) {
		return fmt.Errorf("schema events count mismatch: %v: %w", value, err)
	}
	handlersValue, err := localJSON("show schema handlers | json compact", "show schema handlers")
	if err != nil {
		return err
	}
	handlers := rows(handlersValue, "handlers")
	value, err = localJSON("show schema handlers | count | json compact", "show schema handlers")
	if err != nil || !countPayload(value, len(handlers)) {
		return fmt.Errorf("schema handlers count mismatch: %v: %w", value, err)
	}
	value, err = localJSON("show schema protocol | json compact", "show schema protocol")
	if err != nil || asMap(value)["protocol"] != "Hub Architecture" || asMap(value)["version"] != "1.0" {
		return fmt.Errorf("schema protocol changed: %v: %w", value, err)
	}
	value, err = localJSON("show data ls --path "+blobPath+" | json compact", "show data ls")
	if err != nil || !hasRow(rows(value, "keys"), func(row map[string]any) bool { return row["key"] == "meta/instance/name" }) {
		return fmt.Errorf("data ls lost written key: %v: %w", value, err)
	}
	value, err = localJSON("show data registered | json compact", "show data registered")
	if err != nil || !hasRow(rows(value, "patterns"), func(row map[string]any) bool {
		return row["pattern"] == "meta/instance/name" && row["description"] == "Router instance name"
	}) {
		return fmt.Errorf("data registered lost instance-name: %v: %w", value, err)
	}
	flatten := func(nodes []any) []map[string]any {
		var result []map[string]any
		var walk func([]any)
		walk = func(current []any) {
			for _, raw := range current {
				node, _ := raw.(map[string]any)
				result = append(result, node)
				children, _ := node["children"].([]any)
				walk(children)
			}
		}
		walk(nodes)
		return result
	}
	commandTree, err := localJSON("show yang tree --commands | json compact", "show yang tree")
	if err != nil {
		return err
	}
	commandNodes := flatten(rows(commandTree, ""))
	if len(commandNodes) == 0 {
		return errors.New("yang --commands returned no nodes")
	}
	for _, node := range commandNodes {
		if node["source"] == "config" {
			return errors.New("yang --commands leaked a config-only node")
		}
	}
	configTree, err := localJSON("show yang tree --config | json compact", "show yang tree")
	if err != nil {
		return err
	}
	configNodes := flatten(rows(configTree, ""))
	if len(configNodes) == 0 {
		return errors.New("yang --config returned no nodes")
	}
	for _, node := range configNodes {
		if node["source"] == "command" {
			return errors.New("yang --config leaked a command-only node")
		}
	}
	commandBytes, _ := json.Marshal(commandTree)
	configBytes, _ := json.Marshal(configTree)
	if bytes.Equal(commandBytes, configBytes) {
		return errors.New("the two YANG filters selected the same tree")
	}
	value, err = localJSON("show yang completion --min-prefix 2 | json compact", "show yang completion")
	collisions := rows(value, "collisions")
	if err != nil || len(collisions) == 0 {
		return fmt.Errorf("completion returned no collisions: %v: %w", value, err)
	}
	totalAffected := 0
	for _, raw := range collisions {
		group := asMap(raw)
		if group["max-chars"].(float64) < 2 {
			return fmt.Errorf("completion ignored min-prefix: %v", group)
		}
		totalAffected += len(rows(group["siblings"], ""))
	}
	summary := asMap(asMap(value)["summary"])
	if int(summary["total-groups"].(float64)) != len(collisions) || int(summary["total-affected"].(float64)) != totalAffected {
		return fmt.Errorf("completion summary mismatch: %v", value)
	}
	value, err = localJSON("show env list | json compact", "show env list")
	if err != nil || !hasRow(rows(value, "variables"), func(row map[string]any) bool {
		_, current := row["current"]
		return row["key"] == "ze.cli.format" && current
	}) {
		return fmt.Errorf("env list lost ze.cli.format: %v: %w", value, err)
	}
	value, err = localJSON("show env get ze.cli.format | json compact", "show env get")
	selected := rows(value, "variables")
	if err != nil || len(selected) != 1 || asMap(selected[0])["key"] != "ze.cli.format" {
		return fmt.Errorf("env get returned wrong row: %v: %w", value, err)
	}
	if _, ok := asMap(selected[0])["current"]; !ok {
		return fmt.Errorf("env get omitted current value: %v", selected)
	}
	value, err = localJSON("show env registered | json compact", "show env registered")
	if err != nil || !hasRow(rows(value, "variables"), func(row map[string]any) bool {
		_, current := row["current"]
		return row["key"] == "ze.cli.format" && !current
	}) {
		return fmt.Errorf("env registered leaked or lost declaration: %v: %w", value, err)
	}
	savePath := filepath.Join(work, "local-save.ndjson")
	shown, err := run("ze", "cli", "-c", "show env get ze.cli.format | ndjson | save "+savePath)
	if err != nil {
		return err
	}
	saved, err := os.ReadFile(savePath)
	if err != nil || !bytes.Equal(saved, shown) {
		return fmt.Errorf("local save bytes differ: %w", err)
	}
	saveInfo, err := os.Stat(savePath)
	if err != nil {
		return err
	}
	if saveInfo.Mode().Perm() != 0o600 {
		return fmt.Errorf("local save mode=%v, want 0600", saveInfo.Mode().Perm())
	}
	fmt.Fprintln(os.Stdout, "OK: 15/15 local-data commands and local one-shot save")
	return nil
}
