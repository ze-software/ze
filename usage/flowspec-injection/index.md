# FlowSpec Injection

Use Ze to inject or relay BGP FlowSpec rules when an automation system must distribute traffic filters to routers. Keep the route source, policy, and withdrawal path explicit because a stale FlowSpec rule can affect live traffic.

## Choose the deployment shape

Two common shapes are:

1. A FlowSpec route reflector receives rules from an authorised controller and reflects them to clients.
2. A protected router receives FlowSpec and translates accepted rules into local firewall actions.

The existing [FlowSpec route-reflector](../../docs/guide/flowspec-route-reflector/) and [protected-router](../../docs/guide/flowspec-protected-router/) guides cover those complete roles. This page focuses on the injection boundary.

## Complete injection lab

The destination peer must negotiate the FlowSpec family. Save this minimal
sender configuration as `flowspec-sender.conf`; configure the remote router as
AS 64501 at `192.0.2.20`.

```text
plugin {
    internal rib { use bgp-rib; }
}

bgp {
    router-id 192.0.2.10;
    session { asn { local 64500; } }

    peer flowspec-client {
        connection {
            local { ip 192.0.2.10; }
            remote { ip 192.0.2.20; }
        }
        session {
            asn { remote 64501; }
            family {
                ipv4/flow { prefix { maximum 1000; } }
            }
        }
    }
}
```

Validate and start the sender, then confirm family negotiation:

```console
ze config validate flowspec-sender.conf
ze start flowspec-sender.conf
ze cli -c "show bgp peer flowspec-client capabilities"
```

In a lab, inject a discard rule for TCP destination port 443:

```console
ze cli -c "peer flowspec-client update text extended-community [rate-limit:0] nhop self nlri ipv4/flow add destination 203.0.113.0/24 protocol tcp destination-port =443"
```

The command uses the same registered dispatcher through CLI, REST, gRPC, and
MCP. Confirm the remote router installed the rule, then withdraw the exact NLRI:

```console
ze cli -c "peer flowspec-client update text nlri ipv4/flow del destination 203.0.113.0/24 protocol tcp destination-port =443"
```

## Use an atomic window

When several rules form one mitigation, stage and inspect them before release:

```console
ze cli -c "request commit start mitigation-42"
ze cli -c "peer flowspec-client update text extended-community [rate-limit:0] nhop self nlri ipv4/flow add destination 203.0.113.0/24 protocol tcp destination-port =443"
ze cli -c "peer flowspec-client update text extended-community [rate-limit:9600] nhop self nlri ipv4/flow add destination 203.0.113.0/24 protocol udp destination-port =53"
ze cli -c "request commit show mitigation-42"
ze cli -c "request commit end mitigation-42"
```

Use `request commit rollback mitigation-42` instead of `end` if validation or
review fails. This prevents clients from observing a partial mitigation policy.

## Authorisation and policy

Treat route injection as a privileged operation:

- restrict it with Ze authorisation profiles;
- bind the controller to only the intended peers and families;
- validate component ranges, actions, and traffic rates;
- reject rules outside the protected address space;
- record the actor and request in the audit trail;
- set an external expiry or withdrawal owner for temporary rules.

A controller must be able to withdraw every rule it installs. Do not rely on process exit as cleanup.

## Verification

After injection:

```console
ze cli -c "show bgp peer flowspec-client capabilities"
ze cli -c "show bgp rib status"
ze cli -c "show warnings"
ze cli -c "show errors"
```

Then verify the destination appropriate to the deployment:

- On a reflector, confirm the rule is present in the client's received FlowSpec RIB.
- On a protected router, confirm the translated firewall rule and counters.
- In both cases, withdraw the rule and confirm the destination state disappears.

Test rule replacement as well as add and withdraw. A mitigation controller often changes rate or match criteria while an incident is active.

## Failure cases

- The peer did not negotiate the FlowSpec family.
- Import policy rejected the rule.
- The controller targeted the wrong peer selector.
- A multi-rule update was released before all rules were staged.
- The protected router cannot translate one component or action.
- The controller restarted without recovering its installed-rule inventory.
- The BGP withdrawal succeeded but local firewall cleanup failed.

Monitor both BGP state and the enforcement backend. One without the other is incomplete evidence.
