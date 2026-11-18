// Design: docs/architecture/web-interface.md -- committed backup folder preparation and selection.
package cli

import (
	"context"

	"github.com/ze-software/ze/internal/component/config"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
)

const artifactEnvironmentRoot = "environment"

func init() {
	if err := pluginserver.RegisterConfigPreparation("config-artifact-directory", prepareArtifactDirectory); err != nil {
		panic("BUG: backup directory preparation registration refused: " + err.Error())
	}
}

func prepareArtifactDirectory(server *pluginserver.Server, ctx context.Context, candidate map[string]any, generation uint64) (pluginserver.ConfigPreparation, error) {
	configured, err := artifactDirectoryValue(candidate)
	if err != nil {
		return nil, err
	}
	owner, err := serverArtifacts(server)
	if err != nil {
		return nil, err
	}
	return owner.PrepareDirectory(ctx, configured, generation)
}

func captureArtifactDirectory(server *pluginserver.Server) (string, error) {
	if reactor, ok := server.ReactorAny().(interface{ GetConfigTree() map[string]any }); ok {
		return artifactDirectoryValue(config.ActiveConfigMap(reactor.GetConfigTree()))
	}
	return "", nil
}

func artifactDirectoryValue(tree map[string]any) (string, error) {
	current := tree
	for _, name := range []string{artifactEnvironmentRoot, "backup"} {
		value, found := current[name]
		if !found {
			return "", nil
		}
		child, ok := value.(map[string]any)
		if !ok {
			return "", opererr.New("The backup directory configuration could not be read.", "The environment and backup settings must be configuration containers; the current folder was kept.", "set environment backup directory /absolute/backup-folder")
		}
		current = child
	}
	value, found := current["directory"]
	if !found {
		return "", nil
	}
	directory, ok := value.(string)
	if !ok {
		return "", opererr.New("The backup directory must be path text.", "The current folder was kept; no prepared folder was selected.", "set environment backup directory /absolute/backup-folder")
	}
	return directory, nil
}
