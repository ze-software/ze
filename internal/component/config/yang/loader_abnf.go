// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: loader_grammar.go — the checks of the argument rules this grammar names
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

// rfc7950ABNF is the RFC 7950 Section 14 grammar, the "yang.abnf" code
// component of rfc/full/rfc7950.txt with the page headers and footers removed.
// TestEmbeddedGrammarIsTheRFC7950Grammar re-extracts it from the RFC and
// turns red on any difference.
//
//go:embed rfc7950.abnf
var rfc7950ABNF string

// rfc7950Grammar answers the Section 14 statement grammar, parsed once from
// rfc7950ABNF. The text is part of the binary, so a parse failure is a Ze
// defect.
var rfc7950Grammar = sync.OnceValue(func() *yangGrammar {
	grammar, err := parseYANGGrammar(rfc7950ABNF)
	if err != nil {
		panic("BUG: the embedded RFC 7950 grammar does not parse: " + err.Error())
	}
	return grammar
})

// yangGrammar is the Section 14 grammar read as statement productions: one
// for each "<name>-stmt" rule, so a keyword with several rules (deviate has
// four, augment two) has several productions. parseYANGGrammar is its only
// constructor, and every production it holds has its block resolved, so a
// yangGrammar in hand answers every question the checks ask of it.
type yangGrammar struct {
	// rules is every rule of the grammar, by name.
	rules map[string]abnfExpr
	// productions is every statement production, by rule name.
	productions map[string]*statementProduction
	// keywords is every statement production, by the keyword it spells.
	keywords map[string][]*statementProduction
	// reaches reports, for every rule that is no statement production,
	// whether its body names one; a rule that names none matches no
	// statement and so leaves every substatement count unchanged.
	reaches map[string]bool
	// extension is the block of an extension usage, unknown-statement.
	extension *statementBlock
}

// statementProduction is one Section 14 statement rule: the keyword it
// spells, the argument rule after "sep" ("" when the rule reads
// "<keyword> optsep"), whether its block is required, and the block.
type statementProduction struct {
	rule     string
	keyword  string
	argument string
	form     blockForm
	block    *statementBlock
}

// blockForm is how a statement rule closes: with a block it may omit, as in
// "(";" / "{" ... "}")" and stmtend, or with a block it must give, a bare
// "{". The zero value is no form.
type blockForm uint8

const (
	blockFormUnspecified blockForm = iota
	blockOptional
	blockRequired
)

// statementBlock is what a statement rule's block holds: the expression its
// substatements must match, and the productions that expression reaches, by
// keyword. A block is what a parent statement hands to each substatement: the
// substatement's production is chosen among the block's, so the context of a
// keyword is its parent's rule and never the keyword alone.
type statementBlock struct {
	// owner is the rule the block belongs to, for messages.
	owner string
	// content is the expression between "{" and "}"; an empty sequence for
	// stmtend, whose block "{" stmtsep "}" holds no YANG statement.
	content abnfExpr
	// admitted is every production content reaches, by keyword.
	admitted map[string][]*statementProduction
	// slots is the index of each admitted production in a childCounts.
	slots map[*statementProduction]int
	// emptyAccepted reports whether content matches no substatement at all.
	emptyAccepted bool
}

// abnfExpr is one element of a Section 14 rule body. Its variants are
// abnfRef, abnfTerminal, abnfSequence, abnfAlternation and abnfRepeat, each
// built only by parseABNFBody.
type abnfExpr interface{ abnfExpr() }

// abnfRef names another rule.
type abnfRef struct{ name string }

// abnfTerminal is a literal: a quoted or case-sensitive string, a numeric
// terminal, or a "< prose >" annotation, as written.
type abnfTerminal struct{ text string }

// abnfSequence is a concatenation, every item in turn.
type abnfSequence struct{ items []abnfExpr }

// abnfAlternation is "/", one of the choices.
type abnfAlternation struct{ choices []abnfExpr }

// abnfRepeat is "<min>*<max>item", "[item]" as 0*1, and "<n>item" as n*n.
// max is repeatUnbounded when the RFC gives none.
type abnfRepeat struct {
	min  int
	max  int
	item abnfExpr
}

func (abnfRef) abnfExpr()         {}
func (abnfTerminal) abnfExpr()    {}
func (abnfSequence) abnfExpr()    {}
func (abnfAlternation) abnfExpr() {}
func (abnfRepeat) abnfExpr()      {}

// repeatUnbounded is the max of a repetition with no upper bound.
const repeatUnbounded = -1

// extensionRule is the Section 14 rule of an extension usage. Its block,
// "*((yang-stmt / unknown-statement) optsep)", is the one every extension
// usage hands its substatements; a nested usage is not counted, because its
// substatements are "defined by the "extension" statement" (Section 7.19).
const extensionRule = "unknown-statement"

// abnfRuleHead matches the head of one rule of the Section 14 grammar, which
// the RFC sets at a three-space indent; a continuation line is indented
// further.
var abnfRuleHead = regexp.MustCompile(`(?m)^ {3}([A-Za-z][A-Za-z0-9-]*)\s*=`)

// parseYANGGrammar reads the Section 14 grammar abnf into statement
// productions. A statement rule is a "<name>-stmt" rule whose body opens,
// after an optional "optsep", with a keyword rule.
func parseYANGGrammar(abnf string) (*yangGrammar, error) {
	g := &yangGrammar{
		rules:       map[string]abnfExpr{},
		productions: map[string]*statementProduction{},
		keywords:    map[string][]*statementProduction{},
		reaches:     map[string]bool{},
	}
	text := abnfWithoutComments(abnf)
	heads := abnfRuleHead.FindAllStringSubmatchIndex(text, -1)
	for i, head := range heads {
		bodyEnd := len(text)
		if i+1 < len(heads) {
			bodyEnd = heads[i+1][0]
		}
		name := text[head[2]:head[3]]
		body, err := parseABNFBody(text[head[1]:bodyEnd])
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", name, err)
		}
		g.rules[name] = body
	}
	if err := g.buildProductions(); err != nil {
		return nil, err
	}
	g.buildReaches()
	for _, production := range g.productions {
		production.block.resolve(g)
	}
	extension, err := g.extensionBlock()
	if err != nil {
		return nil, err
	}
	g.extension = extension
	return g, nil
}

// buildProductions reads every statement rule of g.rules into a production
// with its keyword, argument rule, block form and block content.
func (g *yangGrammar) buildProductions() error {
	for name, body := range g.rules {
		if !strings.HasSuffix(name, "-stmt") {
			continue
		}
		items := sequenceItems(body)
		if len(items) > 0 && isRef(items[0], "optsep") {
			items = items[1:]
		}
		if len(items) == 0 {
			continue
		}
		head, isHeadRef := items[0].(abnfRef)
		if !isHeadRef {
			continue
		}
		base, isKeyword := strings.CutSuffix(head.name, "-keyword")
		if !isKeyword {
			continue
		}
		keyword, err := g.keywordSpelling(base)
		if err != nil {
			return fmt.Errorf("statement rule %s: %w", name, err)
		}
		production := &statementProduction{rule: name, keyword: keyword}
		rest := items[1:]
		if len(rest) > 1 && isRef(rest[0], "sep") {
			argument, isArgumentRef := rest[1].(abnfRef)
			if !isArgumentRef {
				return fmt.Errorf("statement rule %s: the argument after sep names no rule", name)
			}
			production.argument = argument.name
			rest = rest[2:]
		}
		form, content, err := abnfBlockOf(rest)
		if err != nil {
			return fmt.Errorf("statement rule %s: %w", name, err)
		}
		production.form = form
		production.block = &statementBlock{owner: name, content: content}
		g.productions[name] = production
		g.keywords[keyword] = append(g.keywords[keyword], production)
	}
	if len(g.productions) == 0 {
		return errors.New("no statement rule")
	}
	for _, productions := range g.keywords {
		slices.SortFunc(productions, func(a, b *statementProduction) int { return strings.Compare(a.rule, b.rule) })
	}
	return nil
}

// keywordSpelling answers the keyword the rule "<base>-keyword" spells:
// "<base>-keyword = %s"<spelling>"".
func (g *yangGrammar) keywordSpelling(base string) (string, error) {
	body, defined := g.rules[base+"-keyword"]
	if !defined {
		return "", fmt.Errorf("%s-keyword is never defined", base)
	}
	terminal, isTerminal := body.(abnfTerminal)
	if !isTerminal {
		return "", fmt.Errorf("%s-keyword is not one literal", base)
	}
	spelling, isCaseSensitive := strings.CutPrefix(terminal.text, `%s"`)
	if !isCaseSensitive {
		return "", fmt.Errorf("%s-keyword is not a case-sensitive string", base)
	}
	return strings.TrimSuffix(spelling, `"`), nil
}

// extensionBlock answers the block of an extension usage, read from the
// unknown-statement rule the way a statement rule's block is.
func (g *yangGrammar) extensionBlock() (*statementBlock, error) {
	body, defined := g.rules[extensionRule]
	if !defined {
		return nil, fmt.Errorf("no %s rule", extensionRule)
	}
	_, content, err := abnfBlockOf(sequenceItems(body))
	if err != nil {
		return nil, fmt.Errorf("rule %s: %w", extensionRule, err)
	}
	block := &statementBlock{owner: extensionRule, content: content}
	block.resolve(g)
	return block, nil
}

// abnfBlockOf answers how the statement rule whose items follow the
// argument closes, and the content of its block. The Section 14 statement
// rules close in one of three ways: "stmtend", whose block holds no YANG
// statement; "(";" / "{" ... "}")", a block that may be omitted; and a bare
// "{" ... "}", a block that must be given.
func abnfBlockOf(items []abnfExpr) (blockForm, abnfExpr, error) {
	for i, item := range items {
		if isRef(item, "stmtend") {
			return blockOptional, abnfSequence{}, nil
		}
		if isTerminal(item, `"{"`) {
			return blockRequired, abnfSequence{items: bracedItems(items[i+1:])}, nil
		}
		alternation, isAlternation := item.(abnfAlternation)
		if !isAlternation {
			continue
		}
		if len(alternation.choices) != 2 {
			continue
		}
		if !isTerminal(alternation.choices[0], `";"`) {
			continue
		}
		braced := sequenceItems(alternation.choices[1])
		if len(braced) == 0 {
			continue
		}
		if !isTerminal(braced[0], `"{"`) {
			continue
		}
		return blockOptional, abnfSequence{items: bracedItems(braced[1:])}, nil
	}
	return blockFormUnspecified, nil, errors.New("no block form: neither stmtend, nor a \"{\"")
}

// bracedItems answers items up to the closing "}".
func bracedItems(items []abnfExpr) []abnfExpr {
	for i, item := range items {
		if isTerminal(item, `"}"`) {
			return items[:i]
		}
	}
	return items
}

// sequenceItems answers the items of expr when it is a sequence, and expr
// alone otherwise.
func sequenceItems(expr abnfExpr) []abnfExpr {
	sequence, isSequence := expr.(abnfSequence)
	if isSequence {
		return sequence.items
	}
	return []abnfExpr{expr}
}

// isRef reports whether expr names the rule name.
func isRef(expr abnfExpr, name string) bool {
	ref, isReference := expr.(abnfRef)
	return isReference && ref.name == name
}

// isTerminal reports whether expr is the literal text, quotes included.
func isTerminal(expr abnfExpr, text string) bool {
	terminal, isLiteral := expr.(abnfTerminal)
	return isLiteral && terminal.text == text
}

// buildReaches records, for every rule that is no statement production,
// whether its body names one, stopping at unknown-statement: an extension
// usage is the stmtsep every block admits, and it is never counted. A rule
// still being walked answers false, which is exact: a cycle among rules that
// are no statement production, as if-feature-expr is, names no statement, and
// a cycle through a statement production stops at it.
func (g *yangGrammar) buildReaches() {
	walking := map[string]bool{}
	for name := range g.rules {
		g.ruleReaches(name, walking)
	}
}

// ruleReaches answers and records whether rule name reaches a statement
// production. The recursion follows rule references in the embedded grammar,
// so its depth is bounded by the grammar's rule count.
func (g *yangGrammar) ruleReaches(name string, walking map[string]bool) bool {
	if _, isProduction := g.productions[name]; isProduction {
		return true
	}
	if name == extensionRule {
		return false
	}
	if reached, known := g.reaches[name]; known {
		return reached
	}
	if walking[name] {
		return false
	}
	body, defined := g.rules[name]
	if !defined {
		return false
	}
	walking[name] = true
	reached := g.exprReaches(body, walking)
	walking[name] = false
	g.reaches[name] = reached
	return reached
}

// exprReaches answers whether expr names a statement production.
func (g *yangGrammar) exprReaches(expr abnfExpr, walking map[string]bool) bool {
	switch e := expr.(type) {
	case abnfRef:
		return g.ruleReaches(e.name, walking)
	case abnfTerminal:
		return false
	case abnfSequence:
		return slices.ContainsFunc(e.items, func(item abnfExpr) bool { return g.exprReaches(item, walking) })
	case abnfAlternation:
		return slices.ContainsFunc(e.choices, func(choice abnfExpr) bool { return g.exprReaches(choice, walking) })
	case abnfRepeat:
		return g.exprReaches(e.item, walking)
	}
	panic("BUG: abnfExpr variant without a case")
}

// resolve fills the productions b's content reaches. The walk is an explicit
// stack over rule names, each expanded once, so it ends within the
// grammar's rule count.
func (b *statementBlock) resolve(g *yangGrammar) {
	b.admitted = map[string][]*statementProduction{}
	b.slots = map[*statementProduction]int{}
	seen := map[string]bool{}
	pending := []abnfExpr{b.content}
	for len(pending) > 0 {
		expr := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		switch e := expr.(type) {
		case abnfRef:
			if production, isProduction := g.productions[e.name]; isProduction {
				b.admit(production)
				continue
			}
			if !g.reaches[e.name] {
				continue
			}
			if seen[e.name] {
				continue
			}
			seen[e.name] = true
			pending = append(pending, g.rules[e.name])
		case abnfTerminal:
		case abnfSequence:
			pending = append(pending, e.items...)
		case abnfAlternation:
			pending = append(pending, e.choices...)
		case abnfRepeat:
			pending = append(pending, e.item)
		}
	}
	b.emptyAccepted = g.matches(b, make(childCounts, len(b.slots)))
}

// admit adds production to b once.
func (b *statementBlock) admit(production *statementProduction) {
	if _, admitted := b.slots[production]; admitted {
		return
	}
	b.slots[production] = len(b.slots)
	b.admitted[production.keyword] = append(b.admitted[production.keyword], production)
}

// childCounts is how many substatements of one statement resolved to each
// production its block admits, indexed by statementBlock.slots.
type childCounts []int

// matches reports whether counts, read in any order, match b's content.
//
// RFC 7950 Section 14 sets the substatements of a block under the comment
// ";; these stmts can appear in any order", so the content is matched as a
// multiset: each repetition takes its count of substatements, wherever they
// stand. The match carries every count vector the content can leave; it is
// exact because a vector is a multiset of the substatements not yet taken.
func (g *yangGrammar) matches(b *statementBlock, counts childCounts) bool {
	left := g.consume(b, b.content, []childCounts{counts})
	return slices.ContainsFunc(left, func(state childCounts) bool {
		return !slices.ContainsFunc(state, func(n int) bool { return n != 0 })
	})
}

// consume answers every count vector expr can leave from one of states. The
// recursion follows expr and the rules it names; a rule is expanded only when
// it reaches a statement production, so the depth is bounded by the
// grammar's nesting, and the states are bounded by the substatement counts.
func (g *yangGrammar) consume(b *statementBlock, expr abnfExpr, states []childCounts) []childCounts {
	switch e := expr.(type) {
	case abnfRef:
		if production, isProduction := g.productions[e.name]; isProduction {
			return takeOne(states, b.slots[production])
		}
		if !g.reaches[e.name] {
			return states
		}
		return g.consume(b, g.rules[e.name], states)
	case abnfTerminal:
		return states
	case abnfSequence:
		for _, item := range e.items {
			if len(states) == 0 {
				return nil
			}
			states = g.consume(b, item, states)
		}
		return states
	case abnfAlternation:
		var left []childCounts
		for _, choice := range e.choices {
			left = appendNewStates(left, g.consume(b, choice, states))
		}
		return left
	case abnfRepeat:
		return g.consumeRepeat(b, e, states)
	}
	panic("BUG: abnfExpr variant without a case")
}

// consumeRepeat answers every count vector "<min>*<max>item" can leave from
// one of states. Past min, a vector already reached is not walked again, so
// an unbounded repetition of an item that can take nothing still ends: the
// vectors only shrink, and there are finitely many.
func (g *yangGrammar) consumeRepeat(b *statementBlock, e abnfRepeat, states []childCounts) []childCounts {
	var left []childCounts
	if e.min == 0 {
		left = appendNewStates(left, states)
	}
	current := states
	for times := 1; e.max == repeatUnbounded || times <= e.max; times++ {
		next := g.consume(b, e.item, current)
		if times < e.min {
			current = appendNewStates(nil, next)
			if len(current) == 0 {
				return nil
			}
			continue
		}
		fresh := make([]childCounts, 0, len(next))
		for _, state := range next {
			if !containsState(left, state) && !containsState(fresh, state) {
				fresh = append(fresh, state)
			}
		}
		if len(fresh) == 0 {
			break
		}
		left = append(left, fresh...)
		current = fresh
	}
	return left
}

// takeOne answers each of states with one substatement of slot taken, for
// the states that hold one.
func takeOne(states []childCounts, slot int) []childCounts {
	left := make([]childCounts, 0, len(states))
	for _, state := range states {
		if state[slot] == 0 {
			continue
		}
		taken := slices.Clone(state)
		taken[slot]--
		left = appendNewStates(left, []childCounts{taken})
	}
	return left
}

// appendNewStates appends to states each of more not already held.
func appendNewStates(states, more []childCounts) []childCounts {
	for _, state := range more {
		if !containsState(states, state) {
			states = append(states, state)
		}
	}
	return states
}

// containsState reports whether states holds state.
func containsState(states []childCounts, state childCounts) bool {
	return slices.ContainsFunc(states, func(held childCounts) bool { return slices.Equal(held, state) })
}

// occurrences answers the fewest and the most substatements of production
// expr admits, the most being repeatUnbounded when there is no bound. It
// reads each repetition on its own, so it bounds the count; whether the
// counts of several productions fit one alternative together is matches'.
func (g *yangGrammar) occurrences(expr abnfExpr, production *statementProduction) (low, high int) {
	switch e := expr.(type) {
	case abnfRef:
		if e.name == production.rule {
			return 1, 1
		}
		if _, isProduction := g.productions[e.name]; isProduction {
			return 0, 0
		}
		if !g.reaches[e.name] {
			return 0, 0
		}
		return g.occurrences(g.rules[e.name], production)
	case abnfTerminal:
		return 0, 0
	case abnfSequence:
		for _, item := range e.items {
			itemLow, itemHigh := g.occurrences(item, production)
			low += itemLow
			high = addBound(high, itemHigh)
		}
		return low, high
	case abnfAlternation:
		low = -1
		for _, choice := range e.choices {
			choiceLow, choiceHigh := g.occurrences(choice, production)
			if low < 0 || choiceLow < low {
				low = choiceLow
			}
			high = maxBound(high, choiceHigh)
		}
		return max(low, 0), high
	case abnfRepeat:
		itemLow, itemHigh := g.occurrences(e.item, production)
		low = itemLow * e.min
		if itemHigh == 0 {
			return low, 0
		}
		if itemHigh == repeatUnbounded || e.max == repeatUnbounded {
			return low, repeatUnbounded
		}
		return low, itemHigh * e.max
	}
	panic("BUG: abnfExpr variant without a case")
}

// addBound adds two upper bounds, either of which may be repeatUnbounded.
func addBound(a, b int) int {
	if a == repeatUnbounded || b == repeatUnbounded {
		return repeatUnbounded
	}
	return a + b
}

// maxBound answers the larger of two upper bounds, either of which may be
// repeatUnbounded.
func maxBound(a, b int) int {
	if a == repeatUnbounded || b == repeatUnbounded {
		return repeatUnbounded
	}
	return max(a, b)
}

// abnfWithoutComments answers abnf with each ";" comment removed, keeping
// the ";" a quoted literal or a "< prose >" holds.
func abnfWithoutComments(abnf string) string {
	lines := strings.Split(abnf, "\n")
	for i, line := range lines {
		quoted, prose := false, false
		for j := range len(line) {
			c := line[j]
			if c == '"' && !prose {
				quoted = !quoted
			}
			if c == '<' && !quoted {
				prose = true
			}
			if c == '>' && !quoted {
				prose = false
			}
			if c == ';' && !quoted && !prose {
				lines[i] = line[:j]
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

// abnfToken is one token of a rule body: a rule name, a terminal, one of the
// punctuation bytes "/()[]", or a repetition prefix.
type abnfToken struct {
	name        string
	terminal    string
	punctuation byte
	repeat      bool
	min         int
	max         int
}

// abnfName matches one rule name.
var abnfName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*`)

// abnfNumericTerminal matches a numeric terminal such as %x41-5A or %x0D.0A.
var abnfNumericTerminal = regexp.MustCompile(`^%[xdb][0-9A-Fa-f.-]+`)

// abnfRepetition matches a repetition prefix: "*", "1*", "0*1", or "4".
var abnfRepetition = regexp.MustCompile(`^(\d*)(\*)(\d*)|^(\d+)`)

// tokenizeABNF splits one rule body into tokens.
func tokenizeABNF(body string) ([]abnfToken, error) {
	var tokens []abnfToken
	rest := body
	for {
		rest = strings.TrimLeft(rest, " \t\r\n")
		if rest == "" {
			return tokens, nil
		}
		token, length, err := nextABNFToken(rest)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
		rest = rest[length:]
	}
}

// nextABNFToken reads the token rest opens with, and its length.
func nextABNFToken(rest string) (abnfToken, int, error) {
	c := rest[0]
	if strings.IndexByte("/()[]", c) >= 0 {
		return abnfToken{punctuation: c}, 1, nil
	}
	if c == '"' {
		closing := strings.IndexByte(rest[1:], '"')
		if closing < 0 {
			return abnfToken{}, 0, fmt.Errorf("unclosed literal %q", rest)
		}
		return abnfToken{terminal: rest[:closing+2]}, closing + 2, nil
	}
	if strings.HasPrefix(rest, `%s"`) || strings.HasPrefix(rest, `%i"`) {
		closing := strings.IndexByte(rest[3:], '"')
		if closing < 0 {
			return abnfToken{}, 0, fmt.Errorf("unclosed literal %q", rest)
		}
		return abnfToken{terminal: rest[:closing+4]}, closing + 4, nil
	}
	if numeric := abnfNumericTerminal.FindString(rest); numeric != "" {
		return abnfToken{terminal: numeric}, len(numeric), nil
	}
	if c == '<' {
		closing := strings.IndexByte(rest, '>')
		if closing < 0 {
			return abnfToken{}, 0, fmt.Errorf("unclosed prose %q", rest)
		}
		return abnfToken{terminal: rest[:closing+1]}, closing + 1, nil
	}
	if m := abnfRepetition.FindStringSubmatch(rest); m != nil {
		return repetitionToken(m)
	}
	if name := abnfName.FindString(rest); name != "" {
		return abnfToken{name: name}, len(name), nil
	}
	return abnfToken{}, 0, fmt.Errorf("unexpected %q", rest)
}

// repetitionToken answers the token of an abnfRepetition match m.
func repetitionToken(m []string) (abnfToken, int, error) {
	if m[4] != "" {
		exact, err := strconv.Atoi(m[4])
		if err != nil {
			return abnfToken{}, 0, err
		}
		return abnfToken{repeat: true, min: exact, max: exact}, len(m[0]), nil
	}
	token := abnfToken{repeat: true, max: repeatUnbounded}
	if m[1] != "" {
		low, err := strconv.Atoi(m[1])
		if err != nil {
			return abnfToken{}, 0, err
		}
		token.min = low
	}
	if m[3] != "" {
		high, err := strconv.Atoi(m[3])
		if err != nil {
			return abnfToken{}, 0, err
		}
		token.max = high
	}
	return token, len(m[0]), nil
}

// parseABNFBody parses one rule body: "alternation = concatenation *("/"
// concatenation)" (RFC 5234 Section 4).
func parseABNFBody(body string) (abnfExpr, error) {
	tokens, err := tokenizeABNF(body)
	if err != nil {
		return nil, err
	}
	parser := abnfParser{tokens: tokens}
	expr, err := parser.alternation()
	if err != nil {
		return nil, err
	}
	if parser.at != len(tokens) {
		return nil, fmt.Errorf("unexpected token %d of %d", parser.at, len(tokens))
	}
	return expr, nil
}

// abnfParser is a recursive-descent reader of one rule body. It reads the
// grammar embedded in the binary, never input, and its depth is the
// bracket nesting of one RFC rule.
type abnfParser struct {
	tokens []abnfToken
	at     int
}

// alternation reads concatenations separated by "/".
func (p *abnfParser) alternation() (abnfExpr, error) {
	first, err := p.concatenation()
	if err != nil {
		return nil, err
	}
	choices := []abnfExpr{first}
	for p.at < len(p.tokens) && p.tokens[p.at].punctuation == '/' {
		p.at++
		choice, err := p.concatenation()
		if err != nil {
			return nil, err
		}
		choices = append(choices, choice)
	}
	if len(choices) == 1 {
		return first, nil
	}
	return abnfAlternation{choices: choices}, nil
}

// concatenation reads repetitions until "/", ")", "]" or the end.
func (p *abnfParser) concatenation() (abnfExpr, error) {
	var items []abnfExpr
	for p.at < len(p.tokens) {
		punctuation := p.tokens[p.at].punctuation
		if punctuation == '/' || punctuation == ')' || punctuation == ']' {
			break
		}
		item, err := p.repetition()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil, errors.New("empty concatenation")
	}
	if len(items) == 1 {
		return items[0], nil
	}
	return abnfSequence{items: items}, nil
}

// repetition reads "[repeat] element".
func (p *abnfParser) repetition() (abnfExpr, error) {
	token := p.tokens[p.at]
	if !token.repeat {
		return p.element()
	}
	p.at++
	if p.at == len(p.tokens) {
		return nil, errors.New("repetition with no element")
	}
	item, err := p.element()
	if err != nil {
		return nil, err
	}
	return abnfRepeat{min: token.min, max: token.max, item: item}, nil
}

// element reads a rule name, a terminal, "(" alternation ")", or "["
// alternation "]", which is "0*1".
func (p *abnfParser) element() (abnfExpr, error) {
	token := p.tokens[p.at]
	p.at++
	if token.name != "" {
		return abnfRef{name: token.name}, nil
	}
	if token.terminal != "" {
		return abnfTerminal{text: token.terminal}, nil
	}
	closing := byte(')')
	if token.punctuation == '[' {
		closing = ']'
	} else if token.punctuation != '(' {
		return nil, fmt.Errorf("unexpected %q", token.punctuation)
	}
	inner, err := p.alternation()
	if err != nil {
		return nil, err
	}
	if p.at == len(p.tokens) || p.tokens[p.at].punctuation != closing {
		return nil, fmt.Errorf("no closing %q", closing)
	}
	p.at++
	if closing == ']' {
		return abnfRepeat{min: 0, max: 1, item: inner}, nil
	}
	return inner, nil
}
