// Design: docs/architecture/api/commands.md -- local-data commands
// Related: localdatacoverage.go -- the scenario, the evidence population and its helpers

package localdatacoverage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// debugModuleCoverage is the subsystem the debug walk enables and removes. Any
// name without a slash is a module the profile can hold, so the walk owns one
// no real subsystem uses.
const debugModuleCoverage = "coverage.local"

// The keys the fake `wg` answers. They are not WireGuard keys: the point is
// that the private key `wg genkey` printed is the one `wg pubkey` read.
const (
	wgFakePrivateKey = "coverage-private-key"
	wgFakePublicKey  = "public-of-" + wgFakePrivateKey
)

// debugChange runs a debug command that changes state and checks the one
// change it answers: the subject, the result, and the name when one is named.
func debugChange(command, evidence, subject, name, result string, output io.Writer) (map[string]any, error) {
	payload, err := localJSON(command, evidence, output)
	if err != nil {
		return nil, err
	}
	document, err := exactObject(payload, evidence, "debug-change")
	if err != nil {
		return nil, err
	}
	change, isObject := object(document["debug-change"])
	if !isObject {
		return nil, fmt.Errorf("%s answered a change that is not an object: %#v", evidence, payload)
	}
	if err := require(change["subject"] == subject && change["result"] == result,
		"%s answered %#v, want subject %q result %q", evidence, change, subject, result); err != nil {
		return nil, err
	}
	if name == "" {
		return change, nil
	}
	return change, require(change["name"] == name, "%s answered %#v, want name %q", evidence, change, name)
}

// runDebugCommands covers the eight debug commands in the order an operator
// uses them, against the debug store under the private workspace. Each answer
// is checked against the state the commands before it left, so an answer that
// ignores the store is refused.
func runDebugCommands(output io.Writer) error {
	if _, err := debugChange("set debug module "+debugModuleCoverage+" flag f | json compact", "set debug module",
		"module", debugModuleCoverage, "enabled", output); err != nil {
		return err
	}
	change, err := debugChange("set debug timeout 90s | json compact", "set debug timeout", "timeout", "", "set", output)
	if err != nil {
		return err
	}
	if err := require(change["minutes"] == float64(2), "set debug timeout 90s answered %#v, want 2 minutes", change); err != nil {
		return err
	}
	if _, err := debugChange("set debug profile name p1 | json compact", "set debug profile name",
		"profile", "p1", "saved", output); err != nil {
		return err
	}

	payload, err := localJSON("show debug profile | json compact", "show debug profile", output)
	if err != nil {
		return err
	}
	// A one-key envelope whose value is a list reaches `| json` as the list
	// itself, the same unwrapping rows() accepts.
	listed := payload
	if document, isObject := object(payload); isObject {
		listed = document["debug-profiles"]
	}
	profiles, _ := listed.([]any)
	if err := require(slices.Contains(profiles, any("default")) && slices.Contains(profiles, any("p1")),
		"show debug profile listed %#v, want default and p1", payload); err != nil {
		return err
	}
	payload, err = localJSON("show debug profile name default | json compact", "show debug profile", output)
	if err != nil {
		return err
	}
	modules, err := rows(payload, "debug-modules")
	if err != nil {
		return err
	}
	if err := requireAnyRow(modules, func(row map[string]any) bool {
		return row["module"] == debugModuleCoverage && row["flags"] == "f"
	}, "show debug profile name default lost module %s flag f: %#v", debugModuleCoverage, payload); err != nil {
		return err
	}

	if _, err := debugChange("set debug active name p1 | json compact", "set debug active name",
		"profile", "p1", "applied", output); err != nil {
		return err
	}
	if _, err := debugChange("delete debug module "+debugModuleCoverage+" | json compact", "delete debug module",
		"module", debugModuleCoverage, "disabled", output); err != nil {
		return err
	}
	if _, err := debugChange("delete debug profile name p1 | json compact", "delete debug profile name",
		"profile", "p1", "deleted", output); err != nil {
		return err
	}
	_, err = debugChange("clear debug | json compact", "clear debug", "state", "", "cleared", output)
	return err
}

// runToolCommands covers the diagnostic, skill and WireGuard commands. Each
// answer is checked against the record it names, so an answer about another
// code or skill, or an empty one, is refused.
func runToolCommands(output io.Writer, work string) error {
	payload, err := localJSON("show diagnostic config-parse | json compact", "show diagnostic", output)
	if err != nil {
		return err
	}
	document, err := exactObject(payload, "show diagnostic", "diagnostic")
	if err != nil {
		return err
	}
	explained, _ := object(document["diagnostic"])
	title, _ := explained["title"].(string)
	if err := require(explained["code"] == "config-parse" && title != "",
		"show diagnostic config-parse answered %#v", payload); err != nil {
		return err
	}

	payload, err = localJSON("show skills | json compact", "show skills", output)
	if err != nil {
		return err
	}
	skills, err := rows(payload, "skills")
	if err != nil {
		return err
	}
	if err := requireAnyRow(skills, func(row map[string]any) bool { return row["name"] == "ze" },
		"show skills did not list the ze skill: %#v", payload); err != nil {
		return err
	}
	payload, err = localJSON("show skill ze | json compact", "show skill", output)
	if err != nil {
		return err
	}
	document, err = exactObject(payload, "show skill", "skill")
	if err != nil {
		return err
	}
	skill, _ := object(document["skill"])
	content, _ := skill["content"].(string)
	if err := require(skill["name"] == "ze" && content != "", "show skill ze answered %#v", payload); err != nil {
		return err
	}

	if err := runWireGuardKeypair(output, work); err != nil {
		return err
	}
	return runSupportArchive(output, work)
}

// runSupportArchive covers `request support archive`. The one module asked
// for keeps the walk fast; the check proves the manifest it answers names a
// file under the directory given and that the file is a gzip stream, so an answer naming a
// path nobody wrote is refused.
func runSupportArchive(output io.Writer, work string) error {
	dir := filepath.Join(work, "support")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create the support archive directory: %w", err)
	}
	payload, err := localJSON("request support archive module version --output "+dir+" | json compact",
		"request support archive", output)
	if err != nil {
		return err
	}
	manifest, isObject := object(payload)
	if !isObject {
		return fmt.Errorf("request support archive answered no manifest: %#v", payload)
	}
	path, isText := manifest["archive-path"].(string)
	if !isText {
		return fmt.Errorf("request support archive answered a manifest with no archive-path: %#v", manifest)
	}
	if err := require(filepath.Dir(path) == dir, "request support archive answered %q, want a file under %q", path, dir); err != nil {
		return err
	}
	archive, err := os.ReadFile(path) //nolint:gosec // the path the command answered, checked to lie under dir
	if err != nil {
		return fmt.Errorf("read the archive request support archive answered: %w", err)
	}
	return require(len(archive) > 2 && archive[0] == 0x1f && archive[1] == 0x8b,
		"request support archive wrote %d bytes to %s that are not a gzip stream", len(archive), path)
}

// runWireGuardKeypair covers `request wireguard keypair`. The command runs
// `wg`, which no build host carries, so a fake one on PATH answers it, and the
// check proves the private key `wg genkey` printed is the one `wg pubkey` read.
func runWireGuardKeypair(output io.Writer, work string) error {
	bin := filepath.Join(work, "fake-bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		return fmt.Errorf("create the fake wg directory: %w", err)
	}
	script := "#!/bin/sh\ncase \"$1\" in\n" +
		"genkey) echo " + wgFakePrivateKey + " ;;\n" +
		"pubkey) read key; echo \"public-of-$key\" ;;\n" +
		"*) exit 2 ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(bin, "wg"), []byte(script), 0o700); err != nil { //nolint:gosec // G306: the fake wg must be executable by its owner
		return fmt.Errorf("write the fake wg: %w", err)
	}
	previousPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", bin+string(os.PathListSeparator)+previousPath); err != nil {
		return fmt.Errorf("put the fake wg on PATH: %w", err)
	}
	payload, err := localJSON("request wireguard keypair | json compact", "request wireguard keypair", output)
	if restoreErr := os.Setenv("PATH", previousPath); restoreErr != nil {
		return fmt.Errorf("restore PATH: %w", restoreErr)
	}
	if err != nil {
		return err
	}
	document, err := exactObject(payload, "request wireguard keypair", "wireguard-keypair")
	if err != nil {
		return err
	}
	keypair, err := exactObject(document["wireguard-keypair"], "wireguard keypair", "private-key", "public-key")
	if err != nil {
		return err
	}
	return require(keypair["private-key"] == wgFakePrivateKey && keypair["public-key"] == wgFakePublicKey,
		"request wireguard keypair answered %#v, want private key %q and public key %q",
		keypair, wgFakePrivateKey, wgFakePublicKey)
}

// runBGPToolCommands covers `show bgp encode` and `show bgp decode` as one
// round trip: the hex the encoder answers is what the decoder reads, and the
// decoded UPDATE must carry the prefix the route announced. An encoder that
// answers other bytes, or a decoder that answers nothing, is refused.
func runBGPToolCommands(output io.Writer) error {
	payload, err := localJSON("show bgp encode route 10.0.0.0/24 next-hop 192.168.1.1 | json compact",
		"show bgp encode", output)
	if err != nil {
		return err
	}
	document, err := exactObject(payload, "show bgp encode", "bgp-encode")
	if err != nil {
		return err
	}
	encoded, _ := object(document["bgp-encode"])
	hex, _ := encoded["hex"].(string)
	if err := require(hex != "" && !strings.ContainsAny(hex, " \n"),
		"show bgp encode answered no single hex message: %#v", payload); err != nil {
		return err
	}

	payload, err = localJSON("show bgp decode "+hex+" | json compact", "show bgp decode", output)
	if err != nil {
		return err
	}
	document, err = exactObject(payload, "show bgp decode", "bgp-decode")
	if err != nil {
		return err
	}
	decoded, _ := object(document["bgp-decode"])
	messages, err := rows(decoded, "messages")
	if err != nil {
		return err
	}
	if err := require(len(messages) == 1, "show bgp decode answered %d messages, want 1: %#v", len(messages), payload); err != nil {
		return err
	}
	message, _ := object(messages[0])
	body, err := json.Marshal(message["decoded"])
	if err != nil {
		return fmt.Errorf("show bgp decode: re-read the decoded message: %w", err)
	}
	return require(message["error"] == nil && strings.Contains(string(body), "10.0.0.0/24") &&
		strings.Contains(string(body), "192.168.1.1"),
		"show bgp decode of the encoded route lost 10.0.0.0/24 via 192.168.1.1: %#v", payload)
}
