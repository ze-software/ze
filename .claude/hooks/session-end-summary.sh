#!/bin/bash
# Stop hook: Write a compact session snapshot to per-spec session state file.
# Keeps the three most recent summaries, and every block that is not a summary
# (phase handoffs, notes) verbatim. Does NOT release the session's spec
# claim: this hook fires between every turn, so releasing here killed the claim
# after turn one. No hook releases it now -- `spec-session.sh release` does, from
# /ze-close, when the spec closes.

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
# a compaction. Nothing reclaims it either: no timer and no hook deletes anything
# under tmp/session/, so the file this hook writes outlives the session that
# wrote it and `make ze-clean-sessions BEFORE=<date>` is what removes it.
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

# Salvage everything the new snapshot does not replace: the two most recent
# snapshots, PLUS every block that is not a snapshot at all.
#
# A snapshot is recognised by its own grammar, never by its position: the
# `## Session:` heading, then the lines this hook writes under it (Branch, Last
# commit, Spec, the `Uncommitted:`/`Staged:` headings, their `- \`path\`` items,
# blank lines, and the `---` separators between blocks). The FIRST line that
# fits none of those ends the snapshot, so a phase handoff appended at the end
# of the file (ai/skills/ze-implement.md, "Phase handoff"), the
# `## Last Compaction` marker pre-compact-save.sh inserts, and any note a
# session wrote by hand all survive verbatim.
#
# The old salvage took a POSITION: print from the first `## Session:` line, exit
# at the third. Once two snapshots existed, everything after them was outside
# that window and the `>` below deleted it. On 2026-08-09 that destroyed a phase
# 4 handoff and a set of main-thread notes, and .claude/rules/post-compaction.md
# makes this file Tier 1 recovery, so the next phase had to re-derive them.
PRESERVED=""
if [ -f "$STATE_FILE" ]; then
    PRESERVED=$(awk -v keep=2 '
        function emit(block) {
            if (emitted++) { print ""; print "---" }
            print block
        }
        function flush(   t) {
            if (kind == "") return
            t = cur
            # Trailing blank lines are re-added by the writer, so trimming them
            # is what stops one blank line accruing per rewrite. A snapshot also
            # sheds the `---` this hook itself put after it.
            if (kind == "snap") {
                while (t ~ /\n[ \t]*(---)?[ \t]*$/) sub(/\n[ \t]*(---)?[ \t]*$/, "", t)
                snaps++
                if (snaps <= keep) snap[snaps] = t
            } else {
                while (t ~ /\n[ \t]*$/) sub(/\n[ \t]*$/, "", t)
                while (t ~ /^[ \t]*\n/) sub(/^[ \t]*\n/, "", t)
                if (t != "") other[++others] = t
            }
            cur = ""; kind = ""
        }
        # The writer re-emits the file header, so drop the one already there.
        /^# Session State[ \t]*$/ { flush(); next }
        /^## Session:/            { flush(); kind = "snap"; cur = $0; next }
        kind == "snap" && ($0 ~ /^(Branch|Last commit|Spec):/ ||
                           $0 ~ /^(Uncommitted|Staged):[ \t]*$/ ||
                           $0 ~ /^- `[^`]*`[ \t]*$/ ||
                           $0 ~ /^[ \t]*$/ ||
                           $0 ~ /^---[ \t]*$/) { cur = cur "\n" $0; next }
        {
            if (kind == "snap") flush()
            if (kind == "") { kind = "other"; cur = $0 } else { cur = cur "\n" $0 }
        }
        END {
            flush()
            for (i = 1; i <= snaps && i <= keep; i++) emit(snap[i])
            for (j = 1; j <= others; j++) emit(other[j])
        }
    ' "$STATE_FILE")
fi

# Write: header + new snapshot + the salvaged blocks
{
    echo "# Session State"
    echo ""
    echo "$NEW_SNAPSHOT"
    if [ -n "$PRESERVED" ]; then
        echo ""
        echo "---"
        echo "$PRESERVED"
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
# The release now runs in `spec-session.sh release`, called by /ze-close when
# the spec closes. It briefly lived on SessionEnd instead; that hook is gone,
# because its other job was deleting tmp/session/, which is now banned outright
# (plan/spec-session-bin-directory.md AC-7).
rm -f ".claude/.compaction-detected-${SID}"

exit 0
