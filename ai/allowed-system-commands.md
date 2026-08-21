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
| _(none yet)_ | | | |

An entry is added by Thomas, and only by Thomas. An agent that believes a fork
is unavoidable states the case and STOPS; it does not add its own row.

## What this governs, and what it does not

It governs **Ze code**: anything under `cmd/`, `internal/`, `pkg/` and
`scripts/evidence/`, in any build, on any platform. A binary Ze ships or a
diagnostic Ze ships is Ze.

It does NOT govern the build and test harness. `Makefile`, `mk/`, `scripts/dev/`,
`test/` runners and `.ci` fixtures drive a developer machine and a CI runner,
where the toolchain is present by construction and calling it is the point.

## The three questions before asking for a row

1. **What does the kernel interface say?** `vishvananda/netlink` covers links,
   routes, addresses, qdiscs and generic netlink families. A family it does not
   wrap is still reachable: build the request by hand, as
   `scripts/evidence/l2tp-tunnel-diag/main.go` does for L2TP, which the library
   does not support at all.
2. **Is the answer a file?** `/proc` and `/sys` answer most questions a
   command-line tool answers, and `os.ReadFile` needs no binary present.
3. **Is the fork only formatting?** Running a tool to print something Ze already
   knows buys a second view that can disagree with Ze's own. State Ze's view.

## Related

- `ai/rules/go-standards.md`, "External Commands" -- the rule this register serves.
- `ai/rules/platform-linux.md` -- Linux-only code and its QEMU evidence.
- `docs/contributing/ze-style.md` -- dependencies carry a supply-chain, safety,
  performance and install cost, multiplied down the stack for infrastructure.
