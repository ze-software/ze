// Design: docs/architecture/testing/test-health.md -- one ledger shard per commit session
// Related: ledger.go -- the row grammar every shard shares.
// Related: internal/le/le/path/commitsession.go -- the identity a shard is named after.
package testweakened

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// WeakenedDir is the per-session ledger directory. A session writes its rows
// into the shard its own commit namespace names, so two authors never resolve
// to one path and neither can replace the other's rows. The gate assembles the
// population from the shards; nobody merges by hand.
//
// The owner's approval of an RFC-tagged test change is NOT a shard here: it
// lives in the session's tmp/ file rfc.ApprovalPath names, and the commit
// that used it carries it as a trailer line.
const WeakenedDir = "test/weakened"

// ShardPath answers the ledger shard one commit session owns inside dir.
func ShardPath(dir, session string) string {
	return dir + "/" + session + ".md"
}

// ShardSession answers the commit session a ledger path is named after, and
// whether path is a shard of dir at all.
func ShardSession(dir, path string) (string, bool) {
	rest, inside := strings.CutPrefix(path, dir+"/")
	if !inside {
		return "", false
	}
	session, isMarkdown := strings.CutSuffix(rest, ".md")
	if !isMarkdown || session == "" || strings.Contains(session, "/") {
		return "", false
	}
	return session, true
}

// Shard is one session's ledger file, the rows it holds, and what the row
// grammar says about them.
type Shard struct {
	Session  string   `json:"session"`
	Path     string   `json:"path"`
	Rows     []Row    `json:"rows,omitempty"`
	Problems []string `json:"problems,omitempty"`
	Mine     bool     `json:"mine"`
}

// ReadShards answers every shard under dir in session order, which is the whole
// ledger population and the answer to "whose rows are in it". An absent
// directory is an empty population: no session holds a row, which is the state
// a checkout is in between commits.
func ReadShards(root, dir, session string) ([]Shard, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	shards := make([]Shard, 0, len(names))
	for _, name := range names {
		path := dir + "/" + name
		shard := Shard{
			Session: strings.TrimSuffix(name, ".md"),
			Path:    path,
			Mine:    session != "" && path == ShardPath(dir, session),
		}
		content, readErr := readRepositoryFile(root, path)
		if readErr != nil {
			var text textbuf.Buffer
			shard.Problems = []string{text.Str("cannot read ").Str(path).Str(": ").Err(readErr).String()}
			shards = append(shards, shard)
			continue
		}
		shard.Rows, shard.Problems = parseLedger(string(content), path)
		shards = append(shards, shard)
	}
	return shards, nil
}

// ForeignShardProblems refuses a commit that names a ledger shard belonging to
// another session.
//
// The shard's name carries its writer, so this is what makes the namespace a
// rule rather than a habit: a commit can publish the rows it wrote and no
// others. It is the guard against the failure that has no other tell, where a
// commit lands carrying somebody else's justification and both the file and
// the row count look right.
//
// children is the commit namespaces of the subagents of the main thread that
// owns session (lepath.ChildCommitSessions), and their shards are not
// foreign to it: the main thread commits the work its subagents did, and the
// row is the subagent's own record of that work. A subagent's children are
// none, so a sibling's shard and the parent's stay foreign to it, and so does
// every shard of an unrelated session.
func ForeignShardProblems(session string, children, paths []string) []string {
	problems := make([]string, 0)
	var text textbuf.Buffer
	for _, path := range paths {
		owner, isShard := ShardSession(WeakenedDir, path)
		if !isShard {
			continue
		}
		if owner == session {
			continue
		}
		if slices.Contains(children, owner) {
			continue
		}
		problems = append(problems, text.Reset().Str(path).
			Str(" is session ").Str(owner).Str("'s ledger shard, and this commit is session ").
			Str(session).Str("'s. A shard records what its own author justified, so carrying ").
			Str("another one publishes their record under your subject. Name ").
			Str(ShardPath(WeakenedDir, session)).Str(" instead.").String())
	}
	return problems
}

// CarriedShards answers the ledger shards of session and of its children that
// paths names, in the order paths names them. They are the shards whose rows
// may accept the commit paths describes.
func CarriedShards(session string, children, paths []string) []string {
	carried := make([]string, 0, 1)
	for _, path := range paths {
		owner, isShard := ShardSession(WeakenedDir, path)
		if !isShard {
			continue
		}
		if slices.Contains(carried, path) {
			continue
		}
		if owner == session || slices.Contains(children, owner) {
			carried = append(carried, path)
		}
	}
	return carried
}

// shardPopulation renders one line per session holding rows, and names the
// shard this session writes even when it holds none. An author reads it to see
// whose rows are in the ledger without preparing a commit, which is the
// question that used to cost two messages to a peer.
func shardPopulation(shards []Shard, mine string) string {
	var page textbuf.Buffer
	held := false
	for _, shard := range shards {
		held = held || shard.Mine
		page.Str("  ").Str(shard.Path).Str(" holds ").Int(int64(len(shard.Rows))).Str(" row(s)")
		if shard.Mine {
			page.Str(", and is yours")
		}
		page.Byte('\n')
		for _, row := range shard.Rows {
			page.Str("    | ").Str(row.Name).Str(" |\n")
		}
	}
	if !held {
		page.Str("  ").Str(mine).Str(" holds none, and is yours to write.\n")
	}
	return page.String()
}

// sessionShard answers the shard path the caller writes and the gate reads. An
// empty session resolves the running harness's own commit namespace, which is
// the same eight hex characters that name its commit script and its commit
// message.
func sessionShard(root, dir, session string) (string, string, string) {
	if session != "" {
		return session, ShardPath(dir, session), ""
	}
	resolved, err := lepath.CommitSession(root, "")
	if err != nil {
		var text textbuf.Buffer
		return "", "", text.Str(cannotRunPrefix).
			Str("cannot resolve this session's commit namespace: ").Err(err).String()
	}
	return resolved, ShardPath(dir, resolved), ""
}

// LandedRows answers the rows the shard already holds at HEAD, keyed by the
// exact text of the row.
//
// A row that landed is a record of a commit that was made: git history holds
// it, it accepts nothing further, and it MUST NOT refuse the next commit. That
// refusal is what made an author reach into a shared file on another author's
// behalf three times, so the gate proves the row landed rather than asking
// anybody to delete it.
func LandedRows(root, shard string) map[string]bool {
	content, problem := baselineText(root, shard, headRevision)
	if problem != "" || strings.TrimSpace(content) == "" {
		return nil
	}
	rows, problems := parseLedger(content, shard)
	if len(problems) != 0 {
		return nil
	}
	landed := make(map[string]bool, len(rows))
	for _, row := range rows {
		landed[row.Key()] = true
	}
	return landed
}

// PruneLanded rewrites shard without the rows git already holds at HEAD that
// keep does not name, preserving every other line in the order it was written.
// It is how the gate keeps the promise the file's own contract used to make an
// author keep: a row whose commit landed does not accumulate.
//
// A row is dropped only on proof that it landed. A row its writer has not
// committed yet is not this commit's to remove, even when this commit uses no
// row at all: a main thread prunes the shard of a subagent whose next commit
// may still need a row, and a prune that dropped every unused row deleted a
// sibling's uncommitted rows in the days siblings shared one shard
// (plan/journal/concurrent-session-corruption.md).
// Rows are matched by their text, never by line number: the file is re-read
// here, and a row a sibling inserted or appended after the check read it
// moves every line number the check recorded.
func PruneLanded(root, shard string, keep []Row) error {
	content, err := readRepositoryFile(root, shard)
	if err != nil {
		return err
	}
	rows, problems := parseLedger(string(content), shard)
	if len(problems) != 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	landed := LandedRows(root, shard)
	kept := make(map[string]bool, len(keep))
	for _, row := range keep {
		kept[row.Key()] = true
	}
	dropped := make(map[int]bool, len(rows))
	for _, row := range rows {
		if !landed[row.Key()] {
			continue
		}
		if !kept[row.Key()] {
			dropped[row.Line] = true
		}
	}
	if len(dropped) == 0 {
		return nil
	}
	var page textbuf.Buffer
	for index, line := range strings.Split(string(content), "\n") {
		if dropped[index+1] {
			continue
		}
		if index != 0 {
			page.Byte('\n')
		}
		page.Str(line)
	}
	return os.WriteFile(filepath.Join(root, filepath.FromSlash(shard)), []byte(page.String()), 0o600)
}
