#!/bin/bash
# Per-session spec claim helper.
#
# Each Claude session records the spec it is working on in its OWN marker file
# (tmp/session/.session-<SID>), never a shared file. This is safe for the normal
# Ze workflow of many agents editing main concurrently: no two sessions ever
# write the same path, so there is no append/remove discipline and no races.
#
# The session id is derived the same way the hooks derive it (state-file.sh ->
# session-id.sh), so a claim written here is read back identically by
# session-start.sh, compaction-reminder.sh and pretool-writeedit.py.
#
# Usage:
#   scripts/dev/spec-session.sh claim <spec>   # claim a spec for this session
#   scripts/dev/spec-session.sh current        # print this session's spec (empty if none)
#   scripts/dev/spec-session.sh release         # clear this session's claim
#
# <spec> may be a bare basename (spec-foo.md) or a path (plan/spec-foo.md).

# No `set -u`: the shared session-id.sh reads $CLAUDE_CODE_SESSION_ACCESS_TOKEN
# without a default, matching the hooks, which never run under `set -u`.
set -eo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

# shellcheck source=../../.claude/hooks/lib/state-file.sh
source .claude/hooks/lib/state-file.sh

cmd="${1:-current}"

case "$cmd" in
    claim)
        spec="${2:-}"
        if [ -z "$spec" ]; then
            echo "usage: spec-session.sh claim <spec>" >&2
            exit 2
        fi
        spec="$(basename "$spec")"
        if [ ! -f "plan/$spec" ]; then
            echo "spec-session: plan/$spec does not exist" >&2
            exit 2
        fi
        _claim_spec "$spec"
        # Auto-transition ready -> in-progress when work actually begins.
        status=$(sed -n 's/^| Status | *\([a-z-]*\).*/\1/p' "plan/$spec" | head -1)
        if [ "$status" = "ready" ]; then
            today=$(date +%Y-%m-%d)
            sed -i '' "s/^| Status | *ready.*/| Status | in-progress |/" "plan/$spec"
            sed -i '' "s/^| Updated | *[0-9-]*.*/| Updated | $today |/" "plan/$spec"
            echo "claimed $spec (ready -> in-progress)"
        else
            echo "claimed $spec (status: ${status:-unknown})"
        fi
        ;;
    current)
        sid=$(_session_id)
        marker="tmp/session/.session-${sid}"
        if [ -f "$marker" ]; then
            spec=$(head -1 "$marker" 2>/dev/null)
            [ "$spec" = "unassigned" ] && spec=""
            [ -n "$spec" ] && echo "$spec"
        fi
        ;;
    release)
        _release_session
        ;;
    *)
        echo "usage: spec-session.sh {claim <spec>|current|release}" >&2
        exit 2
        ;;
esac
