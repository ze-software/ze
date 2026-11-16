// Design: docs/architecture/api/process-protocol.md -- peer dissemination declarations.
// Related: registry.go -- compiled and Stage 1 declarations share this judge.
package registry

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// CheckDisseminationDeclarations refuses incomplete or ambiguous role values
// before either compiled or external registration stores them. It grants no
// permission and never registers a wire-supplied event or role. Safe for
// concurrent use; input MUST remain unchanged until this call returns.
func CheckDisseminationDeclarations(name string, declarations []rpc.DisseminationDecl) error {
	// Runtime input is bounded by the IPC frame; compiled input is a fixed
	// registration value. No recursive or data-producing traversal occurs.
	seen := make(map[[2]string]bool, len(declarations))
	for i := range declarations {
		decl := &declarations[i]
		if !disseminationToken(decl.Role) {
			return fmt.Errorf("plugin %q dissemination role %q is not a nonempty token", name, decl.Role)
		}
		if !disseminationToken(decl.Namespace) {
			return fmt.Errorf("plugin %q dissemination role %q has invalid event namespace %q", name, decl.Role, decl.Namespace)
		}
		key := [2]string{decl.Namespace, decl.Role}
		if seen[key] {
			return fmt.Errorf("plugin %q declares dissemination role %q twice in namespace %q", name, decl.Role, decl.Namespace)
		}
		seen[key] = true
		if err := disseminationReceive(name, decl, "source", decl.SourceReceive); err != nil {
			return err
		}
		if err := disseminationReceive(name, decl, "destination", decl.DestinationReceive); err != nil {
			return err
		}
		if len(decl.DestinationSend) == 0 {
			return fmt.Errorf("plugin %q dissemination role %q declares no destination send grants", name, decl.Role)
		}
		for _, token := range decl.DestinationSend {
			if !disseminationToken(token) {
				return fmt.Errorf("plugin %q dissemination role %q has invalid send grant %q", name, decl.Role, token)
			}
			if token == events.TokenWildcard {
				return fmt.Errorf("plugin %q dissemination role %q must name the send grants it requires instead of *", name, decl.Role)
			}
		}
	}
	return nil
}

func disseminationReceive(name string, decl *rpc.DisseminationDecl, side string, tokens []string) error {
	if len(tokens) == 0 {
		return fmt.Errorf("plugin %q dissemination role %q declares no %s receive grants", name, decl.Role, side)
	}
	for _, token := range tokens {
		if !disseminationToken(token) {
			return fmt.Errorf("plugin %q dissemination role %q has invalid %s receive grant %q", name, decl.Role, side, token)
		}
		if token == events.TokenWildcard {
			return fmt.Errorf("plugin %q dissemination role %q must name the %s receive grants it requires instead of *", name, decl.Role, side)
		}
	}
	return nil
}

// CheckDisseminationEvents resolves required events after component
// initialization. Register cannot do this: Go's sibling init order must not
// decide whether a namespace exists yet. Stage 1 and role resolution MUST call
// this after CheckDisseminationDeclarations. Safe for concurrent use.
func CheckDisseminationEvents(name string, declarations []rpc.DisseminationDecl) error {
	for i := range declarations {
		decl := &declarations[i]
		if !events.IsValidNamespace(decl.Namespace) {
			return fmt.Errorf("plugin %q dissemination role %q names unknown event namespace %q", name, decl.Role, decl.Namespace)
		}
		for _, required := range [][]string{decl.SourceReceive, decl.DestinationReceive} {
			for _, token := range required {
				if _, _, known := events.SplitTypeToken(decl.Namespace, token); !known {
					return fmt.Errorf("plugin %q dissemination role %q names unknown receive grant %q", name, decl.Role, token)
				}
			}
		}
	}
	return nil
}

func disseminationToken(token string) bool {
	if token == "" {
		return false
	}
	return !strings.ContainsFunc(token, func(character rune) bool {
		return unicode.IsSpace(character) || unicode.IsControl(character)
	})
}
