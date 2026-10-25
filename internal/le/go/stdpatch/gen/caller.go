// Design: docs/contributing/patching-the-standard-library.md -- the activation and what it leaves a program outside the checkout
// Related: stdpatch.go -- Activate, which records what this file restores

package gen

import (
	"os"
	"path/filepath"
	"strings"
)

// activatedNames are the three variables Activate sets, in the order it sets
// them. CallerEnvironment puts each back.
var activatedNames = [...]string{"GOROOT", "GOTOOLCHAIN", "PATH"}

// callerValue is one variable as the caller had it: its value, and whether it
// was set at all, because an unset GOROOT and an empty one are different
// instructions to the go command.
type callerValue struct {
	value string
	set   bool
}

// callerValues holds activatedNames as the caller had them, index for index.
type callerValues [len(activatedNames)]callerValue

// captureCaller reads activatedNames from this process's environment. Activate
// calls it before it changes any of them, under its own lock.
func captureCaller() callerValues {
	var values callerValues
	for index, name := range activatedNames {
		values[index].value, values[index].set = os.LookupEnv(name)
	}
	return values
}

// CallerEnvironment answers environ as the program that started le gave it,
// for a child that is not working on Ze: the GOROOT, GOTOOLCHAIN and PATH
// Activate replaced are restored to the caller's values, and a variable the
// caller did not set is removed. When Activate never ran in this process,
// environ already holds the caller's values and they are kept.
//
// A patched tree is then removed whatever put it there: a GOROOT that is a tree
// of root's cache, or one holding a file the patches add, and every PATH entry
// that is such a tree's bin. A le started by another le inherits both from its
// parent, and they are Ze's toolchain, never the caller's: gopls on another
// repository would analyze Ze's crypto/tls, and a `go` found through the tree's
// bin answers the tree as its root even with GOROOT unset.
//
// The error says root's patches could not be read, so no tree can be told from
// a real GOROOT; the caller MUST NOT start the program with an environment it
// cannot vouch for.
func CallerEnvironment(root string, environ []string) ([]string, error) {
	patches, err := Patches(root)
	if err != nil {
		return nil, err
	}
	caches := CacheDirs(root)
	isPatched := func(goroot string) bool {
		for _, cache := range caches {
			if isTree(goroot, cache, patches) {
				return true
			}
		}
		return false
	}

	activation.Lock()
	done, caller := activation.done, activation.caller
	activation.Unlock()

	restored := environ
	if done {
		restored = without(environ, activatedNames[:]...)
		for index, name := range activatedNames {
			if caller[index].set {
				restored = append(restored, name+"="+caller[index].value)
			}
		}
	}
	return withoutTrees(restored, isPatched), nil
}

// withoutTrees removes a GOROOT that isPatched claims, and every PATH entry
// whose parent isPatched claims. The PATH order of the remaining entries is
// kept, because the caller chose it.
func withoutTrees(environ []string, isPatched func(string) bool) []string {
	kept := make([]string, 0, len(environ))
	for _, entry := range environ {
		name, value, _ := strings.Cut(entry, "=")
		switch name {
		case "GOROOT":
			if value != "" && isPatched(value) {
				continue
			}
		case "PATH":
			entry = name + "=" + searchPathWithoutTrees(value, isPatched)
		}
		kept = append(kept, entry)
	}
	return kept
}

// searchPathWithoutTrees answers path with every entry that is a patched tree's bin
// removed.
func searchPathWithoutTrees(path string, isPatched func(string) bool) string {
	entries := filepath.SplitList(path)
	kept := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry != "" && filepath.Base(entry) == "bin" && isPatched(filepath.Dir(entry)) {
			continue
		}
		kept = append(kept, entry)
	}
	return strings.Join(kept, string(filepath.ListSeparator))
}
