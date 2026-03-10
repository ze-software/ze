#!/usr/bin/env bash
# Scenario 08: Triangle topology — Ze ↔ FRR ↔ BIRD
#
# Validates: Three-way BGP topology with all sessions Established.
#            FRR originates 10.99.0.0/24 → BIRD should receive it via FRR.
#            Ze should also receive it from FRR.
# Prevents:  Multi-peer session management failures, attribute forwarding bugs.
#
# Topology:
#   Ze (AS 65001) ──── FRR (AS 65002) ──── BIRD (AS 65003)
#        └──────────────────────────────────────┘

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$(dirname "$SCRIPT_DIR")/../lib.sh"

# Wait for all sessions to establish.
wait_frr_session "$ZE_IP"
wait_frr_session "$BIRD_IP"
wait_bird_session "ze_peer"
wait_bird_session "frr_peer"

# Wait for route propagation.
sleep 10

# FRR originates 10.99.0.0/24. BIRD should receive it via FRR.
check_bird_route "10.99.0.0/24"

# Verify all sessions still up after route exchange.
if ! docker exec "$FRR_CONTAINER" vtysh -c "show bgp neighbor $ZE_IP" 2>/dev/null | grep -q "BGP state = Established"; then
    log_fail "FRR↔Ze session dropped"
    exit 1
fi
if ! docker exec "$FRR_CONTAINER" vtysh -c "show bgp neighbor $BIRD_IP" 2>/dev/null | grep -q "BGP state = Established"; then
    log_fail "FRR↔BIRD session dropped"
    exit 1
fi
if ! docker exec "$BIRD_CONTAINER" birdc show protocols 2>/dev/null | grep "ze_peer" | grep -q "Established"; then
    log_fail "BIRD↔Ze session dropped"
    exit 1
fi
log_pass "all sessions stable after route exchange"
