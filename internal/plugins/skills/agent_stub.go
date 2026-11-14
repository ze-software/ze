// Design: docs/features/ai-first.md — setting an operator's agent up to use Ze
// Related: agent_skill.go — the plan and the routes that call these
// Related: agent_hook.go — the session-start hook item of each harness
//
// What Ze finds at one stub's place, and what create, show and delete do
// there, each as an outcome with the note an operator reads. The file calls
// go through skillstub, which never writes through a link at the stub's own
// directory or file.

package skills

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/skillstub"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// leftAsItIsNote is the note of a delete row whose place holds something Ze
// did not write.
const leftAsItIsNote = "Ze did not write what is at this place, so it left it."

// stubFinding is what sits at a stub's place.
type stubFinding int

const (
	findingUnspecified stubFinding = iota
	// findingAbsent: no ze directory, or no SKILL.md in it.
	findingAbsent
	// findingLinked: the ze entry or SKILL.md is a link. Write replaces the
	// link, never its target.
	findingLinked
	// findingEntryAtDirectory: an entry that is neither a directory nor a
	// link, such as the operator's own file, sits where the ze directory
	// belongs. Ze did not write it, so no command removes it.
	findingEntryAtDirectory
	// findingEntryAtFile: an entry that is neither a regular file, a
	// directory nor a link, such as a pipe, sits where SKILL.md belongs. Ze
	// did not write it, so no command removes it.
	findingEntryAtFile
	// findingDirectoryAtFile: a directory sits where SKILL.md belongs.
	findingDirectoryAtFile
	// findingCurrent: Ze's stub, byte for byte the one this binary writes.
	findingCurrent
	// findingOutdated: Ze's stub by its marker, with other bytes.
	findingOutdated
	// findingForeign: a SKILL.md without Ze's marker.
	findingForeign
)

// stubPlace resolves a harness's skills directory and answers the stub's
// place in it. A linked skills directory is the operator's own placement
// (a dotfiles setup), so the stub goes in the directory it points at.
func stubPlace(skills string) (skillstub.Path, string, error) {
	directory := skills
	info, err := os.Lstat(skills)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return skillstub.Path{}, "", err
	}
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		directory, err = filepath.EvalSymlinks(skills)
		if err != nil {
			return skillstub.Path{}, "", err
		}
	}
	place, err := skillstub.At(directory, stubName, stubFile)
	if err != nil {
		panic("BUG: the stub's name or file is not one directory entry: " + err.Error())
	}
	return place, filepath.Join(directory, stubName), nil
}

// findStub answers what sits at place, against the stub body this binary
// writes. zeDirectory is place's own directory.
func findStub(place skillstub.Path, zeDirectory string, body []byte) (stubFinding, error) {
	info, err := os.Lstat(zeDirectory)
	if errors.Is(err, os.ErrNotExist) {
		return findingAbsent, nil
	}
	if err != nil {
		return findingUnspecified, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return findingLinked, nil
	}
	if !info.IsDir() {
		return findingEntryAtDirectory, nil
	}
	info, err = os.Lstat(place.File())
	if errors.Is(err, os.ErrNotExist) {
		return findingAbsent, nil
	}
	if err != nil {
		return findingUnspecified, err
	}
	if info.IsDir() {
		return findingDirectoryAtFile, nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return findingLinked, nil
	}
	if !info.Mode().IsRegular() {
		return findingEntryAtFile, nil
	}
	owned, err := place.OwnedBy(stubMarker)
	if err != nil {
		return findingUnspecified, err
	}
	if !owned {
		return findingForeign, nil
	}
	held, err := os.ReadFile(place.File()) //nolint:gosec // the stub's own place, built from the harness table
	if err != nil {
		return findingUnspecified, err
	}
	if bytes.Equal(held, body) {
		return findingCurrent, nil
	}
	return findingOutdated, nil
}

// createStub writes the stub unless Ze's current one is already there, and
// leaves a SKILL.md Ze did not write.
func createStub(skills string, body []byte, rerun string) outcome {
	place, zeDirectory, err := stubPlace(skills)
	if err != nil {
		return outcome{state: stateRefused, path: skills, note: failedNote("reach", skills, err, rerun)}
	}
	file := place.File()
	finding, err := findStub(place, zeDirectory, body)
	if err != nil {
		return outcome{state: stateRefused, path: file, note: failedNote("read", file, err, rerun)}
	}
	switch finding {
	case findingCurrent:
		return outcome{state: stateUnchanged, path: file}
	case findingForeign:
		return outcome{state: stateForeign, path: file, note: foreignNote(file, rerun)}
	case findingEntryAtDirectory:
		return outcome{state: stateForeign, path: zeDirectory, note: entryNote(zeDirectory, rerun)}
	case findingEntryAtFile:
		return outcome{state: stateForeign, path: file, note: entryNote(file, rerun)}
	case findingDirectoryAtFile:
		return outcome{state: stateRefused, path: file, note: directoryNote(file, rerun)}
	case findingAbsent, findingLinked, findingOutdated:
		if err := place.Write(body); err != nil {
			if _, isDirectory := errors.AsType[*skillstub.DirectoryAtFileError](err); isDirectory {
				return outcome{state: stateRefused, path: file, note: directoryNote(file, rerun)}
			}
			return outcome{state: stateRefused, path: file, note: failedNote("write", file, err, rerun)}
		}
		if finding == findingOutdated {
			return outcome{state: stateUpdated, path: file}
		}
		if finding == findingLinked {
			return outcome{state: stateCreated, path: file, note: "A link was at this place. Ze replaced the link " +
				"with its own file and left the link's target as it was."}
		}
		return outcome{state: stateCreated, path: file}
	case findingUnspecified:
		panic("BUG: findStub answered no finding without an error")
	}
	panic("BUG: findStub answered an unknown finding")
}

// inspectStub reports what sits at the stub's place and changes nothing.
// create is the command line that writes this harness's stub, for a fix: it
// names the harness, because a create without agent acts only on the
// harnesses found, which may leave this one out.
func inspectStub(skills string, body []byte, rerun, create string) outcome {
	place, zeDirectory, err := stubPlace(skills)
	if err != nil {
		return outcome{state: stateRefused, path: skills, note: failedNote("reach", skills, err, rerun)}
	}
	file := place.File()
	finding, err := findStub(place, zeDirectory, body)
	if err != nil {
		return outcome{state: stateRefused, path: file, note: failedNote("read", file, err, rerun)}
	}
	switch finding {
	case findingCurrent:
		return outcome{state: stateCurrent, path: file}
	case findingOutdated:
		return outcome{state: stateOutdated, path: file, note: noteLines(
			"This stub is Ze's, written by another ze version. To write the one this ze carries, run:",
			create)}
	case findingAbsent:
		return outcome{state: stateMissing, path: file}
	case findingLinked:
		return outcome{state: stateMissing, path: file, note: noteLines(
			"A link is at this place, not Ze's stub. Create replaces the link and leaves its target. Run:",
			create)}
	case findingForeign:
		return outcome{state: stateForeign, path: file, note: foreignNote(file, create)}
	case findingEntryAtDirectory:
		return outcome{state: stateForeign, path: zeDirectory, note: entryNote(zeDirectory, create)}
	case findingEntryAtFile:
		return outcome{state: stateForeign, path: file, note: entryNote(file, create)}
	case findingDirectoryAtFile:
		return outcome{state: stateRefused, path: file, note: directoryNote(file, create)}
	case findingUnspecified:
		panic("BUG: findStub answered no finding without an error")
	}
	panic("BUG: findStub answered an unknown finding")
}

// removeStub removes Ze's stub, current or outdated, and its emptied
// directory, and leaves everything else.
func removeStub(skills, rerun string) outcome {
	place, zeDirectory, err := stubPlace(skills)
	if err != nil {
		return outcome{state: stateRefused, path: skills, note: failedNote("reach", skills, err, rerun)}
	}
	file := place.File()
	finding, err := findStub(place, zeDirectory, nil)
	if err != nil {
		return outcome{state: stateRefused, path: file, note: failedNote("read", file, err, rerun)}
	}
	switch finding {
	case findingAbsent:
		return outcome{state: stateAbsent, path: file}
	case findingLinked, findingForeign, findingDirectoryAtFile, findingEntryAtFile:
		return outcome{state: stateForeign, path: file, note: leftAsItIsNote}
	case findingEntryAtDirectory:
		return outcome{state: stateForeign, path: zeDirectory, note: leftAsItIsNote}
	case findingCurrent, findingOutdated:
		removed, err := place.RemoveOwnedBy(stubMarker)
		if err != nil {
			return outcome{state: stateRefused, path: file, note: failedNote("remove", file, err, rerun)}
		}
		if !removed {
			return outcome{state: stateForeign, path: file, note: leftAsItIsNote}
		}
		return outcome{state: stateRemoved, path: file}
	case findingUnspecified:
		panic("BUG: findStub answered no finding without an error")
	}
	panic("BUG: findStub answered an unknown finding")
}

// keptSharedStub is the delete of one harness whose shared stub other found
// harnesses still read: Ze keeps it and names them.
func keptSharedStub(skills string, readers []*harness) outcome {
	file := filepath.Join(skills, stubName, stubFile)
	return outcome{state: stateKept, path: file, note: noteLines(
		"Ze kept the shared stub, because "+titlesOf(readers)+", found on this machine, still read it.",
		"To remove it for every harness found, run:",
		"ze delete agent skill")}
}

// foreignNote says that a SKILL.md Ze did not write was left, and how to
// replace it: remove it, then run create, the command line that writes the
// stub.
func foreignNote(file, create string) string {
	return noteLines(
		"A SKILL.md that Ze did not write is at this place, so Ze left it as it is.",
		"Ze replaces only a stub whose first line after its frontmatter is Ze's marker. To replace it, run:",
		"rm "+file+" && "+create)
}

// entryNote says that an entry Ze did not write, and that is not a link, was
// left where the stub belongs, and how to replace it: remove it, then run
// create, the command line that writes the stub.
func entryNote(entry, create string) string {
	return noteLines(
		entry+" is not a link and Ze did not write it, so Ze left it as it is.",
		"Ze replaces only a link or its own stub at this place. To replace it, run:",
		"rm "+entry+" && "+create)
}

// directoryNote says that a directory sits where the stub's file belongs.
func directoryNote(file, rerun string) string {
	return noteLines(
		"A directory is at "+file+", where the ze skill's file belongs, so Ze wrote no stub.",
		"Ze never removes a directory, which could hold files it did not write. Move it away, then run:",
		rerun)
}

// failedNote says that a file call failed, with the error as evidence.
func failedNote(action, path string, err error, rerun string) string {
	var problem textbuf.Buffer
	problem.Str("Ze could not ").Str(action).Byte(' ').Str(path).Str(": ").Err(err)
	return noteLines(problem.String(), "Correct what the error names, then run:", rerun)
}

// noteLines joins a note's lines. A note that is a refusal ends with the
// command that fixes it, alone on its line.
func noteLines(lines ...string) string {
	return textbuf.Join(lines, "\n")
}

// joinNotes answers two notes as one, either of which may be empty.
func joinNotes(first, second string) string {
	if first == "" {
		return second
	}
	return first + "\n" + second
}
