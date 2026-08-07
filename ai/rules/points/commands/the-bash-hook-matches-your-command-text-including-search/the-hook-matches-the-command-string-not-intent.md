---
kind: note
level:
stage:
---
`.claude/hooks/pretool-bash.py` blocks the banned git verbs by matching the
command STRING. It cannot tell a verb you are running from a verb you are
searching for, so a read-only grep is rejected when its own pattern spells
one:
