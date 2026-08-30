---
kind: directive
level: MUST
stage:
---
**A consumer MUST NOT be committed without the file that DEFINES every symbol it
newly uses, and a commit script that carried Go MUST be followed by
`./le repository tracked-build check`.**

| Situation | Do |
|-----------|-----|
| You are about to `--file` a consumer | Name the file that DEFINES every symbol it newly uses, and check that file is in the same `--file` list or already committed (`git log -1 -- <path>`) |
| The commit script has just run and it carried Go | Run `./le repository tracked-build check`. About 45s. This is step 7 of the commit workflow, not an optional extra |
| It goes red | Commit the producer. Never revert the consumer, and never park it: HEAD is broken for everyone until you do |
