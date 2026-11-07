// Design: docs/architecture/plugin/feature-gates.md -- ordered method source census.
// Related: externaleffecttypeidentity.go, externalreflectiontypes.go -- fresh method collectors.
package repoinventory

import "go/ast"

// externalMethodCensus is published only after every original receiver row is
// classified. Its lists and receiver facts remain immutable after publication.
type externalMethodCensus struct {
	byType   map[*ast.TypeSpec][]externalMethodRow
	failures []externalMethodRow
}

// externalMethodRow retains its original position and declaration. A nil
// receiver means resolution has not run, including a row with an invalid shape.
// A present receiver is complete even when its declaration is nil.
type externalMethodRow struct {
	ordinal  int
	function sourceFunction
	receiver *externalReceiverResult
}

// externalMethodCursor owns only its slice heads. Its zero is an empty cursor.
// Raw declarations are visited first when present. Otherwise next merges the
// two immutable census lists without changing their rows or backing storage.
type externalMethodCursor struct {
	raw        []sourceFunction
	rawOrdinal int
	targets    []externalMethodRow
	failures   []externalMethodRow
}

// methodCursor accounts for every captured row once per package. A query
// visits matching targets and all source failures in their original order.
// The walks that ask treat the type graph by reachability, so a receiver is
// resolved from depth 0 whatever depth the frame was met at. An uncaptured
// list retains the original lazy traversal.
func (e *externalEffects) methodCursor(pkg *sourcePackage, definition *ast.TypeSpec) externalMethodCursor {
	functions, captured := e.typeIdentities.methods[pkg]
	if !captured {
		return externalMethodCursor{raw: functions}
	}
	census, complete := e.methodCensuses[pkg]
	if !complete {
		census = e.buildMethodCensus(pkg, functions)
		e.methodCensuses[pkg] = census
	}
	return externalMethodCursor{targets: census.byType[definition], failures: census.failures}
}

// buildMethodCensus resolves source facts only. Visibility, execution checks and
// method signatures remain the collectors' decisions when each row is visited.
func (e *externalEffects) buildMethodCensus(pkg *sourcePackage, functions []sourceFunction) *externalMethodCensus {
	census := &externalMethodCensus{byType: make(map[*ast.TypeSpec][]externalMethodRow)}
	identity := externalTypeWalk{effects: e}
	for ordinal, function := range functions {
		row := externalMethodRow{ordinal: ordinal, function: function}
		if len(function.decl.Recv.List) != 1 {
			census.failures = append(census.failures, row)
			continue
		}
		source := sourceExpr{value: function.decl.Recv.List[0].Type, file: function.file, pkg: pkg}
		row.receiver = &externalReceiverResult{}
		row.receiver.definition, row.receiver.generic, row.receiver.reason = identity.receiver(source, 0)
		if row.receiver.reason != "" {
			census.failures = append(census.failures, row)
			continue
		}
		census.byType[row.receiver.definition] = append(census.byType[row.receiver.definition], row)
	}
	return census
}

// next returns only a query-local row copy. A false result marks exhaustion and
// the returned zero row MUST NOT be used by the collector.
func (cursor *externalMethodCursor) next() (externalMethodRow, bool) {
	if len(cursor.raw) != 0 {
		row := externalMethodRow{ordinal: cursor.rawOrdinal, function: cursor.raw[0]}
		cursor.raw = cursor.raw[1:]
		cursor.rawOrdinal++
		return row, true
	}
	if len(cursor.targets) == 0 {
		if len(cursor.failures) == 0 {
			return externalMethodRow{}, false
		}
		row := cursor.failures[0]
		cursor.failures = cursor.failures[1:]
		return row, true
	}
	if len(cursor.failures) != 0 {
		if cursor.failures[0].ordinal < cursor.targets[0].ordinal {
			row := cursor.failures[0]
			cursor.failures = cursor.failures[1:]
			return row, true
		}
	}
	row := cursor.targets[0]
	cursor.targets = cursor.targets[1:]
	return row, true
}

// resolution MUST be called only after the collector proves one receiver
// field. Raw rows then resolve lazily, after its visibility and shape checks.
func (row externalMethodRow) resolution(identity *externalTypeWalk, pkg *sourcePackage) externalReceiverResult {
	if row.receiver != nil {
		return *row.receiver
	}
	source := sourceExpr{value: row.function.decl.Recv.List[0].Type, file: row.function.file, pkg: pkg}
	definition, generic, reason := identity.receiver(source, 0)
	return externalReceiverResult{definition: definition, generic: generic, reason: reason}
}
