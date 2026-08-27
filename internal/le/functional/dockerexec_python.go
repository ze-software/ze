// Design: docs/architecture/testing/interop.md -- fail-open command helpers
// Overview: dockerexec.go -- the Docker exec ratchet and its structured reports
//
// dockerexec_python.go parses the Python syntax that the ratchet classifies.
package functional

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

type dockerExecToken struct {
	text  string
	line  int
	kind  byte
	empty bool
}

type dockerExecStatement struct {
	tokens []dockerExecToken
	indent int
}

type dockerExecFunction struct {
	name     string
	indent   int
	parent   *dockerExecFunction
	guards   []dockerExecGuard
	returned []string
}

type dockerExecGuard struct {
	name       string
	line       int
	ignoreLine bool
}

type dockerExecCall struct {
	file       string
	line       int
	function   string
	member     string
	parentKind byte
	boundName  string
	scope      *dockerExecFunction
	start      int
	end        int
}

type dockerExecParsed struct {
	functions []*dockerExecFunction
	calls     []dockerExecCall
	lines     []string
}

const (
	dockerExecIdent  byte = 'i'
	dockerExecString byte = 's'
	dockerExecNumber byte = 'n'
	dockerExecOther  byte = 'o'
)

const (
	dockerExecAssertKeyword = "assert"
	dockerExecDefKeyword    = "def"
	dockerExecElifKeyword   = "elif"
	dockerExecNotKeyword    = "not"
)

const (
	dockerExecParentOther byte = iota
	dockerExecParentExpr
	dockerExecParentReturn
	dockerExecParentAssign
)

func analyzeDockerExecSources(sources map[string]string) (DockerExecAnalysis, error) {
	paths := make([]string, 0, len(sources))
	for path := range sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	parsed := make(map[string]dockerExecParsed, len(paths))
	returned := make(map[string]map[string]bool)
	for _, path := range paths {
		one, err := parseDockerExecPython(path, sources[path])
		if err != nil {
			return DockerExecAnalysis{}, err
		}
		parsed[path] = one
		for _, function := range one.functions {
			if returned[function.name] == nil {
				returned[function.name] = make(map[string]bool)
			}
			for _, callee := range function.returned {
				returned[function.name][callee] = true
			}
		}
	}
	members := map[string]bool{dockerExecSeed: true}
	for {
		changed := false
		for name, callees := range returned {
			if members[name] {
				continue
			}
			for callee := range callees {
				if members[callee] {
					members[name], changed = true, true
					break
				}
			}
		}
		if !changed {
			break
		}
	}
	analysis := DockerExecAnalysis{FilesScanned: len(paths)}
	for member := range members {
		analysis.FailOpenFunctions = append(analysis.FailOpenFunctions, member)
	}
	sort.Strings(analysis.FailOpenFunctions)
	for _, path := range paths {
		one := parsed[path]
		for _, call := range one.calls {
			if !members[call.member] {
				continue
			}
			verdict := verdictDockerExecCall(call, one.lines)
			analysis.Sites = append(analysis.Sites, DockerExecSite{
				File: call.file, Line: call.line, Function: call.function,
				Member: call.member, Verdict: verdict,
			})
			countDockerExecVerdict(&analysis.Counts, verdict)
		}
	}
	slices.SortFunc(analysis.Sites, func(left, right DockerExecSite) int {
		if order := strings.Compare(left.File, right.File); order != 0 {
			return order
		}
		if left.Line != right.Line {
			return left.Line - right.Line
		}
		return strings.Compare(left.Member, right.Member)
	})
	return analysis, nil
}

func countDockerExecVerdict(counts *DockerExecCounts, verdict string) {
	switch verdict {
	case dockerExecVerdictCheck:
		counts.Checked++
	case dockerExecVerdictDrop:
		counts.Discarded++
	case dockerExecVerdictAllow:
		counts.Exempt++
	case dockerExecVerdictOpen:
		counts.Unchecked++
	}
}

func verdictDockerExecCall(call dockerExecCall, lines []string) string {
	switch call.parentKind {
	case dockerExecParentExpr:
		return dockerExecVerdictDrop
	case dockerExecParentReturn:
		return dockerExecVerdictCheck
	case dockerExecParentAssign:
		for _, guard := range call.scope.guards {
			if guard.name != call.boundName {
				continue
			}
			if guard.ignoreLine || guard.line >= call.line {
				return dockerExecVerdictCheck
			}
		}
	}
	if exemptDockerExecCall(lines, call.line) {
		return dockerExecVerdictAllow
	}
	return dockerExecVerdictOpen
}

func exemptDockerExecCall(lines []string, line int) bool {
	for _, candidate := range []struct {
		index         int
		allowTrailing bool
	}{
		{index: line - 1, allowTrailing: true},
		{index: line - 2, allowTrailing: false},
	} {
		if candidate.index < 0 {
			continue
		}
		if candidate.index >= len(lines) {
			continue
		}
		if dockerExecExemptionLine(lines[candidate.index], candidate.allowTrailing) {
			return true
		}
	}
	return false
}

func dockerExecExemptionLine(line string, allowTrailing bool) bool {
	for start := 0; start < len(line); {
		hash := strings.IndexByte(line[start:], '#')
		if hash < 0 {
			return false
		}
		hash += start
		if !allowTrailing {
			if strings.TrimSpace(line[:hash]) != "" {
				start = hash + 1
				continue
			}
		}
		rest := strings.TrimLeft(line[hash+1:], " \t\r\n\f\v")
		const marker = "fail-open-ok:"
		if strings.HasPrefix(rest, marker) {
			if strings.TrimSpace(rest[len(marker):]) != "" {
				return true
			}
		}
		start = hash + 1
	}
	return false
}

func parseDockerExecPython(path, source string) (dockerExecParsed, error) {
	statements, err := tokenizeDockerExecPython(source)
	if err != nil {
		return dockerExecParsed{}, fmt.Errorf("%s: cannot be parsed: %w", path, err)
	}
	root := &dockerExecFunction{name: "<module>", indent: -1}
	stack := []*dockerExecFunction{root}
	parsed := dockerExecParsed{lines: strings.Split(source, "\n")}
	for _, logical := range statements {
		for _, statement := range splitDockerExecStatements(logical) {
			for len(stack) > 1 && statement.indent <= stack[len(stack)-1].indent {
				stack = stack[:len(stack)-1]
			}
			current := stack[len(stack)-1]
			function, body, err := dockerExecDefinition(statement, current)
			if err != nil {
				return dockerExecParsed{}, fmt.Errorf("%s: cannot be parsed: %w", path, err)
			}
			if function != nil {
				parsed.functions = append(parsed.functions, function)
				stack = append(stack, function)
				current = function
			}
			header := statement.tokens
			if len(body) != 0 {
				header = header[:topDockerExecIndex(header, ":")+1]
			}
			recordDockerExecStatement(path, header, current, &parsed)
			if len(body) != 0 {
				recordDockerExecStatement(path, body, current, &parsed)
			}
		}
	}
	return parsed, nil
}

func dockerExecDefinition(statement dockerExecStatement, parent *dockerExecFunction) (*dockerExecFunction, []dockerExecToken, error) {
	tokens := statement.tokens
	defAt := 0
	if len(tokens) > 0 && tokens[0].text == "async" {
		defAt = 1
	}
	if defAt >= len(tokens) || tokens[defAt].text != dockerExecDefKeyword {
		return nil, inlineDockerExecBody(tokens), nil
	}
	if defAt+2 >= len(tokens) {
		return nil, nil, errors.New("malformed function definition")
	}
	if tokens[defAt+1].kind != dockerExecIdent || tokens[defAt+2].text != "(" {
		return nil, nil, errors.New("malformed function definition")
	}
	colon := topDockerExecIndex(tokens, ":")
	if colon < 0 {
		return nil, nil, errors.New("function definition has no colon")
	}
	function := &dockerExecFunction{name: tokens[defAt+1].text, indent: statement.indent, parent: parent}
	return function, tokens[colon+1:], nil
}

func inlineDockerExecBody(tokens []dockerExecToken) []dockerExecToken {
	if len(tokens) == 0 {
		return nil
	}
	first := tokens[0].text
	if first == "async" && len(tokens) > 1 {
		first = tokens[1].text
	}
	switch first {
	case "if", dockerExecElifKeyword, "else", "while", "for", "with", "try", "except", "finally", "class", "match", "case":
		colon := topDockerExecIndex(tokens, ":")
		if colon >= 0 {
			return tokens[colon+1:]
		}
	}
	return nil
}

func splitDockerExecStatements(statement dockerExecStatement) []dockerExecStatement {
	parts := splitDockerExecTop(statement.tokens, ";")
	out := make([]dockerExecStatement, 0, len(parts))
	for _, part := range parts {
		if len(part) != 0 {
			out = append(out, dockerExecStatement{tokens: part, indent: statement.indent})
		}
	}
	return out
}

func recordDockerExecStatement(path string, tokens []dockerExecToken, scope *dockerExecFunction, parsed *dockerExecParsed) {
	if len(tokens) == 0 {
		return
	}
	for _, guard := range dockerExecGuards(tokens) {
		for owner := scope; owner != nil; owner = owner.parent {
			owner.guards = append(owner.guards, guard)
		}
	}
	calls := callsDockerExec(tokens)
	for index := range calls {
		calls[index].file, calls[index].function, calls[index].scope = path, scope.name, scope
		calls[index].parentKind, calls[index].boundName = parentDockerExecCall(tokens, calls[index])
	}
	parsed.calls = append(parsed.calls, calls...)
	for _, call := range calls {
		if call.parentKind != dockerExecParentReturn {
			continue
		}
		for owner := scope; owner != nil; owner = owner.parent {
			if owner.name != "<module>" {
				owner.returned = append(owner.returned, call.member)
			}
		}
	}
}

func parentDockerExecCall(tokens []dockerExecToken, call dockerExecCall) (byte, string) {
	start, end := trimDockerExecRange(tokens, 0, len(tokens))
	if call.start == start && call.end == end {
		return dockerExecParentExpr, ""
	}
	if start < end && tokens[start].text == "return" {
		valueStart, valueEnd := trimDockerExecRange(tokens, start+1, end)
		if call.start == valueStart && call.end == valueEnd {
			return dockerExecParentReturn, ""
		}
	}
	equals := topDockerExecIndices(tokens[start:end], "=")
	if len(equals) != 1 {
		return dockerExecParentOther, ""
	}
	equal := start + equals[0]
	leftStart, leftEnd := trimDockerExecRange(tokens, start, equal)
	rightStart, rightEnd := trimDockerExecRange(tokens, equal+1, end)
	if leftEnd-leftStart != 1 || tokens[leftStart].kind != dockerExecIdent {
		return dockerExecParentOther, ""
	}
	if call.start == rightStart && call.end == rightEnd {
		return dockerExecParentAssign, tokens[leftStart].text
	}
	return dockerExecParentOther, ""
}

func callsDockerExec(tokens []dockerExecToken) []dockerExecCall {
	pairs := pairDockerExecTokens(tokens)
	var calls []dockerExecCall
	for index := 0; index+1 < len(tokens); index++ {
		if tokens[index].kind != dockerExecIdent || tokens[index+1].text != "(" {
			continue
		}
		if dockerExecCallKeyword(tokens[index].text) {
			continue
		}
		if index > 0 && tokens[index-1].text == dockerExecDefKeyword {
			continue
		}
		end, exists := pairs[index+1]
		if !exists {
			continue
		}
		start := dockerExecCallStart(tokens, pairs, index)
		calls = append(calls, dockerExecCall{
			line: tokens[start].line, member: tokens[index].text, start: start, end: end + 1,
		})
	}
	return calls
}

func dockerExecCallStart(tokens []dockerExecToken, pairs map[int]int, member int) int {
	start := member
	for start >= 2 && tokens[start-1].text == "." {
		start -= 2
		if tokens[start].text == ")" || tokens[start].text == "]" {
			start = pairs[start]
			if start > 0 && tokens[start].text == "(" && tokens[start-1].kind == dockerExecIdent {
				start--
			}
		}
	}
	return start
}

func dockerExecCallKeyword(word string) bool {
	switch word {
	case "if", dockerExecElifKeyword, "while", "for", "with", "except", "class", dockerExecDefKeyword, "return", dockerExecAssertKeyword, "lambda", "match", "case":
		return true
	}
	return false
}

func dockerExecGuards(tokens []dockerExecToken) []dockerExecGuard {
	var guards []dockerExecGuard
	start := -1
	if len(tokens) != 0 {
		switch tokens[0].text {
		case "if", dockerExecElifKeyword, "while", dockerExecAssertKeyword:
			start = 1
		}
	}
	if start >= 0 {
		end := len(tokens)
		if colon := topDockerExecIndex(tokens, ":"); colon >= start {
			end = colon
		}
		if tokens[0].text == dockerExecAssertKeyword {
			if comma := topDockerExecIndex(tokens[start:end], ","); comma >= 0 {
				end = start + comma
			}
		}
		guards = append(guards, guardsDockerExecExpr(tokens[start:end], false)...)
	}
	depth := 0
	for index, token := range tokens {
		depth += dockerExecDepthDelta(token.text)
		if token.text != "if" || index == 0 {
			continue
		}
		end := dockerExecConditionalEnd(tokens, index+1, depth)
		guards = append(guards, guardsDockerExecExpr(tokens[index+1:end], hasDockerExecFor(tokens, index, depth))...)
	}
	return guards
}

func guardsDockerExecExpr(tokens []dockerExecToken, ignoreLine bool) []dockerExecGuard {
	var guards []dockerExecGuard
	for _, operand := range dockerExecTruthOperands(tokens) {
		name := dockerExecTruthName(operand)
		if name == "" {
			continue
		}
		line := 0
		if len(operand) != 0 {
			line = operand[0].line
		}
		guards = append(guards, dockerExecGuard{name: name, line: line, ignoreLine: ignoreLine})
	}
	return guards
}

func dockerExecTruthOperands(tokens []dockerExecToken) [][]dockerExecToken {
	tokens = trimDockerExecParens(tokens)
	for len(tokens) != 0 && tokens[0].text == dockerExecNotKeyword {
		tokens = trimDockerExecParens(tokens[1:])
	}
	parts := splitDockerExecTopWords(tokens, "and", "or")
	if len(parts) == 1 {
		return parts
	}
	var out [][]dockerExecToken
	for _, part := range parts {
		out = append(out, dockerExecTruthOperands(part)...)
	}
	return out
}

func dockerExecTruthName(tokens []dockerExecToken) string {
	tokens = trimDockerExecParens(tokens)
	if name := dockerExecTruthRoot(tokens); name != "" {
		return name
	}
	left, operator, right, count := dockerExecComparison(tokens)
	if count != 1 {
		return ""
	}
	if name := dockerExecLenName(left); name != "" {
		return name
	}
	if name := dockerExecLenName(right); name != "" {
		return name
	}
	if operator != "==" && operator != "!=" && operator != "is" && operator != "is not" {
		return ""
	}
	if name := dockerExecBareName(left); name != "" && dockerExecEmpty(right) {
		return name
	}
	if name := dockerExecBareName(right); name != "" && dockerExecEmpty(left) {
		return name
	}
	return ""
}

func dockerExecTruthRoot(tokens []dockerExecToken) string {
	if len(tokens) == 0 || tokens[0].kind != dockerExecIdent {
		return ""
	}
	pairs := pairDockerExecTokens(tokens)
	for index := 1; index < len(tokens); {
		if tokens[index].text == "." {
			if index+1 >= len(tokens) || tokens[index+1].kind != dockerExecIdent {
				return ""
			}
			index += 2
			continue
		}
		if tokens[index].text == "(" {
			end, exists := pairs[index]
			if !exists {
				return ""
			}
			index = end + 1
			continue
		}
		return ""
	}
	return tokens[0].text
}

func dockerExecComparison(tokens []dockerExecToken) (left []dockerExecToken, operator string, right []dockerExecToken, count int) {
	depth := 0
	for index := 0; index < len(tokens); index++ {
		depth += dockerExecDepthDelta(tokens[index].text)
		if depth != 0 {
			continue
		}
		op, width := dockerExecComparisonAt(tokens, index)
		if op == "" {
			continue
		}
		count++
		if count == 1 {
			left, operator, right = tokens[:index], op, tokens[index+width:]
		}
		index += width - 1
	}
	return left, operator, right, count
}

func dockerExecComparisonAt(tokens []dockerExecToken, index int) (string, int) {
	word := tokens[index].text
	if word == "is" && index+1 < len(tokens) && tokens[index+1].text == dockerExecNotKeyword {
		return "is not", 2
	}
	if word == dockerExecNotKeyword && index+1 < len(tokens) && tokens[index+1].text == "in" {
		return "not in", 2
	}
	switch word {
	case "==", "!=", "<", ">", "<=", ">=", "in", "is":
		return word, 1
	}
	return "", 0
}

func dockerExecLenName(tokens []dockerExecToken) string {
	tokens = trimDockerExecParens(tokens)
	if len(tokens) == 4 && tokens[0].text == "len" && tokens[1].text == "(" && tokens[3].text == ")" {
		return dockerExecBareName(tokens[2:3])
	}
	return ""
}

func dockerExecBareName(tokens []dockerExecToken) string {
	tokens = trimDockerExecParens(tokens)
	if len(tokens) == 1 && tokens[0].kind == dockerExecIdent {
		return tokens[0].text
	}
	return ""
}

func dockerExecEmpty(tokens []dockerExecToken) bool {
	tokens = trimDockerExecParens(tokens)
	if len(tokens) != 1 {
		return false
	}
	return tokens[0].text == "None" || tokens[0].kind == dockerExecString && tokens[0].empty
}

func dockerExecConditionalEnd(tokens []dockerExecToken, start, wantedDepth int) int {
	depth := wantedDepth
	for index := start; index < len(tokens); index++ {
		word := tokens[index].text
		if depth == wantedDepth && (word == "else" || word == "for" || word == "if" || word == ",") {
			return index
		}
		depth += dockerExecDepthDelta(word)
		if depth < wantedDepth {
			return index
		}
	}
	return len(tokens)
}

func hasDockerExecFor(tokens []dockerExecToken, before, wantedDepth int) bool {
	depth := 0
	for _, token := range tokens[:before] {
		if token.text == "for" && depth == wantedDepth {
			return true
		}
		depth += dockerExecDepthDelta(token.text)
	}
	return false
}

func trimDockerExecParens(tokens []dockerExecToken) []dockerExecToken {
	start, end := trimDockerExecRange(tokens, 0, len(tokens))
	return tokens[start:end]
}

func trimDockerExecRange(tokens []dockerExecToken, start, end int) (int, int) {
	for end-start >= 2 && tokens[start].text == "(" {
		pairs := pairDockerExecTokens(tokens)
		if pairs[start] != end-1 {
			break
		}
		start, end = start+1, end-1
	}
	return start, end
}

func pairDockerExecTokens(tokens []dockerExecToken) map[int]int {
	pairs := make(map[int]int)
	var stack []int
	for index, token := range tokens {
		if token.text == "(" || token.text == "[" || token.text == "{" {
			stack = append(stack, index)
			continue
		}
		if token.text != ")" && token.text != "]" && token.text != "}" {
			continue
		}
		if len(stack) == 0 {
			continue
		}
		open := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		pairs[open], pairs[index] = index, open
	}
	return pairs
}

func topDockerExecIndex(tokens []dockerExecToken, word string) int {
	indices := topDockerExecIndices(tokens, word)
	if len(indices) != 0 {
		return indices[0]
	}
	return -1
}

func topDockerExecIndices(tokens []dockerExecToken, word string) []int {
	depth := 0
	var indices []int
	for index, token := range tokens {
		if depth == 0 && token.text == word {
			indices = append(indices, index)
		}
		depth += dockerExecDepthDelta(token.text)
	}
	return indices
}

func splitDockerExecTop(tokens []dockerExecToken, separator string) [][]dockerExecToken {
	indices := topDockerExecIndices(tokens, separator)
	if len(indices) == 0 {
		return [][]dockerExecToken{tokens}
	}
	out := make([][]dockerExecToken, 0, len(indices)+1)
	start := 0
	for _, index := range indices {
		out, start = append(out, tokens[start:index]), index+1
	}
	return append(out, tokens[start:])
}

func splitDockerExecTopWords(tokens []dockerExecToken, words ...string) [][]dockerExecToken {
	depth, start := 0, 0
	var out [][]dockerExecToken
	for index, token := range tokens {
		if depth == 0 && slices.Contains(words, token.text) {
			out, start = append(out, tokens[start:index]), index+1
		}
		depth += dockerExecDepthDelta(token.text)
	}
	if len(out) == 0 {
		return [][]dockerExecToken{tokens}
	}
	return append(out, tokens[start:])
}

func dockerExecDepthDelta(word string) int {
	if word == "(" || word == "[" || word == "{" {
		return 1
	}
	if word == ")" || word == "]" || word == "}" {
		return -1
	}
	return 0
}

func tokenizeDockerExecPython(source string) ([]dockerExecStatement, error) {
	var statements []dockerExecStatement
	var tokens []dockerExecToken
	var brackets []byte
	line, offset, logicalIndent := 1, 0, 0
	atLineStart := true
	for offset < len(source) {
		if atLineStart {
			indent := 0
			for offset < len(source) {
				if source[offset] == ' ' {
					indent, offset = indent+1, offset+1
					continue
				}
				if source[offset] == '\t' {
					indent, offset = indent+8-indent%8, offset+1
					continue
				}
				if source[offset] == '\f' {
					indent, offset = 0, offset+1
					continue
				}
				break
			}
			if len(tokens) == 0 && len(brackets) == 0 {
				logicalIndent = indent
			}
			atLineStart = false
			if offset >= len(source) {
				break
			}
		}
		character := source[offset]
		if character == '\r' || character == ' ' || character == '\t' || character == '\f' {
			offset++
			continue
		}
		if character == '\n' {
			offset++
			if len(brackets) == 0 && len(tokens) != 0 {
				statements = append(statements, dockerExecStatement{tokens: tokens, indent: logicalIndent})
				tokens = nil
			}
			line, atLineStart = line+1, true
			continue
		}
		if character == '#' {
			for offset < len(source) && source[offset] != '\n' {
				offset++
			}
			continue
		}
		if character == '\\' {
			if offset+1 < len(source) && source[offset+1] == '\n' {
				offset, line, atLineStart = offset+2, line+1, true
				continue
			}
			return nil, fmt.Errorf("unexpected character after line continuation character at line %d", line)
		}
		if dockerExecIdentifierStart(source, offset) {
			start := offset
			offset = dockerExecIdentifierEnd(source, offset)
			word := source[start:offset]
			if offset < len(source) && dockerExecQuote(source[offset]) && dockerExecStringPrefix(word) {
				token, next, nextLine, err := scanDockerExecString(source, start, offset, line, word)
				if err != nil {
					return nil, err
				}
				tokens, offset, line = append(tokens, token), next, nextLine
				continue
			}
			tokens = append(tokens, dockerExecToken{text: word, line: line, kind: dockerExecIdent})
			continue
		}
		if dockerExecQuote(character) {
			token, next, nextLine, err := scanDockerExecString(source, offset, offset, line, "")
			if err != nil {
				return nil, err
			}
			tokens, offset, line = append(tokens, token), next, nextLine
			continue
		}
		if character >= '0' && character <= '9' {
			start := offset
			for offset < len(source) && dockerExecNumberCharacter(source[offset]) {
				offset++
			}
			tokens = append(tokens, dockerExecToken{text: source[start:offset], line: line, kind: dockerExecNumber})
			continue
		}
		operator, width := dockerExecOperatorAt(source, offset)
		if operator == "" {
			return nil, fmt.Errorf("invalid character %q at line %d", character, line)
		}
		switch operator {
		case "(", "[", "{":
			brackets = append(brackets, operator[0])
		case ")", "]", "}":
			if len(brackets) == 0 || !dockerExecBracketPair(brackets[len(brackets)-1], operator[0]) {
				return nil, fmt.Errorf("unmatched %s at line %d", operator, line)
			}
			brackets = brackets[:len(brackets)-1]
		}
		tokens = append(tokens, dockerExecToken{text: operator, line: line, kind: dockerExecOther})
		offset += width
	}
	if len(brackets) != 0 {
		return nil, errors.New("unclosed delimiter")
	}
	if len(tokens) != 0 {
		statements = append(statements, dockerExecStatement{tokens: tokens, indent: logicalIndent})
	}
	return statements, nil
}

func dockerExecIdentifierStart(source string, offset int) bool {
	value, _ := utf8.DecodeRuneInString(source[offset:])
	return value == '_' || unicode.IsLetter(value)
}

func dockerExecIdentifierEnd(source string, offset int) int {
	for offset < len(source) {
		value, width := utf8.DecodeRuneInString(source[offset:])
		if value != '_' && !unicode.IsLetter(value) && !unicode.IsDigit(value) {
			break
		}
		offset += width
	}
	return offset
}

func dockerExecStringPrefix(word string) bool {
	if word == "" || len(word) > 3 {
		return false
	}
	for _, character := range strings.ToLower(word) {
		if !strings.ContainsRune("rubf", character) {
			return false
		}
	}
	return true
}

func scanDockerExecString(source string, tokenStart, quoteAt, line int, prefix string) (dockerExecToken, int, int, error) {
	quote := source[quoteAt]
	triple := quoteAt+2 < len(source) && source[quoteAt+1] == quote && source[quoteAt+2] == quote
	contentStart := quoteAt + 1
	if triple {
		contentStart = quoteAt + 3
	}
	startLine := line
	for offset := contentStart; offset < len(source); {
		if source[offset] == '\n' {
			if !triple {
				return dockerExecToken{}, 0, 0, fmt.Errorf("unterminated string literal at line %d", startLine)
			}
			line, offset = line+1, offset+1
			continue
		}
		if source[offset] == '\\' {
			offset += min(2, len(source)-offset)
			continue
		}
		if source[offset] != quote {
			offset++
			continue
		}
		width := 1
		if triple {
			if offset+2 >= len(source) || source[offset+1] != quote || source[offset+2] != quote {
				offset++
				continue
			}
			width = 3
		}
		body := source[contentStart:offset]
		empty := body == "" && !strings.ContainsAny(strings.ToLower(prefix), "bf")
		return dockerExecToken{text: source[tokenStart : offset+width], line: startLine, kind: dockerExecString, empty: empty}, offset + width, line, nil
	}
	return dockerExecToken{}, 0, 0, fmt.Errorf("unterminated string literal at line %d", startLine)
}

func dockerExecQuote(character byte) bool { return character == '\'' || character == '"' }

func dockerExecNumberCharacter(character byte) bool {
	return character == '_' || character == '.' || character == '+' || character == '-' ||
		character >= '0' && character <= '9' || character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z'
}

func dockerExecOperatorAt(source string, offset int) (string, int) {
	for _, width := range [...]int{3, 2, 1} {
		if offset+width <= len(source) {
			operator := source[offset : offset+width]
			if dockerExecOperatorKnown(operator) {
				return operator, width
			}
		}
	}
	return "", 0
}

func dockerExecOperatorKnown(operator string) bool {
	switch operator {
	case "**=", "//=", ">>=", "<<=", "...", ":=", "==", "!=", "<=", ">=", "->", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "//", "**", "<<", ">>", "@=", "(", ")", "[", "]", "{", "}", ":", ",", ";", ".", "+", "-", "*", "/", "%", "&", "|", "^", "~", "<", ">", "=", "@":
		return true
	}
	return false
}

func dockerExecBracketPair(open, close byte) bool {
	return open == '(' && close == ')' || open == '[' && close == ']' || open == '{' && close == '}'
}
