#!/bin/bash
# Stop hook: report blocking rules whose trigger matched this session's files
# but whose file was never read.
#
# ADVISORY ONLY. Exit 1 warns, exit 0 is silent. It MUST NEVER exit 2.
#
# Exit 2 refuses the session an end, and this hook has nothing worth refusing an
# end over: it produces a measurement, not a verdict. It is also registered
# AFTER block-premature-stop.sh so it can never mask that gate's decision
# (plan/spec-knowledge-3-rule-digest.md, "The miss-detector").
#
# It measures assumption A-4 of spec-knowledge-3-rule-digest: whether a model
# that sees a matching **When:** trigger actually loads the rule. It runs in the
# CURRENT world where ai/rules/CONDENSED.md is still eagerly imported, so a miss
# it reports is a session that never consulted a rule its own file types matched
# even with the whole digest in context. The logic, and the blind spot it under-
# reports through, live in scripts/dev/rule_coverage.py.

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.." || exit 0

# shellcheck source=/dev/null
. "$(dirname "$0")/lib/session-id.sh" 2>/dev/null || true

INPUT=$(cat)

# The Stop payload carries this session's transcript. Prefer it: the resolver in
# running_model.py falls back to "most recently written transcript" when no
# session id is exported, and this project directory routinely holds three live
# transcripts, so the mtime winner can be a NEIGHBOUR session's file.
TRANSCRIPT=$(printf '%s' "$INPUT" | jq -r '.transcript_path // empty' 2>/dev/null) || TRANSCRIPT=""
SID=$(printf '%s' "$INPUT" | jq -r '.session_id // empty' 2>/dev/null) || SID=""
if [ -z "$SID" ] && type _session_id &>/dev/null; then
    SID=$(_session_id 2>/dev/null || echo "")
fi

ARGS=(--session "${SID:-unknown}")
[ -n "$TRANSCRIPT" ] && ARGS+=(--transcript "$TRANSCRIPT")

OUT=$(python3 scripts/dev/rule_coverage.py "${ARGS[@]}" 2>&1)
RC=$?

# Any exit code the detector could not produce on purpose (a python crash, a
# missing interpreter) is reported and swallowed. A broken measurement must not
# change whether the session may stop.
if [ "$RC" -ne 0 ] && [ "$RC" -ne 1 ]; then
    echo "rule-coverage: detector exited $RC, skipping the report" >&2
    [ -n "$OUT" ] && echo "$OUT" >&2
    exit 0
fi

[ -n "$OUT" ] && echo "$OUT" >&2

# 1 only when something was missed. Never 2.
exit "$RC"
