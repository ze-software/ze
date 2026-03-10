#!/usr/bin/env bash
# Scenario 02: eBGP IPv4 session Ze ↔ BIRD
#
# Validates: Ze establishes an eBGP session with BIRD using IPv4 unicast.
# Prevents:  OPEN/KEEPALIVE exchange failures between Ze and BIRD.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$(dirname "$SCRIPT_DIR")/../lib.sh"

wait_bird_session "ze_peer"
