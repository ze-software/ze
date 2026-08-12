#!/usr/bin/env python3
"""Process plugin that verifies RPKI decisions from a real FRR peer.

VALIDATES: AC-6 RTR session plus Valid, Invalid, and NotFound route decisions.
PREVENTS: RPKI interop passing without proving received peer routes are selected.
"""

import json
import sys
import time

from ze_api import API, dispatch, wait_for_shutdown

STATUS = "/tmp/rpki-check.json"


def write_status(status, detail):
    with open(STATUS, "w", encoding="utf-8") as fh:
        json.dump({"status": status, "detail": detail}, fh)


def payload(result):
    """The `data` field of a dispatch result, as a dict, whichever shape it has.

    `dispatch` (test/scripts/ze_api.py) hands back the RPC result unchanged, and
    a command that answers `json.RawMessage` arrives already decoded: `data` is
    an object, not a string holding one. `json.loads` on it raises TypeError,
    which is not a JSONDecodeError, so the retry loop below never saw it and the
    plugin died with "the JSON object must be str, bytes or bytearray, not
    dict". Both shapes are accepted here rather than at three call sites.
    """
    data = result.get("data")
    if isinstance(data, dict):
        return data
    return json.loads(data or "{}")


def route_states(result):
    parsed = payload(result)
    routes = {}
    for peer_routes in parsed.get("adj-rib-in", {}).values():
        for route in peer_routes:
            key = route.get("key", "")
            parts = key.split(":")
            if len(parts) >= 2:
                routes[parts[1]] = route.get("validation-state")
    return routes


def main():
    api = API()
    api.declare_done()
    api.wait_for_config()
    api.capability_done()
    api.wait_for_registry()
    api.ready()

    deadline = time.time() + 45
    last = {}
    while time.time() < deadline:
        status = dispatch(api, "show bgp rpki status")
        routes = dispatch(api, "show bgp adj-rib-in")
        last = {"rpki": status, "adj-rib-in": routes}

        try:
            rpki_data = payload(status)
            states = route_states(routes)
        except (json.JSONDecodeError, TypeError):
            time.sleep(1)
            continue

        synced = (
            rpki_data.get("sessions", 0) >= 1
            and rpki_data.get("vrp-count-ipv4", 0) >= 1
        )
        valid_ok = states.get("9.43.0.0/24") == 1
        invalid_ok = "10.43.0.0/24" not in states
        notfound_ok = states.get("11.43.0.0/24") == 2
        if synced and valid_ok and invalid_ok and notfound_ok:
            write_status("ok", {"rpki": rpki_data, "routes": states})
            wait_for_shutdown(timeout=120)
            return

        time.sleep(1)

    write_status("fail", last)
    print("RPKI interop validation failed", file=sys.stderr)
    wait_for_shutdown(timeout=120)


if __name__ == "__main__":
    main()
