#!/usr/bin/env bash
#
# Capture screenshots for the LINX presentation.
# Starts ze with a demo config and a mock BGP peer,
# then captures CLI output and browser screenshots.
#
# Requirements: bin/ze, bin/ze-test, bin/ze-chaos, agent-browser
# Output: gh-pages/talks/linx-2026-06/screenshots/
#
# Usage: <gh-pages>/presentations/tools/linx-screenshots.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DECK_DIR="$(cd "$SCRIPT_DIR/../../talks/linx-2026-06" && pwd)"
PROJECT_DIR="$("$SCRIPT_DIR/project-root.sh")"
OUT="$DECK_DIR/screenshots"
CONF="$(mktemp "${TMPDIR:-/tmp}/ze-linx-presentation.XXXXXX.conf")"
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
    rm -f "$CONF"
}
trap cleanup EXIT

mkdir -p "$OUT"

# Initialize credentials for SSH CLI access
echo "=== Initializing ze credentials ==="
printf 'admin\ndemo\n127.0.0.1\n%s\nlinx-demo\n' "$SSH_PORT" | \
    "$PROJECT_DIR/bin/ze" init 2>/dev/null || true

# --- Demo config with BGP peers, web, LG, SSH ---
cat > "$CONF" << CONF
environment {
    log { destination stdout; level warning; }
    web {
        enabled true;
        server main {
            ip 127.0.0.1;
            port ${WEB_PORT};
        }
    }
    looking-glass {
        enabled true;
        server main {
            ip 127.0.0.1;
            port ${LG_PORT};
        }
    }
}
plugin {
    external rib { use bgp-rib; }
}
bgp {
    router-id 192.0.2.254;
    peer ixp-member-a {
        connection {
            remote { ip 127.0.0.1; port ${BGP_PORT}; }
            local { ip 127.0.0.1; accept false; }
        }
        session {
            asn { local 65500; remote 64501; }
            family {
                ipv4/unicast { prefix { maximum 10000; } }
            }
        }
    }
    peer ixp-member-b {
        connection {
            remote { ip 127.0.0.2; }
            local { ip 127.0.0.1; accept false; }
        }
        session {
            asn { local 65500; remote 64502; }
            family {
                ipv4/unicast { prefix { maximum 10000; } }
                ipv6/unicast { prefix { maximum 5000; } }
            }
        }
    }
    peer upstream-transit {
        connection {
            remote { ip 127.0.0.3; }
            local { ip 127.0.0.1; accept false; }
        }
        session {
            asn { local 65500; remote 64503; }
            family {
                ipv4/unicast { prefix { maximum 800000; } }
                ipv6/unicast { prefix { maximum 200000; } }
            }
        }
    }
}
CONF

export ze_ssh_port="${SSH_PORT}"
export ze_web_insecure=true

echo "=== Starting mock BGP peer on port $BGP_PORT ==="
"$PROJECT_DIR/bin/ze-test" peer \
    --mode sink \
    --port "$BGP_PORT" \
    --asn 64501 &
PEER_PID=$!
sleep 1

echo "=== Starting ze daemon ==="
"$PROJECT_DIR/bin/ze" --insecure-web --web "$WEB_PORT" "$CONF" &
ZE_PID=$!

echo "Waiting for ze to start..."
for i in $(seq 1 30); do
    if curl -sk -o /dev/null "https://127.0.0.1:$WEB_PORT/" 2>/dev/null; then
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

capture_cli "help-ai" help --ai
capture_cli "doctor" doctor
capture_cli "help" help

echo "=== Capturing browser screenshots ==="

agent-browser close 2>/dev/null || true
export AGENT_BROWSER_IGNORE_HTTPS_ERRORS=1
agent-browser launch --ignore-https-errors 2>/dev/null || true
sleep 1

take_screenshot() {
    local name="$1"
    local url="$2"
    local wait="${3:-2000}"
    echo "  screenshot: $name"
    if ! agent-browser open "$url" 2>/dev/null; then
        echo "  FAILED (navigate): $name"
        return 0
    fi
    sleep "$(echo "$wait / 1000" | bc)"
    agent-browser screenshot "$OUT/${name}.png" 2>/dev/null || \
        echo "  FAILED (capture): $name"
}

# Web UI (HTTPS with self-signed cert)
take_screenshot "web-config-editor" "https://127.0.0.1:$WEB_PORT/config/" 2000
take_screenshot "web-show-bgp-peer" "https://127.0.0.1:$WEB_PORT/show/bgp/peer/" 2000
take_screenshot "web-show-environment" "https://127.0.0.1:$WEB_PORT/show/environment/" 2000
take_screenshot "web-dashboard" "https://127.0.0.1:$WEB_PORT/" 2000

# Looking Glass (HTTP)
take_screenshot "lg-peers" "http://127.0.0.1:$LG_PORT/" 2000
take_screenshot "lg-routes" "http://127.0.0.1:$LG_PORT/lg/search" 2000

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

take_screenshot "chaos-dashboard" "http://127.0.0.1:$CHAOS_PORT/" 3000
take_screenshot "chaos-peers" "http://127.0.0.1:$CHAOS_PORT/peers" 2000

agent-browser close 2>/dev/null || true

echo ""
echo "=== Done ==="
echo "Screenshots saved to: $OUT/"
ls -la "$OUT/"
