# Authorization

Ze uses profile-based command authorization (RBAC) to control which
commands each user can execute. Profiles are defined in the daemon
configuration alongside user accounts. Each user is assigned one or more
profiles; each profile contains ordered rules that allow or deny
commands.

Authorization is separate from authentication. Authentication (covered
in `authentication.md`) decides *who the user is*. Authorization decides
*what they can do*. Named `login-service` sets under `system authorization`
sit beside `profile` and decide *which login doors* an account can enter.

## Concepts

### Profiles

A profile is a named set of authorization rules. Each profile has two
sections:

| Section | Governs | ReadOnly flag |
|---------|---------|---------------|
| `run` | Operational (read-only) commands: the read verbs `show`, `monitor` and `resolve`, plus the noun-first roots `validate`, `help`, `system`, `plugin` and `rib` | `true` |
| `edit` | Configuration (write) commands: `clear`, `set`, `delete`, `request`, `commit`, `update`, `create`, `send`, `debug`, `cache`, and everything else | `false` |

The dispatcher classifies commands automatically by their leading verb.
You do not choose which section a command falls into.

The read verbs are not listed twice. `IsReadOnlyPath` asks
`command.IsReadOnlyVerb`, which reads the verb's role out of `command.Verbs`,
so a verb whose role changes there changes the section its commands land in.
The five noun-first roots predate the verb-first grammar and are the only names
this decision holds of its own.
<!-- source: internal/component/command/verbs.go -- Verbs, verbRole, IsReadOnlyVerb -->
<!-- source: internal/component/plugin/server/command.go -- IsReadOnlyPath, legacyReadRoots -->

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

### What happens when a backend will not build

The rules above assume a chain. Where a BACKEND failed to build, it is dropped
and the chain composes from the rest.

These rules then still apply. The first authorizer any surviving backend
contributed decides. Where the local backend survived, that is the local RBAC
store this page describes.

A TACACS+ server declared with no shared secret puts a daemon in that state. So
does any other error while the chain is built.

Where EVERY backend failed there is no chain, and then **every command is
refused.** Nothing was installed to consult, and no session exists to ask: ssh
is not started either, because nothing can authenticate.

<!-- source: internal/component/aaa/types.go -- backendRegistry.Build -->
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

For operational commands entered at the CLI (the common case), the
authorization string is exactly what the user typed: `show bgp peer list`,
`clear counters`, etc. Configuration mode is different, and the next section
covers it.
<!-- source: internal/component/plugin/server/command.go -- isAuthorized -->

**Every command now starts with its verb, so profile entries must too.** The bare
forms that once worked without a verb (`bgp summary`, `system memory`, `daemon
reload`) were removed from the command tree. Prefix matching is anchored at the
start of the typed string, so an entry whose `match` is `bgp` no longer covers
`show bgp`. Review any denylist entry keyed on a bare form: it silently
stops covering the command it was written for. Write `show bgp`, `show system`,
and `request reload` instead.
<!-- source: internal/component/cli/client/verb_tree.go -- BuildVerbCommandTree, verbContextPath -->
<!-- source: internal/plugins/signal/main.go -- Commands table ExecCommand column -->

### Configuration mode

A configuration change is never authorized by what the user typed. Each
change is authorized by a permission string from a fixed table, judged in the
`edit` section. The SSH session editor, the web editor, the web terminal and
the REST and gRPC config sessions use the same strings, so one profile entry
governs a change on every surface.

| Editor verb | Permission string |
|-------------|-------------------|
| `set`, `activate` | `config set` |
| `delete`, `deactivate` | `config delete` |
| `commit` | `config commit` |
| `discard`, `disconnect` | `config discard` |
| `rollback` | `config rollback` |
| `rename` | `config rename` |
| `copy`, `insert` | `config add` |
| `save`, `load` | `config save` |
<!-- source: internal/component/cli/config_permission.go -- ConfigPermission -->

`configure` is never authorized: it only switches the CLI into configuration
mode, and an entry that matches `configure` matches nothing Ze checks.
Navigation and display (`edit`, `up`, `top`, `show`, `who`, `option`, `help`)
are not checked, so a read-only user can browse the configuration. `run
<command>` is authorized as the operational command it runs. `top <command>`
is judged as the command it runs, so `top set ...` needs `config set`.

When the profiles load, Ze logs the warning `authz match entry does not match
any known command` for each prefix entry that matches neither a command nor
one of these permission strings, such as a misspelled `confg set`. An entry
`config`, `config commit` or `config commit client-edge.conf` is known and
logs nothing.
<!-- source: internal/component/config/infra/authz.go -- unknownMatchEntries, matchesConfigPermission -->

An SSH session that edits another stored configuration than the daemon's own,
such as a hub editing a client's configuration with `ze config edit
client-edge.conf`, adds the configuration's name to the string:
`config commit client-edge.conf`. Prefix matching keeps an entry `config` or
`config commit` covering it, and an entry that names the file restricts that
file alone. This profile lets its user change the daemon's configuration and
edit `client-edge.conf`, but refuses a commit of `client-edge.conf`:

```
edit {
    default-action allow
    entry 10 {
        action deny
        match "config commit client-edge.conf"
    }
}
```
<!-- source: cmd/ze/hub/session_factory.go -- buildSessionModelFactory -->

A refused change prints `command restricted by access control:` followed by
the permission string, then `ask an administrator for a profile whose edit
section allows <permission>`. Nothing changes:
not the candidate, not the stored configuration, not the running one.

An SSH session with no authorizer refuses every change. A daemon with no
`system.authorization` stays permissive (see "Fail-closed behaviour" below).
The console of `ze start --cli`, and `ze config edit` in file mode (`-f`) or
backup mode (`--backup`), authenticate nobody and keep every verb.
`ze config edit` in session mode connects to the daemon over SSH, so it is
authorized like any SSH session.
<!-- source: cmd/ze/hub/service_ssh.go -- sshSessionPolicy -->
<!-- source: internal/component/config/cli/cmd_edit.go -- runStoredEditor, runBackupModel -->

### Web pages that a command grants

Some web pages act on the device without running a command. They are
judged by the Ze command that does the same thing, in the section that
command lands in. The gokrazy management UI at `/gokrazy/` is one: its update
handlers need `update system firmware apply` and its reboot handler
`request reboot`, both judged in `edit`. The table of every page is in
[appliance.md](appliance.md), "The gokrazy management UI".
<!-- source: internal/component/gokrazy/gate.go -- forwardedPages -->

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

### Login-service sets

Define named login-service sets under `system authorization login-service` beside `profile`. Each set lists the registered login doors (`ssh`, `web`, `rest`, `grpc`, `mcp`) that an account can authenticate through.

```
system {
    authorization {
        login-service ops-ssh-web {
            service [ ssh web ]
        }
        login-service api-only {
            service [ rest grpc ]
        }
        login-service no-login {
        }
    }
    authentication {
        user alice {
            password "$2a$10$..."
            profile [ admin ]
            login-service ops-ssh-web
        }
        user grafana {
            password "$2a$10$..."
            profile [ monitoring ]
            login-service api-only
        }
        tacacs-profile 15 {
            profile [ admin ]
            login-service ops-ssh-web
        }
        radius {
            server 10.0.0.1 {
                port 1812
                key "$9$encrypted-key"
            }
            default-profile [ read-only ]
            login-service ops-ssh-web
        }
    }
}
```
<!-- source: internal/component/authz/yang/ze-authz-conf.yang -- system.authorization.login-service, system.authentication.user.login-service -->
<!-- source: internal/component/tacacs/yang/ze-tacacs-conf.yang -- system.authentication.tacacs-profile.login-service -->
<!-- source: internal/component/radius/yang/ze-radius-conf.yang -- system.authentication.radius.login-service -->

The `login-service` leaf on `user`, `tacacs-profile`, and `radius` is a YANG leafref to `system authorization login-service` with `require-instance true`. Commit and `ze config validate` reject any reference to an undefined set and list the defined sets. Each `service` value is validated against the login services registered in this build.
<!-- source: internal/component/config/validators.go -- LoginServiceValidator -->
<!-- source: internal/component/config/loginservice/registry.go -- Names, Registered -->

Omitting `login-service` on an account or remote mapping admits that account on every door. Assigning a `login-service` set with an empty `service` list (`no-login` above) denies every checked door. Refer to [Authentication](authentication.md#login-service-authorization) for surface coverage, reload behavior, and the recovery account rule.
<!-- source: internal/component/aaa/login_profiles.go -- CheckLoginService, CheckLoginSet -->

## Built-in profiles

Ze provides two built-in profiles that can be referenced without
defining them in the configuration. If you define a profile with the
same name, your definition overrides the built-in.

### `admin`

Allows everything. Both `run` and `edit` default to `allow` with no
entries.

### `read-only`

Allows all operational commands except dangerous ones. Denies all
configuration commands. The `edit` section judges every command whose verb is
not a read verb, so the store maintenance commands `delete data key`,
`update data key`, `update data config` and `request data repair` are denied
too, before their handler runs, while `show data check` is allowed. Every
configuration-mode change is refused too, because its permission string is
judged in `edit`; browsing the configuration is not.
<!-- source: internal/component/plugin/server/command.go -- IsReadOnlyPath -->
<!-- source: internal/test/fixture/storage_live_authz_fixture.go -- storageLiveAuthzReadOnly -->

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

### Explicit-grant commands (default-deny)

Sensitive commands that expose secret material register `RequireExplicitAuthz`
and `NotReadOnly`. Today `show pki certificate name <name> bundle pem confirm`
uses this gate.

An explicit-grant command always defaults to deny. It refuses unconfigured
RBAC, `default-action allow` (including `admin`), `read-only`, and the recovery
account. It also refuses shared or unauthenticated API callers, internal RPC,
and plugin endpoints without an explicit compiled bus grant. Broad prefixes
(such as `show` or `show pki`) and wildcard regexes (such as `.*`) do not grant
access.

To allow private-key bundle export, add an explicit `allow` entry in the
profile's `edit` section. Write either the canonical command path (for all
certificates) or the certificate-scoped form (for one certificate):

```
system {
    authorization {
        profile pki-exporter {
            run {
                default-action allow
            }
            edit {
                default-action deny
                entry 10 {
                    action allow
                    match "show pki certificate name bundle pem"
                }
            }
        }
        profile pki-dev1-only {
            run {
                default-action allow
            }
            edit {
                default-action deny
                entry 10 {
                    action allow
                    match "show pki certificate name dev-1 bundle pem"
                }
            }
        }
    }
}
```

A `deny` entry with a lower sequence number overrides a later `allow` entry.
When TACACS+ command authorization is enabled, Ze sends the command tokens over
RFC 8907. If the server is unreachable and strict fallback is off, Ze falls back
only to local explicit-grant evaluation.

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
| Referencing a profile name that does not exist | Commit and `ze config validate` refuse it, naming the profiles that exist: a user's `profile` and a `tacacs-profile`'s `profile` are leafrefs to `system authorization profile` | Define the profile in the same commit, or name one that exists |
| Referencing an undefined `login-service` set or unregistered service name | Commit and `ze config validate` refuse undefined set references through the leafref. They also reject unregistered or compiled-out service names and list offered services | Define the set under `system authorization login-service` and use only services offered by this build |

## Reference

| Symbol | Location |
|--------|----------|
| YANG schema | `internal/component/authz/yang/ze-authz-conf.yang` |
| Profile evaluation | `internal/component/authz/authz.go` |
| Store (thread-safe registry) | `internal/component/authz/authz.go` |
| StoreAuthorizer adapter | `internal/component/authz/register.go` |
| Canonical command builder | `internal/component/aaa/command_args.go` (`CanonicalCommand`, `ExplicitAuthorizer`, `AuthorizeExplicit`) |
| Read-only verb classification | `internal/component/plugin/server/command.go` (`IsReadOnlyPath`) |
| Dispatcher authorization check | `internal/component/plugin/server/command.go` (`isAuthorized`, `isAuthorizedExplicit`, `isAuthorizedCommandArgs`) |
| Configuration-mode permission table | `internal/component/cli/config_permission.go` (`ConfigPermission`) |
| SSH editor authorizer binding | `cmd/ze/hub/session_factory.go` (`buildSessionModelFactory`) |
| Built-in admin profile | `internal/component/authz/authz.go` (`builtinAdminProfile`) |
| Built-in read-only profile | `internal/component/authz/authz.go` (`builtinReadOnlyProfile`) |
| Login-service registry | `internal/component/config/loginservice/registry.go` (`Register`, `MustRegister`, `Names`, `Registered`, `Lookup`) |
| Login-service validator | `internal/component/config/validators.go` (`LoginServiceValidator`) |
| Login-service gate | `internal/component/aaa/login_profiles.go` (`CheckLoginService`, `CheckLoginSet`, `SetAcceptedLoginSets`) |

<!-- source: internal/component/authz/authz.go -- Entry.matches, Section.evaluate, Store.AuthorizeExplicit -->
<!-- source: internal/component/authz/register.go -- StoreAuthorizer -->
<!-- source: internal/component/aaa/command_args.go -- CanonicalCommand, ExplicitAuthorizer, AuthorizeExplicit -->
<!-- source: internal/component/plugin/server/command.go -- IsReadOnlyPath, isAuthorized, isAuthorizedExplicit -->
