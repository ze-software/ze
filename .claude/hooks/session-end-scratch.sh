#!/bin/bash
# SessionEnd hook: remove THIS session's private scratch dir, tmp/s/<session-id>/.
#
# Pairs with scripts/dev/session-scratch.sh (which creates it). Removes only this
# session's own dir, so it is safe while sibling sessions run in the same checkout.
# `scripts/dev/session-scratch.sh --reap` (run from session-start.sh) is the
# backstop for sessions that crashed or were killed before SessionEnd fired.
#
# The session id and the end reason arrive in the JSON on stdin -- SessionEnd
# does NOT export CLAUDE_CODE_SESSION_ID. In normal operation this session_id
# equals the id the helper resolved during the session (both derive from the
# session UUID); if the CLI ever fails to export it mid-session the two can
# differ, and --reap is the safety net.

# Resolve our own dir first (absolute), so sourcing works regardless of cwd.
HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$HOOK_DIR/../.." || exit 0
# shellcheck source=lib/session-id.sh
source "$HOOK_DIR/lib/session-id.sh"

input=$(cat)
{
    read -r sid
    read -r reason
} < <(printf '%s' "$input" | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
except Exception:
    d = {}
# Collapse any newline in the id to "/" so a multi-line value is rejected
# whole by _sid_safe below (read -r would otherwise keep only its first line).
print(d.get("session_id", "").replace("\n", "/").replace("\r", "/"))
print(d.get("reason", ""))
' 2>/dev/null)

# A session ending only to be resumed is not done; keep its scratch.
[ "$reason" = "resume" ] && exit 0

# Only a filename-safe id may name the dir we delete (mirrors the helper via the
# shared _sid_safe). Refuse empty or path-bearing ids so we can never rm outside
# tmp/s/.
sid=$(_sid_safe "$sid")
case "$sid" in
    "" | */* | . | ..) exit 0 ;;
esac

rm -rf "tmp/s/${sid}"

# Release this session's spec claim. This lived in session-end-summary.sh, which
# runs on Stop, so it destroyed the claim after the first turn and silenced the
# three marker-gated checks in block-premature-stop.sh for the rest of the
# session. SessionEnd is the event that actually means "this session is over",
# and the `reason = resume` guard above already returned, so a session that will
# come back keeps its claim.
#
# Written inline rather than through _release_session: that helper resolves the
# id from CLAUDE_CODE_SESSION_ID, which SessionEnd does not export (see the
# header). The id used here is the validated one from stdin.
rm -f "tmp/session/.session-${sid}"

# Also remove the session-suffixed binaries mk/session.mk built for this session
# (bin/ze-<sid>, bin/ze-test-<sid>, ...). They sit in bin/ rather than under
# tmp/s/<sid>/ because a binary's location decides where ze finds its config and
# database (internal/core/paths/paths.go ConfigDirFromBinary), so they are not
# covered by the rm -rf above and would otherwise pile up one full set per
# session. The exact `-<sid>` suffix is required, so the shared bin/ze that
# humans and CI build is never matched.
for f in bin/*-"${sid}"; do
    [ -f "$f" ] && rm -f "$f"
done
exit 0
