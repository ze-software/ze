// Design: api/proto/ze.proto -- explicit json_name options govern generated Go tags.
package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

var protoFieldWithJSONName = regexp.MustCompile(`(?m)^\s*(?:repeated\s+)?[A-Za-z_][A-Za-z0-9_.]*\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*\d+\s*\[\s*json_name\s*=\s*"([^"]+)"\s*\]\s*;\s*$`)

type protoJSONDeclaration struct {
	field string
	name  string
}

// protoJSONTags is one injected in-place rewrite of generated protobuf Go.
type protoJSONTags struct {
	ProtoPath     string
	GeneratedPath string
	ReadFile      func(string) ([]byte, error)
	WriteFile     func(string, []byte, os.FileMode) error
}

// run applies explicit proto json_name options and reports whether the file changed.
func (p *protoJSONTags) run() (bool, error) {
	readFile := p.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	writeFile := p.WriteFile
	if writeFile == nil {
		writeFile = os.WriteFile
	}
	proto, err := readFile(p.ProtoPath)
	if err != nil {
		return false, fmt.Errorf("read proto source %s: %w", p.ProtoPath, err)
	}
	generated, err := readFile(p.GeneratedPath)
	if err != nil {
		return false, fmt.Errorf("read generated Go %s: %w", p.GeneratedPath, err)
	}
	rewritten, err := rewriteProtoJSONTags(string(proto), string(generated))
	if err != nil {
		return false, err
	}
	if rewritten == string(generated) {
		return false, nil
	}
	if err := writeFile(p.GeneratedPath, []byte(rewritten), 0o644); err != nil {
		return false, fmt.Errorf("write generated Go %s: %w", p.GeneratedPath, err)
	}
	return true, nil
}

// rewriteProtoJSONTags applies every explicit proto json_name to generated Go.
func rewriteProtoJSONTags(proto, generated string) (string, error) {
	matches := protoFieldWithJSONName.FindAllStringSubmatch(proto, -1)
	if len(matches) == 0 {
		return "", errors.New("proto source has no explicit json_name options")
	}

	counts := make(map[protoJSONDeclaration]int, len(matches))
	names := make(map[string]string, len(matches))
	for _, match := range matches {
		declaration := protoJSONDeclaration{field: match[1], name: match[2]}
		if prior, found := names[declaration.field]; found && prior != declaration.name {
			return "", fmt.Errorf(
				"proto field %q has conflicting json_name options: %q and %q",
				declaration.field, prior, declaration.name,
			)
		}
		names[declaration.field] = declaration.name
		counts[declaration]++
	}

	declarations := make([]protoJSONDeclaration, 0, len(counts))
	for declaration := range counts {
		declarations = append(declarations, declaration)
	}
	sort.Slice(declarations, func(left, right int) bool {
		if declarations[left].field == declarations[right].field {
			return declarations[left].name < declarations[right].name
		}
		return declarations[left].field < declarations[right].field
	})

	rewritten := generated
	for _, declaration := range declarations {
		expected := counts[declaration]
		source := `json:"` + declaration.field + `,omitempty"`
		replacement := `json:"` + declaration.name + `,omitempty"`
		foundSource := strings.Count(rewritten, source)
		if source == replacement {
			if foundSource != expected {
				return "", fmt.Errorf(
					"generated Go has %d %q tags, want %d from the proto declarations",
					foundSource, source, expected,
				)
			}
			continue
		}
		foundReplacement := strings.Count(rewritten, replacement)
		if foundSource+foundReplacement != expected {
			return "", fmt.Errorf(
				"generated Go has %d %q tags and %d %q tags, want %d total from the proto declarations",
				foundSource, source, foundReplacement, replacement, expected,
			)
		}
		rewritten = strings.ReplaceAll(rewritten, source, replacement)
	}
	return rewritten, nil
}

func runProtoJSONTags() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	rewriter := &protoJSONTags{
		ProtoPath:     filepath.Join(root, "api", "proto", "ze.proto"),
		GeneratedPath: filepath.Join(root, "api", "proto", "ze.pb.go"),
	}
	if _, err := rewriter.run(); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return nil, 0
}
