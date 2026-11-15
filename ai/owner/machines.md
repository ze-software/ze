# Thomas's two development machines

Applies only on the two machines below. Anywhere else, ignore this file.

## Which machine you are on

Find out before your first command, from `uname -sm` and `hostname`:

| Machine | `uname -sm` | `hostname` | Checkout |
|---------|-------------|------------|----------|
| Thomas's Mac | `Darwin arm64` | `thomasmangin-…` | `/Users/thomas/Code/github.com/ze-software/ze/main` |
| The server | `Linux x86_64` | `claude` (a KVM guest) | `/home/thomas/Code/github.com/ze-software/ze/main` |

If neither row matches, you are not on Thomas's machines and the rest of this
file does not apply.

## Work that is also tested on the other machine

Develop it in a worktree of its own on the machine you are on, never in the
shared checkout, which other sessions keep dirty. Open the worktree on a new
branch named after the work, under `.claude/worktrees/<name>` in the checkout,
and commit there through `./le commit create`. When the work is ready, test
the worktree on this machine first. Then carry its commits to the other
machine through git and test them there in a worktree of their own, detached
at those commits, so no branch is created there and its shared checkout is
neither pulled into nor tested. The branch reaches `main` only when Thomas
lands it with `git rebase <branch>`.

## What you may do on the other machine

Two things, without asking, and nothing else:

1. Bring your worktree's commits to it through git, and open a detached
   worktree of them under `.claude/worktrees/<name>`.
2. Run the tests of the feature you are working on in that worktree, then
   remove the worktree you opened.

Everything else on the other machine waits for Thomas's authorisation: a build
for its own sake, a lab, a cleanup, reading or listing files, changing its
config. The connection check, the Rex session a test runs in, reading that
test's output and the `./le job report` read before a test starts are part of
running the test.

The other machine's shared checkout changes only through its own sessions and
git. Never edit, copy or patch a file there, never pull into it for your work,
and never stash, discard or overwrite anything in it.

Where the commits come from: the Mac reaches the server, so the Mac fetches
the server's worktree branch directly over SSH. The server cannot reach the
Mac's git, so the Mac's commits travel to the server as a git bundle, which
moves commits between development machines and publishes nothing. Pushing to
origin still needs Thomas's order.

The repository's rules for heavy work hold on both machines. A session runs one
CPU-intensive job at a time, whichever machine runs it, and the main thread
reads `./le job report` on the machine that will run a test before it starts
it.

## From the Mac to the server

Use SSH host `claude`. A non-interactive `ssh claude '<cmd>'` does not read the
profile, so put Go on PATH first. Ship the worktree branch as a bundle, in a
command of its own:

```sh
git bundle create ~/tmp/<name>.bundle <branch>
scp ~/tmp/<name>.bundle claude:tmp/<name>.bundle && rm ~/tmp/<name>.bundle
```

Then fetch it on the server, test it in a detached worktree, and remove what
you created:

```sh
ssh claude 'export PATH=/usr/local/go/bin:$HOME/go/bin:$PATH GOPATH=$HOME/go &&
  cd ~/Code/github.com/ze-software/ze/main &&
  git fetch ~/tmp/<name>.bundle <branch> &&
  git worktree add --detach .claude/worktrees/<name> FETCH_HEAD &&
  cd .claude/worktrees/<name> && ./le test unit <package>'
ssh claude 'cd ~/Code/github.com/ze-software/ze/main &&
  git worktree remove .claude/worktrees/<name> && rm ~/tmp/<name>.bundle'
```

## From the server to the Mac

Use Rex host `laptop`. The server cannot open a connection to the Mac. While the
Rex app runs on the Mac and its `rex-claude` helper holds the SSH tunnel to the
server, that tunnel carries the Mac's Rex server back as a socket on the server,
and `rex -S laptop` reaches it. Check that it answers first:

```sh
rex -S laptop whoami --json   # exits 4, "no Rex server is running at ...", when the tunnel is down
```

Report a down tunnel to Thomas and don't work around it: only he can bring it
up, with `rex-claude start` on the Mac. Never run the test on the server
instead.

A test runs in a pane of a Rex session on the Mac. Name the session after the
work, wait for the run, read the pane, then close the session. A pane's shell
does not read the profile either, so take Go from Homebrew's directory. The Mac
fetches the server's worktree branch over SSH into a detached worktree, and
removes it after the run:

```sh
X='rex -S laptop'
$X new ze-unit-bgp --json
$X run -s ze-unit-bgp --label tests --keep-open --focus=false --json --wait -- sh -c \
  'export PATH=/opt/homebrew/bin:$HOME/go/bin:$PATH &&
   cd /Users/thomas/Code/github.com/ze-software/ze/main &&
   git fetch claude:Code/github.com/ze-software/ze/main <branch> &&
   git worktree add --detach .claude/worktrees/<name> FETCH_HEAD &&
   (cd .claude/worktrees/<name> && ./le test unit <package>);
   status=$?; git worktree remove .claude/worktrees/<name>; exit $status'
$X capture -s ze-unit-bgp -b <block_ids[0] from run> --trim
$X kill ze-unit-bgp
```

`run --wait` exits with the command's status. A pane keeps about 64 MiB of
output, so a long run writes its log to a file and you read that file.
`rex __internal skill` prints the full Rex guide.
