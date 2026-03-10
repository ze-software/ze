#!/usr/bin/env bash
# Scenario 07: Ze announces routes → FRR receives
#
# Validates: Ze can send valid UPDATE messages that FRR accepts.
# Prevents:  Wire encoding bugs producing UPDATEs that real implementations reject.
#
# Ze runs a process plugin that announces 3 prefixes via JSON RPC.
# We verify FRR receives them in its BGP table.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$(dirname "$SCRIPT_DIR")/../lib.sh"

# Wait for session to establish.
wait_frr_session "$ZE_IP"

# Wait for routes to propagate (process plugin needs peer-up event + FRR processing).
# Poll instead of fixed sleep — the plugin may take variable time to announce.
log_info "waiting for Ze to announce routes to FRR..."
deadline=$((SECONDS + 30))
received=false
while [ $SECONDS -lt $deadline ]; do
    if docker exec "$FRR_CONTAINER" vtysh -c "show bgp ipv4 unicast 10.10.0.0/24" 2>/dev/null | grep -q "10.10.0.0/24"; then
        received=true
        break
    fi
    sleep 2
done

if ! $received; then
    log_fail "FRR did not receive 10.10.0.0/24 from Ze within 30s"
    docker exec "$FRR_CONTAINER" vtysh -c "show bgp ipv4 unicast summary" 2>/dev/null || true
    ze_logs 20
    exit 1
fi

# Verify all 3 prefixes arrived.
check_frr_route "10.10.0.0/24"
check_frr_route "10.10.1.0/24"
check_frr_route "10.10.2.0/24"
