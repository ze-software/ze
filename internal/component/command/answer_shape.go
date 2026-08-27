// Design: docs/architecture/api/commands.md — per-command answer shape
// Detail: pipe_catalog.go — the operator contracts this shape is checked against
// Related: column_order.go — the other user of the command-path registry
// Related: docs/architecture/api/commands.md — the answer shape an operator list derives from
//
// answer_shape.go answers one question: does this command's answer have rows?
//
// It is asked twice, because two different moments can answer it and neither
// is enough alone.
//
//   - A command DECLARES its shape, which is known before the command runs, so
//     an operator the shape cannot support is refused without dispatching. It is
//     also what the published per-command documentation is derived from.
//   - An answer HAS a shape, derived from the payload in hand. Every command has
//     one whether it declared anything or not, so this is what makes the refusal
//     universal rather than a property of the commands somebody remembered to
//     annotate.
//
// The answer head's item type is a third reading and is not a contract:
// RenderRecords writes `doc` below rpc.AnswerBufferThreshold and `map` or `tab`
// above it, so the same command reports `doc` at 200 rows and `tab` at 300.

package command

import "sort"

// shapeRegistry holds the answer shape each command declares. The value is a
// slice so that an EMPTY declaration is expressible: registering no shape marks
// a command as declaring none, which stops it inheriting a shorter registered
// path's shape. RegisterColumns and RegisterPipeFilters use the same convention
// for the same reason.
var shapeRegistry = newDeclarationRegistry("answer shape",
	func(shapes []AnswerShape) bool { return len(shapes) == 0 })

// RegisterShape declares the shape of a command's answer, so the operators it
// supports can be published and the ones it cannot can be refused before the
// command runs.
//
// A command that renders rows declares ShapeMap, or ShapeTab when it also
// declares a column order the rows are read against. A command answering one
// document or one value declares ShapeDoc, and every row operator is then
// refused by name rather than answering something plausible: `show version |
// first 1` used to answer the version string, dropping the key the bare command
// prints.
//
// Passing no shape declares NONE, which is what a command under a declared
// parent does when its own answer has a different shape.
//
// Declaring nothing is not an error. The shape of the answer in hand is derived
// at apply time either way, so an undeclared command still refuses what it
// cannot support; it refuses later, and publishes less.
//
// Two packages declaring two DIFFERENT shapes for one command path is a Ze
// defect and panics, and declaring none never overrides a declared shape
// (declarationRegistry.declare).
func RegisterShape(commands []string, shapes ...AnswerShape) {
	if len(shapes) > 1 {
		shapes = shapes[:1]
	}
	shapeRegistry.declare(commands, shapes)
}

// ShapeForCommand answers the shape a command declared, resolved by the longest
// registered command path that is a prefix of it, and whether it declared one.
func ShapeForCommand(command string) (AnswerShape, bool) {
	shapes, ok := shapeRegistry.lookup(command)
	if !ok || len(shapes) == 0 {
		return ShapeDoc, false
	}
	return shapes[0], true
}

// ResetShapesForTest clears every declared shape.
func ResetShapesForTest() {
	shapeRegistry.reset()
}

// addressFieldRegistry holds the fields each command declares to hold an IP
// address. A declaration admits `| resolve` and `| origin` and is also the
// complete set of field names those operators may transform.
var addressFieldRegistry = newDeclarationRegistry("address field list",
	func(fields []string) bool { return len(fields) == 0 })

// RegisterAddressFields declares which of a command's fields hold an IP
// address.
//
// Both enforcement points read this declaration. validateDeclaredShape
// (pipe.go) refuses the operators when the command or its address fields are
// undeclared. parsePipeChain binds the names onto each operator, and
// resolveJSON/originJSON transform only matching keys on document and record
// paths. A string in any other field remains data even when it parses as an IP
// address, which keeps identifiers such as router-id untouched.
//
// Two packages declaring two DIFFERENT field lists for one command path is a Ze
// defect and panics, and declaring none never overrides a declared list
// (declarationRegistry.declare).
func RegisterAddressFields(commands []string, fields ...string) {
	stored := make([]string, 0, len(fields))
	for _, field := range fields {
		if field = normalizeCommand(field); field != "" {
			stored = append(stored, field)
		}
	}
	addressFieldRegistry.declare(commands, stored)
}

// AddressFieldsForCommand answers the fields a command declares as holding an
// IP address, resolved by longest registered prefix. It answers nil when the
// command declares none.
func AddressFieldsForCommand(command string) []string {
	fields, ok := addressFieldRegistry.lookup(command)
	if !ok {
		return nil
	}
	return fields
}

// ResetAddressFieldsForTest clears every declared address field.
func ResetAddressFieldsForTest() {
	addressFieldRegistry.reset()
}

// PluginShape is what one command's answer holds, as a caller outside this
// repository declares it.
//
// The three fields travel together because they describe one answer. Columns and
// AddressFields are read against the rows Shape says the answer has, so an
// address-field list with no shape still admits nothing: validateDeclaredShape
// refuses the operator because the command itself is undeclared.
type PluginShape struct {
	// Command is the command path whose answer this describes.
	Command string
	// Shape is what the answer holds: one document, or rows.
	Shape AnswerShape
	// Columns are the answer's JSON keys, in the order a person reads them.
	// Empty declares no order.
	Columns ColumnOrder
	// AddressFields are the answer's JSON keys whose value holds an IP address.
	// Empty declares none, which refuses the address operators by name.
	AddressFields []string
}

// RegisterPluginShapes declares the answer shapes of a caller outside this
// repository. It reports a bad declaration rather than panicking on it.
//
// RegisterShape, RegisterColumns and RegisterAddressFields panic when two
// packages declare two different values for one path, which is right for a
// table written in init(). These values arrived over a socket, so the same
// conflict is an operating error: the caller is refused, and the daemon and
// every other plugin keep running. declareFor (column_order.go) is that route,
// and it is the only one this function takes.
//
// Every declaration writes all three registries, and a caller that named no
// column and no address field writes an EMPTY declaration into those two. The
// empty declaration is a barrier: a command path that declares nothing resolves
// to the nearest declared ancestor, so without it a caller's `show x rows`
// would read its rows against the column order of its `show x`. Knowing that
// resolution rule is not a thing a declaring author owes, which is the reason
// aliasBarriers (alias.go) exists for the alias channel.
//
// A barrier never overrides a value another package declared for the same path:
// declareFor keeps what the path holds when the incoming declaration is empty.
//
// Nothing survives a refusal. The first error withdraws everything this call
// wrote, so a caller never has to undo a partial declaration it did not ask for.
//
// The owner is the name UnregisterPluginShapes takes the declaration back
// under. It is opaque: nothing else in this package reads it.
//
// The caller MUST have bounded and checked the strings before this point. This
// function is the write, not the gate.
//
// Safe for concurrent use.
func RegisterPluginShapes(owner string, declared []PluginShape) error {
	for _, declaration := range declared {
		if err := shapeRegistry.declareFor(owner, declaration.Command, []AnswerShape{declaration.Shape}); err != nil {
			UnregisterPluginShapes(owner)
			return err
		}

		var orders []ColumnOrder
		if order := normalizedNames(declaration.Columns); len(order) > 0 {
			orders = []ColumnOrder{order}
		}
		if err := columnRegistry.declareFor(owner, declaration.Command, orders); err != nil {
			UnregisterPluginShapes(owner)
			return err
		}
		if err := addressFieldRegistry.declareFor(owner, declaration.Command, normalizedNames(declaration.AddressFields)); err != nil {
			UnregisterPluginShapes(owner)
			return err
		}
	}
	return nil
}

// UnregisterPluginShapes takes back every shape, column order and address-field
// list the owner declared, so each path returns to what it held before.
//
// A path an in-tree package declared EMPTY returns to that empty declaration,
// not to nothing: `show bgp rpki` carries the barrier the BGP peer command
// plugin puts on every direct child of `show bgp`, and removing the path would
// let the child inherit its parent's shape and its parent's columns.
//
// It reports nothing. A caller tears an owner down without knowing whether that
// owner ever declared a shape.
func UnregisterPluginShapes(owner string) {
	shapeRegistry.withdraw(owner)
	columnRegistry.withdraw(owner)
	addressFieldRegistry.withdraw(owner)
}

// normalizedNames is the one spelling a declared field name is stored under:
// lowercase, with no surrounding space. It answers a new slice, so a caller's
// slice is never aliased into a registry.
//
// A name that normalizes to nothing is dropped, which is what RegisterColumns
// and RegisterAddressFields do with one. A caller outside this repository is
// refused for such a name before it reaches here, so the drop is the in-tree
// convention rather than a second answer to the same question.
func normalizedNames(names []string) []string {
	normalized := make([]string, 0, len(names))
	for _, name := range names {
		if name = normalizeCommand(name); name != "" {
			normalized = append(normalized, name)
		}
	}
	return normalized
}

// rowSet answers the rows a value holds, if it is a row set at all.
//
// Two spellings carry rows in Ze's answers and both are common:
//
//   - an ARRAY of objects, which is what a streamed walk collapses to;
//   - a MAP keyed by identity, whose every value is an object. `show bgp peer
//     list` answers this shape, keyed by peer address.
//
// The identity spelling is why this cannot be an array test. The old countItems
// happened to answer correctly for it, by unwrapping a single-key map and
// counting the inner map's keys, and the audit recorded that as right by
// accident. It is a real shape and it is handled deliberately here.
//
// Rows from an identity map come back in sorted key order, so `| first 2`
// answers the same two rows every time.
func rowSet(v any) (rows []any, keys []string, ok bool) {
	switch typed := v.(type) {
	case []any:
		return typed, nil, true
	case map[string]any:
		// An empty identity map is a known row set with zero rows. Its keys are
		// a non-nil empty slice so selectRows preserves map spelling; nil means
		// the row set was an array.
		if len(typed) == 0 {
			return nil, []string{}, true
		}
		names := make([]string, 0, len(typed))
		for name, value := range typed {
			if _, isObject := value.(map[string]any); !isObject {
				return nil, nil, false
			}
			names = append(names, name)
		}
		sort.Strings(names)
		rows = make([]any, 0, len(names))
		for _, name := range names {
			rows = append(rows, typed[name])
		}
		return rows, names, true
	default:
		return nil, nil, false
	}
}

// rowsIn finds the rows in a decoded answer and answers them, the key they sit
// under, and whether the answer has rows at all.
//
// A map is asked about its KEYS first and about itself second. Otherwise
// `{"peers":{"a":{},"b":{}}}` would read as one row named peers, rather than as
// two peers.
//
// A map holding SEVERAL row sets is ambiguous, and this reports no rows rather
// than picking one. countItems used to answer the number of top-level KEYS for
// that case, so `show bgp | count` answered 6, which is a plausible number and
// the wrong question. Refusing beats guessing.
func rowsIn(v any) (rows []any, key string, ok bool) {
	rows, _, key, ok = rowsInKeyed(v)
	return rows, key, ok
}

// rowsInKeyed is rowsIn, and also answers the identity keys when the rows are a
// map keyed by identity rather than an array.
//
// An operator that REBUILDS the rows needs them. Writing an array back over an
// identity map would drop the keys, so `show bgp peer list | first 2` would
// answer two peers with their addresses gone.
func rowsInKeyed(v any) (rows []any, keys []string, key string, ok bool) {
	// An EMPTY answer has zero rows; it does not have a shape that cannot
	// support them. A filter that removed every row, and a command that
	// answered nothing, both land here, and refusing either would turn
	// "nothing to report" into an error. `show bgp peer list | match nothing`
	// answers no rows and exits 0, which is what lets a reader tell an empty
	// answer from a command that did not run.
	if isEmptyAnswer(v) {
		return nil, nil, "", true
	}
	if typed, isMap := v.(map[string]any); isMap {
		found := ""
		count := 0
		for name, value := range typed {
			if _, _, isRows := rowSet(value); isRows {
				found = name
				count++
			}
		}
		if count == 1 {
			inner, innerKeys, _ := rowSet(typed[found])
			return inner, innerKeys, found, true
		}
		if count > 1 {
			return nil, nil, "", false
		}
	}
	if rows, keys, isRows := rowSet(v); isRows {
		return rows, keys, "", true
	}
	return nil, nil, "", false
}

// rowKeys answers the keys of a map answer that hold row sets, so a refusal can
// name the candidates it refused to choose between.
func rowKeys(v any) []string {
	typed, isMap := v.(map[string]any)
	if !isMap {
		return nil
	}
	var keys []string
	for name, value := range typed {
		if _, _, isRows := rowSet(value); isRows {
			keys = append(keys, name)
		}
	}
	return keys
}

// selectRows rebuilds an answer keeping only the rows at the given indices, in
// the SPELLING the answer used: an array stays an array, and a map keyed by
// identity stays keyed, with the keys of the rows kept.
//
// Every row operator that removes rows goes through here, so none of them can
// change the shape of the answer as a side effect of filtering it.
func selectRows(data any, envelopeKey string, keys []string, rows []any, keep []int) any {
	if keys == nil {
		kept := make([]any, 0, len(keep))
		for _, i := range keep {
			kept = append(kept, rows[i])
		}
		return placeRows(data, envelopeKey, kept)
	}
	kept := make(map[string]any, len(keep))
	for _, i := range keep {
		kept[keys[i]] = rows[i]
	}
	return placeRows(data, envelopeKey, kept)
}

// placeRows puts a rebuilt row set back where it came from: under its key when
// the answer is an envelope, or as the whole answer when it is not.
func placeRows(data any, envelopeKey string, rows any) any {
	if envelopeKey == "" {
		return rows
	}
	if envelope, isMap := data.(map[string]any); isMap {
		envelope[envelopeKey] = rows
		return envelope
	}
	return rows
}

// shapeOfAnswer derives the shape of a decoded answer. Every answer has one,
// which is what lets an operator be refused on a command that declared nothing.
func shapeOfAnswer(v any) AnswerShape {
	if _, _, ok := rowsIn(v); ok {
		return ShapeMap
	}
	return ShapeDoc
}

// isEmptyAnswer reports whether a decoded answer carries nothing at all.
func isEmptyAnswer(v any) bool {
	switch typed := v.(type) {
	case nil:
		return true
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}
