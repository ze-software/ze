// Design: docs/architecture/api/commands.md -- structured booleans preserve flag presence.
package command

import (
	"fmt"
	"maps"
	"strconv"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// NormalizeInvocationFlags translates structured boolean flag values into
// WriteInvocation's presence convention. It never changes nonflag data and
// never mutates the caller's map. HTML controls already carry presence and do
// not need this adapter. An invalid boolean is refused rather than enabled.
func NormalizeInvocationFlags(defs []InvocationArg, values map[string]string) (map[string]string, error) {
	result := values
	copied := false
	for _, def := range defs {
		value, present := values[def.Name]
		if !def.Flag || !present {
			continue
		}
		enabled := false
		if value != "" {
			var err error
			enabled, err = strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("parameter %q must be a boolean: %w", def.Name, err)
			}
		}
		if !copied {
			result = maps.Clone(values)
			copied = true
		}
		if enabled {
			result[def.Name] = "true"
		} else {
			delete(result, def.Name)
		}
	}
	return result, nil
}

// InvocationFlagError gives a structured-parameter refusal its registered
// command context. The caller MUST refuse before dispatch when normalization
// fails; this helper never retries or changes the supplied flag.
func InvocationFlagError(path string, cause error) *opererr.Error {
	var fix textbuf.Buffer
	fix.Str("ze help command ")
	WriteInvocationValue(&fix, path)
	return opererr.Wrap(cause, "Cannot run "+path,
		"No command ran. Supply structured flags as true or false and use the command's declared arguments.", fix.String())
}
