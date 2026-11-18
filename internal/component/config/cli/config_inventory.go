// Design: docs/architecture/api/commands.md -- one inventory behind local and daemon routes.
// Related: cmd_list.go -- the root command's text presentation.
// Related: cmd_history.go -- the root history presentation.

package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/zefs"
)

type configListRow struct {
	Source string `json:"source"`
	Path   string `json:"path"`
}

// configListRows owns the active/draft census and optional loose-file scan.
// An artifact asks only for its stored records, never nearby host files.
// Any required read failure refuses the complete list, without partial rows.
func configListRows(store storage.Storage, loose bool) ([]configListRow, error) {
	rows := make([]configListRow, 0)
	for _, prefix := range []string{zefs.KeyFileActive.Dir(), zefs.KeyFileDraft.Dir()} {
		keys, err := store.List(prefix)
		if err != nil {
			return nil, configListFailure(err)
		}
		for _, key := range keys {
			rows = append(rows, configListRow{Source: "data", Path: key})
		}
	}
	if !loose {
		return rows, nil
	}
	dirs, err := configSearchDirs()
	if err != nil {
		return nil, configListFailure(err)
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue // An optional search directory need not exist.
			}
			return nil, configListFailure(err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if !strings.HasSuffix(entry.Name(), ".conf") {
				continue
			}
			rows = append(rows, configListRow{Source: "fs", Path: filepath.Join(dir, entry.Name())})
		}
	}
	return rows, nil
}

func configListFailure(err error) error {
	return opererr.Wrap(err, "Configurations could not all be listed.",
		"The complete configuration list is unknown; no partial list was returned", "ze doctor")
}

// configHistoryRow is the shared wire row. Revision is either the ordinal
// number or the literal draft marker already used by the command contract.
type configHistoryRow struct {
	Revision  any    `json:"revision"`
	Timestamp string `json:"timestamp,omitempty"`
	Path      string `json:"path,omitempty"`
	State     string `json:"state,omitempty"`
}

func configHistoryRows(store storage.Storage, path string) ([]configHistoryRow, error) {
	versions, err := store.ListVersions(path)
	if err != nil {
		return nil, err
	}
	rows := make([]configHistoryRow, 0, len(versions)+1)
	if store.Exists(cli.DraftPath(path)) {
		rows = append(rows, configHistoryRow{Revision: "draft", State: "editing in progress"})
	}
	for index, version := range versions {
		rows = append(rows, configHistoryRow{Revision: index + 1,
			Timestamp: version.Date.Format("2006-01-02 15:04:05"), Path: version.Path})
	}
	return rows, nil
}
