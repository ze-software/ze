// Design: docs/architecture/web-interface.md -- List entry add/rename handlers and add-form overlay
// Overview: handler_config.go -- Config tree view handlers

package web

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errPathTooShortForListEntry       = errors.New("path too short for list entry")
	errCannotRenameKeylessListEntries = errors.New("cannot rename keyless list entries")
	errPathDoesNotEndAtA              = errors.New("path does not end at a named list entry")
)

// suggestAutoDefault is written to a ze:suggest leaf the operator left blank
// when the leaf's YANG type accepts it (e.g. bgp peer connection/local/ip,
// a union of ip-address and the "auto" sentinel). See handleConfigAdd (F8).
const suggestAutoDefault = "auto"

// handleConfigAdd returns a POST handler for /config/add/<yang-path>/.
// It creates a list entry and sets any form field values.
// The entry key comes from the URL path (last segment) or the "name" form field.
// Form fields with "field:" prefix set values on the new entry (e.g., field:remote/ip=1.2.3.4).
// For HTMX requests, returns the updated list table fragment.
func handleConfigAdd(mgr *EditorManager, schema *config.Schema, renderer *Renderer) http.HandlerFunc {
	return HandleConfigAddWithAuthorizer(mgr, schema, renderer, nil)
}

// HandleConfigAddWithAuthorizer returns a POST handler for /config/add/<yang-path>/
// that enforces profile-based RBAC before mutating the user's draft.
func HandleConfigAddWithAuthorizer(mgr *EditorManager, schema *config.Schema, renderer *Renderer, authorizer aaa.Authorizer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !authorizeWebConfigMutation(w, r, authorizer, username, webCommandConfigAdd) {
			return
		}

		parsed, err := ParseURL(r)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 65536)
		if parseErr := r.ParseForm(); parseErr != nil {
			http.Error(w, "bad form data", http.StatusBadRequest)
			return
		}

		path := parsed.Path

		// Accept entry key from "name" form field (HTMX form) or URL path (JS).
		// Keys are always lowercase.
		if name := strings.ToLower(strings.TrimSpace(r.FormValue("name"))); name != "" {
			path = append(path, name)
		} else if len(path) > 0 {
			// Keyless list: auto-generate a sequential key.
			if listNode, ok := findListNode(schema, path); ok && listNode.KeyName == "" {
				existing := collectListKeys(mgr.Tree(username), schema, path)
				path = append(path, textbuf.StringInt(int64(len(existing)+1)))
			}
		}

		if len(path) < 2 {
			http.Error(w, "path must include list name and entry key", http.StatusBadRequest)
			return
		}

		// Validate all fields BEFORE creating the entry.
		entryKey := path[len(path)-1]
		listPath := path[:len(path)-1]
		tree := mgr.Tree(username)
		listNode, walkErr := walkSchema(schema, listPath)
		if walkErr != nil {
			http.Error(w, "invalid list path", http.StatusBadRequest)
			return
		}

		// Check duplicate key.
		if tree != nil && slices.Contains(collectListKeys(tree, schema, listPath), entryKey) {
			returnAddError(w, r, renderer, schema, mgr, username, listPath, fmt.Sprintf("entry %q already exists", entryKey))
			return
		}

		// Enforce ze:required fields: reject if missing and no inherited value.
		if ln, ok := listNode.(*config.ListNode); ok && len(ln.Required) > 0 {
			parentTree := resolveParentDefaults(tree, listPath)
			for _, reqPath := range ln.Required {
				fieldStr := textbuf.Join(reqPath, "/")
				var tb textbuf.Buffer
				formVal := strings.TrimSpace(r.FormValue(tb.Str("field:").Str(fieldStr).String()))
				inherited := resolveInheritedValue(parentTree, fieldStr)
				if formVal == "" && inherited == "" {
					returnAddError(w, r, renderer, schema, mgr, username, listPath,
						fmt.Sprintf("required field %q is missing", fieldStr))
					return
				}
			}
		}

		// Collect and validate field values before any mutation.
		type fieldSet struct {
			fieldPath, leaf, parentSuffix, value string
		}
		var fields []fieldSet
		for formKey, values := range r.PostForm {
			if !strings.HasPrefix(formKey, "field:") || len(values) == 0 {
				continue
			}
			fieldPath := strings.TrimPrefix(formKey, "field:")
			value := strings.TrimSpace(values[0])
			if value == "" {
				continue
			}

			// Validate value against YANG type.
			if ln, ok := listNode.(*config.ListNode); ok {
				fieldType := resolveLeafType(ln, fieldPath)
				if valErr := config.ValidateValue(fieldType, value); valErr != nil {
					returnAddError(w, r, renderer, schema, mgr, username, listPath,
						fmt.Sprintf("invalid %s: %v", fieldPath, valErr))
					return
				}
				// Check unique constraint.
				if conflict := checkUniqueConstraint(tree, schema, listPath, entryKey, fieldPath, value); conflict != "" {
					returnAddError(w, r, renderer, schema, mgr, username, listPath,
						fmt.Sprintf("duplicate %s %q (already used by %s)", fieldPath, value, conflict))
					return
				}
			}

			leaf, parentSuffix := splitFieldPath(fieldPath)
			fields = append(fields, fieldSet{fieldPath, leaf, parentSuffix, value})
		}

		// All validation passed. Create entry and set fields. The entry key is
		// carried by the entry path itself (CreateEntry records it); it must NOT
		// be written as a child leaf. Emitting `set <list> <key> <keyName> <key>`
		// produces e.g. "bgp peer lab-peer name lab-peer", which the hierarchical
		// parser rejects as "unknown field in peer: name" (the key leaf is not a
		// settable child of its own entry) -- the F8 add-peer commit failure.
		if createErr := mgr.CreateEntry(username, path); createErr != nil {
			var tb textbuf.Buffer
			returnAddError(w, r, renderer, schema, mgr, username, listPath, tb.Str("create entry: ").Err(createErr).String())
			return
		}
		for _, f := range fields {
			setPath := make([]string, len(path))
			copy(setPath, path)
			if f.parentSuffix != "" {
				setPath = append(setPath, strings.Split(f.parentSuffix, "/")...)
			}
			if setErr := mgr.SetValue(username, setPath, f.leaf, f.value); setErr != nil {
				serverLogger.Warn("add-entry set field failed", "field", f.fieldPath, "error", setErr)
			}
		}

		// Default any ze:suggest leaf the user left blank to "auto" when the
		// leaf's YANG type accepts it. connection/local/ip is suggested, but the
		// BGP reactor requires a value (IP address or "auto"); without this a
		// form-created peer commits to a config the reactor rejects with "local
		// ip is required" (F8). Schema-driven: only leaves whose union type
		// includes the "auto" sentinel are defaulted, so this generalises beyond
		// peers without hard-coding any plugin's field names.
		if ln, ok := listNode.(*config.ListNode); ok {
			provided := make(map[string]bool, len(fields))
			for _, f := range fields {
				provided[f.fieldPath] = true
			}
			for _, sp := range ln.Suggest {
				fieldPath := textbuf.Join(sp, "/")
				if provided[fieldPath] {
					continue
				}
				if config.ValidateValue(resolveLeafType(ln, fieldPath), suggestAutoDefault) != nil {
					continue // leaf type does not accept "auto"
				}
				leaf, parentSuffix := splitFieldPath(fieldPath)
				setPath := make([]string, len(path))
				copy(setPath, path)
				if parentSuffix != "" {
					setPath = append(setPath, strings.Split(parentSuffix, "/")...)
				}
				if setErr := mgr.SetValue(username, setPath, leaf, suggestAutoDefault); setErr != nil {
					serverLogger.Warn("add-entry default suggest field failed", "field", fieldPath, "error", setErr)
				}
			}
		}

		// HTMX: return updated list table + commit bar + finder for the parent list path.
		if r.Header.Get("HX-Request") == htmxRequestTrue {
			if r.FormValue("_workbench") == "1" {
				var tb textbuf.Buffer
				target := tb.Str("/show/").Join(listPath, "/").Byte('/').String()
				w.Header().Set("HX-Redirect", target)
				w.WriteHeader(http.StatusOK)
				return
			}
			// Keyless lists: redirect to the new entry so the page reloads with it.
			if ln, ok := listNode.(*config.ListNode); ok && ln.KeyName == "" {
				var tb textbuf.Buffer
				target := tb.Str("/show/").Join(path, "/").Byte('/').String()
				w.Header().Set("HX-Redirect", target)
				w.WriteHeader(http.StatusOK)
				return
			}

			tree = mgr.Tree(username)
			data := buildFragmentData(schema, tree, listPath)
			tableHTML := renderer.renderComponent("list_table", listTable(data))

			count := mgr.ChangeCount(username)
			commitHTML := renderer.renderComponent("oob_save_ok", oobSaveOK(saveOKData{ChangeCount: count}))

			// OOB finder update so the peer count refreshes.
			finderHTML := renderer.renderComponent("finder_oob", finderOOB(data))

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if _, writeErr := w.Write([]byte(string(tableHTML) + string(commitHTML) + string(finderHTML))); writeErr != nil {
				return
			}
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// handleConfigRename returns a POST handler for /config/rename/<entry-path>/.
// It renames a keyed list entry through the per-user session editor and then
// redirects back to the parent list view.
func handleConfigRename(mgr *EditorManager, schema *config.Schema) http.HandlerFunc {
	return HandleConfigRenameWithAuthorizer(mgr, schema, nil)
}

// HandleConfigRenameWithAuthorizer returns a POST handler for /config/rename/<entry-path>/
// that enforces profile-based RBAC before mutating the user's draft.
func HandleConfigRenameWithAuthorizer(mgr *EditorManager, schema *config.Schema, authorizer aaa.Authorizer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !authorizeWebConfigMutation(w, r, authorizer, username, webCommandConfigRename) {
			return
		}

		parsed, err := ParseURL(r)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 65536)
		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("parse form: %v", err), http.StatusBadRequest)
			return
		}

		newKey := strings.ToLower(strings.TrimSpace(r.FormValue("new-key")))
		if newKey == "" {
			http.Error(w, "missing new-key", http.StatusBadRequest)
			return
		}
		if err := ValidatePathSegments([]string{newKey}); err != nil {
			http.Error(w, "invalid new key", http.StatusBadRequest)
			return
		}

		parentPath, listName, oldKey, err := resolveNamedListEntryPath(schema, parsed.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		oldKey = strings.ToLower(oldKey)
		if newKey == oldKey {
			http.Error(w, "new key must differ from current key", http.StatusBadRequest)
			return
		}

		if err := mgr.RenameListEntry(username, parentPath, listName, oldKey, newKey); err != nil {
			status := http.StatusBadRequest
			if strings.Contains(err.Error(), "already exists") {
				status = http.StatusConflict
			}
			http.Error(w, err.Error(), status)
			return
		}

		redirectBackOneLevel(w, r, parsed.Path)
	}
}

func resolveNamedListEntryPath(schema *config.Schema, fullPath []string) (parentPath []string, listName, key string, err error) {
	if len(fullPath) < 2 {
		return nil, "", "", errPathTooShortForListEntry
	}

	var currentSchema schemaGetter = schema
	lastListIdx := -1

	i := 0
	for i < len(fullPath) {
		name := fullPath[i]
		schemaNode := currentSchema.Get(name)
		if schemaNode == nil {
			return nil, "", "", fmt.Errorf("unknown path element: %s", name)
		}

		switch n := schemaNode.(type) {
		case *config.ContainerNode:
			currentSchema = n
			i++
		case *config.ListNode:
			if n.KeyName == "" {
				return nil, "", "", errCannotRenameKeylessListEntries
			}
			if i+1 >= len(fullPath) {
				return nil, "", "", fmt.Errorf("list %s requires a key", name)
			}
			if n.Get(fullPath[i+1]) != nil {
				return nil, "", "", errPathDoesNotEndAtA
			}
			lastListIdx = i
			listName = name
			key = fullPath[i+1]
			currentSchema = n
			i += 2
		case *config.FlexNode:
			currentSchema = n
			i++
		default:
			return nil, "", "", errPathDoesNotEndAtA
		}
	}

	if lastListIdx == -1 || lastListIdx+2 != len(fullPath) {
		return nil, "", "", errPathDoesNotEndAtA
	}

	return fullPath[:lastListIdx], listName, key, nil
}

// returnAddError returns the list table with an error notification for HTMX add requests,
// or a plain HTTP error for non-HTMX requests.
func returnAddError(w http.ResponseWriter, r *http.Request, renderer *Renderer, schema *config.Schema, mgr *EditorManager, username string, listPath []string, errMsg string) {
	if r.Header.Get("HX-Request") != htmxRequestTrue {
		http.Error(w, errMsg, http.StatusConflict)
		return
	}
	tree := mgr.Tree(username)
	data := buildFragmentData(schema, tree, listPath)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tableHTML := renderer.renderComponent("list_table", listTable(data))
	notifHTML := renderer.renderComponent("notification_error", notificationError(notificationErrorData{Message: errMsg}))
	if _, writeErr := w.Write([]byte(string(tableHTML) + string(notifHTML))); writeErr != nil {
		return
	}
}

// HandleConfigAddForm returns a GET handler for /config/add-form/<yang-path>/.
// It renders an overlay form with inputs for the list key, required, suggest, and unique fields.
// Required and suggest fields are resolved from the config tree for inherited defaults.
func HandleConfigAddForm(mgr *EditorManager, schema *config.Schema, renderer *Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		parsed, err := ParseURL(r)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		listPath := parsed.Path
		if len(listPath) == 0 {
			http.Error(w, "path required", http.StatusBadRequest)
			return
		}

		schemaNode, walkErr := walkSchema(schema, listPath)
		if walkErr != nil {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		listNode, ok := schemaNode.(*config.ListNode)
		if !ok {
			http.Error(w, "not a list", http.StatusBadRequest)
			return
		}

		listName := strings.ToUpper(listPath[len(listPath)-1][:1]) + listPath[len(listPath)-1][1:]
		keyless := listNode.KeyName == ""
		displayKey := listNode.DisplayKey
		workbench := r.URL.Query().Get("ui") == uiModeTokenWorkbench
		heading := "New " + listName
		keyLabel := listNode.KeyName
		keyInputID := "field-" + formFieldID(keyLabel)
		if keyLabel == "" {
			keyLabel = "name"
			keyInputID = "field-name"
		}
		if workbench && len(listPath) == 2 && listPath[0] == "interface" && listPath[1] == "ethernet" {
			heading = "Add Interface"
			keyLabel = "Interface Name"
			keyInputID = "field-interface-name"
		}
		includeKeyField := workbench && listNode.KeyName != ""

		data := addFormData{
			AddURL: func() string {
				var tb textbuf.Buffer
				return tb.Str("/config/add/").Join(listPath, "/").Byte('/').String()
			}(),
			ListName:        listName,
			Heading:         heading,
			KeyName:         listNode.KeyName,
			KeyLabel:        keyLabel,
			KeyInputID:      keyInputID,
			Keyless:         keyless,
			DisplayKey:      displayKey,
			Workbench:       workbench,
			IncludeKeyField: includeKeyField,
		}

		// Resolve inherited defaults from parent context in the config tree.
		tree := mgr.Tree(username)
		parentTree := resolveParentDefaults(tree, listPath)

		// Track fields already added to avoid duplicates (required/suggest may overlap unique).
		seen := make(map[string]bool)

		for _, field := range collectRequiredFields(listNode) {
			seen[field] = true
			data.Fields = append(data.Fields, addFormField{
				Path:        field,
				Placeholder: resolveLeafDescription(listNode, field),
				Category:    "required",
				Inherited:   resolveInheritedValue(parentTree, field),
			})
		}

		for _, field := range collectSuggestFields(listNode) {
			if seen[field] {
				continue
			}
			seen[field] = true
			data.Fields = append(data.Fields, addFormField{
				Path:        field,
				Placeholder: resolveLeafDescription(listNode, field),
				Category:    "suggest",
				Inherited:   resolveInheritedValue(parentTree, field),
			})
		}

		for _, field := range collectUniqueFields(listNode) {
			if seen[field] {
				continue
			}
			data.Fields = append(data.Fields, addFormField{
				Path:        field,
				Placeholder: resolveLeafDescription(listNode, field),
				Category:    "unique",
			})
		}

		html := renderer.renderComponent("add_form_overlay", addFormOverlay(data))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, writeErr := w.Write([]byte(html)); writeErr != nil {
			return
		}
	}
}

// resolveParentDefaults walks the config tree to find the parent context for a list path.
// For bgp/group/<name>/peer, it returns the group entry tree (inheritable defaults).
// For bgp/peer, it returns the bgp container tree (bgp-level defaults).
// Returns nil if no tree or path is too short.
func resolveParentDefaults(tree *config.Tree, listPath []string) *config.Tree {
	if tree == nil || len(listPath) < 2 {
		return nil
	}
	// Walk to the parent of the list. Segments alternate between container/list
	// names and list entry keys.
	parentPath := listPath[:len(listPath)-1]
	current := tree
	for i := 0; i < len(parentPath); i++ {
		seg := parentPath[i]
		// Try container first.
		if child := current.GetContainer(seg); child != nil {
			current = child
			continue
		}
		// Try as a list name with the next segment as entry key.
		if i+1 < len(parentPath) {
			entries := current.GetList(seg)
			if entry, ok := entries[parentPath[i+1]]; ok {
				current = entry
				i++ // Skip the key segment.
				continue
			}
		}
		return nil
	}
	return current
}

// resolveInheritedValue looks up a slash-separated field path in a config tree.
// Returns the leaf value if found, empty string otherwise.
func resolveInheritedValue(tree *config.Tree, field string) string {
	if tree == nil {
		return ""
	}
	parts := strings.Split(field, "/")
	current := tree
	for i, part := range parts {
		if i == len(parts)-1 {
			val, exists := current.Get(part)
			if exists {
				return val
			}
			return ""
		}
		child := current.GetContainer(part)
		if child == nil {
			return ""
		}
		current = child
	}
	return ""
}
