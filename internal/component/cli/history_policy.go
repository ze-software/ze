// Design: docs/features/cli-commands.md -- one recall policy for terminal surfaces.
// Related: model_keys.go -- Enter applies the mode transition before dispatch.
package cli

import "strings"

// HistoryCommandMode selects the effective recall mode and whether Enter
// records input. A transport MUST pass the same frozen operational matcher as
// its executor, so a changing registry cannot attribute an edit to another mode.
// Unspecified modes refuse recording. Safe for concurrent use.
func HistoryCommandMode(mode EditorMode, input string, registered func(string) bool) (EditorMode, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return mode, false
	}
	switch mode {
	case ModeOperational:
		if IsSessionFormatCommand(input) {
			return mode, true
		}
		if IsConfigureCommand(input) {
			return mode, false
		}
		if ShouldEnterConfigMode(input, registered) {
			mode = ModeConfig
		}
	case ModeConfig:
	default:
		return mode, false
	}
	line := input
	if mode == ModeConfig {
		line, _ = LineBehindTop(input)
		if strings.HasPrefix(line, cmdRun+" ") {
			return mode, true
		}
		if line == cmdRun {
			return mode, false
		}
	}
	switch line {
	case cmdExit, cmdQuit, cmdStop, cmdRestart:
		return mode, false
	}
	return mode, true
}
