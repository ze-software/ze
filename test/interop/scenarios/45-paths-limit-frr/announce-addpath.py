#!/usr/bin/env python3
"""Announce multiple paths for the same prefix to test PATHS-LIMIT."""

import json
import sys
import time

from ze_api import API


def main():
    api = API()
    api.declare_done()
    api.wait_for_config()
    api.capability_done()
    api.wait_for_registry()
    api.ready()
    api.wait_for_post_startup(timeout=10.0)

    time.sleep(2)

    # Announce 3 paths for the same prefix with different path-info
    for i in range(1, 4):
        api.command(
            f"peer * update text path-information 0.0.0.{i} "
            f"origin igp next-hop 10.10.0.{i} "
            f"nlri ipv4/unicast add 10.10.0.0/24"
        )
        time.sleep(0.5)

    time.sleep(5)
    api.command("request shutdown")
    api.wait_for_shutdown()


if __name__ == "__main__":
    main()
