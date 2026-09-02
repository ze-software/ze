// Design: website/AI.md -- every field the live catalog publishes reaches a reader
package site

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// enrichedCommandCatalog is one command carrying every field the live catalog
// publishes, so one render exercises every branch a command surface can take.
//
// The values are realistic rather than arbitrary: `usage` is what
// command.UsageLine writes for the `grammar` beside it, so the test can state
// where each grammar token reaches a reader.
const enrichedCommandCatalog = `[{
 "path": "show test",
 "description": "Show the rows of the test table.",
 "long-help": "Each row is one entry of the test table, in the order the table holds them.",
 "mode": "read-only",
 "wire-method": "ze-test:rows",
 "backend": ["netlink", "vpp"],
 "task-support": "required",
 "args": [
  {"name": "family", "type": "enum", "values": ["ipv4", "ipv6"], "mandatory": true},
  {"name": "level", "type": "string"}
 ],
 "usage": "show test <ipv4|ipv6> [detail <level>]",
 "grammar": [
  {"text": "show", "kind": "keyword"},
  {"text": "test", "kind": "keyword"},
  {"text": "family", "values": ["ipv4", "ipv6"], "kind": "value"},
  {"text": "detail", "group": [{"text": "level", "kind": "value"}], "kind": "group"}
 ],
 "pipes": [{"name": "peer", "description": "Filter by peer", "takes-arg": true}],
 "operators": [{
  "name": "match", "class": "data", "available": "with-rows", "local-only": true,
  "description": "Keep the rows holding this text"
 }],
 "answer-shape": "tab",
 "address-fields": ["target"],
 "pipe-aliases": [{
  "name": "summary", "description": "The aggregate fields", "expansion": "display router-id"
 }],
 "subcommands": ["neighbor", "status"]
}]`

// catalogFieldRendering states where a reader meets one published catalog field.
//
// Reason says why the field is presented that way, and it is owed for every
// field: a surface that carries nothing has to say what carries it instead.
// Reference and Detail hold the words the surface publishes, and each one is
// asserted against BOTH that surface's page and its Markdown mirror, so a
// mirror that drops a fact the page shows is a failure.
type catalogFieldRendering struct {
	Reason    string
	Reference []string
	Detail    []string
}

// catalogFieldRenderings is the disposition of every field the live catalog
// publishes. A field the producer adds and this table does not name fails
// TestEveryPublishedCatalogFieldReachesAReader by that name.
var catalogFieldRenderings = map[string]catalogFieldRendering{
	"path": {
		Reason:    "the registry path is the command's identity and the anchor each surface is keyed on.",
		Reference: []string{"show test"},
		Detail:    []string{"show test"},
	},
	"description": {
		Reason:    "the description is what the command model says the command does.",
		Reference: []string{"Show the rows of the test table."},
		Detail:    []string{"Show the rows of the test table."},
	},
	"long-help": {
		Reason: "the long form is what the command's own page explains, and it is the half a " +
			"list row has no space for. The CLI reference is a table an operator scans, so " +
			"the long form is published on the detail page alone.",
		Detail: []string{"Each row is one entry of the test table"},
	},
	"mode": {
		Reason:    "the mode says whether an operator needs a running daemon.",
		Reference: []string{"Read-only"},
		Detail:    []string{"Read-only"},
	},
	"wire-method": {
		Reason: "the wire method is the name the dispatcher answers to. It serves a tool author " +
			"rather than an operator scanning 395 rows, so it is published on the detail page only.",
		Detail: []string{"ze-test:rows"},
	},
	"backend": {
		Reason:    "the backends say which data plane implements the command.",
		Reference: []string{"netlink"},
		Detail:    []string{"netlink"},
	},
	"task-support": {
		Reason:    "task support says whether the MCP server answers this command with a task handle.",
		Reference: []string{"required", "task handle"},
		Detail:    []string{"required", "task handle"},
	},
	"args.name": {
		Reason:    "an argument's name is what the operator supplies a value for.",
		Reference: []string{"family"},
		Detail:    []string{"family"},
	},
	"args.type": {
		Reason:    "the type says what shape of value the argument takes.",
		Reference: []string{"enum"},
		Detail:    []string{"enum"},
	},
	"args.values": {
		Reason: "the enumerated set is what the argument accepts. Without it a reader has to " +
			"guess what an argument of type enum takes.",
		Reference: []string{"one of", "ipv4"},
		Detail:    []string{"Values", "ipv4"},
	},
	"args.mandatory": {
		Reason:    "a reader who cannot see that an argument is owed will type the command without it.",
		Reference: []string{"required: yes"},
		Detail:    []string{"Required", "yes"},
	},
	"usage": {
		Reason:    "the usage line is the invocation form an operator types.",
		Reference: []string{"[detail <level>]"},
		Detail:    []string{"[detail <level>]"},
	},
	"grammar.text": {
		Reason: "grammar is the usage line as a token list, for a machine reader that must not " +
			"parse brackets out of a string. A person reads the line itself, which " +
			"command.UsageLine writes from these same tokens, so each token's text reaches " +
			"the reader there and a second rendering would state one fact twice.",
		Reference: []string{"show test"},
		Detail:    []string{"show test"},
	},
	"grammar.values": {
		Reason:    "a token's closed set reaches the reader inside the usage line the tokens produce.",
		Reference: []string{"ipv4"},
		Detail:    []string{"ipv4"},
	},
	"grammar.group": {
		Reason:    "a group's members reach the reader as one bracketed unit of the usage line.",
		Reference: []string{"[detail <level>]"},
		Detail:    []string{"[detail <level>]"},
	},
	"grammar.kind": {
		Reason: "the kind reaches the reader as the punctuation of the usage line: a value in " +
			"angle brackets, an optional group in square ones.",
		Reference: []string{"<ipv4"},
		Detail:    []string{"<ipv4"},
	},
	"pipes.name": {
		Reason:    "a command pipe is a filter this command alone accepts after a bar.",
		Reference: []string{"peer"},
		Detail:    []string{"peer"},
	},
	"pipes.description": {
		Reason:    "the description says what the pipe filters on.",
		Reference: []string{"Filter by peer"},
		Detail:    []string{"Filter by peer"},
	},
	"pipes.takes-arg": {
		Reason:    "a pipe that takes a value is written with the placeholder the operator fills.",
		Reference: []string{"<value>"},
		Detail:    []string{"<value>"},
	},
	"operators.name": {
		Reason:    "an operator is what the command accepts after a bar.",
		Reference: []string{"match"},
		Detail:    []string{"match"},
	},
	"operators.class": {
		Reason: "the class groups the operators for a reader learning the set. It belongs to the " +
			"operator rather than to the command, so it is published once, in the CLI " +
			"reference's operator table, rather than on each of 395 detail pages.",
		Reference: []string{"Row data"},
	},
	"operators.available": {
		Reason:    "availability says what an answer must hold before the operator applies.",
		Reference: []string{"With rows"},
		Detail:    []string{"on its rows"},
	},
	"operators.local-only": {
		Reason:    "a local-only operator is refused in a daemon-expanded chain.",
		Reference: []string{"Local process only"},
		Detail:    []string{"local process only"},
	},
	"operators.description": {
		Reason: "the description belongs to the operator rather than to the command, and is " +
			"published once beside its class, in the CLI reference's operator table.",
		Reference: []string{"Keep the rows holding this text"},
	},
	"answer-shape": {
		Reason:    "the declared shape is what decides which operators are always available.",
		Reference: []string{"tab"},
		Detail:    []string{"tab"},
	},
	"address-fields": {
		Reason:    "the address fields are what `resolve` and `origin` act on.",
		Reference: []string{"target"},
		Detail:    []string{"target"},
	},
	"pipe-aliases.name": {
		Reason:    "an alias is a chain the command names.",
		Reference: []string{"summary"},
		Detail:    []string{"summary"},
	},
	"pipe-aliases.description": {
		Reason:    "the description says what the alias answers with.",
		Reference: []string{"The aggregate fields"},
		Detail:    []string{"The aggregate fields"},
	},
	"pipe-aliases.expansion": {
		Reason:    "the expansion is the chain the alias stands for.",
		Reference: []string{"display router-id"},
		Detail:    []string{"display router-id"},
	},
	"subcommands": {
		Reason:    "the subcommands say what an operator can type after this command.",
		Reference: []string{"neighbor"},
		Detail:    []string{"neighbor"},
	},
}

// VALIDATES: every field the live command catalog publishes reaches a reader on
// both command surfaces, in the page and in its Markdown mirror.
//
// The field list is READ FROM THE PRODUCER'S SOURCE rather than listed here, so
// a field added to `cmd/ze/help_command.go` fails this test on the day it is
// added rather than being decoded into nothing and dropped in silence. That is
// how the site came to publish neither `backend`, `task-support`, `subcommands`
// nor an argument's enumerated `values`: the model never named them, and no
// check compared the model against what the catalog carries.
func TestEveryPublishedCatalogFieldReachesAReader(t *testing.T) {
	published := publishedCatalogFields(t)
	for _, field := range published {
		if _, stated := catalogFieldRenderings[field]; !stated {
			t.Errorf("the live catalog publishes %q and no surface states what it does with it: "+
				"render it, then add its row to catalogFieldRenderings", field)
		}
	}
	for field := range catalogFieldRenderings {
		if !slices.Contains(published, field) {
			t.Errorf("catalogFieldRenderings states %q, which the live catalog no longer publishes", field)
		}
	}
	if t.Failed() {
		return
	}

	reference, referenceMirror, detail, detailMirror := renderEnrichedCommandSurfaces(t)
	for _, field := range published {
		rendering := catalogFieldRenderings[field]
		if rendering.Reason == "" {
			t.Errorf("%q states no reason for the way it is presented", field)
		}
		if len(rendering.Reference) == 0 && len(rendering.Detail) == 0 {
			t.Errorf("%q reaches no reader on either command surface", field)
			continue
		}
		assertEvidence(t, field, "the CLI reference", rendering.Reference, reference)
		assertEvidence(t, field, "the CLI reference mirror", rendering.Reference, referenceMirror)
		assertEvidence(t, field, "the detail page", rendering.Detail, detail)
		assertEvidence(t, field, "the detail mirror", rendering.Detail, detailMirror)
	}
}

// assertEvidence reports each word one surface owes and does not carry.
func assertEvidence(t *testing.T, field, surface string, evidence []string, text string) {
	t.Helper()
	for _, want := range evidence {
		if !strings.Contains(text, want) {
			t.Errorf("%s does not carry %q, which is how a reader meets %q", surface, want, field)
		}
	}
}

// renderEnrichedCommandSurfaces publishes both command surfaces from the
// enriched catalog and answers what a reader sees on each: the visible text of
// the page, and the Markdown mirror beside it.
func renderEnrichedCommandSurfaces(t *testing.T) (reference, referenceMirror, detail, detailMirror string) {
	t.Helper()
	paths := commandSurfacePaths(t)
	writeCatalog(t, paths.Output, enrichedCommandCatalog)
	writeEquivalentMapping(t, paths.Output)

	if _, err := renderCLIReference(paths); err != nil {
		t.Fatal(err)
	}
	if _, err := renderCommandEquivalents(paths); err != nil {
		t.Fatal(err)
	}
	detailDirectory := equivalentsDirectory + "/" + commandSlug("show test") + "/"
	return visibleText(mainContent(t, readArtifact(t, paths.Output, cliReferenceDest))),
		readArtifact(t, paths.Output, strings.TrimSuffix(cliReferenceDest, pageIndexFile)+pageMirrorFile),
		visibleText(mainContent(t, readArtifact(t, paths.Output, detailDirectory+pageIndexFile))),
		readArtifact(t, paths.Output, detailDirectory+pageMirrorFile)
}

// vendorOnlyMapping is the smallest vendor map the detail page renders against:
// one vendor and no curated intent, so the page states what Ze itself says
// about the command and nothing else.
//
// It is what every caller of writeEquivalentMapping needs, so the map is stated
// once here rather than passed in. Two callers declared the same bytes under two
// names until 2026-09-02.
const vendorOnlyMapping = `{"schema-version": 1,
 "vendors": {"vyos": {"label": "VyOS", "short-label": "VyOS"}}, "entries": []}`

// writeEquivalentMapping replaces the curated vendor map of one artifact with
// the vendor-only map above.
func writeEquivalentMapping(t *testing.T, output string) {
	t.Helper()
	path := filepath.Join(output, filepath.FromSlash(equivalentsFile))
	if err := os.WriteFile(path, []byte(vendorOnlyMapping), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The two files that between them spell the published catalog: the producer,
// and the package that owns the grammar token it embeds.
var catalogProducerFiles = []string{
	"cmd/ze/help_command.go",
	"internal/component/command/usage.go",
}

// publishedCatalogFields answers every JSON field the live command catalog
// publishes, as dotted paths such as `args.values`.
//
// The shape is read from the producer's SOURCE because this package cannot
// import it: `cmd/ze` is package main behind the `ze_core` build tag. Reading
// the source is what makes the answer follow the producer instead of following
// a copy somebody remembered to update.
func publishedCatalogFields(t *testing.T) []string {
	t.Helper()
	root := repositoryRoot(t)
	structures := make(map[string]*ast.StructType, 16)
	for _, file := range catalogProducerFiles {
		collectStructures(t, filepath.Join(root, filepath.FromSlash(file)), structures)
	}
	entry, found := structures["commandEntry"]
	if !found {
		t.Fatalf("cmd/ze/help_command.go declares no commandEntry: the catalog's shape moved, "+
			"and %v is where this test looks for it", catalogProducerFiles)
	}
	var fields []string
	appendFieldPaths(t, structures, entry, "", []string{"commandEntry"}, &fields)
	return fields
}

// collectStructures reads one file and indexes every struct type it declares.
func collectStructures(t *testing.T, path string, structures map[string]*ast.StructType) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("read the catalog producer %s: %v", path, err)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		declaration, isType := node.(*ast.TypeSpec)
		if !isType {
			return true
		}
		if structure, isStruct := declaration.Type.(*ast.StructType); isStruct {
			structures[declaration.Name.Name] = structure
		}
		return true
	})
}

// appendFieldPaths walks one struct and records the JSON path of every field.
//
// A field whose type is another struct of the catalog contributes its own
// fields under a dotted prefix. The recursion is bounded by `seen`, which holds
// the types on the path: UsageToken carries a slice of itself, so a group token
// is recorded as one leaf rather than descended into forever.
func appendFieldPaths(
	t *testing.T, structures map[string]*ast.StructType,
	structure *ast.StructType, prefix string, seen []string, fields *[]string,
) {
	t.Helper()
	for _, field := range structure.Fields.List {
		name := jsonFieldName(t, field)
		if name == "" {
			continue
		}
		path := prefix + name
		nested, isNested := structures[elementTypeName(field.Type)]
		if !isNested || slices.Contains(seen, elementTypeName(field.Type)) {
			*fields = append(*fields, path)
			continue
		}
		appendFieldPaths(t, structures, nested, path+".",
			append(seen, elementTypeName(field.Type)), fields)
	}
}

// jsonFieldName answers the key one field publishes, and "" for a field the
// catalog does not carry.
func jsonFieldName(t *testing.T, field *ast.Field) string {
	t.Helper()
	if field.Tag == nil {
		return ""
	}
	literal, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		t.Fatalf("read a struct tag of the catalog producer: %v", err)
	}
	name, _, _ := strings.Cut(reflect.StructTag(literal).Get("json"), ",")
	if name == "-" {
		return ""
	}
	return name
}

// elementTypeName answers the name of one field's type, looking through a slice
// and through the package qualifier of an imported type.
func elementTypeName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.ArrayType:
		return elementTypeName(typed.Elt)
	case *ast.StarExpr:
		return elementTypeName(typed.X)
	case *ast.SelectorExpr:
		return typed.Sel.Name
	case *ast.Ident:
		return typed.Name
	default:
		return ""
	}
}
