package localdatacoverage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// The two schema views this coverage walk names in its command, its evidence
// and its expected-shape table.
const (
	commandShowSchemaEvents   = "show schema events"
	commandShowSchemaHandlers = "show schema handlers"
)

// envKeyCLIFormat is the env row this walk reads, sets and clears.
const envKeyCLIFormat = "ze.cli.format"

const CompletionMarker = "OK: 18/18 local-data commands and local one-shot save"

// Marker returns the terminal-delimited evidence emitted after a local command
// has successfully run and answered JSON.
func Marker(evidence string) string {
	return "COVERED: " + evidence + " [done]"
}

type Invocation struct {
	Command  string
	Evidence string
}

// Evidence returns the complete executable command/evidence population. Dynamic
// paths use printf-shaped templates so the command still begins with the exact
// registered local-data path.
func Evidence() []Invocation {
	return []Invocation{
		{Command: "show config dump %s | json compact", Evidence: "show config dump"},
		{Command: "show config diff pipe-local.conf pipe-local-other.conf | json compact", Evidence: "show config diff"},
		{Command: "validate config pipe-local.conf | json compact", Evidence: "validate config"},
		{Command: "show config history pipe-local.conf | json compact", Evidence: "show config history"},
		{Command: "show config ls | json compact", Evidence: "show config ls"},
		{Command: "show schema list | json compact", Evidence: "show schema list"},
		{Command: "show schema methods | json compact", Evidence: "show schema methods"},
		{Command: "show schema events | count | json compact", Evidence: commandShowSchemaEvents},
		{Command: "show schema handlers | count | json compact", Evidence: commandShowSchemaHandlers},
		{Command: "show schema protocol | json compact", Evidence: "show schema protocol"},
		{Command: "show data ls --path %s | json compact", Evidence: "show data ls"},
		{Command: "show data registered | json compact", Evidence: "show data registered"},
		{Command: "show yang tree --commands | json compact", Evidence: "show yang tree"},
		{Command: "show yang tree --config | json compact", Evidence: "show yang tree"},
		{Command: "show yang completion --min-prefix 2 | json compact", Evidence: "show yang completion"},
		{Command: "show env list | json compact", Evidence: "show env list"},
		{Command: "show env get ze.cli.format | json compact", Evidence: "show env get"},
		{Command: "show env registered | json compact", Evidence: "show env registered"},
		{Command: "show plugins | json compact", Evidence: "show plugins"},
	}
}

type commandResult struct {
	stdout []byte
	stderr []byte
}

func run(argv ...string) (commandResult, error) {
	command := exec.CommandContext(context.Background(), argv[0], argv[1:]...) // #nosec G204 -- this compiled test helper owns every executable and argument.
	command.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	return commandResult{stdout: stdout.Bytes(), stderr: stderr.Bytes()}, err
}

func require(condition bool, format string, args ...any) error {
	if condition {
		return nil
	}
	return fmt.Errorf(format, args...)
}

func writeFixture(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

func seedData(key, source string, args ...string) error {
	argv := append([]string{"ze", "data"}, args...)
	argv = append(argv, "write", key, source)
	result, err := run(argv...)
	if err != nil {
		return fmt.Errorf("runtime-store fixture %s: %w: %s%s", key, err, result.stdout, result.stderr)
	}
	return nil
}

func localJSON(command, evidence string, output io.Writer) (any, error) {
	if evidence == "" || (command != evidence && !strings.HasPrefix(command, evidence+" ")) {
		return nil, fmt.Errorf("invalid evidence %q for %q", evidence, command)
	}
	if !strings.Contains(command, "| ") {
		return nil, fmt.Errorf("local command has no real pipe: %s", command)
	}
	result, err := run("ze", "cli", "-c", command)
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s%s", command, err, result.stdout, result.stderr)
	}
	var payload any
	if err := json.Unmarshal(result.stdout, &payload); err != nil {
		return nil, fmt.Errorf("%s did not answer JSON: %w: %s%s", command, err, result.stdout, result.stderr)
	}
	if _, err := fmt.Fprintln(output, Marker(evidence)); err != nil {
		return nil, fmt.Errorf("write coverage evidence: %w", err)
	}
	return payload, nil
}

func object(payload any) (map[string]any, bool) {
	value, ok := payload.(map[string]any)
	return value, ok
}

func rowObject(value any) (map[string]any, error) {
	row, ok := object(value)
	if !ok || row == nil {
		return nil, fmt.Errorf("row is not an object: %#v", value)
	}
	return row, nil
}

func validateRows(values []any, label string) error {
	for index, value := range values {
		if _, err := rowObject(value); err != nil {
			return fmt.Errorf("%s row %d: %w", label, index, err)
		}
	}
	return nil
}

func rows(payload any, key string) ([]any, error) {
	selected := payload
	if document, ok := object(payload); ok {
		selected = document[key]
	}
	values, ok := selected.([]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a row list: %#v", key, selected)
	}
	if err := validateRows(values, key); err != nil {
		return nil, err
	}
	return values, nil
}

func exactObject(payload any, label string, keys ...string) (map[string]any, error) {
	document, ok := object(payload)
	if !ok {
		return nil, fmt.Errorf("%s is not an object: %#v", label, payload)
	}
	if len(document) != len(keys) {
		return nil, fmt.Errorf("%s fields = %v, want exactly %v", label, mapsKeys(document), keys)
	}
	for _, key := range keys {
		if _, exists := document[key]; !exists {
			return nil, fmt.Errorf("%s is missing %q: %#v", label, key, document)
		}
	}
	return document, nil
}

func mapsKeys(document map[string]any) []string {
	keys := make([]string, 0, len(document))
	for key := range document {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func validateCountDocument(payload any, expected int) error {
	document, err := exactObject(payload, "count document", "count", "pipe")
	if err != nil {
		return err
	}
	pipe, ok := document["pipe"].([]any)
	if !ok || len(pipe) != 1 {
		return fmt.Errorf("count pipe is not a one-step array: %#v", document["pipe"])
	}
	step, err := exactObject(pipe[0], "count pipe step", "op")
	if err != nil {
		return err
	}
	if document["count"] != float64(expected) || step["op"] != "count" {
		return fmt.Errorf("count metadata disagrees with %d rows: %#v", expected, payload)
	}
	return nil
}

func validateProtocolDocument(payload any) error {
	document, err := exactObject(payload, "schema protocol", "protocol", "version")
	if err != nil {
		return err
	}
	if document["protocol"] != "Hub Architecture" || document["version"] != "1.0" {
		return fmt.Errorf("schema protocol changed payload: %#v", payload)
	}
	return nil
}
func nested(document map[string]any, keys ...string) any {
	var value any = document
	for _, key := range keys {
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = object[key]
	}
	return value
}

func anyRow(values []any, predicate func(map[string]any) bool) (bool, error) {
	if err := validateRows(values, "row list"); err != nil {
		return false, err
	}
	for _, value := range values {
		row, err := rowObject(value)
		if err != nil {
			return false, err
		}
		if predicate(row) {
			return true, nil
		}
	}
	return false, nil
}

func requireAnyRow(values []any, predicate func(map[string]any) bool, format string, args ...any) error {
	matched, err := anyRow(values, predicate)
	if err != nil {
		return err
	}
	return require(matched, format, args...)
}

func treeNodes(values []any) ([]map[string]any, error) {
	nodes := make([]map[string]any, 0, len(values))
	for _, value := range values {
		node, err := rowObject(value)
		if err != nil {
			return nil, fmt.Errorf("YANG tree: %w", err)
		}
		nodes = append(nodes, node)
		childrenValue, present := node["children"]
		if !present {
			continue
		}
		children, ok := childrenValue.([]any)
		if !ok {
			return nil, fmt.Errorf("YANG tree children are not an array: %#v", childrenValue)
		}
		descendants, err := treeNodes(children)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, descendants...)
	}
	return nodes, nil
}

func collisionAffected(values []any) (int, error) {
	total := 0
	for _, value := range values {
		row, err := rowObject(value)
		if err != nil {
			return 0, fmt.Errorf("completion collision: %w", err)
		}
		if maximum, _ := row["max-chars"].(float64); maximum < 2 {
			return 0, fmt.Errorf("completion ignored --min-prefix 2: %#v", row)
		}
		siblings, ok := row["siblings"].([]any)
		if !ok {
			return 0, fmt.Errorf("completion collision siblings are not an array: %#v", row)
		}
		for index, value := range siblings {
			sibling, err := rowObject(value)
			if err != nil {
				return 0, fmt.Errorf("completion collision sibling %d: %w", index, err)
			}
			name, ok := sibling["name"].(string)
			if !ok || name == "" {
				return 0, fmt.Errorf("completion collision sibling %d has no nonempty name: %#v", index, sibling)
			}
		}
		total += len(siblings)
	}
	return total, nil
}

var runMu sync.Mutex

// Run executes the complete local-data scenario in private cleanup-owned state.
func Run(output io.Writer) error {
	return inPrivateWorkspace(func(work string) error {
		return runScenario(output, work)
	})
}

func inPrivateWorkspace(scenario func(string) error) (result error) {
	runMu.Lock()
	defer runMu.Unlock()
	previousDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("record working directory: %w", err)
	}
	previousConfigDirectory, configDirectoryWasSet := os.LookupEnv("ZE_CONFIG_DIR")
	work, err := os.MkdirTemp("", "ze-local-data-coverage-")
	if err != nil {
		return fmt.Errorf("create private workspace: %w", err)
	}
	// os.Getwd falls back to the getcwd(2) syscall, which always answers the
	// kernel's canonical path. On macOS that resolves /var/folders/... to
	// /private/var/folders/..., so work must already be canonical here or
	// every comparison against a later os.Getwd() inside scenario disagrees
	// with the value this function handed out.
	work, err = filepath.EvalSymlinks(work)
	if err != nil {
		return fmt.Errorf("resolve private workspace: %w", err)
	}
	defer func() {
		var cleanupErrors []error
		if err := os.Chdir(previousDirectory); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("restore working directory: %w", err))
		}
		if configDirectoryWasSet {
			if err := os.Setenv("ZE_CONFIG_DIR", previousConfigDirectory); err != nil {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("restore ZE_CONFIG_DIR: %w", err))
			}
		} else if err := os.Unsetenv("ZE_CONFIG_DIR"); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("restore ZE_CONFIG_DIR: %w", err))
		}
		if err := os.RemoveAll(work); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove private workspace: %w", err))
		}
		result = errors.Join(append([]error{result}, cleanupErrors...)...)
	}()
	if err := os.Chdir(work); err != nil {
		return fmt.Errorf("enter private workspace: %w", err)
	}
	if err := os.Setenv("ZE_CONFIG_DIR", work); err != nil {
		return fmt.Errorf("set ZE_CONFIG_DIR: %w", err)
	}
	return scenario(work)
}

func runScenario(output io.Writer, work string) error {

	configPath := filepath.Join(work, "pipe-local.conf")
	if err := writeFixture(configPath, "environment {\n    cli {\n        format {\n            default table;\n        }\n    }\n}\nbgp {\n}\n"); err != nil {
		return err
	}
	if err := writeFixture(configPath+".draft", "set environment cli format default json\n"); err != nil {
		return err
	}
	// The diff needs a second configuration that differs inside bgp{}: the
	// command resolves the BGP tree, so a difference anywhere else is invisible
	// to it.
	otherPath := filepath.Join(work, "pipe-local-other.conf")
	if err := writeFixture(otherPath, "bgp {\n    router-id 10.0.0.1;\n}\n"); err != nil {
		return err
	}
	resolvedConfigPath, err := filepath.EvalSymlinks(configPath)
	if err != nil {
		return fmt.Errorf("resolve config fixture path: %w", err)
	}
	if err := seedData("file/active/pipe-local.conf", configPath); err != nil {
		return err
	}
	if err := seedData("file/active/pipe-local.conf.draft", configPath+".draft"); err != nil {
		return err
	}
	blobInput := filepath.Join(work, "router-name.txt")
	blobPath := filepath.Join(work, "pipe-local.zefs")
	if err := writeFixture(blobInput, "router-a"); err != nil {
		return err
	}
	if err := seedData("meta/instance/name", blobInput, "--path", blobPath); err != nil {
		return err
	}

	payload, err := localJSON(fmt.Sprintf("show config dump %s | json compact", configPath), "show config dump", output)
	if err != nil {
		return err
	}
	document, _ := object(payload)
	if err := require(nested(document, "environment", "cli", "format", "default") == "table", "config dump lost environment cli format default table: %#v", payload); err != nil {
		return err
	}

	payload, err = localJSON(
		"show config diff pipe-local.conf pipe-local-other.conf | json compact", "show config diff", output)
	if err != nil {
		return err
	}
	document, err = exactObject(payload, "config diff", "added", "changed", "removed")
	if err != nil {
		return err
	}
	added, _ := document["added"].(map[string]any)
	if err := require(added["router-id"] == "10.0.0.1",
		"config diff lost the router-id the second config adds: %#v", payload); err != nil {
		return err
	}

	payload, err = localJSON("validate config pipe-local.conf | json compact", "validate config", output)
	if err != nil {
		return err
	}
	document, _ = object(payload)
	if err := require(document["valid"] == true && document["path"] == "pipe-local.conf",
		"validate config did not answer a verdict for the file it read: %#v", payload); err != nil {
		return err
	}

	payload, err = localJSON("show config history pipe-local.conf | json compact", "show config history", output)
	if err != nil {
		return err
	}
	values, err := rows(payload, "revisions")
	if err != nil {
		return err
	}
	if err := requireAnyRow(values, func(row map[string]any) bool {
		return row["revision"] == "draft" && row["state"] == "editing in progress"
	}, "config history lost the draft row: %#v", values); err != nil {
		return err
	}

	payload, err = localJSON("show config ls | json compact", "show config ls", output)
	if err != nil {
		return err
	}
	values, err = rows(payload, "configs")
	if err != nil {
		return err
	}
	if err := requireAnyRow(values, func(row map[string]any) bool {
		return row["source"] == "fs" && row["path"] == resolvedConfigPath
	}, "config ls lost its filesystem row: %#v", values); err != nil {
		return err
	}

	payload, err = localJSON("show schema list | json compact", "show schema list", output)
	if err != nil {
		return err
	}
	values, err = rows(payload, "schemas")
	if err != nil {
		return err
	}
	if err := requireAnyRow(values, func(row map[string]any) bool {
		return row["module"] == "ze-fib-conf" && row["namespace"] == "ze.fib.conf"
	}, "schema list lost stripped-core ze-fib-conf: %#v", values); err != nil {
		return err
	}

	payload, err = localJSON("show schema methods | json compact", "show schema methods", output)
	if err != nil {
		return err
	}
	values, err = rows(payload, "methods")
	if err != nil {
		return err
	}
	invalidMethod, err := anyRow(values, func(row map[string]any) bool {
		return row["method"] == nil || row["method"] == "" || row["module"] == nil || row["module"] == ""
	})
	if err != nil {
		return err
	}
	if err := require(len(values) > 0 && !invalidMethod, "schema methods did not return method/module rows: %#v", values); err != nil {
		return err
	}

	for _, one := range []struct{ command, evidence, key string }{
		{commandShowSchemaEvents, commandShowSchemaEvents, "events"},
		{commandShowSchemaHandlers, commandShowSchemaHandlers, "handlers"},
	} {
		raw, runErr := localJSON(one.command+" | json compact", one.evidence, output)
		if runErr != nil {
			return runErr
		}
		entries, rowsErr := rows(raw, one.key)
		if rowsErr != nil {
			return rowsErr
		}
		payload, err = localJSON(one.command+" | count | json compact", one.evidence, output)
		if err != nil {
			return err
		}
		if err := validateCountDocument(payload, len(entries)); err != nil {
			return fmt.Errorf("%s: %w", one.evidence, err)
		}
	}
	payload, err = localJSON("show schema protocol | json compact", "show schema protocol", output)
	if err != nil {
		return err
	}
	if err := validateProtocolDocument(payload); err != nil {
		return err
	}

	payload, err = localJSON(fmt.Sprintf("show data ls --path %s | json compact", blobPath), "show data ls", output)
	if err != nil {
		return err
	}
	values, err = rows(payload, "keys")
	if err != nil {
		return err
	}
	if err := requireAnyRow(values, func(row map[string]any) bool {
		return row["key"] == "meta/instance/name"
	}, "data ls lost the written key: %#v", values); err != nil {
		return err
	}
	payload, err = localJSON("show data registered | json compact", "show data registered", output)
	if err != nil {
		return err
	}
	values, err = rows(payload, "patterns")
	if err != nil {
		return err
	}
	if err := requireAnyRow(values, func(row map[string]any) bool {
		return row["pattern"] == "meta/instance/name" && row["description"] == "Router instance name"
	}, "data registered lost the instance-name contract: %#v", values); err != nil {
		return err
	}

	commandTree, err := localJSON("show yang tree --commands | json compact", "show yang tree", output)
	if err != nil {
		return err
	}
	configTree, err := localJSON("show yang tree --config | json compact", "show yang tree", output)
	if err != nil {
		return err
	}
	commandValues, ok := commandTree.([]any)
	if !ok {
		return fmt.Errorf("yang command tree is not a list: %#v", commandTree)
	}
	configValues, ok := configTree.([]any)
	if !ok {
		return fmt.Errorf("yang config tree is not a list: %#v", configTree)
	}
	commandNodes, err := treeNodes(commandValues)
	if err != nil {
		return fmt.Errorf("yang command tree: %w", err)
	}
	configNodes, err := treeNodes(configValues)
	if err != nil {
		return fmt.Errorf("yang config tree: %w", err)
	}
	if err := require(len(commandNodes) > 0 && !slices.ContainsFunc(commandNodes, func(node map[string]any) bool { return node["source"] == "config" }), "yang --commands leaked a config-only node"); err != nil {
		return err
	}
	if err := require(len(configNodes) > 0 && !slices.ContainsFunc(configNodes, func(node map[string]any) bool { return node["source"] == "command" }), "yang --config leaked a command-only node"); err != nil {
		return err
	}
	commandBytes, _ := json.Marshal(commandTree)
	configBytes, _ := json.Marshal(configTree)
	if err := require(!bytes.Equal(commandBytes, configBytes), "the two YANG filters selected the same tree"); err != nil {
		return err
	}

	payload, err = localJSON("show yang completion --min-prefix 2 | json compact", "show yang completion", output)
	if err != nil {
		return err
	}
	document, _ = object(payload)
	values, err = rows(payload, "collisions")
	if err != nil {
		return err
	}
	totalAffected, err := collisionAffected(values)
	if err != nil {
		return err
	}
	summary, _ := document["summary"].(map[string]any)
	if err := require(len(values) > 0 && summary["total-groups"] == float64(len(values)) && summary["total-affected"] == float64(totalAffected), "completion summary disagrees with its rows: %#v", payload); err != nil {
		return err
	}

	payload, err = localJSON("show env list | json compact", "show env list", output)
	if err != nil {
		return err
	}
	values, err = rows(payload, "variables")
	if err != nil {
		return err
	}
	if err := requireAnyRow(values, func(row map[string]any) bool {
		_, current := row["current"]
		return row["key"] == envKeyCLIFormat && current
	}, "env list lost ze.cli.format effective data"); err != nil {
		return err
	}
	payload, err = localJSON("show env get ze.cli.format | json compact", "show env get", output)
	if err != nil {
		return err
	}
	values, err = rows(payload, "variables")
	if err != nil {
		return err
	}
	var selected map[string]any
	if len(values) == 1 {
		selected, err = rowObject(values[0])
		if err != nil {
			return err
		}
	}
	current, hasCurrent := selected["current"]
	if err := require(selected["key"] == envKeyCLIFormat && hasCurrent, "env get returned the wrong row: %#v (current=%#v)", values, current); err != nil {
		return err
	}
	payload, err = localJSON("show env registered | json compact", "show env registered", output)
	if err != nil {
		return err
	}
	values, err = rows(payload, "variables")
	if err != nil {
		return err
	}
	if err := requireAnyRow(values, func(row map[string]any) bool {
		_, current := row["current"]
		return row["key"] == envKeyCLIFormat && !current
	}, "env registered leaked or lost declaration data"); err != nil {
		return err
	}

	payload, err = localJSON("show plugins | json compact", "show plugins", output)
	if err != nil {
		return err
	}
	values, err = rows(payload, "plugins")
	if err != nil {
		return err
	}
	// The system RIB plugin is in every build: it carries no feature tag, so a
	// binary without it is a binary this walk was never run against.
	if err := requireAnyRow(values, func(row map[string]any) bool {
		description, ok := row["description"].(string)
		return row["name"] == "rib" && ok && description != ""
	}, "show plugins lost the system RIB row or its description"); err != nil {
		return err
	}
	// Every row carries the setup outcome its plugin's own init() recorded, so
	// a plugin that recorded nothing reads as "unknown" rather than as a row
	// with an empty cell. An empty or "invalid" outcome is the silence this
	// join exists to remove, so it fails the walk.
	for _, value := range values {
		row, rowErr := rowObject(value)
		if rowErr != nil {
			return rowErr
		}
		name, hasName := row["name"].(string)
		outcome, hasOutcome := row["outcome"].(string)
		if err := require(hasName && name != "", "show plugins row has no name: %#v", value); err != nil {
			return err
		}
		if err := require(hasOutcome && outcome != "" && outcome != "invalid",
			"show plugins row %q carries outcome %#v", name, row["outcome"]); err != nil {
			return err
		}
	}

	savePath := filepath.Join(work, "local-save.ndjson")
	result, err := run("ze", "cli", "-c", "show env get ze.cli.format | ndjson | save "+savePath)
	if err != nil {
		return fmt.Errorf("local save: %w: %s%s", err, result.stdout, result.stderr)
	}
	saved, err := os.ReadFile(savePath) //nolint:gosec // G304: savePath is the scratch file this function just told ze to write
	if err != nil {
		return err
	}
	if err := require(bytes.Equal(saved, result.stdout), "local save bytes differ: file=%q shown=%q", saved, result.stdout); err != nil {
		return err
	}
	info, err := os.Stat(savePath)
	if err != nil {
		return err
	}
	if err := require(info.Mode().Perm() == 0o600, "local save mode=%04o, want 0600", info.Mode().Perm()); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(output, CompletionMarker); err != nil {
		return fmt.Errorf("write coverage completion: %w", err)
	}
	return nil
}
