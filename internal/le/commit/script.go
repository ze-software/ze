// Design: docs/features/ai-first.md -- atomic add/remove/commit script generation
//
// This file is the only native source that spells the raw staging and commit
// verbs. They are emitted into the generated script, never executed here.
package commit

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	scriptMarker = "# ze-commit-script:"
	blockMarker  = "# ze-commit-block:"
	pushMarker   = "# ze-commit-push:"
)

type commitBlock struct {
	Tag     string
	Subject string
	Paths   []string
	Removed []string
	// IndexEntries is `git ls-files -s` for Paths, read when the commit was
	// PREPARED. It is what the block commits, so the content is the content
	// Create's gates judged rather than whatever the working tree holds when
	// the script runs. snapshotIndexEntries writes it.
	IndexEntries []string
	MessagePath  string
	ReviewCheck  string
}

// indexInfoDelimiter closes the heredoc that feeds the snapshot to git. A
// `git ls-files -s` line starts with a six-digit mode, so no entry can spell it.
const indexInfoDelimiter = "ZE_INDEX_INFO"

func renderBlock(block commitBlock, scriptPath string) string {
	all := append(append([]string{}, block.Paths...), block.Removed...)
	lines := []string{
		commentLine("Commit " + block.Tag + ": " + block.Subject),
		blockMarker + " tag=" + block.Tag + " paths=" + quotePaths(all),
	}
	if block.ReviewCheck != "" {
		lines = append(lines, "# critical-review gate re-check", block.ReviewCheck)
	}
	lines = append(lines,
		renderPrivateIndex(block, scriptPath),
		`GIT_INDEX_FILE="$_ze_index" git commit -F `+shellQuote(block.MessagePath),
		renderSharedIndexRepair(block))
	return strings.Join(lines, "\n") + "\n"
}

// renderPrivateIndex emits the staging half of a block: an index of this
// block's own, seeded from HEAD when the script RUNS and filled with the blob
// each path held when `./le commit create` read it.
//
// The shared index is never written before the commit, which is what makes the
// commit's POPULATION exactly the paths the block names. A concurrent session's
// staged file is not in this index, so it cannot ride along, and no window
// exists between the staging and the commit for one to appear in. Seeding from
// HEAD at run time is what keeps a peer's commit made in the meantime: the tree
// this block writes is that HEAD plus its own paths.
//
// The commit's CONTENT is the snapshot rather than the working tree, so an edit
// that lands after preparation is left where it is, for whoever wrote it to
// commit under their own subject. Eight rows in
// `plan/journal/concurrent-session-corruption.md` are one session's unfinished
// sentence published under another session's message, and `git add` reading the
// working tree is how every one of them happened.
func renderPrivateIndex(block commitBlock, scriptPath string) string {
	lines := []string{
		`_ze_index="$PWD"/` + shellQuote(indexFileFor(scriptPath)),
		`rm -f "$_ze_index"`,
		`GIT_INDEX_FILE="$_ze_index" git read-tree HEAD`,
	}
	if len(block.IndexEntries) != 0 {
		lines = append(lines,
			`GIT_INDEX_FILE="$_ze_index" git update-index --index-info <<'`+indexInfoDelimiter+`'`)
		lines = append(lines, block.IndexEntries...)
		lines = append(lines, indexInfoDelimiter)
	}
	if len(block.Removed) != 0 {
		lines = append(lines,
			`GIT_INDEX_FILE="$_ze_index" git update-index --force-remove -- `+quotePaths(block.Removed))
	}
	if len(block.Paths) != 0 {
		lines = append(lines, renderDriftNote(block.Paths))
	}
	return strings.Join(lines, "\n")
}

// renderDriftNote emits the report that a named path no longer holds the
// content this commit carries.
//
// It is a NOTE and not a gate. The commit is already safe, because the snapshot
// is what lands, and refusing here would block an author whose only offense is
// that a peer touched a file they share. What it prevents is the one silence
// the snapshot introduces: an author who edits a named path after preparing the
// commit would otherwise never learn that their newest edit stayed behind. It
// is still in the working tree, so nothing is lost, and `./le commit create`
// run again carries it.
//
// The comparison stages the working tree into a THROWAWAY index and reads the
// symmetric difference of the two entry sets, which answers on content, on mode
// and on a file that has been deleted, for the named paths and nothing else.
//
// `git update-index --refresh` is the obvious tool and it is the wrong one,
// measured twice on 2026-09-06. It REWRITES the entry it reports, so refreshing
// the index in place replaced the snapshot blob with the drifted one and
// committed exactly what this design exists to leave behind. And it refreshes
// the WHOLE index rather than the pathspec, so on a copy it named every dirty
// tracked file in the checkout: the first commit through this route printed 190
// paths, nine of which were its own.
func renderDriftNote(paths []string) string {
	quoted := quotePaths(paths)
	lines := []string{
		`rm -f "$_ze_index.now"`,
		`GIT_INDEX_FILE="$_ze_index.now" git add -f -- ` + quoted + ` 2>/dev/null || true`,
		`_ze_drift=$({ GIT_INDEX_FILE="$_ze_index.now" git -c core.quotePath=false ls-files -s -- ` + quoted + `;` +
			` GIT_INDEX_FILE="$_ze_index" git -c core.quotePath=false ls-files -s -- ` + quoted + `;` +
			` } | sort | uniq -u | cut -f2- | sort -u)`,
		`rm -f "$_ze_index.now"`,
		`if [ -n "$_ze_drift" ]; then`,
		`  echo "NOTE: these paths changed on disk after this commit was prepared." >&2`,
		`  echo "The commit carries the prepared content; the difference stays in the working tree." >&2`,
		`  echo "$_ze_drift" >&2`,
		"fi",
	}
	return strings.Join(lines, "\n")
}

// renderSharedIndexRepair points the shared index at what was just committed,
// for this block's paths and no others.
//
// Without it the shared index still describes the previous HEAD for those
// paths, which `git status` and every other session's tooling read as a staged
// change of this session's. Nobody could clear that without `git restore
// --staged`, which no agent may run. `git commit` does this itself for a
// partial commit; a commit made from a private index has to do it here.
func renderSharedIndexRepair(block commitBlock) string {
	lines := make([]string, 0, 3)
	if len(block.Paths) != 0 {
		lines = append(lines,
			"# Point the shared index at what was committed. Nothing else in it is touched.",
			"git ls-tree HEAD -- "+quotePaths(block.Paths)+" | git update-index --index-info")
	}
	if len(block.Removed) != 0 {
		lines = append(lines, "git update-index --force-remove -- "+quotePaths(block.Removed))
	}
	lines = append(lines, `rm -f "$_ze_index"`)
	return strings.Join(lines, "\n")
}

// indexFileFor names the private index beside the script that uses it. Both
// carry the same random suffix, so no second script and no other session can
// take the name, and a reader who has the script path has the index path.
func indexFileFor(scriptPath string) string {
	return strings.TrimSuffix(scriptPath, ".sh") + ".index"
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func quotePaths(paths []string) string {
	quoted := make([]string, len(paths))
	for index, path := range paths {
		quoted[index] = shellQuote(path)
	}
	return strings.Join(quoted, " ")
}

func commentSafe(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func markerLine(marker, payload string) string {
	return marker + " " + commentSafe(payload)
}

func commentLine(value string) string {
	return markerLine("#", value)
}

func renderPush(authorisation string) string {
	return markerLine(pushMarker, authorisation) + "\n" +
		"# Push authorized by the owner. Reached only after every commit succeeded.\n" +
		"git push\n"
}

func splitPush(text string) (string, string, error) {
	lines := strings.SplitAfter(text, "\n")
	found := -1
	for index, line := range lines {
		if strings.HasPrefix(line, pushMarker) {
			if found >= 0 {
				return "", "", errors.New("refusing a script with more than one push marker")
			}
			found = index
		}
	}
	if found < 0 {
		return text, "", nil
	}
	authorisation := commentSafe(strings.TrimPrefix(strings.TrimSpace(lines[found]), pushMarker))
	tail := strings.Join(lines[found:], "")
	if strings.TrimSpace(tail) != strings.TrimSpace(renderPush(authorisation)) {
		return "", "", errors.New("refusing a script whose push marker is not its final section")
	}
	body := strings.TrimRight(strings.Join(lines[:found], ""), "\n")
	if body != "" {
		body += "\n"
	}
	return body, authorisation, nil
}

func pushAuthorisation(reason string) (string, error) {
	if reason == "" {
		return "", nil
	}
	reason = commentSafe(reason)
	if len(reason) < 12 {
		return "", fmt.Errorf("push authorisation is too short: %q is %d characters, 12 is the minimum", reason, len(reason))
	}
	return reason, nil
}

// validateTag reports whether a caller-supplied tag can name a message file.
// Create MUST call it before it records verification debt: nextTag runs after
// that record is written, so a tag refused there leaves rows naming a commit
// that was never made (plan/journal/record-written-before-the-operation-succeeds.md).
func validateTag(requested string) error {
	if requested == "" {
		return nil
	}
	// The length is reported on its own. tagPattern carries a {0,31} bound as
	// well as a character class, and a message naming only the class sends the
	// author looking for a bad character in a tag that has none.
	if len(requested) > tagMaxLength {
		return fmt.Errorf("tag is %d characters, %d over the %d limit: %s",
			len(requested), len(requested)-tagMaxLength, tagMaxLength, requested)
	}
	if !tagPattern.MatchString(requested) {
		return errors.New("tag must start with an alphanumeric character and contain only alnum, dot, underscore, or dash")
	}
	return nil
}

// nextTag chooses the tag a prepared commit is named by and allocates the
// message file that commit will be made from. It answers both.
//
// Every message path carries a random suffix, for the reason the SCRIPT path
// already carries one: a second prepared commit MUST NOT be able to write over
// the first one's message while the first script is still runnable. Keyed on
// session and tag alone, it could, and the failure was silent -- both calls
// report exit zero and a plausible file list, and the first script then commits
// the second's subject. That reached main as a one-character subject, and
// `plan/journal/pointer-shared-across-the-names-it-indexes.md` records four
// occurrences of it. The trigger is ordinary rather than careless: an author who
// loses the printed `script=` line re-runs this command to recover it.
//
// One prepared commit, one script, one message, and no name a later call can
// take.
func nextTag(root, session, requested string) (string, string, error) {
	if requested != "" {
		if err := validateTag(requested); err != nil {
			return "", "", err
		}
		relative, err := allocateMessage(root, session, requested)
		return requested, relative, err
	}
	// An auto tag is a per-session letter, and a letter is taken once this
	// session has allocated any message under it. The suffix is what keeps the
	// paths apart, so the letter only has to keep tmp/ readable.
	for code := byte('a'); code <= byte('z'); code++ {
		tag := string(code)
		used, err := filepath.Glob(filepath.Join(root, "tmp", "commit-msg-"+session+"-"+tag+"-*.txt"))
		if err != nil {
			return "", "", err
		}
		if len(used) > 0 {
			continue
		}
		relative, err := allocateMessage(root, session, tag)
		if err != nil {
			return "", "", err
		}
		return tag, relative, nil
	}
	return "", "", errors.New("no free message tag; clear old tmp/commit-msg-* files")
}

// allocateMessage creates an empty message file under a suffix no other prepared
// commit holds, and answers its path relative to the checkout root.
//
// O_EXCL is the allocation: the file exists from this moment, so a later call
// that draws the same suffix takes the next one instead. Create removes it again
// when it fails or when it was a dry run, so an unused reservation never holds a
// name.
func allocateMessage(root, session, tag string) (string, error) {
	for range 64 {
		var nonce [3]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return "", err
		}
		relative := filepath.ToSlash(filepath.Join("tmp",
			"commit-msg-"+session+"-"+tag+"-"+hex.EncodeToString(nonce[:])+".txt"))
		file, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(relative)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if err := file.Close(); err != nil {
			return "", err
		}
		return relative, nil
	}
	return "", errors.New("cannot allocate an unused commit message path")
}

func allocateScript(root, session, tag string) (string, error) {
	for range 64 {
		var nonce [3]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return "", err
		}
		relative := filepath.ToSlash(filepath.Join("tmp", "commit-"+session+"-"+tag+"-"+hex.EncodeToString(nonce[:])+".sh"))
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); errors.Is(err, os.ErrNotExist) {
			return relative, nil
		}
	}
	return "", errors.New("cannot allocate an unused commit script path")
}

func declaredPaths(text string) map[string]bool {
	paths := make(map[string]bool)
	for line := range strings.SplitSeq(text, "\n") {
		if !strings.HasPrefix(line, blockMarker) {
			continue
		}
		_, raw, found := strings.Cut(line, "paths=")
		if !found {
			continue
		}
		for _, path := range parseShellWords(raw) {
			paths[path] = true
		}
	}
	return paths
}

func parseShellWords(raw string) []string {
	words := make([]string, 0)
	for raw != "" {
		raw = strings.TrimLeft(raw, " \t")
		if raw == "" {
			break
		}
		if raw[0] != '\'' {
			word, rest, _ := strings.Cut(raw, " ")
			words = append(words, word)
			raw = rest
			continue
		}
		raw = raw[1:]
		var word textbuf.Buffer
		word.Reset()
		for raw != "" {
			at := strings.IndexByte(raw, '\'')
			if at < 0 {
				word.Str(raw)
				raw = ""
				break
			}
			word.Str(raw[:at])
			raw = raw[at+1:]
			if strings.HasPrefix(raw, `"'"'`) {
				word.Byte('\'')
				raw = raw[4:]
				continue
			}
			break
		}
		words = append(words, word.String())
	}
	return words
}
