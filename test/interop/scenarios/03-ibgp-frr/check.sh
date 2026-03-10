#!/usr/bin/env bash
# Scenario 03: iBGP session Ze ↔ FRR (same AS)
#
# Validates: Ze establishes an iBGP session (same AS number) with FRR.
# Prevents:  Same-AS rejection bugs or LOCAL_PREF handling issues.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$(dirname "$SCRIPT_DIR")/../lib.sh"

wait_frr_session "$ZE_IP"
