// Design: docs/architecture/web-components.md -- graphical collection saves
// Related: graphical_values.go -- typed render data

package web

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// handleGraphicalValues is reached only after the set endpoint authenticated,
// authorized and bounded the request body. The schema decides the editable kind.
func handleGraphicalValues(w http.ResponseWriter, r *http.Request, mgr *EditorManager, schema *config.Schema, renderer *Renderer, authorizer aaa.Authorizer, username string, path []string, name string) {
	node := schema.LookupTokenPath(slices.Concat(path, []string{name}))
	values, err := graphicalValuesFromForm(r, path, name, node)
	if err == nil {
		err = mgr.replaceGraphicalValues(username, path, name, values)
	}
	if err != nil {
		var refusal *opererr.Error
		if existing, ok := errors.AsType[*opererr.Error](err); ok {
			refusal = existing
		} else {
			refusal = opererr.Wrap(err, "The collection could not be saved.",
				"The submitted rows were refused; the candidate configuration was not changed.",
				"Correct the row described in this message and press Save")
		}
		if renderer != nil {
			// The operator must be able to correct the current rows. HTMX 4
			// swaps 400 responses too, so suppress only the main error swap.
			if r.Header.Get("HX-Request") == htmxRequestTrue {
				w.Header().Set("HX-Reswap", "none")
			}
			WriteOOBError(w, renderer, strings.Trim(values.Path+"/"+name, "/"), refusal.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, refusal.Error(), http.StatusBadRequest)
		}
		return
	}
	if renderer == nil {
		htmxRedirect(w, r, showPathPrefix+strings.Trim(values.Path+"/"+name, "/")+"/")
		return
	}
	current := buildGraphicalValues(path, name, node, walkTree(mgr.Tree(username), schema, path))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write([]byte(renderer.renderComponent("graphical values", graphicalValues(current, false)))); err != nil {
		return
	}
	_, _ = w.Write([]byte(renderer.renderComponent("oob_save_ok", oobSaveOK(saveOK(r, authorizer, mgr.ChangeCount(username))))))
}

func graphicalValuesFromForm(r *http.Request, path []string, name string, node config.Node) (GraphicalValues, error) {
	v := buildGraphicalValues(path, name, node, nil)
	switch node.(type) {
	case *config.FreeformNode:
		v.Freeform = true
		keys, values := r.PostForm["entry"], r.PostForm["entry-value"]
		if len(keys) != len(values) {
			return v, graphicalRefusal("The entry names and values do not match.")
		}
		for i, key := range keys {
			if strings.TrimSpace(key) == "" {
				return v, graphicalRefusal("An entry has no name.")
			}
			value := values[i]
			if value == "" {
				value = boolTrue
			}
			v.Entries = append(v.Entries, GraphicalEntry{Name: key, Value: value})
		}
	case *config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
		inactive := r.PostForm["member-inactive"]
		for i, member := range r.PostForm["member"] {
			if member == "" {
				return v, graphicalRefusal("A member has no value.")
			}
			state := GraphicalMember{Value: member}
			if i < len(inactive) {
				state.Inactive = inactive[i] == boolTrue
			}
			v.Members = append(v.Members, state)
		}
	default:
		return v, graphicalRefusal("This setting is not a collection or a freeform block.")
	}
	if len(v.Members)+len(v.Entries) > 1024 {
		return v, graphicalRefusal("The editor accepts at most 1024 rows in one save.")
	}
	operation := r.FormValue("operation")
	if operation == "clear" {
		v.Members, v.Entries = nil, nil
		return v, nil
	}
	if operation == "add" {
		if v.Freeform {
			key := r.FormValue("new-entry")
			if strings.TrimSpace(key) == "" {
				return v, graphicalRefusal("The new entry has no name.")
			}
			value := r.FormValue("new-entry-value")
			if value == "" {
				value = boolTrue
			}
			v.Entries = append(v.Entries, GraphicalEntry{Name: key, Value: value})
		} else {
			value := r.FormValue("new-member")
			if value == "" {
				return v, graphicalRefusal("Choose or enter the new member's value.")
			}
			v.Members = append(v.Members, GraphicalMember{Value: value})
		}
		return v, validateGraphicalValues(node, path, v)
	}
	if operation == "save" {
		return v, validateGraphicalValues(node, path, v)
	}
	action, position, hasPosition := strings.Cut(operation, ":")
	index, err := strconv.Atoi(position)
	if !hasPosition {
		return v, graphicalRefusal("The row action is invalid.")
	}
	if err != nil {
		return v, graphicalRefusal("The row action is invalid.")
	}
	count := len(v.Members)
	if v.Freeform {
		count = len(v.Entries)
	}
	if index < 0 {
		return v, graphicalRefusal("The selected row no longer exists.")
	}
	if index >= count {
		return v, graphicalRefusal("The selected row no longer exists.")
	}
	if action == "remove" {
		if v.Freeform {
			v.Entries = slices.Delete(v.Entries, index, index+1)
		} else {
			v.Members = slices.Delete(v.Members, index, index+1)
		}
		return v, validateGraphicalValues(node, path, v)
	}
	if v.Freeform {
		return v, graphicalRefusal("Freeform entries do not have a configurable order.")
	}
	if !v.MemberActivation {
		switch action {
		case verbActivate, verbDeactivate:
			return v, graphicalWholeSettingActivationRefusal()
		}
	}
	switch action {
	case verbActivate:
		v.Members[index].Inactive = false
		return v, validateGraphicalValues(node, path, v)
	case verbDeactivate:
		v.Members[index].Inactive = true
		return v, validateGraphicalValues(node, path, v)
	}
	target := index
	switch action {
	case "up":
		target--
	case "down":
		target++
	default:
		return v, graphicalRefusal("The row action is unknown.")
	}
	if target < 0 {
		return v, graphicalRefusal("The member is already at that end of the list.")
	}
	if target >= count {
		return v, graphicalRefusal("The member is already at that end of the list.")
	}
	v.Members[index], v.Members[target] = v.Members[target], v.Members[index]
	return v, validateGraphicalValues(node, path, v)
}

func validateGraphicalValues(node config.Node, path []string, values GraphicalValues) error {
	if len(values.Members)+len(values.Entries) > 1024 {
		return graphicalRefusal("The editor accepts at most 1024 rows in one save.")
	}
	if values.Freeform {
		seen := make(map[string]bool, len(values.Entries))
		for _, entry := range values.Entries {
			if seen[entry.Name] {
				return graphicalRefusal("Two entries have the same name.")
			}
			seen[entry.Name] = true
		}
		return nil
	}
	if len(values.Members) == 0 {
		return nil
	}
	unique := true
	switch n := node.(type) {
	case *config.ValueOrArrayNode:
		unique = !n.Ordered
	case *config.BracketLeafListNode:
		unique = !n.Ordered
	case *config.MultiLeafNode:
		unique = false
		for _, member := range values.Members {
			if member.Inactive {
				return graphicalWholeSettingActivationRefusal()
			}
		}
	}
	seen := make(map[string]bool, len(values.Members))
	members := make([]string, len(values.Members))
	for i, member := range values.Members {
		if unique && seen[member.Value] {
			return graphicalRefusal("This collection cannot contain the same member twice.")
		}
		seen[member.Value] = true
		members[i] = member.Value
	}
	return config.ValidateNodeValue(node, slices.Concat(path, []string{values.Name}), config.JoinLeafListMembers(members))
}

func graphicalRefusal(problem string) error {
	return opererr.New(problem, candidateUnchangedWhy, "Correct the row and press Save")
}

func graphicalWholeSettingActivationRefusal() error {
	return opererr.New("This setting supports activation of the whole setting only.",
		"Its words have no separate activation state. The candidate configuration was not changed.",
		"Open Configuration actions and use Activate item or Deactivate item")
}

// replaceGraphicalValues replaces one typed node in a cloned candidate, then
// publishes the difference through the editor's atomic load. The lock covers
// snapshot, validation and publication, so a concurrent field save cannot vanish.
func (m *EditorManager) replaceGraphicalValues(username string, path []string, name string, values GraphicalValues) error {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return err
	}
	us.mu.Lock()
	defer us.mu.Unlock()
	tree, ok := us.editor.Tree().(*config.Tree)
	if !ok {
		return fmt.Errorf("BUG: graphical save has no typed candidate")
	}
	if tree == nil {
		return fmt.Errorf("BUG: graphical save has an empty candidate")
	}
	working := tree.Clone()
	parent := walkTree(working, m.schema, path)
	if parent == nil {
		seed := graphicalSeed(path, name, values)
		if seed == "" {
			return nil
		}
		working, err = config.NewSetParser(m.schema).Parse(config.SerializeSet(working, m.schema) + seed)
		if err != nil {
			return err
		}
		parent = walkTree(working, m.schema, path)
		if parent == nil {
			return fmt.Errorf("BUG: parsed graphical setting has no parent")
		}
	}
	switch {
	case values.Freeform:
		entries := config.NewTree()
		for _, entry := range values.Entries {
			entries.Set(entry.Name, entry.Value)
		}
		if len(values.Entries) == 0 {
			parent.DeleteContainer(name)
		} else {
			parent.SetContainer(name, entries)
		}
	case len(values.Members) == 0:
		parent.Delete(name)
	default:
		states := make([]config.MemberState, len(values.Members))
		for i, member := range values.Members {
			states[i] = config.MemberState{Value: member.Value, Inactive: member.Inactive}
		}
		parent.SetMemberStates(name, states)
	}
	body := config.SerializeSet(working, m.schema)
	_, err = us.editor.ExecuteConfigCommand(contract.ConfigCommandRequest{Line: "load terminal absolute replace", Content: &body})
	return err
}

// graphicalSeed builds one quoted set statement to create an absent parent.
// It uses the actual first desired row, never a placeholder setting.
func graphicalSeed(path []string, name string, values GraphicalValues) string {
	words := slices.Concat(path, []string{name})
	if values.Freeform {
		if len(values.Entries) == 0 {
			return ""
		}
		words = append(words, values.Entries[0].Name, values.Entries[0].Value)
	} else {
		if len(values.Members) == 0 {
			return ""
		}
		words = append(words, values.Members[0].Value)
	}
	var b textbuf.Buffer
	b.Str("set")
	for _, word := range words {
		b.Byte(' ').Str(config.QuoteIfNeeded(word))
	}
	return b.Byte('\n').String()
}
