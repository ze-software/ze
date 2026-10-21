// Design: docs/architecture/testing/interop.md -- Docker lab lifecycle and failure contracts
// Related: docker.go -- Docker.Build, the only caller.
// Related: ../setup/buildx.go -- the setup step this refusal names as its fix.
package interoplab

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	lesetup "github.com/ze-software/ze/internal/le/setup"
)

// requireBuildKit refuses a build that carries a named context when the docker
// CLI has no buildx plugin. Only BuildKit accepts `--build-context`, and a CLI
// without buildx runs the legacy builder, which answers the flag with its usage
// text and nothing that names the missing plugin. The context is never dropped
// to make the build run: the Dockerfile reads it, so the image would be wrong.
func (d *Docker) requireBuildKit(ctx context.Context, build ImageBuild) error {
	if refusal := legacyBuilderForced(d.environ, build); refusal != nil {
		return refusal
	}
	_, err := d.command(ctx, dockerCommandTimeout, DockerExecutable, "buildx", "version")
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return err
	}
	var tb textbuf.Buffer
	tb.Str("cannot build the ").Str(build.Name).Str(" image: it uses the named build context ").
		Str(build.NamedContexts[0].Name).Str(", and named build contexts need BuildKit.\n").
		Str("The docker CLI on this machine has no buildx plugin, so `docker build` uses the legacy builder,\n").
		Str("which refuses --build-context. Install the buildx plugin, then run the command again.\n").
		Str("Setup installs it on macOS and names the package to install on Linux:\n").
		Str("  ").Str(lesetup.BuildKitSetupCommand).Str("\n").
		Str("To install it by hand on macOS with Homebrew, run:\n").
		Str("  brew install docker-buildx\n").
		Str("then add the cli-plugins directory Homebrew prints, $(brew --prefix)/lib/docker/cli-plugins,\n").
		Str("to ").Str(d.dockerConfigFile()).Str(". On Apple silicon that is:\n").
		Str("  \"cliPluginsExtraDirs\": [\"/opt/homebrew/lib/docker/cli-plugins\"]\n").
		Str("and on an Intel Mac:\n").
		Str("  \"cliPluginsExtraDirs\": [\"/usr/local/lib/docker/cli-plugins\"]\n").
		Str("On Linux, install the distribution's buildx package (docker-buildx-plugin or docker-buildx).\n").
		Str("Check that the plugin answers with:\n").
		Str("  docker buildx version\n").
		Str("The probe answered: ").Str(err.Error())
	return errors.New(tb.String())
}

// dockerConfigFile names the config file the docker CLI reads under this
// client's environment, from the helper the setup step edits it through, so
// the refusal and the fix cannot name different files. With no home and no
// DOCKER_CONFIG it names the rule instead of a path.
func (d *Docker) dockerConfigFile() string {
	// The last entry for a key wins, as it does for the process docker runs as.
	getenv := func(key string) string {
		value := ""
		for _, entry := range d.environ {
			if found, ok := strings.CutPrefix(entry, key+"="); ok {
				value = found
			}
		}
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	path, err := lesetup.DockerConfigPath(getenv, home)
	if err != nil {
		return "the docker config ($DOCKER_CONFIG/config.json, or ~/.docker/config.json)"
	}
	return path
}

// legacyBuilderForced refuses a named-context build when environ, the
// environment docker runs under, sets DOCKER_BUILDKIT to a false value: the
// docker CLI then runs the legacy builder even with buildx installed. The
// operator's setting is named and never overridden.
func legacyBuilderForced(environ []string, build ImageBuild) error {
	for _, entry := range environ {
		value, found := strings.CutPrefix(entry, "DOCKER_BUILDKIT=")
		if !found {
			continue
		}
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			continue
		}
		if enabled {
			continue
		}
		var tb textbuf.Buffer
		return errors.New(tb.Str("cannot build the ").Str(build.Name).Str(" image: it uses the named build context ").
			Str(build.NamedContexts[0].Name).Str(", and named build contexts need BuildKit.\n").
			Str("This environment sets ").Str(entry).Str(", so `docker build` uses the legacy builder,\n").
			Str("which refuses --build-context. Unset the variable, then run the command again:\n").
			Str("  unset DOCKER_BUILDKIT").String())
	}
	return nil
}
