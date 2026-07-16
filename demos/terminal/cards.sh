#!/usr/bin/env bash
set -euo pipefail

clear
case "${1:-}:${2:-}" in
    launcher:intro)
        cat <<'EOF'

  Ze command launcher
  ===================

  Goal: discover Ze commands without memorizing the command tree.

  This recording will:
    1. Open the interactive launcher.
    2. Filter the Show commands down to traceroute.
    3. Return to the root and find the doctor command.

  Watch the breadcrumb and filter labels as the menu changes.
EOF
        ;;
    launcher:recap)
        cat <<'EOF'

  WHAT THIS PROVED
  =================

  Type to filter. Enter drills down. Escape moves back.

  The launcher is generated from Ze's live command registry, so it stays
  aligned with the commands available in the installed binary.

  Recording complete.
EOF
        ;;
    cli-dashboard:intro)
        cat <<'EOF'

  Ze live BGP dashboard
  =====================

  Goal: inspect active BGP sessions without leaving the CLI.

  This recording will:
    1. Connect through Ze's SSH management plane.
    2. Open the continuously refreshed BGP dashboard.
    3. Sort the peer table and inspect one session in detail.

  Keys used: s sorts, arrows move, Enter opens detail, Escape returns.
EOF
        ;;
    cli-dashboard:recap)
        cat <<'EOF'

  WHAT THIS PROVED
  =================

  One interactive view combines peer state, uptime, message counters,
  update rates, and per-session detail. The data refreshes while you work.

  Recording complete.
EOF
        ;;
    zefs-config:intro)
        cat <<'EOF'

  Create ZeFS, then configure Ze over SSH
  =======================================

  Goal: show the complete configuration path from storage to commit.

  This recording will:
    1. Create a fresh ZeFS database with ze init.
    2. List and validate the stored configuration.
    3. Connect to Ze's SSH configuration editor.
    4. Change a setting, review the diff, commit, and verify it live.
EOF
        ;;
    zefs-config:ssh)
        cat <<'EOF'

  STORAGE CHECK COMPLETE
  ======================

  ZeFS now contains a valid active configuration.

  Next: connect over SSH, change the default CLI output format, inspect
  the pending diff, commit it, then run an operational command.
EOF
        ;;
    zefs-config:recap)
        cat <<'EOF'

  WHAT THIS PROVED
  =================

  ze init created the ZeFS database. The SSH editor changed a draft,
  show | compare exposed the exact change, and commit made it active.

  No configuration file was edited on the running router.

  Recording complete.
EOF
        ;;
    rbac:intro)
        cat <<'EOF'

  Prove role-based command authorization
  ======================================

  Goal: verify that a read-only NOC account can observe but not change state.

  The NOC user holds the read-only profile:
    run  ... default allow, deny "debug", deny "clear"
    edit ... default deny

  This recording will:
    1. Run show version successfully as the NOC user.
    2. Ask the same user to clear interface counters.
    3. Observe an explicit access-control denial.

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

  WHAT THIS PROVED
  =================

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

  Goal: show Ze's one-shot and continuously refreshed traceroute views.

  This recording will:
    1. Probe 192.0.2.53 through an isolated Linux namespace router.
    2. Show the path once as an operational command.
    3. Monitor per-hop loss and latency over several rounds.

  The lab uses documentation addresses. No public host or DNS is required.
EOF
        ;;
    traceroute:recap)
        cat <<'EOF'

  WHAT THIS PROVED
  =================

  Ze sent real ICMP probes through the isolated lab and measured every hop.
  The live view continuously updated loss, latency, and variation without
  contacting any third-party service.

  Recording complete.
EOF
        ;;
    *)
        printf 'usage: %s <demo> <intro|checkpoint|recap>\n' "$0" >&2
        exit 2
        ;;
esac
