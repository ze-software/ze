#!/usr/bin/env bash
#
# Capture screenshots for the LINX presentation.
# Starts ze with a demo config and a mock BGP peer,
# then captures CLI output and browser screenshots.
#
# Requirements: bin/ze, bin/ze-test, bin/ze-chaos, agent-browser
# Output: tmp/presentation-screenshots/

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$("$SCRIPT_DIR/project-root.sh")"
OUT="$PROJECT_DIR/tmp/presentation-screenshots"
CONF="$PROJECT_DIR/tmp/presentation-screenshots/demo.conf"
ZE_PID=""
PEER_PID=""
CHAOS_PID=""

WEB_PORT=19380
LG_PORT=19381
SSH_PORT=19322
BGP_PORT=19179
CHAOS_PORT=19382

cleanup() {
    echo "Cleaning up..."
    [ -n "$CHAOS_PID" ] && kill "$CHAOS_PID" 2>/dev/null || true
    [ -n "$PEER_PID" ] && kill "$PEER_PID" 2>/dev/null || true
    [ -n "$ZE_PID" ] && kill "$ZE_PID" 2>/dev/null || true
    wait 2>/dev/null || true
}
trap cleanup EXIT

if [ ! -d "$PROJECT_DIR/tmp" ]; then
    echo "error: $PROJECT_DIR/tmp does not exist" >&2
    exit 1
fi
mkdir -p "$OUT"

# --- Demo config with BGP peers, web, LG, SSH ---
cat > "$CONF" << 'CONF'
environment {
    log { destination stdout; level warning; }
}
listener {
    web { listen 127.0.0.1:WEB_PORT; }
    looking-glass { listen 127.0.0.1:LG_PORT; }
    ssh { listen 127.0.0.1:SSH_PORT; }
}
plugin {
    external rib { use bgp-rib; }
}
bgp {
    router-id 192.0.2.254;
    peer ixp-member-a {
        connection {
            remote { ip 127.0.0.1; port BGP_PORT; }
            local { ip 127.0.0.1; }
        }
        session {
            asn { local 65500; remote 64501; }
            family {
                ipv4/unicast {}
            }
        }
    }
    peer ixp-member-b {
        connection {
            remote { ip 127.0.0.2; }
            local { ip 127.0.0.1; }
        }
        session {
            asn { local 65500; remote 64502; }
            family {
                ipv4/unicast {}
                ipv6/unicast {}
            }
        }
    }
    peer upstream-transit {
        connection {
            remote { ip 127.0.0.3; }
            local { ip 127.0.0.1; }
        }
        session {
            asn { local 65500; remote 64503; }
            family {
                ipv4/unicast {}
                ipv6/unicast {}
            }
        }
    }
}
CONF

# substitute ports
sed -i '' \
    -e "s/WEB_PORT/$WEB_PORT/g" \
    -e "s/LG_PORT/$LG_PORT/g" \
    -e "s/SSH_PORT/$SSH_PORT/g" \
    -e "s/BGP_PORT/$BGP_PORT/g" \
    "$CONF"

echo "=== Starting mock BGP peer on port $BGP_PORT ==="
"$PROJECT_DIR/bin/ze-test" peer \
    --mode sink \
    --port "$BGP_PORT" \
    --asn 64501 &
PEER_PID=$!
sleep 1

echo "=== Starting ze daemon ==="
"$PROJECT_DIR/bin/ze" "$CONF" &
ZE_PID=$!

echo "Waiting for ze to start..."
for i in $(seq 1 30); do
    if curl -s -o /dev/null "http://127.0.0.1:$WEB_PORT/" 2>/dev/null; then
        echo "ze web ready after ${i}s"
        break
    fi
    sleep 1
done

# --- CLI screenshots (capture command output) ---
echo "=== Capturing CLI output ==="

capture_cli() {
    local name="$1"
    shift
    echo "\$ $*" > "$OUT/cli-${name}.txt"
    "$PROJECT_DIR/bin/ze" "$@" >> "$OUT/cli-${name}.txt" 2>&1 || true
}

capture_cli "summary" summary
capture_cli "show-bgp-peer" show bgp peer
capture_cli "help" help
capture_cli "help-ai" help --ai
capture_cli "show-interface" show interface brief
capture_cli "doctor" doctor

echo "=== Capturing browser screenshots ==="

# --- Web UI screenshots ---
agent-browser launch --headless 2>/dev/null || true

take_screenshot() {
    local name="$1"
    local url="$2"
    local wait="${3:-2000}"
    echo "  screenshot: $name"
    agent-browser navigate "$url" 2>/dev/null
    sleep "$(echo "$wait / 1000" | bc)"
    agent-browser screenshot --full "$OUT/${name}.png" 2>/dev/null || \
        agent-browser screenshot "$OUT/${name}.png" 2>/dev/null || \
        echo "  FAILED: $name"
}

# Web UI
take_screenshot "web-config-editor" "http://127.0.0.1:$WEB_PORT/config/" 2000
take_screenshot "web-show-bgp-peer" "http://127.0.0.1:$WEB_PORT/show/bgp/peer/" 2000
take_screenshot "web-show-environment" "http://127.0.0.1:$WEB_PORT/show/environment/" 2000
take_screenshot "web-dashboard" "http://127.0.0.1:$WEB_PORT/" 2000

# Looking Glass
take_screenshot "lg-peers" "http://127.0.0.1:$LG_PORT/" 2000
take_screenshot "lg-routes" "http://127.0.0.1:$LG_PORT/routes" 2000

agent-browser close 2>/dev/null || true

# --- Ze Chaos screenshot ---
echo "=== Starting ze-chaos for dashboard screenshot ==="
"$PROJECT_DIR/bin/ze-chaos" \
    --in-process \
    --web ":$CHAOS_PORT" \
    --duration 15s \
    --peers 4 \
    --seed 42 \
    --routes 20 \
    --quiet &
CHAOS_PID=$!
sleep 3

agent-browser launch --headless 2>/dev/null || true
take_screenshot "chaos-dashboard" "http://127.0.0.1:$CHAOS_PORT/" 3000
take_screenshot "chaos-peers" "http://127.0.0.1:$CHAOS_PORT/peers" 2000
agent-browser close 2>/dev/null || true

echo ""
echo "=== Done ==="
echo "Screenshots saved to: $OUT/"
ls -la "$OUT/"
