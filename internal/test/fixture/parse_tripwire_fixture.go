// Design: docs/functional-tests.md -- executable tripwire for parse-error refusal tests
// Related: register_parse_tripwire.go -- registration of this fixture

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// parseWriteTripwire writes an executable tripwire at <path> in the test work
// directory. If executed, the tripwire prints <message> to stderr and exits 97.
//
// Arguments: <path> <message>.
func parseWriteTripwire(_ context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: parse/write-tripwire <path> <message>")
	}
	target, message := args[0], args[1]
	if target == "" {
		return errors.New("parse/write-tripwire: target path is empty")
	}
	if filepath.IsAbs(target) {
		return fmt.Errorf("parse/write-tripwire: target path %q must be relative to the test directory", target)
	}
	cleaned := filepath.Clean(target)
	if cleaned == ".." {
		return fmt.Errorf("parse/write-tripwire: target path %q escapes the test directory", target)
	}
	if strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("parse/write-tripwire: target path %q escapes the test directory", target)
	}
	if message == "" {
		return errors.New("parse/write-tripwire: message is empty")
	}
	if strings.ContainsAny(message, "'\r\n") {
		return fmt.Errorf("parse/write-tripwire: message %q must be a single line without single quotes", message)
	}

	body := "#!/bin/sh\nprintf '%s\\n' '" + message + "' >&2\nexit 97\n"
	if err := os.WriteFile(cleaned, []byte(body), 0o755); err != nil { //nolint:gosec // cliio:allow fixture writes an executable tripwire in the test workdir
		return fmt.Errorf("parse/write-tripwire: write %s: %w", cleaned, err)
	}
	return nil
}
