# Handover: `create interface <name> unit <vid>` must auto-create parent

## Problem

`create interface eth0 unit 100` fails if `eth0` does not exist. The user expects
the leaf command to check whether its parent resource exists and create it if missing,
walking from root toward leaf. This is the general rule: a leaf creation command must
ensure every ancestor resource in its path exists before creating itself.

## Current State

`handleUnitAdd` in `internal/component/iface/cmd/manage.go` calls `iface.CreateVLAN(name, vid)`
directly. `CreateVLAN` delegates to the backend (`backend.CreateVLAN`), which issues a
netlink `NEWLINK` with `IFLA_LINK` set to the parent ifindex. If the parent does not exist,
the kernel returns `ENODEV` and the command fails.

## Required Behavior

1. `handleUnitAdd` receives parent name and vid.
2. Check if parent interface exists (`iface.GetInterface(name)`).
3. If not, determine the parent type. The type must come from somewhere:
   - Option A: infer from naming convention (not reliable).
   - Option B: require an explicit type argument (`create interface dummy eth0 unit 100`).
   - Option C: a `create interface <type> <name> unit <vid>` compound grammar where the
     type is a keyword before the name.
4. Create the parent using the appropriate `Create*` function.
5. Then create the VLAN sub-interface.

## Design

The compound command (e.g. `create interface dummy eth0 unit 100`) must be a distinct
entry in the YANG command tree, so the dispatch system treats it as a single command
with rollback semantics:

1. Create parent interface (`dummy eth0`).
2. Create unit (`eth0.100`).
3. If step 2 fails, undo step 1 (`delete interface eth0`).

Each sub-step is an explicit action in a list, so rollback removes exactly what was
created. This avoids partial state: the user never sees a parent interface without
its unit after a failed compound create.

The same principle applies to `create interface <name> address <prefix>`: if the
interface does not exist, auto-create it first, and roll back if the address add fails.

## Design Decision Needed

How does the user specify the parent interface type when it does not exist?

| Option | Grammar | Trade-off |
|--------|---------|-----------|
| Compound command | `create interface dummy eth0 unit 100` | Clean, YANG prefix shared with `create interface dummy <name>` |
| Separate flag | `create interface <name> unit <vid> --type dummy` | Flag-based, no YANG prefix issue |

## Scope

- New compound YANG entries (e.g. `create interface dummy <name> unit <vid>`)
- `handleUnitAdd` in `internal/component/iface/cmd/manage.go`: check parent, create if missing, rollback on failure
- Same pattern for `create interface <name> address <prefix>`
- Rollback must be explicit undo actions (delete what was created), not silent cleanup

## Not In Scope

- Changing the offline CLI (`ze interface create ...`) which runs without a daemon
- Other subsystems (L2TP, PPPoE) that have their own resource hierarchies
