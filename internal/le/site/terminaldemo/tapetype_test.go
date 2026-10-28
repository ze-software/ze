package siteterminaldemo

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// framedEditor is a line editor that paints like a full-screen terminal UI:
// it draws the line only once keystrokes pause for one frame, and Enter
// clears the line without drawing the keys still waiting for that frame. The
// ze editor (bubbletea) paints on its own frame tick the same way, which is
// how the showcase recording lost the last letter of a typed command.
const framedEditor = `stty -echo 2>/dev/null
line=
dirty=
printf '> '
while :; do
	IFS= read -rsn1 -t 0.1 key
	status=$?
	if [ "$status" -eq 0 ]; then
		if [ -z "$key" ]; then
			printf '\r> \033[K\r\nRAN:%s\r\n> ' "$line"
			line=
			dirty=
			continue
		fi
		line=$line$key
		dirty=1
		continue
	fi
	if [ "$status" -le 128 ]; then
		exit 0
	fi
	if [ -n "$dirty" ]; then
		printf '\r> %s\033[K' "$line"
		dirty=
	fi
done
`

// TestTypedCommandIsPaintedBeforeEnter proves a typed command reaches the
// screen whole before the recorder presses Enter. Before the fix the recorder
// sent Enter one typing interval after the last key, inside the program's
// frame, so the frame that would have shown the whole command was never
// drawn: the showcase cast showed "ze# show system hos", and the transcript
// check refused the recording because no viewer saw "ze# show system host".
// Method: drive a frame-painting editor through a real pty, read the cast
// through the transcript checker's own screen model, and require a painted
// prompt row holding the whole command before the line Enter produced.
func TestTypedCommandIsPaintedBeforeEnter(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "editor.sh")
	if err := os.WriteFile(script, []byte(framedEditor), 0o600); err != nil {
		t.Fatalf("write editor: %v", err)
	}
	t.Setenv("HOME", t.TempDir())
	output := filepath.Join(root, "typed.cast")
	settings := defaultTapeSettings()
	settings.shell = "bash --norc --noprofile " + script
	settings.typing = time.Millisecond
	settings.wait = shellWaitTimeout
	settings.resultHold = time.Millisecond
	tape := terminalTape{settings: settings, actions: []tapeAction{
		{name: tapeTypeCommand, text: "show system host", where: "t:1"},
		{name: "Enter", where: "t:2"},
		{name: tapeWaitCommand, pattern: regexp.MustCompile(`RAN:show system host`), where: "t:3"},
	}}
	if _, err := driveTape(tape, output, 80, 24); err != nil {
		t.Fatalf("drive tape: %v", err)
	}
	painted, err := castVisibleText(output)
	if err != nil {
		t.Fatalf("read cast: %v", err)
	}
	typed, ran := -1, -1
	for index, line := range painted {
		if typed < 0 && line == "> show system host" {
			typed = index
		}
		if ran < 0 && strings.HasPrefix(line, "RAN:") {
			ran = index
		}
	}
	if ran < 0 {
		t.Fatalf("the editor never ran the command; painted rows: %q", painted)
	}
	if typed < 0 || typed > ran {
		t.Fatalf("the viewer never saw the whole command before Enter ran it; painted rows: %q", painted)
	}
}
