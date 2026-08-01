# 812: ze bootstrap mode (first-boot DHCP + SSH)

## Context

spec-install-5: when ze starts with zefs but no config and no template, it
enters bootstrap mode. Discovers all interfaces, enables DHCP client on
every ethernet NIC, starts SSH. Operator SSHes in and configures the device.

## Decisions

- **EmitBootstrapConfig is separate from EmitConfig.** A new function rather
  than modifying the existing EmitConfig/EmitSetConfig, which are used by
  `ze init` and `ze interface scan --config`. Avoids regression risk.
- **Ethernet-only DHCP.** Bridge, veth, dummy, loopback, wireguard, xfrm
  interfaces are skipped. Only ethernet NICs get DHCP client blocks, because
  bootstrap mode aims for basic network reachability on physical ports.
- **SSH block in generated config, credentials from zefs.** The generated
  config includes `environment { ssh { enabled true; } }` but no credentials.
  SSH reads username/password from zefs keys written by the installer initrd.
- **Falls through on failure.** If netlink backend fails, discovery fails,
  or no ethernet interfaces exist, bootstrapFromDiscovery returns false and
  the existing startup switch continues to the next case (web-only or error).
- **Inserted between template and web-only.** In the startup switch at
  cmd/ze/main.go, bootstrap-from-discovery sits between the existing
  template-based bootstrap and the web-only fallback. Template is more
  specific (operator provided a template), so it takes priority.

## Consequences

- Devices provisioned via ze-install PXE boot into a reachable state
  without any operator intervention beyond powering on.
- The generated config is written to zefs as the active config. Once the
  operator commits a real config, bootstrap config is replaced permanently.
- No special "exit bootstrap" logic needed. Normal config commit replaces it.

## Gotchas

- EmitBootstrapConfig uses WriteString chains, not fmt.Fprintf, to comply
  with the buffer-first / no-sprintf hook.
- The functional test validates the generated config syntax through
  `ze config validate -`, not through actual DHCP/SSH startup (that requires
  netlink backend and real network interfaces).

## Files

None recorded.
