// Design: docs/architecture/core-design.md -- native Claude lifecycle hooks
// Related: lifecycle.go -- runLifecycleHook dispatches bash-timestamp here
package hookruntime

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// bashStampQuietDefault is the gap during which a session that already printed
// a timestamp prints no other one. A run of quick commands otherwise floods the
// transcript with one line per call.
const bashStampQuietDefault = 60 * time.Second

// bashStampQuietEnv overrides bashStampQuietDefault, in whole seconds.
const bashStampQuietEnv = "CLAUDE_BASH_STAMP_QUIET"

// hookBashTimestamp prints a "▶ HH:MM:SS" system message before a Bash call,
// at most once per quiet gap for each session. It is display only: it never
// judges the command, and it always answers 0 so it can never block one. A
// session it cannot identify, or a marker it cannot write, prints nothing,
// because a stamp it cannot record would print on every call.
func hookBashTimestamp(ctx context, out io.Writer) int {
	marker := sessionMarker(ctx, ".bash-stamp-")
	if marker == "" {
		return 0
	}
	now := time.Now()
	if last, ok := readBashStamp(marker); ok {
		if now.Sub(last) < bashStampQuiet() {
			return 0
		}
	}
	if err := os.MkdirAll(filepath.Dir(marker), 0o750); err != nil {
		return 0
	}
	if err := os.WriteFile(marker, strconv.AppendInt(nil, now.Unix(), 10), 0o600); err != nil {
		return 0
	}
	body, err := json.Marshal(map[string]string{"systemMessage": "▶ " + now.Format(time.TimeOnly)})
	if err != nil {
		return 0
	}
	out.Write(append(body, '\n')) //nolint:errcheck // hook protocol: a display line, nothing depends on it
	return 0
}

// readBashStamp reads the time the session last printed a timestamp. A marker
// that is absent or does not hold a Unix time answers false, which prints one.
func readBashStamp(marker string) (time.Time, bool) {
	body, err := os.ReadFile(marker)
	if err != nil {
		return time.Time{}, false
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(string(body)), 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(seconds, 0), true
}

// bashStampQuiet answers the quiet gap. An unset variable, or one that is not
// a non-negative whole number of seconds, keeps the default.
func bashStampQuiet() time.Duration {
	text := os.Getenv(bashStampQuietEnv)
	if text == "" {
		return bashStampQuietDefault
	}
	seconds, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return bashStampQuietDefault
	}
	if seconds < 0 {
		return bashStampQuietDefault
	}
	return time.Duration(seconds) * time.Second
}
