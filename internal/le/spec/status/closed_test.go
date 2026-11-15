// Related: closed.go -- LoadClosedSpecs, ReadClosedSpecs, the shared record
// Related: specset.go -- GroupSets, which lists each set's closed children
//
// The goal is the record of closed specs: which removal in git history is a
// closure, that the shared record is extended from its own commit and never
// rewalked, that a record HEAD cannot extend is recomputed, and that the
// read-only entry point says "not computed" rather than "none". Each test
// builds an isolated repository through fast-import, so nothing touches the
// developer checkout, and plants the record file by hand where the test needs
// a record git history would not produce.
package specstatus

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	closedFixes    = "plan/release/fixes/"
	closedFeatures = "plan/post-release/features/"
)

// closedFixtureGit runs git in root with no user or system configuration and
// a fixed identity, and answers its trimmed output.
func closedFixtureGit(t *testing.T, root, input string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
	command.Stdin = strings.NewReader(input)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.test",
		"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.test")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

// closedFixtureRepository answers an empty repository on branch main.
func closedFixtureRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	closedFixtureGit(t, root, "", "init", "-q", "--initial-branch=main")
	return root
}

// closedFixtureCommit commits exactly files as main's whole tree, on top of
// main, and answers the commit id. A path the previous commit held and files
// lacks is removed by this commit.
func closedFixtureCommit(t *testing.T, root string, files map[string]string) string {
	t.Helper()
	var stream strings.Builder
	stream.WriteString("commit refs/heads/main\ncommitter Fixture <fixture@example.test> 1700000000 +0000\ndata 7\nfixture\n")
	if tip := closedFixtureGit(t, root, "", "for-each-ref", "--format=%(objectname)", "refs/heads/main"); tip != "" {
		fmt.Fprintf(&stream, "from %s\n", tip)
	}
	stream.WriteString("deleteall\n")
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		body := files[path]
		fmt.Fprintf(&stream, "M 100644 inline %s\ndata %d\n%s\n", strconv.Quote(path), len(body), body)
	}
	stream.WriteString("\ndone\n")
	closedFixtureGit(t, root, stream.String(), "fast-import", "--quiet")
	return closedFixtureGit(t, root, "", "rev-parse", "refs/heads/main")
}

// plantClosedRecord writes the shared record as if it had been computed at
// commit, holding stems: a record no walk of the fixture history produces.
func plantClosedRecord(t *testing.T, root, commit string, stems ...string) {
	t.Helper()
	slices.Sort(stems)
	if err := writeClosedRecord(root, ClosedSpecs{commit: commit, stems: stems, known: true}); err != nil {
		t.Fatal(err)
	}
}

// loadClosed runs LoadClosedSpecs and fails on an error or an unknown record.
func loadClosed(t *testing.T, root string) ClosedSpecs {
	t.Helper()
	closed, err := LoadClosedSpecs(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !closed.Known() {
		t.Fatalf("LoadClosedSpecs answered an unknown record: %s", closed.Unknown())
	}
	return closed
}

// TestARemovalClosesASpecAndARenameOrABucketMoveDoesNot walks a history of
// one closure, one move between buckets and one pure stem rename.
//
// VALIDATES: the walk applies specpath.ClosedStems to each landed commit with
// that commit's own blobs, so only the removal is recorded.
// PREVENTS: the record counting a relocation or a rename as a closed child.
func TestARemovalClosesASpecAndARenameOrABucketMoveDoesNot(t *testing.T) {
	root := closedFixtureRepository(t)
	finished, moved, oldName := "# Spec: finished\n", "# Spec: moved\n", "# Spec: old-name\nSee spec-old-name.md.\n"
	closedFixtureCommit(t, root, map[string]string{
		closedFixes + "spec-finished.md": finished, closedFixes + "spec-moved.md": moved, closedFixes + "spec-old-name.md": oldName,
	})
	closedFixtureCommit(t, root, map[string]string{
		closedFixes + "spec-moved.md": moved, closedFixes + "spec-old-name.md": oldName,
	})
	closedFixtureCommit(t, root, map[string]string{
		closedFeatures + "spec-moved.md": moved, closedFixes + "spec-old-name.md": oldName,
	})
	head := closedFixtureCommit(t, root, map[string]string{
		closedFeatures + "spec-moved.md": moved, closedFixes + "spec-new-name.md": strings.ReplaceAll(oldName, "old-name", "new-name"),
	})

	closed := loadClosed(t, root)
	if want := []string{"finished"}; !reflect.DeepEqual(closed.Stems(), want) {
		t.Fatalf("closed stems = %v, want %v", closed.Stems(), want)
	}
	if closed.Commit() != head {
		t.Fatalf("record computed at %s, want HEAD %s", closed.Commit(), head)
	}
}

// TestTheRecordExtendsFromItsCommitWithoutRewalking plants a record at the
// second commit that names a stem history never closed and omits the one it
// did, then lands a third closure.
//
// VALIDATES: the walk covers only the cached commit to HEAD: the planted stem
// stays, the earlier closure is not found again, the new one is added, and the
// rewritten record names HEAD.
// PREVENTS: every reader paying a whole-history walk.
func TestTheRecordExtendsFromItsCommitWithoutRewalking(t *testing.T) {
	root := closedFixtureRepository(t)
	closedFixtureCommit(t, root, map[string]string{closedFixes + "spec-a.md": "a\n", closedFixes + "spec-b.md": "b\n"})
	second := closedFixtureCommit(t, root, map[string]string{closedFixes + "spec-b.md": "b\n"})
	plantClosedRecord(t, root, second, "planted")
	head := closedFixtureCommit(t, root, map[string]string{})

	closed := loadClosed(t, root)
	if want := []string{"b", "planted"}; !reflect.DeepEqual(closed.Stems(), want) {
		t.Fatalf("closed stems = %v, want %v: a rewalk would find a and drop planted", closed.Stems(), want)
	}
	saved := ReadClosedSpecs(root)
	if saved.Commit() != head {
		t.Fatalf("saved record at %s, want HEAD %s", saved.Commit(), head)
	}
	if !reflect.DeepEqual(saved.Stems(), closed.Stems()) {
		t.Fatalf("saved stems = %v, want %v", saved.Stems(), closed.Stems())
	}
}

// TestARecordHEADCannotExtendIsRecomputed plants a record at a commit that is
// no ancestor of HEAD (as a rebase leaves), at one git does not hold, and a
// record that is corrupt or cut short.
//
// VALIDATES: each is replaced by a walk of the whole history, without failing
// the caller, and the read-only reader calls a corrupt record not computed.
// PREVENTS: a rebased-away closure staying in the record, and a torn file
// read as a shorter list.
func TestARecordHEADCannotExtendIsRecomputed(t *testing.T) {
	plant := map[string]func(t *testing.T, root, first string){
		"no ancestor": func(t *testing.T, root, first string) {
			elsewhere := closedFixtureGit(t, root, "", "commit-tree", first+"^{tree}", "-m", "elsewhere")
			plantClosedRecord(t, root, elsewhere, "planted")
		},
		"a commit git does not hold": func(t *testing.T, root, _ string) {
			plantClosedRecord(t, root, strings.Repeat("f", 40), "planted")
		},
		"corrupt": func(t *testing.T, root, _ string) {
			writeRecordText(t, root, "garbage\n")
		},
		"cut short": func(t *testing.T, root, first string) {
			writeRecordText(t, root, closedRecordHeader+"\n"+closedRecordCommit+first+"\nstem planted\n")
		},
		// The file ends in the bytes "end\n" without its completion line.
		"cut short after a stem ending in end": func(t *testing.T, root, first string) {
			writeRecordText(t, root, closedRecordHeader+"\n"+closedRecordCommit+first+"\nstem bsd-pf-firewall-backend\n")
		},
	}
	for name, plantRecord := range plant {
		t.Run(name, func(t *testing.T) {
			root := closedFixtureRepository(t)
			first := closedFixtureCommit(t, root, map[string]string{closedFixes + "spec-a.md": "a\n", closedFixes + "spec-b.md": "b\n"})
			closedFixtureCommit(t, root, map[string]string{closedFixes + "spec-b.md": "b\n"})
			plantRecord(t, root, first)

			closed := loadClosed(t, root)
			if want := []string{"a"}; !reflect.DeepEqual(closed.Stems(), want) {
				t.Fatalf("closed stems = %v, want %v from a whole-history walk", closed.Stems(), want)
			}
		})
	}
}

// TestAShallowCloneIsNotComputed loads the record in a depth-1 clone of a
// history whose second commit closes a spec.
//
// VALIDATES: LoadClosedSpecs answers no error and a record that is not Known,
// whose reason opens "not computed", and writes no record file.
// PREVENTS: a walk that stops at the graft answering a Known record missing
// every closure before it, and that record being saved for other sessions.
func TestAShallowCloneIsNotComputed(t *testing.T) {
	origin := closedFixtureRepository(t)
	closedFixtureCommit(t, origin, map[string]string{closedFixes + "spec-a.md": "a\n", closedFixes + "spec-b.md": "b\n"})
	closedFixtureCommit(t, origin, map[string]string{closedFixes + "spec-b.md": "b\n"})
	closedFixtureCommit(t, origin, map[string]string{closedFixes + "spec-b.md": "b\n", closedFixes + "spec-c.md": "c\n"})
	parent := t.TempDir()
	closedFixtureGit(t, parent, "", "clone", "-q", "--depth", "1", "--branch", "main", "file://"+origin, "shallow")
	root := filepath.Join(parent, "shallow")
	if shallow := closedFixtureGit(t, root, "", "rev-parse", "--is-shallow-repository"); shallow != "true" {
		t.Fatalf("the fixture clone is not shallow (%q), so the test proves nothing", shallow)
	}

	closed, err := LoadClosedSpecs(t.Context(), root)
	if err != nil {
		t.Fatalf("LoadClosedSpecs failed over a shallow clone, want an answer: %v", err)
	}
	if closed.Known() {
		t.Fatalf("LoadClosedSpecs answered a known record %v over a shallow clone, want not computed", closed.Stems())
	}
	if !strings.HasPrefix(closed.Unknown(), "not computed") {
		t.Fatalf("reason %q, want it to open \"not computed\"", closed.Unknown())
	}
	if _, err := os.Stat(closedRecordPath(root)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("record file at %s after a shallow load: %v, want none written", closedRecordPath(root), err)
	}
}

// TestAMergeIsJudgedByItsFirstParentDiff removes a spec on a side branch and
// merges it into main, once where main also changed plan/ before the merge,
// and once in the usual shape, where the merge's plan/ tree is its side
// parent's and main changed only files outside plan/. A third shape removes a
// spec on the side branch and restores it before the merge, while main closes
// another.
//
// VALIDATES: the walk records the closure at the merge, whose diff against
// its first parent removes the spec, in the first two shapes; and it never
// visits a side commit: in the third, the side branch's removal is no
// closure, and only main's own is recorded.
// PREVENTS: a closure merged from a branch being missed, as a plain log over
// a merge, which prints no diff, would miss it; and a removal a branch undid
// before it landed being recorded as a closure, as a walk of every commit
// would record it.
func TestAMergeIsJudgedByItsFirstParentDiff(t *testing.T) {
	shapes := map[string]struct {
		build func(t *testing.T, root string)
		want  []string
	}{
		"main changed plan/ too": {func(t *testing.T, root string) {
			base := closedFixtureCommit(t, root, map[string]string{closedFixes + "spec-a.md": "a\n", closedFixes + "spec-b.md": "b\n"})
			side := closedFixtureImport(t, root, "refs/heads/side", base, "", map[string]string{closedFixes + "spec-b.md": "b\n"})
			main := closedFixtureCommit(t, root, map[string]string{
				closedFixes + "spec-a.md": "a\n", closedFixes + "spec-b.md": "b\n", closedFixes + "spec-c.md": "c\n",
			})
			closedFixtureImport(t, root, "refs/heads/main", main, side, map[string]string{
				closedFixes + "spec-b.md": "b\n", closedFixes + "spec-c.md": "c\n",
			})
		}, []string{"a"}},
		"the merge takes the side's plan/ tree": {func(t *testing.T, root string) {
			base := closedFixtureCommit(t, root, map[string]string{closedFixes + "spec-a.md": "a\n", closedFixes + "spec-b.md": "b\n"})
			side := closedFixtureImport(t, root, "refs/heads/side", base, "", map[string]string{closedFixes + "spec-b.md": "b\n"})
			main := closedFixtureCommit(t, root, map[string]string{
				closedFixes + "spec-a.md": "a\n", closedFixes + "spec-b.md": "b\n", "README.md": "readme\n",
			})
			closedFixtureImport(t, root, "refs/heads/main", main, side, map[string]string{
				closedFixes + "spec-b.md": "b\n", "README.md": "readme\n",
			})
		}, []string{"a"}},
		// The side branch removes a and restores it, adding d, so the merge's
		// plan/ tree differs from both parents and git cannot simplify the side
		// branch away: a walk of every commit would record a.
		"the side removed a spec and restored it": {func(t *testing.T, root string) {
			files := map[string]string{closedFixes + "spec-a.md": "a\n", closedFixes + "spec-b.md": "b\n", closedFixes + "spec-c.md": "c\n"}
			base := closedFixtureCommit(t, root, files)
			removed := closedFixtureImport(t, root, "refs/heads/side", base, "", map[string]string{
				closedFixes + "spec-b.md": "b\n", closedFixes + "spec-c.md": "c\n",
			})
			restored := closedFixtureImport(t, root, "refs/heads/side", removed, "", map[string]string{
				closedFixes + "spec-a.md": "a\n", closedFixes + "spec-b.md": "b\n", closedFixes + "spec-c.md": "c\n", closedFixes + "spec-d.md": "d\n",
			})
			main := closedFixtureCommit(t, root, map[string]string{closedFixes + "spec-a.md": "a\n", closedFixes + "spec-b.md": "b\n"})
			closedFixtureImport(t, root, "refs/heads/main", main, restored, map[string]string{
				closedFixes + "spec-a.md": "a\n", closedFixes + "spec-b.md": "b\n", closedFixes + "spec-d.md": "d\n",
			})
		}, []string{"c"}},
	}
	for name, shape := range shapes {
		t.Run(name, func(t *testing.T) {
			root := closedFixtureRepository(t)
			shape.build(t, root)
			if closed := loadClosed(t, root); !reflect.DeepEqual(closed.Stems(), shape.want) {
				t.Fatalf("closed stems = %v, want %v", closed.Stems(), shape.want)
			}
		})
	}
}

// TestAQuotedPathRefusesOnlyWhenItMayBeASpec removes a file whose name holds a
// tab, so git prints it quoted, and reads log records whose quoting cannot be
// unquoted.
//
// VALIDATES: a quoted path git wrote is unquoted and judged like any other
// path: one that is no spec path changes nothing and the closure beside it is
// recorded, and a spec whose name holds a tab is recorded closed when it is
// removed, and the saved record reads back with the same stems. A quoting
// that cannot be unquoted refuses when its text names the spec prefix and is
// skipped when it does not.
// PREVENTS: one odd file name under plan/ failing every later load, and a
// spec removal skipped because its name was quoted.
func TestAQuotedPathRefusesOnlyWhenItMayBeASpec(t *testing.T) {
	for name, history := range map[string]struct {
		odd  string
		want []string
	}{
		"not a spec": {closedFixes + "notes\tdraft.txt", []string{"a"}},
		"a spec":     {closedFixes + "spec-odd\tname.md", []string{"a", "odd\tname"}},
	} {
		t.Run(name, func(t *testing.T) {
			root := closedFixtureRepository(t)
			closedFixtureCommit(t, root, map[string]string{closedFixes + "spec-a.md": "a\n", history.odd: "odd\n"})
			closedFixtureCommit(t, root, map[string]string{})
			if closed := loadClosed(t, root); !reflect.DeepEqual(closed.Stems(), history.want) {
				t.Fatalf("closed stems = %q, want %q", closed.Stems(), history.want)
			}
			saved := ReadClosedSpecs(root)
			if !saved.Known() {
				t.Fatalf("the saved record does not read back: %s", saved.Unknown())
			}
			if !reflect.DeepEqual(saved.Stems(), history.want) {
				t.Fatalf("saved stems = %q, want %q", saved.Stems(), history.want)
			}
		})
	}

	// git never writes these: `\q` is no C escape, so strconv.Unquote fails.
	commit, parent := strings.Repeat("c", 40), strings.Repeat("p", 40)
	for name, unreadable := range map[string]struct {
		path    string
		refuses bool
	}{
		"may be a spec": {`"` + closedFixes + `spec-odd\qname.md"`, true},
		"no spec":       {`"` + closedFixes + `notes\qdraft.txt"`, false},
	} {
		t.Run("unreadable quoting, "+name, func(t *testing.T) {
			record := commit + "\x00" + parent + "\x00\n\nD\t" + closedFixes + "spec-a.md\nD\t" + unreadable.path + "\n"
			change, err := parseClosedCommit(record)
			if unreadable.refuses {
				if err == nil {
					t.Fatalf("parseClosedCommit answered %+v over %s, want a refusal", change, unreadable.path)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseClosedCommit refused %s, which cannot name a spec: %v", unreadable.path, err)
			}
			if want := []string{closedFixes + "spec-a.md"}; !reflect.DeepEqual(change.removed, want) {
				t.Fatalf("removed = %q, want %q: the unreadable path is skipped", change.removed, want)
			}
		})
	}
}

// TestAStemTheRecordCannotHoldIsNeverRecorded removes, beside spec-a.md, a
// spec whose name holds a line break, a spec named spec-.md, whose stem is
// empty, and a spec whose name holds a carriage return.
//
// VALIDATES: the first two are never recorded closed: LoadClosedSpecs answers
// only a, with no error, and the record it saved reads back Known with the
// same stems. A carriage return is no line break of the record, so that spec
// is recorded closed and reads back.
// PREVENTS: a stem written as a line the reader refuses, or as two lines,
// which makes every later read of the record corrupt and every load rewalk
// the whole history.
func TestAStemTheRecordCannotHoldIsNeverRecorded(t *testing.T) {
	for name, history := range map[string]struct {
		odd  string
		want []string
	}{
		"a line break":      {closedFixes + "spec-odd\nname.md", []string{"a"}},
		"an empty stem":     {closedFixes + "spec-.md", []string{"a"}},
		"a carriage return": {closedFixes + "spec-odd\rname.md", []string{"a", "odd\rname"}},
	} {
		t.Run(name, func(t *testing.T) {
			root := closedFixtureRepository(t)
			closedFixtureCommit(t, root, map[string]string{closedFixes + "spec-a.md": "a\n", history.odd: "odd\n"})
			closedFixtureCommit(t, root, map[string]string{})
			if closed := loadClosed(t, root); !reflect.DeepEqual(closed.Stems(), history.want) {
				t.Fatalf("closed stems = %q, want %q", closed.Stems(), history.want)
			}
			saved := ReadClosedSpecs(root)
			if !saved.Known() {
				t.Fatalf("the saved record does not read back: %s", saved.Unknown())
			}
			if !reflect.DeepEqual(saved.Stems(), history.want) {
				t.Fatalf("saved stems = %q, want %q", saved.Stems(), history.want)
			}
		})
	}
}

// TestARecordWithoutItsCompletionMarkIsCorrupt reads record texts cut or
// extended around the completion mark.
//
// VALIDATES: an empty file, a file cut mid-line, an `end` line followed by more
// bytes and an `end` with no final newline each answer "no completion mark"
// and a record that is not Known; the whole record beside them answers known.
// PREVENTS: a torn or appended record read as a shorter or longer list.
func TestARecordWithoutItsCompletionMarkIsCorrupt(t *testing.T) {
	whole := closedRecordHeader + "\n" + closedRecordCommit + strings.Repeat("a", 40) + "\nstem planted\n"
	for _, record := range []struct {
		name, text, problem string
	}{
		{"whole", whole + "end\n", ""},
		{"empty", "", "no completion mark"},
		{"cut mid-line", whole + "stem pla", "no completion mark"},
		{"end followed by a line", whole + "end\nstem late\n", "no completion mark"},
		{"end followed by bytes", whole + "end\nx", "no completion mark"},
		{"end followed by an empty line", whole + "end\n\n", "no completion mark"},
		{"end with no final newline", whole + "end", "no completion mark"},
	} {
		t.Run(record.name, func(t *testing.T) {
			closed, problem := parseClosedRecord([]byte(record.text))
			if problem != record.problem {
				t.Fatalf("problem = %q, want %q", problem, record.problem)
			}
			if known := record.problem == ""; closed.Known() != known {
				t.Fatalf("Known = %v, want %v", closed.Known(), known)
			}
		})
	}
}

// closedFixtureImport commits files as the whole tree of ref, with parent
// from and, when merge is not "", a second parent merge, and answers the
// commit id.
func closedFixtureImport(t *testing.T, root, ref, from, merge string, files map[string]string) string {
	t.Helper()
	var stream strings.Builder
	fmt.Fprintf(&stream, "commit %s\ncommitter Fixture <fixture@example.test> 1700000000 +0000\ndata 7\nfixture\nfrom %s\n", ref, from)
	if merge != "" {
		fmt.Fprintf(&stream, "merge %s\n", merge)
	}
	stream.WriteString("deleteall\n")
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		fmt.Fprintf(&stream, "M 100644 inline %s\ndata %d\n%s\n", strconv.Quote(path), len(files[path]), files[path])
	}
	stream.WriteString("\ndone\n")
	closedFixtureGit(t, root, stream.String(), "fast-import", "--quiet")
	return closedFixtureGit(t, root, "", "rev-parse", ref)
}

// writeRecordText writes the record file's bytes as given.
func writeRecordText(t *testing.T, root, text string) {
	t.Helper()
	path := closedRecordPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestTheReadOnlyRecordSaysNotComputed reads the record in a directory that
// is no git repository: absent, corrupt, then whole.
//
// VALIDATES: an absent or corrupt record answers not Known with a reason that
// opens "not computed", and a whole record answers its commit and stems with
// no git at all.
// PREVENTS: the session-start summary printing an empty list that reads as
// "no closed specs" when nothing was computed, or running git to find out.
func TestTheReadOnlyRecordSaysNotComputed(t *testing.T) {
	root := t.TempDir()
	// In this order: the absent case runs before any file is written.
	for _, state := range []struct{ name, text string }{
		{"absent", ""},
		{"corrupt", "ze-closed-specs 1\ncommit nope\nend\n"},
	} {
		if state.text != "" {
			writeRecordText(t, root, state.text)
		}
		read := ReadClosedSpecs(root)
		if read.Known() {
			t.Fatalf("%s record: Known, want not computed", state.name)
		}
		if !strings.HasPrefix(read.Unknown(), "not computed") {
			t.Fatalf("%s record: reason %q, want it to open \"not computed\"", state.name, read.Unknown())
		}
		if len(read.Stems()) != 0 {
			t.Fatalf("%s record: stems %v from a record not computed", state.name, read.Stems())
		}
	}
	commit := strings.Repeat("a", 40)
	plantClosedRecord(t, root, commit, "x-2-b", "x-1-a")
	read := ReadClosedSpecs(root)
	if !read.Known() {
		t.Fatalf("whole record: %s", read.Unknown())
	}
	if read.Commit() != commit {
		t.Fatalf("record commit = %q, want %q", read.Commit(), commit)
	}
	if want := []string{"x-1-a", "x-2-b"}; !reflect.DeepEqual(read.Stems(), want) {
		t.Fatalf("record stems = %v, want %v", read.Stems(), want)
	}
	if (ClosedSpecs{}).Unknown() != notComputed {
		t.Fatalf("the zero record's reason = %q, want %q", (ClosedSpecs{}).Unknown(), notComputed)
	}
}

// TestASetListsItsClosedChildren groups a population shaped like the
// agent-economy set on 2026-11-15: children 1, 2 and 6 closed, 3, 4 and 5
// open, the umbrella present; beside it the edge shapes of the rule.
//
// VALIDATES: closed children decorate a set the files form, sorted by number;
// an umbrella whose children all closed is still a set and lists them; the
// files alone decide membership, so one open child beside a closed sibling and
// no umbrella is no set, a number-less parent whose numbered children all
// closed is no set, and closed stems with no file left form none; a
// closed umbrella is no child; a closed stem written anew is the open file; a
// record not read answers not read and leaves the sets unchanged.
// PREVENTS: a second parser of set names, readers that disagree about a set
// because one read the record, and a started set reading as an idea because
// its closed children are invisible.
func TestASetListsItsClosedChildren(t *testing.T) {
	spec := func(name string) Spec {
		return Spec{Name: name, Path: fixturePath("spec-" + name + ".md"), Status: "ready"}
	}
	specs := []Spec{
		spec("agent-economy-0-umbrella"), spec("agent-economy-3-supervision-hooks"),
		spec("agent-economy-4-supervision"), spec("agent-economy-5-skills"),
		spec("finished-0-umbrella"), spec("half-1-open"), spec("lone-3-open"),
		spec("again-1-reborn"), spec("again-2-b"), spec("solo"),
	}
	stems := []string{
		"agent-economy-1-baseline", "agent-economy-2-grouping", "agent-economy-6-measure",
		"finished-0-umbrella-x", "finished-1-a", "finished-2-b", "half-2-closed",
		"gone-0-umbrella", "gone-1-a", "gone-2-b",
		"again-0-umbrella", "again-1-reborn", "unnumbered",
		"solo-1-a", "solo-2-b",
	}
	slices.Sort(stems)
	closed := ClosedSpecs{commit: strings.Repeat("a", 40), stems: stems, known: true}
	sets := GroupSets(specs, closed)
	unread := GroupSets(specs, ClosedSpecs{})

	economy := memberSet(t, sets, "spec-agent-economy-4-supervision.md")
	children, read := economy.ClosedChildren()
	if !read {
		t.Fatal("agent-economy: closed children not read over a known record")
	}
	wantClosed := []ClosedChild{{1, "agent-economy-1-baseline"}, {2, "agent-economy-2-grouping"}, {6, "agent-economy-6-measure"}}
	if !reflect.DeepEqual(children, wantClosed) {
		t.Errorf("agent-economy closed = %+v, want %+v", children, wantClosed)
	}
	var open []int
	for _, child := range economy.Open {
		open = append(open, child.Number)
	}
	if !reflect.DeepEqual(open, []int{3, 4, 5}) {
		t.Errorf("agent-economy open = %v, want 3, 4, 5", open)
	}
	if economy.Umbrella != fixturePath("spec-agent-economy-0-umbrella.md") {
		t.Errorf("agent-economy umbrella = %q", economy.Umbrella)
	}

	finished := memberSet(t, sets, "spec-finished-0-umbrella.md")
	if len(finished.Open) != 0 {
		t.Errorf("finished open = %+v, want none", finished.Open)
	}
	// finished-0-umbrella-x is a child numbered 0, not the umbrella.
	wantFinished := []ClosedChild{{0, "finished-0-umbrella-x"}, {1, "finished-1-a"}, {2, "finished-2-b"}}
	if children, _ := finished.ClosedChildren(); !reflect.DeepEqual(children, wantFinished) {
		t.Errorf("finished closed = %+v, want %+v", children, wantFinished)
	}
	// The again set's files form it, so a closed again-0-umbrella reaches the
	// closed loop with a set to join, and only the umbrella arm keeps it out.
	if again := memberSet(t, sets, "spec-again-2-b.md"); !reflect.DeepEqual(closedOf(t, again), []ClosedChild(nil)) {
		t.Errorf("again closed = %+v, want none: again-0-umbrella is a closed umbrella and again-1-reborn is open again",
			closedOf(t, again))
	}
	// spec-solo.md is a number-less parent whose numbered children all closed:
	// with no numbered file of its prefix left, it is no umbrella and no set.
	for _, name := range []string{"spec-half-1-open.md", "spec-lone-3-open.md", "spec-solo.md"} {
		if set := sets.Name(fixturePath(name)); set != NoSet {
			t.Errorf("%s is in set %q with the record read, want none: only files form a set", name, set)
		}
		if set := unread.Name(fixturePath(name)); set != NoSet {
			t.Errorf("%s is in set %q with no record read, want none", name, set)
		}
	}
	for _, set := range sets {
		if set.Prefix == "gone" {
			t.Errorf("set gone found with no file left: %+v", set)
		}
		if set.Prefix == "solo" {
			t.Errorf("set solo found with no numbered file left: %+v", set)
		}
	}

	if len(unread) != len(sets) {
		t.Fatalf("%d sets with no record read, %d with one: the record changed membership", len(unread), len(sets))
	}
	for index := range sets {
		if unread[index].Prefix != sets[index].Prefix {
			t.Errorf("set %d is %q with no record read and %q with one", index, unread[index].Prefix, sets[index].Prefix)
		}
		if unread[index].Umbrella != sets[index].Umbrella {
			t.Errorf("set %s: umbrella %q with no record read and %q with one",
				sets[index].Prefix, unread[index].Umbrella, sets[index].Umbrella)
		}
		if !reflect.DeepEqual(unread[index].Open, sets[index].Open) {
			t.Errorf("set %s: open children %+v with no record read and %+v with one",
				sets[index].Prefix, unread[index].Open, sets[index].Open)
		}
		if children, read := unread[index].ClosedChildren(); read {
			t.Errorf("set %s: closed children read (%+v) over a record not read", unread[index].Prefix, children)
		}
	}
}

// closedOf answers a set's closed children, failing the test when the set's
// record was not read.
func closedOf(t *testing.T, set SpecSet) []ClosedChild {
	t.Helper()
	children, read := set.ClosedChildren()
	if !read {
		t.Fatalf("set %s: closed children not read", set.Prefix)
	}
	return children
}
