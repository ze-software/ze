// Design: docs/architecture/web-interface.md -- what a display path may render
// Related: handler_config_form.go -- parseConfigFormFields, the write half
// Related: editor.go -- EditorManager.SetValue, the other write half

package web

import (
	"github.com/ze-software/ze/internal/component/config"
)

// A stored secret must never reach a response body. type="password" hides the
// characters on screen and does nothing to the bytes. The page source, the
// browser cache and any proxy that reads the document all hold the value.
//
// The schema is the one authority on which leaf holds a secret, and
// config.LeafHoldsSecret is the one function that reads it. Masking on
// ze:bcrypt alone published every ze:sensitive leaf, l2tp/shared-secret and the
// IPsec pre-shared secret among them.
//
// Three shapes reach a display path, so three entry points cover them.
//
//   - The schema node is in hand: maskSecretLeaf. It also covers a value that
//     never came from the tree, such as the leaf a form post re-renders.
//   - A page builds its own view data from tree reads: config.MaskSecrets. The
//     leaf identity is gone by the time the value reaches a field, so the tree
//     the page reads is masked instead.
//   - A whole subtree is serialized as text, in the web CLI terminal:
//     config.MaskSecrets again, before the serializer runs.
//
// config.SecretDataPlaceholder is the masked form on every route, which keeps
// the placeholder pair intact. parseConfigFormFields (handler_config_form.go)
// and EditorManager.SetValue (editor.go) read the placeholder back as "the
// operator did not touch this field", and config.RejectMaskedSecretLeaves
// refuses it at commit, load and upload. An empty field still deletes the leaf,
// and a new value still writes.
//
// One value cannot survive the round trip: a secret whose real text IS the
// placeholder. The write half reads it as "unchanged" and the save stores
// nothing. The placeholder is 17 bytes of C comment, so no generated credential
// produces it. The alternative is a display path that leaks whenever an
// operator picks the wrong string.

// maskSecretLeaf answers the value a display path renders for one leaf. An
// empty value stays empty, so an unset secret renders no placeholder and the
// field reads as unconfigured.
func maskSecretLeaf(leaf *config.LeafNode, value string) string {
	if value == "" || !config.LeafHoldsSecret(leaf) {
		return value
	}

	return config.SecretDataPlaceholder
}
