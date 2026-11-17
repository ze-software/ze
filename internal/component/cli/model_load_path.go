// Design: docs/guide/config-editor.md -- daemon file imports stay inside configuration directories.
// Related: ../command/caller_path.go -- the shared policy for daemon-side file reads.

package cli

import (
	"errors"
	"io"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/opererr"
)

const loadFileBytesMax = 4 << 20

// loadFile preserves local-owner reads and confines daemon-hosted reads through
// the same registered policy as daemon file commands. Unknown models, including
// the browser adapter, have no filesystem authority; their caller supplies bytes.
func (m *Model) loadFile(path, location, action string) (data []byte, resultErr error) {
	resolved := m.resolveConfigPath(path)
	fix := "load terminal " + location + " " + action
	switch m.filesystemAuthority {
	case FilesystemAuthorityOperatorLocal:
		data, err := cliio.ReadFile(resolved)
		if err != nil {
			return nil, opererr.Wrap(err, "This editor cannot read the selected configuration file.",
				"The candidate has not changed. Paste the configuration instead.", fix)
		}
		return data, nil
	case FilesystemAuthorityDaemon:
		var err error
		resolved, err = command.CallerPath(command.RouteRemote, "config edit", []string{path}, resolved)
		if err != nil {
			return nil, opererr.New("The selected configuration file cannot be loaded in this session.",
				"The daemon reads only regular configuration files inside its configuration directories, excluding the store's private files. Paste the configuration instead.", fix)
		}
	case FilesystemAuthorityUnknown:
		return nil, opererr.New("This session cannot read a file from the router.",
			"This editor accepts configuration supplied by its caller. Paste the configuration instead.", fix)
	default:
		// An authority outside the declared set never grants a file read.
		return nil, opererr.New("This session has no authority to read configuration files.",
			"The candidate has not changed. Paste the configuration instead.", fix)
	}
	reader, err := openLoadFile(resolved)
	if err != nil {
		return nil, opererr.Wrap(err, "The selected configuration file cannot be opened.",
			"The candidate has not changed. Paste the configuration instead.", fix)
	}
	// This owner MUST close the descriptor returned by openLoadFile.
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, opererr.Wrap(closeErr,
				"The selected configuration file could not be closed.",
				"The candidate has not changed. Paste the configuration instead.", fix))
		}
	}()
	data, err = io.ReadAll(io.LimitReader(reader, loadFileBytesMax+1))
	if err != nil {
		return nil, opererr.Wrap(err, "The selected configuration file cannot be read.",
			"The candidate has not changed. Paste the configuration instead.", fix)
	}
	if len(data) > loadFileBytesMax {
		return nil, opererr.New("The selected configuration file exceeds the 4 MiB limit.",
			"The candidate has not changed. Import a smaller configuration section.", fix)
	}
	return data, nil
}
