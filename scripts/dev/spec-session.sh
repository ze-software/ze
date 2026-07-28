#!/bin/bash
# Per-session spec claim helper.
#
# Each Claude session records the spec it is working on in its OWN marker file
# (tmp/session/.session-<SID>), never a shared file. This is safe for the normal
# Ze workflow of many agents editing main concurrently: no two sessions ever
# write the same path, so there is no append/remove discipline and no races.
#
# That safety is a property of the ID, not of this script, and until 2026-07-16 it
# did NOT hold: an interactive `claude` passes no --session-id and subscription auth
# issues no access token, so every session fell through to the shared constant and
# this helper's guarantee inverted -- `claim` OVERWROTE whatever spec another
# session had claimed (observed 2026-07-16: two sessions, one marker, the second
# clobbered the first). session-id.sh strategy 1 now reads the exported
# $CLAUDE_CODE_SESSION_ID first; `make ze-hook-test` locks writer/reader parity.
#
# The session id is derived the same way the hooks derive it (state-file.sh ->
# session-id.sh), so a claim written here is read back identically by
# session-start.sh, compaction-reminder.sh and pretool-writeedit.py.
#
# Usage:
#   scripts/dev/spec-session.sh claim <spec>   # claim a spec for this session
#   scripts/dev/spec-session.sh current        # print this session's spec (empty if none)
#   scripts/dev/spec-session.sh release         # clear this session's claim
#   scripts/dev/spec-session.sh wip             # list in-progress specs, stalest first
#
# <spec> may be a bare basename (spec-foo.md) or a path (plan/spec-foo.md).
#
# WIP cap: `claim` refuses when it would push the number of in-progress specs
# past $ZE_SPEC_WIP_CAP (default 12). Every rule in ai/rules/ governs how well a
# single spec is executed; none of them limits how MANY are open at once, and by
# 2026-07-28 that was 27 in-progress with the oldest untouched for 40 days. An
# in-progress spec is a standing claim on attention and a collision risk for
# concurrent sessions, so the cap is the one place the system can say "close
# something before you open something".
#
# The cap fires only on the ready -> in-progress transition, which is where WIP
# is actually created (and where /ze-implement enters). Claiming a spec that is
# already in-progress is RESUMING, not adding, and is always allowed; so is
# claiming a skeleton/design spec for research, which starts no implementation.

# No `set -u`: the shared session-id.sh reads $CLAUDE_CODE_SESSION_ID and
# $CLAUDE_CODE_SESSION_ACCESS_TOKEN without a default, matching the hooks, which
# never run under `set -u`.
set -eo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

# shellcheck source=../../.claude/hooks/lib/state-file.sh
source .claude/hooks/lib/state-file.sh

ZE_SPEC_WIP_CAP="${ZE_SPEC_WIP_CAP:-12}"

# Print "<updated-date>\t<spec-file>" per in-progress spec, stalest first.
# Two greps over the whole set, never one per file (ai/rules/no-fork-loops.md).
_wip_list() {
    local files
    files=$(grep -l '^| Status | *in-progress' plan/spec-*.md 2>/dev/null || true)
    [ -z "$files" ] && return 0
    # shellcheck disable=SC2086
    grep -H '^| Updated |' $files 2>/dev/null |
        sed 's/^\([^:]*\):| Updated | *\([0-9-]*\).*/\2\t\1/' |
        sort
}

_wip_count() {
    _wip_list | grep -c . || true
}

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
        status=$(sed -n 's/^| Status | *\([a-z-]*\).*/\1/p' "plan/$spec" | head -1)

        # WIP cap: only the ready -> in-progress transition ADDS work in flight.
        if [ "$status" = "ready" ]; then
            wip=$(_wip_count)
            if [ "$wip" -ge "$ZE_SPEC_WIP_CAP" ]; then
                echo "spec-session: refusing to start $spec" >&2
                echo "  $wip specs are already in-progress (cap ${ZE_SPEC_WIP_CAP})." >&2
                echo "  Close one before starting another. Stalest first:" >&2
                _wip_list | head -5 | sed 's/^/    /' >&2
                echo >&2
                echo "  Closing a spec: /ze-close (learned summary + git rm)." >&2
                echo "  Deliberately going wider: ZE_SPEC_WIP_CAP=$((wip + 1)) $0 claim $spec" >&2
                exit 3
            fi
        fi

        _claim_spec "$spec"
        # Auto-transition ready -> in-progress when work actually begins.
        if [ "$status" = "ready" ]; then
            today=$(date +%Y-%m-%d)
            # -i.bak (no space) works on both GNU and BSD sed; plain -i '' is macOS-only.
            sed -i.bak "s/^| Status | *ready.*/| Status | in-progress |/" "plan/$spec"
            sed -i.bak "s/^| Updated | *[0-9-]*.*/| Updated | $today |/" "plan/$spec"
            rm -f "plan/$spec.bak"
            echo "claimed $spec (ready -> in-progress; $(_wip_count)/${ZE_SPEC_WIP_CAP} in flight)"
        else
            echo "claimed $spec (status: ${status:-unknown})"
        fi
        ;;
    wip)
        count=$(_wip_count)
        echo "$count in-progress spec(s), cap ${ZE_SPEC_WIP_CAP} (stalest first):"
        _wip_list | sed 's/^/  /'
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
        echo "usage: spec-session.sh {claim <spec>|current|release|wip}" >&2
        exit 2
        ;;
esac
