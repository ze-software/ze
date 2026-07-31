#!/bin/bash
# Stop hook: Write a compact session snapshot to per-spec session state file.
# Keeps the three most recent summaries. Does NOT release the session's spec
# claim: this hook fires between every turn, so releasing here killed the claim
# after turn one. That moved to session-end-scratch.sh, on SessionEnd.

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

# Load helpers
source .claude/hooks/lib/state-file.sh

STATE_FILE=$(_state_file)
TIMESTAMP=$(date -Iseconds)

# Gather current state
SID=$(_session_id)
# The marker lives under tmp/session/ (lib/state-file.sh _claim_spec, and
# scripts/dev/spec-session.sh). This read used the legacy .claude/ path, which
# nothing has written since the per-session marker move, so SELECTED_SPEC was
# always empty and no snapshot ever recorded the session's spec.
MARKER="tmp/session/.session-${SID}"
SELECTED_SPEC=""
if [ -f "$MARKER" ]; then
    SELECTED_SPEC=$(head -1 "$MARKER" 2>/dev/null)
    [ "$SELECTED_SPEC" = "unassigned" ] && SELECTED_SPEC=""
fi

MODIFIED=$(git diff --name-only 2>/dev/null | head -20)
STAGED=$(git diff --cached --name-only 2>/dev/null | head -20)
RECENT_COMMIT=$(git log -1 --oneline 2>/dev/null)
BRANCH=$(git branch --show-current 2>/dev/null)

# Skip the snapshot on a clean tree -- but do NOT remove the state file. A Stop
# hook fires between EVERY turn, and a clean-tree pause mid-session (e.g. right
# after a commit) is not session end: deleting the state file here deadlocked the
# next edit against the pretool session-state gate, which requires the file after
# a compaction. The 24h orphan sweep (lib/state-file.sh _cleanup_stale_markers)
# reclaims it once its marker is gone.
#
# The test was `-z "$HAS_CHANGES" && -z "$SELECTED_SPEC"`. SELECTED_SPEC read a
# marker path nothing writes, so it was ALWAYS empty and the test reduced to the
# clean-tree one. Fixing the path made the second half live for the first time,
# which let a clean tree with a claimed spec fall through and write a snapshot
# holding only Branch, Last commit and Spec. The writer below keeps three blocks,
# so that content-free snapshot evicted a real one and cost post-compaction
# recovery a set of Uncommitted digests (.claude/rules/post-compaction.md Tier 2).
# Test the tree alone: the spec is already recorded in the session marker.
HAS_CHANGES=$(git status --porcelain 2>/dev/null | head -1)
if [ -z "$HAS_CHANGES" ]; then
    exit 0
fi

# Build new snapshot
NEW_SNAPSHOT=$(cat <<SNAP
## Session: $TIMESTAMP

Branch: \`$BRANCH\`
$([ -n "$RECENT_COMMIT" ] && echo "Last commit: $RECENT_COMMIT")
$([ -n "$SELECTED_SPEC" ] && echo "Spec: \`$SELECTED_SPEC\`")
$(if [ -n "$MODIFIED" ]; then
    echo ""
    echo "Uncommitted:"
    echo "$MODIFIED" | while read -r f; do echo "- \`$f\`"; done
fi)
$(if [ -n "$STAGED" ]; then
    echo ""
    echo "Staged:"
    echo "$STAGED" | while read -r f; do echo "- \`$f\`"; done
fi)
SNAP
)

# Extract the two most recent snapshots from existing file
PREVIOUS=""
if [ -f "$STATE_FILE" ]; then
    PREVIOUS=$(awk '
        /^## Session:/ { block++; if (block > 2) exit }
        block >= 1 { print }
    ' "$STATE_FILE")
fi

# Write: header + new snapshot + up to 2 previous snapshots
{
    echo "# Session State"
    echo ""
    echo "$NEW_SNAPSHOT"
    if [ -n "$PREVIOUS" ]; then
        echo ""
        echo "---"
        echo "$PREVIOUS"
    fi
} > "$STATE_FILE"

# Clean up the per-session compaction marker.
#
# The session marker is NOT released here. A Stop hook fires between every turn,
# so releasing it here destroyed the claim after the FIRST turn. Three gates in
# block-premature-stop.sh read that marker (the closure check at :159, the
# in-progress warning at :176, the delegation nudge at :188), so all three fired
# once per claim and were silent for the rest of the session. The closure gate
# suffered worst: it can only exit 3 after commit A lands, which is many turns
# after the claim, so it was unreachable in the situation it exists for.
#
# The release now runs in session-end-scratch.sh, on the SessionEnd event, which
# is what this file's own comment above already called for.
rm -f ".claude/.compaction-detected-${SID}"

exit 0
