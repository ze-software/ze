package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

const processTimeout = 15 * time.Second

func init() {
	Register("ui/pipe-review-entry-contracts", uiDriver(pipeReviewEntryContracts))
}

type uiPipeReviewEntryContractsCommandResult struct {
	code   int
	stdout string
	stderr string
}

func pipeReviewEntryContracts(ctx context.Context) error {
	// Standalone stdin is an explicit all-fields surface. This exact
	// non-address document must pass through resolve unchanged.
	standaloneInput := "{\"label\":\"edge\",\"metric\":7}\n"
	standalone := uiPipeReviewEntryContractsRunCommand(ctx, []string{"ze", "pipe", pipeResolve}, &standaloneInput)
	if err := uiPipeReviewEntryContractsRequire(standalone.code == 0,
		"standalone resolve exit=%d: %s%s",
		standalone.code, standalone.stdout, standalone.stderr); err != nil {
		return err
	}
	var standaloneValue any
	if err := json.Unmarshal([]byte(standalone.stdout), &standaloneValue); err != nil {
		return fmt.Errorf("standalone resolve did not answer JSON: %w: %s%s",
			err, standalone.stdout, standalone.stderr)
	}
	wantStandalone := map[string]any{"label": "edge", ipWordMetric: float64(7)}
	if err := uiPipeReviewEntryContractsRequire(reflect.DeepEqual(standaloneValue, wantStandalone),
		"standalone resolve changed exact input data: %q", standalone.stdout); err != nil {
		return err
	}

	// A typo must refuse before any answer is emitted.
	display := uiPipeReviewEntryContractsRunCommand(ctx,
		[]string{"ze", areaCLI, "-c", "show env list | display tpyofield"}, nil)
	if err := requireRefusal(display, "display", "no field"); err != nil {
		return err
	}

	// An explicit local-data format uses the same pipe grammar as dispatch.
	localRaw := uiPipeReviewEntryContractsRunCommand(ctx,
		[]string{"ze", areaCLI, "-c", "show schema protocol", "--format", renderRaw}, nil)
	if err := uiPipeReviewEntryContractsRequire(localRaw.code == 0,
		"local --format raw exit=%d: %s%s",
		localRaw.code, localRaw.stdout, localRaw.stderr); err != nil {
		return err
	}
	if err := uiPipeReviewEntryContractsRequire(localRaw.stdout == "{\"protocol\":\"Hub Architecture\",\"version\":\"1.0\"}\n",
		"local --format raw changed bytes: %q", localRaw.stdout); err != nil {
		return err
	}
	if err := uiPipeReviewEntryContractsRequire(localRaw.stderr == "",
		"local --format raw wrote stderr: %q", localRaw.stderr); err != nil {
		return err
	}

	unknownFormat := "definitely-not-a-format"
	localUnknown := uiPipeReviewEntryContractsRunCommand(ctx, []string{
		"ze", areaCLI, "-c", "show schema protocol", "--format", unknownFormat,
	}, nil)
	if err := requireRefusal(localUnknown, unknownFormat, "unknown pipe operator"); err != nil {
		return err
	}

	// The per-command machine contract is reached through the shipped help
	// command. The local shortcut must not suppress the daemon operator surface.
	versionResult := uiPipeReviewEntryContractsRunCommand(ctx,
		[]string{"ze", "help", argCommand, cmdShowVersion, "--json"}, nil)
	version, err := oneJSONObject(versionResult, "show version published contract")
	if err != nil {
		return err
	}
	operators, ok := objectSlice(version["operators"])
	if err := uiPipeReviewEntryContractsRequire(ok, "show version published no operator list: %#v", version); err != nil {
		return err
	}
	byName, err := operatorsByName(operators, "show version")
	if err != nil {
		return err
	}
	wantNames := stringSet(
		"json", "ndjson", "table", "text", "yaml", "raw", "no-more",
		"log", "save", "match", "count", "first", "last", "display", "fill",
	)
	if err := uiPipeReviewEntryContractsRequire(uiPipeReviewEntryContractsEqualStringSets(keys(byName), wantNames),
		"show version operator population changed: %#v", uiPipeReviewEntryContractsSortedKeys(byName)); err != nil {
		return err
	}
	always := availabilitySet(byName, "always")
	if err := uiPipeReviewEntryContractsRequire(uiPipeReviewEntryContractsEqualStringSets(always, stringSet(
		"json", "ndjson", "table", "text", "yaml", "raw", "no-more", "save")),
		"show version always qualifier changed: %#v", byName); err != nil {
		return err
	}
	withRows := availabilitySet(byName, "with-rows")
	if err := uiPipeReviewEntryContractsRequire(uiPipeReviewEntryContractsEqualStringSets(withRows, stringSet(
		"match", "count", "first", "last", "display", "fill")),
		"show version with-rows qualifier changed: %#v", byName); err != nil {
		return err
	}
	streaming := availabilitySet(byName, "when-streaming")
	if err := uiPipeReviewEntryContractsRequire(uiPipeReviewEntryContractsEqualStringSets(streaming, stringSet("log")),
		"show version streaming qualifier changed: %#v", byName); err != nil {
		return err
	}
	if localOnly, ok := byName["save"]["local-only"].(bool); !ok || !localOnly {
		return fmt.Errorf("show version save lost local-only=true: %#v", byName["save"])
	}
	for name, operator := range byName {
		if name == "save" {
			continue
		}
		if localOnly, exists := operator["local-only"]; exists {
			value, ok := localOnly.(bool)
			if !ok || value {
				return fmt.Errorf("a non-save show version operator became local-only: %#v", byName)
			}
		}
	}
	for _, operator := range operators {
		class, ok := operator["class"].(string)
		if !ok || (class != "global" && class != "data" && class != "stream") {
			return fmt.Errorf("show version operator lost its class qualifier: %#v", operators)
		}
	}

	// These are the exact fields the primary website renders from the same
	// machine contract.
	monitorResult := uiPipeReviewEntryContractsRunCommand(ctx,
		[]string{"ze", "help", argCommand, "monitor ping", "--json"}, nil)
	monitorPing, err := oneJSONObject(monitorResult, "monitor ping published contract")
	if err != nil {
		return err
	}
	if err := uiPipeReviewEntryContractsRequire(monitorPing["answer-shape"] == shapeTab,
		"monitor ping answer-shape=%#v, want tab", monitorPing["answer-shape"]); err != nil {
		return err
	}
	wantAddressFields := []any{"target"}
	if err := uiPipeReviewEntryContractsRequire(reflect.DeepEqual(monitorPing["address-fields"], wantAddressFields),
		"monitor ping address-fields changed: %#v", monitorPing["address-fields"]); err != nil {
		return err
	}
	monitorOperators, _ := objectSlice(monitorPing["operators"])
	monitorByName, err := operatorsByName(monitorOperators, "monitor ping")
	if err != nil {
		return err
	}
	resolveAlways := monitorByName["resolve"] != nil && monitorByName["resolve"]["available"] == valueAlways
	originAlways := monitorByName["origin"] != nil && monitorByName["origin"]["available"] == valueAlways
	if err := uiPipeReviewEntryContractsRequire(resolveAlways && originAlways,
		"declared address field did not publish resolve/origin: %#v", monitorByName); err != nil {
		return err
	}

	// Render the primary catalog page into an isolated fixture tree.
	work, err := os.MkdirTemp("", "ze-ui-pipe-review-entry-contracts-")
	if err != nil {
		return fmt.Errorf("create fixture working directory: %w", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup
	// The precondition that stood here stat'ed $ZE_REPO_ROOT/website/tools, the
	// Python renderer set that commit eae282592 deleted when the site renderers
	// moved to Go. Nothing read the directory. renderCLICatalog below is this
	// fixture's own renderer, so the stat gated the assertions on a path that
	// exists in no checkout. The case passed only where a developer still had
	// the deleted tree, and a __pycache__ left behind is enough for that. The
	// shipped renderer that replaced it is internal/le/site/catalog.go, held to
	// these same two fields by TestEveryPublishedCatalogFieldReachesAReader.
	destination := filepath.Join(work, "rendered-cli", "index.html")
	if err := renderCLICatalog(destination, []map[string]any{monitorPing}); err != nil {
		return err
	}
	renderedBytes, err := os.ReadFile(destination) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("read rendered primary CLI page: %w", err)
	}
	renderedPage := string(renderedBytes)
	if err := uiPipeReviewEntryContractsRequire(strings.Contains(renderedPage,
		"<span>Answer shape</span><code>tab</code>"),
		"primary CLI page omitted answer-shape: %q", renderedPage); err != nil {
		return err
	}
	if err := uiPipeReviewEntryContractsRequire(strings.Contains(renderedPage,
		"<span>Address fields</span><code>target</code>"),
		"primary CLI page omitted address-fields: %q", renderedPage); err != nil {
		return err
	}

	// Operator headings and --format values come from the real catalog help.
	cliHelp := uiPipeReviewEntryContractsRunCommand(ctx, []string{"ze", areaCLI, flagHelp}, nil)
	if err := uiPipeReviewEntryContractsRequire(cliHelp.code == 0,
		"ze cli --help exit=%d: %s%s", cliHelp.code, cliHelp.stdout, cliHelp.stderr); err != nil {
		return err
	}
	helpText := cliHelp.stdout + cliHelp.stderr
	formatSentence := "Output format: json, ndjson, table, text, yaml, raw " +
		"(default: the daemon's environment cli format default)"
	if err := uiPipeReviewEntryContractsRequire(strings.Contains(helpText, formatSentence),
		"ze cli --help format/default sentence changed: %q", helpText); err != nil {
		return err
	}
	for _, heading := range []string{
		"Global pipe operators",
		"Data pipe operators (when the answer has rows)",
		"Stream pipe operators (when the command keeps answering)",
	} {
		if err := uiPipeReviewEntryContractsRequire(strings.Contains(helpText, heading),
			"ze cli --help lost derived section %q: %q", heading, helpText); err != nil {
			return err
		}
	}
	for _, entry := range []string{
		"<command> | raw",
		"<command> | save <path>",
		"<command> | log",
	} {
		if err := uiPipeReviewEntryContractsRequire(strings.Contains(helpText, entry),
			"ze cli --help lost catalog entry %q: %q", entry, helpText); err != nil {
			return err
		}
	}

	fmt.Println("OK: reviewed pipe entry contracts")
	return nil
}

func uiPipeReviewEntryContractsRequire(condition bool, format string, args ...any) error {
	if condition {
		return nil
	}
	return fmt.Errorf(format, args...)
}

func uiPipeReviewEntryContractsRunCommand(ctx context.Context, argv []string, input *string) uiPipeReviewEntryContractsCommandResult {
	if len(argv) == 0 {
		return uiPipeReviewEntryContractsCommandResult{code: 127, stderr: "empty command"}
	}
	commandCtx, cancel := context.WithTimeout(ctx, processTimeout)
	defer cancel()

	cmd := exec.CommandContext(commandCtx, argv[0], argv[1:]...) //nolint:gosec // the fixture chooses the program and its arguments
	if input != nil {
		cmd.Stdin = strings.NewReader(*input)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return uiPipeReviewEntryContractsCommandResult{code: 127, stderr: err.Error()}
	}
	err := cmd.Wait()
	result := uiPipeReviewEntryContractsCommandResult{stdout: stdout.String(), stderr: stderr.String()}
	if commandCtx.Err() != nil {
		result.code = 124
		result.stderr += fmt.Sprintf("process timed out after %d seconds", int(processTimeout/time.Second))
		return result
	}
	if err == nil {
		result.code = 0
		return result
	}
	if exitError, ok := errors.AsType[*exec.ExitError](err); ok {
		result.code = exitError.ExitCode()
		return result
	}
	result.code = 127
	result.stderr += err.Error()
	return result
}

func requireRefusal(result uiPipeReviewEntryContractsCommandResult, operator, reason string) error {
	combined := strings.ToLower(result.stdout + result.stderr)
	if err := uiPipeReviewEntryContractsRequire(result.code != 0,
		"%s refusal exit=%d, want nonzero: %s%s",
		operator, result.code, result.stdout, result.stderr); err != nil {
		return err
	}
	if err := uiPipeReviewEntryContractsRequire(strings.Contains(combined, strings.ToLower(operator)),
		"%s refusal did not name the operator: %q", operator, combined); err != nil {
		return err
	}
	if err := uiPipeReviewEntryContractsRequire(strings.Contains(combined, strings.ToLower(reason)),
		"%s refusal did not say %q: %q", operator, reason, combined); err != nil {
		return err
	}
	return uiPipeReviewEntryContractsRequire(result.stdout == "",
		"%s refusal produced an answer before failing: %q", operator, result.stdout)
}

func oneJSONObject(result uiPipeReviewEntryContractsCommandResult, label string) (map[string]any, error) {
	if result.code != 0 {
		return nil, fmt.Errorf("%s exit=%d: %s%s",
			label, result.code, result.stdout, result.stderr)
	}
	var value any
	if err := json.Unmarshal([]byte(result.stdout), &value); err != nil {
		return nil, fmt.Errorf("%s did not answer JSON: %w\n%s%s",
			label, err, result.stdout, result.stderr)
	}
	if values, ok := value.([]any); ok {
		if len(values) == 0 {
			return nil, fmt.Errorf("%s answered an empty JSON list", label)
		}
		value = values[0]
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s did not answer a JSON object: %#v", label, value)
	}
	return object, nil
}

func objectSlice(value any) ([]map[string]any, bool) {
	values, ok := value.([]any)
	if !ok {
		return nil, false
	}
	objects := make([]map[string]any, 0, len(values))
	for _, value := range values {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		objects = append(objects, object)
	}
	return objects, true
}

func operatorsByName(operators []map[string]any, label string) (map[string]map[string]any, error) {
	result := make(map[string]map[string]any, len(operators))
	for _, operator := range operators {
		name, ok := operator["name"].(string)
		if !ok {
			return nil, fmt.Errorf("%s operator has no string name: %#v", label, operator)
		}
		result[name] = operator
	}
	return result, nil
}

func stringSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func keys(values map[string]map[string]any) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for key := range values {
		result[key] = struct{}{}
	}
	return result
}

func uiPipeReviewEntryContractsSortedKeys(values map[string]map[string]any) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func availabilitySet(operators map[string]map[string]any, availability string) map[string]struct{} {
	result := make(map[string]struct{})
	for name, operator := range operators {
		if operator["available"] == availability {
			result[name] = struct{}{}
		}
	}
	return result
}

func uiPipeReviewEntryContractsEqualStringSets(left, right map[string]struct{}) bool {
	if len(left) != len(right) {
		return false
	}
	for value := range left {
		if _, ok := right[value]; !ok {
			return false
		}
	}
	return true
}

func renderCLICatalog(destination string, commands []map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return fmt.Errorf("create rendered CLI directory: %w", err)
	}
	var page strings.Builder
	page.WriteString("<!doctype html>\n<html><head><meta charset=\"utf-8\"><title>CLI catalog</title></head><body>\n")
	page.WriteString("<main class=\"cli-catalog\">\n")
	for _, command := range commands {
		name, _ := command["name"].(string)
		if name == "" {
			name, _ = command["command"].(string)
		}
		page.WriteString("<article class=\"cli-command\">")
		page.WriteString("<h2>")
		page.WriteString(html.EscapeString(name))
		page.WriteString("</h2>")
		if shape, ok := command["answer-shape"].(string); ok {
			page.WriteString("<div class=\"cli-contract-field\"><span>Answer shape</span><code>")
			page.WriteString(html.EscapeString(shape))
			page.WriteString("</code></div>")
		}
		if rawFields, ok := command["address-fields"].([]any); ok {
			fields := make([]string, 0, len(rawFields))
			for _, rawField := range rawFields {
				if field, ok := rawField.(string); ok {
					fields = append(fields, field)
				}
			}
			page.WriteString("<div class=\"cli-contract-field\"><span>Address fields</span><code>")
			page.WriteString(html.EscapeString(strings.Join(fields, ", ")))
			page.WriteString("</code></div>")
		}
		if operators, ok := objectSlice(command["operators"]); ok {
			page.WriteString("<ul class=\"cli-operators\">")
			for _, operator := range operators {
				operatorName, _ := operator["name"].(string)
				page.WriteString("<li><code>")
				page.WriteString(html.EscapeString(operatorName))
				page.WriteString("</code></li>")
			}
			page.WriteString("</ul>")
		}
		page.WriteString("</article>\n")
	}
	page.WriteString("</main>\n</body></html>\n")
	if err := os.WriteFile(destination, []byte(page.String()), 0o600); err != nil {
		return fmt.Errorf("write rendered primary CLI page: %w", err)
	}
	return nil
}
