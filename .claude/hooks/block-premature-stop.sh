#!/bin/bash
# Stop hook: Block premature stopping when work remains
# BLOCKING: Catches ownership-dodging, permission-seeking, and premature handoff
#
# Exit 2 forces Claude to continue instead of stopping.
# Exit 1 warns but allows the stop.
# Exit 0 allows the stop silently.

set -eo pipefail

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

INPUT=$(cat)

# Loop bound for the PHRASE SCAN ONLY. The harness sets stop_hook_active when
# this session's stop was already refused by a Stop hook and it is trying again.
# Refusing the retry too can trap a session with no way out, because the only
# escape from the phrase scan is rewording, and a session obeying a rule that
# mandates the wording has nothing to reword to.
#
# It is a FLAG, not an early exit. Exiting here would also skip the spec-closure
# gate below, and that gate needs no loop bound: it has two documented escapes,
# running commit B or writing tmp/session/.closure-ack-<stem>, and it prints the
# second one in the very message it blocks with. An early exit here meant that
# tripping any phrase on turn N switched the closure gate off for turn N+1.
STOP_RETRY=false
if [ "$(echo "$INPUT" | jq -r '.stop_hook_active // false' 2>/dev/null || echo false)" = "true" ]; then
    STOP_RETRY=true
fi

# `|| TEXT=""` is load-bearing: under `set -eo pipefail` a jq parse failure kills
# the script before the guard below, and the hook exited 5 on malformed input.
# The header documents 0, 1 and 2 only, so a registered blocking hook was
# returning an undefined code. Unparseable input allows the stop: a Stop gate
# that refuses on garbage it cannot read traps the session for no reason.
TEXT=$(echo "$INPUT" | jq -r '.last_assistant_message // empty' 2>/dev/null) || TEXT=""
[ -z "$TEXT" ] && exit 0

# A phrase inside a fenced block or `backticks` is NAMED, not used. Strip both
# before the phrase scan. Without this the hook blocks any message that documents
# its own banned phrases, which is what it did on the first turn after it was
# registered on 2026-07-31: a report that quoted `would you like me to` as an
# example was refused an end. The hook's own delegation nudge warns rather than
# blocks for the same reason (see the comment above the marker check below): a
# Stop gate that traps a legitimate turn is worse than the miss it prevents.
#
# EVERY failure mode here must scan MORE, never less. A filter that drops text is
# a gate that silently switches itself off, which is worse than the false positive
# it was added to stop. Four guards enforce that:
#
#   1. An UNCLOSED fence is not a code block. Lines after an unterminated ``` are
#      buffered and emitted at EOF. The first version dropped them, so a message
#      whose fence was never closed passed a real request with rc=0.
#   2. A fence closes only on a run at least as long as the one that opened it,
#      per the markdown rule. The first version toggled on any ```, so the inner
#      fence of a ````markdown wrapper flipped parity and leaked its content.
#   3. If stripping consumed EVERYTHING, the message was all markup. Scan the raw
#      text instead of an empty string, which would match nothing at all.
#   4. Inline spans are stripped only on a line whose backticks BALANCE. A stray
#      backtick makes a left-to-right pass pair it with the OPENING tick of a
#      later, legitimate span and delete the text between, which is where a real
#      request tends to sit. A dropped closing backtick is an ordinary typo, so
#      this was reachable without intent. An odd count leaves the line raw.
#
# Backticks only, deliberately. Stripping every double-quoted span would also
# hide real permission-seeking that happens to quote something. The cost is that
# a message quoting the PHRASES array below, which is written in double quotes,
# still blocks.
SCAN=$(printf '%s\n' "$TEXT" | awk '
    function flush(  i) { for (i = 1; i <= np; i++) print pend[i]; np = 0 }
    {
        t = $0; sub(/^[[:space:]]+/, "", t)
        n = 0; while (substr(t, n + 1, 1) == "`") n++
        if (n >= 3) {
            if (!fence)          { fence = 1; fencelen = n; np = 0; next }
            else if (n >= fencelen) { fence = 0; np = 0; next }
        }
        if (fence) { pend[++np] = $0; next }
        line = $0
        if (gsub(/`/, "`", line) % 2 == 0) gsub(/`[^`]*`/, "", line)
        print line
    }
    END { if (fence) flush() }
') || SCAN="$TEXT"
[ -z "${SCAN//[[:space:]]/}" ] && SCAN="$TEXT"

REASONS=()

# --- Stop phrase detection ---
# Each line is a grep -iE pattern. First match wins.

PHRASES=(
    # Ownership-dodging: offering instead of doing
    "let me know if you"
    "would you like me to"
    "feel free to"
    "if you.d like me to"
    "if you want me to"
    "happy to help"
    "I can [a-z]+ .* if you"
    # Premature handoff
    "I.ll stop here"
    "I will stop here"
    "I.ll pause here"
    "I will pause here"
    "that.s all for now"
    "I.ll leave .* to you"
    "I will leave .* to you"
    # Permission-seeking: asking instead of doing
    "should I (proceed|continue|go ahead)"
    "do you want me to"
    "^want me to"
    "want me to .* or"
    "shall I (proceed|continue|go ahead|start|keep)"
    "before I proceed"
    "ready for me to"
    "or (leave|skip|ignore) (them|it|this|that)"
    "or should I"
    # Choice-offering: presenting options instead of deciding
    "or something else"
)

# Asking what to do NEXT is not the same failure as asking permission to do what
# was already requested, and it is only premature when work actually remains.
# .claude/rules/session-start.md:72 REQUIRES the opposite behaviour once the
# original task is done: "stop and ask \"What next?\" instead of picking up other
# uncommitted work". Scanning for these unconditionally blocked a sentence
# another live rule mandates, with no way to comply with both.
#
# So these fire only when this session has an in-progress claimed spec, which is
# the state that means work remains. With no such claim the session is entitled
# to ask, and session-start.md tells it to.
COMPLETION_PHRASES=(
    "what would you like"
    "what do you want to do"
    "what.s next"
    "what next"
)

# --- State check: spec in-progress ---
#
# Runs BEFORE the phrase scan because the scan's completion list depends on
# OPEN_WORK. Reordering is safe: this block's own exit 2 (the closure gate) is
# unconditional, and the REASONS it appends are read only in the decision below.
OPEN_WORK=0

source .claude/hooks/lib/state-file.sh 2>/dev/null || true
SID=$(_session_id 2>/dev/null || echo "")
if [ -n "$SID" ]; then
    MARKER="tmp/session/.session-${SID}"
    if [ -f "$MARKER" ]; then
        # Liveness heartbeat. _cleanup_stale_markers (lib/state-file.sh:82) deletes
        # a marker whose MTIME is over 24h old, and it runs from session-start.sh
        # on every session start in this checkout. _claim_spec sets that mtime once
        # and nothing else ever rewrites it, so a session running longer than 24h
        # had its LIVE claim swept the moment any other session started. Reading
        # the claim here proves this session is alive, so refresh it on the way past.
        #
        # Unreachable before the release moved to SessionEnd: the claim used to die
        # on the first Stop, so it never survived long enough to age out.
        #
        # -c so a MISSING marker is never created. Plain touch creates the path,
        # so if a sibling session's sweep deleted the marker between the -f test
        # above and this line, the hook would resurrect it EMPTY. An empty marker
        # makes every gate below skip silently, and it also blocks the orphan
        # state-file reclaim at state-file.sh:114.
        touch -c "$MARKER" 2>/dev/null || true
        SPEC=$(head -1 "$MARKER" 2>/dev/null || true)
        if [ -n "$SPEC" ] && [ "$SPEC" != "unassigned" ] && [ -f "plan/$SPEC" ]; then
            # Closure gate: block if this session's spec is implemented but the
            # second closure commit (git rm the spec) never ran. The detector
            # exits 3 for exactly that "commit A ran, commit B skipped" state.
            # See ai/rules/planning.md "Spec Closure".
            CLOSURE_RC=0
            CLOSURE_MSG=$(python3 scripts/dev/spec-closure-check.py --spec "plan/$SPEC" 2>&1) || CLOSURE_RC=$?
            if [ "$CLOSURE_RC" -eq 3 ]; then
                {
                    echo "BLOCKED: spec implemented but not closed."
                    echo "$CLOSURE_MSG"
                } >&2
                exit 2
            fi
            # Fixed from a dead `^Status:` grep: specs write status as a
            # `| Status | in-progress |` metadata table, so the old anchored
            # pattern never matched a single spec.
            # Tolerant of surrounding spaces, matching spec-closure-check.py:54
            # (`^\|\s*Status\s*\|`) which reads the same field. A stricter pattern
            # here would let the two consumers disagree about whether a spec is
            # open, and the bug immediately above was a Status regex that matched
            # no spec at all.
            if grep -qE "^\|[[:space:]]*Status[[:space:]]*\|.*in-progress" "plan/$SPEC" 2>/dev/null; then
                REASONS+=("Spec '$SPEC' still in-progress")
                OPEN_WORK=1
            fi
            # Delegation nudge: this session claimed a spec and worked it without
            # ever spawning an agent, so it ran the phase inline instead of
            # supervising it (ai/rules/spec-delegation.md). mark-agent-spawned.sh
            # writes the marker on every Agent/Task call.
            #
            # Warn, never block. A session can legitimately claim a spec and do
            # one mechanical edit, and a Stop hook that traps such a session is a
            # worse failure than the one it is catching.
            #
            # Heartbeat this marker too, for the same reason as the claim above.
            # mark-agent-spawned.sh rewrites it only when an Agent actually runs,
            # and TWO reapers delete it at 24h: lib/state-file.sh:92 and the
            # unfiltered find at session-start.sh:22. A session older than a day
            # that delegated yesterday but not today now KEEPS its claim, so
            # without this it would lose only the spawn marker and the nudge would
            # fire falsely, telling a properly supervising session it never
            # delegated. -c again, so a missing marker is never created: creating
            # it would silence the nudge for a session that really did work inline.
            SPAWNED="tmp/session/.agent-spawned-${SID}"
            touch -c "$SPAWNED" 2>/dev/null || true
            if [ ! -f "$SPAWNED" ]; then
                REASONS+=("Delegation: spec '$SPEC' worked with no subagent spawned")
            fi
        fi
    fi
fi

# --- Stop phrase scan ---
# The always-block list, plus the completion list only when work remains.

SCAN_PATTERNS=("${PHRASES[@]}")
if [ "$OPEN_WORK" = 1 ]; then
    SCAN_PATTERNS+=("${COMPLETION_PHRASES[@]}")
fi

# STOP_RETRY bounds this scan and nothing else. The closure gate above already
# ran, and it keeps its block on a retry.
if [ "$STOP_RETRY" = false ]; then
    for pattern in "${SCAN_PATTERNS[@]}"; do
        if printf '%s\n' "$SCAN" | grep -iqE "$pattern"; then
            REASONS+=("Stop phrase: $pattern")
            break
        fi
    done
fi

# --- Decision ---

if [ ${#REASONS[@]} -eq 0 ]; then
    exit 0
fi

HAS_PHRASE=false
HAS_STATE=false
for r in "${REASONS[@]}"; do
    case "$r" in
        "Stop phrase:"*) HAS_PHRASE=true ;;
        "Spec "*) HAS_STATE=true ;;
        "Delegation:"*) HAS_STATE=true ;;
    esac
done

# Stop phrases always block
if [ "$HAS_PHRASE" = true ]; then
    {
        echo "BLOCKED: Premature stop detected."
        for r in "${REASONS[@]}"; do
            echo "  - $r"
        done
        echo "You have work remaining. Continue without asking permission."
    } >&2
    exit 2
fi

# State reasons without stop phrases: warn, don't block.
# The header stays generic because more than one state reason can fire (spec
# still in-progress, phase never delegated) and they are listed individually
# below; naming only one of them in the header misreports the other.
if [ "$HAS_STATE" = true ]; then
    {
        echo "Warning: stopping with open session state."
        for r in "${REASONS[@]}"; do
            echo "  - $r"
        done
    } >&2
    exit 1
fi

exit 0
