# Authorization

Ze uses profile-based command authorization (RBAC) to control which
commands each user can execute. Profiles are defined in the daemon
configuration alongside user accounts. Each user is assigned one or more
profiles; each profile contains ordered rules that allow or deny
commands.

Authorization is separate from authentication. Authentication (covered
in `authentication.md`) decides *who the user is*. Authorization decides
*what they can do*.

## Concepts

### Profiles

A profile is a named set of authorization rules. Each profile has two
sections:

| Section | Governs | ReadOnly flag |
|---------|---------|---------------|
| `run` | Operational (read-only) commands: `show`, `monitor`, `resolve`, `validate`, `help`, `event`, `subscribe` | `true` |
| `edit` | Configuration (write) commands: `clear`, `set`, `request`, `commit`, `configure`, and everything else | `false` |

The dispatcher classifies commands automatically by their leading verb.
You do not choose which section a command falls into.

### Entries

Each section contains an ordered list of numbered entries. Each entry
has:

| Field | Type | Description |
|-------|------|-------------|
| `number` | uint32 | Sequence number controlling evaluation order (lowest first) |
| `action` | `allow` or `deny` | What to do when this entry matches |
| `match` | string | The pattern to match against the command |
| `regex` | boolean | If `true`, `match` is a Go regular expression. If absent or `false`, `match` is a prefix. |

### Default action

Each section has a `default-action` (`allow` or `deny`). When no entry
matches the command, the default action applies.

### Evaluation order

1. The dispatcher resolves the command and determines which section
   applies (run or edit).
2. Entries are evaluated in ascending `number` order.
3. The **first matching entry wins**. Its action is the verdict.
4. If no entry matches, `default-action` applies.

When a user has multiple profiles, the profiles are evaluated in
assignment order. The first profile with a matching entry wins. If no
profile has a matching entry, the first profile's `default-action`
applies.

### What happens when the AAA chain will not build

The rules above assume the daemon built its AAA chain. Where the build FAILED,
none of them applies: **every command is refused.** With no chain installed
there is no policy to consult, and the profiles this page describes are reached
only through a built chain.

A TACACS+ server declared with no shared secret puts the daemon in that state.
So does any other error while the chain is built.

Authentication still falls back to the local accounts, so an operator can log in
and see the failure. The session runs nothing, and repair goes through the
console. `docs/guide/authentication.md` carries the login half.

A fallback to these profiles was tried on 2026-09-04 and reverted the same day.
It made a box that declares no profile allow every command while its chain was
broken. Falling back to a policy means falling back to what it says, and an
absent one says allow.

<!-- source: cmd/ze/hub/aaa_lifecycle.go -- liveAAABundleAuthorizer -->

## Matching rules

### Prefix matching (default)

When `regex` is absent or `false`, the `match` string is compared as a
**case-insensitive prefix with word-boundary enforcement**.

A match string of `"show bgp peer"` matches:

- `show bgp peer` (exact)
- `show bgp peer list` (prefix + space boundary)
- `show bgp peer list 10.0.0.1` (prefix + deeper arguments)
- `SHOW BGP PEER list` (case insensitive)

It does **not** match:

- `show bgp peering` (no word boundary after "peer")
- `show bgp` (match string is longer than the command)
- `show bgp peers` ("peers" is not "peer" + space)

**The key insight: prefix matching authorizes the command stem, not the
arguments.** When you allow `"show bgp peer"`, all arguments that follow
(peer names, IP addresses, keywords like `detail` or `routes`) are
implicitly permitted. This is the standard network OS approach: authorize
the command, not every parameter.

### Regex matching

When `regex` is `true`, the `match` string is a Go regular expression
(the `regexp` package syntax). The regex is tested with `MatchString`,
meaning it matches anywhere in the command unless you anchor it with `^`
and `$`.

Examples:

| Pattern | Matches | Does not match |
|---------|---------|----------------|
| `"^show bgp peer (list\|summary)( \|$)"` | `show bgp peer list`, `show bgp peer summary`, `show bgp peer list 10.0.0.1` | `show bgp peer detail`, `show bgp peer routes` |
| `"peer .* show"` | `peer 10.0.0.1 show`, `peer myrouter show` | `peer 10.0.0.1 list` |
| `"^restart$"` | `restart` | `restart now` |
| `"^clear (bgp\|counters)"` | `clear bgp peer1`, `clear counters all` | `clear l2tp session` |

Regex entries are compiled and validated at config load. An invalid
regex causes a config validation error.

**Use prefix matching when you can.** It is simpler, faster, and covers
most real-world policies. Use regex only when you need to restrict which
sub-commands are allowed under a common prefix, or when the command
structure requires non-prefix patterns.

## What string gets matched

The authorization check receives the **full command as typed by the
user**, including all arguments, before the dispatcher strips selectors
or validates argument types.

For peer-scoped commands dispatched through the typed command system,
the canonical authorization string prepends `"peer"` to the command
tokens (but not the peer selector value itself). For example, if a
plugin dispatches command `"show routes"` scoped to peer `"10.0.0.1"`,
the authorization string is `"peer show routes"`, not
`"peer 10.0.0.1 show routes"`.

For CLI-entered commands (the common case), the authorization string is exactly
what the user typed: `show bgp peer list`, `clear counters`,
`configure`, etc.

**Every command now starts with its verb, so profile entries must too.** The bare
forms that once worked without a verb (`bgp summary`, `system memory`, `daemon
reload`) were removed from the command tree. Prefix matching is anchored at the
start of the typed string, so an entry whose `match` is `bgp` no longer covers
`show bgp`. Review any denylist entry keyed on a bare form: it silently
stops covering the command it was written for. Write `show bgp`, `show system`,
and `request reload` instead.
<!-- source: internal/component/cli/client/verb_tree.go -- BuildVerbCommandTree, verbContextPath -->
<!-- source: internal/plugins/signal/main.go -- Commands table ExecCommand column -->

## Configuration

### Defining profiles

Profiles are defined under `system > authorization`:

```
system {
    authorization {
        profile admin {
            run {
                default-action allow
            }
            edit {
                default-action allow
            }
        }
        profile noc {
            run {
                default-action allow
                entry 10 {
                    action deny
                    match "restart"
                }
                entry 20 {
                    action deny
                    match "kill"
                }
                entry 30 {
                    action deny
                    match "clear"
                }
            }
            edit {
                default-action deny
            }
        }
        profile monitoring {
            run {
                default-action deny
                entry 10 {
                    action allow
                    match "show bgp peer list"
                }
                entry 20 {
                    action allow
                    match "show bgp"
                }
                entry 30 {
                    action allow
                    match "show bgp peer"
                    regex true
                }
            }
            edit {
                default-action deny
            }
        }
    }
}
```

### Assigning profiles to users

Profiles are assigned in the `system > authentication > user` block.
Each user has a `profile` leaf that takes a list of profile names:

```
system {
    authentication {
        user alice {
            password "$2a$10$..."
            profile [ admin ]
        }
        user bob {
            password "$2a$10$..."
            profile [ noc ]
        }
        user grafana {
            password "$2a$10$..."
            profile [ monitoring ]
        }
    }
}
```

### TACACS+ profile mapping

For TACACS+ authenticated users, profiles are assigned by privilege
level using `tacacs-profile`:

```
system {
    authentication {
        tacacs-profile 15 { profile [ admin ]; }
        tacacs-profile 1  { profile [ read-only ]; }
    }
}
```

See `tacacs.md` for full TACACS+ configuration.

## Built-in profiles

Ze provides two built-in profiles that can be referenced without
defining them in the configuration. If you define a profile with the
same name, your definition overrides the built-in.

### `admin`

Allows everything. Both `run` and `edit` default to `allow` with no
entries.

### `read-only`

Allows all operational commands except dangerous ones. Denies all
configuration commands.

| Section | Default | Entries |
|---------|---------|--------|
| `run` | `allow` | 10: deny `restart`, 20: deny `kill`, 30: deny `clear` |
| `edit` | `deny` | (none) |

## Common patterns

### Allow specific commands only (allowlist)

Set `default-action deny` and add `allow` entries for each permitted
command:

```
profile monitoring {
    run {
        default-action deny
        entry 10 {
            action allow
            match "show bgp peer list"
        }
        entry 20 {
            action allow
            match "show bgp"
        }
    }
    edit {
        default-action deny
    }
}
```

This user can run `show bgp peer list` (with or without arguments after
it, such as an IP address) and `show bgp`, but nothing else.

### Deny dangerous commands (denylist)

Set `default-action allow` and add `deny` entries for commands to block:

```
profile noc {
    run {
        default-action allow
        entry 10 {
            action deny
            match "restart"
        }
        entry 20 {
            action deny
            match "kill"
        }
    }
    edit {
        default-action deny
    }
}
```

### Allow a command family but deny a specific sub-command

Use entry ordering. Lower-numbered entries are evaluated first:

```
run {
    default-action deny
    entry 10 {
        action deny
        match "show bgp peer secret"
    }
    entry 20 {
        action allow
        match "show bgp peer"
    }
}
```

Entry 10 blocks `show bgp peer secret` (and anything starting with
that prefix). Entry 20 allows all other `show bgp peer` commands. Order
matters: if entry 20 came first, it would match `show bgp peer secret`
before entry 10 could deny it.

### Restrict sub-commands with regex

When prefix matching is too broad, use regex to allow only specific
sub-commands:

```
run {
    default-action deny
    entry 10 {
        action allow
        match "^show bgp peer (list|summary)( |$)"
        regex true
    }
}
```

This allows `show bgp peer list` and `show bgp peer summary` (with or
without trailing arguments) but denies `show bgp peer detail` or
`show bgp peer routes`.

### Multiple profiles

A user can be assigned multiple profiles. They are evaluated in list
order; the first profile with a matching entry wins:

```
system {
    authentication {
        user ops {
            password "$2a$10$..."
            profile [ restricted ops-extra ]
        }
    }
    authorization {
        profile restricted {
            run {
                default-action deny
                entry 10 {
                    action allow
                    match "show bgp peer"
                }
            }
            edit { default-action deny; }
        }
        profile ops-extra {
            run {
                default-action deny
                entry 10 {
                    action deny
                    match "kill"
                }
            }
            edit { default-action deny; }
        }
    }
}
```

For user `ops`, the command `show bgp peer list` matches entry 10 in
`restricted` (allow). The command `kill` matches entry 10 in
`ops-extra` (deny). An unmatched command like `summary` falls to the
first profile's default (deny).

## Fail-closed behaviour

When user assignments are configured (at least one user has a profile
list), unassigned users are **denied all commands**. This prevents
unassigned users from silently becoming admin-equivalent.

When no user assignments exist at all (no profiles configured),
all access is allowed. This preserves backward compatibility for
deployments that do not use authorization.

| State | Empty username | Known user, no profile | Known user, profile assigned |
|-------|----------------|----------------------|------------------------------|
| No assignments configured | Allow all | Allow all | N/A |
| Assignments configured | Deny | Deny | Evaluate profiles |

## Validation

`ze config validate` checks that:

- Every profile has a non-empty name
- Every entry has a non-empty `match` string
- Regex entries compile without errors

```
$ ze config validate myconfig.conf
configuration valid
```

An invalid regex pattern causes a validation error:

```
$ ze config validate bad.conf
profile "test" run: entry 10: invalid regex "[broken": error parsing regexp: missing closing ]: `[broken`
```

## Things that do NOT work

| Attempt | Why it fails | Use this instead |
|---------|-------------|------------------|
| Matching on IP addresses with prefix mode | Prefix matches the command stem, not individual tokens in the middle | Use `regex true` with a pattern like `"^show bgp peer list 10\\.0\\.0\\.1$"` |
| Using `match "show"` to allow all show commands and denying specific ones with higher entry numbers | The first match wins; entry 10 `allow "show"` matches before entry 20 `deny "show bgp secret"` | Put the more specific `deny` entry at a lower number than the broader `allow` |
| Expecting regex to be anchored by default | Go regex `MatchString` matches anywhere in the string; `"restart"` matches `"show restart status"` | Anchor explicitly: `"^restart( \|$)"` |
| Referencing a profile name that does not exist | The profile is skipped silently; if all assigned profiles are missing, the user gets admin defaults | Define the profile, or use one of the built-in names (`admin`, `read-only`) |

## Reference

| Symbol | Location |
|--------|----------|
| YANG schema | `internal/component/authz/yang/ze-authz-conf.yang` |
| Profile evaluation | `internal/component/authz/authz.go` |
| Store (thread-safe registry) | `internal/component/authz/authz.go` |
| StoreAuthorizer adapter | `internal/component/authz/register.go` |
| Canonical command builder | `internal/component/aaa/command_args.go` |
| Read-only verb classification | `internal/component/plugin/server/command.go` (`IsReadOnlyPath`) |
| Dispatcher authorization check | `internal/component/plugin/server/command.go` (`isAuthorized`, `isAuthorizedCommandArgs`) |
| Built-in admin profile | `internal/component/authz/authz.go` (`builtinAdminProfile`) |
| Built-in read-only profile | `internal/component/authz/authz.go` (`builtinReadOnlyProfile`) |

<!-- source: internal/component/authz/authz.go -- Entry.matches, Section.evaluate -->
<!-- source: internal/component/authz/register.go -- StoreAuthorizer -->
<!-- source: internal/component/aaa/command_args.go -- CanonicalCommand -->
<!-- source: internal/component/plugin/server/command.go -- IsReadOnlyPath, isAuthorized -->
