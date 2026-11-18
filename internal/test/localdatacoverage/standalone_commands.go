// Design: docs/functional-tests.md -- actual local entry-to-pipe evidence.
// Related: standalone_population_test.go -- the independently identified missing paths.

package localdatacoverage

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/pkg/zefs"
)

const standaloneConfig = "bgp { router-id 203.0.113.11; session { asn { local 65000; } } }\n"
const standaloneReplacement = "bgp { router-id 203.0.113.12; session { asn { local 65000; } } }\n"
const standaloneInactive = "bgp { inactive: router-id 203.0.113.11; session { asn { local 65000; } } }\n"

const (
	standaloneRouterID    = "203.0.113.11"
	standaloneRouterNext  = "203.0.113.12"
	standaloneEnvironment = "environment"
)

func suppliedContent(content string) string {
	return " content " + strconv.Quote(base64.StdEncoding.EncodeToString([]byte(content)))
}

func suppliedFields(first, second, before, after string) (string, error) {
	encoded, err := json.Marshal(map[string]string{first: before, second: after})
	if err != nil {
		return "", err
	}
	return suppliedContent(string(encoded)), nil
}

// standaloneTree reads the returned configuration with the native parser. It
// does not derive an expected answer from the transformation being judged.
func standaloneTree(content string) (*config.Tree, error) {
	schema, _, err := config.SchemaWithDeclaredPlugins(config.ConfigSource{Text: content})
	if err != nil {
		return nil, err
	}
	if config.DetectFormat(content) == config.FormatHierarchical {
		return config.NewParser(schema).Parse(content)
	}
	return config.NewSetParser(schema).Parse(content)
}

func requireStandaloneRouter(content, routerID string, inactive bool) error {
	tree, err := standaloneTree(content)
	if err != nil {
		return fmt.Errorf("returned configuration does not parse: %w", err)
	}
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return fmt.Errorf("returned configuration lost the bgp section")
	}
	value, found := bgp.Get("router-id")
	if !found {
		return fmt.Errorf("returned configuration lost router-id")
	}
	if value != routerID {
		return fmt.Errorf("returned router-id = %q, want %q", value, routerID)
	}
	return require(bgp.IsLeafInactive("router-id") == inactive,
		"returned router-id inactive=%t, want %t", bgp.IsLeafInactive("router-id"), inactive)
}

// runStandaloneCommands adds the previously missing local-data paths to the
// existing binary walk. Every child is the runner's ze, under its private store.
func runStandaloneCommands(output io.Writer, work string) error {
	if err := runStandaloneTransformations(output, work); err != nil {
		return err
	}
	if err := runStandaloneInspections(output, work); err != nil {
		return err
	}
	return runStandaloneCodecs(output)
}

func runStandaloneTransformations(output io.Writer, work string) error {
	for _, test := range []struct {
		operation, before, after, routerID string
		inactive                           bool
	}{
		{"set", standaloneConfig, "set bgp router-id 203.0.113.12", standaloneRouterNext, false},
		{"activate", standaloneInactive, "activate bgp router-id", standaloneRouterID, false},
		{"deactivate", standaloneConfig, "deactivate bgp router-id", standaloneRouterID, true},
		{"replace", standaloneConfig, standaloneReplacement, standaloneRouterNext, false},
	} {
		file := filepath.Join(work, "standalone-"+test.operation+".conf")
		if err := writeFixture(file, test.before); err != nil {
			return err
		}
		filePath := "show config " + test.operation
		fileAnswer, err := blobContent(filePath+" file "+file+suppliedContent(test.after)+" | json compact", filePath, output)
		if err != nil {
			return err
		}
		if err := requireStandaloneRouter(fileAnswer, test.routerID, test.inactive); err != nil {
			return fmt.Errorf("%s: %w", filePath, err)
		}
		pair, err := suppliedFields("before", "after", test.before, test.after)
		if err != nil {
			return err
		}
		suppliedPath := "show config supplied " + test.operation
		suppliedAnswer, err := blobContent(suppliedPath+pair+" | json compact", suppliedPath, output)
		if err != nil {
			return err
		}
		if err := requireStandaloneRouter(suppliedAnswer, test.routerID, test.inactive); err != nil {
			return fmt.Errorf("%s: %w", suppliedPath, err)
		}
		if err := require(fileAnswer == suppliedAnswer, "%s file and supplied results differ", test.operation); err != nil {
			return err
		}
		stored, err := os.ReadFile(file) //nolint:gosec // the private fixture above
		if err != nil {
			return err
		}
		if err := require(string(stored) == test.before, "%s published its detached result into the source", filePath); err != nil {
			return err
		}
	}
	return nil
}

func runStandaloneInspections(output io.Writer, work string) error {
	file := filepath.Join(work, "standalone-inspect.conf")
	if err := writeFixture(file, standaloneConfig); err != nil {
		return err
	}
	for _, operation := range []string{"migrate", "source"} {
		for _, supplied := range []bool{false, true} {
			path := "show config " + operation
			input := " file " + file
			if supplied {
				path = "show config supplied " + operation
				input = suppliedContent(standaloneConfig) + " name standalone-inspect.conf"
			}
			content, err := blobContent(path+input+" | json compact", path, output)
			if err != nil {
				return err
			}
			if err := requireStandaloneRouter(content, standaloneRouterID, false); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
		}
	}
	// Invalid input makes the plan nonempty and binds its diagnostic to the
	// given label. A fake always-valid document cannot pass this check.
	invalid := "bgp { router-id invalid-address; }\n"
	invalidFile := filepath.Join(work, "standalone-invalid.conf")
	if err := writeFixture(invalidFile, invalid); err != nil {
		return err
	}
	for _, supplied := range []bool{false, true} {
		path, input, label := "show config fix-plan", " file "+invalidFile, invalidFile
		if supplied {
			path, input, label = "show config supplied fix-plan", suppliedContent(invalid)+" name standalone-invalid.conf", "standalone-invalid.conf"
		}
		payload, err := localJSON(path+input+" | json compact", path, output)
		if err != nil {
			return err
		}
		document, err := exactObject(payload, path, "schema-version", "path", "diagnostics")
		if err != nil {
			return err
		}
		diagnostics, err := rows(document, "diagnostics")
		if err != nil {
			return err
		}
		if err := require(document["path"] == label && len(diagnostics) > 0,
			"%s lost the invalid source's label or diagnostics: %#v", path, document); err != nil {
			return err
		}
	}
	return runStandaloneSuppliedReaders(output, work)
}

func runStandaloneSuppliedReaders(output io.Writer, work string) error {
	const label = "caller-upload.conf"
	// A different local file with the diagnostic name proves name is a label.
	labelPath := filepath.Join(work, label)
	if err := writeFixture(labelPath, standaloneReplacement); err != nil {
		return err
	}
	input := suppliedContent(standaloneConfig) + " name " + label
	payload, err := localJSON("show config supplied dump"+input+" | json compact", "show config supplied dump", output)
	if err != nil {
		return err
	}
	document, isObject := object(payload)
	if !isObject {
		return fmt.Errorf("supplied dump is not a document: %#v", payload)
	}
	if err := require(nested(document, "bgp", "router-id") == standaloneRouterID, "supplied dump ignored the caller's bytes: %#v", payload); err != nil {
		return err
	}
	payload, err = localJSON("show config supplied validation"+input+" | json compact", "show config supplied validation", output)
	if err != nil {
		return err
	}
	document, isObject = object(payload)
	if !isObject {
		return fmt.Errorf("supplied validation is not a document: %#v", payload)
	}
	if err := require(document["valid"] == true && document["path"] == label, "supplied validation lost verdict/label: %#v", payload); err != nil {
		return err
	}
	content, err := blobContent("show config supplied fmt"+input+" | json compact", "show config supplied fmt", output)
	if err != nil {
		return err
	}
	if err := requireStandaloneRouter(content, standaloneRouterID, false); err != nil {
		return err
	}
	content, err = blobContent("show config supplied graph"+input+" | json compact", "show config supplied graph", output)
	if err != nil {
		return err
	}
	var graph config.Graph
	if err := json.Unmarshal([]byte(content), &graph); err != nil {
		return err
	}
	found := false
	for _, node := range graph.Nodes {
		if node.ID == "section/bgp" {
			found = node.Kind == config.NodeSection && node.Name == "bgp"
		}
	}
	if !found {
		return fmt.Errorf("supplied graph lost the source's bgp section: %#v", graph)
	}
	pair, err := suppliedFields("before", "after", standaloneConfig, standaloneReplacement)
	if err != nil {
		return err
	}
	payload, err = localJSON("show config supplied diff"+pair+" | json compact", "show config supplied diff", output)
	if err != nil {
		return err
	}
	document, err = exactObject(payload, "supplied diff", "added", "changed", "removed")
	if err != nil {
		return err
	}
	if err := require(nested(document, "changed", "router-id", "old") == standaloneRouterID && nested(document, "changed", "router-id", "new") == standaloneRouterNext,
		"supplied diff ignored one source: %#v", payload); err != nil {
		return err
	}
	completion, err := suppliedFields("configuration", "input", standaloneConfig, "set router")
	if err != nil {
		return err
	}
	payload, err = localJSON("show config supplied completion"+completion+" context bgp | json compact", "show config supplied completion", output)
	if err != nil {
		return err
	}
	candidates, err := rows(payload, "completions")
	if err != nil {
		return err
	}
	if err := requireAnyRow(candidates, func(row map[string]any) bool { return row["text"] == "router-id" }, "supplied completion lost router-id: %#v", payload); err != nil {
		return err
	}
	stored, err := os.ReadFile(labelPath) //nolint:gosec // the private label control above
	if err != nil {
		return err
	}
	if err := require(string(stored) == standaloneReplacement, "supplied input overwrote its diagnostic label"); err != nil {
		return err
	}
	payload, err = localJSON("show config environment-validation | json compact", "show config environment-validation", output)
	if err != nil {
		return err
	}
	document, isObject = object(payload)
	if !isObject {
		return fmt.Errorf("environment validation is not a document: %#v", payload)
	}
	if err := require(document["path"] == standaloneEnvironment && document["valid"] == true,
		"environment validation lost its actual verdict: %#v", payload); err != nil {
		return err
	}
	// This compiled-in integer is deliberately malformed only in the child.
	// Its invalid verdict must survive the CLI's nonzero-exit delivery.
	result, runErr := runEnvironment(append(os.Environ(), "ze.fwd.dest.cap=not-an-integer"),
		"ze", "cli", "-c", "show config environment-validation | json compact")
	if runErr == nil {
		return fmt.Errorf("environment validation accepted a malformed registered integer")
	}
	if err := json.Unmarshal(result.stdout, &document); err != nil {
		return fmt.Errorf("environment refusal lost its typed verdict: %w: %s%s", err, result.stdout, result.stderr)
	}
	if err := require(document["path"] == standaloneEnvironment && document["valid"] == false,
		"environment validation did not refuse the actual bad setting: %#v", document); err != nil {
		return err
	}
	diagnostics, err := rows(document, "diagnostics")
	if err != nil {
		return err
	}
	return requireAnyRow(diagnostics, func(row map[string]any) bool { return row["path"] == "ze.fwd.dest.cap" },
		"environment refusal lost the offending setting: %#v", document)
}

func runStandaloneCodecs(output io.Writer) error {
	const data = "literal data\n"
	frame, err := blobContent("show data encode"+suppliedContent(data)+" capacity 20 | json compact", "show data encode", output)
	if err != nil {
		return err
	}
	decoded, capacity, next, err := zefs.DecodeNetcapstringRef([]byte(frame), 0)
	if err != nil {
		return fmt.Errorf("data encode answered no usable frame: %w", err)
	}
	if err := require(string(decoded) == data && capacity == 20 && next == len(frame), "data encode lost bytes/capacity/frame boundary"); err != nil {
		return err
	}
	for _, input := range []string{"", " \t\n", "\x00\xff\n"} {
		payload, err := localJSON("show data encode"+suppliedContent(input)+" | json compact", "show data encode", output)
		if err != nil {
			return err
		}
		document, err := exactObject(payload, "data encode", "blob")
		if err != nil {
			return err
		}
		wire, err := json.Marshal(document["blob"])
		if err != nil {
			return err
		}
		frame, err := command.DecodeBlob(wire)
		if err != nil {
			return err
		}
		decoded, _, next, err := zefs.DecodeNetcapstringRef(frame, 0)
		if err != nil {
			return err
		}
		if err := require(string(decoded) == input && next == len(frame), "data encode changed empty, whitespace or binary input"); err != nil {
			return err
		}
	}
	const plain = "literal private value! "
	encoded, err := blobContent("show secret encode"+suppliedContent(plain)+" | json compact", "show secret encode", output)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(encoded, "$9$") {
		return fmt.Errorf("secret encode answered no reversible value")
	}
	secret, err := blobContent("show secret decode"+suppliedContent(strings.TrimSuffix(encoded, "\n"))+" | json compact", "show secret decode", output)
	if err != nil {
		return err
	}
	if err := require(secret == plain+"\n", "secret round trip changed the literal caller bytes"); err != nil {
		return err
	}
	for _, test := range []struct{ operation, content, path string }{
		{"config", "neighbor 192.0.2.2 { router-id 203.0.113.11; local-address 192.0.2.1; local-as 65000; peer-as 65001; }\n", "bgp"},
		{standaloneEnvironment, "[exabgp.daemon]\nuser = nobody\n", standaloneEnvironment},
	} {
		path := "show exabgp migration " + test.operation
		content, err := blobContent(path+suppliedContent(test.content)+" name caller-input.conf | json compact", path, output)
		if err != nil {
			return err
		}
		tree, err := standaloneTree(content)
		if err != nil {
			return fmt.Errorf("%s answered unusable configuration: %w", path, err)
		}
		section := tree.GetContainer(test.path)
		if section == nil {
			return fmt.Errorf("%s lost its converted %s section", path, test.path)
		}
		if test.operation == standaloneEnvironment {
			daemon := section.GetContainer("daemon")
			if daemon == nil {
				return fmt.Errorf("environment migration lost daemon settings")
			}
			user, found := daemon.Get("user")
			if err := require(found && user == "nobody", "environment migration lost its configured user"); err != nil {
				return err
			}
			continue
		}
		if err := requireMigratedPeer(section); err != nil {
			return err
		}
	}
	return runStandaloneRefusals(output)
}

func requireMigratedPeer(bgp *config.Tree) error {
	for _, group := range bgp.GetListOrdered("group") {
		for _, peer := range group.Value.GetListOrdered("peer") {
			remote := peer.Value.GetContainerPath("connection/remote")
			if remote == nil {
				continue
			}
			address, found := remote.Get("ip")
			if !found {
				continue
			}
			if address != "192.0.2.2" {
				continue
			}
			asn := peer.Value.GetContainerPath("session/asn")
			if asn == nil {
				return fmt.Errorf("migration lost the requested peer AS settings")
			}
			remoteAS, found := asn.Get("remote")
			return require(found && remoteAS == "65001", "migration lost the requested remote AS")
		}
	}
	return fmt.Errorf("migration lost the requested peer address")
}

// Refusals are tested at the actual command process boundary: failure must be
// nonzero, explain itself, and produce no fabricated successful document.
func runStandaloneRefusals(output io.Writer) error {
	for _, line := range []string{
		"show config supplied dump content !!! | json compact",
		"show config supplied source" + suppliedContent(standaloneConfig) + " name ../host.conf | json compact",
		"show config supplied set" + suppliedContent(`{"before":"","after":"delete all"}`) + " | json compact",
		"show exabgp migration config content !!! | json compact",
		"show secret decode" + suppliedContent("$9$~~~~") + " | json compact",
		"show data encode" + suppliedContent("too long") + " capacity 1 | json compact",
	} {
		result, err := run("ze", "cli", "-c", line)
		if err == nil {
			return fmt.Errorf("invalid standalone input was accepted: %s", line)
		}
		if len(result.stderr) == 0 {
			return fmt.Errorf("standalone refusal lost its explanation: %s", line)
		}
		if len(result.stdout) != 0 {
			return fmt.Errorf("standalone refusal returned fabricated successful output: %s: %q", line, result.stdout)
		}
	}
	_, err := fmt.Fprintln(output, "CHECKED: standalone supplied input refusals [done]")
	return err
}
