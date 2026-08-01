# negative-test-must-fail-for-its-reason

Two authorization bypasses shipped behind three "passing" negative tests and one
demo recording. Every one of them asserted that a command FAILED, and every one
of them would have passed with authorization entirely switched off, because the
command they probed with was `configure` -- which Ze does not have as a daemon
command.

## The bug the tests hid

`configure` is an interactive-CLI mode switch (`internal/component/cli/model.go`
`cmdConfigure`, dispatched in `model_keys.go` when `m.mode == ModeOperational`).
It is never registered on the dispatcher, so `ze cli -c "configure"` returns
`unknown command` for **every** user -- denied, allowed, or with no authorizer at
all. A test that runs it and asserts non-zero exit measures nothing.

Underneath, two real bypasses were live:

1. `cmd/ze/hub/main.go` starts SSH on a standalone path when the config has no
   `bgp {}` block, and never called `SetAuthorizer`. `Dispatcher.isAuthorized`
   returns true when `d.authorizer == nil` (`plugin/server/command.go`), so an
   ssh-only box (gokrazy appliance, `environment{}`-only config) accepted
   `system.authorization` profiles and enforced none of them. The bgp path wires
   it from the reactor post-start hook (`infra_setup.go`), which never runs
   without bgp.
2. Profiles resolved at authentication were only LOGGED (`ssh.go`, "SSH auth
   success" ... `profiles=[read-only]`) and then dropped. `authz.Store.Authorize`
   re-derives assignments from `system.authentication.user[*].profile`
   (`bgp/config/loader.go`); a TACACS+ user has no such block, so the lookup
   missed, `hasUsers` was false, and it fell through to `BuiltinAdminProfile` --
   allow all. `tacacs-profile <lvl> { profile [...] }` governed nothing.

## The rule

**A negative test must fail for the reason it claims.** Assert the reason, not
just the failure. `exit != 0` is not evidence of authorization; `Contains(output,
"command restricted by access control")` is.

Two checks before trusting any deny-side test:

- **Probe with something that exists.** If the command is unregistered, the test
  passes on the unknown-command path and never reaches the code under test. Here:
  `clear interface counters` and `request quiesce` are real; `configure` is not.
- **Break the mechanism and watch it go red.** Every fix in this change was
  confirmed by disabling it (`&& false`) and re-running. `rbac-ssh-only-enforced`
  then showed the denied command *executing* ("iface: no backend loaded"), which
  is what proved the bypass rather than a story about it.

## Where the false confidence came from

The failing assertion was never written, and prose filled the gap:

- `tacacs-readonly.ci`'s header described a step 4 (`ze cli -c "configure"` fails)
  that **the script did not contain**. Only the allow side ran. Its own
  `PREVENTS:` line named the exact bug that was live: "priv-lvl mapping table
  parsed but never consulted, every TACACS+ user getting admin by accident".
- `command_test.go` used an *unregistered* command as a proxy for "the plugin
  path" (`TestDispatcherAuthorizationAppliesToUnknownCommands`). Nothing can
  execute on that path, so there was no bypass to prevent. It now registers a
  real plugin command, which is what AC-4 meant.
- The demo transcript explained the symptom as a feature: "Ze removes the denied
  configuration command from that user's command tree". No such mechanism exists
  in the code. A rationalization of unexpected output became documentation.

When a test's comment and its assertions disagree, the comment is what people
read and the assertions are what runs. Grep a `PREVENTS:` clause against the
assertion list occasionally: if no assertion could fail for that reason, the
clause is a wish.

## Traps for the next agent

- **`run` vs `edit` is operational vs write, not readonly vs mutating.** `clear`
  is operational, so it is evaluated against `run` -- a profile with
  `run { default-action allow }` permits it. That is why `BuiltinReadOnlyProfile`
  (`authz.go`) denies `restart`/`kill`/`clear`/`debug` *explicitly in run*. To
  exercise `edit { default-action deny }` you need a write (`request quiesce`).
  Picking the wrong section is how a deny-side test accidentally proves nothing.
- **Authorization resolves the command first.** Since this change, a command that
  matches nothing returns `ErrUnknownCommand` before any authorizer runs, so no
  AUTHOR REQUEST is sent for it and TACACS+ never sees typos. A per-command
  TACACS+ test must therefore use a registered command.
- **A missing log line is not proof of a missing call.** `hub.infra` logs at INFO
  and did not appear, which looked like "the hook never ran". The dispatcher's
  own DEBUG line (`dispatchPlugin: no match`) is what actually proved execution
  had passed the authorization check. Prove control flow with a log on the path
  you care about, or with a behavioral probe, not with silence elsewhere.

## The fix for a fail-open can be a fail-open (found in review, same change)

Routing login-resolved profiles into `Store.Authorize` set `hasAssignment = true`
for whatever names authentication returned. But `ValidateAuthzConfig`
(`bgp/config/loader.go`) validates `user[*].profile` references and **not**
`tacacs-profile <N> { profile [...] }` ones, so a typo'd mapping loads fine. An
unknown name then survived to the profile loop, which skips names it cannot
resolve (`p == nil`), left `firstDefault` nil, and hit "all referenced profiles
were missing -> admin default" (`authz.go`). A typo in `tacacs-profile` would
have authorized that user as **admin** -- strictly worse than the bug being
fixed, since the old code denied them via the `hasUsers` branch.

Fixed by only accepting login-resolved names the store actually defines, so an
unresolvable mapping falls to the fail-closed branch instead of past it.

Two generalisations worth carrying:

- **When you widen an input to a decision, check every downstream branch that
  input can now reach.** The new value was legal at the entry point and toxic
  three branches later. Grep the function's tail for its fall-throughs before
  assuming a new caller is safe.
- **An "unknown -> permissive default" fall-through is a landmine.** Ze has two
  live ones in `Store.Authorize`: unassigned user with no local users, and all
  referenced profiles missing. Both read as reasonable locally and both mean
  allow-all. Treat any `if nothing matched { admin }` as a finding, not a
  default.

## Files

None recorded.
