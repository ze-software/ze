#!/usr/bin/env python3
"""Simple ExaBGP-style plugin for integration testing.

This plugin reads ExaBGP JSON format from stdin and writes ExaBGP commands
to stdout. Used to test the ZeBGP ExaBGP bridge bidirectional translation.

ExaBGP JSON format (from bridge after translation):
{
  "exabgp": "5.0.0",
  "type": "update",
  "neighbor": {
    "address": {"peer": "10.0.0.1"},
    "asn": {"peer": 65001},
    "direction": "receive",
    "message": {
      "update": {
        "attribute": {"origin": "igp"},
        "announce": {"ipv4 unicast": {"10.0.0.1": [{"nlri": "192.168.1.0/24"}]}}
      }
    }
  }
}

ExaBGP command format (to bridge for translation):
neighbor 10.0.0.1 announce route 10.0.1.0/24 next-hop 10.0.0.2

Test modes (via TEST_MODE env):
- echo: For each update received, announce a route back
- log: Log received JSON to stderr (debugging)
- noop: Exit immediately after first message
"""

import json
import os
import signal
import sys


def log(msg: str) -> None:
    """Log to stderr."""
    print(f"[exabgp_echo] {msg}", file=sys.stderr, flush=True)


def parent_alive() -> bool:
    """Check if parent process is still alive (cross-platform)."""
    ppid = os.getppid()
    # On Linux, orphans get ppid=1 (init)
    # On macOS, orphans may get ppid=1 (launchd) or other values
    # Check if parent process exists
    if ppid <= 1:
        return False
    try:
        # Send signal 0 to check if process exists
        os.kill(ppid, 0)
        return True
    except (OSError, ProcessLookupError):
        return False


def main() -> None:
    test_mode = os.environ.get("TEST_MODE", "echo")
    log(f"starting, mode={test_mode}")

    # Read lines from stdin until EOF or parent dies
    counter = 0
    while parent_alive():
        try:
            line = sys.stdin.readline()
            if not line:
                counter += 1
                if counter > 100:
                    break
                continue

            line = line.strip()
            if not line:
                counter += 1
                if counter > 100:
                    break
                continue

            counter = 0
            log(f"received: {line[:100]}...")

            # Parse JSON
            try:
                data = json.loads(line)
            except json.JSONDecodeError as e:
                log(f"json parse error: {e}")
                continue

            # Check for shutdown
            if data.get("type") == "shutdown":
                log("received shutdown")
                break

            # Handle based on mode
            if test_mode == "noop":
                log("noop mode, exiting")
                break

            if test_mode == "log":
                log(f"full json: {json.dumps(data, indent=2)}")

            # Process message types
            msg_type = data.get("type", "")

            # Echo mode: respond to updates with commands
            if test_mode == "echo" and msg_type == "update":
                # Get peer address from ExaBGP format:
                # neighbor.address.peer
                neighbor = data.get("neighbor", {})
                addr_info = neighbor.get("address", {})
                peer = addr_info.get("peer", "127.0.0.1")

                # ExaBGP JSON structure for update:
                # neighbor.message.update.announce/withdraw
                message = neighbor.get("message", {})
                update = message.get("update", {})

                # Check for announces (nested format):
                # announce: {"ipv4 unicast": {"10.0.0.1": [{"nlri": "192.168.1.0/24"}]}}
                announce = update.get("announce", {})
                for family, nh_map in announce.items():
                    if isinstance(nh_map, dict):
                        for next_hop, nlri_list in nh_map.items():
                            # Use actual next-hop from JSON, fallback to documentation address
                            nh = next_hop if next_hop != "null" else "192.0.2.1"
                            if isinstance(nlri_list, list):
                                for nlri_entry in nlri_list:
                                    if isinstance(nlri_entry, dict):
                                        prefix = nlri_entry.get("nlri", "")
                                    else:
                                        prefix = str(nlri_entry)
                                    if prefix:
                                        cmd = f"neighbor {peer} announce route {prefix} next-hop {nh}"
                                        log(f"sending: {cmd}")
                                        print(cmd, flush=True)

                # Check for withdrawals (similar nested format):
                # withdraw: {"ipv4 unicast": [{"nlri": "192.168.1.0/24"}]}
                withdraw = update.get("withdraw", {})
                for family, nlri_list in withdraw.items():
                    if isinstance(nlri_list, list):
                        for nlri_entry in nlri_list:
                            if isinstance(nlri_entry, dict):
                                prefix = nlri_entry.get("nlri", "")
                            else:
                                prefix = str(nlri_entry)
                            if prefix:
                                cmd = f"neighbor {peer} withdraw route {prefix}"
                                log(f"sending: {cmd}")
                                print(cmd, flush=True)

            # All modes log state and notification messages
            if msg_type == "state":
                # State is in neighbor.state for ExaBGP format
                neighbor = data.get("neighbor", {})
                state = neighbor.get("state", data.get("state", "unknown"))
                log(f"state change: {state}")

            elif msg_type == "notification":
                neighbor = data.get("neighbor", {})
                notif = neighbor.get("notification", {})
                log(f"notification: code={notif.get('code', 'unknown')}")

        except KeyboardInterrupt:
            break
        except OSError:
            break

    log("exiting")


if __name__ == "__main__":
    signal.signal(signal.SIGPIPE, signal.SIG_DFL)
    main()
