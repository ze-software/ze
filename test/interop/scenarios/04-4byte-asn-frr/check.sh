#!/usr/bin/env bash
# Scenario 04: 4-byte ASN eBGP Ze ↔ FRR
#
# Validates: Ze negotiates 4-byte ASN capability (RFC 6793) with FRR.
# Prevents:  ASN4 capability encoding/decoding mismatch with real implementations.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$(dirname "$SCRIPT_DIR")/../lib.sh"

wait_frr_session "$ZE_IP"
