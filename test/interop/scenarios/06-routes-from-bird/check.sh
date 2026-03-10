#!/usr/bin/env bash
# Scenario 06: BIRD announces routes → Ze receives
#
# Validates: Ze can receive IPv4 unicast UPDATE messages from BIRD without error.
# Prevents:  UPDATE parsing failures when receiving real routes from BIRD.
#
# BIRD originates 3 static routes and exports them via BGP.
# We verify the session stays Established after route exchange.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$(dirname "$SCRIPT_DIR")/../lib.sh"

# Wait for session to establish.
wait_bird_session "ze_peer"

# Wait for BIRD to export static routes to Ze.
# Parse "Routes: N imported, N filtered, N exported, N preferred" from protocol details.
log_info "waiting for BIRD to export routes to Ze..."
deadline=$((SECONDS + 30))
exported=0
while [ $SECONDS -lt $deadline ]; do
    routes_line=$(docker exec "$BIRD_CONTAINER" birdc "show protocols all ze_peer" 2>/dev/null | grep "Routes:" || true)
    if [ -n "$routes_line" ]; then
        exported=$(echo "$routes_line" | sed -n 's/.*[^0-9]\([0-9][0-9]*\) exported.*/\1/p')
        if [ -n "$exported" ] && [ "$exported" -ge 3 ] 2>/dev/null; then
            break
        fi
    fi
    sleep 2
done

if [ -n "$exported" ] && [ "$exported" -ge 3 ] 2>/dev/null; then
    log_pass "BIRD exported $exported routes to Ze"
else
    log_fail "BIRD exported ${exported:-0} routes to Ze (expected >= 3)"
    docker exec "$BIRD_CONTAINER" birdc "show protocols all ze_peer" 2>/dev/null || true
    exit 1
fi

# Verify BIRD's routing table has the expected static routes.
check_bird_route "10.0.0.0/24"
check_bird_route "10.0.1.0/24"
check_bird_route "10.0.2.0/24"

# Verify Ze's RIB received the routes (proves Ze parsed and stored them, not just accepted the session).
check_ze_rib_received 3

# Verify session is still Established after route exchange.
if ! docker exec "$BIRD_CONTAINER" birdc show protocols 2>/dev/null | grep "ze_peer" | grep -q "Established"; then
    log_fail "session dropped after route exchange (Ze may have sent NOTIFICATION)"
    exit 1
fi
