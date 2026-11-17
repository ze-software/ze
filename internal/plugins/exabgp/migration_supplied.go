// Design: docs/exabgp/exabgp-migration.md -- bounded migration on every command surface.
// Related: migration_bytes.go -- the producers shared with the file and stdin forms.

package exabgp

import (
	"unicode"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// migrationSupplied admits the command's bounded content and diagnostic label
// before conversion. The name never selects a file on the daemon's host.
func migrationSupplied(path, defaultName string, produce migrationProducer) command.LocalDataHandler {
	return func(args command.ValidatedArgs) command.LocalAnswer {
		if err := args.Context().Err(); err != nil {
			return migrationRefusal(path, "The migration request was canceled", err)
		}
		if len(args.PositionalTokens()) != 0 {
			return migrationRefusal(path, "The migration command received unexpected arguments", nil)
		}
		encoded, found := args.Keyed("content")
		if !found {
			return migrationRefusal(path, "The migration input content is missing", nil)
		}
		data, err := command.DecodeContent(encoded)
		if err != nil {
			return migrationRefusal(path, "The migration input could not be decoded", err)
		}
		if len(data) == 0 {
			return migrationRefusal(path, "The migration input is empty", nil)
		}
		name := defaultName
		if label, given := args.Keyed("name"); given {
			if !storage.PlainConfigName(label) {
				return migrationRefusal(path, "The migration input name must be a filename without a directory", nil)
			}
			if label == "" {
				return migrationRefusal(path, "The migration input name is empty", nil)
			}
			for _, character := range label {
				if unicode.IsControl(character) {
					return migrationRefusal(path, "The migration input name contains a control character", nil)
				}
			}
			name = label
		}
		answer := produce(data, name, args.Flag("dry-run"))
		if err := args.Context().Err(); err != nil {
			return migrationRefusal(path, "The migration request was canceled", err)
		}
		if !args.Route().CallerLocal() {
			if blob, isBlob := answer.Payload.(command.Blob); isBlob {
				if len(blob) > command.BlobRemoteOctetsMax {
					return migrationRefusal(path,
						"The converted configuration exceeds the remote answer limit; use the local ze exabgp migrate command", nil)
				}
			}
		}
		return answer
	}
}
