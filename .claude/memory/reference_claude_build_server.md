---
name: claude-build-server
description: "ssh host `claude` (10.12.100.120) is an idle x86_64 32-core Linux box with a ze checkout, usable for amd64 kernel builds; non-interactive ssh lacks Go on PATH"
metadata:
  node_type: memory
  type: reference
  originSessionId: 8b43ed0f-1eb8-486d-ab5d-df74d0e3ec23
  modified: 2026-10-31T02:14:33.579Z
---

- `ssh claude` reaches 10.12.100.120: x86_64, 32 cores, 31G RAM, KVM, Ubuntu QEMU 8.2.2. Checkout at `~/Code/github.com/ze-software/ze/main` (remote `gitlab`, not GitHub).
- Non-interactive `ssh claude cmd` has no Go on PATH: export `PATH=$PATH:/usr/local/go/bin:$HOME/go/bin` or `le` fails to rebuild and silently keeps an old build.
- Root disk had ~17G free (2026-10-30): the Docker kernel builder refuses (needs 40G), so use `--builder qemu`.
- Sync unpushed Mac commits with a git bundle (`git bundle create ... <server-head>..main`, scp, `git pull --rebase --no-autostash <bundle> main`).
- Quote remote paths: an unquoted `~` expands on the Mac. `sudo` there is passwordless.
- Run long jobs detached (`setsid nohup ... & disown`) so a closed laptop does not kill them.
