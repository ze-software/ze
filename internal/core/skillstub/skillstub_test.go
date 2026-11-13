// VALIDATES: a stub is written as a regular file, never through a link at its
// directory or its file; a directory at the file path is refused with a typed
// error naming the path; ownership holds only for the caller's exact marker as
// the first line after the frontmatter; removal takes the owned file and an
// emptied directory and keeps a directory holding another file.
// METHOD: each case builds its own tree under t.TempDir and a marker of its own,
// then reads the file system back with Lstat and ReadFile.
package skillstub

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The caller-supplied marker every case uses, and the stub it owns.
const (
	marker      = "<!-- written by the test caller for demo -->"
	frontmatter = "---\nname: demo\ndescription: a demo skill\n---\n"
	stubBody    = frontmatter + marker + "\nread the real instructions\n"
	stubFile    = "SKILL.md"
	stubName    = "demo"
)

func placeIn(t *testing.T, directory string) Path {
	t.Helper()
	place, err := At(directory, stubName, stubFile)
	if err != nil {
		t.Fatalf("At(%s): %v", directory, err)
	}
	return place
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func mustBeRegular(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if !info.Mode().IsRegular() {
		t.Errorf("%s is %v, want a regular file", path, info.Mode())
	}
}

// Write into a directory that does not exist yet creates it and the stub's
// own directory, and leaves a regular file holding the body.
func TestWriteMakesARegularFile(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "skills")
	place := placeIn(t, directory)

	if err := place.Write([]byte(stubBody)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := filepath.Join(directory, stubName, stubFile)
	if place.File() != want {
		t.Errorf("File is %s, want %s", place.File(), want)
	}
	mustBeRegular(t, want)
	if got := readFile(t, want); got != stubBody {
		t.Errorf("the stub holds %q, want %q", got, stubBody)
	}
}

// A stub file that is a link is replaced by a regular file, and the link's
// target keeps its bytes.
func TestWriteReplacesALinkedStubFileAndLeavesTheTarget(t *testing.T) {
	root := t.TempDir()
	const outside = "the operator's own file\n"
	target := filepath.Join(root, "elsewhere.md")
	writeFile(t, target, outside)
	directory := filepath.Join(root, "skills")
	link := filepath.Join(directory, stubName, stubFile)
	if err := os.MkdirAll(filepath.Dir(link), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if err := placeIn(t, directory).Write([]byte(stubBody)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := readFile(t, target); got != outside {
		t.Errorf("Write changed the link's target to %q", got)
	}
	mustBeRegular(t, link)
	if got := readFile(t, link); got != stubBody {
		t.Errorf("the stub holds %q, want %q", got, stubBody)
	}
}

// An entry at the stub's directory that is a link to a directory, or a plain
// file, is replaced by a real directory, and a link's target keeps its bytes.
func TestWriteReplacesALinkedOrFileEntryAtTheStubDirectory(t *testing.T) {
	cases := []struct {
		kind  string
		place func(t *testing.T, root, entry string) (target string)
	}{
		{"a link to a directory", func(t *testing.T, root, entry string) string {
			t.Helper()
			target := filepath.Join(root, "elsewhere", stubName)
			writeFile(t, filepath.Join(target, stubFile), "outside\n")
			if err := os.Symlink(target, entry); err != nil {
				t.Fatalf("symlink: %v", err)
			}
			return filepath.Join(target, stubFile)
		}},
		{"a plain file", func(t *testing.T, _, entry string) string {
			t.Helper()
			writeFile(t, entry, "a file, not a directory\n")
			return ""
		}},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			root := t.TempDir()
			directory := filepath.Join(root, "skills")
			if err := os.MkdirAll(directory, 0o750); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			entry := filepath.Join(directory, stubName)
			target := tc.place(t, root, entry)

			if err := placeIn(t, directory).Write([]byte(stubBody)); err != nil {
				t.Fatalf("Write: %v", err)
			}
			info, err := os.Lstat(entry)
			if err != nil {
				t.Fatalf("lstat: %v", err)
			}
			if !info.IsDir() {
				t.Errorf("%s is %v, want a real directory", entry, info.Mode())
			}
			if got := readFile(t, filepath.Join(entry, stubFile)); got != stubBody {
				t.Errorf("the stub holds %q, want %q", got, stubBody)
			}
			if target != "" {
				if got := readFile(t, target); got != "outside\n" {
					t.Errorf("Write wrote through the link into %s: %q", target, got)
				}
			}
		})
	}
}

// A directory at the stub's file path is refused with a typed error naming
// that path, and the directory and what it holds are left.
func TestWriteRefusesADirectoryAtTheFilePath(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "skills")
	inside := filepath.Join(directory, stubName, stubFile, "kept.md")
	writeFile(t, inside, "kept\n")

	err := placeIn(t, directory).Write([]byte(stubBody))
	var inTheWay *DirectoryAtFileError
	if !errors.As(err, &inTheWay) {
		t.Fatalf("Write answered %v, want a *DirectoryAtFileError", err)
	}
	if want := filepath.Join(directory, stubName, stubFile); inTheWay.Path != want {
		t.Errorf("the error names %s, want %s", inTheWay.Path, want)
	}
	if got := readFile(t, inside); got != "kept\n" {
		t.Errorf("the refused write changed %s to %q", inside, got)
	}
}

// At refuses a name or a file that is not one directory entry.
func TestAtRefusesANameThatIsNotOneEntry(t *testing.T) {
	for _, bad := range []string{"", ".", "..", "a/b", "../up"} {
		if _, err := At(t.TempDir(), bad, stubFile); !errors.Is(err, ErrNotAName) {
			t.Errorf("At with name %q answered %v, want ErrNotAName", bad, err)
		}
		if _, err := At(t.TempDir(), stubName, bad); !errors.Is(err, ErrNotAName) {
			t.Errorf("At with file %q answered %v, want ErrNotAName", bad, err)
		}
	}
}

// Ownership holds only for the exact marker as the first line after the
// frontmatter, in a regular file.
func TestOwnedByHoldsOnlyForTheExactMarkerFirstAfterTheFrontmatter(t *testing.T) {
	cases := []struct {
		kind string
		body string
		want bool
	}{
		{"the marker first after the frontmatter", stubBody, true},
		{"the marker further down", frontmatter + "my notes\n" + marker + "\n", false},
		{"an edited marker", frontmatter + "<!-- written by the test caller for demo, edited -->\n", false},
		{"another name's marker", frontmatter + "<!-- written by the test caller for other -->\n", false},
		{"no frontmatter", marker + "\nbody\n", false},
		{"an unclosed frontmatter", "---\nname: demo\n" + marker + "\n", false},
		{"CRLF line ends", "---\r\nname: demo\r\n---\r\n" + marker + "\r\nbody\r\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			directory := t.TempDir()
			place := placeIn(t, directory)
			writeFile(t, place.File(), tc.body)

			owned, err := place.OwnedBy(marker)
			if err != nil {
				t.Fatalf("OwnedBy: %v", err)
			}
			if owned != tc.want {
				t.Errorf("OwnedBy is %v, want %v", owned, tc.want)
			}
		})
	}
}

// A link to a file that carries the marker is not owned: only a regular file
// is.
func TestOwnedByRefusesALink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "elsewhere.md")
	writeFile(t, target, stubBody)
	place := placeIn(t, filepath.Join(root, "skills"))
	if err := os.MkdirAll(filepath.Dir(place.File()), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(target, place.File()); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	owned, err := place.OwnedBy(marker)
	if err != nil {
		t.Fatalf("OwnedBy: %v", err)
	}
	if owned {
		t.Error("a link to a marked file is owned, want not owned")
	}
}

// An absent file is an error, never "not owned".
func TestOwnedByAnswersAnErrorForAnAbsentFile(t *testing.T) {
	_, err := placeIn(t, t.TempDir()).OwnedBy(marker)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("OwnedBy of an absent file answered %v, want os.ErrNotExist", err)
	}
}

// Removal takes the owned file and the directory it leaves empty.
func TestRemoveOwnedByRemovesTheFileAndAnEmptiedDirectory(t *testing.T) {
	directory := t.TempDir()
	place := placeIn(t, directory)
	writeFile(t, place.File(), stubBody)

	removed, err := place.RemoveOwnedBy(marker)
	if err != nil {
		t.Fatalf("RemoveOwnedBy: %v", err)
	}
	if !removed {
		t.Error("RemoveOwnedBy answered not removed for an owned stub")
	}
	if _, err := os.Lstat(filepath.Join(directory, stubName)); !os.IsNotExist(err) {
		t.Errorf("the emptied directory survived: %v", err)
	}
}

// Removal keeps a directory that still holds another file.
func TestRemoveOwnedByKeepsADirectoryHoldingAnotherFile(t *testing.T) {
	directory := t.TempDir()
	place := placeIn(t, directory)
	writeFile(t, place.File(), stubBody)
	sibling := filepath.Join(directory, stubName, "notes.md")
	writeFile(t, sibling, "notes\n")

	removed, err := place.RemoveOwnedBy(marker)
	if err != nil {
		t.Fatalf("RemoveOwnedBy: %v", err)
	}
	if !removed {
		t.Error("RemoveOwnedBy answered not removed for an owned stub")
	}
	if _, err := os.Lstat(place.File()); !os.IsNotExist(err) {
		t.Errorf("the stub survived: %v", err)
	}
	if got := readFile(t, sibling); got != "notes\n" {
		t.Errorf("the sibling changed to %q", got)
	}
}

// A file the marker does not own is left byte for byte.
func TestRemoveOwnedByLeavesAFileItDoesNotOwn(t *testing.T) {
	place := placeIn(t, t.TempDir())
	const foreign = "---\nname: demo\ndescription: mine\n---\nmy own skill\n"
	writeFile(t, place.File(), foreign)

	removed, err := place.RemoveOwnedBy(marker)
	if err != nil {
		t.Fatalf("RemoveOwnedBy: %v", err)
	}
	if removed {
		t.Error("RemoveOwnedBy removed a file the marker does not own")
	}
	if got := readFile(t, place.File()); got != foreign {
		t.Errorf("the foreign file changed to %q", got)
	}
}

// Frontmatter splits the block, closing fence and its newline included, from
// the rest, and finds no block where none opens or none closes.
func TestFrontmatterSplitsTheBlockFromTheRest(t *testing.T) {
	cases := []struct {
		kind   string
		source string
		block  string
		rest   string
		found  bool
	}{
		{"a closed block", frontmatter + "body\n", frontmatter, "body\n", true},
		{"a block with nothing after it", "---\nname: x\n---", "---\nname: x\n---", "", true},
		{"CRLF fences", "---\r\nname: x\r\n---\r\nbody", "---\r\nname: x\r\n---\r\n", "body", true},
		{"no opening fence", "name: x\n---\n", "", "", false},
		{"an unclosed block", "---\nname: x\n", "", "", false},
		{"one line", "---", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			block, rest, found := Frontmatter([]byte(tc.source))
			if found != tc.found {
				t.Fatalf("found is %v, want %v", found, tc.found)
			}
			if string(block) != tc.block {
				t.Errorf("block is %q, want %q", block, tc.block)
			}
			if string(rest) != tc.rest {
				t.Errorf("rest is %q, want %q", rest, tc.rest)
			}
		})
	}
}
