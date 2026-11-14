// Design: docs/features/ai-first.md — setting an operator's agent up to use Ze
// Related: agent_skill.go — the plan whose hook items these functions act on
// Related: agent_stub.go — the note helpers the hook rows share
//
// Every call that reads or writes a harness's settings file is in this file.
// A settings file belongs to the operator and to other installers, so Ze reads
// it once, finds the places it acts on with a scanner over the JSON text, and
// splices its own group in or out: every byte outside that text stays as it
// was. A shape Ze does not fully understand is refused, untouched. The new
// content is parsed, written beside the target, and renamed over it only while
// the target is still the file Ze read.

package skills

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// hookCommand is the command of Ze's session-start entry. It is a fixed
// string, with nothing interpolated: the shell discards ze's error stream and
// ends in success, so a missing or failing ze never fails a session.
const hookCommand = "/bin/sh -c 'ze show agent context 2>/dev/null || true'"

// hookTimeoutSeconds is the timeout of Ze's entry, in seconds.
const hookTimeoutSeconds = 5

// hookOwnerWords are the words, in this order, whose presence in an entry's
// command makes the entry Ze's.
var hookOwnerWords = []string{"ze", "show", "agent", "context"}

// hookCommandJSON is hookCommand as a JSON string, written without the HTML
// escapes json.Marshal adds, so the file shows the command as it runs.
var hookCommandJSON = jsonQuoted(hookCommand)

// settingsBytesMax bounds the settings file Ze reads.
const settingsBytesMax = 4 << 20

// The keys of the hook list, in both harnesses' settings files.
const (
	settingsKeyHooks        = "hooks"
	settingsKeySessionStart = "SessionStart"
	settingsKeyCommand      = "command"
)

// indentDefault is the indentation step of a file that has none.
const indentDefault = "  "

// createHook is create agent skill's hook item for h's settings file at file,
// an absolute path. rerun is the command line that runs the command again.
func createHook(h *harness, file, rerun string) outcome {
	return hookEditor{}.create(h, file, rerun)
}

// inspectHook is show agent skill's hook item. It changes nothing.
func inspectHook(h *harness, file, rerun string) outcome {
	read, err := readSettings(file)
	if err != nil {
		return readRefused(file, err, rerun)
	}
	if !read.exists {
		return outcome{state: stateMissing, path: file}
	}
	layout, err := readLayout(read.data)
	if err != nil {
		return readRefused(file, err, rerun)
	}
	if !layout.hasZe {
		return outcome{state: stateMissing, path: file}
	}
	if !layout.zeCurrent {
		return outcome{state: stateOutdated, path: file, note: noteLines(
			"This hook is Ze's, written by another ze version. To write the one this ze carries, run:",
			"ze create agent skill")}
	}
	if h.hookTrustByPosition {
		return outcome{state: stateCurrent, path: file, note: h.title + " keeps which hooks it trusts in its own " +
			"configuration, which Ze does not read, so Ze cannot tell whether it runs this one. " +
			"To check, open " + h.title + " and run /hooks."}
	}
	return outcome{state: stateCurrent, path: file}
}

// deleteHook is delete agent skill's hook item.
func deleteHook(h *harness, file, rerun string) outcome {
	return hookEditor{}.delete(h, file, rerun)
}

// hookEditor writes a settings file. Its zero value is the editor every
// command uses.
type hookEditor struct {
	// beforeReplace, when set, runs after the new content is written beside
	// the target and before Ze checks the target and replaces it. A test sets
	// it to change the file in that moment.
	beforeReplace func(target string)
}

// create installs Ze's group, or rewrites an outdated one in place.
func (e hookEditor) create(h *harness, file, rerun string) outcome {
	read, err := readSettings(file)
	if err != nil {
		return readRefused(file, err, rerun)
	}
	state := stateCreated
	content := newSettings()
	if read.exists {
		layout, err := readLayout(read.data)
		if err != nil {
			return readRefused(file, err, rerun)
		}
		if layout.hasZe && layout.zeCurrent {
			return outcome{state: stateUnchanged, path: file}
		}
		if layout.hasZe {
			content, state = layout.refreshed(read.data), stateUpdated
		} else {
			content = layout.withGroup(read.data)
		}
	}
	if err := e.replace(read, content); err != nil {
		return writeRefused(file, err, rerun)
	}
	note := linkedNote(read)
	if h.hookTrustByPosition {
		note = joinNotes(note, h.title+" skips a new or changed hook until you trust it. To trust it, open "+
			h.title+", run /hooks, and trust the hook that runs ze show agent context.")
	}
	return outcome{state: state, path: file, note: note}
}

// delete removes Ze's group. In a file whose harness trusts a hook by its
// position, a group that is not the last is emptied in place instead, so no
// group after it moves and loses its trust.
func (e hookEditor) delete(h *harness, file, rerun string) outcome {
	read, err := readSettings(file)
	if err != nil {
		return readRefused(file, err, rerun)
	}
	if !read.exists {
		return outcome{state: stateAbsent, path: file}
	}
	layout, err := readLayout(read.data)
	if err != nil {
		return readRefused(file, err, rerun)
	}
	if !layout.hasZe {
		return outcome{state: stateAbsent, path: file}
	}
	content, emptied := layout.withoutGroup(read.data, h.hookTrustByPosition)
	if err := e.replace(read, content); err != nil {
		return writeRefused(file, err, rerun)
	}
	note := linkedNote(read)
	if emptied {
		note = joinNotes(note, "Ze emptied its group's hook list and left the group in place, because "+h.title+
			" trusts each hook by its position, and removing the group would move every group after it.")
	}
	return outcome{state: stateRemoved, path: file, note: note}
}

// --- Reading and replacing the file -----------------------------------------

// settingsRead is a settings file as Ze read it.
type settingsRead struct {
	// file is the path the harness names, and target the regular file it
	// finally names, through every link.
	file   string
	target string
	// exists says target exists. Without it, data and info are unset.
	exists bool
	data   []byte
	// info is target's state when Ze read it: its mode, size and
	// modification time.
	info os.FileInfo
}

// settingsRefusal is why Ze leaves a settings file as it is: what it found, why
// that stops it, and the lead-in to the command that runs the command again.
type settingsRefusal struct {
	found string
	why   string
	fix   string
}

func (r *settingsRefusal) Error() string { return r.found }

// readSettings reads the file at file once, following every link to the
// regular file it finally names. A missing file answers exists false; a
// dangling link, or a target that is not a regular file, is refused.
func readSettings(file string) (settingsRead, error) {
	read := settingsRead{file: file, target: file}
	info, err := os.Lstat(file)
	// refuse-by-default: guards newSettings -- a missing file is read.exists false, which create answers with a new file
	if errors.Is(err, os.ErrNotExist) {
		return read, nil
	}
	if err != nil {
		return settingsRead{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := filepath.EvalSymlinks(file)
		if errors.Is(err, os.ErrNotExist) {
			return settingsRead{}, &settingsRefusal{
				found: file + " is a link to a file that does not exist",
				why:   "Ze writes a linked settings file through to its target, and never creates the file a link points at.",
				fix:   "Point the link at the settings file, or remove the link, then run:"}
		}
		if err != nil {
			return settingsRead{}, err
		}
		read.target = target
	}
	return readTarget(read)
}

// readTarget reads read.target, which must be a regular file of at most
// settingsBytesMax bytes. It is checked before it is opened, so a link to a
// pipe never blocks the command.
func readTarget(read settingsRead) (settingsRead, error) {
	info, err := os.Stat(read.target)
	if err != nil {
		return settingsRead{}, err
	}
	if !info.Mode().IsRegular() {
		return settingsRead{}, &settingsRefusal{
			found: read.target + " is not a regular file",
			why:   "Ze edits only a regular settings file.",
			fix:   "Move what is at that place away, then run:"}
	}
	f, err := os.Open(read.target) //nolint:gosec // the harness's settings file, built from the harness table
	if err != nil {
		return settingsRead{}, err
	}
	info, err = f.Stat()
	if err == nil {
		read.data, err = io.ReadAll(io.LimitReader(f, settingsBytesMax+1))
	}
	closeErr := f.Close()
	if err != nil {
		return settingsRead{}, err
	}
	if closeErr != nil {
		return settingsRead{}, closeErr
	}
	if len(read.data) > settingsBytesMax {
		var found textbuf.Buffer
		found.Str(read.target).Str(" is larger than ").Int(settingsBytesMax).Str(" bytes")
		return settingsRead{}, &settingsRefusal{
			found: found.String(),
			why:   "Ze reads a settings file whole, and a file this large is not one a harness writes.",
			fix:   "Correct the file, or add Ze's hook by hand, then run:"}
	}
	read.info, read.exists = info, true
	return read, nil
}

// replace puts content in place of what read found: it parses content, writes
// it to a temporary file in the target's own directory with the target's
// permission bits (owner-only for a new file), checks that the target is
// still the file Ze read, then renames the temporary file over it. A link at
// read.file stays a link. A missing directory is created owner-only.
func (e hookEditor) replace(read settingsRead, content []byte) error {
	if !json.Valid(content) {
		return &settingsRefusal{
			found: "Ze's edit of " + read.target + " is not valid JSON, which is a defect in Ze",
			why:   "Ze replaces a settings file only with content that parses, so it left the file as it was.",
			fix:   "Add Ze's hook by hand, then run:"}
	}
	directory := filepath.Dir(read.target)
	mode := os.FileMode(0o600)
	if read.exists {
		mode = read.info.Mode().Perm()
		if err := writable(read.target); err != nil {
			return err
		}
	} else if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(read.target)+".ze-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	_, err = temporary.Write(content)
	if err == nil {
		err = temporary.Chmod(mode)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err == nil && e.beforeReplace != nil {
		e.beforeReplace(read.target)
	}
	if err == nil {
		err = stillAsRead(read)
	}
	if err == nil {
		err = os.Rename(name, read.target)
	}
	if err != nil {
		return errors.Join(err, removeTemporary(name))
	}
	return nil
}

// writable answers an error when the file cannot be opened for writing, so a
// file the operator made read-only is not replaced by a rename.
func writable(target string) error {
	f, err := os.OpenFile(target, os.O_WRONLY, 0) //nolint:gosec // the harness's settings file, built from the harness table
	if err != nil {
		return err
	}
	return f.Close()
}

// stillAsRead answers a refusal when the target changed since Ze read it: its
// size or modification time differ, or it appeared or went away.
func stillAsRead(read settingsRead) error {
	changed := &settingsRefusal{
		found: read.target + " changed while Ze was editing it",
		why:   "Another program wrote it after Ze read it, so Ze left that program's content as it is.",
		fix:   "Run the command again:"}
	info, err := os.Stat(read.target)
	if errors.Is(err, os.ErrNotExist) {
		if read.exists {
			return changed
		}
		// refuse-by-default: guards newSettings -- a file absent at the read and still absent is unchanged
		return nil
	}
	if err != nil {
		return err
	}
	if !read.exists {
		return changed
	}
	if info.Size() != read.info.Size() {
		return changed
	}
	if !info.ModTime().Equal(read.info.ModTime()) {
		return changed
	}
	return nil
}

// removeTemporary removes a temporary file Ze wrote and did not rename.
func removeTemporary(name string) error {
	err := os.Remove(name)
	// refuse-by-default: guards replace -- a temporary file already gone leaves nothing to remove
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// readRefused is the row of a settings file Ze could not read or understand.
func readRefused(file string, err error, rerun string) outcome {
	if refusal, isRefusal := errors.AsType[*settingsRefusal](err); isRefusal {
		return refusedRow(file, refusal, rerun)
	}
	return outcome{state: stateRefused, path: file, note: failedNote("read", file, err, rerun)}
}

// writeRefused is the row of a settings file Ze could not replace.
func writeRefused(file string, err error, rerun string) outcome {
	if refusal, isRefusal := errors.AsType[*settingsRefusal](err); isRefusal {
		return refusedRow(file, refusal, rerun)
	}
	return outcome{state: stateRefused, path: file, note: failedNote("write", file, err, rerun)}
}

// refusedRow is a refused row whose note is the refusal, with the command
// that runs the command again on its last line.
func refusedRow(file string, refusal *settingsRefusal, rerun string) outcome {
	return outcome{state: stateRefused, path: file, note: noteLines(refusal.found+".", refusal.why, refusal.fix, rerun)}
}

// linkedNote says where Ze wrote when the harness's file is a link.
func linkedNote(read settingsRead) string {
	if read.target == read.file {
		return ""
	}
	return "This file is a link. Ze wrote the file it points at, " + read.target + ", and kept the link."
}

// --- Where the hook list sits -----------------------------------------------

// jsonSpan is the bytes [start, end) of one JSON value in a settings file.
type jsonSpan struct {
	start int
	end   int
}

// jsonMember is one member of a JSON object: its decoded name, the offset of
// its key, and its value.
type jsonMember struct {
	name  string
	key   int
	value jsonSpan
}

// jsonContainer is an object or a list in a settings file: the offsets of its
// two brackets and, unless it is empty, where its last member's value ends.
type jsonContainer struct {
	open    int
	close   int
	empty   bool
	lastEnd int
}

// settingsLayout is where a settings file holds what Ze acts on.
type settingsLayout struct {
	root jsonContainer
	// hooks is the hooks object, when hasHooks says the file holds one.
	hooks    jsonContainer
	hasHooks bool
	// sessionStart is the SessionStart list, when hasSessionStart says hooks
	// holds one, and groups are its groups in order.
	sessionStart    jsonContainer
	hasSessionStart bool
	groups          []jsonSpan
	// ze is the index in groups of Ze's group, when hasZe says there is one.
	// zeCurrent says it is the group this binary writes, and zeEntries is its
	// hooks list.
	ze        int
	hasZe     bool
	zeCurrent bool
	zeEntries jsonSpan
	// unit is the file's indentation step.
	unit string
}

// shapeError is a settings file shape Ze does not fully understand: what it
// found, and the byte offset where.
type shapeError struct {
	found  string
	offset int
}

func (e *shapeError) Error() string { return e.found }

// errScanEnded is the scanner reaching the end of a text inside a value,
// which json.Valid rules out before the scan starts.
var errScanEnded = errors.New("the text ends inside a value")

// readLayout finds the hook list in data, refusing every shape Ze does not
// fully understand with what it found and where.
func readLayout(data []byte) (settingsLayout, error) {
	layout, err := scanLayout(data)
	if shape, isShape := errors.AsType[*shapeError](err); isShape {
		line, column := lineColumn(data, shape.offset)
		var found textbuf.Buffer
		found.Str("This file holds ").Str(shape.found).Str(" at line ").Int(int64(line)).
			Str(", column ").Int(int64(column))
		return settingsLayout{}, &settingsRefusal{
			found: found.String(),
			why:   "Ze changes a settings file only when it understands all of it, so it left this one as it is.",
			fix:   "Correct the file, or add Ze's hook by hand with the command " + hookCommand + ", then run:"}
	}
	return layout, err
}

// scanLayout is readLayout before its refusal is worded.
func scanLayout(data []byte) (settingsLayout, error) {
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		return settingsLayout{}, &shapeError{found: "a byte-order mark", offset: 0}
	}
	if !json.Valid(data) {
		return settingsLayout{}, invalidJSON(data)
	}
	layout := settingsLayout{unit: indentUnit(data)}
	// json.Valid accepted data, so a value starts here.
	open := skipSpace(data, 0)
	if data[open] != '{' {
		return settingsLayout{}, &shapeError{found: "a top level that is not a JSON object", offset: open}
	}
	members, root, err := objectMembers(data, open)
	if err != nil {
		return settingsLayout{}, err
	}
	layout.root = root
	hooks, found, err := soleMember(members, settingsKeyHooks)
	if err != nil {
		return settingsLayout{}, err
	}
	if found {
		if err := layout.readHooks(data, hooks); err != nil {
			return settingsLayout{}, err
		}
	}
	return layout, nil
}

// readHooks reads the hooks member, which must be an object, and its
// SessionStart list, which must be a list of objects.
func (l *settingsLayout) readHooks(data []byte, hooks jsonMember) error {
	if data[hooks.value.start] != '{' {
		return &shapeError{found: `a "hooks" value that is not an object`, offset: hooks.value.start}
	}
	members, container, err := objectMembers(data, hooks.value.start)
	if err != nil {
		return err
	}
	l.hooks, l.hasHooks = container, true
	sessionStart, found, err := soleMember(members, settingsKeySessionStart)
	if err != nil {
		return err
	}
	if found {
		return l.readSessionStart(data, sessionStart)
	}
	return nil
}

// readSessionStart reads the SessionStart list and finds Ze's group in it.
// Each group must be an object, and at most one may be Ze's.
func (l *settingsLayout) readSessionStart(data []byte, sessionStart jsonMember) error {
	if data[sessionStart.value.start] != '[' {
		return &shapeError{found: `a "SessionStart" value that is not a list`, offset: sessionStart.value.start}
	}
	groups, container, err := listElements(data, sessionStart.value.start)
	if err != nil {
		return err
	}
	l.groups, l.sessionStart, l.hasSessionStart = groups, container, true
	want := decoded([]byte(zeGroupText("", indentDefault)))
	for i, group := range groups {
		if data[group.start] != '{' {
			return &shapeError{found: "a SessionStart group that is not an object", offset: group.start}
		}
		entries, ours, err := zeEntriesOf(data, group)
		if err != nil {
			return err
		}
		if !ours {
			continue
		}
		if l.hasZe {
			return &shapeError{found: "a second group that runs ze show agent context", offset: group.start}
		}
		l.ze, l.hasZe, l.zeEntries = i, true, entries
		l.zeCurrent = reflect.DeepEqual(decoded(data[group.start:group.end]), want)
	}
	return nil
}

// zeEntriesOf answers whether the group is Ze's, and its hooks list when it
// is: a group is Ze's when it holds exactly one entry, whose command holds
// hookOwnerWords in order. Any other group is not Ze's, and is left as it is.
func zeEntriesOf(data []byte, group jsonSpan) (jsonSpan, bool, error) {
	members, _, err := objectMembers(data, group.start)
	if err != nil {
		return jsonSpan{}, false, err
	}
	hooks, found, err := soleMember(members, settingsKeyHooks)
	if err != nil {
		// A group with two hooks lists is not one Ze wrote.
		return jsonSpan{}, false, nil //nolint:nilerr // the group is not Ze's, which is the answer
	}
	if !found {
		return jsonSpan{}, false, nil
	}
	if data[hooks.value.start] != '[' {
		return jsonSpan{}, false, nil
	}
	entries, _, err := listElements(data, hooks.value.start)
	if err != nil {
		return jsonSpan{}, false, err
	}
	if len(entries) != 1 {
		return jsonSpan{}, false, nil
	}
	if data[entries[0].start] != '{' {
		return jsonSpan{}, false, nil
	}
	members, _, err = objectMembers(data, entries[0].start)
	if err != nil {
		return jsonSpan{}, false, err
	}
	command, found, err := soleMember(members, settingsKeyCommand)
	if err != nil {
		// An entry with two commands is not one Ze wrote.
		return jsonSpan{}, false, nil //nolint:nilerr // the group is not Ze's, which is the answer
	}
	if !found {
		return jsonSpan{}, false, nil
	}
	if data[command.value.start] != '"' {
		return jsonSpan{}, false, nil
	}
	var text string
	if err := json.Unmarshal(data[command.value.start:command.value.end], &text); err != nil {
		return jsonSpan{}, false, err
	}
	return hooks.value, ownsCommand(text), nil
}

// ownsCommand answers whether command holds hookOwnerWords, in order, as
// words: a path, a quote or a pipe around them does not matter.
func ownsCommand(command string) bool {
	words := strings.FieldsFunc(command, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_'
	})
	for i := 0; i+len(hookOwnerWords) <= len(words); i++ {
		if slices.Equal(words[i:i+len(hookOwnerWords)], hookOwnerWords) {
			return true
		}
	}
	return false
}

// soleMember answers the member named name, whether there is one, and a
// refusal when the name occurs twice.
func soleMember(members []jsonMember, name string) (jsonMember, bool, error) {
	var sole jsonMember
	found := false
	for i := range members {
		if members[i].name != name {
			continue
		}
		if found {
			return jsonMember{}, false, &shapeError{found: `a second "` + name + `" key`, offset: members[i].key}
		}
		sole, found = members[i], true
	}
	return sole, found, nil
}

// invalidJSON words the decoder's own complaint about data, with its offset.
func invalidJSON(data []byte) error {
	var value any
	err := json.Unmarshal(data, &value)
	if syntax, isSyntax := errors.AsType[*json.SyntaxError](err); isSyntax {
		return &shapeError{found: "text that is not valid JSON (" + syntax.Error() + ")", offset: int(syntax.Offset)}
	}
	return &shapeError{found: "text that is not valid JSON", offset: 0}
}

// decoded answers the JSON value of data, which json.Valid accepted, for an
// equality check.
func decoded(data []byte) any {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		panic("BUG: a JSON value json.Valid accepted does not decode: " + err.Error())
	}
	return value
}

// lineColumn answers the 1-based line and byte column of offset in data.
func lineColumn(data []byte, offset int) (line, column int) {
	offset = min(max(offset, 0), len(data))
	before := data[:offset]
	return 1 + bytes.Count(before, []byte{'\n'}), offset - bytes.LastIndexByte(before, '\n')
}

// --- The scanner --------------------------------------------------------------
//
// The scanner runs only on a text json.Valid accepted, so it reads structure
// and never judges it. Every loop is bounded by the text's length, which
// settingsBytesMax bounds.

// skipSpace answers the offset of the first byte at or after i that is not
// JSON whitespace.
func skipSpace(data []byte, i int) int {
	for i < len(data) && isSpace(data[i]) {
		i++
	}
	return i
}

// isSpace answers whether c is JSON whitespace.
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// skipString answers the offset after the string whose quote is at i.
func skipString(data []byte, i int) (int, error) {
	for j := i + 1; j < len(data); j++ {
		if data[j] == '\\' {
			j++
			continue
		}
		if data[j] == '"' {
			return j + 1, nil
		}
	}
	return 0, errScanEnded
}

// skipValue answers the offset after the value that starts at i.
func skipValue(data []byte, i int) (int, error) {
	if i >= len(data) {
		return 0, errScanEnded
	}
	if data[i] == '"' {
		return skipString(data, i)
	}
	if data[i] != '{' && data[i] != '[' {
		j := i
		for j < len(data) && !isSpace(data[j]) && data[j] != ',' && data[j] != '}' && data[j] != ']' {
			j++
		}
		return j, nil
	}
	depth := 0
	for j := i; j < len(data); j++ {
		switch data[j] {
		case '"':
			end, err := skipString(data, j)
			if err != nil {
				return 0, err
			}
			j = end - 1
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return j + 1, nil
			}
		}
	}
	return 0, errScanEnded
}

// objectMembers answers the members of the object whose brace is at open,
// and the object as a container.
func objectMembers(data []byte, open int) ([]jsonMember, jsonContainer, error) {
	var members []jsonMember
	i := skipSpace(data, open+1)
	if i < len(data) && data[i] == '}' {
		return nil, jsonContainer{open: open, close: i, empty: true}, nil
	}
	for i < len(data) {
		keyEnd, err := skipString(data, i)
		if err != nil {
			return nil, jsonContainer{}, err
		}
		var name string
		if err := json.Unmarshal(data[i:keyEnd], &name); err != nil {
			return nil, jsonContainer{}, err
		}
		// After the key: spaces, the colon, spaces, then the value.
		start := skipSpace(data, skipSpace(data, keyEnd)+1)
		end, err := skipValue(data, start)
		if err != nil {
			return nil, jsonContainer{}, err
		}
		members = append(members, jsonMember{name: name, key: i, value: jsonSpan{start: start, end: end}})
		next := skipSpace(data, end)
		if next < len(data) && data[next] == '}' {
			return members, jsonContainer{open: open, close: next, lastEnd: end}, nil
		}
		// A comma, then the next key.
		i = skipSpace(data, next+1)
	}
	return nil, jsonContainer{}, errScanEnded
}

// listElements answers the elements of the list whose bracket is at open,
// and the list as a container.
func listElements(data []byte, open int) ([]jsonSpan, jsonContainer, error) {
	var elements []jsonSpan
	i := skipSpace(data, open+1)
	if i < len(data) && data[i] == ']' {
		return nil, jsonContainer{open: open, close: i, empty: true}, nil
	}
	for i < len(data) {
		end, err := skipValue(data, i)
		if err != nil {
			return nil, jsonContainer{}, err
		}
		elements = append(elements, jsonSpan{start: i, end: end})
		next := skipSpace(data, end)
		if next < len(data) && data[next] == ']' {
			return elements, jsonContainer{open: open, close: next, lastEnd: end}, nil
		}
		// A comma, then the next element.
		i = skipSpace(data, next+1)
	}
	return nil, jsonContainer{}, errScanEnded
}

// --- The splices --------------------------------------------------------------

// withGroup answers data with Ze's group appended to the SessionStart list,
// inserting the list, and the hooks object, where the file has none.
func (l *settingsLayout) withGroup(data []byte) []byte {
	unit := l.unit
	if !l.hasHooks {
		return insertMember(data, l.root, unit, func(indent string) string { return hooksMemberText(indent, unit) })
	}
	if !l.hasSessionStart {
		return insertMember(data, l.hooks, unit, func(indent string) string { return sessionStartMemberText(indent, unit) })
	}
	return insertMember(data, l.sessionStart, unit, func(indent string) string { return zeGroupText(indent, unit) })
}

// refreshed answers data with Ze's outdated group rewritten in place, at the
// indentation of the line it starts on.
func (l *settingsLayout) refreshed(data []byte) []byte {
	group := l.groups[l.ze]
	return splice(data, group.start, group.end, zeGroupText(lineIndent(data, group.start), l.unit))
}

// withoutGroup answers data without Ze's group and its separating comma. With
// keepPosition, a group that is not the last keeps its place with an empty
// hooks list instead, and emptied says so. A list left empty stays.
func (l *settingsLayout) withoutGroup(data []byte, keepPosition bool) (content []byte, emptied bool) {
	group := l.groups[l.ze]
	last := l.ze == len(l.groups)-1
	if keepPosition && !last {
		return splice(data, l.zeEntries.start+1, l.zeEntries.end-1, ""), true
	}
	if len(l.groups) == 1 {
		return splice(data, l.sessionStart.open+1, l.sessionStart.close, ""), false
	}
	if last {
		return splice(data, l.groups[l.ze-1].end, group.end, ""), false
	}
	return splice(data, group.start, l.groups[l.ze+1].start, ""), false
}

// insertMember answers data with a last member added to c, on a line of its
// own one step deeper than the line c opens on. An empty c has its inside
// replaced; otherwise the member follows the last one after a comma.
func insertMember(data []byte, c jsonContainer, unit string, member func(indent string) string) []byte {
	base := lineIndent(data, c.open)
	indent := base + unit
	if c.empty {
		return splice(data, c.open+1, c.close, "\n"+indent+member(indent)+"\n"+base)
	}
	return splice(data, c.lastEnd, c.lastEnd, ",\n"+indent+member(indent))
}

// splice answers data with [start, end) replaced by text.
func splice(data []byte, start, end int, text string) []byte {
	out := make([]byte, 0, len(data)-(end-start)+len(text))
	out = append(out, data[:start]...)
	out = append(out, text...)
	return append(out, data[end:]...)
}

// lineIndent answers the spaces and tabs that open the line holding offset.
func lineIndent(data []byte, offset int) string {
	start := bytes.LastIndexByte(data[:offset], '\n') + 1
	end := start
	for end < offset && (data[end] == ' ' || data[end] == '\t') {
		end++
	}
	return string(data[start:end])
}

// indentUnit answers the file's indentation step: the leading spaces and tabs
// of its first indented line that holds more than whitespace, or
// indentDefault for a file with none.
func indentUnit(data []byte) string {
	for line := range bytes.SplitSeq(data, []byte{'\n'}) {
		text := bytes.TrimLeft(line, " \t")
		if len(text) == len(line) {
			continue
		}
		if len(bytes.TrimSpace(text)) == 0 {
			continue
		}
		return string(line[:len(line)-len(text)])
	}
	return indentDefault
}

// newSettings is the content of a settings file Ze creates: the hook list
// alone, two spaces deep, with a final newline.
func newSettings() []byte {
	return []byte("{\n" + indentDefault + hooksMemberText(indentDefault, indentDefault) + "\n}\n")
}

// hooksMemberText is the hooks member holding the SessionStart list with Ze's
// group, for a member whose line is indented by indent.
func hooksMemberText(indent, unit string) string {
	var b textbuf.Buffer
	b.Str(`"`).Str(settingsKeyHooks).Str(`": {`).Byte('\n').Str(indent).Str(unit)
	b.Str(sessionStartMemberText(indent+unit, unit)).Byte('\n').Str(indent).Byte('}')
	return b.String()
}

// sessionStartMemberText is the SessionStart member holding Ze's group.
func sessionStartMemberText(indent, unit string) string {
	var b textbuf.Buffer
	b.Str(`"`).Str(settingsKeySessionStart).Str(`": [`).Byte('\n').Str(indent).Str(unit)
	b.Str(zeGroupText(indent+unit, unit)).Byte('\n').Str(indent).Byte(']')
	return b.String()
}

// zeGroupText is Ze's group: no matcher, so it runs at every session start,
// and one entry, the command with its timeout.
func zeGroupText(indent, unit string) string {
	entry := indent + unit + unit + unit
	var b textbuf.Buffer
	b.Str("{\n").Str(indent).Str(unit).Str(`"`).Str(settingsKeyHooks).Str(`": [`).Byte('\n')
	b.Str(indent).Str(unit).Str(unit).Str("{\n")
	b.Str(entry).Str(`"type": "command",`).Byte('\n')
	b.Str(entry).Str(`"`).Str(settingsKeyCommand).Str(`": `).Str(hookCommandJSON).Str(",\n")
	b.Str(entry).Str(`"timeout": `).Int(hookTimeoutSeconds).Byte('\n')
	b.Str(indent).Str(unit).Str(unit).Str("}\n")
	b.Str(indent).Str(unit).Str("]\n")
	b.Str(indent).Byte('}')
	return b.String()
}

// jsonQuoted answers s as a JSON string, without the HTML escapes.
func jsonQuoted(s string) string {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(s); err != nil {
		panic("BUG: a string does not encode as JSON: " + err.Error())
	}
	return strings.TrimSuffix(out.String(), "\n")
}
