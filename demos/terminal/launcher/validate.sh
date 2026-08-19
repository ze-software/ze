#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh

# `ze` with no arguments opens the command launcher only on a terminal. Without
# one it prints the usage text (cmd/ze/ze_core_dispatch.go:298), and that text
# carries `show`, `doctor` and `Interactive` whatever the launcher does -- so
# the three assertions this replaces held with runTUILauncher deleted.
#
# The `@wait` directives ARE the assertions: each one holds until its pattern
# appears in the output that followed the keystroke before it, and the session
# exits non-zero when one does not. So the order is asserted too -- the filter
# narrows as the characters arrive, Enter opens the command tree, and Escape
# leaves it for the root menu, whose section headers appear only there and only
# with no filter set (cmd/ze/tui_menu.go:148).
#
# The breadcrumb is `ze > show` on screen, and matched here as `> show`: the
# launcher repaints the part of the title line that changed, so the two words
# do not reach this capture in one piece.
output=$(
    python3 /src/demos/terminal/pty-session.py \
        --ready 'Operations' \
        --command '@type show' \
        --command '@wait filter: show' \
        --command '@key enter' \
        --command '@wait > show' \
        --command '@escape' \
        --command '@wait Operations' \
        --command '@type doctor' \
        --command '@wait filter: doctor' \
        --command '@escape' \
        --command '@escape' \
        -- ze
)
assert_contains "${output}" 'filter: show'
assert_contains "${output}" '> show'
assert_contains "${output}" 'filter: doctor'
finish_validation launcher
