// Design: docs/architecture/core-design.md -- le's namespaces describe themselves
// Related: leroot.go -- Register, the area registration a namespace sits above
// Related: tree.go -- the menu node that shows a namespace's summary
// Related: manifest.go -- the root page block that shows it
//
// A namespace is the first word several areas share (`spec`, `test`, `go`).
// It holds commands without being one, so no area registration describes it.
// Its one sentence is declared here, by the package that owns the namespace,
// and every surface that lists a namespace derives it from this registry
// (plan/post-release/engineering/spec-le-cli-usability.md, D-P7-namespace).
package leroot

import (
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// namespaceSummaries holds the summary every namespace registered.
// Registration runs in init() and every read happens after it, but help runs
// on any goroutine, so the map is guarded. Safe for concurrent use.
var namespaceSummaries struct {
	sync.RWMutex
	byName map[string]string
}

// RegisterNamespace records what the namespace name is for, in one sentence.
// The package that owns the namespace calls it from its register.go: the
// package at internal/le/<name>, the path the namespace's areas sit under.
//
// A name registered twice is a programming error at init, so it panics. A name
// that is also an area is refused by the startup check rather than here,
// because the area's init may run after this one (requireEveryNamespaceHasASummary).
func RegisterNamespace(name, summary string) {
	if name == "" {
		panic("BUG: leroot.RegisterNamespace: empty name; see the init frame above for the namespace")
	}
	if summary == "" {
		panic("BUG: leroot.RegisterNamespace: empty summary for " + name + "; say in one sentence what the namespace is for")
	}
	namespaceSummaries.Lock()
	defer namespaceSummaries.Unlock()
	if namespaceSummaries.byName == nil {
		namespaceSummaries.byName = make(map[string]string, 32)
	}
	if _, taken := namespaceSummaries.byName[name]; taken {
		panic("BUG: leroot.RegisterNamespace: " + name + " is registered twice; one package owns a namespace")
	}
	namespaceSummaries.byName[name] = summary
}

// NamespaceSummary answers the summary a namespace registered, and whether it
// registered one.
func NamespaceSummary(name string) (string, bool) {
	namespaceSummaries.RLock()
	defer namespaceSummaries.RUnlock()
	summary, held := namespaceSummaries.byName[name]
	return summary, held
}

// registeredNamespaces answers every name RegisterNamespace recorded.
func registeredNamespaces() []string {
	namespaceSummaries.RLock()
	defer namespaceSummaries.RUnlock()
	names := make([]string, 0, len(namespaceSummaries.byName))
	for name := range namespaceSummaries.byName {
		names = append(names, name)
	}
	return names
}

// requireEveryNamespaceHasASummary panics naming the first namespace of areas
// that registered no summary, and the first namespace registration whose name
// is also an area. A namespace with no summary is a blank line in the menu, on
// the root page and in `le help ai`. A name that is both carries two
// descriptions of one menu entry: the area's own summary is the one its page,
// its manifest entry and its completion already show, so the area's wins and
// the namespace registration is the second declaration to delete. Either is a
// Ze defect only its author can fix, never an operating error.
func requireEveryNamespaceHasASummary(areas []string) {
	isArea := make(map[string]bool, len(areas))
	for _, name := range areas {
		isArea[name] = true
	}
	for _, name := range registeredNamespaces() {
		if !isArea[name] {
			continue
		}
		panic("BUG: le namespace \"" + name + "\" is also an area, so it is described twice: delete its " +
			"leroot.RegisterNamespace call; the area's own summary describes it")
	}
	for _, area := range areas {
		words := strings.Fields(area)
		for depth := 1; depth < len(words); depth++ {
			namespace := strings.Join(words[:depth], " ")
			if isArea[namespace] {
				continue
			}
			if _, held := NamespaceSummary(namespace); held {
				continue
			}
			var tb textbuf.Buffer
			panic(tb.Str("BUG: le namespace \"").Str(namespace).Str("\" has no summary: call leroot.RegisterNamespace(\"").
				Str(namespace).Str("\", \"<what it is for, in one sentence>\") from internal/le/").
				Str(strings.ReplaceAll(namespace, " ", "/")).Str("/register.go").String())
		}
	}
}

// namespaceLine answers the one line that describes namespace on a listing:
// the summary of the area of the same name when there is one, else the
// summary the namespace registered. The startup check guarantees one of them.
func namespaceLine(namespace string, areas []ManifestArea) string {
	for _, area := range areas {
		if area.Name == namespace {
			return area.Description
		}
	}
	summary, held := NamespaceSummary(namespace)
	if !held {
		panic("BUG: le namespace \"" + namespace + "\" has no summary; requireEveryNamespaceHasASummary admitted it")
	}
	return summary
}
