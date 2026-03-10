#!/usr/bin/env bash
# Scenario 01: eBGP IPv4 session Ze ↔ FRR
#
# Validates: Ze establishes an eBGP session with FRR using IPv4 unicast.
# Prevents:  OPEN/KEEPALIVE exchange failures between Ze and FRR.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$(dirname "$SCRIPT_DIR")/../lib.sh"

wait_frr_session "$ZE_IP"
