---
kind: note
level:
stage:
---
`git commit`/`git add` inside the script is fine -- the ban is on
direct AI tool invocations, not on what the script does when it runs.
Run the finished script yourself with `bash` and the printed path.
The tool call is `bash <script>`, not a bare `git commit`, so it
passes the hook that blocks the raw verbs. `git restore --staged <file>`
is allowed inside a commit script only; all other `git restore` variants
remain forbidden.
