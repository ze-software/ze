// Design: docs/architecture/testing/test-health.md -- one ledger shard per commit session
package testweakened

import (
	"slices"
	"strings"
	"testing"
)

// VALIDATES: a main thread may carry its own shard and its subagents' shards,
// and CarriedShards answers exactly those, while an unrelated session's shard
// is refused and a subagent (no children) is refused its sibling's.
// PREVENTS: per-agent namespaces locking the main thread out of committing a
// subagent's work, and the allowance opening to any other session.
func TestForeignShardProblemsAdmitsOnlyThisSessionsChildren(t *testing.T) {
	const parent, child, unrelated = "aaaaaaaa", "cccccccc", "dddddddd"
	own := ShardPath(WeakenedDir, parent)
	childShard := ShardPath(WeakenedDir, child)
	foreign := ShardPath(WeakenedDir, unrelated)
	paths := []string{"pkg/x_test.go", own, childShard}

	if problems := ForeignShardProblems(parent, []string{child}, paths); len(problems) != 0 {
		t.Fatalf("parent carrying its child's shard refused: %v", problems)
	}
	if got := CarriedShards(parent, []string{child}, paths); !slices.Equal(got, []string{own, childShard}) {
		t.Fatalf("CarriedShards = %v, want [%s %s]", got, own, childShard)
	}
	problems := ForeignShardProblems(parent, []string{child}, append(paths, foreign))
	if len(problems) != 1 || !strings.Contains(problems[0], "is session "+unrelated+"'s ledger shard") {
		t.Fatalf("unrelated shard problems = %v", problems)
	}
	if got := CarriedShards(parent, []string{child}, append(paths, foreign)); slices.Contains(got, foreign) {
		t.Fatalf("CarriedShards carried the unrelated shard: %v", got)
	}
	problems = ForeignShardProblems(child, nil, []string{childShard, own})
	if len(problems) != 1 || !strings.Contains(problems[0], "is session "+parent+"'s ledger shard") {
		t.Fatalf("a subagent carrying its parent's shard = %v", problems)
	}
}
