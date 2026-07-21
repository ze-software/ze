#!/bin/bash
# Thin shim over the ONE session-id resolver, .claude/hooks/lib/session_id.py.
# Usage: source this file, then call _session_id (unchanged for every caller).
#
# The resolution logic lives in Python ONLY (ai/rules/go-standards.md, "Scripts:
# Python Only"), so there is a single source of truth. This file used to carry a
# full copy of the argv walk / JWT decode / fallback; three such copies drifted for
# weeks despite a "MUST stay identical" note, so the copies are gone. A shim cannot
# drift from the thing it calls. See spec-fixit-session-id-collision and
# .claude/hooks/README.md ("Session Identity").

# Resolve session_id.py to an ABSOLUTE path once, at source time, so _session_id
# works no matter the caller's cwd or how this file was sourced.
_ZE_SID_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

_session_id() {
    python3 "$_ZE_SID_DIR/session_id.py"
}
