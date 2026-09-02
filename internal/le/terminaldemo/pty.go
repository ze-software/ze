package terminaldemo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	defaultPTYTimeout = 15 * time.Second
	defaultPTYDelay   = time.Second
	settleQuiet       = 250 * time.Millisecond
	settleLimit       = 2 * time.Second
)

var (
	defaultReadyPattern = regexp.MustCompile(`ze[>#]`)
	closePattern        = regexp.MustCompile(`Connection .* closed|logout`)
	ansiPattern         = regexp.MustCompile(`\x1b(?:\[[\x30-\x3f]*[\x20-\x2f]*[\x40-\x7e]|\][^\x07]*(?:\x07|\x1b\\))`)
	erasePattern        = regexp.MustCompile(`\x1b\[[23]J`)
	durationPattern     = regexp.MustCompile(`^(\d+(?:\.\d+)?)(ms|s|m)$`)
)

var ptyKeys = map[string][]byte{
	"enter": {'\r'},
	"up":    {0x1b, '[', 'A'},
	"down":  {0x1b, '[', 'B'},
	"left":  {0x1b, '[', 'D'},
	"right": {0x1b, '[', 'C'},
	"tab":   {'\t'},
	"space": {' '},
}

type tapeSettings struct {
	fontSize int
	height   int
	padding  int
	shell    string
	typing   time.Duration
	wait     time.Duration
	width    int
}

type tapeAction struct {
	name     string
	text     string
	duration time.Duration
	pattern  *regexp.Regexp
	where    string
}

type terminalTape struct {
	output   string
	settings tapeSettings
	actions  []tapeAction
}

func defaultTapeSettings() tapeSettings {
	return tapeSettings{
		fontSize: 22,
		height:   600,
		padding:  60,
		shell:    "bash",
		typing:   50 * time.Millisecond,
		wait:     30 * time.Second,
		width:    1200,
	}
}

// RunPTY drives either a checked-in terminal tape or a legacy command list.
// It is the implementation behind ze-terminal-pty, the small binary copied into
// the demo renderer container.
func RunPTY(args []string, stdout, stderr io.Writer) int {
	options, err := parsePTYOptions(args)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}
	if options.tape != "" {
		if err := recordTape(options.tape, stdout); err != nil {
			_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}
	if err := driveCommands(options, stdout); err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

type ptyOptions struct {
	tape     string
	commands []string
	timeout  time.Duration
	ready    *regexp.Regexp
	delay    time.Duration
	program  []string
}

func parsePTYOptions(args []string) (ptyOptions, error) {
	options := ptyOptions{timeout: defaultPTYTimeout, ready: defaultReadyPattern, delay: defaultPTYDelay}
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if argument == "--" {
			options.program = append([]string(nil), args[index+1:]...)
			break
		}
		value := func(name string) (string, error) {
			if index+1 >= len(args) {
				return "", fmt.Errorf("%s needs a value", name)
			}
			index++
			return args[index], nil
		}
		switch argument {
		case "--tape":
			options.tape, _ = value(argument)
			if options.tape == "" {
				return ptyOptions{}, fmt.Errorf("--tape needs a value")
			}
		case "--command":
			command, err := value(argument)
			if err != nil {
				return ptyOptions{}, err
			}
			options.commands = append(options.commands, command)
		case "--timeout", "--delay":
			raw, err := value(argument)
			if err != nil {
				return ptyOptions{}, err
			}
			seconds, err := strconv.ParseFloat(raw, 64)
			if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
				return ptyOptions{}, fmt.Errorf("%s needs finite seconds >= 0, got %q", argument, raw)
			}
			if argument == "--timeout" {
				options.timeout = time.Duration(seconds * float64(time.Second))
			} else {
				options.delay = time.Duration(seconds * float64(time.Second))
			}
		case "--ready":
			raw, err := value(argument)
			if err != nil {
				return ptyOptions{}, err
			}
			options.ready, err = regexp.Compile(raw)
			if err != nil {
				return ptyOptions{}, fmt.Errorf("--ready needs a regex, got %q: %w", raw, err)
			}
		default:
			return ptyOptions{}, fmt.Errorf("unknown argument %q", argument)
		}
	}
	if options.tape != "" {
		if len(options.commands) != 0 || len(options.program) != 0 {
			return ptyOptions{}, errors.New("--tape takes no --command and no program")
		}
		return options, nil
	}
	if len(options.commands) == 0 {
		return ptyOptions{}, errors.New("--command is required unless --tape names a tape")
	}
	if len(options.program) == 0 {
		return ptyOptions{}, errors.New("program is required after --")
	}
	for _, command := range options.commands {
		if err := validatePTYCommand(command); err != nil {
			return ptyOptions{}, err
		}
	}
	last, _ := splitPTYDirective(options.commands[len(options.commands)-1])
	if last == tapeSleepDirective || last == tapeWaitDirective {
		return ptyOptions{}, errors.New("the last --command must not be '@sleep' or '@wait'")
	}
	return options, nil
}

func splitPTYDirective(command string) (string, string) {
	word, argument, found := strings.Cut(command, " ")
	if !strings.HasPrefix(word, "@") {
		return "", ""
	}
	if !found {
		argument = ""
	}
	return word, argument
}

func validatePTYCommand(command string) error {
	word, argument := splitPTYDirective(command)
	if word == "" {
		return nil
	}
	switch word {
	case "@escape":
		if argument != "" {
			return errors.New("directive '@escape' takes no argument")
		}
	case tapeSleepDirective:
		seconds, err := strconv.ParseFloat(strings.TrimSpace(argument), 64)
		if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
			return fmt.Errorf("directive '@sleep' needs finite seconds >= 0, got %q", argument)
		}
	case tapeWaitDirective:
		if strings.TrimSpace(argument) == "" {
			return errors.New("directive '@wait' needs an argument")
		}
		if _, err := regexp.Compile(argument); err != nil {
			return fmt.Errorf("directive '@wait' needs a regex, got %q: %w", argument, err)
		}
	case "@type":
		if strings.TrimSpace(argument) == "" {
			return errors.New("directive '@type' needs an argument")
		}
	case "@key":
		key := strings.TrimSpace(argument)
		if _, ok := ptyKeys[key]; !ok {
			return fmt.Errorf("directive '@key' needs one of down, enter, left, right, space, tab, up; got %q", argument)
		}
	default:
		return fmt.Errorf("unknown directive %q; use @escape, @key, @sleep, @type, @wait", word)
	}
	return nil
}

func parseTape(path, root string) (terminalTape, error) {
	tape := terminalTape{settings: defaultTapeSettings()}
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return terminalTape{}, err
		}
	}
	seen := make(map[string]bool)
	lines, err := tapeLines(path, root, seen)
	if err != nil {
		return terminalTape{}, err
	}
	for _, line := range lines {
		word, argument, _ := strings.Cut(line.text, " ")
		name, modifier, _ := strings.Cut(word, "+")
		requiresArgument := name == "Output" || name == "Screenshot" || name == "Set" || name == tapeSleepCommand || name == "Source" || name == tapeTypeCommand || name == tapeWaitCommand
		known := name == "Down" || name == "Enter" || name == "Escape" || name == "Hide" || name == "Left" || requiresArgument || name == "Show"
		if !known {
			return terminalTape{}, fmt.Errorf("%s: unknown directive %q", line.where, word)
		}
		if requiresArgument && strings.TrimSpace(argument) == "" {
			return terminalTape{}, fmt.Errorf("%s: directive %q needs an argument", line.where, name)
		}
		if !requiresArgument && strings.TrimSpace(argument) != "" {
			return terminalTape{}, fmt.Errorf("%s: directive %q takes no argument", line.where, name)
		}
		if modifier != "" && name != tapeWaitCommand {
			return terminalTape{}, fmt.Errorf("%s: directive %q takes no +%q", line.where, name, modifier)
		}
		switch name {
		case "Output":
			tape.output = strings.TrimSpace(argument)
		case "Screenshot":
		case "Set":
			if len(tape.actions) != 0 {
				return terminalTape{}, fmt.Errorf("%s: a Set after the session starts is refused", line.where)
			}
			if err := applyTapeSetting(&tape.settings, argument); err != nil {
				return terminalTape{}, fmt.Errorf("%s: %w", line.where, err)
			}
		default:
			action, err := parseTapeAction(name, modifier, argument, line.where)
			if err != nil {
				return terminalTape{}, err
			}
			tape.actions = append(tape.actions, action)
		}
	}
	return tape, nil
}

type tapeLine struct{ where, text string }

func tapeLines(path, root string, seen map[string]bool) ([]tapeLine, error) {
	resolved, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if seen[resolved] {
		return nil, fmt.Errorf("%s: sources itself", filepath.Base(path))
	}
	seen[resolved] = true
	content, err := os.ReadFile(path) //nolint:gosec // the tape names the file it includes, from the checked-in demo tree
	if err != nil {
		return nil, err
	}
	lines := make([]tapeLine, 0)
	for index, raw := range strings.Split(string(content), "\n") {
		text := strings.TrimSpace(raw)
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		where := fmt.Sprintf("%s:%d", filepath.Base(path), index+1)
		word, argument, _ := strings.Cut(text, " ")
		if word != "Source" {
			lines = append(lines, tapeLine{where: where, text: text})
			continue
		}
		name := strings.TrimSpace(argument)
		if name == "" {
			return nil, fmt.Errorf("%s: directive 'Source' needs a tape to source", where)
		}
		candidates := []string{filepath.Join(root, name), filepath.Join(filepath.Dir(path), name)}
		found := ""
		for _, candidate := range candidates {
			if info, statErr := os.Stat(candidate); statErr == nil && info.Mode().IsRegular() {
				found = candidate
				break
			}
		}
		if found == "" {
			return nil, fmt.Errorf("%s: sources %q, which is not a file", where, name)
		}
		nested, err := tapeLines(found, root, seen)
		if err != nil {
			return nil, err
		}
		lines = append(lines, nested...)
	}
	return lines, nil
}

func applyTapeSetting(settings *tapeSettings, argument string) error {
	key, value, _ := strings.Cut(strings.TrimSpace(argument), " ")
	value = strings.TrimSpace(value)
	switch key {
	case "FontFamily", "Framerate", "Theme":
		return nil
	case "Shell":
		quoted, err := tapeQuoted(value)
		if err != nil {
			return fmt.Errorf("tape Set Shell: %w", err)
		}
		settings.shell = quoted
	case "TypingSpeed", "WaitTimeout":
		duration, err := tapeDuration(value)
		if err != nil {
			return fmt.Errorf("tape Set %s: %w", key, err)
		}
		if key == "TypingSpeed" {
			settings.typing = duration
		} else {
			settings.wait = duration
		}
	case "FontSize", "Height", "Padding", "Width":
		pixels, err := strconv.Atoi(value)
		if err != nil || pixels <= 0 {
			return fmt.Errorf("tape Set %s needs a whole number of pixels greater than 0, got %q", key, value)
		}
		switch key {
		case "FontSize":
			settings.fontSize = pixels
		case "Height":
			settings.height = pixels
		case "Padding":
			settings.padding = pixels
		case "Width":
			settings.width = pixels
		}
	default:
		return fmt.Errorf("unknown Set key %q", key)
	}
	return nil
}

func tapeDuration(value string) (time.Duration, error) {
	match := durationPattern.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return 0, fmt.Errorf("needs a duration like '500ms' or '3s', got %q", value)
	}
	number, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, err
	}
	unit := time.Millisecond
	if match[2] == "s" {
		unit = time.Second
	}
	if match[2] == "m" {
		unit = time.Minute
	}
	return time.Duration(number * float64(unit)), nil
}

func tapeQuoted(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return "", fmt.Errorf("needs a double-quoted string, got %q", value)
	}
	inner := value[1 : len(value)-1]
	var output textbuf.Buffer
	for index := 0; index < len(inner); index++ {
		if inner[index] == '\\' && index+1 < len(inner) {
			index++
		}
		output.Byte(inner[index])
	}
	return output.String(), nil
}

func parseTapeAction(name, modifier, argument, where string) (tapeAction, error) {
	action := tapeAction{name: name, where: where}
	switch name {
	case tapeTypeCommand:
		text, err := tapeQuoted(argument)
		if err != nil {
			return tapeAction{}, fmt.Errorf("%s: Type %w", where, err)
		}
		action.text = text
	case tapeSleepCommand:
		duration, err := tapeDuration(argument)
		if err != nil {
			return tapeAction{}, fmt.Errorf("%s: Sleep %w", where, err)
		}
		action.duration = duration
	case tapeWaitCommand:
		if modifier != "Screen" {
			return tapeAction{}, fmt.Errorf("%s: use Wait+Screen /regex/", where)
		}
		text := strings.TrimSpace(argument)
		if len(text) < 2 || text[0] != '/' || text[len(text)-1] != '/' {
			return tapeAction{}, fmt.Errorf("%s: Wait+Screen needs a /regex/, got %q", where, argument)
		}
		var err error
		action.pattern, err = regexp.Compile(strings.ReplaceAll(text[1:len(text)-1], `\/`, `/`))
		if err != nil {
			return tapeAction{}, fmt.Errorf("%s: Wait+Screen needs a regex: %w", where, err)
		}
	}
	return action, nil
}

func terminalGrid(settings tapeSettings) (int, int, error) {
	cellWidth := max(1, int(math.Round(float64(settings.fontSize)*0.6)))
	cellHeight := max(1, int(math.Ceil(float64(settings.fontSize)*1.32)))
	columns := (settings.width - 2*settings.padding) / cellWidth
	rows := (settings.height - 2*settings.padding) / cellHeight
	if columns < 1 || rows < 1 {
		return 0, 0, fmt.Errorf("a %dx%d box padded by %d holds no %dx%d cell", settings.width, settings.height, settings.padding, cellWidth, cellHeight)
	}
	return columns, rows, nil
}

func recordTape(path string, stdout io.Writer) error {
	tape, err := parseTape(path, "")
	if err != nil {
		return err
	}
	if tape.output == "" {
		return fmt.Errorf("%s: no Output: the tape names no artifact", path)
	}
	columns, rows, err := terminalGrid(tape.settings)
	if err != nil {
		return err
	}
	output := strings.TrimSuffix(tape.output, filepath.Ext(tape.output)) + ".cast"
	if err := os.MkdirAll(filepath.Dir(output), 0o750); err != nil {
		return err
	}
	writer, err := driveTape(tape, output, columns, rows)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "recorded %s (%d events, %.1fs, %dx%d)\n", output, writer.events, writer.duration(), columns, rows)
	return err
}

func startPTY(program []string, columns, rows int) (*exec.Cmd, *os.File, error) {
	command := exec.CommandContext(context.Background(), program[0], program[1:]...) //nolint:gosec // the checked-in tape or the caller explicitly names the program
	master, err := pty.StartWithSize(command, &pty.Winsize{Rows: uint16(rows), Cols: uint16(columns)})
	return command, master, err
}

func terminatePTY(command *exec.Cmd, master *os.File) {
	_ = master.Close()
	if command.Process != nil {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
		done := make(chan struct{})
		go func() { _ = command.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(time.Second):
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
			<-done
		}
	}
}

// pollPTY waits for master to become readable, or for deadline to pass.
//
// unix.Poll is a raw syscall: a signal delivered to this process while it
// blocks in poll (SIGCHLD from any other child this test binary runs
// concurrently is enough) interrupts it with EINTR. Go's own os.File.Read
// and os.File.Write already retry EINTR internally (internal/poll), but a
// direct syscall wrapper like unix.Poll does not, so pollPTY retries it
// itself rather than surface EINTR as a session failure.
func pollPTY(master *os.File, deadline time.Time) (bool, error) {
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false, nil
		}
		milliseconds := max(int(remaining/time.Millisecond), 1)
		fds := []unix.PollFd{{Fd: int32(master.Fd()), Events: unix.POLLIN}}
		count, err := unix.Poll(fds, milliseconds)
		if err != nil {
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			return false, err
		}
		return count != 0 && fds[0].Revents&(unix.POLLIN|unix.POLLHUP|unix.POLLERR) != 0, nil
	}
}

func readPTY(master *os.File) ([]byte, error) {
	buffer := make([]byte, 64*1024)
	count, err := master.Read(buffer)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, syscall.EIO) {
			return nil, io.EOF
		}
		return nil, err
	}
	if count == 0 {
		return nil, io.EOF
	}
	return buffer[:count], nil
}

type utf8Stream struct{ pending []byte }

func (decoder *utf8Stream) decode(data []byte) string {
	data = append(decoder.pending, data...)
	decoder.pending = decoder.pending[:0]
	var text textbuf.Buffer
	for len(data) != 0 {
		if !utf8.FullRune(data) {
			decoder.pending = append(decoder.pending, data...)
			break
		}
		r, width := utf8.DecodeRune(data)
		text.WriteRune(r)
		data = data[width:]
	}
	return text.String()
}

type castWriter struct {
	file     *os.File
	origin   time.Time
	hiddenAt time.Time
	cleared  bool
	residue  []string
	last     float64
	events   int
}

func newCastWriter(path string, columns, rows int) (*castWriter, error) {
	// Unlink before create. The recorder writes into a directory the host shares
	// with this container, and renderDemo deletes the previous recording from the
	// HOST side just before the container starts. A virtiofs guest keeps its own
	// dentry for that name, so an O_CREAT that reuses the name resolves to the
	// inode the host removed and fails ENOENT. Removing the name here is a guest
	// operation, so the guest's cache agrees with what the create then does.
	// Measured under colima: 9 failures in 10 without this line, 0 in 10 with it.
	_ = os.Remove(path)
	file, err := os.Create(path) // #nosec G304 -- path comes from the checked-in tape's Output directive.
	if err != nil {
		// A tape names its Output relative to the demo tree, so the path alone
		// does not say where the recorder looked. The directory it ran in is what
		// the reader needs to see.
		where, cwdErr := os.Getwd()
		if cwdErr != nil {
			where = cwdErr.Error()
		}
		return nil, fmt.Errorf("create the recording %s from %s: %w", path, where, err)
	}
	writer := &castWriter{file: file, origin: time.Now()}
	header, _ := json.Marshal(map[string]int{commandVersion: 2, "width": columns, "height": rows})
	if _, err := fmt.Fprintf(file, "%s\n", header); err != nil {
		_ = file.Close()
		return nil, err
	}
	return writer, nil
}

func (writer *castWriter) duration() float64 { return writer.last }
func (writer *castWriter) write(text string) error {
	if !writer.hiddenAt.IsZero() {
		matches := erasePattern.FindAllStringIndex(text, -1)
		if len(matches) == 0 {
			writer.residue = append(writer.residue, text)
			return nil
		}
		writer.cleared = true
		writer.residue = []string{text[matches[len(matches)-1][1]:]}
		return nil
	}
	return writer.emit(text)
}
func (writer *castWriter) emit(text string) error {
	if text == "" {
		return nil
	}
	writer.last = math.Max(writer.last, math.Round(time.Since(writer.origin).Seconds()*1e6)/1e6)
	event, _ := json.Marshal([]any{writer.last, "o", text})
	if _, err := fmt.Fprintf(writer.file, "%s\n", event); err != nil {
		return err
	}
	writer.events++
	return nil
}
func (writer *castWriter) hide() {
	if writer.hiddenAt.IsZero() {
		writer.hiddenAt = time.Now()
		writer.cleared = false
		writer.residue = nil
	}
}
func (writer *castWriter) show() error {
	if writer.hiddenAt.IsZero() {
		return nil
	}
	writer.origin = writer.origin.Add(time.Since(writer.hiddenAt))
	writer.hiddenAt = time.Time{}
	if writer.cleared {
		if err := writer.emit("\x1b[H\x1b[2J\x1b[3J" + strings.Join(writer.residue, "")); err != nil {
			return err
		}
	}
	writer.cleared = false
	writer.residue = nil
	return nil
}

type tapeSession struct {
	master  *os.File
	writer  *castWriter
	screen  *terminalScreen
	decoder utf8Stream
	typing  time.Duration
	timeout time.Duration
	closed  bool
}

func (session *tapeSession) read() error {
	chunk, err := readPTY(session.master)
	if errors.Is(err, io.EOF) {
		session.closed = true
		return nil
	}
	if err != nil {
		return err
	}
	text := session.decoder.decode(chunk)
	if err := session.writer.write(text); err != nil {
		return err
	}
	session.screen.settle(text)
	return nil
}
func (session *tapeSession) pump(duration time.Duration) error {
	deadline := time.Now().Add(duration)
	for !session.closed && time.Now().Before(deadline) {
		ready, err := pollPTY(session.master, deadline)
		if err != nil {
			return err
		}
		if ready {
			if err := session.read(); err != nil {
				return err
			}
		}
	}
	return nil
}
func (session *tapeSession) waitForFirstOutput() error {
	deadline := time.Now().Add(session.timeout)
	for session.writer.events == 0 {
		if session.closed {
			return errors.New("terminal closed before the shell produced output")
		}
		if time.Now().After(deadline) {
			return errors.New("timed out waiting for the shell to produce output")
		}
		ready, err := pollPTY(session.master, deadline)
		if err != nil {
			return err
		}
		if ready {
			if err := session.read(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (session *tapeSession) settle() error {
	limit := time.Now().Add(settleLimit)
	quiet := time.Now().Add(settleQuiet)
	for !session.closed && time.Now().Before(quiet) && time.Now().Before(limit) {
		before := session.writer.events
		ready, err := pollPTY(session.master, quiet)
		if err != nil {
			return err
		}
		if ready {
			if err := session.read(); err != nil {
				return err
			}
		}
		if session.writer.events != before {
			quiet = time.Now().Add(settleQuiet)
		}
	}
	return nil
}
func (session *tapeSession) waitFor(pattern *regexp.Regexp) error {
	deadline := time.Now().Add(session.timeout)
	for !pattern.MatchString(session.screen.text()) {
		if session.closed {
			return fmt.Errorf("terminal closed while waiting for %q", pattern.String())
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for %q; screen: %s", pattern.String(), session.screen.text())
		}
		ready, err := pollPTY(session.master, deadline)
		if err != nil {
			return err
		}
		if ready {
			if err := session.read(); err != nil {
				return err
			}
		}
	}
	return session.settle()
}
func (session *tapeSession) send(data []byte) error {
	if session.closed {
		return errors.New("terminal closed before the tape ended")
	}
	if _, err := session.master.Write(data); err != nil {
		return err
	}
	return session.pump(session.typing)
}
func (session *tapeSession) typeText(text string) error {
	for _, character := range text {
		if err := session.send([]byte(string(character))); err != nil {
			return err
		}
	}
	return nil
}

func driveTape(tape terminalTape, output string, columns, rows int) (*castWriter, error) {
	command, master, err := startPTY(strings.Fields(tape.settings.shell), columns, rows)
	if err != nil {
		return nil, err
	}
	writer, err := newCastWriter(output, columns, rows)
	if err != nil {
		terminatePTY(command, master)
		return nil, err
	}
	session := &tapeSession{master: master, writer: writer, screen: newTerminalScreen(rows, columns), typing: tape.settings.typing, timeout: tape.settings.wait}
	defer func() { terminatePTY(command, master); _ = writer.file.Close() }()
	if err := session.waitForFirstOutput(); err != nil {
		return nil, err
	}
	if err := session.settle(); err != nil {
		return nil, err
	}
	for _, action := range tape.actions {
		switch action.name {
		case "Hide":
			writer.hide()
		case "Show":
			err = writer.show()
		case tapeSleepCommand:
			err = session.pump(action.duration)
		case tapeWaitCommand:
			err = session.waitFor(action.pattern)
		case tapeTypeCommand:
			err = session.typeText(action.text)
		case "Enter":
			err = session.send(ptyKeys["enter"])
		case "Escape":
			err = session.send([]byte{0x1b})
		case "Left":
			err = session.send(ptyKeys["left"])
		case "Down":
			err = session.send(ptyKeys["down"])
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", action.where, err)
		}
	}
	return writer, nil
}

func driveCommands(options ptyOptions, stdout io.Writer) error {
	command, master, err := startPTY(options.program, 120, 40)
	if err != nil {
		return err
	}
	defer terminatePTY(command, master)
	captured := make([]byte, 0)
	initial, err := readUntilPTY(master, options.ready, options.timeout, nil, false)
	if err != nil {
		return err
	}
	captured = append(captured, initial...)
	window := len(captured)
	for index, commandText := range options.commands {
		last := index == len(options.commands)-1
		directive, argument := splitPTYDirective(commandText)
		following := ""
		if !last {
			following, _ = splitPTYDirective(options.commands[index+1])
		}
		if directive == tapeWaitDirective {
			pattern := regexp.MustCompile(argument)
			chunk, err := readUntilPTY(master, pattern, options.timeout, captured[window:], false)
			if err != nil {
				return err
			}
			captured = append(captured, chunk...)
			window = len(captured)
			continue
		}
		window = len(captured)
		switch directive {
		case "@escape":
			_, err = master.Write([]byte{0x1b})
		case "@type":
			_, err = master.WriteString(argument)
		case "@key":
			_, err = master.Write(ptyKeys[strings.TrimSpace(argument)])
		case tapeSleepDirective:
			seconds, _ := strconv.ParseFloat(strings.TrimSpace(argument), 64)
			var chunk []byte
			chunk, err = readForPTY(master, time.Duration(seconds*float64(time.Second)))
			captured = append(captured, chunk...)
		default:
			_, err = master.Write(append([]byte(commandText), '\r'))
		}
		if err != nil {
			return err
		}
		if directive == tapeSleepDirective || following == tapeWaitDirective {
			continue
		}
		if !last {
			chunk, err := readForPTY(master, options.delay)
			if err != nil {
				return err
			}
			captured = append(captured, chunk...)
		} else {
			chunk, err := readUntilPTY(master, closePattern, options.timeout, nil, true)
			if err != nil {
				return err
			}
			captured = append(captured, chunk...)
		}
	}
	_, err = stdout.Write(ansiPattern.ReplaceAll(captured, nil))
	return err
}

func readForPTY(master *os.File, duration time.Duration) ([]byte, error) {
	deadline := time.Now().Add(duration)
	var output bytes.Buffer
	for time.Now().Before(deadline) {
		ready, err := pollPTY(master, deadline)
		if err != nil {
			return nil, err
		}
		if !ready {
			break
		}
		chunk, err := readPTY(master)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		output.Write(chunk)
	}
	return output.Bytes(), nil
}

func readUntilPTY(master *os.File, pattern *regexp.Regexp, timeout time.Duration, seen []byte, eofOK bool) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	search := append([]byte(nil), seen...)
	var output bytes.Buffer
	for {
		// The pattern describes what a reader SEES, so it is matched with the
		// escape sequences removed. A phrase that carries a style change inside
		// it ("> show" changes color after the cursor) never matches the raw
		// stream, and the failure reads as a phrase that is plainly on screen.
		if pattern.Match(ansiPattern.ReplaceAll(search, nil)) {
			return output.Bytes(), nil
		}
		ready, err := pollPTY(master, deadline)
		if err != nil {
			return nil, err
		}
		if !ready {
			return nil, fmt.Errorf("timed out waiting for %q; output: %s", pattern.String(), ansiPattern.ReplaceAll(search, nil))
		}
		chunk, err := readPTY(master)
		if errors.Is(err, io.EOF) {
			if eofOK {
				return output.Bytes(), nil
			}
			return nil, fmt.Errorf("terminal closed while waiting for %q", pattern.String())
		}
		if err != nil {
			return nil, err
		}
		output.Write(chunk)
		search = append(search, chunk...)
	}
}
