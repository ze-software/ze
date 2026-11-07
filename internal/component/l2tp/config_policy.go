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
		return 0, fmt.Errorf("l2tp mru %q is not an integer from 68 to 1500; run 'set l2tp mru 1500': %w", value, err)
	}
	// RFC 791 Section 3.2: "Every internet module must be able to forward a datagram of 68
	// octets without further fragmentation."
	if mru < uint64(ppp.MinIPv4MRU) {
		return 0, fmt.Errorf("l2tp mru %d is below the IPv4 minimum 68; run 'set l2tp mru 1500'", mru)
	}
	if mru > 1500 {
		return 0, fmt.Errorf("l2tp mru %d exceeds 1500; run 'set l2tp mru 1500'", mru)
	}
	return uint16(mru), nil
}

// validateMRUNCP checks the effective protocol selection, including defaults.
func validateMRUNCP(mru uint16, ipv6Enabled bool) error {
	// RFC 791 Section 3.2 and RFC 5072 Section 2: apply the enabled protocols' MRU floors.
	if err := ppp.ValidateNetworkMRU(mru, ipv6Enabled); err != nil {
		return fmt.Errorf("l2tp mru: %w; run 'set l2tp mru 1500', or use an MRU of at least 68 with 'set l2tp ncp enable-ipv6cp false' for IPv4-only service", err)
	}
	return nil
}

// validateMRUPolicy MUST NOT mutate the original tree while checking defaults.
func validateMRUPolicy(tree *config.Tree) error {
	if tree == nil {
		return nil
	}
	root := tree.GetContainer("l2tp")
	if root == nil {
		return nil
	}
	mru := DefaultMRU
	if value, present := root.Get("mru"); present {
		parsed, err := parseMRU(value)
		if err != nil {
			return err
		}
		mru = parsed
	}
	ipv6Enabled := true
	if ncp := root.GetContainer("ncp"); ncp != nil {
		if value, present := ncp.Get("enable-ipv6cp"); present {
			ipv6Enabled = value == configTrue
		}
	}
	// RFC 791 Section 3.2 and RFC 5072 Section 2: validate the effective protocol selection.
	return validateMRUNCP(mru, ipv6Enabled)
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

// validateParameters runs the parser the daemon runs at startup over the
// original tree, so `ze config validate`, commit and load refuse what the L2TP
// subsystem would refuse at start: a relay or relay-realm binding naming an
// undeclared remote, a malformed or case-duplicated realm, a remote with no
// address, a dialing listener on the loopback. The two policy validators above
// stay, because they judge the explicit layer before defaults conceal it. It is
// safe for concurrent use and MUST NOT modify tree.
func validateParameters(tree *config.Tree) error {
	_, err := ExtractParameters(tree)
	return err
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
