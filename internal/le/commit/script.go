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
	"strconv"
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
	// PreparedHead is the commit HEAD named when `./le commit create` prepared
	// this block, read before its snapshot. The script compares it with the
	// parent it builds on, so a peer's change to a named path that landed before
	// the run is refused rather than written over. Each block of an appended
	// script carries its own, because each was prepared at its own moment.
	PreparedHead string
	// IndexEntries is `git ls-files -s` for Paths, read when the commit was
	// PREPARED. It is what the block commits, so the content is the content
	// Create's gates judged rather than whatever the working tree holds when
	// the script runs. snapshotIndexEntries writes it.
	IndexEntries []string
	MessagePath  string
	ReviewCheck  string
	// ApprovalsPath is the session's RFC approval file and ApprovalsDropped
	// the exact lines the block removes from it once its commit succeeds:
	// the rows the commit carries as trailers, and any row a landed commit
	// already carried. renderApprovalPrune writes it.
	ApprovalsPath    string
	ApprovalsDropped []string
}

// indexLockWaitSecondsMax bounds how long a block's shared-index repair waits
// for a `.git/index.lock` another process holds. A peer's `git status` or
// `git add` holds the lock for well under a second; a lock still held after
// this long is a stuck holder or an orphan, which waiting cannot clear.
const indexLockWaitSecondsMax = 30

// indexLockWaitSeconds is the bound renderSharedIndexRepair writes into the
// script. It is a variable only so a test can shorten it.
var indexLockWaitSeconds = indexLockWaitSecondsMax

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
	lines = append(lines, renderPublish(block, scriptPath))
	if len(block.Paths) != 0 {
		lines = append(lines, renderDriftNote(block.Paths))
	}
	// The message is deleted once git holds it. A message file is what holds
	// its tag (nextTag), so a landed commit that kept it would never free the
	// letter, and a session would run out after 26 commits. `set -e` keeps a
	// failed commit's message, whose script is still runnable.
	lines = append(lines, `rm -f "$_ze_message"`)
	if len(block.Removed) != 0 {
		lines = append(lines, renderWorkingTreeRemoval(block.Removed))
	}
	if len(block.ApprovalsDropped) != 0 {
		lines = append(lines, renderApprovalPrune(block))
	}
	// The shared-index repair is the block's last step: it is the one step that
	// waits on another process, and when it gives up, everything else this
	// block owes is already done.
	lines = append(lines, renderSharedIndexRepair(block, scriptPath))
	return strings.Join(lines, "\n") + "\n"
}

// renderApprovalPrune emits the step that drops the used RFC approval rows
// from the session file. It sits after the publish under `set -e`, so it
// runs only once the branch holds the trailers: a row removed before that would be
// an approval lost with nothing to show for it. `grep -v` answers 1 when no
// line is left to print, which is not a failure here.
func renderApprovalPrune(block commitBlock) string {
	var page textbuf.Buffer
	page.Str("# RFC approvals this commit carries as trailers, dropped now that git holds them.\n")
	page.Str("_ze_approved=").Str(shellQuote(block.ApprovalsPath)).Byte('\n')
	page.Str(`if [ -f "$_ze_approved" ]; then`).Byte('\n')
	page.Str("  grep -v -x -F")
	for _, line := range block.ApprovalsDropped {
		page.Str(" -e ").Str(shellQuote(line))
	}
	page.Str(` -- "$_ze_approved" > "$_ze_approved.pruned" || [ "$?" -eq 1 ]`).Byte('\n')
	page.Str(`  mv -- "$_ze_approved.pruned" "$_ze_approved"`).Byte('\n')
	page.Str("fi")
	return page.String()
}

// publishAttemptsMax bounds how many times a block builds its commit again on
// a HEAD that moved while it was being built. One attempt is a handful of git
// calls; a HEAD that a peer moves this many times in a row inside that span is
// a storm that another attempt would not outlast.
const publishAttemptsMax = 5

// renderPublish emits the commit half of a block: it reads the parent ONCE,
// builds the tree and the commit on that parent, checks the commit changes
// only the paths the block names, and moves the branch with a compare-and-swap
// against that same parent.
//
// `git commit` cannot be used for this. It reads HEAD again for the parent, so
// a peer's commit that lands between the seeding of the private index and the
// commit becomes the parent of a tree that never held the peer's work, and
// every path this block does not name is silently reverted. 450f9f1ee0 and
// d621be6677 did that to the commit before each of them on 2026-10-14
// (plan/journal/concurrent-session-corruption.md).
//
// Every attempt, the first included, refuses when a peer's commit changed a
// path this block names since the HEAD the attempt compares with: building on
// it would write content prepared before that change over it. The first
// attempt compares with the HEAD `create` read (PreparedHead), each later one
// with the parent the attempt before it read. The commits this script
// published itself are left out, collected in _ze_own, because an appended
// block may name a path an earlier block of the same script committed.
//
// When the swap is refused and HEAD moved, a peer landed a commit, and the
// block builds again on the new HEAD, up to publishAttemptsMax times. When the
// swap is refused and HEAD did not move, git refused for another reason (a
// stale ref lock, a permission) that another attempt would only repeat, so the
// script stops with git's own words. git already waits
// core.filesRefLockTimeout (100ms by default) for a lock a live peer holds.
//
// `git commit-tree` ignores commit.gpgSign (measured with git 2.56 on
// 2026-10-15), so the script reads that setting and passes `-S` itself, which
// signs with user.signingKey exactly as `git commit` does. An unset setting
// means unsigned, and a value git cannot read as a boolean is refused, both as
// `git commit` does. The message goes through `git stripspace`, the cleanup
// `git commit -F` applies, and an empty result is refused as `git commit`
// refuses it. No git hook runs, and none is installed in this checkout.
func renderPublish(block commitBlock, scriptPath string) string {
	// An empty PreparedHead would make the first comparison `..<parent>`, which
	// git reads as HEAD..<parent>: empty, so the check would pass in silence.
	if block.PreparedHead == "" {
		panic("BUG: commit block " + block.Tag + " rendered without the HEAD it was prepared against")
	}
	all := append(append([]string{}, block.Paths...), block.Removed...)
	quoted := quotePaths(all)
	notPublished := notPublishedLine(block)
	// The capacity is a sizing hint for the two appends below, which write
	// fewer entries than this; append grows the slice if an edit adds more.
	lines := make([]string, 0, 64)
	lines = append(lines,
		`_ze_index="$PWD"/`+shellQuote(indexFileFor(scriptPath)),
		`_ze_message=`+shellQuote(block.MessagePath),
		`_ze_text=$(git stripspace < "$_ze_message")`,
		renderRefusal(`-z "$_ze_text"`, notPublished,
			`echo "Its message file $_ze_message holds no message once blank lines are stripped, and git commit refuses an empty message too."`,
			`echo "Write the message into that file, then run this script again."`),
		// git config answers 1 for an unset key and 128 for a value it cannot
		// read as a boolean, where `git commit` stops rather than guess.
		`_ze_status=0`,
		`_ze_gpgsign=$(git config --type=bool --get commit.gpgsign) || _ze_status=$?`,
		renderRefusal(`"$_ze_status" -gt 1`, notPublished,
			`echo "git config could not read commit.gpgsign as a boolean (its error is above), so whether to sign is unknown; git commit refuses here too."`,
			`echo "Set it with: git config commit.gpgsign true (or false), then run this script again."`),
		`_ze_sign=`,
		`if [ "$_ze_gpgsign" = true ]; then _ze_sign=1; fi`,
		`_ze_parent=`+shellQuote(block.PreparedHead),
		`_ze_attempt=1`,
		`while :; do`,
		`_ze_seen=$_ze_parent`,
		`_ze_parent=$(git rev-parse --verify 'HEAD^{commit}')`,
		`_ze_moved=$(git --literal-pathspecs rev-list "$_ze_seen..$_ze_parent" -- `+quoted+`)`,
		// With _ze_own empty, grep reads one empty pattern, which under -x
		// matches only an empty line, and rev-list never prints one.
		`_ze_peer=$(printf '%s\n' "$_ze_moved" | grep -v -x -F -f <(printf '%s\n' ${_ze_own:-}) || [ "$?" -eq 1 ])`,
		`_ze_names=`,
		`if [ -n "$_ze_peer" ]; then`,
		`  _ze_names=$(printf '%s\n' "$_ze_peer" | git --literal-pathspecs -c core.quotePath=false diff-tree --stdin -r --no-renames --name-only --no-commit-id -- `+quoted+` | sort -u)`,
		`fi`,
		renderRefusal(`-n "$_ze_peer"`, notPublished,
			`echo "A commit that landed after it was prepared changed paths it carries:"`,
			`printf '%s\n' "$_ze_names"`,
			`echo "Building on that commit would write content prepared before the change over it."`,
			`echo "Read the change with: git log -p $_ze_seen..$_ze_parent -- <path>"`,
			`echo "then fold it into your copy and prepare the commit again with ./le commit create."`),
		`rm -f "$_ze_index"`,
		`GIT_INDEX_FILE="$_ze_index" git read-tree "$_ze_parent"`,
		renderPrivateIndex(block),
	)
	var strayFilter textbuf.Buffer
	strayFilter.Str("grep -v -x -F")
	for _, path := range all {
		strayFilter.Str(" -e ").Str(shellQuote(path))
	}
	lines = append(lines,
		`_ze_tree=$(GIT_INDEX_FILE="$_ze_index" git write-tree)`,
		`_ze_status=0`,
		`_ze_commit=$(printf '%s\n' "$_ze_text" | git commit-tree ${_ze_sign:+-S} -p "$_ze_parent" "$_ze_tree") || _ze_status=$?`,
		renderRefusal(`"$_ze_status" -ne 0`, notPublished,
			`echo "git commit-tree could not build the commit (its error is above). When commit.gpgsign is true, signing is the usual cause:"`,
			`echo "make gpg, or gpg.program, sign with user.signingKey, then run this script again."`),
		`_ze_changed=$(git diff-tree -r -z --no-renames --name-only "$_ze_parent" "$_ze_commit" | tr '\0' '\n')`,
		renderRefusal(`-z "$_ze_changed"`, notPublished,
			`echo "HEAD already holds the prepared content of every path it names, so it has nothing to commit."`,
			`echo "Drop this block, or prepare the commit again from your working tree with ./le commit create."`),
		// The names are read NUL-terminated because git C-quotes a path holding
		// a double quote or a backslash, and a quoted name would never match the
		// name the block declares.
		// The guard that makes the parent read once a property of the result
		// rather than of the code above it: whatever went wrong while the tree
		// was built, a commit that changes a path this block does not name
		// reverts or adds work nobody asked it to carry, and it never lands.
		`_ze_stray=$(printf '%s\n' "$_ze_changed" | `+strayFilter.String()+` || [ "$?" -eq 1 ])`,
		renderRefusal(`-n "$_ze_stray"`, notPublished,
			`echo "Its tree changes paths the commit does not name, against its parent $_ze_parent:"`,
			`printf '%s\n' "$_ze_stray"`,
			`echo "Publishing it would revert or add work this commit does not own. This is a defect in the"`,
			`echo "generated script: report it with this script's path, then prepare the commit again with ./le commit create."`),
		// HEAD is a symbolic ref to the branch, so update-ref moves the branch,
		// and only if it still names the parent this commit was built on.
		`if _ze_swap=$(git update-ref -m `+shellQuote("commit: "+commentSafe(block.Subject))+` HEAD "$_ze_commit" "$_ze_parent" 2>&1); then`,
		`  _ze_own="${_ze_own:-} $_ze_commit"`,
		`  break`,
		`fi`,
		`_ze_now=$(git rev-parse --verify 'HEAD^{commit}')`,
		renderRefusal(`"$_ze_now" = "$_ze_parent"`, notPublished,
			`echo "git update-ref refused to move HEAD, which still names the parent the commit was built on:"`,
			`printf '%s\n' "$_ze_swap"`,
			`echo "Fix what git names (a lock file a dead git process left, or a permission), then run this script again."`),
		renderRefusal(`"$_ze_attempt" -ge `+strconv.Itoa(publishAttemptsMax), notPublished,
			`echo "HEAD moved under it `+strconv.Itoa(publishAttemptsMax)+` times in a row while other commits landed."`,
			`echo "Run this script again once the other sessions' commits have landed."`),
		`_ze_attempt=$((_ze_attempt + 1))`,
		`echo "NOTE: HEAD moved while commit `+block.Tag+` was being published; building it again on the new HEAD." >&2`,
		`done`,
		`git -c core.quotePath=false show --stat --format='[%h] %s' "$_ze_commit"`)
	return strings.Join(lines, "\n")
}

// notPublishedLine is the first line of every refusal before the branch moves.
func notPublishedLine(block commitBlock) string {
	return "ERROR: commit " + block.Tag + " (" + commentSafe(block.Subject) + ") was not published, and HEAD is unchanged by it."
}

// renderRefusal emits one of renderPublish's refusals: when the test holds,
// say that the commit was not published, run each statement with its output
// on stderr, and stop the script. Every refusal comes before the branch moves,
// so each one leaves HEAD as the peers left it and the message file in place.
func renderRefusal(test, notPublished string, said ...string) string {
	lines := make([]string, 0, len(said)+4)
	lines = append(lines, "if [ "+test+" ]; then", "  echo "+shellQuote(notPublished)+" >&2")
	for _, statement := range said {
		lines = append(lines, "  "+statement+" >&2")
	}
	lines = append(lines, "  exit 1", "fi")
	return strings.Join(lines, "\n")
}

// renderPrivateIndex emits the staging half of a block, inside the publish
// loop: the index of this block's own, already seeded from the parent the loop
// read, filled with the blob each path held when `./le commit create` read it.
//
// The shared index is never written before the commit, which is what makes the
// commit's POPULATION exactly the paths the block names. A concurrent session's
// staged file is not in this index, so it cannot ride along. Seeding from the
// parent the publish swaps against is what keeps a peer's commit: the tree this
// block writes is that parent plus its own paths, and renderPublish refuses to
// move the branch unless the branch still names that parent.
//
// The commit's CONTENT is the snapshot rather than the working tree, so an edit
// that lands after preparation is left where it is, for whoever wrote it to
// commit under their own subject. Eight rows in
// `plan/journal/concurrent-session-corruption.md` are one session's unfinished
// sentence published under another session's message, and `git add` reading the
// working tree is how every one of them happened.
func renderPrivateIndex(block commitBlock) string {
	lines := make([]string, 0, len(block.IndexEntries)+4)
	if len(block.IndexEntries) != 0 {
		lines = append(lines,
			`GIT_INDEX_FILE="$_ze_index" git update-index --index-info <<'`+indexInfoDelimiter+`'`)
		lines = append(lines, block.IndexEntries...)
		lines = append(lines, indexInfoDelimiter)
		lines = append(lines, renderLostBlobs(block)...)
	}
	if len(block.Removed) != 0 {
		// The entries the parent holds for the removed paths, captured while the
		// private index still carries them: renderWorkingTreeRemoval compares
		// the working-tree copies against exactly these lines.
		lines = append(lines,
			`_ze_removed=$(GIT_INDEX_FILE="$_ze_index" git --literal-pathspecs -c core.quotePath=false ls-files -s -- `+quotePaths(block.Removed)+`)`,
			`GIT_INDEX_FILE="$_ze_index" git update-index --force-remove -- `+quotePaths(block.Removed))
	}
	return strings.Join(lines, "\n")
}

// renderLostBlobs emits the refusal for a snapshot blob that is gone from the
// object store, naming every such path.
//
// The blobs are unreferenced until the commit lands, so `git gc --prune=now`
// between preparation and the run collects them. `update-index --index-info`
// accepts an entry whose object is missing, and `git write-tree` then stops at
// the first one it meets in tree order, which is often a path the author did
// not write, such as the verification-debt shard under plan/.
func renderLostBlobs(block commitBlock) []string {
	lines := make([]string, 0, len(block.IndexEntries)+3)
	lines = append(lines, `_ze_lost=$(git cat-file --batch-check='%(objectname)' <<'`+indexInfoDelimiter+`' | sed -n 's/ missing$//p'`)
	for _, entry := range block.IndexEntries {
		fields := strings.Fields(entry)
		if len(fields) > 1 {
			lines = append(lines, fields[1])
		}
	}
	lines = append(lines, indexInfoDelimiter, `)`,
		renderRefusal(`-n "$_ze_lost"`,
			notPublishedLine(block),
			`echo "The content prepared for these paths is gone from the object store, collected by a git gc:"`,
			`GIT_INDEX_FILE="$_ze_index" git --literal-pathspecs -c core.quotePath=false ls-files -s -- `+quotePaths(block.Paths)+
				` | grep -F "$_ze_lost" | while read -r _ze_mode _ze_object _ze_stage _ze_path; do echo "invalid object $_ze_mode $_ze_object for '$_ze_path'"; done`,
			`echo "The working tree still holds each file. Prepare the commit again with ./le commit create."`))
	return lines
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
//
// The entries come from HEAD as it stands when the step runs, not from the
// private index: a peer's commit can land while this step waits, and the
// shared index then has to describe that commit for a path both of them
// touched, or `git status` reads the peer's change as staged in reverse.
//
// The shared index is the one file this block writes that other processes also
// lock. A peer holding `.git/index.lock` made the repair refuse at once three
// times in two days (plan/journal/concurrent-session-corruption.md, 2026-10-04
// and 2026-10-05), each time after the commit had landed. So the step retries
// while git names the lock, once a second, up to indexLockWaitSeconds. Any
// other failure, or a lock still held at the bound, stops the script with the
// commit landed, names the paths the shared index still holds stale, and
// prints the repair to run once the lock is released, private index removal
// included.
func renderSharedIndexRepair(block commitBlock, scriptPath string) string {
	steps := make([]string, 0, 2)
	if len(block.Paths) != 0 {
		steps = append(steps, "git ls-tree HEAD -- "+quotePaths(block.Paths)+" | git update-index --index-info")
	}
	if len(block.Removed) != 0 {
		steps = append(steps, "git update-index --force-remove -- "+quotePaths(block.Removed))
	}
	cleanup := `rm -f ` + shellQuote(indexFileFor(scriptPath))
	if len(steps) == 0 {
		return cleanup
	}
	stale := append(append([]string{}, block.Paths...), block.Removed...)
	lines := []string{
		"# Point the shared index at what was committed. Nothing else in it is touched.",
		`_ze_waited=0`,
		`until _ze_said=$({ ` + strings.Join(steps, " && ") + `; } 2>&1); do`,
		`  case "$_ze_said" in`,
		`    *index.lock*)`,
		`      if [ "$_ze_waited" -lt ` + strconv.Itoa(indexLockWaitSeconds) + ` ]; then`,
		`        sleep 1`,
		`        _ze_waited=$((_ze_waited + 1))`,
		`        continue`,
		`      fi`,
		`      ;;`,
		`  esac`,
		// The commit is named by $_ze_commit, the one this block published,
		// never by HEAD: a peer's commit can have landed while this step waited.
		`  echo "ERROR: commit $_ze_commit landed, but the shared index could not be pointed at it." >&2`,
		`  echo "git said: $_ze_said" >&2`,
		`  echo "These paths read as staged changes in every session until the repair runs:" >&2`,
		`  printf '%s\n' ` + quotePaths(stale) + ` >&2`,
		`  echo "Any later commit in this script did not run." >&2`,
		`  printf 'Run this from %s once .git/index.lock is released:\n' "$PWD" >&2`,
	}
	for _, step := range append(steps, cleanup) {
		lines = append(lines, `  echo `+shellQuote(step)+` >&2`)
	}
	lines = append(lines, "  exit 1", "done", cleanup)
	return strings.Join(lines, "\n")
}

// renderWorkingTreeRemoval emits the deletion of each removed path's
// working-tree copy, for the copies git provably holds and no others.
//
// It runs AFTER the publish under `set -e`, so it is reached only once the
// commit is on the branch, and the content it deletes is then one `git show` away. A
// deletion before the commit would remove content that only a blob hash in
// this script names.
//
// The guard is an intersection, never the absence of a difference: a copy is
// deleted only when the entry git stages for it now, mode included, is one of
// the entries renderPrivateIndex captured from HEAD before the removal. A
// missing file, an unreadable file, a file git cannot stage, and a path HEAD
// never held each produce no matching entry, so each copy is left where it is.
// A directory is kept before it is staged: staging one answers an entry per
// file beneath it, and `grep -F` reads a multi-line pattern as one pattern per
// line, so one matching file would prove the whole directory. A submodule's
// gitlink is the one directory HEAD can hold as a single entry, and it is kept
// too, because `rm -f` cannot delete a directory and `set -e` would stop the
// script after its commit.
// Every copy left on disk is named on stderr, because a kept copy is content
// no commit carries and somebody has to decide about it. The comparison stages
// into a throwaway index, as renderDriftNote does, so neither the shared index
// nor the private one is written.
func renderWorkingTreeRemoval(removed []string) string {
	lines := []string{
		"# Delete each removed path's working-tree copy that git now holds byte for byte.",
		`_ze_gone=(` + quotePaths(removed) + `)`,
		`rm -f "$_ze_index.gone"`,
		`for _ze_path in "${_ze_gone[@]}"; do`,
		`  if [ ! -e "$_ze_path" ] && [ ! -L "$_ze_path" ]; then continue; fi`,
		`  if [ -d "$_ze_path" ] && [ ! -L "$_ze_path" ]; then`,
		`    echo "NOTE: kept $_ze_path: a directory is not the content this commit removed." >&2`,
		"    continue",
		"  fi",
		`  GIT_INDEX_FILE="$_ze_index.gone" git --literal-pathspecs add -f -- "$_ze_path" 2>/dev/null || true`,
		`  _ze_now=$(GIT_INDEX_FILE="$_ze_index.gone" git --literal-pathspecs -c core.quotePath=false ls-files -s -- "$_ze_path")`,
		`  if [ -n "$_ze_now" ] && printf '%s\n' "$_ze_removed" | grep -q -x -F -- "$_ze_now"; then`,
		`    rm -f -- "$_ze_path"`,
		"  else",
		`    echo "NOTE: kept $_ze_path: its working-tree copy is not the content this commit removed." >&2`,
		"  fi",
		"done",
		`rm -f "$_ze_index.gone"`,
	}
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
