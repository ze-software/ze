#!/usr/bin/env bash
# Manual runner for the LG graph lab.
# Starts ze with 36 injected routes and a looking glass on localhost.
# Browse to the printed URL, Ctrl+C to stop.
#
# Usage: ./run.sh [lg-port]
#   lg-port defaults to 8443

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

LG_PORT="${1:-8443}"
BGP_PORT=1790

# Build ze if needed
if [[ ! -x "$REPO_ROOT/bin/ze" ]]; then
    echo "Building ze..."
    (cd "$REPO_ROOT" && go build -o bin/ze ./cmd/ze)
fi

# Prepare working directory with config and plugin
WORKDIR="$(mktemp -d)"
trap 'kill "$ZE_PID" 2>/dev/null; wait "$ZE_PID" 2>/dev/null; rm -r "$WORKDIR"' EXIT

cp "$SCRIPT_DIR/lg-lab.py" "$WORKDIR/lg-lab.py"
chmod 755 "$WORKDIR/lg-lab.py"

# Substitute port into config
sed -e "s/\${LG_PORT}/$LG_PORT/" -e "s/\${LG_IP}/0.0.0.0/" "$SCRIPT_DIR/ze-bgp.conf" > "$WORKDIR/ze-bgp.conf"

# Start ze
cd "$WORKDIR"
PYTHONPATH="$REPO_ROOT/test/scripts" \
PATH="$REPO_ROOT/bin:$PATH" \
ze_test_bgp_port="$BGP_PORT" \
"$REPO_ROOT/bin/ze" "$WORKDIR/ze-bgp.conf" &
ZE_PID=$!
cd "$REPO_ROOT"

# Wait for routes
echo "Waiting for route injection..."
for _ in $(seq 1 30); do
    BODY=$(curl -sf "http://127.0.0.1:$LG_PORT/lg/graph?prefix=10.10.1.0/24&mode=aspath&format=text" 2>/dev/null || true)
    if echo "$BODY" | grep -q AS2914; then break; fi
    sleep 0.5
done

echo ""
echo "Looking glass ready:"
echo "  http://127.0.0.1:$LG_PORT/lg/"
echo ""
echo "Graph URLs:"
echo "  http://127.0.0.1:$LG_PORT/lg/graph?prefix=10.10.1.0/24&mode=aspath"
echo "  http://127.0.0.1:$LG_PORT/lg/graph?prefix=10.10.1.0/24&mode=nexthop"
echo "  http://127.0.0.1:$LG_PORT/lg/graph?prefix=10.10.2.0/24&mode=aspath"
echo "  http://127.0.0.1:$LG_PORT/lg/graph?prefix=10.10.2.0/24&mode=nexthop"
echo "  http://127.0.0.1:$LG_PORT/lg/graph?prefix=10.10.3.0/24&mode=aspath"
echo "  http://127.0.0.1:$LG_PORT/lg/graph?prefix=10.10.3.0/24&mode=nexthop"
echo ""
echo "Update reference SVGs:"
echo "  for f in expect/lg-graph-*.svg; do"
echo "    p=\$(echo \$f | sed 's/.*p\\([123]\\).*/10.10.\\1.0\\/24/'); m=\$(echo \$f | sed 's/.*-\\(aspath\\|nexthop\\).*/\\1/')"
echo "    curl -s \"http://127.0.0.1:$LG_PORT/lg/graph?prefix=\$p&mode=\$m\" > \"\$f\""
echo "  done"
echo ""
echo "Press Ctrl+C to stop."
wait "$ZE_PID"
