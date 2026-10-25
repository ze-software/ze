// VALIDATES: AC-53 (D-P7-namespace): every namespace le registers says what it is for, in the menu, on the root page and in `le help ai`
// PREVENTS: a sub-menu with no description, which the menu showed for every namespace before

package le

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/cli/menu"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// liveNamespaces answers every first word of the live tree that holds
// commands and is no area itself, with the summary it registered. It runs
// here rather than in package leroot because the registry is populated by the
// blank imports in register.go.
func liveNamespaces(t *testing.T) map[string]string {
	t.Helper()
	namespaces := map[string]string{}
	for name, node := range leroot.Tree().Children {
		if leroot.Owns(name) {
			continue
		}
		summary, named := leroot.NamespaceSummary(name)
		if !named || summary == "" {
			t.Errorf("namespace %q registered no summary: call leroot.RegisterNamespace(%q, ...) from internal/le/%s/register.go", name, name, name)
		}
		if node.ShortHelp != summary {
			t.Errorf("the tree's node for namespace %q says %q, want its summary %q", name, node.ShortHelp, summary)
		}
		namespaces[name] = summary
	}
	if len(namespaces) == 0 {
		t.Fatal("le registered no namespace; the check below would pass over an empty set")
	}
	return namespaces
}

// TestEveryNamespaceIsDescribedInTheMenu proves the menu le opens describes
// every sub-menu: each namespace's item on the first screen carries the
// summary its package registered, whole, and entering it heads the level with
// the namespace's long description (D-P7-short-long). Method: build the first
// screen and each namespace's level over the live tree, as openMenu does.
func TestEveryNamespaceIsDescribedInTheMenu(t *testing.T) {
	namespaces := liveNamespaces(t)
	tree := leroot.Tree()
	top := menu.DeclaredLevelOf(tree, nil)
	seen := 0
	for _, item := range top.Items {
		summary, isNamespace := namespaces[item.Name]
		if !isNamespace {
			continue
		}
		seen++
		if item.Desc == "" || item.Desc != summary {
			t.Errorf("the menu item for namespace %q says %q, want %q", item.Name, item.Desc, summary)
		}
		description, _ := leroot.NamespaceDescription(item.Name)
		if level := menu.DeclaredLevelOf(tree, item.Path); description == "" || level.Description != description {
			t.Errorf("entering namespace %q heads the level with %q, want its description %q", item.Name, level.Description, description)
		}
	}
	if seen != len(namespaces) {
		t.Errorf("the first screen lists %d of the %d namespaces", seen, len(namespaces))
	}
}

// TestEveryNamespaceIsDescribedOnTheRootPageAndInHelpAI proves the root page
// heads each namespace's block with its summary, and `le help ai commands`
// lists each namespace with it. Method: dispatch both as a reader types them
// and look for every live namespace.
func TestEveryNamespaceIsDescribedOnTheRootPageAndInHelpAI(t *testing.T) {
	namespaces := liveNamespaces(t)

	page, code := dispatchToStdout(t, "help")
	if code != 0 {
		t.Fatalf("le help answered %d, want 0", code)
	}
	// A heading wraps at the page width, so the page is read as words.
	words := strings.Join(strings.Fields(page), " ")
	for name, summary := range namespaces {
		heading := name + ": " + summary
		if !strings.Contains(page, "\n"+name+": ") || !strings.Contains(words, heading) {
			t.Errorf("the root page does not head namespace %q with %q", name, heading)
		}
	}

	out, code := dispatchToStdout(t, "help", "ai", "commands", "|", "json")
	if code != 0 {
		t.Fatalf("le help ai commands | json answered %d, want 0", code)
	}
	var reference struct {
		Namespaces []struct {
			Name        string `json:"name"`
			ShortHelp   string `json:"short-help"`
			Description string `json:"description"`
		} `json:"namespaces"`
	}
	if err := json.Unmarshal([]byte(out), &reference); err != nil {
		t.Fatalf("help ai is not JSON: %v", err)
	}
	listed := map[string]string{}
	long := map[string]string{}
	for _, entry := range reference.Namespaces {
		listed[entry.Name] = entry.ShortHelp
		long[entry.Name] = entry.Description
	}
	for name, summary := range namespaces {
		if listed[name] != summary {
			t.Errorf("help ai lists namespace %q as %q, want %q", name, listed[name], summary)
		}
		if description, _ := leroot.NamespaceDescription(name); description == "" || long[name] != description {
			t.Errorf("help ai describes namespace %q as %q, want %q", name, long[name], description)
		}
	}
}
