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
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ValidatedArgs is a command's argument tokens after ValidateArgs judged them
// against the definitions the model declares for the command's path.
//
// Only a successful ValidateArgs builds a non-zero value, because its fields
// are private: a route outside this package that holds one has called the
// validator. The zero value is legitimate and means "no tokens, no binding";
// it is what ValidateArgs returns for an empty call against no definitions,
// and a route that hands a handler a zero literal instead of calling the
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
	for i := range defs {
		if !defs[i].constructed {
			return ValidatedArgs{}, fmt.Errorf("%w: argument %d (%q) was not built by a constructor", ErrArgDef, i, defs[i].name)
		}
	}
	consumed := make([]bool, len(args))
	matched := make(map[string]bool, len(preMatched))
	defByName := make(map[string]*ArgDef, len(defs))
	for i := range defs {
		defByName[defs[i].name] = &defs[i]
	}
	for name := range preMatched {
		matched[name] = true
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
			return ValidatedArgs{}, fmt.Errorf("%s requires a value", args[i])
		}
		i++
		consumed[i] = true
		if err := ValidateArgString(args[i], def); err != nil {
			return ValidatedArgs{}, err
		}
		matched[def.name] = true
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
	for i, arg := range args {
		if consumed[i] {
			continue
		}
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

	return ValidatedArgs{tokens: slices.Clone(args), bound: bound, boundValue: boundValue}, nil
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
// flag never waits for one.
func unmatchedDefCount(defs []ArgDef, matched map[string]bool) int {
	n := 0
	for i := range defs {
		if !matched[defs[i].name] && defs[i].kind != ArgFlag {
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
		if !matched[defs[i].name] && defs[i].kind != ArgFlag {
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
