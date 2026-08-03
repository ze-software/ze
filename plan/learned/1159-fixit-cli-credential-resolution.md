# 1159 -- Fixit: CLI Credential Resolution

## Context

The request was to restrict `--user` to root: only root could name a user, everyone
else would be forced to their OS login name. Research killed the premise. `ze cli
--user` is only SSH: the flag picks the username in the handshake and the daemon
authenticates it (`internal/component/ssh/ssh.go`), so a client-side gate
guards nothing that `ssh alice@host` does not walk around. The user then raised role
accounts -- an OS user `thomas` may need to log into kit as `noc` -- which killed the
login-name rule too, and would have broken the project's own install guide
(`docs/guide/ubuntu-build-install.md` documents `ze cli --user noc`). Nothing was
implemented from the original request. The investigation instead uncovered two real
defects in the same over-constrained resolver, and those became the spec.

## Decisions

- **Left `--user` completely unrestricted, over root-gating it.** The daemon is the
  identity authority; the client only proposes. A client-side rule would have locked
  out the role-account operator it was meant to protect, for zero security gain.
- **Made prompting an explicit CALLER policy (`allowPrompt`), over inferring it from
  the tty.** Tab completion runs with stdin on the operator's terminal, so a tty check
  answers "prompting is fine" and hangs the shell. The tty state cannot distinguish
  "a human is waiting" from "a shell is completing".
- **Chose a targeted `NoPrompt` variant over inverting the default** (making resolution
  never prompt and having ~10 interactive callers prompt explicitly). Inverting is
  safe-by-default and structurally prevents recurrence, but costs a 10-site diff for a
  bug with one victim. The footgun remains: prompting is still the default, deterred
  only by a doc comment. Revisit if a second caller ever hangs.
- **Named it `NoPrompt`, not `NonInteractive`.** `client_test.go` already used
  "NonInteractive" to mean "stdin is not a tty" -- a different concept. Overloading it
  would have misled the next reader.
- **Classified store-open failures rather than treating all as fatal or all as absent.**
  Missing/unreadable -> continue without it; anything else (corruption) still surfaces.
  Falling back on ANY error would mask a real store bug as a confusing auth failure.
- **Rejected an OS-login-name default.** The requirement was that operators *supply*
  credentials; a guessed identity had no requester.

## Consequences

- The zefs store is now one credential source among several, not a precondition. A
  YANG user with `--user` + `ze.ssh.password` logs in with no readable store -- which
  is every non-installing user, since the store is one shared `0600` file under a
  binary-derived `/etc/ze` (`paths.go`, `store.go`).
- A store-less user loses the `meta/ssh/default` pointer and falls back to
  `127.0.0.1:2222`; a non-default daemon needs `ze.ssh.host`/`ze.ssh.port` or
  `--remote`. Documented, not guessed.
- Completion degrades silently (no peer completions) when a username is set without a
  password. Better than a hung shell, but it does not explain itself -- stderr would
  corrupt the completion display.
- `isStdinTTY` and `passwordPrompter` are now injectable vars, so any future prompt
  behavior is testable without a terminal.

## Gotchas

- **Credential resolution was never a pure lookup** -- it could block on an interactive
  prompt, and 11 callers treated it as cheap. Any change that makes such a function
  succeed *more often* widens the blast radius of its side effects. Ask "who is allowed
  to block?" before "what is the default value?".
- **The hang was reachable by following the docs.** `authentication.md` said
  `export ZE_SSH_USERNAME=alice` / `export ZE_SSH_PASSWORD=... # or use a key-locked
  secret store` -- taking the comment's advice and omitting the password produced the
  hang. The docs were the bug report nobody filed.
- **A pty test is the only way to prove this class of bug.** `.ci` and `go test` both
  run without a tty, so `isStdinTTY()` is false and the prompt path is never taken --
  a `.ci` would pass vacuously. `internal/component/config/system/console_integration_linux_test.go`
  is the precedent; `github.com/creack/pty` is already a direct dependency.
  Replace `os.Stdin` with the tty side, run the call in a goroutine, fail on a deadline.
- **`chmod 000` cannot deny root**, so a permission-denied test passes vacuously
  wherever the suite runs as root. Prefer an absent file (`fs.ErrNotExist`) to reach the
  same classification branch unconditionally, and guard the genuine permission case on
  `os.Geteuid() != 0`.
- **The `cmd/ze/internal/ssh/client` facade is dead** -- zero Go importers; only a
  `// Related:` comment in `pkg/zefs/store.go` mentions it. The spec said to
  re-export there "so the facade stays complete"; that added a dead symbol and the
  /ze-review wiring step caught it. Check a facade has importers before extending it.
- **The `nilnil` linter rejects `return nil, nil`** for "absent but fine". Use a
  sentinel (`errStoreUnavailable`) and `errors.Is` at the call site.
- **A pretool hook blocks `fmt.Fprint(os.Stderr, ...)` outside `cmd/`**, which fires
  even when merely re-indenting existing prompt code into a `var` form. Add the seam as
  a separate `var x = existingFunc` indirection instead of touching those lines.
- `zefs` preserves the syscall error through `%w` at every hop (`mmap_unix.go` ->
  `store.go` -> `store.go`), so `errors.Is(err, fs.ErrPermission)` works
  through `zefs.Open`, while `decode` corruption errors correctly do not match.
- **Making resolution succeed more often moved a command out of the offline fallback.**
  `ze cli --user alice -c "show crashes"` with no store used to answer from local data:
  not by design, but because credential resolution failed and
  `internal/component/cli/client/main.go` routes a credential error to the
  in-process fallback. With `--user` now working, the command reaches the daemon and
  prompts instead. That is the better answer (the operator named a user; a daemon reply
  beats a local guess), and the no-username case still errors, which is what keeps the
  fallback reachable for a plain `ze cli -c "show crashes"`. Pinned by
  `TestReadCredentialsNoStoreWithUserPrompts` so nobody "fixes" the prompt away.
  General shape: when a function starts succeeding where it used to fail, audit what was
  living off the failure.
- **Test seams that are package-level vars need a stated no-parallel rule.** `isStdinTTY`
  and `passwordPrompter` (and `os.Stdin` in the completion pty test) are swapped by tests;
  two parallel tests would race on the assignment and silently observe each other's stub.
  In practice `t.Setenv` already forbids it -- the testing package refuses `t.Parallel`
  for any test using it (`$GOROOT/src/testing/testing.go`, `parallelConflict`) -- but
  that guard is incidental, so the constraint is written on the seams and on
  `stubPromptPolicy`.
- **`ze-validate` scopes to the working-tree diff, not the whole repo.** It reported
  `TrimErrorPrefix` (exported, no cross-package caller, pre-existing) only while
  `client.go` was uncommitted, and went quiet the moment it was committed -- the issue
  was unchanged. Do not read its silence as absence; an unwired export can sit in HEAD
  indefinitely. It was unexported to `trimErrorPrefix` here.

## Files

- Modified: `internal/core/ssh/client/{client,client_test}.go` -- `allowPrompt` policy,
  `readCredentials`, `openStoreIfReadable`, `storedUsername`, injectable seams,
  `TrimErrorPrefix` -> `trimErrorPrefix`
- Modified: `internal/plugins/completion/{peers,peers_test}.go` -- `LoadCredentialsNoPrompt`, pty test
- Modified: `cmd/ze/internal/ssh/client/client.go` -- comment only (dead facade, no re-export)
- Modified: `docs/guide/{authentication,command-reference}.md`
- New: `test/plugin/cli-credential-resolution.ci`
