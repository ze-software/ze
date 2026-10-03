// Design: docs/architecture/core-design.md -- repository citation grammar
// Related: citation.go -- what a citation is; this file says where one is policed.
// Related: internal/le/doc/check/links.go -- the link sweep that polices these files.
// Related: internal/le/rfc/rename.go -- the rename that rewrites only these files.

package citation

import (
	"path"
	"strings"

	specpath "github.com/ze-software/ze/internal/le/spec/path"
)

// CorpusGlobs names the live instruction files the link sweep checks as its
// markdown corpus. A file it matches is policed even where Excluded names its
// tree, because it is read for what is true NOW: plan/README.md, the two
// templates, and the learned indexes all sit under record trees.
var CorpusGlobs = [...]string{
	"ai/INSTRUCTIONS.md",
	"ai/INDEX.md",
	"ai/NAVIGATION.md",
	"ai/rules/*.md",
	"ai/rationale/*.md",
	"ai/patterns/*.md",
	"ai/skills/*.md",
	"ai/agents/*.md",
	".claude/rules/*.md",
	".claude/README.md",
	".claude/hooks/README.md",
	"plan/README.md",
	"plan/TEMPLATE.md",
	"plan/TEMPLATE-CLOSURE.md",
	"plan/learned/RECURRING-PATTERNS.md",
	"plan/learned/HOOK-FRICTION.md",
	"plan/learned/DESIGN-HISTORY.md",
}

// excludePrefixes names the trees whose prose is not policed for a path that no
// longer resolves.
//
// The plan trees are RECORDS, and that is the whole reason (owner decision,
// 2026-08-29). A spec, a journal row, a deferral and a debt row each describe
// what was true when it was written, so a path inside one is a fact about that
// moment rather than a claim about the tree today. Repointing it at whatever
// replaced the file rewrites the record into something that was never true, and
// leaving it dangling is not a defect to be repaired later: it is the record
// working. A rename that moves a package therefore owes these trees nothing,
// and `./le rfc rename` leaves them byte-identical.
//
// The cost of the opposite policy was measured. One package rename left 383
// dangling references across plan/, and the gate reported them as breakage of
// the same kind as a live doc pointing at a deleted file. Chasing them touched
// hundreds of historical files, each edit racing another session that was
// writing its own rows, to make records say something they had not said.
//
// Live instruction files under plan/ stay in scope through CorpusGlobs.
//
// The rule for everything else is fix-on-touch: repair a stale path in a file
// you are already editing for another reason, and leave the rest alone.
var excludePrefixes = excludes()

// excludes joins the fixed trees to one spec prefix for each release bucket.
// The buckets come from specpath, so a spec is excluded wherever it sits: the
// list spelled plan/spec- alone, and it policed the specs of two buckets as
// live prose the moment the buckets appeared.
func excludes() []string {
	trees := [...]string{
		"vendor/",
		"third_party/",
		"plan/handover/",
		"plan/journal/",
		"plan/verification-debt/",
		"plan/known-failures/",
		// Closed-spec summaries, audits and session handoffs are records of the
		// same kind: each states what the tree held on the day it was written.
		// The learned INDEXES under plan/learned/ stay policed, as corpus files
		// (CorpusGlobs), whatever this prefix says.
		"plan/learned/",
		"plan/audits/",
		"plan/audit-",
		"plan/handoff-",
		// The two test ledgers are records of the same kind, and they are the
		// only ones that live outside plan/. A weakened row exists to name a
		// test that was DELETED and the spec that deleted it, so a path inside
		// one is a fact about that commit rather than a claim about the tree
		// today, and the spec it names is usually closed by the time anyone
		// reads the row.
		//
		// They also have no reachable repair, which the plan trees do. A shard
		// is named for its session, ForeignShardProblems
		// (internal/le/test/weakened/shard.go) refuses a commit that carries
		// another session's, and every commit gets a fresh session id. So the
		// only author who could edit the row is one that will never exist
		// again, and the gate reported a line nobody in the repository is
		// permitted to touch.
		"test/weakened/",
		// The site fixtures are FROZEN PUBLICATIONS, and they have no
		// reachable repair either. published-plugin-registry.json is the
		// registry as gh-pages 2fa8fa2ad published it, and the .md and .html
		// beside it are what the renderer must turn that input into, compared
		// byte for byte (internal/le/site/plugins_test.go). A marker fails that
		// comparison, and rewriting the snapshot makes it claim a publication
		// that never happened. So a path a later commit deleted, such as the
		// bgp-redistribute plugin 1ec5b741f8 removed on 2026-09-04, is a fact
		// about that publication rather than a claim about the tree today.
		"internal/le/site/testdata/",
	}
	buckets := specpath.Dirs()
	prefixes := make([]string, 0, len(trees)+len(buckets))
	prefixes = append(prefixes, trees[:]...)
	for _, dir := range buckets {
		prefixes = append(prefixes, dir+"/spec-")
	}
	return prefixes
}

// Excluded reports whether rel, a slash-separated repository path, sits in a
// record tree whose citations are not policed. A corpus file under such a tree
// is still policed: ask Policed for the whole answer. Safe for concurrent use.
func Excluded(rel string) bool {
	for _, prefix := range excludePrefixes {
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}

// InCorpus reports whether rel matches one of CorpusGlobs.
func InCorpus(rel string) bool {
	for _, pattern := range CorpusGlobs {
		// path.Match errs only on a malformed pattern, and every pattern above
		// is a literal checked by TestCorpusGlobsAreWellFormed.
		if matched, err := path.Match(pattern, rel); err == nil && matched {
			return true
		}
	}
	return false
}

// Policed reports whether the link sweep checks the citations rel holds: a
// corpus file always, any other file unless Excluded names its tree. A tool
// that rewrites citations rewrites exactly these files, so a record keeps the
// path it was written with. Safe for concurrent use.
func Policed(rel string) bool {
	if InCorpus(rel) {
		return true
	}
	return !Excluded(rel)
}
