// Design: docs/guide/config-editor.md — loose-file and stdin editing
package cli

import (
	"errors"
	"fmt"
	"os"

	editor "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/resolve"
)

// openEditableConfig resolves persistence only after the caller parsed its file.
// A missing store does not create one; a busy or invalid store refuses the write.
// The caller MUST close the returned editor to release offline ownership.
func openEditableConfig(store storage.Storage, configPath string) (*editor.Editor, error) {
	if cliio.IsStdin(configPath) {
		data, err := cliio.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
		ed, err := editor.NewEditorFromContent(data, configPath)
		if err != nil {
			return nil, err
		}
		sink, err := cliio.Create(configPath)
		if err != nil {
			return nil, err
		}
		ed.SetStdoutSink(sink)
		return ed, nil
	}
	if store != nil {
		return editor.NewEditorWithStorage(store, configPath)
	}
	store, err := resolve.StorageFor(configPath)
	if err != nil {
		if !errors.Is(err, storage.ErrNoStore) {
			return nil, err
		}
		store = nil
	}
	ed, err := editor.NewLooseFileEditor(store, configPath)
	if err != nil {
		if store != nil {
			store.Close() //nolint:errcheck // Returning the source read error.
		}
		return nil, err
	}
	if store != nil {
		ed.OwnStore()
	}
	return ed, nil
}

func noticeUnrecordedVersion(ed *editor.Editor, configPath string) {
	if cliio.IsStdin(configPath) {
		return
	}
	if !ed.HasHistory() {
		fmt.Fprintf(os.Stderr, "notice: no version recorded: %s has no store; run ze init\n", resolve.StoreDir(configPath))
	}
}
