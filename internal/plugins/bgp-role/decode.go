package bgp_role

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

// decodeRole decodes a Role capability wire value.
// RFC 9234 Section 4.1: Capability length MUST be 1.
func decodeRole(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty Role capability")
	}
	if len(data) != 1 {
		return "", fmt.Errorf("invalid Role capability length: want 1, got %d", len(data))
	}

	name, ok := roleValueToName(data[0])
	if !ok {
		return fmt.Sprintf("unknown(%d)", data[0]), nil
	}
	return name, nil
}

// writeStr writes a string to a writer, ignoring pipe errors (CLI output).
func writeStr(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, format, args...) //nolint:errcheck // CLI output - pipe failure is unrecoverable
}

// RunCLIDecode decodes hex capability data directly from CLI arguments.
// This is for human use: `ze bgp plugin role --capa <hex>` or with `--text`.
func RunCLIDecode(hexData string, textOutput bool, stdout, stderr io.Writer) int {
	if hexData == "" {
		writeStr(stderr, "error: empty input\n")
		return 1
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeStr(stderr, "error: invalid hex: %v\n", err)
		return 1
	}

	roleName, err := decodeRole(data)
	if err != nil {
		writeStr(stderr, "error: %v\n", err)
		return 1
	}

	if textOutput {
		writeStr(stdout, "%-20s %s\n", "role", roleName)
	} else {
		result := map[string]any{
			"name":  "role",
			"value": roleName,
		}
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			writeStr(stderr, "error: JSON encoding: %v\n", err)
			return 1
		}
		writeStr(stdout, "%s\n", string(jsonBytes))
	}
	return 0
}
