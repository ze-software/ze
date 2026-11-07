// Design: docs/architecture/plugin/feature-gates.md -- reflected type authority is exposed separately from callable ownership.
package repoinventory

import "go/ast"

// externalReflection describes methods available through a concrete type
// descriptor, never the implementation of an ordinary function value. Its zero
// grants no authority. It holds no existing storage locations or value fields.
type externalReflection struct {
	methods map[ast.Node]sourceExpr
	origins map[string]bool
	unknown string
	// unknownAt is the type expression that produced unknown, carried with
	// the first reason exactly as externalValue carries its own.
	unknownAt sourceExpr
}

// join owns its output maps independently from either input. Method identities
// belong to this reader's retained declarations, not reconstructed scope chains.
func (r *externalReflection) join(other externalReflection) bool {
	changed := false
	if r.methods == nil {
		if len(other.methods) != 0 {
			r.methods = make(map[ast.Node]sourceExpr, len(other.methods))
		}
	}
	for node, source := range other.methods {
		if _, found := r.methods[node]; !found {
			r.methods[node] = source
			changed = true
		}
	}
	if r.origins == nil {
		if len(other.origins) != 0 {
			r.origins = make(map[string]bool, len(other.origins))
		}
	}
	for identity := range other.origins {
		if !r.origins[identity] {
			r.origins[identity] = true
			changed = true
		}
	}
	if r.unknown == "" {
		if other.unknown != "" {
			r.unknown = other.unknown
			r.unknownAt = other.unknownAt
			changed = true
		}
	}
	return changed
}

// promoteReflection is used only at the opaque exposure boundary. Ordinary
// callee dispatch never reads reflection.methods or treats them as owners.
func (e *externalEffects) promoteReflection(reflection externalReflection) {
	if e.amb.unknown != "" {
		return
	}
	value := externalValue{callables: reflection.methods, origins: reflection.origins, unknown: reflection.unknown, unknownAt: reflection.unknownAt}
	if e.growAmbient(value) {
		e.changed = true
	}
}

// exposeSignature is the sole actual-node signature hook. Raw callable
// construction remains cheap, and bound method types omit their receiver.
func (e *externalEffects) exposeSignature(node ast.Node, source sourceExpr) {
	if e.amb.unknown != "" {
		return
	}
	signature, _ := externalFormalSignature(node)
	if signature == nil {
		e.promoteReflection(externalReflection{unknown: "exposed callable signature is unavailable", unknownAt: source})
		return
	}
	source.value = signature
	walk := externalReflectionWalk{effects: e}
	e.promoteReflection(walk.value(source, e.typeIdentities.lexicalScope(source), true, false, 0))
}

// reflectedValue preserves concrete typed-nil function authority. An interface
// value alone has no concrete runtime descriptor; descriptor child edges do.
// Past the ambient check, the answer is kept for the analysis
// (externalReflectionMemo) and is shared: a caller MUST only read it.
func (e *externalEffects) reflectedValue(source sourceExpr, scope *externalTypeScope, depth int) externalReflection {
	if e.amb.unknown != "" {
		return externalReflection{unknown: e.amb.unknown, unknownAt: e.amb.unknownAt}
	}
	key := externalReflectionMemoKey{source: source, scope: scope}
	if e.counters != nil {
		e.counters.reflected(key)
	}
	memo := true
	if probe := e.typeProbe; probe != nil {
		memo = !probe.unmemoized
	}
	if !memo {
		return e.walkReflectedValue(source, scope, depth)
	}
	if answer, found := e.reflectedValues.answer(key, depth); found {
		if e.counters != nil {
			e.counters.memoHits++
		}
		return answer
	}
	answer := e.walkReflectedValue(source, scope, depth)
	e.reflectedValues.keep(key, depth, answer)
	return answer
}

// walkReflectedValue is reflectedValue's walk, run once per kept answer.
func (e *externalEffects) walkReflectedValue(source sourceExpr, scope *externalTypeScope, depth int) externalReflection {
	walk := externalReflectionWalk{effects: e}
	interfaceRoot, reason := walk.interfaceRoot(source, scope, depth)
	if reason != "" {
		return externalReflection{unknown: reason, unknownAt: source}
	}
	if interfaceRoot {
		return externalReflection{}
	}
	return walk.value(source, scope, true, true, depth)
}
