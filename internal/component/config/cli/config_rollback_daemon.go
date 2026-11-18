// Design: docs/architecture/api/commands.md -- indexed rollback through the store owner.
// Related: cmd_rollback.go -- local stored and loose-file publication adapters.
// Related: config_rename_daemon.go -- the same owning-daemon route for rename.

package cli

import (
	"errors"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/storage/daemonroute"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const configRollbackPath = "request data config rollback"

// configRollbackDaemonAnswer is the external owner's receipt. Its zero value
// is incomplete. selectedTime validates every identity before returning a time.
type configRollbackDaemonAnswer struct {
	Name          string    `json:"name"`
	Revision      int       `json:"revision"`
	SourceVersion string    `json:"source-version"`
	SourceTime    time.Time `json:"source-time"`
	Version       string    `json:"version"`
}

// rollbackConfigThroughDaemon uses the owner's name judgment and registered
// arguments. A missing or mismatched receipt never becomes a success message.
func rollbackConfigThroughDaemon(folder, file string, revision int, busy error, ask daemonroute.Ask) (time.Time, error) {
	reader, err := storage.OpenReadOnly(folder, folder)
	if err != nil {
		return time.Time{}, opererr.Wrap(err, "ze config rollback: the owning store could not be read",
			"The destination daemon cannot be selected safely.", "ze status")
	}
	name, nameErr := storage.StoredConfigName(reader, file)
	closeErr := reader.Close()
	if nameErr != nil {
		return time.Time{}, opererr.Wrap(errors.Join(nameErr, closeErr),
			"ze config rollback: the configuration name is refused",
			"The file must belong to the selected store. No rollback was sent.",
			configRollbackHistoryLine(file))
	}
	if closeErr != nil {
		return time.Time{}, opererr.Wrap(closeErr, "ze config rollback: the store reader could not be closed",
			"No rollback was sent to the daemon.", "ze status")
	}
	index := strconv.Itoa(revision)
	if _, err := command.ValidateModelArgs(configRollbackPath, []string{"name", name, "revision", index}, nil); err != nil {
		return time.Time{}, err
	}
	var line textbuf.Buffer
	line.Str(configRollbackPath).Str(" name ")
	command.WriteInvocationValue(&line, name)
	line.Str(" revision ").Str(index)
	var answer configRollbackDaemonAnswer
	err = daemonroute.Send(ask, filepath.Join(folder, storage.TreeName), line.String(), &answer)
	if unanswered, ok := errors.AsType[*daemonroute.Unanswered](err); ok {
		return time.Time{}, daemonroute.UnansweredRefusal("ze config rollback", busy, unanswered)
	}
	if err != nil {
		return time.Time{}, err
	}
	return answer.selectedTime(name, revision, file)
}

// rollbackRouteRefusal is the root command's final presentation boundary.
// Only a received daemon refusal is decoded from text. Native producers keep
// their parts, and unrecognized or joined failures retain their whole cause.
func rollbackRouteRefusal(err error, file string) *opererr.Error {
	switch refusal := err.(type) { //nolint:errorlint // Only these direct producer contracts may project their complete native text.
	case *opererr.Error:
		return refusal
	case *daemonroute.Refusal:
		native, decodeErr := opererr.FromRendered(refusal.Message)
		if decodeErr == nil {
			return native
		}
		return opererr.Wrap(errors.Join(err, decodeErr),
			"ze config rollback: the daemon's refusal is incomplete",
			"The daemon answered, but no successful rollback was confirmed. Inspect the history before retrying.",
			configRollbackHistoryLine(file))
	case *command.ArgumentFaults:
		// ArgumentFaults renders its first unwrap value, the one refusal
		// that already names every fault and the whole corrected command.
		parts := refusal.Unwrap()
		if len(parts) > 0 {
			if native, known := parts[0].(*opererr.Error); known { //nolint:errorlint // The first value is this exact producer's complete rendered refusal.
				return native
			}
		}
	}
	return opererr.Wrap(err, "ze config rollback: the owning daemon route could not finish",
		"No successful rollback was confirmed. Inspect the history before retrying.", configRollbackHistoryLine(file))
}

func (answer configRollbackDaemonAnswer) selectedTime(name string, revision int, file string) (time.Time, error) {
	if answer.Name != name {
		return time.Time{}, configRollbackReceiptRefusal(name, revision, file)
	}
	if answer.Revision != revision {
		return time.Time{}, configRollbackReceiptRefusal(name, revision, file)
	}
	if answer.SourceVersion == "" {
		return time.Time{}, configRollbackReceiptRefusal(name, revision, file)
	}
	if answer.Version == "" {
		return time.Time{}, configRollbackReceiptRefusal(name, revision, file)
	}
	if answer.SourceTime.IsZero() {
		return time.Time{}, configRollbackReceiptRefusal(name, revision, file)
	}
	return answer.SourceTime, nil
}

func configRollbackReceiptRefusal(name string, revision int, file string) error {
	var problem textbuf.Buffer
	problem.Str("ze config rollback: the daemon did not confirm revision ").Int(int64(revision)).
		Str(" of ").Quoted(name)
	return opererr.New(problem.String(),
		"Its answer is incomplete or names another operation, so the requested rollback is not confirmed.",
		configRollbackHistoryLine(file))
}

func configRollbackHistoryLine(file string) string {
	var line textbuf.Buffer
	line.Str("ze config history ")
	command.WriteInvocationValue(&line, file)
	return line.String()
}
