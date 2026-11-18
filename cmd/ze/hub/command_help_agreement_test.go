//go:build ze_ssh && ze_web

package hub

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	yangloader "github.com/ze-software/ze/internal/component/config/yang"
)

// TestCommandModulesAgreeOnSharedKeywords: across every YANG module this
// build links, a keyword several modules merge their commands under (show,
// request, clear, ...) is described by one module only. A second module that
// states its own help or description for it disagrees with the first, and the
// merge reports that on every `ze` start, as a WARN line each command prints.
//
// PREVENTS: the regression where the class-of-service command module
// described `request` itself. Every `ze data cat` then printed the warning,
// and each fixture that read that command's output as a value failed.
//
// Method: build the merged command tree from the linked schema, with the
// default logger captured, and require that the merge reported no mismatch.
func TestCommandModulesAgreeOnSharedKeywords(t *testing.T) {
	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	schema, err := yangloader.DefaultLoader()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}
	if tree := yangloader.BuildCommandTree(schema); tree == nil {
		t.Fatal("no command tree was built")
	}

	for line := range strings.SplitSeq(logged.String(), "\n") {
		if strings.Contains(line, "YANG command help text mismatch") {
			t.Errorf("two modules describe one keyword differently; state it in the module that owns the keyword only: %s", line)
		}
	}
}
