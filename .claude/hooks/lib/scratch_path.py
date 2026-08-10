#!/usr/bin/env python3
"""Which `tmp/` paths a session may write. Shared by both PreToolUse dispatchers.

`tmp/` is keyed per CHECKOUT (`scripts/dev/ensure-links.py`), so a fixed name at
its root is one file for every session working this tree, and nothing removes it.
A path that carries a directory component under `tmp/` is per-task or
per-session, so it passes. That accepts `tmp/s/<id>/` and
`tmp/session/<YYYY-MM-DD>-<id>/` alike, and it accepts every producer-backed
folder (`verify/`, `review/`, `kernel/`, `qemu/`, ...), so no layout and no
folder is named here.

`check_scratch_path` (Bash) and `c_scratch_path_we` (Write and Edit) must refuse
the same paths. A path one surface refuses while the other accepts it is a guard
with a door in it, so the decision has ONE definition and both dispatchers call
this module. Three independent derivations of the session id drifted for weeks
behind a prose invariant; this is the same shape, and one definition is what
stops it.

Rule: ai/rules/commands.md, "Write Ad-Hoc Scratch Under Your Per-Session Dir".
"""

import os
import re

# Root names that are session-keyed or shared BY DESIGN, which
# ai/rules/commands.md says stay put: ze-verify.log and .ze-verify-duration.txt,
# the commit scripts and their message files, the delete-<sid>.sh a destructive
# command is written to, the mutation reports, and the test-timings record.
# Everything else at the root is ad-hoc.
SHARED_BY_DESIGN = re.compile(
    r"\A\.?(?:ze-verify|commit-|delete-|mutation|test-timings)"
)


def is_ad_hoc_root_file(path, project_dir):
    """True when `path` names an ad-hoc file directly at `<project_dir>/tmp/`.

    `path` is absolute (what the Write tool sends) or relative to the project
    directory (what a Bash redirect writes). A relative path is joined to
    `project_dir` rather than to the caller's CWD, which is not the project
    directory for every caller, so the guard cannot fail open there.

    The test is on the resolved PARENT: a file at the `tmp/` root has `tmp/` as
    its parent, and every deeper path does not. `tmp/` itself can be a symlink
    out of the tree, so both sides are resolved before they are compared.
    """
    given = path if os.path.isabs(path) else os.path.join(project_dir, path)
    tmp_root = os.path.realpath(os.path.join(project_dir, "tmp"))
    if os.path.dirname(os.path.realpath(given)) != tmp_root:
        return False
    return not SHARED_BY_DESIGN.match(os.path.basename(given))
