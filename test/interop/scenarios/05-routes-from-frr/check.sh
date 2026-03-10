#!/usr/bin/env bash
# Scenario 05: FRR announces routes → Ze receives
#
# Validates: Ze can receive IPv4 unicast UPDATE messages from FRR without error.
# Prevents:  UPDATE parsing failures when receiving real routes from FRR.
#
# FRR originates 3 static routes and redistributes them into BGP.
# We verify the session stays Established after route exchange (Ze doesn't
# send NOTIFICATION due to parse errors).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$(dirname "$SCRIPT_DIR")/../lib.sh"

# Wait for session to establish.
wait_frr_session "$ZE_IP"

# Wait for FRR to redistribute static routes and send them to Ze.
# FRR's redistribute is asynchronous — poll instead of using a fixed sleep.
log_info "waiting for FRR to send routes to Ze..."
deadline=$((SECONDS + 30))
sent=0
while [ $SECONDS -lt $deadline ]; do
    # PfxSnt is the second-to-last column (last is Desc which can be "N/A").
    sent=$(docker exec "$FRR_CONTAINER" vtysh -c "show bgp ipv4 unicast summary" 2>/dev/null | \
        grep "$ZE_IP" | awk '{print $(NF-1)}')
    if [ -n "$sent" ] && [ "$sent" -ge 3 ] 2>/dev/null; then
        break
    fi
    sleep 2
done

if [ -n "$sent" ] && [ "$sent" -ge 3 ] 2>/dev/null; then
    log_pass "FRR sent $sent prefixes to Ze"
else
    log_fail "FRR sent ${sent:-0} prefixes to Ze (expected >= 3)"
    exit 1
fi

# Verify FRR's BGP table has the expected static routes.
check_frr_route "10.0.0.0/24"
check_frr_route "10.0.1.0/24"
check_frr_route "10.0.2.0/24"

# Verify Ze's RIB received the routes (proves Ze parsed and stored them, not just accepted the session).
check_ze_rib_received 3

# Verify session is still Established after route exchange.
# If Ze couldn't parse the UPDATEs, it would send NOTIFICATION and session drops.
if ! docker exec "$FRR_CONTAINER" vtysh -c "show bgp neighbor $ZE_IP" 2>/dev/null | grep -q "BGP state = Established"; then
    log_fail "session dropped after route exchange (Ze may have sent NOTIFICATION)"
    exit 1
fi
