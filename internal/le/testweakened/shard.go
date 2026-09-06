// Design: docs/architecture/testing/test-health.md -- one ledger shard per commit session
// Related: ledger.go -- the row grammar every shard shares.
// Related: internal/le/lepath/commitsession.go -- the identity a shard is named after.
package testweakened

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/lepath"
)

// The two per-session ledger directories. A session writes its rows into the
// shard its own commit namespace names, so two authors never resolve to one
// path and neither can replace the other's rows. The gate assembles the
// population from the shards; nobody merges by hand.
const (
	WeakenedDir   = "test/weakened"
	RFCChangedDir = "test/rfc-changed"
)

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
	sort.Strings(names)
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
func ForeignShardProblems(session string, paths []string) []string {
	problems := make([]string, 0)
	var text textbuf.Buffer
	for _, path := range paths {
		for _, dir := range []string{WeakenedDir, RFCChangedDir} {
			owner, isShard := ShardSession(dir, path)
			if !isShard || owner == session {
				continue
			}
			problems = append(problems, text.Reset().Str(path).
				Str(" is session ").Str(owner).Str("'s ledger shard, and this commit is session ").
				Str(session).Str("'s. A shard records what its own author justified, so carrying ").
				Str("another one publishes their record under your subject. Name ").
				Str(ShardPath(dir, session)).Str(" instead.").String())
		}
	}
	return problems
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
// the same eight hex characters that name its commit script and its
// verification-debt shard.
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

// PruneLanded rewrites shard with the rows keep names, in the order they were
// written, under the header the file already carries. It is how the gate keeps
// the promise the file's own contract used to make an author keep: the shard
// carries the rows of THIS commit and never accumulates.
func PruneLanded(root, shard string, keep []Row) error {
	content, err := readRepositoryFile(root, shard)
	if err != nil {
		return err
	}
	lines := strings.Split(string(content), "\n")
	var page textbuf.Buffer
	kept := make(map[int]bool, len(keep))
	for _, row := range keep {
		kept[row.Line] = true
	}
	rows, problems := parseLedger(string(content), shard)
	if len(problems) != 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	dropped := make(map[int]bool, len(rows))
	for _, row := range rows {
		if !kept[row.Line] {
			dropped[row.Line] = true
		}
	}
	if len(dropped) == 0 {
		return nil
	}
	for index, line := range lines {
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
