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

# _sid_safe prints its argument when it is usable as a filename component, and
# nothing otherwise. Callers that receive an id rather than resolve one (the
# SessionEnd hooks read it from their JSON payload) need the safety rule without
# the resolution walk. It delegates to the same _sid_safe in session_id.py, so
# there is still exactly one implementation of the rule -- restoring the shell
# copy that the shim replaced would re-create the drift the shim exists to stop.
_sid_safe() {
    python3 "$_ZE_SID_DIR/session_id.py" --safe "$1"
}
