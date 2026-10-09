package yang

import "testing"

// extensionUsageModule wraps body as the block of an extension usage "m:e",
// under a leaf of a module that declares extension e, so every statement of
// body is checked against the RFC 7950 Section 14 grammar (Section 7.19).
func extensionUsageModule(body string) string {
	return `module m { namespace "urn:m"; prefix m; extension e { argument text; } leaf a { type string; m:e "x" { ` +
		body + ` } } }`
}

// TestRFC7950ExtensionSubstatementURIAndPathArguments drives the two Section
// 14 argument rules that read another grammar: "uri-str", "< URI in RFC 3986
// >", the argument of namespace, and "path-arg-str", "< path-arg >", the
// argument of path. Each refused argument breaks its grammar and its twin
// keeps it.
//
// VALIDATES: checkURI and checkPathArg, the uri-str and path-arg-str checkers.
//
// RFC requirement: RFC7950-7.19-1 negative — under an extension usage, "namespace" whose argument "not a uri" is no RFC 3986 URI, and "path" whose argument "a" or "../a[" is no path-arg, are each refused naming the argument.
// RFC requirement: RFC7950-7.19-1 positive — under an extension usage, "namespace" with the URI "urn:ietf:params:xml:ns:yang:x" and a leafref "path" with "../a" or "/m:a/m:b[m:k = current()/../m:k]" load.
func TestRFC7950ExtensionSubstatementURIAndPathArguments(t *testing.T) {
	requireModuleRefused(t, "namespace not a URI", "not a uri", extensionUsageModule(`namespace "not a uri";`))
	requireModuleRefused(t, "path neither absolute nor relative", "not a path-arg",
		extensionUsageModule(`leaf q { type leafref { path "a"; } }`))
	requireModuleRefused(t, "path predicate unclosed", "not a path-arg",
		extensionUsageModule(`leaf q { type leafref { path "../a["; } }`))
	requireModuleLoaded(t, "namespace URI", extensionUsageModule(`namespace "urn:ietf:params:xml:ns:yang:x";`))
	requireModuleLoaded(t, "relative path", extensionUsageModule(`leaf q { type leafref { path "../a"; } }`))
	requireModuleLoaded(t, "absolute path with predicate",
		extensionUsageModule(`leaf q { type leafref { path "/m:a/m:b[m:k = current()/../m:k]"; } }`))
}

// TestRFC7950ExtensionSubstatementCardinality drives the repetition each
// Section 14 statement rule gives its substatements: leaf-stmt names the bare
// "type-stmt", exactly one, "[description-stmt]", at most one, and
// "*must-stmt", any number. The ABNF comment "these stmts can appear in any
// order" frees the order, never the count.
//
// VALIDATES: extensionSubstatementError, the counts of a rule's block.
//
// RFC requirement: RFC7950-7.19-1 negative — under an extension usage, a leaf holding two "type" statements, or two "description" statements, is refused naming the repeated statement.
// RFC requirement: RFC7950-7.19-1 positive — under an extension usage, a leaf holding one "type", one "description" and two "must" statements, in any order, loads.
func TestRFC7950ExtensionSubstatementCardinality(t *testing.T) {
	requireModuleRefused(t, "two types", "type appears 2 times",
		extensionUsageModule(`leaf q { type string; type int8; }`))
	requireModuleRefused(t, "two descriptions", "description appears 2 times",
		extensionUsageModule(`leaf q { type string; description "a"; description "b"; }`))
	requireModuleLoaded(t, "one of each and two musts",
		extensionUsageModule(`leaf q { must "1"; description "a"; type string; must "2"; }`))
}

// TestRFC7950ExtensionSubstatementRequiredStatementsAndBlocks drives the
// substatements and blocks a Section 14 statement rule requires: leaf-stmt
// names "type-stmt" with no repetition, list-stmt names "1*data-def-stmt",
// and refine-stmt opens its block with a bare "{", so "refine x;" breaks the
// rule even though every statement its block names is optional.
//
// VALIDATES: extensionSubstatementError and statementHasBlock.
//
// RFC requirement: RFC7950-7.19-1 negative — under an extension usage, a leaf with no "type", a list with no data definition, and "refine" with no block are each refused naming what is missing.
// RFC requirement: RFC7950-7.19-1 positive — under an extension usage, a leaf with its "type", a list with a key leaf, and "refine" with an empty block or a "description" load.
func TestRFC7950ExtensionSubstatementRequiredStatementsAndBlocks(t *testing.T) {
	requireModuleRefused(t, "leaf without type", "requires at least 1 type",
		extensionUsageModule(`leaf q { description "d"; }`))
	requireModuleRefused(t, "list without data definition", "do not match list-stmt",
		extensionUsageModule(`list l { key "k"; }`))
	requireModuleRefused(t, "refine without block", "requires a block", extensionUsageModule(`refine "x";`))
	requireModuleLoaded(t, "leaf with type", extensionUsageModule(`leaf q { type string; }`))
	requireModuleLoaded(t, "list with key leaf", extensionUsageModule(`list l { key "k"; leaf k { type string; } }`))
	requireModuleLoaded(t, "refine with empty block", extensionUsageModule(`refine "x" { }`))
	requireModuleLoaded(t, "refine with description", extensionUsageModule(`refine "x" { description "d"; }`))
}

// TestRFC7950ExtensionSubstatementContextForms drives the statements whose
// Section 14 rule depends on the statement holding them. deviation-stmt holds
// four deviate rules, each with its own block and its own argument, and admits
// "deviate-not-supported-stmt" alone or "1*(deviate-add-stmt /
// deviate-replace-stmt / deviate-delete-stmt)". type-stmt admits one
// alternative of type-body-stmts. body-stmts holds augment-stmt, whose
// argument is an absolute-schema-nodeid, and uses-stmt holds
// uses-augment-stmt, whose argument is a descendant-schema-nodeid.
//
// VALIDATES: resolveProduction and yangGrammar.matches.
//
// RFC requirement: RFC7950-7.19-1 negative — under an extension usage, "deviate add" holding "type", "deviate not-supported" holding "description", a deviation mixing "not-supported" and "add", a type holding "length" and "range", a type holding "enum" and "length", "augment /a" under uses and "augment a" in a submodule body are each refused.
// RFC requirement: RFC7950-7.19-1 positive — under an extension usage, "deviate replace" holding "type", "deviate not-supported" alone, "deviate add" with "deviate delete", a type holding "length" and two "pattern", a type holding "range", "augment a" under uses and "augment /a" in a submodule body load.
func TestRFC7950ExtensionSubstatementContextForms(t *testing.T) {
	const submodule = `submodule s { yang-version 1.1; belongs-to m { prefix m; } augment "`
	requireModuleRefused(t, "type under deviate add", "type is not a substatement of deviate-add-stmt",
		extensionUsageModule(`deviation "/m:a" { deviate add { type string; } }`))
	requireModuleRefused(t, "description under deviate not-supported",
		"description is not a substatement of deviate-not-supported-stmt",
		extensionUsageModule(`deviation "/m:a" { deviate not-supported { description "d"; } }`))
	requireModuleRefused(t, "not-supported mixed with add", "do not match deviation-stmt",
		extensionUsageModule(`deviation "/m:a" { deviate not-supported; deviate add { must "1"; } }`))
	requireModuleRefused(t, "length with range", "do not match type-stmt",
		extensionUsageModule(`leaf q { type string { length "1"; range "1"; } }`))
	requireModuleRefused(t, "enum with length", "do not match type-stmt",
		extensionUsageModule(`leaf q { type enumeration { enum a; length "1"; } }`))
	requireModuleRefused(t, "absolute augment under uses", "descendant-schema-nodeid",
		extensionUsageModule(`uses g { augment "/a" { leaf x { type string; } } }`))
	requireModuleRefused(t, "descendant augment in a submodule body", "absolute-schema-nodeid",
		extensionUsageModule(submodule+`a" { leaf x { type string; } } }`))
	requireModuleLoaded(t, "type under deviate replace",
		extensionUsageModule(`deviation "/m:a" { deviate replace { type string; } }`))
	requireModuleLoaded(t, "deviate not-supported alone",
		extensionUsageModule(`deviation "/m:a" { deviate not-supported; }`))
	requireModuleLoaded(t, "deviate add with delete",
		extensionUsageModule(`deviation "/m:a" { deviate add { must "1"; } deviate delete { must "1"; } }`))
	requireModuleLoaded(t, "length with patterns",
		extensionUsageModule(`leaf q { type string { length "1"; pattern "a"; pattern "b"; } }`))
	requireModuleLoaded(t, "range", extensionUsageModule(`leaf q { type int8 { range "1..2"; } }`))
	requireModuleLoaded(t, "descendant augment under uses",
		extensionUsageModule(`uses g { augment "a" { leaf x { type string; } } }`))
	requireModuleLoaded(t, "absolute augment in a submodule body",
		extensionUsageModule(submodule+`/a" { leaf x { type string; } } }`))
}

// TestPublishedExtensionUsagesLoad drives the extension usages the IETF
// publishes, with each extension declared locally: RFC 7952's md:annotation
// holding a type and a description, RFC 8791's sx:structure holding a
// keyless list, RFC 8040's rc:yang-data holding a uses, and RFC 8341's
// nacm:default-deny-write with no block. A grammar check that refused one
// would refuse a valid RFC module.
//
// VALIDATES: the Section 14 check accepts the published extension usages.
func TestPublishedExtensionUsagesLoad(t *testing.T) {
	requireModuleLoaded(t, "published usages", `module m { namespace "urn:m"; prefix m;
		extension annotation { argument name; }
		extension structure { argument name; }
		extension yang-data { argument name; }
		extension default-deny-write;
		grouping errors { container errors { leaf error-tag { type string; } } }
		m:annotation last-modified { type string { length "1..64"; } description "The time of the last change."; }
		m:structure address-book { list address { key "last first"; leaf last { type string; } leaf first { type string; } } }
		m:structure log { list entry { leaf at { type string; } } }
		m:yang-data yang-errors { uses errors; }
		leaf a { type string; m:default-deny-write; }
	}`)
}
