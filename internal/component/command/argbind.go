// Design: docs/architecture/api/commands.md -- YANG-typed argument validation
// Related: argvalidate.go -- one token judged against one definition
// Related: local_data.go -- the in-process route that calls ValidateArgs
//
// argbind.go binds a command's argument tokens to its YANG-declared
// definitions and refuses the call when a token or a missing argument breaks
// them. It is the one validator every dispatch route calls: the daemon
// dispatcher (plugin/server Dispatch) and the in-process local-data route
// (ServeLocal). A route that skipped it let a value past every declared
// length, pattern and range.

package command

import (
	"fmt"
	"maps"
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ValidatedArgs is a command's argument tokens after ValidateArgs judged them
// against the definitions the model declares for the command's path.
//
// Only a successful ValidateArgs builds a non-zero value, because its fields
// are private: a route outside this package that holds one has called the
// validator. The zero value is legitimate and means "no tokens, no binding";
// it is what ValidateArgs returns for a nil token list against no
// definitions, and a route that hands a handler a zero literal instead of calling the
// validator is visible as that literal.
//
// The value owns its token slice: ValidateArgs copies its input, and Tokens
// answers a copy, so neither the route nor a handler can change what was
// judged after the judgment. Safe for concurrent use: it is never written
// after ValidateArgs returns it.
type ValidatedArgs struct {
	tokens []string
	// bound and boundValue are the lone spare positional token ValidateArgs
	// bound to a leaf. bound is empty when it bound none.
	bound      string
	boundValue string
	// keyed holds the value each leaf took by name: typed after its keyword
	// (`name site-b`) or bound by the token matcher. A leaf filled
	// positionally is not in it; bound answers that one. It is nil when no
	// leaf took a value by name, which keeps the zero value an empty call's
	// answer; reading a nil map answers not-found. A rest-of-line leaf is
	// always in it, however it started.
	keyed map[string]string
	// positional holds, in the order typed, every token no keyword consumed
	// and no rest-of-line leaf took.
	positional []string
}

// Tokens answers a copy of the judged tokens, in the order they were typed. A
// handler that writes into the copy changes nothing the value holds.
func (v ValidatedArgs) Tokens() []string {
	return slices.Clone(v.tokens)
}

// Positional answers the value a LONE spare positional token filled for leaf.
// found is false when the call left several spare tokens, none, or bound the
// one it left to a different leaf: which token is the value is then a guess,
// and the caller must not make it.
func (v ValidatedArgs) Positional(leaf string) (value string, found bool) {
	if v.bound == "" {
		return "", false
	}
	if v.bound != leaf {
		return "", false
	}
	return v.boundValue, true
}

// Keyed answers the value leaf took by name: after its own keyword, through
// the token matcher, or as a rest-of-line leaf. A leaf filled positionally is
// not answered here; PositionalTokens holds its token.
func (v ValidatedArgs) Keyed(leaf string) (value string, found bool) {
	value, found = v.keyed[leaf]
	return value, found
}

// PositionalTokens answers a copy of the tokens no keyword consumed and no
// rest-of-line leaf took, in the order they were typed. A command whose
// leaves are typed in a fixed order reads them from here; which leaf each
// token fills is the command's own grammar, not a guess this package makes.
func (v ValidatedArgs) PositionalTokens() []string {
	return slices.Clone(v.positional)
}

// Value answers the value leaf took, however the operator spelled it: after
// the leaf's own keyword (`peer name site-b`), or as the lone spare positional
// token (`peer site-b`). found is false when no token filled leaf, or when it
// was one of several positional tokens, with the fence Positional states.
// found is also false for a flag leaf: a flag is its keyword alone and carries
// no value, so whether it was typed is not something Value answers.
func (v ValidatedArgs) Value(leaf string) (value string, found bool) {
	if value, found = v.keyed[leaf]; found {
		return value, true
	}
	return v.Positional(leaf)
}

// MissingArgumentError is ValidateArgs's refusal of a call that left a
// mandatory definition unfilled.
//
// It carries the lone positional binding the call DID make, because the daemon
// dispatcher chooses between two refusals on it: a command whose selector
// arrived positionally is told which other leaf is missing, not that it
// "requires a selector". The binding answers only that choice; a refused call
// yields no ValidatedArgs, so no handler can run on it.
type MissingArgumentError struct {
	// Leaf is the first mandatory definition, in declaration order, that no
	// token filled.
	Leaf       string
	bound      string
	boundValue string
}

// Error answers the refusal text every route prints.
func (e *MissingArgumentError) Error() string {
	var tb textbuf.Buffer
	return tb.Str("required argument missing: ").Str(e.Leaf).String()
}

// Positional answers the value a lone spare positional token filled for leaf
// in the refused call, with the same fences as ValidatedArgs.Positional.
func (e *MissingArgumentError) Positional(leaf string) (value string, found bool) {
	if e.bound == "" {
		return "", false
	}
	if e.bound != leaf {
		return "", false
	}
	return e.boundValue, true
}

// ValidateArgs implements two-phase validation of command arguments
// against YANG-declared ArgDefs.
//
// Phase 1 (keyword extraction): scan args for tokens matching ArgDef leaf names;
// when found, the next token is validated as that leaf's typed value.
// Phase 2 (positional matching): remaining args are offered to the unmatched
// ArgDefs. Unmatched args pass through to the handler.
// Phase 3 (mandatory check): ArgDefs with Mandatory=true must have been matched.
//
// A rest-of-line definition (ze:rest-of-line) is settled before phase 1
// (restStart): from where it starts, every token is its value, joined by one
// space, and no phase reads those tokens again. A free text can then hold a
// word that names another leaf (`notify 6 2 the code is wrong`) without that
// word being taken for its keyword.
//
// Phase 3 answers BEFORE phase 2's refusal whenever the call left more tokens
// over than it left definitions open. The refusals are ordered, not the phases:
// a missing mandatory argument is what the model itself says is wrong with the
// call, while a token no definition accepts is a bad value only if the
// dispatcher can say which definition it was typed for.
//
// It answers the ValidatedArgs every route hands a handler: a copy of args,
// and the leaf a LONE spare positional token filled (ValidatedArgs.Positional).
// The daemon dispatcher needs that binding for the terminal-noun selector shape
// (see the fences there), and this is the only place a positional token is
// bound to a leaf, so reporting it is cheaper and safer than a second matcher
// that would drift. "Lone" is the point: when a command leaves several tokens
// unconsumed, which of them is the value is a guess, and the caller must not
// make it. A call that left a mandatory definition unfilled is refused with a
// *MissingArgumentError, which carries that binding for the same reason.
//
// No definitions is a legitimate answer from the model, not a skipped check:
// every phase iterates the definitions, so the tokens pass through unchanged.
// Every route calls this function, including for a path that declares none.
//
// A definition no constructor built, the zero ArgDef among them, is refused
// before any token is read (ErrArgDef): it is a Ze defect, and accepting it as
// an unrestricted string would fail open.
func ValidateArgs(args []string, defs []ArgDef, preMatched map[string]string) (ValidatedArgs, error) {
	var rest *ArgDef
	for i := range defs {
		if !defs[i].constructed {
			return ValidatedArgs{}, fmt.Errorf("%w: argument %d (%q) was not built by a constructor", ErrArgDef, i, defs[i].name)
		}
		if !defs[i].rest {
			continue
		}
		if defs[i].kind != ArgString {
			return ValidatedArgs{}, fmt.Errorf("%w: argument %q: only a string leaf can take the rest of the line", ErrArgDef, defs[i].name)
		}
		if rest != nil {
			return ValidatedArgs{}, fmt.Errorf("%w: arguments %q and %q both take the rest of the line", ErrArgDef, rest.name, defs[i].name)
		}
		rest = &defs[i]
	}
	matched := make(map[string]bool, len(preMatched))
	defByName := make(map[string]*ArgDef, len(defs))
	for i := range defs {
		defByName[defs[i].name] = &defs[i]
	}
	// keyed stays nil until a leaf takes a value by name (see ValidatedArgs).
	var keyed map[string]string
	if len(preMatched) > 0 {
		keyed = make(map[string]string, len(preMatched))
	}
	for name, value := range preMatched {
		matched[name] = true
		keyed[name] = value
	}
	// A value the token matcher bound by keyword or by anchor is judged here,
	// with every other argument, and not by the matcher: a matcher that refused
	// it answered "unknown command" for a command the operator typed correctly
	// with one bad value. Definitions are walked in order, so the refusal named
	// is the same on every run.
	for i := range defs {
		value, bound := preMatched[defs[i].name]
		if !bound {
			continue
		}
		if err := ValidateArgString(value, &defs[i]); err != nil {
			return ValidatedArgs{}, err
		}
	}

	// The rest-of-line leaf takes its words before any keyword is read, so the
	// phases below see only the tokens in front of it.
	all := args
	if rest != nil && !matched[rest.name] {
		start, from := restStart(args, defs, defByName, matched, rest)
		if start >= 0 {
			if from >= len(args) {
				return ValidatedArgs{}, missingValueError(rest.name)
			}
			value := textbuf.Join(args[from:], " ")
			if err := ValidateArgString(value, rest); err != nil {
				return ValidatedArgs{}, err
			}
			matched[rest.name] = true
			if keyed == nil {
				keyed = make(map[string]string)
			}
			keyed[rest.name] = value
			args = args[:start]
		}
	}
	consumed := make([]bool, len(args))

	// Phase 1: keyword-value extraction.
	for i := 0; i < len(args); i++ {
		def, ok := defByName[args[i]]
		if !ok {
			continue
		}
		if matched[def.name] {
			return ValidatedArgs{}, fmt.Errorf("duplicate keyword %q", args[i])
		}
		consumed[i] = true
		if def.kind == ArgFlag {
			// A flag is the keyword alone; the next token is its own argument.
			matched[def.name] = true
			continue
		}
		if i+1 >= len(args) {
			return ValidatedArgs{}, missingValueError(def.name)
		}
		i++
		consumed[i] = true
		if err := ValidateArgString(args[i], def); err != nil {
			return ValidatedArgs{}, err
		}
		matched[def.name] = true
		if keyed == nil {
			keyed = make(map[string]string)
		}
		keyed[def.name] = args[i]
	}

	spare := 0
	for i := range consumed {
		if !consumed[i] {
			spare++
		}
	}

	// Phase 2: positional matching for unconsumed args. A token no open
	// definition accepts is kept rather than refused here, because whether it is
	// a bad value or a keyword the handler reads is a question the definitions
	// alone cannot answer, and a later token can still fill a definition this
	// one could not.
	var bound, boundValue string
	unplaced := make([]string, 0, len(args))
	positional := make([]string, 0, spare)
	for i, arg := range args {
		if consumed[i] {
			continue
		}
		positional = append(positional, arg)
		def := positionalDef(arg, defs, matched)
		if def == nil {
			unplaced = append(unplaced, arg)
			continue
		}
		matched[def.name] = true
		if spare == 1 {
			bound, boundValue = def.name, arg
		}
	}

	// A token that filled no definition is a bad VALUE only when every such
	// token can be attributed to a definition of its own: as many tokens left
	// over as definitions still open, or fewer. More tokens than open
	// definitions means at least one of them is a value for nothing, so it is
	// the keyword half of a group the command's own grammar declares and this
	// validator does not hold (`update <hex>` on `show policy test peer`). The
	// dispatcher then has no ground to name any token as the fault, and the
	// missing mandatory argument below is what is certainly wrong with the call.
	open := unmatchedDefCount(defs, matched)
	if len(unplaced) > 0 && open > 0 && len(unplaced) <= open {
		return ValidatedArgs{}, positionalError(unplaced[0], defs, matched)
	}

	// Phase 3: mandatory check.
	for i := range defs {
		if defs[i].mandatory && !matched[defs[i].name] {
			return ValidatedArgs{}, &MissingArgumentError{Leaf: defs[i].name, bound: bound, boundValue: boundValue}
		}
	}

	// Every mandatory definition is filled, so a token left over is the only
	// fault the call has, and naming it is the whole answer.
	if len(unplaced) > 0 && open > 0 {
		return ValidatedArgs{}, positionalError(unplaced[0], defs, matched)
	}

	return ValidatedArgs{tokens: slices.Clone(all), bound: bound, boundValue: boundValue, keyed: keyed, positional: positional}, nil
}

// restStart answers where the rest-of-line leaf rest begins on args: start is
// the index of the first token that belongs to it, its keyword or its first
// word, and from the index its value begins at. start is -1 when no token
// belongs to it.
//
// It walks the tokens in the order typed, as the operator wrote them. The
// keyword of a leaf still open takes the next token as its value (a flag takes
// none), and rest's own keyword starts rest. Any other token fills a
// mandatory leaf still open while one is; once every mandatory leaf is filled,
// it is the first word of rest. A word naming a leaf already filled is
// therefore a word of the text, which is the point: `notify 6 2 code red`
// sends the text "code red". A token no open mandatory leaf accepts is left
// for the phases, which refuse it. The loop is bounded by len(args).
func restStart(args []string, defs []ArgDef, defByName map[string]*ArgDef, matched map[string]bool, rest *ArgDef) (start, from int) {
	filled := maps.Clone(matched)
	if filled == nil {
		filled = make(map[string]bool, len(defs))
	}
	for i := 0; i < len(args); i++ {
		if args[i] == rest.name {
			return i, i + 1
		}
		def, isKeyword := defByName[args[i]]
		if isKeyword && !filled[def.name] {
			filled[def.name] = true
			if def.kind != ArgFlag {
				i++
			}
			continue
		}
		if !mandatoryOpen(defs, filled) {
			return i, i
		}
		def = positionalDef(args[i], defs, filled)
		if def != nil && def.mandatory {
			filled[def.name] = true
		}
	}
	return -1, -1
}

// missingValueError answers a keyword typed last, with nothing after it to
// read as its value, and names the form that fixes it.
func missingValueError(keyword string) error {
	return fmt.Errorf("%s requires a value: it is the last word, so nothing follows it to read as its value; write the value right after the keyword, as in '%s <%s>'", keyword, keyword, keyword)
}

// mandatoryOpen reports whether a mandatory definition other than a
// rest-of-line one is still waiting for a value.
func mandatoryOpen(defs []ArgDef, filled map[string]bool) bool {
	for i := range defs {
		if defs[i].mandatory && !defs[i].rest && !filled[defs[i].name] {
			return true
		}
	}
	return false
}

// positionalDef picks the ArgDef a positional token fills, or nil when none
// accepts it.
//
// EVERY ArgKind is offered the token. The shipped loop tested only ArgEnum,
// ArgUnion and ArgString, which made a mandatory non-string leaf impossible to
// fill positionally: `show tcp-check <host> <port>` skipped the uint16 `port`,
// bound the numeric token to the next STRING leaf, and Phase 3 then rejected a
// fully-formed command with "required argument missing: port".
//
// Mandatory defs are offered the token FIRST. An optional leaf that merely
// accepts the same lexical shape (a pattern-less string accepts anything) would
// otherwise swallow the value a required leaf needed, turning a complete
// command into "required argument missing". Preferring the required leaf can
// only ever fill more of them, never fewer, so this direction cannot invent a
// new failure.
//
// WITHIN a tier the token goes to the definition that constrains it most, and
// a tie goes to the lower name. Slice order decides nothing, which is what lets
// the definitions be reordered for display: `show system sockets 8080` reached
// the port leaf only because the alphabet put "port" before "state", and
// "state" is a pattern-less string that would have accepted it silently.
func positionalDef(arg string, defs []ArgDef, matched map[string]bool) *ArgDef {
	for _, wantMandatory := range [...]bool{true, false} {
		var best *ArgDef
		bestRank := ConstraintUnspecified
		for i := range defs {
			def := &defs[i]
			if matched[def.name] {
				continue
			}
			// restStart alone places a rest-of-line leaf.
			if def.rest {
				continue
			}
			if def.mandatory != wantMandatory {
				continue
			}
			if ValidateArgString(arg, def) != nil {
				continue
			}
			rank := Constraint(def)
			if best == nil {
				best, bestRank = def, rank
				continue
			}
			if rank < bestRank {
				best, bestRank = def, rank
				continue
			}
			if rank == bestRank && def.name < best.name {
				best = def
			}
		}
		if best != nil {
			return best
		}
	}
	return nil
}

// unmatchedDefCount reports how many ArgDefs are still waiting for a value. A
// flag never waits for one, and neither does a rest-of-line leaf, which
// restStart has already placed or found absent.
func unmatchedDefCount(defs []ArgDef, matched map[string]bool) int {
	n := 0
	for i := range defs {
		if !matched[defs[i].name] && defs[i].kind != ArgFlag && !defs[i].rest {
			n++
		}
	}
	return n
}

// positionalError builds an error for a token no OPEN definition accepts. Its
// caller has already found at least one definition still waiting for a value,
// so the list below is never empty, and it has already established that the
// token can be attributed to one: the caller counts the tokens it could not
// place against the definitions still open (ValidateArgs).
//
// A definition that is still open is the one the operator's token was meant
// for, so when exactly one is open the error is that definition's own refusal.
// `show route lookup <ip>` publishes a bare value and no keyword, and the
// keyword list answered "valid keywords: ip" for a value that simply was not an
// address: it named a word the grammar never asks anybody to type and dropped
// the reason the value was refused (plan/journal/guard-addition-drops-what-it-refuses.md).
//
// A definition already filled is named by neither branch. It cannot take this
// token, so offering it as a keyword is an answer the dispatcher would reject.
func positionalError(arg string, defs []ArgDef, matched map[string]bool) error {
	open := make([]*ArgDef, 0, len(defs))
	for i := range defs {
		if !matched[defs[i].name] && defs[i].kind != ArgFlag && !defs[i].rest {
			open = append(open, &defs[i])
		}
	}
	if len(open) == 1 {
		return ValidateArgString(arg, open[0])
	}
	for _, def := range open {
		if def.kind == ArgEnum {
			return ValidateArgString(arg, def)
		}
		if def.kind == ArgUnion {
			return ValidateArgString(arg, def)
		}
	}
	names := make([]string, 0, len(open))
	for _, def := range open {
		names = append(names, def.name)
	}
	return fmt.Errorf("unexpected argument %q, valid keywords: %s", arg, textbuf.Join(names, ", "))
}
