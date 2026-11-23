# One configuration engine, many protocols

*2026-11-22 by Thomas Mangin*

A remote configuration change needs a way back even after the SSH session drops.

When you change a router over SSH, the network carrying the session may be the one you’re changing. The proposed configuration can pass validation and still cut off the connection you need to undo it.

*This article was drafted with Gemini and revised with Sol. The architecture and design decisions are mine.*

## Confirm the change from the network

Junos has a command I like for this. `commit confirmed` applies the change and gives you time to check the result. If the change cuts off your access, the router can restore its previous configuration when the timer expires. [Junos confirmed commits](https://www.juniper.net/documentation/us/en/software/junos/cli/topics/topic-map/junos-configuration-commit.html)

The same workflow is available in Ze. Use `commit confirmed 120` to apply the change with a two-minute window, then open a fresh SSH connection from your usual management network. For a routing change, check that the affected peers have returned and that they see the routes you expect before issuing `commit accept`.

The Ze daemon owns the timer and starts restoring the previous configuration when the timer expires, even if the SSH client that issued the commit has disconnected. [Ze confirmed commits](https://github.com/ze-software/ze/blob/main/docs/guide/config-editor.md#commit-confirmed) · [SSH disconnect test](https://github.com/ze-software/ze/blob/main/test/plugin/session-editor-commit-confirmed-disconnect.ci)

## What YANG checks before a commit

Each plugin that supplies a YANG module contributes settings and constraints to Ze’s configuration schema. For example, the BGP listener uses a shared port type whose YANG range is 1 through 65535, so Ze can refuse a port of `0` before opening a socket. The editor and the configuration loader get that range from the same declaration. [Ze’s configuration model](../../guides/configuration-model/)

Ze extends YANG so a schema can name checks registered in the daemon. A setting marked with `ze:validate` calls a validator by name. BGP uses it to refuse `0.0.0.0` as a router ID even though it has the form of an IPv4 address. [Ze’s validation extensions](https://github.com/ze-software/ze/blob/main/docs/architecture/config/yang-config-design.md#extensions-ze-extensionsyang)

`ze:reference` lets a setting name a source of values in the proposed configuration. BGP peers and static next hops both use `ze:reference "bfd-profile"` to refer to a named Bidirectional Forwarding Detection (BFD) profile. The editor offers the profiles you’ve defined, and Ze refuses a commit that names a missing profile. The two components get the same behaviour from the same lookup. [BFD profile lookup](https://github.com/ze-software/ze/blob/main/internal/component/bfd/api/profile_reference.go)

Built-in and external plugins can import the same extension module and reuse the validators and reference sources registered in the running Ze build. Ze refuses a module that names one it cannot find. The engine checks `ze:reference` across the proposed configuration. The shared `ze:validate` pass currently covers selected sections, and BGP runs its own validation. An external plugin can enforce additional rules in its verify step before Ze applies its changes. [Plugin schema guide](https://github.com/ze-software/ze/blob/main/docs/plugin-development/schema.md) · [`ze:validate` coverage](https://github.com/ze-software/ze/blob/main/docs/architecture/config/yang-config-design.md#where-a-validator-actually-runs)

## What the engine does when the change goes live

Suppose you move an IP address from one interface to another and a BGP peer uses it as its local address. The BGP configuration can stay exactly as it was, yet the live session depends on an address that disappears during the move. The configuration engine orders the work across the two components. Ze stops the session before removing the address and starts it again after assigning the address to the other interface. The test watches the kernel’s address notifications and checks that the peer reconnects and sends its route again. [Address-move test](https://github.com/ze-software/ze/blob/main/test/reload/config-apply-ordering-address-swap.ci)

If a component fails during apply, Ze starts undoing the changes it has already made. In one test a later plugin fails after Ze has removed a BGP peer. The rollback puts the peer back, and the peer has to reconnect and announce its route again. A confirmed commit covers the case where apply succeeds and you still have to see what happened on the network before keeping the change. [Rollback test](https://github.com/ze-software/ze/blob/main/test/reload/config-apply-ordering-mixed-rollback.ci)
