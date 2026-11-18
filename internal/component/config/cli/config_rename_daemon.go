// Design: docs/architecture/api/commands.md -- local mutations through the store owner.
// Related: cmd_rename.go -- standalone rename adapter.
// Related: ../storage/rename_config.go -- complete configuration transaction.

package cli

import (
	"errors"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/storage/daemonroute"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

const configRenamePath = "request data config rename"

type configRenameDaemonAnswer struct {
	Old string `json:"old"`
	New string `json:"new"`
}

// renameConfigThroughDaemon uses the same name judgment as the transaction,
// then requires the owner's answer to identify precisely the requested move.
func renameConfigThroughDaemon(folder, oldName, newName string, busy error, ask daemonroute.Ask) error {
	reader, err := storage.OpenReadOnly(folder, folder)
	if err != nil {
		return opererr.Wrap(err, "ze config rename: the owning store could not be read",
			"The destination daemon cannot be selected safely.", "ze status")
	}
	var names [2]string
	for index, name := range [2]string{oldName, newName} {
		names[index], err = storage.StoredConfigName(reader, name)
		if err != nil {
			break
		}
	}
	closeErr := reader.Close()
	if err != nil {
		return opererr.Wrap(errors.Join(err, closeErr), "ze config rename: a configuration name is refused",
			"Both names must belong to the selected store; no rename was sent.", "ze config rename <old-name> <new-name>")
	}
	if closeErr != nil {
		return opererr.Wrap(closeErr, "ze config rename: the store reader could not be closed",
			"No rename was sent to the daemon.", "ze status")
	}
	if _, err := command.ValidateModelArgs(configRenamePath, []string{"old", names[0], "new", names[1]}, nil); err != nil {
		return err
	}
	var line textbuf.Buffer
	line.Str(configRenamePath).Str(" old ")
	command.WriteInvocationValue(&line, names[0])
	line.Str(" new ")
	command.WriteInvocationValue(&line, names[1])
	var answer configRenameDaemonAnswer
	err = daemonroute.Send(ask, filepath.Join(folder, storage.TreeName), line.String(), &answer)
	if unanswered, ok := errors.AsType[*daemonroute.Unanswered](err); ok {
		return daemonroute.UnansweredRefusal("ze config rename", busy, unanswered)
	}
	if err != nil {
		return err
	}
	if answer.Old != names[0] {
		return answer.mismatch(names[0], names[1])
	}
	if answer.New != names[1] {
		return answer.mismatch(names[0], names[1])
	}
	return nil
}

func (answer configRenameDaemonAnswer) mismatch(oldName, newName string) error {
	var problem, fix textbuf.Buffer
	problem.Str("ze config rename: the daemon answered ").Quoted(answer.Old).Str(" -> ").Quoted(answer.New).
		Str(", not ").Quoted(oldName).Str(" -> ").Quoted(newName)
	fix.Str("ze data cat ")
	command.WriteInvocationValue(&fix, zefs.KeyConfigActive.Key(newName))
	return opererr.New(problem.String(), "The requested rename is not confirmed, so success is not reported.", fix.String())
}
