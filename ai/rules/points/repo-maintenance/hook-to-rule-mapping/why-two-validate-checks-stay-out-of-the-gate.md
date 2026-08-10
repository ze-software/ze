---
kind: note
level:
stage:
---
The two changed-file checks take `changed_files` as their subject, which is `git diff HEAD` plus untracked files. Several sessions share this checkout, so that list is mostly other sessions' half-written work, and both checks demand a completeness (a cross-package caller, a `.ci` test) that a file in the middle of an edit cannot show. Measured in one working tree that nobody committed to in between: `check_cross_package_wiring` reported 0 findings on 2026-08-09 and 22 on 2026-08-10, every one of them in six files another session had modified and left uncommitted. In the gate those 22 are a red on a verify run whose author changed nothing they name. `ze-ste-check` stays out of `ze-doc-test` for the same reason (`mk/inventory.mk`), and the commit-time gates in `scripts/dev/commit_helper.py` are where a changed-file check belongs: they see the files of ONE commit.
