---
name: GPG signing recovery
description: When GPG signing fails on commit, ask user to run echo test | gpg --clearsign to unlock the agent, then retry
type: feedback
---

When `git commit` fails with "gpg failed to sign the data" / "cannot open /dev/tty", ask the user to run `echo "test" | gpg --clearsign` to unlock the GPG agent, then retry the commit.

**Why:** The GPG agent sometimes needs a TTY passphrase prompt that Claude Code can't provide. Running the clearsign command in the user's terminal unlocks the agent for subsequent non-interactive signing.

**How to apply:** On GPG commit failure, suggest: "GPG signing failed. Could you run `! echo test | gpg --clearsign` to unlock the agent so I can retry?"
