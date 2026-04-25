# Deactivating configuration

Ze supports a Junos-style `deactivate` mechanism: you can mark any
configuration node inactive without removing it. The node stays in the
file and round-trips through save/load, but is treated as absent at
apply time. Use it when you want to park a block of config (a peer, a
policy, a single leaf) without losing the values.

## What can be deactivated

Every YANG node is deactivatable. No schema annotation is required.

| Node kind | Example syntax in file | Effect at apply |
|-----------|------------------------|-----------------|
| Leaf | `inactive: router-id 10.0.0.1;` | leaf invisible to consumers; default applies |
| Container | `inactive: filter { ... }` | whole subtree absent |
| List entry | `inactive: peer peer1 { ... }` | entry absent, other entries unaffected |
| Leaf-list value | `import [ inactive:no-self-as reject-bogons ];` | only that one value skipped |

The one exception: positional list entries with all-leaf children
(`nlri`, `nexthop`, `add-path`) are not individually deactivatable.
Deactivate the parent container instead.

## CLI

One-shot verbs (mirror the existing `ze config set`):

```sh
ze config deactivate <file> <path...>
ze config activate   <file> <path...>
```

Examples:

```sh
ze config deactivate router.conf bgp router-id
ze config deactivate router.conf bgp peer peer1
ze config deactivate router.conf bgp filter import no-self-as
ze config activate   router.conf bgp router-id
```

Flags:

| Flag | Purpose |
|------|---------|
| `--dry-run` | Show what would change without writing |
| `--no-reload` | Do not notify the running daemon after save |
| `--user` / `-u` | SSH login username (overrides zefs super-admin) |

## TUI

Inside `ze config edit`, the same verbs work as model commands:

```
deactivate bgp router-id
activate   bgp router-id
```

Tab completion drives the path. The status line reports the result.

## Round-trip

Parse-serialize-parse preserves the inactive flag exactly. The on-disk
form is the `inactive: ` prefix on the structural statement, or the
`inactive:` prefix on a leaf-list value. A file containing
`inactive: router-id 1.2.3.4;` re-saved by the editor produces the same
prefix.

### Set / single-line format

When a config is dumped or migrated to the set format (one statement
per line: `set <path> <value>`), inactivity is declared with a separate
`inactive` line:

```
set bgp router-id 10.0.0.1
inactive bgp router-id

set bgp peer peer1 connection remote ip 10.0.0.2
inactive bgp peer peer1
```

Single-keyword design: there is no `activate` counterpart in the file
syntax. The presence of `inactive <path>` means the node is inactive;
its absence means active. Re-activating is just removing the line, so
the editor's `ze config activate <file> <path>` clears the marker and
re-serializes the file without the `inactive` line.

The legacy round-trip form `set <path> inactive true` (which leaked the
engine's auto-injected leaf into the user-facing file) is no longer
emitted. The setparser still parses it for backward compatibility.

## Apply-time behavior

`PruneInactive` runs at every documented apply site (the daemon
loader, the editor preview, and the BGP peer materialization). After
prune:

- A deactivated leaf returns "not present" from `tree.Get` -- the
  consumer falls back to the schema default if any.
- A deactivated container or list entry has its entire subtree removed.
- A deactivated leaf-list value is dropped from the slice; the rest of
  the leaf-list is unaffected.

## Co-existence with per-feature disable

Some YANG nodes carry a real `enabled` / `admin-state` leaf the
protocol respects (a peer's admin-down, BFD enable, etc.). Those are
operationally distinct: the protocol knows the node exists and treats
it as administratively down (BGP peer in `Idle (Admin)` for example).

`deactivate` is the orthogonal generic mechanism: at apply time the
node is *absent*, not *present-but-disabled*. Pick the right tool:

- "I want the peer to show up in oper state as Idle (Admin)" -- use
  the protocol's own admin-state leaf.
- "I want this block of config parked for a week, but the runtime
  shouldn't know about it" -- use `deactivate`.
