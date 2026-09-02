// Design: plan/spec-le-is-a-ze-binary.md -- native port of internal/le/yang/migration/actions.go
package yangmigration

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// PathOperationKind selects the structural path edit.
type PathOperationKind string

const (
	PathRemove PathOperationKind = "remove"
	PathRename PathOperationKind = "rename"
	PathMove   PathOperationKind = "move"
)

// pathOperation describes one repository-wide YANG path refactor.
type pathOperation struct {
	Kind        PathOperationKind
	Target      string
	Replacement string
	Under       string
	Source      string
	Destination string
	ListNodes   map[string]bool
}

func (op pathOperation) Validate() error {
	switch op.Kind {
	case PathRemove:
		if !validSegment(op.Target) || len(splitPath(op.Under)) == 0 {
			return fmt.Errorf("remove requires target segment and under path")
		}
		if op.Replacement != "" || op.Source != "" || op.Destination != "" {
			return fmt.Errorf("remove does not accept replacement, source, or destination")
		}
	case PathRename:
		if !validSegment(op.Target) || !validSegment(op.Replacement) || len(splitPath(op.Under)) == 0 {
			return fmt.Errorf("rename requires target, replacement, and under path")
		}
		if op.Target == op.Replacement {
			return fmt.Errorf("rename target and replacement are identical")
		}
		if op.Source != "" || op.Destination != "" {
			return fmt.Errorf("rename does not accept source or destination")
		}
	case PathMove:
		if len(splitPath(op.Source)) == 0 || len(splitPath(op.Destination)) == 0 {
			return fmt.Errorf("move requires source and destination paths")
		}
		if op.Source == op.Destination {
			return fmt.Errorf("move source and destination are identical")
		}
		if op.Target != "" || op.Replacement != "" || op.Under != "" {
			return fmt.Errorf("move does not accept target, replacement, or under")
		}
	default:
		return fmt.Errorf("unknown operation %q", op.Kind)
	}
	for name := range op.ListNodes {
		if !validSegment(name) {
			return fmt.Errorf("invalid list node %q", name)
		}
	}
	return nil
}

func defaultListNodes() map[string]bool {
	result := make(map[string]bool)
	for _, name := range []string{"peer", "group", "family", "update", "route", "external", "profile", "user", "server", "tunnel", "bridge", "vlan", "macvlan", "veth", "wireguard", "vxlan"} {
		result[name] = true
	}
	return result
}

func validSegment(segment string) bool {
	if segment == "" {
		return false
	}
	for _, r := range segment {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' && r != ':' {
			return false
		}
	}
	return true
}

func splitPath(path string) []string {
	if path == "" {
		return []string{}
	}
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if !validSegment(part) {
			return nil
		}
	}
	return parts
}

// refactorPaths previews or applies an operation across YANG, Go, .ci, and .et files.
func refactorPaths(root string, op pathOperation, apply bool) (Report, error) {
	report := Report{Workflow: WorkflowPathRefactor, Apply: apply}
	if err := op.Validate(); err != nil {
		return report, err
	}
	categories := []struct {
		roots []string
		ext   string
		kind  string
	}{
		{[]string{"."}, yangSuffix, yangDir},
		{[]string{"internal", "cmd", "pkg"}, goSuffix, "go"},
		{[]string{testTree}, ".ci", "ci"},
		{[]string{testTree}, ".et", "et"},
	}
	seen := make(map[string]bool)
	for _, category := range categories {
		for _, sub := range category.roots {
			paths, err := walkFiles(filepath.Join(root, sub), func(path string) bool { return filepath.Ext(path) == category.ext })
			if err != nil {
				return report, err
			}
			for _, path := range paths {
				clean := filepath.Clean(path)
				if seen[clean] {
					continue
				}
				seen[clean] = true
				report.Scanned++
				before, _, err := readRegular(path)
				if err != nil {
					appendRefusal(&report, root, path, err)
					continue
				}
				after, manual, err := refactorFile(path, before, category.kind, op)
				if err != nil {
					appendRefusal(&report, root, path, err)
					continue
				}
				for index := range manual {
					manual[index].Path = relative(root, path)
				}
				report.Manual = append(report.Manual, manual...)
				planEdit(root, path, before, after, &report)
			}
		}
	}
	sort.Slice(report.Edits, func(i, j int) bool { return report.Edits[i].Path < report.Edits[j].Path })
	sort.Slice(report.Manual, func(i, j int) bool {
		if report.Manual[i].Path == report.Manual[j].Path {
			return report.Manual[i].Line < report.Manual[j].Line
		}
		return report.Manual[i].Path < report.Manual[j].Path
	})
	if err := applyReport(root, &report); err != nil {
		return report, err
	}
	return report, nil
}

func refactorFile(path string, before []byte, category string, op pathOperation) ([]byte, []ManualEdit, error) {
	switch category {
	case yangDir:
		statements, err := gyang.Parse(string(before), path)
		if err != nil {
			return nil, nil, fmt.Errorf("parse YANG: %w", err)
		}
		manual := findYangDefinitions(statements, op)
		after, err := transformQuotedText(before, func(value string) string { return transformSlashPathsInText(value, op) })
		return after, manual, err
	case "go":
		return refactorGo(path, before, op)
	case "ci", "et":
		after, err := transformQuotedText(before, func(value string) string { return transformSlashPathsInText(value, op) })
		if err != nil {
			return nil, nil, err
		}
		after = transformContextFlags(after, op)
		after = transformSetLines(after, op, category == "et")
		after, err = transformBraceBlocks(after, op)
		return after, nil, err
	default:
		return before, nil, nil
	}
}

func operationPaths(op pathOperation) (source, destination []string) {
	if op.Kind == PathMove {
		return splitPath(op.Source), splitPath(op.Destination)
	}
	return append(splitPath(op.Under), op.Target), append(splitPath(op.Under), op.Replacement)
}

func transformSlashPath(value string, op pathOperation) (string, bool) {
	segments := strings.Split(value, "/")
	for _, segment := range segments {
		if !validSegment(segment) {
			return value, false
		}
	}
	if op.Kind == PathMove {
		source, destination := operationPaths(op)
		end, keys, ok := matchConcretePrefix(segments, source, op.ListNodes)
		if !ok {
			return value, false
		}
		result := concreteDestination(destination, keys, op.ListNodes)
		result = append(result, segments[end:]...)
		return strings.Join(result, "/"), true
	}
	under := splitPath(op.Under)
	index, _, ok := matchConcretePrefix(segments, under, op.ListNodes)
	if !ok {
		if len(segments) == 0 || segments[0] != op.Target {
			return value, false
		}
		index = 0
	}
	if index >= len(segments) || segments[index] != op.Target {
		if segments[0] != op.Target {
			return value, false
		}
		index = 0
	}
	if op.Kind == PathRemove {
		segments = append(segments[:index], segments[index+1:]...)
	} else {
		segments[index] = op.Replacement
	}
	if len(segments) == 0 {
		return value, false
	}
	return strings.Join(segments, "/"), true
}

func matchConcretePrefix(path, structural []string, listNodes map[string]bool) (int, map[string]string, bool) {
	position := 0
	keys := make(map[string]string)
	for _, segment := range structural {
		if position >= len(path) || path[position] != segment {
			return 0, nil, false
		}
		position++
		if listNodes[segment] && position < len(path) {
			keys[segment] = path[position]
			position++
		}
	}
	return position, keys, true
}

func concreteDestination(destination []string, keys map[string]string, listNodes map[string]bool) []string {
	result := make([]string, 0, len(destination)+len(keys))
	for _, segment := range destination {
		result = append(result, segment)
		if listNodes[segment] && keys[segment] != "" {
			result = append(result, keys[segment])
		}
	}
	return result
}

func transformSlashPathsInText(text string, op pathOperation) string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("\"'`()[]{}<>,;=", r)
	})
	result := text
	for _, field := range fields {
		if !strings.Contains(field, "/") || strings.Contains(field, "://") || strings.Contains(field, ".") {
			continue
		}
		if changed, ok := transformSlashPath(field, op); ok {
			result = strings.ReplaceAll(result, field, changed)
		}
	}
	return result
}

func findYangDefinitions(statements []*gyang.Statement, op pathOperation) []ManualEdit {
	target := op.Target
	if op.Kind == PathMove {
		parts := splitPath(op.Source)
		target = parts[len(parts)-1]
	}
	var result []ManualEdit
	var visit func([]*gyang.Statement)
	visit = func(items []*gyang.Statement) {
		for _, statement := range items {
			switch statement.Keyword {
			case "container", "leaf", "leaf-list", "list", "grouping":
				if statement.Argument == target {
					location := statement.Location()
					line := 0
					if column := strings.LastIndexByte(location, ':'); column >= 0 {
						prefix := location[:column]
						if separator := strings.LastIndexByte(prefix, ':'); separator >= 0 {
							line, _ = strconv.Atoi(prefix[separator+1:])
						} else {
							line, _ = strconv.Atoi(strings.TrimPrefix(prefix, "line "))
						}
					}
					result = append(result, ManualEdit{Line: line, Text: statement.Keyword + " " + statement.Argument, Reason: "YANG " + statement.Keyword + " definition requires structural editing"})
				}
			}
			visit(statement.SubStatements())
		}
	}
	visit(statements)
	return result
}

func refactorGo(path string, before []byte, op pathOperation) ([]byte, []ManualEdit, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, before, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parse Go: %w", err)
	}
	parents := make(map[ast.Node]ast.Node)
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			return true
		}
		ast.Inspect(node, func(child ast.Node) bool {
			if child != nil && child != node {
				if _, exists := parents[child]; !exists {
					parents[child] = node
				}
				return false
			}
			return true
		})
		return true
	})
	var replacements []textReplacement
	var manual []ManualEdit
	ast.Inspect(file, func(node ast.Node) bool {
		switch current := node.(type) {
		case *ast.BasicLit:
			if current.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(current.Value)
			if err != nil {
				return true
			}
			changed := transformSlashPathsInText(value, op)
			changed = transformSetText(changed, op)
			if changed != value {
				replacements = append(replacements, nodeReplacement(fset, current, quoteLike(current.Value, changed)))
			}
		case *ast.CallExpr:
			selector, ok := current.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "GetContainer" || len(current.Args) != 1 {
				return true
			}
			literal, ok := current.Args[0].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			name, err := strconv.Unquote(literal.Value)
			if err != nil || (op.Kind != PathRemove && op.Kind != PathRename) || name != op.Target {
				return true
			}
			if op.Kind == PathRename {
				replacements = append(replacements, nodeReplacement(fset, literal, strconv.Quote(op.Replacement)))
				return true
			}
			if isGetChainReceiver(current, parents[current]) {
				start := fset.Position(selector.X.End()).Offset
				end := fset.Position(current.End()).Offset
				replacements = append(replacements, textReplacement{start: start, end: end, text: ""})
			} else {
				position := fset.Position(current.Pos())
				manual = append(manual, ManualEdit{Line: position.Line, Text: string(before[fset.Position(current.Pos()).Offset:fset.Position(current.End()).Offset]), Reason: "terminal GetContainer call requires structural editing"})
			}
		}
		return true
	})
	after, err := applyTextReplacements(before, replacements)
	return after, manual, err
}

func isGetChainReceiver(call *ast.CallExpr, parent ast.Node) bool {
	selector, ok := parent.(*ast.SelectorExpr)
	if !ok || selector.X != call || !strings.HasPrefix(selector.Sel.Name, "Get") {
		return false
	}
	return true
}

func nodeReplacement(fset *token.FileSet, node ast.Node, text string) textReplacement {
	return textReplacement{start: fset.Position(node.Pos()).Offset, end: fset.Position(node.End()).Offset, text: text}
}

func quoteLike(original, value string) string {
	if strings.HasPrefix(original, "`") && !strings.Contains(value, "`") {
		return "`" + value + "`"
	}
	return strconv.Quote(value)
}

func transformSetText(text string, op pathOperation) string {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		position := len(line) - len(strings.TrimLeft(line, " \t"))
		if !strings.HasPrefix(line[position:], "set ") {
			continue
		}
		prefix := line[:position]
		command := line[position:]
		if changed, ok := transformSetCommand(command, op); ok {
			lines[index] = prefix + changed
		}
	}
	return strings.Join(lines, "\n")
}

func transformSetLines(content []byte, op pathOperation, et bool) []byte {
	lines := strings.Split(string(content), "\n")
	for index, line := range lines {
		position := len(line) - len(strings.TrimLeft(line, " \t"))
		if et {
			if marker := strings.Index(line, "input=type:text=set "); marker >= 0 {
				position = marker + len("input=type:text=")
			}
		}
		if position >= len(line) || !strings.HasPrefix(line[position:], "set ") {
			continue
		}
		if changed, ok := transformSetCommand(line[position:], op); ok {
			lines[index] = line[:position] + changed
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

func transformSetCommand(command string, op pathOperation) (string, bool) {
	if op.Kind == PathMove {
		return command, false
	}
	parts := strings.Fields(command)
	if len(parts) < 2 || parts[0] != "set" {
		return command, false
	}
	under := splitPath(op.Under)
	structural := make([]string, 0, len(parts))
	for index := 1; index < len(parts); index++ {
		segment := parts[index]
		if len(structural) >= len(under) && equalStrings(structural[:len(under)], under) && segment == op.Target {
			if op.Kind == PathRemove {
				parts = append(parts[:index], parts[index+1:]...)
			} else {
				parts[index] = op.Replacement
			}
			return strings.Join(parts, " "), true
		}
		structural = append(structural, segment)
		if op.ListNodes[segment] && index+1 < len(parts) {
			index++
		}
	}
	return command, false
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func transformContextFlags(content []byte, op pathOperation) []byte {
	text := string(content)
	for offset := 0; ; {
		index := strings.Index(text[offset:], "--context")
		if index < 0 {
			break
		}
		index += offset
		start := index + len("--context")
		for start < len(text) && (text[start] == ' ' || text[start] == '\t') {
			start++
		}
		end := start
		for end < len(text) && !unicode.IsSpace(rune(text[end])) {
			end++
		}
		if changed, ok := transformSlashPath(text[start:end], op); ok {
			text = text[:start] + changed + text[end:]
			offset = start + len(changed)
		} else {
			offset = end
		}
	}
	return []byte(text)
}

func transformBraceBlocks(content []byte, op pathOperation) ([]byte, error) {
	if op.Kind == PathMove {
		return content, nil
	}
	lines := strings.Split(string(content), "\n")
	type block struct {
		name  string
		start int
		end   int
		stack []string
	}
	var stack []block
	var blocks []block
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "}" {
			if len(stack) == 0 {
				continue
			}
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			current.end = index
			blocks = append(blocks, current)
			continue
		}
		if !strings.HasSuffix(trimmed, "{") || strings.ContainsAny(trimmed, "\"'`$()") {
			continue
		}
		words := strings.Fields(strings.TrimSuffix(trimmed, "{"))
		if len(words) == 0 {
			continue
		}
		context := make([]string, 0, len(stack)*2)
		for _, parent := range stack {
			context = append(context, parent.name)
		}
		stack = append(stack, block{name: words[0], start: index, end: -1, stack: context})
	}
	for _, current := range stack {
		if current.name == op.Target && contextMatches(current.stack, splitPath(op.Under), op.ListNodes) {
			return nil, fmt.Errorf("unclosed %s block at line %d", op.Target, current.start+1)
		}
	}
	var targets []block
	for _, current := range blocks {
		if current.name == op.Target && contextMatches(current.stack, splitPath(op.Under), op.ListNodes) {
			targets = append(targets, current)
		}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].start > targets[j].start })
	for _, target := range targets {
		if op.Kind == PathRename {
			indent := lines[target.start][:len(lines[target.start])-len(strings.TrimLeft(lines[target.start], " \t"))]
			rest := strings.TrimSpace(lines[target.start])
			lines[target.start] = indent + op.Replacement + strings.TrimPrefix(rest, op.Target)
			continue
		}
		indent := lines[target.start][:len(lines[target.start])-len(strings.TrimLeft(lines[target.start], " \t"))]
		childIndent := ""
		for index := target.start + 1; index < target.end; index++ {
			if strings.TrimSpace(lines[index]) != "" {
				childIndent = lines[index][:len(lines[index])-len(strings.TrimLeft(lines[index], " \t"))]
				break
			}
		}
		children := append([]string(nil), lines[target.start+1:target.end]...)
		for index, line := range children {
			if childIndent != "" && strings.HasPrefix(line, childIndent) {
				children[index] = indent + strings.TrimPrefix(line, childIndent)
			}
		}
		lines = append(lines[:target.start], append(children, lines[target.end+1:]...)...)
	}
	return []byte(strings.Join(lines, "\n")), nil
}

func contextMatches(stack, under []string, listNodes map[string]bool) bool {
	if len(under) == 0 {
		return true
	}
	position := 0
	for _, name := range stack {
		if position < len(under) && name == under[position] {
			position++
			if position == len(under) {
				return true
			}
		}
	}
	_ = listNodes
	return false
}

type textReplacement struct {
	start int
	end   int
	text  string
}

func applyTextReplacements(content []byte, replacements []textReplacement) ([]byte, error) {
	sort.Slice(replacements, func(i, j int) bool {
		if replacements[i].start == replacements[j].start {
			return replacements[i].end > replacements[j].end
		}
		return replacements[i].start > replacements[j].start
	})
	result := append([]byte(nil), content...)
	lastStart := len(result)
	var previous *textReplacement
	for _, replacement := range replacements {
		if replacement.start < 0 || replacement.end < replacement.start || replacement.end > len(content) {
			return nil, fmt.Errorf("invalid syntax replacement span")
		}
		if replacement.end > lastStart {
			if previous != nil && replacement.start == previous.start && replacement.end == previous.end && replacement.text == previous.text {
				continue
			}
			return nil, fmt.Errorf("overlapping syntax replacement spans")
		}
		result = append(result[:replacement.start], append([]byte(replacement.text), result[replacement.end:]...)...)
		lastStart = replacement.start
		previous = &replacement
	}
	return result, nil
}

func transformQuotedText(content []byte, transform func(string) string) ([]byte, error) {
	var replacements []textReplacement
	for index := 0; index < len(content); {
		if index+1 < len(content) && content[index] == '/' && content[index+1] == '/' {
			if newline := bytes.IndexByte(content[index+2:], '\n'); newline >= 0 {
				index += newline + 3
			} else {
				break
			}
			continue
		}
		if index+1 < len(content) && content[index] == '/' && content[index+1] == '*' {
			close := bytes.Index(content[index+2:], []byte("*/"))
			if close < 0 {
				return nil, fmt.Errorf("unclosed block comment at byte %d", index)
			}
			index += close + 4
			continue
		}
		if content[index] == '#' && hashStartsComment(content, index) {
			if newline := bytes.IndexByte(content[index+1:], '\n'); newline >= 0 {
				index += newline + 2
			} else {
				break
			}
			continue
		}
		quote := content[index]
		if quote != '\'' && quote != '"' {
			index++
			continue
		}
		start := index
		index++
		var value textbuf.Buffer
		closed := false
		for index < len(content) {
			if content[index] == '\\' && index+1 < len(content) {
				value.Write(content[index : index+2])
				index += 2
				continue
			}
			if content[index] == quote {
				closed = true
				break
			}
			value.Byte(content[index])
			index++
		}
		if !closed {
			return nil, fmt.Errorf("unclosed quoted string at byte %d", start)
		}
		original := value.String()
		changed := transform(original)
		if changed != original {
			replacements = append(replacements, textReplacement{start: start + 1, end: index, text: changed})
		}
		index++
	}
	return applyTextReplacements(content, replacements)
}

func hashStartsComment(content []byte, index int) bool {
	return index == 0 || content[index-1] == ' ' || content[index-1] == '\t' || content[index-1] == '\n'
}
