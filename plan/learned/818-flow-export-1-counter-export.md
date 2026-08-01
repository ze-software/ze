# 818 -- Flow Export: Counter Export Completion and Review Fixes

## Context

Counter export (sFlow v5 counter samples, NetFlow v9 / IPFIX interface-counter
records) for the `flowexport` component was implemented in a prior session. This
session closed it out: fixed review findings, wired the CLI entry, regenerated
codegen, and tidied module deps. See [[819-flow-export-2-flow-records]] for the
spec-2 integration layer built in the same session.

## Decisions

- **IPFIX counter records use octetTotalCount (IE 85) / packetTotalCount (IE 86),
  not the Delta IEs (1/2).** Interface counters are raw cumulative kernel values;
  the Delta IEs would tell collectors the value is a per-interval delta and be
  misinterpreted. The Delta IE constants remain defined for the per-flow path,
  where conntrack genuinely supplies deltas.
- **`MaxDatagramSize` has one home** (`flowexport.MaxDatagramSize` = 1400). The
  sflow package's duplicate constant was removed and call sites reference the
  parent package, so the UDP-payload bound cannot drift between packages.
- **Datagram metrics count actual datagrams, not Encode calls.** sFlow batches
  counter samples and spills overflow into additional datagrams, so one
  `Encode()` can send several. `ze_flowexport_datagrams_total` is incremented by
  the sender's datagram-count delta around the call, not once per call.
- **Template timestamp advances only on send success.** `lastTemplate = now` moved
  into the `else` branch of `EncodeTemplate`, so a failed template send is retried
  on the next tick instead of being suppressed for a full refresh interval.

## Consequences

- `show flow-export [<collector>]` is reachable: a `flow-export` container was
  added to the show-command YANG tree (`ze:command "ze-show:flow-export"`), and
  `make generate` added the flowexport blank imports to `all.go`.
- `go mod tidy` promoted `mdlayher/genetlink` and `mdlayher/netlink` to direct
  dependencies (used by the spec-2 psample/conntrack readers).

## Gotchas

- The flowexport plugin runs in-process and registers its counter snapshot
  callback directly with the iface rate tracker via `iface.RegisterCollectNotify`
  -- it does not poll. The rate tracker's 1s ticker (started in the iface plugin's
  runEngine) is the sole kernel reader; flow export is a second consumer of the
  same snapshot, using RAW (pre-baseline) stats.
- sFlow if_counters fields 7-18 are XDR 32-bit; truncating uint64 kernel counters
  to uint32 is per the sFlow v5 spec, not a bug.

## Files

- `internal/plugins/flowexport/exporter.go` -- template-timestamp and datagram-count fixes
- `internal/plugins/flowexport/sflow/encoder.go` -- removed duplicate MaxDatagramSize
- `internal/plugins/flowexport/ipfix/{ie,template,data}.go` -- octetTotalCount/packetTotalCount
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` -- flow-export show container
- `internal/component/plugin/all/all.go` -- regenerated flowexport blank imports
- `go.mod` -- genetlink/netlink promoted to direct
