// VALIDATES: the SIGHUP reload-completion marker is announced by EVERY reload
// loop in this binary, and its phrase does not collide with any other log line
// a daemon can emit.
// PREVENTS: two defects an independent review caught before they shipped.
//   1. reloadComplete() was called only from handleSIGHUPReload (the YANG/BGP
//      config path). The hub-config path in main.go has its own inline SIGHUP
//      loop and printed nothing on success, so the operator-facing defect the
//      marker exists to fix was still live for every hub daemon, and a .ci
//      fencing on the phrase would hang for 10s and then blame the fence.
//   2. The phrase was "reload complete", which is a SUBSTRING of the existing
//      `logger().Info("config reload completed")` in plugin/server/reload.go.
//      That line is emitted inside ReloadConfig, BEFORE storage.PromoteCandidate
//      writes meta/config/active and meta/config/rollback. It is suppressed at
//      the default WARN level, so a fence on it passes today by accident; the
//      first test raising the log level to info would fence on the earlier line,
//      tear the daemon down mid-promotion, and go green while racing.

package hub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every SIGHUP reload loop must announce completion. Counting call sites is a
// proxy, but a deliberately blunt one: the failure it guards is a loop that
// forgets to call, and a new loop that forgets will not add a call site.
func TestEverySIGHUPLoopAnnouncesCompletion(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	var loops, announces []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, rerr := os.ReadFile(name) //nolint:gosec // package-local source file
		if rerr != nil {
			t.Fatal(rerr)
		}
		body := string(data)
		for i, line := range strings.Split(body, "\n") { //nolint:gocritic // index needed for the line number
			// Skip comments in BOTH directions: a comment quoting the SIGHUP
			// phrase would inflate the loop count into a false positive, and a
			// doc comment mentioning reloadComplete() would mask a genuinely
			// missing call. The margin here is exactly zero, so either miscount
			// defeats the test.
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if strings.Contains(line, `"received SIGHUP, reloading config...`) {
				loops = append(loops, name+":"+lineNo(i))
			}
			if strings.Contains(line, "reloadComplete()") && !strings.Contains(line, "func ") {
				announces = append(announces, name+":"+lineNo(i))
			}
		}
	}

	if len(loops) == 0 {
		t.Fatal("found no SIGHUP reload loop: this test can no longer detect the regression it guards")
	}
	if len(announces) < len(loops) {
		t.Errorf("%d SIGHUP reload loop(s) in package hub %v but only %d reloadComplete() call(s) %v:\n"+
			"a reload path that does not announce completion is invisible to operators "+
			"and cannot be fenced on by a .ci test.\n"+
			"NOTE: this test is package-scoped (it reads only cmd/ze/hub). The reactor's own "+
			"reload loop is a separate process and deliberately needs its own distinct marker "+
			"-- see the reloadCompleteLine doc comment",
			len(loops), loops, len(announces), announces)
	}
}

// The marker must not be a substring of any other line the daemon can log,
// because `await=stderr:contains=` is a plain substring match over relayed
// stderr and would fence on the wrong (earlier) line.
func TestReloadCompleteMarkerDoesNotCollide(t *testing.T) {
	marker := strings.TrimSpace(reloadCompleteLine)
	if marker == "" {
		t.Fatal("reloadCompleteLine is empty")
	}

	root := filepath.Join("..", "..", "..")
	var collisions []string

	for _, dir := range []string{
		filepath.Join(root, "internal"),
		filepath.Join(root, "cmd"),
	} {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, werr error) error {
			if werr != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil //nolint:nilerr // an unreadable tree is not this test's failure
			}
			if strings.HasSuffix(path, "main_reload.go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, rerr := os.ReadFile(path) //nolint:gosec // repo source under test
			if rerr != nil {
				return nil //nolint:nilerr // unreadable file is not a collision
			}
			for line := range strings.SplitSeq(string(data), "\n") {
				// Only string literals matter: a comment mentioning the phrase
				// is never written to stderr.
				if !strings.Contains(line, `"`) || strings.HasPrefix(strings.TrimSpace(line), "//") {
					continue
				}
				if strings.Contains(line, marker) {
					collisions = append(collisions, path+": "+strings.TrimSpace(line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	if len(collisions) != 0 {
		t.Errorf("marker %q appears in %d other log line(s); an await=stderr fence would "+
			"match the earlier one and release before the reload finished:\n  %s",
			marker, len(collisions), strings.Join(collisions, "\n  "))
	}
}

func lineNo(zeroBased int) string {
	n := zeroBased + 1
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
