// Design: plan/spec-le-is-a-ze-binary.md -- native port of move_cmd_yang_to_plugins.py
package yangmigration

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"
)

type commandOwner struct {
	component string
	plugin    string
}

var commandOwners = []commandOwner{
	{"aaa", "aaa-cmd"},
	{"bfd", "bfd"},
	{"config/archive", "config-archive-cmd"},
	{"flowexport", "flowexport-cmd"},
	{"gnmi", "gnmi-cmd"},
	{"l2tp", "l2tp-cmd"},
	{"ldp", "ldp-cmd"},
	{"mpls", "mpls-cmd"},
	{"ping", "ping-cmd"},
	{"pki", "pki-cmd"},
	{"pppoe", "pppoe-cmd"},
	{"resolve", "resolve-cmd"},
	{"rsvpte", "rsvpte-cmd"},
	{"storage", "storage-cmd"},
	{"subscriber", "subscriber-cmd"},
	{"traceroute", "traceroute-cmd"},
	{"traffic", "traffic-cmd"},
}

var intrinsicCommandOwners = []string{"doctor", "firewall", "iface", "ike"}

var commandReferenceDirs = []string{
	"internal/component/cmd/show/yang",
	"internal/component/cmd/clear/yang",
	"internal/component/cmd/delete/yang",
	"internal/component/cmd/monitor/yang",
	"internal/component/cmd/set/yang",
}

// commandsToPlugins previews or applies command module ownership moves.
func commandsToPlugins(root string, apply bool) (Report, error) {
	report := Report{Workflow: WorkflowCommandsToPlugins, Apply: apply}
	for _, owner := range intrinsicCommandOwners {
		paths, err := commandFiles(filepath.Join(root, "internal", "component", filepath.FromSlash(owner), yangDir), false)
		if err != nil {
			return report, err
		}
		for _, path := range paths {
			report.Skipped = append(report.Skipped, relative(root, path))
		}
	}
	for _, owner := range commandOwners {
		sourceDir := filepath.Join(root, "internal", "component", filepath.FromSlash(owner.component), yangDir)
		destinationDir := filepath.Join(root, "internal", "plugins", filepath.FromSlash(owner.plugin), yangDir)
		yangFiles, err := commandFiles(sourceDir, false)
		if err != nil {
			return report, err
		}
		tests, err := commandFiles(sourceDir, true)
		if err != nil {
			return report, err
		}
		if len(yangFiles) == 0 {
			report.Skipped = append(report.Skipped, "internal/component/"+owner.component+"/yang")
			continue
		}
		for _, path := range yangFiles {
			content, _, err := readRegular(path)
			if err != nil {
				appendRefusal(&report, root, path, err)
				continue
			}
			if _, err := gyang.Parse(string(content), path); err != nil {
				appendRefusal(&report, root, path, fmt.Errorf("parse YANG: %w", err))
				continue
			}
			planMove(root, path, filepath.Join(destinationDir, filepath.Base(path)), &report)
		}
		for _, path := range tests {
			planMove(root, path, filepath.Join(destinationDir, filepath.Base(path)), &report)
		}
	}
	for _, directory := range commandReferenceDirs {
		paths, err := walkFiles(filepath.Join(root, filepath.FromSlash(directory)), func(path string) bool {
			return strings.HasSuffix(path, "_test.go")
		})
		if err != nil {
			return report, err
		}
		for _, path := range paths {
			before, _, err := readRegular(path)
			if err != nil {
				appendRefusal(&report, root, path, err)
				continue
			}
			after, err := transformGoStrings(path, before, func(value string) string {
				for _, owner := range commandOwners {
					oldPath := "internal/component/" + owner.component + "/yang"
					newPath := "internal/plugins/" + owner.plugin + "/yang"
					value = strings.ReplaceAll(value, oldPath, newPath)
				}
				return value
			})
			if err != nil {
				appendRefusal(&report, root, path, err)
				continue
			}
			planEdit(root, path, before, after, &report)
		}
	}
	sort.Slice(report.Moves, func(i, j int) bool { return report.Moves[i].Source < report.Moves[j].Source })
	sort.Slice(report.Edits, func(i, j int) bool { return report.Edits[i].Path < report.Edits[j].Path })
	sort.Strings(report.Skipped)
	if err := applyReport(root, &report); err != nil {
		return report, err
	}
	return report, nil
}

func commandFiles(directory string, tests bool) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		name := entry.Name()
		if tests {
			if filepath.Ext(name) == goSuffix && strings.Contains(name, "cmd_schema") {
				paths = append(paths, filepath.Join(directory, name))
			}
			continue
		}
		if filepath.Ext(name) == yangSuffix && strings.Contains(strings.TrimSuffix(name, yangSuffix), "-cmd") {
			paths = append(paths, filepath.Join(directory, name))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func transformGoStrings(path string, content []byte, transform func(string) string) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse Go: %w", err)
	}
	var replacements []textReplacement
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			return true
		}
		changed := transform(value)
		if changed != value {
			replacements = append(replacements, nodeReplacement(fset, literal, quoteLike(literal.Value, changed)))
		}
		return true
	})
	return applyTextReplacements(content, replacements)
}
