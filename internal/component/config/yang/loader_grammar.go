// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: loader_structure.go — moduleExtensionSubstatementErrors applies this grammar
// RFC: rfc/short/rfc7950.md -- Sections 7.19 and 14, statements under an extension
package yang

import (
	_ "embed"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// statementGrammar is the Section 14 grammar of one YANG statement keyword:
// the argument rules its statement rules accept after "sep", none when the
// statement takes no argument, and the statement keywords its block admits.
// An extension usage is admitted in every block (stmtsep, unknown-statement),
// so it is never listed.
type statementGrammar struct {
	arguments []string
	children  []string
}

// uncheckedArgumentRules are the Section 14 argument rules no checker in
// argumentCheckers implements yet: "uri-str" (an RFC 3986 URI) and
// "path-arg-str" (the leafref path grammar). An argument under one of these
// rules is accepted unchecked; TestEveryArgumentRuleHasAChecker keeps this
// list and argumentCheckers covering every rule the grammar names.
var uncheckedArgumentRules = []string{"path-arg-str", "uri-str"}

// argumentChecker answers why an argument breaks one Section 14 argument
// rule, or nil when it matches the rule.
type argumentChecker func(argument string) error

// argumentCheckers maps each Section 14 argument rule, by its ABNF name, to
// the check of that rule. Every "<rule>-str" of the grammar reads "< a string
// that matches the rule > < <rule> >", so the checker reads the argument
// after the scanner has unquoted and joined it.
var argumentCheckers = sync.OnceValue(func() map[string]argumentChecker {
	checkers := map[string]argumentChecker{
		"identifier-arg-str":        checkIdentifier,
		"prefix-arg-str":            checkIdentifier,
		"identifier-ref-arg-str":    checkIdentifierRef,
		"revision-date":             checkDate,
		"yang-version-arg-str":      literalChecker("1.1"),
		"config-arg-str":            literalChecker("true", "false"),
		"mandatory-arg-str":         literalChecker("true", "false"),
		"require-instance-arg-str":  literalChecker("true", "false"),
		"yin-element-arg-str":       literalChecker("true", "false"),
		"status-arg-str":            literalChecker("current", "obsolete", "deprecated"),
		"ordered-by-arg-str":        literalChecker("user", "system"),
		"modifier-arg-str":          literalChecker("invert-match"),
		"not-supported-keyword-str": literalChecker("not-supported"),
		"add-keyword-str":           literalChecker("add"),
		"delete-keyword-str":        literalChecker("delete"),
		"replace-keyword-str":       literalChecker("replace"),
		"min-value-arg-str":         checkNonNegativeInteger,
		"position-value-arg-str":    checkNonNegativeInteger,
		"max-value-arg-str":         checkMaxValue,
		"integer-value-str":         checkIntegerValue,
		"fraction-digits-arg-str":   checkFractionDigits,
		"length-arg-str":            checkLengthArgument,
		"range-arg-str":             checkRangeArgument,
		"key-arg-str":               sepListChecker(checkNodeIdentifier),
		"unique-arg-str":            sepListChecker(checkDescendantSchemaNodeid),
		"refine-arg-str":            checkDescendantSchemaNodeid,
		"uses-augment-arg-str":      checkDescendantSchemaNodeid,
		"augment-arg-str":           checkAbsoluteSchemaNodeid,
		"deviation-arg-str":         checkAbsoluteSchemaNodeid,
		"if-feature-expr-str":       checkIfFeatureExpr,
	}
	// RFC 7950 Section 14: "description-stmt = description-keyword sep
	// string stmtend" and "string = < an unquoted string, as returned by the
	// scanner, that matches the rule < yang-string > >". Every argument the
	// scanner returns is one, so the free-text rule, named here as
	// description's, accepts every argument.
	checkers[statementGrammars()["description"].arguments[0]] = func(string) error { return nil }
	return checkers
})

// checkArgument answers why argument matches none of the argument rules
// rules, or nil when it matches one. A rule listed in uncheckedArgumentRules
// accepts every argument.
func checkArgument(rules []string, argument string) error {
	errs := make([]error, 0, len(rules))
	for _, rule := range rules {
		check, checked := argumentCheckers()[rule]
		if !checked {
			return nil
		}
		err := check(argument)
		if err == nil {
			return nil
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// literalChecker answers a checker accepting exactly one of literals. The
// Section 14 keywords are case-sensitive: "true-keyword = %s"true"".
func literalChecker(literals ...string) argumentChecker {
	return func(argument string) error {
		if slices.Contains(literals, argument) {
			return nil
		}
		return fmt.Errorf("%q is not one of %s", argument, strings.Join(literals, ", "))
	}
}

// isALPHA reports whether c is an RFC 5234 ALPHA, %x41-5A / %x61-7A.
func isALPHA(c byte) bool {
	return ('A' <= c && c <= 'Z') || ('a' <= c && c <= 'z')
}

// isDIGIT reports whether c is an RFC 5234 DIGIT, %x30-39.
func isDIGIT(c byte) bool {
	return '0' <= c && c <= '9'
}

// isSep reports whether c can occur in a Section 14 "sep", 1*(WSP /
// line-break), where WSP is SP or HTAB and line-break is CRLF or LF.
func isSep(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// sepLength answers how many bytes of a "sep" open text, zero when none does.
// A CR counts only as the start of a CRLF line-break.
func sepLength(text string) int {
	n := 0
	for n < len(text) {
		c := text[n]
		if c == '\r' {
			if n+1 < len(text) && text[n+1] == '\n' {
				n += 2
				continue
			}
			return n
		}
		if c != ' ' && c != '\t' && c != '\n' {
			return n
		}
		n++
	}
	return n
}

// identifierLength answers how many bytes of one identifier open text, zero
// when text opens with none.
//
// RFC 7950 Section 14: "identifier = (ALPHA / "_") *(ALPHA / DIGIT / "_" /
// "-" / ".")".
func identifierLength(text string) int {
	if text == "" {
		return 0
	}
	if !isALPHA(text[0]) && text[0] != '_' {
		return 0
	}
	n := 1
	for n < len(text) {
		c := text[n]
		if !isALPHA(c) && !isDIGIT(c) && c != '_' && c != '-' && c != '.' {
			break
		}
		n++
	}
	return n
}

// identifierRefLength answers how many bytes of one identifier-ref open text.
//
// RFC 7950 Section 14: "identifier-ref = [prefix ":"] identifier" and
// "prefix = identifier".
func identifierRefLength(text string) int {
	n := identifierLength(text)
	if n == 0 {
		return 0
	}
	if n < len(text) && text[n] == ':' {
		local := identifierLength(text[n+1:])
		if local == 0 {
			return 0
		}
		return n + 1 + local
	}
	return n
}

// checkIdentifier checks an identifier-arg (RFC 7950 Section 14:
// "identifier-arg = identifier") or a prefix-arg ("prefix-arg = prefix").
func checkIdentifier(argument string) error {
	if identifierLength(argument) != len(argument) || argument == "" {
		return fmt.Errorf("%q is not an identifier", argument)
	}
	return nil
}

// checkIdentifierRef checks an identifier-ref-arg (RFC 7950 Section 14:
// "identifier-ref-arg = identifier-ref").
func checkIdentifierRef(argument string) error {
	if identifierRefLength(argument) != len(argument) || argument == "" {
		return fmt.Errorf("%q is not an identifier-ref", argument)
	}
	return nil
}

// checkNodeIdentifier checks one node-identifier (RFC 7950 Section 14:
// "node-identifier = [prefix ":"] identifier"), the shape of an
// identifier-ref.
func checkNodeIdentifier(argument string) error {
	if identifierRefLength(argument) != len(argument) || argument == "" {
		return fmt.Errorf("%q is not a node-identifier", argument)
	}
	return nil
}

// checkAbsoluteSchemaNodeid checks an absolute-schema-nodeid.
//
// RFC 7950 Section 14: "absolute-schema-nodeid = 1*("/" node-identifier)".
func checkAbsoluteSchemaNodeid(argument string) error {
	rest, rooted := strings.CutPrefix(argument, "/")
	if !rooted {
		return fmt.Errorf("%q is not an absolute-schema-nodeid", argument)
	}
	for step := range strings.SplitSeq(rest, "/") {
		if err := checkNodeIdentifier(step); err != nil {
			return fmt.Errorf("%q is not an absolute-schema-nodeid: %w", argument, err)
		}
	}
	return nil
}

// checkDescendantSchemaNodeid checks a descendant-schema-nodeid.
//
// RFC 7950 Section 14: "descendant-schema-nodeid = node-identifier
// [absolute-schema-nodeid]".
func checkDescendantSchemaNodeid(argument string) error {
	for step := range strings.SplitSeq(argument, "/") {
		if err := checkNodeIdentifier(step); err != nil {
			return fmt.Errorf("%q is not a descendant-schema-nodeid: %w", argument, err)
		}
	}
	return nil
}

// sepListChecker answers a checker of one or more items separated by "sep",
// as in RFC 7950 Section 14 "key-arg = node-identifier *(sep
// node-identifier)" and "unique-arg = descendant-schema-nodeid *(sep
// descendant-schema-nodeid)".
func sepListChecker(item argumentChecker) argumentChecker {
	return func(argument string) error {
		rest := argument
		for {
			n := 0
			for n < len(rest) && !isSep(rest[n]) {
				n++
			}
			if err := item(rest[:n]); err != nil {
				return err
			}
			rest = rest[n:]
			if rest == "" {
				return nil
			}
			gap := sepLength(rest)
			if gap == 0 {
				return fmt.Errorf("%q: a lone CR is not a separator", argument)
			}
			rest = rest[gap:]
			if rest == "" {
				return fmt.Errorf("%q ends with a separator", argument)
			}
		}
	}
}

// checkDate checks a date-arg.
//
// RFC 7950 Section 14: "date-arg = 4DIGIT "-" 2DIGIT "-" 2DIGIT".
func checkDate(argument string) error {
	shape := "dddd-dd-dd"
	if len(argument) != len(shape) {
		return fmt.Errorf("%q is not a date YYYY-MM-DD", argument)
	}
	for i := range len(shape) {
		if shape[i] == '-' {
			if argument[i] != '-' {
				return fmt.Errorf("%q is not a date YYYY-MM-DD", argument)
			}
			continue
		}
		if !isDIGIT(argument[i]) {
			return fmt.Errorf("%q is not a date YYYY-MM-DD", argument)
		}
	}
	return nil
}

// checkNonNegativeInteger checks a non-negative-integer-value, the rule of
// "min-value-arg" and "position-value-arg".
//
// RFC 7950 Section 14: "non-negative-integer-value = "0" /
// positive-integer-value" and "positive-integer-value = (non-zero-digit
// *DIGIT)".
func checkNonNegativeInteger(argument string) error {
	if argument == "0" {
		return nil
	}
	return checkPositiveInteger(argument)
}

// checkPositiveInteger checks a positive-integer-value, a non-zero digit and
// then any digits.
func checkPositiveInteger(argument string) error {
	if argument == "" || argument[0] == '0' {
		return fmt.Errorf("%q is not a positive integer", argument)
	}
	for i := range len(argument) {
		if !isDIGIT(argument[i]) {
			return fmt.Errorf("%q is not a positive integer", argument)
		}
	}
	return nil
}

// checkMaxValue checks a max-value-arg.
//
// RFC 7950 Section 14: "max-value-arg = unbounded-keyword /
// positive-integer-value".
func checkMaxValue(argument string) error {
	if argument == "unbounded" {
		return nil
	}
	return checkPositiveInteger(argument)
}

// checkIntegerValue checks an integer-value.
//
// RFC 7950 Section 14: "integer-value = ("-" non-negative-integer-value) /
// non-negative-integer-value".
func checkIntegerValue(argument string) error {
	return checkNonNegativeInteger(strings.TrimPrefix(argument, "-"))
}

// checkFractionDigits checks a fraction-digits-arg, "1" to "18" with no
// leading zero.
//
// RFC 7950 Section 14: "fraction-digits-arg = ("1" ["0" / "1" / "2" / "3" /
// "4" / "5" / "6" / "7" / "8"]) / "2" / "3" / "4" / "5" / "6" / "7" / "8" /
// "9"".
func checkFractionDigits(argument string) error {
	if checkPositiveInteger(argument) != nil {
		return fmt.Errorf("%q is not a fraction-digits value 1 to 18", argument)
	}
	digits, err := strconv.Atoi(argument)
	if err != nil {
		return fmt.Errorf("%q is not a fraction-digits value 1 to 18", argument)
	}
	if digits > 18 {
		return fmt.Errorf("%q is not a fraction-digits value 1 to 18", argument)
	}
	return nil
}

// checkDecimalValue checks a decimal-value.
//
// RFC 7950 Section 14: "decimal-value = integer-value ("."
// zero-integer-value)" and "zero-integer-value = 1*DIGIT".
func checkDecimalValue(argument string) error {
	integer, fraction, dotted := strings.Cut(argument, ".")
	if !dotted || fraction == "" {
		return fmt.Errorf("%q is not a decimal value", argument)
	}
	if err := checkIntegerValue(integer); err != nil {
		return fmt.Errorf("%q is not a decimal value", argument)
	}
	for i := range len(fraction) {
		if !isDIGIT(fraction[i]) {
			return fmt.Errorf("%q is not a decimal value", argument)
		}
	}
	return nil
}

// checkLengthArgument checks a length-arg.
//
// RFC 7950 Section 14: "length-arg = length-part *(optsep "|" optsep
// length-part)", "length-part = length-boundary [optsep ".." optsep
// length-boundary]" and "length-boundary = min-keyword / max-keyword /
// non-negative-integer-value".
func checkLengthArgument(argument string) error {
	return checkBoundaryExpression(argument, func(boundary string) error {
		if boundary == boundaryMin || boundary == boundaryMax {
			return nil
		}
		return checkNonNegativeInteger(boundary)
	})
}

// checkRangeArgument checks a range-arg.
//
// RFC 7950 Section 14: "range-arg = range-part *(optsep "|" optsep
// range-part)", "range-part = range-boundary [optsep ".." optsep
// range-boundary]" and "range-boundary = min-keyword / max-keyword /
// integer-value / decimal-value".
func checkRangeArgument(argument string) error {
	return checkBoundaryExpression(argument, func(boundary string) error {
		if boundary == boundaryMin || boundary == boundaryMax {
			return nil
		}
		if checkIntegerValue(boundary) == nil {
			return nil
		}
		return checkDecimalValue(boundary)
	})
}

// checkBoundaryExpression checks the shape length-arg and range-arg share:
// parts separated by "|", each one boundary or two joined by "..", with
// optional "sep" around both separators and nowhere else.
func checkBoundaryExpression(argument string, boundary argumentChecker) error {
	if argument == "" {
		return errors.New("empty expression")
	}
	if isSep(argument[0]) || isSep(argument[len(argument)-1]) {
		return fmt.Errorf("%q opens or ends with whitespace", argument)
	}
	for part := range strings.SplitSeq(argument, "|") {
		lower, upper, isRange := strings.Cut(part, "..")
		if err := boundary(trimOptsep(lower)); err != nil {
			return fmt.Errorf("%q: %w", argument, err)
		}
		if !isRange {
			continue
		}
		if err := boundary(trimOptsep(upper)); err != nil {
			return fmt.Errorf("%q: %w", argument, err)
		}
	}
	return nil
}

// trimOptsep removes the optional "sep" around one boundary.
func trimOptsep(text string) string {
	return strings.Trim(text, " \t\r\n")
}

// checkIfFeatureExpr checks an if-feature-expr.
//
// RFC 7950 Section 14: "if-feature-expr = if-feature-term [sep or-keyword
// sep if-feature-expr]", "if-feature-term = if-feature-factor [sep
// and-keyword sep if-feature-term]" and "if-feature-factor = not-keyword sep
// if-feature-factor / "(" optsep if-feature-expr optsep ")" /
// identifier-ref-arg".
func checkIfFeatureExpr(argument string) error {
	parser := featureExpr{text: argument}
	if !parser.expr() {
		return fmt.Errorf("%q is not an if-feature expression", argument)
	}
	if parser.at != len(argument) {
		return fmt.Errorf("%q is not an if-feature expression", argument)
	}
	return nil
}

// featureExpr is a recursive-descent reader of one if-feature-expr. Each
// level of recursion consumes at least one byte of text, so the depth is
// bounded by the length of the argument, which a module author writes.
type featureExpr struct {
	text string
	at   int
}

// expr reads "if-feature-term [sep or-keyword sep if-feature-expr]".
func (p *featureExpr) expr() bool {
	return p.joined(p.term, "or", p.expr)
}

// term reads "if-feature-factor [sep and-keyword sep if-feature-term]".
func (p *featureExpr) term() bool {
	return p.joined(p.factor, "and", p.term)
}

// joined reads first, then optionally sep, keyword, sep and rest, restoring
// the position when the optional tail is absent.
func (p *featureExpr) joined(first func() bool, keyword string, rest func() bool) bool {
	if !first() {
		return false
	}
	mark := p.at
	if p.sep() && p.keyword(keyword) && p.sep() {
		return rest()
	}
	p.at = mark
	return true
}

// factor reads "not-keyword sep if-feature-factor / "(" optsep
// if-feature-expr optsep ")" / identifier-ref-arg".
func (p *featureExpr) factor() bool {
	mark := p.at
	if p.keyword("not") && p.sep() {
		return p.factor()
	}
	p.at = mark
	if p.at < len(p.text) && p.text[p.at] == '(' {
		p.at++
		p.at += sepLength(p.text[p.at:])
		if !p.expr() {
			return false
		}
		p.at += sepLength(p.text[p.at:])
		if p.at < len(p.text) && p.text[p.at] == ')' {
			p.at++
			return true
		}
		return false
	}
	n := identifierRefLength(p.text[p.at:])
	p.at += n
	return n > 0
}

// sep reads one "sep" and reports whether there was one.
func (p *featureExpr) sep() bool {
	n := sepLength(p.text[p.at:])
	p.at += n
	return n > 0
}

// keyword reads the case-sensitive keyword word and reports whether it was
// there.
func (p *featureExpr) keyword(word string) bool {
	if !strings.HasPrefix(p.text[p.at:], word) {
		return false
	}
	p.at += len(word)
	return true
}

// rfc7950ABNF is the RFC 7950 Section 14 grammar, the "yang.abnf" code
// component of rfc/full/rfc7950.txt with the page headers and footers removed.
// TestEmbeddedGrammarIsTheRFC7950Grammar re-extracts it from the RFC and
// turns red on any difference.
//
//go:embed rfc7950.abnf
var rfc7950ABNF string

// statementGrammars answers the Section 14 statement grammar, one entry for
// each statement keyword, parsed once from rfc7950ABNF. The text is part of
// the binary, so a parse failure is a Ze defect.
var statementGrammars = sync.OnceValue(func() map[string]statementGrammar {
	grammar, err := parseStatementGrammar(rfc7950ABNF)
	if err != nil {
		panic("BUG: the embedded RFC 7950 grammar does not parse: " + err.Error())
	}
	return grammar
})

// abnfRuleHead matches the head of one rule of the Section 14 grammar, which
// the RFC sets at a three-space indent; a continuation line is indented
// further.
var abnfRuleHead = regexp.MustCompile(`(?m)^ {3}([A-Za-z][A-Za-z0-9-]*)\s*=`)

// abnfLiteral matches what names no rule: a quoted or case-sensitive literal,
// a numeric terminal, and the RFC's "< prose >" annotation.
var abnfLiteral = regexp.MustCompile(`%s"[^"]*"|"[^"]*"|%x[0-9A-Fa-f.-]+|<[^>]*>`)

// abnfName matches one rule name inside a rule body.
var abnfName = regexp.MustCompile(`[A-Za-z][A-Za-z0-9-]*`)

// abnfKeyword matches a keyword rule, "<name>-keyword = %s"<spelling>"".
var abnfKeyword = regexp.MustCompile(`(?m)^\s+([a-z-]+)-keyword\s+=\s+%s"([a-z-]+)"`)

// abnfStatementRule is one statement rule: the keyword it spells, the argument
// rule after "sep" ("" when the rule reads "<keyword> optsep"), and the rule
// names of its block.
type abnfStatementRule struct {
	keyword  string
	argument string
	block    []string
}

// parseStatementGrammar reads the Section 14 grammar abnf and answers, for
// every statement keyword, the argument rules its statement rules name after
// "sep" and the statement keywords its block admits. A statement rule is a
// "<name>-stmt" rule whose body opens, after an optional "optsep", with a
// keyword rule. A block's statements are collected through every rule it
// names (body-stmts, data-def-stmt, type-body-stmts, ...), stopping at
// unknown-statement, the extension usage stmtsep admits in every block.
func parseStatementGrammar(abnf string) (map[string]statementGrammar, error) {
	spelled := map[string]string{}
	for _, m := range abnfKeyword.FindAllStringSubmatch(abnf, -1) {
		spelled[m[1]] = m[2]
	}
	rules := abnfRules(abnf)
	statements := map[string]abnfStatementRule{}
	for name, body := range rules {
		rule, isStatement, err := abnfStatement(name, body, spelled)
		if err != nil {
			return nil, err
		}
		if isStatement {
			statements[name] = rule
		}
	}
	if len(statements) == 0 {
		return nil, errors.New("no statement rule")
	}
	arguments := map[string][]string{}
	children := map[string][]string{}
	for _, rule := range statements {
		if rule.argument != "" && !slices.Contains(arguments[rule.keyword], rule.argument) {
			arguments[rule.keyword] = append(arguments[rule.keyword], rule.argument)
		}
		children[rule.keyword] = abnfBlockStatements(rule.block, rules, statements, children[rule.keyword])
	}
	grammar := make(map[string]statementGrammar, len(children))
	for keyword, admitted := range children {
		slices.Sort(admitted)
		slices.Sort(arguments[keyword])
		grammar[keyword] = statementGrammar{arguments: arguments[keyword], children: admitted}
	}
	return grammar, nil
}

// abnfRules answers every rule of abnf, by name, as the rule names its body
// holds once comments and literals are removed.
func abnfRules(abnf string) map[string][]string {
	var lines []string
	for line := range strings.SplitSeq(abnf, "\n") {
		line = abnfLiteral.ReplaceAllString(line, " ")
		if comment := strings.IndexByte(line, ';'); comment >= 0 {
			line = line[:comment]
		}
		lines = append(lines, line)
	}
	text := strings.Join(lines, "\n")
	rules := map[string][]string{}
	heads := abnfRuleHead.FindAllStringSubmatchIndex(text, -1)
	for i, head := range heads {
		bodyEnd := len(text)
		if i+1 < len(heads) {
			bodyEnd = heads[i+1][0]
		}
		rules[text[head[2]:head[3]]] = abnfName.FindAllString(text[head[1]:bodyEnd], -1)
	}
	return rules
}

// abnfStatement answers the statement rule named name with body, and whether
// name is a statement rule at all.
func abnfStatement(name string, body []string, spelled map[string]string) (abnfStatementRule, bool, error) {
	if !strings.HasSuffix(name, "-stmt") {
		return abnfStatementRule{}, false, nil
	}
	rest := body
	if len(rest) > 0 && rest[0] == "optsep" {
		rest = rest[1:]
	}
	if len(rest) == 0 {
		return abnfStatementRule{}, false, nil
	}
	base, isKeyword := strings.CutSuffix(rest[0], "-keyword")
	if !isKeyword {
		return abnfStatementRule{}, false, nil
	}
	keyword, known := spelled[base]
	if !known {
		return abnfStatementRule{}, false, fmt.Errorf("statement rule %s names %s, which the grammar never spells", name, rest[0])
	}
	rule := abnfStatementRule{keyword: keyword, block: rest[1:]}
	if len(rule.block) > 1 && rule.block[0] == "sep" {
		rule.argument = rule.block[1]
		rule.block = rule.block[2:]
	}
	return rule, true, nil
}

// abnfBlockStatements appends to admitted, once each, the keywords of the
// statement rules block reaches. The walk is an explicit stack over rule
// names, each expanded once, so it ends within the grammar's rule count.
func abnfBlockStatements(block []string, rules map[string][]string, statements map[string]abnfStatementRule, admitted []string) []string {
	seen := map[string]bool{}
	pending := slices.Clone(block)
	for len(pending) > 0 {
		name := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if child, isStatement := statements[name]; isStatement {
			if !slices.Contains(admitted, child.keyword) {
				admitted = append(admitted, child.keyword)
			}
			continue
		}
		body, defined := rules[name]
		if !defined {
			continue
		}
		if name == "unknown-statement" {
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		pending = append(pending, body...)
	}
	return admitted
}
