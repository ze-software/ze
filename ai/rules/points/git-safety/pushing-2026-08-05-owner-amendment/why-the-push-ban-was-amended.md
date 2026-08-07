---
kind: note
level:
stage:
---
Thomas wrote the absolute push ban and amended it on 2026-08-05: a push is
allowed, from the commit script only, and only when he has ordered that push.
His reason for the original ban is what makes the exception safe. It stopped a
partial `git add` landing while several agents shared one index, and one script
bundling add, remove, commit and push leaves no such window open.
