---
kind: directive
level: MUST
stage:
---
**When the work a session finishes was handed over by a file in `plan/handover/`, the session MUST remove that handover file through `./le commit create ... remove <path>` once every item in it has landed, and MUST remove its own unused leftover temporary artifacts (session scratch under `tmp/session/<id>/scratch/`, spent `tmp/commit-*.sh` and `tmp/commit-msg-*.txt`, and empty or superseded `tmp/commit-*-approved-*.md` files) before claiming done.** Leave files another session owns, uncommitted user-requested files, and any artifact still in use untouched.
