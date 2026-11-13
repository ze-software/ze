#!/usr/bin/env bash
set -euo pipefail
proof_le=${1:?named le executable}
proof_tags="ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')"

# Complete changed leaf packages already passed in 6d48eba2; do not repeat
# them. This repetition follows the affected reactor call population rather
# than repeating unrelated known baseline failures.

reactor_scope_files=$(
  {
    rg -l 'AnnounceNLRIBatch\(|QueueAnnounce\(|queueAnnounceForPeer\(|resolveNextHop\(|queuedNextHopPolicy\(|resolveQueuedNextHop\(|sendInitialRoutes\(|drainAndCloseQueueGate\(|sendDefaultOriginateRoutes\(|sendStaticRoutes\(|sendStaticRoutesGrouped\(' \
      internal/component/bgp/reactor -g '*_test.go'
    printf '%s\n' internal/component/bgp/reactor/quiesce_drain_test.go
  } | sort -u
)
reactor_scope_names=$(rg --no-filename '^func Test[^ (]+\(' $reactor_scope_files | sed -E 's/^func (Test[^ (]+)\(.*/\1/' | sort -u)
reactor_scope_run="^($(printf '%s\n' "$reactor_scope_names" | paste -sd '|' -))$"
printf 'Affected reactor population: %s files, %s named tests\n' \
  "$(printf '%s\n' "$reactor_scope_files" | wc -l)" \
  "$(printf '%s\n' "$reactor_scope_names" | wc -l)"
"$proof_le" job run label bgp-default-affected-reactor-race20 quiet command \
  go test -v -count=20 -race -timeout=40m -tags "$proof_tags" \
  -run "$reactor_scope_run" ./internal/component/bgp/reactor/
