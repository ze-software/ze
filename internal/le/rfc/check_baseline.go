// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Related: check.go -- the centralized check driver that consumes these HEAD snapshots
//
// check_baseline.go reads the committed side of every monotonic comparison.
// A baseline that cannot be read accuses nobody. Optional maps preserve the
// cases where an empty committed set and an unreadable commit have opposite
// meanings to a current-minus-baseline comparison.
package rfc

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

type baselineExtraction struct {
	excluded     int
	signedOff    string
	resignReason string
}

func baselineParseError(message string) error { return &ParseError{msg: message} }

func gitOutput(tree string, args ...string) ([]byte, bool) {
	cmd := exec.Command("git", args...) //nolint:gosec,noctx // this developer tool queries the fixture checkout it was given
	cmd.Dir = tree
	out, err := cmd.Output()
	return out, err == nil
}

func gitCatBlobs(tree string, paths []string) map[string]string {
	if len(paths) == 0 {
		return map[string]string{}
	}
	var input bytes.Buffer
	for _, rel := range paths {
		input.WriteString("HEAD:")
		input.WriteString(rel)
		input.WriteByte('\n')
	}
	cmd := exec.Command("git", "cat-file", "--batch") //nolint:gosec,noctx // this developer tool queries the fixture checkout it was given
	cmd.Dir = tree
	cmd.Stdin = &input
	data, err := cmd.Output()
	if err != nil {
		return map[string]string{}
	}
	out := map[string]string{}
	position := 0
	for _, rel := range paths {
		newline := bytes.IndexByte(data[position:], '\n')
		if newline < 0 {
			return map[string]string{}
		}
		newline += position
		header := strings.Fields(string(data[position:newline]))
		position = newline + 1
		if len(header) == 2 && (header[1] == "missing" || header[1] == "ambiguous") {
			continue
		}
		if len(header) != 3 {
			return map[string]string{}
		}
		size, err := strconv.Atoi(header[2])
		if err != nil || position+size >= len(data) {
			return map[string]string{}
		}
		body := data[position : position+size]
		position += size + 1
		if header[1] == "blob" {
			out[rel] = strings.ToValidUTF8(string(body), "�")
		}
	}
	return out
}

func gitTreePaths(tree, dir, suffix string) ([]string, bool) {
	raw, ok := gitOutput(tree, "ls-tree", "-r", "-z", "--name-only", "HEAD", dir)
	if !ok {
		return nil, false
	}
	var paths []string
	for rel := range strings.SplitSeq(string(raw), "\x00") {
		if strings.HasSuffix(rel, suffix) && !strings.Contains(rel, "\n") {
			paths = append(paths, rel)
		}
	}
	return paths, true
}

func baselineEnrolled(tree string) (map[string]bool, bool) {
	var tb textbuf.Buffer
	raw, ok := gitOutput(tree, "show", tb.Str("HEAD:").Str(enrolledRel).Slice())
	if !ok {
		return nil, false
	}
	return parseEnrolled(string(raw)), true
}

func baselineDispositions(tree string) map[string]bool {
	blobs := gitCatBlobs(tree, []string{notEnrolledRel})
	text, held := blobs[notEnrolledRel]
	if !held {
		return map[string]bool{}
	}
	found, err := parseDispositions(text)
	if err != nil {
		return map[string]bool{}
	}
	out := map[string]bool{}
	for stem := range found {
		out[stem] = true
	}
	return out
}

func baselineStatusRows(tree string) (map[string]LedgerRow, bool) {
	text, held := gitCatBlobs(tree, []string{statusRel})[statusRel]
	if !held {
		return nil, false
	}
	return parseStatusLedger(text), true
}

func baselineSummaryStems(tree string) (map[string]bool, bool) {
	paths, ok := gitTreePaths(tree, summaryRel, ".md")
	if !ok {
		return nil, false
	}
	out := map[string]bool{}
	for _, rel := range paths {
		out[strings.TrimSuffix(filepath.Base(rel), ".md")] = true
	}
	return out, true
}

func baselineLevels(tree string) map[string]string {
	paths, ok := gitTreePaths(tree, summaryRel, ".md")
	if !ok {
		return map[string]string{}
	}
	blobs := gitCatBlobs(tree, paths)
	out := map[string]string{}
	for _, rel := range paths {
		text, held := blobs[rel]
		if !held {
			continue
		}
		stem := strings.TrimSuffix(filepath.Base(rel), ".md")
		requirements, err := parseSummaryText(text, stem, rel)
		if err != nil {
			continue
		}
		for _, req := range requirements {
			out[req.RID] = req.Level
		}
	}
	return out
}

func baselineIDs(levels map[string]string) map[string]bool {
	out := make(map[string]bool, len(levels))
	for rid := range levels {
		out[rid] = true
	}
	return out
}

func functionalSuitesFromGo(src, where string) ([]string, error) {
	var tb textbuf.Buffer
	file, err := parser.ParseFile(token.NewFileSet(), where, src, 0)
	if err != nil {
		return nil, baselineParseError(strings.NewReplacer("\n", " ").Replace(err.Error()))
	}
	constants := map[string]string{}
	for _, decl := range file.Decls {
		gen, held := decl.(*ast.GenDecl)
		if !held || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, held := spec.(*ast.ValueSpec)
			if !held {
				continue
			}
			for i, name := range value.Names {
				if i >= len(value.Values) {
					continue
				}
				literal, held := value.Values[i].(*ast.BasicLit)
				if !held || literal.Kind != token.STRING {
					continue
				}
				decoded, decodeErr := strconv.Unquote(literal.Value)
				if decodeErr == nil {
					constants[name.Name] = decoded
				}
			}
		}
	}
	for _, decl := range file.Decls {
		gen, held := decl.(*ast.GenDecl)
		if !held || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value, held := spec.(*ast.ValueSpec)
			if !held {
				continue
			}
			for i, name := range value.Names {
				if name.Name != "Gating" || i >= len(value.Values) {
					continue
				}
				list, held := value.Values[i].(*ast.CompositeLit)
				if !held {
					return nil, baselineParseError(tb.Reset().Str(where).Str(": Gating is not a composite literal").String())
				}
				out := make([]string, 0, len(list.Elts))
				for _, element := range list.Elts {
					switch one := element.(type) {
					case *ast.Ident:
						name, known := constants[one.Name]
						if !known {
							return nil, baselineParseError(tb.Reset().Str(where).Str(": Gating references an unknown suite constant ").Str(one.Name).String())
						}
						out = append(out, name)
					case *ast.BasicLit:
						name, decodeErr := strconv.Unquote(one.Value)
						if decodeErr != nil {
							return nil, baselineParseError(tb.Reset().Str(where).Str(": Gating contains an invalid string").String())
						}
						out = append(out, name)
					default:
						return nil, baselineParseError(tb.Reset().Str(where).Str(": Gating contains a non-name expression").String())
					}
				}
				if len(out) == 0 {
					return nil, baselineParseError(tb.Reset().Str(where).Str(": Gating is empty").String())
				}
				return out, nil
			}
		}
	}
	return nil, baselineParseError(tb.Reset().Str(where).Str(": no Gating declaration found").String())
}

func headCarriers(tree string) ([]Carrier, error) {
	suites := FunctionalSuites()
	if raw, ok := gitOutput(tree, "show", "HEAD:internal/le/functional/suites.go"); ok {
		if found, err := functionalSuitesFromGo(string(raw), "HEAD:internal/le/functional/suites.go"); err == nil {
			suites = found
		}
	}
	scheduled, err := scheduledWorkflowActions(tree)
	if err != nil {
		return nil, err
	}
	paths, ok := gitTreePaths(tree, workflowsRel, ".yml")
	if ok {
		yamlPaths, yamlOK := gitTreePaths(tree, workflowsRel, ".yaml")
		if yamlOK {
			paths = append(paths, yamlPaths...)
		}
		sources := map[string]string{}
		for rel, text := range gitCatBlobs(tree, paths) {
			sources[filepath.Base(rel)] = text
		}
		if parsed := scheduledActionsFrom(sources); len(parsed) > 0 {
			scheduled = parsed
		}
	}
	carriers := carriersFor(suites, scheduled)
	return append(carriers, legacyInteropCarriers(scheduled)...), nil
}

func scanTagsTolerant(blob, rel string, carriers []Carrier) []Tag {
	carrier, held := carrierFor(rel, carriers)
	if !held {
		return nil
	}
	found, err := readerFor(carrier.Reader)(blob, rel)
	if err == nil {
		return found
	}
	if carrier.Reader != "go" {
		return nil
	}
	var out []Tag
	for lineIndex, line := range strings.Split(blob, "\n") {
		match := goTagRE.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		tag, parseErr := parseTagRest(match[1], tagWhere(rel, lineIndex+1))
		if parseErr != nil {
			continue
		}
		tag.File = rel
		tag.Line = lineIndex + 1
		out = append(out, tag)
	}
	return out
}

func baselineTags(tree string) []Tag {
	carriers, err := headCarriers(tree)
	if err != nil {
		return nil
	}
	args := append([]string{"grep", "-l", "-z", "-F", tagMarker, "HEAD", "--"}, testRoots[:]...)
	raw, ok := gitOutput(tree, args...)
	if !ok && len(raw) == 0 {
		return nil
	}
	var paths []string
	for entry := range strings.SplitSeq(string(raw), "\x00") {
		if !strings.HasPrefix(entry, "HEAD:") {
			continue
		}
		rel := strings.TrimPrefix(entry, "HEAD:")
		carrier, held := carrierFor(rel, carriers)
		if !held || carrier.Tier == tierUnrun || strings.Contains(rel, "\n") {
			continue
		}
		parts := strings.Split(rel, "/")
		skipped := false
		for _, part := range parts {
			if part == ".git" || part == "vendor" || part == "testdata" {
				skipped = true
			}
		}
		if !skipped {
			paths = append(paths, rel)
		}
	}
	var out []Tag
	for rel, blob := range gitCatBlobs(tree, paths) {
		out = append(out, scanTagsTolerant(blob, rel, carriers)...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

func baselinePolarities(tags []Tag) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, tag := range tags {
		if out[tag.RID] == nil {
			out[tag.RID] = map[string]bool{}
		}
		out[tag.RID][tag.Polarity] = true
	}
	return out
}

func nonunitEvidence(tags []Tag, carriers []Carrier) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, tag := range tags {
		carrier, held := carrierFor(tag.File, carriers)
		if !held || carrier.Kind == kindUnit {
			continue
		}
		if out[tag.RID] == nil {
			out[tag.RID] = map[string]bool{}
		}
		out[tag.RID][carrier.Label()] = true
	}
	return out
}

func baselineEvidence(tree string, tags []Tag) map[string]map[string]bool {
	carriers, err := headCarriers(tree)
	if err != nil {
		return map[string]map[string]bool{}
	}
	return nonunitEvidence(tags, carriers)
}

func baselineAudits(tree string) (map[string]map[string]map[string]any, bool) {
	paths, ok := gitTreePaths(tree, auditRel, ".json")
	if !ok {
		return nil, false
	}
	out := map[string]map[string]map[string]any{}
	for rel, blob := range gitCatBlobs(tree, paths) {
		var document map[string]any
		if json.Unmarshal([]byte(blob), &document) != nil {
			continue
		}
		raw, held := document["requirements"].(map[string]any)
		if !held {
			continue
		}
		stem := strings.TrimSuffix(filepath.Base(rel), ".json")
		out[stem] = map[string]map[string]any{}
		for rid, value := range raw {
			verdict, held := value.(map[string]any)
			if !held {
				continue
			}
			name, _ := verdict["verdict"].(string)
			if auditVerdicts[name] {
				out[stem][rid] = verdict
			}
		}
	}
	return out, true
}

func baselineExtractions(tree string) (map[string]baselineExtraction, bool) {
	paths, ok := gitTreePaths(tree, extractionRel, ".json")
	if !ok {
		return nil, false
	}
	out := map[string]baselineExtraction{}
	for rel, blob := range gitCatBlobs(tree, paths) {
		var document map[string]any
		if json.Unmarshal([]byte(blob), &document) != nil {
			continue
		}
		excluded := 0
		if sites, held := document[keySites].([]any); held {
			for _, value := range sites {
				site, held := value.(map[string]any)
				if held && site[keyDisposition] == dispositionExcluded {
					excluded++
				}
			}
		}
		signedOff, _ := document[keySignedOff].(string)
		resignReason, _ := document["resign-reason"].(string)
		stem := strings.TrimSuffix(filepath.Base(rel), ".json")
		out[stem] = baselineExtraction{excluded: excluded, signedOff: signedOff, resignReason: resignReason}
	}
	return out, true
}
