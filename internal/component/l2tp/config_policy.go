// Design: docs/guide/l2tp.md -- PPP negotiation policy configuration.
// Related: config.go, subsystem_reload.go -- startup and reload boundaries.

package l2tp

import (
	"fmt"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// parseMRU validates Information-field octets, not PPP frame length.
func parseMRU(value string) (uint16, error) {
	mru, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("l2tp mru %q is not an integer from 64 to 1500; run 'set l2tp mru 1500': %w", value, err)
	}
	if mru < 64 {
		return 0, fmt.Errorf("l2tp mru %d is below 64; run 'set l2tp mru 64' or choose up to 1500", mru)
	}
	if mru > 1500 {
		return 0, fmt.Errorf("l2tp mru %d exceeds 1500; run 'set l2tp mru 1500'", mru)
	}
	return uint16(mru), nil
}

// validateAuthPolicy is safe for concurrent use and MUST NOT modify tree.
func validateAuthPolicy(tree *config.Tree) error {
	if tree == nil {
		return nil
	}
	root := tree.GetContainer("l2tp")
	if root == nil {
		return nil
	}
	method := DefaultAuthMethod
	if name, ok := root.Get("auth-method"); ok {
		parsed, err := ppp.ParseAuthMethod(name)
		if err != nil {
			return fmt.Errorf("l2tp auth-method: %w", err)
		}
		method = parsed
	}
	var names []string
	if auth := root.GetContainer("authentication"); auth != nil {
		names = auth.GetSlice("allow")
	}
	if _, err := ppp.ResolveAuthAllow(method, names); err != nil {
		return fmt.Errorf("l2tp authentication allow %v: %w; run 'delete l2tp authentication allow' to restore the method-dependent default", names, err)
	}
	if method == ppp.AuthMethodNone {
		allow, _ := root.Get("allow-no-auth")
		if allow != configTrue {
			return errL2tpAuthMethodNoneRequiresAllow
		}
	}
	return nil
}

// providerAuthAllow refuses malformed explicit policy rather than deriving a
// broader default from values that a permissive leaf-list decoder would drop.
func providerAuthAllow(root map[string]any) ([]string, error) {
	raw, present := root["authentication"]
	if !present {
		return nil, nil
	}
	auth, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("l2tp authentication value %#v is not a container; run 'delete l2tp authentication' then 'set l2tp authentication allow chap-md5'", raw)
	}
	raw, present = auth["allow"]
	if !present {
		return nil, nil
	}
	switch names := raw.(type) {
	case string:
		return []string{names}, nil
	case []string:
		return names, nil
	case []any:
		result := make([]string, len(names))
		for i, value := range names {
			name, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("l2tp authentication allow member %d value %#v is not a method name; run 'delete l2tp authentication allow' then 'set l2tp authentication allow chap-md5'", i+1, value)
			}
			result[i] = name
		}
		return result, nil
	default:
		return nil, fmt.Errorf("l2tp authentication allow value %#v is not a method list; run 'delete l2tp authentication allow' then 'set l2tp authentication allow chap-md5'", raw)
	}
}
