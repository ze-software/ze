// Design: docs/architecture/web-interface.md -- Leaf set/delete and Workbench form save handlers
// Overview: handler_config.go -- Config tree view handlers
// Related: fragment.go -- HTMX fragment helpers (form field collection)

package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// handleConfigSet returns a POST handler for /config/set/<yang-path>/.
// It extracts the authenticated username from the request context, parses
// the form body for "leaf" (field name) and "value" (new value), and calls
// mgr.SetValue. For TypeBool leaves (checkboxes), the presence of the field
// is treated as "true" and absence as "false".
//
// On success, redirects one level up in the path hierarchy.
// On validation error from SetValue, returns an error notification.
// HTMX requests receive HX-Redirect instead of an HTTP redirect.
func handleConfigSet(mgr *EditorManager, schema *config.Schema, renderer *Renderer) http.HandlerFunc {
	return HandleConfigSetWithAuthorizer(mgr, schema, renderer, nil)
}

// HandleConfigSetWithAuthorizer returns a POST handler for /config/set/<yang-path>/
// that enforces profile-based RBAC before mutating the user's draft.
func HandleConfigSetWithAuthorizer(mgr *EditorManager, schema *config.Schema, renderer *Renderer, authorizer aaa.Authorizer) http.HandlerFunc {
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
		if !authorizeWebConfigMutation(w, r, authorizer, username, webCommandConfigSet) {
			return
		}

		parsed, err := ParseURL(r)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		path := parsed.Path

		r.Body = http.MaxBytesReader(w, r.Body, 65536)

		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("parse form: %v", err), http.StatusBadRequest)
			return
		}

		leaf := r.FormValue("leaf")
		if leaf == "" {
			http.Error(w, "missing leaf field name", http.StatusBadRequest)
			return
		}

		if err := ValidatePathSegments([]string{leaf}); err != nil {
			http.Error(w, "invalid leaf name", http.StatusBadRequest)
			return
		}

		value := r.FormValue("value")

		// __default__ means "delete this leaf, revert to YANG default".
		// Deliberately DeleteValue, not the schema-aware DeleteByPath used by
		// /config/delete/: this is the SET endpoint and "leaf" is unchecked form
		// input, so DeleteByPath would let `leaf=peer&value=__default__` erase a
		// whole list here. Reverting a leaf to its default is exactly a leaf op.
		if value == "__default__" {
			if err := mgr.DeleteValue(username, path, leaf); err != nil {
				errPath := textbuf.Join(append(path, leaf), "/")
				if renderer != nil {
					WriteOOBError(w, renderer, errPath, err.Error(), http.StatusBadRequest)
				} else {
					http.Error(w, fmt.Sprintf("delete value: %v", err), http.StatusBadRequest)
				}
				return
			}
		} else {
			// For boolean leaves, normalize to "true"/"false".
			// Toggle buttons send value=true/false explicitly.
			// HTML checkboxes send the field only when checked (legacy path).
			if isBoolLeaf(schema, path, leaf) {
				if value == boolTrue || value == "1" || value == "on" {
					value = boolTrue
				} else {
					value = boolFalse
				}
			}

			// Validate YANG type for the leaf.
			if leafNode := findLeafNode(schema, path, leaf); leafNode != nil {
				if valErr := config.ValidateValue(leafNode.Type, value); valErr != nil {
					errPath := textbuf.Join(append(path, leaf), "/")
					if renderer != nil {
						WriteOOBError(w, renderer, errPath, valErr.Error(), http.StatusBadRequest)
					} else {
						http.Error(w, valErr.Error(), http.StatusBadRequest)
					}
					return
				}
			}

			// Check unique constraints for inline table edits.
			if uniqueErr := validateUniqueOnSet(mgr.Tree(username), schema, path, leaf, value); uniqueErr != "" {
				errPath := textbuf.Join(append(path, leaf), "/")
				if renderer != nil {
					WriteOOBError(w, renderer, errPath, uniqueErr, http.StatusConflict)
				} else {
					http.Error(w, uniqueErr, http.StatusConflict)
				}
				return
			}

			if err := mgr.SetValue(username, path, leaf, value); err != nil {
				errPath := textbuf.Join(append(path, leaf), "/")
				if renderer != nil {
					WriteOOBError(w, renderer, errPath, err.Error(), http.StatusBadRequest)
				} else {
					http.Error(w, fmt.Sprintf("set value: %v", err), http.StatusBadRequest)
				}
				return
			}
		}

		// HTMX requests: return re-rendered field + OOB commit bar.
		if r.Header.Get("HX-Request") == htmxRequestTrue || r.Header.Get("X-Requested-With") == "fetch" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")

			// Re-render the field with updated value.
			if renderer != nil {
				effectiveValue := value
				if value == "__default__" {
					effectiveValue = ""
				}
				leafNode := findLeafNode(schema, path, leaf)
				if leafNode != nil {
					field := buildFieldMetaFromLeaf(leaf, leafNode, effectiveValue, textbuf.Join(path, "/"))
					fieldHTML := renderer.RenderField(field)
					if _, writeErr := w.Write([]byte(fieldHTML)); writeErr != nil {
						return
					}
				}
			}

			// OOB commit bar with change count.
			count := mgr.ChangeCount(username)
			oob := renderer.renderComponent("oob_save_ok", oobSaveOK(saveOK(r, authorizer, count)))
			if _, writeErr := w.Write([]byte(oob)); writeErr != nil {
				return
			}
			return
		}

		redirectBackOneLevel(w, r, path)
	}
}

type configFormField struct {
	path   []string
	leaf   string
	value  string
	delete bool
}

// HandleConfigFormWithAuthorizer returns a POST handler for Workbench form
// saves and enforces the same RBAC command as /config/set/.
func HandleConfigFormWithAuthorizer(mgr *EditorManager, schema *config.Schema, renderer *Renderer, authorizer aaa.Authorizer) http.HandlerFunc {
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
		if !authorizeWebConfigMutation(w, r, authorizer, username, webCommandConfigSet) {
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

		fields, fieldErr := parseConfigFormFields(r.PostForm, parsed.Path, schema)
		if fieldErr != nil {
			if renderer != nil {
				WriteOOBError(w, renderer, "config form", fieldErr.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, fieldErr.Error(), http.StatusBadRequest)
			}
			return
		}
		currentTree := mgr.Tree(username)

		// Cleared fields delete. DeleteValue (not the schema-aware DeleteByPath)
		// is correct here: parseConfigFormFields resolves every entry through
		// findLeafNode, so f.leaf is provably a leaf.
		for _, f := range fields {
			var setErr error
			if f.delete {
				var pathbuf textbuf.Buffer
				if currentTree == nil || getConfigValue(currentTree, pathbuf.Join(append(append([]string{}, f.path...), f.leaf), "/").String()) == "" {
					continue
				}
				setErr = mgr.DeleteValue(username, f.path, f.leaf)
			} else {
				setErr = mgr.SetValue(username, f.path, f.leaf, f.value)
			}
			if setErr != nil {
				errPath := textbuf.Join(append(f.path, f.leaf), "/")
				if renderer != nil {
					WriteOOBError(w, renderer, errPath, setErr.Error(), http.StatusBadRequest)
				} else {
					http.Error(w, setErr.Error(), http.StatusBadRequest)
				}
				return
			}
		}

		if r.Header.Get("HX-Request") == htmxRequestTrue {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			oob := renderer.renderComponent("oob_save_ok", oobSaveOK(saveOK(r, authorizer, mgr.ChangeCount(username))))
			if _, writeErr := w.Write([]byte(oob)); writeErr != nil {
				return
			}
			return
		}

		http.Redirect(w, r, backToRefererOrShow(r), http.StatusSeeOther)
	}
}

func parseConfigFormFields(form map[string][]string, basePath []string, schema *config.Schema) ([]configFormField, error) {
	fields := make([]configFormField, 0, len(form))
	for key, values := range form {
		if !strings.HasPrefix(key, "field:") || len(values) == 0 {
			continue
		}
		fieldPath := strings.TrimPrefix(key, "field:")
		path, leaf, leafNode, resolveErr := resolveConfigFormField(schema, basePath, fieldPath)
		if resolveErr != nil {
			return nil, resolveErr
		}
		fullPath := append(append([]string{}, path...), leaf)

		value := strings.TrimSpace(lastFormValue(values))
		// A secret input is prefilled with the display placeholder. Two
		// producers write it: maskSecretLeaf (secret.go), where the schema node
		// is in hand, and the masked tree renderPageContent
		// (workbench_pages.go) hands a purpose-built page. Posting it back
		// means the operator did not touch the field. It is neither an edit nor
		// a delete. Without this the stored secret would be overwritten with
		// the placeholder text. EditorManager.SetValue guards the same value on
		// the other write path, and the two must stay in step.
		if value == config.SecretDataPlaceholder {
			continue
		}
		if leafNode.Type == config.TypeBool {
			value = normalizeBoolFormValue(value)
		}
		if value == "" {
			fields = append(fields, configFormField{path: path, leaf: leaf, delete: true})
			continue
		}
		if valErr := config.ValidateValue(leafNode.Type, value); valErr != nil {
			return nil, fmt.Errorf("invalid %s: %w", textbuf.Join(fullPath, "/"), valErr)
		}
		fields = append(fields, configFormField{path: path, leaf: leaf, value: value})
	}
	return fields, nil
}

func resolveConfigFormField(schema *config.Schema, basePath []string, fieldPath string) ([]string, string, *config.LeafNode, error) {
	parts := strings.Split(fieldPath, "/")
	if err := ValidatePathSegments(parts); err != nil {
		return nil, "", nil, fmt.Errorf("invalid config form field %q: %w", fieldPath, err)
	}
	fullPath := append(append([]string{}, basePath...), parts...)
	if len(fullPath) == 0 {
		return nil, "", nil, fmt.Errorf("config form field %q has no leaf", fieldPath)
	}
	path := append([]string{}, fullPath[:len(fullPath)-1]...)
	leaf := fullPath[len(fullPath)-1]
	if leafNode := findLeafNode(schema, path, leaf); leafNode != nil {
		return path, leaf, leafNode, nil
	}
	if strings.Contains(fieldPath, "/") {
		return nil, "", nil, fmt.Errorf("config form field %q is not a leaf", textbuf.Join(fullPath, "/"))
	}
	flattened, leafNode := resolveFlattenedFormField(schema, basePath, fieldPath)
	if leafNode == nil {
		return nil, "", nil, fmt.Errorf("config form field %q is not a leaf", textbuf.Join(fullPath, "/"))
	}
	return flattened[:len(flattened)-1], flattened[len(flattened)-1], leafNode, nil
}

func resolveFlattenedFormField(schema *config.Schema, basePath []string, fieldName string) ([]string, *config.LeafNode) {
	baseNode, err := walkSchema(schema, basePath)
	if err != nil {
		return nil, nil
	}
	getter, ok := baseNode.(schemaGetter)
	if !ok {
		return nil, nil
	}
	tokens := strings.Split(fieldName, "-")
	suffix, leaf := resolveFlattenedFromNode(getter, tokens)
	if leaf == nil {
		return nil, nil
	}
	fullPath := append(append([]string{}, basePath...), suffix...)
	return fullPath, leaf
}

func resolveFlattenedFromNode(node schemaGetter, tokens []string) ([]string, *config.LeafNode) {
	for end := len(tokens); end >= 1; end-- {
		name := strings.Join(tokens[:end], "-")
		child := node.Get(name)
		if child == nil {
			continue
		}
		if end == len(tokens) {
			if leaf, ok := child.(*config.LeafNode); ok {
				return []string{name}, leaf
			}
			continue
		}
		getter, ok := child.(schemaGetter)
		if !ok {
			continue
		}
		rest, leaf := resolveFlattenedFromNode(getter, tokens[end:])
		if leaf != nil {
			return append([]string{name}, rest...), leaf
		}
	}
	return nil, nil
}

func lastFormValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

func normalizeBoolFormValue(value string) string {
	if value == boolTrue || value == "1" || value == "on" {
		return boolTrue
	}
	return boolFalse
}

func backToRefererOrShow(r *http.Request) string {
	const fallback = "/show/"
	ref := r.Referer()
	if ref == "" {
		return fallback
	}
	u, err := url.Parse(ref)
	// isSameOriginPath is the single guard for every request-derived redirect
	// target (see auth.go). The inline check this replaced rejected "//host" but
	// not "/\host", which several browsers normalize to "//host" -- the exact
	// case the shared guard exists to catch.
	if err != nil || !isSameOriginPath(u.Path) {
		return fallback
	}
	return u.Path
}

// handleConfigDelete returns a POST handler for /config/delete/<yang-path>/.
// It extracts the authenticated username, parses the form body for "leaf",
// and calls mgr.DeleteByPath to remove whatever the schema says lives there.
//
// On success, redirects one level up. HTMX support mirrors handleConfigSet.
func handleConfigDelete(mgr *EditorManager) http.HandlerFunc {
	return HandleConfigDeleteWithAuthorizer(mgr, nil)
}

// HandleConfigDeleteWithAuthorizer returns a POST handler for /config/delete/<yang-path>/
// that enforces profile-based RBAC before mutating the user's draft.
func HandleConfigDeleteWithAuthorizer(mgr *EditorManager, authorizer aaa.Authorizer) http.HandlerFunc {
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
		if !authorizeWebConfigMutation(w, r, authorizer, username, webCommandConfigDelete) {
			return
		}

		parsed, err := ParseURL(r)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		path := parsed.Path

		r.Body = http.MaxBytesReader(w, r.Body, 65536)

		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("parse form: %v", err), http.StatusBadRequest)
			return
		}

		leaf := r.FormValue("leaf")
		if leaf == "" {
			http.Error(w, "missing leaf field name", http.StatusBadRequest)
			return
		}

		if err := ValidatePathSegments([]string{leaf}); err != nil {
			http.Error(w, "invalid leaf name", http.StatusBadRequest)
			return
		}

		// Schema-aware: "leaf" is whatever the UI addressed -- a leaf, a
		// container, a whole list, or (the list-table delete button) one list
		// entry key. DeleteValue can only remove a scalar leaf: Tree.Delete
		// early-returns for anything held in t.lists, so a peer delete returned
		// "path not found" and the browser silently swallowed it.
		if err := mgr.DeleteByPath(username, path, leaf); err != nil {
			var tb textbuf.Buffer
			tb.Str("delete ").Join(path, " ")
			if len(path) > 0 {
				tb.Byte(' ')
			}
			tb.Str(leaf).Str(": ").Err(err)
			http.Error(w, tb.String(), http.StatusBadRequest)
			return
		}

		redirectBackOneLevel(w, r, path)
	}
}
