// Design: docs/guide/l2tp.md -- explicit PPP authentication policy.
// Related: auth_events.go -- authentication method names at the config boundary.

package ppp

import (
	"fmt"
	"slices"
)

// ResolveAuthAllow parses an ordered set of permitted authentication methods.
// Empty names derive the method-dependent default. The returned slice is owned
// by the caller; callers MUST keep a published policy immutable so established
// sessions retain the generation with which they started. Safe for concurrent use.
func ResolveAuthAllow(configured AuthMethod, names []string) ([]AuthMethod, error) {
	switch configured {
	case AuthMethodNone, AuthMethodPAP, AuthMethodCHAPMD5, AuthMethodMSCHAPv2:
	default:
		return nil, fmt.Errorf("unknown configured authentication method %d; use none, pap, chap-md5 or ms-chap-v2", configured)
	}
	if len(names) == 0 {
		switch configured {
		case AuthMethodNone:
			return nil, nil
		case AuthMethodPAP:
			return []AuthMethod{AuthMethodCHAPMD5, AuthMethodPAP}, nil
		case AuthMethodCHAPMD5, AuthMethodMSCHAPv2:
			return []AuthMethod{configured}, nil
		}
	}
	if configured == AuthMethodNone {
		return nil, fmt.Errorf("authentication allow cannot accompany auth-method none; remove allow or choose an authenticated method")
	}
	policy := make([]AuthMethod, 0, len(names))
	for _, name := range names {
		method, err := ParseAuthMethod(name)
		if err != nil {
			return nil, fmt.Errorf("authentication allow: %w", err)
		}
		if method == AuthMethodNone {
			return nil, fmt.Errorf("authentication allow cannot contain none; use chap-md5, ms-chap-v2 or pap")
		}
		if !slices.Contains(policy, method) {
			policy = append(policy, method)
		}
	}
	if !slices.Contains(policy, configured) {
		return nil, fmt.Errorf("authentication allow must include auth-method %s; add it to allow or change auth-method", configured.String())
	}
	return policy, nil
}
