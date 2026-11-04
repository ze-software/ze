// Design: docs/contributing/running-commands.md -- preserving an authorization retry's originating session.
package lepath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// KnownSessionID reads the canonical context or existing identity cache without
// creating a directory, minting an identity, or refreshing cache timestamps.
// An empty answer means no valid identity has yet been selected. Authorization
// refusals MUST use this lookup rather than ResolveSessionID, which may mint.
func KnownSessionID(root string) (string, error) {
	return knownSessionID(root, nativeProcess{})
}

func knownSessionID(root string, process sessionProcess) (string, error) {
	id, err := sessionIDFromContext(process)
	if err != nil {
		return "", err
	}
	if id != "" {
		return id, nil
	}
	cache := sessionIdentityCache(root, process)
	id, err = readCachedSessionID(cache)
	if errors.Is(err, os.ErrNotExist) {
		// No cache means no identity was selected before the refused action.
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read session identity cache %s: %w", cache, err)
	}
	return id, nil
}

// sessionIDFromContext is the shared precedence before any identity is minted.
func sessionIDFromContext(process sessionProcess) (string, error) {
	if raw := os.Getenv("CLAUDE_CODE_SESSION_ID"); raw != "" {
		if !safeSessionID(raw) {
			return "", fmt.Errorf("session scratch: unsafe session id %q", raw)
		}
		return raw, nil
	}
	if id := sessionIDFromProcess(process); id != "" {
		return id, nil
	}
	return sessionIDFromJWT(os.Getenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN")), nil
}

func sessionIdentityCache(root string, process sessionProcess) string {
	pid := sessionCLIAncestor(process)
	key := sessionCacheKey(process, pid)
	return filepath.Join(root, "tmp", sessionRoot, ".sid-by-pid-"+key)
}
