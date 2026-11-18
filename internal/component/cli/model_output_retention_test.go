// Design: docs/guide/monitoring.md -- bounded interactive display scrollback.
// Related: model_monitor.go -- event delivery; model_ping.go, model_traceroute.go -- log views.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These are the operator-visible allowance and notice, not an implementation
// helper: a trimmed view backed by an ever-growing builder must still fail.
const (
	retentionDisplayBytes = 512 * 1024
	retentionNotice       = "[earlier output omitted]\n"
)

// TestModelOutputRetentionMonitor keeps real constructor and polling paths under
// 100,000 events. Recent output must remain useful while both stored capacity
// and the visible projection stay bounded after each half of the session.
func TestModelOutputRetentionMonitor(t *testing.T) {
	for _, kind := range []string{"command", "editor"} {
		t.Run(kind, func(t *testing.T) {
			model := retentionModel(t, kind)
			events := make(chan string, 50_000)
			model.monitorSession = &MonitorSession{EventChan: events, Cancel: func() {}}
			middle := "middle-event: " + strings.Repeat("x", 80)
			for half := range 2 {
				for index := range cap(events) {
					event := middle
					if half == 0 && index == 0 {
						event = "old-event-sentinel"
					}
					if index == cap(events)-1 {
						event = "recent-event-sentinel"
					}
					events <- event
				}
				updated, _ := model.handleMonitorPoll()
				model = retentionUpdatedModel(t, updated)
				assert.Empty(t, events, "every submitted event must reach the actual poll")
				retentionAssertDisplay(t, &model, "recent-event-sentinel")
				assert.False(t, strings.Contains(model.outputBuf.String(), "old-event-sentinel"),
					"old events must be evicted while recent events remain")
			}
		})
	}
}

// TestModelOutputRetentionOversizedAndClear delivers one oversized command
// answer, then uses the actual clear command. Neither a partial UTF-8 prefix nor
// the discarded answer's backing storage may survive as session scrollback.
func TestModelOutputRetentionOversizedAndClear(t *testing.T) {
	model := retentionModel(t, "command")
	answer := "old-answer-sentinel\n" + strings.Repeat("é", retentionDisplayBytes) + "\nrecent-answer-sentinel"
	updated, _ := model.handleCommandResult(commandResultMsg{result: commandResult{output: answer}})
	model = retentionUpdatedModel(t, updated)
	retentionAssertDisplay(t, &model, "recent-answer-sentinel")
	assert.False(t, strings.Contains(model.outputBuf.String(), "old-answer-sentinel"))
	assert.True(t, utf8.ValidString(model.outputBuf.String()), "eviction must preserve text boundaries")

	model.textInput.SetValue("clear")
	updated, _ = model.handleEnter()
	model = retentionUpdatedModel(t, updated)
	assert.Empty(t, model.outputBuf.String())
	assert.Zero(t, model.outputBuf.Len())
	assert.LessOrEqual(t, model.outputBuf.Cap(), retentionDisplayBytes)
	assert.Empty(t, model.viewportContent)

	updated, _ = model.handleCommandResult(commandResultMsg{result: commandResult{output: "fresh answer"}})
	model = retentionUpdatedModel(t, updated)
	assert.Equal(t, "fresh answer", model.outputBuf.String(), "clear also resets the omission notice")
}

// TestModelOutputRetentionSanitizedExpansion reaches the actual renderer with
// fewer than 512 KiB of raw text whose control-byte replacement exceeds that
// allowance. The bounded display must retain the sanitizer's replacement text
// and useful suffix rather than publish controls or an unbounded projection.
func TestModelOutputRetentionSanitizedExpansion(t *testing.T) {
	model := retentionModel(t, "command")
	answer := strings.Repeat("\x01", retentionDisplayBytes/2) + "recent-sanitized-sentinel"
	updated, _ := model.handleCommandResult(commandResultMsg{result: commandResult{output: answer}})
	model = retentionUpdatedModel(t, updated)
	retentionAssertDisplay(t, &model, "recent-sanitized-sentinel")
	assert.False(t, strings.ContainsRune(model.viewportContent, '\x01'), "existing sanitization must remain active")
	assert.True(t, strings.ContainsRune(model.viewportContent, '\uFFFD'), "replacement text must remain visible")
	assert.True(t, utf8.ValidString(model.viewportContent))
	assert.Equal(t, sanitizeForDisplay(model.outputBuf.String()), model.viewportContent,
		"sanitizing the display owner's projection again must preserve it")
}

// TestModelOutputRetentionPipedViews passes oversized formatted output through
// the actual ping/traceroute pollers. Display eviction must not truncate the
// independent pipe-save owner, which receives every original formatted byte.
func TestModelOutputRetentionPipedViews(t *testing.T) {
	for _, kind := range []string{"ping", "traceroute"} {
		t.Run(kind, func(t *testing.T) {
			model := retentionModel(t, "command")
			path := filepath.Join(t.TempDir(), "complete-output.txt")
			line := "old-pipe-sentinel\n" + strings.Repeat("p", retentionDisplayBytes*2) + "\nrecent-pipe-sentinel"
			format := func(string) string { return line }
			switch kind {
			case "ping":
				model.SetPingFactory(pingTestFactory([]map[string]any{{"seq": 0, "status": "ok", "rtt-ms": 1.5}}))
				require.NotNil(t, model.startPingMonitorPiped("monitor ping 192.0.2.1 | json | log | save "+path))
				model.activePingPiped().formatFn = format
				updated, _ := model.handlePingPipedPoll()
				model = retentionUpdatedModel(t, updated)
			case "traceroute":
				model.setTracerouteFactory(testFactory([]map[string]any{{"ttl": 1, "addr": "192.0.2.1", "rtt-ms": 1.5}}))
				require.NotNil(t, model.startTraceroutePiped("monitor traceroute 192.0.2.1 | json | log | save "+path))
				model.activeTraceroutePiped().formatFn = format
				updated, _ := model.handleTraceroutePipedPoll()
				model = retentionUpdatedModel(t, updated)
				model.stopTraceroutePiped()
			}
			retentionAssertDisplay(t, &model, "recent-pipe-sentinel")
			assert.False(t, strings.Contains(model.outputBuf.String(), "old-pipe-sentinel"))
			saved, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, line+"\n", string(saved), "display retention must not alter pipe-save bytes")
		})
	}
}

func retentionModel(t *testing.T, kind string) Model {
	t.Helper()
	var model Model
	if kind == "editor" {
		editor, err := NewEditorFromContent([]byte(testValidBGPConfig), "retention.conf")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, editor.Close()) })
		model, err = NewModel(editor, FilesystemAuthorityOperatorLocal)
		require.NoError(t, err)
	} else {
		model = NewCommandModel(FilesystemAuthorityOperatorLocal)
	}
	t.Cleanup(func() { require.NoError(t, model.Close()) })
	return model
}

func retentionUpdatedModel(t *testing.T, updated tea.Model) Model {
	t.Helper()
	model, ok := updated.(Model)
	require.True(t, ok, "the real model update must return a CLI model")
	return model
}

func retentionAssertDisplay(t *testing.T, model *Model, recent string) {
	t.Helper()
	assert.LessOrEqual(t, model.outputBuf.Cap(), retentionDisplayBytes, "retained backing allocation must be bounded")
	assert.LessOrEqual(t, model.outputBuf.Len(), retentionDisplayBytes, "retained text must be bounded")
	assert.LessOrEqual(t, len(model.viewportContent), retentionDisplayBytes, "viewport must not keep the discarded transcript")
	assert.True(t, strings.HasPrefix(model.outputBuf.String(), retentionNotice), "eviction must be visible")
	assert.True(t, strings.Contains(model.outputBuf.String(), recent), "useful recent output must survive")
	assert.True(t, strings.Contains(model.viewportContent, recent), "the operator must see the recent output")
}
