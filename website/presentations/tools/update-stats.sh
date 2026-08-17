#!/usr/bin/env bash
#
# Gather project stats from the main Ze source worktree.
#
# Outputs stats to stderr and key=value pairs to stdout so the calling
# deck script can source them for its own replacements.
#
# Usage: eval "$(<gh-pages>/presentations/tools/update-stats.sh)"
# The caller then has COMMITS, COAUTHORED, GO_SRC, etc. as shell variables.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

REPO="$("$SCRIPT_DIR/project-root.sh")"

echo "=== Gathering stats from $REPO ===" >&2

COMMITS=$(git -C "$REPO" log --oneline | wc -l | tr -d ' ')
COAUTHORED=$(git -C "$REPO" log --oneline --grep='Co-Authored-By' | wc -l | tr -d ' ')
GO_SRC=$(find "$REPO/internal" "$REPO/cmd" -name '*.go' -not -name '*_test.go' -not -path '*/vendor/*' | wc -l | tr -d ' ')
GO_SRC_LINES=$(find "$REPO/internal" "$REPO/cmd" -name '*.go' -not -name '*_test.go' -not -path '*/vendor/*' -exec cat {} + | wc -l | tr -d ' ')
GO_TEST=$(find "$REPO/internal" "$REPO/cmd" -name '*_test.go' -not -path '*/vendor/*' | wc -l | tr -d ' ')
GO_TEST_LINES=$(find "$REPO/internal" "$REPO/cmd" -name '*_test.go' -not -path '*/vendor/*' -exec cat {} + | wc -l | tr -d ' ')
GO_ALL_LINES=$(find "$REPO/internal" "$REPO/cmd" -name '*.go' -not -path '*/vendor/*' -exec cat {} + | wc -l | tr -d ' ')
FUNC_TESTS=$(find "$REPO/test" -name '*.ci' 2>/dev/null | wc -l | tr -d ' ')
EDITOR_TESTS=$(find "$REPO/test" -name '*.et' 2>/dev/null | wc -l | tr -d ' ')
YANG_FILES=$(find "$REPO" -name '*.yang' -not -path '*/vendor/*' | wc -l | tr -d ' ')
YANG_NODES=$(grep -rh 'leaf \|leaf-list \|container \|list ' --include='*.yang' "$REPO" 2>/dev/null | grep -v vendor | grep -cE '^\s*(leaf|leaf-list|container|list) ' || echo 0)
PLUGINS=$(find "$REPO/internal/plugins" -maxdepth 1 -mindepth 1 -type d | wc -l | tr -d ' ')
RATIONALE=$(find "$REPO/ai/rationale" -name '*.md' 2>/dev/null | wc -l | tr -d ' ')
LEARNED=$(find "$REPO/plan/learned" -name '*.md' 2>/dev/null | wc -l | tr -d ' ')
INTEROP=$(find "$REPO/test/interop/scenarios" -mindepth 1 -maxdepth 1 2>/dev/null | wc -l | tr -d ' ')
GO_SIZE_KB=$((GO_ALL_LINES / 1000))
VENDOR_SIZE=$(du -sh "$REPO/vendor" 2>/dev/null | cut -f1 | tr -d ' ')

format_k() {
    local n=$1
    if [ "$n" -ge 1000 ]; then
        echo "$((n / 1000))k"
    else
        echo "$n"
    fi
}

GO_SRC_K=$(format_k "$GO_SRC_LINES")
GO_TEST_K=$(format_k "$GO_TEST_LINES")

printf "  commits:       %s (%s co-authored)\n" "$COMMITS" "$COAUTHORED" >&2
printf "  Go source:     %s files (%s lines)\n" "$GO_SRC" "$GO_SRC_K" >&2
printf "  Go tests:      %s files (%s lines)\n" "$GO_TEST" "$GO_TEST_K" >&2
printf "  func tests:    %s\n" "$FUNC_TESTS" >&2
printf "  editor tests:  %s\n" "$EDITOR_TESTS" >&2
printf "  YANG:          %s files, %s config nodes\n" "$YANG_FILES" "$YANG_NODES" >&2
printf "  plugins:       %s\n" "$PLUGINS" >&2
printf "  rationale:     %s\n" "$RATIONALE" >&2
printf "  learned:       %s\n" "$LEARNED" >&2
printf "  interop:       %s scenarios\n" "$INTEROP" >&2
printf "  Go code size:  ~%sk lines / %s vendor\n" "$GO_SIZE_KB" "$VENDOR_SIZE" >&2

# Output key=value pairs to stdout for the caller to eval
cat <<EOF
COMMITS=$COMMITS
COAUTHORED=$COAUTHORED
GO_SRC=$GO_SRC
GO_SRC_LINES=$GO_SRC_LINES
GO_SRC_K=$GO_SRC_K
GO_TEST=$GO_TEST
GO_TEST_LINES=$GO_TEST_LINES
GO_TEST_K=$GO_TEST_K
GO_ALL_LINES=$GO_ALL_LINES
FUNC_TESTS=$FUNC_TESTS
EDITOR_TESTS=$EDITOR_TESTS
YANG_FILES=$YANG_FILES
YANG_NODES=$YANG_NODES
PLUGINS=$PLUGINS
RATIONALE=$RATIONALE
LEARNED=$LEARNED
INTEROP=$INTEROP
GO_SIZE_KB=$GO_SIZE_KB
VENDOR_SIZE=$VENDOR_SIZE
EOF
