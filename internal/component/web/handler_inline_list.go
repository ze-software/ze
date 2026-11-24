// Design: docs/architecture/web-components.md -- inline list creation

package web

import (
	"errors"
	"net/http"
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/opererr"
)

// handleInlineListAdd follows the add endpoint's authentication and edit gate.
// Inline entries carry a typed key and reuse the normal candidate creator.
func handleInlineListAdd(w http.ResponseWriter, r *http.Request, mgr *EditorManager, schema *config.Schema, username string, path []string, node *config.InlineListNode) {
	key := r.FormValue("name")
	if key == "" {
		http.Error(w, graphicalRefusal("Enter the new entry's key.").Error(), http.StatusBadRequest)
		return
	}
	if err := ValidatePathSegments([]string{key}); err != nil {
		http.Error(w, graphicalRefusal("The entry key cannot be used in a configuration path.").Error(), http.StatusBadRequest)
		return
	}
	if err := config.ValidateValue(node.KeyType, key); err != nil {
		var refusal *opererr.Error
		if existing, ok := errors.AsType[*opererr.Error](err); ok {
			refusal = existing
		} else {
			refusal = opererr.Wrap(err, "The new entry key "+config.QuoteIfNeeded(key)+" is not accepted by this list.", "The entry was not added because its key does not match the list type.", "Enter a key that meets the type described in this message and press Add entry")
		}
		http.Error(w, refusal.Error(), http.StatusBadRequest)
		return
	}
	where := slices.Concat(path, []string{key})
	exists := false
	mgr.ReadTree(username, func(tree *config.Tree) {
		exists = walkTree(tree, schema, where) != nil
	})
	if exists {
		http.Error(w, graphicalRefusal("An entry with that key already exists.").Error(), http.StatusConflict)
		return
	}
	if err := mgr.CreateEntry(username, where); err != nil {
		var refusal *opererr.Error
		if existing, ok := errors.AsType[*opererr.Error](err); ok {
			refusal = existing
		} else {
			refusal = opererr.Wrap(err, "The entry "+config.QuoteIfNeeded(key)+" could not be added.", "The candidate configuration was not changed.", "Refresh the list, correct the entry key using this message and press Add entry again")
		}
		http.Error(w, refusal.Error(), http.StatusBadRequest)
		return
	}
	htmxRedirect(w, r, showPathPrefix+encodeConfigPath(where)+"/")
}
