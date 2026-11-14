// Design: docs/guide/docker.md -- "Docker and its Compose plugin must already be installed"
// Overview: compose.go -- the proof this check gates

package testdeployment

import (
	"context"
	"errors"
	"runtime"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// composeVersionMin is the oldest Compose the proof's override file parses:
// `!override` and `!reset` arrived in Compose 2.24.4.
var composeVersionMin = [3]int{2, 24, 4}

// composeInstallFix is the command that installs or updates the plugin here.
func composeInstallFix(goos string) string {
	if goos == "darwin" {
		return "`brew install docker-compose` (then link it as a docker CLI plugin as brew prints)"
	}
	return "install the distribution's `docker-compose-plugin` package"
}

// requireCompose refuses a Docker endpoint whose CLI lacks the Compose plugin,
// or carries one older than composeVersionMin, before any project object or
// scratch file exists.
func requireCompose(ctx context.Context, docker *interoplab.Docker, goos string) error {
	version, err := docker.ComposeVersion(ctx)
	if err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str("the Docker Compose plugin is missing: `docker compose version` failed (").Err(err).
			Str("). The proof walks the Docker guide's Compose journey, so it needs the Compose plugin. Fix: ").
			Str(composeInstallFix(goos)).String())
	}
	parsed, ok := parseComposeVersion(version)
	if !ok {
		return errors.New("the Docker Compose plugin printed version " + strconv.Quote(version) +
			", which the proof cannot compare with 2.24.4. Fix: " + composeInstallFix(goos))
	}
	if composeVersionOlder(parsed, composeVersionMin) {
		return errors.New("the Docker Compose plugin " + version + " is older than 2.24.4, which the proof's override" +
			" file needs for `!override` and `!reset`. Fix: update it: " + composeInstallFix(goos))
	}
	return nil
}

// parseComposeVersion reads major, minor and patch from "2.29.1", "v2.29.1"
// or "2.24.4-desktop.1".
func parseComposeVersion(version string) ([3]int, bool) {
	var parsed [3]int
	fields := strings.SplitN(strings.TrimPrefix(version, "v"), ".", 3)
	if len(fields) != 3 {
		return parsed, false
	}
	for index, field := range fields {
		digits := field
		if end := strings.IndexFunc(field, func(r rune) bool { return r < '0' || r > '9' }); end >= 0 {
			digits = field[:end]
		}
		number, err := strconv.Atoi(digits)
		if err != nil {
			return parsed, false
		}
		parsed[index] = number
	}
	return parsed, true
}

func composeVersionOlder(have, want [3]int) bool {
	for index := range have {
		if have[index] != want[index] {
			return have[index] < want[index]
		}
	}
	return false
}

// hostGOOS is the platform the fix text names.
var hostGOOS = runtime.GOOS
