#!/usr/bin/env bash
set -euo pipefail

clear
case "${1:-}:${2:-}" in
    launcher:intro)
        cat <<'EOF'

  Find a Ze command without memorizing the tree
  =============================================

  You know the task but not the exact command path. Ze's launcher finds
  it as you type.

  We will open the launcher, filter the show commands down to traceroute,
  step back to the root, and find the doctor readiness check.

  Type to filter. Enter drills down. Escape steps back.
EOF
        ;;
    launcher:recap)
        cat <<'EOF'

  GENERATED FROM THE LIVE COMMAND REGISTRY
  ========================================

  Type to filter. Enter drills down. Escape moves back.

  The launcher is generated from Ze's live command registry, so it stays
  aligned with the commands available in the installed binary.

  Recording complete.
EOF
        ;;
    cli-dashboard:intro)
        cat <<'EOF'

  Inspect live BGP sessions from the CLI
  ======================================

  You want to check active BGP sessions without leaving the terminal or
  standing up a separate monitoring stack.

  We will connect over Ze's SSH management plane, open the continuously
  refreshed BGP dashboard, sort the peer table, and open one session in
  detail.

  s sorts. Arrows move. Enter opens detail. Escape returns.
EOF
        ;;
    cli-dashboard:recap)
        cat <<'EOF'

  ONE LIVE VIEW OF EVERY SESSION
  ==============================

  One interactive view combines peer state, uptime, message counters,
  update rates, and per-session detail. The data refreshes while you work.

  Recording complete.
EOF
        ;;
    zefs-config:intro)
        cat <<'EOF'

  Configure Ze from storage to live commit
  ========================================

  You are setting up a new router and need the whole path, from creating
  the configuration store to committing a change on the running daemon.

  We will create a fresh ZeFS database with ze init, list and validate
  the stored configuration, connect to Ze's SSH editor, change a setting,
  review the diff, commit, and confirm it live.
EOF
        ;;
    zefs-config:ssh)
        cat <<'EOF'

  STORAGE CHECK COMPLETE
  ======================

  ZeFS now contains a valid active configuration.

  Next: connect over SSH and run show bgp summary in its default text
  format. Then switch the default to table, review and commit the diff,
  and run the same command again to compare the presentation.
EOF
        ;;
    zefs-config:recap)
        cat <<'EOF'

  FROM ZE INIT TO A LIVE COMMIT
  =============================

  ze init created the ZeFS database. The same BGP summary changed from
  text to a box-drawing table after the SSH editor committed the new
  default. show | compare exposed the exact change.

  No configuration file was edited on the running router.

  Recording complete.
EOF
        ;;
    commit-confirmed:intro)
        cat <<'EOF'

  Protect a remote commit with a safety window
  ============================================

  You are changing a router you can only reach over the network. A bad
  commit could lock you out, so you want the change to undo itself unless
  you confirm it in time.

  Ze starts with hostname edge-original. In one editor we will commit
  edge-trial and stay silent so it rolls back, verify edge-original
  returned, then commit edge-confirmed, run confirm, wait past the same
  deadline, and verify it stayed.
EOF
        ;;
    commit-confirmed:recap)
        cat <<'EOF'

  UNCONFIRMED ROLLS BACK, CONFIRMED PERSISTS
  ==========================================

  The unconfirmed edge-trial value became active, then Ze restored
  edge-original when its deadline expired. The second change received
  an explicit confirm command and remained edge-confirmed after the
  same deadline passed. Both outcomes were observed in one editor.

  Recording complete.
EOF
        ;;
    irr-filter:intro)
        cat <<'EOF'

  Add IRR filtering to an existing BGP peer
  =========================================

  The stored configuration already has customer-a and Adj-RIB-In, but no
  IRR plugin, IRR server, AS-SET, or import filter.

  We will add all five settings with one-shot `ze config set` commands,
  then start Ze and prove the generated list filters received routes.
EOF
        ;;
    irr-filter:configured)
        cat <<'EOF'

  CONFIGURATION POPULATED
  =======================

  The stored configuration now loads bgp-filter-irr, points it at the IRR
  server, maps customer-a to AS-TEST, and applies the ASN 65001 import list.

  The operational commands next use Ze's default `text` format. Its
  key/value layout resembles YAML, but YAML is only selected with `| yaml`.
EOF
        ;;
    irr-filter:recap)
        cat <<'EOF'

  ONE AS-SET, ONE DYNAMIC IMPORT POLICY
  =====================================

  10.0.0.0/24 matched the generated list and entered Adj-RIB-In.
  192.168.0.0/24 did not match and was rejected.

  Ze refreshes the list automatically. Failed refreshes preserve the
  last known good data in ZeFS instead of replacing it with an empty list.

  Recording complete.
EOF
        ;;
    rpki:intro)
        cat <<'EOF'

  Enforce route origin authorization locally
  ==========================================

  One local BGP peer announces three prefixes from AS 65001:
    9.43.0.0/24   Valid      matching ROA
   10.43.0.0/24   Invalid    ROA names another origin
   11.43.0.0/24   NotFound   no covering ROA

  Policy accepts Valid and NotFound, and rejects Invalid.
  A deterministic local RTR cache supplies every VRP.
EOF
        ;;
    rpki:recap)
        cat <<'EOF'

  VALIDATION RESULT
  =================

   9.43.0.0/24  Valid      installed with validation-state 1
  10.43.0.0/24  Invalid    rejected before Adj-RIB-In
  11.43.0.0/24  NotFound   installed with validation-state 2

  The route table showed only the two policy-accepted routes.
  No public RPKI cache or route collector was contacted.

  Recording complete.
EOF
        ;;
    rib-fib:intro)
        cat <<'EOF'

  Follow one route from control plane to Linux
  ============================================

  You want to see Ze do real routing work, taking a BGP route all the
  way into the Linux forwarding table.

  We will inject 198.51.100.0/24 into the BGP RIB, inspect best-path
  selection, and verify the kernel FIB that netlink programmed.
EOF
        ;;
    rib-fib:recap)
        cat <<'EOF'

  ROUTE INSTALLATION PATH
  =======================

  request bgp rib inject
      -> BGP best path
      -> system RIB
      -> Linux kernel FIB

  The automated validator also withdrew the prefix and proved kernel removal.
  Control-plane and kernel state came from the running system.

  Recording complete.
EOF
        ;;
    config-views:intro)
        cat <<'EOF'

  One configuration model, views for humans and automation
  ========================================================

  The same YANG-backed configuration can be presented as:

    hierarchical  Compact Junos-style blocks for operators
    set           One complete path per line for scripts and reviews

  We will render one peer both ways, convert set syntax back to blocks,
  and prove the round trip produces identical canonical set commands.

  Then ze pipe will filter and count the live plugin registry.
EOF
        ;;
    config-views:recap)
        cat <<'EOF'

  ONE MODEL, TWO PRESENTATIONS
  ============================

  Hierarchical config -> set commands -> hierarchical config
                                |
                                +-> identical canonical set output

  ze pipe composed match and count without a separate jq dependency.
  Every frame was generated from Ze's parser and plugin registry.

  Recording complete.
EOF
        ;;
    health-reports:intro)
        cat <<'EOF'

  Diagnose current conditions and recent events
  =============================================

  You are triaging a router and need three different answers from one
  CLI:

    show health    Is every registered component operational?
    show warnings  What needs attention right now?
    show errors    What happened recently?

  A deliberately stale prefix-data date creates a live warning. An
  administrative reset then creates a retained BGP error event.
EOF
        ;;
    health-reports:recap)
        cat <<'EOF'

  ONE OPERATIONAL VIEW, PRECISE SEMANTICS
  =======================================

  Health    aggregate component status
  Warnings  current state, cleared automatically
  Errors    immutable recent events

  The SSH login banner and filtered commands read the same report bus.
  Structured codes and details make every signal scriptable.

  Recording complete.
EOF
        ;;
    rbac:intro)
        cat <<'EOF'

  Enforce read-only access for a NOC account
  ==========================================

  Your NOC team needs to observe the router but must never change its
  state, even by mistake. The read-only profile allows run commands
  except debug and clear, and denies every edit.

  We will show the profile and its NOC binding, run show version as the
  NOC user, then ask the same user to clear interface counters and watch
  Ze refuse it.

  Passwords are injected outside the recording.
EOF
        ;;
    rbac:deny)
        cat <<'EOF'

  Next: test the denied path
  ==========================

  The profile denies every command matching the "clear" prefix.

  Ze resolves the command first, then checks the profile, and refuses
  before the command runs. The response names the reason explicitly:
  "command restricted by access control".
EOF
        ;;
    rbac:recap)
        cat <<'EOF'

  THE DAEMON ENFORCES THE PROFILE
  ===============================

  The same authenticated user could run show version but could not clear
  interface counters. Authorization is enforced by the daemon, not by
  shell policy, and a refusal is reported as such rather than as a typo.

  Recording complete.
EOF
        ;;
    traceroute:intro)
        cat <<'EOF'

  Trace a live path without the Internet
  ======================================

  You need to check reachability and per-hop latency from the router
  itself, with no dependency on a public looking glass or DNS.

  We will probe 192.0.2.53 through an isolated Linux namespace router,
  show the path once as an operational command, then monitor per-hop
  loss and latency over several rounds.

  The lab uses documentation addresses only.
EOF
        ;;
    traceroute:recap)
        cat <<'EOF'

  REAL PROBES THROUGH AN ISOLATED LAB
  ===================================

  Ze sent real ICMP probes through the isolated lab and measured every hop.
  The live view continuously updated loss, latency, and variation without
  contacting any third-party service.

  Recording complete.
EOF
        ;;
    bfd-failover:intro)
        cat <<'EOF'

  Verify BFD protects an edge session
  ===================================

  You are about to deploy a BGP peer with a 300-second hold timer. You
  need proof that a failed link will not leave traffic black-holed for
  five minutes.

  We will inspect the active config and full live CLI output, cut the
  kernel link, observe BFD and BGP, then restore the link and verify the
  peer establishes again.
EOF
        ;;
    bfd-failover:recap)
        cat <<'EOF'

  BFD TOOK DOWN BGP BEFORE THE HOLD TIMER
  =======================================

  The kernel link was visibly cut. Within four seconds the live BFD
  session count was zero and BGP had left Established, despite its
  negotiated 300-second hold time. Restoring the same link brought the
  real BGP session back to Established.

  Recording complete.
EOF
        ;;
    ospf-adjacency:intro)
        cat <<'EOF'

  Diagnose a missing OSPF route from Ze's CLI
  ===========================================

  You are logged into a router and 10.255.0.3/32 is missing.

  We will inspect the active OSPF configuration, query the running
  control plane with `ze cli`, verify the FRR neighbor is Full, find its
  Router-LSA, and confirm SPF installed the loopback route.
EOF
        ;;
    ospf-adjacency:recap)
        cat <<'EOF'

  HELLO TO ROUTE
  ==============

  OSPF neighbors reached Full, exchanged Router-LSAs, and Ze computed an
  intra-area route to FRR's loopback.

  The validator reads all three stages from Ze's running control plane.

  Recording complete.
EOF
        ;;
    traffic-anomaly:intro)
        cat <<'EOF'

  Attribute a traffic burst without packet capture
  ================================================

  Users report a slowdown on `traffic0`. You need to identify the source
  and application without collecting payloads.

  We will inspect the active eBPF accounting configuration, view the full
  baseline, generate ICMP and HTTP traffic, then read the complete live
  snapshot from `ze cli`.
EOF
        ;;
    traffic-anomaly:recap)
        cat <<'EOF'

  TRAFFIC ATTRIBUTION WITHOUT PACKET MODIFICATION
  ===============================================

  The live snapshot attributed the burst to source 10.77.0.2,
  protocol ICMP, and TCP destination port 8080.

  Bounded eBPF maps and stale-entry cleanup control metric cardinality.

  Recording complete.
EOF
        ;;
    vrrp-failover:intro)
        cat <<'EOF'

  Keep the gateway reachable while Ze is stopped
  ==============================================

  You need to stop the active router for maintenance without changing
  the default gateway on every host.

  We will inspect the active ZeFS and live VRRP state, verify Ze owns the
  VIP, stop that node, then prove keepalived owns the same reachable VIP.
EOF
        ;;
    vrrp-failover:recap)
        cat <<'EOF'

  ONE VIP AND MAC, A DIFFERENT ROUTER
  ===================================

  Keepalived promoted after Ze stopped. The VIP stayed reachable and
  retained virtual MAC 00:00:5e:00:01:0a, so hosts need no ARP change.

  Election and failover used real VRRP packets on an isolated segment.

  Recording complete.
EOF
        ;;
    host-inventory:intro)
        cat <<'EOF'

  Inspect an unfamiliar Linux host before Ze starts
  =================================================

  You need the kernel, CPU topology, and memory headroom, but the Ze
  daemon is not running yet.

  We will run the commands directly and keep every field in the output.
  YAML makes the complete structured inventory readable in the terminal.
EOF
        ;;
    host-inventory:recap)
        cat <<'EOF'

  STRUCTURED INVENTORY, AVAILABLE OFFLINE
  =======================================

  The same command surface reported kernel, CPU, and memory inventory.
  Every field stayed structured data, shown here as YAML and ready to
  feed fleet automation unchanged.

  Recording complete.
EOF
        ;;
    config-graph:intro)
        cat <<'EOF'

  Know which peers inherit a group before changing it
  ===================================================

  You need to change the transit group's remote ASN. Before scheduling
  maintenance, you need the exact peers that inherit that value.

  We will inspect and validate the config, then filter Ze's machine-readable
  dependency graph to expose the complete group-to-peer blast radius.
EOF
        ;;
    config-graph:recap)
        cat <<'EOF'

  CONFIGURATION AS AN EXPLICIT GRAPH
  ==================================

  The BGP section contained a transit group. Both upstream peers inherited
  from it. The JSON graph exposed these relationships for operators,
  automation, and agent impact analysis.

  Recording complete.
EOF
        ;;
    *)
        printf 'usage: %s <demo> <intro|checkpoint|recap>\n' "$0" >&2
        exit 2
        ;;
esac
