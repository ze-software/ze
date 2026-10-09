// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: loader_abnf.go — the Section 14 statement grammar whose argument rules these check
// Related: loader_structure.go — moduleExtensionSubstatementErrors applies this grammar
// RFC: rfc/short/rfc7950.md -- Sections 7.19 and 14, statements under an extension
package yang

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"sync"
)

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
		"uri-str":                   checkURI,
		"path-arg-str":              checkPathArg,
	}
	// RFC 7950 Section 14: "description-stmt = description-keyword sep
	// string stmtend" and "string = < an unquoted string, as returned by the
	// scanner, that matches the rule < yang-string > >". Every argument the
	// scanner returns is one, so the free-text rule, named here as
	// description's, accepts every argument.
	checkers[rfc7950Grammar().keywords["description"][0].argument] = func(string) error { return nil }
	return checkers
})

// checkArgument answers why argument breaks the Section 14 argument rule
// rule, or nil when it matches. A rule with no checker is answered as an
// error rather than accepted: TestEveryArgumentRuleHasAChecker keeps the
// checkers covering every rule the grammar names, so that error is a Ze
// defect surfacing, never a module fault.
func checkArgument(rule, argument string) error {
	check, checked := argumentCheckers()[rule]
	if !checked {
		return fmt.Errorf("BUG: Section 14 argument rule %s has no checker", rule)
	}
	return check(argument)
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

// checkURI checks a uri-str, the argument of namespace.
//
// RFC 7950 Section 14: "uri-str = < a string that matches the rule > < URI
// in RFC 3986 >". RFC 3986 Section 3: "URI = scheme ":" hier-part [ "?"
// query ] [ "#" fragment ]". A scheme holds no ":", a hier-part no "?" and a
// query no "#", so the first of each ends the part before it.
func checkURI(argument string) error {
	scheme, rest, found := strings.Cut(argument, ":")
	if !found {
		return fmt.Errorf("%q is not a URI: no scheme", argument)
	}
	if !isURIScheme(scheme) {
		return fmt.Errorf("%q is not a URI: scheme %q", argument, scheme)
	}
	rest, fragment, _ := strings.Cut(rest, "#")
	// RFC 3986 Section 3.5: "fragment = *( pchar / "/" / "?" )".
	if !isURIText(fragment, ":@/?") {
		return fmt.Errorf("%q is not a URI: fragment %q", argument, fragment)
	}
	hier, query, _ := strings.Cut(rest, "?")
	// RFC 3986 Section 3.4: "query = *( pchar / "/" / "?" )".
	if !isURIText(query, ":@/?") {
		return fmt.Errorf("%q is not a URI: query %q", argument, query)
	}
	if err := checkURIHierPart(hier); err != nil {
		return fmt.Errorf("%q is not a URI: %w", argument, err)
	}
	return nil
}

// isURIScheme reports whether scheme is an RFC 3986 scheme.
//
// RFC 3986 Section 3.1: "scheme = ALPHA *( ALPHA / DIGIT / "+" / "-" / "." )".
func isURIScheme(scheme string) bool {
	if scheme == "" {
		return false
	}
	if !isALPHA(scheme[0]) {
		return false
	}
	for i := 1; i < len(scheme); i++ {
		c := scheme[i]
		if !isALPHA(c) && !isDIGIT(c) && c != '+' && c != '-' && c != '.' {
			return false
		}
	}
	return true
}

// checkURIHierPart checks an RFC 3986 hier-part.
//
// RFC 3986 Section 3: "hier-part = "//" authority path-abempty /
// path-absolute / path-rootless / path-empty". Every path form is segments
// of pchar joined by "/" (Section 3.3, "segment = *pchar"), and the forms
// differ only in how they open, which the "//" test already decides.
func checkURIHierPart(hier string) error {
	rest, hasAuthority := strings.CutPrefix(hier, "//")
	if !hasAuthority {
		if !isURIText(hier, ":@/") {
			return fmt.Errorf("path %q", hier)
		}
		return nil
	}
	authority, path := rest, ""
	if slash := strings.IndexByte(rest, '/'); slash >= 0 {
		authority, path = rest[:slash], rest[slash:]
	}
	// RFC 3986 Section 3.3: "path-abempty = *( "/" segment )".
	if !isURIText(path, ":@/") {
		return fmt.Errorf("path %q", path)
	}
	return checkURIAuthority(authority)
}

// checkURIAuthority checks an RFC 3986 authority.
//
// RFC 3986 Section 3.2: "authority = [ userinfo "@" ] host [ ":" port ]",
// Section 3.2.1: "userinfo = *( unreserved / pct-encoded / sub-delims / ":"
// )", Section 3.2.3: "port = *DIGIT". A userinfo holds no "@", so the first
// "@" ends it.
func checkURIAuthority(authority string) error {
	hostPort := authority
	if userinfo, rest, found := strings.Cut(authority, "@"); found {
		if !isURIText(userinfo, ":") {
			return fmt.Errorf("userinfo %q", userinfo)
		}
		hostPort = rest
	}
	host, port := hostPort, ""
	if strings.HasPrefix(hostPort, "[") {
		closing := strings.IndexByte(hostPort, ']')
		if closing < 0 {
			return fmt.Errorf("IP-literal %q has no closing bracket", hostPort)
		}
		if err := checkURIIPLiteral(hostPort[1:closing]); err != nil {
			return err
		}
		host = ""
		rest := hostPort[closing+1:]
		if rest != "" {
			after, hasPort := strings.CutPrefix(rest, ":")
			if !hasPort {
				return fmt.Errorf("%q follows the IP-literal", rest)
			}
			port = after
		}
	} else if before, after, hasPort := strings.Cut(hostPort, ":"); hasPort {
		host, port = before, after
	}
	// RFC 3986 Section 3.2.2: "reg-name = *( unreserved / pct-encoded /
	// sub-delims )", which every IPv4address also matches.
	if !isURIText(host, "") {
		return fmt.Errorf("host %q", host)
	}
	for i := range len(port) {
		if !isDIGIT(port[i]) {
			return fmt.Errorf("port %q", port)
		}
	}
	return nil
}

// checkURIIPLiteral checks the text between the brackets of an RFC 3986
// IP-literal.
//
// RFC 3986 Section 3.2.2: "IP-literal = "[" ( IPv6address / IPvFuture ) "]""
// and "IPvFuture = "v" 1*HEXDIG "." 1*( unreserved / sub-delims / ":" )".
// RFC 3986 defines no zone identifier, so netip's zone form is refused.
func checkURIIPLiteral(literal string) error {
	if future, isFuture := strings.CutPrefix(strings.ToLower(literal), "v"); isFuture {
		version, address, dotted := strings.Cut(future, ".")
		if !dotted || version == "" || address == "" {
			return fmt.Errorf("IPvFuture %q", literal)
		}
		for i := range len(version) {
			if !isHEXDIG(version[i]) {
				return fmt.Errorf("IPvFuture %q", literal)
			}
		}
		if !isURIText(address, ":") || strings.Contains(address, "%") {
			return fmt.Errorf("IPvFuture %q", literal)
		}
		return nil
	}
	addr, err := netip.ParseAddr(literal)
	if err != nil {
		return fmt.Errorf("IPv6address %q: %w", literal, err)
	}
	if !addr.Is6() {
		return fmt.Errorf("IPv6address %q is not IPv6", literal)
	}
	if addr.Zone() != "" {
		return fmt.Errorf("IPv6address %q carries a zone", literal)
	}
	return nil
}

// isHEXDIG reports whether c is an RFC 5234 HEXDIG, which RFC 3986 reads
// case-insensitively.
func isHEXDIG(c byte) bool {
	return isDIGIT(c) || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

// isURIText reports whether every character of text is an RFC 3986
// unreserved character, a pct-encoded triplet, a sub-delim, or one of extra.
//
// RFC 3986 Section 2.1: "pct-encoded = "%" HEXDIG HEXDIG". Section 2.2:
// "sub-delims = "!" / "$" / "&" / "'" / "(" / ")" / "*" / "+" / "," / ";" /
// "="". Section 2.3: "unreserved = ALPHA / DIGIT / "-" / "." / "_" / "~"".
func isURIText(text, extra string) bool {
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '%' {
			if i+2 >= len(text) {
				return false
			}
			if !isHEXDIG(text[i+1]) || !isHEXDIG(text[i+2]) {
				return false
			}
			i += 2
			continue
		}
		if isALPHA(c) || isDIGIT(c) {
			continue
		}
		if strings.IndexByte("-._~!$&'()*+,;="+extra, c) < 0 {
			return false
		}
	}
	return true
}

// checkPathArg checks a path-arg-str, the argument of a leafref path.
//
// RFC 7950 Section 14: "path-arg = absolute-path / relative-path". An
// absolute-path opens with "/" and a relative-path with "../", so the first
// byte decides which is read.
func checkPathArg(argument string) error {
	path := pathArg{text: argument}
	var matched bool
	if strings.HasPrefix(argument, "/") {
		matched = path.absolute()
	} else {
		matched = path.relative()
	}
	if !matched {
		return fmt.Errorf("%q is not a path-arg", argument)
	}
	if path.at != len(argument) {
		return fmt.Errorf("%q is not a path-arg", argument)
	}
	return nil
}

// pathArg is a reader of one Section 14 path-arg. No rule of path-arg
// recurses into itself, so the reader is iterative and every loop consumes
// at least one byte of text.
type pathArg struct {
	text string
	at   int
}

// absolute reads "absolute-path = 1*("/" (node-identifier *path-predicate))".
func (p *pathArg) absolute() bool {
	steps := 0
	for p.peek('/') {
		p.at++
		if !p.nodeIdentifier() {
			return false
		}
		for p.peek('[') {
			if !p.predicate() {
				return false
			}
		}
		steps++
	}
	return steps > 0
}

// relative reads "relative-path = 1*("../") descendant-path" and
// "descendant-path = node-identifier [*path-predicate absolute-path]".
func (p *pathArg) relative() bool {
	ups := 0
	for strings.HasPrefix(p.text[p.at:], "../") {
		p.at += len("../")
		ups++
	}
	if ups == 0 {
		return false
	}
	if !p.nodeIdentifier() {
		return false
	}
	if !p.peek('[') && !p.peek('/') {
		return true
	}
	for p.peek('[') {
		if !p.predicate() {
			return false
		}
	}
	return p.absolute()
}

// predicate reads "path-predicate = "[" *WSP path-equality-expr *WSP "]"",
// "path-equality-expr = node-identifier *WSP "=" *WSP path-key-expr" and
// "path-key-expr = current-function-invocation *WSP "/" *WSP
// rel-path-keyexpr".
func (p *pathArg) predicate() bool {
	p.at++
	p.wsp()
	if !p.nodeIdentifier() {
		return false
	}
	p.wsp()
	if !p.byte('=') {
		return false
	}
	p.wsp()
	// RFC 7950 Section 14: "current-function-invocation = current-keyword
	// *WSP "(" *WSP ")"".
	if !strings.HasPrefix(p.text[p.at:], "current") {
		return false
	}
	p.at += len("current")
	p.wsp()
	if !p.byte('(') {
		return false
	}
	p.wsp()
	if !p.byte(')') {
		return false
	}
	p.wsp()
	if !p.byte('/') {
		return false
	}
	p.wsp()
	if !p.relativeKey() {
		return false
	}
	p.wsp()
	return p.byte(']')
}

// relativeKey reads "rel-path-keyexpr = 1*(".." *WSP "/" *WSP)
// *(node-identifier *WSP "/" *WSP) node-identifier".
func (p *pathArg) relativeKey() bool {
	ups := 0
	for strings.HasPrefix(p.text[p.at:], "..") {
		p.at += len("..")
		p.wsp()
		if !p.byte('/') {
			return false
		}
		p.wsp()
		ups++
	}
	if ups == 0 {
		return false
	}
	for {
		if !p.nodeIdentifier() {
			return false
		}
		mark := p.at
		p.wsp()
		if !p.byte('/') {
			p.at = mark
			return true
		}
		p.wsp()
	}
}

// nodeIdentifier reads "node-identifier = [prefix ":"] identifier".
func (p *pathArg) nodeIdentifier() bool {
	n := identifierRefLength(p.text[p.at:])
	p.at += n
	return n > 0
}

// wsp reads "*WSP", spaces and tabs only: a path-arg admits no line-break.
func (p *pathArg) wsp() {
	for p.at < len(p.text) && (p.text[p.at] == ' ' || p.text[p.at] == '\t') {
		p.at++
	}
}

// peek reports whether the next byte is c, without reading it.
func (p *pathArg) peek(c byte) bool {
	return p.at < len(p.text) && p.text[p.at] == c
}

// byte reads c and reports whether it was next.
func (p *pathArg) byte(c byte) bool {
	if !p.peek(c) {
		return false
	}
	p.at++
	return true
}
