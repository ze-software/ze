# Allowed System Commands

The register of external binaries Ze code is permitted to run, and the only
authority for running one.

Ze is a network operating system. It runs on an appliance image that carries no
shell utilities, inside a network namespace that may carry nothing at all, and
on a router nobody restarts. A fork of a system tool is therefore three defects
waiting for the right environment: a dependency on a binary that is absent, a
second implementation's opinion about the kernel where Ze should hold its own,
and a diagnostic that reports nothing exactly when the environment is minimal
and the operator most needs an answer.

**The default is NO.** Use Go. Read the kernel interface directly: netlink for
links, routes, addresses and generic families, `/proc` and `/sys` through
`os.ReadFile`, `os.Stat` for a device node, and the `x/sys/unix` syscall
wrappers for the rest. Nearly every `ip`, `cat`, `ls` and `sysctl` invocation
has a native answer that is shorter, typed, and available everywhere Ze runs.

## The register

| Command | Where it runs | Authorised | Why no Go path exists |
|---------|---------------|------------|-----------------------|
| The operator's own plugin program, named by `plugin { external <name> { run ... } }` | `(*Process).startExternal` (`internal/component/plugin/process/process.go`) for a live start, and the declaration query reader (`internal/component/plugin/declarations.go`) for `show plugin declarations` | Thomas, 2026-09-08 | An external plugin IS a program Ze does not carry. Running it is the feature, so there is nothing for a native path to replace: no kernel interface answers "what does this operator's binary declare" |
| `systemctl` | The `ze install systemd` and `ze uninstall systemd` service handlers through `realServiceOps.run` (`internal/plugins/systemd/main.go`: `daemon-reload`, `enable`, `start`, `stop`, `disable` of `ze.service`), and the `systemd-service` doctor check's `systemctlShowStop` (`internal/plugins/systemd/doctor.go`: `systemctl show -p TimeoutStopUSec -p KillMode ze.service`). Scope: non-appliance hosts that run systemd only. The install and uninstall handlers refuse through `requireSystemd` when `systemctl` is not on `PATH`, and the doctor check returns nothing on gokrazy and in a container, so the appliance image never runs it | Thomas, 2026-10-15: "ok" | Asking systemd to reload, enable, start and stop a unit is the feature, and the stop settings the doctor judges are the ones systemd applies after merging every drop-in, which only systemd computes. The D-Bus API is the native alternative; it would add a client dependency to speak to the same manager `systemctl` speaks to |
| `getent` | `ensureServiceAccount` (`internal/plugins/systemd/cmd_install.go`: `getent group ze`, `getent passwd ze`) before `ze install systemd` creates the service account, and `purgeServiceAccount` (`internal/plugins/systemd/cmd_uninstall.go`: the same two lookups) before `ze uninstall systemd --purge` removes it, through `realServiceOps.output`. Scope: non-appliance hosts that run systemd only; both handlers refuse through `requireSystemd` and `requireRoot` first, so the appliance image never runs it | Thomas, 2026-10-15: "yes list them" | The question is whether the account exists for the tools that create and remove it, and those tools resolve accounts through NSS. `os/user` in a `CGO_ENABLED=0` build reads `/etc/passwd` and `/etc/group` only, so it would answer "absent" for an account NSS serves from LDAP or sssd and the create would then fail |
| `groupadd`, or `addgroup` where `groupadd` is absent | `createGroup` (`internal/plugins/systemd/cmd_install.go`): `groupadd --system ze` (shadow-utils: Debian, Ubuntu, RHEL, Fedora, SUSE, Arch), else `addgroup -S ze` (BusyBox: Alpine), chosen by `exec.LookPath` in that order, when `getent group ze` finds no group during `ze install systemd`. Scope: non-appliance systemd hosts, behind `requireSystemd` and `requireRoot` | Thomas, 2026-10-15: "yes list them" | Creating a system group is the distribution's job: it picks the GID from its own system range (`/etc/login.defs`), writes `/etc/group` and `/etc/gshadow` under the shadow lock, and refreshes the nscd cache. Writing those files from Go would be a second implementation of that policy |
| `useradd`, or `adduser` where `useradd` is absent | `createUser` (`internal/plugins/systemd/cmd_install.go`): `useradd --system --no-create-home --gid ze --home-dir /nonexistent --shell <nologin> ze` (shadow-utils), else `adduser -S -D -H -G ze -s <nologin> ze` (BusyBox: Alpine), chosen by `exec.LookPath` in that order, when `getent passwd ze` finds no user during `ze install systemd`. `<nologin>` is `/usr/sbin/nologin` or `/sbin/nologin`, whichever exists. Scope: non-appliance systemd hosts, behind `requireSystemd` and `requireRoot` | Thomas, 2026-10-15: "yes list them" | As for the group: the UID range, the `/etc/passwd` and `/etc/shadow` writes under lock and the cache refresh are the distribution's policy, which only its own tool applies |
| `userdel`, or `deluser` where `userdel` is absent | `deleteUser` (`internal/plugins/systemd/cmd_install.go`): `userdel ze` (shadow-utils), else `deluser ze` (BusyBox: Alpine), chosen by `exec.LookPath` in that order, called by `purgeServiceAccount` only when `ze uninstall systemd --purge` finds the user through `getent passwd ze`. Scope: non-appliance systemd hosts, behind `requireSystemd` and `requireRoot` | Thomas, 2026-10-15: "yes list them" | Removing the account it created is the inverse of `useradd` and edits the same locked files, so the same tool owns it |
| `groupdel`, or `delgroup` where `groupdel` is absent | `deleteGroup` (`internal/plugins/systemd/cmd_install.go`): `groupdel ze` (shadow-utils), else `delgroup ze` (BusyBox: Alpine), chosen by `exec.LookPath` in that order, called by `purgeServiceAccount` only when `ze uninstall systemd --purge` finds the group through `getent group ze`, and never while the user removal failed. Scope: non-appliance systemd hosts, behind `requireSystemd` and `requireRoot` | Thomas, 2026-10-15: "yes list them" | The inverse of `groupadd`, editing the same locked files, so the same tool owns it |

An entry is added by Thomas, and only by Thomas. An agent that believes a fork
is unavoidable states the case and STOPS; it does not add its own row.

## What this governs, and what it does not

It governs shipped **Ze code**: anything under `cmd/`, `pkg/` and `internal/`
except the native developer harness in `internal/le/`, in any build and on any
platform. A binary Ze ships or a diagnostic Ze ships is Ze.

It does NOT govern the native build and test harness in `internal/le/`, `test/`
runners or `.ci` fixtures. They drive a developer machine and a CI runner, where
the toolchain is present by construction and calling it is the point.

## The three questions before asking for a row

1. **What does the kernel interface say?** `vishvananda/netlink` covers links,
   routes, addresses, qdiscs and generic netlink families. A family it does not
   wrap is still reachable: build the request by hand, as
   `internal/le/test/deployment/l2tpdiag_linux.go` does for L2TP, which the library
   does not support at all.
2. **Is the answer a file?** `/proc` and `/sys` answer most questions a
   command-line tool answers, and `os.ReadFile` needs no binary present.
3. **Is the fork only formatting?** Running a tool to print something Ze already
   knows buys a second view that can disagree with Ze's own. State Ze's view.

## Related

- `ai/rules/go-standards.md`, "External Commands" -- the rule this register serves.
- `ai/rules/platform-linux.md` -- Linux-only code and its QEMU evidence.
- `docs/contributing/ze-go-style.md` -- dependencies carry a supply-chain, safety,
  performance and install cost, multiplied down the stack for infrastructure.
