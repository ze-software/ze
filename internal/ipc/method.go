// Design: docs/architecture/api/ipc_protocol.md — IPC framing and dispatch

package ipc

import (
	"fmt"
	"strings"
)

// MaxMethodLength is the maximum allowed method name length.
const MaxMethodLength = 256

// ParseMethod splits a "module:rpc-name" method string into its components.
// Returns an error if the format is invalid.
func ParseMethod(method string) (module, rpc string, err error) {
	if method == "" {
		return "", "", fmt.Errorf("empty method name")
	}
	if len(method) > MaxMethodLength {
		return "", "", fmt.Errorf("method name too long: %d > %d", len(method), MaxMethodLength)
	}

	var ok bool
	module, rpc, ok = strings.Cut(method, ":")
	if !ok {
		return "", "", fmt.Errorf("invalid method %q: missing colon separator", method)
	}

	if module == "" {
		return "", "", fmt.Errorf("invalid method %q: empty module name", method)
	}
	if rpc == "" {
		return "", "", fmt.Errorf("invalid method %q: empty RPC name", method)
	}

	if strings.ContainsAny(module, " \t:") {
		return "", "", fmt.Errorf("invalid module name %q: contains whitespace or colon", module)
	}
	if strings.ContainsAny(rpc, " \t:") {
		return "", "", fmt.Errorf("invalid RPC name %q: contains whitespace or colon", rpc)
	}

	return module, rpc, nil
}

// FormatMethod constructs a "module:rpc-name" method string from components.
func FormatMethod(module, rpc string) string {
	return module + ":" + rpc
}
