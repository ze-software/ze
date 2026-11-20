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
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/redact"
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
	// flags holds every flag leaf whose keyword was typed. It is nil when
	// none was, so the zero value answers no flag.
	flags map[string]bool
	// positional holds, in the order typed, every token no keyword consumed
	// and no rest-of-line leaf took.
	positional []string
	// route is the local-data route that called the handler holding this
	// value (route.go). ValidateArgs leaves it RouteUnspecified; only the
	// local-data routes in this package mark it.
	route Route
	// request is the context of the caller's request on the daemon route
	// (WithContext). It is nil on every other route, where the work
	// lives as long as the operator's own process (route.go, Context).
	request context.Context
	// pipeOperations is the declared list: the operations of the typed chain
	// that the command declares as its own, in chain order. It is nil when the
	// chain held none. Only the route that partitioned the chain
	// (partitionPipeChain) attaches it, through WithPipeOperations.
	pipeOperations []PipeOperation
}

// WithPipeOperations answers v carrying operations as its declared list. The
// route that partitioned the typed chain calls it once, after ValidateArgs, so
// the handler receives the operations it declares beside its arguments rather
// than as words rewritten into them. The list is copied.
func (v ValidatedArgs) WithPipeOperations(operations []PipeOperation) ValidatedArgs {
	v.pipeOperations = slices.Clone(operations)
	return v
}

// RefusePipeOperationWords refuses an argument word that is one of the pipe
// operations path declares (RegisterPipeFilters), before any argument is bound:
// such an operation is typed after a pipe and reaches the handler in its
// declared list, never as an argument, so the line is never rewritten to carry
// one. The refusal names the word and gives the same line in the pipe form as
// its fix: the words before the first operation stay arguments, each operation
// starts a pipe stage, and the words after it until the next one are its
// argument words.
//
// path is the command path the args follow. A command that declares no
// operation refuses nothing. Generic operators and aliases are not judged
// here: only an operation the command itself runs can be typed as an argument
// of that command by mistake.
func RefusePipeOperationWords(path string, args []string) error {
	set, declared := lookupPipeFilters(path)
	if !declared || len(set.filters) == 0 {
		return nil
	}
	first := -1
	for i, word := range args {
		if _, operation := set.byName[strings.ToLower(word)]; operation {
			first = i
			break
		}
	}
	if first < 0 {
		return nil
	}
	var fix textbuf.Buffer
	fix.Str(path)
	for i, word := range args {
		if i >= first {
			if _, operation := set.byName[strings.ToLower(word)]; operation {
				fix.Str(" |")
			}
		}
		fix.Byte(' ')
		WriteInvocationValue(&fix, word)
	}
	var problem, why textbuf.Buffer
	problem.Str(args[first]).Str(" is a pipe operation of ").Str(path).Str(", not an argument")
	why.Str(path).Str(" runs its own pipe operations, which are typed after a pipe")
	// The fix repeats the operator's arguments, so a secret among them is
	// masked as every echo of a line is (redact.Command).
	return opererr.New(problem.String(), why.String(), redact.Command(fix.String()))
}

// PipeOperation is one operation of the typed pipe chain that the command
// declares as its own, handed to its handler instead of being rewritten into
// the command line: the operation's name and its decoded argument words, in
// the order typed.
type PipeOperation struct {
	Name string
	Args []string
}

// PipeOperations answers a copy of the declared list, in chain order. The
// handler computes with it before the answer exists.
func (v ValidatedArgs) PipeOperations() []PipeOperation {
	return slices.Clone(v.pipeOperations)
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

// Flag answers whether the keyword of flag leaf was typed. A flag carries no
// value, so neither Keyed nor Value answers it, and a word of a rest-of-line
// leaf that spells the keyword is not the flag.
func (v ValidatedArgs) Flag(leaf string) bool {
	return v.flags[leaf]
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
// mandatory definition unfilled. The validator keeps one MissingArgumentError
// per missing leaf inside an *ArgumentFaults, even when only one is missing,
// so each route can supply the complete command as the correction.
//
// It carries the lone positional binding the call DID make, because the daemon
// dispatcher chooses between two refusals on it: a command whose selector
// arrived positionally is told which other leaf is missing, not that it
// "requires a selector". Every MissingArgumentError of one call carries the
// same binding. The binding answers only that choice; a refused call yields no
// ValidatedArgs, so no handler can run on it.
type MissingArgumentError struct {
	// Leaf is a mandatory definition that no token filled.
	Leaf       string
	usage      string
	purpose    string
	unplaced   []string
	bound      string
	boundValue string
}

// Error explains the missing leaf and how to supply it when the command path
// is not known. Routes restate ArgumentFaults with that complete command.
func (e *MissingArgumentError) Error() string {
	return opererr.New(e.problem(), e.why(), e.fix()).Error()
}

// problem is also the compact label of this cause in a multi-fault list.
func (e *MissingArgumentError) problem() string {
	var tb textbuf.Buffer
	return tb.Str("required argument missing: ").Str(e.Leaf).String()
}

// why carries the model's declared argument meaning. A missing declaration is
// an explicit gap, never a purpose inferred from the leaf's name.
func (e *MissingArgumentError) why() string {
	var tb textbuf.Buffer
	if e.purpose == "" {
		return tb.Str("The command definition makes ").Str(e.Leaf).
			Str(" mandatory and supplies no further explanation").String()
	}
	return tb.Str("This command needs ").Str(e.Leaf).Str(": ").Str(e.purpose).String()
}

// fix uses the model's actual value form when this is a validator-built cause.
// A directly constructed cause has no usage metadata and cannot invent it.
func (e *MissingArgumentError) fix() string {
	var tb textbuf.Buffer
	if e.usage == "" {
		return tb.Str("supply the required ").Str(e.Leaf).Str(" argument").String()
	}
	return tb.Str("type the command again with ").Str(e.usage).String()
}

// Usage answers the words that supply the missing leaf, its keyword and then
// its value as the usage line writes it (`asn <asn>`), or the keyword alone for
// a flag. A list of faults names it beside the leaf, and every route writes it
// into the whole command it offers as the fix (ArgumentFaults.RestatedForLine). It answers "" for a
// value ValidateArgs did not build.
func (e *MissingArgumentError) Usage() string {
	return e.usage
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

// UnplacedTokens answers a copy of the positional values ValidateArgs could
// not bind to any definition. Named keyword/value pairs never contribute.
// The validator snapshots this evidence before returning its refusal, so a
// selector guard need not parse the caller's line again. A directly constructed
// cause holds no validation evidence and answers nil.
func (e *MissingArgumentError) UnplacedTokens() []string {
	return slices.Clone(e.unplaced)
}

// ValidateArgs implements two-phase validation of command arguments
// against YANG-declared ArgDefs.
//
// Phase 1 (keyword extraction): scan args for tokens matching ArgDef leaf names;
// when found, the next token is validated as that leaf's typed value.
// Phase 2 (positional matching): remaining args are offered to the unmatched
// ArgDefs, except a keyword-only one (ze:keyword-only), which phase 1 alone
// binds. Unmatched args pass through to the handler.
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
// *ArgumentFaults containing a *MissingArgumentError, which carries that binding
// for the same reason.
//
// The whole line is read before the call is refused, and every fault it holds
// is named: a keyword with no value, a value its leaf refuses, a keyword typed
// twice, a token no definition takes, and each missing mandatory argument.
// A line with one fault is refused with that fault alone, except a value its
// leaf refuses (*ValueArgumentError) or a missing mandatory argument
// (*MissingArgumentError), which is an *ArgumentFaults of one so
// every route restates it with the whole command; a line
// with more is refused with an *ArgumentFaults, whose Unwrap answers each, so
// errors.As finds every fault kind on both shapes. The faults are in the order
// the line was read, so the first one is the one a single-fault refusal named.
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
	// with one bad value. Definitions are walked in order, so the faults named
	// are in the same order on every run.
	//
	// Every fault of the line is collected rather than the first one returned:
	// an operator who fixes the one fault a refusal named and is then refused
	// for the next has been sent round twice. A leaf whose keyword or value is
	// at fault counts as matched from then on, so the same leaf is never also
	// reported missing.
	var faults faultList
	for i := range defs {
		value, bound := preMatched[defs[i].name]
		if !bound {
			continue
		}
		if err := ValidateArgString(value, &defs[i]); err != nil {
			faults.add(err)
		}
	}

	// The rest-of-line leaf takes its words before any keyword is read, so the
	// phases below see only the tokens in front of it, whether or not its value
	// is valid.
	all := args
	if rest != nil && !matched[rest.name] {
		start, from := restStart(args, defs, defByName, matched, rest)
		if start >= 0 {
			matched[rest.name] = true
			faults.rest = restPosition{typed: true, at: start}
			if from < len(args) {
				value := textbuf.Join(args[from:], " ")
				if err := ValidateArgString(value, rest); err != nil {
					faults.add(typedAt(err, from, len(args)-from))
				}
				keyed = keyedWith(keyed, rest.name, value)
			} else {
				faults.add(missingValueError(rest.name))
			}
			args = args[:start]
		}
	}
	consumed := make([]bool, len(args))
	var flags map[string]bool

	// Phase 1: keyword-value extraction.
	for i := 0; i < len(args); i++ {
		def, ok := defByName[args[i]]
		if !ok {
			continue
		}
		consumed[i] = true
		if matched[def.name] {
			// The second keyword and the value after it are one fault: the
			// operator deletes the pair. Its value is not judged, because the
			// leaf already holds the value the first keyword gave it.
			faults.add(fmt.Errorf("duplicate keyword %q", args[i]))
			if def.kind != ArgFlag && i+1 < len(args) {
				i++
				consumed[i] = true
			}
			continue
		}
		matched[def.name] = true
		if def.kind == ArgFlag {
			// A flag is the keyword alone; the next token is its own argument.
			if flags == nil {
				flags = make(map[string]bool)
			}
			flags[def.name] = true
			continue
		}
		if i+1 >= len(args) {
			faults.add(missingValueError(def.name))
			continue
		}
		i++
		consumed[i] = true
		if err := ValidateArgString(args[i], def); err != nil {
			faults.add(typedAt(err, i, 1))
			continue
		}
		keyed = keyedWith(keyed, def.name, args[i])
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
	unplaced := make([]int, 0, len(args))
	placed := make(map[int]*ArgDef, spare)
	positional := make([]string, 0, spare)
	for i, arg := range args {
		if consumed[i] {
			continue
		}
		positional = append(positional, arg)
		def := positionalDef(arg, defs, matched)
		if def == nil {
			unplaced = append(unplaced, i)
			continue
		}
		matched[def.name] = true
		placed[i] = def
		if spare == 1 {
			bound, boundValue = def.name, arg
		}
	}
	unplaced = reattributeUnplaced(args, defs, matched, placed, unplaced)

	// A token that filled no definition is a bad VALUE only when every such
	// token can be attributed to a definition of its own: as many tokens left
	// over as definitions still open, or fewer. Each such token is then a fault
	// of its own, and the definitions they were typed for are not also reported
	// missing. More tokens than open definitions means at least one of them is
	// a value for nothing, so it is the keyword half of a group the command's
	// own grammar declares and this validator does not hold (`update <hex>` on
	// `show policy test peer`). The dispatcher then has no ground to name any
	// token as the fault, and the missing mandatory argument below is what is
	// certainly wrong with the call.
	open := unmatchedDefCount(defs, matched)
	if len(unplaced) > 0 && open > 0 && len(unplaced) <= open {
		for _, at := range unplaced {
			faults.add(typedAt(positionalError(args[at], defs, matched), at, 1))
		}
		return ValidatedArgs{}, faults.verdict()
	}

	// Phase 3: mandatory check. Every missing leaf is named, each with its own
	// *MissingArgumentError, so an operator who types the first one is not
	// refused again for the next. A leaf whose keyword or value is at fault
	// is matched by now, so it is never also reported missing.
	// Snapshot unresolved positional values only when a missing cause needs
	// them. The indexes came from this bounded token walk; all missing causes
	// share one immutable snapshot, while callers receive copies.
	var unplacedValues []string
	missing := false
	for i := range defs {
		if !defs[i].mandatory || matched[defs[i].name] {
			continue
		}
		if unplacedValues == nil {
			unplacedValues = make([]string, len(unplaced))
			for i, at := range unplaced {
				unplacedValues[i] = args[at]
			}
		}
		purpose := defs[i].Description()
		if purpose == "" {
			purpose = defs[i].ShortHelp()
		}
		faults.addMissing(&MissingArgumentError{Leaf: defs[i].name, usage: keywordUsage(&defs[i]),
			purpose: purpose, unplaced: unplacedValues, bound: bound, boundValue: boundValue})
		missing = true
	}

	// Every mandatory definition is filled, so each token left over is a
	// fault of the call.
	if !missing && len(unplaced) > 0 && open > 0 {
		for _, at := range unplaced {
			faults.add(typedAt(positionalError(args[at], defs, matched), at, 1))
		}
	}

	if err := faults.verdict(); err != nil {
		return ValidatedArgs{}, err
	}
	return ValidatedArgs{tokens: slices.Clone(all), bound: bound, boundValue: boundValue, keyed: keyed, flags: flags, positional: positional}, nil
}

// reattributeUnplaced answers which tokens are left unplaced once each is read
// in the order the definitions are declared, which is the order the usage line
// writes them (`raw <encoding> <data>`). positionalDef offers a token to every
// open definition, so an earlier token the operator mistyped for an earlier
// leaf can fill a later, looser leaf instead: in `raw gzip DEADBEEF` the free
// string data takes gzip, and DEADBEEF, which data was declared to hold, is
// left over and blamed as the encoding. The operator typed gzip where the
// encoding goes, so gzip is the token to refuse, and the fix must replace it.
//
// For each unplaced token, the earliest token before it that a definition
// declared after an open definition took is swapped with it, when that
// definition accepts the unplaced token too: the later definition keeps a
// value it accepts, and the earlier token is the one left for the open
// definition to refuse. The count of unplaced tokens and of matched
// definitions is unchanged, so no refusal is loosened and none is added; only
// which token is named changes. placed maps a token's index to the definition
// it filled and is updated in place.
func reattributeUnplaced(args []string, defs []ArgDef, matched map[string]bool, placed map[int]*ArgDef, unplaced []int) []int {
	declared := make(map[string]int, len(defs))
	for i := range defs {
		declared[defs[i].name] = i
	}
	firstOpen := len(defs)
	for i := range defs {
		if !matched[defs[i].name] && awaitsPositional(&defs[i]) {
			firstOpen = i
			break
		}
	}
	out := slices.Clone(unplaced)
	for n, at := range out {
		swap := -1
		for p := range at {
			def, ok := placed[p]
			if !ok {
				continue
			}
			if declared[def.name] <= firstOpen {
				continue
			}
			if ValidateArgString(args[at], def) != nil {
				continue
			}
			swap = p
			break
		}
		if swap < 0 {
			continue
		}
		placed[at] = placed[swap]
		delete(placed, swap)
		out[n] = swap
	}
	slices.Sort(out)
	return out
}

// typedAt answers err with the value fault it is recorded as typed at index at
// of the line, over span tokens, so the whole-command fix writes the value's
// correction in its place (ArgumentFaults.RestatedForLine). Any other error is
// answered unchanged.
func typedAt(err error, at, span int) error {
	if value, ok := errors.AsType[*ValueArgumentError](err); ok {
		return value.typedAt(at, span)
	}
	return err
}

// keyedWith answers keyed with leaf bound to value, allocating the map on the
// first value a leaf takes by name (keyed stays nil until then, see
// ValidatedArgs).
func keyedWith(keyed map[string]string, leaf, value string) map[string]string {
	if keyed == nil {
		keyed = make(map[string]string)
	}
	keyed[leaf] = value
	return keyed
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
			// A keyword-only leaf (ze:keyword-only) is bound by the keyword
			// phase alone: a word no keyword introduced is never its value.
			if def.keywordOnly {
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

// unmatchedDefCount reports how many ArgDefs are still waiting for a value a
// spare token could be (awaitsPositional).
func unmatchedDefCount(defs []ArgDef, matched map[string]bool) int {
	n := 0
	for i := range defs {
		if !matched[defs[i].name] && awaitsPositional(&defs[i]) {
			n++
		}
	}
	return n
}

// awaitsPositional reports whether def, while unfilled, is a definition a
// spare token may have been typed for. A flag never waits for a value, a
// rest-of-line leaf is placed by restStart or found absent, and a keyword-only
// leaf takes its value after its keyword alone: none of the three is open to a
// spare token, so none is counted when a token is attributed or blamed. A
// mandatory keyword-only leaf left out is still reported missing (phase 3).
func awaitsPositional(def *ArgDef) bool {
	if def.kind == ArgFlag {
		return false
	}
	if def.rest {
		return false
	}
	return !def.keywordOnly
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
		if !matched[defs[i].name] && awaitsPositional(&defs[i]) {
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
